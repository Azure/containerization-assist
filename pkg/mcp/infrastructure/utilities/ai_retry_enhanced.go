// Package utilities provides enhanced AI-powered retry with progressive error context
package utilities

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// RetryContext provides additional context for AI-powered retries
type RetryContext struct {
	ErrorHistory   string                 // Previous error context
	StepContext    map[string]interface{} // Current step context
	FixesAttempted []string               // Previously attempted fixes
}

// WithAIRetryEnhanced performs retries with progressive error context
func WithAIRetryEnhanced(ctx context.Context, name string, max int, fn func() error, retryCtx *RetryContext, logger *slog.Logger) error {
	logger.Info("Starting operation with enhanced AI retry",
		"operation", name,
		"max_retries", max,
		"has_error_history", retryCtx != nil && retryCtx.ErrorHistory != "")

	samplingClient := sampling.NewClient(logger)

	for i := 1; i <= max; i++ {
		logger.Debug("Attempting operation", "operation", name, "attempt", i, "max", max)

		err := fn()
		if err == nil {
			logger.Info("Operation completed successfully", "operation", name, "attempt", i)
			return nil
		}

		logger.Error("Operation failed", "operation", name, "attempt", i, "max", max, "error", err)

		// If this was the last attempt, return enhanced error
		if i == max {
			logger.Error("Operation exhausted all retries", "operation", name, "attempts", max)
			return enhanceErrorWithContext(name, err, i, max, retryCtx, logger)
		}

		// Build context for AI analysis
		contextInfo := buildAIContext(name, err, i, max, retryCtx)

		// Use AI to analyze the error with full context
		analysis, analysisErr := samplingClient.AnalyzeError(ctx, err, contextInfo)
		if analysisErr != nil {
			logger.Warn("Failed to get AI analysis", "error", analysisErr)
			continue
		}

		// Log the analysis
		logger.Info("AI Error Analysis",
			"operation", name,
			"root_cause", analysis.RootCause,
			"can_auto_fix", analysis.CanAutoFix,
			"fix_steps", len(analysis.FixSteps))

		// Update retry context with attempted fixes
		if retryCtx != nil && len(analysis.FixSteps) > 0 {
			retryCtx.FixesAttempted = append(retryCtx.FixesAttempted, analysis.FixSteps...)
		}

		// Show AI guidance
		logAIGuidance(name, err, i, max, analysis, logger)

		// If we can auto-fix, attempt to apply fixes
		if analysis.CanAutoFix && len(analysis.FixSteps) > 0 {
			logger.Info("Attempting automated fixes suggested by AI", "fix_count", len(analysis.FixSteps))

			fixApplied, fixErr := applyAIFixSteps(ctx, name, analysis.FixSteps, logger)
			if fixErr != nil {
				logger.Warn("Failed to apply AI-suggested fixes", "error", fixErr)
			} else if fixApplied {
				logger.Info("AI fixes applied successfully, retrying operation")
				time.Sleep(100 * time.Millisecond)
			}
		}

		// Add backoff before next retry
		backoff := time.Duration(i) * time.Second // Simple linear backoff
		logger.Debug("Backing off before retry", "duration", backoff)
		time.Sleep(backoff)
	}

	return fmt.Errorf("operation %s failed after %d attempts", name, max)
}

// buildAIContext creates comprehensive context for AI analysis
func buildAIContext(name string, err error, attempt int, max int, retryCtx *RetryContext) string {
	context := fmt.Sprintf("Operation: %s\nAttempt: %d of %d\n", name, attempt, max)
	context += fmt.Sprintf("Current Error: %v\n", err)

	if retryCtx != nil {
		if retryCtx.ErrorHistory != "" {
			context += "\n" + retryCtx.ErrorHistory + "\n"
		}

		if len(retryCtx.StepContext) > 0 {
			context += "\nStep Context:\n"
			for k, v := range retryCtx.StepContext {
				context += fmt.Sprintf("- %s: %v\n", k, v)
			}
		}

		if len(retryCtx.FixesAttempted) > 0 {
			context += "\nPreviously Attempted Fixes:\n"
			for _, fix := range retryCtx.FixesAttempted {
				context += fmt.Sprintf("- %s\n", fix)
			}
			context += "\nPlease suggest different fixes that haven't been tried yet.\n"
		}
	}

	return context
}

// enhanceErrorWithContext adds comprehensive context to the final error
func enhanceErrorWithContext(operation string, err error, attempt int, maxAttempts int, retryCtx *RetryContext, logger *slog.Logger) error {
	errorMsg := err.Error()

	// Generate fix suggestions similar to the original generateFixSuggestions
	suggestions := []string{}

	// Common error patterns and suggestions
	if containsPattern(errorMsg, "docker", "daemon", "not running") {
		suggestions = append(suggestions, "• Ensure Docker daemon is running: sudo systemctl start docker")
		suggestions = append(suggestions, "• Check Docker status: docker info")
	} else if containsPattern(errorMsg, "permission denied") {
		suggestions = append(suggestions, "• Check file permissions")
		suggestions = append(suggestions, "• Run with appropriate privileges")
		suggestions = append(suggestions, "• Ensure user is in docker group: sudo usermod -aG docker $USER")
	} else if containsPattern(errorMsg, "no such file", "not found") {
		suggestions = append(suggestions, "• Verify all required files exist in build context")
		suggestions = append(suggestions, "• Check COPY/ADD paths in Dockerfile")
		suggestions = append(suggestions, "• Ensure build context includes necessary files")
	} else if containsPattern(errorMsg, "network", "connection", "timeout") {
		suggestions = append(suggestions, "• Check network connectivity")
		suggestions = append(suggestions, "• Verify proxy settings if behind corporate firewall")
		suggestions = append(suggestions, "• Ensure Docker registry is accessible")
	} else {
		// Generic suggestions
		suggestions = append(suggestions, "• Review error details and retry with correct parameters")
		suggestions = append(suggestions, "• Check system prerequisites and dependencies")
	}

	// Add context from retry context
	if retryCtx != nil && retryCtx.ErrorHistory != "" {
		suggestions = append([]string{
			"• Review the error history above for patterns",
			"• Consider if multiple issues need to be addressed",
		}, suggestions...)
	}

	logger.Error("Enhanced error for AI assistant",
		"operation", operation,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"fix_suggestions", strings.Join(suggestions, "\n"))

	enhancedMsg := fmt.Sprintf(
		"[WORKFLOW FAILURE] %s failed after %d attempts.\n\n"+
			"FINAL ERROR: %v\n\n"+
			"SUGGESTED FIXES:\n%s\n\n"+
			"HOW TO RETRY: Run the workflow again after addressing the issues above.",
		operation, maxAttempts, err, strings.Join(suggestions, "\n"))

	if retryCtx != nil && retryCtx.ErrorHistory != "" {
		enhancedMsg = retryCtx.ErrorHistory + "\n\n" + enhancedMsg
	}

	return fmt.Errorf("%s", enhancedMsg)
}

// logAIGuidance provides structured guidance based on AI analysis
func logAIGuidance(operation string, err error, attempt int, maxAttempts int, analysis *sampling.ErrorAnalysis, logger *slog.Logger) {
	// Build troubleshooting checklist
	checklist := []string{}
	if analysis.RootCause != "" {
		checklist = append(checklist, fmt.Sprintf("• Root cause: %s", analysis.RootCause))
	}
	if len(analysis.FixSteps) > 0 {
		checklist = append(checklist, "• Suggested fixes:")
		for _, step := range analysis.FixSteps {
			checklist = append(checklist, fmt.Sprintf("  - %s", step))
		}
	}
	if len(checklist) == 0 {
		checklist = append(checklist, "• Review error details and retry with correct parameters")
		checklist = append(checklist, "• Check system prerequisites and dependencies")
	}

	logger.Warn("🤖 AI ASSISTANT: Workflow step failed but retrying automatically",
		"operation", operation,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"error_type", fmt.Sprintf("%T", err),
		"error_message", err.Error(),
		"root_cause", analysis.RootCause,
		"retry_available", attempt < maxAttempts,
		"auto_retry_in_progress", true,
		"troubleshooting_checklist", strings.Join(checklist, "\n"),
		"ai_instruction", "The workflow is automatically retrying this step. If all retries fail, you will receive explicit instructions to retry the entire workflow.")
}
