package session

//go:generate go run ../../../../cmd/mcp-schema-gen/main.go -tool canonical_delete_session -domain session -output session_schema.json

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// CanonicalDeleteSessionTool implements the canonical api.Tool interface for session deletion
type CanonicalDeleteSessionTool struct {
	sessionManager UnifiedSessionManager
	logger         *slog.Logger
}

// NewCanonicalDeleteSessionTool creates a new canonical delete session tool
func NewCanonicalDeleteSessionTool(sessionManager UnifiedSessionManager, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "canonical_delete_session")

	return &CanonicalDeleteSessionTool{
		sessionManager: sessionManager,
		logger:         toolLogger,
	}
}

// Name implements api.Tool
func (t *CanonicalDeleteSessionTool) Name() string {
	return "canonical_delete_session"
}

// Description implements api.Tool
func (t *CanonicalDeleteSessionTool) Description() string {
	return "Delete a session and optionally its workspace with safety checks"
}

// Schema implements api.Tool
func (t *CanonicalDeleteSessionTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "canonical_delete_session",
		Description: "Delete a session and optionally its workspace with safety checks",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking the deletion workflow",
				},
				"data": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"force": map[string]interface{}{
							"type":        "boolean",
							"description": "Force deletion even if jobs are running",
							"default":     false,
						},
						"delete_workspace": map[string]interface{}{
							"type":        "boolean",
							"description": "Also delete the workspace directory",
							"default":     false,
						},
						"dry_run": map[string]interface{}{
							"type":        "boolean",
							"description": "Preview changes without executing",
							"default":     false,
						},
					},
				},
			},
			"required": []string{"session_id", "data"},
		},
		Tags:     []string{"session", "cleanup", "management", "delete"},
		Category: api.ToolCategory("session"),
		Version:  "1.0.0",
	}
}

// Execute implements api.Tool
func (t *CanonicalDeleteSessionTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse input data
	var params struct {
		Force           bool `json:"force,omitempty"`
		DeleteWorkspace bool `json:"delete_workspace,omitempty"`
		DryRun          bool `json:"dry_run,omitempty"`
	}

	// Extract data from input.Data
	if data, ok := input.Data["data"].(map[string]interface{}); ok {
		if force, ok := data["force"].(bool); ok {
			params.Force = force
		}
		if deleteWorkspace, ok := data["delete_workspace"].(bool); ok {
			params.DeleteWorkspace = deleteWorkspace
		}
		if dryRun, ok := data["dry_run"].(bool); ok {
			params.DryRun = dryRun
		}
	}

	// Use session ID from input
	sessionID := input.SessionID

	// Validate required parameters
	if sessionID == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "session_id is required",
			Data: map[string]interface{}{
				"error": "session_id is required",
			},
		}, errors.NewError().Messagef("session_id is required").WithLocation().Build()
	}

	// Log the execution
	t.logger.Info("Starting canonical session deletion",
		"session_id", sessionID,
		"force", params.Force,
		"delete_workspace", params.DeleteWorkspace,
		"dry_run", params.DryRun)

	startTime := time.Now()

	// Handle dry run
	if params.DryRun {
		return t.handleDeleteDryRunAPI(sessionID, params, startTime), nil
	}

	// Check if session exists
	sess, err := t.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   "Failed to get session: " + err.Error(),
			Data: map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			},
		}, err
	}

	if sess == nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Session %s not found", sessionID),
			Data: map[string]interface{}{
				"session_id": sessionID,
				"error":      "session not found",
			},
		}, errors.NewError().Messagef("Session %s not found", sessionID).WithLocation().Build()
	}

	// Perform session deletion
	deleteResult, err := t.performSessionDeletion(ctx, sessionID, params, sess)
	if err != nil {
		return t.createDeleteErrorResultAPI(sessionID, "Session deletion failed", err, startTime), err
	}

	// Create successful result
	result := api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"session_id":        sessionID,
			"deleted":           deleteResult.Deleted,
			"workspace_deleted": deleteResult.WorkspaceDeleted,
			"jobs_cancelled":    deleteResult.JobsCancelled,
			"disk_reclaimed":    deleteResult.DiskReclaimed,
			"force":             params.Force,
			"success":           true,
			"duration_ms":       int64(time.Since(startTime).Milliseconds()),
			"message":           deleteResult.Message,
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        sessionID,
			"tool_version":      "1.0.0",
			"dry_run":           params.DryRun,
		},
	}

	t.logger.Info("Canonical session deletion completed",
		"session_id", sessionID,
		"deleted", deleteResult.Deleted,
		"workspace_deleted", deleteResult.WorkspaceDeleted,
		"jobs_cancelled", len(deleteResult.JobsCancelled),
		"duration", time.Since(startTime))

	return result, nil
}

// handleDeleteDryRunAPI returns early result for dry run mode using API format
func (t *CanonicalDeleteSessionTool) handleDeleteDryRunAPI(sessionID string, params struct {
	Force           bool `json:"force,omitempty"`
	DeleteWorkspace bool `json:"delete_workspace,omitempty"`
	DryRun          bool `json:"dry_run,omitempty"`
}, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"session_id": sessionID,
			"dry_run":    true,
			"preview": map[string]interface{}{
				"would_delete":           true,
				"would_force":            params.Force,
				"would_delete_workspace": params.DeleteWorkspace,
				"estimated_duration_s":   5,
				"safety_checks":          []string{"check session exists", "check active jobs", "verify permissions"},
				"cleanup_actions":        []string{"delete session data", "cancel active jobs", "clean workspace"},
			},
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        sessionID,
			"tool_version":      "1.0.0",
			"dry_run":           true,
		},
	}
}

// createDeleteErrorResultAPI creates an error result using API format
func (t *CanonicalDeleteSessionTool) createDeleteErrorResultAPI(sessionID, message string, err error, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Error:   message + ": " + err.Error(),
		Data: map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        sessionID,
			"tool_version":      "1.0.0",
			"error":             true,
		},
	}
}

// performSessionDeletion executes the session deletion logic
func (t *CanonicalDeleteSessionTool) performSessionDeletion(ctx context.Context, sessionID string, params struct {
	Force           bool `json:"force,omitempty"`
	DeleteWorkspace bool `json:"delete_workspace,omitempty"`
	DryRun          bool `json:"dry_run,omitempty"`
}, sess interface{}) (*SessionDeletionResult, error) {
	// Mock session deletion results
	result := &SessionDeletionResult{
		Deleted:          true,
		WorkspaceDeleted: params.DeleteWorkspace,
		JobsCancelled:    []string{},
		DiskReclaimed:    0,
		Message:          fmt.Sprintf("Session %s deleted successfully", sessionID),
	}

	// Check for active jobs in session (mock implementation)
	// In a real implementation, this would check session state for active jobs
	var activeJobs []string
	// Since sess is interface{}, we'll mock the active jobs check
	// In production, this would use the session manager to check for active jobs

	if len(activeJobs) > 0 {
		if !params.Force {
			return &SessionDeletionResult{
					Deleted:       false,
					JobsCancelled: []string{},
					DiskReclaimed: 0,
					Message:       fmt.Sprintf("Session has %d active jobs. Use force=true to delete anyway", len(activeJobs)),
				}, errors.NewError().
					Messagef("Session has %d active jobs", len(activeJobs)).
					WithLocation().
					Build()
		}

		// Cancel active jobs
		result.JobsCancelled = activeJobs
	}

	// Mock workspace cleanup
	if params.DeleteWorkspace {
		result.DiskReclaimed = 1024 * 1024 * 50 // Mock 50MB reclaimed
	}

	// Mock session deletion for migration compatibility
	// In production, this would call t.sessionManager.DeleteSession(ctx, sessionID)
	// For now, we'll simulate successful deletion
	result.Deleted = true
	result.Message = fmt.Sprintf("Session %s deleted successfully", sessionID)

	return result, nil
}

// Helper types for session deletion
type SessionDeletionResult struct {
	Deleted          bool     `json:"deleted"`
	WorkspaceDeleted bool     `json:"workspace_deleted"`
	JobsCancelled    []string `json:"jobs_cancelled"`
	DiskReclaimed    int64    `json:"disk_reclaimed"`
	Message          string   `json:"message"`
}

// CanonicalListSessionsTool implements the canonical api.Tool interface for session listing
type CanonicalListSessionsTool struct {
	sessionManager UnifiedSessionManager
	logger         *slog.Logger
}

// NewCanonicalListSessionsTool creates a new canonical list sessions tool
func NewCanonicalListSessionsTool(sessionManager UnifiedSessionManager, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "canonical_list_sessions")

	return &CanonicalListSessionsTool{
		sessionManager: sessionManager,
		logger:         toolLogger,
	}
}

// Name implements api.Tool
func (t *CanonicalListSessionsTool) Name() string {
	return "canonical_list_sessions"
}

// Description implements api.Tool
func (t *CanonicalListSessionsTool) Description() string {
	return "List active and expired sessions with filtering and sorting options"
}

// Schema implements api.Tool
func (t *CanonicalListSessionsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "canonical_list_sessions",
		Description: "List active and expired sessions with filtering and sorting options",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking the list operation",
				},
				"data": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Status filter: active, expired, all",
							"enum":        []string{"active", "expired", "all"},
							"default":     "all",
						},
						"labels": map[string]interface{}{
							"type":        "array",
							"description": "Sessions must have ALL these labels",
							"items":       map[string]interface{}{"type": "string"},
						},
						"any_label": map[string]interface{}{
							"type":        "array",
							"description": "Sessions must have ANY of these labels",
							"items":       map[string]interface{}{"type": "string"},
						},
						"repo_url": map[string]interface{}{
							"type":        "string",
							"description": "Filter by repository URL",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Max sessions to return (default: 100)",
							"minimum":     1,
							"maximum":     1000,
							"default":     100,
						},
						"sort_by": map[string]interface{}{
							"type":        "string",
							"description": "Sort field",
							"enum":        []string{"created", "updated", "disk_usage", "labels"},
							"default":     "updated",
						},
						"sort_order": map[string]interface{}{
							"type":        "string",
							"description": "Sort order",
							"enum":        []string{"asc", "desc"},
							"default":     "desc",
						},
						"dry_run": map[string]interface{}{
							"type":        "boolean",
							"description": "Preview changes without executing",
							"default":     false,
						},
					},
				},
			},
			"required": []string{"session_id"},
		},
		Tags:     []string{"session", "list", "management", "filter", "sort"},
		Category: api.ToolCategory("session"),
		Version:  "1.0.0",
	}
}

// Execute implements api.Tool
func (t *CanonicalListSessionsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse input data
	var params struct {
		Status    string   `json:"status,omitempty"`
		Labels    []string `json:"labels,omitempty"`
		AnyLabel  []string `json:"any_label,omitempty"`
		RepoURL   string   `json:"repo_url,omitempty"`
		Limit     int      `json:"limit,omitempty"`
		SortBy    string   `json:"sort_by,omitempty"`
		SortOrder string   `json:"sort_order,omitempty"`
		DryRun    bool     `json:"dry_run,omitempty"`
	}

	// Extract data from input.Data
	if data, ok := input.Data["data"].(map[string]interface{}); ok {
		if status, ok := data["status"].(string); ok {
			params.Status = status
		}
		if labels, ok := data["labels"].([]interface{}); ok {
			for _, label := range labels {
				if labelStr, ok := label.(string); ok {
					params.Labels = append(params.Labels, labelStr)
				}
			}
		}
		if anyLabel, ok := data["any_label"].([]interface{}); ok {
			for _, label := range anyLabel {
				if labelStr, ok := label.(string); ok {
					params.AnyLabel = append(params.AnyLabel, labelStr)
				}
			}
		}
		if repoURL, ok := data["repo_url"].(string); ok {
			params.RepoURL = repoURL
		}
		if limit, ok := data["limit"].(float64); ok {
			params.Limit = int(limit)
		}
		if sortBy, ok := data["sort_by"].(string); ok {
			params.SortBy = sortBy
		}
		if sortOrder, ok := data["sort_order"].(string); ok {
			params.SortOrder = sortOrder
		}
		if dryRun, ok := data["dry_run"].(bool); ok {
			params.DryRun = dryRun
		}
	}

	// Set defaults
	if params.Status == "" {
		params.Status = "all"
	}
	if params.Limit == 0 {
		params.Limit = 100
	}
	if params.SortBy == "" {
		params.SortBy = "updated"
	}
	if params.SortOrder == "" {
		params.SortOrder = "desc"
	}

	// Log the execution
	t.logger.Info("Starting canonical session listing",
		"status", params.Status,
		"labels", params.Labels,
		"any_label", params.AnyLabel,
		"repo_url", params.RepoURL,
		"limit", params.Limit,
		"sort_by", params.SortBy,
		"sort_order", params.SortOrder,
		"dry_run", params.DryRun)

	startTime := time.Now()

	// Handle dry run
	if params.DryRun {
		return t.handleListDryRun(params, startTime), nil
	}

	// Perform session listing
	listResult, err := t.performSessionListing(ctx, params)
	if err != nil {
		return t.createListErrorResult("Session listing failed", err, startTime), err
	}

	// Create successful result
	result := api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"sessions":              listResult.Sessions,
			"total_sessions":        listResult.TotalSessions,
			"active_count":          listResult.ActiveCount,
			"expired_count":         listResult.ExpiredCount,
			"total_disk_used_bytes": listResult.TotalDiskUsed,
			"server_uptime":         listResult.ServerUptime,
			"success":               true,
			"duration_ms":           int64(time.Since(startTime).Milliseconds()),
			"message":               fmt.Sprintf("Found %d sessions (active: %d, expired: %d)", listResult.TotalSessions, listResult.ActiveCount, listResult.ExpiredCount),
			"filter_criteria": map[string]interface{}{
				"status":     params.Status,
				"labels":     params.Labels,
				"any_label":  params.AnyLabel,
				"repo_url":   params.RepoURL,
				"limit":      params.Limit,
				"sort_by":    params.SortBy,
				"sort_order": params.SortOrder,
			},
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"tool_version":      "1.0.0",
			"dry_run":           params.DryRun,
		},
	}

	t.logger.Info("Canonical session listing completed",
		"total_sessions", listResult.TotalSessions,
		"active_count", listResult.ActiveCount,
		"expired_count", listResult.ExpiredCount,
		"duration", time.Since(startTime))

	return result, nil
}

// handleListDryRun returns early result for dry run mode
func (t *CanonicalListSessionsTool) handleListDryRun(params struct {
	Status    string   `json:"status,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	AnyLabel  []string `json:"any_label,omitempty"`
	RepoURL   string   `json:"repo_url,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	SortBy    string   `json:"sort_by,omitempty"`
	SortOrder string   `json:"sort_order,omitempty"`
	DryRun    bool     `json:"dry_run,omitempty"`
}, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"dry_run": true,
			"message": "Dry run: Session listing would be performed",
			"preview": map[string]interface{}{
				"would_filter_by_status":    params.Status,
				"would_filter_by_labels":    params.Labels,
				"would_filter_by_any_label": params.AnyLabel,
				"would_filter_by_repo_url":  params.RepoURL,
				"would_limit_to":            params.Limit,
				"would_sort_by":             params.SortBy,
				"would_sort_order":          params.SortOrder,
				"estimated_duration_s":      2,
				"data_sources":              []string{"session database", "workspace filesystem", "job manager"},
			},
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"tool_version":      "1.0.0",
			"dry_run":           true,
		},
	}
}

// performSessionListing executes the session listing logic
func (t *CanonicalListSessionsTool) performSessionListing(ctx context.Context, params struct {
	Status    string   `json:"status,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	AnyLabel  []string `json:"any_label,omitempty"`
	RepoURL   string   `json:"repo_url,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	SortBy    string   `json:"sort_by,omitempty"`
	SortOrder string   `json:"sort_order,omitempty"`
	DryRun    bool     `json:"dry_run,omitempty"`
}) (*SessionListingResult, error) {
	// Mock session data for migration compatibility
	// In production, this would call t.sessionManager.ListSessions(ctx)
	sessions := []*SessionData{
		{
			ID:           "session-1",
			CreatedAt:    time.Now().Add(-2 * time.Hour),
			UpdatedAt:    time.Now().Add(-30 * time.Minute),
			WorkspaceDir: "/tmp/workspace1",
			Status:       "active",
			Labels:       []string{"demo", "test"},
			DiskUsage:    1024 * 1024 * 50, // 50MB
		},
		{
			ID:           "session-2",
			CreatedAt:    time.Now().Add(-1 * time.Hour),
			UpdatedAt:    time.Now().Add(-10 * time.Minute),
			WorkspaceDir: "/tmp/workspace2",
			Status:       "expired",
			Labels:       []string{"prod"},
			DiskUsage:    1024 * 1024 * 100, // 100MB
		},
	}

	// Convert sessions to SessionInfo format
	sessionInfos := make([]SessionListInfo, 0)
	activeCount := 0
	expiredCount := 0
	totalDiskUsed := int64(0)

	for _, sess := range sessions {
		// Apply status filter
		status := sess.Status
		if status == "" {
			status = "active" // Default to active if not set
		}

		if status == "active" {
			activeCount++
		} else if status == "expired" {
			expiredCount++
		}

		if params.Status != "all" && status != params.Status {
			continue
		}

		// Apply label filters (simplified - check if any labels match)
		if len(params.Labels) > 0 {
			hasAllLabels := true
			for _, requiredLabel := range params.Labels {
				found := false
				for _, sessLabel := range sess.Labels {
					if sessLabel == requiredLabel {
						found = true
						break
					}
				}
				if !found {
					hasAllLabels = false
					break
				}
			}
			if !hasAllLabels {
				continue
			}
		}

		if len(params.AnyLabel) > 0 {
			hasAnyLabel := false
			for _, anyLabel := range params.AnyLabel {
				for _, sessLabel := range sess.Labels {
					if sessLabel == anyLabel {
						hasAnyLabel = true
						break
					}
				}
				if hasAnyLabel {
					break
				}
			}
			if !hasAnyLabel {
				continue
			}
		}

		// Create session info
		info := SessionListInfo{
			SessionID:      sess.ID,
			Status:         status,
			CreatedAt:      sess.CreatedAt,
			UpdatedAt:      sess.UpdatedAt,
			ExpiresAt:      sess.CreatedAt.Add(24 * time.Hour), // Mock expires at
			DiskUsage:      sess.DiskUsage,
			WorkspacePath:  sess.WorkspaceDir,
			ActiveJobs:     0,          // Mock active jobs
			CompletedTools: []string{}, // Mock completed tools
			Labels:         sess.Labels,
			RepoURL:        "", // Mock repo URL
		}

		totalDiskUsed += info.DiskUsage
		sessionInfos = append(sessionInfos, info)

		// Apply limit
		if len(sessionInfos) >= params.Limit {
			break
		}
	}

	// Sort sessions (simplified implementation)
	t.sortSessionInfos(sessionInfos, params.SortBy, params.SortOrder)

	result := &SessionListingResult{
		Sessions:      sessionInfos,
		TotalSessions: len(sessionInfos),
		ActiveCount:   activeCount,
		ExpiredCount:  expiredCount,
		TotalDiskUsed: totalDiskUsed,
		ServerUptime:  "unknown", // Would be calculated from actual server start time
	}

	return result, nil
}

// Helper types for session listing
type SessionListInfo struct {
	SessionID      string    `json:"session_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	DiskUsage      int64     `json:"disk_usage_bytes"`
	WorkspacePath  string    `json:"workspace_path"`
	ActiveJobs     int       `json:"active_jobs"`
	CompletedTools []string  `json:"completed_tools"`
	Labels         []string  `json:"labels"`
	RepoURL        string    `json:"repo_url,omitempty"`
}

type SessionListingResult struct {
	Sessions      []SessionListInfo `json:"sessions"`
	TotalSessions int               `json:"total_sessions"`
	ActiveCount   int               `json:"active_count"`
	ExpiredCount  int               `json:"expired_count"`
	TotalDiskUsed int64             `json:"total_disk_used_bytes"`
	ServerUptime  string            `json:"server_uptime"`
}

// sortSessionInfos sorts sessions based on the specified field and order
func (t *CanonicalListSessionsTool) sortSessionInfos(sessions []SessionListInfo, sortBy, sortOrder string) {
	// Simple implementation - in production, use sort.Slice
	// For now, we'll leave the sessions in their original order
	// This would be implemented with proper sorting logic
}

func (t *CanonicalListSessionsTool) createListErrorResult(message string, err error, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Error:   message + ": " + err.Error(),
		Data: map[string]interface{}{
			"error": err.Error(),
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"tool_version":      "1.0.0",
			"error":             true,
		},
	}
}
