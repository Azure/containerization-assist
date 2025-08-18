package prompts

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	domainprompts "github.com/Azure/containerization-assist/pkg/mcp/domain/prompts"
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
	watcher   *HotReloadWatcher // Hot-reload watcher
}

type ManagerConfig struct {
	TemplateDir     string // External template directory
	EnableHotReload bool   // Watch for file changes
	AllowOverride   bool   // Allow external templates to override embedded ones
}

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

	// Start hot-reload watcher if enabled
	if config.EnableHotReload {
		ctx := context.Background()
		if err := m.StartHotReload(ctx); err != nil {
			m.logger.Warn("Failed to start hot-reload watcher", "error", err)
		}
	}

	return m, nil
}

func (m *Manager) GetTemplate(id string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	template, exists := m.templates[id]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", id)
	}

	return template, nil
}

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

// GetPrompt implements domainprompts.Manager interface
func (m *Manager) GetPrompt(ctx context.Context, promptID string) (domainprompts.Template, error) {
	template, err := m.GetTemplate(promptID)
	if err != nil {
		return domainprompts.Template{}, err
	}

	// Convert to domain Template
	var variables []domainprompts.Variable
	for _, param := range template.Parameters {
		variables = append(variables, domainprompts.Variable{
			Name:        param.Name,
			Type:        param.Type,
			Required:    param.Required,
			Default:     param.Default,
			Description: param.Description,
		})
	}

	return domainprompts.Template{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Content:     template.Template,
		Variables:   variables,
		Metadata: map[string]interface{}{
			"category":    template.Category,
			"tags":        template.Tags,
			"version":     template.Version,
			"max_tokens":  template.MaxTokens,
			"temperature": template.Temperature,
		},
	}, nil
}

// ListPrompts implements domainprompts.Manager interface
func (m *Manager) ListPrompts(ctx context.Context) ([]domainprompts.PromptSummary, error) {
	templates := m.ListTemplates()

	summaries := make([]domainprompts.PromptSummary, 0, len(templates))
	for _, template := range templates {
		summaries = append(summaries, domainprompts.PromptSummary{
			ID:          template.ID,
			Name:        template.Name,
			Description: template.Description,
		})
	}

	return summaries, nil
}

// RenderPrompt implements domainprompts.Manager interface
func (m *Manager) RenderPrompt(ctx context.Context, promptID string, variables map[string]interface{}) (string, error) {
	// Use existing RenderTemplate method
	rendered, err := m.RenderTemplate(promptID, variables)
	if err != nil {
		return "", err
	}
	return rendered.Content, nil
}

// RegisterPrompt implements domainprompts.Manager interface
func (m *Manager) RegisterPrompt(ctx context.Context, template domainprompts.Template) error {
	// Convert domain Template to internal Template
	var parameters []Parameter
	for _, variable := range template.Variables {
		parameters = append(parameters, Parameter{
			Name:        variable.Name,
			Type:        variable.Type,
			Required:    variable.Required,
			Default:     variable.Default,
			Description: variable.Description,
		})
	}

	// Extract metadata
	category := "general"
	if cat, ok := template.Metadata["category"].(string); ok {
		category = cat
	}

	var tags []string
	if tagSlice, ok := template.Metadata["tags"].([]string); ok {
		tags = tagSlice
	}

	version := "1.0.0"
	if ver, ok := template.Metadata["version"].(string); ok {
		version = ver
	}

	maxTokens := int32(2048)
	if mt, ok := template.Metadata["max_tokens"].(int32); ok {
		maxTokens = mt
	}

	temperature := float32(0.3)
	if temp, ok := template.Metadata["temperature"].(float32); ok {
		temperature = temp
	}

	internalTemplate := &Template{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Category:    category,
		Tags:        tags,
		Version:     version,
		Template:    template.Content,
		Parameters:  parameters,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	// Store in templates map
	m.mu.Lock()
	defer m.mu.Unlock()

	m.templates[template.ID] = internalTemplate

	m.logger.Info("Template registered",
		"id", template.ID,
		"name", template.Name,
		"category", category)

	return nil
}
