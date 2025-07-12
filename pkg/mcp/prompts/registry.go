// Package prompts provides MCP prompt registration and management
package prompts

import (
	"fmt"
	"log/slog"

	"github.com/localrivet/gomcp/server"
)

// Registry manages MCP prompts with template versioning
type Registry struct {
	server server.Server
	logger *slog.Logger
	loader *TemplateLoader
}

// NewRegistry creates a new prompt registry with template support
func NewRegistry(s server.Server, logger *slog.Logger) *Registry {
	loader, err := NewTemplateLoader()
	if err != nil {
		logger.Error("Failed to initialize template loader", "error", err)
		// Continue without templates for fallback
	}

	return &Registry{
		server: s,
		logger: logger.With("component", "prompt-registry"),
		loader: loader,
	}
}

// RegisterAll registers all Container Kit prompts
func (r *Registry) RegisterAll() error {
	r.logger.Info("MCP prompts feature not yet supported by gomcp library - using template system")

	if r.loader == nil {
		r.logger.Warn("Template loader not available, skipping prompt registration")
		return nil
	}

	// Log available templates
	templates := r.loader.ListTemplates()
	r.logger.Info("Available prompt templates", "count", len(templates))

	for name := range templates {
		latestVersion := r.loader.GetLatestVersion(name)
		info, err := r.loader.GetTemplateInfo(name, latestVersion)
		if err != nil {
			r.logger.Error("Failed to get template info", "template", name, "error", err)
			continue
		}

		r.logger.Info("Template loaded",
			"name", name,
			"version", latestVersion,
			"category", info.Category,
			"complexity", info.Metadata.Complexity,
			"estimated_tokens", info.Metadata.EstimatedTokens)
	}

	// TODO: Enable when gomcp library supports prompt registration
	// The gomcp library currently doesn't expose a RegisterPrompt or Prompt method
	// This will be implemented when the MCP specification for prompts is fully supported
	//
	// For now, templates are available through the template loader API

	r.logger.Info("Container Kit prompts loaded and ready for template rendering")
	return nil
}

// RenderPrompt renders a prompt template with given parameters
func (r *Registry) RenderPrompt(name, version string, params map[string]interface{}) (string, error) {
	if r.loader == nil {
		return "", fmt.Errorf("template loader not available")
	}

	return r.loader.RenderTemplate(name, version, params)
}

// GetPromptInfo returns information about a prompt template
func (r *Registry) GetPromptInfo(name, version string) (*TemplateInfo, error) {
	if r.loader == nil {
		return nil, fmt.Errorf("template loader not available")
	}

	return r.loader.GetTemplateInfo(name, version)
}

// ListPrompts returns all available prompt templates
func (r *Registry) ListPrompts() map[string][]string {
	if r.loader == nil {
		return make(map[string][]string)
	}

	return r.loader.ListTemplates()
}

// GetLatestVersion returns the latest version of a prompt template
func (r *Registry) GetLatestVersion(name string) string {
	if r.loader == nil {
		return ""
	}

	return r.loader.GetLatestVersion(name)
}
