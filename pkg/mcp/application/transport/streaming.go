// Package transport provides streaming transport implementation for MCP
package transport

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/server"
)

// StreamingTransport implements the full API Transport interface with streaming support
type StreamingTransport struct {
	logger     *slog.Logger
	mcpServer  *server.MCPServer
	streamChan chan interface{}
	mu         sync.RWMutex
	connected  bool
	closed     bool
	bufferSize int
}

// NewStreamingTransport creates a new streaming transport with the specified buffer size
func NewStreamingTransport(logger *slog.Logger, bufferSize int) *StreamingTransport {
	if bufferSize <= 0 {
		bufferSize = 100 // Default buffer size
	}

	return &StreamingTransport{
		logger:     logger.With("component", "streaming_transport"),
		streamChan: make(chan interface{}, bufferSize),
		bufferSize: bufferSize,
	}
}

// Start starts the streaming transport
func (t *StreamingTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return errors.New("transport already started")
	}

	t.logger.Info("Starting streaming transport", "buffer_size", t.bufferSize)
	t.connected = true
	t.closed = false

	return nil
}

// Stop stops the streaming transport
func (t *StreamingTransport) Stop(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return errors.New("transport not started")
	}

	t.logger.Info("Stopping streaming transport")
	t.connected = false
	t.closed = true

	// Close the stream channel
	close(t.streamChan)

	return nil
}

// Send sends a message through the streaming transport
func (t *StreamingTransport) Send(message interface{}) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		return errors.New("transport not connected")
	}

	if t.closed {
		return errors.New("transport closed")
	}

	select {
	case t.streamChan <- message:
		t.logger.Debug("Message sent to stream", "message_type", getMessageType(message))
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("send timeout - buffer full")
	}
}

// Receive receives a message (blocking)
func (t *StreamingTransport) Receive() (interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		return nil, errors.New("transport not connected")
	}

	if t.closed {
		return nil, errors.New("transport closed")
	}

	select {
	case msg := <-t.streamChan:
		t.logger.Debug("Message received from stream", "message_type", getMessageType(msg))
		return msg, nil
	case <-time.After(30 * time.Second):
		return nil, errors.New("receive timeout")
	}
}

// ReceiveStream returns a channel for streaming messages
func (t *StreamingTransport) ReceiveStream() (<-chan interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		return nil, errors.New("transport not connected")
	}

	if t.closed {
		return nil, errors.New("transport closed")
	}

	t.logger.Info("Streaming channel requested")
	return t.streamChan, nil
}

// IsConnected checks if the transport is connected
func (t *StreamingTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// Serve implements the simplified Transport interface for compatibility
func (t *StreamingTransport) Serve(ctx context.Context, mcpServer *server.MCPServer) error {
	t.mcpServer = mcpServer

	// Start the streaming transport
	if err := t.Start(ctx); err != nil {
		return err
	}

	// Create a streaming bridge that intercepts MCP messages
	bridge := NewStreamingBridge(t, mcpServer, t.logger)

	// Run the bridge in a separate goroutine
	go func() {
		if err := bridge.Run(ctx); err != nil {
			t.logger.Error("Streaming bridge failed", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	return t.Stop(ctx)
}

// StreamingBridge bridges MCP protocol messages to streaming transport
type StreamingBridge struct {
	transport *StreamingTransport
	mcpServer *server.MCPServer
	logger    *slog.Logger
}

// NewStreamingBridge creates a new streaming bridge
func NewStreamingBridge(transport *StreamingTransport, mcpServer *server.MCPServer, logger *slog.Logger) *StreamingBridge {
	return &StreamingBridge{
		transport: transport,
		mcpServer: mcpServer,
		logger:    logger.With("component", "streaming_bridge"),
	}
}

// Run starts the streaming bridge
func (b *StreamingBridge) Run(ctx context.Context) error {
	b.logger.Info("Starting streaming bridge")

	// Create a ticker for periodic status updates
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Streaming bridge stopped")
			return ctx.Err()
		case <-ticker.C:
			// Send periodic status updates
			status := map[string]interface{}{
				"timestamp":   time.Now(),
				"type":        "status",
				"connected":   b.transport.IsConnected(),
				"buffer_size": b.transport.bufferSize,
			}

			if err := b.transport.Send(status); err != nil {
				b.logger.Warn("Failed to send status update", "error", err)
			}
		}
	}
}

// StreamingProgressEmitter implements api.ProgressEmitter for streaming transport
type StreamingProgressEmitter struct {
	transport *StreamingTransport
	logger    *slog.Logger
}

// NewStreamingProgressEmitter creates a new streaming progress emitter
func NewStreamingProgressEmitter(transport *StreamingTransport, logger *slog.Logger) *StreamingProgressEmitter {
	return &StreamingProgressEmitter{
		transport: transport,
		logger:    logger.With("component", "streaming_progress_emitter"),
	}
}

// Emit reports progress with step, percent, and message
func (e *StreamingProgressEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	progress := map[string]interface{}{
		"type":      "progress",
		"stage":     stage,
		"percent":   percent,
		"message":   message,
		"timestamp": time.Now(),
	}

	return e.transport.Send(progress)
}

// EmitDetailed reports progress with full structured update
func (e *StreamingProgressEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	progress := map[string]interface{}{
		"type":      "progress_detailed",
		"update":    update,
		"timestamp": time.Now(),
	}

	return e.transport.Send(progress)
}

// Close finalizes the progress reporting
func (e *StreamingProgressEmitter) Close() error {
	final := map[string]interface{}{
		"type":      "progress_final",
		"timestamp": time.Now(),
	}

	return e.transport.Send(final)
}

// getMessageType extracts the message type for logging
func getMessageType(message interface{}) string {
	if msg, ok := message.(map[string]interface{}); ok {
		if msgType, exists := msg["type"]; exists {
			return msgType.(string)
		}
	}
	return "unknown"
}
