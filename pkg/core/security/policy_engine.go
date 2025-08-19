// Package security provides security policy enforcement capabilities
package security

import (
	"context"
	"fmt"
	"time"

	mcperrors "github.com/Azure/containerization-assist/pkg/common/errors"
	"github.com/rs/zerolog"
)

// PolicyEngine enforces security policies on scan results
type PolicyEngine struct {
	logger   zerolog.Logger
	policies []Policy
}

// NewPolicyEngine creates a new policy enforcement engine
func NewPolicyEngine(logger zerolog.Logger) *PolicyEngine {
	return &PolicyEngine{
		logger:   logger.With().Str("component", "policy_engine").Logger(),
		policies: make([]Policy, 0),
	}
}

// LoadPolicies loads security policies from configuration
func (pe *PolicyEngine) LoadPolicies(policies []Policy) error {
	pe.logger.Info().Int("count", len(policies)).Msg("Loading security policies")

	// Validate policies
	for _, policy := range policies {
		if err := pe.validatePolicy(policy); err != nil {
			return mcperrors.New(mcperrors.CodeValidationFailed, "core", fmt.Sprintf("invalid policy %s: %v", policy.ID, err), err)
		}
	}

	pe.policies = policies
	pe.logger.Info().Int("loaded", len(pe.policies)).Msg("Security policies loaded successfully")
	return nil
}

// EvaluatePolicies evaluates all enabled policies against the scan context
func (pe *PolicyEngine) EvaluatePolicies(ctx context.Context, scanCtx *ScanContext) ([]PolicyEvaluationResult, error) {
	pe.logger.Debug().
		Str("image", scanCtx.ImageRef).
		Int("policies", len(pe.policies)).
		Msg("Evaluating security policies")

	var results []PolicyEvaluationResult

	for _, policy := range pe.policies {
		if !policy.Enabled {
			pe.logger.Debug().Str("policy", policy.ID).Msg("Skipping disabled policy")
			continue
		}

		result, err := pe.evaluatePolicy(ctx, policy, scanCtx)
		if err != nil {
			pe.logger.Error().
				Err(err).
				Str("policy", policy.ID).
				Msg("Failed to evaluate policy")
			continue
		}

		results = append(results, *result)
	}

	pe.logger.Info().
		Int("evaluated", len(results)).
		Int("violations", pe.countViolations(results)).
		Msg("Policy evaluation completed")

	return results, nil
}

// evaluatePolicy evaluates a single policy against the scan context
func (pe *PolicyEngine) evaluatePolicy(_ context.Context, policy Policy, scanCtx *ScanContext) (*PolicyEvaluationResult, error) {
	pe.logger.Debug().Str("policy", policy.ID).Msg("Evaluating policy")

	result := &PolicyEvaluationResult{
		PolicyID:    policy.ID,
		PolicyName:  policy.Name,
		Passed:      true,
		Violations:  make([]PolicyViolation, 0),
		Actions:     policy.Actions,
		EvaluatedAt: time.Now(),
		Metadata: map[string]interface{}{
			"policy_severity": policy.Severity,
			"policy_category": policy.Category,
		},
	}

	// Evaluate each rule in the policy
	for _, rule := range policy.Rules {
		violation, err := pe.evaluateRule(rule, scanCtx)
		if err != nil {
			pe.logger.Error().
				Err(err).
				Str("rule", rule.ID).
				Msg("Failed to evaluate rule")
			continue
		}

		if violation != nil {
			violation.Severity = policy.Severity
			result.Violations = append(result.Violations, *violation)
			result.Passed = false
		}
	}

	pe.logger.Debug().
		Str("policy", policy.ID).
		Bool("passed", result.Passed).
		Int("violations", len(result.Violations)).
		Msg("Policy evaluation completed")

	return result, nil
}

// validatePolicy validates a security policy
func (pe *PolicyEngine) validatePolicy(policy Policy) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if len(policy.Rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}
	if len(policy.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}

	// Validate each rule
	for _, rule := range policy.Rules {
		if err := pe.validateRule(rule); err != nil {
			return fmt.Errorf("invalid rule %s: %v", rule.ID, err)
		}
	}

	return nil
}

// validateRule validates a policy rule
func (pe *PolicyEngine) validateRule(rule PolicyRule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if rule.Type == "" {
		return fmt.Errorf("rule type is required")
	}
	if rule.Operator == "" {
		return fmt.Errorf("rule operator is required")
	}
	if rule.Value == nil {
		return fmt.Errorf("rule value is required")
	}

	// Validate rule type specific requirements
	switch rule.Type {
	case RuleTypeVulnerabilityCount, RuleTypeVulnerabilitySeverity:
		if rule.Field == "" {
			return fmt.Errorf("field is required for %s rule", rule.Type)
		}
	}

	return nil
}

// GetPolicies returns all loaded policies
func (pe *PolicyEngine) GetPolicies() []Policy {
	return pe.policies
}

// GetPolicyByID returns a policy by ID
func (pe *PolicyEngine) GetPolicyByID(id string) (*Policy, error) {
	for _, policy := range pe.policies {
		if policy.ID == id {
			return &policy, nil
		}
	}
	return nil, mcperrors.New(mcperrors.CodeNotFound, "security", fmt.Sprintf("policy not found: %s", id), nil)
}

func (pe *PolicyEngine) AddPolicy(policy Policy) error {
	// Check for duplicate ID
	for _, existing := range pe.policies {
		if existing.ID == policy.ID {
			return mcperrors.New(mcperrors.CodeIoError, "security", fmt.Sprintf("policy with ID %s already exists", policy.ID), nil)
		}
	}

	if err := pe.validatePolicy(policy); err != nil {
		return err
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	pe.policies = append(pe.policies, policy)
	pe.logger.Info().Str("policy", policy.ID).Msg("Policy added")

	return nil
}

// UpdatePolicy updates an existing policy
func (pe *PolicyEngine) UpdatePolicy(policy Policy) error {
	// Validate the policy
	if err := pe.validatePolicy(policy); err != nil {
		return err
	}

	for i, existing := range pe.policies {
		if existing.ID == policy.ID {
			policy.CreatedAt = existing.CreatedAt
			policy.UpdatedAt = time.Now()
			pe.policies[i] = policy
			pe.logger.Info().Str("policy", policy.ID).Msg("Policy updated")
			return nil
		}
	}

	return mcperrors.New(mcperrors.CodeNotFound, "security", fmt.Sprintf("policy not found: %s", policy.ID), nil)
}

func (pe *PolicyEngine) RemovePolicy(id string) error {
	for i, policy := range pe.policies {
		if policy.ID == id {
			pe.policies = append(pe.policies[:i], pe.policies[i+1:]...)
			pe.logger.Info().Str("policy", id).Msg("Policy removed")
			return nil
		}
	}
	return mcperrors.New(mcperrors.CodeNotFound, "security", fmt.Sprintf("policy not found: %s", id), nil)
}

func (pe *PolicyEngine) ShouldBlock(results []PolicyEvaluationResult) bool {
	for _, result := range results {
		if !result.Passed {
			for _, action := range result.Actions {
				if action.Type == ActionTypeBlock {
					return true
				}
			}
		}
	}
	return false
}

// GetViolationsSummary returns a summary of all violations
func (pe *PolicyEngine) GetViolationsSummary(results []PolicyEvaluationResult) map[string]interface{} {
	severityCounts := map[string]int{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
	}

	actionCounts := map[string]int{
		"block":  0,
		"warn":   0,
		"notify": 0,
		"log":    0,
	}

	summary := map[string]interface{}{
		"total_policies":         len(results),
		"passed_policies":        0,
		"failed_policies":        0,
		"total_violations":       0,
		"severity_counts":        severityCounts,
		"action_counts":          actionCounts,
		"blocking_policies":      0,
		"violations_by_category": make(map[string]int),
	}

	blockingPoliciesCount := 0
	violationsByCategory := make(map[string]int)

	for _, result := range results {
		if result.Passed {
			summary["passed_policies"] = summary["passed_policies"].(int) + 1
		} else {
			summary["failed_policies"] = summary["failed_policies"].(int) + 1

			// Check if this is a blocking policy and count actions
			hasBlockingAction := false
			for _, action := range result.Actions {
				switch action.Type {
				case ActionTypeBlock:
					hasBlockingAction = true
					actionCounts["block"]++
				case ActionTypeWarn:
					actionCounts["warn"]++
				case ActionTypeNotify:
					actionCounts["notify"]++
				case ActionTypeLog:
					actionCounts["log"]++
				}
			}
			if hasBlockingAction {
				blockingPoliciesCount++
			}
		}

		// Count violations by severity
		for _, violation := range result.Violations {
			summary["total_violations"] = summary["total_violations"].(int) + 1

			switch violation.Severity {
			case PolicySeverityCritical:
				severityCounts["critical"]++
			case PolicySeverityHigh:
				severityCounts["high"]++
			case PolicySeverityMedium:
				severityCounts["medium"]++
			case PolicySeverityLow:
				severityCounts["low"]++
			}
		}

		// Count by category
		if category, ok := result.Metadata["policy_category"].(PolicyCategory); ok {
			violationsByCategory[string(category)]++
		}
	}

	summary["blocking_policies"] = blockingPoliciesCount
	summary["violations_by_category"] = violationsByCategory

	return summary
}

// countViolations counts total violations across all results
func (pe *PolicyEngine) countViolations(results []PolicyEvaluationResult) int {
	count := 0
	for _, result := range results {
		count += len(result.Violations)
	}
	return count
}
