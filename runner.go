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
	OutputFile  string
	Workers     int
	Command     string
	CommandArgs []string
}

type Runner struct {
	config        RunnerConfig
	signalHandler *SignalHandler
	strategy      ToolStrategy
	tasks         []Task
	mu            sync.RWMutex
	outputFile    *os.File
	outputMutex   sync.Mutex
	outputPath    string
	sessionName   string
	inputLines    []string // Store input lines directly
	// Performance tracking
	startTime       time.Time
	endTime         time.Time
	initialMemStats runtime.MemStats
	finalMemStats   runtime.MemStats
}

type Task struct {
	ID         int
	InputData  string
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
		strategy:      GetToolStrategy(config.Command),
		outputPath:    config.OutputFile,
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

	LogInfo("Read %d lines from input file", len(r.inputLines))
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
	// Start performance tracking
	r.startTime = time.Now()
	runtime.ReadMemStats(&r.initialMemStats)

	// Setup signal handling
	r.signalHandler.Setup(r.handleInterrupt)

	// Backup existing output file if it exists
	if err := r.backupOutputFile(); err != nil {
		return fmt.Errorf("failed to backup output file: %w", err)
	}

	// Create output directory if needed
	outputDir := filepath.Dir(r.config.OutputFile)
	if outputDir != "." && outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
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

	// Setup tool strategy
	if err := r.setupToolStrategy(); err != nil {
		return fmt.Errorf("failed to setup tool strategy: %w", err)
	}

	// Run tasks
	if err := r.runTasks(); err != nil {
		return fmt.Errorf("failed to run tasks: %w", err)
	}

	// Monitor and wait for completion
	if err := r.monitor(); err != nil {
		return fmt.Errorf("monitoring failed: %w", err)
	}

	// End performance tracking
	r.endTime = time.Now()
	runtime.ReadMemStats(&r.finalMemStats)

	// Processing completed
	LogInfo("Processing completed")

	LogSuccess("All tasks completed successfully! Output written to: %s", r.outputPath)

	// Display performance metrics
	r.displayPerformanceMetrics()

	return nil
}

func (r *Runner) backupOutputFile() error {
	if _, err := os.Stat(r.outputPath); os.IsNotExist(err) {
		// File doesn't exist, no need to backup
		return nil
	} else if err != nil {
		// Other error
		return err
	}

	// File exists, create backup name
	timestamp := time.Now().Format("20060102_150405")
	ext := filepath.Ext(r.outputPath)
	base := strings.TrimSuffix(r.outputPath, ext)
	backupPath := fmt.Sprintf("%s_%s%s", base, timestamp, ext)

	LogInfo("Output file %s exists. Backing up to %s", r.outputPath, backupPath)

	return os.Rename(r.outputPath, backupPath)
}

func (r *Runner) createTasks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	totalLines := len(r.inputLines)
	if totalLines == 0 {
		LogWarn("No lines to process")
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

	LogInfo("Total lines: %d, Workers: %d, Chunk size: %d", totalLines, r.config.Workers, chunkSize)

	// Create tasks based on line ranges
	r.tasks = make([]Task, 0, r.config.Workers)
	taskID := 0

	for startLine := 0; startLine < totalLines; startLine += chunkSize {
		endLine := startLine + chunkSize
		if endLine > totalLines {
			endLine = totalLines
		}

		LogInfo("Creating task %d: lines %d-%d (%d lines)", taskID, startLine, endLine-1, endLine-startLine)
		r.tasks = append(r.tasks, Task{
			ID:         taskID,
			InputData:  fmt.Sprintf("lines_%d_%d", startLine, endLine-1), // Store line range info
			WindowName: fmt.Sprintf("worker_%d", taskID),
			Status:     TaskPending,
		})
		taskID++
	}
}

func (r *Runner) setupToolStrategy() error {
	// Setup tool strategy for processing
	LogInfo("Setup tool strategy for %s", r.config.Command)
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

	var tempOutputFile string
	cleanupFunc := func() {
		if tempOutputFile != "" {
			// Read content from temp file, write to main output, then delete temp file
			content, err := os.ReadFile(tempOutputFile)
			if err != nil && !os.IsNotExist(err) {
				LogError("Failed to read temp output file %s: %v", tempOutputFile, err)
			} else if err == nil {
				// Trim null bytes or other non-printable chars from the content
				trimmedContent := strings.Trim(string(content), "\\x00")
				r.writeToOutput(trimmedContent)
			}
			os.Remove(tempOutputFile)
		}
		// Cleanup input chunk file
		if r.strategy.NeedsFileChunk() {
			startLine, endLine, err := r.parseLineRange(task.InputData)
			if err == nil {
				inputData, err := r.strategy.PrepareInput(r.inputLines, taskIndex, startLine, endLine)
				if err == nil {
					r.strategy.Cleanup(inputData)
				}
			}
		}
	}
	defer cleanupFunc()

	// Use strategy pattern
	if fileOutputStrategy, ok := r.strategy.(FileOutputStrategy); ok && fileOutputStrategy.HandlesFileOutput(r.config.CommandArgs) {
		startLine, endLine, err := r.parseLineRange(task.InputData)
		if err != nil {
			LogError("Failed to parse line range %s: %v", task.InputData, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}

		inputData, err := fileOutputStrategy.PrepareInput(r.inputLines, taskIndex, startLine, endLine)
		if err != nil {
			LogError("Failed to prepare input for task %d: %v", task.ID, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}

		var cmdParts []string
		cmdParts, tempOutputFile = fileOutputStrategy.BuildCommandWithFileOutput(inputData, r.config.CommandArgs, task.ID)
		r.runTaskWithCommand(taskIndex, cmdParts, true)

	} else if r.strategy.NeedsFileChunk() {
		// Tạo file chunk cho tools như httpx
		startLine, endLine, err := r.parseLineRange(task.InputData)
		if err != nil {
			LogError("Failed to parse line range %s: %v", task.InputData, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}

		inputData, err := r.strategy.PrepareInput(r.inputLines, taskIndex, startLine, endLine)
		if err != nil {
			LogError("Failed to prepare input for task %d: %v", task.ID, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}

		// Build command using strategy
		cmdParts := r.strategy.BuildCommand(inputData, r.config.CommandArgs)
		r.runTaskWithCommand(taskIndex, cmdParts, false)
	} else {
		// Xử lý trực tiếp cho echo
		r.runTaskDirect(taskIndex)
	}
}

func (r *Runner) writeToOutput(content string) {
	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	if r.outputFile != nil && content != "" {
		// Write line to output file with newline
		if _, err := r.outputFile.WriteString(content + "\n"); err != nil {
			LogError("Failed to write to output file: %v", err)
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
			LogWarn("Received interrupt signal, cleaning up...")
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
	LogInfo("Progress: %d/%d completed, %d running, %d failed", completedCount, total, runningCount, failedCount)

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
	LogInfo("Handling interrupt, stopping all tasks...")

	// Close output file
	if r.outputFile != nil {
		r.outputFile.Close()
	}

	LogInfo("Partial results saved to: %s", r.outputPath)
	return nil
}

func (r *Runner) displayPerformanceMetrics() {
	duration := r.endTime.Sub(r.startTime)

	// Calculate memory usage
	memUsed := r.finalMemStats.Alloc - r.initialMemStats.Alloc
	memUsedMB := float64(memUsed) / 1024 / 1024

	// Peak memory usage
	peakMemMB := float64(r.finalMemStats.Sys) / 1024 / 1024

	LogPerf("=== Performance Metrics ===")
	LogPerf("Total execution time: %v", duration)
	LogPerf("Memory allocated: %.2f MB", memUsedMB)
	LogPerf("Peak memory usage: %.2f MB", peakMemMB)
	LogPerf("Total goroutines: %d", runtime.NumGoroutine())

	// Task statistics
	r.mu.RLock()
	completedCount := 0
	failedCount := 0
	var totalTaskTime time.Duration

	for _, task := range r.tasks {
		switch task.Status {
		case TaskCompleted:
			completedCount++
			if !task.EndTime.IsZero() && !task.StartTime.IsZero() {
				totalTaskTime += task.EndTime.Sub(task.StartTime)
			}
		case TaskFailed:
			failedCount++
		}
	}
	r.mu.RUnlock()

	LogPerf("Tasks completed: %d", completedCount)
	LogPerf("Tasks failed: %d", failedCount)
	LogPerf("Total task time: %v", totalTaskTime)

	if completedCount > 0 {
		avgTaskTime := totalTaskTime / time.Duration(completedCount)
		LogPerf("Average task time: %v", avgTaskTime)
	}

	LogPerf("===========================")
}

// runTaskDirect xử lý trực tiếp cho echo command
func (r *Runner) runTaskDirect(taskIndex int) {
	r.mu.RLock()
	task := &r.tasks[taskIndex]
	r.mu.RUnlock()

	// Parse line range from task.InputData (format: "lines_start_end")
	startLine, endLine, err := r.parseLineRange(task.InputData)
	if err != nil {
		LogError("Failed to parse line range %s: %v", task.InputData, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	LogTask(task.ID, "Started: %s (processing lines %d-%d)", task.WindowName, startLine, endLine)

	// Process each line from memory and write directly to shared output file
	for i := startLine; i <= endLine && i < len(r.inputLines); i++ {
		r.writeToOutput(r.inputLines[i])
	}

	LogTask(task.ID, "completed successfully")
	r.updateTaskStatus(taskIndex, TaskCompleted)
}

// runTaskWithCommand chạy command với external tools
func (r *Runner) runTaskWithCommand(taskIndex int, cmdParts []string, ignoreStdout bool) {
	r.mu.RLock()
	task := &r.tasks[taskIndex]
	r.mu.RUnlock()

	// Create command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		fullCommand := strings.Join(cmdParts, " ")
		LogInfo("Running command: cmd /c %s", fullCommand)
		cmd = exec.Command("cmd", "/c", fullCommand)
	} else {
		fullCommand := strings.Join(cmdParts, " ")
		LogInfo("Running command: bash -c %s", fullCommand)
		cmd = exec.Command("bash", "-c", fullCommand)
	}

	// Create pipes to capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		LogError("Failed to create stdout pipe for task %d: %v", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		LogError("Failed to create stderr pipe for task %d: %v", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		LogError("Failed to start command for task %d: %v", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	LogTask(task.ID, "Started: %s (PID: %d)", task.WindowName, cmd.Process.Pid)

	// Read output line by line and write directly to shared output file
	var wg sync.WaitGroup

	if !ignoreStdout {
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
	} else {
		// Khi tool tự quản lý output, vẫn hiển thị stdout cho user xem progress
		go func() {
			defer stdout.Close()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				// Hiển thị trực tiếp stdout của tool ra console
				fmt.Println(line)
			}
		}()
	}

	// Capture stderr và hiển thị realtime
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stderr.Close()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Hiển thị stderr realtime để user biết có lỗi gì
			LogTask(task.ID, "[STDERR] %s", line)
		}
	}()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		LogError("Task %d failed: %v", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
	} else {
		// Wait for output goroutine to finish before marking as completed
		wg.Wait()
		LogTask(task.ID, "completed successfully")
		r.updateTaskStatus(taskIndex, TaskCompleted)
	}
}
