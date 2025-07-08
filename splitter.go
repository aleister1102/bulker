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
	workers   int
}

func NewFileSplitter(inputFile, outputDir string, workers int) *FileSplitter {
	return &FileSplitter{
		inputFile: inputFile,
		outputDir: outputDir,
		workers:   workers,
	}
}

func (fs *FileSplitter) Split() ([]string, error) {
	// Create output directory
	if err := os.MkdirAll(fs.outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Count total lines first
	totalLines, err := fs.countLines()
	if err != nil {
		return nil, fmt.Errorf("failed to count lines: %w", err)
	}

	// Calculate chunk size based on workers
	chunkSize := totalLines / fs.workers
	if totalLines%fs.workers != 0 {
		chunkSize++ // Round up to ensure all lines are included
	}

	// Ensure minimum chunk size of 1
	if chunkSize < 1 {
		chunkSize = 1
	}

	fmt.Printf("Total lines: %d, Workers: %d, Chunk size: %d\n", totalLines, fs.workers, chunkSize)

	// Open input file
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
		// Create new chunk when needed
		if lineCount%chunkSize == 0 {
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

		// Write line to current chunk
		if _, err := currentWriter.WriteString(scanner.Text() + "\n"); err != nil {
			return nil, fmt.Errorf("failed to write to chunk file: %w", err)
		}
		lineCount++
	}

	// Close last chunk
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

func (fs *FileSplitter) countLines() (int, error) {
	file, err := os.Open(fs.inputFile)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	return lineCount, scanner.Err()
}

func (fs *FileSplitter) GetChunkPrefix() string {
	return filepath.Join(fs.outputDir, "chunk_")
}

func (fs *FileSplitter) GetResultPrefix() string {
	return filepath.Join(fs.outputDir, "result_")
}
