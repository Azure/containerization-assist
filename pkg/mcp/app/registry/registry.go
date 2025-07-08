package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Registry is the unified registry implementation that consolidates all registry variants
type Registry struct {
	tools map[string]api.Tool
	mu    sync.RWMutex
	opts  options
}

// options contains configuration for the registry
type options struct {
	maxTools      int
	enableMetrics bool
	namespace     string
	cacheTTL      time.Duration
}

// Option configures registry behavior
type Option func(*options)

// WithMaxTools sets the maximum number of tools that can be registered
func WithMaxTools(n int) Option {
	return func(o *options) { o.maxTools = n }
}

// WithMetrics enables or disables metrics collection
func WithMetrics(enabled bool) Option {
	return func(o *options) { o.enableMetrics = enabled }
}

// WithNamespace sets the registry namespace
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithCacheTTL sets the cache TTL for registry operations
func WithCacheTTL(ttl time.Duration) Option {
	return func(o *options) { o.cacheTTL = ttl }
}

// New creates a new unified registry with the given options
func New(opts ...Option) *Registry {
	o := options{
		maxTools:      1000,
		enableMetrics: true,
		namespace:     "default",
		cacheTTL:      5 * time.Minute,
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &Registry{
		tools: make(map[string]api.Tool),
		opts:  o,
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool api.Tool, opts ...api.RegistryOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

	if len(r.tools) >= r.opts.maxTools {
		return errors.NewError().
			Code(errors.CodeResourceExhausted).
			Type(errors.ErrTypeValidation).
			Message(fmt.Sprintf("registry full: %d tools registered", len(r.tools))).Build()
	}

	r.tools[name] = tool
	return nil
}

// Unregister removes a tool from the registry
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Message("tool not found for unregistration").
			Context("tool_name", name).Build()
	}

	delete(r.tools, name)
	return nil
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (api.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Message("tool not found").
			Context("tool_name", name).Build()
	}

	return tool, nil
}

// List returns all registered tool names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ListByCategory returns tools filtered by category
func (r *Registry) ListByCategory(category api.ToolCategory) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, tool := range r.tools {
		if tool.Schema().Category == category {
			names = append(names, name)
		}
	}
	return names
}

// ListByTags returns tools that match any of the given tags
func (r *Registry) ListByTags(tags ...string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name, tool := range r.tools {
		toolTags := tool.Schema().Tags
		for _, tag := range tags {
			for _, toolTag := range toolTags {
				if tag == toolTag {
					names = append(names, name)
					break
				}
			}
		}
	}
	return names
}

// Execute runs a tool with the given input
func (r *Registry) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	tool, err := r.Get(name)
	if err != nil {
		return api.ToolOutput{}, err
	}

	return tool.Execute(ctx, input)
}

// ExecuteWithRetry runs a tool with automatic retry on failure
func (r *Registry) ExecuteWithRetry(ctx context.Context, name string, input api.ToolInput, policy api.RetryPolicy) (api.ToolOutput, error) {
	var lastErr error
	
	for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
		result, err := r.Execute(ctx, name, input)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		
		// If this is the last attempt, don't sleep
		if attempt < policy.MaxAttempts-1 {
			time.Sleep(policy.InitialDelay)
		}
	}
	
	return api.ToolOutput{}, lastErr
}

// GetMetadata returns metadata about a registered tool
func (r *Registry) GetMetadata(name string) (api.ToolMetadata, error) {
	tool, err := r.Get(name)
	if err != nil {
		return api.ToolMetadata{}, err
	}

	schema := tool.Schema()
	return api.ToolMetadata{
		Name:        name,
		Description: tool.Description(),
		Version:     schema.Version,
		Category:    schema.Category,
		Tags:        schema.Tags,
		Status:      api.StatusActive,
	}, nil
}

// GetStatus returns the current status of a tool
func (r *Registry) GetStatus(name string) (api.ToolStatus, error) {
	_, err := r.Get(name)
	if err != nil {
		return api.StatusInactive, err
	}

	return api.StatusActive, nil
}

// SetStatus updates the status of a tool
func (r *Registry) SetStatus(name string, status api.ToolStatus) error {
	_, err := r.Get(name)
	if err != nil {
		return err
	}

	// For now, this is a no-op as we don't maintain separate status
	// In a full implementation, we would maintain status metadata
	return nil
}

// Stats returns registry statistics
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RegistryStats{
		ToolCount: len(r.tools),
		MaxTools:  r.opts.maxTools,
		Namespace: r.opts.namespace,
	}
}

// Close releases all resources used by the registry
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear all tools
	r.tools = make(map[string]api.Tool)
	return nil
}

// RegistryStats contains statistics about the registry
type RegistryStats struct {
	ToolCount int    `json:"tool_count"`
	MaxTools  int    `json:"max_tools"`
	Namespace string `json:"namespace"`
}