package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

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
	// Change short flag from -w to -t to avoid conflict with wordlist flag (-w in tools like ffuf)
	runCmd.Flags().IntVarP(&workers, "threads", "t", 4, "Number of parallel threads")
	runCmd.Flags().StringArrayVarP(&extraArgs, "extra-args", "e", []string{}, "Extra arguments to pass to the tool (supports multiple args in one flag: -e '--strict --verify')")
	runCmd.Flags().StringVarP(&configFile, "config", "c", "config.toml", "Path to custom config file")
	runCmd.Flags().StringVarP(&wordlist, "wordlist", "w", "", "Path to wordlist file (for tools like ffuf)")

	listCmd.Flags().StringVarP(&configFile, "config", "c", "config.toml", "Path to custom config file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		LogError("%v", err)
		os.Exit(1)
	}
}

// splitArgsRespectingQuotes splits a string into arguments while respecting quoted strings
func splitArgsRespectingQuotes(input string) []string {
	if strings.TrimSpace(input) == "" {
		return []string{}
	}

	var args []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(input); i++ {
		char := input[i]

		switch {
		case (char == '"' || char == '\'') && !inQuotes:
			inQuotes = true
			quoteChar = char
		case char == quoteChar && inQuotes:
			inQuotes = false
			quoteChar = 0
		case char == ' ' && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
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

	// Kiểm tra cấu hình tool để xác định các yêu cầu đặc biệt
	configManager, err := NewConfigManager(configFile)
	if err != nil {
		LogError("Error loading config file: %v", err)
		os.Exit(1)
	}

	toolConfig, exists := configManager.GetToolConfig(command)
	if !exists {
		LogError("Error: tool '%s' not found in config file", command)
		os.Exit(1)
	}

	// Kiểm tra xem tool có yêu cầu wordlist không
	if strings.Contains(toolConfig.Command, "{wordlist}") && wordlist == "" {
		LogError("Error: -w/--wordlist flag is required when running %s", command)
		os.Exit(1)
	}

	commandArgs := args[1:]

	// Process extra args - split each arg string by spaces to allow multiple args in one flag
	if len(extraArgs) > 0 {
		var processedArgs []string
		for _, arg := range extraArgs {
			// Split by spaces while preserving quoted strings
			splitArgs := splitArgsRespectingQuotes(arg)
			processedArgs = append(processedArgs, splitArgs...)
		}
		commandArgs = append(commandArgs, processedArgs...)
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
		LogWarn("Could not load config file: %v. No tools available.", err)
		return
	}

	fmt.Println("Available tools:")
	fmt.Println("================")

	// Get tools from config file
	tools := configManager.GetAllTools()

	// Sort by name
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	for _, tool := range tools {
		fmt.Printf("\n%s\n", tool.Name)
		fmt.Printf("  Description: %s\n", tool.Description)

		if len(tool.AutoOptimizations) > 0 {
			fmt.Printf("  Auto optimizations: %s\n", strings.Join(tool.AutoOptimizations, " "))
		}

		if len(tool.Examples) > 0 {
			fmt.Printf("  Examples:\n")
			for _, example := range tool.Examples {
				fmt.Printf("    %s\n", example)
			}
		}
	}

	fmt.Println("\nGeneral Usage:")
	fmt.Println("  bulker run <tool> --input <file> --output <file> [flags]")
	fmt.Println("\nCommon flags:")
	fmt.Println("  -i, --input <file>     Input file path (required)")
	fmt.Println("  -o, --output <file>    Output file path (required)")
	fmt.Println("  -t, --threads <num>    Number of parallel threads (default: 4)")
	fmt.Println("  -e, --extra-args       Extra arguments to pass to the tool")
	fmt.Println("                         Examples: -e '--strict --verify' or -e '--timeout 30'")
	fmt.Println("  -w, --wordlist <file>  Wordlist file (required for ffuf)")
	fmt.Println("  -c, --config <file>    Custom config file (default: config.toml)")
}
