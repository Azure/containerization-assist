package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ToolRegistry provides a generic registry for typed tools
type ToolRegistry struct {
	registeredTools map[string]interface{}
	metadata        map[string]ToolMetadata
	mu              sync.RWMutex
	logger          *slog.Logger
	config          RegistryConfig
	metrics         RegistryMetrics
}

// ToolMetadata contains metadata for typed tool instances
type ToolMetadata struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	RegisteredAt time.Time `json:"registered_at"`
	LastUsed     time.Time `json:"last_used"`
	UsageCount   int64     `json:"usage_count"`
	Enabled      bool      `json:"enabled"`
	ErrorCount   int64     `json:"error_count"`
	LastError    string    `json:"last_error,omitempty"`
	ToolType     string    `json:"tool_type"`
}

// RegistryConfig provides configuration for ToolRegistry
type RegistryConfig struct {
	EnableMetrics  bool  `json:"enable_metrics"`
	MaxTools       int   `json:"max_tools"`
	ErrorThreshold int64 `json:"error_threshold"`
}

// RegistryMetrics tracks registry performance
type RegistryMetrics struct {
	TotalTools      int64                    `json:"total_tools"`
	TotalExecutions int64                    `json:"total_executions"`
	SuccessfulRuns  int64                    `json:"successful_runs"`
	FailedRuns      int64                    `json:"failed_runs"`
	LastResetTime   time.Time                `json:"last_reset_time"`
	ToolMetrics     map[string]int64         `json:"tool_metrics"`
	ErrorsByTool    map[string]int64         `json:"errors_by_tool"`
	LatencyByTool   map[string]time.Duration `json:"latency_by_tool"`
}

// NewToolRegistry creates a new registry for typed tools
func NewToolRegistry(logger *slog.Logger) *ToolRegistry {
	config := RegistryConfig{
		EnableMetrics:  true,
		MaxTools:       100,
		ErrorThreshold: 10,
	}

	return &ToolRegistry{
		registeredTools: make(map[string]interface{}),
		metadata:        make(map[string]ToolMetadata),
		logger:          logger.With("component", "tool_registry"),
		config:          config,
		metrics: RegistryMetrics{
			LastResetTime: time.Now(),
			ToolMetrics:   make(map[string]int64),
			ErrorsByTool:  make(map[string]int64),
			LatencyByTool: make(map[string]time.Duration),
		},
	}
}

// RegisterTool registers a tool with the registry
func (r *ToolRegistry) RegisterTool(name string, tool interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.registeredTools[name]; exists {
		return errors.NewError().Messagef("tool %s already registered", name).Build()
	}

	if r.config.MaxTools > 0 && len(r.registeredTools) >= r.config.MaxTools {
		return errors.NewError().Messagef("registry is full (max %d tools)", r.config.MaxTools).Build()
	}

	r.registeredTools[name] = tool

	metadata := ToolMetadata{
		Name:         name,
		Description:  name,
		Category:     "typed",
		RegisteredAt: time.Now(),
		Enabled:      true,
		ToolType:     fmt.Sprintf("%T", tool),
	}

	r.metadata[name] = metadata

	if r.config.EnableMetrics {
		r.metrics.TotalTools++
	}

	r.logger.Info("Typed tool registered successfully",
		"tool", name,
		"type", metadata.ToolType)

	return nil
}

// GetTool retrieves a tool from the registry
func (r *ToolRegistry) GetTool(name string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	toolInterface, exists := r.registeredTools[name]
	if !exists {
		return nil, errors.NewError().Messagef("tool %s not found", name).Build()
	}

	return toolInterface, nil
}

// ExecuteTool executes a tool with the given parameters
func (r *ToolRegistry) ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error) {
	start := time.Now()

	tool, err := r.GetTool(name)
	if err != nil {
		r.recordError(name, err)
		return nil, err
	}

	var result interface{}
	var execErr error

	result = tool
	execErr = nil

	duration := time.Since(start)

	r.updateMetrics(name, duration, execErr == nil)

	if execErr != nil {
		r.recordError(name, execErr)
		return nil, execErr
	}

	return result, nil
}

// ListTools returns a list of all registered tool names
func (r *ToolRegistry) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]string, 0, len(r.registeredTools))
	for name := range r.registeredTools {
		tools = append(tools, name)
	}
	return tools
}

// GetMetadata returns metadata for a specific tool
func (r *ToolRegistry) GetMetadata(name string) (ToolMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return ToolMetadata{}, errors.NewError().Messagef("tool %s not found", name).Build()
	}

	return metadata, nil
}

// updateMetrics updates tool execution metrics
func (r *ToolRegistry) updateMetrics(name string, duration time.Duration, success bool) {
	if !r.config.EnableMetrics {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.metrics.TotalExecutions++
	if success {
		r.metrics.SuccessfulRuns++
	} else {
		r.metrics.FailedRuns++
		r.metrics.ErrorsByTool[name]++
	}

	r.metrics.ToolMetrics[name]++
	r.metrics.LatencyByTool[name] = duration

	if metadata, exists := r.metadata[name]; exists {
		metadata.LastUsed = time.Now()
		metadata.UsageCount++
		if !success {
			metadata.ErrorCount++
		}
		r.metadata[name] = metadata
	}
}

// recordError records an error for a specific tool
func (r *ToolRegistry) recordError(name string, err error) {
	if !r.config.EnableMetrics {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.metrics.ErrorsByTool[name]++

	if metadata, exists := r.metadata[name]; exists {
		metadata.ErrorCount++
		metadata.LastError = err.Error()
		r.metadata[name] = metadata
	}

	r.logger.Error("Tool execution error recorded",
		"tool", name,
		"error", err)
}

// GetMetrics returns current registry metrics
func (r *ToolRegistry) GetMetrics() RegistryMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := r.metrics
	metrics.ToolMetrics = make(map[string]int64)
	metrics.ErrorsByTool = make(map[string]int64)
	metrics.LatencyByTool = make(map[string]time.Duration)

	for k, v := range r.metrics.ToolMetrics {
		metrics.ToolMetrics[k] = v
	}
	for k, v := range r.metrics.ErrorsByTool {
		metrics.ErrorsByTool[k] = v
	}
	for k, v := range r.metrics.LatencyByTool {
		metrics.LatencyByTool[k] = v
	}

	return metrics
}

// TypedToolRegistry is a type alias for backward compatibility during migration
type TypedToolRegistry = ToolRegistry

// TypedToolMetadata is a type alias for backward compatibility during migration
type TypedToolMetadata = ToolMetadata

// TypedRegistryConfig is a type alias for backward compatibility during migration
type TypedRegistryConfig = RegistryConfig

// TypedRegistryMetrics is a type alias for backward compatibility during migration
type TypedRegistryMetrics = RegistryMetrics

// NewTypedToolRegistry is a function alias for backward compatibility
var NewTypedToolRegistry = NewToolRegistry

// RegisterTool is a package-level convenience function that delegates to methods
func RegisterTool(r *ToolRegistry, name string, tool interface{}) error {
	return r.RegisterTool(name, tool)
}

// GetTool is a package-level convenience function that delegates to methods
func GetTool(r *ToolRegistry, name string) (interface{}, error) {
	return r.GetTool(name)
}

// ExecuteTool is a package-level convenience function that delegates to methods
func ExecuteTool(r *ToolRegistry, ctx context.Context, name string, params interface{}) (interface{}, error) {
	return r.ExecuteTool(ctx, name, params)
}
