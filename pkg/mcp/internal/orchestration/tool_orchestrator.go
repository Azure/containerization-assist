package orchestration

import (
	"context"
	"fmt"
	"time"

	// mcp import removed - using mcptypes

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// MCPToolOrchestrator implements ToolOrchestrationExecutor for MCP atomic tools
// This is the updated version that uses type-safe dispatch instead of reflection
type MCPToolOrchestrator struct {
	toolRegistry       *MCPToolRegistry
	sessionManager     SessionManager
	logger             zerolog.Logger
	dispatcher         *NoReflectToolOrchestrator
	pipelineOperations interface{} // Store for passing to dispatcher
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

// SetPipelineOperations sets the pipeline operations for tool creation
func (o *MCPToolOrchestrator) SetPipelineOperations(operations interface{}) {
	o.pipelineOperations = operations
	if o.dispatcher != nil {
		o.dispatcher.SetPipelineOperations(operations)
	}
}

// SetAnalyzer sets the AI analyzer for tool fixing capabilities
func (o *MCPToolOrchestrator) SetAnalyzer(analyzer core.AIAnalyzer) {
	if o.dispatcher != nil {
		o.dispatcher.SetAnalyzer(analyzer)
	}
}

// ExecuteTool executes a tool with the given arguments
func (o *MCPToolOrchestrator) ExecuteTool(
	ctx context.Context,
	toolName string,
	args interface{},
) (interface{}, error) {
	o.logger.Info().
		Str("tool_name", toolName).
		Msg("Executing tool")

	startTime := time.Now()

	// Delegate to the no-reflection dispatcher
	result, err := o.dispatcher.ExecuteTool(ctx, toolName, args)

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

// RegisterTool registers a tool with the orchestrator (required by core.Orchestrator interface)
func (o *MCPToolOrchestrator) RegisterTool(name string, tool core.Tool) error {
	// This is part of the simplified interface - delegate to tool registry if needed
	if o.toolRegistry != nil {
		// Convert the core.Tool to the orchestration.Tool format if needed
		// For now, just log the registration
		o.logger.Info().
			Str("tool_name", name).
			Msg("Tool registration requested")
		return nil
	}
	return fmt.Errorf("tool registry not available")
}

// ValidateToolArgs validates arguments for a specific tool
func (o *MCPToolOrchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	return o.dispatcher.ValidateToolArgs(toolName, args)
}

// GetToolMetadata returns metadata for a specific tool
func (o *MCPToolOrchestrator) GetToolMetadata(toolName string) (*core.ToolMetadata, error) {
	localMetadata, err := o.toolRegistry.GetToolMetadata(toolName)
	if err != nil {
		return nil, err
	}

	// Convert from orchestration.ToolMetadata to core.ToolMetadata
	converted := &core.ToolMetadata{
		Name:         localMetadata.Name,
		Description:  localMetadata.Description,
		Version:      localMetadata.Version,
		Category:     localMetadata.Category,
		Dependencies: localMetadata.Dependencies,
		Capabilities: localMetadata.Capabilities,
		Requirements: localMetadata.Requirements,
		Parameters:   make(map[string]string),
		Examples:     convertExamples(localMetadata.Examples),
	}

	// Parameters are already map[string]string, no conversion needed
	converted.Parameters = localMetadata.Parameters

	return converted, nil
}

// convertExamples converts from orchestration.ToolExample to mcptypes.ToolExample
func convertExamples(examples []ToolExample) []mcptypes.ToolExample {
	converted := make([]mcptypes.ToolExample, len(examples))
	for i, example := range examples {
		// Type assert Input and Output to map[string]interface{}
		var input, output map[string]interface{}
		if inputMap, ok := example.Input.(map[string]interface{}); ok {
			input = inputMap
		} else {
			input = make(map[string]interface{})
		}
		if outputMap, ok := example.Output.(map[string]interface{}); ok {
			output = outputMap
		} else {
			output = make(map[string]interface{})
		}

		converted[i] = mcptypes.ToolExample{
			Name:        example.Name,
			Description: example.Description,
			Input:       input,
			Output:      output,
		}
	}
	return converted
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
// toSnakeCase function has been moved to utils.ToSnakeCase and is no longer needed here
