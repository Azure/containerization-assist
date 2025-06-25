package contract

import (
	"context"
	"encoding/json"
)

// LLMTransport provides a way to invoke tools on the hosting LLM
// This is separate from the MCP transport which serves tools TO the LLM
type LLMTransport interface {
	// InvokeTool calls a tool exposed by the hosting LLM (usually "chat")
	//   name    – tool name to call (e.g., "chat")
	//   payload – JSON-serializable map passed as arguments
	//   stream  – if true, receive partial chunks on the channel
	InvokeTool(ctx context.Context,
		name string,
		payload map[string]any,
		stream bool) (<-chan json.RawMessage, error)
}

// ToolInvocationPayload represents the standard payload for tool invocations
type ToolInvocationPayload struct {
	SessionID   string   `json:"session_id"`
	Message     string   `json:"message"`
	Temperature *float64 `json:"temperature,omitempty"`
	Model       *string  `json:"model,omitempty"`
	Stream      bool     `json:"stream"`
}

// ToolInvocationResponse represents the response from a tool invocation
type ToolInvocationResponse struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}
