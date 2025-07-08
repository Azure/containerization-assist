package session

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// Session constants
const (
	SessionStatusActive   = "active"
	SessionStatusExpired  = "expired"
	SessionStatusInactive = "inactive"
	SessionSortOrderAsc   = "asc"
)

// ListSessionsArgs represents the arguments for listing sessions
type ListSessionsArgs struct {
	DryRun    bool   `json:"dry_run,omitempty"`    // Preview changes without executing
	SessionID string `json:"session_id,omitempty"` // Session ID for state correlation
	// Filter options
	Status    string   `json:"status,omitempty"`     // Status filter: active, expired, all
	Labels    []string `json:"labels,omitempty"`     // Sessions must have ALL these labels
	AnyLabel  []string `json:"any_label,omitempty"`  // Sessions must have ANY of these labels
	RepoURL   string   `json:"repo_url,omitempty"`   // Filter by repository URL
	Limit     int      `json:"limit,omitempty"`      // Max sessions to return
	SortBy    string   `json:"sort_by,omitempty"`    // "created", "updated", "disk_usage", "labels"
	SortOrder string   `json:"sort_order,omitempty"` // Sort order: asc, desc
}

// SessionInfo represents information about a session
type SessionInfo struct {
	SessionID      string            `json:"session_id"`
	Status         string            `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	ExpiresAt      time.Time         `json:"expires_at"`
	DiskUsage      int64             `json:"disk_usage_bytes"`
	WorkspacePath  string            `json:"workspace_path"`
	ActiveJobs     int               `json:"active_jobs"`
	CompletedTools []string          `json:"completed_tools"`
	LastError      string            `json:"last_error,omitempty"`
	Labels         []string          `json:"labels"`
	RepoURL        string            `json:"repo_url,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// ListSessionsResult represents the result of listing sessions
type ListSessionsResult struct {
	Version       string            `json:"version"`    // Schema version
	Tool          string            `json:"tool"`       // Tool name for correlation
	Timestamp     time.Time         `json:"timestamp"`  // Execution timestamp
	SessionID     string            `json:"session_id"` // Session correlation
	DryRun        bool              `json:"dry_run"`    // Whether this was a dry-run
	Sessions      []SessionInfo     `json:"sessions"`
	TotalSessions int               `json:"total_sessions"`
	ActiveCount   int               `json:"active_count"`
	ExpiredCount  int               `json:"expired_count"`
	TotalDiskUsed int64             `json:"total_disk_used_bytes"`
	ServerUptime  string            `json:"server_uptime"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ToolSchema represents the schema for a tool
type ToolSchema struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Version      string      `json:"version"`
	InputSchema  interface{} `json:"input_schema"`
	OutputSchema interface{} `json:"output_schema"`
}

// ListSessionsToolOutput implements api.ToolOutput
type ListSessionsToolOutput struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// IsSuccess implements api.ToolOutput
func (o *ListSessionsToolOutput) IsSuccess() bool {
	return o.Success
}

// GetData implements api.ToolOutput
func (o *ListSessionsToolOutput) GetData() interface{} {
	return o.Data
}

// UnifiedListSessionsTool is a wrapper that implements api.Tool
type UnifiedListSessionsTool struct {
	tool *ListSessionsTool
}

// NewUnifiedListSessionsTool creates a Tool wrapper for ListSessionsTool
func NewUnifiedListSessionsTool(tool *ListSessionsTool) api.Tool {
	return &UnifiedListSessionsTool{tool: tool}
}

// Name implements api.Tool
func (u *UnifiedListSessionsTool) Name() string {
	return "list_sessions"
}

// Description implements api.Tool
func (u *UnifiedListSessionsTool) Description() string {
	return "List active and expired sessions with filtering and sorting options"
}

// Schema implements api.Tool
func (u *UnifiedListSessionsTool) Schema() api.ToolSchema {
	schema := u.GetSchema()
	// Convert local ToolSchema to api.ToolSchema
	return api.ToolSchema{
		Name:         schema.Name,
		Description:  schema.Description,
		Version:      schema.Version,
		InputSchema:  schema.InputSchema.(map[string]interface{}),
		OutputSchema: schema.OutputSchema.(map[string]interface{}),
	}
}

// Execute implements api.Tool
func (u *UnifiedListSessionsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Convert ToolInput to ListSessionsArgs
	args := ListSessionsArgs{
		SessionID: input.SessionID,
	}

	// Extract parameters from input data
	if inputData := input.Data; inputData != nil {
		if status, ok := inputData["status"].(string); ok {
			args.Status = status
		}
		if labels, ok := inputData["labels"].([]string); ok {
			args.Labels = labels
		}
		if anyLabel, ok := inputData["any_label"].([]string); ok {
			args.AnyLabel = anyLabel
		}
		if repoURL, ok := inputData["repo_url"].(string); ok {
			args.RepoURL = repoURL
		}
		if limit, ok := inputData["limit"].(int); ok {
			args.Limit = limit
		}
		if sortBy, ok := inputData["sort_by"].(string); ok {
			args.SortBy = sortBy
		}
		if sortOrder, ok := inputData["sort_order"].(string); ok {
			args.SortOrder = sortOrder
		}
		if dryRun, ok := inputData["dry_run"].(bool); ok {
			args.DryRun = dryRun
		}
	}

	// Call the original tool
	result, err := u.tool.ExecuteTyped(ctx, args)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Convert result to ToolOutput
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"sessions":              result.Sessions,
			"total_sessions":        result.TotalSessions,
			"active_count":          result.ActiveCount,
			"expired_count":         result.ExpiredCount,
			"total_disk_used_bytes": result.TotalDiskUsed,
			"server_uptime":         result.ServerUptime,
			"metadata":              result.Metadata,
		},
	}, nil
}

// GetSchema returns the tool schema
func (u *UnifiedListSessionsTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "list_sessions",
		Description: "List active and expired sessions with filtering and sorting options",
		Version:     "1.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status":     map[string]interface{}{"type": "string", "enum": []string{"active", "expired", "all"}},
				"labels":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"any_label":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"repo_url":   map[string]interface{}{"type": "string"},
				"limit":      map[string]interface{}{"type": "integer", "minimum": 1},
				"sort_by":    map[string]interface{}{"type": "string", "enum": []string{"created", "updated", "disk_usage", "labels"}},
				"sort_order": map[string]interface{}{"type": "string", "enum": []string{"asc", "desc"}},
				"dry_run":    map[string]interface{}{"type": "boolean"},
				"session_id": map[string]interface{}{"type": "string"},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sessions":              map[string]interface{}{"type": "array"},
				"total_sessions":        map[string]interface{}{"type": "integer"},
				"active_count":          map[string]interface{}{"type": "integer"},
				"expired_count":         map[string]interface{}{"type": "integer"},
				"total_disk_used_bytes": map[string]interface{}{"type": "integer"},
			},
		},
	}
}

// ListSessionsTool implements the list_sessions MCP tool
type ListSessionsTool struct {
	logger         zerolog.Logger
	sessionManager *SessionManager
}

// NewListSessionsTool creates a new list sessions tool
func NewListSessionsTool(logger zerolog.Logger, sessionManager *SessionManager) *ListSessionsTool {
	return &ListSessionsTool{
		logger:         logger,
		sessionManager: sessionManager,
	}
}

// Execute implements the api.Tool interface for backward compatibility
func (t *ListSessionsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	listArgs, ok := args.(ListSessionsArgs)
	if !ok {
		return nil, errors.NewError().Messagef("invalid arguments type: expected ListSessionsArgs, got %T", args).Build()
	}

	return t.ExecuteTyped(ctx, listArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *ListSessionsTool) ExecuteTyped(ctx context.Context, args ListSessionsArgs) (*ListSessionsResult, error) {
	t.logger.Info().
		Str("status", args.Status).
		Strs("labels", args.Labels).
		Strs("any_label", args.AnyLabel).
		Str("repo_url", args.RepoURL).
		Int("limit", args.Limit).
		Str("sort_by", args.SortBy).
		Msg("Listing sessions")

	// Set defaults
	if args.Status == "" {
		args.Status = "all"
	}
	if args.Limit == 0 {
		args.Limit = 100
	}
	if args.SortBy == "" {
		args.SortBy = "updated"
	}
	if args.SortOrder == "" {
		args.SortOrder = "desc"
	}

	// Get all sessions
	sessionsMap, err := t.sessionManager.GetAllSessions()
	if err != nil {
		return nil, errors.Wrapf(err, "list_sessions", "Failed to retrieve sessions from session manager")
	}

	// Convert map to slice
	sessions := make([]*SessionData, 0, len(sessionsMap))
	for _, session := range sessionsMap {
		sessions = append(sessions, &SessionData{
			ID:           session.SessionID,
			CreatedAt:    session.CreatedAt,
			UpdatedAt:    session.LastAccessed,
			WorkspaceDir: session.WorkspaceDir,
			Metadata:     session.Metadata,
			Status:       session.Status,
			Labels:       session.Labels,
			DiskUsage:    session.DiskUsage,
		})
	}

	// Get stats
	stats, err := t.sessionManager.GetStats(ctx)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session stats").Cause(err).WithLocation(

		// Filter sessions
		).Build()
	}

	filteredSessions := t.filterSessions(sessions, args)

	// Sort sessions
	t.sortSessions(filteredSessions, args.SortBy, args.SortOrder)

	// Apply limit
	if args.Limit > 0 && len(filteredSessions) > args.Limit {
		filteredSessions = filteredSessions[:args.Limit]
	}

	// Convert to SessionInfo
	sessionInfos := make([]SessionInfo, 0, len(filteredSessions))
	for _, session := range filteredSessions {
		info := SessionInfo{
			SessionID:      session.ID,
			Status:         t.getSessionStatus(session),
			CreatedAt:      session.CreatedAt,
			UpdatedAt:      session.UpdatedAt,
			ExpiresAt:      time.Time{}, // Set default
			DiskUsage:      session.DiskUsage,
			WorkspacePath:  session.WorkspaceDir,
			ActiveJobs:     0,          // Set default
			CompletedTools: []string{}, // Set default
			LastError:      "",         // Set default
			Labels:         session.Labels,
			RepoURL:        "",                      // Set default
			Metadata:       make(map[string]string), // Convert from interface{} map
		}

		// Convert metadata if it exists
		if session.Metadata != nil {
			for k, v := range session.Metadata {
				if strVal, ok := v.(string); ok {
					info.Metadata[k] = strVal
				}
			}
		}

		sessionInfos = append(sessionInfos, info)
	}

	// Get additional stats from the session manager
	var expiredCount int

	// Calculate expired sessions
	for _, session := range sessionInfos {
		if session.Status == "expired" {
			expiredCount++
		}
	}

	result := &ListSessionsResult{
		Version:       "v1.0.0",
		Tool:          "list_sessions",
		Timestamp:     time.Now(),
		SessionID:     args.SessionID,
		DryRun:        args.DryRun,
		Sessions:      sessionInfos,
		TotalSessions: stats.TotalSessions,
		ActiveCount:   stats.ActiveSessions,
		ExpiredCount:  expiredCount,
		TotalDiskUsed: 0,
		ServerUptime:  "unknown",
		Metadata: map[string]string{
			"filter_status": args.Status,
			"sort_by":       args.SortBy,
			"sort_order":    args.SortOrder,
			"limit":         fmt.Sprintf("%d", args.Limit),
		},
	}

	t.logger.Info().
		Int("total_sessions", len(sessionInfos)).
		Int("active_count", stats.ActiveSessions).
		Int64("total_disk_bytes", 0).
		Msg("Sessions listed successfully")

	return result, nil
}

// filterSessions filters sessions based on multiple criteria
func (t *ListSessionsTool) filterSessions(sessions []*SessionData, args ListSessionsArgs) []*SessionData {
	filtered := make([]*SessionData, 0)

	for _, session := range sessions {
		if t.matchesFilters(session, args) {
			filtered = append(filtered, session)
		}
	}

	return filtered
}

// matchesFilters checks if a session matches all filter criteria
func (t *ListSessionsTool) matchesFilters(session *SessionData, args ListSessionsArgs) bool {
	// Check status filter
	if args.Status != "all" && args.Status != "" {
		sessionStatus := t.getSessionStatus(session)
		if sessionStatus != args.Status {
			return false
		}
	}

	// Check ALL labels requirement
	if len(args.Labels) > 0 {
		for _, requiredLabel := range args.Labels {
			if !t.hasLabel(session, requiredLabel) {
				return false
			}
		}
	}

	// Check ANY label requirement
	if len(args.AnyLabel) > 0 {
		hasAnyLabel := false
		for _, anyLabel := range args.AnyLabel {
			if t.hasLabel(session, anyLabel) {
				hasAnyLabel = true
				break
			}
		}
		if !hasAnyLabel {
			return false
		}
	}

	// Check repository URL (check in metadata)
	if args.RepoURL != "" {
		if session.Metadata == nil {
			return false
		}
		if repoURL, ok := session.Metadata["repo_url"].(string); !ok || repoURL != args.RepoURL {
			return false
		}
	}

	return true
}

// hasLabel checks if a session has a specific label
func (t *ListSessionsTool) hasLabel(session *SessionData, label string) bool {
	for _, l := range session.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// getSessionStatus determines the status of a session
func (t *ListSessionsTool) getSessionStatus(session *SessionData) string {
	// Use the status field directly
	if session.Status != "" {
		return session.Status
	}

	// Default status
	return SessionStatusActive
}

// sortSessions sorts sessions based on the specified field and order
func (t *ListSessionsTool) sortSessions(sessions []*SessionData, sortBy, sortOrder string) {
	// Simple bubble sort for demonstration (in production, use sort.Slice)
	n := len(sessions)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			shouldSwap := false

			switch sortBy {
			case "created":
				if sortOrder == SessionSortOrderAsc {
					shouldSwap = sessions[j].CreatedAt.After(sessions[j+1].CreatedAt)
				} else {
					shouldSwap = sessions[j].CreatedAt.Before(sessions[j+1].CreatedAt)
				}
			case "updated":
				if sortOrder == SessionSortOrderAsc {
					shouldSwap = sessions[j].UpdatedAt.After(sessions[j+1].UpdatedAt)
				} else {
					shouldSwap = sessions[j].UpdatedAt.Before(sessions[j+1].UpdatedAt)
				}
			case "disk_usage":
				if sortOrder == SessionSortOrderAsc {
					shouldSwap = sessions[j].DiskUsage > sessions[j+1].DiskUsage
				} else {
					shouldSwap = sessions[j].DiskUsage < sessions[j+1].DiskUsage
				}
			case "labels":
				// Sort by number of labels
				labelCount1 := len(sessions[j].Labels)
				labelCount2 := len(sessions[j+1].Labels)
				if sortOrder == SessionSortOrderAsc {
					shouldSwap = labelCount1 > labelCount2
				} else {
					shouldSwap = labelCount1 < labelCount2
				}
			}

			if shouldSwap {
				sessions[j], sessions[j+1] = sessions[j+1], sessions[j]
			}
		}
	}
}

// GetMetadata returns comprehensive metadata about the list sessions tool
func (t *ListSessionsTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:        "list_sessions",
		Description: "List and filter active sessions with detailed statistics and sorting options",
		Version:     "1.0.0",
		Category:    "Session Management",
		Dependencies: []string{
			"Session Manager",
			"Session Storage",
		},
		Capabilities: []string{
			"Session enumeration",
			"Multi-criteria filtering",
			"Flexible sorting",
			"Status-based filtering",
			"Label-based filtering",
			"Repository filtering",
			"Statistics reporting",
		},
		Requirements: []string{
			"Session manager instance",
			"Session storage access",
		},
	}
}

// Validate checks if the provided arguments are valid for the list sessions tool
func (t *ListSessionsTool) Validate(ctx context.Context, args interface{}) error {
	listArgs, ok := args.(ListSessionsArgs)
	if !ok {
		return errors.NewError().Messagef("invalid arguments type: expected ListSessionsArgs, got %T", args).WithLocation(

		// Validate status filter
		).Build()
	}

	if listArgs.Status != "" {
		validStatuses := map[string]bool{
			"active":  true,
			"expired": true,
			"all":     true,
			"idle":    true,
		}
		if !validStatuses[listArgs.Status] {
			return errors.NewError().Messagef("invalid status filter: %s (valid values: active, expired, idle, all)", listArgs.Status).WithLocation(

			// Validate limit
			).Build()
		}
	}

	if listArgs.Limit < 0 {
		return errors.NewError().Messagef("limit cannot be negative").Build()
	}
	if listArgs.Limit > 1000 {
		return errors.NewError().Messagef("limit cannot exceed 1000").WithLocation(

		// Validate sort_by
		).Build()
	}

	if listArgs.SortBy != "" {
		validSortFields := map[string]bool{
			"created":    true,
			"updated":    true,
			"disk_usage": true,
			"labels":     true,
		}
		if !validSortFields[listArgs.SortBy] {
			return errors.NewError().Messagef("invalid sort_by field: %s (valid values: created, updated, disk_usage, labels)", listArgs.SortBy).WithLocation(

			// Validate sort_order
			).Build()
		}
	}

	if listArgs.SortOrder != "" {
		if listArgs.SortOrder != "asc" && listArgs.SortOrder != "desc" {
			return errors.NewError().Messagef("invalid sort_order: %s (valid values: asc, desc)", listArgs.SortOrder).WithLocation(

			// Validate repository URL format
			).Build()
		}
	}

	if listArgs.RepoURL != "" {
		if len(listArgs.RepoURL) < 10 || len(listArgs.RepoURL) > 500 {
			return errors.NewError().Messagef("repo_url length must be between 10 and 500 characters").WithLocation(

			// Validate session manager is available
			).Build()
		}
	}

	if t.sessionManager == nil {
		return errors.NewError().Messagef("session manager is not configured").Build()
	}

	return nil
}
