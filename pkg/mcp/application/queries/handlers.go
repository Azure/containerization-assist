// Package queries provides query handlers for Container Kit MCP.
package queries

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
)

// QueryHandler handles query execution
type QueryHandler interface {
	Handle(ctx context.Context, query Query) (interface{}, error)
}

// WorkflowStatusQueryHandler handles workflow status queries
type WorkflowStatusQueryHandler struct {
	sessionManager session.SessionManager
	resourceStore  *resources.Store
	logger         *slog.Logger
}

// NewWorkflowStatusQueryHandler creates a new workflow status query handler
func NewWorkflowStatusQueryHandler(
	sessionManager session.SessionManager,
	resourceStore *resources.Store,
	logger *slog.Logger,
) *WorkflowStatusQueryHandler {
	return &WorkflowStatusQueryHandler{
		sessionManager: sessionManager,
		resourceStore:  resourceStore,
		logger:         logger.With("component", "workflow_status_query_handler"),
	}
}

// Handle executes a workflow status query
func (h *WorkflowStatusQueryHandler) Handle(ctx context.Context, query Query) (interface{}, error) {
	statusQuery, ok := query.(WorkflowStatusQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type: expected WorkflowStatusQuery")
	}

	if err := statusQuery.Validate(); err != nil {
		return nil, fmt.Errorf("query validation failed: %w", err)
	}

	h.logger.Info("Handling workflow status query",
		"query_id", statusQuery.QueryID(),
		"session_id", statusQuery.SessionID,
		"workflow_id", statusQuery.WorkflowID)

	// Get session information
	sessionState, err := h.sessionManager.GetSession(ctx, statusQuery.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Build workflow status view from session metadata
	view := h.buildWorkflowStatusView(sessionState, statusQuery.WorkflowID)

	h.logger.Info("Workflow status query completed",
		"query_id", statusQuery.QueryID(),
		"session_id", statusQuery.SessionID,
		"status", view.Status)

	return view, nil
}

// buildWorkflowStatusView constructs a workflow status view from session state
func (h *WorkflowStatusQueryHandler) buildWorkflowStatusView(sessionState *session.SessionState, workflowID string) *WorkflowStatusView {
	view := &WorkflowStatusView{
		SessionID:   sessionState.SessionID,
		Status:      sessionState.Status,
		StartTime:   sessionState.CreatedAt,
		Progress:    0.0,
		CurrentStep: 0,
		TotalSteps:  10, // Default for containerization workflow
	}

	// Extract workflow information from session metadata
	if sessionState.Metadata != nil {
		if repoURL, ok := sessionState.Metadata["repo_url"].(string); ok {
			view.RepoURL = repoURL
		}
		if branch, ok := sessionState.Metadata["branch"].(string); ok {
			view.Branch = branch
		}
		if workflowResult, ok := sessionState.Metadata["last_workflow_result"].(map[string]interface{}); ok {
			h.populateViewFromWorkflowResult(view, workflowResult)
		}
		if wfID, ok := sessionState.Metadata["workflow_id"].(string); ok {
			view.WorkflowID = wfID
		}
	}

	// Use provided workflow ID if available
	if workflowID != "" {
		view.WorkflowID = workflowID
	}

	// Calculate duration if workflow has ended
	if sessionState.Status == "completed" || sessionState.Status == "failed" {
		endTime := sessionState.UpdatedAt
		view.EndTime = &endTime
		duration := endTime.Sub(sessionState.CreatedAt)
		view.Duration = &duration
	}

	return view
}

// populateViewFromWorkflowResult fills the view with data from workflow result
func (h *WorkflowStatusQueryHandler) populateViewFromWorkflowResult(view *WorkflowStatusView, result map[string]interface{}) {
	if imageRef, ok := result["image_ref"].(string); ok {
		view.ImageRef = imageRef
	}
	if namespace, ok := result["k8s_namespace"].(string); ok {
		view.Namespace = namespace
	}
	if endpoint, ok := result["endpoint"].(string); ok {
		view.Endpoint = endpoint
	}
	if scanReport, ok := result["scan_report"].(map[string]interface{}); ok {
		view.ScanReport = scanReport
	}
	if errorMsg, ok := result["error"].(string); ok {
		view.Error = errorMsg
	}
	if steps, ok := result["steps"].([]interface{}); ok {
		view.Steps = h.convertSteps(steps)
		view.TotalSteps = len(view.Steps)
		view.CurrentStep = h.countCompletedSteps(view.Steps)
		view.Progress = float64(view.CurrentStep) / float64(view.TotalSteps) * 100
	}
}

// convertSteps converts generic steps to WorkflowStep structs
func (h *WorkflowStatusQueryHandler) convertSteps(steps []interface{}) []workflow.WorkflowStep {
	workflowSteps := make([]workflow.WorkflowStep, 0, len(steps))
	for _, step := range steps {
		if stepMap, ok := step.(map[string]interface{}); ok {
			workflowStep := workflow.WorkflowStep{}
			if name, ok := stepMap["name"].(string); ok {
				workflowStep.Name = name
			}
			if status, ok := stepMap["status"].(string); ok {
				workflowStep.Status = status
			}
			if duration, ok := stepMap["duration"].(string); ok {
				workflowStep.Duration = duration
			}
			if errorMsg, ok := stepMap["error"].(string); ok {
				workflowStep.Error = errorMsg
			}
			workflowSteps = append(workflowSteps, workflowStep)
		}
	}
	return workflowSteps
}

// countCompletedSteps counts the number of completed steps
func (h *WorkflowStatusQueryHandler) countCompletedSteps(steps []workflow.WorkflowStep) int {
	completed := 0
	for _, step := range steps {
		if step.Status == "completed" {
			completed++
		}
	}
	return completed
}

// SessionListQueryHandler handles session list queries
type SessionListQueryHandler struct {
	sessionManager session.SessionManager
	logger         *slog.Logger
}

// NewSessionListQueryHandler creates a new session list query handler
func NewSessionListQueryHandler(
	sessionManager session.SessionManager,
	logger *slog.Logger,
) *SessionListQueryHandler {
	return &SessionListQueryHandler{
		sessionManager: sessionManager,
		logger:         logger.With("component", "session_list_query_handler"),
	}
}

// Handle executes a session list query
func (h *SessionListQueryHandler) Handle(ctx context.Context, query Query) (interface{}, error) {
	listQuery, ok := query.(SessionListQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type: expected SessionListQuery")
	}

	if err := listQuery.Validate(); err != nil {
		return nil, fmt.Errorf("query validation failed: %w", err)
	}

	h.logger.Info("Handling session list query",
		"query_id", listQuery.QueryID(),
		"user_id", listQuery.UserID,
		"limit", listQuery.Limit,
		"offset", listQuery.Offset)

	// Get session summaries using existing session manager
	sessionSummaries, err := h.sessionManager.ListSessionSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	// Apply pagination
	total := len(sessionSummaries)
	start := listQuery.Offset
	if start > total {
		start = total
	}

	limit := listQuery.Limit
	if limit <= 0 {
		limit = 50 // Default limit
	}

	end := start + limit
	if end > total {
		end = total
	}

	paginatedSummaries := sessionSummaries[start:end]

	// Convert to view objects
	sessions := make([]SessionSummaryView, len(paginatedSummaries))
	for i, summary := range paginatedSummaries {
		sessions[i] = SessionSummaryView{
			SessionID: summary.ID,
			Labels:    summary.Labels,
			// Note: SessionSummary from the interface doesn't have all fields
			// In a real implementation, you'd extend the interface or use a different approach
		}
	}

	view := &SessionListView{
		Sessions: sessions,
		Total:    total,
		Limit:    limit,
		Offset:   listQuery.Offset,
		HasMore:  end < total,
	}

	h.logger.Info("Session list query completed",
		"query_id", listQuery.QueryID(),
		"total_sessions", total,
		"returned_sessions", len(sessions))

	return view, nil
}
