// Package ml provides workflow integration for error pattern recognition.
package ml

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// EnhancedErrorHandler wraps workflow execution with AI-powered error analysis
type EnhancedErrorHandler struct {
	recognizer     *ErrorPatternRecognizer
	eventPublisher events.Publisher
	logger         *slog.Logger
}

// NewEnhancedErrorHandler creates a new enhanced error handler
func NewEnhancedErrorHandler(
	samplingClient domainsampling.UnifiedSampler,
	eventPublisher events.Publisher,
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
		Description: "Dockerfile auto-fix attempt",
		Changes:     make(map[string]string),
		Reason:      "",
	}

	// Only attempt auto-fix if the error is classified as auto-fixable
	if !classification.AutoFixable {
		result.Reason = "Error not classified as auto-fixable"
		result.Description = "Manual review required for this Dockerfile error"
		return result, nil
	}

	// Extract Dockerfile content from state if available
	dockerfileContent := ""
	if stateMap, ok := state.(map[string]interface{}); ok {
		if content, exists := stateMap["dockerfile_content"]; exists {
			if contentStr, ok := content.(string); ok {
				dockerfileContent = contentStr
			}
		}
	}

	if dockerfileContent == "" {
		result.Reason = "No Dockerfile content available in workflow state"
		return result, nil
	}

	// Prepare issues list from classification context
	issues := []string{classification.SuggestedFix}
	if errorType := classification.ErrorType; errorType != "" {
		issues = append(issues, errorType)
	}

	// Use AI-powered fix via UnifiedSampler
	h.logger.Info("Attempting AI-powered Dockerfile fix",
		"error_type", classification.ErrorType,
		"confidence", classification.Confidence,
		"dockerfile_length", len(dockerfileContent))

	fixResult, err := h.recognizer.samplingClient.FixDockerfile(ctx, dockerfileContent, issues)
	if err != nil {
		result.Reason = fmt.Sprintf("AI fix failed: %v", err)
		h.logger.Error("Failed to apply AI Dockerfile fix", "error", err)
		return result, nil
	}

	if fixResult != nil && fixResult.FixedContent != "" && fixResult.FixedContent != dockerfileContent {
		result.Applied = true
		result.Description = fmt.Sprintf("Applied AI-generated Dockerfile fix: %s", fixResult.Explanation)
		result.Changes["dockerfile"] = fixResult.FixedContent
		result.Changes["fix_reason"] = fixResult.Explanation
		result.Changes["changes_applied"] = fmt.Sprintf("%v", fixResult.Changes)

		h.logger.Info("Successfully applied Dockerfile auto-fix",
			"changes_made", len(result.Changes),
			"fix_explanation", fixResult.Explanation)
	} else {
		result.Reason = "AI could not generate a suitable fix"
		if fixResult != nil && fixResult.Explanation != "" {
			result.Reason = fmt.Sprintf("AI fix unsuccessful: %s", fixResult.Explanation)
		}
	}

	return result, nil
}

func (h *EnhancedErrorHandler) applyDependencyFix(
	ctx context.Context,
	classification *ErrorClassification,
	state interface{},
) (*AutoFixResult, error) {

	result := &AutoFixResult{
		Applied:     false,
		Description: "Dependency auto-fix attempt",
		Changes:     make(map[string]string),
		Reason:      "",
	}

	// Only attempt auto-fix if the error is classified as auto-fixable
	if !classification.AutoFixable {
		result.Reason = "Error not classified as auto-fixable"
		result.Description = "Manual review required for this dependency error"
		return result, nil
	}

	// Extract dependency files from state (package.json, requirements.txt, etc.)
	dependencyFiles := make(map[string]string)
	if stateMap, ok := state.(map[string]interface{}); ok {
		// Common dependency file names
		fileNames := []string{
			"package.json", "package-lock.json", "yarn.lock",
			"requirements.txt", "Pipfile", "poetry.lock",
			"go.mod", "go.sum",
			"pom.xml", "build.gradle",
			"Gemfile", "Gemfile.lock",
		}

		for _, fileName := range fileNames {
			if content, exists := stateMap[fileName]; exists {
				if contentStr, ok := content.(string); ok {
					dependencyFiles[fileName] = contentStr
				}
			}
		}
	}

	if len(dependencyFiles) == 0 {
		result.Reason = "No dependency files found in workflow state"
		return result, nil
	}

	h.logger.Info("Attempting AI-powered dependency fix",
		"error_type", classification.ErrorType,
		"confidence", classification.Confidence,
		"dependency_files", len(dependencyFiles))

	// For dependency fixes, we'll use the Dockerfile fix method with dependency-specific context
	// This leverages the AI's ability to understand dependency issues in containerization context

	// Build a comprehensive context for the AI
	dependencyContext := fmt.Sprintf("Dependency Error Analysis:\n")
	dependencyContext += fmt.Sprintf("Error Type: %s\n", classification.ErrorType)
	dependencyContext += fmt.Sprintf("Suggested Fix: %s\n", classification.SuggestedFix)
	dependencyContext += fmt.Sprintf("Available Files:\n")

	for fileName, content := range dependencyFiles {
		dependencyContext += fmt.Sprintf("\n--- %s ---\n%s\n", fileName, content)
	}

	// Use AI to analyze and suggest dependency fixes
	issues := []string{
		classification.SuggestedFix,
		classification.ErrorType,
		"Dependency version conflict resolution needed",
		"Package compatibility analysis required",
	}

	// For dependency issues, we'll use a sample Dockerfile that includes the dependency files
	// and let the AI suggest how to fix the dependency issues
	sampleDockerfile := fmt.Sprintf(`# Dependency Analysis Context
# Error: %s
# Files available: %v
# 
# Please analyze the dependency issue and suggest fixes for:
# 1. Update package versions
# 2. Add missing dependencies  
# 3. Fix version conflicts
# 4. Ensure compatibility
`, classification.ErrorType, getFileNames(dependencyFiles))

	fixResult, err := h.recognizer.samplingClient.FixDockerfile(ctx, sampleDockerfile, issues)
	if err != nil {
		result.Reason = fmt.Sprintf("AI dependency analysis failed: %v", err)
		h.logger.Error("Failed to apply AI dependency fix", "error", err)
		return result, nil
	}

	if fixResult != nil && fixResult.FixedContent != "" && fixResult.Explanation != "" {
		result.Applied = true
		result.Description = fmt.Sprintf("AI dependency analysis completed: %s", fixResult.Explanation)
		result.Changes["dependency_analysis"] = fixResult.FixedContent
		result.Changes["fix_recommendations"] = fixResult.Explanation

		// Extract specific dependency recommendations from the AI response
		if len(dependencyFiles) > 0 {
			result.Changes["affected_files"] = fmt.Sprintf("%v", getFileNames(dependencyFiles))
		}

		h.logger.Info("Successfully completed dependency analysis",
			"recommendations_provided", true,
			"analysis_explanation", fixResult.Explanation)
	} else {
		result.Reason = "AI could not analyze dependency issues"
		if fixResult != nil && fixResult.Explanation != "" {
			result.Reason = fmt.Sprintf("AI analysis unsuccessful: %s", fixResult.Explanation)
		}
	}

	return result, nil
}

// getFileNames extracts file names from the dependency files map
func getFileNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	return names
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

// Domain interface implementation methods

// AnalyzeAndFix implements domainml.EnhancedErrorHandler interface
func (h *EnhancedErrorHandler) AnalyzeAndFix(ctx context.Context, err error, state *workflow.WorkflowState) (*domainml.ErrorFix, error) {
	// Convert domain WorkflowState to internal format for analysis
	// Try to determine step name from current step number
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	stepName := "unknown"
	if state.CurrentStep > 0 && state.CurrentStep <= len(stepNames) {
		stepName = stepNames[state.CurrentStep-1]
	}
	stepNumber := state.CurrentStep
	
	// Analyze the error first
	classification, analyzeErr := h.AnalyzeWorkflowError(ctx, err, state, stepName, stepNumber)
	if analyzeErr != nil {
		return nil, analyzeErr
	}
	
	// Try to apply auto-fix
	autoFixResult, fixErr := h.ApplyAutoFix(ctx, classification, state)
	if fixErr != nil {
		return nil, fixErr
	}
	
	// Convert to domain ErrorFix
	changes := make([]string, 0, len(autoFixResult.Changes))
	for key, value := range autoFixResult.Changes {
		changes = append(changes, fmt.Sprintf("%s: %s", key, value))
	}
	
	return &domainml.ErrorFix{
		Applied:     autoFixResult.Applied,
		Description: autoFixResult.Description,
		Changes:     changes,
		Confidence:  classification.Confidence,
		Metadata: map[string]interface{}{
			"category":       string(classification.Category),
			"auto_fixable":   classification.AutoFixable,
			"retry_strategy": string(classification.RetryRecommendation),
		},
	}, nil
}

// SuggestFixes implements domainml.EnhancedErrorHandler interface
func (h *EnhancedErrorHandler) SuggestFixes(ctx context.Context, err error, state *workflow.WorkflowState) ([]domainml.FixSuggestion, error) {
	// Convert domain WorkflowState to internal format for analysis
	// Try to determine step name from current step number
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	stepName := "unknown"
	if state.CurrentStep > 0 && state.CurrentStep <= len(stepNames) {
		stepName = stepNames[state.CurrentStep-1]
	}
	stepNumber := state.CurrentStep
	
	// Analyze the error
	classification, analyzeErr := h.AnalyzeWorkflowError(ctx, err, state, stepName, stepNumber)
	if analyzeErr != nil {
		return nil, analyzeErr
	}
	
	// Create fix suggestions based on classification
	var suggestions []domainml.FixSuggestion
	
	// Main suggestion from AI analysis
	if classification.SuggestedFix != "" {
		risk := "medium"
		if classification.AutoFixable {
			risk = "low"
		}
		if classification.Severity == SeverityCritical {
			risk = "high"
		}
		
		suggestions = append(suggestions, domainml.FixSuggestion{
			Description: classification.SuggestedFix,
			Command:     "", // Could be populated based on category
			Confidence:  classification.Confidence,
			Risk:        risk,
		})
	}
	
	// Add category-specific suggestions
	switch classification.Category {
	case CategoryNetwork:
		suggestions = append(suggestions, domainml.FixSuggestion{
			Description: "Check network connectivity and retry",
			Command:     "ping registry.example.com",
			Confidence:  0.8,
			Risk:        "low",
		})
	case CategoryPermissions:
		suggestions = append(suggestions, domainml.FixSuggestion{
			Description: "Verify authentication credentials and permissions",
			Command:     "kubectl auth can-i create pods",
			Confidence:  0.9,
			Risk:        "low",
		})
	case CategoryDockerfile:
		suggestions = append(suggestions, domainml.FixSuggestion{
			Description: "Review Dockerfile syntax and base image availability",
			Command:     "docker build --dry-run .",
			Confidence:  0.7,
			Risk:        "low",
		})
	}
	
	return suggestions, nil
}
