package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// ConversationHandler handles conversation messages and orchestrates tool execution
type ConversationHandler struct {
	toolRegistry  services.ToolRegistry
	sessionStore  services.SessionStore
	sessionState  services.SessionState
	stateManager  services.StateManager
	autoFixHelper *AutoFixHelper
	logger        *slog.Logger
}

// NewConversationHandler creates a new conversation handler
func NewConversationHandler(
	toolRegistry services.ToolRegistry,
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	stateManager services.StateManager,
	logger *slog.Logger,
) *ConversationHandler {
	return &ConversationHandler{
		toolRegistry:  toolRegistry,
		sessionStore:  sessionStore,
		sessionState:  sessionState,
		stateManager:  stateManager,
		autoFixHelper: NewAutoFixHelper(logger),
		logger:        logger,
	}
}

// HandleMessage processes a conversation message and returns a response
func (h *ConversationHandler) HandleMessage(ctx context.Context, msg *ConversationMessage) (*ConversationResponse, error) {
	h.logger.Debug("Handling conversation message",
		slog.String("session_id", msg.SessionID),
		slog.String("type", msg.Type))

	// Get or create session
	session, err := h.getOrCreateSession(ctx, msg.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Process message based on type
	switch msg.Type {
	case "tool_request":
		return h.handleToolRequest(ctx, session, msg)
	case "workflow_request":
		return h.handleWorkflowRequest(ctx, session, msg)
	case "status_request":
		return h.handleStatusRequest(ctx, session, msg)
	default:
		return nil, fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleToolRequest processes a tool execution request
func (h *ConversationHandler) handleToolRequest(ctx context.Context, session *api.Session, msg *ConversationMessage) (*ConversationResponse, error) {
	toolName := msg.ToolName
	if toolName == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	// Get tool from registry
	tool, err := h.toolRegistry.GetTool(toolName)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}

	// Create tool input
	toolInput := api.ToolInput{
		SessionID: msg.SessionID,
		Data:      h.convertArgumentsToMap(msg.Arguments),
		Context:   msg.Context,
	}

	// Execute tool
	result, err := tool.Execute(ctx, toolInput)
	if err != nil {
		// Try auto-fix if enabled
		if msg.AutoFix && h.autoFixHelper != nil {
			fixedResult, fixErr := h.autoFixHelper.AttemptFix(ctx, tool, msg.Arguments, err)
			if fixErr == nil {
				result = h.convertToToolOutput(fixedResult)
				err = nil
			}
		}

		if err != nil {
			return &ConversationResponse{
				Success:   false,
				Error:     err.Error(),
				Data:      map[string]interface{}{"original_error": err.Error()},
				Timestamp: time.Now(),
			}, nil
		}
	}

	// Update session state
	h.updateSessionState(ctx, session, toolName, result)

	return &ConversationResponse{
		Success:   true,
		Result:    result.Data,
		Data:      map[string]interface{}{"tool": toolName},
		Timestamp: time.Now(),
	}, nil
}

// handleWorkflowRequest processes a workflow execution request
func (h *ConversationHandler) handleWorkflowRequest(ctx context.Context, session *api.Session, msg *ConversationMessage) (*ConversationResponse, error) {
	// Extract workflow request
	workflowReq, ok := msg.Arguments.(*WorkflowRequest)
	if !ok {
		return nil, fmt.Errorf("invalid workflow request format")
	}

	h.logger.Info("Executing workflow",
		slog.String("workflow_id", workflowReq.WorkflowID),
		slog.String("session_id", msg.SessionID))

	// TODO: Implement workflow execution using WorkflowExecutor service
	// This is a placeholder for now
	return &ConversationResponse{
		Success: true,
		Result: map[string]interface{}{
			"workflow_id": workflowReq.WorkflowID,
			"status":      "completed",
			"message":     "Workflow execution not yet implemented",
		},
		Data:      map[string]interface{}{"workflow_id": workflowReq.WorkflowID},
		Timestamp: time.Now(),
	}, nil
}

// handleStatusRequest processes a status check request
func (h *ConversationHandler) handleStatusRequest(ctx context.Context, session *api.Session, msg *ConversationMessage) (*ConversationResponse, error) {
	// Extract status request
	statusReq, ok := msg.Arguments.(*StatusRequest)
	if !ok {
		return nil, fmt.Errorf("invalid status request format")
	}

	h.logger.Debug("Processing status request",
		slog.String("type", statusReq.Type),
		slog.String("session_id", msg.SessionID))

	var statusResp StatusResponse

	switch statusReq.Type {
	case "session":
		// Get session status
		sessionState, err := h.sessionState.GetState(ctx, msg.SessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get session state: %w", err)
		}

		statusResp = StatusResponse{
			Type: "session",
			Entries: []StatusEntry{
				{
					ID:     msg.SessionID,
					Name:   "Current Session",
					Status: "active",
					Details: map[string]interface{}{
						"state": sessionState,
					},
				},
			},
			Summary: StatusSummary{
				Total:     1,
				Active:    1,
				Completed: 0,
				Failed:    0,
			},
		}

	case "tool":
		// Get tool metrics
		metrics := h.toolRegistry.GetMetrics()
		statusResp = StatusResponse{
			Type: "tool",
			Summary: StatusSummary{
				Total:     metrics.TotalTools,
				Active:    metrics.ActiveTools,
				Completed: int(metrics.TotalExecutions - metrics.FailedExecutions),
				Failed:    int(metrics.FailedExecutions),
			},
		}

	default:
		return nil, fmt.Errorf("unknown status type: %s", statusReq.Type)
	}

	return &ConversationResponse{
		Success:   true,
		Result:    statusResp,
		Timestamp: time.Now(),
	}, nil
}

// getOrCreateSession retrieves an existing session or creates a new one
func (h *ConversationHandler) getOrCreateSession(ctx context.Context, sessionID string) (*api.Session, error) {
	// Try to get existing session
	session, err := h.sessionStore.Get(ctx, sessionID)
	if err == nil && session != nil {
		return session, nil
	}

	// Create new session
	session = &api.Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		State:     make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
	}

	if err := h.sessionStore.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	h.logger.Info("Created new session", slog.String("session_id", sessionID))
	return session, nil
}

// updateSessionState updates the session state after tool execution
func (h *ConversationHandler) updateSessionState(ctx context.Context, session *api.Session, toolName string, result api.ToolOutput) {
	state := map[string]interface{}{
		"last_tool":      toolName,
		"last_execution": time.Now(),
		"last_result":    result.Data,
		"last_success":   result.Success,
	}

	if err := h.stateManager.UpdateState(ctx, session.ID, toolName, state); err != nil {
		h.logger.Warn("Failed to update session state",
			slog.String("session_id", session.ID),
			slog.String("error", err.Error()))
	}

	// Also update session state service
	if err := h.sessionState.SaveState(ctx, session.ID, state); err != nil {
		h.logger.Warn("Failed to save session state",
			slog.String("session_id", session.ID),
			slog.String("error", err.Error()))
	}
}

// convertArgumentsToMap converts generic arguments to a map
func (h *ConversationHandler) convertArgumentsToMap(args interface{}) map[string]interface{} {
	if args == nil {
		return make(map[string]interface{})
	}

	// If already a map, return it
	if m, ok := args.(map[string]interface{}); ok {
		return m
	}

	// Try to convert using reflection or JSON marshaling
	// For now, return empty map
	return make(map[string]interface{})
}

// convertToToolOutput converts auto-fix result to ToolOutput
func (h *ConversationHandler) convertToToolOutput(result interface{}) api.ToolOutput {
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"result":   result,
			"auto_fix": true,
		},
	}
}
