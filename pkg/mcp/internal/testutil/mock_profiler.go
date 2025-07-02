package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// MockProfiler provides a test implementation of profiling
type MockProfiler struct {
	mu         sync.RWMutex
	executions map[string][]ProfiledExecution
	logger     zerolog.Logger
}

// ProfiledExecution represents a profiled execution
type ProfiledExecution struct {
	ToolName  string
	SessionID string
	Duration  time.Duration
	Result    interface{}
	Error     error
	Timestamp time.Time
}

// ProfiledTestSuite provides profiling capabilities for tests
type ProfiledTestSuite struct {
	t        *testing.T
	logger   zerolog.Logger
	profiler *MockProfiler
}

// MockBenchmark represents benchmark results
type MockBenchmark struct {
	ToolName        string
	Iterations      int
	AverageDuration time.Duration
	TotalDuration   time.Duration
	Executions      []ProfiledExecution
}

// NewMockProfiler creates a new mock profiler
func NewMockProfiler() *MockProfiler {
	return &MockProfiler{
		executions: make(map[string][]ProfiledExecution),
	}
}

// NewProfiledTestSuite creates a new profiled test suite
func NewProfiledTestSuite(t *testing.T, logger zerolog.Logger) *ProfiledTestSuite {
	return &ProfiledTestSuite{
		t:        t,
		logger:   logger,
		profiler: NewMockProfiler(),
	}
}

// ProfileExecution profiles a function execution
func (m *MockProfiler) ProfileExecution(toolName, sessionID string, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	start := time.Now()
	result, err := fn(context.Background())
	duration := time.Since(start)

	m.mu.Lock()
	defer m.mu.Unlock()

	execution := ProfiledExecution{
		ToolName:  toolName,
		SessionID: sessionID,
		Duration:  duration,
		Result:    result,
		Error:     err,
		Timestamp: start,
	}

	if m.executions[toolName] == nil {
		m.executions[toolName] = make([]ProfiledExecution, 0)
	}
	m.executions[toolName] = append(m.executions[toolName], execution)

	return result, err
}

// RunBenchmark runs a benchmark for a tool
func (m *MockProfiler) RunBenchmark(toolName string, iterations, concurrency int, fn func(context.Context) (interface{}, error)) MockBenchmark {
	executions := make([]ProfiledExecution, 0, iterations)
	totalStart := time.Now()

	for i := 0; i < iterations; i++ {
		start := time.Now()
		result, err := fn(context.Background())
		duration := time.Since(start)

		execution := ProfiledExecution{
			ToolName:  toolName,
			SessionID: "benchmark",
			Duration:  duration,
			Result:    result,
			Error:     err,
			Timestamp: start,
		}
		executions = append(executions, execution)
	}

	totalDuration := time.Since(totalStart)
	averageDuration := totalDuration / time.Duration(iterations)

	return MockBenchmark{
		ToolName:        toolName,
		Iterations:      iterations,
		AverageDuration: averageDuration,
		TotalDuration:   totalDuration,
		Executions:      executions,
	}
}

// GetExecutionsForTool returns executions for a specific tool
func (m *MockProfiler) GetExecutionsForTool(toolName string) []ProfiledExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if toolName == "" {
		// Return all executions
		var allExecutions []ProfiledExecution
		for _, execs := range m.executions {
			allExecutions = append(allExecutions, execs...)
		}
		return allExecutions
	}

	if executions, exists := m.executions[toolName]; exists {
		result := make([]ProfiledExecution, len(executions))
		copy(result, executions)
		return result
	}

	return make([]ProfiledExecution, 0)
}

// Clear clears all profiling data
func (m *MockProfiler) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = make(map[string][]ProfiledExecution)
}

// GetProfiler returns the underlying profiler
func (p *ProfiledTestSuite) GetProfiler() *MockProfiler {
	return p.profiler
}
