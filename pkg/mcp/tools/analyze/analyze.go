package analyze

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// Analyzer handles repository analysis operations
type Analyzer struct {
	logger *slog.Logger
}

// NewAnalyzer creates a new repository analyzer
func NewAnalyzer(logger *slog.Logger) *Analyzer {
	return &Analyzer{
		logger: logger.With("component", "repository_analyzer"),
	}
}

// Analyze performs analysis on a repository
func (a *Analyzer) Analyze(ctx context.Context, opts core.AnalysisOptions) (*core.AnalysisResult, error) {
	startTime := time.Now()

	if err := a.validateAnalysisOptions(opts); err != nil {
		return nil, errors.NewError().Message("invalid analysis options").Cause(err).Build()
	}

	a.logger.Info("Starting repository analysis",
		"repo_path", opts.RepoPath,
		"language_hint", opts.LanguageHint)

	analyzer := analysis.NewRepositoryAnalyzer(a.logger)
	coreResult, err := analyzer.AnalyzeRepository(opts.RepoPath)
	if err != nil {
		return nil, errors.NewError().Message("failed to analyze repository").Cause(err).WithLocation().Build()
	}

	analysisContext, err := a.generateAnalysisContext(opts.RepoPath, coreResult)
	if err != nil {
		a.logger.Warn("Failed to generate full analysis context", "error", err)
	}

	analysisContext.ContainerizationSuggestions = a.generateContainerizationSuggestions(coreResult)
	analysisContext.NextStepSuggestions = a.generateNextStepSuggestions(coreResult, analysisContext)

	result := &core.AnalysisResult{
		Success:      coreResult.Success,
		Language:     coreResult.Language,
		Framework:    coreResult.Framework,
		Dependencies: convertDeps(coreResult.Dependencies),
		Duration:     time.Since(startTime),
	}

	// Add additional context in metadata
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["analysis_context"] = analysisContext

	return result, nil
}

// validateAnalysisOptions validates the analysis options
func (a *Analyzer) validateAnalysisOptions(opts core.AnalysisOptions) error {
	if opts.RepoPath == "" {
		return errors.NewError().Messagef("repository path is required").WithLocation().Build()
	}

	if _, err := os.Stat(opts.RepoPath); err != nil {
		return errors.NewError().Message("repository path does not exist").Cause(err).WithLocation().Build()
	}

	return nil
}

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

	if analysis.ConfigFiles != nil {
		ctx.FilesAnalyzed = len(analysis.ConfigFiles) + len(analysis.BuildFiles) + len(analysis.EntryPoints)
	}

	for _, configFile := range analysis.ConfigFiles {
		path := configFile.Path
		if a.isConfigFile(path) {
			ctx.ConfigFilesFound = append(ctx.ConfigFilesFound, path)
		}

		if strings.Contains(strings.ToLower(path), "dockerfile") || strings.HasSuffix(path, ".dockerfile") {
			ctx.DockerFiles = append(ctx.DockerFiles, path)
		}

		if a.isTestFile(path) {
			ctx.TestFilesFound = append(ctx.TestFilesFound, path)
		}

		if a.isBuildFile(path) {
			ctx.BuildFilesFound = append(ctx.BuildFilesFound, path)
		}

		if a.isK8sFile(path) {
			ctx.K8sFiles = append(ctx.K8sFiles, path)
		}

		if a.isDatabaseFile(path) {
			ctx.DatabaseFiles = append(ctx.DatabaseFiles, path)
		}
	}

	ctx.EntryPointsFound = analysis.EntryPoints

	for _, buildFile := range analysis.BuildFiles {
		if a.isBuildFile(buildFile) {
			ctx.BuildFilesFound = append(ctx.BuildFilesFound, buildFile)
		}
	}

	ctx.HasGitIgnore = a.fileExists(filepath.Join(repoPath, ".gitignore"))
	ctx.HasReadme = a.hasReadmeFile(repoPath)
	ctx.HasLicense = a.hasLicenseFile(repoPath)
	ctx.HasCI = a.hasCIConfig(repoPath)

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

	if len(ctx.DockerFiles) == 0 {
		suggestions = append(suggestions, "Generate a Dockerfile using 'generate_dockerfile' tool")
	} else {
		suggestions = append(suggestions, "Review and optimize existing Dockerfile")
	}

	suggestions = append(suggestions, "Build container image using 'build_image' tool")

	suggestions = append(suggestions, "Scan for security vulnerabilities using 'scan_image_security' tool")

	if len(ctx.K8sFiles) == 0 {
		suggestions = append(suggestions, "Generate Kubernetes manifests using 'generate_manifests' tool")
	}

	suggestions = append(suggestions, "Scan for secrets using 'scan_secrets' tool")

	return suggestions
}

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

// convertDeps converts analysis.Dependency to strings for core.AnalysisResult
func convertDeps(deps []analysis.Dependency) []string {
	result := make([]string, len(deps))
	for i, dep := range deps {
		result[i] = dep.Name
	}
	return result
}
