// Package commands provides command handlers for Container Kit MCP.
package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
)

// ContainerizeCommandHandler handles containerization commands
type ContainerizeCommandHandler struct {
	orchestrator   *workflow.EventOrchestrator
	sessionManager session.SessionManager
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewContainerizeCommandHandler creates a new containerization command handler
func NewContainerizeCommandHandler(
	orchestrator *workflow.EventOrchestrator,
	sessionManager session.SessionManager,
	eventPublisher *events.Publisher,
	logger *slog.Logger,
) *ContainerizeCommandHandler {
	return &ContainerizeCommandHandler{
		orchestrator:   orchestrator,
		sessionManager: sessionManager,
		eventPublisher: eventPublisher,
		logger:         logger.With("component", "containerize_command_handler"),
	}
}

// Handle executes a containerization command
func (h *ContainerizeCommandHandler) Handle(ctx context.Context, cmd Command) error {
	containerizeCmd, ok := cmd.(ContainerizeCommand)
	if !ok {
		return fmt.Errorf("invalid command type: expected ContainerizeCommand")
	}

	if err := containerizeCmd.Validate(); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	h.logger.Info("Handling containerize command",
		"command_id", containerizeCmd.CommandID(),
		"session_id", containerizeCmd.SessionID,
		"repo_url", containerizeCmd.Args.RepoURL)

	// Create a mock MCP request for the orchestrator
	// In a real implementation, this would come from the MCP protocol
	req := &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "containerize_and_deploy",
			Arguments: map[string]interface{}{
				"repo_url":  containerizeCmd.Args.RepoURL,
				"branch":    containerizeCmd.Args.Branch,
				"scan":      containerizeCmd.Args.Scan,
				"deploy":    containerizeCmd.Args.Deploy,
				"test_mode": containerizeCmd.Args.TestMode,
			},
		},
	}

	// Execute the workflow using the event orchestrator
	result, err := h.orchestrator.Execute(ctx, req, &containerizeCmd.Args)
	if err != nil {
		h.logger.Error("Workflow execution failed",
			"command_id", containerizeCmd.CommandID(),
			"session_id", containerizeCmd.SessionID,
			"error", err)
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	// Update session with result
	updateErr := h.sessionManager.UpdateSession(ctx, containerizeCmd.SessionID, func(state *session.SessionState) error {
		// Update session metadata with workflow result
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		state.Metadata["last_workflow_result"] = result
		state.Metadata["last_command_id"] = containerizeCmd.CommandID()

		if result.Success {
			state.Status = "completed"
		} else {
			state.Status = "failed"
		}

		return nil
	})

	if updateErr != nil {
		h.logger.Error("Failed to update session",
			"command_id", containerizeCmd.CommandID(),
			"session_id", containerizeCmd.SessionID,
			"error", updateErr)
		// Don't fail the command for session update errors
	}

	h.logger.Info("Containerize command completed",
		"command_id", containerizeCmd.CommandID(),
		"session_id", containerizeCmd.SessionID,
		"success", result.Success)

	return nil
}

// CancelWorkflowCommandHandler handles workflow cancellation commands
type CancelWorkflowCommandHandler struct {
	sessionManager session.SessionManager
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewCancelWorkflowCommandHandler creates a new cancellation command handler
func NewCancelWorkflowCommandHandler(
	sessionManager session.SessionManager,
	eventPublisher *events.Publisher,
	logger *slog.Logger,
) *CancelWorkflowCommandHandler {
	return &CancelWorkflowCommandHandler{
		sessionManager: sessionManager,
		eventPublisher: eventPublisher,
		logger:         logger.With("component", "cancel_workflow_command_handler"),
	}
}

// Handle executes a workflow cancellation command
func (h *CancelWorkflowCommandHandler) Handle(ctx context.Context, cmd Command) error {
	cancelCmd, ok := cmd.(CancelWorkflowCommand)
	if !ok {
		return fmt.Errorf("invalid command type: expected CancelWorkflowCommand")
	}

	if err := cancelCmd.Validate(); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	h.logger.Info("Handling cancel workflow command",
		"command_id", cancelCmd.CommandID(),
		"session_id", cancelCmd.SessionID,
		"workflow_id", cancelCmd.WorkflowID,
		"reason", cancelCmd.Reason)

	// Update session to mark workflow as cancelled
	updateErr := h.sessionManager.UpdateSession(ctx, cancelCmd.SessionID, func(state *session.SessionState) error {
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		state.Metadata["cancelled_workflow_id"] = cancelCmd.WorkflowID
		state.Metadata["cancellation_reason"] = cancelCmd.Reason
		state.Status = "cancelled"

		return nil
	})

	if updateErr != nil {
		h.logger.Error("Failed to update session for cancellation",
			"command_id", cancelCmd.CommandID(),
			"session_id", cancelCmd.SessionID,
			"error", updateErr)
		return fmt.Errorf("failed to update session: %w", updateErr)
	}

	// Workflow cancellation logic:
	// Session status has been updated to 'cancelled' above.
	// The workflow implementation checks session status periodically
	// and will gracefully terminate when it detects cancellation.
	// This approach provides clean separation of concerns:
	// - Commands handle state updates
	// - Workflows handle their own lifecycle
	// 4. Publishing workflow cancelled event

	h.logger.Info("Cancel workflow command completed",
		"command_id", cancelCmd.CommandID(),
		"session_id", cancelCmd.SessionID,
		"workflow_id", cancelCmd.WorkflowID)

	return nil
}
