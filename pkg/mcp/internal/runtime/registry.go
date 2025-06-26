// pkg/mcp/tools/registry.go
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/util/schema"
	"github.com/rs/zerolog"
)

///////////////////////////////////////////////////////////////////////////////
// Contracts
///////////////////////////////////////////////////////////////////////////////

// NOTE: Tool interface is now defined in pkg/mcp/interfaces.go
// Using mcp.Tool for the unified interface

// UnifiedTool represents the unified interface for all MCP tools
type UnifiedTool interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	GetMetadata() mcptypes.ToolMetadata
	Validate(ctx context.Context, args interface{}) error
}

type ExecutableTool[TArgs, TResult any] interface {
	UnifiedTool
	PreValidate(ctx context.Context, args TArgs) error
}

///////////////////////////////////////////////////////////////////////////////
// Registry primitives
///////////////////////////////////////////////////////////////////////////////

type ToolRegistration struct {
	Tool         UnifiedTool
	InputSchema  map[string]any
	OutputSchema map[string]any
	Handler      func(ctx context.Context, args json.RawMessage) (interface{}, error)
}

type ToolRegistry struct {
	mu     sync.RWMutex
	tools  map[string]*ToolRegistration
	logger zerolog.Logger
	frozen bool
}

func NewToolRegistry(l zerolog.Logger) *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]*ToolRegistration),
		logger: l.With().Str("component", "tool_registry").Logger(),
	}
}

///////////////////////////////////////////////////////////////////////////////
// RegisterTool
///////////////////////////////////////////////////////////////////////////////

func RegisterTool[TArgs, TResult any](reg *ToolRegistry, t ExecutableTool[TArgs, TResult]) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.frozen {
		return types.NewRichError("INVALID_REQUEST", "tool registry frozen", "system_error")
	}
	metadata := t.GetMetadata()
	if _, dup := reg.tools[metadata.Name]; dup {
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("tool %s already registered", metadata.Name), "validation_error")
	}

	// Use GoMCP's built-in schema generator
	gomcpGenerator := schema.NewGenerator()
	var a TArgs
	var r TResult

	// Generate schemas using GoMCP's generator
	inputSchema, err := gomcpGenerator.GenerateSchema(a)
	if err != nil {
		return types.NewRichError("SCHEMA_GENERATION_ERROR", fmt.Sprintf("failed to generate input schema for %s: %v", metadata.Name, err), "internal_error")
	}

	outputSchema, err := gomcpGenerator.GenerateSchema(r)
	if err != nil {
		return types.NewRichError("SCHEMA_GENERATION_ERROR", fmt.Sprintf("failed to generate output schema for %s: %v", metadata.Name, err), "internal_error")
	}

	// Convert to JSON and back to get map[string]interface{} format
	inputSchemaMap := convertGoMCPSchemaToMap(inputSchema)
	outputSchemaMap := convertGoMCPSchemaToMap(outputSchema)

	// Apply post-processing fixes for array items and GitHub Copilot compatibility
	utils.AddMissingArrayItems(inputSchemaMap)
	utils.AddMissingArrayItems(outputSchemaMap)
	utils.RemoveCopilotIncompatible(inputSchemaMap)
	utils.RemoveCopilotIncompatible(outputSchemaMap)

	cleanInput := inputSchemaMap
	cleanOutput := outputSchemaMap

	reg.tools[metadata.Name] = &ToolRegistration{
		Tool:         t,
		InputSchema:  cleanInput,
		OutputSchema: cleanOutput,
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args TArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, types.NewRichError("INVALID_ARGUMENTS", "unmarshal args: "+err.Error(), "validation_error")
			}
			if err := t.PreValidate(ctx, args); err != nil {
				return nil, err
			}
			return t.Execute(ctx, args)
		},
	}

	reg.logger.Info().
		Str("tool", metadata.Name).
		Str("version", metadata.Version).
		Msg("registered")
	return nil
}

// convertGoMCPSchemaToMap converts GoMCP's schema format to standard JSON Schema map format
func convertGoMCPSchemaToMap(gomcpSchema map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy most fields directly
	for key, value := range gomcpSchema {
		if key == "properties" {
			// Convert PropertyDetail map to standard interface{} map
			if propDetails, ok := value.(map[string]schema.PropertyDetail); ok {
				convertedProps := make(map[string]interface{})
				for propName, detail := range propDetails {
					propMap := map[string]interface{}{
						"type": detail.Type,
					}
					if detail.Description != "" {
						propMap["description"] = detail.Description
					}
					if len(detail.Enum) > 0 {
						propMap["enum"] = detail.Enum
					}
					if detail.Format != "" {
						propMap["format"] = detail.Format
					}
					if detail.Minimum != nil {
						propMap["minimum"] = *detail.Minimum
					}
					if detail.Maximum != nil {
						propMap["maximum"] = *detail.Maximum
					}
					if detail.MinLength != nil {
						propMap["minLength"] = *detail.MinLength
					}
					if detail.MaxLength != nil {
						propMap["maxLength"] = *detail.MaxLength
					}
					if detail.Pattern != "" {
						propMap["pattern"] = detail.Pattern
					}
					if detail.Default != nil {
						propMap["default"] = detail.Default
					}
					convertedProps[propName] = propMap
				}
				result[key] = convertedProps
			} else {
				result[key] = value
			}
		} else {
			result[key] = value
		}
	}

	return result
}

///////////////////////////////////////////////////////////////////////////////
// Accessors (unchanged)
///////////////////////////////////////////////////////////////////////////////

func (r *ToolRegistry) GetTool(name string) (*ToolRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) GetAllTools() map[string]*ToolRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make(map[string]*ToolRegistration, len(r.tools))
	for k, v := range r.tools {
		cp[k] = v
	}
	return cp
}

func (r *ToolRegistry) Freeze() { r.mu.Lock(); r.frozen = true; r.mu.Unlock() }
func (r *ToolRegistry) IsFrozen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.frozen
}

type ProgressCallback func(stage string, percent float64, message string)

// LongRunningTool indicates a tool can stream progress updates.
type LongRunningTool interface {
	ExecuteWithProgress(ctx context.Context, args interface{},
		cb ProgressCallback) (interface{}, error)
}

// ExecuteTool runs a registered tool by name with raw JSON arguments.
func (r *ToolRegistry) ExecuteTool(ctx context.Context, name string, raw json.RawMessage) (interface{}, error) {
	reg, ok := r.GetTool(name)
	if !ok {
		return nil, types.NewRichError("INVALID_REQUEST", fmt.Sprintf("tool %s not found", name), "validation_error")
	}

	r.logger.Debug().Str("tool", name).Msg("executing tool")
	res, err := reg.Handler(ctx, raw)
	if err != nil {
		r.logger.Error().Err(err).Str("tool", name).Msg("tool execution failed")
		return nil, err
	}
	r.logger.Debug().Str("tool", name).Msg("tool execution completed")
	return res, nil
}
