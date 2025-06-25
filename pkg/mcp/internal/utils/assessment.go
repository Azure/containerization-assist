package utils

// UnifiedAssessment provides a standardized assessment structure for all tools
type UnifiedAssessment struct {
	// Core Assessment
	ReadinessScore  int    `json:"readiness_score"`  // 0-100
	RiskLevel       string `json:"risk_level"`       // critical/high/medium/low
	ConfidenceLevel int    `json:"confidence_level"` // 0-100
	OverallHealth   string `json:"overall_health"`   // excellent/good/fair/poor

	// Strength and Challenge Analysis
	StrengthAreas  []AssessmentArea `json:"strength_areas"`
	ChallengeAreas []AssessmentArea `json:"challenge_areas"`
	RiskFactors    []RiskFactor     `json:"risk_factors"`

	// AI Reasoning Context
	DecisionFactors   []DecisionFactor       `json:"decision_factors"`
	AssessmentBasis   []EvidenceItem         `json:"assessment_basis"`
	QualityIndicators map[string]interface{} `json:"quality_indicators"`

	// Recommendations
	RecommendedApproach string   `json:"recommended_approach"`
	NextSteps           []string `json:"next_steps"`
	ConsiderationsNote  string   `json:"considerations_note"`
}

// AssessmentArea represents a specific area of strength or challenge
type AssessmentArea struct {
	Area        string   `json:"area"`
	Category    string   `json:"category"` // technical/operational/security/etc.
	Description string   `json:"description"`
	Impact      string   `json:"impact"` // low/medium/high
	Evidence    []string `json:"evidence"`
	Score       int      `json:"score"` // 0-100 for this specific area
}

// RiskFactor represents a potential risk with mitigation strategies
type RiskFactor struct {
	Risk           string   `json:"risk"`
	Category       string   `json:"category"`      // security/performance/maintenance/etc.
	Likelihood     string   `json:"likelihood"`    // low/medium/high
	Impact         string   `json:"impact"`        // low/medium/high
	CurrentLevel   string   `json:"current_level"` // critical/high/medium/low
	Mitigation     string   `json:"mitigation"`
	PreventionTips []string `json:"prevention_tips"`
}

// DecisionFactor represents factors that influence AI decision-making
type DecisionFactor struct {
	Factor    string                 `json:"factor"`
	Weight    float64                `json:"weight"` // 0.0-1.0
	Value     interface{}            `json:"value"`
	Reasoning string                 `json:"reasoning"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// EvidenceItem represents evidence supporting an assessment
type EvidenceItem struct {
	Type        string                 `json:"type"` // file/metric/pattern/etc.
	Source      string                 `json:"source"`
	Description string                 `json:"description"`
	Weight      float64                `json:"weight"` // 0.0-1.0
	Details     map[string]interface{} `json:"details"`
}

// TradeoffAnalysis provides structured analysis of choices and their implications
type TradeoffAnalysis struct {
	Option   string `json:"option"`
	Category string `json:"category"`

	// Benefit Analysis
	Benefits     []Benefit `json:"benefits"`
	TotalBenefit int       `json:"total_benefit"` // 0-100

	// Cost Analysis
	Costs     []Cost `json:"costs"`
	TotalCost int    `json:"total_cost"` // 0-100

	// Risk Analysis
	Risks     []Risk `json:"risks"`
	TotalRisk int    `json:"total_risk"` // 0-100

	// Implementation Analysis
	Complexity       string `json:"complexity"`        // simple/moderate/complex
	TimeToValue      string `json:"time_to_value"`     // immediate/short/medium/long
	SkillRequirement string `json:"skill_requirement"` // basic/intermediate/advanced

	// Context
	BestForScenarios []string               `json:"best_for_scenarios"`
	Limitations      []string               `json:"limitations"`
	Dependencies     []string               `json:"dependencies"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// Benefit represents a positive outcome of choosing an option
type Benefit struct {
	Description string `json:"description"`
	Category    string `json:"category"`   // performance/security/maintenance/etc.
	Impact      string `json:"impact"`     // low/medium/high
	Likelihood  string `json:"likelihood"` // low/medium/high
	Value       int    `json:"value"`      // 0-100
}

// Cost represents a negative aspect or expense of choosing an option
type Cost struct {
	Description string `json:"description"`
	Category    string `json:"category"`   // time/money/complexity/etc.
	Impact      string `json:"impact"`     // low/medium/high
	Likelihood  string `json:"likelihood"` // low/medium/high
	Value       int    `json:"value"`      // 0-100
}

// Risk represents a potential negative outcome
type Risk struct {
	Description string `json:"description"`
	Category    string `json:"category"`    // security/performance/operational/etc.
	Probability string `json:"probability"` // low/medium/high
	Impact      string `json:"impact"`      // low/medium/high
	Mitigation  string `json:"mitigation"`
	Value       int    `json:"value"` // 0-100
}

// ComparisonMatrix provides a side-by-side comparison of alternatives
type ComparisonMatrix struct {
	Criteria     []ComparisonCriterion     `json:"criteria"`
	Alternatives []string                  `json:"alternatives"`
	Scores       map[string]map[string]int `json:"scores"`  // alternative -> criterion -> score
	Weights      map[string]float64        `json:"weights"` // criterion -> weight
	Totals       map[string]float64        `json:"totals"`  // alternative -> weighted total
	Winner       string                    `json:"winner"`
	Confidence   int                       `json:"confidence"` // 0-100
}

// ComparisonCriterion defines what to compare alternatives on
type ComparisonCriterion struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`    // 0.0-1.0
	Direction   string  `json:"direction"` // higher_better/lower_better
}

// DecisionRecommendation provides AI-friendly decision guidance
type DecisionRecommendation struct {
	RecommendedOption string                 `json:"recommended_option"`
	Confidence        int                    `json:"confidence"` // 0-100
	Reasoning         []string               `json:"reasoning"`
	Assumptions       []string               `json:"assumptions"`
	Conditions        []string               `json:"conditions"`   // when this recommendation applies
	Alternatives      []string               `json:"alternatives"` // other viable options
	RiskMitigation    []string               `json:"risk_mitigation"`
	SuccessMetrics    []string               `json:"success_metrics"`
	Metadata          map[string]interface{} `json:"metadata"`
}
