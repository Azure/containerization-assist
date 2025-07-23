// Package orchestration provides infrastructure implementation for adaptive workflow orchestration
package orchestration

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/ml"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
)

// AdaptiveOrchestratorAdapter adapts the domain adaptive orchestrator to the infrastructure layer
type AdaptiveOrchestratorAdapter struct {
	adaptiveOrchestrator *workflow.AdaptiveWorkflowOrchestrator
	logger               *slog.Logger
}

// NewAdaptiveOrchestratorAdapter creates a new adaptive orchestrator adapter
func NewAdaptiveOrchestratorAdapter(
	baseOrchestrator workflow.WorkflowOrchestrator,
	patternRecognizer ml.ErrorPatternRecognizer,
	stepEnhancer ml.StepEnhancer,
	logger *slog.Logger,
) *AdaptiveOrchestratorAdapter {
	// Create adapter to bridge domain interfaces
	patternAdapter := &PatternRecognizerAdapter{
		patternRecognizer: patternRecognizer,
		logger:            logger,
	}

	stepAdapter := NewStepEnhancerAdapter(stepEnhancer, logger)

	adaptiveOrchestrator := workflow.NewAdaptiveWorkflowOrchestrator(
		baseOrchestrator,
		patternAdapter,
		stepAdapter,
		logger,
	)

	return &AdaptiveOrchestratorAdapter{
		adaptiveOrchestrator: adaptiveOrchestrator,
		logger:               logger.With("component", "adaptive_orchestrator_adapter"),
	}
}

// Execute runs the workflow with adaptive capabilities
func (a *AdaptiveOrchestratorAdapter) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	if args.RepoURL != "" {
		a.logger.Info("Executing adaptive workflow",
			"repo_url", args.RepoURL,
			"branch", args.Branch)
	} else {
		a.logger.Info("Executing adaptive workflow",
			"repo_path", args.RepoPath)
	}

	return a.adaptiveOrchestrator.Execute(ctx, req, args)
}

// GetAdaptationStatistics returns statistics about workflow adaptations
func (a *AdaptiveOrchestratorAdapter) GetAdaptationStatistics() *workflow.AdaptationStatistics {
	return a.adaptiveOrchestrator.GetAdaptationStatistics()
}

// UpdateAdaptationStrategy allows manual updates to adaptation strategies
func (a *AdaptiveOrchestratorAdapter) UpdateAdaptationStrategy(patternID string, strategy *workflow.AdaptationStrategy) error {
	return a.adaptiveOrchestrator.UpdateAdaptationStrategy(patternID, strategy)
}

// ClearAdaptationHistory clears the adaptation history
func (a *AdaptiveOrchestratorAdapter) ClearAdaptationHistory() error {
	return a.adaptiveOrchestrator.ClearAdaptationHistory()
}

// PatternRecognizerAdapter adapts ml.ErrorPatternRecognizer to workflow.ErrorPatternRecognizer
type PatternRecognizerAdapter struct {
	patternRecognizer ml.ErrorPatternRecognizer
	logger            *slog.Logger
}

// RecognizePattern adapts the interface between domain layers
func (p *PatternRecognizerAdapter) RecognizePattern(ctx context.Context, err error, stepContext *workflow.WorkflowState) (*workflow.ErrorClassification, error) {
	// Call the ml domain recognizer
	classification, recognizeErr := p.patternRecognizer.RecognizePattern(ctx, err, stepContext)
	if recognizeErr != nil {
		return nil, recognizeErr
	}

	// Convert ml.ErrorClassification to workflow.ErrorClassification
	return &workflow.ErrorClassification{
		Category:    classification.Category,
		Confidence:  classification.Confidence,
		Patterns:    classification.Patterns,
		Suggestions: classification.Suggestions,
	}, nil
}
