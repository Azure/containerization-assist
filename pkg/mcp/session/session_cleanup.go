package session

import (
	"context"
	"os"
	"path/filepath"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// GarbageCollect implements UnifiedSessionManager interface
func (sm *SessionManager) GarbageCollect(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.garbageCollectUnsafe()
}

// GarbageCollectLegacy performs garbage collection (legacy interface)
func (sm *SessionManager) GarbageCollectLegacy() error {
	return sm.GarbageCollect(context.Background())
}

// garbageCollectUnsafe performs garbage collection without acquiring the mutex
func (sm *SessionManager) garbageCollectUnsafe() error {
	if sm.sessionTTL <= 0 {
		return nil // No TTL configured
	}

	now := time.Now()
	expiredSessions := []string{}

	// Identify expired sessions
	for id, session := range sm.sessions {
		if now.Sub(session.UpdatedAt) > sm.sessionTTL {
			expiredSessions = append(expiredSessions, id)
		}
	}

	// Delete expired sessions
	for _, sessionID := range expiredSessions {
		if err := sm.deleteSessionInternal(sessionID); err != nil {
			sm.logger.Warn("Failed to delete expired session", "error", err, "session_id", sessionID)
		} else {
			sm.logger.Info("Deleted expired session", "session_id", sessionID)
		}
	}

	sm.logger.Info("Garbage collection completed", "expired_count", len(expiredSessions))
	return nil
}

// StartCleanupRoutine starts the background cleanup routine
func (sm *SessionManager) StartCleanupRoutine() {
	go sm.cleanupRoutine()
}

// cleanupRoutine runs periodic cleanup tasks
func (sm *SessionManager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.logger.Debug("Running periodic cleanup")

		// Garbage collect expired sessions
		if err := sm.GarbageCollect(context.Background()); err != nil {
			sm.logger.Warn("Failed to garbage collect sessions", "error", err)
		}

		// Update disk usage for all sessions
		if err := sm.updateAllDiskUsage(); err != nil {
			sm.logger.Warn("Failed to update disk usage", "error", err)
		}

		// Check total disk usage
		if err := sm.checkTotalDiskUsage(); err != nil {
			sm.logger.Warn("Failed to check total disk usage", "error", err)
		}
	}
}

// updateAllDiskUsage updates disk usage for all sessions
func (sm *SessionManager) updateAllDiskUsage() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, session := range sm.sessions {
		usage, err := calculateDiskUsage(session.WorkspaceDir)
		if err != nil {
			sm.logger.Warn("Failed to calculate disk usage", "error", err, "session_id", session.ID)
			continue
		}
		session.DiskUsage = usage
	}

	return nil
}

// calculateDiskUsage calculates the total disk usage of a directory
func calculateDiskUsage(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// checkTotalDiskUsage checks if total disk usage exceeds limits
func (sm *SessionManager) checkTotalDiskUsage() error {
	stats, err := sm.GetStats(context.Background())
	if err != nil {
		return err
	}

	// Log warning if approaching limits
	if stats.TotalDiskUsage > DefaultTotalDiskLimit*0.8 {
		sm.logger.Warn("Total disk usage approaching limit",
			"total_usage", stats.TotalDiskUsage,
			"limit", DefaultTotalDiskLimit,
			"percentage", float64(stats.TotalDiskUsage)/float64(DefaultTotalDiskLimit)*100)
	}

	return nil
}

// CleanupSession performs cleanup for a specific session
func (sm *SessionManager) CleanupSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	// Clean workspace directory contents (but keep the directory)
	entries, err := os.ReadDir(session.WorkspaceDir)
	if err != nil {
		return errors.NewError().Message("failed to read workspace directory").Cause(err).Build()
	}

	for _, entry := range entries {
		entryPath := filepath.Join(session.WorkspaceDir, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			sm.logger.Warn("Failed to remove workspace entry", "error", err, "path", entryPath)
		}
	}

	// Update session metadata
	session.UpdatedAt = time.Now()
	session.DiskUsage = 0
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["last_cleanup"] = time.Now()

	// Save to persistent store
	if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
		sm.logger.Warn("Failed to save cleaned session", "error", err, "session_id", sessionID)
	}

	sm.logger.Info("Session workspace cleaned", "session_id", sessionID)
	return nil
}

// PurgeExpiredSessions removes all expired sessions
func (sm *SessionManager) PurgeExpiredSessions() (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.sessionTTL <= 0 {
		return 0, nil // No TTL configured
	}

	now := time.Now()
	purgedCount := 0

	// Collect expired session IDs
	expiredIDs := []string{}
	for id, session := range sm.sessions {
		if now.Sub(session.UpdatedAt) > sm.sessionTTL {
			expiredIDs = append(expiredIDs, id)
		}
	}

	// Delete expired sessions
	for _, id := range expiredIDs {
		if err := sm.deleteSessionInternal(id); err != nil {
			sm.logger.Warn("Failed to purge expired session", "error", err, "session_id", id)
		} else {
			purgedCount++
		}
	}

	return purgedCount, nil
}
