package conversation

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ChatToolArgs defines arguments for the chat tool
type ChatToolArgs struct {
	types.BaseToolArgs
	Message   string `json:"message" description:"Your message to the assistant"`
	SessionID string `json:"session_id,omitempty" description:"Session ID for continuing a conversation (optional for first message)"`
}

// ChatToolResult defines the response from the chat tool
type ChatToolResult struct {
	types.BaseToolResponse
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Stage     string `json:"stage,omitempty"`
	Status    string `json:"status,omitempty"`

	// Optional structured data
	Options   []map[string]interface{} `json:"options,omitempty"`
	NextSteps []string                 `json:"next_steps,omitempty"`
	Progress  map[string]interface{}   `json:"progress,omitempty"`
}

// ChatTool implements the chat tool for conversation mode
type ChatTool struct {
	Handler func(context.Context, ChatToolArgs) (*ChatToolResult, error)
	Logger  zerolog.Logger
}

// Execute implements the unified Tool interface
func (ct *ChatTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assert the arguments
	chatArgs, ok := args.(ChatToolArgs)
	if !ok {
		return nil, fmt.Errorf("invalid argument type for chat tool: %T", args)
	}

	return ct.ExecuteTyped(ctx, chatArgs)
}

// ExecuteTyped handles the chat tool execution with typed arguments
func (ct *ChatTool) ExecuteTyped(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
	ct.Logger.Debug().
		Interface("args", args).
		Msg("Executing chat tool")

	// Call the handler
	result, err := ct.Handler(ctx, args)
	if err != nil {
		ct.Logger.Error().Err(err).Msg("Chat handler error")
		return &ChatToolResult{
			BaseToolResponse: types.NewBaseResponse("chat", args.SessionID, args.DryRun),
			Success:          false,
			Message:          fmt.Sprintf("Error: %v", err),
		}, nil
	}

	return result, nil
}

// GetMetadata returns comprehensive metadata about the chat tool
func (ct *ChatTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "chat",
		Description: "Interactive chat tool for conversation mode with AI assistance",
		Version:     "1.0.0",
		Category:    "Communication",
		Dependencies: []string{
			"AI Handler",
			"Session Management",
		},
		Capabilities: []string{
			"Interactive conversation",
			"Session continuity",
			"Multi-turn dialogue",
			"Structured responses",
			"Progress tracking",
		},
		Requirements: []string{
			"Valid message content",
			"AI handler function",
		},
		Parameters: map[string]string{
			"message":    "Required: Your message to the assistant",
			"session_id": "Optional: Session ID for continuing a conversation",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Start new conversation",
				Description: "Begin a new chat session with the AI assistant",
				Input: map[string]interface{}{
					"message": "Hello, I need help with my application deployment",
				},
				Output: map[string]interface{}{
					"success":    true,
					"session_id": "chat-session-123",
					"message":    "Hello! I'd be happy to help with your application deployment. What type of application are you working with?",
					"stage":      "conversation",
					"status":     "active",
				},
			},
			{
				Name:        "Continue conversation",
				Description: "Continue an existing chat session",
				Input: map[string]interface{}{
					"message":    "I have a Node.js application that needs to be containerized",
					"session_id": "chat-session-123",
				},
				Output: map[string]interface{}{
					"success":    true,
					"session_id": "chat-session-123",
					"message":    "Great! I can help you containerize your Node.js application. Let me analyze your project structure and create a Dockerfile for you.",
					"stage":      "analysis",
					"status":     "processing",
					"next_steps": []string{
						"Analyze repository structure",
						"Generate Dockerfile",
						"Build container image",
					},
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the chat tool
func (ct *ChatTool) Validate(ctx context.Context, args interface{}) error {
	chatArgs, ok := args.(ChatToolArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type: expected ChatToolArgs, got %T", args)
	}

	// Validate required fields
	if chatArgs.Message == "" {
		return fmt.Errorf("message is required and cannot be empty")
	}

	// Validate message length (reasonable limits)
	if len(chatArgs.Message) > 10000 {
		return fmt.Errorf("message is too long (max 10,000 characters)")
	}

	// Validate session ID format if provided
	if chatArgs.SessionID != "" {
		if len(chatArgs.SessionID) < 3 || len(chatArgs.SessionID) > 100 {
			return fmt.Errorf("session_id must be between 3 and 100 characters")
		}
	}

	// Validate handler is available
	if ct.Handler == nil {
		return fmt.Errorf("chat handler is not configured")
	}

	return nil
}
