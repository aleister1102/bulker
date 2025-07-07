package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bulker",
	Short: "Parallel processing tool using tmux",
	Long:  `Tool for running command line tools in parallel through tmux detach with input file splitting and interrupt handling`,
}

var runCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "Run command in parallel",
	Long:  `Split input file and run command on multiple threads using tmux`,
	Args:  cobra.MinimumNArgs(1),
	Run:   runCommand,
}

var (
	inputFile   string
	outputDir   string
	workers     int
	chunkSize   int
	sessionName string
	cleanupMode bool
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input file path (required)")
	runCmd.Flags().StringVarP(&outputDir, "output", "o", "output", "Output directory")
	runCmd.Flags().IntVarP(&workers, "workers", "w", 4, "Number of parallel workers")
	runCmd.Flags().IntVarP(&chunkSize, "chunk-size", "c", 1000, "Chunk size (lines)")
	runCmd.Flags().StringVarP(&sessionName, "session", "s", "bulker", "Tmux session name")
	runCmd.Flags().BoolVar(&cleanupMode, "cleanup", false, "Cleanup mode - collect results from interrupted run")

	runCmd.MarkFlagRequired("input")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runCommand(cmd *cobra.Command, args []string) {
	command := args[0]
	commandArgs := args[1:]

	runner := NewRunner(RunnerConfig{
		InputFile:   inputFile,
		OutputDir:   outputDir,
		Workers:     workers,
		ChunkSize:   chunkSize,
		SessionName: sessionName,
		Command:     command,
		CommandArgs: commandArgs,
		CleanupMode: cleanupMode,
	})

	if err := runner.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
