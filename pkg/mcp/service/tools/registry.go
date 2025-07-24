package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkg/errors"
	"log/slog"

	domainworkflow "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
)

// ToolCategory defines the type of tool
type ToolCategory string

const (
	CategoryWorkflow      ToolCategory = "workflow"
	CategoryOrchestration ToolCategory = "orchestration"
	CategoryUtility       ToolCategory = "utility"
)

// ToolConfig defines the configuration for a tool
type ToolConfig struct {
	// Basic metadata
	Name        string
	Description string
	Category    ToolCategory

	// Input schema parameters
	RequiredParams []string
	OptionalParams map[string]interface{}

	// Dependencies configuration
	NeedsStepProvider    bool
	NeedsProgressFactory bool
	NeedsSessionManager  bool
	NeedsLogger          bool

	// Workflow-specific configuration
	StepGetterName string // Name of the StepProvider method (e.g., "GetAnalyzeStep")

	// Chain hint configuration
	NextTool    string
	ChainReason string // Can include %s placeholders for dynamic values

	// Custom handler (optional - for special tools)
	CustomHandler func(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// ToolDependencies holds all possible dependencies a tool might need
type ToolDependencies struct {
	StepProvider    domainworkflow.StepProvider
	ProgressFactory domainworkflow.ProgressEmitterFactory
	SessionManager  session.OptimizedSessionManager
	Logger          *slog.Logger
}

// ToolResult represents a tool execution result
type ToolResult struct {
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	ChainHint *ChainHint             `json:"chain_hint,omitempty"`
}

// ChainHint provides information about the next suggested tool
type ChainHint struct {
	NextTool string `json:"next_tool"`
	Reason   string `json:"reason"`
}

// createToolResult creates a standardized tool result
func createToolResult(success bool, data map[string]interface{}, chainHint *ChainHint) mcp.CallToolResult {
	return mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: MarshalJSON(ToolResult{Success: success, Data: data, ChainHint: chainHint}),
			},
		},
	}
}

// createErrorResult creates a standardized error result
func createErrorResult(err error) mcp.CallToolResult {
	return mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: MarshalJSON(ToolResult{Success: false, Error: err.Error()}),
			},
		},
	}
}

// createChainHint creates a chain hint for tool chaining
func createChainHint(nextTool, reason string) *ChainHint {
	if nextTool == "" {
		return nil
	}
	return &ChainHint{
		NextTool: nextTool,
		Reason:   reason,
	}
}

// All tool configurations in a single table
var toolConfigs = []ToolConfig{
	// Workflow Step Tools
	{
		Name:                 "analyze_repository",
		Description:          "Analyze repository to detect language, framework, and build requirements",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"repo_path", "session_id"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetAnalyzeStep",
		NextTool:             "generate_dockerfile",
		ChainReason:          "Repository analyzed successfully. Ready to generate Dockerfile",
	},
	{
		Name:                 "generate_dockerfile",
		Description:          "Generate an optimized Dockerfile based on repository analysis",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetDockerfileStep",
		NextTool:             "build_image",
		ChainReason:          "Dockerfile generated successfully. Ready to build Docker image",
	},
	{
		Name:                 "build_image",
		Description:          "Build Docker image from generated Dockerfile",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetBuildStep",
		NextTool:             "scan_image",
		ChainReason:          "Docker image built successfully. Ready for security scanning",
	},
	{
		Name:                 "scan_image",
		Description:          "Scan the Docker image for security vulnerabilities",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetScanStep",
		NextTool:             "tag_image",
		ChainReason:          "Security scan completed. Ready to tag image",
	},
	{
		Name:                 "tag_image",
		Description:          "Tag the Docker image with version and metadata",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id", "tag"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetTagStep",
		NextTool:             "push_image",
		ChainReason:          "Image tagged successfully. Ready to push to registry",
	},
	{
		Name:                 "push_image",
		Description:          "Push the Docker image to a container registry",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id", "registry"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetPushStep",
		NextTool:             "generate_k8s_manifests",
		ChainReason:          "Image pushed successfully. Ready to generate Kubernetes manifests",
	},
	{
		Name:                 "generate_k8s_manifests",
		Description:          "Generate Kubernetes manifests for the application",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetManifestStep",
		NextTool:             "prepare_cluster",
		ChainReason:          "Kubernetes manifests generated. Ready to prepare cluster",
	},
	{
		Name:                 "prepare_cluster",
		Description:          "Prepare the Kubernetes cluster for deployment",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id"},
		OptionalParams:       map[string]interface{}{"cluster_config": "object"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetPrepareClusterStep",
		NextTool:             "deploy_application",
		ChainReason:          "Cluster prepared successfully. Ready to deploy application",
	},
	{
		Name:                 "deploy_application",
		Description:          "Deploy the application to Kubernetes",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetDeployStep",
		NextTool:             "verify_deployment",
		ChainReason:          "Application deployed successfully. Ready to verify deployment",
	},
	{
		Name:                 "verify_deployment",
		Description:          "Verify the deployment is healthy and running correctly",
		Category:             CategoryWorkflow,
		RequiredParams:       []string{"session_id"},
		NeedsStepProvider:    true,
		NeedsProgressFactory: true,
		NeedsSessionManager:  true,
		NeedsLogger:          true,
		StepGetterName:       "GetVerifyStep",
		NextTool:             "", // End of workflow
		ChainReason:          "Deployment verified successfully. Workflow complete!",
	},

	// Orchestration Tools
	{
		Name:           "start_workflow",
		Description:    "Start a complete containerization workflow",
		Category:       CategoryOrchestration,
		RequiredParams: []string{"repo_path"},
		NeedsLogger:    true,
		NextTool:       "workflow_status",
		ChainReason:    "Workflow started. Use workflow_status to check progress",
	},
	{
		Name:           "workflow_status",
		Description:    "Check the status of a running workflow",
		Category:       CategoryOrchestration,
		RequiredParams: []string{"session_id"},
		NeedsLogger:    true,
	},

	// Utility Tools
	{
		Name:        "list_tools",
		Description: "List all available MCP tools and their descriptions",
		Category:    CategoryUtility,
		// No dependencies needed
	},

	// Diagnostic Tools
	{
		Name:        "ping",
		Description: "Simple ping tool to test MCP connectivity",
		Category:    CategoryUtility,
		OptionalParams: map[string]interface{}{
			"message": "string",
		},
		CustomHandler: createPingHandler,
	},
	{
		Name:        "server_status",
		Description: "Get basic server status information",
		Category:    CategoryUtility,
		OptionalParams: map[string]interface{}{
			"details": "boolean",
		},
		CustomHandler: createServerStatusHandler,
	},
}

// GetToolConfigs returns all tool configurations
func GetToolConfigs() []ToolConfig {
	return toolConfigs
}

// GetToolConfig returns a specific tool configuration by name
func GetToolConfig(name string) (*ToolConfig, error) {
	for _, config := range toolConfigs {
		if config.Name == name {
			return &config, nil
		}
	}
	return nil, errors.Errorf("tool %s not found", name)
}

// BuildToolSchema creates the MCP input schema for a tool
func BuildToolSchema(config ToolConfig) mcp.ToolInputSchema {
	properties := make(map[string]interface{})
	required := config.RequiredParams

	// Add required parameters
	for _, param := range config.RequiredParams {
		properties[param] = map[string]interface{}{
			"type":        "string",
			"description": getParamDescription(param),
		}
	}

	// Add optional parameters
	for param, paramType := range config.OptionalParams {
		paramSchema := map[string]interface{}{
			"description": getParamDescription(param),
		}

		// Handle different parameter types
		switch paramType {
		case "array":
			// Arrays must have an items schema for JSON Schema compliance
			paramSchema["type"] = "array"
			paramSchema["items"] = map[string]interface{}{
				"type": "string",
			}
		case "string":
			paramSchema["type"] = "string"
		case "boolean":
			paramSchema["type"] = "boolean"
		case "object":
			paramSchema["type"] = "object"
		case "number":
			paramSchema["type"] = "number"
		case "integer":
			paramSchema["type"] = "integer"
		default:
			// Default to string if type is not recognized
			paramSchema["type"] = "string"
		}

		properties[param] = paramSchema
	}

	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

// getParamDescription returns a human-readable description for common parameters
func getParamDescription(param string) string {
	descriptions := map[string]string{
		"repo_path":      "Path to the repository to analyze",
		"session_id":     "Session ID for workflow state management",
		"tag":            "Tag for the Docker image",
		"registry":       "Container registry URL",
		"cluster_config": "Kubernetes cluster configuration",
		"skip_steps":     "Comma-separated list of step names to skip",
		"cluster_type":   "Type of Kubernetes cluster (e.g., 'kind', 'aks', 'eks')",
		"registry_type":  "Type of container registry (e.g., 'dockerhub', 'acr', 'ecr')",
	}

	if desc, exists := descriptions[param]; exists {
		return desc
	}
	return fmt.Sprintf("The %s parameter", param)
}
