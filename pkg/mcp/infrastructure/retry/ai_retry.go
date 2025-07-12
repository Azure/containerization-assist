package retry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
	"github.com/mark3labs/mcp-go/server"
)

// WithAIRetry wraps a function with AI-powered retry logic
// This works with external AI assistants (like Claude) using the MCP server
// The AI assistant observes failures through structured error reporting and can retry the workflow
func WithAIRetry(ctx context.Context, name string, max int, fn func() error, logger *slog.Logger) error {
	// Try to get MCP server from context for enhanced retry with sampling
	if srv := server.ServerFromContext(ctx); srv != nil {
		return WithLLMGuidedRetry(ctx, name, max, fn, logger)
	}

	// Fallback to basic retry logic
	return withBasicAIRetry(ctx, name, max, fn, logger)
}

// WithLLMGuidedRetry uses MCP sampling for intelligent retry logic
func WithLLMGuidedRetry(ctx context.Context, name string, max int, fn func() error, logger *slog.Logger) error {
	logger.Info("Starting operation with LLM-guided retry", "operation", name, "max_retries", max)

	samplingClient := sampling.NewClient(ctx, logger)

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
			return enhanceErrorForAI(name, err, i, max, logger)
		}

		// Use LLM to analyze the error and suggest fixes
		analysis, analysisErr := samplingClient.AnalyzeError(ctx, name, err, fmt.Sprintf("Attempt %d of %d", i, max))
		if analysisErr != nil {
			logger.Warn("Failed to get LLM analysis", "error", analysisErr)
			// Continue with basic retry
			continue
		}

		// Log the analysis for visibility
		logger.Info("LLM Error Analysis",
			"operation", name,
			"root_cause", analysis.RootCause,
			"can_auto_fix", analysis.CanAutoFix,
			"fix_steps", len(analysis.FixSteps))

		// If we can auto-fix, attempt to apply fixes
		if analysis.CanAutoFix && len(analysis.FixSteps) > 0 {
			logger.Info("Attempting automated fixes suggested by LLM")
			// Note: Actual fix application would be operation-specific
			// This is a hook for future enhancement
		}

		// Continue to next retry with LLM insights logged
	}

	return fmt.Errorf("%s: exhausted %d retries", name, max)
}

// withBasicAIRetry is the original retry logic without MCP sampling
func withBasicAIRetry(ctx context.Context, name string, max int, fn func() error, logger *slog.Logger) error {
	logger.Info("Starting operation with AI retry", "operation", name, "max_retries", max)

	for i := 1; i <= max; i++ {
		logger.Debug("Attempting operation", "operation", name, "attempt", i, "max", max)

		if err := fn(); err != nil {
			logger.Error("Operation failed", "operation", name, "attempt", i, "max", max, "error", err)

			// If this was the last attempt, return the structured error
			if i == max {
				logger.Error("Operation exhausted all retries", "operation", name, "attempts", max)

				// Return structured error with fix suggestions for the AI assistant
				errorWithSuggestions := enhanceErrorForAI(name, err, i, max, logger)
				return errorWithSuggestions
			}

			// Log structured error information for AI assistant to observe
			logAIObservableError(name, err, i, max, logger)

			// Continue to next attempt - the external AI assistant can observe
			// this failure pattern and make decisions about retrying the workflow
			continue
		}

		// Success!
		logger.Info("Operation completed successfully", "operation", name, "attempt", i)
		return nil
	}

	// This should never be reached due to the logic above
	return fmt.Errorf("%s: exhausted %d retries", name, max)
}

// Fix represents an AI-suggested fix for an error
type Fix struct {
	Description string `json:"description"`
	Action      string `json:"action"`
	Target      string `json:"target,omitempty"`
	Content     string `json:"content,omitempty"`
}

// logAIObservableError logs structured error information for AI assistant observation
func logAIObservableError(operation string, err error, attempt, maxAttempts int, logger *slog.Logger) {
	// Log detailed error information in a structured format
	// that the AI assistant can observe and learn from
	suggestions := generateFixSuggestions(operation, err.Error(), logger)

	logger.Warn("🤖 AI ASSISTANT: Workflow step failed but retrying automatically",
		"operation", operation,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"error_type", fmt.Sprintf("%T", err),
		"error_message", err.Error(),
		"retry_available", attempt < maxAttempts,
		"auto_retry_in_progress", true,
		"troubleshooting_checklist", suggestions,
		"ai_instruction", "The workflow is automatically retrying this step. If all retries fail, you will receive explicit instructions to retry the entire workflow.")
}

// enhanceErrorForAI creates a structured error with fix suggestions for the AI assistant
func enhanceErrorForAI(operation string, originalErr error, attempt, maxAttempts int, logger *slog.Logger) error {
	errorMsg := originalErr.Error()

	// Generate fix suggestions based on error patterns
	fixSuggestions := generateFixSuggestions(operation, errorMsg, logger)

	// Create enhanced error message with explicit instructions for AI assistant
	enhancedMsg := fmt.Sprintf(`🤖 AI ASSISTANT: The containerize_and_deploy workflow failed and needs your help.

OPERATION FAILED: "%s" (attempt %d/%d)
ERROR: %s

🔧 REQUIRED ACTION: Please retry the workflow by calling the containerize_and_deploy tool again with these exact parameters:
- repo_url: (use the same repository URL from the previous attempt)
- branch: "master" (recommended - main branch may not exist)
- scan: (use the same scan setting from the previous attempt)

📋 TROUBLESHOOTING CHECKLIST:
%s

⚡ IMMEDIATE NEXT STEP: Call containerize_and_deploy tool with the same parameters to continue the workflow.`,
		operation, attempt, maxAttempts, errorMsg, fixSuggestions)

	logger.Error("Enhanced error for AI assistant",
		"operation", operation,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"fix_suggestions", fixSuggestions)

	return fmt.Errorf("%s", enhancedMsg)
}

// generateFixSuggestions creates fix suggestions based on error patterns
func generateFixSuggestions(operation string, errorMsg string, logger *slog.Logger) string {
	var suggestions []string

	// Analyze the error message for common patterns and suggest fixes
	if containsPattern(errorMsg, "dockerfile", "syntax error", "unknown instruction") {
		suggestions = append(suggestions, "• Check Dockerfile syntax and instruction names")
		suggestions = append(suggestions, "• Verify base image names and tags")
		suggestions = append(suggestions, "• Ensure proper FROM instruction format")
	}

	if containsPattern(errorMsg, "docker build", "failed", "no such file") {
		suggestions = append(suggestions, "• Verify all required files exist in build context")
		suggestions = append(suggestions, "• Check COPY/ADD paths in Dockerfile")
		suggestions = append(suggestions, "• Ensure build context includes necessary files")
	}

	if containsPattern(errorMsg, "kubernetes", "deploy", "image pull") {
		suggestions = append(suggestions, "• Verify image exists in local registry (localhost:5001)")
		suggestions = append(suggestions, "• Check image name and tag format")
		suggestions = append(suggestions, "• Ensure kind cluster can access the image")
	}

	if containsPattern(errorMsg, "port", "connection", "refused") {
		suggestions = append(suggestions, "• Verify application listens on correct port")
		suggestions = append(suggestions, "• Check port bindings in Dockerfile and K8s manifests")
		suggestions = append(suggestions, "• Ensure no port conflicts with existing services")
	}

	if containsPattern(errorMsg, "permission", "denied", "access") {
		suggestions = append(suggestions, "• Check file permissions in repository")
		suggestions = append(suggestions, "• Verify Docker daemon permissions")
		suggestions = append(suggestions, "• Ensure kubectl has proper cluster access")
	}

	if containsPattern(errorMsg, "kind", "cluster", "not found") {
		suggestions = append(suggestions, "• Ensure kind cluster 'container-kit' exists")
		suggestions = append(suggestions, "• Verify kind and kubectl are installed")
		suggestions = append(suggestions, "• Check cluster connectivity")
	}

	if containsPattern(errorMsg, "git", "clone", "repository") {
		suggestions = append(suggestions, "• Verify repository URL is accessible")
		suggestions = append(suggestions, "• Check network connectivity")
		suggestions = append(suggestions, "• Try different branch (main/master)")
	}

	// Default suggestions if no specific patterns match
	if len(suggestions) == 0 {
		switch operation {
		case "analyze_repository":
			suggestions = append(suggestions, "• Verify repository URL and branch name")
			suggestions = append(suggestions, "• Check network connectivity and access permissions")
		case "generate_dockerfile":
			suggestions = append(suggestions, "• Review detected language and framework")
			suggestions = append(suggestions, "• Check if repository structure matches expectations")
		case "build_image":
			suggestions = append(suggestions, "• Verify Docker daemon is running")
			suggestions = append(suggestions, "• Check Dockerfile content and build context")
		case "deploy_to_k8s":
			suggestions = append(suggestions, "• Verify kind cluster is running")
			suggestions = append(suggestions, "• Check kubectl configuration and permissions")
		default:
			suggestions = append(suggestions, "• Review error details and retry with correct parameters")
			suggestions = append(suggestions, "• Check system prerequisites and dependencies")
		}
	}

	if len(suggestions) == 0 {
		return "No specific suggestions available - review error details"
	}

	return strings.Join(suggestions, "\n")
}

// containsPattern checks if the prompt contains any of the given patterns
func containsPattern(prompt string, patterns ...string) bool {
	promptLower := strings.ToLower(prompt) // Convert to lowercase for case-insensitive matching
	for _, pattern := range patterns {
		if contains(promptLower, pattern) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple substring check - in a real implementation you might use regex
	// or more sophisticated pattern matching
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring performs a simple substring search
func findSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Apply applies the suggested fix (kept for backward compatibility)
func (f *Fix) Apply() error {
	// This method is now deprecated as we rely on external AI assistant
	// to observe errors and make fixing decisions through the MCP workflow
	slog.Info("Fix application delegated to external AI assistant",
		"description", f.Description,
		"action", f.Action)
	return nil
}

// RetryableOperation represents an operation that can be retried with AI assistance
type RetryableOperation struct {
	Name       string
	MaxRetries int
	Logger     *slog.Logger
}

// Execute runs the operation with AI retry logic
func (op *RetryableOperation) Execute(ctx context.Context, fn func() error) error {
	return WithAIRetry(ctx, op.Name, op.MaxRetries, fn, op.Logger)
}

// NewRetryableOperation creates a new retryable operation
func NewRetryableOperation(name string, maxRetries int, logger *slog.Logger) *RetryableOperation {
	return &RetryableOperation{
		Name:       name,
		MaxRetries: maxRetries,
		Logger:     logger,
	}
}
