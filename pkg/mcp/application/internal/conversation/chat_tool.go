package conversation

//go:generate ../../../../../bin/schemaGen -tool=conversation_chat_tool -domain=conversation -output=.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// ChatToolArgs defines arguments for the chat tool
type ChatToolArgs struct {
	domaintypes.BaseToolArgs
	Message   string `json:"message" description:"Your message to the assistant"`
	SessionID string `json:"session_id,omitempty" description:"Session ID for continuing a conversation (optional for first message)"`
}

// ChatToolResult defines the response from the chat tool
type ChatToolResult struct {
	domaintypes.BaseToolResponse
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
	Logger    *slog.Logger
	createdAt time.Time
}

// Execute implements the unified Tool interface
func (ct *ChatTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Convert ToolInput to ChatToolArgs
	chatArgs := ChatToolArgs{
		BaseToolArgs: domaintypes.BaseToolArgs{
			SessionID: input.SessionID,
		},
	}

	// Extract message from input data
	if msg, ok := input.Data["message"].(string); ok {
		chatArgs.Message = msg
	} else {
		return api.ToolOutput{
			Success: false,
			Error:   "message parameter is required",
		}, nil
	}

	// Extract optional session_id from input data
	if sessionID, ok := input.Data["session_id"].(string); ok {
		chatArgs.SessionID = sessionID
	}

	result, err := ct.ExecuteTyped(ctx, chatArgs)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Convert result to map for ToolOutput.Data
	resultData := map[string]interface{}{
		"success":    result.Success,
		"session_id": result.SessionID,
		"message":    result.Message,
		"stage":      result.Stage,
		"status":     result.Status,
		"options":    result.Options,
		"next_steps": result.NextSteps,
		"progress":   result.Progress,
	}

	return api.ToolOutput{
		Success: result.Success,
		Data:    resultData,
	}, nil
}

// ExecuteTyped handles the chat tool execution with typed arguments
func (ct *ChatTool) ExecuteTyped(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
	ct.Logger.Debug("Executing chat tool",
		"message", args.Message,
		"session_id", args.SessionID)

	// Basic validation - using direct validation logic instead of validator for now
	// TODO: Update to use proper validation when validators.go is migrated
	if args.Message == "" {
		return &ChatToolResult{
			BaseToolResponse: domaintypes.NewBaseResponse("chat", args.SessionID, args.DryRun),
			Success:          false,
			Message:          "message is required and cannot be empty",
			Status:           "validation_error",
		}, nil
	}

	if len(args.Message) > 10000 {
		return &ChatToolResult{
			BaseToolResponse: domaintypes.NewBaseResponse("chat", args.SessionID, args.DryRun),
			Success:          false,
			Message:          "message is too long (max 10,000 characters)",
			Status:           "validation_error",
		}, nil
	}

	result, err := ct.Handler(ctx, args)
	if err != nil {
		ct.Logger.Error("Chat handler error", "error", err)
		return &ChatToolResult{
			BaseToolResponse: domaintypes.NewBaseResponse("chat", args.SessionID, args.DryRun),
			Success:          false,
			Message:          fmt.Sprintf("Error: %v", err),
		}, nil
	}

	return result, nil
}

// Name returns the unique identifier for this tool
func (ct *ChatTool) Name() string {
	return "chat"
}

// Description returns a human-readable description of the tool
func (ct *ChatTool) Description() string {
	return "Interactive chat tool for conversation mode with AI assistance"
}

// Schema returns the JSON schema for the tool's parameters and results
func (ct *ChatTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "chat",
		Description: "Interactive chat tool for conversation mode with AI assistance",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Your message to the assistant",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for continuing a conversation (optional for first message)",
				},
			},
			"required": []string{"message"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success":    map[string]interface{}{"type": "boolean"},
				"session_id": map[string]interface{}{"type": "string"},
				"message":    map[string]interface{}{"type": "string"},
				"stage":      map[string]interface{}{"type": "string"},
				"status":     map[string]interface{}{"type": "string"},
			},
		},
	}
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
