// Package workflow provides middleware for AI-powered step enhancement
package workflow

import (
	"context"
	"log/slog"
	"time"
)

// StepEnhancer defines the interface for ML-based workflow step enhancement
// This is a local interface to avoid import cycles
type StepEnhancer interface {
	// EnhanceStep applies ML optimizations to a workflow step
	EnhanceStep(ctx context.Context, step Step, state *WorkflowState) (Step, error)

	// OptimizeWorkflow suggests workflow optimizations
	OptimizeWorkflow(ctx context.Context, steps []Step) (*WorkflowOptimization, error)
}

// WorkflowOptimization represents ML-suggested workflow optimizations
type WorkflowOptimization struct {
	Suggestions          []OptimizationSuggestion `json:"suggestions"`
	EstimatedImprovement float64                  `json:"estimated_improvement"`
	Metadata             map[string]interface{}   `json:"metadata,omitempty"`
}

// OptimizationSuggestion represents a single optimization suggestion
type OptimizationSuggestion struct {
	StepName    string  `json:"step_name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"`
}

// StepEnhancementMiddleware applies AI-powered optimizations to workflow steps
// This middleware wraps steps with ML-based error handling, retry logic, and optimization
func StepEnhancementMiddleware(enhancer StepEnhancer, logger *slog.Logger) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if enhancer == nil {
				// No enhancer available, execute normally
				return next(ctx, step, state)
			}

			startTime := time.Now()
			stepName := step.Name()

			logger.Info("Enhancing step with AI optimizations",
				"step", stepName,
				"workflow_id", state.WorkflowID,
				"step_number", state.CurrentStep)

			// Apply AI-powered step enhancement
			enhancedStep, err := enhancer.EnhanceStep(ctx, step, state)
			if err != nil {
				logger.Error("Failed to enhance step, falling back to original",
					"step", stepName,
					"error", err)
				// Fall back to original step if enhancement fails
				enhancedStep = step
			} else {
				logger.Info("Step enhanced successfully",
					"step", stepName,
					"enhancement_duration", time.Since(startTime))
			}

			// Execute the enhanced step
			return next(ctx, enhancedStep, state)
		}
	}
}

// WorkflowOptimizationMiddleware provides workflow-level optimization suggestions
// This middleware analyzes the entire workflow and suggests optimizations
func WorkflowOptimizationMiddleware(enhancer StepEnhancer, logger *slog.Logger) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if enhancer == nil {
				return next(ctx, step, state)
			}

			// Only run optimization analysis on the first step
			if state.CurrentStep == 1 {
				logger.Info("Analyzing workflow for optimization opportunities",
					"workflow_id", state.WorkflowID,
					"total_steps", state.TotalSteps)

				// Get all steps from the workflow state (we'll need to collect them)
				steps := state.GetAllSteps()
				if len(steps) > 0 {
					optimization, err := enhancer.OptimizeWorkflow(ctx, steps)
					if err != nil {
						logger.Error("Failed to optimize workflow", "error", err)
					} else {
						logger.Info("Workflow optimization analysis completed",
							"suggestions", len(optimization.Suggestions),
							"estimated_improvement", optimization.EstimatedImprovement)

						// Log optimization suggestions
						for _, suggestion := range optimization.Suggestions {
							logger.Info("Optimization suggestion",
								"step", suggestion.StepName,
								"type", suggestion.Type,
								"description", suggestion.Description,
								"impact", suggestion.Impact)
						}

						// Store optimization results in workflow state for later use
						state.SetOptimization(optimization)
					}
				}
			}

			return next(ctx, step, state)
		}
	}
}

// AdaptiveStepMiddleware applies adaptive behavior based on workflow history
// This middleware learns from previous executions and adapts step behavior
func AdaptiveStepMiddleware(enhancer StepEnhancer, logger *slog.Logger) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if enhancer == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			logger.Debug("Applying adaptive step behavior",
				"step", stepName,
				"workflow_id", state.WorkflowID)

			// Apply adaptive enhancements based on historical data
			// This could include:
			// - Adjusting retry strategies based on past failures
			// - Optimizing resource allocation based on historical usage
			// - Predicting likely failure points and pre-emptively addressing them

			enhancedStep, err := enhancer.EnhanceStep(ctx, step, state)
			if err != nil {
				logger.Warn("Adaptive enhancement failed, using original step",
					"step", stepName,
					"error", err)
				enhancedStep = step
			}

			return next(ctx, enhancedStep, state)
		}
	}
}

// CombinedEnhancementMiddleware combines multiple AI enhancement strategies
// This is a convenience function that applies all enhancement middlewares in optimal order
func CombinedEnhancementMiddleware(enhancer StepEnhancer, logger *slog.Logger) StepMiddleware {
	return Chain(
		WorkflowOptimizationMiddleware(enhancer, logger),
		StepEnhancementMiddleware(enhancer, logger),
		AdaptiveStepMiddleware(enhancer, logger),
	)
}
