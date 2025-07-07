package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type RunnerConfig struct {
	InputFile   string
	OutputDir   string
	Workers     int
	ChunkSize   int
	SessionName string
	Command     string
	CommandArgs []string
	CleanupMode bool
}

type Runner struct {
	config        RunnerConfig
	splitter      *FileSplitter
	collector     *ResultCollector
	signalHandler *SignalHandler
	tasks         []Task
	mu            sync.RWMutex
}

type Task struct {
	ID         int
	ChunkFile  string
	ResultFile string
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
		splitter:      NewFileSplitter(config.InputFile, config.OutputDir, config.ChunkSize),
		collector:     NewResultCollector(config.OutputDir),
		signalHandler: NewSignalHandler(),
	}
}

func (r *Runner) Run() error {
	// Setup signal handling
	r.signalHandler.Setup(r.handleInterrupt)

	if r.config.CleanupMode {
		fmt.Println("Running in cleanup mode...")
		return r.cleanup()
	}

	// Chia file input thành chunks
	chunkFiles, err := r.splitter.Split()
	if err != nil {
		return fmt.Errorf("failed to split input file: %w", err)
	}

	// Tạo tasks
	r.createTasks(chunkFiles)

	// Tạo tmux session
	if err := r.createTmuxSession(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Chạy tasks
	if err := r.runTasks(); err != nil {
		return fmt.Errorf("failed to run tasks: %w", err)
	}

	// Monitor và wait for completion
	if err := r.monitor(); err != nil {
		return fmt.Errorf("monitoring failed: %w", err)
	}

	// Merge results
	if err := r.collector.MergeResults(); err != nil {
		return fmt.Errorf("failed to merge results: %w", err)
	}

	fmt.Println("All tasks completed successfully!")
	return nil
}

func (r *Runner) createTasks(chunkFiles []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks = make([]Task, len(chunkFiles))
	for i, chunkFile := range chunkFiles {
		resultFile := strings.Replace(chunkFile, "chunk_", "result_", 1)
		r.tasks[i] = Task{
			ID:         i,
			ChunkFile:  chunkFile,
			ResultFile: resultFile,
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
	exec.Command("tmux", "kill-session", "-t", r.config.SessionName).Run()

	// Create new session
	cmd := exec.Command("tmux", "new-session", "-d", "-s", r.config.SessionName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

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

	// Tạo command
	fullCommand := r.buildCommand(task.ChunkFile, task.ResultFile)

	if runtime.GOOS == "windows" {
		r.runTaskWindows(taskIndex, fullCommand)
	} else {
		r.runTaskUnix(taskIndex, fullCommand)
	}
}

func (r *Runner) runTaskWindows(taskIndex int, fullCommand string) {
	task := &r.tasks[taskIndex]

	// Parse command và arguments
	cmdParts := strings.Fields(fullCommand)
	if len(cmdParts) == 0 {
		fmt.Printf("Empty command for task %d\n", task.ID)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	// Tạo cmd
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)

	// Redirect output để capture kết quả
	if strings.Contains(fullCommand, ">") {
		// Nếu command có redirect, chạy qua shell
		cmd = exec.Command("cmd", "/C", fullCommand)
	} else {
		// Nếu không có redirect, tạo output file
		outputFile, err := os.Create(task.ResultFile)
		if err != nil {
			fmt.Printf("Failed to create output file for task %d: %v\n", task.ID, err)
			r.updateTaskStatus(taskIndex, TaskFailed)
			return
		}
		cmd.Stdout = outputFile
		defer outputFile.Close()
	}

	// Chạy command
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start command for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	fmt.Printf("Started task %d: %s (PID: %d)\n", task.ID, task.WindowName, cmd.Process.Pid)

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

	// Tạo tmux window
	windowCmd := exec.Command("tmux", "new-window", "-t", r.config.SessionName, "-n", task.WindowName)
	if err := windowCmd.Run(); err != nil {
		fmt.Printf("Failed to create tmux window for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	// Chạy command trong tmux window
	sendCmd := exec.Command("tmux", "send-keys", "-t", fmt.Sprintf("%s:%s", r.config.SessionName, task.WindowName), fullCommand, "Enter")
	if err := sendCmd.Run(); err != nil {
		fmt.Printf("Failed to send command to tmux window for task %d: %v\n", task.ID, err)
		r.updateTaskStatus(taskIndex, TaskFailed)
		return
	}

	fmt.Printf("Started task %d: %s\n", task.ID, task.WindowName)
}

func (r *Runner) buildCommand(chunkFile, resultFile string) string {
	var cmdParts []string

	// Thêm command chính
	cmdParts = append(cmdParts, r.config.Command)

	// Thêm arguments, thay {input} và {output} placeholders
	for _, arg := range r.config.CommandArgs {
		arg = strings.ReplaceAll(arg, "{input}", chunkFile)
		arg = strings.ReplaceAll(arg, "{output}", resultFile)
		cmdParts = append(cmdParts, arg)
	}

	return strings.Join(cmdParts, " ")
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

	for i := range r.tasks {
		task := &r.tasks[i]

		// Kiểm tra xem file result đã tồn tại chưa
		if task.Status == TaskRunning {
			if _, err := os.Stat(task.ResultFile); err == nil {
				r.updateTaskStatus(i, TaskCompleted)
				completedCount++
			}
		} else if task.Status == TaskCompleted {
			completedCount++
		} else if task.Status == TaskFailed {
			failedCount++
		}
	}

	total := len(r.tasks)
	fmt.Printf("Progress: %d/%d completed, %d failed\n", completedCount, total, failedCount)

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
	fmt.Println("Handling interrupt, collecting partial results...")

	if runtime.GOOS != "windows" {
		// Kill tmux session on Unix systems
		exec.Command("tmux", "kill-session", "-t", r.config.SessionName).Run()
	}

	// Collect partial results
	return r.collector.MergeResults()
}

func (r *Runner) cleanup() error {
	// Scan output directory for result files
	pattern := filepath.Join(r.config.OutputDir, "result_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find result files: %w", err)
	}

	if len(matches) == 0 {
		fmt.Println("No result files found for cleanup")
		return nil
	}

	fmt.Printf("Found %d result files to merge\n", len(matches))
	return r.collector.MergeResults()
}
