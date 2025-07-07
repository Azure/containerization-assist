package build

import (
	"context"
	"fmt"
	"sync"
	"time"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	opstypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
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
	logger       *slog.Logger
	defaultTTL   time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	errorRouter  *ErrorRouter // Advanced error routing
}

// NewDefaultContextSharer creates a new context sharer
func NewDefaultContextSharer(logger *slog.Logger) *DefaultContextSharer {
	ctx, cancel := context.WithCancel(context.Background())
	sharer := &DefaultContextSharer{
		contextStore: make(map[string]map[string]*SharedContext),
		routingRules: getDefaultRoutingRules(),
		logger:       logger.With("component", "context_sharer"),
		defaultTTL:   time.Hour, // Default 1-hour TTL for shared context
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start cleanup goroutine
	go sharer.cleanupExpiredContext(ctx)

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
	c.logger.Debug("Shared context saved",
		"session_id", sessionID,
		"context_type", contextType,
		"created_by", sharedCtx.CreatedByTool)
	return nil
}

// GetSharedContext retrieves shared context
func (c *DefaultContextSharer) GetSharedContext(ctx context.Context, sessionID string, contextType string) (interface{}, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	sessionStore := c.contextStore[sessionID]
	if sessionStore == nil {
		return nil, errors.NewError().Messagef("no shared context found for session %s", sessionID).Build()
	}
	sharedCtx, exists := sessionStore[contextType]
	if !exists {
		return nil, errors.NewError().Messagef("no shared context of type %s found for session %s", contextType, sessionID).WithLocation(

		// Check if context has expired
		).Build()
	}

	if time.Now().After(sharedCtx.ExpiresAt) {
		delete(sessionStore, contextType)
		return nil, errors.NewError().Messagef("shared context of type %s has expired for session %s", contextType, sessionID).Build()
	}
	return sharedCtx.Data, nil
}

// ClearContext clears all shared context for a session
func (c *DefaultContextSharer) ClearContext(ctx context.Context, sessionID string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if _, exists := c.contextStore[sessionID]; exists {
		delete(c.contextStore, sessionID)
		c.logger.Debug("Cleared all shared context for session",
			"session_id", sessionID)
	}
	return nil
}

// Close gracefully shuts down the context sharer
func (c *DefaultContextSharer) Close() error {
	c.cancel()
	return nil
}

// SetErrorRouter sets the error router for advanced routing
func (c *DefaultContextSharer) SetErrorRouter(router *ErrorRouter) {
	c.errorRouter = router
}

// RouteError routes an error using the error router
func (c *DefaultContextSharer) RouteError(ctx context.Context, config opstypes.ErrorRouteConfig) (*RoutingDecision, error) {
	if c.errorRouter == nil {
		return nil, errors.NewError().Messagef("error router not configured").Build()
	}

	errorCtx := &ConsolidatedErrorContext{
		SessionID:      config.SessionID,
		SourceTool:     config.SourceTool,
		ErrorType:      config.ErrorType,
		ErrorCode:      config.ErrorCode,
		ErrorMessage:   config.ErrorMessage,
		Timestamp:      time.Now(),
		ExecutionTrace: []string{config.SourceTool},
	}

	// Get tool context from shared context
	if toolData, err := c.GetSharedContext(ctx, config.SessionID, fmt.Sprintf("tool_%s", config.SourceTool)); err == nil {
		if toolContext, ok := toolData.(map[string]interface{}); ok {
			errorCtx.ToolContext = toolContext
		}
	}

	return c.errorRouter.RouteError(ctx, errorCtx)
}

// getToolFromContext extracts tool name from context
func getToolFromContext(ctx context.Context) string {
	// Check for tool name in context values
	if toolName := ctx.Value("tool_name"); toolName != nil {
		if name, ok := toolName.(string); ok {
			return name
		}
	}

	// Check for operation name in context values
	if opName := ctx.Value("operation"); opName != nil {
		if name, ok := opName.(string); ok {
			return name
		}
	}

	// Check for MCP tool identifier
	if mcpTool := ctx.Value("mcp_tool"); mcpTool != nil {
		if name, ok := mcpTool.(string); ok {
			return name
		}
	}

	return "unknown"
}

// getDefaultRoutingRules returns default failure routing rules
func getDefaultRoutingRules() []FailureRoutingRule {
	return []FailureRoutingRule{
		{
			FromTool:    "build",
			ErrorTypes:  []string{"dockerfile_error", "dependency_error"},
			ErrorCodes:  []string{"BUILD_FAILED", "DEPENDENCY_RESOLUTION_FAILED"},
			ToTool:      "analyze",
			Priority:    1,
			Description: "Route build failures to analyzer for deeper inspection",
			Conditions:  map[string]interface{}{"retry_count": 0},
		},
		{
			FromTool:    "deploy",
			ErrorTypes:  []string{"resource_error", "validation_error"},
			ErrorCodes:  []string{"DEPLOY_FAILED", "VALIDATION_FAILED"},
			ToTool:      "generate",
			Priority:    1,
			Description: "Route deployment failures back to manifest generation",
			Conditions:  map[string]interface{}{"can_regenerate": true},
		},
		{
			FromTool:    "scan",
			ErrorTypes:  []string{"security_issue"},
			ErrorCodes:  []string{"VULNERABILITY_FOUND"},
			ToTool:      "build",
			Priority:    2,
			Description: "Route security issues to build for base image updates",
			Conditions:  map[string]interface{}{"severity": "high"},
		},
	}
}

// cleanupExpiredContext periodically removes expired shared context
func (c *DefaultContextSharer) cleanupExpiredContext(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 15) // Cleanup every 15 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mutex.Lock()
			now := time.Now()
			expiredSessions := []string{}

			for sessionID, sessionStore := range c.contextStore {
				expiredTypes := []string{}
				for contextType, sharedCtx := range sessionStore {
					if now.After(sharedCtx.ExpiresAt) {
						expiredTypes = append(expiredTypes, contextType)
					}
				}

				// Remove expired context types
				for _, contextType := range expiredTypes {
					delete(sessionStore, contextType)
					c.logger.Debug("Cleaned up expired shared context",
						"session_id", sessionID,
						"context_type", contextType)
				}

				// If session has no remaining context, mark for removal
				if len(sessionStore) == 0 {
					expiredSessions = append(expiredSessions, sessionID)
				}
			}

			// Remove empty sessions
			for _, sessionID := range expiredSessions {
				delete(c.contextStore, sessionID)
				c.logger.Debug("Cleaned up empty session context store",
					"session_id", sessionID)
			}

			c.mutex.Unlock()
		}
	}
}
