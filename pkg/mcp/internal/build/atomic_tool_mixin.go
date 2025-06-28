package build

import (
	"context"
	"fmt"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// AtomicToolFixingMixin provides iterative fixing capabilities to atomic tools
type AtomicToolFixingMixin struct {
	fixer  *AnalyzerIntegratedFixer
	config *EnhancedFixingConfiguration
	logger zerolog.Logger
}

// NewAtomicToolFixingMixin creates a new fixing mixin
func NewAtomicToolFixingMixin(analyzer mcptypes.AIAnalyzer, toolName string, logger zerolog.Logger) *AtomicToolFixingMixin {
	return &AtomicToolFixingMixin{
		fixer:  NewAnalyzerIntegratedFixer(analyzer, logger),
		config: GetEnhancedConfiguration(toolName),
		logger: logger.With().Str("component", "atomic_tool_fixing_mixin").Str("tool", toolName).Logger(),
	}
}

// ExecuteWithRetry executes an operation with AI-driven retry logic
func (m *AtomicToolFixingMixin) ExecuteWithRetry(ctx context.Context, sessionID string, baseDir string, operation mcptypes.FixableOperation) error {
	m.logger.Info().
		Str("session_id", sessionID).
		Str("tool", m.config.ToolName).
		Int("max_attempts", m.config.MaxAttempts).
		Msg("Starting operation with AI-driven retry")
	var lastError error
	for attempt := 1; attempt <= m.config.MaxAttempts; attempt++ {
		m.logger.Debug().
			Int("attempt", attempt).
			Int("max_attempts", m.config.MaxAttempts).
			Msg("Attempting operation")
		// Try the operation
		err := operation.ExecuteOnce(ctx)
		if err == nil {
			m.logger.Info().
				Int("attempt", attempt).
				Str("session_id", sessionID).
				Msg("Operation succeeded")
			return nil
		}
		lastError = err
		m.logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Msg("Operation failed")
		// Don't attempt fixing on the last attempt
		if attempt >= m.config.MaxAttempts {
			break
		}
		// Get failure analysis
		richError, analysisErr := operation.GetFailureAnalysis(ctx, err)
		if analysisErr != nil {
			m.logger.Error().Err(analysisErr).Msg("Failed to analyze failure")
			continue
		}
		// Check if we should attempt fixing based on error severity
		if !m.shouldAttemptFix(richError) {
			m.logger.Info().
				Str("error_type", richError.Type).
				Str("severity", richError.Severity).
				Msg("Skipping fix attempt based on error characteristics")
			break
		}
		// Attempt AI-driven fix
		m.logger.Info().
			Int("attempt", attempt).
			Str("error_type", richError.Type).
			Msg("Attempting AI-driven fix")
		fixResult, fixErr := m.fixer.FixWithAnalyzer(
			ctx,
			sessionID,
			m.config.ToolName,
			"operation", // operation type would be more specific in real implementation
			richError,
			1, // Single fix attempt per operation retry
			baseDir,
		)
		if fixErr != nil {
			m.logger.Error().Err(fixErr).Int("attempt", attempt).Msg("Fix attempt failed")
			continue
		}
		if !fixResult.Success {
			m.logger.Warn().
				Int("attempt", attempt).
				Int("fix_attempts", fixResult.TotalAttempts).
				Msg("Fix was not successful")
			continue
		}
		// Apply the fix to prepare for retry
		if fixResult.FinalAttempt != nil {
			prepareErr := operation.PrepareForRetry(ctx, fixResult.FinalAttempt)
			if prepareErr != nil {
				m.logger.Error().Err(prepareErr).Msg("Failed to prepare for retry after fix")
				continue
			}
		}
		m.logger.Info().
			Int("attempt", attempt).
			Dur("fix_duration", fixResult.TotalDuration).
			Str("fix_strategy", fixResult.FinalAttempt.FixStrategy.Name).
			Msg("Fix applied successfully, retrying operation")
	}
	// All attempts failed
	m.logger.Error().
		Err(lastError).
		Int("total_attempts", m.config.MaxAttempts).
		Str("session_id", sessionID).
		Msg("Operation failed after all retry attempts")
	return fmt.Errorf("operation failed after %d attempts, last error: %w", m.config.MaxAttempts, lastError)
}

// GetRecommendations provides fixing recommendations without executing fixes
func (m *AtomicToolFixingMixin) GetRecommendations(ctx context.Context, sessionID string, err error, baseDir string) ([]mcptypes.FixStrategy, error) {
	return m.fixer.GetFixingRecommendations(ctx, sessionID, m.config.ToolName, err, baseDir)
}

// AnalyzeError provides enhanced error analysis
func (m *AtomicToolFixingMixin) AnalyzeError(ctx context.Context, sessionID string, err error, baseDir string) (string, error) {
	return m.fixer.AnalyzeErrorWithContext(ctx, sessionID, err, baseDir)
}

// shouldAttemptFix determines if fixing should be attempted based on error characteristics
func (m *AtomicToolFixingMixin) shouldAttemptFix(richError *mcptypes.RichError) bool {
	// Don't attempt fixing for certain error types
	nonFixableTypes := []string{
		"permission_denied",
		"authentication_failed",
		"quota_exceeded",
		"resource_not_found",
	}
	for _, nonFixable := range nonFixableTypes {
		if richError.Type == nonFixable {
			return false
		}
	}
	// Check severity threshold
	severityLevels := map[string]int{
		"Critical": 4,
		"High":     3,
		"Medium":   2,
		"Low":      1,
	}
	errorLevel := severityLevels[richError.Severity]
	thresholdLevel := severityLevels[m.config.SeverityThreshold]
	return errorLevel >= thresholdLevel
}

// BuildOperationWrapper wraps build operations with fixing capabilities
type BuildOperationWrapper struct {
	originalOperation func(ctx context.Context) error
	failureAnalyzer   func(ctx context.Context, err error) (*mcptypes.RichError, error)
	retryPreparer     func(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error
	logger            zerolog.Logger
}

// NewBuildOperationWrapper creates a wrapper for build operations
func NewBuildOperationWrapper(
	operation func(ctx context.Context) error,
	analyzer func(ctx context.Context, err error) (*mcptypes.RichError, error),
	preparer func(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error,
	logger zerolog.Logger,
) *BuildOperationWrapper {
	return &BuildOperationWrapper{
		originalOperation: operation,
		failureAnalyzer:   analyzer,
		retryPreparer:     preparer,
		logger:            logger,
	}
}

// ExecuteOnce implements mcptypes.FixableOperation
func (w *BuildOperationWrapper) ExecuteOnce(ctx context.Context) error {
	return w.originalOperation(ctx)
}

// GetFailureAnalysis implements mcptypes.FixableOperation
func (w *BuildOperationWrapper) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.RichError, error) {
	if w.failureAnalyzer != nil {
		return w.failureAnalyzer(ctx, err)
	}
	// Default analysis
	return &mcptypes.RichError{
		Code:     "OPERATION_FAILED",
		Type:     "build_error",
		Severity: "High",
		Message:  err.Error(),
	}, nil
}

// PrepareForRetry implements mcptypes.FixableOperation
func (w *BuildOperationWrapper) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if w.retryPreparer != nil {
		return w.retryPreparer(ctx, fixAttempt)
	}
	w.logger.Debug().Msg("No retry preparation needed")
	return nil
}

// Usage example pattern for integrating with existing atomic tools:
//
// func (t *AtomicBuildImageTool) ExecuteWithFixes(ctx context.Context, args AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
//     // Create fixing mixin
//     fixingMixin := fixing.NewAtomicToolFixingMixin(t.analyzer, "atomic_build_image", t.logger)
//
//     // Wrap the core operation
//     operation := fixing.NewBuildOperationWrapper(
//         func(ctx context.Context) error {
//             return t.executeCoreOperation(ctx, args)
//         },
//         func(ctx context.Context, err error) (*mcptypes.RichError, error) {
//             return t.analyzeFailure(ctx, err, args)
//         },
//         func(ctx context.Context, fixAttempt *fixing.mcptypes.FixAttempt) error {
//             return t.applyFix(ctx, fixAttempt, args)
//         },
//         t.logger,
//     )
//
//     // Execute with retry
//     err := fixingMixin.ExecuteWithRetry(ctx, args.SessionID, args.BuildContext, operation)
//     if err != nil {
//         return nil, err
//     }
//
//     return t.buildSuccessResult(ctx, args)
// }
