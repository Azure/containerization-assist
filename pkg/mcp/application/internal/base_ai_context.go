package internal

import (
	"time"
)

// BaseAIContextResult provides common AI context implementations for all atomic tool results
// This eliminates code duplication across 10+ tool result types that implement identical methods
type BaseAIContextResult struct {
	IsSuccessful  bool
	Duration      time.Duration
	OperationType string
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

// CalculateScore implements unified scoring logic
func (b BaseAIContextResult) CalculateScore() int {
	if !b.IsSuccessful {
		return 20 // Poor score for failed operations
	}

	var baseScore int
	switch b.OperationType {
	case "build":
		baseScore = 70
	case "deploy":
		baseScore = 75
	case "scan":
		baseScore = 60
	case "analysis":
		baseScore = 40
	case "pull", "push", "tag":
		baseScore = 80
	case "health":
		baseScore = 85
	case "validate":
		baseScore = 50
	default:
		baseScore = 60
	}

	if b.Duration > 0 {
		switch {
		case b.Duration < 30*time.Second:
			baseScore += 15
		case b.Duration > 5*time.Minute:
			baseScore -= 10
		}
	}

	baseScore -= (b.ErrorCount * 15)
	baseScore -= (b.WarningCount * 5)

	if baseScore < 0 {
		baseScore = 0
	}
	if baseScore > 100 {
		baseScore = 100
	}

	return baseScore
}

// DetermineRiskLevel implements unified risk assessment
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

// GetStrengths implements operation-specific strengths
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

// GetChallenges implements operation-specific challenges
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

// GetMetadataForAI provides basic metadata for AI context
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
