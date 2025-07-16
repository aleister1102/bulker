package main

import (
	"fmt"
	"os"
	"strings"
)

// HttpxStrategy xử lý cho httpx command
type HttpxStrategy struct{}

func (h *HttpxStrategy) PrepareInput(inputLines []string, taskIndex int, startLine, endLine int) (string, error) {
	// Tạo file chunk cho httpx
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

func (h *HttpxStrategy) BuildCommand(inputData string, args []string) []string {
	// Xây dựng command cho httpx
	var cmdParts []string
	cmdParts = append(cmdParts, "httpx")

	// Thêm -l flag cho input file
	cmdParts = append(cmdParts, "-l", inputData)

	// Thêm các arguments khác, nhưng lọc bỏ -o flag
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		// Bỏ qua -o flag và giá trị của nó
		if arg == "-o" {
			skipNext = true
			continue
		}

		// Thay thế {input} placeholder nếu có
		arg = strings.ReplaceAll(arg, "{input}", inputData)
		cmdParts = append(cmdParts, arg)
	}

	return cmdParts
}

func (h *HttpxStrategy) Cleanup(inputData string) error {
	// Xóa file chunk
	if err := os.Remove(inputData); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove chunk file %s: %w", inputData, err)
	}
	LogInfo("Cleaned up chunk file: %s", inputData)
	return nil
}

func (h *HttpxStrategy) NeedsFileChunk() bool {
	return true
}
