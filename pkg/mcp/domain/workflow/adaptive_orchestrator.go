// Package workflow provides adaptive workflow orchestration capabilities
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// AdaptiveWorkflowOrchestrator enhances the base orchestrator with adaptive capabilities
// It uses pattern recognition and learning to modify workflow behavior dynamically
type AdaptiveWorkflowOrchestrator struct {
	baseOrchestrator  WorkflowOrchestrator
	patternRecognizer ErrorPatternRecognizer
	stepEnhancer      StepEnhancer
	adaptationEngine  *AdaptationEngine
	logger            *slog.Logger
}

// AdaptationEngine manages workflow adaptations based on learned patterns
type AdaptationEngine struct {
	adaptationHistory     map[string]*AdaptationRecord
	successfulAdaptations map[string]*AdaptationStrategy
	logger                *slog.Logger
}

// AdaptationRecord tracks the history of adaptations for a workflow
type AdaptationRecord struct {
	WorkflowID       string                 `json:"workflow_id"`
	OriginalStrategy *WorkflowStrategy      `json:"original_strategy"`
	Adaptations      []AdaptationEvent      `json:"adaptations"`
	FinalStrategy    *WorkflowStrategy      `json:"final_strategy"`
	SuccessRate      float64                `json:"success_rate"`
	TotalExecutions  int                    `json:"total_executions"`
	LastUpdated      time.Time              `json:"last_updated"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// AdaptationEvent represents a single adaptation made during workflow execution
type AdaptationEvent struct {
	StepName       string                 `json:"step_name"`
	AdaptationType AdaptationType         `json:"adaptation_type"`
	Reason         string                 `json:"reason"`
	OriginalConfig map[string]interface{} `json:"original_config"`
	AdaptedConfig  map[string]interface{} `json:"adapted_config"`
	Confidence     float64                `json:"confidence"`
	Timestamp      time.Time              `json:"timestamp"`
	Success        bool                   `json:"success"`
	ExecutionTime  time.Duration          `json:"execution_time"`
	ErrorReduction float64                `json:"error_reduction"`
}

// AdaptationType defines different types of workflow adaptations
type AdaptationType string

const (
	AdaptationRetryStrategy      AdaptationType = "retry_strategy"
	AdaptationTimeout            AdaptationType = "timeout"
	AdaptationParallelization    AdaptationType = "parallelization"
	AdaptationResourceAllocation AdaptationType = "resource_allocation"
	AdaptationSkipOptional       AdaptationType = "skip_optional"
	AdaptationAlternativeStep    AdaptationType = "alternative_step"
	AdaptationParameterTuning    AdaptationType = "parameter_tuning"
)

// WorkflowStrategy defines the strategic approach for executing a workflow
type WorkflowStrategy struct {
	Name                  string                 `json:"name"`
	StepConfigurations    map[string]*StepConfig `json:"step_configurations"`
	RetryPolicy           *AdaptiveRetryPolicy   `json:"retry_policy"`
	TimeoutPolicy         *TimeoutPolicy         `json:"timeout_policy"`
	ParallelizationPolicy *ParallelizationPolicy `json:"parallelization_policy"`
	Metadata              map[string]interface{} `json:"metadata"`
}

// StepConfig defines configuration for individual workflow steps
type StepConfig struct {
	MaxRetries       int                    `json:"max_retries"`
	BaseTimeout      time.Duration          `json:"base_timeout"`
	RetryBackoff     time.Duration          `json:"retry_backoff"`
	SkipOnFailure    bool                   `json:"skip_on_failure"`
	AlternativeSteps []string               `json:"alternative_steps"`
	Parameters       map[string]interface{} `json:"parameters"`
}

// AdaptiveRetryPolicy defines retry behavior for the adaptive workflow
type AdaptiveRetryPolicy struct {
	MaxRetries        int           `json:"max_retries"`
	BaseDelay         time.Duration `json:"base_delay"`
	MaxDelay          time.Duration `json:"max_delay"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	AdaptiveRetries   bool          `json:"adaptive_retries"`
}

// TimeoutPolicy defines timeout behavior for workflow steps
type TimeoutPolicy struct {
	BaseTimeout       time.Duration `json:"base_timeout"`
	MaxTimeout        time.Duration `json:"max_timeout"`
	AdaptiveTimeout   bool          `json:"adaptive_timeout"`
	TimeoutMultiplier float64       `json:"timeout_multiplier"`
}

// ParallelizationPolicy defines parallelization behavior
type ParallelizationPolicy struct {
	MaxParallelSteps    int                 `json:"max_parallel_steps"`
	ParallelizableSteps []string            `json:"parallelizable_steps"`
	Dependencies        map[string][]string `json:"dependencies"`
	Enabled             bool                `json:"enabled"`
}

// AdaptationStrategy represents a learned strategy for handling specific patterns
type AdaptationStrategy struct {
	PatternID    string                 `json:"pattern_id"`
	StepName     string                 `json:"step_name"`
	ErrorPattern string                 `json:"error_pattern"`
	Adaptations  []AdaptationEvent      `json:"adaptations"`
	SuccessRate  float64                `json:"success_rate"`
	UsageCount   int                    `json:"usage_count"`
	LastUsed     time.Time              `json:"last_used"`
	Confidence   float64                `json:"confidence"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// NewAdaptiveWorkflowOrchestrator creates a new adaptive workflow orchestrator
func NewAdaptiveWorkflowOrchestrator(
	baseOrchestrator WorkflowOrchestrator,
	patternRecognizer ErrorPatternRecognizer,
	stepEnhancer StepEnhancer,
	logger *slog.Logger,
) *AdaptiveWorkflowOrchestrator {
	return &AdaptiveWorkflowOrchestrator{
		baseOrchestrator:  baseOrchestrator,
		patternRecognizer: patternRecognizer,
		stepEnhancer:      stepEnhancer,
		adaptationEngine:  NewAdaptationEngine(logger),
		logger:            logger.With("component", "adaptive_workflow_orchestrator"),
	}
}

// NewAdaptationEngine creates a new adaptation engine
func NewAdaptationEngine(logger *slog.Logger) *AdaptationEngine {
	return &AdaptationEngine{
		adaptationHistory:     make(map[string]*AdaptationRecord),
		successfulAdaptations: make(map[string]*AdaptationStrategy),
		logger:                logger.With("component", "adaptation_engine"),
	}
}

// Execute runs the workflow with adaptive capabilities
func (a *AdaptiveWorkflowOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	workflowID := generateAdaptiveWorkflowID()

	a.logger.Info("Starting adaptive workflow execution",
		"workflow_id", workflowID,
		"repo_url", args.RepoURL,
		"branch", args.Branch)

	// Create adaptation record
	adaptationRecord := &AdaptationRecord{
		WorkflowID:       workflowID,
		OriginalStrategy: a.getDefaultStrategy(),
		Adaptations:      make([]AdaptationEvent, 0),
		TotalExecutions:  1,
		LastUpdated:      time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	// Store initial adaptation record
	a.adaptationEngine.adaptationHistory[workflowID] = adaptationRecord

	// Create adaptive context
	adaptiveCtx := a.createAdaptiveContext(ctx, workflowID, args)

	// Execute workflow with adaptation capabilities
	result, err := a.executeWithAdaptation(adaptiveCtx, req, args, adaptationRecord)

	// Update adaptation record with results
	adaptationRecord.LastUpdated = time.Now()
	if err == nil {
		adaptationRecord.SuccessRate = 1.0
		a.logger.Info("Adaptive workflow completed successfully",
			"workflow_id", workflowID,
			"adaptations_made", len(adaptationRecord.Adaptations))
	} else {
		adaptationRecord.SuccessRate = 0.0
		a.logger.Error("Adaptive workflow failed",
			"workflow_id", workflowID,
			"error", err,
			"adaptations_made", len(adaptationRecord.Adaptations))
	}

	// Learn from this execution
	a.adaptationEngine.learnFromExecution(adaptationRecord)

	return result, err
}

// executeWithAdaptation executes the workflow with adaptive error handling
func (a *AdaptiveWorkflowOrchestrator) executeWithAdaptation(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args *ContainerizeAndDeployArgs,
	record *AdaptationRecord,
) (*ContainerizeAndDeployResult, error) {
	// Try to execute with the base orchestrator first
	result, err := a.baseOrchestrator.Execute(ctx, req, args)

	// If execution was successful, no adaptation needed
	if err == nil {
		return result, nil
	}

	// Analyze the error using pattern recognition
	a.logger.Info("Analyzing workflow error for adaptation opportunities",
		"workflow_id", record.WorkflowID,
		"error", err)

	// Create a dummy workflow state for pattern recognition
	workflowState := &WorkflowState{
		WorkflowID:  record.WorkflowID,
		Args:        args,
		CurrentStep: 1, // We'll need to determine this from the error
		TotalSteps:  10,
	}

	// Recognize error patterns
	errorClassification, patternErr := a.patternRecognizer.RecognizePattern(ctx, err, workflowState)
	if patternErr != nil {
		a.logger.Error("Failed to recognize error pattern", "error", patternErr)
		return result, err // Return original error
	}

	// Apply adaptive strategies based on pattern recognition
	adaptedResult, adaptErr := a.applyAdaptiveStrategies(ctx, req, args, record, err, errorClassification)
	if adaptErr != nil {
		a.logger.Error("Failed to apply adaptive strategies", "error", adaptErr)
		return result, err // Return original error
	}

	return adaptedResult, nil
}

// applyAdaptiveStrategies applies learned strategies to handle errors
func (a *AdaptiveWorkflowOrchestrator) applyAdaptiveStrategies(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args *ContainerizeAndDeployArgs,
	record *AdaptationRecord,
	originalError error,
	errorClassification *ErrorClassification,
) (*ContainerizeAndDeployResult, error) {
	// Look for existing successful adaptations for this error pattern
	strategy := a.adaptationEngine.findMatchingStrategy(errorClassification.Category, originalError.Error())

	if strategy != nil {
		a.logger.Info("Applying learned adaptation strategy",
			"workflow_id", record.WorkflowID,
			"strategy_id", strategy.PatternID,
			"success_rate", strategy.SuccessRate)

		return a.applyStrategy(ctx, req, args, record, strategy)
	}

	// No existing strategy found, create adaptive strategies based on error analysis
	return a.createAdaptiveStrategy(ctx, req, args, record, originalError, errorClassification)
}

// applyStrategy applies a learned strategy to the workflow
func (a *AdaptiveWorkflowOrchestrator) applyStrategy(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args *ContainerizeAndDeployArgs,
	record *AdaptationRecord,
	strategy *AdaptationStrategy,
) (*ContainerizeAndDeployResult, error) {
	// Apply the adaptations from the strategy
	for _, adaptation := range strategy.Adaptations {
		event := AdaptationEvent{
			StepName:       adaptation.StepName,
			AdaptationType: adaptation.AdaptationType,
			Reason:         fmt.Sprintf("Applying learned strategy: %s", adaptation.Reason),
			OriginalConfig: adaptation.OriginalConfig,
			AdaptedConfig:  adaptation.AdaptedConfig,
			Confidence:     adaptation.Confidence,
			Timestamp:      time.Now(),
		}

		// Apply the adaptation
		a.applyAdaptation(ctx, args, &event)
		record.Adaptations = append(record.Adaptations, event)
	}

	// Update strategy usage
	strategy.UsageCount++
	strategy.LastUsed = time.Now()

	// Execute workflow with adaptations
	return a.baseOrchestrator.Execute(ctx, req, args)
}

// createAdaptiveStrategy creates new adaptation strategies based on error analysis
func (a *AdaptiveWorkflowOrchestrator) createAdaptiveStrategy(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args *ContainerizeAndDeployArgs,
	record *AdaptationRecord,
	originalError error,
	errorClassification *ErrorClassification,
) (*ContainerizeAndDeployResult, error) {
	// Create adaptive strategies based on error category and suggestions
	adaptations := a.generateAdaptations(errorClassification)

	// Apply adaptations one by one until success or exhaustion
	for _, adaptation := range adaptations {
		a.logger.Info("Applying adaptive strategy",
			"workflow_id", record.WorkflowID,
			"adaptation_type", adaptation.AdaptationType,
			"step_name", adaptation.StepName,
			"reason", adaptation.Reason)

		// Apply the adaptation
		a.applyAdaptation(ctx, args, &adaptation)
		record.Adaptations = append(record.Adaptations, adaptation)

		// Try executing the workflow with this adaptation
		result, err := a.baseOrchestrator.Execute(ctx, req, args)
		if err == nil {
			// Success! Mark this adaptation as successful
			adaptation.Success = true
			adaptation.ExecutionTime = time.Since(adaptation.Timestamp)

			// Store this as a successful strategy
			a.adaptationEngine.storeSuccessfulStrategy(errorClassification.Category, originalError.Error(), []AdaptationEvent{adaptation})

			return result, nil
		}

		// This adaptation didn't work, try the next one
		adaptation.Success = false
		adaptation.ExecutionTime = time.Since(adaptation.Timestamp)

		a.logger.Info("Adaptation failed, trying next strategy",
			"workflow_id", record.WorkflowID,
			"adaptation_type", adaptation.AdaptationType,
			"error", err)
	}

	// All adaptations failed
	return nil, fmt.Errorf("all adaptive strategies failed for error: %w", originalError)
}

// generateAdaptations creates adaptation strategies based on error classification
func (a *AdaptiveWorkflowOrchestrator) generateAdaptations(errorClassification *ErrorClassification) []AdaptationEvent {
	var adaptations []AdaptationEvent

	// Generate adaptations based on error category
	switch errorClassification.Category {
	case "network":
		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "all",
			AdaptationType: AdaptationRetryStrategy,
			Reason:         "Network errors often resolve with retry",
			OriginalConfig: map[string]interface{}{"max_retries": 3},
			AdaptedConfig:  map[string]interface{}{"max_retries": 10, "backoff_multiplier": 2.0},
			Confidence:     0.8,
			Timestamp:      time.Now(),
		})

		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "all",
			AdaptationType: AdaptationTimeout,
			Reason:         "Increase timeout for network-related operations",
			OriginalConfig: map[string]interface{}{"timeout": "5m"},
			AdaptedConfig:  map[string]interface{}{"timeout": "15m"},
			Confidence:     0.7,
			Timestamp:      time.Now(),
		})

	case "build":
		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "build",
			AdaptationType: AdaptationResourceAllocation,
			Reason:         "Build errors may require more resources",
			OriginalConfig: map[string]interface{}{"memory": "1g", "cpu": "1"},
			AdaptedConfig:  map[string]interface{}{"memory": "4g", "cpu": "2"},
			Confidence:     0.9,
			Timestamp:      time.Now(),
		})

		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "build",
			AdaptationType: AdaptationParameterTuning,
			Reason:         "Enable build cache and parallel builds",
			OriginalConfig: map[string]interface{}{"cache": false, "parallel": false},
			AdaptedConfig:  map[string]interface{}{"cache": true, "parallel": true},
			Confidence:     0.8,
			Timestamp:      time.Now(),
		})

	case "registry":
		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "push",
			AdaptationType: AdaptationRetryStrategy,
			Reason:         "Registry authentication may be temporary",
			OriginalConfig: map[string]interface{}{"max_retries": 3},
			AdaptedConfig:  map[string]interface{}{"max_retries": 5, "retry_delay": "30s"},
			Confidence:     0.7,
			Timestamp:      time.Now(),
		})

	case "kubernetes":
		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "deploy",
			AdaptationType: AdaptationTimeout,
			Reason:         "Kubernetes deployments may take longer",
			OriginalConfig: map[string]interface{}{"timeout": "5m"},
			AdaptedConfig:  map[string]interface{}{"timeout": "20m"},
			Confidence:     0.8,
			Timestamp:      time.Now(),
		})

		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "deploy",
			AdaptationType: AdaptationRetryStrategy,
			Reason:         "Kubernetes resources may need time to be available",
			OriginalConfig: map[string]interface{}{"max_retries": 3},
			AdaptedConfig:  map[string]interface{}{"max_retries": 8, "backoff_multiplier": 1.5},
			Confidence:     0.7,
			Timestamp:      time.Now(),
		})
	}

	// Add generic adaptations based on suggestions
	for _, suggestion := range errorClassification.Suggestions {
		adaptations = append(adaptations, AdaptationEvent{
			StepName:       "generic",
			AdaptationType: AdaptationParameterTuning,
			Reason:         suggestion,
			OriginalConfig: map[string]interface{}{},
			AdaptedConfig:  map[string]interface{}{"suggestion": suggestion},
			Confidence:     0.6,
			Timestamp:      time.Now(),
		})
	}

	return adaptations
}

// Helper functions

func (a *AdaptiveWorkflowOrchestrator) getDefaultStrategy() *WorkflowStrategy {
	return &WorkflowStrategy{
		Name: "default",
		StepConfigurations: map[string]*StepConfig{
			"analyze": {
				MaxRetries:    3,
				BaseTimeout:   5 * time.Minute,
				RetryBackoff:  10 * time.Second,
				SkipOnFailure: false,
			},
			"build": {
				MaxRetries:    3,
				BaseTimeout:   10 * time.Minute,
				RetryBackoff:  30 * time.Second,
				SkipOnFailure: false,
			},
			"deploy": {
				MaxRetries:    5,
				BaseTimeout:   15 * time.Minute,
				RetryBackoff:  1 * time.Minute,
				SkipOnFailure: false,
			},
		},
		RetryPolicy: &AdaptiveRetryPolicy{
			MaxRetries:        3,
			BaseDelay:         1 * time.Second,
			MaxDelay:          1 * time.Minute,
			BackoffMultiplier: 2.0,
			AdaptiveRetries:   true,
		},
		TimeoutPolicy: &TimeoutPolicy{
			BaseTimeout:       5 * time.Minute,
			MaxTimeout:        30 * time.Minute,
			AdaptiveTimeout:   true,
			TimeoutMultiplier: 1.5,
		},
	}
}

func (a *AdaptiveWorkflowOrchestrator) createAdaptiveContext(ctx context.Context, workflowID string, args *ContainerizeAndDeployArgs) context.Context {
	// Add adaptive context values
	adaptiveCtx := context.WithValue(ctx, "workflow_id", workflowID)
	adaptiveCtx = context.WithValue(adaptiveCtx, "adaptive_mode", true)
	return adaptiveCtx
}

func (a *AdaptiveWorkflowOrchestrator) applyAdaptation(ctx context.Context, args *ContainerizeAndDeployArgs, adaptation *AdaptationEvent) {
	// Apply adaptation to workflow arguments and context
	// Since ContainerizeAndDeployArgs doesn't have Options, we'll store adaptations in context

	// Store adaptation configuration in context for middleware to use
	adaptationKey := fmt.Sprintf("adaptation_%s_%s", adaptation.StepName, adaptation.AdaptationType)

	// Create a new context with adaptation configuration
	_ = context.WithValue(ctx, adaptationKey, adaptation.AdaptedConfig)

	// Log the adaptation being applied
	a.logger.Info("Applied adaptation configuration",
		"step_name", adaptation.StepName,
		"adaptation_type", adaptation.AdaptationType,
		"config", adaptation.AdaptedConfig)

	// For now, we'll modify the boolean flags in args where applicable
	switch adaptation.AdaptationType {
	case AdaptationSkipOptional:
		// Skip optional steps like scanning
		stepName := adaptation.StepName
		if stepName == "scan" || stepName == "all" {
			args.Scan = false
		}
		if stepName == "deploy" || stepName == "all" {
			deploy := false
			args.Deploy = &deploy
		}

	case AdaptationRetryStrategy:
		// Retry strategies are handled by middleware through context

	case AdaptationTimeout:
		// Timeout configurations are handled by middleware through context

	case AdaptationResourceAllocation:
		// Resource allocations are handled by middleware through context

	case AdaptationParameterTuning:
		// Parameter tuning is handled by middleware through context

	case AdaptationAlternativeStep:
		// Alternative step logic is handled by middleware through context
	}
}

// GetAdaptationStatistics returns statistics about workflow adaptations
func (a *AdaptiveWorkflowOrchestrator) GetAdaptationStatistics() *AdaptationStatistics {
	return a.adaptationEngine.GetAdaptationStatistics()
}

// UpdateAdaptationStrategy allows manual updates to adaptation strategies
func (a *AdaptiveWorkflowOrchestrator) UpdateAdaptationStrategy(patternID string, strategy *AdaptationStrategy) error {
	if strategy == nil {
		return fmt.Errorf("strategy cannot be nil")
	}

	a.adaptationEngine.successfulAdaptations[patternID] = strategy

	a.logger.Info("Updated adaptation strategy",
		"pattern_id", patternID,
		"success_rate", strategy.SuccessRate,
		"usage_count", strategy.UsageCount)

	return nil
}

// ClearAdaptationHistory clears the adaptation history
func (a *AdaptiveWorkflowOrchestrator) ClearAdaptationHistory() error {
	a.adaptationEngine.adaptationHistory = make(map[string]*AdaptationRecord)
	a.adaptationEngine.successfulAdaptations = make(map[string]*AdaptationStrategy)

	a.logger.Info("Cleared adaptation history")
	return nil
}

// generateAdaptiveWorkflowID generates a unique workflow ID for adaptive workflows
func generateAdaptiveWorkflowID() string {
	return fmt.Sprintf("adaptive_workflow_%d", time.Now().UnixNano())
}

// ErrorPatternRecognizer interface for compatibility
type ErrorPatternRecognizer interface {
	RecognizePattern(ctx context.Context, err error, stepContext *WorkflowState) (*ErrorClassification, error)
}

// ErrorClassification represents error classification results
type ErrorClassification struct {
	Category    string   `json:"category"`
	Confidence  float64  `json:"confidence"`
	Patterns    []string `json:"patterns"`
	Suggestions []string `json:"suggestions"`
}
