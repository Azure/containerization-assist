// Package workflow provides progress manager constructor
package workflow

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
)

// Manager defines the interface all progress managers must implement
type Manager interface {
	Begin(msg string)
	Update(step int, msg string, metadata map[string]interface{})
	Complete(msg string)
	Finish()

	// State getters
	GetCurrent() int
	SetCurrent(current int)
	GetTotal() int
	IsComplete() bool
	GetTraceID() string

	// Error budget methods
	RecordError(err error) bool
	RecordSuccess()
	IsCircuitOpen() bool
	GetErrorBudgetStatus() ErrorBudgetStatus
	UpdateWithErrorHandling(step int, msg string, metadata map[string]interface{}, err error) bool
}

// NewManager creates a new progress manager
func NewManager(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) Manager {
	return NewChannelManager(ctx, req, totalSteps, logger)
}
