package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// ToolConfig defines configuration for a tool
type ToolConfig struct {
	Name              string   `toml:"-"` // Ignored by toml
	Description       string   `toml:"description"`
	Mode              string   `toml:"mode"`
	Command           string   `toml:"command"`
	AutoOptimizations []string `toml:"auto_optimizations"`
	Header            string   `toml:"header"`
	Examples          []string `toml:"examples"`
}

// Config holds all tool configurations
type Config struct {
	Tools map[string]ToolConfig `toml:"tools"`
}

// ConfigManager manages tool configurations
type ConfigManager struct {
	config Config
}

// findConfigFile looks for config file in the following order:
// 1. Current directory
// 2. User's home directory
func findConfigFile(customPath string) (string, error) {
	// If custom path is provided and exists, use it
	if customPath != "" && customPath != "config.toml" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil
		}
		return "", fmt.Errorf("custom config file %s not found", customPath)
	}

	// Check current directory first
	currentDir, err := os.Getwd()
	if err == nil {
		configPath := filepath.Join(currentDir, "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
	}

	// Check home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
	}

	return "", fmt.Errorf("config file not found in current directory or home directory")
}

// NewConfigManager creates a new config manager
func NewConfigManager(configPath string) (*ConfigManager, error) {
	// Find the actual config file path
	actualConfigPath, err := findConfigFile(configPath)
	if err != nil {
		return nil, err
	}

	// Read config file
	data, err := os.ReadFile(actualConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse TOML
	var config Config
	if _, err := toml.Decode(string(data), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &ConfigManager{config: config}, nil
}

// GetToolConfig returns configuration for a tool
func (cm *ConfigManager) GetToolConfig(toolName string) (ToolConfig, bool) {
	config, exists := cm.config.Tools[strings.ToLower(toolName)]
	return config, exists
}

// GetAllTools returns all available tools as a slice for consistent ordering
func (cm *ConfigManager) GetAllTools() []ToolConfig {
	if cm == nil {
		return []ToolConfig{}
	}

	tools := make([]ToolConfig, 0, len(cm.config.Tools))
	for name, config := range cm.config.Tools {
		// Add the name to the config struct for easy access
		config.Name = name
		tools = append(tools, config)
	}

	// Sort by name
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	return tools
}

// BuildCommand builds the command for a tool based on config
func (cm *ConfigManager) BuildCommand(toolName, inputData string, args []string, tempOutputFile string, wordlist string) ([]string, error) {
	toolConfig, exists := cm.GetToolConfig(toolName)
	if !exists {
		return nil, fmt.Errorf("tool %s not found in config", toolName)
	}

	// Build auto optimizations string
	autoOptimizations := strings.Join(toolConfig.AutoOptimizations, " ")
	argsString := strings.Join(args, " ")

	// Replace placeholders in command
	command := toolConfig.Command
	command = strings.ReplaceAll(command, "{input}", inputData)
	command = strings.ReplaceAll(command, "{auto_optimizations}", autoOptimizations)
	command = strings.ReplaceAll(command, "{args}", argsString)
	command = strings.ReplaceAll(command, "{output}", tempOutputFile)
	command = strings.ReplaceAll(command, "{wordlist}", wordlist)

	// Split command into parts for execution
	return strings.Fields(command), nil
}
