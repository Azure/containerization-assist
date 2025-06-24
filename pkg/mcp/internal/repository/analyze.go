package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/rs/zerolog"
)

// Analyzer handles repository analysis operations
type Analyzer struct {
	logger zerolog.Logger
}

// NewAnalyzer creates a new repository analyzer
func NewAnalyzer(logger zerolog.Logger) *Analyzer {
	return &Analyzer{
		logger: logger.With().Str("component", "repository_analyzer").Logger(),
	}
}

// Analyze performs analysis on a repository
func (a *Analyzer) Analyze(ctx context.Context, opts AnalysisOptions) (*AnalysisResult, error) {
	startTime := time.Now()

	// Validate options
	if err := a.validateAnalysisOptions(opts); err != nil {
		return nil, fmt.Errorf("invalid analysis options: %w", err)
	}

	a.logger.Info().
		Str("repo_path", opts.RepoPath).
		Str("language_hint", opts.LanguageHint).
		Msg("Starting repository analysis")

	// Perform core analysis
	analyzer := analysis.NewRepositoryAnalyzer(a.logger)
	coreResult, err := analyzer.AnalyzeRepository(opts.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze repository: %w", err)
	}

	// Generate analysis context
	analysisContext, err := a.generateAnalysisContext(opts.RepoPath, coreResult)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to generate full analysis context")
		// Continue with partial context
	}

	// Generate suggestions
	analysisContext.ContainerizationSuggestions = a.generateContainerizationSuggestions(coreResult)
	analysisContext.NextStepSuggestions = a.generateNextStepSuggestions(coreResult, analysisContext)

	return &AnalysisResult{
		AnalysisResult: coreResult,
		Duration:       time.Since(startTime),
		Context:        analysisContext,
	}, nil
}

// validateAnalysisOptions validates the analysis options
func (a *Analyzer) validateAnalysisOptions(opts AnalysisOptions) error {
	if opts.RepoPath == "" {
		return fmt.Errorf("repository path is required")
	}

	// Check if path exists
	if _, err := os.Stat(opts.RepoPath); err != nil {
		return fmt.Errorf("repository path does not exist: %w", err)
	}

	return nil
}

// generateAnalysisContext generates rich context from the analysis
func (a *Analyzer) generateAnalysisContext(repoPath string, analysis *analysis.AnalysisResult) (*AnalysisContext, error) {
	ctx := &AnalysisContext{
		ConfigFilesFound: []string{},
		EntryPointsFound: []string{},
		TestFilesFound:   []string{},
		BuildFilesFound:  []string{},
		PackageManagers:  []string{},
		DatabaseFiles:    []string{},
		DockerFiles:      []string{},
		K8sFiles:         []string{},
	}

	if analysis == nil {
		return ctx, nil
	}

	// Count analyzed files
	if analysis.ConfigFiles != nil {
		ctx.FilesAnalyzed = len(analysis.ConfigFiles) + len(analysis.BuildFiles) + len(analysis.EntryPoints)
	}

	// Process config files
	for _, configFile := range analysis.ConfigFiles {
		path := configFile.Path
		// Config files
		if a.isConfigFile(path) {
			ctx.ConfigFilesFound = append(ctx.ConfigFilesFound, path)
		}

		// Docker files
		if strings.Contains(strings.ToLower(path), "dockerfile") || strings.HasSuffix(path, ".dockerfile") {
			ctx.DockerFiles = append(ctx.DockerFiles, path)
		}

		// Test files
		if a.isTestFile(path) {
			ctx.TestFilesFound = append(ctx.TestFilesFound, path)
		}

		// Build files
		if a.isBuildFile(path) {
			ctx.BuildFilesFound = append(ctx.BuildFilesFound, path)
		}

		// K8s files
		if a.isK8sFile(path) {
			ctx.K8sFiles = append(ctx.K8sFiles, path)
		}

		// Database files
		if a.isDatabaseFile(path) {
			ctx.DatabaseFiles = append(ctx.DatabaseFiles, path)
		}
	}

	// Add entry points
	ctx.EntryPointsFound = analysis.EntryPoints

	// Add build files
	for _, buildFile := range analysis.BuildFiles {
		if a.isBuildFile(buildFile) {
			ctx.BuildFilesFound = append(ctx.BuildFilesFound, buildFile)
		}
	}

	// Repository metadata
	ctx.HasGitIgnore = a.fileExists(filepath.Join(repoPath, ".gitignore"))
	ctx.HasReadme = a.hasReadmeFile(repoPath)
	ctx.HasLicense = a.hasLicenseFile(repoPath)
	ctx.HasCI = a.hasCIConfig(repoPath)

	// Calculate repository size
	repoSize, _ := a.calculateDirectorySize(repoPath)
	ctx.RepositorySize = repoSize

	return ctx, nil
}

// generateContainerizationSuggestions generates containerization suggestions
func (a *Analyzer) generateContainerizationSuggestions(analysis *analysis.AnalysisResult) []string {
	suggestions := []string{}

	if analysis.Language != "" {
		suggestions = append(suggestions, fmt.Sprintf("Detected %s application - consider using official %s base image",
			analysis.Language, strings.ToLower(analysis.Language)))
	}

	if analysis.Framework != "" {
		suggestions = append(suggestions, fmt.Sprintf("Framework %s detected - ensure framework-specific requirements are included",
			analysis.Framework))
	}

	if len(analysis.Dependencies) > 0 {
		suggestions = append(suggestions, "Dependencies detected - ensure they are properly installed in the container")
	}

	if len(analysis.ConfigFiles) > 0 {
		suggestions = append(suggestions, "Configuration files detected - consider using environment variables or config maps")
	}

	return suggestions
}

// generateNextStepSuggestions generates next step suggestions
func (a *Analyzer) generateNextStepSuggestions(analysis *analysis.AnalysisResult, ctx *AnalysisContext) []string {
	suggestions := []string{}

	// Dockerfile generation
	if len(ctx.DockerFiles) == 0 {
		suggestions = append(suggestions, "Generate a Dockerfile using 'generate_dockerfile' tool")
	} else {
		suggestions = append(suggestions, "Review and optimize existing Dockerfile")
	}

	// Build suggestion
	suggestions = append(suggestions, "Build container image using 'build_image' tool")

	// Security scanning
	suggestions = append(suggestions, "Scan for security vulnerabilities using 'scan_image_security' tool")

	// Kubernetes manifests
	if len(ctx.K8sFiles) == 0 {
		suggestions = append(suggestions, "Generate Kubernetes manifests using 'generate_manifests' tool")
	}

	// Secrets scanning
	suggestions = append(suggestions, "Scan for secrets using 'scan_secrets' tool")

	return suggestions
}

// Helper methods

func (a *Analyzer) isConfigFile(path string) bool {
	configPatterns := []string{
		"config", "settings", ".env", ".properties", ".yaml", ".yml", ".json", ".toml", ".ini",
	}
	lowerPath := strings.ToLower(path)
	for _, pattern := range configPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}
	return false
}

func (a *Analyzer) isTestFile(path string) bool {
	testPatterns := []string{"test", "spec", "_test.go", ".test."}
	lowerPath := strings.ToLower(path)
	for _, pattern := range testPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}
	return false
}

func (a *Analyzer) isBuildFile(path string) bool {
	buildFiles := []string{
		"makefile", "build.gradle", "pom.xml", "package.json", "cargo.toml",
		"go.mod", "requirements.txt", "gemfile", "build.sbt", "project.clj",
	}
	lowerPath := strings.ToLower(filepath.Base(path))
	for _, bf := range buildFiles {
		if lowerPath == bf {
			return true
		}
	}
	return false
}

func (a *Analyzer) isK8sFile(path string) bool {
	k8sPatterns := []string{
		"deployment", "service", "ingress", "configmap", "secret",
		"statefulset", "daemonset", "job", "cronjob", ".k8s.", "-k8s.",
	}
	lowerPath := strings.ToLower(path)
	for _, pattern := range k8sPatterns {
		if strings.Contains(lowerPath, pattern) && (strings.HasSuffix(lowerPath, ".yaml") || strings.HasSuffix(lowerPath, ".yml")) {
			return true
		}
	}
	return false
}

func (a *Analyzer) isDatabaseFile(path string) bool {
	dbPatterns := []string{
		".sql", "migration", "schema", "database", ".db", ".sqlite",
	}
	lowerPath := strings.ToLower(path)
	for _, pattern := range dbPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}
	return false
}

func (a *Analyzer) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (a *Analyzer) hasReadmeFile(repoPath string) bool {
	readmeFiles := []string{"README.md", "README.txt", "README", "readme.md", "Readme.md"}
	for _, rf := range readmeFiles {
		if a.fileExists(filepath.Join(repoPath, rf)) {
			return true
		}
	}
	return false
}

func (a *Analyzer) hasLicenseFile(repoPath string) bool {
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "license", "License"}
	for _, lf := range licenseFiles {
		if a.fileExists(filepath.Join(repoPath, lf)) {
			return true
		}
	}
	return false
}

func (a *Analyzer) hasCIConfig(repoPath string) bool {
	ciPaths := []string{
		".github/workflows",
		".gitlab-ci.yml",
		".travis.yml",
		"Jenkinsfile",
		".circleci/config.yml",
		"azure-pipelines.yml",
	}
	for _, cp := range ciPaths {
		if a.fileExists(filepath.Join(repoPath, cp)) {
			return true
		}
	}
	return false
}

func (a *Analyzer) calculateDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
