package session

import (
	"sort"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// GetDetailedStats returns detailed session statistics
func (sm *SessionManager) GetDetailedStats() *DetailedSessionStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := &DetailedSessionStats{
		TotalSessions:     len(sm.sessions),
		ActiveSessions:    0,
		InactiveSessions:  0,
		ExpiredSessions:   0,
		TotalDiskUsage:    0,
		AverageDiskUsage:  0,
		MaxDiskUsage:      0,
		MinDiskUsage:      int64(^uint64(0) >> 1), // Max int64
		SessionsByStatus:  make(map[string]int),
		SessionsByLabel:   make(map[string]int),
		AgeDistribution:   make(map[string]int),
		DiskDistribution:  make(map[string]int),
		OldestSessionAge:  0,
		NewestSessionAge:  time.Duration(^uint64(0) >> 1), // Max duration
		AverageSessionAge: 0,
	}

	if len(sm.sessions) == 0 {
		stats.MinDiskUsage = 0
		stats.NewestSessionAge = 0
		return stats
	}

	now := time.Now()
	var totalAge time.Duration

	for _, session := range sm.sessions {
		// Disk usage statistics
		stats.TotalDiskUsage += session.DiskUsage
		if session.DiskUsage > stats.MaxDiskUsage {
			stats.MaxDiskUsage = session.DiskUsage
		}
		if session.DiskUsage < stats.MinDiskUsage {
			stats.MinDiskUsage = session.DiskUsage
		}

		// Status statistics
		stats.SessionsByStatus[session.Status]++
		switch session.Status {
		case SessionStatusActive:
			stats.ActiveSessions++
		case SessionStatusInactive:
			stats.InactiveSessions++
		case SessionStatusExpired:
			stats.ExpiredSessions++
		}

		// Check if expired by TTL
		age := now.Sub(session.CreatedAt)
		if sm.sessionTTL > 0 && now.Sub(session.UpdatedAt) > sm.sessionTTL {
			stats.ExpiredSessions++
		}

		// Age statistics
		totalAge += age
		if age > stats.OldestSessionAge {
			stats.OldestSessionAge = age
		}
		if age < stats.NewestSessionAge {
			stats.NewestSessionAge = age
		}

		// Age distribution
		ageCategory := categorizeAge(age)
		stats.AgeDistribution[ageCategory]++

		// Disk distribution
		diskCategory := categorizeDiskUsage(session.DiskUsage)
		stats.DiskDistribution[diskCategory]++

		// Label statistics
		for _, label := range session.Labels {
			stats.SessionsByLabel[label]++
		}
	}

	// Calculate averages
	if len(sm.sessions) > 0 {
		stats.AverageDiskUsage = stats.TotalDiskUsage / int64(len(sm.sessions))
		stats.AverageSessionAge = totalAge / time.Duration(len(sm.sessions))
	}

	return stats
}

// GetSessionHistory returns session activity history
func (sm *SessionManager) GetSessionHistory(sessionID string, limit int) ([]SessionEvent, error) {
	// This would typically query from an event store
	// For now, return basic history from metadata
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	events := []SessionEvent{
		{
			Type:      "created",
			Timestamp: session.CreatedAt,
			Details:   map[string]interface{}{"session_id": sessionID},
		},
	}

	// Add last update event if different from creation
	if session.UpdatedAt.After(session.CreatedAt) {
		events = append(events, SessionEvent{
			Type:      "updated",
			Timestamp: session.UpdatedAt,
			Details:   map[string]interface{}{"session_id": sessionID},
		})
	}

	// Add cleanup event if exists in metadata
	if cleanupTime, ok := session.Metadata["last_cleanup"].(time.Time); ok {
		events = append(events, SessionEvent{
			Type:      "cleaned",
			Timestamp: cleanupTime,
			Details:   map[string]interface{}{"session_id": sessionID},
		})
	}

	// Sort by timestamp, newest first
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

// GetTopSessionsByDiskUsage returns sessions with highest disk usage
func (sm *SessionManager) GetTopSessionsByDiskUsage(limit int) []*SessionData {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Convert to slice for sorting
	sessions := make([]*SessionData, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, &SessionData{
			ID:           session.ID,
			CreatedAt:    session.CreatedAt,
			UpdatedAt:    session.UpdatedAt,
			WorkspaceDir: session.WorkspaceDir,
			Metadata:     session.Metadata,
			Status:       session.Status,
			Labels:       session.Labels,
			DiskUsage:    session.DiskUsage,
		})
	}

	// Sort by disk usage, highest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].DiskUsage > sessions[j].DiskUsage
	})

	// Apply limit
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions
}

// GetOldestSessions returns the oldest sessions
func (sm *SessionManager) GetOldestSessions(limit int) []*SessionData {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Convert to slice for sorting
	sessions := make([]*SessionData, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, &SessionData{
			ID:           session.ID,
			CreatedAt:    session.CreatedAt,
			UpdatedAt:    session.UpdatedAt,
			WorkspaceDir: session.WorkspaceDir,
			Metadata:     session.Metadata,
			Status:       session.Status,
			Labels:       session.Labels,
			DiskUsage:    session.DiskUsage,
		})
	}

	// Sort by creation time, oldest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
	})

	// Apply limit
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions
}

// Helper functions

func categorizeAge(age time.Duration) string {
	switch {
	case age < 1*time.Hour:
		return "< 1 hour"
	case age < 24*time.Hour:
		return "1-24 hours"
	case age < 7*24*time.Hour:
		return "1-7 days"
	case age < 30*24*time.Hour:
		return "7-30 days"
	default:
		return "> 30 days"
	}
}

func categorizeDiskUsage(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size < MB:
		return "< 1MB"
	case size < 10*MB:
		return "1-10MB"
	case size < 100*MB:
		return "10-100MB"
	case size < GB:
		return "100MB-1GB"
	default:
		return "> 1GB"
	}
}

// DetailedSessionStats provides comprehensive statistics
type DetailedSessionStats struct {
	// Basic counts
	TotalSessions    int
	ActiveSessions   int
	InactiveSessions int
	ExpiredSessions  int

	// Disk usage
	TotalDiskUsage   int64
	AverageDiskUsage int64
	MaxDiskUsage     int64
	MinDiskUsage     int64

	// Categorized counts
	SessionsByStatus map[string]int
	SessionsByLabel  map[string]int
	AgeDistribution  map[string]int
	DiskDistribution map[string]int

	// Age statistics
	OldestSessionAge  time.Duration
	NewestSessionAge  time.Duration
	AverageSessionAge time.Duration
}

// SessionEvent represents an event in session history
type SessionEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details"`
}
