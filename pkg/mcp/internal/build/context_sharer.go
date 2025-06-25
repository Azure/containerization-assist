package build

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// SharedContext represents context shared between tools
type SharedContext struct {
	SessionID     string                 `json:"session_id"`
	ContextType   string                 `json:"context_type"`
	Data          interface{}            `json:"data"`
	CreatedAt     time.Time              `json:"created_at"`
	CreatedByTool string                 `json:"created_by_tool"`
	ExpiresAt     time.Time              `json:"expires_at"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// FailureRoutingRule defines how to route failures between tools
type FailureRoutingRule struct {
	FromTool    string                 `json:"from_tool"`
	ErrorTypes  []string               `json:"error_types"`
	ErrorCodes  []string               `json:"error_codes"`
	ToTool      string                 `json:"to_tool"`
	Priority    int                    `json:"priority"`
	Description string                 `json:"description"`
	Conditions  map[string]interface{} `json:"conditions"`
}

// DefaultContextSharer implements cross-tool context sharing
type DefaultContextSharer struct {
	contextStore map[string]map[string]*SharedContext // sessionID -> contextType -> context
	routingRules []FailureRoutingRule
	mutex        sync.RWMutex
	logger       zerolog.Logger
	defaultTTL   time.Duration
}

// NewDefaultContextSharer creates a new context sharer
func NewDefaultContextSharer(logger zerolog.Logger) *DefaultContextSharer {
	sharer := &DefaultContextSharer{
		contextStore: make(map[string]map[string]*SharedContext),
		routingRules: getDefaultRoutingRules(),
		logger:       logger.With().Str("component", "context_sharer").Logger(),
		defaultTTL:   time.Hour, // Default 1-hour TTL for shared context
	}

	// Start cleanup goroutine
	go sharer.cleanupExpiredContext()

	return sharer
}

// ShareContext saves context for other tools to use
func (c *DefaultContextSharer) ShareContext(ctx context.Context, sessionID string, contextType string, data interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.contextStore[sessionID] == nil {
		c.contextStore[sessionID] = make(map[string]*SharedContext)
	}

	sharedCtx := &SharedContext{
		SessionID:     sessionID,
		ContextType:   contextType,
		Data:          data,
		CreatedAt:     time.Now(),
		CreatedByTool: getToolFromContext(ctx),
		ExpiresAt:     time.Now().Add(c.defaultTTL),
		Metadata:      make(map[string]interface{}),
	}

	c.contextStore[sessionID][contextType] = sharedCtx

	c.logger.Debug().
		Str("session_id", sessionID).
		Str("context_type", contextType).
		Str("created_by", sharedCtx.CreatedByTool).
		Msg("Shared context saved")

	return nil
}

// GetSharedContext retrieves shared context
func (c *DefaultContextSharer) GetSharedContext(ctx context.Context, sessionID string, contextType string) (interface{}, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	sessionStore := c.contextStore[sessionID]
	if sessionStore == nil {
		return nil, fmt.Errorf("no shared context found for session %s", sessionID)
	}

	sharedCtx := sessionStore[contextType]
	if sharedCtx == nil {
		return nil, fmt.Errorf("no shared context of type %s found for session %s", contextType, sessionID)
	}

	// Check if context has expired
	if time.Now().After(sharedCtx.ExpiresAt) {
		delete(sessionStore, contextType)
		return nil, fmt.Errorf("shared context of type %s has expired for session %s", contextType, sessionID)
	}

	c.logger.Debug().
		Str("session_id", sessionID).
		Str("context_type", contextType).
		Str("created_by", sharedCtx.CreatedByTool).
		Msg("Retrieved shared context")

	return sharedCtx.Data, nil
}

// GetFailureRouting determines which tool should handle a specific failure
func (c *DefaultContextSharer) GetFailureRouting(ctx context.Context, sessionID string, failure *types.RichError) (string, error) {
	currentTool := getToolFromContext(ctx)

	c.logger.Debug().
		Str("session_id", sessionID).
		Str("current_tool", currentTool).
		Str("error_code", failure.Code).
		Str("error_type", failure.Type).
		Msg("Determining failure routing")

	// Find matching routing rules
	var bestMatch *FailureRoutingRule
	bestPriority := 999

	for _, rule := range c.routingRules {
		if rule.FromTool != currentTool {
			continue
		}

		// Check error type match
		if len(rule.ErrorTypes) > 0 && !contains(rule.ErrorTypes, failure.Type) {
			continue
		}

		// Check error code match
		if len(rule.ErrorCodes) > 0 && !contains(rule.ErrorCodes, failure.Code) {
			continue
		}

		// Check additional conditions
		if !c.matchesConditions(ctx, sessionID, failure, rule.Conditions) {
			continue
		}

		// Select rule with highest priority (lowest number)
		if rule.Priority < bestPriority {
			bestPriority = rule.Priority
			bestMatch = &rule
		}
	}

	if bestMatch == nil {
		return "", fmt.Errorf("no routing rule found for error type %s code %s from tool %s",
			failure.Type, failure.Code, currentTool)
	}

	c.logger.Info().
		Str("session_id", sessionID).
		Str("from_tool", currentTool).
		Str("to_tool", bestMatch.ToTool).
		Str("rule_description", bestMatch.Description).
		Int("priority", bestMatch.Priority).
		Msg("Found failure routing")

	return bestMatch.ToTool, nil
}

// matchesConditions checks if additional routing conditions are met
func (c *DefaultContextSharer) matchesConditions(ctx context.Context, sessionID string, failure *types.RichError, conditions map[string]interface{}) bool {
	if len(conditions) == 0 {
		return true
	}

	// Check severity condition
	if requiredSeverity, ok := conditions["min_severity"]; ok {
		if !c.severityMeetsThreshold(failure.Severity, requiredSeverity.(string)) {
			return false
		}
	}

	// Check if specific shared context is available
	if requiredContext, ok := conditions["requires_context"]; ok {
		_, err := c.GetSharedContext(ctx, sessionID, requiredContext.(string))
		if err != nil {
			return false
		}
	}

	return true
}

// severityMeetsThreshold checks if error severity meets minimum threshold
func (c *DefaultContextSharer) severityMeetsThreshold(severity, threshold string) bool {
	severityLevels := map[string]int{
		"Critical": 4,
		"High":     3,
		"Medium":   2,
		"Low":      1,
	}

	currentLevel := severityLevels[severity]
	thresholdLevel := severityLevels[threshold]

	return currentLevel >= thresholdLevel
}

// cleanupExpiredContext periodically removes expired context
func (c *DefaultContextSharer) cleanupExpiredContext() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()
		now := time.Now()

		for sessionID, sessionStore := range c.contextStore {
			for contextType, sharedCtx := range sessionStore {
				if now.After(sharedCtx.ExpiresAt) {
					delete(sessionStore, contextType)
					c.logger.Debug().
						Str("session_id", sessionID).
						Str("context_type", contextType).
						Msg("Cleaned up expired shared context")
				}
			}

			// Remove empty session stores
			if len(sessionStore) == 0 {
				delete(c.contextStore, sessionID)
			}
		}

		c.mutex.Unlock()
	}
}

// getDefaultRoutingRules returns the default failure routing rules
func getDefaultRoutingRules() []FailureRoutingRule {
	return []FailureRoutingRule{
		{
			FromTool:    "atomic_build_image",
			ErrorTypes:  []string{"dockerfile_error", "dependency_error"},
			ToTool:      "generate_dockerfile",
			Priority:    1,
			Description: "Route Dockerfile build failures to Dockerfile generation",
		},
		{
			FromTool:    "atomic_deploy_kubernetes",
			ErrorTypes:  []string{"manifest_error", "validation_error"},
			ToTool:      "generate_manifests_atomic",
			Priority:    1,
			Description: "Route manifest deployment failures to manifest generation",
		},
		{
			FromTool:    "atomic_deploy_kubernetes",
			ErrorTypes:  []string{"image_pull_error"},
			ToTool:      "atomic_build_image",
			Priority:    2,
			Description: "Route image pull failures back to image building",
		},
		{
			FromTool:    "atomic_push_image",
			ErrorTypes:  []string{"registry_error", "authentication_error"},
			ErrorCodes:  []string{"REGISTRY_AUTH_FAILED", "REGISTRY_UNREACHABLE"},
			ToTool:      "atomic_build_image",
			Priority:    2,
			Description: "Route registry push failures back to build for retry",
		},
		{
			FromTool:    "scan_image_security_atomic",
			ErrorTypes:  []string{"vulnerability_error"},
			ToTool:      "atomic_build_image",
			Priority:    3,
			Description: "Route critical security failures back to rebuilding",
			Conditions:  map[string]interface{}{"min_severity": "High"},
		},
	}
}

// getToolFromContext extracts tool name from context
func getToolFromContext(ctx context.Context) string {
	if tool := ctx.Value("tool_name"); tool != nil {
		return tool.(string)
	}
	return "unknown"
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
