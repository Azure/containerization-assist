package conversation

//go:generate ../../../../bin/schemaGen -input=chat_tool.go -output=generated_chat_schemas.go -package=conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/types"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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

	Options   []map[string]interface{} `json:"options,omitempty"`
	NextSteps []string                 `json:"next_steps,omitempty"`
	Progress  map[string]interface{}   `json:"progress,omitempty"`
}

// ChatTool implements the chat tool for conversation mode
type ChatTool struct {
	Handler   func(context.Context, ChatToolArgs) (*ChatToolResult, error)
	Logger    zerolog.Logger
	createdAt time.Time
}

// Execute implements the unified Tool interface
func (ct *ChatTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	chatArgs, ok := args.(ChatToolArgs)
	if !ok {
		return nil, errors.NewError().Messagef("invalid argument type for chat tool: %T", args).Build()
	}

	return ct.ExecuteTyped(ctx, chatArgs)
}

// ExecuteTyped handles the chat tool execution with typed arguments
func (ct *ChatTool) ExecuteTyped(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
	ct.Logger.Debug().
		Interface("args", args).
		Msg("Executing chat tool")

	validator := NewConversationValidator()
	if err := validator.Validate(ctx, &args); err != nil {
		ct.Logger.Warn().Err(err).Msg("Chat tool input validation failed")

		var errorMessage string
		if validationErr, ok := err.(*validation.ValidationError); ok {
			errorMessage = fmt.Sprintf("Validation failed for %s: %s", validationErr.Field, validationErr.Message)
		} else {
			errorMessage = fmt.Sprintf("Input validation failed: %v", err)
		}

		return &ChatToolResult{
			BaseToolResponse: types.NewBaseResponse("chat", args.SessionID, args.DryRun),
			Success:          false,
			Message:          errorMessage,
			Status:           "validation_error",
		}, nil
	}

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
func (ct *ChatTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "chat",
		Description:  "Interactive chat tool for conversation mode with AI assistance",
		Version:      "1.0.0",
		Category:     api.ToolCategory("Communication"),
		Status:       api.ToolStatus("active"),
		Tags:         []string{"chat", "conversation", "ai"},
		RegisteredAt: ct.createdAt,
		LastModified: ct.createdAt,
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
	}
}

// Validate checks if the provided arguments are valid for the chat tool
func (ct *ChatTool) Validate(ctx context.Context, args interface{}) error {
	chatArgs, ok := args.(ChatToolArgs)
	if !ok {
		return errors.NewError().Messagef("invalid arguments type: expected ChatToolArgs, got %T", args).WithLocation(

		// Validate required fields
		).Build()
	}

	if chatArgs.Message == "" {
		return errors.NewError().Messagef("message is required and cannot be empty").WithLocation(

		// Validate message length (reasonable limits)
		).Build()
	}

	if len(chatArgs.Message) > 10000 {
		return errors.NewError().Messagef("message is too long (max 10,000 characters)").WithLocation(

		// Validate session ID format if provided
		).Build()
	}

	if chatArgs.SessionID != "" {
		if len(chatArgs.SessionID) < 3 || len(chatArgs.SessionID) > 100 {
			return errors.NewError().Messagef("session_id must be between 3 and 100 characters").WithLocation(

			// Validate handler is available
			).Build()
		}
	}

	if ct.Handler == nil {
		return errors.NewError().Messagef("chat handler is not configured").Build()
	}

	return nil
}
