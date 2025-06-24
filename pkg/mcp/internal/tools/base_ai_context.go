package tools

import (
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/ai_context"
)

// BaseAIContextResult provides common AI context implementations for all atomic tool results
// This eliminates code duplication across 10+ tool result types that implement identical methods
type BaseAIContextResult struct {
	// Embed the success field that all tools have
	IsSuccessful bool

	// Common timing info for performance assessment
	Duration time.Duration

	// Common context for AI reasoning
	OperationType string // "build", "deploy", "scan", etc.
	ErrorCount    int
	WarningCount  int
}

// NewBaseAIContextResult creates a new base AI context result
func NewBaseAIContextResult(operationType string, isSuccessful bool, duration time.Duration) BaseAIContextResult {
	return BaseAIContextResult{
		IsSuccessful:  isSuccessful,
		Duration:      duration,
		OperationType: operationType,
	}
}

// CalculateScore implements ai_context.Assessable with unified scoring logic
func (b BaseAIContextResult) CalculateScore() int {
	if !b.IsSuccessful {
		return 20 // Poor score for failed operations
	}

	// Base score for successful operations varies by operation type
	var baseScore int
	switch b.OperationType {
	case "build":
		baseScore = 70 // Builds are complex, higher base score
	case "deploy":
		baseScore = 75 // Deployments are critical
	case "scan":
		baseScore = 60 // Scans are informational
	case "analysis":
		baseScore = 40 // Analysis is preparatory
	case "pull", "push", "tag":
		baseScore = 80 // Registry operations are simpler
	case "health":
		baseScore = 85 // Health checks are straightforward
	case "validate":
		baseScore = 50 // Validation is verification
	default:
		baseScore = 60 // Default for unknown operations
	}

	// Adjust for performance
	if b.Duration > 0 {
		switch {
		case b.Duration < 30*time.Second:
			baseScore += 15 // Fast operations
		case b.Duration > 5*time.Minute:
			baseScore -= 10 // Slow operations
		}
	}

	// Adjust for error/warning counts
	baseScore -= (b.ErrorCount * 15)  // Significant penalty for errors
	baseScore -= (b.WarningCount * 5) // Minor penalty for warnings

	// Ensure score is within valid range
	if baseScore < 0 {
		baseScore = 0
	}
	if baseScore > 100 {
		baseScore = 100
	}

	return baseScore
}

// DetermineRiskLevel implements ai_context.Assessable with unified risk assessment
func (b BaseAIContextResult) DetermineRiskLevel() string {
	score := b.CalculateScore()

	switch {
	case score >= 80:
		return "low"
	case score >= 60:
		return "medium"
	case score >= 40:
		return "high"
	default:
		return "critical"
	}
}

// GetStrengths implements ai_context.Assessable with operation-specific strengths
func (b BaseAIContextResult) GetStrengths() []string {
	var strengths []string

	if b.IsSuccessful {
		strengths = append(strengths, "Operation completed successfully")
	}

	if b.Duration > 0 && b.Duration < 1*time.Minute {
		strengths = append(strengths, "Fast execution time")
	}

	if b.ErrorCount == 0 {
		strengths = append(strengths, "No errors encountered")
	}

	if b.WarningCount == 0 {
		strengths = append(strengths, "No warnings generated")
	}

	// Operation-specific strengths
	switch b.OperationType {
	case "build":
		strengths = append(strengths, "Image built with container best practices")
	case "deploy":
		strengths = append(strengths, "Deployment follows Kubernetes standards")
	case "scan":
		strengths = append(strengths, "Comprehensive security analysis performed")
	case "analysis":
		strengths = append(strengths, "Thorough repository analysis completed")
	case "pull", "push":
		strengths = append(strengths, "Registry operations handled efficiently")
	case "health":
		strengths = append(strengths, "Application health verified")
	case "validate":
		strengths = append(strengths, "Validation checks passed")
	}

	if len(strengths) == 0 {
		strengths = append(strengths, "Operation executed as requested")
	}

	return strengths
}

// GetChallenges implements ai_context.Assessable with operation-specific challenges
func (b BaseAIContextResult) GetChallenges() []string {
	var challenges []string

	if !b.IsSuccessful {
		challenges = append(challenges, "Operation failed to complete successfully")
	}

	if b.Duration > 5*time.Minute {
		challenges = append(challenges, "Operation took longer than expected")
	}

	if b.ErrorCount > 0 {
		challenges = append(challenges, "Errors were encountered during execution")
	}

	if b.WarningCount > 3 {
		challenges = append(challenges, "Multiple warnings indicate potential issues")
	}

	// Operation-specific challenges
	switch b.OperationType {
	case "build":
		if !b.IsSuccessful {
			challenges = append(challenges, "Build failures may indicate dependency or configuration issues")
		}
	case "deploy":
		if !b.IsSuccessful {
			challenges = append(challenges, "Deployment failures may require cluster or manifest fixes")
		}
	case "scan":
		challenges = append(challenges, "Security scan results require review and potential remediation")
	case "analysis":
		if !b.IsSuccessful {
			challenges = append(challenges, "Analysis failures may prevent proper containerization")
		}
	case "pull", "push":
		if !b.IsSuccessful {
			challenges = append(challenges, "Registry connectivity or authentication issues")
		}
	case "health":
		if !b.IsSuccessful {
			challenges = append(challenges, "Application health issues require investigation")
		}
	case "validate":
		if !b.IsSuccessful {
			challenges = append(challenges, "Validation failures indicate configuration problems")
		}
	}

	if len(challenges) == 0 {
		challenges = append(challenges, "Consider monitoring for potential improvements")
	}

	return challenges
}

// GetAssessment implements ai_context.Assessable with unified assessment
func (b BaseAIContextResult) GetAssessment() *ai_context.UnifiedAssessment {
	// Determine overall health based on score
	var overallHealth string
	score := b.CalculateScore()
	switch {
	case score >= 80:
		overallHealth = "excellent"
	case score >= 60:
		overallHealth = "good"
	case score >= 40:
		overallHealth = "fair"
	default:
		overallHealth = "poor"
	}

	strengths := b.GetStrengths()
	challenges := b.GetChallenges()

	// Create strength assessment areas
	var strengthAreas []ai_context.AssessmentArea
	for i, strength := range strengths {
		if i < 3 { // Limit to top 3 strengths
			strengthAreas = append(strengthAreas, ai_context.AssessmentArea{
				Area:        "strength_" + b.OperationType,
				Category:    "operational",
				Description: strength,
				Impact:      "high",
				Evidence:    []string{strength},
				Score:       score,
			})
		}
	}

	// Create challenge assessment areas
	var challengeAreas []ai_context.AssessmentArea
	for i, challenge := range challenges {
		if i < 3 { // Limit to top 3 challenges
			challengeAreas = append(challengeAreas, ai_context.AssessmentArea{
				Area:        "challenge_" + b.OperationType,
				Category:    "operational",
				Description: challenge,
				Impact:      "medium",
				Evidence:    []string{challenge},
				Score:       score,
			})
		}
	}

	return &ai_context.UnifiedAssessment{
		ReadinessScore:    score,
		RiskLevel:         b.DetermineRiskLevel(),
		ConfidenceLevel:   90, // High confidence for atomic operations
		OverallHealth:     overallHealth,
		StrengthAreas:     strengthAreas,
		ChallengeAreas:    challengeAreas,
		RiskFactors:       []ai_context.RiskFactor{},     // Default empty
		DecisionFactors:   []ai_context.DecisionFactor{}, // Default empty
		AssessmentBasis:   []ai_context.EvidenceItem{},   // Default empty
		QualityIndicators: b.GetMetadataForAI(),
	}
}

// GetAIContext implements ai_context.ContextEnriched with operation context
func (b BaseAIContextResult) GetAIContext() *ai_context.ToolContext {
	return &ai_context.ToolContext{
		ToolName:        b.OperationType + "_atomic",
		OperationID:     "", // Can be set by individual tools
		Timestamp:       time.Now(),
		Assessment:      b.GetAssessment(),
		Recommendations: []ai_context.Recommendation{},    // Default empty, tools can override
		DecisionPoints:  []ai_context.DecisionPoint{},     // Default empty, tools can override
		TradeOffs:       []ai_context.TradeoffAnalysis{},  // Default empty, tools can override
		Insights:        []ai_context.ContextualInsight{}, // Default empty, tools can override
	}
}

// EnrichWithInsights implements ai_context.ContextEnriched (no-op by default)
func (b BaseAIContextResult) EnrichWithInsights(insights []*ai_context.ContextualInsight) {
	// Default implementation does nothing
	// Individual tools can override if they need insight processing
}

// GetMetadataForAI implements ai_context.ContextEnriched with basic metadata
func (b BaseAIContextResult) GetMetadataForAI() map[string]interface{} {
	return map[string]interface{}{
		"operation_type": b.OperationType,
		"success":        b.IsSuccessful,
		"duration_ms":    b.Duration.Milliseconds(),
		"error_count":    b.ErrorCount,
		"warning_count":  b.WarningCount,
		"score":          b.CalculateScore(),
		"risk_level":     b.DetermineRiskLevel(),
	}
}

// ToolAIContextProvider is a helper interface for tools to embed BaseAIContextResult
// This interface captures the methods that BaseAIContextResult actually implements
type ToolAIContextProvider interface {
	// Assessment capabilities (from legacy Assessable interface)
	CalculateScore() int
	DetermineRiskLevel() string
	GetStrengths() []string
	GetChallenges() []string
	GetAssessment() *ai_context.UnifiedAssessment

	// Context enrichment capabilities (from legacy ContextEnriched interface)
	GetAIContext() *ai_context.ToolContext
	EnrichWithInsights(insights []*ai_context.ContextualInsight)
	GetMetadataForAI() map[string]interface{}
}
