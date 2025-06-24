package orchestration

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// MCPToolOrchestrator implements ToolOrchestrator for MCP atomic tools
// This is the updated version that uses type-safe dispatch instead of reflection
type MCPToolOrchestrator struct {
	toolRegistry    *MCPToolRegistry
	sessionManager  SessionManager
	logger          zerolog.Logger
	dispatcher      *NoReflectToolOrchestrator
	pipelineAdapter interface{} // Store for passing to dispatcher
}

// NewMCPToolOrchestrator creates a new tool orchestrator for MCP atomic tools
func NewMCPToolOrchestrator(
	toolRegistry *MCPToolRegistry,
	sessionManager SessionManager,
	logger zerolog.Logger,
) *MCPToolOrchestrator {
	return &MCPToolOrchestrator{
		toolRegistry:   toolRegistry,
		sessionManager: sessionManager,
		logger:         logger.With().Str("component", "tool_orchestrator").Logger(),
		dispatcher:     NewNoReflectToolOrchestrator(toolRegistry, sessionManager, logger),
	}
}

// GetDispatcher returns the NoReflectToolOrchestrator for direct access
func (o *MCPToolOrchestrator) GetDispatcher() *NoReflectToolOrchestrator {
	return o.dispatcher
}

// SetPipelineAdapter sets the pipeline adapter for tool creation
func (o *MCPToolOrchestrator) SetPipelineAdapter(adapter interface{}) {
	o.pipelineAdapter = adapter
	if o.dispatcher != nil {
		o.dispatcher.SetPipelineAdapter(adapter)
	}
}

// ExecuteTool executes a tool with the given arguments and session context
func (o *MCPToolOrchestrator) ExecuteTool(
	ctx context.Context,
	toolName string,
	args interface{},
	session interface{},
) (interface{}, error) {
	o.logger.Info().
		Str("tool_name", toolName).
		Msg("Executing tool")

	startTime := time.Now()

	// Delegate to the no-reflection dispatcher
	result, err := o.dispatcher.ExecuteTool(ctx, toolName, args, session)

	duration := time.Since(startTime)

	if err != nil {
		o.logger.Error().
			Err(err).
			Str("tool_name", toolName).
			Dur("duration", duration).
			Msg("Tool execution failed")
		return nil, err
	}

	o.logger.Info().
		Str("tool_name", toolName).
		Dur("duration", duration).
		Msg("Tool execution completed successfully")

	return result, nil
}

// ValidateToolArgs validates arguments for a specific tool
func (o *MCPToolOrchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	return o.dispatcher.ValidateToolArgs(toolName, args)
}

// GetToolMetadata returns metadata for a specific tool
func (o *MCPToolOrchestrator) GetToolMetadata(toolName string) (*ToolMetadata, error) {
	return o.toolRegistry.GetToolMetadata(toolName)
}

// The following methods maintain backward compatibility but delegate to the new implementation

// validateRequiredParameters validates that all required parameters are present
func (o *MCPToolOrchestrator) validateRequiredParameters(
	toolName string,
	args map[string]interface{},
	metadata *ToolMetadata,
) error {
	// Delegate to dispatcher's validation
	return o.dispatcher.ValidateToolArgs(toolName, args)
}

// validateParameterTypes validates parameter types match expectations
func (o *MCPToolOrchestrator) validateParameterTypes(
	toolName string,
	args map[string]interface{},
	metadata *ToolMetadata,
) error {
	// Type validation now happens at compile time in the dispatcher
	// This method is kept for backward compatibility
	return nil
}

// toSnakeCase converts a string to snake_case (kept for compatibility)
func (o *MCPToolOrchestrator) toSnakeCase(str string) string {
	var result []byte
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, byte(r+32))
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}
