package build

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core"
)

// UnifiedAnalyzer combines AI and tool-specific analysis capabilities
type UnifiedAnalyzer interface {
	// Core AI capabilities from AIAnalyzer interface
	mcp.AIAnalyzer
	// Enhanced analysis capabilities
	AnalyzeFailure(ctx context.Context, failure *AnalysisRequest) (*AnalysisResult, error)
	GetCapabilities() *AnalyzerCapabilities
	// Repository integration
	AnalyzeWithRepository(ctx context.Context, request *RepositoryAnalysisRequest) (*RepositoryAwareAnalysis, error)
	// Cross-tool coordination
	ShareInsights(ctx context.Context, insights *ToolInsights) error
	GetSharedKnowledge(ctx context.Context, domain string) (*SharedKnowledge, error)
	// Performance optimization
	OptimizeBuildStrategy(ctx context.Context, request *BuildOptimizationRequest) (*OptimizedBuildStrategy, error)
	PredictFailures(ctx context.Context, buildContext *AnalysisBuildContext) (*FailurePrediction, error)
}

// AnalyzerCapabilities defines what an analyzer can do
type AnalyzerCapabilities struct {
	SupportedFailureTypes  []string `json:"supported_failure_types"`
	SupportedLanguages     []string `json:"supported_languages"`
	CanAnalyzeRepository   bool     `json:"can_analyze_repository"`
	CanGenerateFixes       bool     `json:"can_generate_fixes"`
	CanOptimizePerformance bool     `json:"can_optimize_performance"`
	CanAssessSecurity      bool     `json:"can_assess_security"`
	CanPredictFailures     bool     `json:"can_predict_failures"`
	CanShareKnowledge      bool     `json:"can_share_knowledge"`
}

// AnalysisRequest represents a failure analysis request
type AnalysisRequest struct {
	SessionID     string                 `json:"session_id"`
	ToolName      string                 `json:"tool_name"`
	OperationType string                 `json:"operation_type"`
	Error         error                  `json:"error"`
	Context       map[string]interface{} `json:"context"`
	WorkspaceDir  string                 `json:"workspace_dir"`
	HistoryLimit  int                    `json:"history_limit,omitempty"`
}

// AnalysisResult contains comprehensive failure analysis
type AnalysisResult struct {
	FailureType      string                 `json:"failure_type"`
	RootCause        string                 `json:"root_cause"`
	Severity         string                 `json:"severity"`
	IsRetryable      bool                   `json:"is_retryable"`
	FixStrategies    []*FixStrategy         `json:"fix_strategies"`
	RelatedFailures  []*RelatedFailure      `json:"related_failures"`
	ImpactAssessment *ImpactAssessment      `json:"impact_assessment"`
	Recommendations  []string               `json:"recommendations"`
	AnalysisMetadata map[string]interface{} `json:"analysis_metadata"`
	AnalysisDuration time.Duration          `json:"analysis_duration"`
	ConfidenceScore  float64                `json:"confidence_score"`
}

// RepositoryAnalysisRequest combines repository info with analysis context
type RepositoryAnalysisRequest struct {
	AnalysisRequest
	RepositoryInfo  *core.RepositoryInfo `json:"repository_info"` // Use core interface
	ProjectMetadata *ProjectMetadata     `json:"project_metadata"`
	BuildHistory    []*BuildHistoryEntry `json:"build_history"`
}

// RepositoryAwareAnalysis includes repository-specific insights
type RepositoryAwareAnalysis struct {
	*AnalysisResult
	ProjectSpecificInsights *ProjectInsights         `json:"project_specific_insights"`
	LanguageRecommendations []string                 `json:"language_recommendations"`
	FrameworkOptimizations  []*FrameworkOptimization `json:"framework_optimizations"`
	DependencyAnalysis      *DependencyAnalysis      `json:"dependency_analysis"`
	SecurityImplications    *GeneralSecurityAnalysis `json:"security_implications"`
}

// ToolInsights represents insights to share across tools
type ToolInsights struct {
	ToolName           string                 `json:"tool_name"`
	OperationType      string                 `json:"operation_type"`
	SuccessPattern     *SuccessPattern        `json:"success_pattern,omitempty"`
	FailurePattern     *FailurePattern        `json:"failure_pattern,omitempty"`
	OptimizationTips   []string               `json:"optimization_tips"`
	PerformanceMetrics *PerformanceMetrics    `json:"performance_metrics"`
	Metadata           map[string]interface{} `json:"metadata"`
	Timestamp          time.Time              `json:"timestamp"`
}

// SharedKnowledge represents accumulated knowledge from all tools
type SharedKnowledge struct {
	Domain           string                    `json:"domain"`
	CommonPatterns   []*FailurePattern         `json:"common_patterns"`
	BestPractices    []string                  `json:"best_practices"`
	OptimizationTips []*GeneralOptimizationTip `json:"optimization_tips"`
	SuccessMetrics   *AggregatedMetrics        `json:"success_metrics"`
	LastUpdated      time.Time                 `json:"last_updated"`
	SourceTools      []string                  `json:"source_tools"`
}

// BuildOptimizationRequest requests build strategy optimization
type BuildOptimizationRequest struct {
	SessionID       string                  `json:"session_id"`
	ProjectType     string                  `json:"project_type"`
	CurrentStrategy *OptimizedBuildStrategy `json:"current_strategy"`
	Constraints     *BuildConstraints       `json:"constraints"`
	Goals           *OptimizationGoals      `json:"goals"`
	Context         map[string]interface{}  `json:"context"`
}

// OptimizedBuildStrategy represents an optimized build approach
type OptimizedBuildStrategy struct {
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Steps            []*BuildStep           `json:"steps"`
	ExpectedDuration time.Duration          `json:"expected_duration"`
	ResourceUsage    *ResourceEstimate      `json:"resource_usage"`
	RiskAssessment   *RiskAssessment        `json:"risk_assessment"`
	Advantages       []string               `json:"advantages"`
	Disadvantages    []string               `json:"disadvantages"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// FailurePrediction predicts potential build failures
type FailurePrediction struct {
	PotentialFailures []*PredictedFailure `json:"potential_failures"`
	RiskScore         float64             `json:"risk_score"`
	PreventiveActions []string            `json:"preventive_actions"`
	MonitoringPoints  []string            `json:"monitoring_points"`
	ConfidenceLevel   float64             `json:"confidence_level"`
}

// Supporting types for comprehensive analysis
type FixStrategy struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Steps         []string               `json:"steps"`
	EstimatedTime time.Duration          `json:"estimated_time"`
	SuccessRate   float64                `json:"success_rate"`
	RiskLevel     string                 `json:"risk_level"`
	Prerequisites []string               `json:"prerequisites"`
	Metadata      map[string]interface{} `json:"metadata"`
}
type RelatedFailure struct {
	FailureType  string    `json:"failure_type"`
	Similarity   float64   `json:"similarity"`
	Resolution   string    `json:"resolution"`
	LastOccurred time.Time `json:"last_occurred"`
	Frequency    int       `json:"frequency"`
}
type ImpactAssessment struct {
	BusinessImpact  string        `json:"business_impact"`
	TechnicalImpact string        `json:"technical_impact"`
	UserImpact      string        `json:"user_impact"`
	RecoveryTime    time.Duration `json:"recovery_time"`
	CostEstimate    float64       `json:"cost_estimate"`
	UrgencyScore    float64       `json:"urgency_score"`
}
type ProjectMetadata struct {
	Language     string                 `json:"language"`
	Framework    string                 `json:"framework"`
	Dependencies []string               `json:"dependencies"`
	BuildSystem  string                 `json:"build_system"`
	ProjectSize  string                 `json:"project_size"`
	Complexity   string                 `json:"complexity"`
	Attributes   map[string]interface{} `json:"attributes"`
}
type BuildHistoryEntry struct {
	Timestamp    time.Time      `json:"timestamp"`
	Success      bool           `json:"success"`
	Duration     time.Duration  `json:"duration"`
	ErrorType    string         `json:"error_type,omitempty"`
	FixApplied   string         `json:"fix_applied,omitempty"`
	ResourceUsed *ResourceUsage `json:"resource_used"`
}
type ProjectInsights struct {
	CodeQuality     *QualityMetrics       `json:"code_quality"`
	Architecture    *ArchitectureInsights `json:"architecture"`
	TechDebt        *TechDebtAnalysis     `json:"tech_debt"`
	Maintainability *MaintainabilityScore `json:"maintainability"`
	TestCoverage    *TestCoverageInfo     `json:"test_coverage"`
}
type FrameworkOptimization struct {
	Framework     string   `json:"framework"`
	Optimizations []string `json:"optimizations"`
	Impact        string   `json:"impact"`
	Difficulty    string   `json:"difficulty"`
}
type DependencyAnalysis struct {
	OutdatedDeps    []string            `json:"outdated_deps"`
	SecurityIssues  []string            `json:"security_issues"`
	LicenseIssues   []string            `json:"license_issues"`
	SizeImpact      *SizeImpactAnalysis `json:"size_impact"`
	Recommendations []string            `json:"recommendations"`
}
type GeneralSecurityAnalysis struct {
	VulnerabilityCount int      `json:"vulnerability_count"`
	RiskLevel          string   `json:"risk_level"`
	CriticalIssues     []string `json:"critical_issues"`
	Recommendations    []string `json:"recommendations"`
}
type SuccessPattern struct {
	PatternName string                 `json:"pattern_name"`
	Conditions  []string               `json:"conditions"`
	Actions     []string               `json:"actions"`
	SuccessRate float64                `json:"success_rate"`
	Context     map[string]interface{} `json:"context"`
}
type FailurePattern struct {
	PatternName      string                 `json:"pattern_name"`
	FailureType      string                 `json:"failure_type"`
	CommonCauses     []string               `json:"common_causes"`
	TypicalSolutions []string               `json:"typical_solutions"`
	Frequency        int                    `json:"frequency"`
	Context          map[string]interface{} `json:"context"`
}
type GeneralOptimizationTip struct {
	Category      string  `json:"category"`
	Tip           string  `json:"tip"`
	Impact        string  `json:"impact"`
	Difficulty    string  `json:"difficulty"`
	Applicability float64 `json:"applicability"`
}
type AggregatedMetrics struct {
	TotalOperations  int           `json:"total_operations"`
	SuccessRate      float64       `json:"success_rate"`
	AverageTime      time.Duration `json:"average_time"`
	ImprovementTrend float64       `json:"improvement_trend"`
	CommonIssueTypes []string      `json:"common_issue_types"`
}
type BuildConstraints struct {
	MaxDuration   time.Duration `json:"max_duration"`
	MaxMemory     int64         `json:"max_memory"`
	MaxCPU        float64       `json:"max_cpu"`
	AllowedTools  []string      `json:"allowed_tools"`
	SecurityLevel string        `json:"security_level"`
}
type OptimizationGoals struct {
	PrimarGoal      string        `json:"primary_goal"` // speed, size, security, reliability
	AcceptableRisk  string        `json:"acceptable_risk"`
	TimeConstraints time.Duration `json:"time_constraints"`
	QualityLevel    string        `json:"quality_level"`
}
type BuildStep struct {
	Name         string            `json:"name"`
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	WorkingDir   string            `json:"working_dir"`
	Environment  map[string]string `json:"environment"`
	ExpectedTime time.Duration     `json:"expected_time"`
	CriticalPath bool              `json:"critical_path"`
	Parallel     bool              `json:"parallel"`
}
type ResourceEstimate struct {
	CPU     float64 `json:"cpu"`
	Memory  int64   `json:"memory"`
	Disk    int64   `json:"disk"`
	Network int64   `json:"network"`
}
type RiskAssessment struct {
	OverallRisk   string   `json:"overall_risk"`
	RiskFactors   []string `json:"risk_factors"`
	Mitigations   []string `json:"mitigations"`
	FailurePoints []string `json:"failure_points"`
}
type PredictedFailure struct {
	FailureType       string   `json:"failure_type"`
	Probability       float64  `json:"probability"`
	TriggerConditions []string `json:"trigger_conditions"`
	PreventiveActions []string `json:"preventive_actions"`
	ImpactLevel       string   `json:"impact_level"`
}

// Additional supporting types
type QualityMetrics struct {
	OverallScore    float64 `json:"overall_score"`
	Maintainability float64 `json:"maintainability"`
	Reliability     float64 `json:"reliability"`
	Security        float64 `json:"security"`
	Performance     float64 `json:"performance"`
}
type ArchitectureInsights struct {
	Pattern         string   `json:"pattern"`
	Complexity      string   `json:"complexity"`
	Modularity      float64  `json:"modularity"`
	CouplingLevel   string   `json:"coupling_level"`
	Recommendations []string `json:"recommendations"`
}
type TechDebtAnalysis struct {
	DebtLevel        string        `json:"debt_level"`
	CriticalAreas    []string      `json:"critical_areas"`
	RefactoringNeeds []string      `json:"refactoring_needs"`
	EstimatedCost    time.Duration `json:"estimated_cost"`
}
type MaintainabilityScore struct {
	Score            float64  `json:"score"`
	Factors          []string `json:"factors"`
	ImprovementAreas []string `json:"improvement_areas"`
}
type TestCoverageInfo struct {
	LineCoverage   float64  `json:"line_coverage"`
	BranchCoverage float64  `json:"branch_coverage"`
	UncoveredAreas []string `json:"uncovered_areas"`
	TestQuality    string   `json:"test_quality"`
}
type SizeImpactAnalysis struct {
	TotalSize       int64    `json:"total_size"`
	LargestDeps     []string `json:"largest_deps"`
	OptimizationOps []string `json:"optimization_opportunities"`
	SizeReduction   int64    `json:"potential_size_reduction"`
}
type ResourceUsage struct {
	CPU      float64       `json:"cpu"`
	Memory   int64         `json:"memory"`
	Disk     int64         `json:"disk"`
	Network  int64         `json:"network"`
	Duration time.Duration `json:"duration"`
}
type AnalysisBuildContext struct {
	SessionID    string                 `json:"session_id"`
	ProjectInfo  *ProjectMetadata       `json:"project_info"`
	BuildHistory []*BuildHistoryEntry   `json:"build_history"`
	CurrentState *BuildState            `json:"current_state"`
	Environment  map[string]interface{} `json:"environment"`
	Constraints  *BuildConstraints      `json:"constraints"`
}
type BuildState struct {
	Phase            string         `json:"phase"`
	CompletedSteps   []string       `json:"completed_steps"`
	RemainingSteps   []string       `json:"remaining_steps"`
	CurrentResources *ResourceUsage `json:"current_resources"`
	Artifacts        []string       `json:"artifacts"`
	Errors           []string       `json:"errors"`
	Warnings         []string       `json:"warnings"`
}
