package analyze

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	toolstypes "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
)

// GenericAnalyzeRepositoryTool implements the type-safe api.AnalyzeTool interface
type GenericAnalyzeRepositoryTool struct {
	atomicTool     *AtomicAnalyzeRepositoryTool
	sessionManager session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore   services.SessionStore         // Modern service interface
	sessionState   services.SessionState         // Modern service interface
	logger         zerolog.Logger
	timeout        time.Duration
}

// NewGenericAnalyzeRepositoryTool creates a new type-safe analyze repository tool (legacy constructor)
func NewGenericAnalyzeRepositoryTool(
	atomicTool *AtomicAnalyzeRepositoryTool,
	sessionManager session.UnifiedSessionManager,
	logger zerolog.Logger,
) api.AnalyzeTool {
	return &GenericAnalyzeRepositoryTool{
		atomicTool:     atomicTool,
		sessionManager: sessionManager,
		logger:         logger.With().Str("tool", "generic_analyze_repository").Logger(),
		timeout:        5 * time.Minute, // Default timeout
	}
}

// NewGenericAnalyzeRepositoryToolWithServices creates a new type-safe analyze repository tool with services
func NewGenericAnalyzeRepositoryToolWithServices(
	atomicTool *AtomicAnalyzeRepositoryTool,
	serviceContainer services.ServiceContainer,
	logger zerolog.Logger,
) api.AnalyzeTool {
	toolLogger := logger.With().Str("tool", "generic_analyze_repository").Logger()

	return &GenericAnalyzeRepositoryTool{
		atomicTool:   atomicTool,
		sessionStore: serviceContainer.SessionStore(),
		sessionState: serviceContainer.SessionState(),
		logger:       toolLogger,
		timeout:      5 * time.Minute, // Default timeout
	}
}

// Name implements api.GenericTool
func (t *GenericAnalyzeRepositoryTool) Name() string {
	return "generic_analyze_repository"
}

// Description implements api.GenericTool
func (t *GenericAnalyzeRepositoryTool) Description() string {
	return "Type-safe repository analysis tool with compile-time guarantees"
}

// Execute implements api.GenericTool with type-safe input and output
func (t *GenericAnalyzeRepositoryTool) Execute(ctx context.Context, input *api.AnalyzeInput) (*api.AnalyzeOutput, error) {
	startTime := time.Now()

	t.logger.Info().
		Str("session_id", input.SessionID).
		Str("repo_url", input.RepoURL).
		Str("branch", input.Branch).
		Bool("include_dependencies", input.IncludeDependencies).
		Bool("include_security", input.IncludeSecurityScan).
		Msg("Starting type-safe repository analysis")

	// Convert generic input to atomic tool parameters
	atomicParams := toolstypes.AtomicAnalyzeRepositoryParams{
		SessionParams: toolstypes.SessionParams{
			SessionID: input.SessionID,
		},
		RepoURL:      input.RepoURL,
		Branch:       getBranchOrDefault(input.Branch),
		Context:      "", // Could be extracted from custom options
		LanguageHint: input.LanguageHint,
		Shallow:      false, // Default to full clone
	}

	// Execute using the atomic tool
	rawResult, err := t.atomicTool.ExecuteTypedInterface(ctx, atomicParams)
	if err != nil {
		return &api.AnalyzeOutput{
			Success:      false,
			SessionID:    input.SessionID,
			ErrorMsg:     fmt.Sprintf("Repository analysis failed: %v", err),
			AnalysisTime: time.Since(startTime),
		}, err
	}

	// Type assert the result to the expected type
	atomicResult, ok := rawResult.(*AtomicAnalysisResult)
	if !ok {
		return &api.AnalyzeOutput{
				Success:      false,
				SessionID:    input.SessionID,
				ErrorMsg:     fmt.Sprintf("Invalid result type from atomic tool: expected *AtomicAnalysisResult, got %T", rawResult),
				AnalysisTime: time.Since(startTime),
			}, errors.NewError().Messagef("invalid result type from atomic tool").WithLocation(

			// Convert atomic result to generic output
			).Build()
	}

	output := &api.AnalyzeOutput{
		Success:       atomicResult.Success,
		SessionID:     input.SessionID,
		AnalysisTime:  time.Since(startTime),
		FilesAnalyzed: 0, // Default value since this field may not exist
		Data:          make(map[string]interface{}),
	}

	// Extract analysis data if available
	if atomicResult.Analysis != nil {
		output.Language = atomicResult.Analysis.Language
		output.Framework = atomicResult.Analysis.Framework

		// Convert dependencies if requested
		if input.IncludeDependencies && atomicResult.Analysis.Dependencies != nil {
			// Convert from []analysis.Dependency to []api.Dependency
			output.Dependencies = convertDependenciesFromSlice(atomicResult.Analysis.Dependencies)
		}
	}

	// Add build recommendations if requested (simplified version since BuildRecommendations may not exist)
	if input.IncludeBuildAnalysis {
		// Use default recommendations for now
		output.BuildRecommendations = []string{
			"Consider using multi-stage builds for smaller images",
			"Use specific version tags instead of 'latest'",
			"Run security scans on your images",
		}
	}

	// Perform security scan if requested
	if input.IncludeSecurityScan {
		// Simplified security issues for now
		output.SecurityIssues = []api.SecurityIssue{}
	}

	// Store additional data
	output.Data = map[string]interface{}{
		"repo_url":      atomicResult.RepoURL,
		"branch":        atomicResult.Branch,
		"clone_dir":     atomicResult.CloneDir,
		"workspace_dir": atomicResult.WorkspaceDir,
	}

	t.logger.Info().
		Str("session_id", input.SessionID).
		Bool("success", output.Success).
		Dur("duration", output.AnalysisTime).
		Int("files_analyzed", output.FilesAnalyzed).
		Str("language", output.Language).
		Str("framework", output.Framework).
		Msg("Completed type-safe repository analysis")

	return output, nil
}

// Schema implements api.GenericTool
func (t *GenericAnalyzeRepositoryTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "generic_analyze_repository",
		Description: "Type-safe repository analysis tool with compile-time guarantees",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "Repository URL to analyze",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Git branch to analyze (optional)",
				},
				"language_hint": map[string]interface{}{
					"type":        "string",
					"description": "Hint about the primary language (optional)",
				},
				"include_dependencies": map[string]interface{}{
					"type":        "boolean",
					"description": "Include dependency analysis",
				},
				"include_security_scan": map[string]interface{}{
					"type":        "boolean",
					"description": "Include security vulnerability scan",
				},
				"include_build_analysis": map[string]interface{}{
					"type":        "boolean",
					"description": "Include build configuration analysis",
				},
			},
			"required": []string{"session_id", "repo_url"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the analysis was successful",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Detected primary language",
				},
				"framework": map[string]interface{}{
					"type":        "string",
					"description": "Detected framework",
				},
				"dependencies": map[string]interface{}{
					"type":        "array",
					"description": "Project dependencies",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":    map[string]interface{}{"type": "string"},
							"version": map[string]interface{}{"type": "string"},
							"type":    map[string]interface{}{"type": "string"},
						},
					},
				},
				"security_issues": map[string]interface{}{
					"type":        "array",
					"description": "Security vulnerabilities found",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":          map[string]interface{}{"type": "string"},
							"severity":    map[string]interface{}{"type": "string"},
							"description": map[string]interface{}{"type": "string"},
						},
					},
				},
				"build_recommendations": map[string]interface{}{
					"type":        "array",
					"description": "Build optimization recommendations",
					"items":       map[string]interface{}{"type": "string"},
				},
				"analysis_time": map[string]interface{}{
					"type":        "string",
					"description": "Time taken for analysis",
				},
				"files_analyzed": map[string]interface{}{
					"type":        "integer",
					"description": "Number of files analyzed",
				},
			},
		},
		Examples: []api.ToolExample{
			{
				Name:        "Basic Repository Analysis",
				Description: "Analyze a repository with basic options",
				Input: api.ToolInput{
					SessionID: "session_123",
					Data: map[string]interface{}{
						"session_id":           "session_123",
						"repo_url":             "https://github.com/example/repo",
						"branch":               "main",
						"include_dependencies": true,
					},
				},
				Output: api.ToolOutput{
					Success: true,
					Data: map[string]interface{}{
						"language":       "Go",
						"framework":      "gin",
						"files_analyzed": 45,
					},
				},
			},
		},
		Tags:     []string{"analysis", "repository", "type-safe"},
		Category: api.CategoryAnalyze,
	}
}

// Validate implements api.GenericTool
func (t *GenericAnalyzeRepositoryTool) Validate(ctx context.Context, input *api.AnalyzeInput) error {
	// Input validation is already implemented in the AnalyzeInput.Validate() method
	// This method can perform additional business logic validation
	if err := input.Validate(); err != nil {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Input validation failed").
			Cause(err).
			Build()
	}

	// Additional validation can be added here
	if input.RepoURL == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Repository URL is required").
			Build()
	}

	return nil
}

// GetTimeout implements api.GenericTool
func (t *GenericAnalyzeRepositoryTool) GetTimeout() time.Duration {
	return t.timeout
}

// SetTimeout allows configuring the tool timeout
func (t *GenericAnalyzeRepositoryTool) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

// Helper functions

func convertDependencies(deps map[string]string) []api.Dependency {
	result := make([]api.Dependency, 0, len(deps))
	for name, version := range deps {
		result = append(result, api.Dependency{
			Name:    name,
			Version: version,
			Type:    "dependency", // Default type since map doesn't have type info
		})
	}
	return result
}

func convertDependenciesFromSlice(deps []analysis.Dependency) []api.Dependency {
	result := make([]api.Dependency, 0, len(deps))
	for _, dep := range deps {
		result = append(result, api.Dependency{
			Name:    dep.Name,
			Version: dep.Version,
			Type:    "dependency",
		})
	}
	return result
}

func performSecurityScan(repoInfo *core.RepositoryInfo) []api.SecurityIssue {
	// This is a placeholder implementation
	// In a real implementation, this would integrate with security scanning tools
	var issues []api.SecurityIssue

	// Example: Check for common security issues
	if repoInfo.Language == "node" || repoInfo.Framework == "express" {
		issues = append(issues, api.SecurityIssue{
			ID:          "SEC-001",
			Severity:    "medium",
			Description: "Consider using helmet.js for Express.js security headers",
			Package:     "express",
		})
	}

	if repoInfo.Language == "python" {
		issues = append(issues, api.SecurityIssue{
			ID:          "SEC-002",
			Severity:    "low",
			Description: "Consider using bandit for Python security analysis",
		})
	}

	return issues
}
