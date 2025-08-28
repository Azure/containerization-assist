// Package registrar handles tool and prompt registration
package registrar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	domainworkflow "github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/service/session"
	"github.com/Azure/containerization-assist/pkg/service/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolRegistrar handles tool registration using the new consolidated system
type ToolRegistrar struct {
	logger         *slog.Logger
	orchestrator   domainworkflow.WorkflowOrchestrator
	stepProvider   domainworkflow.StepProvider
	sessionManager session.OptimizedSessionManager
	config         domainworkflow.ServerConfig
}

// NewToolRegistrar creates a new tool registrar
func NewToolRegistrar(
	logger *slog.Logger,
	orchestrator domainworkflow.WorkflowOrchestrator,
	stepProvider domainworkflow.StepProvider,
	sessionManager session.OptimizedSessionManager,
	config domainworkflow.ServerConfig,
) *ToolRegistrar {
	return &ToolRegistrar{
		logger:         logger,
		orchestrator:   orchestrator,
		stepProvider:   stepProvider,
		sessionManager: sessionManager,
		config:         config,
	}
}

// RegisterAll registers all tools with the MCP server using direct registration
func (tr *ToolRegistrar) RegisterAll(mcpServer *server.MCPServer) error {

	// Register tools in priority order - individual workflow steps first
	if err := tr.registerWorkflowTools(mcpServer); err != nil {
		return err
	}

	if err := tr.registerUtilityTools(mcpServer); err != nil {
		return err
	}

	if err := tr.registerOrchestrationTools(mcpServer); err != nil {
		return err
	}

	return nil
}

// registerOrchestrationTools registers orchestration tools using direct registration
func (tr *ToolRegistrar) registerOrchestrationTools(mcpServer *server.MCPServer) error {

	// Set description based on workflow mode
	var startWorkflowDescription string
	if tr.config.WorkflowMode == "automated" {
		// In automated mode, promote start_workflow as primary tool
		startWorkflowDescription = "🚀 Runs the complete containerization workflow automatically. Analyzes repository, generates Dockerfile, builds/scans/pushes image, generates K8s manifests, and deploys to cluster."
	} else {
		// In interactive mode (default), deprecate in favor of individual steps
		startWorkflowDescription = "⚠️ DEPRECATED: Runs entire workflow at once. PREFER individual steps (analyze_repository, generate_dockerfile, etc.) for better control and visibility."
	}

	// Register start_workflow tool with appropriate description
	startWorkflowTool := mcp.Tool{
		Name:        "start_workflow",
		Description: startWorkflowDescription,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"repo_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the repository to analyze",
				},
			},
			Required: []string{"repo_path"},
		},
	}

	mcpServer.AddTool(startWorkflowTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Get arguments
		args := req.GetArguments()
		repoPath, ok := args["repo_path"].(string)
		if !ok || repoPath == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: `{"success":false,"error":"repo_path parameter is required"}`,
					},
				},
			}, nil
		}

		// Use the workflow orchestrator to start the full workflow
		workflowArgs := &domainworkflow.ContainerizeAndDeployArgs{
			RepoPath: repoPath,
		}

		result, err := tr.orchestrator.Execute(ctx, &req, workflowArgs)

		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf(`{"success":false,"error":"Workflow failed: %s"}`, err.Error()),
					},
				},
			}, nil
		}

		// Return successful result with workflow info
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"success":%t,"message":"Complete containerization workflow executed successfully","repo_path":"%s","endpoint":"%s","image_ref":"%s","namespace":"%s","timestamp":"%s"}`, result.Success, repoPath, result.Endpoint, result.ImageRef, result.Namespace, time.Now().Format(time.RFC3339)),
				},
			},
		}, nil
	})

	// Register workflow_status tool
	workflowStatusTool := mcp.Tool{
		Name:        "workflow_status",
		Description: "📊 Check workflow progress and see which steps are completed. Only use when you need to understand the current workflow state.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
			},
			Required: []string{"session_id"},
		},
	}

	mcpServer.AddTool(workflowStatusTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		sessionID, ok := args["session_id"].(string)
		if !ok || sessionID == "" {
			return tr.createErrorResult("session_id parameter is required for workflow_status")
		}

		// Load workflow state to show current progress
		simpleState, err := tools.LoadWorkflowState(ctx, tr.sessionManager, sessionID)
		if err != nil {
			// If session doesn't exist yet, return empty state
			statusData := map[string]interface{}{
				"success":             true,
				"session_id":          sessionID,
				"message":             "Session not found - no workflow started yet",
				"current_status":      "not_started",
				"completed_steps":     []string{},
				"total_steps":         11,
				"progress_percentage": 0,
				"next_step":           "analyze_repository",
				"timestamp":           time.Now().Format(time.RFC3339),
			}
			jsonData, _ := json.Marshal(statusData)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: string(jsonData),
					},
				},
			}, nil
		}

		// Create progress response
		allSteps := []string{}

		// Build step status details in order
		stepStatuses := make([]map[string]interface{}, 0, len(allSteps))
		for _, step := range allSteps {
			stepStatuses = append(stepStatuses, map[string]interface{}{
				"step":   step,
				"status": simpleState.GetStepStatus(step),
			})
		}

		// Calculate next step based on current step's workflow configuration
		nextStep := func() string {
			// If no current step, start with first step
			if simpleState.CurrentStep == "" {
				return "analyze_repository"
			}

			// If current step failed, check redirect configuration
			if simpleState.IsStepFailed(simpleState.CurrentStep) {
				if redirectConfig, exists := RedirectConfigs[simpleState.CurrentStep]; exists {
					return redirectConfig.RedirectTo
				}
			}

			// Get next tool from workflow configuration
			if config, err := tools.GetToolConfig(simpleState.CurrentStep); err == nil && config.NextTool != "" {
				return config.NextTool
			}

			return "workflow_complete"
		}()

		// Determine overall workflow status
		overallStatus := simpleState.Status
		if len(simpleState.FailedSteps) > 0 {
			overallStatus = "error"
		} else if len(simpleState.CompletedSteps) == len(allSteps) {
			overallStatus = "completed"
		} else if len(simpleState.CompletedSteps) > 0 {
			overallStatus = "running"
		}

		statusData := map[string]interface{}{
			"success":             true,
			"session_id":          sessionID,
			"repo_path":           simpleState.RepoPath,
			"current_status":      overallStatus,
			"current_step":        simpleState.CurrentStep,
			"completed_steps":     simpleState.CompletedSteps,
			"failed_steps":        simpleState.FailedSteps,
			"step_statuses":       stepStatuses,
			"total_steps":         len(allSteps),
			"progress_percentage": int(float64(len(simpleState.CompletedSteps)) / float64(len(allSteps)) * 100),
			"next_step":           nextStep,
			"timestamp":           time.Now().Format(time.RFC3339),
		}

		jsonData, _ := json.Marshal(statusData)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(jsonData),
				},
			},
		}, nil
	})

	return nil
}

// registerWorkflowTools registers workflow step tools using direct registration
func (tr *ToolRegistrar) registerWorkflowTools(mcpServer *server.MCPServer) error {

	// All 10 workflow step tools
	workflowTools := []struct {
		name        string
		description string
		params      map[string]interface{}
		required    []string
	}{
		{
			name:        "analyze_repository",
			description: "🔍 STEP 1: Analyze repository to detect language, framework, and build requirements. Start here for new containerization workflows.",
			params: map[string]interface{}{
				"repo_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the repository to analyze",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"repo_path", "session_id"},
		},
		{
			name:        "resolve_base_images",
			description: "🐳 STEP 2: Resolve recommended base images based on repository analysis. Requires analyze_repository to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id"},
		},
		{
			name:        "verify_dockerfile",
			description: "📝 STEP 3: Verify an AI-generated Dockerfile based on repository analysis. Requires resolve_base_images to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id", "dockerfile_content"},
		},
		{
			name:        "build_image",
			description: "🏗️ STEP 4: Build Docker image from verified Dockerfile. Requires verify_dockerfile to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id"},
		},
		{
			name:        "scan_image",
			description: "🔒 STEP 5: Scan the Docker image for security vulnerabilities. Requires build_image to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id"},
		},
		{
			name:        "tag_image",
			description: "🏷️ STEP 6: Tag the Docker image with version and metadata. Requires build_image to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag for the Docker image",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id", "tag"},
		},
		{
			name:        "push_image",
			description: "📤 STEP 7: Push the Docker image to a container registry. Requires tag_image to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Container registry URL",
					"default":     "localhost:5001",
				},
				"redirect_attempt": map[string]interface{}{
					"type":        "integer",
					"description": "Current retry attempt number",
					"default":     1,
					"minimum":     1,
				},
				"maxRetries": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum retry attempts (0 = no retries)",
					"default":     5,
					"minimum":     0,
				},
			},
			required: []string{"session_id", "registry"},
		},
		{
			name:        "verify_k8s_manifests",
			description: "☸️ STEP 8: Verify Kubernetes manifests for the application. Requires push_image to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id", "manifests"},
		},
		{
			name:        "prepare_cluster",
			description: "⚙️ STEP 9: Prepare the Kubernetes cluster for deployment. Requires verify_k8s_manifests to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id"},
		},
		{
			name:        "deploy_application",
			description: "🚀 STEP 10: Deploy the application to Kubernetes. Requires prepare_cluster to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id"},
		},
		{
			name:        "verify_deployment",
			description: "✅ STEP 11: Verify the deployment is healthy and running correctly. Final step in the containerization workflow.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"fixing_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this tool is being called to fix a previous failure",
					"default":     false,
				},
				"previous_error": map[string]interface{}{
					"type":        "string",
					"description": "Error from the previous failed tool (when fixing_mode is true)",
				},
				"failed_tool": map[string]interface{}{
					"type":        "string",
					"description": "Name of the tool that failed (when fixing_mode is true)",
				},
			},
			required: []string{"session_id"},
		},
	}

	// Register each workflow tool using direct registration with retry wrapper
	for _, toolDef := range workflowTools {
		tool := mcp.Tool{
			Name:        toolDef.name,
			Description: toolDef.description,
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: toolDef.params,
				Required:   toolDef.required,
			},
		}

		// Capture toolDef.name in closure
		toolName := toolDef.name

		// Create handler with redirect logic
		handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return tr.executeWorkflowStep(ctx, req, toolName)
		}

		// Register the handler (metrics collection removed)
		mcpServer.AddTool(tool, handler)
	}

	return nil
}

// registerUtilityTools registers utility tools using direct registration
func (tr *ToolRegistrar) registerUtilityTools(mcpServer *server.MCPServer) error {

	// list_tools
	listToolsTool := mcp.Tool{
		Name:        "list_tools",
		Description: "📋 List all available containerization tools. Use this to see the 10-step workflow sequence.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}

	mcpServer.AddTool(listToolsTool, tools.CreateListToolsHandler())

	return nil
}

// executeWorkflowStep executes an individual workflow step
func (tr *ToolRegistrar) executeWorkflowStep(ctx context.Context, req mcp.CallToolRequest, stepName string) (*mcp.CallToolResult, error) {
	// Get arguments
	args := req.GetArguments()

	// Extract fixing information
	fixingMode := false
	if fm, ok := args["fixing_mode"].(bool); ok {
		fixingMode = fm
	}

	previousError := ""
	if pe, ok := args["previous_error"].(string); ok {
		previousError = pe
	}

	failedTool := ""
	if ft, ok := args["failed_tool"].(string); ok {
		failedTool = ft
	}

	// Log fixing information if this is a fixing call
	if fixingMode {
		// Fixing mode logging removed
	}

	// Define required parameters for each tool
	requiredParams := tr.getRequiredParameters(stepName)

	// Validate required parameters
	validatedParams := make(map[string]any)

	for paramName, paramType := range requiredParams {
		switch paramType {
		case "string":
			value, ok := args[paramName].(string)
			if !ok || value == "" {
				return tr.createErrorResult(fmt.Sprintf("%s parameter is required for %s", paramName, stepName))
			}
			validatedParams[paramName] = value
		case "object":
			value, ok := args[paramName]
			if !ok || value == nil {
				return tr.createErrorResult(fmt.Sprintf("%s parameter is required for %s", paramName, stepName))
			}
			validatedParams[paramName] = value
		}
	}

	if validatedParams["session_id"] == nil {
		return tr.createErrorResult("session_id parameter is required for all workflow steps")
	}

	sessionID, ok := validatedParams["session_id"].(string)
	if !ok || sessionID == "" {
		return tr.createErrorResult("session_id parameter must be a non-empty string")
	}

	if stepName == "analyze_repository" && validatedParams["repo_path"] == nil {
		return tr.createErrorResult("repo_path parameter is required for analyze_repository step")
	}

	repoPath := ""
	if validatedParams["repo_path"] != nil {
		repoPath = validatedParams["repo_path"].(string)
	}

	// Map tool names to actual step names
	actualStepName := tr.mapToolNameToStepName(stepName)
	if actualStepName != stepName {
	}

	// Get the step from step provider
	step, err := tr.stepProvider.GetStep(actualStepName)
	if err != nil {
		return tr.createRedirectResponse(stepName, fmt.Sprintf("Failed to get step %s (mapped to %s): %s", stepName, actualStepName, err.Error()), sessionID)
	}

	// Create workflow state for this step with fixing context
	workflowState, simpleState, err := tr.createStepState(ctx, sessionID, repoPath, fixingMode, previousError, failedTool, args)
	if err != nil {
		return tr.createRedirectResponse(stepName, fmt.Sprintf("Failed to create workflow state: %s", err.Error()), sessionID)
	}

	// Execute the step and get result
	stepResult, err := step.Execute(ctx, workflowState)
	if err != nil {
		// Log step failure

		// Mark step as failed and save state
		simpleState.MarkStepFailed(stepName)
		simpleState.CurrentStep = stepName
		simpleState.Status = "error"
		if saveErr := tools.SaveWorkflowState(ctx, tr.sessionManager, simpleState); saveErr != nil {
		}

		// Prepare step result data for context, even on failure
		var stepResultData map[string]interface{}
		if stepResult != nil && len(stepResult.Data) > 0 {
			stepResultData = stepResult.Data
			if len(stepResult.Metadata) > 0 {
				if stepResultData == nil {
					stepResultData = make(map[string]interface{})
				}
				stepResultData["metadata"] = stepResult.Metadata
			}
		}

		return tr.createRedirectResponse(stepName, fmt.Sprintf("Step %s failed with the following error: %v", stepName, err), sessionID, stepResultData)
	}

	// Save step results to session artifacts
	tr.saveStepResults(workflowState, simpleState, stepName)

	// Mark step as completed and save state
	simpleState.MarkStepCompleted(stepName)
	simpleState.CurrentStep = stepName
	simpleState.Status = "running"
	if err := tools.SaveWorkflowState(ctx, tr.sessionManager, simpleState); err != nil {
	}

	// Prepare response data with step result information
	responseData := map[string]interface{}{
		"session_id": sessionID,
	}

	// Include step result data if available (for both success and failure cases)
	if stepResult != nil {

		// Add the step result for rich formatting
		responseData["step_result"] = map[string]interface{}{
			"success": stepResult.Success,
			"data":    stepResult.Data,
		}

		// Include metadata if present
		if len(stepResult.Metadata) > 0 {
			responseData["step_metadata"] = stepResult.Metadata
		}
	}

	return tr.createProgressResponse(stepName, responseData, sessionID)
}

// getRequiredParameters returns the required parameters for each tool
func (tr *ToolRegistrar) getRequiredParameters(stepName string) map[string]string {
	if config, err := tools.GetToolConfig(stepName); err == nil {
		params := make(map[string]string)
		for _, param := range config.RequiredParams {
			params[param] = "string" // Default to string type
		}
		return params

	}

	return map[string]string{
		"session_id": "string",
	}
}

// Helper methods for creating responses
func (tr *ToolRegistrar) createErrorResult(message string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf(`{"success":false,"error":"%s","timestamp":"%s"}`, message, time.Now().Format(time.RFC3339)),
			},
		},
	}, nil
}

func (tr *ToolRegistrar) createStepState(ctx context.Context, sessionID, repoPath string, fixingMode bool, previousError, failedTool string, requestParams map[string]interface{}) (*domainworkflow.WorkflowState, *tools.SimpleWorkflowState, error) {
	// Load existing session state or create new one
	simpleState, err := tools.LoadWorkflowState(ctx, tr.sessionManager, sessionID)
	if err != nil {

		// First, ensure the session exists by using GetOrCreate
		_, err := tr.sessionManager.GetOrCreate(ctx, sessionID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create session: %w", err)
		}

		// Create new simple state
		simpleState = &tools.SimpleWorkflowState{
			SessionID:      sessionID,
			RepoPath:       repoPath,
			Status:         "initialized",
			CompletedSteps: []string{},
			FailedSteps:    []string{},
			Artifacts:      &tools.WorkflowArtifacts{},
			Metadata:       &tools.ToolMetadata{SessionID: sessionID},
		}

		// Save the initial workflow state
		if err := tools.SaveWorkflowState(ctx, tr.sessionManager, simpleState); err != nil {
			// Don't fail here, just log the warning and continue
		}
	}

	// Update repo path if provided and different (avoid unnecessary updates)
	if repoPath != "" && simpleState.RepoPath != repoPath {
		simpleState.RepoPath = repoPath
	}

	// Convert to domain workflow state for step execution
	args := &domainworkflow.ContainerizeAndDeployArgs{
		RepoPath: simpleState.RepoPath,
	}

	workflowState := &domainworkflow.WorkflowState{
		WorkflowID:       sessionID,
		Args:             args,
		RepoIdentifier:   domainworkflow.GetRepositoryIdentifier(args),
		Result:           &domainworkflow.ContainerizeAndDeployResult{},
		Logger:           tr.logger,
		TotalSteps:       11, // Standard workflow has 11 steps
		CurrentStep:      len(simpleState.CompletedSteps),
		WorkflowProgress: domainworkflow.NewWorkflowProgress(sessionID, "containerization", 11),
		RequestParams:    requestParams,
		PreviousError:    previousError,
		FailedTool:       failedTool,
	}

	// Restore step results from session artifacts
	tr.restoreStepResults(workflowState, simpleState)

	return workflowState, simpleState, nil
}

// restoreStepResults restores workflow step results from session artifacts
func (tr *ToolRegistrar) restoreStepResults(workflowState *domainworkflow.WorkflowState, simpleState *tools.SimpleWorkflowState) {
	// Restore AnalyzeResult if available
	if simpleState.Artifacts != nil && simpleState.Artifacts.AnalyzeResult != nil {
		analyzeArtifact := simpleState.Artifacts.AnalyzeResult
		analyzeResult := &domainworkflow.AnalyzeResult{
			Language:        analyzeArtifact.Language,
			Framework:       analyzeArtifact.Framework,
			Port:            analyzeArtifact.Port,
			BuildCommand:    analyzeArtifact.BuildCommand,
			StartCommand:    analyzeArtifact.StartCommand,
			RepoPath:        analyzeArtifact.RepoPath,
			Dependencies:    analyzeArtifact.Dependencies,
			DevDependencies: analyzeArtifact.DevDependencies,
			Metadata:        analyzeArtifact.Metadata,
		}
		workflowState.AnalyzeResult = analyzeResult
	}

	// Restore DockerfileResult if available
	if simpleState.Artifacts != nil && simpleState.Artifacts.DockerfileResult != nil {
		dockerfileArtifact := simpleState.Artifacts.DockerfileResult
		dockerfileResult := &domainworkflow.DockerfileResult{
			Content:  dockerfileArtifact.Content,
			Path:     dockerfileArtifact.Path,
			Metadata: dockerfileArtifact.Metadata,
		}
		workflowState.DockerfileResult = dockerfileResult
	}

	// Restore BuildResult if available
	if simpleState.Artifacts != nil && simpleState.Artifacts.BuildResult != nil {
		buildArtifact := simpleState.Artifacts.BuildResult
		buildResult := &domainworkflow.BuildResult{
			ImageRef:  buildArtifact.ImageRef,
			ImageID:   buildArtifact.ImageID,
			ImageSize: buildArtifact.ImageSize,
			BuildTime: buildArtifact.BuildTime,
			Metadata:  buildArtifact.Metadata,
		}
		workflowState.BuildResult = buildResult
	}

	// Restore K8sResult if available
	if simpleState.Artifacts != nil && simpleState.Artifacts.K8sResult != nil {
		k8sArtifact := simpleState.Artifacts.K8sResult
		k8sResult := &domainworkflow.K8sResult{
			Manifests: k8sArtifact.Manifests,
			Namespace: k8sArtifact.Namespace,
			Endpoint:  k8sArtifact.Endpoint,
			Metadata:  k8sArtifact.Metadata,
		}
		workflowState.K8sResult = k8sResult
	}
}

// mapToolNameToStepName maps tool names to actual step registry names
func (tr *ToolRegistrar) mapToolNameToStepName(toolName string) string {
	stepNameMap := map[string]string{
		"analyze_repository":   "analyze_repository",
		"resolve_base_images":  "resolve_base_images",
		"verify_dockerfile":    "verify_dockerfile",
		"build_image":          "build_image",
		"scan_image":           "security_scan", // Tool name → actual step name
		"tag_image":            "tag_image",
		"push_image":           "push_image",
		"verify_k8s_manifests": "verify_manifests", // Tool name → actual step name
		"prepare_cluster":      "setup_cluster",    // Tool name → actual step name
		"deploy_application":   "deploy_application",
		"verify_deployment":    "verify_deployment",
	}

	if actualName, exists := stepNameMap[toolName]; exists {
		return actualName
	}

	// Fallback to original name if not found in map
	return toolName
}

// saveStepResults saves workflow step results to session artifacts
func (tr *ToolRegistrar) saveStepResults(workflowState *domainworkflow.WorkflowState, simpleState *tools.SimpleWorkflowState, stepName string) {
	// Ensure artifacts structure exists
	if simpleState.Artifacts == nil {
		simpleState.Artifacts = &tools.WorkflowArtifacts{}
	}

	switch stepName {
	case "analyze_repository":
		if workflowState.AnalyzeResult != nil {
			simpleState.Artifacts.AnalyzeResult = &tools.AnalyzeArtifact{
				Language:        workflowState.AnalyzeResult.Language,
				Framework:       workflowState.AnalyzeResult.Framework,
				Port:            workflowState.AnalyzeResult.Port,
				BuildCommand:    workflowState.AnalyzeResult.BuildCommand,
				StartCommand:    workflowState.AnalyzeResult.StartCommand,
				Dependencies:    workflowState.AnalyzeResult.Dependencies,
				DevDependencies: workflowState.AnalyzeResult.DevDependencies,
				RepoPath:        workflowState.AnalyzeResult.RepoPath,
				Metadata:        workflowState.AnalyzeResult.Metadata,
			}
		}

	case "resolve_base_images":
		// resolve_base_images step result is stored in step result data, not in workflow state
		// The step returns data with "builder" and "runtime" keys
		// This is handled generically through the step result response
		tr.logger.Info("Resolve base images step completed - results included in step response")

	case "verify_dockerfile":
		if workflowState.DockerfileResult != nil {
			simpleState.Artifacts.DockerfileResult = &tools.DockerfileArtifact{
				Content:  workflowState.DockerfileResult.Content,
				Path:     workflowState.DockerfileResult.Path,
				Metadata: workflowState.DockerfileResult.Metadata,
			}
		}

	case "build_image", "tag_image":
		if workflowState.BuildResult != nil {
			simpleState.Artifacts.BuildResult = &tools.BuildArtifact{
				ImageID:   workflowState.BuildResult.ImageID,
				ImageRef:  workflowState.BuildResult.ImageRef,
				ImageSize: workflowState.BuildResult.ImageSize,
				BuildTime: workflowState.BuildResult.BuildTime,
				Metadata:  workflowState.BuildResult.Metadata,
			}
		}

	case "verify_k8s_manifests", "prepare_cluster", "deploy_application", "verify_deployment":
		if workflowState.K8sResult != nil {
			simpleState.Artifacts.K8sResult = &tools.K8sArtifact{
				Manifests: workflowState.K8sResult.Manifests,
				Namespace: workflowState.K8sResult.Namespace,
				Endpoint:  workflowState.K8sResult.Endpoint,
				Metadata:  workflowState.K8sResult.Metadata,
			}
		}
	}
}
