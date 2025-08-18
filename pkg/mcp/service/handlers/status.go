package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/containerization-assist/pkg/common/errors"
	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/core/resources"
	"github.com/Azure/containerization-assist/pkg/mcp/service/session"
)

// StatusHandler provides direct status and query operations
type StatusHandler struct {
	sessionManager session.OptimizedSessionManager
	resourceStore  *resources.Store
	logger         *slog.Logger
}

// NewStatusHandler creates a new status handler
func NewStatusHandler(
	sessionManager session.OptimizedSessionManager,
	resourceStore *resources.Store,
	logger *slog.Logger,
) *StatusHandler {
	return &StatusHandler{
		sessionManager: sessionManager,
		resourceStore:  resourceStore,
		logger:         logger.With("component", "status_handler"),
	}
}

// WorkflowStatusRequest represents a workflow status request
type WorkflowStatusRequest struct {
	SessionID  string `json:"session_id"`
	WorkflowID string `json:"workflow_id,omitempty"`
}

// Validate validates the workflow status request
func (r WorkflowStatusRequest) Validate() error {
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

// WorkflowStatusResponse represents the workflow status response
type WorkflowStatusResponse struct {
	WorkflowID  string                  `json:"workflow_id"`
	SessionID   string                  `json:"session_id"`
	Status      string                  `json:"status"`
	Progress    float64                 `json:"progress"`
	CurrentStep int                     `json:"current_step"`
	TotalSteps  int                     `json:"total_steps"`
	Steps       []workflow.WorkflowStep `json:"steps"`
	StartTime   time.Time               `json:"start_time"`
	EndTime     *time.Time              `json:"end_time,omitempty"`
	Duration    *time.Duration          `json:"duration,omitempty"`

	// Result information
	ImageRef   string                 `json:"image_ref,omitempty"`
	Namespace  string                 `json:"k8s_namespace,omitempty"`
	Endpoint   string                 `json:"endpoint,omitempty"`
	ScanReport map[string]interface{} `json:"scan_report,omitempty"`
	Error      string                 `json:"error,omitempty"`

	// Repository information
	RepoURL string `json:"repo_url"`
	Branch  string `json:"branch,omitempty"`
}

// GetWorkflowStatus retrieves the current status of a workflow
func (h *StatusHandler) GetWorkflowStatus(ctx context.Context, req WorkflowStatusRequest) (*WorkflowStatusResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	h.logger.Info("Getting workflow status",
		"session_id", req.SessionID,
		"workflow_id", req.WorkflowID)

	// Get session state
	sessionState, err := h.sessionManager.Get(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Extract workflow information from session metadata
	response := &WorkflowStatusResponse{
		SessionID: req.SessionID,
		Status:    sessionState.Status,
	}

	if sessionState.Metadata != nil {
		// Try to extract workflow result
		if workflowResult, ok := sessionState.Metadata["last_workflow_result"].(*workflow.ContainerizeAndDeployResult); ok {
			// Set basic result information
			if sessionState.SessionID != "" {
				response.WorkflowID = sessionState.SessionID
			}

			// Calculate progress from completed steps
			completedSteps := 0
			totalSteps := len(workflowResult.Steps)
			for _, step := range workflowResult.Steps {
				if step.Status == "completed" {
					completedSteps++
				}
			}

			if totalSteps > 0 {
				response.Progress = float64(completedSteps) / float64(totalSteps) * 100
				response.CurrentStep = completedSteps
				response.TotalSteps = totalSteps
			}

			response.Steps = workflowResult.Steps

			// Set result fields from workflow result
			if workflowResult.ImageRef != "" {
				response.ImageRef = workflowResult.ImageRef
			}
			if workflowResult.Namespace != "" {
				response.Namespace = workflowResult.Namespace
			}
			if workflowResult.Endpoint != "" {
				response.Endpoint = workflowResult.Endpoint
			}
			if len(workflowResult.ScanReport) > 0 {
				response.ScanReport = workflowResult.ScanReport
			}
			if workflowResult.Error != "" {
				response.Error = workflowResult.Error
			}
		}

		// Try to get repo information
		if repoURL, ok := sessionState.Metadata["repo_url"].(string); ok {
			response.RepoURL = repoURL
		}
		if branch, ok := sessionState.Metadata["branch"].(string); ok {
			response.Branch = branch
		}
	}

	h.logger.Info("Retrieved workflow status",
		"session_id", req.SessionID,
		"workflow_id", response.WorkflowID,
		"status", response.Status)

	return response, nil
}

// SessionListRequest represents a session list request
type SessionListRequest struct {
	UserID string `json:"user_id,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// Validate validates the session list request
func (r SessionListRequest) Validate() error {
	if r.Limit < 0 {
		return errors.New(
			errors.CodeInvalidParameter,
			"request.limit",
			"limit must be non-negative",
			nil,
		)
	}
	if r.Offset < 0 {
		return errors.New(
			errors.CodeInvalidParameter,
			"request.offset",
			"offset must be non-negative",
			nil,
		)
	}
	return nil
}

// SessionSummary represents a session summary
type SessionSummary struct {
	SessionID       string                 `json:"session_id"`
	Status          string                 `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	ExpiresAt       time.Time              `json:"expires_at"`
	Labels          map[string]string      `json:"labels"`
	Metadata        map[string]interface{} `json:"metadata"`
	ActiveWorkflows int                    `json:"active_workflows"`
	TotalWorkflows  int                    `json:"total_workflows"`
}

// SessionListResponse represents the response for listing sessions
type SessionListResponse struct {
	Sessions []SessionSummary `json:"sessions"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	HasMore  bool             `json:"has_more"`
}

// ListSessions retrieves a list of sessions
func (h *StatusHandler) ListSessions(ctx context.Context, req SessionListRequest) (*SessionListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	h.logger.Info("Listing sessions",
		"user_id", req.UserID,
		"limit", req.Limit,
		"offset", req.Offset)

	// Get all sessions
	sessions, err := h.sessionManager.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	// Convert to summary format
	var summaries []SessionSummary
	for _, sessionState := range sessions {
		// Apply user filter if specified
		if req.UserID != "" && sessionState.UserID != req.UserID {
			continue
		}

		summary := SessionSummary{
			SessionID: sessionState.SessionID,
			Status:    sessionState.Status,
			CreatedAt: sessionState.CreatedAt,
			UpdatedAt: sessionState.UpdatedAt,
			ExpiresAt: sessionState.ExpiresAt,
			Labels:    sessionState.Labels,
			Metadata:  sessionState.Metadata,
			// TODO: Calculate workflow counts from metadata
			ActiveWorkflows: 0,
			TotalWorkflows:  1, // Simplified for now
		}
		summaries = append(summaries, summary)
	}

	// Apply pagination
	total := len(summaries)

	// Set default limit if not specified
	if req.Limit == 0 {
		req.Limit = 50 // Default page size
	}

	start := req.Offset
	if start > total {
		start = total
	}

	end := start + req.Limit
	if end > total {
		end = total
	}

	paginatedSessions := summaries[start:end]
	hasMore := end < total

	response := &SessionListResponse{
		Sessions: paginatedSessions,
		Total:    total,
		Limit:    req.Limit,
		Offset:   req.Offset,
		HasMore:  hasMore,
	}

	h.logger.Info("Retrieved sessions",
		"total", total,
		"returned", len(paginatedSessions),
		"has_more", hasMore)

	return response, nil
}
