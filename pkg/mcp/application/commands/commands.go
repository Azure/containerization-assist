// Package commands provides consolidated command implementations for the MCP server
//
// This package consolidates all tool implementations from the original scattered
// pkg/mcp/tools/ structure into unified command implementations following the
// three-layer architecture.
//
// Architecture:
//   - ConsolidatedAnalyzeCommand: Repository analysis and Dockerfile generation
//   - ConsolidatedBuildCommand: Container build operations
//   - ConsolidatedDeployCommand: Kubernetes deployment operations
//   - ConsolidatedScanCommand: Security scanning operations
//
// Each command follows the consolidated pattern:
//   1. Single command struct with all tool functionality
//   2. Comprehensive implementations without stubs
//   3. Proper domain integration
//   4. Unified error handling
//
package commands

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// CommandExecutor represents the interface for all command implementations
type CommandExecutor interface {
	Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
	Name() string
	Description() string
	Schema() api.ToolSchema
}

// BaseCommand provides common functionality for all commands
type BaseCommand struct {
	sessionStore services.SessionStore
	sessionState services.SessionState
	logger       *slog.Logger
	timeout      time.Duration
}

// NewBaseCommand creates a new base command
func NewBaseCommand(sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *BaseCommand {
	return &BaseCommand{
		sessionStore: sessionStore,
		sessionState: sessionState,
		logger:       logger,
		timeout:      30 * time.Second,
	}
}

// withTimeout applies a timeout to the context
func (b *BaseCommand) withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithTimeout(ctx, b.timeout)
}

// createErrorOutput creates a standardized error output
func (b *BaseCommand) createErrorOutput(code string, message string, cause error) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Data: map[string]interface{}{
			"error": errors.NewError().
				Code(code).
				Message(message).
				Cause(cause).
				Build(),
		},
	}
}

// createSuccessOutput creates a standardized success output
func (b *BaseCommand) createSuccessOutput(data map[string]interface{}) api.ToolOutput {
	return api.ToolOutput{
		Success: true,
		Data:    data,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Command registry for all available commands
var registeredCommands = make(map[string]CommandExecutor)

// RegisterCommand registers a command with the registry
func RegisterCommand(name string, command CommandExecutor) {
	registeredCommands[name] = command
}

// GetCommand retrieves a command by name
func GetCommand(name string) (CommandExecutor, bool) {
	cmd, exists := registeredCommands[name]
	return cmd, exists
}

// GetAllCommands returns all registered commands
func GetAllCommands() map[string]CommandExecutor {
	return registeredCommands
}

// CommandInfo represents information about a command
type CommandInfo struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Schema      api.ToolSchema `json:"schema"`
	Category    string     `json:"category"`
	Version     string     `json:"version"`
}

// GetCommandInfo returns information about all commands
func GetCommandInfo() []CommandInfo {
	var info []CommandInfo
	for name, cmd := range registeredCommands {
		schema := cmd.Schema()
		info = append(info, CommandInfo{
			Name:        name,
			Description: cmd.Description(),
			Schema:      schema,
			Category:    string(schema.Category),
			Version:     "1.0.0",
		})
	}
	return info
}

// Helper functions for parameter extraction

// getStringParam extracts a string parameter from input data
func getStringParam(data map[string]interface{}, key string, defaultValue string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return defaultValue
}

// getIntParam extracts an integer parameter from input data
func getIntParam(data map[string]interface{}, key string, defaultValue int) int {
	if val, ok := data[key].(float64); ok {
		return int(val)
	}
	if val, ok := data[key].(int); ok {
		return val
	}
	return defaultValue
}

// getBoolParam extracts a boolean parameter from input data
func getBoolParam(data map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := data[key].(bool); ok {
		return val
	}
	return defaultValue
}

// getDurationParam extracts a duration parameter from input data
func getDurationParam(data map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if val, ok := data[key].(string); ok {
		if duration, err := time.ParseDuration(val); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getStringArrayParam extracts a string array parameter from input data
func getStringArrayParam(data map[string]interface{}, key string) []string {
	if val, ok := data[key].([]interface{}); ok {
		result := make([]string, len(val))
		for i, v := range val {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return []string{}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// initializeCommands initializes all commands
func InitializeCommands(
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	logger *slog.Logger,
) {
	// Initialize consolidated commands
	analyzeCmd := NewConsolidatedAnalyzeCommand(sessionStore, sessionState, nil, nil, logger)
	buildCmd := NewConsolidatedBuildCommand(sessionStore, sessionState, nil, logger)
	deployCmd := NewConsolidatedDeployCommand(sessionStore, sessionState, nil, logger)
	scanCmd := NewConsolidatedScanCommand(sessionStore, sessionState, nil, logger)

	// Register commands
	RegisterCommand(analyzeCmd.Name(), analyzeCmd)
	RegisterCommand(buildCmd.Name(), buildCmd)
	RegisterCommand(deployCmd.Name(), deployCmd)
	RegisterCommand(scanCmd.Name(), scanCmd)

	logger.Info("Commands initialized successfully", "count", len(registeredCommands))
}