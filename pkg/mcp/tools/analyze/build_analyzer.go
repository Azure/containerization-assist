package analyze

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// BuildAnalyzer analyzes build systems and entry points
type BuildAnalyzer struct {
	logger zerolog.Logger
}

// NewBuildAnalyzer creates a new build analyzer
func NewBuildAnalyzer(logger zerolog.Logger) *BuildAnalyzer {
	return &BuildAnalyzer{
		logger: logger.With().Str("engine", "build").Logger(),
	}
}

// GetName returns the name of this engine
func (b *BuildAnalyzer) GetName() string {
	return "build_analyzer"
}

// GetCapabilities returns what this engine can analyze
func (b *BuildAnalyzer) GetCapabilities() []string {
	return []string{
		"build_systems",
		"entry_points",
		"build_scripts",
		"ci_cd_configuration",
		"containerization_readiness",
		"deployment_artifacts",
	}
}

// IsApplicable determines if this engine should run
func (b *BuildAnalyzer) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	// Build analysis is always useful
	return true
}

// Analyze performs build system analysis
func (b *BuildAnalyzer) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	startTime := time.Now()
	result := &EngineAnalysisResult{
		Engine:   b.GetName(),
		Findings: make([]Finding, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Analyze build systems
	b.analyzeBuildSystems(config, result)
	b.analyzeEntryPoints(config, result)
	b.analyzeBuildScripts(config, result)
	b.analyzeCICDConfiguration(config, result)

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0

	// Calculate confidence
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
	result.Metadata["build_files_found"] = b.countBuildFiles(config)
	result.Metadata["findings_count"] = len(result.Findings)

	return result, nil
}

// Helper types and methods

type BuildSystemConfig struct {
	Files       []string
	Scripts     []string
	Description string
	Type        string
}
