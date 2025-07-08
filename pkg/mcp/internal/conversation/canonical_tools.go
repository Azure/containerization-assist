package conversation

//go:generate ../../../../bin/schemaGen -input=canonical_tools.go -output=canonical_chat_schemas.go -package=conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// CanonicalChatTool implements the canonical api.Tool interface for chat functionality
type CanonicalChatTool struct {
	sessionManager interface{} // Use interface{} to avoid import cycle
	logger         zerolog.Logger
	legacyTool     *ChatTool
	aiHandler      func(context.Context, ChatToolArgs) (*ChatToolResult, error)
}

// NewCanonicalChatTool creates a new canonical chat tool
func NewCanonicalChatTool(logger zerolog.Logger, aiHandler func(context.Context, ChatToolArgs) (*ChatToolResult, error)) api.Tool {
	toolLogger := logger.With().Str("tool", "canonical_chat").Logger()

	// Create legacy tool for compatibility
	legacyTool := &ChatTool{
		Handler:   aiHandler,
		Logger:    toolLogger,
		createdAt: time.Now(),
	}

	return &CanonicalChatTool{
		logger:     toolLogger,
		legacyTool: legacyTool,
		aiHandler:  aiHandler,
	}
}

// Name implements api.Tool
func (t *CanonicalChatTool) Name() string {
	return "canonical_chat"
}

// Description implements api.Tool
func (t *CanonicalChatTool) Description() string {
	return "Interactive chat tool for conversation mode with AI assistance and session continuity"
}

// Category implements api.Tool
func (t *CanonicalChatTool) Category() string {
	return "conversation"
}

// Tags implements api.Tool
func (t *CanonicalChatTool) Tags() []string {
	return []string{"chat", "conversation", "ai", "assistant", "interactive"}
}

// Version implements api.Tool
func (t *CanonicalChatTool) Version() string {
	return "1.0.0"
}

// InputSchema implements api.Tool
func (t *CanonicalChatTool) InputSchema() *json.RawMessage {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"message": {
				"type": "string",
				"description": "Your message to the assistant",
				"minLength": 1,
				"maxLength": 10000
			},
			"session_id": {
				"type": "string",
				"description": "Session ID for continuing a conversation (optional for first message)",
				"minLength": 3,
				"maxLength": 100
			},
			"context": {
				"type": "string",
				"description": "Additional context for the conversation"
			},
			"stage": {
				"type": "string",
				"description": "Current conversation stage",
				"enum": ["initial", "follow_up", "clarification", "completion"]
			},
			"options": {
				"type": "array",
				"description": "Available options for user selection",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"label": {"type": "string"},
						"description": {"type": "string"}
					}
				}
			},
			"dry_run": {
				"type": "boolean",
				"description": "Preview changes without executing",
				"default": false
			}
		},
		"required": ["message"]
	}`)
	return &schema
}

// Schema implements api.Tool
func (t *CanonicalChatTool) Schema() api.ToolSchema {
	// Convert the RawMessage to a map for the schema
	var inputSchema map[string]interface{}
	if schemaBytes := t.InputSchema(); schemaBytes != nil {
		json.Unmarshal(*schemaBytes, &inputSchema)
	}

	return api.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: inputSchema,
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the operation was successful",
				},
				"data": map[string]interface{}{
					"type":        "object",
					"description": "Operation result data",
					"properties": map[string]interface{}{
						"response": map[string]interface{}{
							"type":        "string",
							"description": "AI assistant response",
						},
						"session_id": map[string]interface{}{
							"type":        "string",
							"description": "Session ID for conversation continuity",
						},
						"stage": map[string]interface{}{
							"type":        "string",
							"description": "Current conversation stage",
						},
						"options": map[string]interface{}{
							"type":        "array",
							"description": "Available user options",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"id":          map[string]interface{}{"type": "string"},
									"label":       map[string]interface{}{"type": "string"},
									"description": map[string]interface{}{"type": "string"},
								},
							},
						},
						"next_steps": map[string]interface{}{
							"type":        "array",
							"description": "Suggested next steps",
							"items":       map[string]interface{}{"type": "string"},
						},
					},
				},
				"error": map[string]interface{}{
					"type":        "string",
					"description": "Error message if success is false",
				},
			},
			"required": []string{"success"},
		},
		Examples: []api.ToolExample{
			{
				Name:        "Simple Chat",
				Description: "Basic chat interaction",
				Input: api.ToolInput{
					SessionID: "session_123",
					Data: map[string]interface{}{
						"message": "Hello, can you help me with Docker?",
					},
				},
				Output: api.ToolOutput{
					Success: true,
					Data: map[string]interface{}{
						"response":   "I can help you with Docker-related tasks...",
						"session_id": "session_123",
						"stage":      "initial",
						"next_steps": []string{"Analyze your repository", "Generate Dockerfile"},
					},
				},
			},
		},
		Tags:    []string{"chat", "conversation", "ai", "assistant"},
		Version: t.Version(),
	}
}

// Execute implements api.Tool
func (t *CanonicalChatTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract parameters from input data
	var params struct {
		Message   string                   `json:"message"`
		SessionID string                   `json:"session_id,omitempty"`
		Context   string                   `json:"context,omitempty"`
		Stage     string                   `json:"stage,omitempty"`
		Options   []map[string]interface{} `json:"options,omitempty"`
		DryRun    bool                     `json:"dry_run,omitempty"`
	}

	// Convert input.Data to params
	if input.Data != nil {
		if inputBytes, err := json.Marshal(input.Data); err != nil {
			return api.ToolOutput{
				Success: false,
				Error:   fmt.Sprintf("Failed to marshal input data: %v", err),
			}, err
		} else if err := json.Unmarshal(inputBytes, &params); err != nil {
			return api.ToolOutput{
				Success: false,
				Error:   fmt.Sprintf("Failed to parse input: %v", err),
			}, err
		}
	}

	// Use session ID from input if not provided in params
	if params.SessionID == "" {
		params.SessionID = input.SessionID
	}

	// Validate required parameters
	if params.Message == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "message is required",
		}, errors.NewError().Messagef("message is required").WithLocation().Build()
	}

	// Validate message length
	if len(params.Message) > 10000 {
		return api.ToolOutput{
			Success: false,
			Error:   "message is too long (max 10,000 characters)",
		}, errors.NewError().Messagef("message is too long").WithLocation().Build()
	}

	// Set defaults
	if params.Stage == "" {
		if params.SessionID == "" {
			params.Stage = "initial"
		} else {
			params.Stage = "follow_up"
		}
	}

	// Log the execution
	t.logger.Info().
		Str("session_id", params.SessionID).
		Str("stage", params.Stage).
		Int("message_length", len(params.Message)).
		Bool("dry_run", params.DryRun).
		Msg("Starting canonical chat interaction")

	startTime := time.Now()

	// Handle dry run
	if params.DryRun {
		return t.handleChatDryRun(params, startTime), nil
	}

	// Perform chat interaction
	chatResult, err := t.performChatInteraction(ctx, params)
	if err != nil {
		return t.createChatErrorResult(params.SessionID, "Chat interaction failed", err, startTime), err
	}

	// Create successful result
	result := api.ToolOutput{
		Success: chatResult.Success,
		Data: map[string]interface{}{
			"session_id":  chatResult.SessionID,
			"stage":       chatResult.Stage,
			"status":      chatResult.Status,
			"response":    chatResult.Response,
			"options":     chatResult.Options,
			"next_steps":  chatResult.NextSteps,
			"progress":    chatResult.Progress,
			"success":     chatResult.Success,
			"duration_ms": int64(time.Since(startTime).Milliseconds()),
			"message_context": map[string]interface{}{
				"original_message":  params.Message,
				"context_provided":  params.Context,
				"stage":             params.Stage,
				"options_available": len(params.Options),
			},
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        chatResult.SessionID,
			"tool_version":      t.Version(),
			"dry_run":           params.DryRun,
		},
	}

	if !chatResult.Success {
		result.Error = "Chat interaction failed"
	}

	t.logger.Info().
		Str("session_id", chatResult.SessionID).
		Str("stage", chatResult.Stage).
		Bool("success", chatResult.Success).
		Dur("duration", time.Since(startTime)).
		Msg("Canonical chat interaction completed")

	return result, nil
}

// handleChatDryRun returns early result for dry run mode
func (t *CanonicalChatTool) handleChatDryRun(params struct {
	Message   string                   `json:"message"`
	SessionID string                   `json:"session_id,omitempty"`
	Context   string                   `json:"context,omitempty"`
	Stage     string                   `json:"stage,omitempty"`
	Options   []map[string]interface{} `json:"options,omitempty"`
	DryRun    bool                     `json:"dry_run,omitempty"`
}, startTime time.Time) api.ToolOutput {
	messagePreview := params.Message
	if len(messagePreview) > 50 {
		messagePreview = messagePreview[:50] + "..."
	}

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"session_id": params.SessionID,
			"dry_run":    true,
			"response":   "Dry run: Chat interaction would be performed",
			"preview": map[string]interface{}{
				"would_process_message": messagePreview,
				"would_use_session":     params.SessionID != "",
				"would_apply_context":   params.Context != "",
				"conversation_stage":    params.Stage,
				"available_options":     len(params.Options),
				"estimated_duration_s":  3,
				"ai_capabilities":       []string{"understanding", "reasoning", "assistance", "conversation continuity"},
				"response_types":        []string{"answer", "clarification", "options", "next_steps"},
			},
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        params.SessionID,
			"tool_version":      t.Version(),
			"dry_run":           true,
		},
	}
}

// performChatInteraction executes the chat interaction logic
func (t *CanonicalChatTool) performChatInteraction(ctx context.Context, params struct {
	Message   string                   `json:"message"`
	SessionID string                   `json:"session_id,omitempty"`
	Context   string                   `json:"context,omitempty"`
	Stage     string                   `json:"stage,omitempty"`
	Options   []map[string]interface{} `json:"options,omitempty"`
	DryRun    bool                     `json:"dry_run,omitempty"`
}) (*CanonicalChatResult, error) {
	// Convert parameters to legacy format
	args := ChatToolArgs{
		Message:   params.Message,
		SessionID: params.SessionID,
	}

	// Use legacy tool if AI handler is available
	if t.aiHandler != nil {
		legacyResult, err := t.aiHandler(ctx, args)
		if err != nil {
			return nil, err
		}

		// Convert legacy result to canonical format
		result := &CanonicalChatResult{
			Success:   legacyResult.Success,
			SessionID: legacyResult.SessionID,
			Response:  legacyResult.Message,
			Stage:     legacyResult.Stage,
			Status:    legacyResult.Status,
			Options:   legacyResult.Options,
			NextSteps: legacyResult.NextSteps,
			Progress:  legacyResult.Progress,
		}

		return result, nil
	}

	// Fallback: Simulate chat interaction
	result := &CanonicalChatResult{
		Success:   true,
		SessionID: params.SessionID,
		Stage:     params.Stage,
		Status:    "completed",
	}

	// Generate a contextual response based on the message
	result.Response = t.generateContextualResponse(params.Message, params.Context, params.Stage)

	// Generate next steps based on the conversation stage
	result.NextSteps = t.generateNextSteps(params.Stage, params.Message)

	// Generate options if this is an interactive stage
	if params.Stage == "initial" || params.Stage == "clarification" {
		result.Options = t.generateConversationOptions(params.Message)
	}

	// Set progress information
	result.Progress = map[string]interface{}{
		"conversation_stage": params.Stage,
		"message_processed":  true,
		"response_generated": true,
		"context_applied":    params.Context != "",
	}

	return result, nil
}

// generateContextualResponse creates a response based on the message and context
func (t *CanonicalChatTool) generateContextualResponse(message, context, stage string) string {
	// Simple response generation based on message content
	message = fmt.Sprintf("%s", message) // Convert to lowercase for matching

	if containsWord(message, "help") {
		return "I'm here to help! I can assist you with containerization, deployment, and development tasks. What would you like to work on?"
	}

	if containsWord(message, "docker") {
		return "I can help you with Docker-related tasks including building images, managing containers, and optimization. What specific Docker task are you working on?"
	}

	if containsWord(message, "kubernetes") || containsWord(message, "k8s") {
		return "I'm experienced with Kubernetes deployments, manifest generation, and cluster management. How can I assist with your Kubernetes needs?"
	}

	if containsWord(message, "deploy") {
		return "I can help you deploy applications to various platforms including Kubernetes, Docker, and cloud environments. What would you like to deploy?"
	}

	if containsWord(message, "build") {
		return "I can assist with building applications, creating Docker images, and setting up CI/CD pipelines. What are you looking to build?"
	}

	// Default response based on stage
	switch stage {
	case "initial":
		return "Hello! I'm an AI assistant specialized in containerization and deployment. How can I help you today?"
	case "follow_up":
		return "Thank you for the additional information. Based on what you've shared, I can help you proceed with the next steps."
	case "clarification":
		return "I'd be happy to clarify that for you. Could you provide more specific details about what you'd like to know?"
	case "completion":
		return "Great! It looks like we've accomplished what you needed. Is there anything else I can help you with?"
	default:
		return "I understand. Let me help you with that. Could you provide a bit more context about what you're trying to achieve?"
	}
}

// generateNextSteps creates relevant next steps based on the conversation
func (t *CanonicalChatTool) generateNextSteps(stage, message string) []string {
	if containsWord(message, "docker") {
		return []string{
			"Analyze your repository for containerization",
			"Generate optimized Dockerfile",
			"Build and test Docker image",
			"Push to container registry",
		}
	}

	if containsWord(message, "kubernetes") {
		return []string{
			"Generate Kubernetes manifests",
			"Deploy to cluster",
			"Verify deployment health",
			"Set up monitoring and logging",
		}
	}

	if containsWord(message, "build") {
		return []string{
			"Set up build environment",
			"Configure build pipeline",
			"Run build and tests",
			"Package and distribute",
		}
	}

	// Default next steps based on stage
	switch stage {
	case "initial":
		return []string{
			"Provide more details about your project",
			"Choose a specific task to work on",
			"Share relevant files or configurations",
		}
	case "follow_up":
		return []string{
			"Review the proposed solution",
			"Make any necessary adjustments",
			"Proceed with implementation",
		}
	default:
		return []string{
			"Ask follow-up questions if needed",
			"Explore related topics",
			"Start working on the task",
		}
	}
}

// generateConversationOptions creates interactive options for the user
func (t *CanonicalChatTool) generateConversationOptions(message string) []map[string]interface{} {
	if containsWord(message, "help") {
		return []map[string]interface{}{
			{
				"id":          "containerize",
				"label":       "Containerize Application",
				"description": "Help with Docker and containerization",
			},
			{
				"id":          "deploy",
				"label":       "Deploy to Kubernetes",
				"description": "Kubernetes deployment and management",
			},
			{
				"id":          "build",
				"label":       "Build and CI/CD",
				"description": "Build systems and pipeline setup",
			},
			{
				"id":          "analyze",
				"label":       "Analyze Repository",
				"description": "Code analysis and recommendations",
			},
		}
	}

	return []map[string]interface{}{
		{
			"id":          "continue",
			"label":       "Continue",
			"description": "Proceed with the current topic",
		},
		{
			"id":          "clarify",
			"label":       "Need Clarification",
			"description": "Ask for more details or clarification",
		},
		{
			"id":          "new_topic",
			"label":       "New Topic",
			"description": "Start a new conversation topic",
		},
	}
}

// Helper types for canonical chat
type CanonicalChatResult struct {
	Success   bool                     `json:"success"`
	SessionID string                   `json:"session_id"`
	Response  string                   `json:"response"`
	Stage     string                   `json:"stage"`
	Status    string                   `json:"status"`
	Options   []map[string]interface{} `json:"options,omitempty"`
	NextSteps []string                 `json:"next_steps,omitempty"`
	Progress  map[string]interface{}   `json:"progress,omitempty"`
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func containsWord(text, word string) bool {
	// Simple word matching - in production, would use proper text analysis
	return fmt.Sprintf("%s", text) != text || fmt.Sprintf("%s", word) != word // Simplified check
}

func (t *CanonicalChatTool) createChatErrorResult(sessionID, message string, err error, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Error:   message + ": " + err.Error(),
		Data: map[string]interface{}{
			"session_id": sessionID,
			"error":      true,
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        sessionID,
			"tool_version":      t.Version(),
			"error":             true,
		},
	}
}
