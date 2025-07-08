package core

// NOTE: The following interfaces have been consolidated into AnalysisService
// in unified_interfaces.go for better maintainability:
// - ProgressReporter
// - Analyzer
// - RepositoryAnalyzer
// - AIAnalyzer
//
// Use AnalysisService instead of these interfaces for new implementations.

// The supporting types and concrete implementations remain in this file.

// ConsolidatedSecretFinding represents a detected secret with basic fields
type ConsolidatedSecretFinding struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Confidence  string `json:"confidence"`
	RuleID      string `json:"rule_id"`
}
