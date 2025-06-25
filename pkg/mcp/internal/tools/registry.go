// pkg/mcp/tools/registry.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/alecthomas/jsonschema"
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
	if _, dup := reg.tools[t.GetName()]; dup {
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("tool %s already registered", t.GetName()), "validation_error")
	}

	reflector := jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true, // respects `jsonschema:",required"`
		AllowAdditionalProperties:  false,
		DoNotReference:             true, // no "$ref" / "$defs"
	}
	var a TArgs
	var r TResult

	// Generate schemas using reflector
	inputSchema := reflector.Reflect(a)
	outputSchema := reflector.Reflect(r)

	inputSchema.Version = ""  // drop $schema
	outputSchema.Version = "" // drop $schema

	cleanInput := sanitizeSchema(inputSchema)
	cleanOutput := sanitizeSchema(outputSchema)

	reg.tools[t.GetName()] = &ToolRegistration{
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
		Str("tool", t.GetName()).
		Str("version", t.GetVersion()).
		Msg("registered")
	return nil
}

// sanitizeSchema converts a *jsonschema.Schema to map[string]any and removes
// every keyword Copilot's AJV-Draft-07 validator chokes on.
func sanitizeSchema(raw *jsonschema.Schema) map[string]any {
	// 1. Marshal + unmarshal once so we can walk the structure.
	b, err := json.Marshal(raw)
	if err != nil {
		// Return empty map if marshaling fails
		return make(map[string]any)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		// Return empty map if unmarshaling fails
		return make(map[string]any)
	}

	removeCopilotIncompatible(m)
	return m
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
