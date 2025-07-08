package analyze

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// LanguageAnalyzer analyzes programming languages and frameworks
type LanguageAnalyzer struct {
	logger zerolog.Logger
}

// NewLanguageAnalyzer creates a new language analyzer
func NewLanguageAnalyzer(logger zerolog.Logger) *LanguageAnalyzer {
	return &LanguageAnalyzer{
		logger: logger.With().Str("engine", "language").Logger(),
	}
}

// GetName returns the name of this engine
func (l *LanguageAnalyzer) GetName() string {
	return "language_analyzer"
}

// GetCapabilities returns what this engine can analyze
func (l *LanguageAnalyzer) GetCapabilities() []string {
	return []string{
		"programming_languages",
		"web_frameworks",
		"runtime_detection",
		"technology_stack",
		"version_analysis",
	}
}

// IsApplicable determines if this engine should run
func (l *LanguageAnalyzer) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	// Always applicable - every repo has some language/framework
	return true
}

// Analyze performs language and framework analysis
func (l *LanguageAnalyzer) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	startTime := time.Now()
	result := &EngineAnalysisResult{
		Engine:   l.GetName(),
		Findings: make([]Finding, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Analyze programming languages
	l.analyzeProgrammingLanguages(config, result)
	l.analyzeWebFrameworks(config, result)
	l.analyzeRuntimeDetection(config, result)
	l.analyzeTechnologyStack(config, result)

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
		result.Confidence = 0.6
	}

	// Store metadata
	result.Metadata["languages_detected"] = l.countLanguages(config)
	result.Metadata["frameworks_detected"] = l.countFrameworks(result)

	return result, nil
}
