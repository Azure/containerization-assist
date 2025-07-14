package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTransport is a mock implementation for testing
type MockTransport struct {
	mock.Mock
}

func (m *MockTransport) Serve(ctx context.Context, mcpServer *server.MCPServer) error {
	args := m.Called(ctx, mcpServer)
	return args.Error(0)
}

func TestTransportRegistry_RegisterAndStart(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)
	mockTransport := &MockTransport{}

	// Test registration
	registry.Register(TransportTypeStdio, mockTransport)

	// Test starting registered transport
	mockServer := &server.MCPServer{}
	ctx := context.Background()

	// Mock successful serve
	mockTransport.On("Serve", ctx, mockServer).Return(nil)

	err := registry.Start(ctx, TransportTypeStdio, mockServer)
	assert.NoError(t, err, "Should start registered transport successfully")

	mockTransport.AssertExpectations(t)
}

func TestTransportRegistry_StartUnregisteredTransport(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)
	mockServer := &server.MCPServer{}
	ctx := context.Background()

	err := registry.Start(ctx, TransportTypeHTTP, mockServer)
	assert.Error(t, err, "Should error with unregistered transport")
	assert.Contains(t, err.Error(), "unsupported transport type", "Should indicate unsupported transport")
}

func TestTransportRegistry_StartWithTransportError(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)
	mockTransport := &MockTransport{}
	registry.Register(TransportTypeStdio, mockTransport)

	mockServer := &server.MCPServer{}
	ctx := context.Background()

	expectedError := errors.New("transport serve error")
	mockTransport.On("Serve", ctx, mockServer).Return(expectedError)

	err := registry.Start(ctx, TransportTypeStdio, mockServer)
	assert.Error(t, err, "Should propagate transport error")
	assert.Equal(t, expectedError, err, "Should return the exact transport error")

	mockTransport.AssertExpectations(t)
}

func TestRegistry_UnsupportedTransport(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)
	mockServer := &server.MCPServer{}
	ctx := context.Background()

	err := registry.Start(ctx, "unsupported", mockServer)
	assert.Error(t, err, "Should error with unsupported transport")
	assert.Contains(t, err.Error(), "unsupported transport type", "Should indicate unsupported transport")
}

func TestTransportTypes(t *testing.T) {
	// Test that transport type constants are correct
	assert.Equal(t, "stdio", string(TransportTypeStdio), "Stdio transport type should be 'stdio'")
	assert.Equal(t, "http", string(TransportTypeHTTP), "HTTP transport type should be 'http'")
}

func TestHTTPTransport_Creation(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name        string
		port        int
		expectPanic bool
	}{
		{
			name:        "valid port",
			port:        8080,
			expectPanic: false,
		},
		{
			name:        "zero port (should default)",
			port:        0,
			expectPanic: false,
		},
		{
			name:        "high port",
			port:        9999,
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					NewHTTPTransport(logger, tt.port)
				}, "Should panic with invalid port")
			} else {
				transport := NewHTTPTransport(logger, tt.port)
				assert.NotNil(t, transport, "Should create transport successfully")
			}
		})
	}
}

func TestStdioTransport_Creation(t *testing.T) {
	logger := slog.Default()
	transport := NewStdioTransport(logger)
	assert.NotNil(t, transport, "Should create stdio transport successfully")
}

func TestTransportConcurrency(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)

	// Create multiple mock transports
	numTransports := 5
	mockTransports := make([]*MockTransport, numTransports)

	for i := 0; i < numTransports; i++ {
		mockTransports[i] = &MockTransport{}
		transportType := TransportType(fmt.Sprintf("mock-transport-%d", i))
		registry.Register(transportType, mockTransports[i])
	}

	// Test concurrent access to different transports
	ctx := context.Background()
	mockServer := &server.MCPServer{}

	// Set up expectations for all transports
	for i := 0; i < numTransports; i++ {
		mockTransports[i].On("Serve", ctx, mockServer).Return(nil)
	}

	// Start all transports concurrently
	done := make(chan error, numTransports)

	for i := 0; i < numTransports; i++ {
		go func(transportIndex int) {
			transportType := TransportType(fmt.Sprintf("mock-transport-%d", transportIndex))
			err := registry.Start(ctx, transportType, mockServer)
			done <- err
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < numTransports; i++ {
		err := <-done
		assert.NoError(t, err, "Concurrent transport start should succeed")
	}

	// Verify all expectations
	for i := 0; i < numTransports; i++ {
		mockTransports[i].AssertExpectations(t)
	}
}

func TestTransportInterface(t *testing.T) {
	// Test that our transports implement the Transport interface
	logger := slog.Default()

	var _ Transport = NewStdioTransport(logger)
	var _ Transport = NewHTTPTransport(logger, 8080)
	var _ Transport = &MockTransport{}
}

func TestTransportErrorPropagation(t *testing.T) {
	logger := slog.Default()
	registry := NewRegistry(logger)

	// Test different error types
	errorTests := []struct {
		name  string
		error error
	}{
		{
			name:  "generic error",
			error: errors.New("generic transport error"),
		},
		{
			name:  "context canceled",
			error: context.Canceled,
		},
		{
			name:  "context deadline exceeded",
			error: context.DeadlineExceeded,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			mockTransport := &MockTransport{}
			registry.Register("test-transport", mockTransport)

			mockServer := &server.MCPServer{}
			ctx := context.Background()

			mockTransport.On("Serve", ctx, mockServer).Return(tt.error)

			err := registry.Start(ctx, "test-transport", mockServer)
			assert.Error(t, err, "Should propagate error")
			assert.Equal(t, tt.error, err, "Should return the exact error")

			mockTransport.AssertExpectations(t)
		})
	}
}
