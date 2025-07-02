package testutil

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
)

// MockToolOrchestrator provides a test implementation of tool orchestration
type MockToolOrchestrator struct {
	mu          sync.RWMutex
	executions  []MockExecution
	ExecuteFunc func(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error)
	logger      zerolog.Logger
}

// MockExecution represents a captured tool execution
type MockExecution struct {
	ToolName string
	Args     interface{}
	Session  interface{}
	Result   interface{}
	Error    error
}

// ExecutionCapture captures tool executions for testing
type ExecutionCapture struct {
	executions []MockExecution
	mu         sync.RWMutex
	logger     zerolog.Logger
}

// NewMockToolOrchestrator creates a new mock orchestrator
func NewMockToolOrchestrator() *MockToolOrchestrator {
	return &MockToolOrchestrator{
		executions: make([]MockExecution, 0),
	}
}

// NewExecutionCapture creates a new execution capture
func NewExecutionCapture(logger zerolog.Logger) *ExecutionCapture {
	return &ExecutionCapture{
		executions: make([]MockExecution, 0),
		logger:     logger,
	}
}

// ExecuteTool executes a tool through the mock orchestrator
func (m *MockToolOrchestrator) ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result interface{}
	var err error

	if m.ExecuteFunc != nil {
		result, err = m.ExecuteFunc(ctx, toolName, args, session)
	} else {
		// Default mock response
		result = map[string]interface{}{
			"tool":    toolName,
			"success": true,
			"mock":    true,
		}
	}

	// Record the execution
	execution := MockExecution{
		ToolName: toolName,
		Args:     args,
		Session:  session,
		Result:   result,
		Error:    err,
	}
	m.executions = append(m.executions, execution)

	return result, err
}

// GetExecutions returns all recorded executions
func (m *MockToolOrchestrator) GetExecutions() []MockExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	executions := make([]MockExecution, len(m.executions))
	copy(executions, m.executions)
	return executions
}

// Clear clears all recorded executions
func (m *MockToolOrchestrator) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = make([]MockExecution, 0)
}

// CaptureExecution captures a tool execution
func (e *ExecutionCapture) CaptureExecution(ctx context.Context, toolName string, args interface{}, sessionID string, fn func() (interface{}, error)) (interface{}, error) {
	result, err := fn()

	e.mu.Lock()
	defer e.mu.Unlock()

	execution := MockExecution{
		ToolName: toolName,
		Args:     args,
		Session:  sessionID,
		Result:   result,
		Error:    err,
	}
	e.executions = append(e.executions, execution)

	return result, err
}

// GetExecutions returns all captured executions
func (e *ExecutionCapture) GetExecutions() []MockExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	executions := make([]MockExecution, len(e.executions))
	copy(executions, e.executions)
	return executions
}

// GetExecutionsForTool returns all captured executions for a specific tool
func (e *ExecutionCapture) GetExecutionsForTool(toolName string) []MockExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var filtered []MockExecution
	for _, execution := range e.executions {
		if execution.ToolName == toolName {
			filtered = append(filtered, execution)
		}
	}
	return filtered
}
