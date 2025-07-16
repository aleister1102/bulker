package main

import (
	"fmt"
	"os"
)

// ArjunStrategy xử lý cho arjun command
type ArjunStrategy struct{}

func (a *ArjunStrategy) PrepareInput(inputLines []string, taskIndex int, startLine, endLine int) (string, error) {
	// Tạo file chunk cho arjun
	chunkFile := fmt.Sprintf("chunk_%d.txt", taskIndex)

	file, err := os.Create(chunkFile)
	if err != nil {
		return "", fmt.Errorf("failed to create chunk file: %w", err)
	}
	defer file.Close()

	// Ghi các dòng vào file chunk
	for i := startLine; i <= endLine && i < len(inputLines); i++ {
		if _, err := file.WriteString(inputLines[i] + "\n"); err != nil {
			return "", fmt.Errorf("failed to write to chunk file: %w", err)
		}
	}

	return chunkFile, nil
}

func (a *ArjunStrategy) BuildCommand(inputData string, args []string) []string {
	// Xây dựng command cho arjun
	var cmdParts []string
	cmdParts = append(cmdParts, "arjun")

	// Thêm -i flag cho input file
	cmdParts = append(cmdParts, "-i", inputData)

	// Theo dõi xem user đã set các flag tối ưu chưa
	hasThreads := false
	hasDelay := false
	hasRateLimit := false
	hasTimeout := false

	// Thêm các arguments khác, nhưng lọc bỏ output flags
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		// Bỏ qua -o, -oJ, -oT, -oB flags và giá trị của nó
		if arg == "-o" || arg == "-oJ" || arg == "-oT" || arg == "-oB" {
			skipNext = true
			continue
		}

		// Kiểm tra các flag tối ưu
		if arg == "-t" || arg == "--threads" {
			hasThreads = true
		} else if arg == "-d" || arg == "--delay" {
			hasDelay = true
		} else if arg == "--rate-limit" {
			hasRateLimit = true
		} else if arg == "-T" || arg == "--timeout" {
			hasTimeout = true
		}

		cmdParts = append(cmdParts, arg)
	}

	// Tự động thêm các flag tối ưu nếu user chưa set
	if !hasThreads {
		cmdParts = append(cmdParts, "-t", "10") // Tăng threads lên 10
	}
	if !hasDelay {
		cmdParts = append(cmdParts, "-d", "0") // Không delay giữa requests
	}
	if !hasRateLimit {
		cmdParts = append(cmdParts, "--rate-limit", "50") // Tăng rate limit
	}
	if !hasTimeout {
		cmdParts = append(cmdParts, "-T", "5") // Giảm timeout xuống 5s
	}

	// Thêm các flag tối ưu khác
	// cmdParts = append(cmdParts, "-q") // Bỏ quiet mode để hiển thị progress

	return cmdParts
}

func (a *ArjunStrategy) HandlesFileOutput(args []string) bool {
	for _, arg := range args {
		if arg == "-o" || arg == "-oJ" || arg == "-oT" || arg == "-oB" {
			return true
		}
	}
	return false
}

func (a *ArjunStrategy) BuildCommandWithFileOutput(inputData string, args []string, taskIndex int) ([]string, string) {
	var cmdParts []string
	cmdParts = append(cmdParts, "arjun", "-i", inputData)

	tempOutputFile := fmt.Sprintf("bulker_arjun_output_%d.tmp", taskIndex)
	outputFlagFound := false

	// Theo dõi xem user đã set các flag tối ưu chưa
	hasThreads := false
	hasDelay := false
	hasRateLimit := false
	hasTimeout := false

	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if arg == "-o" || arg == "-oJ" || arg == "-oT" {
			cmdParts = append(cmdParts, arg, tempOutputFile)
			skipNext = true
			outputFlagFound = true
		} else if arg == "-oB" {
			// -oB is a special case, it might not have a value
			cmdParts = append(cmdParts, arg)
			outputFlagFound = true
		} else {
			// Kiểm tra các flag tối ưu
			if arg == "-t" || arg == "--threads" {
				hasThreads = true
			} else if arg == "-d" || arg == "--delay" {
				hasDelay = true
			} else if arg == "--rate-limit" {
				hasRateLimit = true
			} else if arg == "-T" || arg == "--timeout" {
				hasTimeout = true
			}
			cmdParts = append(cmdParts, arg)
		}
	}

	// Tự động thêm các flag tối ưu nếu user chưa set
	if !hasThreads {
		cmdParts = append(cmdParts, "-t", "10") // Tăng threads lên 10
	}
	if !hasDelay {
		cmdParts = append(cmdParts, "-d", "0") // Không delay giữa requests
	}
	if !hasRateLimit {
		cmdParts = append(cmdParts, "--rate-limit", "50") // Tăng rate limit
	}
	if !hasTimeout {
		cmdParts = append(cmdParts, "-T", "5") // Giảm timeout xuống 5s
	}

	// Thêm các flag tối ưu khác
	// cmdParts = append(cmdParts, "-q") // Quiet mode để giảm output không cần thiết

	// Nếu không có flag output nào được cung cấp, thêm -oT mặc định
	if !outputFlagFound {
		cmdParts = append(cmdParts, "-oT", tempOutputFile)
	}

	return cmdParts, tempOutputFile
}

func (a *ArjunStrategy) Cleanup(inputData string) error {
	// Xóa file chunk
	if err := os.Remove(inputData); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove chunk file %s: %w", inputData, err)
	}
	LogInfo("Cleaned up chunk file: %s", inputData)
	return nil
}

func (a *ArjunStrategy) NeedsFileChunk() bool {
	return true
}
