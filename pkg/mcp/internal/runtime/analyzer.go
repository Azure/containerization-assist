package runtime

import (
	"context"
	"time"
)

// Analyzer defines the base interface for all analyzers
type Analyzer interface {
	// Analyze performs analysis and returns results
	Analyze(ctx context.Context, input interface{}, options AnalysisOptions) (*AnalysisResult, error)

	// GetName returns the analyzer name
	GetName() string

	// GetCapabilities returns what this analyzer can do
	GetCapabilities() AnalyzerCapabilities
}

// AnalysisOptions provides common options for analysis
type AnalysisOptions struct {
	// Depth of analysis (shallow, normal, deep)
	Depth string

	// Specific aspects to analyze
	Aspects []string

	// Enable recommendations
	GenerateRecommendations bool

	// Custom analysis parameters
	CustomParams map[string]interface{}
}

// AnalysisResult represents the result of analysis
type AnalysisResult struct {
	// Summary of findings
	Summary AnalysisSummary

	// Detailed findings
	Findings []Finding

	// Recommendations based on analysis
	Recommendations []Recommendation

	// Metrics collected during analysis
	Metrics map[string]interface{}

	// Risk assessment
	RiskAssessment RiskAssessment

	// Additional context
	Context  map[string]interface{}
	Metadata AnalysisMetadata
}

// AnalysisSummary provides a high-level summary
type AnalysisSummary struct {
	TotalFindings    int
	CriticalFindings int
	Strengths        []string
	Weaknesses       []string
	OverallScore     int // 0-100
}

// Finding represents a specific finding during analysis
type Finding struct {
	ID          string
	Type        string
	Category    string
	Severity    string
	Title       string
	Description string
	Evidence    []string
	Impact      string
	Location    FindingLocation
}

// FindingLocation provides location information for a finding
type FindingLocation struct {
	File      string
	Line      int
	Component string
	Context   string
}

// Recommendation represents an actionable recommendation
type Recommendation struct {
	ID          string
	Priority    string // high, medium, low
	Category    string
	Title       string
	Description string
	Benefits    []string
	Effort      string // low, medium, high
	Impact      string // low, medium, high
}

// RiskAssessment provides risk analysis
type RiskAssessment struct {
	OverallRisk string // low, medium, high, critical
	RiskFactors []RiskFactor
	Mitigations []Mitigation
}

// RiskFactor represents a specific risk
type RiskFactor struct {
	ID          string
	Category    string
	Description string
	Likelihood  string // low, medium, high
	Impact      string // low, medium, high
	Score       int
}

// Mitigation represents a way to reduce risk
type Mitigation struct {
	RiskID        string
	Description   string
	Effort        string
	Effectiveness string
}

// AnalysisMetadata provides metadata about the analysis
type AnalysisMetadata struct {
	AnalyzerName    string
	AnalyzerVersion string
	Duration        time.Duration
	Timestamp       time.Time
	Parameters      map[string]interface{}
}

// AnalyzerCapabilities describes what an analyzer can do
type AnalyzerCapabilities struct {
	SupportedTypes   []string
	SupportedAspects []string
	RequiresContext  bool
	SupportsDeepScan bool
}

// BaseAnalyzer provides common functionality for analyzers
type BaseAnalyzer struct {
	Name         string
	Version      string
	Capabilities AnalyzerCapabilities
}

// NewBaseAnalyzer creates a new base analyzer
func NewBaseAnalyzer(name, version string, capabilities AnalyzerCapabilities) *BaseAnalyzer {
	return &BaseAnalyzer{
		Name:         name,
		Version:      version,
		Capabilities: capabilities,
	}
}

// GetName returns the analyzer name
func (a *BaseAnalyzer) GetName() string {
	return a.Name
}

// GetCapabilities returns the analyzer capabilities
func (a *BaseAnalyzer) GetCapabilities() AnalyzerCapabilities {
	return a.Capabilities
}

// CreateResult creates a new analysis result with metadata
func (a *BaseAnalyzer) CreateResult() *AnalysisResult {
	return &AnalysisResult{
		Summary: AnalysisSummary{
			Strengths:  make([]string, 0),
			Weaknesses: make([]string, 0),
		},
		Findings:        make([]Finding, 0),
		Recommendations: make([]Recommendation, 0),
		Metrics:         make(map[string]interface{}),
		Context:         make(map[string]interface{}),
		Metadata: AnalysisMetadata{
			AnalyzerName:    a.Name,
			AnalyzerVersion: a.Version,
			Timestamp:       time.Now(),
			Parameters:      make(map[string]interface{}),
		},
	}
}

// AddFinding adds a finding to the analysis result
func (r *AnalysisResult) AddFinding(finding Finding) {
	r.Findings = append(r.Findings, finding)
	r.Summary.TotalFindings++

	if finding.Severity == "critical" || finding.Severity == "high" {
		r.Summary.CriticalFindings++
	}
}

// AddRecommendation adds a recommendation to the analysis result
func (r *AnalysisResult) AddRecommendation(rec Recommendation) {
	r.Recommendations = append(r.Recommendations, rec)
}

// AddStrength adds a strength to the summary
func (r *AnalysisResult) AddStrength(strength string) {
	r.Summary.Strengths = append(r.Summary.Strengths, strength)
}

// AddWeakness adds a weakness to the summary
func (r *AnalysisResult) AddWeakness(weakness string) {
	r.Summary.Weaknesses = append(r.Summary.Weaknesses, weakness)
}

// CalculateScore calculates the overall score based on findings
func (r *AnalysisResult) CalculateScore() {
	score := 100

	// Deduct points for findings based on severity
	for _, finding := range r.Findings {
		switch finding.Severity {
		case "critical":
			score -= 20
		case "high":
			score -= 15
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	// Add points for strengths
	score += len(r.Summary.Strengths) * 2

	// Ensure score is within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	r.Summary.OverallScore = score
}

// CalculateRisk calculates the overall risk assessment
func (r *AnalysisResult) CalculateRisk() {
	if r.RiskAssessment.RiskFactors == nil {
		r.RiskAssessment.RiskFactors = make([]RiskFactor, 0)
	}

	totalScore := 0
	for _, factor := range r.RiskAssessment.RiskFactors {
		// Simple scoring: low=1, medium=2, high=3
		likelihood := scoreRiskLevel(factor.Likelihood)
		impact := scoreRiskLevel(factor.Impact)
		factor.Score = likelihood * impact
		totalScore += factor.Score
	}

	// Determine overall risk level
	avgScore := 0
	if len(r.RiskAssessment.RiskFactors) > 0 {
		avgScore = totalScore / len(r.RiskAssessment.RiskFactors)
	}

	switch {
	case avgScore >= 7:
		r.RiskAssessment.OverallRisk = "critical"
	case avgScore >= 5:
		r.RiskAssessment.OverallRisk = "high"
	case avgScore >= 3:
		r.RiskAssessment.OverallRisk = "medium"
	default:
		r.RiskAssessment.OverallRisk = "low"
	}
}

func scoreRiskLevel(level string) int {
	switch level {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// AnalysisContext provides context for analysis operations
type AnalysisContext struct {
	SessionID  string
	WorkingDir string
	Options    AnalysisOptions
	StartTime  time.Time
	Custom     map[string]interface{}
}

// NewAnalysisContext creates a new analysis context
func NewAnalysisContext(sessionID, workingDir string, options AnalysisOptions) *AnalysisContext {
	return &AnalysisContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Options:    options,
		StartTime:  time.Now(),
		Custom:     make(map[string]interface{}),
	}
}

// Duration returns the elapsed time since analysis started
func (c *AnalysisContext) Duration() time.Duration {
	return time.Since(c.StartTime)
}

// AnalyzerChain allows chaining multiple analyzers
type AnalyzerChain struct {
	analyzers []Analyzer
}

// NewAnalyzerChain creates a new analyzer chain
func NewAnalyzerChain(analyzers ...Analyzer) *AnalyzerChain {
	return &AnalyzerChain{
		analyzers: analyzers,
	}
}

// Analyze runs all analyzers in the chain
func (c *AnalyzerChain) Analyze(ctx context.Context, input interface{}, options AnalysisOptions) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Findings:        make([]Finding, 0),
		Recommendations: make([]Recommendation, 0),
		Metrics:         make(map[string]interface{}),
		Context:         make(map[string]interface{}),
	}

	// Run each analyzer
	for _, analyzer := range c.analyzers {
		aResult, err := analyzer.Analyze(ctx, input, options)
		if err != nil {
			return nil, err
		}

		// Merge results
		result.Findings = append(result.Findings, aResult.Findings...)
		result.Recommendations = append(result.Recommendations, aResult.Recommendations...)
		result.Summary.Strengths = append(result.Summary.Strengths, aResult.Summary.Strengths...)
		result.Summary.Weaknesses = append(result.Summary.Weaknesses, aResult.Summary.Weaknesses...)

		// Merge metrics and context
		for k, v := range aResult.Metrics {
			result.Metrics[k] = v
		}
		for k, v := range aResult.Context {
			result.Context[k] = v
		}
	}

	// Update summary
	result.Summary.TotalFindings = len(result.Findings)
	for _, f := range result.Findings {
		if f.Severity == "critical" || f.Severity == "high" {
			result.Summary.CriticalFindings++
		}
	}

	// Calculate final score and risk
	result.CalculateScore()
	result.CalculateRisk()

	return result, nil
}

// GetName returns the chain name
func (c *AnalyzerChain) GetName() string {
	return "AnalyzerChain"
}

// GetCapabilities returns combined capabilities
func (c *AnalyzerChain) GetCapabilities() AnalyzerCapabilities {
	caps := AnalyzerCapabilities{
		SupportedTypes:   make([]string, 0),
		SupportedAspects: make([]string, 0),
	}

	// Combine capabilities from all analyzers
	typeMap := make(map[string]bool)
	aspectMap := make(map[string]bool)

	for _, analyzer := range c.analyzers {
		aCaps := analyzer.GetCapabilities()

		for _, t := range aCaps.SupportedTypes {
			typeMap[t] = true
		}
		for _, a := range aCaps.SupportedAspects {
			aspectMap[a] = true
		}

		if aCaps.RequiresContext {
			caps.RequiresContext = true
		}
		if aCaps.SupportsDeepScan {
			caps.SupportsDeepScan = true
		}
	}

	// Convert maps to slices
	for t := range typeMap {
		caps.SupportedTypes = append(caps.SupportedTypes, t)
	}
	for a := range aspectMap {
		caps.SupportedAspects = append(caps.SupportedAspects, a)
	}

	return caps
}
