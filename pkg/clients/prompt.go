package clients

import (
	"fmt"
	"path/filepath"

	"github.com/Azure/container-copilot/pkg/prompt"
)

// PromptClient provides an interface for working with prompt templates
type PromptClient struct {
	manager       prompt.Manager
	templateCache map[string]string
}

// NewPromptClient creates a new PromptClient with the given templates directory
func NewPromptClient(templatesDir string) (*PromptClient, error) {
	absPath, err := filepath.Abs(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for templates directory: %w", err)
	}

	manager := prompt.NewFileSystemManager(absPath)
	return &PromptClient{
		manager:       manager,
		templateCache: make(map[string]string),
	}, nil
}

// GetTemplate retrieves a prompt template by name
// Templates are cached in memory after the first load
func (c *PromptClient) GetTemplate(name string) (string, error) {
	// First check the cache
	if template, found := c.templateCache[name]; found {
		return template, nil
	}

	// Not in cache, load from file system
	template, err := c.manager.GetTemplate(name)
	if err != nil {
		return "", err
	}

	// Store in cache for future use
	c.templateCache[name] = template

	return template, nil
}

// ListTemplates returns all available template names
func (c *PromptClient) ListTemplates() ([]string, error) {
	return c.manager.ListTemplates()
}

// PreloadTemplates loads all templates into memory cache
func (c *PromptClient) PreloadTemplates() error {
	templateNames, err := c.ListTemplates()
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	for _, name := range templateNames {
		_, err := c.GetTemplate(name)
		if err != nil {
			return fmt.Errorf("failed to preload template %s: %w", name, err)
		}
	}

	return nil
}
