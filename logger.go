package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
	Gray   = "\033[90m"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	SUCCESS
)

type Logger struct {
	level LogLevel
}

var logger = &Logger{level: INFO}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	message := fmt.Sprintf(format, args...)

	var levelStr, color string
	switch level {
	case DEBUG:
		levelStr = "DEBUG"
		color = Gray
	case INFO:
		levelStr = "INFO"
		color = Blue
	case WARN:
		levelStr = "WARN"
		color = Yellow
	case ERROR:
		levelStr = "ERROR"
		color = Red
	case SUCCESS:
		levelStr = "SUCCESS"
		color = Green
	}

	// Format: [timestamp] [LEVEL] message
	fmt.Printf("%s[%s] [%s%s%s] %s%s\n",
		Gray, timestamp, color, levelStr, Reset, message, Reset)
}

func LogDebug(format string, args ...interface{}) {
	logger.log(DEBUG, format, args...)
}

func LogInfo(format string, args ...interface{}) {
	logger.log(INFO, format, args...)
}

func LogWarn(format string, args ...interface{}) {
	logger.log(WARN, format, args...)
}

func LogError(format string, args ...interface{}) {
	logger.log(ERROR, format, args...)
}

func LogSuccess(format string, args ...interface{}) {
	logger.log(SUCCESS, format, args...)
}

func LogTask(taskID int, format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s] [%sTASK-%d%s] %s%s\n",
		Gray, timestamp, Cyan, taskID, Reset, message, Reset)
}

func LogPerf(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s] [%sPERF%s] %s%s\n",
		Gray, timestamp, Purple, Reset, message, Reset)
}

func SetLogLevel(level LogLevel) {
	logger.level = level
}

func isColorSupported() bool {
	term := os.Getenv("TERM")
	return term != "" && !strings.Contains(strings.ToLower(term), "dumb")
}

func init() {
	// Disable colors on Windows if not supported
	if !isColorSupported() {
		// Keep colors for now, most modern terminals support them
	}
}
