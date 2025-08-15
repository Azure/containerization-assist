package tools

import (
"context"
"encoding/json"
"fmt"
"time"

"log/slog"

"github.com/mark3labs/mcp-go/mcp"
"github.com/pkg/errors"

domainworkflow "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
"github.com/mark3labs/mcp-go/server"
)

// RegisterTools registers all tools based on their configurations
func RegisterTools(mcpServer *server.MCPServer, deps ToolDependencies) error {
	for _, config := range toolConfigs {
		if err := RegisterTool(mcpServer, config, deps); err != nil {
			return errors.Wrapf(err, "failed to register tool %s", config.Name)
		}
	}
	return nil
}


// RegisterTool registers a single tool based on its configuration
func RegisterTool(mcpServer *server.MCPServer, config ToolConfig, deps ToolDependencies) error {
	// Validate dependencies
	if err := validateDependencies(config, deps); err != nil {
		return errors.Wrapf(err, "invalid dependencies for tool %s", config.Name)
	}

	// Create the tool definition
	schema := BuildToolSchema(config)
	tool := mcp.Tool{
		Name:        config.Name,
		Description: config.Description,
		InputSchema: schema,
	}

	// Debug logging for schema validation
	if deps.Logger != nil {
		if config.Name == "start_workflow" {
			// Log the actual schema being used for the problematic tool
			schemaJSON, _ := json.Marshal(schema)
			deps.Logger.Debug("Tool schema for start_workflow",
				slog.String("schema", string(schemaJSON)))
		}
	}

	// Create the handler
	var handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	if config.CustomHandler != nil {
		// Use custom handler if provided
		handler = config.CustomHandler(deps)
	} else {
		// Use generic handler based on category
		switch config.Category {
		case CategoryWorkflow:
			handler = createWorkflowHandler(config, deps)
		case CategoryOrchestration:
			handler = createOrchestrationHandler(config, deps)
		case CategoryUtility:
			handler = createUtilityHandler(config, deps)
		default:
			return errors.Errorf("unknown tool category: %s", config.Category)
		}
	}

	// Register the tool
	mcpServer.AddTool(tool, handler)

	if deps.Logger != nil {
		deps.Logger.Info("Registered tool", slog.String("name", config.Name), slog.String("category", string(config.Category)))
	}

	return nil
}

// validateDependencies ensures required dependencies are provided
func validateDependencies(config ToolConfig, deps ToolDependencies) error {
	if config.NeedsStepProvider && deps.StepProvider == nil {
		return errors.New("StepProvider is required but not provided")
	}
	if config.NeedsSessionManager && deps.SessionManager == nil {
		return errors.New("SessionManager is required but not provided")
	}
	if config.NeedsLogger && deps.Logger == nil {
		return errors.New("Logger is required but not provided")
	}
	return nil
}

// createWorkflowHandler creates a generic handler for workflow tools
func createWorkflowHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		args := req.GetArguments()
		if args == nil {
			result := createErrorResult(errors.New("missing arguments"))
			return &result, nil
		}

		// Validate required parameters
		for _, param := range config.RequiredParams {
			if _, exists := args[param]; !exists {
				result := createErrorResult(errors.Errorf("missing required parameter: %s", param))
				return &result, nil
			}
		}

		// Extract session ID
		sessionID, ok := args["session_id"].(string)
		if !ok || sessionID == "" {
			result := createErrorResult(errors.New("invalid or missing session_id"))
			return &result, nil
		}

		// Load workflow state
		state, err := LoadWorkflowState(ctx, deps.SessionManager, sessionID)
		if err != nil {
			result := createErrorResult(errors.Wrap(err, "failed to load workflow state"))
			return &result, nil
		}

		// Setup progress emitter
		progressEmitter := CreateProgressEmitter(ctx, &req, 1, deps.Logger)
		defer progressEmitter.Close()

		// For now, individual tools will handle their own execution
		// This is a placeholder for the simplified tool execution
		// The step execution pattern will be updated to work with the simplified state
		result := make(map[string]interface{})
		execErr := fmt.Errorf("step execution not yet implemented for tool %s", config.Name)
		if execErr != nil {
			state.SetError(domainworkflow.NewWorkflowError(config.Name, 1, execErr))
			// Try to save state even on error
			_ = SaveWorkflowState(ctx, deps.SessionManager, state)

			errorResult := createErrorResult(execErr)
			return &errorResult, nil
		}

		// Update state
		state.MarkStepCompleted(config.Name)
		state.UpdateArtifacts(result)

		// Save state
		if err := SaveWorkflowState(ctx, deps.SessionManager, state); err != nil {
			errorResult := createErrorResult(errors.Wrap(err, "failed to save workflow state"))
			return &errorResult, nil
		}

		// Create response with chain hint
		var chainHint *ChainHint
		if config.NextTool != "" {
			chainHint = createChainHint(config.NextTool, config.ChainReason)
		}

		toolResult := createToolResult(true, result, chainHint)
		return &toolResult, nil
	}
}

// createOrchestrationHandler creates a handler for orchestration tools
func createOrchestrationHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch config.Name {
	case "start_workflow":
		return createStartWorkflowHandler(config, deps)
	case "workflow_status":
		return createWorkflowStatusHandler(config, deps)
	default:
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result := createErrorResult(errors.Errorf("orchestration handler not implemented for %s", config.Name))
			return &result, nil
		}
	}
}

// createUtilityHandler creates a handler for utility tools
func createUtilityHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch config.Name {
	case "list_tools":
		return CreateListToolsHandler()
	default:
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result := createErrorResult(errors.Errorf("utility handler not implemented for %s", config.Name))
			return &result, nil
		}
	}
}


// Handler implementations for specific tools

func createStartWorkflowHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			result := createErrorResult(errors.New("missing arguments"))
			return &result, nil
		}

		repoPath, ok := args["repo_path"].(string)
		if !ok || repoPath == "" {
			result := createErrorResult(errors.New("invalid or missing repo_path"))
			return &result, nil
		}

		// Generate session ID
		sessionID := GenerateSessionID()

		// Create initial workflow state
		state := &SimpleWorkflowState{
			SessionID:      sessionID,
			RepoPath:       repoPath,
			Status:         "started",
			CurrentStep:    "analyze_repository",
			CompletedSteps: []string{},
			Artifacts:      make(map[string]interface{}),
			Metadata:       make(map[string]interface{}),
		}

		// Handle optional parameters
		if skipSteps, ok := args["skip_steps"].([]interface{}); ok {
			steps := make([]string, len(skipSteps))
			for i, step := range skipSteps {
				steps[i] = fmt.Sprintf("%v", step)
			}
			state.SkipSteps = steps
		}

		// Save initial state
		if deps.SessionManager != nil {
			if err := SaveWorkflowState(ctx, deps.SessionManager, state); err != nil {
				deps.Logger.Error("Failed to save initial workflow state", slog.String("error", err.Error()))
			}
		}

		// Create response
		data := map[string]interface{}{
			"session_id": sessionID,
			"message":    "Workflow started successfully",
			"next_step":  "analyze_repository",
		}

		chainHint := createChainHint("analyze_repository", "Workflow initialized. Starting with repository analysis")
		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

func createWorkflowStatusHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			result := createErrorResult(errors.New("missing arguments"))
			return &result, nil
		}

		sessionID, ok := args["session_id"].(string)
		if !ok || sessionID == "" {
			result := createErrorResult(errors.New("invalid or missing session_id"))
			return &result, nil
		}

		// Load workflow state
		state, err := LoadWorkflowState(ctx, deps.SessionManager, sessionID)
		if err != nil {
			result := createErrorResult(errors.Wrap(err, "failed to load workflow state"))
			return &result, nil
		}

		// Prepare status data
		data := map[string]interface{}{
			"session_id":      state.SessionID,
			"status":          state.Status,
			"current_step":    state.CurrentStep,
			"completed_steps": state.CompletedSteps,
			"artifacts":       state.Artifacts,
		}

		if state.Error != nil {
			data["error"] = state.Error.Error()
		}

		// Determine next tool hint based on current state
		var chainHint *ChainHint
		if state.Status == "in_progress" && state.CurrentStep != "" {
			if _, err := GetToolConfig(state.CurrentStep); err == nil {
				chainHint = createChainHint(state.CurrentStep,
					fmt.Sprintf("Workflow in progress. Continue with %s", state.CurrentStep))
			}
		}

		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

func CreateListToolsHandler() func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tools := make([]map[string]interface{}, 0, len(toolConfigs))

		for _, config := range toolConfigs {
			tool := map[string]interface{}{
				"name":        config.Name,
				"description": config.Description,
				"category":    config.Category,
			}

			if config.NextTool != "" {
				tool["next_tool"] = config.NextTool
			}

			tools = append(tools, tool)
		}

		data := map[string]interface{}{
			"tools": tools,
			"total": len(tools),
		}

		result := createToolResult(true, data, nil)
		return &result, nil
	}
}

func createPingHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.GetArguments()
		message, _ := arguments["message"].(string)

		response := "pong"
		if message != "" {
			response = "pong: " + message
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"response":"%s","timestamp":"%s"}`, response, time.Now().Format(time.RFC3339)),
				},
			},
		}, nil
	}
}

// Track server start time at package level
var serverStartTime = time.Now()

func createServerStatusHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.GetArguments()
		details, _ := arguments["details"].(bool)

		status := struct {
			Status  string `json:"status"`
			Version string `json:"version"`
			Uptime  string `json:"uptime"`
			Details bool   `json:"details,omitempty"`
		}{
			Status:  "running",
			Version: "dev",
			Uptime:  time.Since(serverStartTime).String(),
			Details: details,
		}

		statusJSON, _ := json.Marshal(status)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(statusJSON),
				},
			},
		}, nil
	}
}
