package runtime

import (
	"context"
	"time"
)

type ToolProgressTracker struct {
	toolName  string
	sessionID string
	startTime time.Time
}

func NewToolProgressTracker(toolName, sessionID string) *ToolProgressTracker {
	return &ToolProgressTracker{
		toolName:  toolName,
		sessionID: sessionID,
		startTime: time.Now(),
	}
}
func (t *ToolProgressTracker) TrackProgress(ctx context.Context, operation string, progress float64) {
}
