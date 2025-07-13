// Package registrar handles tool and prompt registration
package registrar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolRegistrar handles tool registration
type ToolRegistrar struct {
	logger    *slog.Logger
	startTime time.Time
}

// NewToolRegistrar creates a new tool registrar
func NewToolRegistrar(logger *slog.Logger) *ToolRegistrar {
	return &ToolRegistrar{
		logger:    logger.With("component", "tool_registrar"),
		startTime: time.Now(),
	}
}

// RegisterAll registers all tools with the MCP server
func (tr *ToolRegistrar) RegisterAll(mcpServer *server.MCPServer) error {
	tr.logger.Info("Registering tools")

	// Register workflow tools
	tr.logger.Info("Registering single comprehensive workflow tool for AI-powered containerization")
	if err := workflow.RegisterWorkflowTools(mcpServer, tr.logger); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "registrar", "failed to register workflow tools", err)
	}

	// Register diagnostic tools
	if err := tr.registerDiagnosticTools(mcpServer); err != nil {
		return err
	}

	tr.logger.Info("All tools registered successfully")
	return nil
}

// registerDiagnosticTools registers diagnostic tools like ping and status
func (tr *ToolRegistrar) registerDiagnosticTools(mcpServer *server.MCPServer) error {
	// Register ping tool
	pingTool := mcp.Tool{
		Name:        "ping",
		Description: "Simple ping tool to test MCP connectivity",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Optional message to echo back",
				},
			},
		},
	}

	mcpServer.AddTool(pingTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	})

	// Register status tool
	statusTool := mcp.Tool{
		Name:        "server_status",
		Description: "Get basic server status information",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"details": map[string]interface{}{
					"type":        "boolean",
					"description": "Include detailed information",
				},
			},
		},
	}

	mcpServer.AddTool(statusTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			Uptime:  time.Since(tr.startTime).String(),
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
	})

	return nil
}
