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
//  1. Single command struct with all tool functionality
//  2. Comprehensive implementations without stubs
//  3. Proper domain integration
//  4. Unified error handling
package commands

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// CommandExecutor represents the interface for all command implementations
type CommandExecutor interface {
	Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
	Name() string
	Description() string
	Schema() api.ToolSchema
}

// commandToolAdapter adapts CommandExecutor to api.Tool interface
type commandToolAdapter struct {
	command CommandExecutor
	name    string
}

func (c *commandToolAdapter) Name() string {
	return c.command.Name()
}

func (c *commandToolAdapter) Description() string {
	return c.command.Description()
}

func (c *commandToolAdapter) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	return c.command.Execute(ctx, input)
}

func (c *commandToolAdapter) Schema() api.ToolSchema {
	return c.command.Schema()
}

// Ensure commandToolAdapter implements api.Tool
var _ api.Tool = (*commandToolAdapter)(nil)

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
				Code(errors.ErrorCode(code)).
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

// Note: ValidationError is defined in common.go

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ToolFactory represents a typed factory function (local definition)
type ToolFactory func() (api.Tool, error)

// Command registry using unified registry system
var commandRegistry api.ToolRegistry

// RegisterCommand registers a command with the unified registry
func RegisterCommand(name string, command CommandExecutor) error {
	if commandRegistry == nil {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Message("command registry not initialized").
			Suggestion("Call InitializeCommands() first").
			Build()
	}

	// Wrap CommandExecutor as ToolFactory
	factory := ToolFactory(func() (api.Tool, error) {
		return &commandToolAdapter{command: command, name: name}, nil
	})

	return commandRegistry.Register(name, factory)
}

// GetCommand retrieves a command by name
func GetCommand(name string) (CommandExecutor, bool) {
	if commandRegistry == nil {
		return nil, false
	}

	tool, err := commandRegistry.Discover(name)
	if err != nil {
		return nil, false
	}

	if adapter, ok := tool.(*commandToolAdapter); ok {
		return adapter.command, true
	}
	return nil, false
}

// GetAllCommands returns all registered commands
func GetAllCommands() map[string]CommandExecutor {
	if commandRegistry == nil {
		return make(map[string]CommandExecutor)
	}

	result := make(map[string]CommandExecutor)
	for _, name := range commandRegistry.List() {
		if cmd, exists := GetCommand(name); exists {
			result[name] = cmd
		}
	}
	return result
}

// CommandInfo represents information about a command
type CommandInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      api.ToolSchema `json:"schema"`
	Category    string         `json:"category"`
	Version     string         `json:"version"`
}

// GetCommandInfo returns information about all commands
func GetCommandInfo() []CommandInfo {
	var info []CommandInfo
	for name, cmd := range GetAllCommands() {
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

// Note: Helper functions (getStringParam, getIntParam, getBoolParam) are defined in common.go

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

// Note: contains function is defined in common.go

// initializeCommands initializes all commands
func InitializeCommands(
	registry api.ToolRegistry,
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	logger *slog.Logger,
) error {
	// Set the unified registry for command registration
	commandRegistry = registry

	registeredCount := len(registry.List())
	logger.Info("Commands initialized successfully", "count", registeredCount)
	return nil
}
