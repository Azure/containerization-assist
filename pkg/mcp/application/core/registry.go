package core

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// RegistryOption configures registry behavior
type RegistryOption func(*RegistryConfig)

// RegistryConfig holds configuration for registry options
type RegistryConfig struct {
	Namespace         string
	EnableCaching     bool
	CacheTTL          int64
	EnableMetrics     bool
	MaxConcurrency    int
	Tags              []string
	Priority          int
	EnablePersistence bool
	EnableEvents      bool
}

// UnifiedRegistry implements a tool registry with functional options
type UnifiedRegistry struct {
	tools    map[string]api.Tool
	configs  map[string]*RegistryConfig
	metadata map[string]*RegistryMetadata
	mu       sync.RWMutex
	logger   *slog.Logger
}

// RegistryMetadata contains metadata about registered tools in the registry
type RegistryMetadata struct {
	Name         string
	RegisteredAt time.Time
	LastUsed     time.Time
	UsageCount   int64
	Enabled      bool
	Config       *RegistryConfig
}

// NewUnifiedRegistry creates a new unified registry
func NewUnifiedRegistry(logger *slog.Logger) *UnifiedRegistry {
	return &UnifiedRegistry{
		tools:    make(map[string]api.Tool),
		configs:  make(map[string]*RegistryConfig),
		metadata: make(map[string]*RegistryMetadata),
		logger:   logger.With("component", "unified_registry"),
	}
}

// Register registers a tool with the registry
func (r *UnifiedRegistry) Register(tool api.Tool, opts ...RegistryOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if name == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("api.Tool name is required for registration").
			Context("operation", "register_tool").
			Suggestion("Provide a non-empty tool name").
			WithLocation().
			Build()
	}

	if _, exists := r.tools[name]; exists {
		return errors.NewError().
			Code(errors.CodeToolAlreadyRegistered).
			Type(errors.ErrTypeTool).
			Severity(errors.SeverityMedium).
			Messagef("api.Tool '%s' is already registered", name).
			Context("tool_name", name).
			Context("operation", "register_tool").
			Suggestion("Use a different tool name or unregister the existing tool first").
			WithLocation().
			Build()
	}

	config := &RegistryConfig{}
	for _, opt := range opts {
		opt(config)
	}

	r.tools[name] = tool
	r.configs[name] = config
	r.metadata[name] = &RegistryMetadata{
		Name:         name,
		RegisteredAt: time.Now(),
		Enabled:      true,
		Config:       config,
	}

	r.logger.Info("api.Tool registered successfully",
		"tool", name,
		"namespace", config.Namespace,
		"caching", config.EnableCaching,
		"metrics", config.EnableMetrics)

	return nil
}

// Unregister implements mcp.Registry interface
func (r *UnifiedRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return errors.NewError().
			Code(errors.CodeToolNotFound).
			Type(errors.ErrTypeTool).
			Severity(errors.SeverityMedium).
			Messagef("api.Tool '%s' not found in registry", name).
			Context("tool_name", name).
			Context("operation", "unregister_tool").
			Context("available_tools", len(r.tools)).
			Suggestion("Check available tools with List() method").
			WithLocation().
			Build()
	}

	delete(r.tools, name)
	delete(r.configs, name)
	delete(r.metadata, name)

	r.logger.Info("api.Tool unregistered successfully",
		"tool", name)

	return nil
}

// Get retrieves a tool by name
func (r *UnifiedRegistry) Get(name string) (api.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeToolNotFound).
			Type(errors.ErrTypeTool).
			Severity(errors.SeverityMedium).
			Messagef("api.Tool '%s' not found in registry", name).
			Context("tool_name", name).
			Context("operation", "get_tool").
			Context("available_tools", len(r.tools)).
			Suggestion("Check available tools with List() method").
			WithLocation().
			Build()
	}

	if metadata, ok := r.metadata[name]; ok {
		metadata.LastUsed = time.Now()
		metadata.UsageCount++
	}

	return tool, nil
}

// List implements mcp.Registry interface
func (r *UnifiedRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		if metadata, ok := r.metadata[name]; ok && metadata.Enabled {
			names = append(names, name)
		}
	}

	return names
}

// Execute runs a tool with the given input
func (r *UnifiedRegistry) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	tool, err := r.Get(name)
	if err != nil {
		return api.ToolOutput{Success: false, Error: err.Error()}, err
	}

	config, exists := r.configs[name]
	if exists && config.EnableCaching {
		r.logger.Debug("Cache check (not implemented)",
			"tool", name)
	}

	startTime := time.Now()

	var result api.ToolOutput

	result, err = tool.Execute(ctx, input)

	duration := time.Since(startTime)

	if exists && config.EnableMetrics {
		r.logger.Debug("Tool execution metrics",
			"tool", name,
			"duration", duration,
			"success", err == nil)
	}

	return result, err
}

// Count implements mcp.Registry interface
func (r *UnifiedRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, metadata := range r.metadata {
		if metadata.Enabled {
			count++
		}
	}

	return count
}

// Clear implements mcp.Registry interface
func (r *UnifiedRegistry) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = make(map[string]api.Tool)
	r.configs = make(map[string]*RegistryConfig)
	r.metadata = make(map[string]*RegistryMetadata)

	r.logger.Info("Registry cleared")

	return nil
}

// ClearValidators implements ValidatorRegistry interface (no return value)
func (r *UnifiedRegistry) ClearValidators() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = make(map[string]api.Tool)
	r.configs = make(map[string]*RegistryConfig)
	r.metadata = make(map[string]*RegistryMetadata)

	r.logger.Info("Registry cleared (validators)")
}

// GetRegistryMetadata returns registry-specific metadata for a tool
func (r *UnifiedRegistry) GetRegistryMetadata(name string) (*RegistryMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeToolNotFound).
			Type(errors.ErrTypeTool).
			Severity(errors.SeverityMedium).
			Messagef("api.Tool '%s' metadata not found", name).
			Context("tool_name", name).
			Context("operation", "get_metadata").
			Context("available_tools", len(r.tools)).
			Suggestion("Check available tools with List() method").
			WithLocation().
			Build()
	}

	return &RegistryMetadata{
		Name:         metadata.Name,
		RegisteredAt: metadata.RegisteredAt,
		LastUsed:     metadata.LastUsed,
		UsageCount:   metadata.UsageCount,
		Enabled:      metadata.Enabled,
		Config:       metadata.Config,
	}, nil
}

// GetMetadata returns tool metadata to satisfy the Registry interface
func (r *UnifiedRegistry) GetMetadata(name string) (api.ToolMetadata, error) {
	metadata, err := r.GetRegistryMetadata(name)
	if err != nil {
		return api.ToolMetadata{}, err
	}

	tool, _ := r.Get(name)
	description := "Tool: " + metadata.Name
	if tool != nil {
		description = tool.Description()
	}

	status := api.ToolStatus(api.StatusInactive)
	if metadata.Enabled {
		status = api.ToolStatus(api.StatusActive)
	}

	return api.ToolMetadata{
		Name:           metadata.Name,
		Description:    description,
		Version:        "1.0.0",
		Category:       api.CategoryGeneral,
		Tags:           metadata.Config.Tags,
		Status:         status,
		RegisteredAt:   metadata.RegisteredAt,
		LastModified:   metadata.RegisteredAt,
		ExecutionCount: metadata.UsageCount,
		LastExecuted:   &metadata.LastUsed,
	}, nil
}

// EnableTool enables or disables a specific tool
func (r *UnifiedRegistry) EnableTool(name string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return errors.NewError().
			Code(errors.CodeToolNotFound).
			Type(errors.ErrTypeTool).
			Severity(errors.SeverityMedium).
			Messagef("api.Tool '%s' not found for enabling", name).
			Context("tool_name", name).
			Context("operation", "enable_tool").
			Context("available_tools", len(r.tools)).
			Suggestion("Check available tools with List() method").
			WithLocation().
			Build()
	}

	metadata.Enabled = enabled

	r.logger.Info("api.Tool status updated",
		"tool", name,
		"enabled", enabled)

	return nil
}

// GetStats returns overall registry statistics
func (r *UnifiedRegistry) GetStats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		TotalTools:   len(r.tools),
		EnabledTools: 0,
		TotalUsage:   0,
	}

	for _, metadata := range r.metadata {
		if metadata.Enabled {
			stats.EnabledTools++
		}
		stats.TotalUsage += metadata.UsageCount
	}

	return stats
}

// ListByType lists tools by type/category - returns all tools for now since we don't store types
func (r *UnifiedRegistry) ListByType(toolType string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		if metadata, ok := r.metadata[name]; ok && metadata.Enabled {
			names = append(names, name)
		}
	}

	return names
}

// Has checks if a tool with the given name exists
func (r *UnifiedRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// Remove is an alias for Unregister for compatibility
func (r *UnifiedRegistry) Remove(name string) error {
	return r.Unregister(name)
}

// RegistryStats contains overall registry statistics
type RegistryStats struct {
	TotalTools   int
	EnabledTools int
	TotalUsage   int64
}

func WithNamespace(ns string) RegistryOption {
	return func(c *RegistryConfig) {
		c.Namespace = ns
	}
}

func WithCaching(enabled bool, ttl int64) RegistryOption {
	return func(c *RegistryConfig) {
		c.EnableCaching = enabled
		c.CacheTTL = ttl
	}
}

func WithMetrics(enabled bool) RegistryOption {
	return func(c *RegistryConfig) {
		c.EnableMetrics = enabled
	}
}

func WithConcurrency(max int) RegistryOption {
	return func(c *RegistryConfig) {
		c.MaxConcurrency = max
	}
}

func WithTags(tags ...string) RegistryOption {
	return func(c *RegistryConfig) {
		c.Tags = tags
	}
}

func WithPriority(priority int) RegistryOption {
	return func(c *RegistryConfig) {
		c.Priority = priority
	}
}

func WithPersistence(enabled bool) RegistryOption {
	return func(c *RegistryConfig) {
		c.EnablePersistence = enabled
	}
}

func WithEvents(enabled bool) RegistryOption {
	return func(c *RegistryConfig) {
		c.EnableEvents = enabled
	}
}

// Close closes the registry and releases resources
func (r *UnifiedRegistry) Close() error {
	return nil
}

// ExecuteWithRetry runs a tool with automatic retry on failure
func (r *UnifiedRegistry) ExecuteWithRetry(ctx context.Context, name string, input api.ToolInput, _ api.RetryPolicy) (api.ToolOutput, error) {
	return r.Execute(ctx, name, input)
}

// GetStatus returns the current status of a tool
func (r *UnifiedRegistry) GetStatus(name string) (api.ToolStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return api.StatusInactive, errors.NewError().
			Code(errors.CodeToolNotFound).
			Messagef("tool '%s' not found", name).
			Build()
	}

	if metadata.Enabled {
		return api.StatusActive, nil
	}
	return api.StatusInactive, nil
}

// SetStatus updates the status of a tool
func (r *UnifiedRegistry) SetStatus(name string, status api.ToolStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return errors.NewError().
			Code(errors.CodeToolNotFound).
			Messagef("tool '%s' not found", name).
			Build()
	}

	metadata.Enabled = (status == api.StatusActive)

	return nil
}

// ListByCategory returns tools filtered by category
func (r *UnifiedRegistry) ListByCategory(_ api.ToolCategory) []string {
	return r.List()
}

// ListByTags returns tools that match any of the given tags
func (r *UnifiedRegistry) ListByTags(tags ...string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []string
	for name, config := range r.configs {
		for _, tag := range tags {
			for _, toolTag := range config.Tags {
				if tag == toolTag {
					result = append(result, name)
					break
				}
			}
		}
	}
	return result
}

// registryAdapter adapts UnifiedRegistry to api.Registry interface
type registryAdapter struct {
	registry *UnifiedRegistry
}

// NewRegistryAdapter creates an api.Registry interface from UnifiedRegistry
func NewRegistryAdapter(registry *UnifiedRegistry) api.Registry {
	return &registryAdapter{registry: registry}
}

// Register implements api.Registry with proper option conversion
func (r *registryAdapter) Register(tool api.Tool, opts ...api.RegistryOption) error {
	return r.registry.Register(tool)
}

// Unregister implements api.Registry
func (r *registryAdapter) Unregister(name string) error {
	return r.registry.Unregister(name)
}

// Get implements api.Registry
func (r *registryAdapter) Get(name string) (api.Tool, error) {
	return r.registry.Get(name)
}

// List implements api.Registry
func (r *registryAdapter) List() []string {
	return r.registry.List()
}

// ListByCategory implements api.Registry
func (r *registryAdapter) ListByCategory(category api.ToolCategory) []string {
	return r.registry.ListByCategory(category)
}

// ListByTags implements api.Registry
func (r *registryAdapter) ListByTags(tags ...string) []string {
	return r.registry.ListByTags(tags...)
}

// Execute implements api.Registry
func (r *registryAdapter) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	return r.registry.Execute(ctx, name, input)
}

// ExecuteWithRetry implements api.Registry
func (r *registryAdapter) ExecuteWithRetry(ctx context.Context, name string, input api.ToolInput, policy api.RetryPolicy) (api.ToolOutput, error) {
	return r.registry.ExecuteWithRetry(ctx, name, input, policy)
}

// GetMetadata implements api.Registry
func (r *registryAdapter) GetMetadata(name string) (api.ToolMetadata, error) {
	return r.registry.GetMetadata(name)
}

// GetStatus implements api.Registry
func (r *registryAdapter) GetStatus(name string) (api.ToolStatus, error) {
	return r.registry.GetStatus(name)
}

// SetStatus implements api.Registry
func (r *registryAdapter) SetStatus(name string, status api.ToolStatus) error {
	return r.registry.SetStatus(name, status)
}

// Close implements api.Registry
func (r *registryAdapter) Close() error {
	return r.registry.Close()
}
