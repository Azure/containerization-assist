package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// StdioLLMTransport implements types.LLMTransport for stdio transport
// It can invoke tools back to the hosting LLM via stdio
type StdioLLMTransport struct {
	stdioTransport *StdioTransport
	logger         zerolog.Logger
	jsonrpcClient  *Client
	mu             sync.Mutex
	connected      bool
}

// NewStdioLLMTransport creates a new stdio LLM transport
func NewStdioLLMTransport(stdioTransport *StdioTransport, logger zerolog.Logger) *StdioLLMTransport {
	return &StdioLLMTransport{
		stdioTransport: stdioTransport,
		logger:         logger.With().Str("component", "stdio_llm_transport").Logger(),
		// JSON-RPC client will be initialized on first use
	}
}

// InvokeTool implements types.LLMTransport
// For stdio, this means sending a JSON-RPC request back through the stdio channel
func (s *StdioLLMTransport) InvokeTool(ctx context.Context, name string, payload map[string]any, stream bool) (<-chan json.RawMessage, error) {
	s.logger.Debug().
		Str("tool_name", name).
		Bool("stream", stream).
		Msg("Invoking tool on hosting LLM via stdio")

	// Initialize JSON-RPC client if not already done
	s.mu.Lock()
	if s.jsonrpcClient == nil {
		// Use stdin/stdout for bidirectional communication
		s.jsonrpcClient = NewClient(os.Stdin, os.Stdout)
	}
	jsonrpcClient := s.jsonrpcClient
	s.mu.Unlock()

	// Create a response channel
	responseCh := make(chan json.RawMessage, 1)

	go func() {
		defer close(responseCh)

		// For streaming responses, we'll use the same JSON-RPC approach
		// The streaming will be handled by the response channel
		if stream {
			s.logger.Debug().
				Str("tool_name", name).
				Msg("Processing streaming tool invocation via stdio")
		}

		// Prepare the tool invocation request
		// According to MCP spec, tool invocations use "tools/call" method
		params := map[string]interface{}{
			"name":      name,
			"arguments": payload,
		}

		// Send the JSON-RPC request
		result, err := jsonrpcClient.Call(ctx, "tools/call", params)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("tool_name", name).
				Msg("Failed to invoke tool via JSON-RPC")

			response := types.ToolInvocationResponse{
				Error: fmt.Sprintf("Failed to invoke tool '%s': %v", name, err),
			}
			if responseBytes, err := json.Marshal(response); err == nil {
				responseCh <- json.RawMessage(responseBytes)
			}
			return
		}

		// Send the result to the response channel
		select {
		case responseCh <- result:
		case <-ctx.Done():
			s.logger.Debug().Msg("Context cancelled while sending response")
		}
	}()

	return responseCh, nil
}

// Close cleans up the JSON-RPC client
func (s *StdioLLMTransport) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.jsonrpcClient != nil {
		return s.jsonrpcClient.Close()
	}
	return nil
}

// Start implements types.LLMTransport
func (s *StdioLLMTransport) Start(ctx context.Context) error {
	s.logger.Info().Msg("Starting Stdio LLM transport")
	s.connected = true
	return nil
}

// Stop implements types.LLMTransport
func (s *StdioLLMTransport) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Stopping Stdio LLM transport")
	s.connected = false
	return s.Close()
}

// Send implements types.LLMTransport
func (s *StdioLLMTransport) Send(ctx context.Context, message interface{}) error {
	s.logger.Debug().Interface("message", message).Msg("Sending message via Stdio LLM transport")
	// For stdio transport, sending is handled via InvokeTool
	return nil
}

// Receive implements types.LLMTransport
func (s *StdioLLMTransport) Receive(ctx context.Context) (interface{}, error) {
	s.logger.Debug().Msg("Receiving message via Stdio LLM transport")
	// For stdio transport, receiving is handled via InvokeTool response channels
	return nil, nil
}

// IsConnected implements types.LLMTransport
func (s *StdioLLMTransport) IsConnected() bool {
	return s.connected
}

// Ensure interface compliance
var _ types.LLMTransport = (*StdioLLMTransport)(nil)
