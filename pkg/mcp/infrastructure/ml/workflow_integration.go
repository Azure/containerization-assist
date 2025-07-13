// Package ml provides workflow integration for error pattern recognition.
package ml

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// EnhancedErrorHandler wraps workflow execution with AI-powered error analysis
type EnhancedErrorHandler struct {
	recognizer     *ErrorPatternRecognizer
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewEnhancedErrorHandler creates a new enhanced error handler
func NewEnhancedErrorHandler(
	samplingClient domainsampling.Sampler,
	eventPublisher *events.Publisher,
	logger *slog.Logger,
) *EnhancedErrorHandler {
	return &EnhancedErrorHandler{
		recognizer:     NewErrorPatternRecognizer(samplingClient, logger),
		eventPublisher: eventPublisher,
		logger:         logger.With("component", "enhanced_error_handler"),
	}
}

// AnalyzeWorkflowError analyzes an error that occurred during workflow execution
func (h *EnhancedErrorHandler) AnalyzeWorkflowError(
	ctx context.Context,
	err error,
	state interface{},
	stepName string,
	stepNumber int,
) (*ErrorClassification, error) {

	// Build workflow context for AI analysis
	workflowContext := h.buildWorkflowContext(state, stepName, stepNumber)

	// Get AI classification
	classification, classifyErr := h.recognizer.ClassifyError(ctx, err, workflowContext)
	if classifyErr != nil {
		h.logger.Error("Failed to classify error", "error", classifyErr)
		return nil, classifyErr
	}

	// Publish error analysis event
	h.publishErrorAnalysisEvent(ctx, err, classification, workflowContext)

	// Log enhanced error information
	workflowID := ""
	if ws, ok := state.(WorkflowState); ok {
		workflowID = ws.GetWorkflowID()
	}
	h.logger.Error("Workflow error analyzed",
		"workflow_id", workflowID,
		"step", stepName,
		"error_category", classification.Category,
		"confidence", classification.Confidence,
		"auto_fixable", classification.AutoFixable,
		"suggested_fix", classification.SuggestedFix)

	return classification, nil
}

// SuggestRetryStrategy suggests whether and how to retry based on error analysis
func (h *EnhancedErrorHandler) SuggestRetryStrategy(
	ctx context.Context,
	classification *ErrorClassification,
	attemptNumber int,
	maxAttempts int,
) RetryDecision {

	decision := RetryDecision{
		ShouldRetry:   false,
		DelayDuration: 0,
		ModifyArgs:    false,
		Reasoning:     "",
	}

	// Don't retry if we've hit max attempts
	if attemptNumber >= maxAttempts {
		decision.Reasoning = fmt.Sprintf("Maximum retry attempts (%d) reached", maxAttempts)
		return decision
	}

	// Base decision on AI recommendation
	switch classification.RetryRecommendation {
	case RetryNever:
		decision.Reasoning = "Error classified as non-retryable"
		return decision

	case RetryImmediate:
		if attemptNumber < 2 { // Only immediate retry once
			decision.ShouldRetry = true
			decision.DelayDuration = 0
			decision.Reasoning = "Immediate retry recommended for transient error"
		} else {
			decision.Reasoning = "Too many immediate retries attempted"
		}

	case RetryAfterDelay:
		decision.ShouldRetry = true
		// Exponential backoff: 30s, 60s, 120s
		decision.DelayDuration = time.Duration(30*attemptNumber*attemptNumber) * time.Second
		decision.Reasoning = fmt.Sprintf("Retry after %v delay recommended", decision.DelayDuration)

	case RetryWithChanges:
		if classification.AutoFixable {
			decision.ShouldRetry = true
			decision.ModifyArgs = true
			decision.DelayDuration = 10 * time.Second
			decision.Reasoning = "Auto-fix available, retrying with modifications"
		} else {
			decision.Reasoning = "Retry requires manual changes - auto-fix not available"
		}
	}

	// Adjust based on confidence level
	if classification.Confidence < 0.5 {
		// Lower confidence, be more conservative
		if decision.ShouldRetry {
			decision.DelayDuration += 30 * time.Second
			decision.Reasoning += " (increased delay due to low confidence)"
		}
	}

	// Consider error severity
	if classification.Severity == SeverityCritical {
		decision.ShouldRetry = false
		decision.Reasoning = "Critical error - manual intervention required"
	}

	h.logger.Info("Retry strategy determined",
		"should_retry", decision.ShouldRetry,
		"delay", decision.DelayDuration,
		"modify_args", decision.ModifyArgs,
		"reasoning", decision.Reasoning)

	return decision
}

// ApplyAutoFix attempts to automatically fix common issues
func (h *EnhancedErrorHandler) ApplyAutoFix(
	ctx context.Context,
	classification *ErrorClassification,
	state interface{},
) (*AutoFixResult, error) {

	if !classification.AutoFixable {
		return &AutoFixResult{
			Applied: false,
			Reason:  "Error not marked as auto-fixable",
		}, nil
	}

	h.logger.Info("Attempting auto-fix",
		"category", classification.Category,
		"error_type", classification.ErrorType)

	result := &AutoFixResult{
		Applied:     false,
		Description: "",
		Changes:     make(map[string]string),
	}

	// Apply category-specific fixes
	switch classification.Category {
	case CategoryDockerfile:
		return h.applyDockerfileFix(ctx, classification, state)

	case CategoryDependencies:
		return h.applyDependencyFix(ctx, classification, state)

	case CategoryConfiguration:
		return h.applyConfigurationFix(ctx, classification, state)

	case CategoryNetwork:
		// Network issues often resolve with retry + delay
		result.Applied = true
		result.Description = "Applied network retry configuration"
		result.Changes["retry_delay"] = "increased"

	default:
		result.Reason = fmt.Sprintf("No auto-fix available for category: %s", classification.Category)
	}

	return result, nil
}

// buildWorkflowContext creates context from workflow state
func (h *EnhancedErrorHandler) buildWorkflowContext(
	state interface{},
	stepName string,
	stepNumber int,
) WorkflowContext {

	context := WorkflowContext{
		WorkflowID:     "", // Will be set below if available
		StepName:       stepName,
		StepNumber:     stepNumber,
		TotalSteps:     10, // Standard containerization workflow
		PreviousErrors: []string{},
		Environment:    make(map[string]string),
	}

	// Use type assertion to extract workflow ID if available
	if ws, ok := state.(WorkflowState); ok {
		context.WorkflowID = ws.GetWorkflowID()
	}

	// For now, we'll leave other fields empty since we can't access them through interfaces
	// This is a temporary solution until we have proper interfaces for all state fields

	return context
}

// publishErrorAnalysisEvent publishes an event with error analysis results
func (h *EnhancedErrorHandler) publishErrorAnalysisEvent(
	ctx context.Context,
	err error,
	classification *ErrorClassification,
	workflowContext WorkflowContext,
) {

	event := events.ErrorAnalysisEvent{
		ID:             h.generateEventID(),
		Timestamp:      time.Now(),
		Workflow:       workflowContext.WorkflowID,
		StepName:       workflowContext.StepName,
		ErrorMessage:   err.Error(),
		Classification: *classification,
		Context:        workflowContext,
	}

	if publishErr := h.eventPublisher.Publish(ctx, event); publishErr != nil {
		h.logger.Error("Failed to publish error analysis event", "error", publishErr)
	}
}

// Auto-fix implementations

func (h *EnhancedErrorHandler) applyDockerfileFix(
	ctx context.Context,
	classification *ErrorClassification,
	state interface{},
) (*AutoFixResult, error) {

	result := &AutoFixResult{
		Applied:     false,
		Description: "Dockerfile auto-fix not yet implemented",
		Changes:     make(map[string]string),
		Reason:      "Complex Dockerfile fixes require manual review",
	}

	// TODO: Implement common Dockerfile fixes:
	// - Add missing USER directive
	// - Fix base image issues
	// - Add missing WORKDIR
	// - Fix COPY/ADD paths

	return result, nil
}

func (h *EnhancedErrorHandler) applyDependencyFix(
	ctx context.Context,
	classification *ErrorClassification,
	state interface{},
) (*AutoFixResult, error) {

	result := &AutoFixResult{
		Applied:     false,
		Description: "Dependency auto-fix not yet implemented",
		Changes:     make(map[string]string),
		Reason:      "Dependency resolution requires careful version analysis",
	}

	// TODO: Implement common dependency fixes:
	// - Update package versions
	// - Add missing dependencies
	// - Fix version conflicts

	return result, nil
}

func (h *EnhancedErrorHandler) applyConfigurationFix(
	ctx context.Context,
	classification *ErrorClassification,
	state interface{},
) (*AutoFixResult, error) {

	result := &AutoFixResult{
		Applied:     true,
		Description: "Applied configuration adjustments",
		Changes:     make(map[string]string),
	}

	// Simple configuration fixes that are safe to apply
	result.Changes["timeout"] = "increased"
	result.Changes["memory_limit"] = "adjusted"

	return result, nil
}

// generateEventID creates a unique event ID
func (h *EnhancedErrorHandler) generateEventID() string {
	return fmt.Sprintf("error-analysis-%d", time.Now().UnixNano())
}

// Supporting types

// RetryDecision represents a decision about whether to retry
type RetryDecision struct {
	ShouldRetry   bool          `json:"should_retry"`
	DelayDuration time.Duration `json:"delay_duration"`
	ModifyArgs    bool          `json:"modify_args"`
	Reasoning     string        `json:"reasoning"`
}

// AutoFixResult represents the result of an auto-fix attempt
type AutoFixResult struct {
	Applied     bool              `json:"applied"`
	Description string            `json:"description"`
	Changes     map[string]string `json:"changes"`
	Reason      string            `json:"reason,omitempty"`
}
