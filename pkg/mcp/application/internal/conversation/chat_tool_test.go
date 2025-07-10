package conversation

import (
	"context"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"k8s.io/kube-openapi/pkg/validation/errors"

	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatTool_Execute(t *testing.T) {
	tests := []struct {
		name        string
		args        interface{}
		setup       func() *ChatTool
		expectedErr string
	}{
		{
			name: "valid chat args",
			args: api.ToolInput{
				SessionID: "test-session",
				Data: map[string]interface{}{
					"message": "Hello, world!",
				},
			},
			setup: func() *ChatTool {
				return &ChatTool{
					Handler: func(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
						return &ChatToolResult{
							BaseToolResponse: shared.NewBaseResponse("chat", args.SessionID, args.DryRun),
							Success:          true,
							SessionID:        args.SessionID,
							Message:          "Hello back!",
						}, nil
					},
					Logger: logging.NewTestLogger(),
				}
			},
		},
		{
			name: "invalid args type",
			args: api.ToolInput{
				SessionID: "test-session",
				Data:      map[string]interface{}{
					// Missing message field
				},
			},
			setup:       func() *ChatTool { return &ChatTool{Logger: logging.NewTestLogger()} },
			expectedErr: "", // Should return ToolOutput with Success: false, not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.setup()
			result, err := tool.Execute(context.Background(), tt.args.(api.ToolInput))

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.False(t, result.Success)
			} else {
				assert.NoError(t, err)
				if tt.name == "invalid args type" {
					// For validation errors, we should get a failed ToolOutput
					assert.False(t, result.Success)
					assert.Contains(t, result.Error, "message parameter is required")
				} else {
					assert.True(t, result.Success)
				}
			}
		})
	}
}

func TestChatTool_ExecuteTyped(t *testing.T) {
	tests := []struct {
		name     string
		args     ChatToolArgs
		setup    func() *ChatTool
		validate func(t *testing.T, result *ChatToolResult, err error)
	}{
		{
			name: "successful execution",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "Test message",
				SessionID:    "test-session",
			},
			setup: func() *ChatTool {
				return &ChatTool{
					Handler: func(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
						return &ChatToolResult{
							BaseToolResponse: shared.NewBaseResponse("chat", args.SessionID, args.DryRun),
							Success:          true,
							SessionID:        args.SessionID,
							Message:          "Response message",
							Stage:            "conversation",
							Status:           "active",
						}, nil
					},
					Logger: logging.NewTestLogger(),
				}
			},
			validate: func(t *testing.T, result *ChatToolResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.Success)
				assert.Equal(t, "test-session", result.SessionID)
				assert.Equal(t, "Response message", result.Message)
				assert.Equal(t, "conversation", result.Stage)
				assert.Equal(t, "active", result.Status)
			},
		},
		{
			name: "handler error",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "Test message",
				SessionID:    "test-session",
			},
			setup: func() *ChatTool {
				return &ChatTool{
					Handler: func(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
						return nil, errors.Validation("test", "handler error")
					},
					Logger: logging.NewTestLogger(),
				}
			},
			validate: func(t *testing.T, result *ChatToolResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.False(t, result.Success)
				assert.Contains(t, result.Message, "handler error")
			},
		},
		{
			name: "with optional fields",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "Test message",
				SessionID:    "chat-session-123",
			},
			setup: func() *ChatTool {
				return &ChatTool{
					Handler: func(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
						return &ChatToolResult{
							BaseToolResponse: shared.NewBaseResponse("chat", args.SessionID, args.DryRun),
							Success:          true,
							SessionID:        args.SessionID,
							Message:          "Response with options",
							Options: []map[string]interface{}{
								{"action": "continue", "label": "Continue conversation"},
								{"action": "restart", "label": "Start over"},
							},
							NextSteps: []string{"step1", "step2"},
							Progress:  map[string]interface{}{"completed": 50},
						}, nil
					},
					Logger: logging.NewTestLogger(),
				}
			},
			validate: func(t *testing.T, result *ChatToolResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.Success)
				assert.Len(t, result.Options, 2)
				assert.Len(t, result.NextSteps, 2)
				assert.NotNil(t, result.Progress)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.setup()
			result, err := tool.ExecuteTyped(context.Background(), tt.args)
			tt.validate(t, result, err)
		})
	}
}

func TestChatTool_GetMetadata(t *testing.T) {
	tool := &ChatTool{Logger: logging.NewTestLogger()}
	metadata := tool.GetMetadata()

	assert.Equal(t, "chat", metadata.Name)
	assert.Equal(t, "Interactive chat tool for conversation mode with AI assistance", metadata.Description)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, api.ToolCategory("Communication"), metadata.Category)

	assert.Contains(t, metadata.Dependencies, "AI Handler")
	assert.Contains(t, metadata.Dependencies, "Session Management")

	assert.Contains(t, metadata.Capabilities, "Interactive conversation")
	assert.Contains(t, metadata.Capabilities, "Session continuity")
	assert.Contains(t, metadata.Capabilities, "Multi-turn dialogue")

	assert.Contains(t, metadata.Requirements, "Valid message content")
	assert.Contains(t, metadata.Requirements, "AI handler function")

	// Note: Parameters and Examples fields were removed from ToolMetadata
	// These tests have been removed as they reference non-existent fields
}

func TestChatTool_Validate(t *testing.T) {
	tests := []struct {
		name        string
		args        interface{}
		setup       func() *ChatTool
		expectedErr string
	}{
		{
			name: "valid args",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "Valid message",
				SessionID:    "valid-session-id",
			},
			setup: func() *ChatTool {
				return &ChatTool{
					Handler: func(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
						return &ChatToolResult{}, nil
					},
					Logger: logging.NewTestLogger(),
				}
			},
		},
		{
			name:        "invalid args type",
			args:        "invalid",
			setup:       func() *ChatTool { return &ChatTool{Logger: logging.NewTestLogger()} },
			expectedErr: "invalid arguments type",
		},
		{
			name: "empty message",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "",
			},
			setup:       func() *ChatTool { return &ChatTool{Logger: logging.NewTestLogger()} },
			expectedErr: "message is required and cannot be empty",
		},
		{
			name: "message too long",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      string(make([]byte, 10001)), // 10,001 characters
			},
			setup:       func() *ChatTool { return &ChatTool{Logger: logging.NewTestLogger()} },
			expectedErr: "message is too long",
		},
		{
			name: "session ID too short",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "Valid message",
				SessionID:    "ab", // Too short
			},
			setup:       func() *ChatTool { return &ChatTool{Logger: logging.NewTestLogger()} },
			expectedErr: "session_id must be between 3 and 100 characters",
		},
		{
			name: "session ID too long",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "Valid message",
				SessionID:    string(make([]byte, 101)), // Too long
			},
			setup:       func() *ChatTool { return &ChatTool{Logger: logging.NewTestLogger()} },
			expectedErr: "session_id must be between 3 and 100 characters",
		},
		{
			name: "no handler configured",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test-session"},
				Message:      "Valid message",
			},
			setup: func() *ChatTool {
				return &ChatTool{
					Handler: nil, // No handler
					Logger:  logging.NewTestLogger(),
				}
			},
			expectedErr: "chat handler is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.setup()
			err := tool.Validate(context.Background(), tt.args)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChatToolArgs_Validation(t *testing.T) {
	tests := []struct {
		name string
		args ChatToolArgs
	}{
		{
			name: "minimal valid args",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{SessionID: "test"},
				Message:      "Hello",
			},
		},
		{
			name: "full valid args",
			args: ChatToolArgs{
				BaseToolArgs: shared.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true,
				},
				Message:   "Hello, world!",
				SessionID: "chat-session-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.args.Message)
			assert.NotEmpty(t, tt.args.BaseToolArgs.SessionID)
		})
	}
}

func TestChatToolResult_Structure(t *testing.T) {
	result := &ChatToolResult{
		BaseToolResponse: shared.NewBaseResponse("chat", "session-123", false),
		Success:          true,
		SessionID:        "session-123",
		Message:          "Test response",
		Stage:            "conversation",
		Status:           "active",
		Options: []map[string]interface{}{
			{"action": "continue", "label": "Continue"},
		},
		NextSteps: []string{"step1", "step2"},
		Progress:  map[string]interface{}{"completed": 75},
	}

	assert.True(t, result.Success)
	assert.Equal(t, "session-123", result.SessionID)
	assert.Equal(t, "Test response", result.Message)
	assert.Equal(t, "conversation", result.Stage)
	assert.Equal(t, "active", result.Status)
	assert.Len(t, result.Options, 1)
	assert.Len(t, result.NextSteps, 2)
	assert.NotNil(t, result.Progress)
}
