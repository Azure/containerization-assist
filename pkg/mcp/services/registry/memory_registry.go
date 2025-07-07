package registry

import (
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MemoryToolRegistry implements ToolRegistry interface using in-memory storage
type MemoryToolRegistry struct {
	tools     map[string]api.Tool
	metadata  map[string]api.ToolMetadata
	toolsMux  sync.RWMutex
	createdAt time.Time
}

// NewMemoryToolRegistry creates a new in-memory tool registry
func NewMemoryToolRegistry() *MemoryToolRegistry {
	return &MemoryToolRegistry{
		tools:     make(map[string]api.Tool),
		metadata:  make(map[string]api.ToolMetadata),
		createdAt: time.Now(),
	}
}

// RegisterTool implements ToolRegistry.RegisterTool
func (r *MemoryToolRegistry) RegisterTool(tool api.Tool, opts ...api.RegistryOption) error {
	r.toolsMux.Lock()
	defer r.toolsMux.Unlock()

	name := tool.Name()
	if name == "" {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("tool name cannot be empty").Build()
	}

	if _, exists := r.tools[name]; exists {
		return errors.NewError().
			Code(errors.CodeResourceAlreadyExists).
			Type(errors.ErrTypeValidation).
			Message("tool already registered").
			Context("tool_name", name).Build()
	}

	config := &api.RegistryConfig{
		Enabled:     true,
		Priority:    0,
		Metadata:    make(map[string]interface{}),
		Timeout:     30 * time.Second,
		Concurrency: 1,
	}

	for _, opt := range opts {
		opt(config)
	}

	r.tools[name] = tool

	schema := tool.Schema()
	metadata := api.ToolMetadata{
		Name:                 name,
		Description:          tool.Description(),
		Version:              schema.Version,
		Category:             schema.Category,
		Tags:                 schema.Tags,
		Status:               api.StatusActive,
		RegisteredAt:         time.Now(),
		LastModified:         time.Now(),
		ExecutionCount:       0,
		AverageExecutionTime: 0,
	}

	if config.Metadata != nil {
		metadata.Requirements = make([]string, 0)
		metadata.Capabilities = make([]string, 0)
		metadata.Dependencies = make([]string, 0)

		if deps, ok := config.Metadata["dependencies"].([]string); ok {
			metadata.Dependencies = deps
		}
		if caps, ok := config.Metadata["capabilities"].([]string); ok {
			metadata.Capabilities = caps
		}
		if reqs, ok := config.Metadata["requirements"].([]string); ok {
			metadata.Requirements = reqs
		}
	}

	r.metadata[name] = metadata

	return nil
}

// UnregisterTool implements ToolRegistry.UnregisterTool
func (r *MemoryToolRegistry) UnregisterTool(name string) error {
	r.toolsMux.Lock()
	defer r.toolsMux.Unlock()

	if _, exists := r.tools[name]; !exists {
		return errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Message("tool not found for unregistration").
			Context("tool_name", name).Build()
	}

	delete(r.tools, name)
	delete(r.metadata, name)

	return nil
}

// GetTool implements ToolRegistry.GetTool
func (r *MemoryToolRegistry) GetTool(name string) (api.Tool, error) {
	r.toolsMux.RLock()
	defer r.toolsMux.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Message("tool not found").
			Context("tool_name", name).Build()
	}

	if metadata, exists := r.metadata[name]; exists {
		now := time.Now()
		metadata.LastExecuted = &now
		metadata.ExecutionCount++
		r.metadata[name] = metadata
	}

	return tool, nil
}

// ListTools implements ToolRegistry.ListTools
func (r *MemoryToolRegistry) ListTools() []string {
	r.toolsMux.RLock()
	defer r.toolsMux.RUnlock()

	tools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		tools = append(tools, name)
	}

	return tools
}

// GetMetadata implements ToolRegistry.GetMetadata
func (r *MemoryToolRegistry) GetMetadata(name string) (api.ToolMetadata, error) {
	r.toolsMux.RLock()
	defer r.toolsMux.RUnlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return api.ToolMetadata{}, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Message("tool metadata not found").
			Context("tool_name", name).Build()
	}

	return metadata, nil
}

// GetStats returns registry statistics
func (r *MemoryToolRegistry) GetStats() api.RegistryStats {
	r.toolsMux.RLock()
	defer r.toolsMux.RUnlock()

	var totalExecutions, failedExecutions int64
	var totalDuration time.Duration
	var lastExecution *time.Time
	activeTools := 0

	for _, metadata := range r.metadata {
		totalExecutions += metadata.ExecutionCount
		totalDuration += metadata.AverageExecutionTime

		if metadata.Status == api.StatusActive {
			activeTools++
		}

		if metadata.LastExecuted != nil {
			if lastExecution == nil || metadata.LastExecuted.After(*lastExecution) {
				lastExecution = metadata.LastExecuted
			}
		}
	}

	var avgExecTime time.Duration
	if len(r.tools) > 0 {
		avgExecTime = totalDuration / time.Duration(len(r.tools))
	}

	return api.RegistryStats{
		TotalTools:       len(r.tools),
		ActiveTools:      activeTools,
		TotalExecutions:  totalExecutions,
		FailedExecutions: failedExecutions,
		AverageExecTime:  avgExecTime,
		LastExecution:    lastExecution,
		UpTime:           time.Since(r.createdAt),
	}
}

// Close closes the registry (no resources to close for memory registry)
func (r *MemoryToolRegistry) Close() error {
	r.toolsMux.Lock()
	defer r.toolsMux.Unlock()

	r.tools = make(map[string]api.Tool)
	r.metadata = make(map[string]api.ToolMetadata)

	return nil
}
