package orchestration

import (
	"fmt"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	"github.com/rs/zerolog"
)

// ErrorRouter handles routing of errors to appropriate actions
type ErrorRouter struct {
	logger       zerolog.Logger
	routingRules map[string][]ErrorRoutingRule
}

// NewErrorRouter creates a new error router
func NewErrorRouter(logger zerolog.Logger) *ErrorRouter {
	return &ErrorRouter{
		logger:       logger.With().Str("component", "error_router").Logger(),
		routingRules: make(map[string][]ErrorRoutingRule),
	}
}

// AddRoutingRule adds a custom routing rule
func (er *ErrorRouter) AddRoutingRule(stageName string, rule ErrorRoutingRule) {
	if er.routingRules[stageName] == nil {
		er.routingRules[stageName] = []ErrorRoutingRule{}
	}
	er.routingRules[stageName] = append(er.routingRules[stageName], rule)

	er.logger.Info().
		Str("stage_name", stageName).
		Str("rule_id", rule.ID).
		Str("rule_name", rule.Name).
		Msg("Added custom routing rule")
}

// FindMatchingRule finds the best matching routing rule for an error
func (er *ErrorRouter) FindMatchingRule(workflowError *workflow.WorkflowError) *ErrorRoutingRule {
	rules := er.getApplicableRules(workflowError)
	if len(rules) == 0 {
		er.logger.Debug().
			Str("stage_name", workflowError.StageName).
			Msg("No routing rules found")
		return nil
	}

	return er.findBestMatchingRule(workflowError, rules)
}

// MatchesConditions checks if all conditions match for a routing rule
func (er *ErrorRouter) MatchesConditions(rule ErrorRoutingRule, workflowError *workflow.WorkflowError) bool {
	return er.ruleMatches(rule, workflowError)
}

// Internal methods

func (er *ErrorRouter) getApplicableRules(workflowError *workflow.WorkflowError) []ErrorRoutingRule {
	var applicableRules []ErrorRoutingRule

	// Get stage-specific rules
	if rules, exists := er.routingRules[workflowError.StageName]; exists {
		applicableRules = append(applicableRules, rules...)
	}

	// Get global rules (*)
	if rules, exists := er.routingRules["*"]; exists {
		applicableRules = append(applicableRules, rules...)
	}

	// Filter enabled rules
	var enabledRules []ErrorRoutingRule
	for _, rule := range applicableRules {
		if rule.Enabled {
			enabledRules = append(enabledRules, rule)
		}
	}

	return enabledRules
}

func (er *ErrorRouter) findBestMatchingRule(
	workflowError *workflow.WorkflowError,
	rules []ErrorRoutingRule,
) *ErrorRoutingRule {
	var matchingRules []ErrorRoutingRule

	// Find rules that match all conditions
	for _, rule := range rules {
		if er.ruleMatches(rule, workflowError) {
			matchingRules = append(matchingRules, rule)
		}
	}

	if len(matchingRules) == 0 {
		return nil
	}

	// Sort by priority (highest first)
	var bestRule *ErrorRoutingRule
	highestPriority := -1

	for i, rule := range matchingRules {
		if rule.Priority > highestPriority {
			highestPriority = rule.Priority
			bestRule = &matchingRules[i]
		}
	}

	return bestRule
}

func (er *ErrorRouter) ruleMatches(rule ErrorRoutingRule, workflowError *workflow.WorkflowError) bool {
	if len(rule.Conditions) == 0 {
		return true // Rule with no conditions matches everything
	}

	// All conditions must match
	for _, condition := range rule.Conditions {
		if !er.conditionMatches(condition, workflowError) {
			return false
		}
	}

	return true
}

func (er *ErrorRouter) conditionMatches(condition RoutingCondition, workflowError *workflow.WorkflowError) bool {
	var fieldValue string

	// Get field value from error
	switch condition.Field {
	case "error_type":
		fieldValue = workflowError.ErrorType
	case "stage_name":
		fieldValue = workflowError.StageName
	case "tool_name":
		fieldValue = workflowError.ToolName
	case "message":
		fieldValue = workflowError.Message
	case "severity":
		fieldValue = workflowError.Severity
	default:
		return false
	}

	// Apply case sensitivity
	expectedValue := fmt.Sprintf("%v", condition.Value)
	if !condition.CaseSensitive {
		fieldValue = strings.ToLower(fieldValue)
		expectedValue = strings.ToLower(expectedValue)
	}

	// Apply operator
	switch condition.Operator {
	case "equals":
		return fieldValue == expectedValue
	case "not_equals":
		return fieldValue != expectedValue
	case "contains":
		return strings.Contains(fieldValue, expectedValue)
	case "matches":
		// Simple glob-style matching for now
		return er.globMatch(expectedValue, fieldValue)
	default:
		return false
	}
}

func (er *ErrorRouter) globMatch(pattern, text string) bool {
	// Simple glob matching - just handle * for now
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(text, parts[0]) && strings.HasSuffix(text, parts[1])
		}
	}
	return pattern == text
}
