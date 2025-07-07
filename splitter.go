package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

type FileSplitter struct {
	inputFile string
	outputDir string
	chunkSize int
}

func NewFileSplitter(inputFile, outputDir string, chunkSize int) *FileSplitter {
	return &FileSplitter{
		inputFile: inputFile,
		outputDir: outputDir,
		chunkSize: chunkSize,
	}
}

func (fs *FileSplitter) Split() ([]string, error) {
	// Tạo output directory
	if err := os.MkdirAll(fs.outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Mở input file
	file, err := os.Open(fs.inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var chunkFiles []string
	chunkIndex := 0
	lineCount := 0

	var currentChunk *os.File
	var currentWriter *bufio.Writer

	for scanner.Scan() {
		// Tạo chunk mới khi cần
		if lineCount%fs.chunkSize == 0 {
			if currentChunk != nil {
				currentWriter.Flush()
				currentChunk.Close()
			}

			chunkFileName := fmt.Sprintf("chunk_%04d.txt", chunkIndex)
			chunkPath := filepath.Join(fs.outputDir, chunkFileName)
			chunkFiles = append(chunkFiles, chunkPath)

			currentChunk, err = os.Create(chunkPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create chunk file: %w", err)
			}
			currentWriter = bufio.NewWriter(currentChunk)
			chunkIndex++
		}

		// Ghi line vào chunk hiện tại
		if _, err := currentWriter.WriteString(scanner.Text() + "\n"); err != nil {
			return nil, fmt.Errorf("failed to write to chunk file: %w", err)
		}
		lineCount++
	}

	// Đóng chunk cuối cùng
	if currentChunk != nil {
		currentWriter.Flush()
		currentChunk.Close()
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input file: %w", err)
	}

	fmt.Printf("Split %d lines into %d chunks\n", lineCount, len(chunkFiles))
	return chunkFiles, nil
}

func (fs *FileSplitter) GetChunkPrefix() string {
	return filepath.Join(fs.outputDir, "chunk_")
}

func (fs *FileSplitter) GetResultPrefix() string {
	return filepath.Join(fs.outputDir, "result_")
}
