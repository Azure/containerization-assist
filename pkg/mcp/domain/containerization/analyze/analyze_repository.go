package analyze

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/types/tools"
	toolstypes "github.com/Azure/container-kit/pkg/mcp/domain/types/tools"
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
		return nil, errors.NewError().Messagef("invalid arguments: expected map[string]interface{}, got %T", args).WithLocation(

		// Extract required fields
		).Build()
	}

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
				return nil, errors.NewError().Messagef("missing required parameter: 'repo_url', 'repo_path' or 'path' must be provided").

					// Extract optional branch parameter
					Build()
			}
		}
	}

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

	// Convert to typed parameters for the atomic tool
	typedParams := toolstypes.AtomicAnalyzeRepositoryParams{
		SessionParams: toolstypes.SessionParams{
			SessionID: atomicArgs.SessionID,
		},
		RepoURL:      atomicArgs.RepoURL,
		Branch:       atomicArgs.Branch,
		Context:      atomicArgs.Context,
		LanguageHint: atomicArgs.LanguageHint,
		Shallow:      atomicArgs.Shallow,
	}

	// Call atomic tool with standard Execute interface
	toolInput := api.ToolInput{
		SessionID: typedParams.SessionID,
		Data: map[string]interface{}{
			"repo_url":      typedParams.RepoURL,
			"branch":        typedParams.Branch,
			"context":       typedParams.Context,
			"language_hint": typedParams.LanguageHint,
			"shallow":       typedParams.Shallow,
		},
	}

	toolOutput, err := t.atomicTool.Execute(ctx, toolInput)
	if err != nil {
		return nil, err
	}

	// Extract result from ToolOutput and convert to expected format
	if !toolOutput.Success {
		return nil, errors.NewError().Messagef("atomic tool execution failed: %s", toolOutput.Error).WithLocation(

		// Create a compatible result structure from the tool output data
		).Build()
	}

	typedResult := &toolstypes.AtomicAnalyzeRepositoryResult{
		BaseToolResponse: types.BaseToolResponse{
			Success: toolOutput.Success,
		},
		SessionID: extractStringFromToolOutput(toolOutput.Data, "session_id"),
	}

	// Extract other fields if present
	if workspaceDir := extractStringFromToolOutput(toolOutput.Data, "workspace_dir"); workspaceDir != "" {
		typedResult.WorkspaceDir = workspaceDir
	}

	// Convert typed result back to internal format for compatibility
	result := &AtomicAnalysisResult{
		BaseToolResponse: types.BaseToolResponse{
			Success: typedResult.Success,
		},
		Success:      typedResult.Success,
		SessionID:    typedResult.SessionID,
		WorkspaceDir: typedResult.WorkspaceDir,
		RepoURL:      extractStringFromToolOutput(toolOutput.Data, "repo_url"),
		Branch:       extractStringFromToolOutput(toolOutput.Data, "branch"),
		CloneDir:     extractStringFromToolOutput(toolOutput.Data, "clone_dir"),
		AnalysisContext: &AnalysisContext{
			FilesAnalyzed: extractIntFromToolOutput(toolOutput.Data, "files_analyzed"),
		},
	}

	// Set BaseToolResponse fields
	result.BaseToolResponse.Success = true
	result.BaseToolResponse.Message = "Repository analysis completed"
	result.BaseToolResponse.Timestamp = time.Now()

	// Convert result to legacy format if needed
	legacyResponse := &LegacyAnalysisResponse{
		Success:   result.Success,
		SessionID: result.SessionID,
		RepoURL:   result.RepoURL,
		Branch:    result.Branch,
		Workspace: result.WorkspaceDir,
	}

	if !result.Success {
		legacyResponse.Error = "Analysis failed"
		return legacyResponse.ToMap(), nil
	}

	// Add analysis data from tool output if available
	if language := extractStringFromToolOutput(toolOutput.Data, "language"); language != "" {
		legacyResponse.Language = language
	}
	if framework := extractStringFromToolOutput(toolOutput.Data, "framework"); framework != "" {
		legacyResponse.Framework = framework
	}

	// Create analysis map from available data
	legacyResponse.Analysis = map[string]interface{}{
		"language":     legacyResponse.Language,
		"framework":    legacyResponse.Framework,
		"dependencies": extractDependenciesFromToolOutput(toolOutput.Data),
	}

	return legacyResponse.ToMap(), nil
}

// Validate validates the input arguments
func (t *AnalyzeRepositoryRedirectTool) Validate(ctx context.Context, args interface{}) error {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("invalid arguments: expected map[string]interface{}, got %T", args).WithLocation(

		// Check required fields - session ID is optional, will be generated if missing
		).Build()
	}

	if sessionID, _ := argsMap["session_id"].(string); sessionID == "" {
		t.logger.Debug().Msg("Session ID not provided, will be generated")
	}

	// Check for repo_url, repo_path or path
	if repoURL, ok := argsMap["repo_url"].(string); !ok || repoURL == "" {
		if repoPath, ok := argsMap["repo_path"].(string); !ok || repoPath == "" {
			if path, ok := argsMap["path"].(string); !ok || path == "" {
				return errors.NewError().Messagef("missing required parameter: 'repo_url', 'repo_path' or 'path' must be provided").Build()
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

// LegacyAnalysisResponse represents the legacy response format for backward compatibility
type LegacyAnalysisResponse struct {
	Success   bool                   `json:"success"`
	SessionID string                 `json:"session_id,omitempty"`
	RepoURL   string                 `json:"repo_url,omitempty"`
	Branch    string                 `json:"branch,omitempty"`
	Workspace string                 `json:"workspace,omitempty"`
	Language  string                 `json:"language,omitempty"`
	Framework string                 `json:"framework,omitempty"`
	Analysis  map[string]interface{} `json:"analysis,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// ToMap converts LegacyAnalysisResponse to map[string]interface{} for backward compatibility
func (r *LegacyAnalysisResponse) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"success": r.Success,
	}

	if r.SessionID != "" {
		result["session_id"] = r.SessionID
	}
	if r.RepoURL != "" {
		result["repo_url"] = r.RepoURL
	}
	if r.Branch != "" {
		result["branch"] = r.Branch
	}
	if r.Workspace != "" {
		result["workspace"] = r.Workspace
	}
	if r.Language != "" {
		result["language"] = r.Language
	}
	if r.Framework != "" {
		result["framework"] = r.Framework
	}
	if r.Analysis != nil {
		result["analysis"] = r.Analysis
	}
	if r.Error != "" {
		result["error"] = r.Error
	}

	return result
}

// extractStringFromToolOutput safely extracts a string value from tool output data
func extractStringFromToolOutput(data map[string]interface{}, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

// extractIntFromToolOutput safely extracts an int value from tool output data
func extractIntFromToolOutput(data map[string]interface{}, key string) int {
	if value, ok := data[key].(int); ok {
		return value
	}
	if value, ok := data[key].(float64); ok {
		return int(value)
	}
	return 0
}

// extractDependenciesFromToolOutput safely extracts dependencies from tool output data
func extractDependenciesFromToolOutput(data map[string]interface{}) []interface{} {
	if deps, ok := data["dependencies"].([]interface{}); ok {
		return deps
	}
	return []interface{}{}
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
		return errors.NewError().Messagef("one of repo_url, repo_path, or path must be provided").Build(

		// GetSessionID returns the session ID
		)
	}
	return nil
}

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

// Schema returns the tool's schema
func (t *AnalyzeRepositoryRedirectTool) Schema() tools.Schema[RedirectToolParams, RedirectToolResult] {
	return tools.Schema[RedirectToolParams, RedirectToolResult]{
		Name:        "analyze_repository",
		Description: t.GetDescription(),
		Version:     "1.0.0",
		ParamsSchema: tools.FromMap(map[string]interface{}{
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
		}),
		ResultSchema: tools.FromMap(map[string]interface{}{
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
		}),
	}
}

// GetMetadata returns the tool metadata
func (t *AnalyzeRepositoryRedirectTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "analyze_repository",
		Description:  "Analyzes a repository to determine language, framework, dependencies, and containerization requirements",
		Version:      "1.0.0",
		Category:     api.ToolCategory("analysis"),
		Tags:         []string{"analysis", "repository", "containerization"},
		Status:       api.ToolStatus("active"),
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
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}
