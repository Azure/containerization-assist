package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/runtime/conversation"
	"github.com/Azure/container-copilot/pkg/mcp/internal/session/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/preference"
	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

// UnifiedMCPServer provides both chat and workflow capabilities
type UnifiedMCPServer struct {
	// Chat mode components
	promptManager  *conversation.PromptManager
	sessionManager *session.SessionManager

	// Workflow mode components
	workflowOrchestrator *orchestration.WorkflowOrchestrator
	workflowEngine       *workflow.Engine

	// Shared components
	toolRegistry     *orchestration.MCPToolRegistry
	toolOrchestrator *orchestration.MCPToolOrchestrator

	// Server state
	currentMode ServerMode
	logger      zerolog.Logger
}

// ServerMode defines the operational mode of the server
type ServerMode string

const (
	ModeDual     ServerMode = "dual"     // Both interfaces available
	ModeChat     ServerMode = "chat"     // Chat-only mode
	ModeWorkflow ServerMode = "workflow" // Workflow-only mode
)

// ServerCapabilities defines what the server can do
type ServerCapabilities struct {
	ChatSupport     bool     `json:"chat_support"`
	WorkflowSupport bool     `json:"workflow_support"`
	AvailableModes  []string `json:"available_modes"`
	SharedTools     []string `json:"shared_tools"`
}

// NewUnifiedMCPServer creates a new unified MCP server
func NewUnifiedMCPServer(
	db *bbolt.DB,
	logger zerolog.Logger,
	mode ServerMode,
) (*UnifiedMCPServer, error) {
	// Create shared components
	toolRegistry := orchestration.NewMCPToolRegistry(logger)

	// Create session manager with temporary directory
	sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      "/tmp/mcp-sessions",
		MaxSessions:       100,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,      // 1GB per session
		TotalDiskLimit:    10 * 1024 * 1024 * 1024, // 10GB total
		StorePath:         "/tmp/mcp-sessions.db",
		Logger:            logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	// Create a direct session manager implementation for the tool orchestrator
	sessionMgrImpl := &directSessionManager{sessionManager: sessionManager}
	toolOrchestrator := orchestration.NewMCPToolOrchestrator(toolRegistry, sessionMgrImpl, logger)

	server := &UnifiedMCPServer{
		toolRegistry:     toolRegistry,
		toolOrchestrator: toolOrchestrator,
		sessionManager:   sessionManager,
		currentMode:      mode,
		logger:           logger.With().Str("component", "unified_mcp_server").Logger(),
	}

	// Initialize chat components if needed
	if mode == ModeDual || mode == ModeChat {
		preferenceStore, err := preference.NewPreferenceStore("/tmp/mcp-preferences.db", logger, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create preference store: %w", err)
		}

		// Create conversation adapter for MCP tool orchestrator
		conversationOrchestrator := &ConversationOrchestratorAdapter{
			toolOrchestrator: toolOrchestrator,
			logger:           logger,
		}

		server.promptManager = conversation.NewPromptManager(conversation.PromptManagerConfig{
			SessionManager:   sessionManager,
			ToolOrchestrator: conversationOrchestrator,
			PreferenceStore:  preferenceStore,
			Logger:           logger,
		})
	}

	// Initialize workflow components if needed
	if mode == ModeDual || mode == ModeWorkflow {
		server.workflowOrchestrator = orchestration.NewWorkflowOrchestrator(
			db, toolRegistry, toolOrchestrator, logger)
	}

	server.logger.Info().
		Str("mode", string(mode)).
		Msg("Initialized unified MCP server")

	return server, nil
}

// GetCapabilities returns the server's capabilities
func (s *UnifiedMCPServer) GetCapabilities() ServerCapabilities {
	capabilities := ServerCapabilities{
		SharedTools: s.toolRegistry.ListTools(),
	}

	switch s.currentMode {
	case ModeDual:
		capabilities.ChatSupport = true
		capabilities.WorkflowSupport = true
		capabilities.AvailableModes = []string{"chat", "workflow"}
	case ModeChat:
		capabilities.ChatSupport = true
		capabilities.WorkflowSupport = false
		capabilities.AvailableModes = []string{"chat"}
	case ModeWorkflow:
		capabilities.ChatSupport = false
		capabilities.WorkflowSupport = true
		capabilities.AvailableModes = []string{"workflow"}
	}

	return capabilities
}

// GetAvailableTools returns tools available based on current mode
func (s *UnifiedMCPServer) GetAvailableTools() []ToolDefinition {
	var tools []ToolDefinition

	// Add mode-specific tools
	if s.currentMode == ModeDual || s.currentMode == ModeChat {
		tools = append(tools, s.getChatModeTools()...)
	}

	if s.currentMode == ModeDual || s.currentMode == ModeWorkflow {
		tools = append(tools, s.getWorkflowModeTools()...)
	}

	// Add shared atomic tools (always available)
	tools = append(tools, s.getAtomicTools()...)

	return tools
}

// ExecuteTool executes a tool based on the current mode and tool name
func (s *UnifiedMCPServer) ExecuteTool(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
) (interface{}, error) {
	s.logger.Info().
		Str("tool_name", toolName).
		Str("mode", string(s.currentMode)).
		Msg("Executing tool")

	// Route to appropriate handler based on tool name
	switch {
	case toolName == "chat":
		if s.currentMode != ModeChat && s.currentMode != ModeDual {
			return nil, fmt.Errorf("chat mode not available in %s mode", s.currentMode)
		}
		return s.executeChatTool(ctx, args)

	case toolName == "execute_workflow":
		if s.currentMode != ModeWorkflow && s.currentMode != ModeDual {
			return nil, fmt.Errorf("workflow mode not available in %s mode", s.currentMode)
		}
		return s.executeWorkflowTool(ctx, args)

	case toolName == "list_workflows":
		if s.currentMode != ModeWorkflow && s.currentMode != ModeDual {
			return nil, fmt.Errorf("workflow mode not available in %s mode", s.currentMode)
		}
		return s.listWorkflows()

	case toolName == "get_workflow_status":
		if s.currentMode != ModeWorkflow && s.currentMode != ModeDual {
			return nil, fmt.Errorf("workflow mode not available in %s mode", s.currentMode)
		}
		return s.getWorkflowStatus(args)

	case s.isAtomicTool(toolName):
		// Atomic tools are available in all modes
		return s.toolOrchestrator.ExecuteTool(ctx, toolName, args, nil)

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Chat mode tool definitions
func (s *UnifiedMCPServer) getChatModeTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "chat",
			Description: "Interactive chat interface for exploring and executing tools",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Your message or question",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional session ID for conversation continuity",
					},
					"context": map[string]interface{}{
						"type":        "object",
						"description": "Additional context for the conversation",
					},
				},
				"required": []string{"message"},
			},
		},
		{
			Name:        "list_conversation_history",
			Description: "List previous conversations and their outcomes",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID to get history for",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of entries to return",
					},
				},
			},
		},
	}
}

// Workflow mode tool definitions
func (s *UnifiedMCPServer) getWorkflowModeTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "execute_workflow",
			Description: "Execute a declarative workflow specification",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workflow_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of predefined workflow to execute",
					},
					"workflow_spec": map[string]interface{}{
						"type":        "object",
						"description": "Custom workflow specification",
					},
					"variables": map[string]interface{}{
						"type":        "object",
						"description": "Variables to pass to the workflow",
					},
					"options": map[string]interface{}{
						"type":        "object",
						"description": "Execution options (dry_run, checkpoints, etc.)",
					},
				},
			},
		},
		{
			Name:        "list_workflows",
			Description: "List available predefined workflows",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Filter by workflow category",
					},
				},
			},
		},
		{
			Name:        "get_workflow_status",
			Description: "Get the status of a running workflow",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Workflow session ID",
						"required":    true,
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			Name:        "pause_workflow",
			Description: "Pause a running workflow",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":     "string",
						"required": true,
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			Name:        "resume_workflow",
			Description: "Resume a paused workflow",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":     "string",
						"required": true,
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			Name:        "cancel_workflow",
			Description: "Cancel a running workflow",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":     "string",
						"required": true,
					},
				},
				"required": []string{"session_id"},
			},
		},
	}
}

// Get atomic tool definitions
func (s *UnifiedMCPServer) getAtomicTools() []ToolDefinition {
	var tools []ToolDefinition

	for _, toolName := range s.toolRegistry.ListTools() {
		if metadata, err := s.toolRegistry.GetToolMetadata(toolName); err == nil {
			tools = append(tools, ToolDefinition{
				Name:        toolName,
				Description: metadata.Description,
				InputSchema: s.buildInputSchema(metadata),
			})
		}
	}

	return tools
}

// Execute chat tool
func (s *UnifiedMCPServer) executeChatTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	message, ok := args["message"].(string)
	if !ok {
		return nil, fmt.Errorf("message is required and must be a string")
	}

	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		sessionID = "default"
	}

	// Route to prompt manager
	return s.promptManager.ProcessPrompt(ctx, sessionID, message)
}

// Execute workflow tool
func (s *UnifiedMCPServer) executeWorkflowTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Handle predefined workflow execution
	if workflowName, ok := args["workflow_name"].(string); ok {
		variables, _ := args["variables"].(map[string]string)

		var options []workflow.ExecutionOption
		if vars := variables; vars != nil {
			options = append(options, workflow.WithVariables(vars))
		}

		return s.workflowOrchestrator.ExecuteWorkflow(ctx, workflowName, options...)
	}

	// Handle custom workflow execution
	if workflowSpec, ok := args["workflow_spec"].(map[string]interface{}); ok {
		// Convert map to WorkflowSpec
		specBytes, err := json.Marshal(workflowSpec)
		if err != nil {
			return nil, fmt.Errorf("invalid workflow specification: %w", err)
		}

		var spec workflow.WorkflowSpec
		if err := json.Unmarshal(specBytes, &spec); err != nil {
			return nil, fmt.Errorf("failed to parse workflow specification: %w", err)
		}

		return s.workflowOrchestrator.ExecuteCustomWorkflow(ctx, &spec)
	}

	return nil, fmt.Errorf("either workflow_name or workflow_spec is required")
}

// List available workflows
func (s *UnifiedMCPServer) listWorkflows() (interface{}, error) {
	return orchestration.ListAvailableWorkflows(), nil
}

// Get workflow status
func (s *UnifiedMCPServer) getWorkflowStatus(args map[string]interface{}) (interface{}, error) {
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return nil, fmt.Errorf("session_id is required")
	}

	return s.workflowOrchestrator.GetWorkflowStatus(sessionID)
}

// Check if a tool is an atomic tool
func (s *UnifiedMCPServer) isAtomicTool(toolName string) bool {
	atomicTools := s.toolRegistry.ListTools()
	for _, tool := range atomicTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// Build input schema from tool metadata
func (s *UnifiedMCPServer) buildInputSchema(metadata *orchestration.ToolMetadata) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID for tracking",
				"required":    true,
			},
		},
		"required": []string{"session_id"},
	}

	// Add tool-specific properties from metadata
	if params, ok := metadata.Parameters["fields"].(map[string]interface{}); ok {
		properties := schema["properties"].(map[string]interface{})
		for fieldName, fieldInfo := range params {
			properties[fieldName] = fieldInfo
		}
	}

	return schema
}

// ToolDefinition represents a tool definition for MCP
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// directSessionManager provides direct implementation of orchestration.SessionManager interface
// This replaces the SessionManagerAdapter pattern with direct calls
type directSessionManager struct {
	sessionManager *session.SessionManager
}

func (dsm *directSessionManager) GetSession(sessionID string) (interface{}, error) {
	return dsm.sessionManager.GetOrCreateSession(sessionID)
}

func (dsm *directSessionManager) UpdateSession(session interface{}) error {
	// Direct session updates are handled internally by the session manager
	// The orchestration layer doesn't need to explicitly update sessions
	return nil
}

// ConversationOrchestratorAdapter adapts MCPToolOrchestrator to conversation.ToolOrchestrator interface
type ConversationOrchestratorAdapter struct {
	toolOrchestrator *orchestration.MCPToolOrchestrator
	logger           zerolog.Logger
}

func (adapter *ConversationOrchestratorAdapter) ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error) {
	// Execute tool using MCP orchestrator
	// The session can be either a string sessionID or a session object
	result, err := adapter.toolOrchestrator.ExecuteTool(ctx, toolName, args, session)
	if err != nil {
		return nil, err
	}

	// Return result directly
	return result, nil
}

func (adapter *ConversationOrchestratorAdapter) ValidateToolArgs(toolName string, args interface{}) error {
	return adapter.toolOrchestrator.ValidateToolArgs(toolName, args)
}

func (adapter *ConversationOrchestratorAdapter) GetToolMetadata(toolName string) (*orchestration.ToolMetadata, error) {
	return adapter.toolOrchestrator.GetToolMetadata(toolName)
}
