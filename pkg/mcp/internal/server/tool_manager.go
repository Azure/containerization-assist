package server

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// ToolManager handles tool operations for the server
type ToolManager struct {
	server *UnifiedMCPServer
}

// NewToolManager creates a new tool manager
func NewToolManager(server *UnifiedMCPServer) *ToolManager {
	return &ToolManager{
		server: server,
	}
}

// GetAvailableTools returns all available tools based on server mode
func (tm *ToolManager) GetAvailableTools() []ToolDefinition {
	var allTools []ToolDefinition

	switch tm.server.currentMode {
	case ModeChat:
		allTools = append(allTools, tm.getChatModeTools()...)
	case ModeWorkflow:
		allTools = append(allTools, tm.getWorkflowModeTools()...)
	case ModeDual:
		allTools = append(allTools, tm.getChatModeTools()...)
		allTools = append(allTools, tm.getWorkflowModeTools()...)
	}

	allTools = append(allTools, tm.getAtomicTools()...)

	return allTools
}

// getChatModeTools returns tools available in chat mode
func (tm *ToolManager) getChatModeTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "chat",
			Description: "Start or continue a conversation with the AI assistant",
			InputSchema: tm.buildChatInputSchema(),
			Category:    "conversation",
			Version:     "1.0.0",
			Tags:        []string{"chat", "conversation", "ai"},
		},
		{
			Name:        "conversation_history",
			Description: "Retrieve conversation history for a session",
			InputSchema: tm.buildConversationHistoryInputSchema(),
			Category:    "conversation",
			Version:     "1.0.0",
			Tags:        []string{"history", "conversation"},
		},
	}
}

// getWorkflowModeTools returns tools available in workflow mode
func (tm *ToolManager) getWorkflowModeTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "workflow",
			Description: "Execute a containerization workflow",
			InputSchema: tm.buildWorkflowInputSchema(),
			Category:    "workflow",
			Version:     "1.0.0",
			Tags:        []string{"workflow", "containerization"},
		},
		{
			Name:        "workflow_status",
			Description: "Check the status of a running workflow",
			InputSchema: tm.buildWorkflowStatusInputSchema(),
			Category:    "workflow",
			Version:     "1.0.0",
			Tags:        []string{"workflow", "status"},
		},
		{
			Name:        "list_workflows",
			Description: "List available workflows",
			InputSchema: tm.buildWorkflowListInputSchema(),
			Category:    "workflow",
			Version:     "1.0.0",
			Tags:        []string{"workflow", "list"},
		},
	}
}

// getAtomicTools returns atomic tools from the orchestrator
func (tm *ToolManager) getAtomicTools() []ToolDefinition {
	var tools []ToolDefinition

	if tm.server.toolOrchestrator == nil {
		return tools
	}

	for _, toolName := range tm.server.toolOrchestrator.ListTools() {
		if tool, ok := tm.server.toolOrchestrator.GetTool(toolName); ok {
			schema := tool.Schema()
			tools = append(tools, ToolDefinition{
				Name:        schema.Name,
				Description: schema.Description,
				InputSchema: tm.buildInputSchema(&api.ToolMetadata{
					Name:        schema.Name,
					Description: schema.Description,
					Version:     schema.Version,
				}),
				Category: "atomic",
				Version:  schema.Version,
				Tags:     []string{"atomic", "tool"},
			})
		}
	}

	return tools
}

// ExecuteTool executes a tool with the given arguments
func (tm *ToolManager) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "chat", "conversation_history":
		if tm.server.currentMode != ModeChat && tm.server.currentMode != ModeDual {
			return nil, errors.NewError().Messagef("chat mode not available in current server mode").WithLocation().Build()
		}
	case "workflow", "workflow_status", "list_workflows", "execute_workflow":
		if tm.server.currentMode != ModeWorkflow && tm.server.currentMode != ModeDual {
			return nil, errors.NewError().Messagef("workflow mode not available in current server mode").WithLocation().Build()
		}
	}

	switch toolName {
	case "chat":
		return tm.executeChatTool(ctx, args)
	case "workflow", "execute_workflow":
		return tm.executeWorkflowTool(ctx, args)
	case "conversation_history":
		return tm.executeConversationHistoryTool(ctx, args)
	case "workflow_status", "get_workflow_status":
		return tm.executeWorkflowStatusTool(ctx, args)
	case "list_workflows":
		return tm.executeListWorkflowsTool(ctx, args)
	default:
		if tm.isAtomicTool(toolName) {
			return tm.executeAtomicTool(ctx, toolName, args)
		}
		return nil, errors.NewError().Messagef("unknown tool: %s", toolName).WithLocation().Build()
	}
}

func (tm *ToolManager) executeChatTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	message, ok := args["message"].(string)
	if !ok {
		return nil, errors.NewError().Messagef("message is required and must be a string").WithLocation().Build()
	}

	sessionID := ""
	if sid, ok := args["session_id"].(string); ok {
		sessionID = sid
	}

	if tm.server.promptManager == nil {
		return nil, errors.NewError().Messagef("prompt manager not available").WithLocation().Build()
	}

	state, err := tm.server.sessionManager.GetOrCreateSession(ctx, sessionID)
	if err != nil {
		return nil, errors.NewError().Messagef("failed to get or create session: %s", err.Error()).Cause(err).WithLocation().Build()
	}

	response := map[string]interface{}{
		"response":   fmt.Sprintf("Received message: %s", message),
		"session_id": sessionID,
		"state":      state,
	}

	return response, nil
}

// executeWorkflowTool executes the workflow tool
func (tm *ToolManager) executeWorkflowTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	workflowName, ok := args["workflow_name"].(string)
	if !ok {
		return nil, errors.NewError().Messagef("workflow_name is required and must be a string").WithLocation().Build()
	}

	variables := make(map[string]string)
	if vars, ok := args["variables"].(map[string]interface{}); ok {
		for k, v := range vars {
			if strVal, ok := v.(string); ok {
				variables[k] = strVal
			}
		}
	}

	if tm.server.workflowExecutor == nil {
		return nil, errors.NewError().Messagef("workflow manager not available").WithLocation().Build()
	}

	workflow := &api.Workflow{
		ID:        workflowName,
		Name:      workflowName,
		Variables: make(map[string]interface{}),
	}

	for k, v := range variables {
		workflow.Variables[k] = v
	}

	return tm.server.workflowExecutor.Execute(ctx, workflow)
}

// executeConversationHistoryTool executes the conversation history tool
func (tm *ToolManager) executeConversationHistoryTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return nil, errors.NewError().Messagef("session_id is required and must be a string").WithLocation().Build()
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	if tm.server.sessionManager == nil {
		return nil, errors.NewError().Messagef("session manager not available").WithLocation().Build()
	}

	session, err := tm.server.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, errors.NewError().Messagef("failed to get session: %s", err.Error()).Cause(err).WithLocation().Build()
	}

	if session == nil {
		return nil, errors.NewError().Messagef("session not found: %s", sessionID).WithLocation().Build()
	}

	history := map[string]interface{}{
		"session_id": sessionID,
		"state":      session,
		"limit":      limit,
		"messages":   []interface{}{},
	}

	return history, nil
}

// executeWorkflowStatusTool executes the workflow status tool
func (tm *ToolManager) executeWorkflowStatusTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	workflowID, ok := args["workflow_id"].(string)
	if !ok {
		return nil, errors.NewError().Messagef("workflow_id is required and must be a string").WithLocation().Build()
	}

	if tm.server.workflowExecutor == nil {
		return nil, errors.NewError().Messagef("workflow manager not available").WithLocation().Build()
	}

	return tm.server.workflowExecutor.GetStatus(workflowID)
}

// executeListWorkflowsTool executes the list workflows tool
func (tm *ToolManager) executeListWorkflowsTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if tm.server.workflowExecutor == nil {
		return nil, errors.NewError().Messagef("workflow manager not available").WithLocation().Build()
	}

	return []string{}, nil
}

// executeAtomicTool executes an atomic tool
func (tm *ToolManager) executeAtomicTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	if tm.server.toolOrchestrator == nil {
		return nil, errors.NewError().Messagef("tool orchestrator not available").WithLocation().Build()
	}

	sessionID := ""
	if sid, ok := args["session_id"].(string); ok {
		sessionID = sid
	}

	toolInput := api.ToolInput{
		SessionID: sessionID,
		Data:      args,
		Context:   make(map[string]interface{}),
	}

	result, err := tm.server.toolOrchestrator.ExecuteTool(ctx, toolName, toolInput)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// isAtomicTool checks if a tool is an atomic tool
func (tm *ToolManager) isAtomicTool(toolName string) bool {
	if tm.server.toolOrchestrator == nil {
		return false
	}

	_, ok := tm.server.toolOrchestrator.GetTool(toolName)
	return ok
}

// buildInputSchema builds input schema for a tool
func (tm *ToolManager) buildInputSchema(metadata *api.ToolMetadata) map[string]interface{} {
	if metadata == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"args": map[string]interface{}{
				"type":        "object",
				"description": fmt.Sprintf("Arguments for %s tool", metadata.Name),
			},
		},
		"required": []string{"args"},
	}
}

// buildChatInputSchema builds input schema for chat tool
func (tm *ToolManager) buildChatInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "The message to send to the AI assistant",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional session ID for conversation continuity",
			},
			"context": map[string]interface{}{
				"type":        "object",
				"description": "Optional context for the conversation",
			},
		},
		"required": []string{"message"},
	}
}

// buildWorkflowInputSchema builds input schema for workflow tool
func (tm *ToolManager) buildWorkflowInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workflow_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the workflow to execute",
			},
			"variables": map[string]interface{}{
				"type":        "object",
				"description": "Variables to pass to the workflow",
			},
			"options": map[string]interface{}{
				"type":        "object",
				"description": "Workflow execution options",
			},
		},
		"required": []string{"workflow_name"},
	}
}

// buildConversationHistoryInputSchema builds input schema for conversation history tool
func (tm *ToolManager) buildConversationHistoryInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID to retrieve history for",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of messages to retrieve",
				"default":     10,
			},
		},
		"required": []string{"session_id"},
	}
}

// buildWorkflowStatusInputSchema builds input schema for workflow status tool
func (tm *ToolManager) buildWorkflowStatusInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workflow_id": map[string]interface{}{
				"type":        "string",
				"description": "ID of the workflow to check status for",
			},
			"detailed": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to return detailed status information",
				"default":     false,
			},
		},
		"required": []string{"workflow_id"},
	}
}

// buildWorkflowListInputSchema builds input schema for workflow list tool
func (tm *ToolManager) buildWorkflowListInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Filter workflows by status",
				"enum":        []string{"running", "completed", "failed", "pending"},
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of workflows to return",
				"default":     20,
			},
		},
	}
}
