package types

import (
	"time"
)

// BaseAIContextResult provides common AI context implementations for all atomic tool results
// This is the mcptypes equivalent of internal.BaseAIContextResult to break import cycles
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

// CalculateScore implements scoring logic
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

// DetermineRiskLevel determines risk level based on score
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

// GetStrengths returns operation-specific strengths
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

// GetChallenges returns operation-specific challenges
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

// GetMetadataForAI returns basic metadata for AI context
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