package analyze

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// RepoData represents repository data for analysis
type RepoData struct {
	Path      string                 `json:"path"`
	Files     []FileData             `json:"files"`
	Languages map[string]float64     `json:"languages"`
	Structure map[string]interface{} `json:"structure"`
}

// FileData represents a file in the repository
type FileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// AnalysisEngine defines the interface for repository analysis engines
type AnalysisEngine interface {
	// GetName returns the name of the analysis engine
	GetName() string

	// Analyze performs analysis on the repository
	Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error)

	// GetCapabilities returns what this engine can analyze
	GetCapabilities() []string

	// IsApplicable determines if this engine should run for the given repository
	IsApplicable(ctx context.Context, repoData *RepoData) bool

	// DetectDatabases detects database usage in the repository (was DatabaseDetector)
	DetectDatabases(path string) ([]DetectedDatabase, error)
}

// AnalysisConfig provides configuration for analysis engines
type AnalysisConfig struct {
	RepositoryPath string
	RepoData       *RepoData
	Options        AnalysisOptions
	Logger         zerolog.Logger
}

// EngineAnalysisOptions provides options for analysis engines (renamed to avoid conflict with types.go)
type EngineAnalysisOptions struct {
	IncludeFrameworks    bool
	IncludeDependencies  bool
	IncludeConfiguration bool
	IncludeDatabase      bool
	IncludeBuild         bool
	DeepAnalysis         bool
	MaxDepth             int
}

// EngineAnalysisResult represents the result from an analysis engine (renamed to avoid conflict with types.go)
type EngineAnalysisResult struct {
	Engine     string
	Success    bool
	Duration   time.Duration
	Findings   []Finding
	Metadata   map[string]interface{}
	Confidence float64
	Errors     []error
}

// Finding represents a specific analysis finding
type Finding struct {
	Type        FindingType
	Category    string
	Title       string
	Description string
	Confidence  float64
	Severity    Severity
	Location    *Location
	Metadata    map[string]interface{}
	Evidence    []Evidence
}

// FindingType represents the type of finding
type FindingType string

const (
	FindingTypeLanguage      FindingType = "language"
	FindingTypeFramework     FindingType = "framework"
	FindingTypeDependency    FindingType = "dependency"
	FindingTypeConfiguration FindingType = "configuration"
	FindingTypeDatabase      FindingType = "database"
	FindingTypeBuild         FindingType = "build"
	FindingTypePort          FindingType = "port"
	FindingTypeEnvironment   FindingType = "environment"
	FindingTypeEntrypoint    FindingType = "entrypoint"
	FindingTypeSecurity      FindingType = "security"
)

// Severity represents the severity of a finding
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Location represents a location in the repository
type Location struct {
	Path       string
	LineNumber int
	Column     int
	Section    string
}

// Evidence represents evidence supporting a finding
type Evidence struct {
	Type        string
	Description string
	Location    *Location
	Value       interface{}
}

// AnalysisOrchestrator coordinates multiple analysis engines
type AnalysisOrchestrator struct {
	engines []AnalysisEngine
	logger  zerolog.Logger
}

// NewAnalysisOrchestrator creates a new analysis orchestrator
func NewAnalysisOrchestrator(logger zerolog.Logger) *AnalysisOrchestrator {
	return &AnalysisOrchestrator{
		engines: make([]AnalysisEngine, 0),
		logger:  logger.With().Str("component", "orchestrator").Logger(),
	}
}

// RegisterEngine registers an analysis engine
func (o *AnalysisOrchestrator) RegisterEngine(engine AnalysisEngine) {
	o.engines = append(o.engines, engine)
	o.logger.Debug().Str("engine", engine.GetName()).Msg("Analysis engine registered")
}

// Analyze runs all applicable engines and aggregates results
func (o *AnalysisOrchestrator) Analyze(ctx context.Context, config AnalysisConfig) (*CombinedAnalysisResult, error) {
	result := &CombinedAnalysisResult{
		StartTime:     time.Now(),
		EngineResults: make(map[string]*EngineAnalysisResult),
		AllFindings:   make([]Finding, 0),
		Summary:       make(map[string]interface{}),
	}

	// Run applicable engines
	for _, engine := range o.engines {
		if !engine.IsApplicable(ctx, config.RepoData) {
			o.logger.Debug().Str("engine", engine.GetName()).Msg("Engine not applicable, skipping")
			continue
		}

		o.logger.Info().Str("engine", engine.GetName()).Msg("Running analysis engine")
		engineResult, err := engine.Analyze(ctx, config)
		if err != nil {
			o.logger.Error().Err(err).Str("engine", engine.GetName()).Msg("Engine analysis failed")
			continue
		}

		result.EngineResults[engine.GetName()] = engineResult
		result.AllFindings = append(result.AllFindings, engineResult.Findings...)
	}

	result.Duration = time.Since(result.StartTime)
	result.Summary = o.generateSummary(result)

	return result, nil
}

// CombinedAnalysisResult represents the combined result from all engines
type CombinedAnalysisResult struct {
	StartTime     time.Time
	Duration      time.Duration
	EngineResults map[string]*EngineAnalysisResult
	AllFindings   []Finding
	Summary       map[string]interface{}
}

// generateSummary generates a summary of all analysis results
func (o *AnalysisOrchestrator) generateSummary(result *CombinedAnalysisResult) map[string]interface{} {
	summary := map[string]interface{}{
		"total_engines":  len(result.EngineResults),
		"total_findings": len(result.AllFindings),
		"by_type":        make(map[string]int),
		"by_severity":    make(map[string]int),
		"confidence_avg": 0.0,
	}

	// Aggregate findings by type and severity
	var confidenceSum float64
	for _, finding := range result.AllFindings {
		// Safely access type map
		if typeMap, ok := summary["by_type"].(map[string]int); ok {
			typeMap[string(finding.Type)]++
		}
		// Safely access severity map
		if severityMap, ok := summary["by_severity"].(map[string]int); ok {
			severityMap[string(finding.Severity)]++
		}
		confidenceSum += finding.Confidence
	}

	if len(result.AllFindings) > 0 {
		summary["confidence_avg"] = confidenceSum / float64(len(result.AllFindings))
	}

	return summary
}

// GetEngineNames returns the names of all registered engines
func (o *AnalysisOrchestrator) GetEngineNames() []string {
	names := make([]string, len(o.engines))
	for i, engine := range o.engines {
		names[i] = engine.GetName()
	}
	return names
}

// GetEngine returns an engine by name
func (o *AnalysisOrchestrator) GetEngine(name string) AnalysisEngine {
	for _, engine := range o.engines {
		if engine.GetName() == name {
			return engine
		}
	}
	return nil
}
