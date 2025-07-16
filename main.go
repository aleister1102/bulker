package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bulker",
	Short: "Parallel processing tool for command-line utilities",
	Long:  `A tool for running command-line utilities in parallel, with support for input file splitting and streamlined output handling.`,
}

var runCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "Run a command in parallel",
	Long:  `Splits an input file and runs a command concurrently across multiple workers.`,
	Run:   runCommand,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tools and their optimizations",
	Long:  `Display all supported tools with their automatic optimizations and example usage`,
	Run:   listTools,
}

var (
	inputFile  string
	outputFile string
	workers    int
	extraArgs  []string
	configFile string
	wordlist   string
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(listCmd)

	runCmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input file path (required for running a tool)")
	runCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (required)")
	runCmd.Flags().IntVarP(&workers, "workers", "w", 4, "Number of parallel workers")
	runCmd.Flags().StringArrayVarP(&extraArgs, "extra-args", "e", []string{}, "Extra arguments to pass to the tool (can be used multiple times)")
	runCmd.Flags().StringVarP(&configFile, "config", "c", "config.toml", "Path to custom config file")
	runCmd.Flags().StringVar(&wordlist, "wl", "", "Path to wordlist file (for tools like ffuf)")

	listCmd.Flags().StringVarP(&configFile, "config", "c", "config.toml", "Path to custom config file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		LogError("%v", err)
		os.Exit(1)
	}
}

func runCommand(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		// If no tool is specified, list available tools
		listTools(cmd, args)
		return
	}

	command := args[0]

	// If a tool is specified, input and output files are required
	if inputFile == "" {
		LogError("Error: --input flag is required when running a command")
		cmd.Help()
		os.Exit(1)
	}
	if outputFile == "" {
		LogError("Error: --output flag is required when running a command")
		cmd.Help()
		os.Exit(1)
	}

	// ffuf-specific validation
	if command == "ffuf" && wordlist == "" {
		LogError("Error: --wl (wordlist) flag is required when running ffuf")
		os.Exit(1)
	}

	commandArgs := args[1:]

	// Append extra args if provided
	if len(extraArgs) > 0 {
		commandArgs = append(commandArgs, extraArgs...)
	}

	runner, err := NewRunner(RunnerConfig{
		InputFile:   inputFile,
		OutputFile:  outputFile,
		Workers:     workers,
		Command:     command,
		CommandArgs: commandArgs,
		ConfigFile:  configFile,
		Wordlist:    wordlist,
	})

	if err != nil {
		LogError("Error creating runner: %v", err)
		os.Exit(1)
	}

	if err := runner.Run(); err != nil {
		LogError("Error: %v", err)
		os.Exit(1)
	}
}

func listTools(cmd *cobra.Command, args []string) {
	configManager, err := NewConfigManager(configFile)
	if err != nil {
		LogWarn("Could not load config file: %v. Listing built-in tools only.", err)
	}

	fmt.Println("Available tools:")

	// Get tools from config file (will be empty if config fails)
	tools := configManager.GetAllTools()

	// Add built-in echo
	tools = append(tools, ToolConfig{Name: "echo", Description: "Simple text processing and output"})

	// Sort again just in case (or handle insertion sort)
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	for _, tool := range tools {
		fmt.Printf("  %-10s %s\n", tool.Name, tool.Description)
	}
	fmt.Println("\nUsage: bulker run <tool> --input <file> [flags]")
}
