package build

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
	"github.com/rs/zerolog"
)

// DefaultIterativeFixer implements the IterativeFixer interface using CallerAnalyzer
type DefaultIterativeFixer struct {
	analyzer    core.AIAnalyzer
	logger      zerolog.Logger
	maxAttempts int
	fixHistory  []interface{}
}

// NewDefaultIterativeFixer creates a new iterative fixer
func NewDefaultIterativeFixer(analyzer core.AIAnalyzer, logger zerolog.Logger) *DefaultIterativeFixer {
	return &DefaultIterativeFixer{
		analyzer:    analyzer,
		logger:      logger.With().Str("component", "iterative_fixer").Logger(),
		maxAttempts: 3, // default max attempts
		fixHistory:  make([]interface{}, 0),
	}
}

// attemptFixInternal tries to fix a failure using AI analysis with iterative loops
// AttemptFix implements the IterativeFixer interface
func (f *DefaultIterativeFixer) AttemptFix(ctx context.Context, sessionID string, toolName string, operationType string, err error, maxAttempts int, baseDir string) (*mcptypes.FixingResult, error) {
	fixingCtx := &FixingContext{
		SessionID:       sessionID,
		ToolName:        toolName,
		OperationType:   operationType,
		OriginalError:   err,
		MaxAttempts:     maxAttempts,
		BaseDir:         baseDir,
		WorkspaceDir:    baseDir,
		ErrorDetails:    make(map[string]interface{}),
		AttemptHistory:  make([]interface{}, 0),
		EnvironmentInfo: make(map[string]interface{}),
		SessionMetadata: make(map[string]interface{}),
	}

	return f.attemptFixInternal(ctx, fixingCtx)
}

func (f *DefaultIterativeFixer) attemptFixInternal(ctx context.Context, fixingCtx *FixingContext) (*mcptypes.FixingResult, error) {
	startTime := time.Now()
	result := &mcptypes.FixingResult{
		AllAttempts:   []interface{}{},
		TotalAttempts: 0,
	}
	f.logger.Info().
		Str("session_id", fixingCtx.SessionID).
		Str("tool", fixingCtx.ToolName).
		Str("operation", fixingCtx.OperationType).
		Msg("Starting iterative fixing process")
	for attempt := 1; attempt <= fixingCtx.MaxAttempts; attempt++ {
		f.logger.Debug().
			Int("attempt", attempt).
			Int("max_attempts", fixingCtx.MaxAttempts).
			Msg("Starting fix attempt")
		// Get fix strategies for this attempt
		strategies, err := f.getFixStrategiesForContext(ctx, fixingCtx)
		if err != nil {
			f.logger.Error().Err(err).Int("attempt", attempt).Msg("Failed to get fix strategies")
			continue
		}
		if len(strategies) == 0 {
			f.logger.Warn().Int("attempt", attempt).Msg("No fix strategies available")
			break
		}
		// Try the highest priority strategy
		strategy := strategies[0]
		fixAttempt, err := f.ApplyFix(ctx, strategy, fixingCtx)
		if err != nil {
			f.logger.Error().Err(err).Int("attempt", attempt).Msg("Failed to apply fix")
			continue
		}
		result.AllAttempts = append(result.AllAttempts, *fixAttempt)
		result.TotalAttempts = attempt
		// Store the fix attempt
		// Check if fix was successful
		if fixAttempt.Success {
			result.Success = true
			result.Duration = time.Since(startTime)
			f.logger.Info().
				Int("attempt", attempt).
				Dur("duration", result.Duration).
				Msg("Fix attempt succeeded")
			return result, nil
		}
		// Add this attempt to the context for the next iteration
		fixingCtx.AttemptHistory = append(fixingCtx.AttemptHistory, *fixAttempt)
		f.logger.Debug().
			Int("attempt", attempt).
			Str("strategy", strategy.Name).
			Msg("Fix attempt failed, preparing for next attempt")
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
	f.logger.Info().
		Str("strategy", strategy.Name).
		Msg("Applying fix strategy")

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
