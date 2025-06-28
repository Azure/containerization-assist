package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
	"github.com/rs/zerolog"
)

type DefaultErrorRouter struct {
	logger             zerolog.Logger
	classifier         *ErrorClassifier
	router             *ErrorRouter
	recoveryManager    *RecoveryManager
	retryCoordinator   *retry.Coordinator
	redirectionManager *RedirectionManager
	escalationHandler  *CrossToolEscalationHandler
}

func NewDefaultErrorRouter(logger zerolog.Logger) *DefaultErrorRouter {
	router := &DefaultErrorRouter{
		logger:             logger.With().Str("component", "error_router").Logger(),
		classifier:         NewErrorClassifier(logger),
		router:             NewErrorRouter(logger),
		recoveryManager:    NewRecoveryManager(logger),
		retryCoordinator:   retry.New(),
		redirectionManager: NewRedirectionManager(logger),
	}

	router.initializeDefaultRules()

	return router
}

func (er *DefaultErrorRouter) SetEscalationHandler(handler *CrossToolEscalationHandler) {
	er.escalationHandler = handler
	er.logger.Info().Msg("Cross-tool escalation handler configured")
}

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

	return er.executeRoutingAction(ctx, bestRule, workflowError, session)
}

func (er *DefaultErrorRouter) IsFatalError(workflowError *WorkflowError) bool {
	return er.classifier.IsFatalError(workflowError)
}

func (er *DefaultErrorRouter) CanRecover(workflowError *WorkflowError) bool {
	recoveryStrategies := make(map[string]RecoveryStrategy)
	for _, id := range []string{"network_recovery", "resource_recovery"} {
		if strategy, exists := er.recoveryManager.GetRecoveryStrategy(id); exists {
			recoveryStrategies[id] = strategy
		}
	}
	return er.classifier.CanRecover(workflowError, recoveryStrategies)
}

func (er *DefaultErrorRouter) GetRecoveryOptions(workflowError *WorkflowError) []RecoveryOption {
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

func (er *DefaultErrorRouter) AddRoutingRule(stageName string, rule ErrorRoutingRule) {
	er.router.AddRoutingRule(stageName, rule)
}

func (er *DefaultErrorRouter) AddRecoveryStrategy(strategy RecoveryStrategy) {
	er.recoveryManager.AddRecoveryStrategy(strategy)
}

func (er *DefaultErrorRouter) SetRetryPolicy(stageName string, policy *RetryPolicy) {
	retryPolicy := &retry.Policy{
		MaxAttempts:     policy.MaxAttempts,
		InitialDelay:    policy.InitialDelay,
		MaxDelay:        policy.MaxDelay,
		BackoffStrategy: retry.BackoffExponential,
		Multiplier:      policy.Multiplier,
		Jitter:          true,
		ErrorPatterns:   []string{"timeout", "connection refused", "temporary failure"},
	}
	er.retryCoordinator.SetPolicy(stageName, retryPolicy)
	er.logger.Info().Str("stage", stageName).Msg("Retry policy configuration moved to unified coordinator")
}

func (er *DefaultErrorRouter) ValidateRedirectTarget(redirectTo string, workflowError *WorkflowError) error {
	return er.redirectionManager.ValidateRedirectTarget(redirectTo, workflowError)
}

func (er *DefaultErrorRouter) CreateRedirectionPlan(
	redirectTo string,
	workflowError *WorkflowError,
	session *WorkflowSession,
) (*RedirectionPlan, error) {
	return er.redirectionManager.CreateRedirectionPlan(redirectTo, workflowError, session)
}

func (er *DefaultErrorRouter) initializeDefaultRules() {
	er.recoveryManager.InitializeDefaultStrategies()
	er.logger.Info().Msg("Default retry policies initialized by unified coordinator")

	er.InitializeSprintAEscalationRules()

	er.addDefaultRule("*", ErrorRoutingRule{
		ID:          "fatal_error_fail",
		Name:        "Fatal Error Immediate Failure",
		Description: "Immediately fail workflow for fatal errors",
		Conditions: []RoutingCondition{
			{Field: "severity", Operator: "equals", Value: "critical"},
		},
		Action:   "fail",
		Priority: 200,
		Enabled:  true,
	})

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
			retryPolicy = &RetryPolicy{
				MaxAttempts:  3,
				BackoffMode:  "exponential",
				InitialDelay: 5 * time.Second,
				MaxDelay:     60 * time.Second,
				Multiplier:   2.0,
			}
		}

		retryCount := 0
		if session.ErrorContext != nil {
			if count, ok := session.ErrorContext["retry_count"].(int); ok {
				retryCount = count
			}
		}

		unifiedPolicy := &retry.Policy{
			MaxAttempts:     retryPolicy.MaxAttempts,
			InitialDelay:    retryPolicy.InitialDelay,
			MaxDelay:        retryPolicy.MaxDelay,
			BackoffStrategy: retry.BackoffExponential,
			Multiplier:      retryPolicy.Multiplier,
			Jitter:          true,
			ErrorPatterns:   []string{"timeout", "connection refused", "temporary failure"},
		}

		retryAfter := er.retryCoordinator.CalculateDelay(unifiedPolicy, retryCount)
		action.RetryAfter = &retryAfter

		er.logger.Info().
			Str("stage_name", workflowError.StageName).
			Int("retry_count", retryCount).
			Dur("retry_after", retryAfter).
			Msg("Scheduling retry for stage")

	case "redirect":
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

		if action.Parameters == nil {
			action.Parameters = make(map[string]interface{})
		}
		action.Parameters["redirection_plan"] = redirectPlan
		action.Parameters["estimated_duration"] = redirectPlan.EstimatedDuration.String()
		action.Parameters["context_preservation"] = redirectPlan.ContextPreservation
		action.Parameters["intervention_required"] = redirectPlan.InterventionRequired

		if len(redirectPlan.MissingContext) > 0 {
			er.logger.Warn().
				Strs("missing_context", redirectPlan.MissingContext).
				Str("redirect_to", rule.RedirectTo).
				Msg("Redirection proceeding with missing context")
			action.Parameters["missing_context"] = redirectPlan.MissingContext
		}

		if er.escalationHandler != nil && er.IsEscalatedOperation(parameters) {
			er.logger.Info().
				Str("source_tool", workflowError.ToolName).
				Str("target_tool", rule.RedirectTo).
				Msg("Using enhanced cross-tool escalation handler")

			action.Parameters["escalation_enhanced"] = true
			action.Parameters["escalation_source"] = er.GetEscalationSource(parameters)
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
