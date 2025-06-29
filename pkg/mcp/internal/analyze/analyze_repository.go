package analyze

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
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
		return nil, mcp.NewRichError("INVALID_ARGUMENTS", "invalid argument type: expected map[string]interface{}", "validation_error")
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
			return nil, mcp.NewRichError("INVALID_ARGUMENTS", "repo_path is required", "validation_error")
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
		return nil, mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("unexpected result type: %T", resultInterface), "execution_error")
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

// Validate validates the input arguments
func (t *AnalyzeRepositoryRedirectTool) Validate(ctx context.Context, args interface{}) error {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewRichError("INVALID_ARGUMENTS", "invalid argument type: expected map[string]interface{}", "validation_error")
	}

	// Check required fields - session ID is optional, will be generated if missing
	if sessionID, _ := argsMap["session_id"].(string); sessionID == "" {
		t.logger.Debug().Msg("Session ID not provided, will be generated")
	}

	// Check for repo_path or path
	if repoPath, ok := argsMap["repo_path"].(string); !ok || repoPath == "" {
		if path, ok := argsMap["path"].(string); !ok || path == "" {
			return mcp.NewRichError("INVALID_ARGUMENTS", "repo_path or path is required", "validation_error")
		}
	}

	return nil
}

// GetMetadata returns the tool metadata
func (t *AnalyzeRepositoryRedirectTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:         "analyze_repository",
		Description:  "Analyzes a repository to determine language, framework, dependencies, and containerization requirements",
		Version:      "1.0.0",
		Category:     "analysis",
		Dependencies: []string{"analyze_repository_atomic"},
		Capabilities: []string{
			"language_detection",
			"framework_analysis",
			"dependency_scanning",
			"structure_analysis",
			"containerization_assessment",
		},
		Requirements: []string{
			"repository_access",
			"workspace_access",
		},
		Parameters: map[string]string{
			"session_id": "Session identifier (optional, will be generated if not provided)",
			"repo_path":  "Path to the repository to analyze",
			"path":       "Alternative field name for repo_path",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Analyze Local Repository",
				Description: "Analyze a local repository for containerization",
				Input: map[string]interface{}{
					"session_id": "analysis-session",
					"repo_path":  "/path/to/repository",
				},
				Output: map[string]interface{}{
					"success":    true,
					"language":   "javascript",
					"framework":  "express",
					"port":       3000,
					"dockerfile": "Generated Dockerfile ready",
				},
			},
		},
	}
}
