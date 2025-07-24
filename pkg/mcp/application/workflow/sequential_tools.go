package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	domainworkflow "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Simple workflow state storage (in-memory for now)
var (
	workflowStates = make(map[string]*domainworkflow.WorkflowState)
	stateMutex     sync.RWMutex
)

// RegisterSequentialTools registers all 10 individual workflow tools
func RegisterSequentialTools(mcpServer interface {
	AddTool(tool mcp.Tool, handler server.ToolHandlerFunc)
}, logger *slog.Logger) error {

	tools := []struct {
		name        string
		description string
		stepName    string
		isFirst     bool
	}{
		{"analyze_repository", "Analyze repository structure and detect language/framework", "analyze_repository", true},
		{"generate_dockerfile", "Generate Dockerfile based on analysis", "generate_dockerfile", false},
		{"build_image", "Build Docker image from Dockerfile", "build_image", false},
		{"security_scan", "Scan image for security vulnerabilities", "security_scan", false},
		{"tag_image", "Tag image for registry push", "tag_image", false},
		{"push_image", "Push image to container registry", "push_image", false},
		{"generate_manifests", "Generate Kubernetes deployment manifests", "generate_k8s_manifests", false},
		{"setup_cluster", "Setup and validate Kubernetes cluster", "setup_cluster", false},
		{"deploy_application", "Deploy application to Kubernetes", "deploy_application", false},
		{"verify_deployment", "Verify deployment health and get endpoint", "verify_deployment", false},
	}

	for i, toolDef := range tools {
		tool := createSequentialTool(toolDef.name, toolDef.description, toolDef.isFirst)
		handler := createSequentialHandler(toolDef.stepName, i+1, toolDef.isFirst, logger)
		mcpServer.AddTool(tool, handler)
	}

	logger.Info("Registered 10 sequential workflow tools")
	return nil
}

func createSequentialTool(name, description string, isFirst bool) mcp.Tool {
	properties := map[string]interface{}{
		"workflow_id": map[string]interface{}{
			"type":        "string",
			"description": "Workflow ID to track state across steps",
		},
	}

	required := []string{"workflow_id"}

	// First tool needs repo_url instead of workflow_id
	if isFirst {
		properties = map[string]interface{}{
			"repo_url": map[string]interface{}{
				"type":        "string",
				"description": "Repository URL to containerize",
			},
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Git branch (optional, defaults to main)",
			},
			"scan": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable security scanning (optional, defaults to true)",
			},
			"deploy": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable Kubernetes deployment (optional, defaults to true)",
			},
		}
		required = []string{"repo_url"}
	}

	return mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: properties,
			Required:   required,
		},
	}
}

func createSequentialHandler(stepName string, stepNumber int, isFirst bool, logger *slog.Logger) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		var workflowState *domainworkflow.WorkflowState
		var workflowID string

		if isFirst {
			// Create new workflow state for first step
			repoURL := args["repo_url"].(string)
			branch, _ := args["branch"].(string)
			scan, _ := args["scan"].(bool)
			deploy, _ := args["deploy"].(bool)

			workflowArgs := &domainworkflow.ContainerizeAndDeployArgs{
				RepoURL:  repoURL,
				Branch:   branch,
				Scan:     scan,
				Deploy:   &deploy,
				TestMode: false,
			}

			workflowState = domainworkflow.NewWorkflowState(ctx, &req, workflowArgs, nil, logger)
			workflowID = workflowState.WorkflowID

			// Store state
			stateMutex.Lock()
			workflowStates[workflowID] = workflowState
			stateMutex.Unlock()

		} else {
			// Load existing workflow state
			workflowID = args["workflow_id"].(string)

			stateMutex.RLock()
			var exists bool
			workflowState, exists = workflowStates[workflowID]
			stateMutex.RUnlock()

			if !exists {
				return nil, fmt.Errorf("workflow %s not found - make sure to call analyze_repository first", workflowID)
			}
		}

		// Execute the specific step
		err := executeWorkflowStep(ctx, stepName, workflowState)
		if err != nil {
			return nil, fmt.Errorf("step %s failed: %v", stepName, err)
		}

		// Update stored state
		stateMutex.Lock()
		workflowStates[workflowID] = workflowState
		stateMutex.Unlock()

		// Create response with next step guidance
		responseText := formatStepResponse(stepName, stepNumber, workflowState)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: responseText,
				},
			},
		}, nil
	}
}

func executeWorkflowStep(ctx context.Context, stepName string, state *domainworkflow.WorkflowState) error {
	// Get the appropriate step implementation and execute it
	switch stepName {
	case "analyze_repository":
		step := steps.NewAnalyzeStep()
		return step.Execute(ctx, state)
	case "generate_dockerfile":
		step := steps.NewDockerfileStep()
		return step.Execute(ctx, state)
	case "build_image":
		step := steps.NewBuildStep()
		return step.Execute(ctx, state)
	case "security_scan":
		step := steps.NewScanStep()
		return step.Execute(ctx, state)
	case "tag_image":
		step := steps.NewTagStep()
		return step.Execute(ctx, state)
	case "push_image":
		step := steps.NewPushStep()
		return step.Execute(ctx, state)
	case "generate_k8s_manifests":
		step := steps.NewManifestStep()
		return step.Execute(ctx, state)
	case "setup_cluster":
		step := steps.NewClusterStep()
		return step.Execute(ctx, state)
	case "deploy_application":
		step := steps.NewDeployStep()
		return step.Execute(ctx, state)
	case "verify_deployment":
		step := steps.NewVerifyStep()
		return step.Execute(ctx, state)
	default:
		return fmt.Errorf("unknown step: %s", stepName)
	}
}

func formatStepResponse(stepName string, stepNumber int, state *domainworkflow.WorkflowState) string {
	nextSteps := []string{
		"analyze_repository", "generate_dockerfile", "build_image", "security_scan",
		"tag_image", "push_image", "generate_manifests", "setup_cluster",
		"deploy_application", "verify_deployment",
	}

	baseResponse := fmt.Sprintf("✅ Step %d completed: %s\n\nWorkflow ID: %s\n",
		stepNumber, stepName, state.WorkflowID)

	// Add step-specific results
	switch stepName {
	case "analyze_repository":
		if state.AnalyzeResult != nil {
			baseResponse += fmt.Sprintf("\n**Analysis Results:**\n- Language: %s\n- Framework: %s\n- Port: %d\n",
				state.AnalyzeResult.Language, state.AnalyzeResult.Framework, state.AnalyzeResult.Port)
		}
	case "build_image":
		if state.BuildResult != nil {
			baseResponse += fmt.Sprintf("\n**Build Results:**\n- Image: %s\n- Size: %s\n",
				state.BuildResult.ImageRef, state.BuildResult.ImageSize)
		}
	case "verify_deployment":
		if state.K8sResult != nil {
			baseResponse += fmt.Sprintf("\n🎉 **Deployment Complete!**\n- Namespace: %s\n- Endpoint: %s\n",
				state.K8sResult.Namespace, state.K8sResult.Endpoint)
		}
	}

	// Add next step guidance
	if stepNumber < len(nextSteps) {
		nextTool := nextSteps[stepNumber]
		baseResponse += fmt.Sprintf("\n**Next Step:** Call '%s' with workflow_id='%s'\n",
			nextTool, state.WorkflowID)
	} else {
		baseResponse += "\n🎉 **Workflow Complete!** All steps finished successfully.\n"
	}

	return baseResponse
}

// CleanupOldWorkflows removes old workflow states to prevent memory leaks
func CleanupOldWorkflows() {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour) // Remove workflows older than 1 hour
	for id, state := range workflowStates {
		if state.WorkflowProgress.StartTime.Before(cutoff) {
			delete(workflowStates, id)
		}
	}
}
