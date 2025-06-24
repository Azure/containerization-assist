package ai_context

// AIContext provides essential AI context capabilities for tool responses
// This simplified interface consolidates the over-engineered hierarchy into essential features
type AIContext interface {
	// Assessment capabilities
	GetAssessment() *UnifiedAssessment

	// Recommendation capabilities
	GenerateRecommendations() []Recommendation

	// Context enrichment
	GetToolContext() *ToolContext

	// Essential metadata
	GetMetadata() map[string]interface{}
}

// ScoreCalculator provides unified scoring algorithms
type ScoreCalculator interface {
	CalculateScore(data interface{}) int
	DetermineRiskLevel(score int, factors map[string]interface{}) string
	CalculateConfidence(evidence []string) int
}

// TradeoffAnalyzer provides unified trade-off analysis
type TradeoffAnalyzer interface {
	AnalyzeTradeoffs(options []string, context map[string]interface{}) []TradeoffAnalysis
	CompareAlternatives(alternatives []AlternativeStrategy) *ComparisonMatrix
	RecommendBestOption(analysis []TradeoffAnalysis) *DecisionRecommendation
}

// Legacy type aliases removed as part of Workstream 2: Adapter Deprecation cleanup
// All code now uses direct interface types (AIContext, ContextualInsight, AssessmentArea)
