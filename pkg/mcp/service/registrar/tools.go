// Package registrar handles tool and prompt registration
package registrar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	domainworkflow "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
	"github.com/Azure/container-kit/pkg/mcp/service/tools"
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
		logger:         logger.With("component", "tool_registrar"),
		orchestrator:   orchestrator,
		stepProvider:   stepProvider,
		sessionManager: sessionManager,
		config:         config,
	}
}

// RegisterAll registers all tools with the MCP server using direct registration
func (tr *ToolRegistrar) RegisterAll(mcpServer *server.MCPServer) error {
	tr.logger.Info("Registering tools using direct registration (old working method)")

	// Register tools in priority order - individual workflow steps first
	tr.logger.Info("Registering individual workflow step tools first (preferred)")
	if err := tr.registerWorkflowTools(mcpServer); err != nil {
		return err
	}

	tr.logger.Info("Registering utility tools for workflow management")
	if err := tr.registerUtilityTools(mcpServer); err != nil {
		return err
	}

	tr.logger.Info("Registering orchestration tools (deprecated start_workflow)")
	if err := tr.registerOrchestrationTools(mcpServer); err != nil {
		return err
	}

	tr.logger.Info("All tools registered successfully using direct registration")
	return nil
}

// registerOrchestrationTools registers orchestration tools using direct registration
func (tr *ToolRegistrar) registerOrchestrationTools(mcpServer *server.MCPServer) error {
	tr.logger.Info("Registering orchestration tools", "workflow_mode", tr.config.WorkflowMode)

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
			tr.logger.Info("Session not found, returning empty workflow status", "session_id", sessionID)
			statusData := map[string]interface{}{
				"success":             true,
				"session_id":          sessionID,
				"message":             "Session not found - no workflow started yet",
				"current_status":      "not_started",
				"completed_steps":     []string{},
				"total_steps":         10,
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
		allSteps := []string{
			"analyze_repository", "generate_dockerfile", "build_image", "scan_image",
			"tag_image", "push_image", "generate_k8s_manifests", "prepare_cluster",
			"deploy_application", "verify_deployment",
		}

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

	tr.logger.Info("Orchestration tools registered successfully")
	return nil
}

// registerWorkflowTools registers workflow step tools using direct registration
func (tr *ToolRegistrar) registerWorkflowTools(mcpServer *server.MCPServer) error {
	tr.logger.Info("Registering workflow tools")

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
			name:        "generate_dockerfile",
			description: "📝 STEP 2: Generate an optimized Dockerfile based on repository analysis. Requires analyze_repository to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"dockerfile_content": map[string]interface{}{
					"type":        "string",
					"description": "AI-generated Dockerfile content to save",
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
			description: "🏗️ STEP 3: Build Docker image from generated Dockerfile. Requires generate_dockerfile to be completed first.",
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
			description: "🔒 STEP 4: Scan the Docker image for security vulnerabilities. Requires build_image to be completed first.",
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
			description: "🏷️ STEP 5: Tag the Docker image with version and metadata. Requires build_image to be completed first.",
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
			description: "📤 STEP 6: Push the Docker image to a container registry. Requires tag_image to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Container registry URL",
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
			name:        "generate_k8s_manifests",
			description: "☸️ STEP 7: Generate Kubernetes manifests for the application. Requires push_image to be completed first.",
			params: map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for workflow state management",
				},
				"manifests": map[string]interface{}{
					"type":        "object",
					"description": "AI-generated Kubernetes manifests as key-value pairs (filename: content)",
					"properties": map[string]interface{}{
						"deployment.yaml": map[string]interface{}{
							"type":        "string",
							"description": "Kubernetes Deployment manifest content",
						},
						"service.yaml": map[string]interface{}{
							"type":        "string",
							"description": "Kubernetes Service manifest content",
						},
						"ingress.yaml": map[string]interface{}{
							"type":        "string",
							"description": "Kubernetes Ingress manifest content (optional)",
						},
					},
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
			description: "⚙️ STEP 8: Prepare the Kubernetes cluster for deployment. Requires generate_k8s_manifests to be completed first.",
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
			description: "🚀 STEP 9: Deploy the application to Kubernetes. Requires prepare_cluster to be completed first.",
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
			description: "✅ STEP 10: Verify the deployment is healthy and running correctly. Final step in the containerization workflow.",
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

		// Wrap with metrics collection
		metricsHandler := MetricsMiddleware(toolName, handler)

		// Register the handler
		mcpServer.AddTool(tool, metricsHandler)
	}

	tr.logger.Info("Workflow tools registered successfully", slog.Int("count", len(workflowTools)))
	return nil
}

// registerUtilityTools registers utility tools using direct registration
func (tr *ToolRegistrar) registerUtilityTools(mcpServer *server.MCPServer) error {
	tr.logger.Info("Registering utility tools")

	// list_tools
	listToolsTool := mcp.Tool{
		Name:        "list_tools",
		Description: "📋 List all available containerization tools. Use this to see the 10-step workflow sequence.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}

	mcpServer.AddTool(listToolsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"success":true,"message":"list_tools called","timestamp":"%s"}`, time.Now().Format(time.RFC3339)),
				},
			},
		}, nil
	})

	tr.logger.Info("Utility tools registered successfully")
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
		tr.logger.Info("Executing tool in fixing mode",
			"tool", stepName,
			"failed_tool", failedTool,
			"previous_error", previousError,
			"sessionId", args["session_id"],
		)
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
		tr.logger.Info("Mapping tool name to step name", "tool_name", stepName, "step_name", actualStepName)
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

	// Execute the step
	err = step.Execute(ctx, workflowState)
	if err != nil {
		// Log step failure
		tr.logger.Info("Step failed",
			"step", stepName,
			"error", err.Error(),
		)

		// Mark step as failed and save state
		simpleState.MarkStepFailed(stepName)
		simpleState.CurrentStep = stepName
		simpleState.Status = "error"
		if saveErr := tools.SaveWorkflowState(ctx, tr.sessionManager, simpleState); saveErr != nil {
			tr.logger.Warn("Failed to save workflow state after step failure", "session_id", sessionID, "step", stepName, "error", saveErr)
		}

		return tr.createRedirectResponse(stepName, fmt.Sprintf("Step %s failed with the following error: %v", stepName, err), sessionID)
	}

	// Save step results to session artifacts
	tr.saveStepResults(workflowState, simpleState, stepName)

	// Mark step as completed and save state
	simpleState.MarkStepCompleted(stepName)
	simpleState.CurrentStep = stepName
	simpleState.Status = "running"
	if err := tools.SaveWorkflowState(ctx, tr.sessionManager, simpleState); err != nil {
		tr.logger.Warn("Failed to save workflow state after step execution", "session_id", sessionID, "step", stepName, "error", err)
	}

	// TODO: Handle any additional results or artifacts from the step execution
	responseData := map[string]interface{}{
		"session_id": sessionID,
	}

	if stepName == "analyze_repository" {
		// If this is the analyze step, include the analyze result in the response
		if workflowState.AnalyzeResult != nil {
			responseData["analyze_result"] = workflowState.AnalyzeResult
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
		tr.logger.Info("Creating new session and workflow state", "session_id", sessionID)

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
			Artifacts:      make(map[string]interface{}),
			Metadata:       make(map[string]interface{}),
		}

		// Save the initial workflow state
		if err := tools.SaveWorkflowState(ctx, tr.sessionManager, simpleState); err != nil {
			tr.logger.Warn("Failed to save initial workflow state", "session_id", sessionID, "error", err)
			// Don't fail here, just log the warning and continue
		}
	}

	// Update repo path if provided and different
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
		TotalSteps:       10, // Standard workflow has 10 steps
		CurrentStep:      len(simpleState.CompletedSteps),
		WorkflowProgress: domainworkflow.NewWorkflowProgress(sessionID, "containerization", 10),
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
	if analyzeData, exists := simpleState.Artifacts["analyze_result"]; exists {
		if analyzeMap, ok := analyzeData.(map[string]interface{}); ok {
			analyzeResult := &domainworkflow.AnalyzeResult{}

			if language, ok := analyzeMap["language"].(string); ok {
				analyzeResult.Language = language
			}
			if framework, ok := analyzeMap["framework"].(string); ok {
				analyzeResult.Framework = framework
			}
			if port, ok := analyzeMap["port"].(float64); ok {
				analyzeResult.Port = int(port)
			}
			if buildCmd, ok := analyzeMap["build_command"].(string); ok {
				analyzeResult.BuildCommand = buildCmd
			}
			if startCmd, ok := analyzeMap["start_command"].(string); ok {
				analyzeResult.StartCommand = startCmd
			}
			if repoPath, ok := analyzeMap["repo_path"].(string); ok {
				analyzeResult.RepoPath = repoPath
			}
			if metadata, ok := analyzeMap["metadata"].(map[string]interface{}); ok {
				analyzeResult.Metadata = metadata
			}

			workflowState.AnalyzeResult = analyzeResult
			tr.logger.Info("Restored analyze result from session", "language", analyzeResult.Language, "framework", analyzeResult.Framework)
		}
	}

	// Restore DockerfileResult if available
	if dockerfileData, exists := simpleState.Artifacts["dockerfile_result"]; exists {
		if dockerfileMap, ok := dockerfileData.(map[string]interface{}); ok {
			dockerfileResult := &domainworkflow.DockerfileResult{}

			if content, ok := dockerfileMap["content"].(string); ok {
				dockerfileResult.Content = content
			}
			if path, ok := dockerfileMap["path"].(string); ok {
				dockerfileResult.Path = path
			}
			if metadata, ok := dockerfileMap["metadata"].(map[string]interface{}); ok {
				dockerfileResult.Metadata = metadata
			}

			workflowState.DockerfileResult = dockerfileResult
			tr.logger.Info("Restored dockerfile result from session", "path", dockerfileResult.Path)
		}
	}

	// Restore BuildResult if available
	if buildData, exists := simpleState.Artifacts["build_result"]; exists {
		if buildMap, ok := buildData.(map[string]interface{}); ok {
			buildResult := &domainworkflow.BuildResult{}

			if imageRef, ok := buildMap["image_ref"].(string); ok {
				buildResult.ImageRef = imageRef
			}
			if imageID, ok := buildMap["image_id"].(string); ok {
				buildResult.ImageID = imageID
			}
			if imageSize, ok := buildMap["image_size"].(float64); ok {
				buildResult.ImageSize = int64(imageSize)
			}
			if buildTime, ok := buildMap["build_time"].(string); ok {
				buildResult.BuildTime = buildTime
			}
			if metadata, ok := buildMap["metadata"].(map[string]interface{}); ok {
				buildResult.Metadata = metadata
			}

			workflowState.BuildResult = buildResult
			tr.logger.Info("Restored build result from session", "image_ref", buildResult.ImageRef)
		}
	}

	// Restore K8sResult if available
	if k8sData, exists := simpleState.Artifacts["k8s_result"]; exists {
		if k8sMap, ok := k8sData.(map[string]interface{}); ok {
			k8sResult := &domainworkflow.K8sResult{}

			if manifests, ok := k8sMap["manifests"].([]interface{}); ok {
				manifestStrs := make([]string, len(manifests))
				for i, m := range manifests {
					if manifestStr, ok := m.(string); ok {
						manifestStrs[i] = manifestStr
					}
				}
				k8sResult.Manifests = manifestStrs
			}
			if namespace, ok := k8sMap["namespace"].(string); ok {
				k8sResult.Namespace = namespace
			}
			if endpoint, ok := k8sMap["endpoint"].(string); ok {
				k8sResult.Endpoint = endpoint
			}
			if metadata, ok := k8sMap["metadata"].(map[string]interface{}); ok {
				k8sResult.Metadata = metadata
			}

			workflowState.K8sResult = k8sResult
			tr.logger.Info("Restored k8s result from session", "namespace", k8sResult.Namespace)
		}
	}
}

// mapToolNameToStepName maps tool names to actual step registry names
func (tr *ToolRegistrar) mapToolNameToStepName(toolName string) string {
	stepNameMap := map[string]string{
		"analyze_repository":     "analyze_repository",
		"generate_dockerfile":    "generate_dockerfile",
		"build_image":            "build_image",
		"scan_image":             "security_scan", // Tool name → actual step name
		"tag_image":              "tag_image",
		"push_image":             "push_image",
		"generate_k8s_manifests": "generate_manifests", // Tool name → actual step name
		"prepare_cluster":        "setup_cluster",      // Tool name → actual step name
		"deploy_application":     "deploy_application",
		"verify_deployment":      "verify_deployment",
	}

	if actualName, exists := stepNameMap[toolName]; exists {
		return actualName
	}

	// Fallback to original name if not found in map
	return toolName
}

// saveStepResults saves workflow step results to session artifacts
func (tr *ToolRegistrar) saveStepResults(workflowState *domainworkflow.WorkflowState, simpleState *tools.SimpleWorkflowState, stepName string) {
	switch stepName {
	case "analyze_repository":
		if workflowState.AnalyzeResult != nil {
			analyzeData := map[string]interface{}{
				"language":         workflowState.AnalyzeResult.Language,
				"framework":        workflowState.AnalyzeResult.Framework,
				"port":             workflowState.AnalyzeResult.Port,
				"build_command":    workflowState.AnalyzeResult.BuildCommand,
				"start_command":    workflowState.AnalyzeResult.StartCommand,
				"dependencies":     workflowState.AnalyzeResult.Dependencies,
				"dev_dependencies": workflowState.AnalyzeResult.DevDependencies,
				"repo_path":        workflowState.AnalyzeResult.RepoPath,
				"metadata":         workflowState.AnalyzeResult.Metadata,
			}
			simpleState.UpdateArtifacts(map[string]interface{}{
				"analyze_result": analyzeData,
			})
			tr.logger.Info("Saved analyze result to session artifacts", "language", workflowState.AnalyzeResult.Language)
		}

	case "generate_dockerfile":
		if workflowState.DockerfileResult != nil {
			dockerfileData := map[string]interface{}{
				"content":  workflowState.DockerfileResult.Content,
				"path":     workflowState.DockerfileResult.Path,
				"metadata": workflowState.DockerfileResult.Metadata,
			}
			simpleState.UpdateArtifacts(map[string]interface{}{
				"dockerfile_result": dockerfileData,
			})
			tr.logger.Info("Saved dockerfile result to session artifacts", "path", workflowState.DockerfileResult.Path)
		}

	case "build_image", "tag_image":
		if workflowState.BuildResult != nil {
			buildData := map[string]interface{}{
				"image_id":   workflowState.BuildResult.ImageID,
				"image_ref":  workflowState.BuildResult.ImageRef,
				"image_size": workflowState.BuildResult.ImageSize,
				"build_time": workflowState.BuildResult.BuildTime,
				"metadata":   workflowState.BuildResult.Metadata,
			}
			simpleState.UpdateArtifacts(map[string]interface{}{
				"build_result": buildData,
			})
			tr.logger.Info("Saved build result to session artifacts", "image_ref", workflowState.BuildResult.ImageRef)
		}

	case "generate_k8s_manifests", "prepare_cluster", "deploy_application", "verify_deployment":
		if workflowState.K8sResult != nil {
			k8sData := map[string]interface{}{
				"manifests": workflowState.K8sResult.Manifests,
				"namespace": workflowState.K8sResult.Namespace,
				"endpoint":  workflowState.K8sResult.Endpoint,
				"metadata":  workflowState.K8sResult.Metadata,
			}
			simpleState.UpdateArtifacts(map[string]interface{}{
				"k8s_result": k8sData,
			})
			tr.logger.Info("Saved k8s result to session artifacts", "namespace", workflowState.K8sResult.Namespace)
		}
	}
}
