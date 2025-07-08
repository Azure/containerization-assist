package build

import (
	"context"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
)

// AtomicToolFixingMixin provides iterative fixing capabilities to atomic tools
type AtomicToolFixingMixin struct {
	fixer  *AnalyzerIntegratedFixer
	config *EnhancedFixingConfiguration
	logger *slog.Logger
}

// NewAtomicToolFixingMixin creates a new fixing mixin
func NewAtomicToolFixingMixin(analyzer core.AIAnalyzer, toolName string, logger *slog.Logger) *AtomicToolFixingMixin {
	return &AtomicToolFixingMixin{
		fixer:  NewAnalyzerIntegratedFixer(analyzer, logger),
		config: GetEnhancedConfiguration(toolName),
		logger: logger.With("component", "atomic_tool_fixing_mixin", "tool", toolName),
	}
}

// ExecuteWithRetry executes an operation with AI-driven retry logic
func (m *AtomicToolFixingMixin) ExecuteWithRetry(ctx context.Context, sessionID string, baseDir string, operation mcptypes.ConsolidatedFixableOperation) error {
	m.logger.Info("Starting operation with AI-driven retry",
		"session_id", sessionID,
		"tool", m.config.ToolName,
		"max_attempts", m.config.MaxAttempts)
	var lastError error
	for attempt := 1; attempt <= m.config.MaxAttempts; attempt++ {
		m.logger.Debug("Attempting operation",
			"attempt", attempt,
			"max_attempts", m.config.MaxAttempts)
		// Try the operation
		err := operation.ExecuteOnce(ctx)
		if err == nil {
			m.logger.Info("Operation succeeded",
				"attempt", attempt,
				"session_id", sessionID)
			return nil
		}
		lastError = err
		m.logger.Warn("Operation failed",
			"error", err,
			"attempt", attempt)
		// Don't attempt fixing on the last attempt
		if attempt >= m.config.MaxAttempts {
			break
		}
		// Get failure analysis
		richError, analysisErr := operation.GetFailureAnalysis(ctx, err)
		if analysisErr != nil {
			m.logger.Error("Failed to analyze failure", "error", analysisErr)
			continue
		}
		// Check if we should attempt fixing based on error severity
		if !m.shouldAttemptFix(richError) {
			m.logger.Info("Skipping fix attempt based on error characteristics",
				"error_type", "unknown",
				"severity", "medium")
			break
		}
		// Attempt AI-driven fix
		m.logger.Info("Attempting AI-driven fix",
			"attempt", attempt,
			"error_type", "unknown")
		fixResult, fixErr := m.fixer.FixWithAnalyzer(ctx, types.FixRequest{
			SessionID:     sessionID,
			ToolName:      m.config.ToolName,
			OperationType: "operation", // operation type would be more specific in real implementation
			Error:         richError,
			MaxAttempts:   1, // Single fix attempt per operation retry
			BaseDir:       baseDir,
		})
		if fixErr != nil {
			m.logger.Error("Fix attempt failed", "error", fixErr, "attempt", attempt)
			continue
		}
		if !fixResult.Success {
			m.logger.Warn("Fix was not successful",
				"attempt", attempt,
				"fix_attempts", fixResult.TotalAttempts)
			continue
		}
		// Apply the fix to prepare for retry
		if len(fixResult.AllAttempts) > 0 {
			prepareErr := operation.PrepareForRetry(ctx, fixResult.AllAttempts[len(fixResult.AllAttempts)-1])
			if prepareErr != nil {
				m.logger.Error("Failed to prepare for retry after fix", "error", prepareErr)
				continue
			}
		}
		m.logger.Info("Fix applied successfully, retrying operation",
			"attempt", attempt,
			"fix_duration", fixResult.Duration,
			"attempts_made", fixResult.AttemptsUsed)
	}
	// All attempts failed
	m.logger.Error("Operation failed after all retry attempts",
		"error", lastError,
		"total_attempts", m.config.MaxAttempts,
		"session_id", sessionID)

	return lastError
}

// shouldAttemptFix determines whether a fix should be attempted based on error characteristics
func (m *AtomicToolFixingMixin) shouldAttemptFix(err error) bool {
	// Simple heuristic - attempt fix for most errors except certain types
	// Add more sophisticated logic here based on error types
	return err != nil
}
