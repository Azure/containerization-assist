package workflow

import (
	"context"
	"errors"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

var (
	// ErrInvalidInput indicates invalid input
	ErrInvalidInput = errors.New("invalid input")
)

// ToolInputHelper provides helper methods for ToolInput
type ToolInputHelper struct {
	*api.ToolInput
}

// NewToolInputHelper creates a new helper
func NewToolInputHelper(input *api.ToolInput) *ToolInputHelper {
	return &ToolInputHelper{ToolInput: input}
}

// GetSessionID implements compatibility methods
func (h *ToolInputHelper) GetSessionID() string {
	return h.SessionID
}

// Validate implements basic validation
func (h *ToolInputHelper) Validate() error {
	if h.SessionID == "" {
		return ErrInvalidInput
	}
	return nil
}

// GetContext returns execution context for compatibility
func (h *ToolInputHelper) GetContext() map[string]interface{} {
	if h.Context == nil {
		return make(map[string]interface{})
	}
	return h.Context
}

// ToolOutputHelper provides helper methods for ToolOutput
type ToolOutputHelper struct {
	*api.ToolOutput
}

// NewToolOutputHelper creates a new helper
func NewToolOutputHelper(output *api.ToolOutput) *ToolOutputHelper {
	return &ToolOutputHelper{ToolOutput: output}
}

// IsSuccess implements compatibility methods
func (h *ToolOutputHelper) IsSuccess() bool {
	return h.Success
}

// GetData implements compatibility methods
func (h *ToolOutputHelper) GetData() interface{} {
	return h.Data
}

// GetError implements compatibility methods
func (h *ToolOutputHelper) GetError() string {
	return h.Error
}

// ============================================================================
// Registry Options - Implementation Functions
// ============================================================================

// RegistryOption provides configuration for tool registration
type RegistryOption func(*RegistryConfig)

// RegistryConfig contains configuration for tool registration
type RegistryConfig struct {
	Namespace          string
	Tags               []string
	Priority           int
	Enabled            bool
	Metadata           map[string]interface{}
	Concurrency        int
	Timeout            time.Duration
	RetryPolicy        *RetryPolicy
	CacheEnabled       bool
	CacheDuration      time.Duration
	RateLimitPerMinute int
}

// RetryPolicy defines how tools should handle retries
type RetryPolicy struct {
	MaxAttempts       int           `json:"max_attempts"`
	InitialDelay      time.Duration `json:"initial_delay"`
	MaxDelay          time.Duration `json:"max_delay"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	RetryableErrors   []string      `json:"retryable_errors,omitempty"`
}

// WithNamespace sets the namespace for the tool
func WithNamespace(namespace string) RegistryOption {
	return func(c *RegistryConfig) {
		c.Namespace = namespace
	}
}

// WithTags adds tags to the tool
func WithTags(tags ...string) RegistryOption {
	return func(c *RegistryConfig) {
		c.Tags = append(c.Tags, tags...)
	}
}

// WithPriority sets the tool priority
func WithPriority(priority int) RegistryOption {
	return func(c *RegistryConfig) {
		c.Priority = priority
	}
}

// WithEnabled sets whether the tool is enabled
func WithEnabled(enabled bool) RegistryOption {
	return func(c *RegistryConfig) {
		c.Enabled = enabled
	}
}

// WithMetadata adds metadata to the tool
func WithMetadata(key string, value interface{}) RegistryOption {
	return func(c *RegistryConfig) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]interface{})
		}
		c.Metadata[key] = value
	}
}

// WithConcurrency sets the maximum concurrent executions
func WithConcurrency(maxConcurrency int) RegistryOption {
	return func(c *RegistryConfig) {
		c.Concurrency = maxConcurrency
	}
}

// WithTimeout sets the execution timeout
func WithTimeout(timeout time.Duration) RegistryOption {
	return func(c *RegistryConfig) {
		c.Timeout = timeout
	}
}

// WithRetryPolicy sets the retry policy
func WithRetryPolicy(policy RetryPolicy) RegistryOption {
	return func(c *RegistryConfig) {
		c.RetryPolicy = &policy
	}
}

// WithCache enables caching with the specified duration
func WithCache(duration time.Duration) RegistryOption {
	return func(c *RegistryConfig) {
		c.CacheEnabled = true
		c.CacheDuration = duration
	}
}

// WithRateLimit sets the rate limit per minute
func WithRateLimit(perMinute int) RegistryOption {
	return func(c *RegistryConfig) {
		c.RateLimitPerMinute = perMinute
	}
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []string{"timeout", "network", "temporary"},
	}
}

// ============================================================================
// Validator Chain Implementation
// ============================================================================

// ChainStrategy defines how validators are executed
type ChainStrategy int

const (
	// StopOnFirstError stops chain on first validation error
	StopOnFirstError ChainStrategy = iota
	// ContinueOnError continues chain collecting all errors
	ContinueOnError
	// StopOnFirstWarning stops chain on first warning
	StopOnFirstWarning
)

// ValidatorChain allows composing multiple validators
type ValidatorChain[T any] struct {
	validators []api.Validator[T]
	strategy   ChainStrategy
}

// NewValidatorChain creates a new validator chain
func NewValidatorChain[T any](strategy ChainStrategy) *ValidatorChain[T] {
	return &ValidatorChain[T]{
		validators: make([]api.Validator[T], 0),
		strategy:   strategy,
	}
}

// Add adds a validator to the chain
func (c *ValidatorChain[T]) Add(validator api.Validator[T]) *ValidatorChain[T] {
	c.validators = append(c.validators, validator)
	return c
}

// Validate executes the validator chain
func (c *ValidatorChain[T]) Validate(ctx context.Context, value T) api.ValidationResult {
	result := api.ValidationResult{
		Valid:    true,
		Errors:   make([]api.ValidationError, 0),
		Warnings: make([]api.ValidationWarning, 0),
	}

	for _, validator := range c.validators {
		validationResult := validator.Validate(ctx, value)

		// Collect errors and warnings
		result.Errors = append(result.Errors, validationResult.Errors...)
		result.Warnings = append(result.Warnings, validationResult.Warnings...)

		// Apply strategy
		if !validationResult.Valid {
			result.Valid = false
			if c.strategy == StopOnFirstError {
				break
			}
		}

		if len(validationResult.Warnings) > 0 && c.strategy == StopOnFirstWarning {
			break
		}
	}

	return result
}

// Name returns the chain name
func (c *ValidatorChain[T]) Name() string {
	return "ValidatorChain"
}
