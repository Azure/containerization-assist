package conversation

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// Helper methods for session management and context

// extractSessionID extracts session ID from arguments
func (h *AutoFixHelper) extractSessionID(args interface{}) string {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if sessionID, ok := argsMap["session_id"].(string); ok {
			return sessionID
		}
	}
	return "default"
}

// buildSessionContext builds session context for auto-fix decisions
func (h *AutoFixHelper) buildSessionContext(ctx context.Context, sessionID string) (*SessionContext, error) {
	return &SessionContext{
		SessionID: sessionID,
		Metadata:  make(map[string]interface{}),
	}, nil
}

// attemptContextAwareFix attempts to fix using context-aware strategies
func (h *AutoFixHelper) attemptContextAwareFix(ctx context.Context, tool api.Tool, args interface{}, err error, sessionCtx *SessionContext) (interface{}, error) {
	// For now, delegate to basic fix - can be enhanced with context-aware logic
	return h.attemptBasicFix(ctx, tool, args, err)
}

// attemptBasicFix attempts to fix using basic strategies
func (h *AutoFixHelper) attemptBasicFix(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
	errorMsg := err.Error()

	// Try each registered fix strategy
	for name, strategy := range h.fixes {
		h.logger.Debug("Trying fix strategy", slog.String("strategy", name))

		if result, fixErr := strategy(ctx, tool, args, err); fixErr == nil {
			return result, nil
		} else if fixErr.Error() != errorMsg {
			// Strategy returned a different error (enhanced error message)
			return nil, fixErr
		}
	}

	return nil, err
}

// recordFixAttempt records a fix attempt for analytics
func (h *AutoFixHelper) recordFixAttempt(sessionID, toolName, errorMsg, strategy string, successful bool) {
	attempt := FixAttempt{
		SessionID:  sessionID,
		ToolName:   toolName,
		Error:      errorMsg,
		Strategy:   strategy,
		Successful: successful,
		Timestamp:  time.Now(),
	}

	if h.fixHistory[sessionID] == nil {
		h.fixHistory[sessionID] = make([]FixAttempt, 0)
	}

	h.fixHistory[sessionID] = append(h.fixHistory[sessionID], attempt)

	// Keep only last 10 attempts per session
	if len(h.fixHistory[sessionID]) > 10 {
		h.fixHistory[sessionID] = h.fixHistory[sessionID][len(h.fixHistory[sessionID])-10:]
	}
}
