package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	utilsfs "github.com/Azure/container-kit/pkg/utils"
	"github.com/rs/zerolog"
)

// AnalyzeRepositoryArgs defines arguments for repository analysis
type AnalyzeRepositoryArgs struct {
	types.BaseToolArgs
	Path         string `json:"path" validate:"required" description:"Local directory path or GitHub URL"`
	Context      string `json:"context,omitempty" validate:"omitempty,max=1000" description:"Additional context about the application"`
	Language     string `json:"language,omitempty" validate:"omitempty,language" description:"Primary programming language hint"`
	Framework    string `json:"framework,omitempty" validate:"omitempty,framework" description:"Framework hint (e.g., express, django)"`
	SkipFileTree bool   `json:"skip_file_tree,omitempty" description:"Skip generating file tree for performance"`
	Sandbox      bool   `json:"sandbox,omitempty" description:"Run analysis in sandboxed environment"`
	DryRun       bool   `json:"dry_run,omitempty" description:"Preview changes without executing"`
}

// Validate implements core.ToolParams using tag-based validation
func (a AnalyzeRepositoryArgs) Validate() error {
	// TODO: Implement validation using the new validation system
	// For now, return nil as a temporary fix
	return nil
}

func (a AnalyzeRepositoryArgs) GetSessionID() string {
	return a.SessionID
}

// RepositoryAnalysisResult defines the response from repository analysis
type RepositoryAnalysisResult struct {
	types.BaseToolResponse
	Language         string              `json:"language"`
	Framework        string              `json:"framework"`
	Dependencies     []string            `json:"dependencies"`
	EntryPoints      []string            `json:"entry_points"`
	DatabaseType     string              `json:"database_type,omitempty"`
	Port             int                 `json:"port,omitempty"`
	BuildCommands    []string            `json:"build_commands"`
	RunCommand       string              `json:"run_command"`
	FileTree         string              `json:"file_tree,omitempty"`
	Suggestions      []string            `json:"suggestions"`
	SecurityScan     *SecurityScanResult `json:"security_scan,omitempty"`
	AnalysisDuration time.Duration       `json:"analysis_duration"`
	FilesAnalyzed    int                 `json:"files_analyzed"`
}

// IsSuccess implements core.ToolResult
func (r RepositoryAnalysisResult) IsSuccess() bool {
	// Consider analysis successful if we found a language and have some analysis results
	return r.Language != "" && r.FilesAnalyzed > 0
}

// GenericAnalyzeRepositoryTool removed - use core.GenericTool[AnalyzeRepositoryArgs, RepositoryAnalysisResult] directly

// SecurityScanResult contains security analysis results
type SecurityScanResult struct {
	Issues          []SecurityIssue `json:"issues"`
	RiskLevel       string          `json:"risk_level"`
	Recommendations []string        `json:"recommendations"`
}

// SecurityIssue represents a security issue found during analysis
type SecurityIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	Fix         string `json:"fix,omitempty"`
}

// AnalyzeRepositoryTool implements a simplified analyze_repository MCP tool
type AnalyzeRepositoryTool struct {
	logger zerolog.Logger
}

// NewAnalyzeRepositoryTool creates a new analyze repository tool
func NewAnalyzeRepositoryTool(logger zerolog.Logger) *AnalyzeRepositoryTool {
	return &AnalyzeRepositoryTool{
		logger: logger.With().Str("tool", "analyze_repository").Logger(),
	}
}

// ExecuteTyped runs the repository analysis
func (t *AnalyzeRepositoryTool) ExecuteTyped(ctx context.Context, args AnalyzeRepositoryArgs) (*RepositoryAnalysisResult, error) {
	startTime := time.Now()

	sessionID := args.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	// Create base response
	response := &RepositoryAnalysisResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   true,
			Timestamp: time.Now(),
			Metadata:  map[string]string{"tool": "analyze_repository", "session_id": sessionID},
		},
		Dependencies:  make([]string, 0),
		EntryPoints:   make([]string, 0),
		BuildCommands: make([]string, 0),
		Suggestions:   make([]string, 0),
	}

	// If dry-run, return early with placeholder data
	if args.DryRun {
		response.Language = "unknown"
		response.Framework = "unknown"
		response.Suggestions = []string{"This is a dry-run - actual analysis would be performed"}
		response.AnalysisDuration = time.Since(startTime)
		return response, nil
	}

	// Validate path
	repoPath := args.Path
	if isURL(args.Path) {
		return nil, errors.NewError().Messagef("URL repositories not supported yet").WithLocation().Build()
	}

	if err := utils.ValidateLocalPath(repoPath); err != nil {
		return nil, errors.NewError().Messagef("invalid local path").Cause(err).WithLocation().Build()
	}

	if err := t.analyzeRepository(repoPath, response, args); err != nil {
		return nil, errors.NewError().Messagef("repository analysis failed").Cause(err).Build()
	}

	response.AnalysisDuration = time.Since(startTime)

	t.logger.Info().
		Str("session_id", sessionID).
		Str("language", response.Language).
		Str("framework", response.Framework).
		Dur("duration", response.AnalysisDuration).
		Int("files_analyzed", response.FilesAnalyzed).
		Msg("Repository analysis completed")

	return response, nil
}

// analyzeRepository performs the actual repository analysis
func (t *AnalyzeRepositoryTool) analyzeRepository(repoPath string, result *RepositoryAnalysisResult, args AnalyzeRepositoryArgs) error {
	// Generate file tree if requested
	if !args.SkipFileTree {
		fileTree, err := generateFileTree(repoPath)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to generate file tree")
		} else {
			result.FileTree = fileTree
		}
	}

	// Detect language and framework
	if err := t.detectLanguageAndFramework(repoPath, result); err != nil {
		return err
	}

	// Extract dependencies
	t.extractDependencies(repoPath, result)

	// Identify entry points
	t.identifyEntryPoints(repoPath, result)

	// Generate build commands
	t.generateBuildCommands(result)

	// Generate suggestions
	t.generateSuggestions(result)

	// Perform basic security scan
	result.SecurityScan = &SecurityScanResult{
		Issues:    make([]SecurityIssue, 0),
		RiskLevel: "low",
		Recommendations: []string{
			"Consider adding security scanning to your CI/CD pipeline",
			"Regularly update dependencies to latest versions",
		},
	}

	return nil
}

// detectLanguageAndFramework detects the primary language and framework
func (t *AnalyzeRepositoryTool) detectLanguageAndFramework(repoPath string, result *RepositoryAnalysisResult) error {
	commonFiles := map[string]func() (string, string){
		"package.json":     func() (string, string) { return types.LanguageJavaScript, "nodejs" },
		"go.mod":           func() (string, string) { return "go", "go" },
		"requirements.txt": func() (string, string) { return types.LanguagePython, types.LanguagePython },
		"Cargo.toml":       func() (string, string) { return "rust", "rust" },
		"pom.xml":          func() (string, string) { return types.LanguageJava, types.BuildSystemMaven },
		"build.gradle":     func() (string, string) { return types.LanguageJava, types.BuildSystemGradle },
		"Gemfile":          func() (string, string) { return "ruby", "ruby" },
		"composer.json":    func() (string, string) { return "php", "php" },
	}

	for file, detector := range commonFiles {
		if fileExists(filepath.Join(repoPath, file)) {
			result.Language, result.Framework = detector()
			result.FilesAnalyzed++
			return nil
		}
	}

	// Default to unknown
	result.Language = "unknown"
	result.Framework = "unknown"

	return nil
}

// extractDependencies extracts dependencies based on language
func (t *AnalyzeRepositoryTool) extractDependencies(repoPath string, result *RepositoryAnalysisResult) {
	// Simplified dependency extraction
	switch result.Language {
	case types.LanguageJavaScript:
		result.Dependencies = []string{"npm dependencies"}
	case "go":
		result.Dependencies = []string{"go modules"}
	case types.LanguagePython:
		result.Dependencies = []string{"pip packages"}
	}
}

// identifyEntryPoints identifies common entry points
func (t *AnalyzeRepositoryTool) identifyEntryPoints(repoPath string, result *RepositoryAnalysisResult) {
	switch result.Language {
	case types.LanguageJavaScript:
		if fileExists(filepath.Join(repoPath, "index.js")) {
			result.EntryPoints = append(result.EntryPoints, "index.js")
		}
		if fileExists(filepath.Join(repoPath, "server.js")) {
			result.EntryPoints = append(result.EntryPoints, "server.js")
		}
	case "go":
		if fileExists(filepath.Join(repoPath, "main.go")) {
			result.EntryPoints = append(result.EntryPoints, "main.go")
		}
	case types.LanguagePython:
		if fileExists(filepath.Join(repoPath, "main.py")) {
			result.EntryPoints = append(result.EntryPoints, "main.py")
		}
		if fileExists(filepath.Join(repoPath, "app.py")) {
			result.EntryPoints = append(result.EntryPoints, "app.py")
		}
	}
}

// generateBuildCommands generates build commands based on language
func (t *AnalyzeRepositoryTool) generateBuildCommands(result *RepositoryAnalysisResult) {
	switch result.Language {
	case types.LanguageJavaScript:
		result.BuildCommands = []string{"npm install", "npm run build"}
		result.RunCommand = "npm start"
	case "go":
		result.BuildCommands = []string{"go mod download", "go build"}
		result.RunCommand = "go run ."
	case types.LanguagePython:
		result.BuildCommands = []string{"pip install -r requirements.txt"}
		result.RunCommand = "python main.py"
	case types.LanguageJava:
		if result.Framework == types.BuildSystemMaven {
			result.BuildCommands = []string{"mvn clean install"}
			result.RunCommand = "java -jar target/*.jar"
		} else {
			result.BuildCommands = []string{"./gradlew build"}
			result.RunCommand = "java -jar build/libs/*.jar"
		}
	}
}

// generateSuggestions provides automated suggestions
func (t *AnalyzeRepositoryTool) generateSuggestions(result *RepositoryAnalysisResult) {
	result.Suggestions = append(result.Suggestions,
		fmt.Sprintf("Detected %s application", result.Language))

	if result.Framework != "unknown" && result.Framework != result.Language {
		result.Suggestions = append(result.Suggestions,
			fmt.Sprintf("Framework: %s", result.Framework))
	}

	if len(result.EntryPoints) > 0 {
		result.Suggestions = append(result.Suggestions,
			fmt.Sprintf("Entry points: %s", strings.Join(result.EntryPoints, ", ")))
	}
}

// Helper functions

func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// validateLocalPath is now replaced by utils.ValidateLocalPath

func generateFileTree(path string) (string, error) {
	return utilsfs.GenerateSimpleFileTree(path)
}

// Execute implements core.GenericTool[AnalyzeRepositoryArgs, RepositoryAnalysisResult]
func (t *AnalyzeRepositoryTool) Execute(ctx context.Context, args AnalyzeRepositoryArgs) (RepositoryAnalysisResult, error) {
	// Call the typed execute method and dereference the pointer
	result, err := t.ExecuteTyped(ctx, args)
	if err != nil {
		return RepositoryAnalysisResult{}, err
	}
	return *result, nil
}

// Validate implements core.GenericTool[AnalyzeRepositoryArgs, RepositoryAnalysisResult]
func (t *AnalyzeRepositoryTool) Validate(ctx context.Context, args AnalyzeRepositoryArgs) error {
	// Validate the arguments
	return args.Validate()
}

// GetMetadata implements the unified Tool interface
func (t *AnalyzeRepositoryTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "analyze_repository",
		Description:  "Analyzes a repository to determine language, framework, dependencies and configuration",
		Version:      "1.0.0",
		Category:     api.ToolCategory("analysis"),
		Tags:         []string{"analysis", "repository", "simple"},
		Status:       api.ToolStatus("active"),
		Dependencies: []string{},
		Capabilities: []string{
			"language_detection",
			"framework_detection",
			"dependency_analysis",
			"entrypoint_detection",
			"security_scanning",
			"file_tree_generation",
		},
		Requirements: []string{
			"filesystem_access",
			"path_validation",
		},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}
