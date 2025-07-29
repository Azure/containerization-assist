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
	server interface {
		AddPrompt(prompt mcp.Prompt, handler server.PromptHandlerFunc)
	}
	logger   *slog.Logger
	handlers map[string]server.PromptHandlerFunc
	mu       sync.RWMutex
}

// NewRegistry creates a new prompt registry with native MCP prompt support
func NewRegistry(s interface {
	AddPrompt(prompt mcp.Prompt, handler server.PromptHandlerFunc)
}, logger *slog.Logger) *Registry {
	return &Registry{
		server:   s,
		logger:   logger.With("component", "prompt-registry"),
		handlers: make(map[string]server.PromptHandlerFunc),
	}
}

// RegisterAll registers all Container Kit prompts using native mcp-go support
func (r *Registry) RegisterAll() error {
	r.logger.Info("Registering Container Kit prompts with native mcp-go support")

	// Register containerization prompts using native mcp-go API
	prompts := map[string]string{
		"analyze_dockerfile_errors": "Analyze Dockerfile for issues and suggest fixes",
		"analyze_manifest_errors":   "Analyze Kubernetes manifests for issues and suggest fixes",
		"analyze_repository":        "Analyze repository for containerization requirements",
		"generate_dockerfile":       "Generate optimized Dockerfile for a repository",
	}

	for name, description := range prompts {
		prompt := mcp.NewPrompt(name, mcp.WithPromptDescription(description))

		// Create and store the handler
		handler := r.createBasicHandler(name, description)
		r.mu.Lock()
		r.handlers[name] = handler
		r.mu.Unlock()
		r.server.AddPrompt(prompt, handler)
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
		case "analyze_dockerfile_errors":
			systemPrompt = "You are an expert Dockerfile analyst and containerization specialist."
			userPrompt = "Please analyze the provided Dockerfile for potential issues and suggest fixes. Look for security vulnerabilities, optimization opportunities, and best practices violations."
		case "analyze_manifest_errors":
			systemPrompt = "You are an expert Kubernetes administrator and YAML configuration specialist."
			userPrompt = "Please analyze the provided Kubernetes manifest for potential issues and suggest fixes. Check for resource limits, networking, security, and deployment best practices."
		case "analyze_repository":
			systemPrompt = "You are an expert in application containerization and DevOps practices."
			userPrompt = "Please analyze this repository to determine the best containerization approach. Identify the language, framework, dependencies, and provide containerization recommendations."
		case "generate_dockerfile":
			systemPrompt = "You are an expert in creating production-ready, optimized Dockerfiles."
			userPrompt = "Please generate a production-ready, multi-stage Dockerfile based on the repository analysis. Include security best practices, proper layer caching, and optimization techniques."
		default:
			systemPrompt = "You are a containerization expert."
			userPrompt = "Please help with containerization tasks."
		}

		messages := []mcp.PromptMessage{
			{
				Role: "system",
				Content: mcp.TextContent{
					Type: "text",
					Text: systemPrompt,
				},
			},
			{
				Role: "user",
				Content: mcp.TextContent{
					Type: "text",
					Text: userPrompt,
				},
			},
		}

		return &mcp.GetPromptResult{
			Description: description,
			Messages:    messages,
		}, nil
	}
}
