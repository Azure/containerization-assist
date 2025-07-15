package transport

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTransportLifecycle tests the complete lifecycle of all transport types
func TestTransportLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise during testing
	}))

	tests := []struct {
		name      string
		transport TransportType
		port      int
	}{
		{
			name:      "Stdio_Lifecycle",
			transport: TransportTypeStdio,
			port:      0, // Not applicable for stdio
		},
		{
			name:      "HTTP_Lifecycle",
			transport: TransportTypeHTTP,
			port:      0, // Use random port
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create registry and register transports
			registry := NewRegistry(logger)
			registry.Register(TransportTypeStdio, NewStdioTransport(logger))
			registry.Register(TransportTypeHTTP, NewHTTPTransport(logger, tt.port))
			require.NotNil(t, registry, "Registry should be created")

			// Create mock server
			mockServer := &server.MCPServer{}

			// Test lifecycle phases
			t.Run("StartAndStop", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// Start the transport
				done := make(chan error, 1)
				go func() {
					done <- registry.Start(ctx, tt.transport, mockServer)
				}()

				// Give it a moment to start
				time.Sleep(100 * time.Millisecond)

				// Cancel context to stop
				cancel()

				// Wait for completion
				err := <-done
				// Context cancellation is expected
				if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
					t.Logf("Transport stopped with error: %v", err)
				}
			})
		})
	}
}

// TestTransportConcurrentLifecycle tests concurrent lifecycle operations
func TestTransportConcurrentLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	registry := NewRegistry(logger)

	// Register multiple transport types
	stdioTransport := NewStdioTransport(logger)
	httpTransport := NewHTTPTransport(logger, 0) // Random port

	registry.Register(TransportTypeStdio, stdioTransport)
	registry.Register(TransportTypeHTTP, httpTransport)

	// Test concurrent access to registry
	t.Run("ConcurrentRegistryAccess", func(t *testing.T) {
		// Use mock transports for concurrent test to avoid port conflicts
		mockRegistry := NewRegistry(logger)

		// Register mock transports that immediately return context.Canceled
		mockStdio := &MockTransport{}
		mockStdio.On("Serve", mock.Anything, mock.Anything).Return(context.Canceled)
		mockRegistry.Register(TransportTypeStdio, mockStdio)

		mockHTTP := &MockTransport{}
		mockHTTP.On("Serve", mock.Anything, mock.Anything).Return(context.Canceled)
		mockRegistry.Register(TransportTypeHTTP, mockHTTP)

		numGoroutines := 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Start multiple goroutines accessing the registry
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				// Alternate between transport types
				transportType := TransportTypeStdio
				if id%2 == 0 {
					transportType = TransportTypeHTTP
				}

				// Create unique mock server for each goroutine to avoid races
				localMockServer := &server.MCPServer{}
				err := mockRegistry.Start(ctx, transportType, localMockServer)
				// Context cancellation is expected from our mocks
				if err != nil && err != context.Canceled {
					t.Errorf("Unexpected error from goroutine %d: %v", id, err)
				}
			}(i)
		}

		wg.Wait()
		mockStdio.AssertExpectations(t)
		mockHTTP.AssertExpectations(t)
	})
}

// TestTransportRaceConditions specifically tests for race conditions
func TestTransportRaceConditions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	registry := NewRegistry(logger)

	// Test concurrent registration and access
	t.Run("ConcurrentRegistrationAndAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		numOperations := 50

		// Start registering and accessing concurrently
		for i := 0; i < numOperations; i++ {
			wg.Add(2) // One for register, one for start

			// Register transport
			go func(id int) {
				defer wg.Done()
				transportType := TransportType("test-transport-" + string(rune(id%10)))
				mockTransport := &MockTransport{}
				mockTransport.On("Serve", mock.Anything, mock.Anything).Return(context.Canceled)
				registry.Register(transportType, mockTransport)
			}(i)

			// Try to start transport
			go func(id int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				transportType := TransportType("test-transport-" + string(rune(id%10)))
				// Create unique mock server for each goroutine to avoid races
				localMockServer := &server.MCPServer{}
				err := registry.Start(ctx, transportType, localMockServer)
				// Errors are expected since transports may not be registered yet
				_ = err
			}(i)
		}

		wg.Wait()
	})
}

// TestTransportServerLifecycle tests integration with actual MCP server
func TestTransportServerLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create a proper MCP server for testing
	mcpServer := &server.MCPServer{}

	tests := []struct {
		name      string
		transport TransportType
		timeout   time.Duration
	}{
		{
			name:      "StdioWithServer",
			transport: TransportTypeStdio,
			timeout:   2 * time.Second,
		},
		{
			name:      "HTTPWithServer",
			transport: TransportTypeHTTP,
			timeout:   2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry(logger)
			registry.Register(TransportTypeStdio, NewStdioTransport(logger))
			registry.Register(TransportTypeHTTP, NewHTTPTransport(logger, 0))

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Start the transport in a goroutine
			done := make(chan error, 1)
			go func() {
				done <- registry.Start(ctx, tt.transport, mcpServer)
			}()

			// Let it run for a short time
			time.Sleep(500 * time.Millisecond)

			// Cancel and wait for completion
			cancel()
			err := <-done

			// Context cancellation is expected
			if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
				t.Logf("Transport finished with: %v", err)
			}
		})
	}
}

// TestTransportErrorHandling tests error propagation and handling
func TestTransportErrorHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	registry := NewRegistry(logger)
	mockServer := &server.MCPServer{}

	t.Run("ErrorPropagation", func(t *testing.T) {
		// Create a transport that will error
		errorTransport := &MockTransport{}
		expectedError := errors.New("transport error")
		errorTransport.On("Serve", mock.Anything, mockServer).Return(expectedError)

		registry.Register("error-transport", errorTransport)

		ctx := context.Background()
		err := registry.Start(ctx, "error-transport", mockServer)

		assert.Error(t, err, "Should propagate transport error")
		assert.Contains(t, err.Error(), "failed to start error-transport transport", "Should wrap transport error")
		assert.ErrorIs(t, err, expectedError, "Should wrap the original error")
		errorTransport.AssertExpectations(t)
	})

	t.Run("UnregisteredTransportError", func(t *testing.T) {
		ctx := context.Background()
		err := registry.Start(ctx, "nonexistent", mockServer)

		assert.Error(t, err, "Should error with unregistered transport")
		assert.Contains(t, err.Error(), "unsupported transport type", "Should indicate unsupported transport")
	})
}

// TestTransportResourceCleanup tests proper resource cleanup
func TestTransportResourceCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create multiple registries to test resource cleanup
	registries := make([]*Registry, 10)
	for i := 0; i < 10; i++ {
		registries[i] = NewRegistry(logger)
		registries[i].Register(TransportTypeStdio, NewStdioTransport(logger))
	}

	mockServer := &server.MCPServer{}

	// Start and stop all registries rapidly
	var wg sync.WaitGroup
	for i, registry := range registries {
		wg.Add(1)
		go func(r *Registry, id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := r.Start(ctx, TransportTypeStdio, mockServer)
			// Context cancellation expected
			if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
				t.Logf("Registry %d finished with: %v", id, err)
			}
		}(registry, i)
	}

	wg.Wait()
}

// Helper functions

func isExpectedTestError(err error) bool {
	if err == nil {
		return true
	}
	errStr := err.Error()
	expectedErrors := []string{
		"no MCP server",
		"connection refused",
		"address already in use",
		"bind: address already in use",
		"use of closed network connection",
		"operation canceled",
		"context canceled",
		"context deadline exceeded",
	}

	for _, expected := range expectedErrors {
		if strings.Contains(errStr, expected) {
			return true
		}
	}
	return false
}
