package deploy

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/rs/zerolog"
)

// ToolAnalyzer provides analysis capabilities for atomic deployment tools
// Deprecated: Use common.FailureAnalyzer for unified failure analysis
type ToolAnalyzer interface {
	AnalyzeDeploymentFailure(namespace, sessionID string) error
	AnalyzeHealthCheckFailure(namespace, appName, sessionID string) error
}

// DefaultToolAnalyzer provides a default implementation using the unified analyzer
type DefaultToolAnalyzer struct {
	*common.DefaultFailureAnalyzer
}

// NewDefaultToolAnalyzer creates a new default tool analyzer
func NewDefaultToolAnalyzer(toolName string) *DefaultToolAnalyzer {
	return &DefaultToolAnalyzer{
		DefaultFailureAnalyzer: common.NewDefaultFailureAnalyzer(toolName, "deploy", zerolog.Nop()),
	}
}

// AnalyzeDeploymentFailure analyzes deployment failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzeDeploymentFailure(namespace, sessionID string) error {
	params := map[string]interface{}{
		"namespace": namespace,
	}
	return a.AnalyzeFailure(context.Background(), "deployment", sessionID, params)
}

// AnalyzeHealthCheckFailure analyzes health check failures (backward compatibility)
func (a *DefaultToolAnalyzer) AnalyzeHealthCheckFailure(namespace, appName, sessionID string) error {
	params := map[string]interface{}{
		"namespace": namespace,
		"app_name":  appName,
	}
	return a.AnalyzeFailure(context.Background(), "health_check", sessionID, params)
}
