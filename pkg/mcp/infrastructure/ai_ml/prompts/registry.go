// Package prompts provides MCP prompt registration and management
package prompts

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Registry manages MCP prompts with native mcp-go support
type Registry struct {
	logger   *slog.Logger
	handlers map[string]server.PromptHandlerFunc
	mu       sync.RWMutex
}

// NewRegistry creates a new prompt registry with native MCP prompt support
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		logger:   logger.With("component", "prompt-registry"),
		handlers: make(map[string]server.PromptHandlerFunc),
	}
}

// RegisterAll registers all Container Kit prompts using native mcp-go support
func (r *Registry) RegisterAll(mcpServer *server.MCPServer) error {
	r.logger.Info("Registering Container Kit prompts with native mcp-go support")

	// Register containerization prompts using native mcp-go API
	prompts := map[string]string{
		"containerize_repository": "Containerize a repository using Container Kit",
	}

	for name, description := range prompts {
		prompt := mcp.NewPrompt(name, mcp.WithPromptDescription(description))

		// Create and store the handler
		handler := r.createBasicHandler(name, description)
		r.mu.Lock()
		r.handlers[name] = handler
		mcpServer.AddPrompt(prompt, handler)
		r.mu.Unlock()
		r.logger.Info("Registered prompt", "name", name, "description", description)
	}

	r.logger.Info("Container Kit prompts registered successfully", "count", len(prompts))
	return nil
}

// createBasicHandler creates a basic prompt handler for containerization tasks
func (r *Registry) createBasicHandler(name, description string) server.PromptHandlerFunc {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Create a basic containerization prompt based on the prompt name
		var systemPrompt, userPrompt string

		switch name {
		case "containerize_repository":
			userPrompt = "You are an expert in Kubernetes and docker. Execute the #analyze_repository tool and execute subsequent tools from the Container Kit mcp based on results to containerize the application. Update file contents as instructed by tool responses. When a tool fails, attempt to update Kubernetes manifests and dockerfiles before retrying the tool a maximum of 5 times."
		default:
			systemPrompt = "You are a containerization expert."
			userPrompt = "Please help with containerization tasks."
		}

		messages := []mcp.PromptMessage{}
		if userPrompt != "" {
			messages = append(messages, mcp.PromptMessage{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: userPrompt,
				},
			})
		}
		if systemPrompt != "" {
			messages = append(messages, mcp.PromptMessage{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: systemPrompt,
				},
			})
		}

		return &mcp.GetPromptResult{
			Description: description,
			Messages:    messages,
		}, nil
	}
}
