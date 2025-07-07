package runtime

import (
	"context"
	"time"
)

type BaseAnalyzer interface {
	Analyze(ctx context.Context, input interface{}, options AnalysisOptions) (*AnalysisResult, error)
	GetName() string
	GetCapabilities() AnalyzerCapabilities
}
type AnalysisOptions struct {
	Depth string

	Aspects []string

	GenerateRecommendations bool

	CustomParams map[string]interface{}
}
type AnalysisResult struct {
	Summary AnalysisSummary

	Findings []Finding

	Recommendations []Recommendation

	Metrics map[string]interface{}

	RiskAssessment RiskAssessment

	Context  map[string]interface{}
	Metadata AnalysisMetadata
}
type AnalysisSummary struct {
	TotalFindings    int
	CriticalFindings int
	Strengths        []string
	Weaknesses       []string
	OverallScore     int
}
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
type FindingLocation struct {
	File      string
	Line      int
	Component string
	Context   string
}
type Recommendation struct {
	ID          string
	Priority    string
	Category    string
	Title       string
	Description string
	Benefits    []string
	Effort      string
	Impact      string
}
type RiskAssessment struct {
	OverallRisk string
	RiskFactors []RiskFactor
	Mitigations []Mitigation
}
type RiskFactor struct {
	ID          string
	Category    string
	Description string
	Likelihood  string
	Impact      string
	Score       int
}
type Mitigation struct {
	RiskID        string
	Description   string
	Effort        string
	Effectiveness string
}
type AnalysisMetadata struct {
	AnalyzerName    string
	AnalyzerVersion string
	Duration        time.Duration
	Timestamp       time.Time
	Parameters      map[string]interface{}
}
type AnalyzerCapabilities struct {
	SupportedTypes   []string
	SupportedAspects []string
	RequiresContext  bool
	SupportsDeepScan bool
}
type BaseAnalyzerImpl struct {
	Name         string
	Version      string
	Capabilities AnalyzerCapabilities
}

func NewBaseAnalyzer(name, version string, capabilities AnalyzerCapabilities) *BaseAnalyzerImpl {
	return &BaseAnalyzerImpl{
		Name:         name,
		Version:      version,
		Capabilities: capabilities,
	}
}
func (a *BaseAnalyzerImpl) GetName() string {
	return a.Name
}
func (a *BaseAnalyzerImpl) GetCapabilities() AnalyzerCapabilities {
	return a.Capabilities
}
func (a *BaseAnalyzerImpl) CreateResult() *AnalysisResult {
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
func (r *AnalysisResult) AddFinding(finding Finding) {
	r.Findings = append(r.Findings, finding)
	r.Summary.TotalFindings++

	if finding.Severity == "critical" || finding.Severity == "high" {
		r.Summary.CriticalFindings++
	}
}
func (r *AnalysisResult) AddRecommendation(rec Recommendation) {
	r.Recommendations = append(r.Recommendations, rec)
}
func (r *AnalysisResult) AddStrength(strength string) {
	r.Summary.Strengths = append(r.Summary.Strengths, strength)
}
func (r *AnalysisResult) AddWeakness(weakness string) {
	r.Summary.Weaknesses = append(r.Summary.Weaknesses, weakness)
}
func (r *AnalysisResult) CalculateScore() {
	score := 100

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

	score += len(r.Summary.Strengths) * 2

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	r.Summary.OverallScore = score
}
func (r *AnalysisResult) CalculateRisk() {
	if r.RiskAssessment.RiskFactors == nil {
		r.RiskAssessment.RiskFactors = make([]RiskFactor, 0)
	}

	totalScore := 0
	for _, factor := range r.RiskAssessment.RiskFactors {
		likelihood := scoreRiskLevel(factor.Likelihood)
		impact := scoreRiskLevel(factor.Impact)
		factor.Score = likelihood * impact
		totalScore += factor.Score
	}

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

type AnalysisContext struct {
	SessionID  string
	WorkingDir string
	Options    AnalysisOptions
	StartTime  time.Time
	Custom     map[string]interface{}
}

func NewAnalysisContext(sessionID, workingDir string, options AnalysisOptions) *AnalysisContext {
	return &AnalysisContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Options:    options,
		StartTime:  time.Now(),
		Custom:     make(map[string]interface{}),
	}
}
func (c *AnalysisContext) Duration() time.Duration {
	return time.Since(c.StartTime)
}

type AnalyzerChain struct {
	analyzers []BaseAnalyzer
}

func NewAnalyzerChain(analyzers ...BaseAnalyzer) *AnalyzerChain {
	return &AnalyzerChain{
		analyzers: analyzers,
	}
}
func (c *AnalyzerChain) Analyze(ctx context.Context, input interface{}, options AnalysisOptions) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Findings:        make([]Finding, 0),
		Recommendations: make([]Recommendation, 0),
		Metrics:         make(map[string]interface{}),
		Context:         make(map[string]interface{}),
	}

	for _, analyzer := range c.analyzers {
		aResult, err := analyzer.Analyze(ctx, input, options)
		if err != nil {
			return nil, err
		}

		result.Findings = append(result.Findings, aResult.Findings...)
		result.Recommendations = append(result.Recommendations, aResult.Recommendations...)
		result.Summary.Strengths = append(result.Summary.Strengths, aResult.Summary.Strengths...)
		result.Summary.Weaknesses = append(result.Summary.Weaknesses, aResult.Summary.Weaknesses...)

		for k, v := range aResult.Metrics {
			result.Metrics[k] = v
		}
		for k, v := range aResult.Context {
			result.Context[k] = v
		}
	}

	result.Summary.TotalFindings = len(result.Findings)
	for _, f := range result.Findings {
		if f.Severity == "critical" || f.Severity == "high" {
			result.Summary.CriticalFindings++
		}
	}

	result.CalculateScore()
	result.CalculateRisk()

	return result, nil
}
func (c *AnalyzerChain) GetName() string {
	return "AnalyzerChain"
}
func (c *AnalyzerChain) GetCapabilities() AnalyzerCapabilities {
	caps := AnalyzerCapabilities{
		SupportedTypes:   make([]string, 0),
		SupportedAspects: make([]string, 0),
	}

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

	for t := range typeMap {
		caps.SupportedTypes = append(caps.SupportedTypes, t)
	}
	for a := range aspectMap {
		caps.SupportedAspects = append(caps.SupportedAspects, a)
	}

	return caps
}
