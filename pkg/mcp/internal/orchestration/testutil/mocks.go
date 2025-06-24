package testutil

import (
	"context"
	"sync"
	"time"
)

// MockToolOrchestrator provides a controllable mock for testing tool execution
type MockToolOrchestrator struct {
	mu sync.RWMutex

	// Configuration
	ExecuteFunc    func(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error)
	ValidateFunc   func(toolName string, args interface{}) error
	ExecutionDelay time.Duration
	ShouldFail     bool
	FailureError   error

	// Execution tracking
	ExecutionHistory []ExecutionRecord
	ValidationCalls  []ValidationRecord

	// State tracking
	PipelineAdapter interface{}
}

// ExecutionRecord tracks a tool execution call
type ExecutionRecord struct {
	ToolName  string
	Args      interface{}
	Session   interface{}
	Timestamp time.Time
	Result    interface{}
	Error     error
	Duration  time.Duration
}

// ValidationRecord tracks a validation call
type ValidationRecord struct {
	ToolName  string
	Args      interface{}
	Timestamp time.Time
	Error     error
}

// NewMockToolOrchestrator creates a new mock orchestrator
func NewMockToolOrchestrator() *MockToolOrchestrator {
	return &MockToolOrchestrator{
		ExecutionHistory: make([]ExecutionRecord, 0),
		ValidationCalls:  make([]ValidationRecord, 0),
	}
}

// ExecuteTool implements the ToolOrchestrator interface
func (m *MockToolOrchestrator) ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	startTime := time.Now()

	// Apply execution delay if configured
	if m.ExecutionDelay > 0 {
		time.Sleep(m.ExecutionDelay)
	}

	var result interface{}
	var err error

	// Use custom execution function if provided
	if m.ExecuteFunc != nil {
		result, err = m.ExecuteFunc(ctx, toolName, args, session)
	} else if m.ShouldFail {
		// Return configured failure
		if m.FailureError != nil {
			err = m.FailureError
		} else {
			err = NewMockError("mock execution failed")
		}
	} else {
		// Default successful execution
		result = map[string]interface{}{
			"tool":     toolName,
			"success":  true,
			"mock":     true,
			"executed": true,
		}
	}

	// Record execution
	record := ExecutionRecord{
		ToolName:  toolName,
		Args:      args,
		Session:   session,
		Timestamp: startTime,
		Result:    result,
		Error:     err,
		Duration:  time.Since(startTime),
	}
	m.ExecutionHistory = append(m.ExecutionHistory, record)

	return result, err
}

// ValidateToolArgs implements the ToolOrchestrator interface
func (m *MockToolOrchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error

	// Use custom validation function if provided
	if m.ValidateFunc != nil {
		err = m.ValidateFunc(toolName, args)
	}
	// Otherwise, validation succeeds by default

	// Record validation call
	record := ValidationRecord{
		ToolName:  toolName,
		Args:      args,
		Timestamp: time.Now(),
		Error:     err,
	}
	m.ValidationCalls = append(m.ValidationCalls, record)

	return err
}

// SetPipelineAdapter implements dependency injection for testing
func (m *MockToolOrchestrator) SetPipelineAdapter(adapter interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PipelineAdapter = adapter
}

// Test utility methods

// GetExecutionCount returns the number of tool executions
func (m *MockToolOrchestrator) GetExecutionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.ExecutionHistory)
}

// GetExecutionCountForTool returns executions for a specific tool
func (m *MockToolOrchestrator) GetExecutionCountForTool(toolName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, record := range m.ExecutionHistory {
		if record.ToolName == toolName {
			count++
		}
	}
	return count
}

// GetLastExecution returns the most recent execution record
func (m *MockToolOrchestrator) GetLastExecution() *ExecutionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.ExecutionHistory) == 0 {
		return nil
	}
	record := m.ExecutionHistory[len(m.ExecutionHistory)-1]
	return &record
}

// GetExecutionsForTool returns all executions for a specific tool
func (m *MockToolOrchestrator) GetExecutionsForTool(toolName string) []ExecutionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var records []ExecutionRecord
	for _, record := range m.ExecutionHistory {
		if record.ToolName == toolName {
			records = append(records, record)
		}
	}
	return records
}

// Clear resets the mock state
func (m *MockToolOrchestrator) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ExecutionHistory = make([]ExecutionRecord, 0)
	m.ValidationCalls = make([]ValidationRecord, 0)
}

// MockToolRegistry provides a controllable mock for testing tool registration
type MockToolRegistry struct {
	mu sync.RWMutex

	// Configuration
	RegisterFunc  func(name string, tool interface{}) error
	GetToolFunc   func(name string) (interface{}, bool)
	ShouldFailReg bool
	FailureError  error

	// State tracking
	RegisteredTools   map[string]interface{}
	RegistrationCalls []RegistrationRecord
}

// RegistrationRecord tracks a tool registration call
type RegistrationRecord struct {
	ToolName  string
	Tool      interface{}
	Timestamp time.Time
	Error     error
}

// NewMockToolRegistry creates a new mock registry
func NewMockToolRegistry() *MockToolRegistry {
	return &MockToolRegistry{
		RegisteredTools:   make(map[string]interface{}),
		RegistrationCalls: make([]RegistrationRecord, 0),
	}
}

// RegisterTool implements the tool registry interface
func (m *MockToolRegistry) RegisterTool(name string, tool interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error

	// Use custom registration function if provided
	if m.RegisterFunc != nil {
		err = m.RegisterFunc(name, tool)
	} else if m.ShouldFailReg {
		// Return configured failure
		if m.FailureError != nil {
			err = m.FailureError
		} else {
			err = NewMockError("mock registration failed")
		}
	} else {
		// Default successful registration
		m.RegisteredTools[name] = tool
	}

	// Record registration call
	record := RegistrationRecord{
		ToolName:  name,
		Tool:      tool,
		Timestamp: time.Now(),
		Error:     err,
	}
	m.RegistrationCalls = append(m.RegistrationCalls, record)

	return err
}

// GetTool implements the tool registry interface
func (m *MockToolRegistry) GetTool(name string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use custom get function if provided
	if m.GetToolFunc != nil {
		return m.GetToolFunc(name)
	}

	// Default behavior
	tool, exists := m.RegisteredTools[name]
	return tool, exists
}

// Test utility methods for registry

// GetRegistrationCount returns the number of tool registrations
func (m *MockToolRegistry) GetRegistrationCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.RegistrationCalls)
}

// IsToolRegistered checks if a tool is registered
func (m *MockToolRegistry) IsToolRegistered(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.RegisteredTools[name]
	return exists
}

// GetRegisteredToolNames returns all registered tool names
func (m *MockToolRegistry) GetRegisteredToolNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.RegisteredTools))
	for name := range m.RegisteredTools {
		names = append(names, name)
	}
	return names
}

// Clear resets the registry state
func (m *MockToolRegistry) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RegisteredTools = make(map[string]interface{})
	m.RegistrationCalls = make([]RegistrationRecord, 0)
}

// MockToolFactory provides a controllable mock for testing tool creation
type MockToolFactory struct {
	mu sync.RWMutex

	// Configuration
	CreateFunc   func(toolName string) (interface{}, error)
	ShouldFail   bool
	FailureError error

	// State tracking
	CreationCalls []CreationRecord
	CreatedTools  map[string]interface{}
}

// CreationRecord tracks a tool creation call
type CreationRecord struct {
	ToolName  string
	Timestamp time.Time
	Tool      interface{}
	Error     error
}

// NewMockToolFactory creates a new mock factory
func NewMockToolFactory() *MockToolFactory {
	return &MockToolFactory{
		CreationCalls: make([]CreationRecord, 0),
		CreatedTools:  make(map[string]interface{}),
	}
}

// CreateTool implements the factory interface
func (m *MockToolFactory) CreateTool(toolName string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var tool interface{}
	var err error

	// Use custom creation function if provided
	if m.CreateFunc != nil {
		tool, err = m.CreateFunc(toolName)
	} else if m.ShouldFail {
		// Return configured failure
		if m.FailureError != nil {
			err = m.FailureError
		} else {
			err = NewMockError("mock tool creation failed")
		}
	} else {
		// Default successful creation
		tool = &MockTool{
			Name:    toolName,
			Created: time.Now(),
		}
		m.CreatedTools[toolName] = tool
	}

	// Record creation call
	record := CreationRecord{
		ToolName:  toolName,
		Timestamp: time.Now(),
		Tool:      tool,
		Error:     err,
	}
	m.CreationCalls = append(m.CreationCalls, record)

	return tool, err
}

// Test utility methods for factory

// GetCreationCount returns the number of tool creations
func (m *MockToolFactory) GetCreationCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.CreationCalls)
}

// GetCreationCountForTool returns creations for a specific tool
func (m *MockToolFactory) GetCreationCountForTool(toolName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, record := range m.CreationCalls {
		if record.ToolName == toolName {
			count++
		}
	}
	return count
}

// Clear resets the factory state
func (m *MockToolFactory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreationCalls = make([]CreationRecord, 0)
	m.CreatedTools = make(map[string]interface{})
}

// MockTool represents a mock tool implementation
type MockTool struct {
	Name    string
	Created time.Time
}

// MockError represents a mock error for testing
type MockError struct {
	Message string
}

func (e *MockError) Error() string {
	return e.Message
}

// NewMockError creates a new mock error
func NewMockError(message string) *MockError {
	return &MockError{Message: message}
}

// TestToolBuilder provides a builder pattern for creating test tools
type TestToolBuilder struct {
	toolName     string
	executeFunc  func(ctx context.Context, args interface{}) (interface{}, error)
	validateFunc func(args interface{}) error
}

// NewTestToolBuilder creates a new test tool builder
func NewTestToolBuilder(toolName string) *TestToolBuilder {
	return &TestToolBuilder{
		toolName: toolName,
	}
}

// WithExecuteFunc sets the execute function
func (b *TestToolBuilder) WithExecuteFunc(fn func(ctx context.Context, args interface{}) (interface{}, error)) *TestToolBuilder {
	b.executeFunc = fn
	return b
}

// WithValidateFunc sets the validate function
func (b *TestToolBuilder) WithValidateFunc(fn func(args interface{}) error) *TestToolBuilder {
	b.validateFunc = fn
	return b
}

// Build creates the test tool
func (b *TestToolBuilder) Build() *TestTool {
	return &TestTool{
		name:         b.toolName,
		executeFunc:  b.executeFunc,
		validateFunc: b.validateFunc,
	}
}

// TestTool provides a configurable tool for testing
type TestTool struct {
	name         string
	executeFunc  func(ctx context.Context, args interface{}) (interface{}, error)
	validateFunc func(args interface{}) error
	executions   []TestExecution
	mu           sync.RWMutex
}

// TestExecution tracks a test tool execution
type TestExecution struct {
	Args      interface{}
	Result    interface{}
	Error     error
	Timestamp time.Time
	Duration  time.Duration
}

// Execute implements the tool execution interface
func (t *TestTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	startTime := time.Now()
	var result interface{}
	var err error

	if t.executeFunc != nil {
		result, err = t.executeFunc(ctx, args)
	} else {
		// Default execution
		result = map[string]interface{}{
			"tool":      t.name,
			"executed":  true,
			"timestamp": startTime,
		}
	}

	// Record execution
	execution := TestExecution{
		Args:      args,
		Result:    result,
		Error:     err,
		Timestamp: startTime,
		Duration:  time.Since(startTime),
	}
	t.executions = append(t.executions, execution)

	return result, err
}

// Validate implements the tool validation interface
func (t *TestTool) Validate(args interface{}) error {
	if t.validateFunc != nil {
		return t.validateFunc(args)
	}
	return nil // Default validation succeeds
}

// GetExecutionCount returns the number of executions
func (t *TestTool) GetExecutionCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.executions)
}

// GetExecutions returns all executions
func (t *TestTool) GetExecutions() []TestExecution {
	t.mu.RLock()
	defer t.mu.RUnlock()

	executions := make([]TestExecution, len(t.executions))
	copy(executions, t.executions)
	return executions
}

// Clear resets the tool state
func (t *TestTool) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.executions = make([]TestExecution, 0)
}
