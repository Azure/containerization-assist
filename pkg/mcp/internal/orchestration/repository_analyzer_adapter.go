package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// Forward declaration to avoid circular imports
type ToolFactoryInterface interface {
	CreateAnalyzeRepositoryTool() *analyze.AtomicAnalyzeRepositoryTool
}

// RepositoryAnalyzerAdapter bridges AtomicAnalyzeRepositoryTool and build.RepositoryAnalyzerInterface
// This adapter prevents circular dependencies between the analyze and build packages
type RepositoryAnalyzerAdapter struct {
	toolFactory    ToolFactoryInterface
	sessionManager mcptypes.ToolSessionManager
	logger         zerolog.Logger
}

// NewRepositoryAnalyzerAdapter creates a new repository analyzer adapter
func NewRepositoryAnalyzerAdapter(
	toolFactory ToolFactoryInterface,
	sessionManager mcptypes.ToolSessionManager,
	logger zerolog.Logger,
) *RepositoryAnalyzerAdapter {
	return &RepositoryAnalyzerAdapter{
		toolFactory:    toolFactory,
		sessionManager: sessionManager,
		logger:         logger.With().Str("component", "repository_analyzer_adapter").Logger(),
	}
}

// AnalyzeRepository analyzes a repository by delegating to AtomicAnalyzeRepositoryTool
func (r *RepositoryAnalyzerAdapter) AnalyzeRepository(ctx context.Context, repoPath string) (*build.RepositoryInfo, error) {
	r.logger.Debug().
		Str("repo_path", repoPath).
		Msg("Starting repository analysis via adapter")

	// Create the atomic analyze repository tool
	tool := r.toolFactory.CreateAnalyzeRepositoryTool()
	if tool == nil {
		return nil, fmt.Errorf("failed to create analyze repository tool")
	}

	// Create session for the analysis if needed
	session, err := r.sessionManager.GetOrCreateSession("")
	if err != nil {
		return nil, fmt.Errorf("failed to get or create session: %w", err)
	}

	// Extract session ID
	var sessionID string
	if sessionState, ok := session.(interface{ GetSessionID() string }); ok {
		sessionID = sessionState.GetSessionID()
	} else {
		r.logger.Warn().Msg("Could not extract session ID, using empty string")
		sessionID = ""
	}

	// Prepare arguments for the atomic tool
	args := analyze.AtomicAnalyzeRepositoryArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: sessionID,
		},
		RepoURL:      repoPath, // For local repos, this is the path
		Branch:       "",       // Default branch
		Context:      "adapter-analysis",
		LanguageHint: "",    // No hint
		Shallow:      false, // Full analysis
	}

	// Execute the analysis
	result, err := tool.ExecuteTyped(ctx, args)
	if err != nil {
		r.logger.Error().
			Err(err).
			Str("repo_path", repoPath).
			Msg("Repository analysis failed")
		return nil, fmt.Errorf("repository analysis failed: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("repository analysis was not successful")
	}

	// Convert AtomicAnalysisResult to build.RepositoryInfo
	repoInfo := r.convertToRepositoryInfo(result)

	r.logger.Debug().
		Str("repo_path", repoPath).
		Str("language", repoInfo.Language).
		Str("framework", repoInfo.Framework).
		Int("dependencies", len(repoInfo.Dependencies)).
		Msg("Repository analysis completed successfully")

	return repoInfo, nil
}

// GetProjectMetadata extracts project metadata from repository analysis
func (r *RepositoryAnalyzerAdapter) GetProjectMetadata(ctx context.Context, repoPath string) (*build.ProjectMetadata, error) {
	r.logger.Debug().
		Str("repo_path", repoPath).
		Msg("Getting project metadata via adapter")

	// First get the repository info
	repoInfo, err := r.AnalyzeRepository(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze repository for metadata: %w", err)
	}

	// Convert to ProjectMetadata
	metadata := r.convertToProjectMetadata(repoInfo)

	r.logger.Debug().
		Str("repo_path", repoPath).
		Str("language", metadata.Language).
		Str("build_system", metadata.BuildSystem).
		Msg("Project metadata extracted successfully")

	return metadata, nil
}

// convertToRepositoryInfo converts AtomicAnalysisResult to build.RepositoryInfo
func (r *RepositoryAnalyzerAdapter) convertToRepositoryInfo(result *analyze.AtomicAnalysisResult) *build.RepositoryInfo {
	if result == nil || result.Analysis == nil {
		r.logger.Warn().Msg("Nil result or analysis, returning empty repository info")
		return &build.RepositoryInfo{
			Language:     "unknown",
			Framework:    "unknown",
			Dependencies: []string{},
			BuildSystem:  "unknown",
			ProjectSize:  "unknown",
			Complexity:   "unknown",
			Metadata:     make(map[string]interface{}),
		}
	}

	analysis := result.Analysis

	// Extract dependencies as string slice
	dependencyNames := make([]string, 0, len(analysis.Dependencies))
	for _, dep := range analysis.Dependencies {
		dependencyNames = append(dependencyNames, dep.Name)
	}

	// Determine project size based on file count or structure
	projectSize := r.determineProjectSize(analysis)
	complexity := r.determineComplexity(analysis)

	// Extract entry point from slice
	entryPoint := ""
	if len(analysis.EntryPoints) > 0 {
		entryPoint = analysis.EntryPoints[0]
	}

	// Build metadata map with all available information
	metadata := map[string]interface{}{
		"entry_point":   entryPoint,
		"entry_points":  analysis.EntryPoints,
		"port":          analysis.Port,
		"structure":     analysis.Structure,
		"session_id":    result.SessionID,
		"workspace_dir": result.WorkspaceDir,
		"clone_dir":     result.CloneDir,
		"repo_url":      result.RepoURL,
		"branch":        result.Branch,
	}

	// Add analysis duration and other metadata if available
	if result.AnalysisDuration > 0 {
		metadata["analysis_duration"] = result.AnalysisDuration
	}
	if result.Analysis != nil && result.Analysis.Context != nil {
		for k, v := range result.Analysis.Context {
			metadata[k] = v
		}
	}

	return &build.RepositoryInfo{
		Language:     analysis.Language,
		Framework:    analysis.Framework,
		Dependencies: dependencyNames,
		BuildSystem:  r.determineBuildSystem(analysis),
		ProjectSize:  projectSize,
		Complexity:   complexity,
		Metadata:     metadata,
	}
}

// convertToProjectMetadata converts RepositoryInfo to ProjectMetadata
func (r *RepositoryAnalyzerAdapter) convertToProjectMetadata(repoInfo *build.RepositoryInfo) *build.ProjectMetadata {
	return &build.ProjectMetadata{
		Language:     repoInfo.Language,
		Framework:    repoInfo.Framework,
		Dependencies: repoInfo.Dependencies,
		BuildSystem:  repoInfo.BuildSystem,
		ProjectSize:  repoInfo.ProjectSize,
		Complexity:   repoInfo.Complexity,
		Attributes:   repoInfo.Metadata,
	}
}

// determineBuildSystem determines the build system from analysis
func (r *RepositoryAnalyzerAdapter) determineBuildSystem(analysis *analysis.AnalysisResult) string {
	if analysis == nil {
		return "unknown"
	}

	// Check language-specific build systems
	switch analysis.Language {
	case "go":
		return "go"
	case "javascript", "typescript":
		return "npm"
	case "python":
		if r.hasFile(analysis, "requirements.txt") || r.hasFile(analysis, "pyproject.toml") {
			return "pip"
		}
		return "python"
	case "java":
		if r.hasFile(analysis, "pom.xml") {
			return "maven"
		} else if r.hasFile(analysis, "build.gradle") {
			return "gradle"
		}
		return "java"
	case "rust":
		return "cargo"
	case "csharp":
		return "dotnet"
	default:
		return "make" // Generic fallback
	}
}

// determineProjectSize estimates project size from analysis
func (r *RepositoryAnalyzerAdapter) determineProjectSize(analysis *analysis.AnalysisResult) string {
	if analysis == nil || analysis.Structure == nil {
		return "unknown"
	}

	// Simple heuristic based on number of dependencies and files
	depCount := len(analysis.Dependencies)

	// Try to count files if structure information is available
	fileCount := r.estimateFileCount(analysis.Structure)

	if depCount > 50 || fileCount > 1000 {
		return "large"
	} else if depCount > 10 || fileCount > 100 {
		return "medium"
	} else {
		return "small"
	}
}

// determineComplexity estimates project complexity
func (r *RepositoryAnalyzerAdapter) determineComplexity(analysis *analysis.AnalysisResult) string {
	if analysis == nil {
		return "unknown"
	}

	// Simple heuristic based on dependencies, framework, and language
	depCount := len(analysis.Dependencies)

	complexityScore := 0

	// Add points based on dependencies
	if depCount > 20 {
		complexityScore += 3
	} else if depCount > 5 {
		complexityScore += 2
	} else if depCount > 0 {
		complexityScore += 1
	}

	// Add points for complex frameworks
	complexFrameworks := []string{"spring", "django", "rails", "angular", "react", "vue"}
	for _, framework := range complexFrameworks {
		if analysis.Framework == framework {
			complexityScore += 2
			break
		}
	}

	// Add points for compiled languages
	compiledLanguages := []string{"go", "java", "rust", "csharp", "cpp"}
	for _, lang := range compiledLanguages {
		if analysis.Language == lang {
			complexityScore += 1
			break
		}
	}

	if complexityScore >= 5 {
		return "high"
	} else if complexityScore >= 3 {
		return "medium"
	} else {
		return "low"
	}
}

// hasFile checks if a specific file exists in the analysis structure
func (r *RepositoryAnalyzerAdapter) hasFile(analysis *analysis.AnalysisResult, filename string) bool {
	if analysis == nil || analysis.Structure == nil {
		return false
	}

	// This is a simplified check - in a real implementation you'd walk the file structure
	// For now, just check if it's mentioned in the structure data
	structStr := fmt.Sprintf("%+v", analysis.Structure)
	return contains(structStr, filename)
}

// estimateFileCount estimates the number of files from structure information
func (r *RepositoryAnalyzerAdapter) estimateFileCount(structure interface{}) int {
	if structure == nil {
		return 0
	}

	// This is a placeholder implementation
	// In practice, you'd walk the actual file structure
	structStr := fmt.Sprintf("%+v", structure)

	// Simple heuristic: count common file extensions
	extensions := []string{".go", ".py", ".js", ".ts", ".java", ".rs", ".cs", ".cpp", ".h"}
	count := 0
	for _, ext := range extensions {
		count += countOccurrences(structStr, ext)
	}

	// If no files detected, return a small default
	if count == 0 {
		return 10
	}

	return count
}

// Helper functions
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func countOccurrences(s, substr string) int {
	return strings.Count(s, substr)
}
