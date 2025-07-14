// Package ml provides machine learning capabilities for Container Kit MCP.
package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"

	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ErrorClassification represents the AI analysis of an error
type ErrorClassification struct {
	ErrorType           string            `json:"error_type"`
	Confidence          float64           `json:"confidence"` // 0.0 to 1.0
	Category            ErrorCategory     `json:"category"`
	Severity            ErrorSeverity     `json:"severity"`
	SuggestedFix        string            `json:"suggested_fix"`
	AutoFixable         bool              `json:"auto_fixable"`
	RetryRecommendation RetryStrategy     `json:"retry_recommendation"`
	Context             map[string]string `json:"context"`
	Patterns            []string          `json:"patterns"`
}

// ErrorCategory represents the broad category of an error
type ErrorCategory string

const (
	CategoryDockerfile    ErrorCategory = "dockerfile"
	CategoryBuild         ErrorCategory = "build"
	CategoryRegistry      ErrorCategory = "registry"
	CategoryKubernetes    ErrorCategory = "kubernetes"
	CategoryNetwork       ErrorCategory = "network"
	CategoryPermissions   ErrorCategory = "permissions"
	CategoryDependencies  ErrorCategory = "dependencies"
	CategoryConfiguration ErrorCategory = "configuration"
	CategoryResource      ErrorCategory = "resource"
	CategoryUnknown       ErrorCategory = "unknown"
)

// ErrorSeverity represents how severe an error is
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// RetryStrategy represents recommended retry approach
type RetryStrategy string

const (
	RetryImmediate   RetryStrategy = "immediate"
	RetryAfterDelay  RetryStrategy = "after_delay"
	RetryWithChanges RetryStrategy = "with_changes"
	RetryNever       RetryStrategy = "never"
)

// WorkflowContext provides context about the workflow when error occurred
type WorkflowContext struct {
	WorkflowID     string            `json:"workflow_id"`
	StepName       string            `json:"step_name"`
	StepNumber     int               `json:"step_number"`
	TotalSteps     int               `json:"total_steps"`
	RepoURL        string            `json:"repo_url"`
	Branch         string            `json:"branch"`
	Language       string            `json:"language,omitempty"`
	Framework      string            `json:"framework,omitempty"`
	Dependencies   []string          `json:"dependencies,omitempty"`
	PreviousErrors []string          `json:"previous_errors,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	// Args field removed to break circular dependency
}

// ErrorPatternRecognizer provides AI-powered error analysis using existing sampling client
type ErrorPatternRecognizer struct {
	samplingClient domainsampling.UnifiedSampler
	errorHistory   *ErrorHistoryStore
	logger         *slog.Logger
}

// NewErrorPatternRecognizer creates a new error pattern recognizer
func NewErrorPatternRecognizer(samplingClient domainsampling.UnifiedSampler, logger *slog.Logger) *ErrorPatternRecognizer {
	return &ErrorPatternRecognizer{
		samplingClient: samplingClient,
		errorHistory:   NewErrorHistoryStore(),
		logger:         logger.With("component", "error_pattern_recognizer"),
	}
}

// ClassifyError analyzes an error using AI and historical patterns
func (r *ErrorPatternRecognizer) ClassifyError(ctx context.Context, err error, context WorkflowContext) (*ErrorClassification, error) {
	if err == nil {
		return nil, fmt.Errorf("cannot classify nil error")
	}

	r.logger.Info("Classifying error",
		"workflow_id", context.WorkflowID,
		"step", context.StepName,
		"error", err.Error())

	// Build AI analysis prompt
	prompt := r.buildErrorAnalysisPrompt(err, context)

	// Use existing sampling client for AI analysis
	response, aiErr := r.samplingClient.Sample(ctx, domainsampling.Request{
		Prompt:      prompt,
		Temperature: 0.1, // Low temperature for consistent analysis
		MaxTokens:   1000,
	})

	if aiErr != nil {
		r.logger.Error("AI analysis failed", "error", aiErr)
		// Fallback to rule-based classification
		return r.fallbackClassification(err, context), nil
	}

	// Parse AI response into classification
	classification, parseErr := r.parseAIResponse(response.Content, err, context)
	if parseErr != nil {
		r.logger.Error("Failed to parse AI response", "error", parseErr)
		// Fallback to rule-based classification
		return r.fallbackClassification(err, context), nil
	}

	// Enhance with historical patterns
	r.enhanceWithHistory(classification, err, context)

	// Store in history for future learning
	r.errorHistory.RecordError(err, context, classification)

	r.logger.Info("Error classification completed",
		"workflow_id", context.WorkflowID,
		"category", classification.Category,
		"confidence", classification.Confidence,
		"auto_fixable", classification.AutoFixable)

	return classification, nil
}

// buildErrorAnalysisPrompt creates a detailed prompt for AI error analysis
func (r *ErrorPatternRecognizer) buildErrorAnalysisPrompt(err error, context WorkflowContext) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert containerization troubleshooter. Analyze this error and provide a structured response.\n\n")

	// Error details
	prompt.WriteString(fmt.Sprintf("ERROR: %s\n\n", err.Error()))

	// Workflow context
	prompt.WriteString("WORKFLOW CONTEXT:\n")
	prompt.WriteString(fmt.Sprintf("- Step: %s (%d/%d)\n", context.StepName, context.StepNumber, context.TotalSteps))
	prompt.WriteString(fmt.Sprintf("- Repository: %s\n", context.RepoURL))
	if context.Branch != "" {
		prompt.WriteString(fmt.Sprintf("- Branch: %s\n", context.Branch))
	}
	if context.Language != "" {
		prompt.WriteString(fmt.Sprintf("- Language: %s\n", context.Language))
	}
	if context.Framework != "" {
		prompt.WriteString(fmt.Sprintf("- Framework: %s\n", context.Framework))
	}
	if len(context.Dependencies) > 0 {
		prompt.WriteString(fmt.Sprintf("- Dependencies: %s\n", strings.Join(context.Dependencies, ", ")))
	}

	// Previous errors if any
	if len(context.PreviousErrors) > 0 {
		prompt.WriteString("\nPREVIOUS ERRORS:\n")
		for i, prevErr := range context.PreviousErrors {
			prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, prevErr))
		}
	}

	// Instructions for structured response
	prompt.WriteString(`
PROVIDE A JSON RESPONSE WITH THIS STRUCTURE:
{
  "error_type": "specific error type",
  "confidence": 0.95,
  "category": "dockerfile|build|registry|kubernetes|network|permissions|dependencies|configuration|resource|unknown",
  "severity": "low|medium|high|critical",
  "suggested_fix": "specific actionable fix",
  "auto_fixable": true/false,
  "retry_recommendation": "immediate|after_delay|with_changes|never",
  "context": {
    "key": "value"
  },
  "patterns": ["pattern1", "pattern2"]
}

ANALYSIS GUIDELINES:
- confidence: 0.0-1.0 based on how certain you are
- auto_fixable: true only if the fix can be automated
- suggested_fix: specific, actionable steps
- patterns: common error patterns this matches
- context: relevant technical details

Focus on containerization workflow errors (Docker, Kubernetes, registry, build issues).`)

	return prompt.String()
}

// parseAIResponse parses the AI response into an ErrorClassification
func (r *ErrorPatternRecognizer) parseAIResponse(response string, err error, context WorkflowContext) (*ErrorClassification, error) {
	// Extract JSON from response (AI might include extra text)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}") + 1

	if start == -1 || end <= start {
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	jsonStr := response[start:end]

	var classification ErrorClassification
	if parseErr := json.Unmarshal([]byte(jsonStr), &classification); parseErr != nil {
		return nil, fmt.Errorf("failed to parse AI response JSON: %w", parseErr)
	}

	// Validate and sanitize the classification
	if classification.Confidence < 0.0 {
		classification.Confidence = 0.0
	}
	if classification.Confidence > 1.0 {
		classification.Confidence = 1.0
	}

	if classification.Category == "" {
		classification.Category = CategoryUnknown
	}

	if classification.Severity == "" {
		classification.Severity = SeverityMedium
	}

	if classification.RetryRecommendation == "" {
		classification.RetryRecommendation = RetryAfterDelay
	}

	return &classification, nil
}

// fallbackClassification provides rule-based classification when AI fails
func (r *ErrorPatternRecognizer) fallbackClassification(err error, context WorkflowContext) *ErrorClassification {
	errorMsg := strings.ToLower(err.Error())

	classification := &ErrorClassification{
		ErrorType:           "unknown",
		Confidence:          0.3, // Lower confidence for rule-based
		Category:            CategoryUnknown,
		Severity:            SeverityMedium,
		SuggestedFix:        "Check logs and retry with different configuration",
		AutoFixable:         false,
		RetryRecommendation: RetryAfterDelay,
		Context:             make(map[string]string),
		Patterns:            []string{},
	}

	// Rule-based pattern matching
	switch {
	case strings.Contains(errorMsg, "dockerfile"):
		classification.Category = CategoryDockerfile
		classification.SuggestedFix = "Review Dockerfile syntax and base image availability"

	case strings.Contains(errorMsg, "build") && strings.Contains(errorMsg, "failed"):
		classification.Category = CategoryBuild
		classification.SuggestedFix = "Check build dependencies and ensure all required files are present"

	case strings.Contains(errorMsg, "registry") || strings.Contains(errorMsg, "push") || strings.Contains(errorMsg, "pull"):
		classification.Category = CategoryRegistry
		classification.SuggestedFix = "Verify registry credentials and connectivity"

	case strings.Contains(errorMsg, "kubernetes") || strings.Contains(errorMsg, "kubectl"):
		classification.Category = CategoryKubernetes
		classification.SuggestedFix = "Check Kubernetes cluster connectivity and permissions"

	case strings.Contains(errorMsg, "network") || strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "connection"):
		classification.Category = CategoryNetwork
		classification.SuggestedFix = "Check network connectivity and firewall settings"
		classification.RetryRecommendation = RetryImmediate

	case strings.Contains(errorMsg, "permission") || strings.Contains(errorMsg, "denied") || strings.Contains(errorMsg, "unauthorized"):
		classification.Category = CategoryPermissions
		classification.SuggestedFix = "Check file permissions and authentication credentials"
		classification.RetryRecommendation = RetryNever

	case strings.Contains(errorMsg, "dependency") || strings.Contains(errorMsg, "package") || strings.Contains(errorMsg, "module"):
		classification.Category = CategoryDependencies
		classification.SuggestedFix = "Review dependency versions and availability"
	}

	// Set error type based on category
	classification.ErrorType = string(classification.Category) + "_error"

	return classification
}

// enhanceWithHistory enhances classification with historical patterns
func (r *ErrorPatternRecognizer) enhanceWithHistory(classification *ErrorClassification, err error, context WorkflowContext) {
	// Look for similar errors in history
	similarErrors := r.errorHistory.FindSimilarErrors(err, context)

	if len(similarErrors) > 0 {
		// Increase confidence if we've seen this pattern before
		classification.Confidence = math.Min(classification.Confidence+0.2, 1.0)

		// Add historical patterns
		for _, similar := range similarErrors {
			if similar.Classification != nil && len(similar.Classification.Patterns) > 0 {
				classification.Patterns = append(classification.Patterns, similar.Classification.Patterns...)
			}
		}

		// Remove duplicates
		classification.Patterns = removeDuplicates(classification.Patterns)

		r.logger.Info("Enhanced classification with historical data",
			"similar_errors", len(similarErrors),
			"patterns", len(classification.Patterns))
	}
}

// Helper functions

func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// Domain interface implementation methods

// RecognizePattern implements domainml.ErrorPatternRecognizer interface
func (r *ErrorPatternRecognizer) RecognizePattern(ctx context.Context, err error, stepContext *workflow.WorkflowState) (*domainml.ErrorClassification, error) {
	// Convert WorkflowState to our internal WorkflowContext
	stepName := "unknown"
	repoURL := ""
	branch := ""
	
	if stepContext.Args != nil {
		repoURL = stepContext.Args.RepoURL
		branch = stepContext.Args.Branch
	}
	
	// Try to determine step name from current step number
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	if stepContext.CurrentStep > 0 && stepContext.CurrentStep <= len(stepNames) {
		stepName = stepNames[stepContext.CurrentStep-1]
	}
	
	context := WorkflowContext{
		WorkflowID: stepContext.WorkflowID,
		StepName:   stepName,
		StepNumber: stepContext.CurrentStep,
		TotalSteps: stepContext.TotalSteps,
		RepoURL:    repoURL,
		Branch:     branch,
	}
	
	// Use existing ClassifyError method
	classification, classifyErr := r.ClassifyError(ctx, err, context)
	if classifyErr != nil {
		return nil, classifyErr
	}
	
	// Convert to domain ErrorClassification
	return &domainml.ErrorClassification{
		Category:    classification.ErrorType,
		Confidence:  classification.Confidence,
		Patterns:    classification.Patterns,
		Suggestions: []string{classification.SuggestedFix},
		Metadata: map[string]interface{}{
			"severity":         string(classification.Severity),
			"auto_fixable":     classification.AutoFixable,
			"retry_strategy":   string(classification.RetryRecommendation),
			"category":         string(classification.Category),
		},
	}, nil
}

// GetSimilarErrors implements domainml.ErrorPatternRecognizer interface
func (r *ErrorPatternRecognizer) GetSimilarErrors(ctx context.Context, err error) ([]domainml.HistoricalError, error) {
	// Get similar errors from our error history
	similarErrors := r.errorHistory.FindSimilarErrors(err, WorkflowContext{})
	
	// Convert to domain HistoricalError format
	var result []domainml.HistoricalError
	for _, similar := range similarErrors {
		solutions := []string{}
		if similar.Classification != nil && similar.Classification.SuggestedFix != "" {
			solutions = append(solutions, similar.Classification.SuggestedFix)
		}
		
		result = append(result, domainml.HistoricalError{
			Error:      similar.Error,
			Context:    similar.StepName,
			Solutions:  solutions,
			Similarity: 0.8, // Default similarity score
			Timestamp:  similar.Timestamp.Format("2006-01-02T15:04:05Z"),
		})
	}
	
	return result, nil
}
