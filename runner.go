package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	ConfigFile  string
	Wordlist    string
}

type Runner struct {
	config        RunnerConfig
	signalHandler *SignalHandler
	configManager *ConfigManager
	toolConfig    ToolConfig
	tasks         []Task
	mu            sync.RWMutex
	outputFile    *os.File
	outputMutex   sync.Mutex
	outputPath    string
	inputLines    []string // Store input lines directly
	cancelChan    chan struct{}
	cancelOnce    sync.Once
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

func NewRunner(config RunnerConfig) (*Runner, error) {
	configManager, err := NewConfigManager(config.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("could not load config file: %v", err)
	}

	var toolConfig ToolConfig
	var exists bool

	toolConfig, exists = configManager.GetToolConfig(config.Command)
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found in config file '%s'", config.Command, config.ConfigFile)
	}

	return &Runner{
		config:        config,
		signalHandler: NewSignalHandler(),
		configManager: configManager,
		toolConfig:    toolConfig,
		outputPath:    config.OutputFile,
		cancelChan:    make(chan struct{}),
	}, nil
}

func (r *Runner) readInputFile() error {
	var scanner *bufio.Scanner

	// If no input file is specified, read from stdin
	if r.config.InputFile == "" {
		LogInfo("Reading input from stdin")
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		file, err := os.Open(r.config.InputFile)
		if err != nil {
			return fmt.Errorf("failed to open input file: %w", err)
		}
		defer file.Close()
		scanner = bufio.NewScanner(file)
	}

	r.inputLines = make([]string, 0)

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" { // Only add non-empty lines
			r.inputLines = append(r.inputLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	LogInfo("Read %d lines of input", len(r.inputLines))
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

func (r *Runner) Run() error {
	// Start performance tracking
	r.startTime = time.Now()
	runtime.ReadMemStats(&r.initialMemStats)

	// Setup signal handling
	r.signalHandler.Setup(r.handleInterrupt)
	defer r.signalHandler.Stop()

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
	// Write header if defined in config
	if r.toolConfig.Header != "" {
		r.outputFile.WriteString(r.toolConfig.Header + "\n")
	}
	defer func() {
		if r.outputFile != nil {
			r.outputFile.Sync()
			r.outputFile.Close()
		}
	}()

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

	r.tasks = make([]Task, 0)

	switch r.toolConfig.Mode {
	case "multiple":
		// Chia input thành các chunks, mỗi chunk là một task
		chunkSize := totalLines / r.config.Workers
		if totalLines%r.config.Workers != 0 {
			chunkSize++
		}
		if chunkSize < 1 {
			chunkSize = 1
		}
		LogInfo("Total lines: %d, Workers: %d, Chunk size: %d", totalLines, r.config.Workers, chunkSize)
		taskID := 0
		for startLine := 0; startLine < totalLines; startLine += chunkSize {
			endLine := startLine + chunkSize
			if endLine > totalLines {
				endLine = totalLines
			}
			LogInfo("Creating task %d: lines %d-%d", taskID, startLine, endLine-1)
			r.tasks = append(r.tasks, Task{
				ID:         taskID,
				InputData:  fmt.Sprintf("lines_%d_%d", startLine, endLine-1),
				WindowName: fmt.Sprintf("worker_%d", taskID),
				Status:     TaskPending,
			})
			taskID++
		}
	case "single":
		// Mỗi dòng là một task
		LogInfo("Total lines: %d, Mode: single. Creating %d tasks.", totalLines, totalLines)
		for i, line := range r.inputLines {
			r.tasks = append(r.tasks, Task{
				ID:         i,
				InputData:  line,
				WindowName: fmt.Sprintf("worker_%d", i),
				Status:     TaskPending,
			})
		}
	default:
		// Sẽ không xảy ra nếu config hợp lệ
		LogError("Invalid tool mode: %s", r.toolConfig.Mode)
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

			select {
			case <-r.cancelChan:
				LogWarn("Task %d cancelled.", r.tasks[taskIndex].ID)
				return
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				r.runTask(taskIndex)
			}
		}(i)
	}

	wg.Wait()
	return nil
}

func (r *Runner) runTask(taskIndex int) {
	// Check if cancelled before starting
	select {
	case <-r.cancelChan:
		LogWarn("Task %d cancelled before start.", taskIndex)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	default:
	}

	r.mu.Lock()
	task := &r.tasks[taskIndex]
	task.Status = TaskRunning
	task.StartTime = time.Now()
	r.mu.Unlock()

	var tempOutputFile string
	var chunkFile string
	var inputData string

	cleanupFunc := func() {
		if tempOutputFile != "" {
			// If the tool writes its main output to stdout, we skip merging the temp file but still remove it.
			if !r.toolConfig.UseStdout {
				content, err := os.ReadFile(tempOutputFile)
				if err == nil {
					// Trim header if it exists
					lines := strings.Split(string(content), "\n")
					var contentToWrite string
					if len(lines) > 0 && r.toolConfig.Header != "" && strings.TrimSpace(lines[0]) == r.toolConfig.Header {
						contentToWrite = strings.Join(lines[1:], "\n")
					} else {
						contentToWrite = string(content)
					}

					trimmedContent := strings.Trim(contentToWrite, "\x00")
					r.writeToOutput(trimmedContent)

				} else if !os.IsNotExist(err) {
					LogError("Failed to read temp output file %s: %v", tempOutputFile, err)
				}
			}
			os.Remove(tempOutputFile)
		}
		if chunkFile != "" {
			os.Remove(chunkFile)
		}
	}
	defer cleanupFunc()

	// Check cancellation again before processing
	select {
	case <-r.cancelChan:
		LogWarn("Task %d cancelled during setup.", taskIndex)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	default:
	}

	// Tất cả các tool đều được xử lý thông qua config

	tempOutputFile = fmt.Sprintf("temp_output_%d.txt", task.ID)

	switch r.toolConfig.Mode {
	case "multiple":
		startLine, endLine, err := r.parseLineRange(task.InputData)
		if err != nil {
			LogError("Failed to parse line range for task %d: %v", task.ID, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}

		chunkFile = fmt.Sprintf("chunk_%d.txt", taskIndex)
		file, err := os.Create(chunkFile)
		if err != nil {
			LogError("Failed to create chunk file for task %d: %v", task.ID, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}
		for i := startLine; i <= endLine && i < len(r.inputLines); i++ {
			if _, err := file.WriteString(r.inputLines[i] + "\n"); err != nil {
				file.Close()
				LogError("Failed to write to chunk file for task %d: %v", task.ID, err)
				r.updateTaskStatus(taskIndex, TaskFailed)
				return
			}
		}
		file.Close()
		inputData = chunkFile

	case "single":
		inputData = task.InputData

	default:
		LogError("Unknown tool mode: %s", r.toolConfig.Mode)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	cmdParts, err := r.configManager.BuildCommand(r.config.Command, inputData, r.config.CommandArgs, tempOutputFile, r.config.Wordlist)
	if err != nil {
		LogError("Failed to build command for task %d: %v", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	// Decide whether to capture stdout based on tool configuration
	ignoreStdout := !r.toolConfig.UseStdout
	r.runTaskWithCommand(taskIndex, cmdParts, ignoreStdout)
}

func (r *Runner) writeToOutput(content string) {
	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	if r.outputFile != nil && content != "" {
		// Content already has newlines handled by the cleanup function
		if _, err := r.outputFile.WriteString(content); err != nil {
			LogError("Failed to write to output file: %v", err)
		} else {
			// Ensure data is written to disk immediately
			r.outputFile.Sync()
		}
	}
}

func (r *Runner) monitor() error {
	ticker := time.NewTicker(1 * time.Second) // Check more frequently
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if r.checkAllCompleted() {
				return nil
			}
		case <-r.signalHandler.InterruptChan():
			LogWarn("Received interrupt signal, cleaning up...")
			r.cancelTasks()
			return r.handleInterrupt()
		case <-r.cancelChan:
			LogWarn("Cancellation signal received, waiting for tasks to terminate...")
			// Wait a bit for tasks to notice cancellation before returning
			time.Sleep(2 * time.Second)
			return nil
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

func (r *Runner) cancelTasks() {
	r.cancelOnce.Do(func() {
		close(r.cancelChan)
	})
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

	// Cancel all running tasks
	r.cancelTasks()

	// Wait for tasks to finish gracefully (with timeout)
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			LogWarn("Timeout waiting for tasks to finish, forcing shutdown...")
			goto cleanup
		case <-ticker.C:
			if r.checkAllTasksStopped() {
				LogInfo("All tasks stopped gracefully")
				goto cleanup
			}
		}
	}

cleanup:
	// Close output file
	if r.outputFile != nil {
		r.outputFile.Sync() // Ensure all data is written
		r.outputFile.Close()
		r.outputFile = nil
	}

	LogInfo("Partial results saved to: %s", r.outputPath)
	return nil
}

func (r *Runner) checkAllTasksStopped() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, task := range r.tasks {
		if task.Status == TaskRunning {
			return false
		}
	}
	return true
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

// runTaskWithCommand chạy command với external tools
func (r *Runner) runTaskWithCommand(taskIndex int, cmdParts []string, ignoreStdout bool) {
	r.mu.RLock()
	task := &r.tasks[taskIndex]
	r.mu.RUnlock()

	// Check cancellation before starting command
	select {
	case <-r.cancelChan:
		LogWarn("Task %d cancelled before command execution.", task.ID)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	default:
	}

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

	// Channel to signal goroutines to stop
	done := make(chan struct{})
	defer close(done)

	if !ignoreStdout {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer stdout.Close()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				select {
				case <-done:
					return
				case <-r.cancelChan:
					return
				default:
					line := scanner.Text()
					// Write each line immediately to the shared output file, preserving line breaks
					r.writeToOutput(line + "\n")
				}
			}
		}()
	} else {
		// Khi tool tự quản lý output, vẫn hiển thị stdout cho user xem progress
		go func() {
			defer stdout.Close()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				select {
				case <-done:
					return
				case <-r.cancelChan:
					return
				default:
					line := scanner.Text()
					// Hiển thị trực tiếp stdout của tool ra console
					fmt.Println(line)
				}
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
			select {
			case <-done:
				return
			case <-r.cancelChan:
				return
			default:
				line := scanner.Text()
				// Hiển thị stderr realtime để user biết có lỗi gì
				LogTask(task.ID, "[STDERR] %s", line)
			}
		}
	}()

	// Monitor for cancellation and kill process if needed
	go func() {
		select {
		case <-r.cancelChan:
			if cmd.Process != nil {
				LogWarn("Killing process %d for task %d due to cancellation", cmd.Process.Pid, task.ID)
				cmd.Process.Kill()
			}
		case <-done:
			// Command finished naturally
		}
	}()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		// Check if error is due to cancellation
		select {
		case <-r.cancelChan:
			LogWarn("Task %d was cancelled", task.ID)
			r.updateTaskStatus(taskIndex, TaskFailed)
		default:
			LogError("Task %d failed: %v", task.ID, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			// Signal other tasks to cancel only if it's not already cancelled
			r.cancelTasks()
		}
	} else {
		// Wait for output goroutine to finish before marking as completed
		wg.Wait()
		LogTask(task.ID, "completed successfully")
		r.updateTaskStatus(taskIndex, TaskCompleted)
	}
}
