package session

// Package session provides session management functionality for the MCP server.
// This file acts as a unified interface that delegates to focused components:
//
// - session_types.go: Type definitions and interfaces
// - session_core.go: Core session operations (create, get, update, delete)
// - session_queries.go: Session listing and query operations
// - session_cleanup.go: Garbage collection and cleanup operations
// - session_labels.go: Label management operations
// - session_stats.go: Statistics and monitoring operations
//
// The SessionManager implements the UnifiedSessionManager interface providing
// type-safe, context-aware session management with persistent storage support.
//
// Example usage:
//
//	config := SessionManagerConfig{
//	    WorkspaceDir: "/var/mcp/workspaces",
//	    MaxSessions: 100,
//	    SessionTTL: 24 * time.Hour,
//	    StorePath: "/var/mcp/sessions.db",
//	    Logger: logger,
//	}
//
//	sm, err := NewSessionManager(config)
//	if err != nil {
//	    return err
//	}
//	defer sm.Close()
//
//	// Create or get a session
//	session, err := sm.GetOrCreateSession(ctx, "")
//	if err != nil {
//	    return err
//	}
//
//	// Update session metadata
//	err = sm.UpdateSession(ctx, session.ID, func(s *SessionState) error {
//	    s.Metadata["key"] = "value"
//	    return nil
//	})
//
// For detailed documentation and migration guidance, see:
// - docs/session-management.md
// - docs/migration/SESSION_MANAGER_MIGRATION_GUIDE.md
