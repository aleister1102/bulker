package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	CleanupMode bool
}

type Runner struct {
	config        RunnerConfig
	splitter      *FileSplitter
	signalHandler *SignalHandler
	tasks         []Task
	mu            sync.RWMutex
	outputFile    *os.File
	outputMutex   sync.Mutex
	outputPath    string
	sessionName   string
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
		splitter:      NewFileSplitter(config.InputFile, config.OutputDir, config.Workers),
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

	if r.config.CleanupMode {
		fmt.Println("Running in cleanup mode...")
		return r.cleanup()
	}

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

	// Split input file into chunks
	chunkFiles, err := r.splitter.Split()
	if err != nil {
		return fmt.Errorf("failed to split input file: %w", err)
	}

	// Create tasks
	r.createTasks(chunkFiles)

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

	fmt.Printf("All tasks completed successfully! Output written to: %s\n", r.outputPath)
	return nil
}

func (r *Runner) createTasks(chunkFiles []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks = make([]Task, len(chunkFiles))
	for i, chunkFile := range chunkFiles {
		r.tasks[i] = Task{
			ID:         i,
			ChunkFile:  chunkFile,
			WindowName: fmt.Sprintf("worker_%d", i),
			Status:     TaskPending,
		}
	}
}

func (r *Runner) createTmuxSession() error {
	if runtime.GOOS == "windows" {
		fmt.Println("Windows detected, using background processes instead of tmux")
		return nil
	}

	// Kill existing session if exists
	exec.Command("tmux", "kill-session", "-t", r.sessionName).Run()

	// Create new session
	cmd := exec.Command("tmux", "new-session", "-d", "-s", r.sessionName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	fmt.Printf("Created tmux session: %s\n", r.sessionName)
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

	// Run command through cmd.exe for Windows compatibility
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

	// Read output and write to shared file
	go func() {
		defer stdout.Close()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			r.writeToOutput(line)
		}
	}()

	// Wait for command to complete
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Printf("Task %d failed: %v\n", task.ID, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
		} else {
			fmt.Printf("Task %d completed successfully\n", task.ID)
			r.updateTaskStatus(taskIndex, TaskCompleted)
		}
	}()
}

func (r *Runner) runTaskUnix(taskIndex int, fullCommand string) {
	task := &r.tasks[taskIndex]

	// Create tmux window
	windowCmd := exec.Command("tmux", "new-window", "-t", r.sessionName, "-n", task.WindowName)
	if err := windowCmd.Run(); err != nil {
		fmt.Printf("Failed to create tmux window for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	// Create a pipe to capture output from tmux
	tmpFile := fmt.Sprintf("/tmp/bulker_output_%d_%d.txt", task.ID, time.Now().UnixNano())
	commandWithRedirect := fmt.Sprintf("(%s) > %s 2>&1; echo \"TASK_COMPLETED\" >> %s", fullCommand, tmpFile, tmpFile)

	// Run command in tmux window
	sendCmd := exec.Command("tmux", "send-keys", "-t", fmt.Sprintf("%s:%s", r.sessionName, task.WindowName), commandWithRedirect, "Enter")
	if err := sendCmd.Run(); err != nil {
		fmt.Printf("Failed to send command to tmux window for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	fmt.Printf("Started task %d: %s\n", task.ID, task.WindowName)

	// Monitor the temp file and copy output to shared file
	go func() {
		defer func() {
			if _, err := os.Stat(tmpFile); err == nil {
				os.Remove(tmpFile) // Clean up temp file
			}
		}()

		// Wait for command to complete by checking for completion marker
		for {
			time.Sleep(1 * time.Second)
			if content, err := os.ReadFile(tmpFile); err == nil {
				if strings.Contains(string(content), "TASK_COMPLETED") {
					// Remove the completion marker and write content
					lines := strings.Split(string(content), "\n")
					var outputLines []string
					for _, line := range lines {
						if line != "TASK_COMPLETED" && line != "" {
							outputLines = append(outputLines, line)
						}
					}
					if len(outputLines) > 0 {
						r.writeToOutput(strings.Join(outputLines, "\n"))
					}
					r.updateTaskStatus(taskIndex, TaskCompleted)
					break
				}
			}
		}
	}()
}

func (r *Runner) buildCommand(chunkFile string) string {
	var cmdParts []string

	// Add main command
	cmdParts = append(cmdParts, r.config.Command)

	// Add arguments, replace {input} placeholder
	for _, arg := range r.config.CommandArgs {
		arg = strings.ReplaceAll(arg, "{input}", chunkFile)
		// Remove {output} placeholder since we don't use it anymore
		arg = strings.ReplaceAll(arg, "{output}", "")
		cmdParts = append(cmdParts, arg)
	}

	return strings.Join(cmdParts, " ")
}

func (r *Runner) writeToOutput(content string) {
	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	if r.outputFile != nil {
		if content != "" {
			if _, err := r.outputFile.WriteString(content + "\n"); err != nil {
				fmt.Printf("Failed to write to output file: %v\n", err)
			}
			r.outputFile.Sync() // Ensure data is written to disk
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

	if runtime.GOOS != "windows" {
		// Kill tmux session on Unix systems
		exec.Command("tmux", "kill-session", "-t", r.sessionName).Run()
	}

	// Close output file
	if r.outputFile != nil {
		r.outputFile.Close()
	}

	fmt.Printf("Partial results saved to: %s\n", r.outputPath)
	return nil
}

func (r *Runner) cleanup() error {
	// In cleanup mode, just report the existing output file
	if _, err := os.Stat(r.outputPath); err == nil {
		fmt.Printf("Output file found: %s\n", r.outputPath)
		return nil
	}

	fmt.Println("No output file found for cleanup")
	return nil
}
