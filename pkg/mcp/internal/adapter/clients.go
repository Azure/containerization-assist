package adapter

import (
	"fmt"

	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
	"github.com/Azure/container-copilot/pkg/mcp/internal/analyzer"
	"github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// MCPClients provides MCP-specific clients without external AI dependencies
// This replaces pkg/clients.Clients for MCP usage to ensure no AI dependencies
type MCPClients struct {
	Docker   docker.DockerClient
	Kind     kind.KindRunner
	Kube     k8s.KubeRunner
	Analyzer types.AIAnalyzer // Always use stub or caller analyzer - never external AI
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

// SetAnalyzer allows injection of different analyzer implementations
// For tests: MockAnalyzer
// For MCP with caller: CallerAnalyzer
// For MCP without caller: StubAnalyzer (default)
func (c *MCPClients) SetAnalyzer(a types.AIAnalyzer) {
	c.Analyzer = a
}

// ValidateAnalyzerForProduction validates that the analyzer is appropriate for production use
// Returns an error if StubAnalyzer is used in production environments
func (c *MCPClients) ValidateAnalyzerForProduction(logger zerolog.Logger, transportEnabled bool) error {
	if c.Analyzer == nil {
		return fmt.Errorf("analyzer is nil - this should never happen")
	}

	// Check if using StubAnalyzer
	if _, isStub := c.Analyzer.(*analyzer.StubAnalyzer); isStub {
		if transportEnabled {
			// Fatal: StubAnalyzer in production with enabled transport will cause silent failures
			return fmt.Errorf("CRITICAL: StubAnalyzer detected with enabled transport - this will cause silent AI failures in production. Either disable transport or configure a proper analyzer (CallerAnalyzer)")
		} else {
			// Warning: StubAnalyzer without transport is acceptable but should be logged
			logger.Warn().Msg("Using StubAnalyzer - AI features disabled (this is normal for MCP without conversation mode)")
		}
	} else {
		// Log what analyzer is being used
		logger.Info().
			Str("analyzer_type", fmt.Sprintf("%T", c.Analyzer)).
			Msg("AI analyzer configured successfully")
	}

	return nil
}
