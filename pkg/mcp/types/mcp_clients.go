package types

import (
	"fmt"

	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
	analyzer "github.com/Azure/container-copilot/pkg/mcp/internal/analyze"
	"github.com/rs/zerolog"
)

// MCPClients provides MCP-specific clients without external AI dependencies
// This replaces pkg/clients.Clients for MCP usage to ensure no AI dependencies
type MCPClients struct {
	Docker   docker.DockerClient
	Kind     kind.KindRunner
	Kube     k8s.KubeRunner
	Analyzer AIAnalyzer // Always use stub or caller analyzer - never external AI
}

// NewMCPClients creates MCP-specific clients with stub analyzer
func NewMCPClients(docker docker.DockerClient, kind kind.KindRunner, kube k8s.KubeRunner) *MCPClients {
	return &MCPClients{
		Docker:   docker,
		Kind:     kind,
		Kube:     kube,
		Analyzer: analyzer.NewStubAnalyzer(), // Default to stub - no external AI
	}
}

// SetAnalyzer allows dependency injection of the analyzer implementation
func (mc *MCPClients) SetAnalyzer(analyzer AIAnalyzer) {
	mc.Analyzer = analyzer
}

// ValidateAnalyzerForProduction ensures the analyzer is appropriate for production
func (mc *MCPClients) ValidateAnalyzerForProduction(logger zerolog.Logger) error {
	if mc.Analyzer == nil {
		return fmt.Errorf("analyzer cannot be nil")
	}

	// In production, we should never use external AI analyzers
	// Only stub or caller analyzers are allowed
	analyzerType := fmt.Sprintf("%T", mc.Analyzer)
	logger.Debug().Str("analyzer_type", analyzerType).Msg("Validating analyzer for production")

	// Check for known safe analyzer types
	switch analyzerType {
	case "*analyze.StubAnalyzer", "*analyze.CallerAnalyzer":
		logger.Info().Str("analyzer_type", analyzerType).Msg("Using safe analyzer for production")
		return nil
	default:
		logger.Warn().Str("analyzer_type", analyzerType).Msg("Unknown analyzer type - may not be safe for production")
		return fmt.Errorf("analyzer type %s may not be safe for production", analyzerType)
	}
}