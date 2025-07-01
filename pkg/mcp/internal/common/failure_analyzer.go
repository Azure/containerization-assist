package common

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
)

// FailureAnalyzer provides unified failure analysis across all tool domains
type FailureAnalyzer interface {
	AnalyzeFailure(ctx context.Context, operation, sessionID string, params map[string]interface{}) error
}

// DefaultFailureAnalyzer provides a default implementation for failure analysis
type DefaultFailureAnalyzer struct {
	toolName string
	domain   string
	logger   zerolog.Logger
}

// NewDefaultFailureAnalyzer creates a new default failure analyzer
func NewDefaultFailureAnalyzer(toolName, domain string, logger zerolog.Logger) *DefaultFailureAnalyzer {
	return &DefaultFailureAnalyzer{
		toolName: toolName,
		domain:   domain,
		logger:   logger.With().Str("component", "failure_analyzer").Str("domain", domain).Logger(),
	}
}

// AnalyzeFailure provides generic failure analysis
func (a *DefaultFailureAnalyzer) AnalyzeFailure(ctx context.Context, operation, sessionID string, params map[string]interface{}) error {
	a.logger.Info().
		Str("operation", operation).
		Str("session_id", sessionID).
		Interface("params", params).
		Msg("Analyzing failure")

	// Generic failure analysis logic - can be enhanced with domain-specific analysis
	switch strings.ToLower(operation) {
	case "build", "docker_build":
		return a.analyzeBuildFailure(sessionID, params)
	case "push", "docker_push":
		return a.analyzePushFailure(sessionID, params)
	case "pull", "docker_pull":
		return a.analyzePullFailure(sessionID, params)
	case "tag", "docker_tag":
		return a.analyzeTagFailure(sessionID, params)
	case "deployment", "deploy":
		return a.analyzeDeploymentFailure(sessionID, params)
	case "health_check", "health":
		return a.analyzeHealthCheckFailure(sessionID, params)
	case "validation", "validate":
		return a.analyzeValidationFailure(sessionID, params)
	case "scan", "security_scan":
		return a.analyzeScanFailure(sessionID, params)
	case "secrets", "secret_scan":
		return a.analyzeSecretsFailure(sessionID, params)
	default:
		a.logger.Warn().Str("operation", operation).Msg("Unknown operation type for failure analysis")
		return nil
	}
}

// Domain-specific failure analysis methods

func (a *DefaultFailureAnalyzer) analyzeBuildFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing build failure")
	// Default implementation - could be enhanced with actual analysis
	return nil
}

func (a *DefaultFailureAnalyzer) analyzePushFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing push failure")
	return nil
}

func (a *DefaultFailureAnalyzer) analyzePullFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing pull failure")
	return nil
}

func (a *DefaultFailureAnalyzer) analyzeTagFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing tag failure")
	return nil
}

func (a *DefaultFailureAnalyzer) analyzeDeploymentFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing deployment failure")
	return nil
}

func (a *DefaultFailureAnalyzer) analyzeHealthCheckFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing health check failure")
	return nil
}

func (a *DefaultFailureAnalyzer) analyzeValidationFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing validation failure")
	return nil
}

func (a *DefaultFailureAnalyzer) analyzeScanFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing scan failure")
	return nil
}

func (a *DefaultFailureAnalyzer) analyzeSecretsFailure(sessionID string, params map[string]interface{}) error {
	a.logger.Debug().Str("session_id", sessionID).Msg("Analyzing secrets failure")
	return nil
}

// GetSupportedOperations returns the operations supported by this analyzer
func (a *DefaultFailureAnalyzer) GetSupportedOperations() []string {
	return []string{
		"build", "push", "pull", "tag",
		"deployment", "health_check", "validation",
		"scan", "secrets",
	}
}

// GetDomain returns the domain this analyzer serves
func (a *DefaultFailureAnalyzer) GetDomain() string {
	return a.domain
}

// GetToolName returns the tool name this analyzer serves
func (a *DefaultFailureAnalyzer) GetToolName() string {
	return a.toolName
}

// GetToolNamePublic provides public access to tool name (for testing)
func (a *DefaultFailureAnalyzer) GetToolNamePublic() string {
	return a.toolName
}