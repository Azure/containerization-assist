//go:build test

package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// MockLLMTransport implements both types.LLMTransport and legacy SendPrompt for testing
type MockLLMTransport struct {
	responses       map[string]types.ToolInvocationResponse
	promptResponses map[string]*types.LLMResponse // For SendPrompt method
	callHistory     []MockCall
	simulateDelay   time.Duration
	logger          zerolog.Logger
}

// MockCall records a tool invocation or prompt send for testing
type MockCall struct {
	CallType string // "tool" or "prompt"
	ToolName string
	Payload  map[string]any
	Envelope *types.PromptEnvelope // For SendPrompt calls
	Stream   bool
}

// NewMockLLMTransport creates a new mock LLM transport
func NewMockLLMTransport(logger zerolog.Logger) *MockLLMTransport {
	return &MockLLMTransport{
		responses:       make(map[string]types.ToolInvocationResponse),
		promptResponses: make(map[string]*types.LLMResponse),
		callHistory:     make([]MockCall, 0),
		logger:          logger.With().Str("component", "mock_llm_transport").Logger(),
	}
}

// Deprecated: Use NewMockLLMTransport instead
func NewMockToolInvokerTransport(logger zerolog.Logger) *MockLLMTransport {
	return NewMockLLMTransport(logger)
}

// SetResponse configures a canned response for a specific tool call
func (m *MockLLMTransport) SetResponse(toolName string, response types.ToolInvocationResponse) {
	m.responses[toolName] = response
}

// SetDefaultResponse sets a default response for all tool calls
func (m *MockLLMTransport) SetDefaultResponse(response types.ToolInvocationResponse) {
	m.responses["*"] = response
}

// SetSimulateDelay configures artificial delay for testing timeouts
func (m *MockLLMTransport) SetSimulateDelay(delay time.Duration) {
	m.simulateDelay = delay
}

// GetCallHistory returns all recorded calls
func (m *MockLLMTransport) GetCallHistory() []MockCall {
	return m.callHistory
}

// Reset clears all responses and call history
func (m *MockLLMTransport) Reset() {
	m.responses = make(map[string]types.ToolInvocationResponse)
	m.promptResponses = make(map[string]*types.LLMResponse)
	m.callHistory = make([]MockCall, 0)
	m.simulateDelay = 0
}

// InvokeTool implements types.LLMTransport
func (m *MockLLMTransport) InvokeTool(ctx context.Context, name string, payload map[string]any, stream bool) (<-chan json.RawMessage, error) {
	m.logger.Debug().
		Str("tool_name", name).
		Bool("stream", stream).
		Msg("Mock LLM transport received tool invocation")

	// Record the call
	call := MockCall{
		CallType: "tool",
		ToolName: name,
		Payload:  payload,
		Stream:   stream,
	}
	m.callHistory = append(m.callHistory, call)

	// Create response channel
	responseCh := make(chan json.RawMessage, 1)

	go func() {
		defer close(responseCh)

		// Simulate delay if configured
		if m.simulateDelay > 0 {
			select {
			case <-time.After(m.simulateDelay):
			case <-ctx.Done():
				m.logger.Debug().Msg("Context cancelled during simulated delay")
				return
			}
		}

		// Look up response
		var response types.ToolInvocationResponse
		var found bool

		// Try specific tool name first
		if resp, ok := m.responses[name]; ok {
			response = resp
			found = true
		} else if defaultResp, ok := m.responses["*"]; ok {
			// Fall back to default response
			response = defaultResp
			found = true
		}

		if !found {
			// Generate intelligent default responses based on tool name and payload
			response = m.generateDefaultResponse(name, payload)
		}

		// Handle streaming responses
		if stream {
			// For streaming, we could split the response into chunks
			// For simplicity, just send the full response as one chunk
			responseBytes, err := json.Marshal(response)
			if err != nil {
				m.logger.Error().Err(err).Msg("Failed to marshal mock response")
				return
			}

			select {
			case responseCh <- json.RawMessage(responseBytes):
			case <-ctx.Done():
				m.logger.Debug().Msg("Context cancelled while sending streamed mock response")
			}
		} else {
			// Non-streaming: send response immediately
			responseBytes, err := json.Marshal(response)
			if err != nil {
				m.logger.Error().Err(err).Msg("Failed to marshal mock response")
				return
			}

			select {
			case responseCh <- json.RawMessage(responseBytes):
			case <-ctx.Done():
				m.logger.Debug().Msg("Context cancelled while sending mock response")
			}
		}
	}()

	return responseCh, nil
}

// generateDefaultResponse creates intelligent default responses for common scenarios
func (m *MockLLMTransport) generateDefaultResponse(toolName string, payload map[string]any) types.ToolInvocationResponse {
	message, _ := payload["message"].(string) //nolint:errcheck // Will use empty string if not present

	// Generate contextual responses based on the message content
	var content string

	if strings.Contains(strings.ToLower(message), "language") || strings.Contains(strings.ToLower(message), "detect") {
		content = `{"language": "go", "framework": "none", "confidence": 0.9}`
	} else if strings.Contains(strings.ToLower(message), "dockerfile") {
		content = "Based on the analysis, I recommend using a multi-stage build with golang:1.21-alpine as the base image."
	} else if strings.Contains(strings.ToLower(message), "port") {
		content = "The application appears to expose port 8080 based on the configuration files."
	} else if strings.Contains(strings.ToLower(message), "kubernetes") || strings.Contains(strings.ToLower(message), "manifest") {
		content = "I recommend creating a Deployment with 3 replicas and a ClusterIP service for internal communication."
	} else {
		content = fmt.Sprintf("Mock response for tool '%s': %s", toolName, strings.TrimSpace(message))
	}

	return types.ToolInvocationResponse{
		Content: content,
		Error:   "",
	}
}

// SetPromptResponse configures a response for a specific stage/correlation ID
func (m *MockLLMTransport) SetPromptResponse(key string, response *types.LLMResponse) {
	m.promptResponses[key] = response
}

// SetPromptResponseFunc sets a function to generate responses dynamically
func (m *MockLLMTransport) SetPromptResponseFunc(fn func(*types.PromptEnvelope) *types.LLMResponse) {
	// Store as a special key that SendPrompt will check
	m.promptResponses["_func_"] = &types.LLMResponse{
		Metadata: map[string]interface{}{
			"_responseFunc": fn,
		},
	}
}

// SendPrompt implements the legacy prompt-based interface for compatibility
func (m *MockLLMTransport) SendPrompt(ctx context.Context, envelope *types.PromptEnvelope) (*types.LLMResponse, error) {
	m.logger.Debug().
		Str("session_id", envelope.SessionID).
		Str("stage", envelope.Stage).
		Str("correlation_id", envelope.CorrelationID).
		Msg("Mock LLM transport received prompt")

	// Record the call
	call := MockCall{
		CallType: "prompt",
		Envelope: envelope,
	}
	m.callHistory = append(m.callHistory, call)

	// Simulate delay if configured
	if m.simulateDelay > 0 {
		select {
		case <-time.After(m.simulateDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Check for response function
	if funcResp, ok := m.promptResponses["_func_"]; ok {
		if fn, ok := funcResp.Metadata["_responseFunc"].(func(*types.PromptEnvelope) *types.LLMResponse); ok {
			return fn(envelope), nil
		}
	}

	// Look for specific responses by correlation ID or stage
	if response, ok := m.promptResponses[envelope.CorrelationID]; ok {
		// Ensure correlation ID is set
		response.CorrelationID = envelope.CorrelationID
		return response, nil
	}
	if response, ok := m.promptResponses[envelope.Stage]; ok {
		// Ensure correlation ID is set
		response.CorrelationID = envelope.CorrelationID
		return response, nil
	}
	if response, ok := m.promptResponses["*"]; ok {
		// Ensure correlation ID is set
		response.CorrelationID = envelope.CorrelationID
		return response, nil
	}

	// Generate default response based on stage
	return m.generateDefaultPromptResponse(envelope), nil
}

// generateDefaultPromptResponse creates stage-aware default responses
func (m *MockLLMTransport) generateDefaultPromptResponse(envelope *types.PromptEnvelope) *types.LLMResponse {
	response := &types.LLMResponse{
		CorrelationID: envelope.CorrelationID,
		Timestamp:     time.Now(),
		Confidence:    0.9,
	}

	switch envelope.Stage {
	case types.StageWelcome:
		response.ResponseMessage = "Welcome! Let's analyze your repository."
		response.NextStage = types.StageAnalysis
		response.RequiresInput = false

	case types.StageAnalysis:
		response.ResponseMessage = "I'll analyze your repository structure and dependencies."
		response.ToolCalls = []types.ToolCall{
			{
				ToolName:      "analyze_repository",
				Arguments:     map[string]interface{}{"repo_url": "./"},
				CallID:        "call_1",
				CorrelationID: envelope.CorrelationID,
				Timestamp:     time.Now(),
			},
		}

	case types.StageDockerfile:
		response.ResponseMessage = "I'll generate an optimized Dockerfile for your application."
		response.ToolCalls = []types.ToolCall{
			{
				ToolName:      "generate_dockerfile",
				Arguments:     map[string]interface{}{},
				CallID:        "call_2",
				CorrelationID: envelope.CorrelationID,
				Timestamp:     time.Now(),
			},
		}

	case types.StageBuild:
		response.ResponseMessage = "Building your Docker image..."
		response.ToolCalls = []types.ToolCall{
			{
				ToolName:      "build_image",
				Arguments:     map[string]interface{}{"dry_run": false},
				CallID:        "call_3",
				CorrelationID: envelope.CorrelationID,
				Timestamp:     time.Now(),
			},
		}

	default:
		response.ResponseMessage = fmt.Sprintf("Processing stage: %s", envelope.Stage)
		response.RequiresInput = true
	}

	return response
}

// Ensure interface compliance
var _ types.LLMTransport = (*MockLLMTransport)(nil)
