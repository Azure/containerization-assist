// Package ml provides enhanced workflow steps with AI-powered error handling.
package ml

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// Step represents a workflow step interface
type Step interface {
	Name() string
	MaxRetries() int
	Execute(ctx context.Context, state interface{}) error
}

// WorkflowState represents the workflow state interface
type WorkflowState interface {
	GetWorkflowID() string
}

// EnhancedStep wraps a regular workflow step with AI-powered error analysis and retry logic
type EnhancedStep struct {
	step         Step
	errorHandler *EnhancedErrorHandler
	maxRetries   int
	logger       *slog.Logger
}

// NewEnhancedStep creates a new enhanced step with error analysis capabilities
func NewEnhancedStep(
	step Step,
	errorHandler *EnhancedErrorHandler,
	logger *slog.Logger,
) *EnhancedStep {
	return &EnhancedStep{
		step:         step,
		errorHandler: errorHandler,
		maxRetries:   step.MaxRetries(),
		logger:       logger.With("enhanced_step", step.Name()),
	}
}

// Name returns the name of the wrapped step
func (e *EnhancedStep) Name() string {
	return e.step.Name()
}

// MaxRetries returns the maximum number of retries for this step
func (e *EnhancedStep) MaxRetries() int {
	return e.maxRetries
}

// Execute runs the step with enhanced error handling and intelligent retry logic
func (e *EnhancedStep) Execute(ctx context.Context, state interface{}) error {
	stepName := e.step.Name()
	e.logger.Info("Starting enhanced step execution", "step", stepName)

	var lastError error
	var lastClassification *ErrorClassification

	for attempt := 1; attempt <= e.maxRetries+1; attempt++ {
		e.logger.Info("Executing step attempt",
			"step", stepName,
			"attempt", attempt,
			"max_attempts", e.maxRetries+1)

		// Execute the actual step
		startTime := time.Now()
		err := e.step.Execute(ctx, state)
		duration := time.Since(startTime)

		if err == nil {
			// Step succeeded
			e.logger.Info("Enhanced step completed successfully",
				"step", stepName,
				"attempt", attempt,
				"duration", duration)

			// Mark any previous errors as resolved
			if lastError != nil && e.errorHandler.recognizer.errorHistory != nil {
				if ws, ok := state.(WorkflowState); ok {
					e.errorHandler.recognizer.errorHistory.MarkResolved(ws.GetWorkflowID(), stepName)
				}
			}

			return nil
		}

		// Step failed - analyze the error
		lastError = err
		e.logger.Error("Step failed, analyzing error",
			"step", stepName,
			"attempt", attempt,
			"error", err.Error(),
			"duration", duration)

		// Get AI analysis of the error
		classification, analyzeErr := e.errorHandler.AnalyzeWorkflowError(
			ctx, err, state, stepName, e.getStepNumber(stepName))

		if analyzeErr != nil {
			e.logger.Error("Error analysis failed", "error", analyzeErr)
			// Continue with basic retry logic if AI analysis fails
			if attempt <= e.maxRetries {
				e.logger.Info("Using basic retry logic due to analysis failure")
				time.Sleep(time.Duration(30*attempt) * time.Second)
				continue
			}
			return err
		}

		lastClassification = classification

		// Decide whether to retry based on AI recommendation
		retryDecision := e.errorHandler.SuggestRetryStrategy(
			ctx, classification, attempt-1, e.maxRetries)

		if !retryDecision.ShouldRetry {
			e.logger.Info("AI recommends not to retry",
				"step", stepName,
				"reasoning", retryDecision.Reasoning)
			break
		}

		// Apply auto-fix if recommended
		if retryDecision.ModifyArgs && classification.AutoFixable {
			autoFixResult, fixErr := e.errorHandler.ApplyAutoFix(ctx, classification, state)
			if fixErr != nil {
				e.logger.Error("Auto-fix failed", "error", fixErr)
			} else if autoFixResult.Applied {
				e.logger.Info("Auto-fix applied",
					"description", autoFixResult.Description,
					"changes", autoFixResult.Changes)
			} else {
				e.logger.Info("Auto-fix not applied", "reason", autoFixResult.Reason)
			}
		}

		// Wait before retry if recommended
		if retryDecision.DelayDuration > 0 {
			e.logger.Info("Waiting before retry",
				"delay", retryDecision.DelayDuration,
				"reasoning", retryDecision.Reasoning)
			time.Sleep(retryDecision.DelayDuration)
		}

		e.logger.Info("Retrying step based on AI recommendation",
			"step", stepName,
			"attempt", attempt+1,
			"reasoning", retryDecision.Reasoning)
	}

	// All retries exhausted
	e.logger.Error("Enhanced step failed after all retries",
		"step", stepName,
		"max_attempts", e.maxRetries+1,
		"final_error", lastError.Error())

	// Return enriched error with AI analysis
	if lastClassification != nil {
		return e.createEnrichedError(lastError, lastClassification, stepName)
	}

	return lastError
}

// getStepNumber maps step names to their position in the workflow
func (e *EnhancedStep) getStepNumber(stepName string) int {
	stepMap := map[string]int{
		"analyze":    1,
		"dockerfile": 2,
		"build":      3,
		"scan":       4,
		"tag":        5,
		"push":       6,
		"manifest":   7,
		"cluster":    8,
		"deploy":     9,
		"verify":     10,
	}

	if num, exists := stepMap[stepName]; exists {
		return num
	}
	return 0
}

// createEnrichedError creates an error with AI analysis information
func (e *EnhancedStep) createEnrichedError(
	originalError error,
	classification *ErrorClassification,
	stepName string,
) error {

	enrichedMsg := fmt.Sprintf(`Step '%s' failed with AI analysis:

Original Error: %s

AI Analysis:
- Category: %s (confidence: %.2f)
- Severity: %s
- Auto-fixable: %v
- Suggested Fix: %s

Patterns: %v`,
		stepName,
		originalError.Error(),
		classification.Category,
		classification.Confidence,
		classification.Severity,
		classification.AutoFixable,
		classification.SuggestedFix,
		classification.Patterns)

	return fmt.Errorf("%s", enrichedMsg)
}

// StepEnhancer provides utilities for enhancing existing workflow steps
type StepEnhancer struct {
	errorHandler *EnhancedErrorHandler
	logger       *slog.Logger
}

// NewStepEnhancer creates a new step enhancer
func NewStepEnhancer(errorHandler *EnhancedErrorHandler, logger *slog.Logger) *StepEnhancer {
	return &StepEnhancer{
		errorHandler: errorHandler,
		logger:       logger,
	}
}

// EnhanceStepInternal wraps a regular step with AI capabilities (internal method)
func (se *StepEnhancer) EnhanceStepInternal(step Step) Step {
	return NewEnhancedStep(step, se.errorHandler, se.logger)
}

// EnhanceAllSteps enhances a slice of workflow steps
func (se *StepEnhancer) EnhanceAllSteps(steps []Step) []Step {
	enhanced := make([]Step, len(steps))
	for i, step := range steps {
		enhanced[i] = se.EnhanceStepInternal(step)
	}
	return enhanced
}

// CreateEnhancedWorkflow creates a workflow with all steps enhanced for AI error handling
func CreateEnhancedWorkflow(
	originalSteps []Step,
	errorHandler *EnhancedErrorHandler,
	logger *slog.Logger,
) []Step {

	enhancer := NewStepEnhancer(errorHandler, logger)
	return enhancer.EnhanceAllSteps(originalSteps)
}

// Domain interface implementation methods

// EnhanceStep implements domainml.StepEnhancer interface
func (se *StepEnhancer) EnhanceStep(ctx context.Context, step workflow.Step, state *workflow.WorkflowState) (workflow.Step, error) {
	// This is a simple adapter - we could enhance this to actually modify steps based on state
	// For now, we just enhance with error handling capabilities
	
	// Create an adapter that bridges our internal Step interface with the domain Step interface
	adapter := &StepAdapter{
		domainStep: step,
		logger:     se.logger,
	}
	
	// Enhance the adapted step using the internal method (not the domain interface method)
	enhanced := se.EnhanceStepInternal(adapter)
	
	// Return the enhanced adapter wrapped as a domain step
	return &EnhancedStepAdapter{
		enhancedStep: enhanced,
		logger:       se.logger,
	}, nil
}

// OptimizeWorkflow implements domainml.StepEnhancer interface
func (se *StepEnhancer) OptimizeWorkflow(ctx context.Context, steps []workflow.Step) (*domainml.WorkflowOptimization, error) {
	// Analyze the workflow and provide optimization suggestions
	suggestions := []domainml.OptimizationSuggestion{}
	
	for i, step := range steps {
		stepName := step.Name()
		
		// Basic optimization suggestions based on step patterns
		switch stepName {
		case "build":
			suggestions = append(suggestions, domainml.OptimizationSuggestion{
				StepName:    stepName,
				Type:        "caching",
				Description: "Enable Docker layer caching to reduce build times",
				Impact:      0.3, // 30% improvement estimate
			})
		case "scan":
			suggestions = append(suggestions, domainml.OptimizationSuggestion{
				StepName:    stepName,
				Type:        "parallel",
				Description: "Run security scanning in parallel with other steps",
				Impact:      0.15, // 15% improvement estimate
			})
		case "deploy":
			if i > 0 && steps[i-1].Name() == "verify" {
				suggestions = append(suggestions, domainml.OptimizationSuggestion{
					StepName:    stepName,
					Type:        "reorder",
					Description: "Move verification step after deployment for faster feedback",
					Impact:      0.1, // 10% improvement estimate
				})
			}
		}
	}
	
	// Calculate estimated improvement (average of all suggestions)
	totalImpact := 0.0
	for _, suggestion := range suggestions {
		totalImpact += suggestion.Impact
	}
	estimatedImprovement := totalImpact / float64(len(suggestions))
	if len(suggestions) == 0 {
		estimatedImprovement = 0.0
	}
	
	return &domainml.WorkflowOptimization{
		Suggestions:          suggestions,
		EstimatedImprovement: estimatedImprovement,
		Metadata: map[string]interface{}{
			"analysis_type": "rule_based",
			"step_count":    len(steps),
			"suggestions":   len(suggestions),
		},
	}, nil
}

// StepAdapter adapts domain Step interface to internal Step interface
type StepAdapter struct {
	domainStep workflow.Step
	logger     *slog.Logger
}

func (sa *StepAdapter) Name() string {
	return sa.domainStep.Name()
}

func (sa *StepAdapter) MaxRetries() int {
	return 3 // Default retry count
}

func (sa *StepAdapter) Execute(ctx context.Context, state interface{}) error {
	// Convert state to WorkflowState if possible
	if workflowState, ok := state.(*workflow.WorkflowState); ok {
		return sa.domainStep.Execute(ctx, workflowState)
	}
	
	// Fallback - create a basic workflow state
	basicState := &workflow.WorkflowState{
		WorkflowID:  "unknown",
		CurrentStep: 1, // Default to step 1
	}
	return sa.domainStep.Execute(ctx, basicState)
}

// EnhancedStepAdapter adapts enhanced internal step back to domain interface
type EnhancedStepAdapter struct {
	enhancedStep Step
	logger       *slog.Logger
}

func (esa *EnhancedStepAdapter) Name() string {
	return esa.enhancedStep.Name()
}

func (esa *EnhancedStepAdapter) MaxRetries() int {
	return esa.enhancedStep.MaxRetries()
}

func (esa *EnhancedStepAdapter) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	return esa.enhancedStep.Execute(ctx, state)
}
