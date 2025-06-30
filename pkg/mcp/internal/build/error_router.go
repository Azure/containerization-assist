package build

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ErrorRouter provides advanced error routing capabilities
type ErrorRouter struct {
	contextSharer *DefaultContextSharer
	routingRules  []FailureRoutingRule
	errorHandlers map[string]ErrorHandler
	eventBus      EventPublisher
	logger        zerolog.Logger
	mutex         sync.RWMutex
}

// ErrorHandler processes errors and determines routing decisions
type ErrorHandler func(ctx context.Context, err error, context *ErrorContext) (*RoutingDecision, error)

// ErrorContext provides context about the error
type ErrorContext struct {
	SessionID      string                 `json:"session_id"`
	SourceTool     string                 `json:"source_tool"`
	OperationType  string                 `json:"operation_type"`
	ErrorCode      string                 `json:"error_code"`
	ErrorType      string                 `json:"error_type"`
	ErrorMessage   string                 `json:"error_message"`
	RetryCount     int                    `json:"retry_count"`
	Timestamp      time.Time              `json:"timestamp"`
	SharedContext  map[string]interface{} `json:"shared_context"`
	ToolContext    map[string]interface{} `json:"tool_context"`
	ExecutionTrace []string               `json:"execution_trace"`
}

// RoutingDecision represents the decision made by the error router
type RoutingDecision struct {
	Action          string                 `json:"action"` // "route", "retry", "fail", "ignore"
	TargetTool      string                 `json:"target_tool,omitempty"`
	RetryPolicy     *RetryPolicy           `json:"retry_policy,omitempty"`
	TransformData   map[string]interface{} `json:"transform_data,omitempty"`
	ContextUpdates  map[string]interface{} `json:"context_updates,omitempty"`
	NotifyTools     []string               `json:"notify_tools,omitempty"`
	SaveCheckpoint  bool                   `json:"save_checkpoint"`
	DecisionReason  string                 `json:"decision_reason"`
	AlternativeFlow string                 `json:"alternative_flow,omitempty"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts   int           `json:"max_attempts"`
	BackoffType   string        `json:"backoff_type"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	RetryableErrs []string      `json:"retryable_errors"`
}

// EventPublisher publishes error routing events
type EventPublisher interface {
	PublishErrorEvent(eventType string, data map[string]interface{})
}

// NewErrorRouter creates a new error router
func NewErrorRouter(contextSharer *DefaultContextSharer, eventBus EventPublisher, logger zerolog.Logger) *ErrorRouter {
	router := &ErrorRouter{
		contextSharer: contextSharer,
		routingRules:  getDefaultRoutingRules(),
		errorHandlers: make(map[string]ErrorHandler),
		eventBus:      eventBus,
		logger:        logger.With().Str("component", "error_router").Logger(),
	}

	// Register default error handlers
	router.registerDefaultHandlers()

	return router
}

// RouteError routes an error based on context and rules
func (er *ErrorRouter) RouteError(ctx context.Context, errorCtx *ErrorContext) (*RoutingDecision, error) {
	er.logger.Info().
		Str("session_id", errorCtx.SessionID).
		Str("source_tool", errorCtx.SourceTool).
		Str("error_type", errorCtx.ErrorType).
		Str("error_code", errorCtx.ErrorCode).
		Msg("Routing error")

	// Load shared context
	if sharedData, err := er.contextSharer.GetSharedContext(ctx, errorCtx.SessionID, "execution_context"); err == nil {
		if contextMap, ok := sharedData.(map[string]interface{}); ok {
			errorCtx.SharedContext = contextMap
		}
	}

	// Find matching routing rules
	matchedRules := er.findMatchingRules(errorCtx)

	if len(matchedRules) == 0 {
		er.logger.Debug().
			Str("source_tool", errorCtx.SourceTool).
			Str("error_type", errorCtx.ErrorType).
			Msg("No matching routing rules found")

		// Use default handler
		return er.handleDefaultError(ctx, errorCtx)
	}

	// Sort rules by priority
	er.sortRulesByPriority(matchedRules)

	// Apply the highest priority rule
	rule := matchedRules[0]
	decision, err := er.applyRoutingRule(ctx, errorCtx, rule)
	if err != nil {
		return nil, fmt.Errorf("failed to apply routing rule: %w", err)
	}

	// Save routing decision to context
	er.saveRoutingDecision(ctx, errorCtx, decision)

	// Publish routing event
	if er.eventBus != nil {
		er.eventBus.PublishErrorEvent("error_routed", map[string]interface{}{
			"session_id":     errorCtx.SessionID,
			"source_tool":    errorCtx.SourceTool,
			"target_tool":    decision.TargetTool,
			"action":         decision.Action,
			"error_code":     errorCtx.ErrorCode,
			"routing_reason": decision.DecisionReason,
		})
	}

	return decision, nil
}

// findMatchingRules finds routing rules that match the error context
func (er *ErrorRouter) findMatchingRules(errorCtx *ErrorContext) []FailureRoutingRule {
	var matched []FailureRoutingRule

	for _, rule := range er.routingRules {
		if er.matchesRule(errorCtx, rule) {
			matched = append(matched, rule)
		}
	}

	return matched
}

// matchesRule checks if an error context matches a routing rule
func (er *ErrorRouter) matchesRule(errorCtx *ErrorContext, rule FailureRoutingRule) bool {
	// Check source tool
	if rule.FromTool != "" && rule.FromTool != errorCtx.SourceTool {
		return false
	}

	// Check error type
	if len(rule.ErrorTypes) > 0 {
		matched := false
		for _, errorType := range rule.ErrorTypes {
			if errorType == errorCtx.ErrorType || strings.Contains(errorCtx.ErrorMessage, errorType) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check error code
	if len(rule.ErrorCodes) > 0 {
		matched := false
		for _, errorCode := range rule.ErrorCodes {
			if errorCode == errorCtx.ErrorCode {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check conditions
	if rule.Conditions != nil {
		if !er.evaluateConditions(errorCtx, rule.Conditions) {
			return false
		}
	}

	return true
}

// evaluateConditions evaluates rule conditions against error context
func (er *ErrorRouter) evaluateConditions(errorCtx *ErrorContext, conditions map[string]interface{}) bool {
	// Check retry count condition
	if maxRetries, exists := conditions["retry_count"]; exists {
		if maxRetriesInt, ok := maxRetries.(int); ok {
			if errorCtx.RetryCount > maxRetriesInt {
				return false
			}
		}
	}

	// Check auto-fix condition
	if autoFix, exists := conditions["auto_fix"]; exists {
		if autoFixBool, ok := autoFix.(bool); ok && autoFixBool {
			// Check if auto-fix is available in shared context
			if errorCtx.SharedContext != nil {
				if _, hasAutoFix := errorCtx.SharedContext["auto_fix_available"]; !hasAutoFix {
					return false
				}
			}
		}
	}

	// Check severity condition
	if severity, exists := conditions["severity"]; exists {
		if severityStr, ok := severity.(string); ok {
			// Extract severity from error context
			if errorCtx.SharedContext != nil {
				if ctxSeverity, hasSeverity := errorCtx.SharedContext["severity"].(string); hasSeverity {
					if ctxSeverity != severityStr {
						return false
					}
				}
			}
		}
	}

	return true
}

// sortRulesByPriority sorts routing rules by priority (lower number = higher priority)
func (er *ErrorRouter) sortRulesByPriority(rules []FailureRoutingRule) {
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority > rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}
}

// applyRoutingRule applies a routing rule to generate a decision
func (er *ErrorRouter) applyRoutingRule(ctx context.Context, errorCtx *ErrorContext, rule FailureRoutingRule) (*RoutingDecision, error) {
	decision := &RoutingDecision{
		Action:         "route",
		TargetTool:     rule.ToTool,
		DecisionReason: rule.Description,
		SaveCheckpoint: true,
		ContextUpdates: make(map[string]interface{}),
	}

	// Add error context to updates
	decision.ContextUpdates["previous_error"] = map[string]interface{}{
		"tool":       errorCtx.SourceTool,
		"error_code": errorCtx.ErrorCode,
		"error_type": errorCtx.ErrorType,
		"message":    errorCtx.ErrorMessage,
		"timestamp":  errorCtx.Timestamp,
	}

	// Check if custom handler exists for this rule
	handlerKey := fmt.Sprintf("%s_%s", rule.FromTool, rule.ToTool)
	if handler, exists := er.errorHandlers[handlerKey]; exists {
		customDecision, err := handler(ctx, fmt.Errorf(errorCtx.ErrorMessage), errorCtx)
		if err == nil && customDecision != nil {
			// Merge custom decision with base decision
			if customDecision.Action != "" {
				decision.Action = customDecision.Action
			}
			if customDecision.TargetTool != "" {
				decision.TargetTool = customDecision.TargetTool
			}
			if customDecision.RetryPolicy != nil {
				decision.RetryPolicy = customDecision.RetryPolicy
			}
			for k, v := range customDecision.ContextUpdates {
				decision.ContextUpdates[k] = v
			}
		}
	}

	// Add routing metadata
	decision.ContextUpdates["routing_metadata"] = map[string]interface{}{
		"rule_priority":  rule.Priority,
		"rule_from_tool": rule.FromTool,
		"rule_to_tool":   rule.ToTool,
		"routed_at":      time.Now(),
	}

	return decision, nil
}

// handleDefaultError handles errors when no routing rules match
func (er *ErrorRouter) handleDefaultError(ctx context.Context, errorCtx *ErrorContext) (*RoutingDecision, error) {
	// Default behavior: retry with exponential backoff up to 3 times
	if errorCtx.RetryCount < 3 {
		return &RoutingDecision{
			Action: "retry",
			RetryPolicy: &RetryPolicy{
				MaxAttempts:  3,
				BackoffType:  "exponential",
				InitialDelay: time.Second,
				MaxDelay:     30 * time.Second,
			},
			DecisionReason: "Default retry policy for unmatched errors",
			SaveCheckpoint: true,
		}, nil
	}

	// After max retries, fail the operation
	return &RoutingDecision{
		Action:         "fail",
		DecisionReason: "Maximum retry attempts reached",
		SaveCheckpoint: true,
	}, nil
}

// saveRoutingDecision saves the routing decision to shared context
func (er *ErrorRouter) saveRoutingDecision(ctx context.Context, errorCtx *ErrorContext, decision *RoutingDecision) {
	routingData := map[string]interface{}{
		"decision":    decision,
		"error_ctx":   errorCtx,
		"timestamp":   time.Now(),
		"decision_id": fmt.Sprintf("routing_%d", time.Now().UnixNano()),
	}

	if err := er.contextSharer.ShareContext(ctx, errorCtx.SessionID, "error_routing", routingData); err != nil {
		er.logger.Warn().
			Err(err).
			Str("session_id", errorCtx.SessionID).
			Msg("Failed to save routing decision to context")
	}

	// Update context with decision updates
	if len(decision.ContextUpdates) > 0 {
		if err := er.contextSharer.ShareContext(ctx, errorCtx.SessionID, "routing_updates", decision.ContextUpdates); err != nil {
			er.logger.Warn().
				Err(err).
				Str("session_id", errorCtx.SessionID).
				Msg("Failed to save context updates")
		}
	}
}

// RegisterHandler registers a custom error handler
func (er *ErrorRouter) RegisterHandler(fromTool, toTool string, handler ErrorHandler) {
	er.mutex.Lock()
	defer er.mutex.Unlock()

	key := fmt.Sprintf("%s_%s", fromTool, toTool)
	er.errorHandlers[key] = handler

	er.logger.Info().
		Str("from_tool", fromTool).
		Str("to_tool", toTool).
		Msg("Registered custom error handler")
}

// RegisterRule adds a new routing rule
func (er *ErrorRouter) RegisterRule(rule FailureRoutingRule) {
	er.mutex.Lock()
	defer er.mutex.Unlock()

	er.routingRules = append(er.routingRules, rule)

	er.logger.Info().
		Str("from_tool", rule.FromTool).
		Str("to_tool", rule.ToTool).
		Int("priority", rule.Priority).
		Msg("Registered routing rule")
}

// registerDefaultHandlers registers default error handlers
func (er *ErrorRouter) registerDefaultHandlers() {
	// Build failure -> Analyze handler
	er.RegisterHandler("build_image", "analyze_repository", func(ctx context.Context, err error, errorCtx *ErrorContext) (*RoutingDecision, error) {
		// Extract dockerfile path from context
		dockerfilePath := ""
		if errorCtx.ToolContext != nil {
			if path, ok := errorCtx.ToolContext["dockerfile_path"].(string); ok {
				dockerfilePath = path
			}
		}

		return &RoutingDecision{
			Action:     "route",
			TargetTool: "analyze_repository",
			TransformData: map[string]interface{}{
				"analyze_mode":    "fix_dockerfile",
				"dockerfile_path": dockerfilePath,
				"error_details":   errorCtx.ErrorMessage,
			},
			ContextUpdates: map[string]interface{}{
				"fix_requested": true,
				"fix_type":      "dockerfile",
			},
			DecisionReason: "Routing to analyzer for Dockerfile fix",
		}, nil
	})

	// Security scan failure -> Build handler
	er.RegisterHandler("scan_security", "build_image", func(ctx context.Context, err error, errorCtx *ErrorContext) (*RoutingDecision, error) {
		// Check if it's a base image vulnerability
		if strings.Contains(errorCtx.ErrorMessage, "base image") {
			return &RoutingDecision{
				Action:     "route",
				TargetTool: "build_image",
				TransformData: map[string]interface{}{
					"update_base_image": true,
					"security_patches":  true,
				},
				ContextUpdates: map[string]interface{}{
					"security_fix": true,
				},
				DecisionReason: "Routing to build for base image update",
			}, nil
		}

		return nil, fmt.Errorf("not a base image vulnerability")
	})

	// Deploy failure -> Generate manifests handler
	er.RegisterHandler("deploy_kubernetes", "generate_manifests", func(ctx context.Context, err error, errorCtx *ErrorContext) (*RoutingDecision, error) {
		// Check if it's a resource validation error
		if strings.Contains(errorCtx.ErrorMessage, "validation") || strings.Contains(errorCtx.ErrorMessage, "resource") {
			return &RoutingDecision{
				Action:     "route",
				TargetTool: "generate_manifests",
				TransformData: map[string]interface{}{
					"fix_validation": true,
					"error_details":  errorCtx.ErrorMessage,
				},
				AlternativeFlow: "regenerate_and_deploy",
				DecisionReason:  "Routing to regenerate manifests for validation fix",
			}, nil
		}

		return nil, fmt.Errorf("not a validation error")
	})
}

// GetRoutingHistory retrieves routing history for a session
func (er *ErrorRouter) GetRoutingHistory(ctx context.Context, sessionID string) ([]map[string]interface{}, error) {
	data, err := er.contextSharer.GetSharedContext(ctx, sessionID, "error_routing")
	if err != nil {
		return nil, err
	}

	// Convert to history array
	if historyData, ok := data.([]map[string]interface{}); ok {
		return historyData, nil
	}

	// Single entry - wrap in array
	if routingData, ok := data.(map[string]interface{}); ok {
		return []map[string]interface{}{routingData}, nil
	}

	return nil, fmt.Errorf("invalid routing history format")
}

// AnalyzeErrorPatterns analyzes error patterns across sessions
func (er *ErrorRouter) AnalyzeErrorPatterns(ctx context.Context) *ErrorPatternAnalysis {
	analysis := &ErrorPatternAnalysis{
		Timestamp:     time.Now(),
		Patterns:      make(map[string]*ErrorPattern),
		TopErrorPaths: make([]ErrorPath, 0),
	}

	// This would analyze patterns across multiple sessions
	// For now, return empty analysis

	return analysis
}

// ErrorPatternAnalysis represents error pattern analysis results
type ErrorPatternAnalysis struct {
	Timestamp     time.Time                `json:"timestamp"`
	Patterns      map[string]*ErrorPattern `json:"patterns"`
	TopErrorPaths []ErrorPath              `json:"top_error_paths"`
	Suggestions   []RoutingSuggestion      `json:"suggestions"`
}

// ErrorPattern represents a recurring error pattern
type ErrorPattern struct {
	ErrorType   string   `json:"error_type"`
	SourceTools []string `json:"source_tools"`
	Frequency   int      `json:"frequency"`
	AvgRetries  float64  `json:"avg_retries"`
	SuccessRate float64  `json:"success_rate"`
}

// ErrorPath represents a common error routing path
type ErrorPath struct {
	Path      []string `json:"path"`
	Frequency int      `json:"frequency"`
	Success   bool     `json:"success"`
}

// RoutingSuggestion represents a suggested routing rule improvement
type RoutingSuggestion struct {
	CurrentRule         FailureRoutingRule `json:"current_rule"`
	SuggestedRule       FailureRoutingRule `json:"suggested_rule"`
	Reason              string             `json:"reason"`
	ExpectedImprovement float64            `json:"expected_improvement"`
}
