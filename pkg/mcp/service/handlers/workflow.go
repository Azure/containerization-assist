// Package handlers provides direct request handlers for Container Kit MCP,
// replacing the complex CQRS pattern with simple, direct handlers.
package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
	"github.com/mark3labs/mcp-go/mcp"
)

// WorkflowHandler provides direct workflow operations
type WorkflowHandler struct {
	orchestrator   workflow.EventAwareOrchestrator
	sessionManager session.OptimizedSessionManager
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewWorkflowHandler creates a new workflow handler
func NewWorkflowHandler(
	orchestrator workflow.EventAwareOrchestrator,
	sessionManager session.OptimizedSessionManager,
	eventPublisher *events.Publisher,
	logger *slog.Logger,
) *WorkflowHandler {
	return &WorkflowHandler{
		orchestrator:   orchestrator,
		sessionManager: sessionManager,
		eventPublisher: eventPublisher,
		logger:         logger.With("component", "workflow_handler"),
	}
}

// ContainerizeRequest represents a direct containerization request
type ContainerizeRequest struct {
	SessionID string                             `json:"session_id"`
	Args      workflow.ContainerizeAndDeployArgs `json:"args"`
}

// Validate validates the containerization request
func (r ContainerizeRequest) Validate() error {
	if r.SessionID == "" {
		return errors.New(
			errors.CodeMissingParameter,
			"request.session_id",
			"session ID is required",
			nil,
		)
	}

	if r.Args.RepoURL == "" {
		return errors.New(
			errors.CodeMissingParameter,
			"request.args.repo_url",
			"repository URL is required",
			nil,
		)
	}

	return nil
}

// Containerize executes a containerization workflow
func (h *WorkflowHandler) Containerize(ctx context.Context, req ContainerizeRequest) (*workflow.ContainerizeAndDeployResult, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	h.logger.Info("Starting containerization workflow",
		"session_id", req.SessionID,
		"repo_url", req.Args.RepoURL)

	// Create MCP request for the orchestrator
	mcpReq := &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "start_workflow",
			Arguments: map[string]interface{}{
				"repo_url":  req.Args.RepoURL,
				"branch":    req.Args.Branch,
				"scan":      req.Args.Scan,
				"deploy":    req.Args.Deploy,
				"test_mode": req.Args.TestMode,
			},
		},
	}

	// Execute the workflow
	result, err := h.orchestrator.Execute(ctx, mcpReq, &req.Args)
	if err != nil {
		h.logger.Error("Workflow execution failed",
			"session_id", req.SessionID,
			"error", err)
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	// Update session with result
	updateErr := h.sessionManager.Update(ctx, req.SessionID, func(state *session.SessionState) error {
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		state.Metadata["last_workflow_result"] = result

		if result.Success {
			state.Status = "completed"
		} else {
			state.Status = "failed"
		}

		return nil
	})

	if updateErr != nil {
		h.logger.Error("Failed to update session",
			"session_id", req.SessionID,
			"error", updateErr)
		// Don't fail the operation for session update errors
	}

	h.logger.Info("Containerization workflow completed",
		"session_id", req.SessionID,
		"success", result.Success)

	return result, nil
}

// CancelWorkflowRequest represents a workflow cancellation request
type CancelWorkflowRequest struct {
	SessionID  string `json:"session_id"`
	WorkflowID string `json:"workflow_id"`
	Reason     string `json:"reason,omitempty"`
}

// Validate validates the cancellation request
func (r CancelWorkflowRequest) Validate() error {
	if r.SessionID == "" {
		return errors.New(
			errors.CodeMissingParameter,
			"request.session_id",
			"session ID is required",
			nil,
		)
	}

	if r.WorkflowID == "" {
		return errors.New(
			errors.CodeMissingParameter,
			"request.workflow_id",
			"workflow ID is required",
			nil,
		)
	}

	return nil
}

// CancelWorkflow cancels a running workflow
func (h *WorkflowHandler) CancelWorkflow(ctx context.Context, req CancelWorkflowRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("request validation failed: %w", err)
	}

	h.logger.Info("Cancelling workflow",
		"session_id", req.SessionID,
		"workflow_id", req.WorkflowID,
		"reason", req.Reason)

	// Update session to mark workflow as cancelled
	updateErr := h.sessionManager.Update(ctx, req.SessionID, func(state *session.SessionState) error {
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		state.Metadata["cancelled_workflow_id"] = req.WorkflowID
		state.Metadata["cancellation_reason"] = req.Reason
		state.Status = "cancelled"

		return nil
	})

	if updateErr != nil {
		h.logger.Error("Failed to update session for cancellation",
			"session_id", req.SessionID,
			"error", updateErr)
		return fmt.Errorf("failed to update session: %w", updateErr)
	}

	h.logger.Info("Workflow cancellation completed",
		"session_id", req.SessionID,
		"workflow_id", req.WorkflowID)

	return nil
}

// UpdateConfigRequest represents a configuration update request
type UpdateConfigRequest struct {
	SessionID string                `json:"session_id"`
	Config    workflow.ServerConfig `json:"config"`
}

// Validate validates the configuration update request
func (r UpdateConfigRequest) Validate() error {
	if r.SessionID == "" {
		return errors.New(
			errors.CodeMissingParameter,
			"request.session_id",
			"session ID is required",
			nil,
		)
	}

	return nil
}

// UpdateConfig updates workflow configuration
func (h *WorkflowHandler) UpdateConfig(ctx context.Context, req UpdateConfigRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("request validation failed: %w", err)
	}

	h.logger.Info("Updating workflow configuration",
		"session_id", req.SessionID)

	// Update session with new configuration
	updateErr := h.sessionManager.Update(ctx, req.SessionID, func(state *session.SessionState) error {
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		state.Metadata["config"] = req.Config

		return nil
	})

	if updateErr != nil {
		h.logger.Error("Failed to update session configuration",
			"session_id", req.SessionID,
			"error", updateErr)
		return fmt.Errorf("failed to update session: %w", updateErr)
	}

	h.logger.Info("Configuration update completed",
		"session_id", req.SessionID)

	return nil
}
