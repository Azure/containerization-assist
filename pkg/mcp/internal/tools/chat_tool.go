package tools

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
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

// Execute handles the chat tool execution
func (ct *ChatTool) Execute(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
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
