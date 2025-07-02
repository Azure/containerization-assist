package orchestration

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
	"github.com/rs/zerolog"
)

// GenericRegistry provides type-safe tool registration and orchestration
type GenericRegistry[T tools.Tool[TParams, TResult], TParams tools.ToolParams, TResult tools.ToolResult] struct {
	tools    map[string]T
	schemas  map[string]tools.Schema[TParams, TResult]
	metadata map[string]GenericToolMetadata
	mu       sync.RWMutex
	logger   zerolog.Logger
}

// GenericToolMetadata contains metadata about registered tools in generic registry
type GenericToolMetadata struct {
	Name         string
	Description  string
	Version      string
	Category     string
	RegisteredAt time.Time
	LastUsed     time.Time
	UsageCount   int64
	Enabled      bool
	Tags         []string
}

// NewGenericRegistry creates a new type-safe registry
func NewGenericRegistry[T tools.Tool[TParams, TResult], TParams tools.ToolParams, TResult tools.ToolResult](logger zerolog.Logger) *GenericRegistry[T, TParams, TResult] {
	return &GenericRegistry[T, TParams, TResult]{
		tools:    make(map[string]T),
		schemas:  make(map[string]tools.Schema[TParams, TResult]),
		metadata: make(map[string]GenericToolMetadata),
		logger:   logger.With().Str("component", "generic_registry").Logger(),
	}
}

// Register adds a tool to the registry with type safety and conflict detection
func (r *GenericRegistry[T, TParams, TResult]) Register(name string, tool T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for tool name conflicts
	if _, exists := r.tools[name]; exists {
		return rich.NewError().
			Code("TOOL_ALREADY_REGISTERED").
			Message("Tool with this name is already registered").
			Type(rich.ErrTypeBusiness).
			Severity(rich.SeverityMedium).
			Context("tool_name", name).
			Context("existing_description", r.metadata[name].Description).
			Suggestion("Use a different tool name or unregister the existing tool first").
			WithLocation().
			Build()
	}

	// Validate tool
	if name == "" {
		return rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Tool name cannot be empty").
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Suggestion("Provide a valid tool name").
			WithLocation().
			Build()
	}

	// Register the tool
	r.tools[name] = tool
	r.schemas[name] = tool.GetSchema()
	r.metadata[name] = GenericToolMetadata{
		Name:         name,
		Description:  tool.GetDescription(),
		Version:      "1.0.0", // Would come from tool in real implementation
		Category:     "generic",
		RegisteredAt: time.Now(),
		Enabled:      true,
		Tags:         []string{},
	}

	r.logger.Info().
		Str("tool_name", name).
		Str("description", tool.GetDescription()).
		Msg("Tool registered successfully")

	return nil
}

// Unregister removes a tool from the registry
func (r *GenericRegistry[T, TParams, TResult]) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return rich.NewError().
			Code("TOOL_NOT_FOUND").
			Message("Tool not found in registry").
			Type(rich.ErrTypeNotFound).
			Severity(rich.SeverityLow).
			Context("tool_name", name).
			Context("available_tools", r.getToolNamesList()).
			Suggestion("Check tool name spelling and available tools").
			WithLocation().
			Build()
	}

	delete(r.tools, name)
	delete(r.schemas, name)
	delete(r.metadata, name)

	r.logger.Info().Str("tool_name", name).Msg("Tool unregistered successfully")
	return nil
}

// Get retrieves a tool by name with type safety
func (r *GenericRegistry[T, TParams, TResult]) Get(name string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		var zeroTool T
		return zeroTool, rich.NewError().
			Code("TOOL_NOT_FOUND").
			Message("Tool not found in registry").
			Type(rich.ErrTypeNotFound).
			Severity(rich.SeverityLow).
			Context("tool_name", name).
			Context("available_tools", r.getToolNamesList()).
			Suggestion("Check tool name spelling and available tools").
			WithLocation().
			Build()
	}

	// Check if tool is enabled
	if metadata, ok := r.metadata[name]; ok && !metadata.Enabled {
		var zeroTool T
		return zeroTool, rich.NewError().
			Code("TOOL_DISABLED").
			Message("Tool is currently disabled").
			Type(rich.ErrTypeBusiness).
			Severity(rich.SeverityMedium).
			Context("tool_name", name).
			Suggestion("Enable the tool or use an alternative").
			WithLocation().
			Build()
	}

	return tool, nil
}

// Execute runs a tool with the given parameters and comprehensive error handling
func (r *GenericRegistry[T, TParams, TResult]) Execute(ctx context.Context, name string, params TParams) (TResult, error) {
	startTime := time.Now()

	// Get tool
	tool, err := r.Get(name)
	if err != nil {
		var zeroResult TResult
		return zeroResult, err
	}

	// Update usage metadata
	r.updateUsageMetadata(name, startTime)

	// Validate parameters
	if err := params.Validate(); err != nil {
		var zeroResult TResult
		return zeroResult, rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Tool parameter validation failed").
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Cause(err).
			Context("tool_name", name).
			Context("session_id", params.GetSessionID()).
			Suggestion("Check parameter types and required fields").
			WithLocation().
			Build()
	}

	// Execute tool with error context
	result, err := tool.Execute(ctx, params)
	duration := time.Since(startTime)

	if err != nil {
		r.logger.Error().
			Err(err).
			Str("tool_name", name).
			Dur("duration", duration).
			Msg("Tool execution failed")

		// Wrap execution errors with registry context
		return result, rich.NewError().
			Code("TOOL_EXECUTION_FAILED").
			Message("Tool execution failed").
			Type(rich.ErrTypeBusiness).
			Severity(rich.SeverityHigh).
			Cause(err).
			Context("tool_name", name).
			Context("execution_duration", duration.String()).
			Context("session_id", params.GetSessionID()).
			Suggestion("Check tool parameters and system resources").
			WithLocation().
			Build()
	}

	r.logger.Info().
		Str("tool_name", name).
		Dur("duration", duration).
		Bool("success", result.IsSuccess()).
		Msg("Tool executed successfully")

	return result, nil
}

// List returns all registered tool names
func (r *GenericRegistry[T, TParams, TResult]) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getToolNamesList()
}

// ListEnabled returns all enabled tool names
func (r *GenericRegistry[T, TParams, TResult]) ListEnabled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var enabledTools []string
	for name, metadata := range r.metadata {
		if metadata.Enabled {
			enabledTools = append(enabledTools, name)
		}
	}
	return enabledTools
}

// GetMetadata returns metadata for a specific tool
func (r *GenericRegistry[T, TParams, TResult]) GetMetadata(name string) (GenericToolMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return GenericToolMetadata{}, rich.NewError().
			Code("TOOL_NOT_FOUND").
			Message("Tool metadata not found").
			Type(rich.ErrTypeNotFound).
			Context("tool_name", name).
			WithLocation().
			Build()
	}

	return metadata, nil
}

// GetSchema returns the schema for a specific tool
func (r *GenericRegistry[T, TParams, TResult]) GetSchema(name string) (tools.Schema[TParams, TResult], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schema, exists := r.schemas[name]
	if !exists {
		var zeroSchema tools.Schema[TParams, TResult]
		return zeroSchema, rich.NewError().
			Code("TOOL_NOT_FOUND").
			Message("Tool schema not found").
			Type(rich.ErrTypeNotFound).
			Context("tool_name", name).
			WithLocation().
			Build()
	}

	return schema, nil
}

// EnableTool enables a tool for execution
func (r *GenericRegistry[T, TParams, TResult]) EnableTool(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return rich.NewError().
			Code("TOOL_NOT_FOUND").
			Message("Tool not found").
			Type(rich.ErrTypeNotFound).
			Context("tool_name", name).
			WithLocation().
			Build()
	}

	metadata.Enabled = true
	r.metadata[name] = metadata

	r.logger.Info().Str("tool_name", name).Msg("Tool enabled")
	return nil
}

// DisableTool disables a tool to prevent execution
func (r *GenericRegistry[T, TParams, TResult]) DisableTool(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return rich.NewError().
			Code("TOOL_NOT_FOUND").
			Message("Tool not found").
			Type(rich.ErrTypeNotFound).
			Context("tool_name", name).
			WithLocation().
			Build()
	}

	metadata.Enabled = false
	r.metadata[name] = metadata

	r.logger.Info().Str("tool_name", name).Msg("Tool disabled")
	return nil
}

// Count returns the number of registered tools
func (r *GenericRegistry[T, TParams, TResult]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}

// Clear removes all tools from the registry
func (r *GenericRegistry[T, TParams, TResult]) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = make(map[string]T)
	r.schemas = make(map[string]tools.Schema[TParams, TResult])
	r.metadata = make(map[string]GenericToolMetadata)

	r.logger.Info().Msg("Registry cleared")
	return nil
}

// GetStats returns registry statistics
func (r *GenericRegistry[T, TParams, TResult]) GetStats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var totalUsage int64
	var enabledCount int
	var mostUsedTool string
	var maxUsage int64

	for name, metadata := range r.metadata {
		totalUsage += metadata.UsageCount
		if metadata.Enabled {
			enabledCount++
		}
		if metadata.UsageCount > maxUsage {
			maxUsage = metadata.UsageCount
			mostUsedTool = name
		}
	}

	return RegistryStats{
		TotalTools:    len(r.tools),
		EnabledTools:  enabledCount,
		DisabledTools: len(r.tools) - enabledCount,
		TotalUsage:    totalUsage,
		MostUsedTool:  mostUsedTool,
	}
}

// RegistryStats contains registry statistics
type RegistryStats struct {
	TotalTools    int
	EnabledTools  int
	DisabledTools int
	TotalUsage    int64
	MostUsedTool  string
}

// Helper methods

func (r *GenericRegistry[T, TParams, TResult]) getToolNamesList() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func (r *GenericRegistry[T, TParams, TResult]) updateUsageMetadata(name string, timestamp time.Time) {
	// Update metadata without holding the lock for too long
	go func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		if metadata, exists := r.metadata[name]; exists {
			metadata.LastUsed = timestamp
			metadata.UsageCount++
			r.metadata[name] = metadata
		}
	}()
}
