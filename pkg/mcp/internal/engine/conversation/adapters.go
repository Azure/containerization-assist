package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
)

// ToolResult represents the result of a tool execution in the conversation engine
type ToolResult struct {
	CallID        string        `json:"call_id"`
	CorrelationID string        `json:"correlation_id"`
	ToolName      string        `json:"tool_name"`
	Success       bool          `json:"success"`
	Result        interface{}   `json:"result,omitempty"`
	Error         string        `json:"error,omitempty"`
	ExecutionTime time.Duration `json:"execution_time"`
	Timestamp     time.Time     `json:"timestamp"`
}

// IsSuccess implements the mcptypes.ToolResult interface
func (tr *ToolResult) IsSuccess() bool {
	return tr.Success
}

// GetError implements the mcptypes.ToolResult interface
func (tr *ToolResult) GetError() error {
	if tr.Error == "" {
		return nil
	}
	return fmt.Errorf("%s", tr.Error)
}

// ToolOrchestrator defines the interface expected by the conversation engine
type ToolOrchestrator interface {
	ExecuteTool(ctx context.Context, toolName string, args interface{}, sessionID string) (*ToolResult, error)
}

// sessionManagerAdapter adapts the conversation session manager to orchestration.SessionManager interface
type sessionManagerAdapter struct {
	sessionManager *session.SessionManager
}

func (s *sessionManagerAdapter) GetSession(sessionID string) (interface{}, error) {
	return s.sessionManager.GetSession(sessionID)
}

func (s *sessionManagerAdapter) UpdateSession(session interface{}) error {
	// Type assert to get the session state
	sessionState, ok := session.(*sessiontypes.SessionState)
	if !ok {
		return fmt.Errorf("invalid session type: expected *sessiontypes.SessionState, got %T", session)
	}

	// Extract session ID and update using the underlying session manager
	if sessionState.SessionID == "" {
		return fmt.Errorf("session ID is required for update")
	}

	// Update the session by replacing the entire state
	return s.sessionManager.UpdateSession(sessionState.SessionID, func(existing *sessiontypes.SessionState) {
		// Copy all fields from the provided session to the existing one
		*existing = *sessionState
	})
}

// modernOrchestratorAdapter adapts MCPToolOrchestrator to the conversation ToolOrchestrator interface
type modernOrchestratorAdapter struct {
	orchestrator *orchestration.MCPToolOrchestrator
}

func (m *modernOrchestratorAdapter) ExecuteTool(
	ctx context.Context,
	toolName string,
	args interface{},
	sessionID string,
) (*ToolResult, error) {
	// Execute using the modern orchestrator
	result, err := m.orchestrator.ExecuteTool(ctx, toolName, args, sessionID)
	if err != nil {
		return &ToolResult{
			CallID:        "error-" + toolName,
			CorrelationID: "",
			ToolName:      toolName,
			Success:       false,
			Error:         err.Error(),
			ExecutionTime: 0,
			Timestamp:     time.Now(),
		}, err
	}

	// Convert result to ToolResult format
	return &ToolResult{
		CallID:        "call-" + toolName,
		CorrelationID: "",
		ToolName:      toolName,
		Success:       true,
		Result:        result,
		ExecutionTime: 100 * time.Millisecond, // Placeholder
		Timestamp:     time.Now(),
	}, nil
}
