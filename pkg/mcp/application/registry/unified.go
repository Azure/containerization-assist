package registry

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
)

// UnifiedRegistry implements api.ToolRegistry with thread safety and minimal reflection
type UnifiedRegistry struct {
	mu       sync.RWMutex
	tools    map[string]interface{} // stores factory functions
	metadata map[string]api.ToolMetadata
	metrics  api.RegistryMetrics
	logger   logging.Logger
	closed   bool

	// Performance tracking
	totalRegistrations int64
	totalDiscoveries   int64
	totalExecutions    int64
	discoveryTimeSum   int64
	executionTimeSum   int64
}

// NewUnified creates a new unified registry instance
func NewUnified() api.ToolRegistry {
	// Create a logger using the domain logging interface
	logger := &loggerAdapter{}
	return &UnifiedRegistry{
		tools:    make(map[string]interface{}),
		metadata: make(map[string]api.ToolMetadata),
		logger:   logger,
	}
}

// loggerAdapter implements logging.Logger interface
type loggerAdapter struct{}

func (l *loggerAdapter) Debug(msg string, args ...any)                 {}
func (l *loggerAdapter) Info(msg string, args ...any)                  {}
func (l *loggerAdapter) Warn(msg string, args ...any)                  {}
func (l *loggerAdapter) Error(msg string, args ...any)                 {}
func (l *loggerAdapter) With(args ...any) logging.Logger               { return l }
func (l *loggerAdapter) WithComponent(component string) logging.Logger { return l }

// Register registers a tool factory function
func (r *UnifiedRegistry) Register(name string, factory interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Message("registry is closed").
			Build()
	}

	if name == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("tool name cannot be empty").
			Build()
	}

	if factory == nil {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("factory cannot be nil").
			Build()
	}

	// Validate factory is a function
	factoryType := reflect.TypeOf(factory)
	if factoryType.Kind() != reflect.Func {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("factory must be a function").
			Context("actual_type", factoryType.String()).
			Build()
	}

	if _, exists := r.tools[name]; exists {
		return errors.NewError().
			Code(errors.CodeResourceAlreadyExists).
			Message("tool already registered").
			Context("tool_name", name).
			Suggestion("Use a different tool name or unregister the existing tool first").
			WithLocation().
			Build()
	}

	// Store the factory function
	r.tools[name] = factory

	// Initialize metadata
	r.metadata[name] = api.ToolMetadata{
		Name:                 name,
		Description:          fmt.Sprintf("Tool %s", name),
		Category:             api.ToolCategory("general"),
		Version:              "1.0.0",
		Tags:                 []string{},
		Status:               api.ToolStatus("active"),
		Dependencies:         []string{},
		Capabilities:         []string{},
		Requirements:         []string{},
		RegisteredAt:         time.Now(),
		LastModified:         time.Now(),
		ExecutionCount:       0,
		AverageExecutionTime: 0,
	}

	r.totalRegistrations++

	r.logger.Info("tool registered",
		"tool", name,
		"factory_type", factoryType.String())

	return nil
}

// Discover finds a tool by name and returns the factory result
func (r *UnifiedRegistry) Discover(name string) (interface{}, error) {
	start := time.Now()
	defer func() {
		r.mu.Lock()
		r.totalDiscoveries++
		r.discoveryTimeSum += time.Since(start).Nanoseconds()
		r.mu.Unlock()
	}()

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, errors.NewError().
			Code(errors.CodeInvalidState).
			Message("registry is closed").
			Build()
	}

	factory, exists := r.tools[name]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeNotFound).
			Message("tool not found").
			Context("tool_name", name).
			Suggestion("Use List() to see available tools").
			Build()
	}

	// Call the factory function using reflection
	factoryValue := reflect.ValueOf(factory)
	results := factoryValue.Call(nil)

	if len(results) != 1 {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Message("factory must return exactly one value").
			Context("tool_name", name).
			Context("return_count", len(results)).
			Build()
	}

	return results[0].Interface(), nil
}

// List returns all registered tool names
func (r *UnifiedRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Metadata returns tool metadata
func (r *UnifiedRegistry) Metadata(name string) (api.ToolMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return api.ToolMetadata{}, errors.NewError().
			Code(errors.CodeInvalidState).
			Message("registry is closed").
			Build()
	}

	metadata, exists := r.metadata[name]
	if !exists {
		return api.ToolMetadata{}, errors.NewError().
			Code(errors.CodeNotFound).
			Message("tool metadata not found").
			Context("tool_name", name).
			Build()
	}

	return metadata, nil
}

// SetMetadata updates tool metadata
func (r *UnifiedRegistry) SetMetadata(name string, metadata api.ToolMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Message("registry is closed").
			Build()
	}

	if _, exists := r.tools[name]; !exists {
		return errors.NewError().
			Code(errors.CodeNotFound).
			Message("tool not found").
			Context("tool_name", name).
			Build()
	}

	// No need to preserve creation time since ToolMetadata doesn't have it

	r.metadata[name] = metadata
	return nil
}

// Unregister removes a tool from the registry
func (r *UnifiedRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Message("registry is closed").
			Build()
	}

	if _, exists := r.tools[name]; !exists {
		return errors.NewError().
			Code(errors.CodeNotFound).
			Message("tool not found").
			Context("tool_name", name).
			Build()
	}

	delete(r.tools, name)
	delete(r.metadata, name)

	r.logger.Info("tool unregistered", "tool", name)
	return nil
}

// Execute runs a tool by name with the given input
func (r *UnifiedRegistry) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	start := time.Now()
	defer func() {
		r.mu.Lock()
		r.totalExecutions++
		r.executionTimeSum += time.Since(start).Nanoseconds()
		r.mu.Unlock()
	}()

	// Discover the tool
	toolInterface, err := r.Discover(name)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Type assert to api.Tool
	tool, ok := toolInterface.(api.Tool)
	if !ok {
		return api.ToolOutput{
				Success: false,
				Error:   "tool does not implement api.Tool interface",
			}, errors.NewError().
				Code(errors.CodeTypeMismatch).
				Message("tool does not implement api.Tool interface").
				Context("tool_name", name).
				Context("actual_type", fmt.Sprintf("%T", toolInterface)).
				Build()
	}

	// Execute the tool
	output, err := tool.Execute(ctx, input)
	if err != nil {
		r.logger.Error("tool execution failed",
			"tool", name,
			"error", err)
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Update execution metrics
	r.mu.Lock()
	if meta, ok := r.metadata[name]; ok {
		meta.ExecutionCount++
		// Update average execution time
		prevAvg := int64(meta.AverageExecutionTime)
		newTime := time.Since(start)
		meta.AverageExecutionTime = time.Duration((prevAvg*(meta.ExecutionCount-1) + int64(newTime)) / meta.ExecutionCount)

		// Update last executed time
		now := time.Now()
		meta.LastExecuted = &now
		r.metadata[name] = meta
	}
	r.mu.Unlock()

	return output, nil
}

// Close releases all resources used by the registry
func (r *UnifiedRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Message("registry already closed").
			Build()
	}

	r.closed = true

	// Clear all tools and metadata
	r.tools = make(map[string]interface{})
	r.metadata = make(map[string]api.ToolMetadata)

	r.logger.Info("registry closed")
	return nil
}

// GetMetrics returns registry performance metrics
func (r *UnifiedRegistry) GetMetrics() api.RegistryMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// We don't track discovery time metrics in the current RegistryMetrics structure
	// avgDiscoveryTime := int64(0)
	// if r.totalDiscoveries > 0 {
	// 	avgDiscoveryTime = r.discoveryTimeSum / r.totalDiscoveries
	// }

	avgExecutionTime := int64(0)
	if r.totalExecutions > 0 {
		avgExecutionTime = r.executionTimeSum / r.totalExecutions
	}

	return api.RegistryMetrics{
		TotalTools:           len(r.tools),
		ActiveTools:          len(r.tools), // For now, all tools are considered active
		TotalExecutions:      r.totalExecutions,
		FailedExecutions:     0, // TODO: Track failed executions
		AverageExecutionTime: time.Duration(avgExecutionTime),
		UpTime:               time.Duration(0), // TODO: Track uptime
	}
}

// Generic helper functions for type-safe registration and discovery

// RegisterTool provides type-safe tool registration
func RegisterTool[T any](registry api.ToolRegistry, name string, factory func() T) error {
	return registry.Register(name, factory)
}

// DiscoverTool provides type-safe tool discovery
func DiscoverTool[T any](registry api.ToolRegistry, name string) (T, error) {
	var zero T

	result, err := registry.Discover(name)
	if err != nil {
		return zero, err
	}

	typed, ok := result.(T)
	if !ok {
		return zero, errors.NewError().
			Code(errors.CodeTypeMismatch).
			Message("tool type mismatch").
			Context("tool_name", name).
			Context("expected_type", fmt.Sprintf("%T", zero)).
			Context("actual_type", fmt.Sprintf("%T", result)).
			Build()
	}

	return typed, nil
}
