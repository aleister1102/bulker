package main

import "strings"

// ToolStrategy interface định nghĩa cách xử lý cho từng tool
type ToolStrategy interface {
	// Chuẩn bị dữ liệu cho tool (tạo file chunk nếu cần)
	PrepareInput(inputLines []string, taskIndex int, startLine, endLine int) (string, error)

	// Xây dựng command để chạy tool
	BuildCommand(inputData string, args []string) []string

	// Dọn dẹp sau khi chạy xong (xóa file chunk nếu có)
	Cleanup(inputData string) error

	// Có cần tạo file chunk không
	NeedsFileChunk() bool
}

// FileOutputStrategy là interface cho các tool có khả năng ghi output ra file riêng
type FileOutputStrategy interface {
	ToolStrategy
	// Build command và trả về file output tạm thời
	BuildCommandWithFileOutput(inputData string, args []string, taskIndex int) (cmdArgs []string, tempOutputFile string)
	// Tool có muốn tự xử lý file output không (dựa trên args)
	HandlesFileOutput(args []string) bool
}

// GetToolStrategy trả về strategy phù hợp cho tool
func GetToolStrategy(toolName string) ToolStrategy {
	switch strings.ToLower(toolName) {
	case "echo":
		return &EchoStrategy{}
	case "httpx":
		return &HttpxStrategy{}
	case "arjun":
		return &ArjunStrategy{}
	default:
		// Mặc định sử dụng strategy tạo file chunk
		return &HttpxStrategy{}
	}
}
