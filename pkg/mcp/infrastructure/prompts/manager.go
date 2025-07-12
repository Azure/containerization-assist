// Package prompts provides template management functionality
package prompts

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var embeddedTemplates embed.FS

// Manager manages prompt templates with hot-reload support
type Manager struct {
	templates map[string]*Template
	mu        sync.RWMutex
	logger    *slog.Logger
	config    ManagerConfig
}

// ManagerConfig holds configuration for the template manager
type ManagerConfig struct {
	TemplateDir     string // External template directory
	EnableHotReload bool   // Watch for file changes
	AllowOverride   bool   // Allow external templates to override embedded ones
}

// NewManager creates a new template manager
func NewManager(logger *slog.Logger, config ManagerConfig) (*Manager, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	}

	m := &Manager{
		templates: make(map[string]*Template),
		logger:    logger.With("component", "prompt-manager"),
		config:    config,
	}

	// Load embedded templates first
	if err := m.loadEmbeddedTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load embedded templates: %w", err)
	}

	// Load external templates if directory is specified
	if config.TemplateDir != "" {
		if err := m.loadExternalTemplates(); err != nil {
			return nil, fmt.Errorf("failed to load external templates: %w", err)
		}
	}

	m.logger.Info("Template manager initialized",
		"embedded_count", len(m.templates),
		"template_dir", config.TemplateDir,
		"hot_reload", config.EnableHotReload)

	return m, nil
}

// GetTemplate retrieves a template by ID
func (m *Manager) GetTemplate(id string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	template, exists := m.templates[id]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", id)
	}

	return template, nil
}

// RenderTemplate renders a template with the given data
func (m *Manager) RenderTemplate(id string, data TemplateData) (*RenderedPrompt, error) {
	template, err := m.GetTemplate(id)
	if err != nil {
		return nil, err
	}

	rendered, err := template.Render(data)
	if err != nil {
		m.logger.Error("Template rendering failed",
			"template_id", id,
			"error", err)
		return nil, err
	}

	m.logger.Debug("Template rendered successfully",
		"template_id", id,
		"content_length", len(rendered.Content),
		"parameters", len(rendered.Parameters))

	return rendered, nil
}

// ListTemplates returns all available templates
func (m *Manager) ListTemplates() []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*Template, 0, len(m.templates))
	for _, template := range m.templates {
		templates = append(templates, template)
	}

	// Sort by category then name
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Category != templates[j].Category {
			return templates[i].Category < templates[j].Category
		}
		return templates[i].Name < templates[j].Name
	})

	return templates
}

// GetTemplatesByCategory returns templates in a specific category
func (m *Manager) GetTemplatesByCategory(category string) []*Template {
	all := m.ListTemplates()
	var filtered []*Template

	for _, template := range all {
		if template.Category == category {
			filtered = append(filtered, template)
		}
	}

	return filtered
}

// GetTemplatesByTag returns templates with a specific tag
func (m *Manager) GetTemplatesByTag(tag string) []*Template {
	all := m.ListTemplates()
	var filtered []*Template

	for _, template := range all {
		for _, templateTag := range template.Tags {
			if templateTag == tag {
				filtered = append(filtered, template)
				break
			}
		}
	}

	return filtered
}

// ReloadTemplates reloads all templates from disk
func (m *Manager) ReloadTemplates() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing templates
	m.templates = make(map[string]*Template)

	// Reload embedded templates
	if err := m.loadEmbeddedTemplates(); err != nil {
		return fmt.Errorf("failed to reload embedded templates: %w", err)
	}

	// Reload external templates
	if m.config.TemplateDir != "" {
		if err := m.loadExternalTemplates(); err != nil {
			return fmt.Errorf("failed to reload external templates: %w", err)
		}
	}

	m.logger.Info("Templates reloaded", "count", len(m.templates))
	return nil
}

// loadEmbeddedTemplates loads templates from embedded filesystem
func (m *Manager) loadEmbeddedTemplates() error {
	return fs.WalkDir(embeddedTemplates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := embeddedTemplates.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", path, err)
		}

		template, err := m.parseTemplate(data, path, "embedded")
		if err != nil {
			return fmt.Errorf("failed to parse embedded template %s: %w", path, err)
		}

		m.templates[template.ID] = template
		return nil
	})
}

// loadExternalTemplates loads templates from external directory
func (m *Manager) loadExternalTemplates() error {
	if _, err := os.Stat(m.config.TemplateDir); os.IsNotExist(err) {
		m.logger.Warn("Template directory does not exist", "dir", m.config.TemplateDir)
		return nil
	}

	return filepath.Walk(m.config.TemplateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read external template %s: %w", path, err)
		}

		template, err := m.parseTemplate(data, path, "external")
		if err != nil {
			m.logger.Error("Failed to parse external template",
				"path", path,
				"error", err)
			return nil // Continue loading other templates
		}

		// Check for override
		if _, exists := m.templates[template.ID]; exists && !m.config.AllowOverride {
			m.logger.Warn("Template override not allowed",
				"template_id", template.ID,
				"existing_source", "embedded",
				"new_source", "external")
			return nil
		}

		if existing, exists := m.templates[template.ID]; exists {
			m.logger.Info("Template overridden",
				"template_id", template.ID,
				"previous_version", existing.Version,
				"new_version", template.Version)
		}

		m.templates[template.ID] = template
		return nil
	})
}

// parseTemplate parses a YAML template
func (m *Manager) parseTemplate(data []byte, path, source string) (*Template, error) {
	var template Template
	if err := yaml.Unmarshal(data, &template); err != nil {
		return nil, err
	}

	// Validate required fields
	if template.ID == "" {
		return nil, fmt.Errorf("template ID is required")
	}
	if template.Template == "" {
		return nil, fmt.Errorf("template content is required")
	}

	// Set defaults
	if template.MaxTokens == 0 {
		template.MaxTokens = 2048
	}
	if template.Temperature == 0 {
		template.Temperature = 0.3
	}
	if template.Version == "" {
		template.Version = "1.0.0"
	}

	m.logger.Debug("Template loaded",
		"id", template.ID,
		"name", template.Name,
		"category", template.Category,
		"source", source,
		"path", path)

	return &template, nil
}

// GetStats returns statistics about loaded templates
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categories := make(map[string]int)
	tags := make(map[string]int)

	for _, template := range m.templates {
		categories[template.Category]++
		for _, tag := range template.Tags {
			tags[tag]++
		}
	}

	return map[string]interface{}{
		"total_templates": len(m.templates),
		"categories":      categories,
		"tags":            tags,
	}
}
