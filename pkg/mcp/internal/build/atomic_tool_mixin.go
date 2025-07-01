package build

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// AtomicToolFixingMixin provides iterative fixing capabilities to atomic tools
type AtomicToolFixingMixin struct {
	fixer  *AnalyzerIntegratedFixer
	config *EnhancedFixingConfiguration
	logger zerolog.Logger
}

// NewAtomicToolFixingMixin creates a new fixing mixin
func NewAtomicToolFixingMixin(analyzer core.AIAnalyzer, toolName string, logger zerolog.Logger) *AtomicToolFixingMixin {
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
				Str("error_type", "unknown").
				Str("severity", "medium").
				Msg("Skipping fix attempt based on error characteristics")
			break
		}
		// Attempt AI-driven fix
		m.logger.Info().
			Int("attempt", attempt).
			Str("error_type", "unknown").
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
		if len(fixResult.AllAttempts) > 0 {
			prepareErr := operation.PrepareForRetry(ctx, fixResult.AllAttempts[len(fixResult.AllAttempts)-1])
			if prepareErr != nil {
				m.logger.Error().Err(prepareErr).Msg("Failed to prepare for retry after fix")
				continue
			}
		}
		m.logger.Info().
			Int("attempt", attempt).
			Dur("fix_duration", fixResult.Duration).
			Int("attempts_made", fixResult.AttemptsUsed).
			Msg("Fix applied successfully, retrying operation")
	}
	// All attempts failed
	m.logger.Error().
		Err(lastError).
		Int("total_attempts", m.config.MaxAttempts).
		Str("session_id", sessionID).
		Msg("Operation failed after all retry attempts")

	return lastError
}

// shouldAttemptFix determines whether a fix should be attempted based on error characteristics
func (m *AtomicToolFixingMixin) shouldAttemptFix(err error) bool {
	// Simple heuristic - attempt fix for most errors except certain types
	// Add more sophisticated logic here based on error types
	return err != nil
}
