package analyze

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
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
		return nil, fmt.Errorf("invalid arguments: expected map[string]interface{}, got %T", args)
	}

	// Extract required fields
	sessionID, _ := argsMap["session_id"].(string) //nolint:errcheck // Has default
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().Unix())
	}

	// Try different parameter names for repository URL
	repoPath, ok := argsMap["repo_url"].(string)
	if !ok {
		repoPath, ok = argsMap["repo_path"].(string)
		if !ok {
			repoPath, ok = argsMap["path"].(string) // Try alternative field name
			if !ok {
				return nil, fmt.Errorf("missing required parameter: 'repo_url', 'repo_path' or 'path' must be provided")
			}
		}
	}

	// Extract optional branch parameter
	branch, _ := argsMap["branch"].(string)
	if branch == "" {
		branch = "main" // Default branch
	}

	// Create atomic tool args
	atomicArgs := AtomicAnalyzeRepositoryArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: sessionID,
		},
		RepoURL: repoPath,
		Branch:  branch,
	}

	// Call atomic tool
	resultInterface, err := t.atomicTool.Execute(ctx, atomicArgs)
	if err != nil {
		return nil, err
	}

	// Type assert to get the actual result
	result, ok := resultInterface.(*AtomicAnalysisResult)
	if !ok {
		return nil, fmt.Errorf("unexpected result type from atomic tool: expected *AtomicAnalysisResult, got %T", resultInterface)
	}

	// Convert result to legacy format if needed
	if !result.Success {
		// Return error in legacy format
		return map[string]interface{}{
			"success": false,
			"error":   "Analysis failed",
		}, nil
	}

	// Return successful result with all necessary fields
	response := map[string]interface{}{
		"success":    result.Success,
		"session_id": result.SessionID,
		"repo_url":   result.RepoURL,
		"branch":     result.Branch,
		"workspace":  result.WorkspaceDir,
	}

	// Add analysis data if available
	if result.Analysis != nil {
		response["analysis"] = result.Analysis
		if result.Analysis.Language != "" {
			response["language"] = result.Analysis.Language
		}
		if result.Analysis.Framework != "" {
			response["framework"] = result.Analysis.Framework
		}
	}

	return response, nil
}

// Validate validates the input arguments
func (t *AnalyzeRepositoryRedirectTool) Validate(ctx context.Context, args interface{}) error {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid arguments: expected map[string]interface{}, got %T", args)
	}

	// Check required fields - session ID is optional, will be generated if missing
	if sessionID, _ := argsMap["session_id"].(string); sessionID == "" {
		t.logger.Debug().Msg("Session ID not provided, will be generated")
	}

	// Check for repo_url, repo_path or path
	if repoURL, ok := argsMap["repo_url"].(string); !ok || repoURL == "" {
		if repoPath, ok := argsMap["repo_path"].(string); !ok || repoPath == "" {
			if path, ok := argsMap["path"].(string); !ok || path == "" {
				return fmt.Errorf("missing required parameter: 'repo_url', 'repo_path' or 'path' must be provided")
			}
		}
	}

	return nil
}

// GetName returns the tool's name
func (t *AnalyzeRepositoryRedirectTool) GetName() string {
	return "analyze_repository"
}

// GetDescription returns the tool's description
func (t *AnalyzeRepositoryRedirectTool) GetDescription() string {
	return "Analyzes a repository to determine language, framework, dependencies, and containerization requirements. This tool manages session state automatically."
}

// RedirectToolParams represents the parameters for the redirect tool
type RedirectToolParams struct {
	SessionID string `json:"session_id,omitempty"`
	RepoURL   string `json:"repo_url,omitempty"`
	RepoPath  string `json:"repo_path,omitempty"`
	Path      string `json:"path,omitempty"`
	Branch    string `json:"branch,omitempty"`
}

// Validate validates the parameters
func (p RedirectToolParams) Validate() error {
	if p.RepoURL == "" && p.RepoPath == "" && p.Path == "" {
		return fmt.Errorf("one of repo_url, repo_path, or path must be provided")
	}
	return nil
}

// GetSessionID returns the session ID
func (p RedirectToolParams) GetSessionID() string {
	return p.SessionID
}

// RedirectToolResult represents the result of the redirect tool
type RedirectToolResult struct {
	Success   bool                   `json:"success"`
	SessionID string                 `json:"session_id,omitempty"`
	RepoURL   string                 `json:"repo_url,omitempty"`
	Branch    string                 `json:"branch,omitempty"`
	Workspace string                 `json:"workspace,omitempty"`
	Analysis  map[string]interface{} `json:"analysis,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// IsSuccess returns whether the result was successful
func (r RedirectToolResult) IsSuccess() bool {
	return r.Success
}

// GetSchema returns the tool's schema
func (t *AnalyzeRepositoryRedirectTool) GetSchema() tools.Schema[RedirectToolParams, RedirectToolResult] {
	return tools.Schema[RedirectToolParams, RedirectToolResult]{
		Name:        "analyze_repository",
		Description: t.GetDescription(),
		Version:     "1.0.0",
		ParamsSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier (optional, will be generated if not provided)",
				},
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "Repository URL or path to analyze",
				},
				"repo_path": map[string]interface{}{
					"type":        "string",
					"description": "Alternative field name for repo_url",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Alternative field name for repo_url",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Git branch to analyze (default: main)",
				},
			},
			"anyOf": []interface{}{
				map[string]interface{}{"required": []string{"repo_url"}},
				map[string]interface{}{"required": []string{"repo_path"}},
				map[string]interface{}{"required": []string{"path"}},
			},
		},
		ResultSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the analysis was successful",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for the analysis",
				},
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "Repository URL that was analyzed",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Branch that was analyzed",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "Workspace directory containing the analyzed repository",
				},
				"analysis": map[string]interface{}{
					"type":        "object",
					"description": "Analysis results",
				},
			},
		},
	}
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
			"repo_url":   "Repository URL or path to analyze",
			"repo_path":  "Alternative field name for repo_url",
			"path":       "Alternative field name for repo_url",
			"branch":     "Git branch to analyze (default: main)",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Analyze Local Repository",
				Description: "Analyze a local repository for containerization",
				Input: map[string]interface{}{
					"session_id": "analysis-session",
					"repo_url":   "/path/to/repository",
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
