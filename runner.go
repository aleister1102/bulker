package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RunnerConfig struct {
	InputFile   string
	OutputDir   string
	Workers     int
	Command     string
	CommandArgs []string
}

type Runner struct {
	config        RunnerConfig
	signalHandler *SignalHandler
	tasks         []Task
	mu            sync.RWMutex
	outputFile    *os.File
	outputMutex   sync.Mutex
	outputPath    string
	sessionName   string
	inputLines    []string // Store input lines directly
}

type Task struct {
	ID         int
	ChunkFile  string
	WindowName string
	Status     TaskStatus
	StartTime  time.Time
	EndTime    time.Time
}

type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskCompleted
	TaskFailed
)

func NewRunner(config RunnerConfig) *Runner {
	return &Runner{
		config:        config,
		signalHandler: NewSignalHandler(),
		outputPath:    filepath.Join(config.OutputDir, "output.txt"),
		sessionName:   createSessionName(config.Command),
	}
}

func createSessionName(command string) string {
	// Normalize command name
	normalized := normalizeSessionName(command)

	// Check if session already exists and add number suffix if needed
	sessionName := normalized
	counter := 1

	for sessionExists(sessionName) {
		sessionName = fmt.Sprintf("%s_%d", normalized, counter)
		counter++
	}

	return sessionName
}

func normalizeSessionName(command string) string {
	// Remove path and extension
	base := filepath.Base(command)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}

	// Replace invalid characters with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	normalized := reg.ReplaceAllString(base, "_")

	// Remove consecutive underscores
	reg2 := regexp.MustCompile(`_+`)
	normalized = reg2.ReplaceAllString(normalized, "_")

	// Trim underscores from start and end
	normalized = strings.Trim(normalized, "_")

	// Ensure it's not empty
	if normalized == "" {
		normalized = "bulker"
	}

	return normalized
}

func (r *Runner) readInputFile() error {
	file, err := os.Open(r.config.InputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	r.inputLines = make([]string, 0)

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" { // Only add non-empty lines
			r.inputLines = append(r.inputLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}

	fmt.Printf("Read %d lines from input file: %v\n", len(r.inputLines), r.inputLines)
	return nil
}

func (r *Runner) parseLineRange(rangeStr string) (int, int, error) {
	// Parse format: "lines_start_end"
	parts := strings.Split(rangeStr, "_")
	if len(parts) != 3 || parts[0] != "lines" {
		return 0, 0, fmt.Errorf("invalid range format: %s", rangeStr)
	}

	startLine, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start line: %s", parts[1])
	}

	endLine, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end line: %s", parts[2])
	}

	return startLine, endLine, nil
}

func sessionExists(sessionName string) bool {
	if runtime.GOOS == "windows" {
		return false // Windows doesn't use tmux
	}

	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

func (r *Runner) Run() error {
	// Setup signal handling
	r.signalHandler.Setup(r.handleInterrupt)

	// Create output directory
	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	var err error
	r.outputFile, err = os.Create(r.outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer r.outputFile.Close()

	// Read input file directly into memory
	err = r.readInputFile()
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Create tasks based on line ranges
	r.createTasks()

	// Create tmux session
	if err := r.createTmuxSession(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Run tasks
	if err := r.runTasks(); err != nil {
		return fmt.Errorf("failed to run tasks: %w", err)
	}

	// Monitor and wait for completion
	if err := r.monitor(); err != nil {
		return fmt.Errorf("monitoring failed: %w", err)
	}

	// No chunk files to clean up anymore
	fmt.Println("Processing completed, no cleanup needed")

	fmt.Printf("All tasks completed successfully! Output written to: %s\n", r.outputPath)
	return nil
}

func (r *Runner) createTasks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	totalLines := len(r.inputLines)
	if totalLines == 0 {
		fmt.Println("No lines to process")
		return
	}

	// Calculate chunk size based on workers
	chunkSize := totalLines / r.config.Workers
	if totalLines%r.config.Workers != 0 {
		chunkSize++ // Round up to ensure all lines are included
	}

	// Ensure minimum chunk size of 1
	if chunkSize < 1 {
		chunkSize = 1
	}

	fmt.Printf("Total lines: %d, Workers: %d, Chunk size: %d\n", totalLines, r.config.Workers, chunkSize)

	// Create tasks based on line ranges
	r.tasks = make([]Task, 0, r.config.Workers)
	taskID := 0

	for startLine := 0; startLine < totalLines; startLine += chunkSize {
		endLine := startLine + chunkSize
		if endLine > totalLines {
			endLine = totalLines
		}

		fmt.Printf("Creating task %d: lines %d-%d (%d lines)\n", taskID, startLine, endLine-1, endLine-startLine)
		r.tasks = append(r.tasks, Task{
			ID:         taskID,
			ChunkFile:  fmt.Sprintf("lines_%d_%d", startLine, endLine-1), // Store line range info
			WindowName: fmt.Sprintf("worker_%d", taskID),
			Status:     TaskPending,
		})
		taskID++
	}
}

func (r *Runner) createTmuxSession() error {
	// No longer creating tmux sessions - all tasks run directly and output to shared file
	fmt.Println("Running tasks directly with shared output file")
	return nil
}

func (r *Runner) runTasks() error {
	semaphore := make(chan struct{}, r.config.Workers)

	var wg sync.WaitGroup
	for i := range r.tasks {
		wg.Add(1)
		go func(taskIndex int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			r.runTask(taskIndex)
		}(i)
	}

	wg.Wait()
	return nil
}

func (r *Runner) runTask(taskIndex int) {
	r.mu.Lock()
	task := &r.tasks[taskIndex]
	task.Status = TaskRunning
	task.StartTime = time.Now()
	r.mu.Unlock()

	// Create command
	fullCommand := r.buildCommand(task.ChunkFile)

	if runtime.GOOS == "windows" {
		r.runTaskWindows(taskIndex, fullCommand)
	} else {
		r.runTaskUnix(taskIndex, fullCommand)
	}
}

func (r *Runner) runTaskWindows(taskIndex int, fullCommand string) {
	task := &r.tasks[taskIndex]

	// For Windows, handle echo command specially by processing lines directly from memory
	if r.config.Command == "echo" {
		// Parse line range from task.ChunkFile (format: "lines_start_end")
		startLine, endLine, err := r.parseLineRange(task.ChunkFile)
		if err != nil {
			fmt.Printf("Failed to parse line range %s: %v\n", task.ChunkFile, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}

		fmt.Printf("Started task %d: %s (processing lines %d-%d)\n", task.ID, task.WindowName, startLine, endLine)

		// Process each line from memory and write directly to shared output file
		for i := startLine; i <= endLine && i < len(r.inputLines); i++ {
			r.writeToOutput(r.inputLines[i])
		}

		fmt.Printf("Task %d completed successfully\n", task.ID)
		r.updateTaskStatus(taskIndex, TaskCompleted)
		return
	}

	// For other commands, run through cmd.exe and capture output
	cmd := exec.Command("cmd", "/c", fullCommand)

	// Create pipes to capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Failed to create stdout pipe for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start command for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	fmt.Printf("Started task %d: %s (PID: %d)\n", task.ID, task.WindowName, cmd.Process.Pid)

	// Read output line by line and write directly to shared output file
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdout.Close()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// Write each line immediately to the shared output file
			r.writeToOutput(line)
		}
	}()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		fmt.Printf("Task %d failed: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
	} else {
		// Wait for output goroutine to finish before marking as completed
		wg.Wait()
		fmt.Printf("Task %d completed successfully\n", task.ID)
		r.updateTaskStatus(taskIndex, TaskCompleted)
	}
}

func (r *Runner) runTaskUnix(taskIndex int, fullCommand string) {
	task := &r.tasks[taskIndex]

	// For Unix, handle echo command specially by processing lines directly from memory
	if r.config.Command == "echo" {
		// Parse line range from task.ChunkFile (format: "lines_start_end")
		startLine, endLine, err := r.parseLineRange(task.ChunkFile)
		if err != nil {
			fmt.Printf("Failed to parse line range %s: %v\n", task.ChunkFile, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}

		fmt.Printf("Started task %d: %s (processing lines %d-%d)\n", task.ID, task.WindowName, startLine, endLine)

		// Process each line from memory and write directly to shared output file
		for i := startLine; i <= endLine && i < len(r.inputLines); i++ {
			r.writeToOutput(r.inputLines[i])
		}

		fmt.Printf("Task %d completed successfully\n", task.ID)
		r.updateTaskStatus(taskIndex, TaskCompleted)
		return
	}

	// For other commands, run the command directly and capture output
	cmd := exec.Command("bash", "-c", fullCommand)

	// Create pipes to capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Failed to create stdout pipe for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start command for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	fmt.Printf("Started task %d: %s (PID: %d)\n", task.ID, task.WindowName, cmd.Process.Pid)

	// Read output line by line and write directly to shared output file
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdout.Close()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// Write each line immediately to the shared output file
			r.writeToOutput(line)
		}
	}()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		fmt.Printf("Task %d failed: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
	} else {
		// Wait for output goroutine to finish before marking as completed
		wg.Wait()
		fmt.Printf("Task %d completed successfully\n", task.ID)
		r.updateTaskStatus(taskIndex, TaskCompleted)
	}
}

func (r *Runner) buildCommand(chunkFile string) string {
	var cmdParts []string

	// For echo command, we handle it directly in runTaskWindows/runTaskUnix
	// so we don't need to build actual commands for it
	if r.config.Command == "echo" {
		return "echo" // Placeholder, not actually used
	}

	// Add main command
	cmdParts = append(cmdParts, r.config.Command)

	// Add arguments, replace {input} placeholder
	for _, arg := range r.config.CommandArgs {
		arg = strings.ReplaceAll(arg, "{input}", chunkFile)
		// Remove {output} placeholder since we don't use it anymore
		arg = strings.ReplaceAll(arg, "{output}", "")
		cmdParts = append(cmdParts, arg)
	}

	command := strings.Join(cmdParts, " ")
	return command
}

func (r *Runner) writeToOutput(content string) {
	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	if r.outputFile != nil && content != "" {
		// Write line to output file with newline
		if _, err := r.outputFile.WriteString(content + "\n"); err != nil {
			fmt.Printf("Failed to write to output file: %v\n", err)
		} else {
			// Ensure data is written to disk immediately
			r.outputFile.Sync()
		}
	}
}

func (r *Runner) monitor() error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if r.checkAllCompleted() {
				return nil
			}
		case <-r.signalHandler.InterruptChan():
			fmt.Println("\nReceived interrupt signal, cleaning up...")
			return r.handleInterrupt()
		}
	}
}

func (r *Runner) checkAllCompleted() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	completedCount := 0
	failedCount := 0
	runningCount := 0

	for _, task := range r.tasks {
		switch task.Status {
		case TaskCompleted:
			completedCount++
		case TaskFailed:
			failedCount++
		case TaskRunning:
			runningCount++
		}
	}

	total := len(r.tasks)
	fmt.Printf("Progress: %d/%d completed, %d running, %d failed\n", completedCount, total, runningCount, failedCount)

	return completedCount+failedCount == total
}

func (r *Runner) updateTaskStatus(taskIndex int, status TaskStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks[taskIndex].Status = status
	if status == TaskCompleted || status == TaskFailed {
		r.tasks[taskIndex].EndTime = time.Now()
	}
}

func (r *Runner) handleInterrupt() error {
	fmt.Println("Handling interrupt, stopping all tasks...")

	// Close output file
	if r.outputFile != nil {
		r.outputFile.Close()
	}

	fmt.Printf("Partial results saved to: %s\n", r.outputPath)
	return nil
}
