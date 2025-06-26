package types

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
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
		Analyzer: &stubAnalyzer{}, // Default to stub - no external AI
	}
}

// NewMCPClientsWithAnalyzer creates MCP-specific clients with a specific analyzer
func NewMCPClientsWithAnalyzer(docker docker.DockerClient, kind kind.KindRunner, kube k8s.KubeRunner, analyzer AIAnalyzer) *MCPClients {
	return &MCPClients{
		Docker:   docker,
		Kind:     kind,
		Kube:     kube,
		Analyzer: analyzer,
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
	case "*types.stubAnalyzer", "*analyze.StubAnalyzer", "*analyze.CallerAnalyzer":
		logger.Info().Str("analyzer_type", analyzerType).Msg("Using safe analyzer for production")
		return nil
	default:
		logger.Warn().Str("analyzer_type", analyzerType).Msg("Unknown analyzer type - may not be safe for production")
		return fmt.Errorf("analyzer type %s may not be safe for production", analyzerType)
	}
}

// stubAnalyzer is a local stub implementation to avoid import cycles
type stubAnalyzer struct{}

// Analyze returns a basic stub response
func (s *stubAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "stub analysis result", nil
}

// AnalyzeWithFileTools returns a basic stub response
func (s *stubAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "stub analysis result", nil
}

// AnalyzeWithFormat returns a basic stub response
func (s *stubAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "stub analysis result", nil
}

// GetTokenUsage returns empty usage
func (s *stubAnalyzer) GetTokenUsage() TokenUsage {
	return TokenUsage{}
}

// ResetTokenUsage does nothing for stub
func (s *stubAnalyzer) ResetTokenUsage() {
}
