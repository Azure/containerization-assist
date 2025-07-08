package analyze

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// DependencyAnalyzer analyzes package dependencies and their security/compatibility
type DependencyAnalyzer struct {
	logger zerolog.Logger
}

// NewDependencyAnalyzer creates a new dependency analyzer
func NewDependencyAnalyzer(logger zerolog.Logger) *DependencyAnalyzer {
	return &DependencyAnalyzer{
		logger: logger.With().Str("engine", "dependency").Logger(),
	}
}

// GetName returns the name of this engine
func (d *DependencyAnalyzer) GetName() string {
	return "dependency_analyzer"
}

// GetCapabilities returns what this engine can analyze
func (d *DependencyAnalyzer) GetCapabilities() []string {
	return []string{
		"package_dependencies",
		"dependency_versions",
		"security_vulnerabilities",
		"license_analysis",
		"dependency_graph",
		"outdated_packages",
	}
}

// IsApplicable determines if this engine should run
func (d *DependencyAnalyzer) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	// Check if any dependency files exist
	dependencyFiles := []string{
		"package.json", "yarn.lock", "package-lock.json",
		"requirements.txt", "Pipfile", "poetry.lock", "pyproject.toml",
		"go.mod", "go.sum",
		"pom.xml", "build.gradle", "Gemfile", "composer.json",
		"Cargo.toml", "Cargo.lock",
	}

	for _, file := range dependencyFiles {
		if d.fileExists(repoData, file) {
			return true
		}
	}
	return false
}

// Analyze performs dependency analysis
func (d *DependencyAnalyzer) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	startTime := time.Now()
	result := &EngineAnalysisResult{
		Engine:   d.GetName(),
		Findings: make([]Finding, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Analyze different dependency file types
	d.analyzePackageFiles(config, result)
	d.analyzeDependencyVersions(config, result)
	d.analyzeSecurityVulnerabilities(config, result)

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0

	// Calculate confidence based on findings
	if len(result.Findings) > 0 {
		totalConfidence := 0.0
		for _, finding := range result.Findings {
			totalConfidence += finding.Confidence
		}
		result.Confidence = totalConfidence / float64(len(result.Findings))
	} else {
		result.Confidence = 0.5
	}

	// Store metadata
	result.Metadata["dependency_files_found"] = d.countDependencyFiles(config)
	result.Metadata["findings_count"] = len(result.Findings)

	return result, nil
}

// Helper methods

func (d *DependencyAnalyzer) fileExists(repoData *RepoData, filename string) bool {
	for _, file := range repoData.Files {
		if strings.HasSuffix(file.Path, filename) || filepath.Base(file.Path) == filename {
			return true
		}
	}
	return false
}
