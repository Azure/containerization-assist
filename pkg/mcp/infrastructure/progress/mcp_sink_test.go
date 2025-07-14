package progress

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockMCPServer is a mock implementation that can be embedded in MCPSink for testing
type MockMCPServer struct {
	mock.Mock
}

func (m *MockMCPServer) SendNotificationToClient(ctx context.Context, method string, params map[string]any) error {
	args := m.Called(ctx, method, params)
	return args.Error(0)
}

// createTestSink creates an MCPSink with a mock server for testing
func createTestSink(mockServer *MockMCPServer, token interface{}, logger *slog.Logger) *MCPSink {
	sink := &MCPSink{
		baseSink: newBaseSink(logger, "mcp-sink"),
		srv:      mockServer,
		token:    token,
	}
	return sink
}

func TestMCPSink_NewMCPSink(t *testing.T) {
	logger := slog.Default()
	realServer := &server.MCPServer{}
	token := "test-token"

	sink := NewMCPSink(realServer, token, logger)

	assert.NotNil(t, sink)
	assert.NotNil(t, sink.baseSink)
	assert.Equal(t, realServer, sink.srv)
	assert.Equal(t, token, sink.token)
}

func TestMCPSink_PublishWithNilServer(t *testing.T) {
	logger := slog.Default()
	sink := NewMCPSink(nil, "token", logger)
	ctx := context.Background()

	update := progress.Update{
		Step:       1,
		Total:      5,
		Percentage: 20,
		Status:     "running",
		Message:    "Processing",
		UserMeta:   map[string]interface{}{},
	}

	err := sink.Publish(ctx, update)
	// With nil server, we expect an error about notification channel not initialized
	assert.Error(t, err, "Should return error when server is nil")
	assert.Contains(t, err.Error(), "notification channel not initialized")
}

func TestMCPSink_PublishBasicUpdate(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	token := "test-token-123"
	sink := createTestSink(mockServer, token, logger)
	ctx := context.Background()

	update := progress.Update{
		Step:       2,
		Total:      10,
		Percentage: 20,
		Status:     "running",
		Message:    "Processing step 2",
		TraceID:    "trace-456",
		StartedAt:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		ETA:        3 * time.Minute,
		UserMeta: map[string]interface{}{
			"step_name":    "Build Docker Image",
			"substep_name": "downloading base image",
			"can_abort":    true,
		},
	}

	// Set up mock expectation - note the map[string]any conversion
	mockServer.On("SendNotificationToClient", ctx, "notifications/progress", mock.MatchedBy(func(payload map[string]any) bool {
		// Verify required fields
		return payload["progressToken"] == token &&
			payload["step"] == 2 &&
			payload["total"] == 10 &&
			payload["percentage"] == 20 &&
			payload["status"] == "running" &&
			payload["message"] == "Processing step 2" &&
			payload["trace_id"] == "trace-456" &&
			payload["eta_ms"] == int64(180000) && // 3 minutes in ms
			payload["step_name"] == "Build Docker Image" &&
			payload["substep_name"] == "downloading base image" &&
			payload["can_abort"] == true
	})).Return(nil)

	err := sink.Publish(ctx, update)
	assert.NoError(t, err)

	mockServer.AssertExpectations(t)
}

func TestMCPSink_PublishWithServerError(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	token := "test-token"
	sink := createTestSink(mockServer, token, logger)
	ctx := context.Background()

	update := progress.Update{
		Step:       1,
		Total:      5,
		Percentage: 20,
		Status:     "running",
		Message:    "Processing",
		UserMeta:   map[string]interface{}{},
	}

	expectedError := assert.AnError
	mockServer.On("SendNotificationToClient", ctx, "notifications/progress", mock.Anything).Return(expectedError)

	err := sink.Publish(ctx, update)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockServer.AssertExpectations(t)
}

func TestMCPSink_HeartbeatThrottling(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	token := "test-token"
	sink := createTestSink(mockServer, token, logger)
	ctx := context.Background()

	heartbeatUpdate := progress.Update{
		Step:       1,
		Total:      5,
		Percentage: 20,
		Status:     "running",
		Message:    "Processing",
		UserMeta: map[string]interface{}{
			"kind": "heartbeat",
		},
	}

	// First heartbeat should go through
	mockServer.On("SendNotificationToClient", ctx, "notifications/progress", mock.Anything).Return(nil).Once()

	err := sink.Publish(ctx, heartbeatUpdate)
	require.NoError(t, err)

	// Second heartbeat immediately after should be throttled (no mock call expected)
	err = sink.Publish(ctx, heartbeatUpdate)
	assert.NoError(t, err, "Throttled heartbeat should not return error")

	mockServer.AssertExpectations(t)
}

func TestMCPSink_NonHeartbeatNotThrottled(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	token := "test-token"
	sink := createTestSink(mockServer, token, logger)
	ctx := context.Background()

	regularUpdate := progress.Update{
		Step:       1,
		Total:      5,
		Percentage: 20,
		Status:     "running",
		Message:    "Processing",
		UserMeta: map[string]interface{}{
			"kind": "regular", // Not a heartbeat
		},
	}

	// Both calls should go through since they're not heartbeats
	mockServer.On("SendNotificationToClient", ctx, "notifications/progress", mock.Anything).Return(nil).Twice()

	err := sink.Publish(ctx, regularUpdate)
	require.NoError(t, err)

	err = sink.Publish(ctx, regularUpdate)
	assert.NoError(t, err)

	mockServer.AssertExpectations(t)
}

func TestMCPSink_PayloadStructure(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	token := "test-token-payload"
	sink := createTestSink(mockServer, token, logger)
	ctx := context.Background()

	update := progress.Update{
		Step:       3,
		Total:      8,
		Percentage: 37,
		Status:     "generating",
		Message:    "AI is working",
		TraceID:    "trace-789",
		StartedAt:  time.Date(2025, 1, 1, 15, 30, 0, 0, time.UTC),
		ETA:        45 * time.Second,
		UserMeta: map[string]interface{}{
			"step_name":        "Generate Dockerfile",
			"substep_name":     "analyzing dependencies",
			"can_abort":        false,
			"tokens_generated": 250,
			"estimated_total":  500,
		},
	}

	var capturedPayload map[string]any
	mockServer.On("SendNotificationToClient", ctx, "notifications/progress", mock.MatchedBy(func(payload map[string]any) bool {
		capturedPayload = payload
		return true
	})).Return(nil)

	err := sink.Publish(ctx, update)
	require.NoError(t, err)

	// Verify top-level fields
	assert.Equal(t, token, capturedPayload["progressToken"])
	assert.Equal(t, 3, capturedPayload["step"])
	assert.Equal(t, 8, capturedPayload["total"])
	assert.Equal(t, 37, capturedPayload["percentage"])
	assert.Equal(t, "generating", capturedPayload["status"])
	assert.Equal(t, "AI is working", capturedPayload["message"])
	assert.Equal(t, "trace-789", capturedPayload["trace_id"])
	assert.Equal(t, update.StartedAt, capturedPayload["started_at"])

	// Verify enhanced fields
	assert.Equal(t, int64(45000), capturedPayload["eta_ms"])
	assert.Equal(t, "Generate Dockerfile", capturedPayload["step_name"])
	assert.Equal(t, "analyzing dependencies", capturedPayload["substep_name"])
	// can_abort=false should still be present in the payload
	assert.Contains(t, capturedPayload, "can_abort")
	assert.Equal(t, false, capturedPayload["can_abort"])

	// Verify metadata block (backward compatibility)
	metadata, exists := capturedPayload["metadata"]
	require.True(t, exists)
	metadataMap := metadata.(map[string]interface{})
	assert.Equal(t, 3, metadataMap["step"])
	assert.Equal(t, 8, metadataMap["total"])
	assert.Equal(t, 37, metadataMap["percentage"])
	assert.Equal(t, "generating", metadataMap["status"])
	assert.Equal(t, int64(45000), metadataMap["eta_ms"])
	assert.Equal(t, update.UserMeta, metadataMap["user_meta"])

	mockServer.AssertExpectations(t)
}

func TestMCPSink_PayloadWithoutOptionalFields(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	token := "test-token-minimal"
	sink := createTestSink(mockServer, token, logger)
	ctx := context.Background()

	update := progress.Update{
		Step:       1,
		Total:      3,
		Percentage: 33,
		Status:     "running",
		Message:    "Simple message",
		UserMeta:   map[string]interface{}{}, // No optional metadata
	}

	var capturedPayload map[string]any
	mockServer.On("SendNotificationToClient", ctx, "notifications/progress", mock.MatchedBy(func(payload map[string]any) bool {
		capturedPayload = payload
		return true
	})).Return(nil)

	err := sink.Publish(ctx, update)
	require.NoError(t, err)

	// Verify that optional fields are not present when not set
	_, hasStepName := capturedPayload["step_name"]
	_, hasSubstepName := capturedPayload["substep_name"]
	_, hasETAMs := capturedPayload["eta_ms"]

	assert.False(t, hasStepName, "step_name should not be present when not set")
	assert.False(t, hasSubstepName, "substep_name should not be present when not set")
	assert.False(t, hasETAMs, "eta_ms should not be present when ETA is zero")

	// can_abort should always be present (defaults to false)
	assert.Contains(t, capturedPayload, "can_abort")
	assert.Equal(t, false, capturedPayload["can_abort"])

	mockServer.AssertExpectations(t)
}

func TestMCPSink_Close(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	sink := createTestSink(mockServer, "token", logger)

	err := sink.Close()
	assert.NoError(t, err, "Close should not return an error")
}

func TestMCPSink_InterfaceCompliance(t *testing.T) {
	logger := slog.Default()
	mockServer := &MockMCPServer{}
	sink := createTestSink(mockServer, "token", logger)

	// Verify that MCPSink implements progress.Sink interface
	var _ progress.Sink = sink
}
