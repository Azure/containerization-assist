package utils

import (
	"time"
)

// RemediationPlan provides a structured approach to addressing issues and improvements
type RemediationPlan struct {
	// Plan identification
	PlanID      string `json:"plan_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`   // critical/high/medium/low
	Complexity  string `json:"complexity"` // simple/moderate/complex

	// Categorization
	Category      string   `json:"category"` // security/performance/maintenance/etc.
	Tags          []string `json:"tags"`     // searchable tags
	AffectedAreas []string `json:"affected_areas"`

	// Steps and alternatives
	Steps        []RemediationStep     `json:"steps"`
	Alternatives []AlternativeStrategy `json:"alternatives"`

	// Timeline and effort
	EstimatedEffort   string        `json:"estimated_effort"`   // 15min/1h/1d/1w/etc.
	EstimatedDuration time.Duration `json:"estimated_duration"` // machine-readable duration
	Prerequisites     []string      `json:"prerequisites"`

	// Risk and validation
	RiskAssessment  RiskAssessment   `json:"risk_assessment"`
	ValidationSteps []ValidationStep `json:"validation_steps"`
	RollbackPlan    []string         `json:"rollback_plan"`

	// Success criteria
	SuccessMetrics     []SuccessMetric `json:"success_metrics"`
	AcceptanceCriteria []string        `json:"acceptance_criteria"`

	// Context and reasoning
	Reasoning    []string               `json:"reasoning"`
	Assumptions  []string               `json:"assumptions"`
	Dependencies []string               `json:"dependencies"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// RemediationStep represents a single step in a remediation plan
type RemediationStep struct {
	StepID      string `json:"step_id"`
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Description string `json:"description"`

	// Action details
	Action   string `json:"action"`              // create/modify/delete/verify/etc.
	Target   string `json:"target"`              // what to act on
	Command  string `json:"command,omitempty"`   // command to run
	ToolCall string `json:"tool_call,omitempty"` // MCP tool to call

	// Validation and safety
	ExpectedResult  string   `json:"expected_result"`
	VerificationCmd string   `json:"verification_cmd,omitempty"`
	RollbackAction  string   `json:"rollback_action,omitempty"`
	SafetyChecks    []string `json:"safety_checks"`

	// Context
	Notes      string                 `json:"notes,omitempty"`
	Warnings   []string               `json:"warnings"`
	References []string               `json:"references"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// AlternativeStrategy represents an alternative approach to solving the problem
type AlternativeStrategy struct {
	StrategyID  string `json:"strategy_id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Analysis
	Approach    string             `json:"approach"` // automated/manual/hybrid
	TradeOffs   []TradeoffAnalysis `json:"trade_offs"`
	Suitability string             `json:"suitability"` // best_for_beginners/advanced_users/etc.

	// Implementation
	Steps      []RemediationStep `json:"steps"`
	Timeline   string            `json:"timeline"`
	Complexity string            `json:"complexity"` // simple/moderate/complex

	// Comparison
	Benefits   []string `json:"benefits"`
	Drawbacks  []string `json:"drawbacks"`
	RiskLevel  string   `json:"risk_level"` // low/medium/high
	Confidence int      `json:"confidence"` // 0-100

	// Context
	BestFor  []string               `json:"best_for"` // scenarios where this is preferred
	AvoidIf  []string               `json:"avoid_if"` // scenarios to avoid this approach
	Metadata map[string]interface{} `json:"metadata"`
}

// RiskAssessment provides structured risk analysis for remediation
type RiskAssessment struct {
	OverallRisk     string       `json:"overall_risk"` // low/medium/high/critical
	RiskFactors     []RiskFactor `json:"risk_factors"`
	MitigationSteps []string     `json:"mitigation_steps"`

	// Impact analysis
	PotentialImpact     []ImpactArea `json:"potential_impact"`
	RecoveryTime        string       `json:"recovery_time"`        // if things go wrong
	BusinessCriticality string       `json:"business_criticality"` // low/medium/high

	// Safety measures
	TestingStrategy  string   `json:"testing_strategy"`
	MonitoringPoints []string `json:"monitoring_points"`
	EscalationPath   []string `json:"escalation_path"`
}

// ImpactArea describes potential impact in specific areas
type ImpactArea struct {
	Area        string `json:"area"` // performance/security/availability/etc.
	Description string `json:"description"`
	Likelihood  string `json:"likelihood"` // low/medium/high
	Severity    string `json:"severity"`   // low/medium/high/critical
	Duration    string `json:"duration"`   // temporary/permanent
}

// ValidationStep represents a step to validate remediation success
type ValidationStep struct {
	StepID      string `json:"step_id"`
	Order       int    `json:"order"`
	Description string `json:"description"`

	// Validation method
	Method   string `json:"method"` // manual/automated/tool
	Command  string `json:"command,omitempty"`
	ToolCall string `json:"tool_call,omitempty"`

	// Success criteria
	ExpectedResult    string   `json:"expected_result"`
	SuccessIndicators []string `json:"success_indicators"`
	FailureIndicators []string `json:"failure_indicators"`

	// Context
	Notes    string                 `json:"notes,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
}

// SuccessMetric defines measurable success criteria
type SuccessMetric struct {
	MetricID    string `json:"metric_id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Measurement
	Type      string      `json:"type"`      // numeric/boolean/percentage/etc.
	Target    interface{} `json:"target"`    // target value
	Threshold interface{} `json:"threshold"` // minimum acceptable value
	Unit      string      `json:"unit,omitempty"`

	// Collection
	MeasurementMethod  string `json:"measurement_method"`
	CollectionCommand  string `json:"collection_command,omitempty"`
	CollectionInterval string `json:"collection_interval,omitempty"`

	// Context
	Baseline interface{}            `json:"baseline,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Recommendation provides AI-friendly recommendations with rich context
type Recommendation struct {
	RecommendationID string `json:"recommendation_id"`
	Title            string `json:"title"`
	Description      string `json:"description"`

	// Categorization
	Category string   `json:"category"` // security/performance/maintenance/etc.
	Priority string   `json:"priority"` // critical/high/medium/low
	Type     string   `json:"type"`     // fix/improvement/optimization/etc.
	Tags     []string `json:"tags"`

	// Implementation guidance
	ActionType     string                `json:"action_type"` // immediate/planned/optional
	Implementation RemediationPlan       `json:"implementation"`
	Alternatives   []AlternativeStrategy `json:"alternatives"`

	// Decision support
	Benefits      []string `json:"benefits"`
	Risks         []string `json:"risks"`
	Prerequisites []string `json:"prerequisites"`
	Dependencies  []string `json:"dependencies"`

	// Context and reasoning
	Reasoning   []string       `json:"reasoning"`
	Evidence    []EvidenceItem `json:"evidence"`
	Assumptions []string       `json:"assumptions"`
	Confidence  int            `json:"confidence"` // 0-100

	// Timing and effort
	Urgency string `json:"urgency"` // immediate/soon/eventually
	Effort  string `json:"effort"`  // low/medium/high
	Impact  string `json:"impact"`  // low/medium/high

	// Lifecycle
	ApplicableUntil time.Time `json:"applicable_until,omitempty"`
	ReviewDate      time.Time `json:"review_date,omitempty"`

	// Rich metadata
	Metadata map[string]interface{} `json:"metadata"`
}

// ToolContext provides rich AI context for any tool result
type ToolContext struct {
	// Basic context
	ToolName    string    `json:"tool_name"`
	OperationID string    `json:"operation_id"`
	Timestamp   time.Time `json:"timestamp"`

	// Assessment context
	Assessment      *UnifiedAssessment `json:"assessment"`
	Recommendations []Recommendation   `json:"recommendations"`

	// Decision context
	DecisionPoints []DecisionPoint    `json:"decision_points"`
	TradeOffs      []TradeoffAnalysis `json:"trade_offs"`

	// Rich insights
	Insights       []ContextualInsight `json:"insights"`
	LessonsLearned []string            `json:"lessons_learned"`
	BestPractices  []string            `json:"best_practices"`

	// Quality indicators
	QualityMetrics  map[string]interface{} `json:"quality_metrics"`
	PerformanceData map[string]interface{} `json:"performance_data"`

	// AI reasoning support
	ReasoningContext map[string]interface{} `json:"reasoning_context"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// DecisionPoint represents a key decision made during operation
type DecisionPoint struct {
	DecisionID  string `json:"decision_id"`
	Title       string `json:"title"`
	Description string `json:"description"`

	// Decision details
	Chosen       string   `json:"chosen"`
	Alternatives []string `json:"alternatives"`
	Reasoning    []string `json:"reasoning"`
	Confidence   int      `json:"confidence"` // 0-100

	// Context
	Factors     []DecisionFactor `json:"factors"`
	Constraints []string         `json:"constraints"`
	Assumptions []string         `json:"assumptions"`

	// Impact
	Impact     string                 `json:"impact"` // low/medium/high
	Reversible bool                   `json:"reversible"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ContextualInsight provides rich insights for AI reasoning
type ContextualInsight struct {
	InsightID   string `json:"insight_id"`
	Type        string `json:"type"` // pattern/anomaly/optimization/etc.
	Title       string `json:"title"`
	Description string `json:"description"`

	// Insight details
	Observation  string   `json:"observation"`
	Implications []string `json:"implications"`
	Actionable   bool     `json:"actionable"`

	// Relevance
	Relevance  string `json:"relevance"`  // high/medium/low
	Confidence int    `json:"confidence"` // 0-100
	Source     string `json:"source"`     // analysis/metrics/pattern/etc.

	// Context
	Evidence   []string               `json:"evidence"`
	References []string               `json:"references"`
	Metadata   map[string]interface{} `json:"metadata"`
}
