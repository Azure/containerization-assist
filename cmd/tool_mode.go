package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/service"
	"github.com/Azure/containerization-assist/pkg/service/config"
	"github.com/Azure/containerization-assist/pkg/service/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// handleToolMode handles execution when the binary is called in tool mode
// Usage: containerization-assist-mcp tool <tool-name>
func handleToolMode() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s tool <tool-name>\n", os.Args[0])
		os.Exit(1)
	}

	toolName := os.Args[2]

	// Get tool parameters from environment variable
	paramsJSON := os.Getenv("TOOL_PARAMS")
	if paramsJSON == "" {
		fmt.Fprintf(os.Stderr, "Error: TOOL_PARAMS environment variable not set\n")
		os.Exit(1)
	}

	// Parse parameters
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing TOOL_PARAMS: %v\n", err)
		os.Exit(1)
	}

	// Auto-generate session_id if not provided and tool needs it
	if needsSessionId(toolName) && params["session_id"] == nil {
		params["session_id"] = generateSessionId()
	}

	// Execute the tool and output result
	result, err := executeToolDirectly(toolName, params)
	if err != nil {
		// Output error as JSON
		errorResult := tools.ToolResult{
			Success: false,
			Error:   err.Error(),
		}
		output, _ := json.Marshal(errorResult)
		fmt.Println(string(output))
		os.Exit(1)
	}

	// Output successful result as JSON
	fmt.Println(result)
}

// executeToolDirectly executes a tool by initializing the full server infrastructure
func executeToolDirectly(toolName string, params map[string]interface{}) (string, error) {
	ctx := context.Background()

	// Set up logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		// Use default configuration if load fails
		cfg = config.DefaultConfig()
	}

	// Create server factory and build dependencies
	factory := service.NewServerFactory(logger, cfg.ToServerConfig())
	deps, err := factory.BuildDependenciesForTools(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build dependencies: %w", err)
	}

	// Execute tool using the tools package directly
	return executeToolWithDeps(ctx, toolName, params, deps, logger)
}

// executeToolWithDeps executes a tool using full dependencies
func executeToolWithDeps(ctx context.Context, toolName string, params map[string]interface{}, deps *service.Dependencies, logger *slog.Logger) (string, error) {
	// Get tool configuration
	toolConfig, err := tools.GetToolConfig(toolName)
	if err != nil {
		return "", fmt.Errorf("tool %s not found", toolName)
	}

	// Extract step provider from orchestrator
	var stepProvider workflow.StepProvider
	if concreteOrchestrator, ok := deps.WorkflowOrchestrator.(*workflow.Orchestrator); ok {
		stepProvider = concreteOrchestrator.GetStepProvider()
	}

	// Create tool dependencies
	toolDeps := tools.ToolDependencies{
		StepProvider:   stepProvider,
		SessionManager: deps.SessionManager,
		Logger:         logger,
	}

	// Create MCP request
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: params,
		},
	}

	// Get the handler for this tool
	var handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

	if toolConfig.CustomHandler != nil {
		// Use custom handler if provided
		handler = toolConfig.CustomHandler(toolDeps)
	} else {
		// Create handler based on category
		switch toolConfig.Category {
		case tools.CategoryWorkflow:
			handler = tools.CreateWorkflowHandler(*toolConfig, toolDeps)
		case tools.CategoryOrchestration:
			handler = tools.CreateOrchestrationHandler(*toolConfig, toolDeps)
		case tools.CategoryUtility:
			handler = tools.CreateUtilityHandler(*toolConfig, toolDeps)
		default:
			return "", fmt.Errorf("unknown tool category: %s", toolConfig.Category)
		}
	}

	// Execute the tool
	result, err := handler(ctx, request)
	if err != nil {
		errorResult := tools.ToolResult{
			Success: false,
			Error:   err.Error(),
		}
		output, _ := json.Marshal(errorResult)
		return string(output), nil
	}

	// Extract text content from result
	if result != nil && len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	// Fallback
	output, _ := json.Marshal(result)
	return string(output), nil
}

// needsSessionId checks if a tool requires a session_id parameter
func needsSessionId(toolName string) bool {
	toolsNeedingSession := []string{
		"analyze_repository",
		"resolve_base_images",
		"verify_dockerfile",
		"build_image",
		"scan_image",
		"tag_image",
		"push_image",
		"verify_k8s_manifests",
		"prepare_cluster",
		"deploy_application",
		"verify_deployment",
		"start_workflow",
		"workflow_status",
	}

	for _, name := range toolsNeedingSession {
		if name == toolName {
			return true
		}
	}
	return false
}

// generateSessionId creates a unique session ID
func generateSessionId() string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("session-%s-%d", timestamp, time.Now().UnixNano()%1000000)
}
