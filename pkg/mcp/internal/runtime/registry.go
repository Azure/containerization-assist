// pkg/mcp/tools/registry.go
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/invopop/jsonschema"
	"github.com/rs/zerolog"
)

// mcp import removed - using mcptypes

///////////////////////////////////////////////////////////////////////////////
// Contracts
///////////////////////////////////////////////////////////////////////////////

type ExecutableTool[TArgs, TResult any] interface {
	core.Tool
	PreValidate(ctx context.Context, args TArgs) error
}

///////////////////////////////////////////////////////////////////////////////
// Registry primitives
///////////////////////////////////////////////////////////////////////////////

type ToolRegistration struct {
	Tool         core.Tool
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
		return fmt.Errorf("registry operation failed")
	}
	metadata := t.GetMetadata()
	if _, dup := reg.tools[metadata.Name]; dup {
		return fmt.Errorf("registry operation failed")
	}

	// Use invopop/jsonschema which properly handles array items
	reg.logger.Info().Str("tool", metadata.Name).Msg("🔧 Using invopop/jsonschema for schema generation with array fixes")

	reflector := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  false,
		DoNotReference:             true, // avoid $ref/$defs for better compatibility
	}
	var a TArgs
	var r TResult

	// Generate schemas using invopop reflector
	inputSchema := reflector.Reflect(a)
	outputSchema := reflector.Reflect(r)

	// Remove schema version for compatibility
	inputSchema.Version = ""
	outputSchema.Version = ""

	// Convert to map format and apply compatibility fixes
	cleanInput := sanitizeInvopopSchema(inputSchema)
	cleanOutput := sanitizeInvopopSchema(outputSchema)

	// Log if we fixed any arrays
	if hasArrays := containsArrays(cleanInput); hasArrays {
		reg.logger.Info().Str("tool", metadata.Name).Msg("✅ Generated schema with proper array items using invopop/jsonschema")
	}

	reg.tools[metadata.Name] = &ToolRegistration{
		Tool:         t,
		InputSchema:  cleanInput,
		OutputSchema: cleanOutput,
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args TArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("registry operation failed")
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

// sanitizeInvopopSchema converts invopop jsonschema.Schema to map[string]any
// and removes keywords that GitHub Copilot's AJV-Draft-7 validator cannot handle
func sanitizeInvopopSchema(schema *jsonschema.Schema) map[string]interface{} {
	// Marshal and unmarshal to get map format
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return make(map[string]interface{})
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return make(map[string]interface{})
	}

	// Apply GitHub Copilot compatibility fixes
	utils.RemoveCopilotIncompatible(schemaMap)

	return schemaMap
}

// containsArrays checks if a schema contains any array fields (for logging purposes)
func containsArrays(schema map[string]interface{}) bool {
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for _, prop := range properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				if propMap["type"] == "array" {
					return true
				}
			}
		}
	}
	return false
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

type ToolProgressCallback func(stage string, percent float64, message string)

// LongRunningTool indicates a tool can stream progress updates.
type LongRunningTool interface {
	ExecuteWithProgress(ctx context.Context, args interface{},
		cb ToolProgressCallback) (interface{}, error)
}

// ExecuteTool runs a registered tool by name with raw JSON arguments.
func (r *ToolRegistry) ExecuteTool(ctx context.Context, name string, raw json.RawMessage) (interface{}, error) {
	reg, ok := r.GetTool(name)
	if !ok {
		return nil, fmt.Errorf("registry operation failed")
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
