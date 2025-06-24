package workflow

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Executor handles the execution of workflow stages
type Executor struct {
	logger              zerolog.Logger
	stageExecutor       StageExecutor
	errorRouter         ErrorRouter
	stateMachine        *StateMachine
	circuitBreakerMgr   *StageCircuitBreakerManager
	mu                  sync.Mutex
}

// NewExecutor creates a new workflow executor
func NewExecutor(
	logger zerolog.Logger,
	stageExecutor StageExecutor,
	errorRouter ErrorRouter,
	stateMachine *StateMachine,
) *Executor {
	return &Executor{
		logger:            logger.With().Str("component", "workflow_executor").Logger(),
		stageExecutor:     stageExecutor,
		errorRouter:       errorRouter,
		stateMachine:      stateMachine,
		circuitBreakerMgr: NewStageCircuitBreakerManager(logger),
	}
}

// ExecuteStage executes a single workflow stage with retry and error handling
func (e *Executor) ExecuteStage(
	ctx context.Context,
	stage *WorkflowStage,
	session *WorkflowSession,
	workflowSpec *WorkflowSpec,
) (*StageResult, error) {
	e.logger.Info().
		Str("stage_name", stage.Name).
		Str("session_id", session.ID).
		Msg("Beginning stage execution")

	// Check circuit breaker status
	stageType := stage.Type
	if stageType == "" {
		stageType = "default"
	}
	circuitBreaker := e.circuitBreakerMgr.GetCircuitBreaker(stageType)
	
	if !circuitBreaker.CanExecute(stage.Name) {
		e.logger.Warn().
			Str("stage_name", stage.Name).
			Str("stage_type", stageType).
			Str("circuit_state", string(circuitBreaker.GetState())).
			Msg("Stage execution blocked by circuit breaker")
		
		return &StageResult{
			StageName: stage.Name,
			Success:   false,
			Duration:  0,
			Error: &WorkflowError{
				ID:        fmt.Sprintf("%s-circuit-breaker", stage.Name),
				StageName: stage.Name,
				ErrorType: "circuit_breaker_open",
				Message:   "Stage execution blocked by circuit breaker",
				Severity:  "warning",
				Retryable: false,
				Timestamp: time.Now(),
			},
		}, fmt.Errorf("circuit breaker is open for stage type %s", stageType)
	}

	// Update stage status to running
	if err := e.stateMachine.UpdateStageStatus(session, stage.Name, StageStatusRunning); err != nil {
		return nil, fmt.Errorf("failed to update stage status: %w", err)
	}

	// Check conditions
	if !e.evaluateConditions(stage.Conditions, session) {
		e.logger.Info().
			Str("stage_name", stage.Name).
			Msg("Stage conditions not met, skipping")

		if err := e.stateMachine.UpdateStageStatus(session, stage.Name, StageStatusSkipped); err != nil {
			e.logger.Warn().Err(err).Msg("Failed to update skipped stage status")
		}

		return &StageResult{
			StageName: stage.Name,
			Success:   true,
			Duration:  0,
			Results:   map[string]interface{}{"skipped": true, "reason": "conditions not met"},
		}, nil
	}

	// Apply stage timeout
	stageCtx := ctx
	if stage.Timeout != nil {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, *stage.Timeout)
		defer cancel()
	} else if workflowSpec.Spec.Timeout != nil {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, *workflowSpec.Spec.Timeout)
		defer cancel()
	}

	// Determine retry policy
	retryPolicy := e.getRetryPolicy(stage, workflowSpec)

	// Execute with retry
	var result *StageResult
	var lastErr error
	attempt := 0

	for attempt <= retryPolicy.MaxAttempts {
		if attempt > 0 {
			// Calculate backoff delay
			delay := e.calculateBackoff(attempt, retryPolicy)
			e.logger.Info().
				Str("stage_name", stage.Name).
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying stage execution")

			select {
			case <-time.After(delay):
			case <-stageCtx.Done():
				return nil, fmt.Errorf("context cancelled during retry backoff: %w", stageCtx.Err())
			}
		}

		// Execute the stage
		startTime := time.Now()
		result, lastErr = e.stageExecutor.ExecuteStage(stageCtx, stage, session)
		duration := time.Since(startTime)

		if lastErr == nil && result != nil {
			// Success - record in circuit breaker
			circuitBreaker.RecordSuccess(stage.Name)
			
			result.Duration = duration
			if err := e.stateMachine.UpdateStageStatus(session, stage.Name, StageStatusCompleted); err != nil {
				e.logger.Warn().Err(err).Msg("Failed to update completed stage status")
			}
			return result, nil
		}

		// Handle error
		if lastErr != nil {
			workflowError := e.createWorkflowError(stage.Name, "", lastErr)

			// Check if error is retryable
			if !workflowError.Retryable || attempt >= retryPolicy.MaxAttempts {
				// No more retries
				if err := e.handleStageFailure(ctx, stage, session, workflowError, workflowSpec); err != nil {
					return nil, err
				}

				return &StageResult{
					StageName: stage.Name,
					Success:   false,
					Duration:  duration,
					Error:     workflowError,
				}, lastErr
			}
		}

		attempt++
	}

	// All retry attempts exhausted - record failure in circuit breaker
	workflowError := e.createWorkflowError(stage.Name, "", lastErr)
	circuitBreaker.RecordFailure(stage.Name, lastErr)
	
	if err := e.handleStageFailure(ctx, stage, session, workflowError, workflowSpec); err != nil {
		return nil, err
	}

	return &StageResult{
		StageName: stage.Name,
		Success:   false,
		Error:     workflowError,
	}, fmt.Errorf("stage %s failed after %d attempts: %w", stage.Name, attempt, lastErr)
}

// ExecuteStageGroup executes a group of stages in parallel with concurrency control
func (e *Executor) ExecuteStageGroup(
	ctx context.Context,
	stages []WorkflowStage,
	session *WorkflowSession,
	workflowSpec *WorkflowSpec,
	enableParallel bool,
) ([]StageResult, error) {
	if len(stages) == 0 {
		return []StageResult{}, nil
	}

	// Get concurrency configuration
	concurrencyConfig := e.getConcurrencyConfig(workflowSpec, session)

	// Sequential execution
	if !enableParallel || len(stages) == 1 || concurrencyConfig.MaxParallelStages == 1 {
		results := make([]StageResult, 0, len(stages))
		for _, stage := range stages {
			result, err := e.ExecuteStage(ctx, &stage, session, workflowSpec)
			if err != nil {
				return results, err
			}
			results = append(results, *result)
		}
		return results, nil
	}

	// Parallel execution with concurrency control
	e.logger.Info().
		Int("stage_count", len(stages)).
		Int("max_parallel", concurrencyConfig.MaxParallelStages).
		Str("session_id", session.ID).
		Msg("Executing stages in parallel with concurrency control")

	results := make([]StageResult, len(stages))
	resultsChan := make(chan struct {
		index  int
		result *StageResult
		err    error
	}, len(stages))

	// Create semaphore for concurrency control
	maxConcurrent := concurrencyConfig.MaxParallelStages
	if maxConcurrent <= 0 || maxConcurrent > len(stages) {
		maxConcurrent = len(stages)
	}
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	groupCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch goroutines for each stage with concurrency control
	for i, stage := range stages {
		wg.Add(1)
		go func(idx int, s WorkflowStage) {
			defer wg.Done()

			// Acquire semaphore slot
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }() // Release slot when done
			case <-groupCtx.Done():
				resultsChan <- struct {
					index  int
					result *StageResult
					err    error
				}{idx, nil, groupCtx.Err()}
				return
			}

			result, err := e.ExecuteStage(groupCtx, &s, session, workflowSpec)
			resultsChan <- struct {
				index  int
				result *StageResult
				err    error
			}{idx, result, err}

			// Cancel other stages on error if fail-fast is enabled
			if err != nil && workflowSpec.Spec.ErrorPolicy.Mode == "fail_fast" {
				cancel()
			}
		}(i, stage)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var firstErr error
	for res := range resultsChan {
		if res.result != nil {
			results[res.index] = *res.result
		}
		if res.err != nil && firstErr == nil {
			firstErr = res.err
		}
	}

	return results, firstErr
}

// Private helper methods

func (e *Executor) getConcurrencyConfig(workflowSpec *WorkflowSpec, session *WorkflowSession) *ConcurrencyConfig {
	// Check for session-level override first
	if session.ExecutionOptions != nil && session.ExecutionOptions.ConcurrencyConfig != nil {
		return session.ExecutionOptions.ConcurrencyConfig
	}

	// Use workflow-level configuration
	if workflowSpec.Spec.ConcurrencyConfig != nil {
		return workflowSpec.Spec.ConcurrencyConfig
	}

	// Default configuration
	return &ConcurrencyConfig{
		MaxParallelStages: 10, // Default to max 10 parallel stages
		StageTimeout:      5 * time.Minute,
		QueueSize:         100,
		WorkerPoolSize:    10,
	}
}

func (e *Executor) evaluateConditions(conditions []StageCondition, session *WorkflowSession) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, condition := range conditions {
		value, exists := session.SharedContext[condition.Key]

		switch condition.Operator {
		case "required", "exists":
			if !exists {
				return false
			}
		case "not_exists":
			if exists {
				return false
			}
		case "equals":
			if !exists || value != condition.Value {
				return false
			}
		case "not_equals":
			if exists && value == condition.Value {
				return false
			}
		default:
			e.logger.Warn().
				Str("operator", condition.Operator).
				Msg("Unknown condition operator")
			return false
		}
	}

	return true
}

func (e *Executor) getRetryPolicy(stage *WorkflowStage, workflowSpec *WorkflowSpec) *RetryPolicy {
	// Stage-level retry policy takes precedence
	if stage.RetryPolicy != nil {
		return stage.RetryPolicy
	}

	// Check for stage type-specific retry policy
	if stage.Type != "" && workflowSpec.Spec.StageTypeRetryPolicies != nil {
		if typePolicy, exists := workflowSpec.Spec.StageTypeRetryPolicies[stage.Type]; exists {
			return typePolicy
		}
	}

	// Fall back to workflow-level retry policy
	if workflowSpec.Spec.RetryPolicy != nil {
		return workflowSpec.Spec.RetryPolicy
	}

	// Default retry policy based on stage type
	return e.getDefaultRetryPolicyForType(stage.Type)
}

func (e *Executor) getDefaultRetryPolicyForType(stageType string) *RetryPolicy {
	switch stageType {
	case "build":
		// Build stages typically benefit from retries due to transient resource issues
		return &RetryPolicy{
			MaxAttempts:  3,
			BackoffMode:  "exponential",
			InitialDelay: 2 * time.Second,
			MaxDelay:     60 * time.Second,
			Multiplier:   2.0,
		}
	case "test":
		// Test stages may have flaky tests, limited retries
		return &RetryPolicy{
			MaxAttempts:  2,
			BackoffMode:  "fixed",
			InitialDelay: 5 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   1.0,
		}
	case "deploy":
		// Deploy stages are critical, more aggressive retries
		return &RetryPolicy{
			MaxAttempts:  5,
			BackoffMode:  "exponential",
			InitialDelay: 3 * time.Second,
			MaxDelay:     120 * time.Second,
			Multiplier:   1.5,
		}
	case "analysis", "scan":
		// Analysis stages often interact with external services
		return &RetryPolicy{
			MaxAttempts:  3,
			BackoffMode:  "linear",
			InitialDelay: 1 * time.Second,
			MaxDelay:     45 * time.Second,
			Multiplier:   1.5,
		}
	case "cleanup":
		// Cleanup stages should be retried aggressively to avoid resource leaks
		return &RetryPolicy{
			MaxAttempts:  4,
			BackoffMode:  "fixed",
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
			Multiplier:   1.0,
		}
	default:
		// Default retry policy for unspecified types
		return &RetryPolicy{
			MaxAttempts:  1, // Single retry by default
			BackoffMode:  "exponential",
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
		}
	}
}

func (e *Executor) calculateBackoff(attempt int, policy *RetryPolicy) time.Duration {
	var delay time.Duration

	switch policy.BackoffMode {
	case "fixed":
		delay = policy.InitialDelay
	case "linear":
		delay = time.Duration(attempt) * policy.InitialDelay
	case "exponential":
		multiplier := policy.Multiplier
		if multiplier <= 0 {
			multiplier = 2.0
		}
		delay = time.Duration(float64(policy.InitialDelay) * math.Pow(multiplier, float64(attempt-1)))
	default:
		delay = policy.InitialDelay
	}

	// Add jitter to prevent thundering herd
	jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
	delay += jitter

	// Apply max delay cap
	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	return delay
}

func (e *Executor) createWorkflowError(stageName, toolName string, err error) *WorkflowError {
	return &WorkflowError{
		ID:        fmt.Sprintf("%s-%d", stageName, time.Now().UnixNano()),
		StageName: stageName,
		ToolName:  toolName,
		ErrorType: "execution_error",
		Message:   err.Error(),
		Details:   map[string]interface{}{"error": err.Error()},
		Timestamp: time.Now(),
		Severity:  "error",
		Retryable: true, // Default to retryable
	}
}

func (e *Executor) handleStageFailure(
	ctx context.Context,
	stage *WorkflowStage,
	session *WorkflowSession,
	workflowError *WorkflowError,
	workflowSpec *WorkflowSpec,
) error {
	// Update stage status
	if err := e.stateMachine.UpdateStageStatus(session, stage.Name, StageStatusFailed); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to update failed stage status")
	}

	// Update error context
	if session.ErrorContext == nil {
		session.ErrorContext = &WorkflowErrorContext{
			ErrorHistory:  []WorkflowError{},
			RetryAttempts: make(map[string]int),
		}
	}
	session.ErrorContext.LastError = workflowError
	session.ErrorContext.ErrorHistory = append(session.ErrorContext.ErrorHistory, *workflowError)
	session.ErrorContext.FailedStage = stage.Name

	// Route error for potential recovery
	if e.errorRouter != nil {
		action, err := e.errorRouter.RouteError(ctx, workflowError, session)
		if err == nil && action != nil {
			switch action.Action {
			case "redirect":
				e.logger.Info().
					Str("stage_name", stage.Name).
					Str("redirect_to", action.RedirectTo).
					Msg("Redirecting stage execution")
				// Store redirect information in session context
				if session.SharedContext == nil {
					session.SharedContext = make(map[string]interface{})
				}
				session.SharedContext["_redirect_from_"+stage.Name] = action.RedirectTo
				session.SharedContext["_redirect_reason_"+stage.Name] = workflowError.Message
			case "skip":
				e.logger.Info().
					Str("stage_name", stage.Name).
					Msg("Skipping failed stage")
				return e.stateMachine.UpdateStageStatus(session, stage.Name, StageStatusSkipped)
			}
		}
	}

	// Handle stage-level failure action
	if stage.OnFailure != nil {
		switch stage.OnFailure.Action {
		case "skip":
			return e.stateMachine.UpdateStageStatus(session, stage.Name, StageStatusSkipped)
		case "redirect":
			if stage.OnFailure.RedirectTo != "" {
				// Store redirect information in session context
				if session.SharedContext == nil {
					session.SharedContext = make(map[string]interface{})
				}
				session.SharedContext["_redirect_from_"+stage.Name] = stage.OnFailure.RedirectTo
				session.SharedContext["_redirect_reason_"+stage.Name] = workflowError.Message
				e.logger.Info().
					Str("stage_name", stage.Name).
					Str("redirect_to", stage.OnFailure.RedirectTo).
					Msg("Stage redirection configured")
			}
		}
	}

	return nil
}

// GetCircuitBreakerStats returns circuit breaker statistics for all stage types
func (e *Executor) GetCircuitBreakerStats() map[string]CircuitBreakerStats {
	return e.circuitBreakerMgr.GetAllStats()
}

// ResetCircuitBreakers resets all circuit breakers
func (e *Executor) ResetCircuitBreakers() {
	e.circuitBreakerMgr.ResetAll()
}
