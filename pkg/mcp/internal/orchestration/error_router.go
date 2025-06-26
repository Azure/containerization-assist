package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// DefaultErrorRouter implements ErrorRouter for workflow error handling and recovery
type DefaultErrorRouter struct {
	logger             zerolog.Logger
	classifier         *ErrorClassifier
	router             *ErrorRouter
	recoveryManager    *RecoveryManager
	retryManager       *RetryManager
	redirectionManager *RedirectionManager
}

// NewDefaultErrorRouter creates a new error router with default rules
func NewDefaultErrorRouter(logger zerolog.Logger) *DefaultErrorRouter {
	router := &DefaultErrorRouter{
		logger:             logger.With().Str("component", "error_router").Logger(),
		classifier:         NewErrorClassifier(logger),
		router:             NewErrorRouter(logger),
		recoveryManager:    NewRecoveryManager(logger),
		retryManager:       NewRetryManager(logger),
		redirectionManager: NewRedirectionManager(logger),
	}

	// Initialize with default routing rules
	router.initializeDefaultRules()

	return router
}

// Type aliases removed - types are now directly available in the orchestration package

// RouteError routes an error and determines the appropriate action
func (er *DefaultErrorRouter) RouteError(
	ctx context.Context,
	workflowError *WorkflowError,
	session *WorkflowSession,
) (*ErrorAction, error) {
	er.logger.Info().
		Str("error_id", workflowError.ID).
		Str("stage_name", workflowError.StageName).
		Str("tool_name", workflowError.ToolName).
		Str("error_type", workflowError.ErrorType).
		Msg("Routing workflow error")

	// Find the best matching rule
	bestRule := er.router.FindMatchingRule(workflowError)
	if bestRule == nil {
		er.logger.Debug().
			Str("stage_name", workflowError.StageName).
			Msg("No rules matched error conditions, using default fail action")
		return &ErrorAction{
			Action:  "fail",
			Message: "No routing rules matched error conditions",
		}, nil
	}

	er.logger.Info().
		Str("rule_id", bestRule.ID).
		Str("rule_name", bestRule.Name).
		Str("action", bestRule.Action).
		Msg("Found matching error routing rule")

	// Execute the routing action
	return er.executeRoutingAction(ctx, bestRule, workflowError, session)
}

// IsFatalError determines if an error should be considered fatal and cause immediate workflow failure
func (er *DefaultErrorRouter) IsFatalError(workflowError *WorkflowError) bool {
	return er.classifier.IsFatalError(workflowError)
}

// CanRecover determines if an error can be recovered from
func (er *DefaultErrorRouter) CanRecover(workflowError *WorkflowError) bool {
	recoveryStrategies := make(map[string]RecoveryStrategy)
	// Get all recovery strategies from recovery manager
	for _, id := range []string{"network_recovery", "resource_recovery"} {
		if strategy, exists := er.recoveryManager.GetRecoveryStrategy(id); exists {
			recoveryStrategies[id] = strategy
		}
	}
	return er.classifier.CanRecover(workflowError, recoveryStrategies)
}

// GetRecoveryOptions returns available recovery options for an error
func (er *DefaultErrorRouter) GetRecoveryOptions(workflowError *WorkflowError) []RecoveryOption {
	// Convert from errors.RecoveryOption to RecoveryOption
	options := er.recoveryManager.GetRecoveryOptions(workflowError, er.classifier)
	result := make([]RecoveryOption, len(options))
	for i, opt := range options {
		result[i] = RecoveryOption{
			Name:        opt.Name,
			Description: opt.Description,
			Action:      opt.Action,
			Parameters:  opt.Parameters,
			Probability: opt.Probability,
			Cost:        opt.Cost,
		}
	}
	return result
}

// AddRoutingRule adds a custom routing rule
func (er *DefaultErrorRouter) AddRoutingRule(stageName string, rule ErrorRoutingRule) {
	er.router.AddRoutingRule(stageName, rule)
}

// AddRecoveryStrategy adds a custom recovery strategy
func (er *DefaultErrorRouter) AddRecoveryStrategy(strategy RecoveryStrategy) {
	er.recoveryManager.AddRecoveryStrategy(strategy)
}

// SetRetryPolicy sets a retry policy for a specific stage
func (er *DefaultErrorRouter) SetRetryPolicy(stageName string, policy *RetryPolicy) {
	// Convert from RetryPolicy to RetryPolicy
	errorsPolicy := &RetryPolicy{
		MaxAttempts:  policy.MaxAttempts,
		BackoffMode:  policy.BackoffMode,
		InitialDelay: policy.InitialDelay,
		MaxDelay:     policy.MaxDelay,
		Multiplier:   policy.Multiplier,
	}
	er.retryManager.SetRetryPolicy(stageName, errorsPolicy)
}

// Internal implementation methods

// ValidateRedirectTarget validates that a redirect target is valid and available
func (er *DefaultErrorRouter) ValidateRedirectTarget(redirectTo string, workflowError *WorkflowError) error {
	return er.redirectionManager.ValidateRedirectTarget(redirectTo, workflowError)
}

// CreateRedirectionPlan creates a detailed plan for error redirection
func (er *DefaultErrorRouter) CreateRedirectionPlan(
	redirectTo string,
	workflowError *WorkflowError,
	session *WorkflowSession,
) (*RedirectionPlan, error) {
	return er.redirectionManager.CreateRedirectionPlan(redirectTo, workflowError, session)
}

func (er *DefaultErrorRouter) initializeDefaultRules() {
	// Initialize modules with default configurations
	er.recoveryManager.InitializeDefaultStrategies()
	er.retryManager.InitializeDefaultPolicies()

	// Initialize Sprint A enhanced cross-tool escalation rules
	er.InitializeSprintAEscalationRules()

	// Default rules for common error types

	// Fatal errors - immediate failure with no retry
	er.addDefaultRule("*", ErrorRoutingRule{
		ID:          "fatal_error_fail",
		Name:        "Fatal Error Immediate Failure",
		Description: "Immediately fail workflow for fatal errors",
		Conditions: []RoutingCondition{
			{Field: "severity", Operator: "equals", Value: "critical"},
		},
		Action:   "fail",
		Priority: 200, // Highest priority
		Enabled:  true,
	})

	// Authentication errors - redirect to retry authentication
	er.addDefaultRule("*", ErrorRoutingRule{
		ID:          "auth_error_redirect",
		Name:        "Authentication Error Redirect",
		Description: "Redirect authentication errors to credential refresh",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "authentication"},
			{Field: "severity", Operator: "not_equals", Value: "critical"},
		},
		Action:     "redirect",
		RedirectTo: "retry_authentication",
		Parameters: &ErrorRoutingParameters{
			CustomParams: map[string]string{
				"clear_cache":      "true",
				"prompt_for_creds": "true",
			},
		},
		Priority: 150,
		Enabled:  true,
	})

	// Network errors - retry with backoff
	er.addDefaultRule("*", ErrorRoutingRule{
		ID:          "network_error_retry",
		Name:        "Network Error Retry",
		Description: "Retry network-related errors with exponential backoff",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "network"},
		},
		Action: "retry",
		RetryPolicy: &RetryPolicy{
			MaxAttempts:  3,
			BackoffMode:  "exponential",
			InitialDelay: 5 * time.Second,
			MaxDelay:     60 * time.Second,
			Multiplier:   2.0,
		},
		Priority: 100,
		Enabled:  true,
	})

	// Timeout errors - retry with longer timeout
	er.addDefaultRule("*", ErrorRoutingRule{
		ID:          "timeout_error_retry",
		Name:        "Timeout Error Retry",
		Description: "Retry timeout errors with increased timeout",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "timeout"},
		},
		Action: "retry",
		RetryPolicy: &RetryPolicy{
			MaxAttempts:  2,
			BackoffMode:  "fixed",
			InitialDelay: 10 * time.Second,
		},
		Parameters: &ErrorRoutingParameters{
			IncreaseTimeout:   true,
			TimeoutMultiplier: 2.0,
		},
		Priority: 90,
		Enabled:  true,
	})

	// Resource unavailable - wait and retry
	er.addDefaultRule("*", ErrorRoutingRule{
		ID:          "resource_unavailable_retry",
		Name:        "Resource Unavailable Retry",
		Description: "Wait and retry when resources are unavailable",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "resource_unavailable"},
		},
		Action: "retry",
		RetryPolicy: &RetryPolicy{
			MaxAttempts:  5,
			BackoffMode:  "linear",
			InitialDelay: 30 * time.Second,
			MaxDelay:     300 * time.Second,
			Multiplier:   1.5,
		},
		Priority: 80,
		Enabled:  true,
	})

	// Authentication errors - fail fast (usually need manual intervention)
	er.addDefaultRule("*", ErrorRoutingRule{
		ID:          "auth_error_fail",
		Name:        "Authentication Error Fail",
		Description: "Fail fast on authentication errors",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "authentication"},
			{Field: "severity", Operator: "equals", Value: "high"},
		},
		Action:   "fail",
		Priority: 120,
		Enabled:  true,
	})

	// Build errors in Dockerfile generation - redirect to manual validation
	er.addDefaultRule("build_image", ErrorRoutingRule{
		ID:          "build_error_redirect",
		Name:        "Build Error Redirect",
		Description: "Redirect build errors to Dockerfile validation",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "build_error"},
		},
		Action:     "redirect",
		RedirectTo: "validate_dockerfile",
		Parameters: &ErrorRoutingParameters{
			ValidationMode: "strict",
			FixErrors:      true,
		},
		Priority: 110,
		Enabled:  true,
	})

	// Security scan failures - continue with warnings for non-critical
	er.addDefaultRule("scan_image_security", ErrorRoutingRule{
		ID:          "security_scan_warning",
		Name:        "Security Scan Warning",
		Description: "Continue with warnings for non-critical security issues",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "contains", Value: "security_scan"},
			{Field: "severity", Operator: "not_equals", Value: "critical"},
		},
		Action: "skip",
		Parameters: &ErrorRoutingParameters{
			AddWarning:       true,
			ContinueWorkflow: true,
		},
		Priority: 70,
		Enabled:  true,
	})

}

func (er *DefaultErrorRouter) executeRoutingAction(
	ctx context.Context,
	rule *ErrorRoutingRule,
	workflowError *WorkflowError,
	session *WorkflowSession,
) (*ErrorAction, error) {
	// Convert ErrorRoutingParameters to map[string]interface{}
	parameters := make(map[string]interface{})
	if rule.Parameters != nil {
		parameters["increase_timeout"] = rule.Parameters.IncreaseTimeout
		parameters["timeout_multiplier"] = rule.Parameters.TimeoutMultiplier
		parameters["validation_mode"] = rule.Parameters.ValidationMode
		parameters["fix_errors"] = rule.Parameters.FixErrors
		parameters["add_warning"] = rule.Parameters.AddWarning
		parameters["continue_workflow"] = rule.Parameters.ContinueWorkflow
		if rule.Parameters.CustomParams != nil {
			for k, v := range rule.Parameters.CustomParams {
				parameters[k] = v
			}
		}
	}

	action := &ErrorAction{
		Action:     rule.Action,
		Parameters: parameters,
		Message:    fmt.Sprintf("Applied routing rule: %s", rule.Name),
	}

	switch rule.Action {
	case "retry":
		retryPolicy := rule.RetryPolicy
		if retryPolicy == nil {
			// Use stage-specific retry policy or default
			retryPolicy = er.retryManager.GetRetryPolicy(workflowError.StageName)
		}

		// Calculate retry delay
		retryCount := 0
		if session.ErrorContext != nil {
			if count, ok := session.ErrorContext["retry_count"].(int); ok {
				retryCount = count
			}
		}

		retryAfter := er.retryManager.CalculateRetryDelay(retryPolicy, retryCount)
		action.RetryAfter = &retryAfter

		er.logger.Info().
			Str("stage_name", workflowError.StageName).
			Int("retry_count", retryCount).
			Dur("retry_after", retryAfter).
			Msg("Scheduling retry for stage")

	case "redirect":
		// Validate redirect target
		if err := er.ValidateRedirectTarget(rule.RedirectTo, workflowError); err != nil {
			er.logger.Error().
				Err(err).
				Str("redirect_to", rule.RedirectTo).
				Str("from_stage", workflowError.StageName).
				Msg("Invalid redirect target, falling back to fail action")
			action.Action = "fail"
			action.Message = fmt.Sprintf("Redirection failed: %v", err)
			break
		}

		// Create detailed redirection plan
		redirectPlan, err := er.CreateRedirectionPlan(rule.RedirectTo, workflowError, session)
		if err != nil {
			er.logger.Error().
				Err(err).
				Str("redirect_to", rule.RedirectTo).
				Msg("Failed to create redirection plan, falling back to fail action")
			action.Action = "fail"
			action.Message = fmt.Sprintf("Redirection planning failed: %v", err)
			break
		}

		action.RedirectTo = rule.RedirectTo

		// Add redirection plan details to parameters
		if action.Parameters == nil {
			action.Parameters = make(map[string]interface{})
		}
		action.Parameters["redirection_plan"] = redirectPlan
		action.Parameters["estimated_duration"] = redirectPlan.EstimatedDuration.String()
		action.Parameters["context_preservation"] = redirectPlan.ContextPreservation
		action.Parameters["intervention_required"] = redirectPlan.InterventionRequired

		// Check for missing context and warn
		if len(redirectPlan.MissingContext) > 0 {
			er.logger.Warn().
				Strs("missing_context", redirectPlan.MissingContext).
				Str("redirect_to", rule.RedirectTo).
				Msg("Redirection proceeding with missing context")
			action.Parameters["missing_context"] = redirectPlan.MissingContext
		}

		er.logger.Info().
			Str("from_stage", workflowError.StageName).
			Str("to_stage", rule.RedirectTo).
			Dur("estimated_duration", redirectPlan.EstimatedDuration).
			Str("expected_outcome", redirectPlan.ExpectedOutcome).
			Msg("Redirecting to alternative stage with detailed plan")

	case "skip":
		er.logger.Info().
			Str("stage_name", workflowError.StageName).
			Msg("Skipping stage due to routing rule")

	case "fail":
		er.logger.Info().
			Str("stage_name", workflowError.StageName).
			Msg("Failing workflow due to routing rule")
	}

	return action, nil
}

func (er *DefaultErrorRouter) addDefaultRule(stageName string, rule ErrorRoutingRule) {
	er.router.AddRoutingRule(stageName, rule)
}
