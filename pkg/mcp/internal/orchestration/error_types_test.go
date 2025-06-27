package orchestration

import (
	"testing"
	"time"
)

// Test ErrorRoutingRule type
func TestErrorRoutingRule(t *testing.T) {
	retryPolicy := &RetryPolicy{
		MaxAttempts:  3,
		BackoffMode:  "exponential",
		InitialDelay: time.Second,
		MaxDelay:     time.Minute,
		Multiplier:   2.0,
	}

	parameters := &ErrorRoutingParameters{
		IncreaseTimeout:   true,
		TimeoutMultiplier: 1.5,
		ValidationMode:    "strict",
		FixErrors:         true,
		CustomParams:      map[string]string{"debug": "true"},
	}

	rule := ErrorRoutingRule{
		ID:          "rule-001",
		Name:        "Docker Build Retry",
		Description: "Retry Docker build failures",
		Conditions: []RoutingCondition{
			{Field: "error_type", Operator: "equals", Value: "build_failure", CaseSensitive: false},
		},
		Action:      "retry",
		RedirectTo:  "",
		RetryPolicy: retryPolicy,
		Parameters:  parameters,
		Priority:    10,
		Enabled:     true,
	}

	if rule.ID != "rule-001" {
		t.Errorf("Expected ID to be 'rule-001', got '%s'", rule.ID)
	}
	if rule.Name != "Docker Build Retry" {
		t.Errorf("Expected Name to be 'Docker Build Retry', got '%s'", rule.Name)
	}
	if rule.Description != "Retry Docker build failures" {
		t.Errorf("Expected Description to be 'Retry Docker build failures', got '%s'", rule.Description)
	}
	if len(rule.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(rule.Conditions))
	}
	if rule.Conditions[0].Field != "error_type" {
		t.Errorf("Expected condition field to be 'error_type', got '%s'", rule.Conditions[0].Field)
	}
	if rule.Action != "retry" {
		t.Errorf("Expected Action to be 'retry', got '%s'", rule.Action)
	}
	if rule.RetryPolicy == nil {
		t.Error("Expected RetryPolicy to not be nil")
	}
	if rule.Parameters == nil {
		t.Error("Expected Parameters to not be nil")
	}
	if rule.Priority != 10 {
		t.Errorf("Expected Priority to be 10, got %d", rule.Priority)
	}
	if !rule.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

// Test RoutingCondition type
func TestRoutingCondition(t *testing.T) {
	condition := RoutingCondition{
		Field:         "tool_name",
		Operator:      "contains",
		Value:         "docker",
		CaseSensitive: true,
	}

	if condition.Field != "tool_name" {
		t.Errorf("Expected Field to be 'tool_name', got '%s'", condition.Field)
	}
	if condition.Operator != "contains" {
		t.Errorf("Expected Operator to be 'contains', got '%s'", condition.Operator)
	}
	if condition.Value != "docker" {
		t.Errorf("Expected Value to be 'docker', got '%v'", condition.Value)
	}
	if !condition.CaseSensitive {
		t.Error("Expected CaseSensitive to be true")
	}
}

// Test ErrorRoutingParameters type
func TestErrorRoutingParameters(t *testing.T) {
	params := ErrorRoutingParameters{
		IncreaseTimeout:   true,
		TimeoutMultiplier: 2.5,
		ValidationMode:    "lenient",
		FixErrors:         false,
		AddWarning:        true,
		ContinueWorkflow:  false,
		CustomParams:      map[string]string{"level": "debug", "verbose": "true"},
	}

	if !params.IncreaseTimeout {
		t.Error("Expected IncreaseTimeout to be true")
	}
	if params.TimeoutMultiplier != 2.5 {
		t.Errorf("Expected TimeoutMultiplier to be 2.5, got %f", params.TimeoutMultiplier)
	}
	if params.ValidationMode != "lenient" {
		t.Errorf("Expected ValidationMode to be 'lenient', got '%s'", params.ValidationMode)
	}
	if params.FixErrors {
		t.Error("Expected FixErrors to be false")
	}
	if !params.AddWarning {
		t.Error("Expected AddWarning to be true")
	}
	if params.ContinueWorkflow {
		t.Error("Expected ContinueWorkflow to be false")
	}
	if len(params.CustomParams) != 2 {
		t.Errorf("Expected 2 custom params, got %d", len(params.CustomParams))
	}
	if params.CustomParams["level"] != "debug" {
		t.Errorf("Expected CustomParams['level'] to be 'debug', got '%s'", params.CustomParams["level"])
	}
}

// Test RetryPolicy type
func TestRetryPolicy(t *testing.T) {
	initialDelay := time.Second * 2
	maxDelay := time.Minute * 5

	policy := RetryPolicy{
		MaxAttempts:  5,
		BackoffMode:  "exponential",
		InitialDelay: initialDelay,
		MaxDelay:     maxDelay,
		Multiplier:   1.5,
	}

	if policy.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts to be 5, got %d", policy.MaxAttempts)
	}
	if policy.BackoffMode != "exponential" {
		t.Errorf("Expected BackoffMode to be 'exponential', got '%s'", policy.BackoffMode)
	}
	if policy.InitialDelay != initialDelay {
		t.Errorf("Expected InitialDelay to be %v, got %v", initialDelay, policy.InitialDelay)
	}
	if policy.MaxDelay != maxDelay {
		t.Errorf("Expected MaxDelay to be %v, got %v", maxDelay, policy.MaxDelay)
	}
	if policy.Multiplier != 1.5 {
		t.Errorf("Expected Multiplier to be 1.5, got %f", policy.Multiplier)
	}
}

// Test RetryPolicy with different backoff modes
func TestRetryPolicyBackoffModes(t *testing.T) {
	// Test fixed backoff
	fixedPolicy := RetryPolicy{
		MaxAttempts:  3,
		BackoffMode:  "fixed",
		InitialDelay: time.Second,
	}

	if fixedPolicy.BackoffMode != "fixed" {
		t.Errorf("Expected BackoffMode to be 'fixed', got '%s'", fixedPolicy.BackoffMode)
	}

	// Test linear backoff
	linearPolicy := RetryPolicy{
		MaxAttempts:  4,
		BackoffMode:  "linear",
		InitialDelay: time.Second * 3,
		Multiplier:   1.0,
	}

	if linearPolicy.BackoffMode != "linear" {
		t.Errorf("Expected BackoffMode to be 'linear', got '%s'", linearPolicy.BackoffMode)
	}

	// Test exponential backoff
	exponentialPolicy := RetryPolicy{
		MaxAttempts:  6,
		BackoffMode:  "exponential",
		InitialDelay: time.Millisecond * 500,
		MaxDelay:     time.Minute,
		Multiplier:   2.0,
	}

	if exponentialPolicy.BackoffMode != "exponential" {
		t.Errorf("Expected BackoffMode to be 'exponential', got '%s'", exponentialPolicy.BackoffMode)
	}
}
