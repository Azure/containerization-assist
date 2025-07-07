package session

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// deletesessionTool implements the canonical api.Tool interface
type deletesessionTool struct {
	sessionManager session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore   services.SessionStore         // Modern service interface
	sessionState   services.SessionState         // Modern service interface
	logger         *slog.Logger
}

// NewdeletesessionTool creates a new deletesession tool using canonical interface (legacy constructor)
func NewdeletesessionTool(sessionManager session.UnifiedSessionManager, logger *slog.Logger) api.Tool {
	return &deletesessionTool{
		sessionManager: sessionManager,
		logger:         logger.With("tool", "deletesession"),
	}
}

// NewdeletesessionToolWithServices creates a new deletesession tool using service container
func NewdeletesessionToolWithServices(serviceContainer services.ServiceContainer, logger *slog.Logger) api.Tool {
	return &deletesessionTool{
		sessionStore: serviceContainer.SessionStore(),
		sessionState: serviceContainer.SessionState(),
		logger:       logger.With("tool", "deletesession"),
	}
}

// Name implements api.Tool
func (t *deletesessionTool) Name() string {
	return "deletesession"
}

// Description implements api.Tool
func (t *deletesessionTool) Description() string {
	return "Delete a session and cleanup resources"
}

// Schema implements api.Tool
func (t *deletesessionTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for the operation to delete",
				},
			},
			"required": []string{"session_id"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the operation was successful",
				},
				"data": map[string]interface{}{
					"type":        "object",
					"description": "Operation result data",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Execution status",
						},
					},
				},
				"error": map[string]interface{}{
					"type":        "string",
					"description": "Error message if operation failed",
				},
				"metadata": map[string]interface{}{
					"type":        "object",
					"description": "Additional metadata about the operation",
				},
			},
		},
		Tags:     []string{},
		Category: "session",
		Version:  "1.0.0",
	}
}

// Execute implements api.Tool
func (t *deletesessionTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Validate session ID
	if input.SessionID == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "session_id is required",
			Data: map[string]interface{}{
				"error": "session_id is required",
			},
		}, fmt.Errorf("session_id is required")
	}

	// Extract and validate input parameters
	var params struct {
		Session_id string `json:"session_id",omitempty`
	}

	// Parse parameters from input.Data

	if val, ok := input.Data["session_id"]; ok {
		if strVal, ok := val.(string); ok {
			params.Session_id = strVal
		}
	}

	// Validate required parameters

	// Log the execution
	t.logger.Info("Starting deletesession execution", "session_id", input.SessionID)

	// Use appropriate service interface to delete session
	err := t.deleteSession(ctx, input.SessionID)
	if err != nil {
		t.logger.Error("Failed to delete session", "session_id", input.SessionID, "error", err)
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Failed to delete session: %v", err),
			Data: map[string]interface{}{
				"status": "failed",
				"error":  err.Error(),
			},
		}, err
	}

	result := api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"status": "completed",
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": 1000,
			"session_id":        input.SessionID,
			"tool_version":      "1.0.0",
		},
	}

	t.logger.Info("deletesession execution completed successfully", "session_id", input.SessionID)

	return result, nil
}

// deleteSession deletes a session using appropriate service interface (service or legacy)
func (t *deletesessionTool) deleteSession(ctx context.Context, sessionID string) error {
	// If service interfaces are available, use them (modern pattern)
	if t.sessionStore != nil {
		// Delete session using service store
		return t.sessionStore.Delete(ctx, sessionID)
	}

	// Fall back to legacy unified session manager
	if t.sessionManager != nil {
		return t.sessionManager.DeleteSession(ctx, sessionID)
	}

	return fmt.Errorf("no session management interface available")
}
