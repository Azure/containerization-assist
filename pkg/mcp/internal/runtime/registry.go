package runtime

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/invopop/jsonschema"
	"github.com/rs/zerolog"
)

type ToolRegistration struct {
	Tool         api.Tool
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

func RegisterTool[TArgs, TResult any](reg *ToolRegistry, t api.Tool) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.frozen {
		return errors.NewError().
			Code("REGISTRY_FROZEN").
			Message("Cannot register tool on frozen registry").
			Type(errors.ErrTypeBusiness).
			Severity(errors.SeverityMedium).
			Context("tool_name", t.Name()).
			Context("registry_state", "frozen").
			Suggestion("Registry is frozen after first tool execution - register all tools during initialization").
			WithLocation().
			Build()
	}
	toolName := t.Name()
	if _, dup := reg.tools[toolName]; dup {
		return errors.NewError().
			Code("TOOL_ALREADY_REGISTERED").
			Message("Tool with this name is already registered").
			Type(errors.ErrTypeBusiness).
			Severity(errors.SeverityMedium).
			Context("tool_name", toolName).
			Context("existing_version", "1.0.0").
			Context("new_version", "1.0.0").
			Suggestion("Use a different tool name or unregister the existing tool first").
			WithLocation().
			Build()
	}

	reg.logger.Info().Str("tool", toolName).Msg("🔧 Using invopop/jsonschema for schema generation with array fixes")

	reflector := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  false,
		DoNotReference:             true,
	}
	var a TArgs
	var r TResult

	inputSchema := reflector.Reflect(a)
	outputSchema := reflector.Reflect(r)

	inputSchema.Version = ""
	outputSchema.Version = ""

	cleanInput := sanitizeInvopopSchema(inputSchema)
	cleanOutput := sanitizeInvopopSchema(outputSchema)

	if hasArrays := containsArrays(cleanInput); hasArrays {
		reg.logger.Info().Str("tool", toolName).Msg("✅ Generated schema with proper array items using invopop/jsonschema")
	}

	reg.tools[toolName] = &ToolRegistration{
		Tool:         t,
		InputSchema:  cleanInput,
		OutputSchema: cleanOutput,
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args TArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, errors.NewError().
					Code("TOOL_PARAMETER_UNMARSHAL_FAILED").
					Message("Failed to unmarshal tool parameters").
					Type(errors.ErrTypeValidation).
					Severity(errors.SeverityMedium).
					Cause(err).
					Context("tool_name", toolName).
					Context("raw_args", string(raw)).
					Suggestion("Check parameter format matches the tool's expected schema").
					WithLocation().
					Build()
			}
			toolInput := api.ToolInput{
				SessionID: "",
				Data:      map[string]interface{}{"args": args},
			}
			toolOutput, err := t.Execute(ctx, toolInput)
			if err != nil {
				return nil, err
			}
			return toolOutput.Data, nil
		},
	}

	reg.logger.Info().
		Str("tool", toolName).
		Str("version", "1.0.0").
		Msg("registered")
	return nil
}

func sanitizeInvopopSchema(schema *jsonschema.Schema) map[string]interface{} {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return make(map[string]interface{})
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return make(map[string]interface{})
	}

	utils.RemoveCopilotIncompatible(schemaMap)

	return schemaMap
}

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

func (r *ToolRegistry) ExecuteTool(ctx context.Context, name string, raw json.RawMessage) (interface{}, error) {
	reg, ok := r.GetTool(name)
	if !ok {
		return nil, errors.NewError().Messagef("registry operation failed").Build()
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
