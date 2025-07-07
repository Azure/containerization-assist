package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
)

// SessionManager manages MCP sessions
// Deprecated: This implementation will be replaced by UnifiedSessionManager in v2.0.0.
// For new code, use session.UnifiedSessionManager for type-safe, context-aware session management.
// For migration guidance, see docs/migration/SESSION_MANAGER_MIGRATION_GUIDE.md
type SessionManager struct {
	sessions     map[string]*SessionState
	workspaceDir string
	maxSessions  int
	sessionTTL   time.Duration
	store        SessionStore
	logger       *slog.Logger
	mu           sync.RWMutex
}

// SessionManagerConfig represents session manager configuration
type SessionManagerConfig struct {
	WorkspaceDir      string
	MaxSessions       int
	SessionTTL        time.Duration
	MaxDiskPerSession int64
	TotalDiskLimit    int64
	StorePath         string
	Logger            *slog.Logger
}

// UnifiedSessionManager defines the new unified session management interface
type UnifiedSessionManager interface {
	// Core session operations
	CreateSession(ctx context.Context, id string) (*SessionState, error)
	GetOrCreateSession(ctx context.Context, sessionID string) (*SessionState, error)
	GetSession(ctx context.Context, sessionID string) (*SessionState, error)
	UpdateSession(ctx context.Context, sessionID string, updater func(*SessionState) error) error
	DeleteSession(ctx context.Context, sessionID string) error

	// Session queries
	ListSessions(ctx context.Context) ([]*SessionData, error)
	ListSessionSummaries(ctx context.Context) ([]SessionSummary, error)
	GetSessionData(ctx context.Context, sessionID string) (*SessionData, error)

	// Session management
	SaveSession(ctx context.Context, sessionID string, session *SessionState) error
	GarbageCollect(ctx context.Context) error
	GetStats(ctx context.Context) (*core.SessionManagerStats, error)

	// Session labeling
	AddSessionLabel(ctx context.Context, sessionID, label string) error
	RemoveSessionLabel(ctx context.Context, sessionID, label string) error
	GetSessionsByLabel(ctx context.Context, label string) ([]*SessionData, error)

	// Workflow session management
	GetWorkflowSession(ctx context.Context, workflowID string) (*WorkflowSession, error)
	UpdateWorkflowSession(ctx context.Context, workflowSession *WorkflowSession) error

	// Lifecycle
	Close() error
}

// ManagerInterface removed as part of EPSILON workstream.
// Use services.SessionStore and services.SessionState interfaces instead
// for focused session management functionality.

// SessionData represents session data for API responses
type SessionData struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	WorkspaceDir string                 `json:"workspace_dir"`
	Metadata     map[string]interface{} `json:"metadata"`
	Status       string                 `json:"status"`
	Labels       []string               `json:"labels,omitempty"`
	DiskUsage    int64                  `json:"disk_usage"`
}

// ToCoreSessionState converts SessionData to core.SessionState
func (sd *SessionData) ToCoreSessionState() *core.SessionState {
	return &core.SessionState{
		SessionID:    sd.ID,
		CreatedAt:    sd.CreatedAt,
		UpdatedAt:    sd.UpdatedAt,
		WorkspaceDir: sd.WorkspaceDir,
		Status:       sd.Status,
	}
}

// SessionSummary provides a summary of a session
type SessionSummary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    string    `json:"status"`
	Labels    []string  `json:"labels,omitempty"`
}

// SessionStore defines the interface for session persistence
type SessionStore interface {
	Save(ctx context.Context, sessionID string, session *SessionState) error
	Load(ctx context.Context, sessionID string) (*SessionState, error)
	Delete(ctx context.Context, sessionID string) error
	List(ctx context.Context) ([]string, error)
	Close() error
}

// Constants for session operations
const (
	DefaultSessionTTL        = 24 * time.Hour
	DefaultMaxSessions       = 100
	DefaultMaxDiskPerSession = 1024 * 1024 * 1024      // 1GB
	DefaultTotalDiskLimit    = 10 * 1024 * 1024 * 1024 // 10GB
)
