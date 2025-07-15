// Package ai_ml provides configurable AI/ML service providers
package ai_ml

import (
	"log/slog"

	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/ml"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
	"github.com/google/wire"
)

// ConfigurableProviders provides AI/ML services with optional configuration
var ConfigurableProviders = wire.NewSet(
	// Sampling client with optional configuration
	ProvideConfigurableSamplingClient,
	wire.Bind(new(domainsampling.UnifiedSampler), new(*sampling.DomainAdapter)),
	sampling.NewDomainAdapter,

	// Prompt management with optional configuration
	ProvideConfigurablePromptManager,

	// Machine learning services (these don't need configuration changes)
	ml.NewErrorPatternRecognizer,
	wire.Bind(new(domainml.ErrorPatternRecognizer), new(*ml.ErrorPatternRecognizer)),
	ml.NewEnhancedErrorHandler,
	wire.Bind(new(domainml.EnhancedErrorHandler), new(*ml.EnhancedErrorHandler)),
	ml.NewStepEnhancer,
	wire.Bind(new(domainml.StepEnhancer), new(*ml.StepEnhancer)),
	ProvideResourcePredictor,
	ProvideBuildOptimizer,
	wire.Bind(new(workflow.BuildOptimizer), new(*ml.BuildOptimizer)),

	// Prompt manager binding
	wire.Bind(new(domainprompts.Manager), new(*prompts.Manager)),
)

// ConfigurableServiceOptions holds configuration for AI/ML services
type ConfigurableServiceOptions struct {
	SamplingConfig *sampling.Config
	PromptConfig   *prompts.ManagerConfig
}

// ProvideConfigurableSamplingClient creates a sampling client with optional configuration
func ProvideConfigurableSamplingClient(logger *slog.Logger, opts *ConfigurableServiceOptions) (*sampling.Client, error) {
	if opts == nil || opts.SamplingConfig == nil {
		// Fall back to default behavior
		return ProvideSamplingClient(logger)
	}

	// Create client with provided configuration
	return sampling.NewClientWithConfig(logger, *opts.SamplingConfig)
}

// ProvideConfigurablePromptManager creates a prompt manager with optional configuration
func ProvideConfigurablePromptManager(logger *slog.Logger, opts *ConfigurableServiceOptions) (*prompts.Manager, error) {
	config := prompts.ManagerConfig{
		TemplateDir:     "", // Use embedded templates
		EnableHotReload: false,
		AllowOverride:   false,
	}

	// Apply configuration overrides if provided
	if opts != nil && opts.PromptConfig != nil {
		config = *opts.PromptConfig
	}

	return prompts.NewManager(logger, config)
}

// ProvideConfigurableServiceOptions creates service options from LLM configuration
func ProvideConfigurableServiceOptions(samplingConfig *sampling.Config, promptConfig *prompts.ManagerConfig) *ConfigurableServiceOptions {
	return &ConfigurableServiceOptions{
		SamplingConfig: samplingConfig,
		PromptConfig:   promptConfig,
	}
}

// Wire provider set for configurable services
var ConfigurableProvidersWithOptions = wire.NewSet(
	ConfigurableProviders,
	ProvideConfigurableServiceOptions,
)
