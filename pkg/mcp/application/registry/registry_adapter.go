package registry

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Adapter adapts ToolRegistry to the api.Registry interface
type Adapter struct {
	toolRegistry api.ToolRegistry
}

// NewRegistryAdapter creates a new adapter from ToolRegistry to Registry
func NewRegistryAdapter(toolRegistry api.ToolRegistry) api.Registry {
	return &Adapter{
		toolRegistry: toolRegistry,
	}
}

// Register adds a tool to the registry
func (r *Adapter) Register(tool api.Tool, _ ...api.RegistryOption) error {
	if tool == nil {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("tool cannot be nil").
			Build()
	}

	// Create a factory that returns the tool instance
	factory := ToolFactory(func() (api.Tool, error) {
		return tool, nil
	})

	return r.toolRegistry.Register(tool.Name(), factory)
}

// Unregister removes a tool from the registry
func (r *Adapter) Unregister(name string) error {
	return r.toolRegistry.Unregister(name)
}

// Get retrieves a tool by name
func (r *Adapter) Get(name string) (api.Tool, error) {
	result, err := r.toolRegistry.Discover(name)
	if err != nil {
		return nil, err
	}

	tool, ok := result.(api.Tool)
	if !ok {
		return nil, errors.NewError().
			Code(errors.CodeTypeMismatch).
			Message("discovered item is not a tool").
			Context("tool_name", name).
			Build()
	}

	return tool, nil
}

// List returns all registered tool names
func (r *Adapter) List() []string {
	return r.toolRegistry.List()
}

// ListByCategory returns tools filtered by category
func (r *Adapter) ListByCategory(category api.ToolCategory) []string {
	var result []string
	for _, name := range r.toolRegistry.List() {
		if metadata, err := r.toolRegistry.Metadata(name); err == nil {
			if metadata.Category == category {
				result = append(result, name)
			}
		}
	}
	return result
}

// ListByTags returns tools that match any of the given tags
func (r *Adapter) ListByTags(tags ...string) []string {
	if len(tags) == 0 {
		return []string{}
	}

	var result []string
	for _, name := range r.toolRegistry.List() {
		if metadata, err := r.toolRegistry.Metadata(name); err == nil {
			for _, tag := range tags {
				for _, metaTag := range metadata.Tags {
					if metaTag == tag {
						result = append(result, name)
						goto nextTool
					}
				}
			}
		nextTool:
		}
	}
	return result
}

// Execute runs a tool with the given input
func (r *Adapter) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	return r.toolRegistry.Execute(ctx, name, input)
}

// ExecuteWithRetry runs a tool with automatic retry on failure
func (r *Adapter) ExecuteWithRetry(ctx context.Context, name string, input api.ToolInput, policy api.RetryPolicy) (api.ToolOutput, error) {
	// Simple retry implementation - execute with retry logic
	var lastErr error
	delay := policy.InitialDelay

	for attempt := 0; attempt <= policy.MaxAttempts; attempt++ {
		result, err := r.toolRegistry.Execute(ctx, name, input)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if attempt < policy.MaxAttempts {
			// Wait for retry delay
			select {
			case <-ctx.Done():
				return api.ToolOutput{}, ctx.Err()
			case <-time.After(delay):
				// Apply exponential backoff
				delay = time.Duration(float64(delay) * policy.BackoffMultiplier)
				if delay > policy.MaxDelay {
					delay = policy.MaxDelay
				}
			}
		}
	}
	return api.ToolOutput{}, lastErr
}

// GetMetadata returns metadata for a tool
func (r *Adapter) GetMetadata(name string) (api.ToolMetadata, error) {
	return r.toolRegistry.Metadata(name)
}

// UpdateMetadata updates metadata for a tool
func (r *Adapter) UpdateMetadata(name string, metadata api.ToolMetadata) error {
	return r.toolRegistry.SetMetadata(name, metadata)
}

// GetStatus returns the current status of a tool
func (r *Adapter) GetStatus(name string) (api.ToolStatus, error) {
	metadata, err := r.toolRegistry.Metadata(name)
	if err != nil {
		return "", err
	}
	return metadata.Status, nil
}

// SetStatus updates the status of a tool
func (r *Adapter) SetStatus(name string, status api.ToolStatus) error {
	metadata, err := r.toolRegistry.Metadata(name)
	if err != nil {
		return err
	}
	metadata.Status = status
	return r.toolRegistry.SetMetadata(name, metadata)
}

// Subscribe registers a callback for registry events (optional monitoring)
func (r *Adapter) Subscribe(_ api.RegistryEventType, _ api.RegistryEventCallback) error {
	// Not implemented - would need event system in ToolRegistry
	return errors.NewError().
		Code(errors.CodeNotImplemented).
		Message("event subscription not implemented").
		Build()
}

// Unsubscribe removes a callback (optional monitoring)
func (r *Adapter) Unsubscribe(_ api.RegistryEventType, _ api.RegistryEventCallback) error {
	// Not implemented - would need event system in ToolRegistry
	return errors.NewError().
		Code(errors.CodeNotImplemented).
		Message("event unsubscription not implemented").
		Build()
}

// GetMetrics returns registry metrics
func (r *Adapter) GetMetrics() api.RegistryMetrics {
	// Try to get metrics if the tool registry supports it
	if metricsProvider, ok := r.toolRegistry.(interface{ GetMetrics() api.RegistryMetrics }); ok {
		return metricsProvider.GetMetrics()
	}

	// Return basic metrics based on available data
	tools := r.toolRegistry.List()
	return api.RegistryMetrics{
		TotalTools:  len(tools),
		ActiveTools: len(tools),
		// Other metrics would need to be tracked separately
	}
}

// Close releases resources used by the registry
func (r *Adapter) Close() error {
	return r.toolRegistry.Close()
}

// Ensure Adapter implements api.Registry
var _ api.Registry = (*Adapter)(nil)
