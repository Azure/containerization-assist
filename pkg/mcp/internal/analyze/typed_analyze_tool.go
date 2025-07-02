package analyze

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
	"github.com/rs/zerolog"
)

// TypedAnalyzeRepositoryTool implements a type-safe analyze repository tool
type TypedAnalyzeRepositoryTool struct {
	atomicTool     *AtomicAnalyzeRepositoryTool
	sessionManager core.ToolSessionManager
	logger         zerolog.Logger
}

// NewTypedAnalyzeRepositoryTool creates a new type-safe analyze repository tool
func NewTypedAnalyzeRepositoryTool(
	atomicTool *AtomicAnalyzeRepositoryTool,
	sessionManager core.ToolSessionManager,
	logger zerolog.Logger,
) core.GenericTool[tools.AnalyzeToolParams, tools.AnalyzeToolResult] {
	return &TypedAnalyzeRepositoryTool{
		atomicTool:     atomicTool,
		sessionManager: sessionManager,
		logger:         logger.With().Str("tool", "typed_analyze_repository").Logger(),
	}
}

// Execute implements GenericTool[AnalyzeToolParams, AnalyzeToolResult]
func (t *TypedAnalyzeRepositoryTool) Execute(ctx context.Context, params tools.AnalyzeToolParams) (tools.AnalyzeToolResult, error) {
	startTime := time.Now()

	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return tools.AnalyzeToolResult{}, rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Analysis parameters validation failed").
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Cause(err).
			Context("repository_path", params.RepositoryPath).
			Context("session_id", params.SessionID).
			Suggestion("Check that repository_path and session_id are provided and valid").
			WithLocation().
			Build()
	}

	t.logger.Info().
		Str("session_id", params.SessionID).
		Str("repository_path", params.RepositoryPath).
		Str("branch", params.Branch).
		Msg("Starting type-safe repository analysis")

	// Convert to atomic tool args format
	atomicArgs := AtomicAnalyzeRepositoryArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: params.SessionID,
		},
		RepoURL: params.RepositoryPath,
		Branch:  getBranchOrDefault(params.Branch),
	}

	// Execute using atomic tool
	resultInterface, err := t.atomicTool.Execute(ctx, atomicArgs)
	if err != nil {
		return tools.AnalyzeToolResult{}, rich.NewError().
			Code(rich.CodeToolExecutionFailed).
			Message("Repository analysis execution failed").
			Type(rich.ErrTypeInternal).
			Severity(rich.SeverityHigh).
			Cause(err).
			Context("repository_path", params.RepositoryPath).
			Context("session_id", params.SessionID).
			Suggestion("Check repository accessibility and path validity").
			WithLocation().
			Build()
	}

	// Type-safe conversion from atomic result
	atomicResult, ok := resultInterface.(*AtomicAnalysisResult)
	if !ok {
		return tools.AnalyzeToolResult{}, rich.NewError().
			Code(rich.CodeTypeConversionFailed).
			Message("Unexpected result type from atomic analysis tool").
			Type(rich.ErrTypeInternal).
			Severity(rich.SeverityHigh).
			Context("expected_type", "*AtomicAnalysisResult").
			Context("actual_type", fmt.Sprintf("%T", resultInterface)).
			Suggestion("This is an internal error - please report this issue").
			WithLocation().
			Build()
	}

	// Convert to typed result format
	result := tools.AnalyzeToolResult{
		BaseToolResponse: core.BaseToolResponse{
			Success:   atomicResult.Success,
			Message:   getAnalysisMessage(atomicResult),
			Timestamp: time.Now(),
		},
		RepositoryPath: params.RepositoryPath,
		AnalysisTime:   time.Since(startTime),
	}

	// Add repository information if available
	if atomicResult.Analysis != nil {
		result.RepositoryInfo = convertToRepositoryInfo(atomicResult.Analysis)
		result.FilesAnalyzed = len(atomicResult.Analysis.ConfigFiles) // Use config files count as proxy for files analyzed
	}

	// Add build recommendations if requested and available
	if params.IncludeBuildRecommendations && atomicResult.Analysis != nil {
		result.BuildRecommendations = convertToBuildRecommendations(atomicResult.Analysis)
	}

	// Add security analysis if requested
	if params.IncludeSecurityAnalysis {
		result.SecurityAnalysis = &tools.SecurityAnalysisResult{
			SecretsFound:         0, // Would come from actual security scan
			VulnerabilitiesFound: 0, // Would come from actual security scan
			SecurityIssues:       []string{},
			Recommendations:      []string{},
		}
	}

	// Add dependency analysis if requested
	if params.IncludeDependencyAnalysis && atomicResult.Analysis != nil {
		result.DependencyAnalysis = convertToDependencyAnalysis(atomicResult.Analysis)
	}

	t.logger.Info().
		Str("session_id", params.SessionID).
		Bool("success", result.Success).
		Dur("duration", result.AnalysisTime).
		Msg("Completed type-safe repository analysis")

	return result, nil
}

// GetMetadata implements GenericTool interface
func (t *TypedAnalyzeRepositoryTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:        "typed_analyze_repository",
		Description: "Type-safe repository analysis tool for containerization planning",
		Version:     "1.0.0",
		Category:    "analysis",
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
		Parameters: map[string]string{
			"session_id":                    "Session identifier",
			"repository_path":               "Path or URL to repository",
			"branch":                        "Branch to analyze (optional)",
			"include_build_recommendations": "Include build recommendations",
			"include_security_analysis":     "Include security analysis",
			"include_dependency_analysis":   "Include dependency analysis",
		},
		Examples: []core.ToolExample{
			{
				Name:        "Basic Repository Analysis",
				Description: "Analyze a local repository",
				Input: map[string]interface{}{
					"session_id":      "session_123",
					"repository_path": "/path/to/repo",
				},
				Output: map[string]interface{}{
					"success":         true,
					"repository_info": "analysis_data",
				},
			},
		},
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
