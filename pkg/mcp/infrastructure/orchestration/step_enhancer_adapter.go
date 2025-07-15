// Package orchestration provides adapters for AI-powered step enhancement
package orchestration

import (
	"context"
	"log/slog"

	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// StepEnhancerAdapter adapts domainml.StepEnhancer to workflow.StepEnhancer
type StepEnhancerAdapter struct {
	domainEnhancer domainml.StepEnhancer
	logger         *slog.Logger
}

// NewStepEnhancerAdapter creates a new adapter
func NewStepEnhancerAdapter(domainEnhancer domainml.StepEnhancer, logger *slog.Logger) *StepEnhancerAdapter {
	return &StepEnhancerAdapter{
		domainEnhancer: domainEnhancer,
		logger:         logger,
	}
}

// EnhanceStep adapts the domain interface to workflow interface
func (a *StepEnhancerAdapter) EnhanceStep(ctx context.Context, step workflow.Step, state *workflow.WorkflowState) (workflow.Step, error) {
	if a.domainEnhancer == nil {
		return step, nil
	}
	return a.domainEnhancer.EnhanceStep(ctx, step, state)
}

// OptimizeWorkflow adapts the domain interface to workflow interface
func (a *StepEnhancerAdapter) OptimizeWorkflow(ctx context.Context, steps []workflow.Step) (*workflow.WorkflowOptimization, error) {
	if a.domainEnhancer == nil {
		return &workflow.WorkflowOptimization{
			Suggestions:          []workflow.OptimizationSuggestion{},
			EstimatedImprovement: 0.0,
			Metadata:             map[string]interface{}{},
		}, nil
	}

	// Call domain enhancer
	domainOptimization, err := a.domainEnhancer.OptimizeWorkflow(ctx, steps)
	if err != nil {
		return nil, err
	}

	// Convert domain optimization to workflow optimization
	workflowOptimization := &workflow.WorkflowOptimization{
		EstimatedImprovement: domainOptimization.EstimatedImprovement,
		Metadata:             domainOptimization.Metadata,
	}

	// Convert suggestions
	workflowSuggestions := make([]workflow.OptimizationSuggestion, len(domainOptimization.Suggestions))
	for i, suggestion := range domainOptimization.Suggestions {
		workflowSuggestions[i] = workflow.OptimizationSuggestion{
			StepName:    suggestion.StepName,
			Type:        suggestion.Type,
			Description: suggestion.Description,
			Impact:      suggestion.Impact,
		}
	}
	workflowOptimization.Suggestions = workflowSuggestions

	return workflowOptimization, nil
}

// ProvideStepEnhancerAdapter creates a step enhancer adapter
func ProvideStepEnhancerAdapter(domainEnhancer domainml.StepEnhancer, logger *slog.Logger) workflow.StepEnhancer {
	return NewStepEnhancerAdapter(domainEnhancer, logger)
}
