package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type ResultCollector struct {
	outputDir string
}

func NewResultCollector(outputDir string) *ResultCollector {
	return &ResultCollector{
		outputDir: outputDir,
	}
}

func (rc *ResultCollector) MergeResults() error {
	// Tìm tất cả result files
	pattern := filepath.Join(rc.outputDir, "result_*.txt")
	resultFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find result files: %w", err)
	}

	if len(resultFiles) == 0 {
		fmt.Println("No result files found to merge")
		return nil
	}

	// Sắp xếp files theo thứ tự
	sort.Strings(resultFiles)

	// Tạo merged result file
	mergedFile := filepath.Join(rc.outputDir, "merged_result.txt")
	output, err := os.Create(mergedFile)
	if err != nil {
		return fmt.Errorf("failed to create merged result file: %w", err)
	}
	defer output.Close()

	writer := bufio.NewWriter(output)
	defer writer.Flush()

	totalLines := 0
	for _, resultFile := range resultFiles {
		lines, err := rc.copyFile(resultFile, writer)
		if err != nil {
			fmt.Printf("Warning: failed to copy %s: %v\n", resultFile, err)
			continue
		}
		totalLines += lines
		fmt.Printf("Merged %s (%d lines)\n", filepath.Base(resultFile), lines)
	}

	fmt.Printf("Merged %d files into %s (%d total lines)\n", len(resultFiles), mergedFile, totalLines)
	return nil
}

func (rc *ResultCollector) copyFile(src string, writer *bufio.Writer) (int, error) {
	file, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		if _, err := writer.WriteString(scanner.Text() + "\n"); err != nil {
			return lineCount, err
		}
		lineCount++
	}

	return lineCount, scanner.Err()
}

func (rc *ResultCollector) GetResultFiles() ([]string, error) {
	pattern := filepath.Join(rc.outputDir, "result_*.txt")
	return filepath.Glob(pattern)
}

func (rc *ResultCollector) GetChunkFiles() ([]string, error) {
	pattern := filepath.Join(rc.outputDir, "chunk_*.txt")
	return filepath.Glob(pattern)
}

func (rc *ResultCollector) CleanupChunks() error {
	chunkFiles, err := rc.GetChunkFiles()
	if err != nil {
		return fmt.Errorf("failed to get chunk files: %w", err)
	}

	for _, chunkFile := range chunkFiles {
		if err := os.Remove(chunkFile); err != nil {
			fmt.Printf("Warning: failed to remove chunk file %s: %v\n", chunkFile, err)
		}
	}

	fmt.Printf("Cleaned up %d chunk files\n", len(chunkFiles))
	return nil
}

func (rc *ResultCollector) GetStats() (map[string]interface{}, error) {
	resultFiles, err := rc.GetResultFiles()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"result_files": len(resultFiles),
		"total_size":   int64(0),
		"files":        make([]map[string]interface{}, 0),
	}

	for _, file := range resultFiles {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		fileStats := map[string]interface{}{
			"name":     filepath.Base(file),
			"size":     info.Size(),
			"mod_time": info.ModTime(),
		}

		stats["files"] = append(stats["files"].([]map[string]interface{}), fileStats)
		stats["total_size"] = stats["total_size"].(int64) + info.Size()
	}

	return stats, nil
}
