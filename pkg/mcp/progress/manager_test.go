package progress

import (
	"bytes"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProgressReporter for testing
type MockProgressReporter struct {
	mock.Mock
	beginCalled    bool
	updateCalls    []updateCall
	completeCalled bool
	doneCalled     bool
	mu             sync.Mutex
}

type updateCall struct {
	step    float64
	message string
}

func (m *MockProgressReporter) Begin(msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beginCalled = true
	return m.Called(msg).Error(0)
}

func (m *MockProgressReporter) Update(step float64, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls = append(m.updateCalls, updateCall{step: step, message: msg})
	return m.Called(step, msg).Error(0)
}

func (m *MockProgressReporter) Complete(msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCalled = true
	return m.Called(msg).Error(0)
}

func (m *MockProgressReporter) Done() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.doneCalled = true
	m.Called()
}

// MockContext for testing MCP context
type MockContext struct {
	hasProgressToken bool
	progressReporter *MockProgressReporter
}

func (m *MockContext) HasProgressToken() bool {
	return m.hasProgressToken
}

func (m *MockContext) CreateSimpleProgressReporter(total *float64) (*mcp.ProgressReporter, error) {
	if !m.hasProgressToken {
		return nil, nil
	}
	// Return the mock reporter cast as the real type (would need adapter in real implementation)
	return &mcp.ProgressReporter{}, nil
}

// TestNewManager tests manager creation with different contexts
func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name      string
		ctx       *server.Context
		expectMCP bool
		expectCLI bool
		envCI     string
	}{
		{
			name:      "nil context falls back to CLI",
			ctx:       nil,
			expectMCP: false,
			expectCLI: true,
		},
		{
			name:      "CI environment uses simple output",
			ctx:       nil,
			expectMCP: false,
			expectCLI: true,
			envCI:     "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set CI environment
			if tt.envCI != "" {
				os.Setenv("CI", tt.envCI)
				defer os.Unsetenv("CI")
			}

			m := New(tt.ctx, 10, logger)
			assert.NotNil(t, m)
			assert.Equal(t, float64(10), m.total)
			assert.Equal(t, 0, m.current)
			assert.Equal(t, tt.expectCLI, m.isCLI)
			assert.Equal(t, tt.envCI == "true", m.isCI)
			assert.NotEmpty(t, m.traceID)
		})
	}
}

// TestProgressManagerUpdate tests the update functionality
func TestProgressManagerUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("update advances progress", func(t *testing.T) {
		m := New(nil, 10, logger)
		// Set minUpdateTime to 0 to avoid throttling
		m.minUpdateTime = 0

		// Update progress
		metadata := map[string]interface{}{
			"step_name": "test_step",
		}
		m.Update(3, "Processing step 3", metadata)

		assert.Equal(t, 3, m.current)
		assert.Contains(t, metadata, "percentage")
		assert.Equal(t, 30, metadata["percentage"])
		assert.Contains(t, metadata, "trace_id")
		assert.Equal(t, m.traceID, metadata["trace_id"])
	})

	t.Run("update throttling", func(t *testing.T) {
		m := New(nil, 10, logger)
		m.minUpdateTime = 100 * time.Millisecond
		// Set lastUpdate to past to ensure first update goes through
		m.lastUpdate = time.Now().Add(-200 * time.Millisecond)

		// First update should go through
		m.Update(1, "Step 1", nil)
		assert.Equal(t, 1, m.current)

		// Immediate second update should be throttled
		m.Update(2, "Step 2", nil)
		assert.Equal(t, 1, m.current) // Should not have updated due to throttling

		// Wait for throttle period
		time.Sleep(110 * time.Millisecond)
		m.Update(2, "Step 2", nil)
		assert.Equal(t, 2, m.current) // Should update now
	})
}

// TestProgressManagerComplete tests completion
func TestProgressManagerComplete(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	m := New(nil, 10, logger)
	m.Complete("Test completed")

	// Check that completion was logged
	output := buf.String()
	assert.Contains(t, output, "Progress completed")
	assert.Contains(t, output, "Test completed")
}

// TestProgressBar tests the progress bar rendering
func TestProgressBar(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		percentage int
		expected   string
	}{
		{0, "[░░░░░░░░░░░░░░░░░░░░]"},
		{25, "[█████░░░░░░░░░░░░░░░]"},
		{50, "[██████████░░░░░░░░░░]"},
		{75, "[███████████████░░░░░]"},
		{100, "[████████████████████]"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.percentage)), func(t *testing.T) {
			result := m.renderProgressBar(tt.percentage)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestETA tests ETA calculation
func TestETA(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("no ETA at start", func(t *testing.T) {
		m := New(nil, 10, logger)
		eta := m.calculateETA()
		assert.Equal(t, time.Duration(0), eta)
	})

	t.Run("calculates ETA after progress", func(t *testing.T) {
		m := New(nil, 10, logger)
		m.startTime = time.Now().Add(-10 * time.Second) // Started 10 seconds ago
		m.current = 2                                   // Completed 2 of 10 steps

		eta := m.calculateETA()
		// Should estimate about 40 seconds remaining (8 steps * 5 seconds per step)
		assert.Greater(t, eta.Seconds(), float64(35))
		assert.Less(t, eta.Seconds(), float64(45))
	})
}

// TestWatchdog tests the watchdog timer
func TestWatchdog(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("watchdog sends heartbeat", func(t *testing.T) {
		// This test would need a more sophisticated setup with mock MCP context
		// For now, just verify the watchdog starts and stops
		m := New(nil, 10, logger)

		// Give watchdog time to start
		time.Sleep(50 * time.Millisecond)

		// Stop watchdog
		m.stopWatchdog()

		// Verify no panic and clean shutdown
		assert.NotNil(t, m.watchdogStop)
	})
}

// TestTraceID tests trace ID generation
func TestTraceID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	m1 := New(nil, 10, logger)
	m2 := New(nil, 10, logger)

	// Each manager should have a unique trace ID
	assert.NotEmpty(t, m1.traceID)
	assert.NotEmpty(t, m2.traceID)
	assert.NotEqual(t, m1.traceID, m2.traceID)

	// Trace ID should be accessible
	assert.Equal(t, m1.traceID, m1.GetTraceID())
}

// TestStatusCodeMapping tests status code mapping
func TestStatusCodeMapping(t *testing.T) {
	tests := []struct {
		status string
		code   int
	}{
		{"running", 1},
		{"completed", 2},
		{"failed", 3},
		{"skipped", 4},
		{"retrying", 5},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			code := mapStatusToCode(tt.status)
			assert.Equal(t, tt.code, code)
		})
	}
}

// TestConcurrency tests thread safety
func TestConcurrency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(nil, 100, logger)

	// Run multiple goroutines updating progress
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(step int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				m.Update(step*10+j, "Concurrent update", nil)
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Should complete without race conditions
	assert.True(t, m.current >= 0)
	assert.True(t, m.current <= 100)
}

// BenchmarkUpdate benchmarks the update operation
func BenchmarkUpdate(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(nil, 1000, logger)

	metadata := map[string]interface{}{
		"step_name": "benchmark_step",
		"status":    "running",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(i%1000, "Benchmark update", metadata)
	}
}

// TestGettersAndHelpers tests various getter methods
func TestGettersAndHelpers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(nil, 10, logger)
	// Set minUpdateTime to 0 to avoid throttling
	m.minUpdateTime = 0

	// Test GetCurrent
	assert.Equal(t, 0, m.GetCurrent())
	m.Update(5, "Test", nil)
	assert.Equal(t, 5, m.GetCurrent())

	// Test GetTotal
	assert.Equal(t, 10, m.GetTotal())

	// Test IsComplete
	assert.False(t, m.IsComplete())
	m.Update(10, "Final", nil)
	assert.True(t, m.IsComplete())
}

// TestRepeatChar tests the string helper
func TestRepeatChar(t *testing.T) {
	tests := []struct {
		char     rune
		count    int
		expected string
	}{
		{'█', 0, ""},
		{'█', 1, "█"},
		{'█', 5, "█████"},
		{'░', 3, "░░░"},
		{'x', -1, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := repeatChar(tt.char, tt.count)
			assert.Equal(t, tt.expected, result)
		})
	}
}
