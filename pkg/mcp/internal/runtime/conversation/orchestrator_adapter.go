package conversation

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
)

// OrchestratorAdapter adapts the old MCPToolOrchestrator interface to the new ToolOrchestrationExecutor interface
type OrchestratorAdapter struct {
	orchestrator *orchestration.MCPToolOrchestrator
}

// NewOrchestratorAdapter creates a new adapter
func NewOrchestratorAdapter(orchestrator *orchestration.MCPToolOrchestrator) *OrchestratorAdapter {
	return &OrchestratorAdapter{
		orchestrator: orchestrator,
	}
}

// ExecuteTool implements the new ToolOrchestrationExecutor interface
func (a *OrchestratorAdapter) ExecuteTool(ctx context.Context, request mcp.ToolExecutionRequest) (*mcp.ToolExecutionResult, error) {
	// Extract session ID from metadata or args
	var sessionID interface{}
	if request.Metadata != nil {
		if sid, ok := request.Metadata["session_id"]; ok {
			sessionID = sid
		}
	}
	if sessionID == nil && request.Args != nil {
		if sid, ok := request.Args["session_id"]; ok {
			sessionID = sid
		}
	}

	// Call the old interface
	result, err := a.orchestrator.ExecuteTool(ctx, request.ToolName, request.Args, sessionID)

	// Wrap the result in ToolExecutionResult
	return &mcp.ToolExecutionResult{
		Result:   result,
		Error:    err,
		Metadata: make(map[string]interface{}),
	}, err
}

// GetDispatcher returns the underlying dispatcher
func (a *OrchestratorAdapter) GetDispatcher() mcp.ToolDispatcher {
	// This method might need to be implemented based on the actual needs
	return nil
}
