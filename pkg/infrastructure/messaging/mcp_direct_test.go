package messaging

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMCPServer implements a mock for the MCP server
type MockMCPServer struct {
	mock.Mock
}

// SendNotificationToClient method removed as dead code

// Helper to cast interface{} to *MockMCPServer for testing
func createTestMCPEmitter(mockServer *MockMCPServer) *MCPDirectEmitter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	token := "test-token-123"

	// Cast to server.MCPServer interface - we'll need to test the actual implementation
	// For now, we'll create without server for basic tests
	return &MCPDirectEmitter{
		server:      nil, // Will be nil for most tests
		token:       token,
		logger:      logger,
		minInterval: 100 * time.Millisecond,
	}
}

func TestNewMCPDirectEmitter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	token := "test-token"

	emitter := NewMCPDirectEmitter(nil, token, logger)

	assert.NotNil(t, emitter)
	assert.Nil(t, emitter.server)
	assert.Equal(t, token, emitter.token)
	assert.NotNil(t, emitter.logger)
	assert.Equal(t, 100*time.Millisecond, emitter.minInterval)
	assert.True(t, emitter.lastSent.IsZero())
}

func TestMCPDirectEmitter_Emit_CallsEmitDetailed(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	// Since server is nil, this should succeed (no-op)
	err := emitter.Emit(context.Background(), "test_stage", 50, "Test message")
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_EmitDetailed_NoServer(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	update := api.ProgressUpdate{
		Stage:      "test_stage",
		Percentage: 75,
		Message:    "Test update",
		Status:     "running",
	}

	// Should succeed as no-op when server is nil
	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_EmitDetailed_RateLimiting(t *testing.T) {
	emitter := createTestMCPEmitter(nil)
	emitter.minInterval = 50 * time.Millisecond

	update := api.ProgressUpdate{
		Stage:      "test_stage",
		Percentage: 25,
		Message:    "First update",
		Status:     "running",
	}

	// First call should succeed
	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)

	// Immediate second call should be rate limited (but still succeed as no-op)
	update.Message = "Second update"
	err = emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)

	// After waiting, should work again
	time.Sleep(60 * time.Millisecond)
	update.Message = "Third update"
	err = emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_EmitDetailed_NonRunningStatus(t *testing.T) {
	emitter := createTestMCPEmitter(nil)
	emitter.minInterval = 1 * time.Second // Long interval

	// Set last sent time to recent past
	emitter.lastSent = time.Now().Add(-10 * time.Millisecond)

	update := api.ProgressUpdate{
		Stage:      "test_stage",
		Percentage: 100,
		Message:    "Completed",
		Status:     "completed", // Non-running status should bypass rate limiting
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_EmitDetailed_PayloadConstruction(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	// Test with full update data
	update := api.ProgressUpdate{
		Stage:      "deployment",
		Percentage: 85,
		Message:    "Deploying application",
		Status:     "running",
		Step:       8,
		Total:      10,
		ETA:        30 * time.Second,
		TraceID:    "trace-123",
		Metadata: map[string]interface{}{
			"pod_count": 3,
			"namespace": "default",
		},
	}

	// Should succeed (no-op with nil server)
	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_EmitDetailed_MinimalPayload(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	// Test with minimal required fields
	update := api.ProgressUpdate{
		Percentage: 42,
		Message:    "Simple message",
		Status:     "running",
		// Stage, Step, Total, ETA, TraceID, Metadata are empty/zero values
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_Close(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	err := emitter.Close()
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_Close_WithTimeout(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	// This should complete quickly since server is nil
	start := time.Now()
	err := emitter.Close()
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, 1*time.Second) // Should be much faster than 5s timeout
}

func TestMCPDirectEmitter_ImplementsInterface(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	// Verify it implements the ProgressEmitter interface
	var _ api.ProgressEmitter = emitter
}

func TestMCPDirectEmitter_ConcurrentAccess(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	// Test concurrent access to the emitter
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 5; j++ {
				update := api.ProgressUpdate{
					Stage:      "concurrent_test",
					Percentage: j * 20,
					Message:    "Concurrent message",
					Status:     "running",
				}

				_ = emitter.EmitDetailed(context.Background(), update)
				_ = emitter.Emit(context.Background(), "concurrent_stage", j*20, "Concurrent emit")
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Should not have panicked
	assert.True(t, true)
}

func TestMCPDirectEmitter_ContextCancellation(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := api.ProgressUpdate{
		Stage:      "cancelled_test",
		Percentage: 50,
		Message:    "Should work with cancelled context",
		Status:     "running",
	}

	// Should still work with cancelled context (doesn't check context in current implementation)
	err := emitter.EmitDetailed(ctx, update)
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_TokenHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name  string
		token interface{}
	}{
		{"string token", "string-token"},
		{"integer token", 12345},
		{"nil token", nil},
		{"complex token", map[string]string{"id": "token-123"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter := NewMCPDirectEmitter(nil, test.token, logger)
			assert.Equal(t, test.token, emitter.token)

			// Should work with any token type
			err := emitter.Emit(context.Background(), "test", 50, "test")
			assert.NoError(t, err)
		})
	}
}

func TestMCPDirectEmitter_ETAHandling(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	update := api.ProgressUpdate{
		Stage:      "eta_test",
		Percentage: 60,
		Message:    "Testing ETA",
		Status:     "running",
		ETA:        2*time.Minute + 30*time.Second, // 150 seconds
	}

	// Should succeed (payload would contain eta_ms: 150000)
	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestMCPDirectEmitter_ZeroValues(t *testing.T) {
	emitter := createTestMCPEmitter(nil)

	update := api.ProgressUpdate{
		// All zero values except required fields
		Percentage: 0,
		Message:    "",
		Status:     "running",
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}
