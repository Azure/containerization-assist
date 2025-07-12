// Package progress provides unified progress reporting interfaces
package progress

import (
	"context"
)

// Reporter is the main interface for progress reporting
type Reporter interface {
	Begin(message string) error
	Update(step, total int, message string) error
	Complete(message string) error
	Close() error
}

// ReporterFactory creates the appropriate reporter based on context
type ReporterFactory func(ctx context.Context, totalSteps int) Reporter

// UpdateRequest represents a progress update
type UpdateRequest struct {
	Step     int
	Total    int
	Message  string
	Metadata *Metadata
}

// Reporter implementations should be created through factory functions
// This allows us to hide implementation details and switch between
// CLI, MCP, or other reporting mechanisms transparently
