// Package prompts provides MCP prompt registration and management
package prompts

import (
	"context"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Registry manages MCP prompts with native mcp-go support
type Registry struct {
}

// NewRegistry creates a new prompt registry with native MCP prompt support
func NewRegistry(logger *slog.Logger) *Registry {
	_ = logger // Keep for API compatibility
	return &Registry{}
}

// RegisterAll registers all Containerization Assist prompts using native mcp-go support
func (r *Registry) RegisterAll(mcpServer *server.MCPServer) error {

	for _, p := range prompts {
		mcpServer.AddPrompt(p.GetPrompt(), p.GetHandler())
	}
	return nil
}

var prompts = []PromptAndHandler{
	{
		toolname:    "containerize_repository",
		description: "Containerize a repository",
		userText: []string{"You are an expert in Kubernetes and Docker.",
			"Execute the #analyze_repository tool and execute subsequent tools from the Containerization Assist MCP based on results to containerize the application.",
			"Update file contents as instructed by tool responses.",
			"When a tool fails, attempt to update Kubernetes manifests and Dockerfiles before retrying the tool a maximum of 5 times.",
			"Write manifest files to the ./manifests directory",
		},
	},
}

type PromptAndHandler struct {
	toolname    string
	description string
	userText    []string
}

func (p *PromptAndHandler) GetPrompt() mcp.Prompt {
	return mcp.NewPrompt(p.toolname, mcp.WithPromptDescription(p.description))
}

func (p *PromptAndHandler) GetHandler() server.PromptHandlerFunc {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: p.description,
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: strings.Join(p.userText, "\n"),
					},
				},
			},
		}, nil
	}
}
