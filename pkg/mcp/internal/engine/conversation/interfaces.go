package conversation

import (
	"context"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
)

// ToolOrchestrator defines the interface for tool execution
type ToolOrchestrator interface {
	ExecuteTool(ctx context.Context, toolName string, args interface{}, sessionID string) (*ToolResult, error)
}

// ToolResult is imported from contract package
type ToolResult = contract.ToolResult

// RetryManager manages retry logic
type RetryManager interface {
	ShouldRetry(err error, attempt int) bool
	GetBackoff(attempt int) time.Duration
}
