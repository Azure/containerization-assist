package commands

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ConsolidatedAnalyzeCommand consolidates all analyze tool functionality into a single command
// This replaces the 44 files in pkg/mcp/tools/analyze/ with a unified implementation
type ConsolidatedAnalyzeCommand struct {
	sessionStore   services.SessionStore
	sessionState   services.SessionState
	logger         *slog.Logger
	analysisEngine *analysis.Engine
}

// NewConsolidatedAnalyzeCommand creates a new consolidated analyze command
func NewConsolidatedAnalyzeCommand(
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	logger *slog.Logger,
	analysisEngine *analysis.Engine,
) *ConsolidatedAnalyzeCommand {
	return &ConsolidatedAnalyzeCommand{
		sessionStore:   sessionStore,
		sessionState:   sessionState,
		logger:         logger,
		analysisEngine: analysisEngine,
	}
}

// Execute performs repository analysis with full functionality from original tools
func (cmd *ConsolidatedAnalyzeCommand) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Extract and validate input parameters
	analysisRequest, err := cmd.parseAnalysisInput(input)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("failed to parse analysis input").
			Cause(err).
			Build()
	}

	// Validate using domain rules
	if validationErrors := cmd.validateAnalysisRequest(analysisRequest); len(validationErrors) > 0 {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("analysis request validation failed").
			Context("validation_errors", validationErrors).
			Build()
	}

	// Get workspace directory for the session
	workspaceDir, err := cmd.getSessionWorkspace(analysisRequest.SessionID)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("failed to get session workspace").
			Cause(err).
			Build()
	}

	// Perform comprehensive repository analysis
	analysisResult, err := cmd.performAnalysis(ctx, analysisRequest, workspaceDir)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("analysis execution failed").
			Cause(err).
			Build()
	}

	// Update session state with analysis results
	if err := cmd.updateSessionState(analysisRequest.SessionID, analysisResult); err != nil {
		cmd.logger.Warn("failed to update session state", "error", err)
	}

	// Create consolidated response
	response := cmd.createAnalysisResponse(analysisResult, time.Since(startTime))

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"analysis_result": response,
		},
	}, nil
}

// parseAnalysisInput extracts and validates analysis parameters from tool input
func (cmd *ConsolidatedAnalyzeCommand) parseAnalysisInput(input api.ToolInput) (*AnalysisRequest, error) {
	// Extract required parameters
	repositoryPath := getStringParam(input.Data, "repository_path", "")
	repoURL := getStringParam(input.Data, "repo_url", "")

	// Support both repository_path and repo_url for backward compatibility
	if repositoryPath == "" && repoURL == "" {
		return nil, fmt.Errorf("either repository_path or repo_url must be provided")
	}

	targetPath := repositoryPath
	if targetPath == "" {
		targetPath = repoURL
	}

	// Extract optional parameters with defaults
	request := &AnalysisRequest{
		SessionID:      input.SessionID,
		RepositoryPath: targetPath,
		RepoURL:        repoURL,
		AnalysisOptions: AnalysisOptions{
			IncludeSecrets:         getBoolParam(input.Data, "include_secrets", true),
			IncludeDependencies:    getBoolParam(input.Data, "include_dependencies", true),
			IncludeDockerfile:      getBoolParam(input.Data, "include_dockerfile", true),
			IncludeVulnerabilities: getBoolParam(input.Data, "include_vulnerabilities", false),
			IncludeCompliance:      getBoolParam(input.Data, "include_compliance", false),
			IncludeTests:           getBoolParam(input.Data, "include_tests", true),
			IncludeMetrics:         getBoolParam(input.Data, "include_metrics", false),
			MaxDepth:               getIntParam(input.Data, "max_depth", 10),
			OutputFormat:           getStringParam(input.Data, "output_format", "json"),
			Language:               getStringParam(input.Data, "language", ""),
			Framework:              getStringParam(input.Data, "framework", ""),
			CustomPatterns:         getStringSliceParam(input.Data, "custom_patterns"),
			ExcludePatterns:        getStringSliceParam(input.Data, "exclude_patterns"),
		},
		CreatedAt: time.Now(),
	}

	return request, nil
}

// validateAnalysisRequest validates analysis request using domain rules
func (cmd *ConsolidatedAnalyzeCommand) validateAnalysisRequest(request *AnalysisRequest) []ValidationError {
	var errors []ValidationError

	// Session ID validation
	if request.SessionID == "" {
		errors = append(errors, ValidationError{
			Field:   "session_id",
			Message: "session ID is required",
			Code:    "MISSING_SESSION_ID",
		})
	}

	// Repository path validation
	if request.RepositoryPath == "" {
		errors = append(errors, ValidationError{
			Field:   "repository_path",
			Message: "repository path is required",
			Code:    "MISSING_REPOSITORY_PATH",
		})
	}

	// Validate optional parameters
	if request.AnalysisOptions.MaxDepth < 1 || request.AnalysisOptions.MaxDepth > 100 {
		errors = append(errors, ValidationError{
			Field:   "max_depth",
			Message: "max_depth must be between 1 and 100",
			Code:    "INVALID_MAX_DEPTH",
		})
	}

	// Validate output format
	validFormats := []string{"json", "yaml", "xml", "csv"}
	if !slices.Contains(validFormats, request.AnalysisOptions.OutputFormat) {
		errors = append(errors, ValidationError{
			Field:   "output_format",
			Message: fmt.Sprintf("output_format must be one of: %s", strings.Join(validFormats, ", ")),
			Code:    "INVALID_OUTPUT_FORMAT",
		})
	}

	return errors
}

// getSessionWorkspace retrieves the workspace directory for a session
func (cmd *ConsolidatedAnalyzeCommand) getSessionWorkspace(sessionID string) (string, error) {
	sessionMetadata, err := cmd.sessionState.GetSessionMetadata(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session metadata: %w", err)
	}

	workspaceDir, ok := sessionMetadata["workspace_dir"].(string)
	if !ok || workspaceDir == "" {
		return "", fmt.Errorf("workspace directory not found for session %s", sessionID)
	}

	return workspaceDir, nil
}

// performAnalysis executes the comprehensive repository analysis
func (cmd *ConsolidatedAnalyzeCommand) performAnalysis(ctx context.Context, request *AnalysisRequest, workspaceDir string) (*analyze.AnalysisResult, error) {
	// Create repository entity from domain
	repository := analyze.Repository{
		Path: request.RepositoryPath,
		Name: filepath.Base(request.RepositoryPath),
	}

	// Initialize analysis result
	result := &analyze.AnalysisResult{
		Repository: repository,
		Language:   analyze.Language{},
		Framework:  analyze.Framework{},
		Confidence: analyze.ConfidenceMedium,
		AnalysisMetadata: analyze.AnalysisMetadata{
			StartTime: time.Now(),
			Options:   request.AnalysisOptions,
		},
	}

	// Perform language detection
	if err := cmd.detectLanguage(ctx, result, workspaceDir); err != nil {
		return nil, fmt.Errorf("language detection failed: %w", err)
	}

	// Perform framework detection
	if err := cmd.detectFramework(ctx, result, workspaceDir); err != nil {
		return nil, fmt.Errorf("framework detection failed: %w", err)
	}

	// Perform dependency analysis if requested
	if request.AnalysisOptions.IncludeDependencies {
		if err := cmd.analyzeDependencies(ctx, result, workspaceDir); err != nil {
			cmd.logger.Warn("dependency analysis failed", "error", err)
		}
	}

	// Perform Dockerfile analysis if requested
	if request.AnalysisOptions.IncludeDockerfile {
		if err := cmd.analyzeDockerfile(ctx, result, workspaceDir); err != nil {
			cmd.logger.Warn("dockerfile analysis failed", "error", err)
		}
	}

	// Perform security analysis if requested
	if request.AnalysisOptions.IncludeSecrets {
		if err := cmd.analyzeSecrets(ctx, result, workspaceDir); err != nil {
			cmd.logger.Warn("secrets analysis failed", "error", err)
		}
	}

	// Perform vulnerability analysis if requested
	if request.AnalysisOptions.IncludeVulnerabilities {
		if err := cmd.analyzeVulnerabilities(ctx, result, workspaceDir); err != nil {
			cmd.logger.Warn("vulnerability analysis failed", "error", err)
		}
	}

	// Perform compliance analysis if requested
	if request.AnalysisOptions.IncludeCompliance {
		if err := cmd.analyzeCompliance(ctx, result, workspaceDir); err != nil {
			cmd.logger.Warn("compliance analysis failed", "error", err)
		}
	}

	// Perform test analysis if requested
	if request.AnalysisOptions.IncludeTests {
		if err := cmd.analyzeTests(ctx, result, workspaceDir); err != nil {
			cmd.logger.Warn("test analysis failed", "error", err)
		}
	}

	// Perform metrics analysis if requested
	if request.AnalysisOptions.IncludeMetrics {
		if err := cmd.analyzeMetrics(ctx, result, workspaceDir); err != nil {
			cmd.logger.Warn("metrics analysis failed", "error", err)
		}
	}

	// Calculate final confidence and generate recommendations
	cmd.calculateConfidence(result)
	cmd.generateRecommendations(result)

	// Update metadata
	result.AnalysisMetadata.EndTime = time.Now()
	result.AnalysisMetadata.Duration = result.AnalysisMetadata.EndTime.Sub(result.AnalysisMetadata.StartTime)

	return result, nil
}

// detectLanguage performs language detection using multiple strategies
func (cmd *ConsolidatedAnalyzeCommand) detectLanguage(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Language detection logic from original tools
	languageMap := make(map[string]int)

	// File extension-based detection
	if err := cmd.detectLanguageByExtension(workspaceDir, languageMap); err != nil {
		return fmt.Errorf("extension-based language detection failed: %w", err)
	}

	// Content-based detection
	if err := cmd.detectLanguageByContent(workspaceDir, languageMap); err != nil {
		cmd.logger.Warn("content-based language detection failed", "error", err)
	}

	// Determine primary language
	primaryLang, confidence := cmd.determinePrimaryLanguage(languageMap)

	result.Language = analyze.Language{
		Name:       primaryLang,
		Confidence: confidence,
		Percentage: cmd.calculateLanguagePercentage(primaryLang, languageMap),
	}

	return nil
}

// detectFramework performs framework detection based on detected language
func (cmd *ConsolidatedAnalyzeCommand) detectFramework(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Framework detection logic based on language
	switch result.Language.Name {
	case "go":
		return cmd.detectGoFramework(result, workspaceDir)
	case "javascript", "typescript":
		return cmd.detectJSFramework(result, workspaceDir)
	case "python":
		return cmd.detectPythonFramework(result, workspaceDir)
	case "java":
		return cmd.detectJavaFramework(result, workspaceDir)
	case "csharp":
		return cmd.detectDotNetFramework(result, workspaceDir)
	default:
		result.Framework = analyze.Framework{
			Name:       "unknown",
			Type:       analyze.FrameworkTypeUnknown,
			Confidence: analyze.ConfidenceLow,
		}
	}

	return nil
}

// analyzeDependencies performs dependency analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeDependencies(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Dependency analysis logic from original tools
	dependencies := []analyze.Dependency{}

	// Language-specific dependency analysis
	switch result.Language.Name {
	case "go":
		deps, err := cmd.analyzeGoDependencies(workspaceDir)
		if err != nil {
			return err
		}
		dependencies = append(dependencies, deps...)
	case "javascript", "typescript":
		deps, err := cmd.analyzeNodeDependencies(workspaceDir)
		if err != nil {
			return err
		}
		dependencies = append(dependencies, deps...)
	case "python":
		deps, err := cmd.analyzePythonDependencies(workspaceDir)
		if err != nil {
			return err
		}
		dependencies = append(dependencies, deps...)
	case "java":
		deps, err := cmd.analyzeJavaDependencies(workspaceDir)
		if err != nil {
			return err
		}
		dependencies = append(dependencies, deps...)
	}

	result.Dependencies = dependencies
	return nil
}

// analyzeDockerfile performs Dockerfile analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeDockerfile(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Dockerfile analysis logic from original tools
	dockerfilePath := filepath.Join(workspaceDir, "Dockerfile")

	// Check if Dockerfile exists
	if !fileExists(dockerfilePath) {
		return nil // No Dockerfile found, not an error
	}

	// Parse and analyze Dockerfile
	dockerfile, err := cmd.parseDockerfile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile: %w", err)
	}

	// Analyze Dockerfile for security issues
	securityIssues, err := cmd.analyzeDockerfileSecurity(dockerfile)
	if err != nil {
		return fmt.Errorf("dockerfile security analysis failed: %w", err)
	}

	result.SecurityIssues = append(result.SecurityIssues, securityIssues...)

	// Generate Dockerfile recommendations
	recommendations, err := cmd.generateDockerfileRecommendations(dockerfile)
	if err != nil {
		return fmt.Errorf("dockerfile recommendations failed: %w", err)
	}

	result.Recommendations = append(result.Recommendations, recommendations...)

	return nil
}

// updateSessionState updates session state with analysis results
func (cmd *ConsolidatedAnalyzeCommand) updateSessionState(sessionID string, result *analyze.AnalysisResult) error {
	// Update session state with analysis results
	stateUpdate := map[string]interface{}{
		"last_analysis": result,
		"analysis_time": time.Now(),
		"language":      result.Language.Name,
		"framework":     result.Framework.Name,
		"confidence":    result.Confidence,
	}

	return cmd.sessionState.UpdateSessionData(sessionID, stateUpdate)
}

// createAnalysisResponse creates the final analysis response
func (cmd *ConsolidatedAnalyzeCommand) createAnalysisResponse(result *analyze.AnalysisResult, duration time.Duration) *ConsolidatedAnalysisResponse {
	return &ConsolidatedAnalysisResponse{
		Repository:       result.Repository,
		Language:         result.Language,
		Framework:        result.Framework,
		Dependencies:     result.Dependencies,
		Databases:        result.Databases,
		BuildTools:       result.BuildTools,
		TestFrameworks:   result.TestFrameworks,
		SecurityIssues:   result.SecurityIssues,
		Recommendations:  result.Recommendations,
		Confidence:       result.Confidence,
		AnalysisMetadata: result.AnalysisMetadata,
		TotalDuration:    duration,
	}
}

// Helper types and methods for consolidated analysis

// AnalysisRequest represents a consolidated analysis request
type AnalysisRequest struct {
	SessionID       string          `json:"session_id"`
	RepositoryPath  string          `json:"repository_path"`
	RepoURL         string          `json:"repo_url,omitempty"`
	AnalysisOptions AnalysisOptions `json:"analysis_options"`
	CreatedAt       time.Time       `json:"created_at"`
}

// AnalysisOptions contains all analysis configuration options
type AnalysisOptions struct {
	IncludeSecrets         bool     `json:"include_secrets"`
	IncludeDependencies    bool     `json:"include_dependencies"`
	IncludeDockerfile      bool     `json:"include_dockerfile"`
	IncludeVulnerabilities bool     `json:"include_vulnerabilities"`
	IncludeCompliance      bool     `json:"include_compliance"`
	IncludeTests           bool     `json:"include_tests"`
	IncludeMetrics         bool     `json:"include_metrics"`
	MaxDepth               int      `json:"max_depth"`
	OutputFormat           string   `json:"output_format"`
	Language               string   `json:"language,omitempty"`
	Framework              string   `json:"framework,omitempty"`
	CustomPatterns         []string `json:"custom_patterns,omitempty"`
	ExcludePatterns        []string `json:"exclude_patterns,omitempty"`
}

// ConsolidatedAnalysisResponse represents the consolidated analysis response
type ConsolidatedAnalysisResponse struct {
	Repository       analyze.Repository       `json:"repository"`
	Language         analyze.Language         `json:"language"`
	Framework        analyze.Framework        `json:"framework"`
	Dependencies     []analyze.Dependency     `json:"dependencies"`
	Databases        []analyze.Database       `json:"databases"`
	BuildTools       []analyze.BuildTool      `json:"build_tools"`
	TestFrameworks   []analyze.TestFramework  `json:"test_frameworks"`
	SecurityIssues   []analyze.SecurityIssue  `json:"security_issues"`
	Recommendations  []analyze.Recommendation `json:"recommendations"`
	Confidence       analyze.ConfidenceLevel  `json:"confidence"`
	AnalysisMetadata analyze.AnalysisMetadata `json:"metadata"`
	TotalDuration    time.Duration            `json:"total_duration"`
}

// Note: ValidationError is defined in common.go

// Tool registration for consolidated analyze command
func (cmd *ConsolidatedAnalyzeCommand) Name() string {
	return "analyze_repository"
}

func (cmd *ConsolidatedAnalyzeCommand) Description() string {
	return "Comprehensive repository analysis tool that consolidates all analysis capabilities"
}

func (cmd *ConsolidatedAnalyzeCommand) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        cmd.Name(),
		Description: cmd.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repository_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the repository to analyze",
				},
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of the repository to analyze (alternative to repository_path)",
				},
				"include_secrets": map[string]interface{}{
					"type":        "boolean",
					"description": "Include secrets analysis",
					"default":     true,
				},
				"include_dependencies": map[string]interface{}{
					"type":        "boolean",
					"description": "Include dependency analysis",
					"default":     true,
				},
				"include_dockerfile": map[string]interface{}{
					"type":        "boolean",
					"description": "Include Dockerfile analysis",
					"default":     true,
				},
				"include_vulnerabilities": map[string]interface{}{
					"type":        "boolean",
					"description": "Include vulnerability analysis",
					"default":     false,
				},
				"include_compliance": map[string]interface{}{
					"type":        "boolean",
					"description": "Include compliance analysis",
					"default":     false,
				},
				"include_tests": map[string]interface{}{
					"type":        "boolean",
					"description": "Include test framework analysis",
					"default":     true,
				},
				"include_metrics": map[string]interface{}{
					"type":        "boolean",
					"description": "Include code metrics analysis",
					"default":     false,
				},
				"max_depth": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum directory depth to analyze",
					"default":     10,
					"minimum":     1,
					"maximum":     100,
				},
				"output_format": map[string]interface{}{
					"type":        "string",
					"description": "Output format for analysis results",
					"enum":        []string{"json", "yaml", "xml", "csv"},
					"default":     "json",
				},
				"custom_patterns": map[string]interface{}{
					"type":        "array",
					"description": "Custom patterns for analysis",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"exclude_patterns": map[string]interface{}{
					"type":        "array",
					"description": "Patterns to exclude from analysis",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"repository_path"},
		},
		Tags:     []string{"analysis", "repository", "containerization"},
		Category: api.CategoryAnalysis,
	}
}
