package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/tools"
	toolstypes "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
)

// AtomicAnalysisResult represents the result of atomic analysis
type AtomicAnalysisResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult
	SessionID        string               `json:"session_id"`
	WorkspaceDir     string               `json:"workspace_dir"`
	RepositoryPath   string               `json:"repository_path"`
	AnalysisResult   *core.AnalysisResult `json:"analysis_result"`
	AnalysisDuration time.Duration        `json:"analysis_duration"`
	TotalDuration    time.Duration        `json:"total_duration"`
	AnalysisContext  interface{}          `json:"analysis_context,omitempty"`
}

// TypedAnalyzeRepositoryTool implements a type-safe analyze repository tool
// It implements api.Tool interface
type TypedAnalyzeRepositoryTool struct {
	atomicTool     *AtomicAnalyzeRepositoryTool
	sessionManager session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore   services.SessionStore         // Modern service interface
	sessionState   services.SessionState         // Modern service interface
	logger         zerolog.Logger
}

// NewTypedAnalyzeRepositoryTool creates a new type-safe analyze repository tool (legacy constructor)
// Updated to use the new simplified Tool interface from BETA workstream
func NewTypedAnalyzeRepositoryTool(
	atomicTool *AtomicAnalyzeRepositoryTool,
	sessionManager session.UnifiedSessionManager,
	logger zerolog.Logger,
) api.Tool {
	return &TypedAnalyzeRepositoryTool{
		atomicTool:     atomicTool,
		sessionManager: sessionManager,
		logger:         logger.With().Str("tool", "typed_analyze_repository").Logger(),
	}
}

// NewTypedAnalyzeRepositoryToolWithServices creates a new type-safe analyze repository tool using service interfaces
func NewTypedAnalyzeRepositoryToolWithServices(
	atomicTool *AtomicAnalyzeRepositoryTool,
	serviceContainer services.ServiceContainer,
	logger zerolog.Logger,
) api.Tool {
	toolLogger := logger.With().Str("tool", "typed_analyze_repository").Logger()

	return &TypedAnalyzeRepositoryTool{
		atomicTool:   atomicTool,
		sessionStore: serviceContainer.SessionStore(),
		sessionState: serviceContainer.SessionState(),
		logger:       toolLogger,
	}
}

// Execute implements api.Tool interface
func (t *TypedAnalyzeRepositoryTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract params from ToolInput
	var analyzeParams *tools.AnalyzeToolParams
	if rawParams, ok := input.Data["params"]; ok {
		if typedParams, ok := rawParams.(*tools.AnalyzeToolParams); ok {
			analyzeParams = typedParams
		} else {
			return api.ToolOutput{
					Success: false,
					Error:   "Invalid input type for analyze tool",
				}, errors.NewError().
					Code(errors.CodeInvalidParameter).
					Message("Invalid input type for analyze tool").
					Type(errors.ErrTypeValidation).
					Severity(errors.SeverityHigh).
					Context("tool", "typed_analyze_repository").
					Context("operation", "type_assertion").
					Build()
		}
	} else {
		return api.ToolOutput{
				Success: false,
				Error:   "No params provided",
			}, errors.NewError().
				Code(errors.CodeInvalidParameter).
				Message("No params provided").
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityHigh).
				Build()
	}

	// Use session ID from input if available
	if input.SessionID != "" {
		analyzeParams.SessionID = input.SessionID
	}

	startTime := time.Now()

	// Validate parameters at compile time
	if err := analyzeParams.Validate(); err != nil {
		return api.ToolOutput{
				Success: false,
				Error:   "Analysis parameters validation failed",
			}, errors.NewError().
				Code(errors.CodeInvalidParameter).
				Message("Analysis parameters validation failed").
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Cause(err).
				Context("repository_path", analyzeParams.RepositoryPath).
				Context("session_id", analyzeParams.SessionID).
				Suggestion("Check that repository_path and session_id are provided and valid").
				WithLocation().
				Build()
	}

	t.logger.Info().
		Str("session_id", analyzeParams.SessionID).
		Str("repository_path", analyzeParams.RepositoryPath).
		Str("branch", analyzeParams.Branch).
		Msg("Starting type-safe repository analysis")

	// Convert to typed parameters for the atomic tool
	typedParams := toolstypes.AtomicAnalyzeRepositoryParams{
		SessionParams: toolstypes.SessionParams{
			SessionID: analyzeParams.SessionID,
		},
		RepoURL:      analyzeParams.RepositoryPath,
		Branch:       getBranchOrDefault(analyzeParams.Branch),
		Context:      "",    // No additional context from legacy params
		LanguageHint: "",    // No language hint from legacy params
		Shallow:      false, // Default to full clone
	}

	// Execute using atomic tool interface
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
	rawResult, err := t.atomicTool.Execute(context.Background(), toolInput)
	if err != nil {
		return api.ToolOutput{
				Success: false,
				Error:   "Repository analysis execution failed",
			}, errors.NewError().
				Code(errors.CodeToolExecutionFailed).
				Message("Repository analysis execution failed").
				Type(errors.ErrTypeInternal).
				Severity(errors.SeverityHigh).
				Cause(err).
				Context("repository_path", analyzeParams.RepositoryPath).
				Context("session_id", analyzeParams.SessionID).
				Suggestion("Check repository accessibility and path validity").
				WithLocation().
				Build()
	}

	// Extract result data from ToolOutput
	if !rawResult.Success {
		return api.ToolOutput{
			Success: false,
			Error:   rawResult.Error,
		}, errors.NewError().Message("atomic tool execution failed").Build()
	}

	// Create a mock typed result from the tool output data
	typedResult := &toolstypes.AtomicAnalyzeRepositoryResult{
		BaseToolResponse: types.BaseToolResponse{
			Success: rawResult.Success,
		},
		SessionID:    extractStringFromToolOutput(rawResult.Data, "session_id"),
		WorkspaceDir: extractStringFromToolOutput(rawResult.Data, "workspace_dir"),
	}

	atomicResult := &AtomicAnalysisResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   true,
			Message:   "Repository analysis completed successfully",
			Timestamp: time.Now(),
		},
		SessionID:      typedResult.SessionID,
		WorkspaceDir:   typedResult.WorkspaceDir,
		RepositoryPath: extractStringFromToolOutput(rawResult.Data, "clone_dir"),
		TotalDuration:  time.Since(startTime),
		AnalysisContext: &AnalysisContext{
			FilesAnalyzed: 0, // Default value since this field may not exist
		},
	}

	// Check if analysis was successful
	if !atomicResult.BaseToolResponse.Success {
		return api.ToolOutput{
				Success: false,
				Error:   "Repository analysis failed",
			}, errors.NewError().
				Code(errors.CodeToolExecutionFailed).
				Message("Repository analysis failed").
				Type(errors.ErrTypeInternal).
				Severity(errors.SeverityHigh).
				Context("repository_path", analyzeParams.RepositoryPath).
				Context("session_id", analyzeParams.SessionID).
				Suggestion("Check repository accessibility and analysis configuration").
				WithLocation().
				Build()
	}

	// Convert to typed result format
	result := tools.AnalyzeToolResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   atomicResult.BaseToolResponse.Success,
			Message:   "Repository analysis completed successfully",
			Timestamp: time.Now(),
		},
		RepositoryPath: analyzeParams.RepositoryPath,
		AnalysisTime:   time.Since(startTime),
	}

	// Add repository information if available from tool output
	result.RepositoryInfo = &core.RepositoryInfo{
		Path:           analyzeParams.RepositoryPath,
		Name:           filepath.Base(analyzeParams.RepositoryPath),
		Language:       extractStringFromToolOutput(rawResult.Data, "language"),
		Framework:      extractStringFromToolOutput(rawResult.Data, "framework"),
		Dependencies:   extractStringSliceFromToolOutput(rawResult.Data, "dependencies"),
		BuildTool:      extractStringFromToolOutput(rawResult.Data, "build_tool"),
		PackageManager: extractStringFromToolOutput(rawResult.Data, "package_manager"),
		Metadata:       make(map[string]string),
	}
	result.FilesAnalyzed = extractIntFromToolOutput(rawResult.Data, "files_analyzed")

	// Add build recommendations if requested
	if analyzeParams.IncludeBuildRecommendations {
		result.BuildRecommendations = &core.BuildRecommendations{
			OptimizationSuggestions: []core.Recommendation{
				{Type: "optimization", Priority: 1, Title: "Use multi-stage builds", Description: "Reduce image size with multi-stage builds", Action: "implement_multistage", Metadata: make(map[string]string)},
				{Type: "optimization", Priority: 2, Title: "Optimize layer caching", Description: "Arrange Dockerfile for better layer caching", Action: "optimize_caching", Metadata: make(map[string]string)},
			},
			SecurityRecommendations: []core.Recommendation{
				{Type: "security", Priority: 1, Title: "Use non-root user", Description: "Run container as non-root user", Action: "add_user", Metadata: make(map[string]string)},
				{Type: "security", Priority: 2, Title: "Pin base image versions", Description: "Use specific image tags instead of 'latest'", Action: "pin_versions", Metadata: make(map[string]string)},
			},
		}
	}

	// Add security analysis if requested
	if analyzeParams.IncludeSecurityAnalysis {
		result.SecurityAnalysis = &tools.SecurityAnalysisResult{
			SecretsFound:         0, // Would come from actual security scan
			VulnerabilitiesFound: 0, // Would come from actual security scan
			SecurityIssues:       []string{},
			Recommendations:      []string{},
		}
	}

	// Add dependency analysis if requested
	if analyzeParams.IncludeDependencyAnalysis {
		deps := make(map[string]string)
		for _, dep := range result.RepositoryInfo.Dependencies {
			deps[dep] = "latest" // Default version since we don't have version info
		}
		result.DependencyAnalysis = &tools.DependencyAnalysisResult{
			TotalDependencies:      len(result.RepositoryInfo.Dependencies),
			OutdatedDependencies:   0,
			VulnerableDependencies: 0,
			Dependencies:           deps,
			Updates:                make(map[string]string),
			SecurityAdvisories:     []string{},
		}
	}

	t.logger.Info().
		Str("session_id", analyzeParams.SessionID).
		Bool("success", result.Success).
		Dur("duration", result.AnalysisTime).
		Msg("Completed type-safe repository analysis")

	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": &result},
	}, nil
}

// Name implements api.Tool interface
func (t *TypedAnalyzeRepositoryTool) Name() string {
	return "typed_analyze_repository"
}

// Description implements api.Tool interface
func (t *TypedAnalyzeRepositoryTool) Description() string {
	return "Type-safe repository analysis tool for containerization planning"
}

// Schema implements api.Tool interface
func (t *TypedAnalyzeRepositoryTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "typed_analyze_repository",
		Description: "Type-safe repository analysis tool for containerization planning",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"params": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"session_id": map[string]interface{}{
							"type":        "string",
							"description": "Session identifier",
						},
						"repository_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to the repository to analyze",
						},
						"branch": map[string]interface{}{
							"type":        "string",
							"description": "Git branch to analyze",
						},
						"include_build_recommendations": map[string]interface{}{
							"type":        "boolean",
							"description": "Include build recommendations",
						},
						"include_security_analysis": map[string]interface{}{
							"type":        "boolean",
							"description": "Include security analysis",
						},
						"include_dependency_analysis": map[string]interface{}{
							"type":        "boolean",
							"description": "Include dependency analysis",
						},
					},
					"required": []string{"session_id", "repository_path"},
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the analysis was successful",
						},
						"repository_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to the analyzed repository",
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
			},
		},
	}
}

// GetMetadata returns tool metadata
func (t *TypedAnalyzeRepositoryTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:        "typed_analyze_repository",
		Description: "Type-safe repository analysis tool for containerization planning",
		Version:     "1.0.0",
		Category:    api.ToolCategory("analysis"),
		Tags:        []string{"analysis", "repository", "typed"},
		Status:      api.ToolStatus("active"),
		Capabilities: []string{
			"repository_analysis",
			"language_detection",
			"dependency_analysis",
			"build_recommendations",
		},
		Requirements: []string{
			"git",
			"filesystem_access",
		},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// Validate implements GenericTool interface
func (t *TypedAnalyzeRepositoryTool) Validate(ctx context.Context, params tools.AnalyzeToolParams) error {
	return params.Validate()
}

// Helper functions

func getBranchOrDefault(branch string) string {
	if branch == "" {
		return "main"
	}
	return branch
}

func getAnalysisMessage(result *AtomicAnalysisResult) string {
	if !result.BaseToolResponse.Success {
		return "Repository analysis failed"
	}
	return fmt.Sprintf("Successfully analyzed repository: %s", result.RepositoryPath)
}

func convertToRepositoryInfo(analysisResult *analysis.AnalysisResult) *core.RepositoryInfo {
	if analysisResult == nil {
		return nil
	}

	// Build dependencies map
	dependencies := make(map[string]string)
	for _, dep := range analysisResult.Dependencies {
		dependencies[dep.Name] = dep.Version
	}

	// Extract languages - using primary language detected
	languages := []string{}
	if analysisResult.Language != "" {
		languages = append(languages, analysisResult.Language)
	}

	// Extract build tools from config files
	buildTools := extractBuildTools(analysisResult.ConfigFiles)

	// These variables are not used in the new RepositoryInfo struct
	_ = checkForDockerfile(analysisResult.ConfigFiles)
	_ = ""
	if len(analysisResult.EntryPoints) > 0 {
		_ = analysisResult.EntryPoints[0]
	}

	return &core.RepositoryInfo{
		Path:           "", // Path not available in AnalysisResult
		Name:           "repository",
		Language:       analysisResult.Language,
		Framework:      analysisResult.Framework,
		Dependencies:   convertDepsToStringSlice(analysisResult.Dependencies),
		BuildTool:      getBuildToolFromList(buildTools),
		PackageManager: getPackageManagerFromDeps(analysisResult.Dependencies),
		Metadata:       make(map[string]string),
	}
}

func convertToBuildRecommendations(analysisResult *analysis.AnalysisResult) *core.BuildRecommendations {
	if analysisResult == nil {
		return nil
	}

	// Extract recommendations from analysis result
	return &core.BuildRecommendations{
		OptimizationSuggestions: []core.Recommendation{
			{Type: "optimization", Priority: 1, Title: "Use multi-stage builds", Description: "Reduce image size", Action: "implement", Metadata: make(map[string]string)},
			{Type: "optimization", Priority: 2, Title: "Optimize layer caching", Description: "Improve build speed", Action: "optimize", Metadata: make(map[string]string)},
		},
		SecurityRecommendations: []core.Recommendation{
			{Type: "security", Priority: 1, Title: "Use non-root user", Description: "Improve security", Action: "add_user", Metadata: make(map[string]string)},
			{Type: "security", Priority: 2, Title: "Use minimal base image", Description: "Reduce attack surface", Action: "change_base", Metadata: make(map[string]string)},
		},
	}
}

func convertToDependencyAnalysis(analysisResult *analysis.AnalysisResult) *tools.DependencyAnalysisResult {
	if analysisResult == nil {
		return nil
	}

	dependencies := make(map[string]string)
	for _, dep := range analysisResult.Dependencies {
		dependencies[dep.Name] = dep.Version
	}

	return &tools.DependencyAnalysisResult{
		TotalDependencies:      len(analysisResult.Dependencies),
		OutdatedDependencies:   0, // Would need actual version checking
		VulnerableDependencies: 0, // Would need vulnerability database
		Dependencies:           dependencies,
		Updates:                make(map[string]string),
		SecurityAdvisories:     []string{},
	}
}

// Helper functions for data extraction

func extractBuildTools(configFiles []analysis.ConfigFile) []string {
	buildTools := []string{}
	for _, file := range configFiles {
		switch file.Type {
		case "package":
			if strings.Contains(file.Path, "package.json") {
				buildTools = append(buildTools, "npm")
			} else if strings.Contains(file.Path, "pom.xml") {
				buildTools = append(buildTools, "maven")
			} else if strings.Contains(file.Path, "build.gradle") {
				buildTools = append(buildTools, "gradle")
			}
		case "build":
			if strings.Contains(file.Path, "Makefile") {
				buildTools = append(buildTools, "make")
			} else if strings.Contains(file.Path, "Dockerfile") {
				buildTools = append(buildTools, "docker")
			}
		}
	}
	return buildTools
}

func checkForDockerfile(configFiles []analysis.ConfigFile) bool {
	for _, file := range configFiles {
		if strings.Contains(file.Path, "Dockerfile") {
			return true
		}
	}
	return false
}

// Helper functions for extracting data from tool output

func extractStringSliceFromToolOutput(data map[string]interface{}, key string) []string {
	if value, ok := data[key].([]interface{}); ok {
		result := make([]string, len(value))
		for i, v := range value {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return []string{}
}


func convertDepsToStringSlice(deps []analysis.Dependency) []string {
	result := make([]string, len(deps))
	for i, dep := range deps {
		result[i] = dep.Name
	}
	return result
}

func getBuildToolFromList(buildTools []string) string {
	if len(buildTools) > 0 {
		return buildTools[0]
	}
	return "unknown"
}

func getPackageManagerFromDeps(deps []analysis.Dependency) string {
	for _, dep := range deps {
		if dep.Manager != "" {
			return dep.Manager
		}
	}
	return "unknown"
}
