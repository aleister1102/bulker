package main

import "strings"

// EchoStrategy xử lý cho echo command
type EchoStrategy struct{}

func (e *EchoStrategy) PrepareInput(inputLines []string, taskIndex int, startLine, endLine int) (string, error) {
	// Echo không cần file, trả về dữ liệu trực tiếp
	var lines []string
	for i := startLine; i <= endLine && i < len(inputLines); i++ {
		lines = append(lines, inputLines[i])
	}
	return strings.Join(lines, "\n"), nil
}

func (e *EchoStrategy) BuildCommand(inputData string, args []string) []string {
	// Echo không cần command thực tế, xử lý trực tiếp
	return []string{"echo", inputData}
}

func (e *EchoStrategy) Cleanup(inputData string) error {
	// Echo không cần cleanup
	return nil
}

func (e *EchoStrategy) NeedsFileChunk() bool {
	return false
}
