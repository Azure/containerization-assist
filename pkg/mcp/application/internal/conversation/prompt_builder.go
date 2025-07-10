package conversation

import (
	"fmt"
	"log/slog"
	"strings"
)

// ErrorRecoveryPromptBuilder builds prompts for error recovery scenarios
type ErrorRecoveryPromptBuilder struct {
	logger *slog.Logger
}

// NewErrorRecoveryPromptBuilder creates a new ErrorRecoveryPromptBuilder
func NewErrorRecoveryPromptBuilder(logger *slog.Logger) *ErrorRecoveryPromptBuilder {
	return &ErrorRecoveryPromptBuilder{
		logger: logger.With("component", "error_recovery_prompt_builder"),
	}
}

// BuildPrompt builds a recovery prompt based on the retry context
func (b *ErrorRecoveryPromptBuilder) BuildPrompt(retryCtx *RetryContext) string {
	var sb strings.Builder

	sb.WriteString("🔄 **Error Recovery Assistance**\n\n")
	sb.WriteString(fmt.Sprintf("**Session**: %s\n", retryCtx.SessionID))
	sb.WriteString(fmt.Sprintf("**Attempt**: %d\n", retryCtx.AttemptCount))
	sb.WriteString(fmt.Sprintf("**Original Error**: %s\n\n", retryCtx.OriginalError))

	if retryCtx.ProjectContext != nil {
		sb.WriteString("**Project Context**:\n")
		sb.WriteString(fmt.Sprintf("- Workspace: %s\n", retryCtx.ProjectContext.WorkspaceDir))
		if retryCtx.ProjectContext.Language != "" {
			sb.WriteString(fmt.Sprintf("- Language: %s\n", retryCtx.ProjectContext.Language))
		}
		if retryCtx.ProjectContext.Framework != "" {
			sb.WriteString(fmt.Sprintf("- Framework: %s\n", retryCtx.ProjectContext.Framework))
		}
		sb.WriteString("\n")
	}

	if len(retryCtx.PreviousAttempts) > 0 {
		sb.WriteString("**Previous Attempts**:\n")
		for _, attempt := range retryCtx.PreviousAttempts {
			sb.WriteString(fmt.Sprintf("- Attempt %d: %s (Result: %s)\n",
				attempt.AttemptNumber, attempt.Approach, attempt.Result))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// RepositoryContext holds context about the repository being processed
type RepositoryContext struct {
	WorkspaceDir   string            `json:"workspace_dir"`
	Language       string            `json:"language,omitempty"`
	Framework      string            `json:"framework,omitempty"`
	Dependencies   []string          `json:"dependencies,omitempty"`
	BuildTool      string            `json:"build_tool,omitempty"`
	PackageManager string            `json:"package_manager,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}
