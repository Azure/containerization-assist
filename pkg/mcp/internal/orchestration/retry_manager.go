package orchestration

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// RetryManager manages retry logic for workflow operations
type RetryManager struct {
	logger         zerolog.Logger
	defaultPolicy  *RetryPolicyExecution
	policyRegistry map[string]*RetryPolicyExecution
}

// NewRetryManager creates a new retry manager
func NewRetryManager(logger zerolog.Logger) *RetryManager {
	return &RetryManager{
		logger: logger.With().Str("component", "retry_manager").Logger(),
		defaultPolicy: &RetryPolicyExecution{
			MaxAttempts:  3,
			Delay:        time.Second,
			BackoffType:  "exponential",
			BackoffMode:  "fixed",
			InitialDelay: time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
		},
		policyRegistry: make(map[string]*RetryPolicyExecution),
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation interface {
	Execute(ctx context.Context) (interface{}, error)
	GetName() string
	CanRetry(err error) bool
	OnRetryAttempt(attempt int, lastError error)
}

// RetryResult represents the result of a retry operation
type RetryResult struct {
	Success      bool           `json:"success"`
	Result       interface{}    `json:"result,omitempty"`
	Error        error          `json:"error,omitempty"`
	Attempts     int            `json:"attempts"`
	TotalDelay   time.Duration  `json:"total_delay"`
	LastAttempt  time.Time      `json:"last_attempt"`
	RetryHistory []RetryAttempt `json:"retry_history"`
}

// RetryAttempt represents a single retry attempt
type RetryAttempt struct {
	AttemptNumber int           `json:"attempt_number"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration"`
	Error         string        `json:"error,omitempty"`
	Success       bool          `json:"success"`
}

// ExecuteWithRetry executes an operation with retry logic
func (rm *RetryManager) ExecuteWithRetry(ctx context.Context, operation RetryableOperation, policy *RetryPolicyExecution) (*RetryResult, error) {
	if policy == nil {
		policy = rm.defaultPolicy
	}

	result := &RetryResult{
		RetryHistory: make([]RetryAttempt, 0, policy.MaxAttempts),
	}

	var lastError error
	startTime := time.Now()

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		attemptStart := time.Now()

		rm.logger.Debug().
			Str("operation", operation.GetName()).
			Int("attempt", attempt).
			Int("max_attempts", policy.MaxAttempts).
			Msg("Executing operation")

		// Execute the operation
		operationResult, err := operation.Execute(ctx)
		attemptEnd := time.Now()
		attemptDuration := attemptEnd.Sub(attemptStart)

		// Record attempt
		attemptRecord := RetryAttempt{
			AttemptNumber: attempt,
			StartTime:     attemptStart,
			EndTime:       attemptEnd,
			Duration:      attemptDuration,
			Success:       err == nil,
		}
		if err != nil {
			attemptRecord.Error = err.Error()
		}
		result.RetryHistory = append(result.RetryHistory, attemptRecord)

		// Check if operation succeeded
		if err == nil {
			result.Success = true
			result.Result = operationResult
			result.Attempts = attempt
			result.TotalDelay = time.Since(startTime) - attemptDuration
			result.LastAttempt = attemptEnd

			rm.logger.Info().
				Str("operation", operation.GetName()).
				Int("attempts", attempt).
				Dur("total_duration", time.Since(startTime)).
				Msg("Operation succeeded")

			return result, nil
		}

		lastError = err

		// Check if we should retry
		if attempt == policy.MaxAttempts || !operation.CanRetry(err) {
			break
		}

		// Calculate delay before next attempt
		delay := rm.calculateDelay(policy, attempt)

		rm.logger.Warn().
			Err(err).
			Str("operation", operation.GetName()).
			Int("attempt", attempt).
			Dur("next_delay", delay).
			Msg("Operation failed, retrying")

		// Notify operation of retry
		operation.OnRetryAttempt(attempt, err)

		// Wait before retry
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
			// Continue with next attempt
		}
	}

	// All attempts failed
	result.Success = false
	result.Error = lastError
	result.Attempts = len(result.RetryHistory)
	result.TotalDelay = time.Since(startTime)
	result.LastAttempt = time.Now()

	rm.logger.Error().
		Err(lastError).
		Str("operation", operation.GetName()).
		Int("attempts", result.Attempts).
		Msg("Operation failed after all retries")

	return result, fmt.Errorf("operation %s failed after %d attempts: %w",
		operation.GetName(), result.Attempts, lastError)
}

// calculateDelay calculates the delay before the next retry attempt
func (rm *RetryManager) calculateDelay(policy *RetryPolicyExecution, attempt int) time.Duration {
	var baseDelay time.Duration

	switch policy.BackoffType {
	case "exponential":
		// Exponential backoff: delay = initialDelay * (multiplier ^ (attempt - 1))
		baseDelay = time.Duration(float64(policy.InitialDelay) * math.Pow(policy.Multiplier, float64(attempt-1)))

	case "linear":
		// Linear backoff: delay = initialDelay * attempt
		baseDelay = policy.InitialDelay * time.Duration(attempt)

	case "constant":
		// Constant delay
		baseDelay = policy.Delay

	default:
		// Default to exponential
		baseDelay = time.Duration(float64(policy.InitialDelay) * math.Pow(policy.Multiplier, float64(attempt-1)))
	}

	// Apply jitter if configured
	if policy.BackoffMode == "jitter" {
		// Add random jitter (±25% of base delay)
		jitter := time.Duration(rand.Float64() * float64(baseDelay) * 0.5)
		if rand.Intn(2) == 0 {
			baseDelay = baseDelay + jitter
		} else {
			baseDelay = baseDelay - jitter
		}
	}

	// Ensure delay doesn't exceed max delay
	if baseDelay > policy.MaxDelay {
		baseDelay = policy.MaxDelay
	}

	return baseDelay
}

// RegisterPolicy registers a custom retry policy for a specific operation
func (rm *RetryManager) RegisterPolicy(operationName string, policy *RetryPolicyExecution) {
	rm.policyRegistry[operationName] = policy

	rm.logger.Info().
		Str("operation", operationName).
		Int("max_attempts", policy.MaxAttempts).
		Str("backoff_type", policy.BackoffType).
		Msg("Registered custom retry policy")
}

// GetPolicy retrieves the retry policy for an operation
func (rm *RetryManager) GetPolicy(operationName string) *RetryPolicyExecution {
	if policy, exists := rm.policyRegistry[operationName]; exists {
		return policy
	}
	return rm.defaultPolicy
}

// WorkflowRetryableOperation wraps a workflow stage execution as a retryable operation
type WorkflowRetryableOperation struct {
	orchestrator *WorkflowOrchestrator
	stage        *ExecutionStage
	session      *ExecutionSession
	options      ExecutionOption
	logger       zerolog.Logger
}

// NewWorkflowRetryableOperation creates a new workflow retryable operation
func NewWorkflowRetryableOperation(
	orchestrator *WorkflowOrchestrator,
	stage *ExecutionStage,
	session *ExecutionSession,
	options ExecutionOption,
	logger zerolog.Logger,
) *WorkflowRetryableOperation {
	return &WorkflowRetryableOperation{
		orchestrator: orchestrator,
		stage:        stage,
		session:      session,
		options:      options,
		logger:       logger,
	}
}

// Execute implements RetryableOperation
func (w *WorkflowRetryableOperation) Execute(ctx context.Context) (interface{}, error) {
	return w.orchestrator.executeStage(ctx, w.stage, w.session, w.options)
}

// GetName implements RetryableOperation
func (w *WorkflowRetryableOperation) GetName() string {
	return fmt.Sprintf("stage_%s", w.stage.ID)
}

// CanRetry implements RetryableOperation
func (w *WorkflowRetryableOperation) CanRetry(err error) bool {
	// Check if error is retryable
	if err == nil {
		return false
	}

	// Check for specific non-retryable errors
	errorStr := err.Error()
	nonRetryableErrors := []string{
		"validation failed",
		"unauthorized",
		"forbidden",
		"not found",
		"invalid configuration",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if contains(errorStr, nonRetryable) {
			return false
		}
	}

	// Check stage-specific retry policy
	if w.stage.RetryPolicy != nil && w.stage.RetryPolicy.MaxAttempts == 0 {
		return false
	}

	return true
}

// OnRetryAttempt implements RetryableOperation
func (w *WorkflowRetryableOperation) OnRetryAttempt(attempt int, lastError error) {
	// Update session with retry information
	w.session.LastActivity = time.Now()
	if w.session.ErrorContext == nil {
		w.session.ErrorContext = make(map[string]interface{})
	}

	retryKey := fmt.Sprintf("stage_%s_retry_%d", w.stage.ID, attempt)
	w.session.ErrorContext[retryKey] = map[string]interface{}{
		"attempt":   attempt,
		"error":     lastError.Error(),
		"timestamp": time.Now(),
	}

	w.logger.Info().
		Str("stage_id", w.stage.ID).
		Str("stage_name", w.stage.Name).
		Int("retry_attempt", attempt).
		Err(lastError).
		Msg("Stage execution retry attempt")
}

// ToolRetryableOperation wraps a tool execution as a retryable operation
type ToolRetryableOperation struct {
	orchestrator *WorkflowOrchestrator
	toolName     string
	stage        *ExecutionStage
	session      *ExecutionSession
	logger       zerolog.Logger
}

// NewToolRetryableOperation creates a new tool retryable operation
func NewToolRetryableOperation(
	orchestrator *WorkflowOrchestrator,
	toolName string,
	stage *ExecutionStage,
	session *ExecutionSession,
	logger zerolog.Logger,
) *ToolRetryableOperation {
	return &ToolRetryableOperation{
		orchestrator: orchestrator,
		toolName:     toolName,
		stage:        stage,
		session:      session,
		logger:       logger,
	}
}

// Execute implements RetryableOperation
func (t *ToolRetryableOperation) Execute(ctx context.Context) (interface{}, error) {
	return t.orchestrator.executeTool(ctx, t.toolName, t.stage, t.session)
}

// GetName implements RetryableOperation
func (t *ToolRetryableOperation) GetName() string {
	return fmt.Sprintf("tool_%s_stage_%s", t.toolName, t.stage.ID)
}

// CanRetry implements RetryableOperation
func (t *ToolRetryableOperation) CanRetry(err error) bool {
	if err == nil {
		return false
	}

	// Tool-specific retry logic
	errorStr := err.Error()

	// Always retry timeout errors
	if contains(errorStr, "timeout") || contains(errorStr, "deadline exceeded") {
		return true
	}

	// Always retry connection errors
	if contains(errorStr, "connection") || contains(errorStr, "network") {
		return true
	}

	// Don't retry validation errors
	if contains(errorStr, "validation") || contains(errorStr, "invalid") {
		return false
	}

	return true
}

// OnRetryAttempt implements RetryableOperation
func (t *ToolRetryableOperation) OnRetryAttempt(attempt int, lastError error) {
	t.logger.Debug().
		Str("tool_name", t.toolName).
		Str("stage_id", t.stage.ID).
		Int("retry_attempt", attempt).
		Err(lastError).
		Msg("Tool execution retry attempt")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	str = strings.ToLower(str)
	substr = strings.ToLower(substr)
	return strings.Contains(str, substr)
}

// CreateRetryPolicy creates a retry policy from configuration
func CreateRetryPolicy(config map[string]interface{}) *RetryPolicyExecution {
	policy := &RetryPolicyExecution{
		MaxAttempts:  3,
		Delay:        time.Second,
		BackoffType:  "exponential",
		BackoffMode:  "fixed",
		InitialDelay: time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}

	if maxAttempts, ok := config["max_attempts"].(int); ok {
		policy.MaxAttempts = maxAttempts
	}

	if delayMs, ok := config["delay_ms"].(int); ok {
		policy.Delay = time.Duration(delayMs) * time.Millisecond
	}

	if backoffType, ok := config["backoff_type"].(string); ok {
		policy.BackoffType = backoffType
	}

	if backoffMode, ok := config["backoff_mode"].(string); ok {
		policy.BackoffMode = backoffMode
	}

	if initialDelayMs, ok := config["initial_delay_ms"].(int); ok {
		policy.InitialDelay = time.Duration(initialDelayMs) * time.Millisecond
	}

	if maxDelayMs, ok := config["max_delay_ms"].(int); ok {
		policy.MaxDelay = time.Duration(maxDelayMs) * time.Millisecond
	}

	if multiplier, ok := config["multiplier"].(float64); ok {
		policy.Multiplier = multiplier
	}

	return policy
}
