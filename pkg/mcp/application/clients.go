package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	"github.com/Azure/container-kit/pkg/mcp/domain"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// Local interface definitions to avoid import cycles

// NOTE: AIAnalyzer and TokenUsage moved to analysis_types.go to avoid redeclaration

// Analysis-related types for backward compatibility (local definitions to avoid import cycles)

// ProgressCallback is called during long-running operations to report progress
type ProgressCallback func(status string, current int, total int)

// AnalysisService provides analysis operations
type AnalysisService interface {
	AnalyzeRepository(ctx context.Context, path string, callback ProgressCallback) (*RepositoryAnalysis, error)
	AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error)
	GetAnalysisProgress(ctx context.Context, analysisID string) (*AnalysisProgress, error)
}

// RepositoryAnalysis represents the result of analyzing a repository
type RepositoryAnalysis struct {
	Language        string                 `json:"language"`
	Framework       string                 `json:"framework"`
	Dependencies    []string               `json:"dependencies"`
	EntryPoint      string                 `json:"entry_point"`
	Port            int                    `json:"port"`
	BuildCommand    string                 `json:"build_command"`
	RunCommand      string                 `json:"run_command"`
	Issues          []AnalysisIssue        `json:"issues"`
	Recommendations []string               `json:"recommendations"`
	Metadata        map[string]interface{} `json:"metadata"`
	Structure       map[string]interface{} `json:"structure"`
	Metrics         map[string]float64     `json:"metrics"`
	Suggestions     []string               `json:"suggestions"`
}

// AnalysisIssue represents an issue found during analysis
type AnalysisIssue struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Suggestion string `json:"suggestion"`
}

// AIAnalysis represents the result of AI-powered analysis
type AIAnalysis struct {
	Summary         string                 `json:"summary"`
	Insights        []string               `json:"insights"`
	Recommendations []string               `json:"recommendations"`
	Confidence      float64                `json:"confidence"`
	Metadata        map[string]interface{} `json:"metadata"`
	Analysis        map[string]interface{} `json:"analysis"`
}

// AnalysisProgress represents the progress of an ongoing analysis
type AnalysisProgress struct {
	ID       string   `json:"id"`
	Stage    string   `json:"stage"`
	Progress int      `json:"progress"`
	Total    int      `json:"total"`
	Complete bool     `json:"complete"`
	Messages []string `json:"messages"`
}

// MCPClients provides MCP-specific clients without external AI dependencies
// This replaces pkg/clients.Clients for MCP usage to ensure no AI dependencies
type MCPClients struct {
	Docker   docker.DockerClient
	Kind     kind.KindRunner
	Kube     k8s.KubeRunner
	Analyzer AnalysisService // Always use stub or caller analyzer - never external AI
}

// NewMCPClients creates MCP-specific clients with stub analyzer
func NewMCPClients(docker docker.DockerClient, kind kind.KindRunner, kube k8s.KubeRunner) *MCPClients {
	return &MCPClients{
		Docker:   docker,
		Kind:     kind,
		Kube:     kube,
		Analyzer: &stubAnalyzer{}, // Default to stub - no external AI
	}
}

// NewMCPClientsWithAnalyzer creates MCP-specific clients with a specific analyzer
func NewMCPClientsWithAnalyzer(docker docker.DockerClient, kind kind.KindRunner, kube k8s.KubeRunner, analyzer AnalysisService) *MCPClients {
	return &MCPClients{
		Docker:   docker,
		Kind:     kind,
		Kube:     kube,
		Analyzer: analyzer,
	}
}

// Note: Analyzer field is exported for direct access
// Use mc.Analyzer = analyzer instead of SetAnalyzer(analyzer)

// ValidateAnalyzerForProduction ensures the analyzer is appropriate for production
func (mc *MCPClients) ValidateAnalyzerForProduction(logger zerolog.Logger) error {
	if mc.Analyzer == nil {
		return errors.NewError().Messagef("analyzer cannot be nil").WithLocation(

		// In production, we should never use external AI analyzers
		// Only stub or caller analyzers are allowed
		).Build()
	}

	analyzerType := fmt.Sprintf("%T", mc.Analyzer)
	logger.Debug().Str("analyzer_type", analyzerType).Msg("Validating analyzer for production")

	// Check for known safe analyzer types
	switch analyzerType {
	case "*core.stubAnalyzer", "*analyze.StubAnalyzer", "*analyze.CallerAnalyzer":
		logger.Info().Str("analyzer_type", analyzerType).Msg("Using safe analyzer for production")
		return nil
	default:
		logger.Warn().Str("analyzer_type", analyzerType).Msg("Unknown analyzer type - may not be safe for production")
		return errors.NewError().Messagef("analyzer type %s may not be safe for production", analyzerType).WithLocation(

		// stubAnalyzer is a local stub implementation to avoid import cycles
		).Build()
	}
}

type stubAnalyzer struct{}

// Analyze returns a basic stub response
func (s *stubAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "stub analysis result", nil
}

// AnalyzeWithFileTools returns a basic stub response
func (s *stubAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "stub analysis result", nil
}

// AnalyzeWithFormat returns a basic stub response
func (s *stubAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "stub analysis result", nil
}

// GetTokenUsage returns empty usage
func (s *stubAnalyzer) GetTokenUsage() domain.TokenUsage {
	return domain.TokenUsage{}
}

// ResetTokenUsage does nothing for stub
func (s *stubAnalyzer) ResetTokenUsage() {
}

// AnalyzeRepository implements AnalysisService interface
func (s *stubAnalyzer) AnalyzeRepository(ctx context.Context, path string, callback ProgressCallback) (*RepositoryAnalysis, error) {
	// Progress callback
	if callback != nil {
		callback("starting", 0, 100)
	}

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path)
	}

	result := &RepositoryAnalysis{
		Language:     detectPrimaryLanguage(path),
		Framework:    detectFramework(path),
		Dependencies: []string{},
		Structure:    make(map[string]interface{}),
		Metrics:      make(map[string]float64),
		Issues:       []AnalysisIssue{},
		Suggestions:  []string{},
	}

	// Analyze repository structure
	err = analyzeDirectory(ctx, path, result, callback)
	if err != nil {
		return nil, err
	}

	if callback != nil {
		callback("completed", 100, 100)
	}

	return result, nil
}

// AnalyzeWithAI implements AnalysisService interface
func (s *stubAnalyzer) AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error) {
	// For now, provide basic analysis without actual AI
	// This can be enhanced later with real AI integration

	analysis := &AIAnalysis{
		Summary:         "Code analysis completed",
		Recommendations: []string{},
		Confidence:      0.8,
		Analysis:        make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
	}

	// Basic content analysis
	lines := strings.Split(content, "\n")
	analysis.Metadata["line_count"] = len(lines)

	// Simple recommendations based on content
	if strings.Contains(content, "TODO") {
		analysis.Recommendations = append(analysis.Recommendations, "Complete TODO items")
	}
	if strings.Contains(content, "FIXME") {
		analysis.Recommendations = append(analysis.Recommendations, "Address FIXME comments")
	}

	return analysis, nil
}

// GetAnalysisProgress implements AnalysisService interface
func (s *stubAnalyzer) GetAnalysisProgress(ctx context.Context, analysisID string) (*AnalysisProgress, error) {
	// Simple implementation - in real system would track actual progress
	return &AnalysisProgress{
		ID:       analysisID,
		Stage:    "complete",
		Progress: 100,
		Total:    100,
		Complete: true,
		Messages: []string{"Analysis completed"},
	}, nil
}

// Helper function to detect primary language
func detectPrimaryLanguage(path string) string {
	// Check for language-specific files
	if exists(filepath.Join(path, "go.mod")) {
		return "go"
	}
	if exists(filepath.Join(path, "package.json")) {
		return "javascript"
	}
	if exists(filepath.Join(path, "requirements.txt")) || exists(filepath.Join(path, "setup.py")) {
		return "python"
	}
	if exists(filepath.Join(path, "pom.xml")) {
		return "java"
	}
	return "unknown"
}

// Helper function to detect framework
func detectFramework(path string) string {
	lang := detectPrimaryLanguage(path)

	switch lang {
	case "go":
		if exists(filepath.Join(path, "main.go")) {
			return "cli"
		}
		if exists(filepath.Join(path, "go.mod")) {
			content, _ := os.ReadFile(filepath.Join(path, "go.mod"))
			if strings.Contains(string(content), "gin-gonic/gin") {
				return "gin"
			}
			if strings.Contains(string(content), "gorilla/mux") {
				return "mux"
			}
		}
	case "javascript":
		if exists(filepath.Join(path, "package.json")) {
			content, _ := os.ReadFile(filepath.Join(path, "package.json"))
			if strings.Contains(string(content), "react") {
				return "react"
			}
			if strings.Contains(string(content), "express") {
				return "express"
			}
		}
	}

	return "none"
}

// Helper function to check if file exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// analyzeDirectory performs directory analysis
func analyzeDirectory(ctx context.Context, path string, result *RepositoryAnalysis, callback ProgressCallback) error {
	// Check context for cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	if callback != nil {
		callback("scanning files", 25, 100)
	}

	// Count files and directories
	fileCount := 0
	dirCount := 0
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, don't fail the whole analysis
		}
		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to analyze directory structure: %w", err)
	}

	if callback != nil {
		callback("analyzing structure", 50, 100)
	}

	// Set basic metrics
	result.Metrics["file_count"] = float64(fileCount)
	result.Metrics["directory_count"] = float64(dirCount)

	// Set basic structure info
	result.Structure["files"] = fileCount
	result.Structure["directories"] = dirCount
	result.Structure["analyzed_path"] = path

	if callback != nil {
		callback("generating suggestions", 75, 100)
	}

	// Add basic suggestions based on language
	switch result.Language {
	case "go":
		result.Suggestions = append(result.Suggestions, "Consider adding go.sum for dependency verification")
		if !exists(filepath.Join(path, "README.md")) {
			result.Suggestions = append(result.Suggestions, "Add a README.md file for documentation")
		}
	case "javascript":
		result.Suggestions = append(result.Suggestions, "Consider adding package-lock.json for dependency locking")
		if !exists(filepath.Join(path, ".gitignore")) {
			result.Suggestions = append(result.Suggestions, "Add a .gitignore file")
		}
	}

	return nil
}

// NOTE: AIAnalyzer and TokenUsage moved to analysis_types.go to avoid redeclaration
