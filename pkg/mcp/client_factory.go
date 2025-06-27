package mcp

import (
	"context"

	"github.com/Azure/container-kit/pkg/ai"
	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/Azure/container-kit/pkg/runner"
	"github.com/rs/zerolog"
)

// ClientFactory creates and manages external client instances with proper dependency injection
type ClientFactory interface {
	CreateDockerClient() docker.DockerClient
	CreateK8sClient() k8s.KubeRunner
	CreateKindClient() kind.KindRunner
	CreateAIClient() mcptypes.AIAnalyzer
	CreateMCPClients() *mcptypes.MCPClients
}

// ClientConfiguration holds configuration for all external clients
type ClientConfiguration struct {
	// AI Configuration
	AIEndpoint     string
	AIAPIKey       string
	AIDeploymentID string

	// Docker Configuration
	DockerHost      string
	DockerCertPath  string
	DockerTLSVerify bool

	// Kubernetes Configuration
	KubeconfigPath string
	KubeContext    string

	// Kind Configuration
	KindClusterName string

	// Logging
	Logger zerolog.Logger
}

// standardClientFactory implements ClientFactory with configurable external clients
type standardClientFactory struct {
	config        ClientConfiguration
	commandRunner runner.CommandRunner

	// Cached clients (singleton pattern for expensive-to-create clients)
	dockerClient docker.DockerClient
	k8sClient    k8s.KubeRunner
	kindClient   kind.KindRunner
	aiClient     mcptypes.AIAnalyzer
}

// NewClientFactory creates a new client factory with the given configuration
func NewClientFactory(config ClientConfiguration) ClientFactory {
	return &standardClientFactory{
		config:        config,
		commandRunner: &runner.DefaultCommandRunner{}, // Single instance
	}
}

// CreateDockerClient creates or returns a cached Docker client
func (f *standardClientFactory) CreateDockerClient() docker.DockerClient {
	if f.dockerClient == nil {
		f.dockerClient = docker.NewDockerCmdRunner(f.commandRunner)
	}
	return f.dockerClient
}

// CreateK8sClient creates or returns a cached Kubernetes client
func (f *standardClientFactory) CreateK8sClient() k8s.KubeRunner {
	if f.k8sClient == nil {
		f.k8sClient = k8s.NewKubeCmdRunner(f.commandRunner)
	}
	return f.k8sClient
}

// CreateKindClient creates or returns a cached Kind client
func (f *standardClientFactory) CreateKindClient() kind.KindRunner {
	if f.kindClient == nil {
		f.kindClient = kind.NewKindCmdRunner(f.commandRunner)
	}
	return f.kindClient
}

// CreateAIClient creates or returns a cached AI client
func (f *standardClientFactory) CreateAIClient() mcptypes.AIAnalyzer {
	if f.aiClient == nil {
		// Use configuration to create appropriate AI client
		if f.config.AIEndpoint != "" {
			// Create Azure OpenAI client with configuration
			azClient, err := ai.NewAzOpenAIClient(
				f.config.AIEndpoint,
				f.config.AIAPIKey,
				f.config.AIDeploymentID,
			)
			if err != nil {
				f.config.Logger.Error().Err(err).Msg("Failed to create Azure OpenAI client, falling back to no-op")
				f.aiClient = &noOpAIAnalyzer{}
			} else {
				f.aiClient = &aiAnalyzerAdapter{client: azClient}
			}
		} else {
			// No AI configuration provided, use no-op implementation
			f.aiClient = &noOpAIAnalyzer{}
		}
	}
	return f.aiClient
}

// CreateMCPClients creates a complete MCPClients instance with all clients
func (f *standardClientFactory) CreateMCPClients() *mcptypes.MCPClients {
	return mcptypes.NewMCPClientsWithAnalyzer(
		f.CreateDockerClient(),
		f.CreateKindClient(),
		f.CreateK8sClient(),
		f.CreateAIClient(),
	)
}

// =============================================================================
// AI Client Adapter (converts between interfaces)
// =============================================================================

// aiAnalyzerAdapter adapts the ai.LLMClient to mcptypes.AIAnalyzer interface
type aiAnalyzerAdapter struct {
	client ai.LLMClient
}

func (a *aiAnalyzerAdapter) Analyze(ctx context.Context, prompt string) (string, error) {
	response, _, err := a.client.GetChatCompletion(ctx, prompt)
	return response, err
}

func (a *aiAnalyzerAdapter) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	response, _, err := a.client.GetChatCompletionWithFileTools(ctx, prompt, baseDir)
	return response, err
}

func (a *aiAnalyzerAdapter) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	response, _, err := a.client.GetChatCompletionWithFormat(ctx, promptTemplate, args...)
	return response, err
}

func (a *aiAnalyzerAdapter) GetTokenUsage() mcptypes.TokenUsage {
	usage := a.client.GetTokenUsage()
	return mcptypes.TokenUsage{
		CompletionTokens: usage.CompletionTokens,
		PromptTokens:     usage.PromptTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func (a *aiAnalyzerAdapter) ResetTokenUsage() {
	// No-op for now - can be enhanced if the underlying client supports it
}

// =============================================================================
// No-Op AI Analyzer (for when AI is not configured)
// =============================================================================

// noOpAIAnalyzer provides a no-op implementation of AIAnalyzer
type noOpAIAnalyzer struct{}

func (n *noOpAIAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "AI analysis not available (no client configured)", nil
}

func (n *noOpAIAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "AI analysis not available (no client configured)", nil
}

func (n *noOpAIAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "AI analysis not available (no client configured)", nil
}

func (n *noOpAIAnalyzer) GetTokenUsage() mcptypes.TokenUsage {
	return mcptypes.TokenUsage{}
}

func (n *noOpAIAnalyzer) ResetTokenUsage() {
	// No-op
}

// =============================================================================
// Injectable Client Provider
// =============================================================================

// InjectableClientProvider allows tools to receive clients via dependency injection
type InjectableClientProvider interface {
	SetClientFactory(factory ClientFactory)
	GetClientFactory() ClientFactory
}

// BaseInjectableClients provides a base implementation for tools that need client injection
type BaseInjectableClients struct {
	clientFactory ClientFactory
}

// SetClientFactory implements InjectableClientProvider
func (b *BaseInjectableClients) SetClientFactory(factory ClientFactory) {
	b.clientFactory = factory
}

// GetClientFactory implements InjectableClientProvider
func (b *BaseInjectableClients) GetClientFactory() ClientFactory {
	return b.clientFactory
}

// GetDockerClient provides convenient access to Docker client
func (b *BaseInjectableClients) GetDockerClient() docker.DockerClient {
	if b.clientFactory == nil {
		panic("client factory not injected - call SetClientFactory first")
	}
	return b.clientFactory.CreateDockerClient()
}

// GetK8sClient provides convenient access to Kubernetes client
func (b *BaseInjectableClients) GetK8sClient() k8s.KubeRunner {
	if b.clientFactory == nil {
		panic("client factory not injected - call SetClientFactory first")
	}
	return b.clientFactory.CreateK8sClient()
}

// GetKindClient provides convenient access to Kind client
func (b *BaseInjectableClients) GetKindClient() kind.KindRunner {
	if b.clientFactory == nil {
		panic("client factory not injected - call SetClientFactory first")
	}
	return b.clientFactory.CreateKindClient()
}

// GetAIClient provides convenient access to AI client
func (b *BaseInjectableClients) GetAIClient() mcptypes.AIAnalyzer {
	if b.clientFactory == nil {
		panic("client factory not injected - call SetClientFactory first")
	}
	return b.clientFactory.CreateAIClient()
}
