package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// AnalyzeRepositoryRedirectTool redirects to the atomic tool
type AnalyzeRepositoryRedirectTool struct {
	atomicTool *AtomicAnalyzeRepositoryTool
	logger     zerolog.Logger
}

// NewAnalyzeRepositoryRedirectTool creates a new redirect tool
func NewAnalyzeRepositoryRedirectTool(atomicTool *AtomicAnalyzeRepositoryTool, logger zerolog.Logger) *AnalyzeRepositoryRedirectTool {
	return &AnalyzeRepositoryRedirectTool{
		atomicTool: atomicTool,
		logger:     logger.With().Str("tool", "analyze_repository_redirect").Logger(),
	}
}

// Execute redirects to the atomic tool
func (t *AnalyzeRepositoryRedirectTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	t.logger.Info().Msg("Redirecting analyze_repository to atomic tool")

	// Convert args to map if needed
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return nil, types.NewRichError("INVALID_ARGUMENTS", "invalid argument type: expected map[string]interface{}", "validation_error")
	}

	// Extract required fields
	sessionID, _ := argsMap["session_id"].(string) //nolint:errcheck // Has default
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().Unix())
	}

	repoPath, ok := argsMap["repo_path"].(string)
	if !ok {
		repoPath, ok = argsMap["path"].(string) // Try alternative field name
		if !ok {
			return nil, types.NewRichError("INVALID_ARGUMENTS", "repo_path is required", "validation_error")
		}
	}

	// Create atomic tool args
	atomicArgs := AtomicAnalyzeRepositoryArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: sessionID,
		},
		RepoURL: repoPath,
	}

	// Call atomic tool
	resultInterface, err := t.atomicTool.Execute(ctx, atomicArgs)
	if err != nil {
		return nil, err
	}

	// Type assert to get the actual result
	result, ok := resultInterface.(*AtomicAnalysisResult)
	if !ok {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("unexpected result type: %T", resultInterface), "execution_error")
	}

	// Convert result to legacy format if needed
	if !result.Success {
		// Return error in legacy format
		return map[string]interface{}{
			"success": false,
			"error":   "Analysis failed",
		}, nil
	}

	// Return successful result
	return map[string]interface{}{
		"success":    result.Success,
		"session_id": result.SessionID,
		"repo_url":   result.RepoURL,
		"analysis":   result.Analysis,
		"workspace":  result.WorkspaceDir,
	}, nil
}

// GetToolName returns the tool name
func (t *AnalyzeRepositoryRedirectTool) GetToolName() string {
	return "analyze_repository"
}

// GetToolDescription returns the tool description
func (t *AnalyzeRepositoryRedirectTool) GetToolDescription() string {
	return "Analyzes a repository to determine language, framework, dependencies, and containerization requirements"
}
