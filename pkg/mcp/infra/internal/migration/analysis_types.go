package migration

import (
	"go/ast"
	"go/token"
	"regexp"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infra/logging"
)

// Detector automatically detects migration opportunities in the codebase
type Detector struct {
	logger    logging.Standards
	config    Config
	fileSet   *token.FileSet
	patterns  map[string]*regexp.Regexp
	analyzers map[string]AnalyzerFunc
}

// Config defines migration detection configuration
type Config struct {
	EnablePatternDetection     bool              `json:"enable_pattern_detection"`
	EnableStructuralAnalysis   bool              `json:"enable_structural_analysis"`
	EnableDeprecationDetection bool              `json:"enable_deprecation_detection"`
	IgnoreDirectories          []string          `json:"ignore_directories"`
	IgnoreFiles                []string          `json:"ignore_files"`
	CustomPatterns             map[string]string `json:"custom_patterns"`
	MaxDepth                   int               `json:"max_depth"`
	EnableReporting            bool              `json:"enable_reporting"`
}

// PatternAnalyzer analyzes code patterns to identify refactoring opportunities
type PatternAnalyzer struct {
	logger     logging.Standards
	config     PatternAnalysisConfig
	fileSet    *token.FileSet
	statistics PatternStatistics
}

// PatternAnalysisConfig defines pattern analysis configuration
type PatternAnalysisConfig struct {
	EnableComplexityAnalysis   bool              `json:"enable_complexity_analysis"`
	EnableDuplicationDetection bool              `json:"enable_duplication_detection"`
	EnableAntiPatternDetection bool              `json:"enable_anti_pattern_detection"`
	ComplexityThreshold        int               `json:"complexity_threshold"`
	DuplicationThreshold       float64           `json:"duplication_threshold"`
	CustomAntiPatterns         map[string]string `json:"custom_anti_patterns"`
}

// AnalyzerFunc defines a function that analyzes code for migration opportunities
type AnalyzerFunc func(*ast.File, *token.FileSet) []Opportunity

// Opportunity represents a detected migration opportunity
type Opportunity struct {
	Type            string                 `json:"type"`
	Priority        string                 `json:"priority"`   // HIGH, MEDIUM, LOW
	Confidence      float64                `json:"confidence"` // 0.0 to 1.0
	File            string                 `json:"file"`
	Line            int                    `json:"line"`
	Column          int                    `json:"column"`
	Description     string                 `json:"description"`
	Suggestion      string                 `json:"suggestion"`
	Context         map[string]interface{} `json:"context"`
	EstimatedEffort string                 `json:"estimated_effort"` // TRIVIAL, MINOR, MAJOR, CRITICAL
	Dependencies    []string               `json:"dependencies"`
	Examples        []CodeExample          `json:"examples,omitempty"`
}

// CodeExample shows before and after code snippets
type CodeExample struct {
	Title  string `json:"title"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// Report contains comprehensive migration analysis results
type Report struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	TotalFiles      int                    `json:"total_files"`
	AnalyzedFiles   int                    `json:"analyzed_files"`
	Opportunities   []Opportunity          `json:"opportunities"`
	Statistics      Statistics             `json:"statistics"`
	Recommendations []string               `json:"recommendations"`
	Summary         ReportSummary          `json:"summary"`
	PatternAnalysis *PatternAnalysisResult `json:"pattern_analysis,omitempty"`
	EstimatedEffort EffortEstimate         `json:"estimated_effort"`
}

// Statistics contains statistical information about migrations
type Statistics struct {
	ByType       map[string]int `json:"by_type"`
	ByPriority   map[string]int `json:"by_priority"`
	ByConfidence map[string]int `json:"by_confidence"`
	ByEffort     map[string]int `json:"by_effort"`
	ByFile       map[string]int `json:"by_file"`
}

// ReportSummary provides a high-level summary of migration opportunities
type ReportSummary struct {
	TotalOpportunities       int      `json:"total_opportunities"`
	HighPriorityCount        int      `json:"high_priority_count"`
	MediumPriorityCount      int      `json:"medium_priority_count"`
	LowPriorityCount         int      `json:"low_priority_count"`
	AverageConfidence        float64  `json:"average_confidence"`
	EstimatedTotalEffort     string   `json:"estimated_total_effort"`
	MostCommonType           string   `json:"most_common_type"`
	MostCommonTypeCount      int      `json:"most_common_type_count"`
	FilesMostImpacted        []string `json:"files_most_impacted"`
	RecommendedStartingPoint string   `json:"recommended_starting_point"`
}

// PatternAnalysisResult contains results from pattern analysis
type PatternAnalysisResult struct {
	ComplexityHotspots []ComplexityHotspot    `json:"complexity_hotspots"`
	DuplicationGroups  []DuplicationGroup     `json:"duplication_groups"`
	AntiPatterns       []AntiPatternDetection `json:"anti_patterns"`
	Metrics            CodeMetrics            `json:"metrics"`
}

// ComplexityHotspot identifies areas of high cyclomatic complexity
type ComplexityHotspot struct {
	File           string         `json:"file"`
	Function       string         `json:"function"`
	Complexity     int            `json:"complexity"`
	LinesOfCode    int            `json:"lines_of_code"`
	Position       token.Position `json:"position"`
	Recommendation string         `json:"recommendation"`
}

// DuplicationGroup represents a group of duplicated code instances
type DuplicationGroup struct {
	ID          string                `json:"id"`
	Instances   []DuplicationInstance `json:"instances"`
	LineCount   int                   `json:"line_count"`
	Similarity  float64               `json:"similarity"`
	ImpactScore float64               `json:"impact_score"`
}

// DuplicationInstance represents a single instance of duplicated code
type DuplicationInstance struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	CodeHash  string `json:"code_hash"`
}

// AntiPatternDetection represents a detected anti-pattern
type AntiPatternDetection struct {
	Type        string                 `json:"type"`
	File        string                 `json:"file"`
	Position    token.Position         `json:"position"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity"`
	Suggestion  string                 `json:"suggestion"`
	Context     map[string]interface{} `json:"context"`
}

// CodeMetrics contains code quality metrics
type CodeMetrics struct {
	TotalLines         int     `json:"total_lines"`
	TotalFunctions     int     `json:"total_functions"`
	AverageComplexity  float64 `json:"average_complexity"`
	MaxComplexity      int     `json:"max_complexity"`
	DuplicationRatio   float64 `json:"duplication_ratio"`
	TechnicalDebtScore float64 `json:"technical_debt_score"`
}

// PatternStatistics tracks pattern detection statistics
type PatternStatistics struct {
	TotalFiles       int            `json:"total_files"`
	FilesAnalyzed    int            `json:"files_analyzed"`
	PatternsDetected map[string]int `json:"patterns_detected"`
	TotalDetections  int            `json:"total_detections"`
	DetectionTime    time.Duration  `json:"detection_time"`
}

// EffortEstimate provides effort estimation for migrations
type EffortEstimate struct {
	TotalEffortHours     float64            `json:"total_effort_hours"`
	EffortByType         map[string]float64 `json:"effort_by_type"`
	EffortByPriority     map[string]float64 `json:"effort_by_priority"`
	ResourceRequirements []string           `json:"resource_requirements"`
	Timeline             TimelineEstimate   `json:"timeline"`
	RiskFactors          []string           `json:"risk_factors"`
	Dependencies         []string           `json:"dependencies"`
}

// TimelineEstimate provides timeline information for migrations
type TimelineEstimate struct {
	MinDays         int     `json:"min_days"`
	MaxDays         int     `json:"max_days"`
	RecommendedDays int     `json:"recommended_days"`
	Phases          []Phase `json:"phases"`
}

// Phase represents a phase in the migration timeline
type Phase struct {
	Name        string   `json:"name"`
	Duration    int      `json:"duration_days"`
	Description string   `json:"description"`
	Tasks       []string `json:"tasks"`
}
