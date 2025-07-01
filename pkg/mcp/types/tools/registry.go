package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Registry is the generic interface for tool registries
type Registry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	// Register adds a tool to the registry
	Register(name string, tool T) error

	// Unregister removes a tool from the registry
	Unregister(name string) error

	// Get retrieves a tool by name
	Get(name string) (T, bool)

	// Execute runs a tool with the given parameters
	Execute(ctx context.Context, name string, params TParams) (TResult, error)

	// List returns all registered tool names
	List() []string

	// ListTools returns all registered tools with their metadata
	ListTools() []ToolInfo[T, TParams, TResult]

	// Has checks if a tool is registered
	Has(name string) bool

	// Count returns the number of registered tools
	Count() int

	// Clear removes all tools from the registry
	Clear() error

	// GetSchema returns the schema for a specific tool
	GetSchema(name string) (Schema[TParams, TResult], error)

	// ValidateParams validates parameters for a specific tool
	ValidateParams(name string, params TParams) error
}

// ToolInfo contains metadata about a registered tool
type ToolInfo[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] struct {
	Name       string
	Tool       T
	Schema     Schema[TParams, TResult]
	Registered int64 // Unix timestamp
	LastUsed   int64 // Unix timestamp
	UsageCount int64
	Enabled    bool
	Tags       []string
	Category   string
	Priority   int
}

// ToolFactory creates tool instances
type ToolFactory[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	// Create creates a new tool instance
	Create(config interface{}) (T, error)

	// GetName returns the factory's tool name
	GetName() string

	// GetDescription returns the factory's description
	GetDescription() string

	// GetConfigSchema returns the configuration schema
	GetConfigSchema() interface{}

	// Validate validates the configuration
	Validate(config interface{}) error
}

// BatchRegistry extends Registry with batch operations
type BatchRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// ExecuteBatch executes multiple tools in batch
	ExecuteBatch(ctx context.Context, requests []BatchRequest[TParams]) ([]BatchResponse[TResult], error)

	// RegisterBatch registers multiple tools at once
	RegisterBatch(tools map[string]T) error

	// UnregisterBatch removes multiple tools at once
	UnregisterBatch(names []string) error
}

// BatchRequest represents a single request in a batch
type BatchRequest[TParams ToolParams] struct {
	ID       string
	ToolName string
	Params   TParams
	Priority int
}

// BatchResponse represents a single response in a batch
type BatchResponse[TResult ToolResult] struct {
	ID       string
	Result   TResult
	Error    error
	Duration int64 // milliseconds
}

// CachingRegistry extends Registry with caching capabilities
type CachingRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// ExecuteWithCache executes a tool with caching
	ExecuteWithCache(ctx context.Context, name string, params TParams, ttl int64) (TResult, error)

	// InvalidateCache invalidates cache for a specific tool
	InvalidateCache(name string) error

	// ClearCache clears all cached results
	ClearCache() error

	// GetCacheStats returns cache statistics
	GetCacheStats() CacheStats
}

// CacheStats contains cache performance metrics
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int64
	MaxSize   int64
	HitRate   float64
	MissRate  float64
}

// FilterableRegistry extends Registry with filtering capabilities
type FilterableRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// Filter returns tools matching the given filter
	Filter(filter ToolFilter) []ToolInfo[T, TParams, TResult]

	// FindByCategory returns tools in a specific category
	FindByCategory(category string) []ToolInfo[T, TParams, TResult]

	// FindByTag returns tools with a specific tag
	FindByTag(tag string) []ToolInfo[T, TParams, TResult]

	// Search performs a text search across tool names and descriptions
	Search(query string) []ToolInfo[T, TParams, TResult]
}

// ToolFilter defines criteria for filtering tools
type ToolFilter struct {
	Categories  []string
	Tags        []string
	Enabled     *bool
	MinPriority *int
	MaxPriority *int
	NamePattern string
}

// ObservableRegistry extends Registry with observability features
type ObservableRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// GetMetrics returns registry-wide metrics
	GetMetrics() RegistryMetrics

	// GetToolMetrics returns metrics for a specific tool
	GetToolMetrics(name string) (ToolMetrics, error)

	// ResetMetrics resets all metrics
	ResetMetrics()

	// SetMetricsEnabled enables or disables metrics collection
	SetMetricsEnabled(enabled bool)
}

// RegistryMetrics contains registry-wide performance metrics
type RegistryMetrics struct {
	TotalTools      int64
	TotalExecutions int64
	SuccessfulRuns  int64
	FailedRuns      int64
	AverageLatency  float64 // milliseconds
	P95Latency      float64 // milliseconds
	P99Latency      float64 // milliseconds
	MostUsedTool    string
	LeastUsedTool   string
	ErrorRate       float64
	ThroughputRPS   float64 // requests per second
}

// ConcurrentRegistry provides thread-safe registry operations
type ConcurrentRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// ExecuteConcurrent executes multiple tools concurrently
	ExecuteConcurrent(ctx context.Context, requests []ConcurrentRequest[TParams]) <-chan ConcurrentResponse[TResult]

	// SetMaxConcurrency sets the maximum number of concurrent executions
	SetMaxConcurrency(max int)

	// GetMaxConcurrency returns the current concurrency limit
	GetMaxConcurrency() int
}

// ConcurrentRequest represents a concurrent execution request
type ConcurrentRequest[TParams ToolParams] struct {
	ID       string
	ToolName string
	Params   TParams
	Timeout  int64 // milliseconds
}

// ConcurrentResponse represents a concurrent execution response
type ConcurrentResponse[TResult ToolResult] struct {
	ID       string
	Result   TResult
	Error    error
	Duration int64 // milliseconds
	Started  int64 // Unix timestamp
	Finished int64 // Unix timestamp
}

// PersistentRegistry extends Registry with persistence capabilities
type PersistentRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// Save persists the registry state
	Save() error

	// Load restores the registry state
	Load() error

	// AutoSave enables automatic saving
	AutoSave(enabled bool, interval int64) // interval in seconds

	// GetLastSaved returns the timestamp of the last save
	GetLastSaved() int64
}

// HierarchicalRegistry supports nested registries
type HierarchicalRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// AddChild adds a child registry
	AddChild(name string, child Registry[T, TParams, TResult]) error

	// RemoveChild removes a child registry
	RemoveChild(name string) error

	// GetChild returns a child registry
	GetChild(name string) (Registry[T, TParams, TResult], bool)

	// ListChildren returns all child registry names
	ListChildren() []string

	// ExecuteInChild executes a tool in a specific child registry
	ExecuteInChild(ctx context.Context, childName, toolName string, params TParams) (TResult, error)
}

// EventRegistry supports event-driven operations
type EventRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] interface {
	Registry[T, TParams, TResult]

	// Subscribe subscribes to registry events
	Subscribe(eventType EventType, handler EventHandler[T, TParams, TResult]) error

	// Unsubscribe removes an event handler
	Unsubscribe(eventType EventType, handler EventHandler[T, TParams, TResult]) error

	// Publish publishes a custom event
	Publish(event Event[T, TParams, TResult]) error
}

// EventType represents the type of registry event
type EventType string

const (
	EventToolRegistered   EventType = "tool_registered"
	EventToolUnregistered EventType = "tool_unregistered"
	EventToolExecuted     EventType = "tool_executed"
	EventToolFailed       EventType = "tool_failed"
	EventRegistryCleared  EventType = "registry_cleared"
)

// Event represents a registry event
type Event[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] struct {
	Type      EventType
	ToolName  string
	Tool      T
	Params    TParams
	Result    TResult
	Error     error
	Timestamp int64
	Context   map[string]interface{}
}

// EventHandler handles registry events
type EventHandler[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] func(event Event[T, TParams, TResult])

// BaseRegistry provides a basic implementation foundation
type BaseRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult] struct {
	tools     map[string]T
	schemas   map[string]Schema[TParams, TResult]
	metadata  map[string]ToolInfo[T, TParams, TResult]
	mu        sync.RWMutex
	enabled   bool
	observers []EventHandler[T, TParams, TResult]
}

// NewBaseRegistry creates a new base registry
func NewBaseRegistry[T Tool[TParams, TResult], TParams ToolParams, TResult ToolResult]() *BaseRegistry[T, TParams, TResult] {
	return &BaseRegistry[T, TParams, TResult]{
		tools:    make(map[string]T),
		schemas:  make(map[string]Schema[TParams, TResult]),
		metadata: make(map[string]ToolInfo[T, TParams, TResult]),
		enabled:  true,
	}
}

// Register implements Registry.Register
func (r *BaseRegistry[T, TParams, TResult]) Register(name string, tool T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}

	r.tools[name] = tool
	r.schemas[name] = tool.GetSchema()
	r.metadata[name] = ToolInfo[T, TParams, TResult]{
		Name:       name,
		Tool:       tool,
		Schema:     tool.GetSchema(),
		Registered: time.Now().Unix(),
		Enabled:    true,
	}

	return nil
}

// Get implements Registry.Get
func (r *BaseRegistry[T, TParams, TResult]) Get(name string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// Execute implements Registry.Execute
func (r *BaseRegistry[T, TParams, TResult]) Execute(ctx context.Context, name string, params TParams) (TResult, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()

	var zero TResult
	if !exists {
		return zero, fmt.Errorf("tool not found: %s", name)
	}

	// Update metadata
	r.mu.Lock()
	if info, ok := r.metadata[name]; ok {
		info.LastUsed = time.Now().Unix()
		info.UsageCount++
		r.metadata[name] = info
	}
	r.mu.Unlock()

	return tool.Execute(ctx, params)
}

// List implements Registry.List
func (r *BaseRegistry[T, TParams, TResult]) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Has implements Registry.Has
func (r *BaseRegistry[T, TParams, TResult]) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// Count implements Registry.Count
func (r *BaseRegistry[T, TParams, TResult]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}

// Unregister implements Registry.Unregister
func (r *BaseRegistry[T, TParams, TResult]) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool not found: %s", name)
	}

	delete(r.tools, name)
	delete(r.schemas, name)
	delete(r.metadata, name)

	return nil
}

// ListTools implements Registry.ListTools
func (r *BaseRegistry[T, TParams, TResult]) ListTools() []ToolInfo[T, TParams, TResult] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolInfo[T, TParams, TResult], 0, len(r.metadata))
	for _, info := range r.metadata {
		tools = append(tools, info)
	}
	return tools
}

// Clear implements Registry.Clear
func (r *BaseRegistry[T, TParams, TResult]) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = make(map[string]T)
	r.schemas = make(map[string]Schema[TParams, TResult])
	r.metadata = make(map[string]ToolInfo[T, TParams, TResult])

	return nil
}

// GetSchema implements Registry.GetSchema
func (r *BaseRegistry[T, TParams, TResult]) GetSchema(name string) (Schema[TParams, TResult], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var zeroSchema Schema[TParams, TResult]
	schema, exists := r.schemas[name]
	if !exists {
		return zeroSchema, fmt.Errorf("tool not found: %s", name)
	}

	return schema, nil
}

// ValidateParams implements Registry.ValidateParams
func (r *BaseRegistry[T, TParams, TResult]) ValidateParams(name string, params TParams) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool not found: %s", name)
	}

	// Use tool's schema validation if available
	schema := tool.GetSchema()
	if validator, ok := interface{}(schema).(interface {
		ValidateParams(TParams) error
	}); ok {
		return validator.ValidateParams(params)
	}

	// Fall back to basic parameter validation
	return params.Validate()
}
