package session

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// Session constants
const (
	SessionStatusActive  = "active"
	SessionStatusExpired = "expired"
	SessionSortOrderAsc  = "asc"
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

// ListSessionsManager interface for listing sessions
type ListSessionsManager interface {
	GetAllSessions() ([]*SessionData, error)
	GetSessionData(sessionID string) (*SessionData, error)
	GetStats() *core.SessionManagerStats
}

// SessionData represents the session data structure
type SessionData struct {
	ID             string
	State          interface{}
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      time.Time
	WorkspacePath  string
	DiskUsage      int64
	ActiveJobs     []string
	CompletedTools []string
	LastError      string
	Labels         []string
	RepoURL        string
	Metadata       map[string]string
}

// ListSessionsTool implements the list_sessions MCP tool
type ListSessionsTool struct {
	logger         zerolog.Logger
	sessionManager ListSessionsManager
}

// NewListSessionsTool creates a new list sessions tool
func NewListSessionsTool(logger zerolog.Logger, sessionManager ListSessionsManager) *ListSessionsTool {
	return &ListSessionsTool{
		logger:         logger,
		sessionManager: sessionManager,
	}
}

// Execute implements the unified Tool interface
func (t *ListSessionsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	listArgs, ok := args.(ListSessionsArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments type: expected ListSessionsArgs, got %T", args)
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
	sessions, err := t.sessionManager.GetAllSessions()
	if err != nil {
		return nil, errors.Wrapf(err, "list_sessions", "Failed to retrieve sessions from session manager")
	}

	// Get stats
	stats := t.sessionManager.GetStats()

	// Filter sessions
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
			ExpiresAt:      session.ExpiresAt,
			DiskUsage:      session.DiskUsage,
			WorkspacePath:  session.WorkspacePath,
			ActiveJobs:     len(session.ActiveJobs),
			CompletedTools: session.CompletedTools,
			LastError:      session.LastError,
			Labels:         session.Labels,
			RepoURL:        session.RepoURL,
			Metadata:       session.Metadata,
		}
		sessionInfos = append(sessionInfos, info)
	}

	// Get additional stats from the concrete session manager
	sm, ok := t.sessionManager.(*SessionManager)
	var uptime time.Duration
	var expiredCount int
	var totalDiskUsed int64

	if ok {
		uptime = time.Since(sm.startTime)
		// Calculate expired sessions
		for _, session := range sessionInfos {
			if session.Status == "expired" {
				expiredCount++
			}
		}
		// Calculate total disk usage
		sm.mutex.RLock()
		for _, usage := range sm.diskUsage {
			totalDiskUsed += usage
		}
		sm.mutex.RUnlock()
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
		TotalDiskUsed: totalDiskUsed,
		ServerUptime:  uptime.String(),
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
		Int64("total_disk_bytes", totalDiskUsed).
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

	// Check repository URL
	if args.RepoURL != "" && session.RepoURL != args.RepoURL {
		return false
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
	if time.Now().After(session.ExpiresAt) {
		return SessionStatusExpired
	}

	if len(session.ActiveJobs) > 0 {
		return SessionStatusActive
	}

	// Session is not expired and has no active jobs
	return "idle"
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
func (t *ListSessionsTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
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
		Parameters: map[string]string{
			"status":     "Optional: Filter by status (active, expired, all)",
			"labels":     "Optional: Sessions must have ALL these labels",
			"any_label":  "Optional: Sessions must have ANY of these labels",
			"repo_url":   "Optional: Filter by repository URL",
			"limit":      "Optional: Maximum sessions to return (default: 100)",
			"sort_by":    "Optional: Sort field (created, updated, disk_usage, labels)",
			"sort_order": "Optional: Sort order (asc, desc)",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "List all active sessions",
				Description: "Get all currently active sessions",
				Input: map[string]interface{}{
					"status": "active",
					"limit":  50,
				},
				Output: map[string]interface{}{
					"sessions": []map[string]interface{}{
						{
							"session_id":     "session-123",
							"status":         "active",
							"created_at":     "2024-12-17T10:00:00Z",
							"updated_at":     "2024-12-17T10:30:00Z",
							"disk_usage":     1024000,
							"workspace_path": "/workspaces/session-123",
							"active_jobs":    2,
							"labels":         []string{"development", "nodejs"},
						},
					},
					"total_sessions":  10,
					"active_count":    5,
					"expired_count":   3,
					"total_disk_used": 10240000,
					"server_uptime":   "24h30m",
				},
			},
			{
				Name:        "Filter by labels and repository",
				Description: "Find sessions with specific labels and repository",
				Input: map[string]interface{}{
					"labels":   []string{"production", "backend"},
					"repo_url": "https://github.com/company/api-service",
					"sort_by":  "updated",
				},
				Output: map[string]interface{}{
					"sessions": []map[string]interface{}{
						{
							"session_id":     "session-456",
							"status":         "active",
							"labels":         []string{"production", "backend", "api"},
							"repo_url":       "https://github.com/company/api-service",
							"workspace_path": "/workspaces/session-456",
						},
					},
					"total_sessions":  1,
					"active_count":    1,
					"expired_count":   0,
					"total_disk_used": 5120000,
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the list sessions tool
func (t *ListSessionsTool) Validate(ctx context.Context, args interface{}) error {
	listArgs, ok := args.(ListSessionsArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type: expected ListSessionsArgs, got %T", args)
	}

	// Validate status filter
	if listArgs.Status != "" {
		validStatuses := map[string]bool{
			"active":  true,
			"expired": true,
			"all":     true,
			"idle":    true,
		}
		if !validStatuses[listArgs.Status] {
			return fmt.Errorf("invalid status filter: %s (valid values: active, expired, idle, all)", listArgs.Status)
		}
	}

	// Validate limit
	if listArgs.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if listArgs.Limit > 1000 {
		return fmt.Errorf("limit cannot exceed 1000")
	}

	// Validate sort_by
	if listArgs.SortBy != "" {
		validSortFields := map[string]bool{
			"created":    true,
			"updated":    true,
			"disk_usage": true,
			"labels":     true,
		}
		if !validSortFields[listArgs.SortBy] {
			return fmt.Errorf("invalid sort_by field: %s (valid values: created, updated, disk_usage, labels)", listArgs.SortBy)
		}
	}

	// Validate sort_order
	if listArgs.SortOrder != "" {
		if listArgs.SortOrder != "asc" && listArgs.SortOrder != "desc" {
			return fmt.Errorf("invalid sort_order: %s (valid values: asc, desc)", listArgs.SortOrder)
		}
	}

	// Validate repository URL format
	if listArgs.RepoURL != "" {
		if len(listArgs.RepoURL) < 10 || len(listArgs.RepoURL) > 500 {
			return fmt.Errorf("repo_url length must be between 10 and 500 characters")
		}
	}

	// Validate session manager is available
	if t.sessionManager == nil {
		return fmt.Errorf("session manager is not configured")
	}

	return nil
}
