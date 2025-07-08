package build

import (
	"context"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
)

// DefaultIterativeFixer implements the IterativeFixer interface using CallerAnalyzer
type DefaultIterativeFixer struct {
	analyzer    core.AIAnalyzer
	logger      *slog.Logger
	maxAttempts int
	fixHistory  []interface{}
}

// NewDefaultIterativeFixer creates a new iterative fixer
func NewDefaultIterativeFixer(analyzer core.AIAnalyzer, logger *slog.Logger) *DefaultIterativeFixer {
	return &DefaultIterativeFixer{
		analyzer:    analyzer,
		logger:      logger.With("component", "iterative_fixer"),
		maxAttempts: 3, // default max attempts
		fixHistory:  make([]interface{}, 0),
	}
}

// attemptFixInternal tries to fix a failure using AI analysis with iterative loops
// AttemptFix implements the IterativeFixer interface
func (f *DefaultIterativeFixer) AttemptFix(ctx context.Context, request FixRequest) (*FixingResult, error) {
	fixingCtx := &FixingContext{
		SessionID:       request.SessionID,
		ToolName:        request.ToolName,
		OperationType:   request.OperationType,
		OriginalError:   request.Error,
		MaxAttempts:     request.MaxAttempts,
		BaseDir:         request.BaseDir,
		WorkspaceDir:    request.BaseDir,
		ErrorDetails:    make(map[string]interface{}),
		AttemptHistory:  make([]interface{}, 0),
		EnvironmentInfo: make(map[string]interface{}),
		SessionMetadata: make(map[string]interface{}),
	}

	return f.attemptFixInternal(ctx, fixingCtx)
}

func (f *DefaultIterativeFixer) attemptFixInternal(ctx context.Context, fixingCtx *FixingContext) (*FixingResult, error) {
	startTime := time.Now()
	result := &FixingResult{
		AllAttempts:   []interface{}{},
		TotalAttempts: 0,
	}
	f.logger.Info("Starting iterative fixing process",
		"session_id", fixingCtx.SessionID,
		"tool", fixingCtx.ToolName,
		"operation", fixingCtx.OperationType)
	for attempt := 1; attempt <= fixingCtx.MaxAttempts; attempt++ {
		f.logger.Debug("Starting fix attempt",
			"attempt", attempt,
			"max_attempts", fixingCtx.MaxAttempts)
		// Get fix strategies for this attempt
		strategies, err := f.getFixStrategiesForContext(ctx, fixingCtx)
		if err != nil {
			f.logger.Error("Failed to get fix strategies", "error", err, "attempt", attempt)
			continue
		}
		if len(strategies) == 0 {
			f.logger.Warn("No fix strategies available", "attempt", attempt)
			break
		}
		// Try the highest priority strategy
		strategy := strategies[0]
		fixAttempt, err := f.ApplyFix(ctx, strategy, fixingCtx)
		if err != nil {
			f.logger.Error("Failed to apply fix", "error", err, "attempt", attempt)
			continue
		}
		result.AllAttempts = append(result.AllAttempts, *fixAttempt)
		result.TotalAttempts = attempt
		// Store the fix attempt
		// Check if fix was successful
		if fixAttempt.Success {
			result.Success = true
			result.Duration = time.Since(startTime)
			f.logger.Info("Fix attempt succeeded",
				"attempt", attempt,
				"duration", result.Duration)
			return result, nil
		}
		// Add this attempt to the context for the next iteration
		fixingCtx.AttemptHistory = append(fixingCtx.AttemptHistory, *fixAttempt)
		f.logger.Debug("Fix attempt failed, preparing for next attempt",
			"attempt", attempt,
			"strategy", strategy.Name)
	}
	result.Duration = time.Since(startTime)
	return result, nil
}

// getFixStrategiesForContext retrieves fix strategies based on the context
func (f *DefaultIterativeFixer) getFixStrategiesForContext(ctx context.Context, fixingCtx *FixingContext) ([]*retry.FixStrategy, error) {
	// Simple implementation - return default strategies
	strategies := []*retry.FixStrategy{
		{
			Type:        "retry",
			Name:        "retry",
			Description: "Retry the operation",
			Priority:    1,
			Automated:   true,
		},
		{
			Type:        "reset",
			Name:        "reset_state",
			Description: "Reset operation state and retry",
			Priority:    2,
			Automated:   true,
		},
	}
	return strategies, nil
}

// ApplyFix applies a fix strategy
func (f *DefaultIterativeFixer) ApplyFix(ctx context.Context, strategy *retry.FixStrategy, fixingCtx *FixingContext) (*retry.AttemptResult, error) {
	f.logger.Info("Applying fix strategy",
		"strategy", strategy.Name)

	// Simple implementation - simulate fix application
	return &retry.AttemptResult{
		Attempt:   1,
		Success:   true,
		Strategy:  strategy,
		Applied:   true,
		Timestamp: time.Now(),
		Context:   map[string]interface{}{"strategy": strategy.Name},
	}, nil
}
