// Package ai_ml provides unified dependency injection for AI/ML services
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

// Providers provides all AI/ML domain dependencies
var Providers = wire.NewSet(
	// Sampling client
	ProvideSamplingClient,
	wire.Bind(new(domainsampling.UnifiedSampler), new(*sampling.DomainAdapter)),
	sampling.NewDomainAdapter,

	// Prompt management
	ProvidePromptManager,

	// Machine learning services
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

	// Interface bindings would go here if needed
)

// ProvideSamplingClient creates a sampling client for LLM integration
func ProvideSamplingClient(logger *slog.Logger) (*sampling.Client, error) {
	// Create sampling client with default configuration
	client, err := sampling.NewClientFromEnv(logger)
	if err != nil {
		// Fall back to default client
		return sampling.NewClient(logger), nil
	}

	return client, nil
}

// ProvidePromptManager creates a prompt manager
func ProvidePromptManager(logger *slog.Logger) (*prompts.Manager, error) {
	// Use default configuration for prompt manager
	config := prompts.ManagerConfig{
		TemplateDir:     "", // Use embedded templates
		EnableHotReload: false,
		AllowOverride:   false,
	}

	return prompts.NewManager(logger, config)
}

// ProvideResourcePredictor creates a resource predictor
func ProvideResourcePredictor(sampler domainsampling.UnifiedSampler, logger *slog.Logger) *ml.ResourcePredictor {
	return ml.NewResourcePredictor(sampler, logger)
}

// ProvideBuildOptimizer creates a build optimizer
func ProvideBuildOptimizer(predictor *ml.ResourcePredictor, logger *slog.Logger) *ml.BuildOptimizer {
	return ml.NewBuildOptimizer(predictor, logger)
}
