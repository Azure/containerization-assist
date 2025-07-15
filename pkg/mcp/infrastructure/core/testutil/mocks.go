package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// MockStep implements workflow.Step for testing
type MockStep struct {
	NameFunc       func() string
	ExecuteFunc    func(ctx context.Context, state *workflow.WorkflowState) error
	MaxRetriesFunc func() int

	// Track execution
	ExecuteCount int
	mu           sync.Mutex
}

// NewMockStep creates a new mock step with sensible defaults
func NewMockStep(name string) *MockStep {
	return &MockStep{
		NameFunc: func() string { return name },
		ExecuteFunc: func(ctx context.Context, state *workflow.WorkflowState) error {
			return nil
		},
		MaxRetriesFunc: func() int { return 3 },
	}
}

// Name implements workflow.Step
func (m *MockStep) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-step"
}

// Execute implements workflow.Step
func (m *MockStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	m.mu.Lock()
	m.ExecuteCount++
	m.mu.Unlock()

	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, state)
	}
	return nil
}

// MaxRetries implements workflow.Step
func (m *MockStep) MaxRetries() int {
	if m.MaxRetriesFunc != nil {
		return m.MaxRetriesFunc()
	}
	return 0
}

// GetExecuteCount returns the number of times Execute was called
func (m *MockStep) GetExecuteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ExecuteCount
}

// Reset resets the execution count
func (m *MockStep) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ExecuteCount = 0
}

// MockStepWithError creates a mock step that returns an error
func MockStepWithError(name string, err error) *MockStep {
	return &MockStep{
		NameFunc: func() string { return name },
		ExecuteFunc: func(ctx context.Context, state *workflow.WorkflowState) error {
			return err
		},
	}
}

// MockStepWithDelay creates a mock step that delays before completing
func MockStepWithDelay(name string, delay time.Duration) *MockStep {
	return &MockStep{
		NameFunc: func() string { return name },
		ExecuteFunc: func(ctx context.Context, state *workflow.WorkflowState) error {
			select {
			case <-time.After(delay):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

// MockStepProvider implements workflow.StepProvider for testing
type MockStepProvider struct {
	Steps map[string]workflow.Step
	mu    sync.RWMutex
}

// NewMockStepProvider creates a new mock step provider
func NewMockStepProvider() *MockStepProvider {
	return &MockStepProvider{
		Steps: make(map[string]workflow.Step),
	}
}

// SetStep sets a step for a specific name
func (m *MockStepProvider) SetStep(name string, step workflow.Step) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Steps[name] = step
}

// GetAnalyzeStep implements workflow.StepProvider
func (m *MockStepProvider) GetAnalyzeStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["analyze"]; ok {
		return step
	}
	return NewMockStep("analyze")
}

// GetDockerfileStep implements workflow.StepProvider
func (m *MockStepProvider) GetDockerfileStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["dockerfile"]; ok {
		return step
	}
	return NewMockStep("dockerfile")
}

// GetBuildStep implements workflow.StepProvider
func (m *MockStepProvider) GetBuildStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["build"]; ok {
		return step
	}
	return NewMockStep("build")
}

// GetScanStep implements workflow.StepProvider
func (m *MockStepProvider) GetScanStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["scan"]; ok {
		return step
	}
	return NewMockStep("scan")
}

// GetTagStep implements workflow.StepProvider
func (m *MockStepProvider) GetTagStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["tag"]; ok {
		return step
	}
	return NewMockStep("tag")
}

// GetPushStep implements workflow.StepProvider
func (m *MockStepProvider) GetPushStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["push"]; ok {
		return step
	}
	return NewMockStep("push")
}

// GetManifestStep implements workflow.StepProvider
func (m *MockStepProvider) GetManifestStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["manifest"]; ok {
		return step
	}
	return NewMockStep("manifest")
}

// GetClusterStep implements workflow.StepProvider
func (m *MockStepProvider) GetClusterStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["cluster"]; ok {
		return step
	}
	return NewMockStep("cluster")
}

// GetDeployStep implements workflow.StepProvider
func (m *MockStepProvider) GetDeployStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["deploy"]; ok {
		return step
	}
	return NewMockStep("deploy")
}

// GetVerifyStep implements workflow.StepProvider
func (m *MockStepProvider) GetVerifyStep() workflow.Step {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if step, ok := m.Steps["verify"]; ok {
		return step
	}
	return NewMockStep("verify")
}

// MockProgressTracker implements progress tracking for tests
type MockProgressTracker struct {
	Updates []ProgressUpdate
	mu      sync.Mutex
}

// ProgressUpdate represents a captured progress update
type ProgressUpdate struct {
	Step     string
	Message  string
	Progress float64
	Error    error
	Time     time.Time
}

// NewMockProgressTracker creates a new mock progress tracker
func NewMockProgressTracker() *MockProgressTracker {
	return &MockProgressTracker{
		Updates: make([]ProgressUpdate, 0),
	}
}

// Update captures a progress update
func (m *MockProgressTracker) Update(step, message string, progress float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Updates = append(m.Updates, ProgressUpdate{
		Step:     step,
		Message:  message,
		Progress: progress,
		Time:     time.Now(),
	})
}

// Error captures an error update
func (m *MockProgressTracker) Error(step string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Updates = append(m.Updates, ProgressUpdate{
		Step:  step,
		Error: err,
		Time:  time.Now(),
	})
}

// GetUpdates returns all captured updates
func (m *MockProgressTracker) GetUpdates() []ProgressUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ProgressUpdate, len(m.Updates))
	copy(result, m.Updates)
	return result
}

// Reset clears all captured updates
func (m *MockProgressTracker) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Updates = m.Updates[:0]
}

// MockContext creates a context with common test values
func MockContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, workflow.WorkflowIDKey, "test-workflow-id")
	ctx = context.WithValue(ctx, workflow.TraceIDKey, "test-trace-id")
	return ctx
}

// MockContextWithTimeout creates a context with timeout and test values
func MockContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx := MockContext()
	return context.WithTimeout(ctx, timeout)
}

// StepExecutionRecorder records step executions for verification
type StepExecutionRecorder struct {
	executions []StepExecution
	mu         sync.Mutex
}

// StepExecution represents a recorded step execution
type StepExecution struct {
	StepName  string
	StartTime time.Time
	EndTime   time.Time
	Error     error
}

// NewStepExecutionRecorder creates a new execution recorder
func NewStepExecutionRecorder() *StepExecutionRecorder {
	return &StepExecutionRecorder{
		executions: make([]StepExecution, 0),
	}
}

// Record records a step execution
func (r *StepExecutionRecorder) Record(stepName string, fn func() error) error {
	r.mu.Lock()
	exec := StepExecution{
		StepName:  stepName,
		StartTime: time.Now(),
	}
	r.mu.Unlock()

	err := fn()

	r.mu.Lock()
	exec.EndTime = time.Now()
	exec.Error = err
	r.executions = append(r.executions, exec)
	r.mu.Unlock()

	return err
}

// GetExecutions returns all recorded executions
func (r *StepExecutionRecorder) GetExecutions() []StepExecution {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]StepExecution, len(r.executions))
	copy(result, r.executions)
	return result
}

// GetExecutionCount returns the number of recorded executions
func (r *StepExecutionRecorder) GetExecutionCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.executions)
}

// MockElicitationClient creates a mock elicitation client for testing
type MockElicitationClient struct {
	Responses map[string]string
	mu        sync.RWMutex
}

// NewMockElicitationClient creates a new mock elicitation client
func NewMockElicitationClient() *MockElicitationClient {
	return &MockElicitationClient{
		Responses: make(map[string]string),
	}
}

// SetResponse sets a response for a given prompt
func (m *MockElicitationClient) SetResponse(prompt, response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[prompt] = response
}

// Elicit returns a mocked response
func (m *MockElicitationClient) Elicit(ctx context.Context, prompt string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if response, ok := m.Responses[prompt]; ok {
		return response, nil
	}
	return "", fmt.Errorf("no mock response for prompt: %s", prompt)
}
