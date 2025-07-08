package analyze

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	core "github.com/Azure/container-kit/pkg/mcp/core/tools"
	toolstypes "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
)

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

	// Execute using typed atomic tool interface
	rawResult, err := t.atomicTool.ExecuteTypedInterface(context.Background(), typedParams)
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

	// Type assert the result to the expected type
	typedResult, ok := rawResult.(*AtomicAnalysisResult)
	if !ok {
		return api.ToolOutput{
				Success: false,
				Error:   "Invalid result type from atomic tool",
			}, errors.NewError().Messagef("invalid result type from atomic tool: expected *AtomicAnalysisResult, got %T", rawResult).WithLocation(

			// Convert typed result for compatibility
			).Build()
	}

	atomicResult := &AtomicAnalysisResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   true,
			Message:   "Repository analysis completed successfully",
			Timestamp: time.Now(),
		},
		Success:       typedResult.Success,
		SessionID:     typedResult.SessionID,
		WorkspaceDir:  typedResult.WorkspaceDir,
		RepoURL:       typedResult.RepoURL,
		Branch:        typedResult.Branch,
		CloneDir:      typedResult.CloneDir,
		TotalDuration: typedResult.TotalDuration,
		AnalysisContext: &AnalysisContext{
			FilesAnalyzed: 0, // Default value since this field may not exist
		},
	}

	// Check if analysis was successful
	if !atomicResult.Success {
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
			Success:   atomicResult.Success,
			Message:   getAnalysisMessage(atomicResult),
			Timestamp: time.Now(),
		},
		RepositoryPath: analyzeParams.RepositoryPath,
		AnalysisTime:   time.Since(startTime),
	}

	// Add repository information if available
	if atomicResult.Analysis != nil {
		result.RepositoryInfo = convertToRepositoryInfo(atomicResult.Analysis)
		result.FilesAnalyzed = len(atomicResult.Analysis.ConfigFiles) // Use config files count as proxy for files analyzed
	}

	// Add build recommendations if requested and available
	if analyzeParams.IncludeBuildRecommendations && atomicResult.Analysis != nil {
		result.BuildRecommendations = convertToBuildRecommendations(atomicResult.Analysis)
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
	if analyzeParams.IncludeDependencyAnalysis && atomicResult.Analysis != nil {
		result.DependencyAnalysis = convertToDependencyAnalysis(atomicResult.Analysis)
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
	if !result.Success {
		return "Repository analysis failed"
	}
	return fmt.Sprintf("Successfully analyzed repository: %s", result.RepoURL)
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

	// Check for dockerfile
	hasDockerfile := checkForDockerfile(analysisResult.ConfigFiles)

	// Get entry point
	entryPoint := ""
	if len(analysisResult.EntryPoints) > 0 {
		entryPoint = analysisResult.EntryPoints[0]
	}

	return &core.RepositoryInfo{
		Path:          "", // Path not available in AnalysisResult
		Type:          "git",
		Language:      analysisResult.Language,
		Languages:     languages,
		Framework:     analysisResult.Framework,
		Dependencies:  dependencies,
		BuildTools:    buildTools,
		EntryPoint:    entryPoint,
		Port:          analysisResult.Port,
		HasDockerfile: hasDockerfile,
		Metadata:      make(map[string]string), // Convert to string map for type safety
	}
}

func convertToBuildRecommendations(analysisResult *analysis.AnalysisResult) *core.BuildRecommendations {
	if analysisResult == nil {
		return nil
	}

	// Extract recommendations from analysis result
	return &core.BuildRecommendations{
		OptimizationTips: analysisResult.Suggestions,
		SecurityTips:     []string{"Use non-root user", "Use minimal base image", "Scan for vulnerabilities"},
		PerformanceTips:  []string{"Use multi-stage builds", "Optimize layer caching", "Minimize image size"},
		BestPractices:    []string{"Use .dockerignore", "Pin base image versions", "Set health checks"},
		Suggestions:      make(map[string]string), // Type-safe string map
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
