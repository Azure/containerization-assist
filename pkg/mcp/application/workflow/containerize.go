package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/core/docker"
	domainworkflow "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterWorkflowTools registers the single comprehensive workflow tool
func RegisterWorkflowTools(mcpServer interface {
	AddTool(tool mcp.Tool, handler server.ToolHandlerFunc)
}, orchestrator domainworkflow.WorkflowOrchestrator, logger *slog.Logger) error {
	logger.Info("Registering workflow tools")

	// Register the single containerize_and_deploy workflow tool
	tool := mcp.Tool{
		Name:        "containerize_and_deploy",
		Description: "Complete end-to-end containerization and deployment with AI-powered error fixing",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "Repository URL to containerize",
				},
				"repo_path": map[string]interface{}{
					"type":        "string",
					"description": "Local repository path to containerize (alternative to repo_url)",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Branch to use (optional, only applicable when using repo_url)",
				},
				"scan": map[string]interface{}{
					"type":        "boolean",
					"description": "Run security scan (optional)",
				},
				"deploy": map[string]interface{}{
					"type":        "boolean",
					"description": "Deploy to Kubernetes (optional, defaults to true)",
				},
				"test_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Test mode - skip actual Docker operations (optional)",
				},
			},
		},
	}

	mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.GetArguments()

		// Extract arguments
		args := domainworkflow.ContainerizeAndDeployArgs{}

		// Validate that either repo_url or repo_path is provided
		repoURL, hasRepoURL := arguments["repo_url"].(string)
		repoPath, hasRepoPath := arguments["repo_path"].(string)

		if !hasRepoURL && !hasRepoPath {
			return nil, fmt.Errorf("either repo_url or repo_path is required")
		}

		if hasRepoURL && hasRepoPath {
			return nil, fmt.Errorf("cannot specify both repo_url and repo_path, choose one")
		}

		if hasRepoURL {
			args.RepoURL = repoURL
		} else {
			args.RepoPath = repoPath
		}

		if branch, ok := arguments["branch"].(string); ok {
			if hasRepoPath {
				return nil, fmt.Errorf("branch parameter is only applicable when using repo_url")
			}
			args.Branch = branch
		}

		if scan, ok := arguments["scan"].(bool); ok {
			args.Scan = scan
		}

		if deploy, ok := arguments["deploy"].(bool); ok {
			args.Deploy = &deploy
		}

		if testMode, ok := arguments["test_mode"].(bool); ok {
			args.TestMode = testMode
		}

		// Check if Docker is available (skip in test mode)
		if !args.TestMode {
			if err := docker.CheckDockerInstalled(); err != nil {
				return nil, fmt.Errorf("docker is not installed: %v", err)
			}

			dockerClient := docker.NewDockerCmdRunner(&runner.DefaultCommandRunner{})
			if _, err := dockerClient.Info(ctx); err != nil {
				return nil, fmt.Errorf("docker daemon is not running or not accessible: %v", err)
			}
		}

		// Use injected orchestrator
		result, err := orchestrator.Execute(ctx, &req, &args)
		if err != nil {
			return nil, err
		}

		// Marshal result to JSON
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(resultJSON),
				},
			},
		}, nil
	})

	logger.Info("Workflow tools registered successfully")
	return nil
}
