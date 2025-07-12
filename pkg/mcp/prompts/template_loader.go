package prompts

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var templateFS embed.FS

// PromptTemplate represents a versioned prompt template
type PromptTemplate struct {
	Name        string               `yaml:"name"`
	Version     string               `yaml:"version"`
	Description string               `yaml:"description"`
	Category    string               `yaml:"category"`
	Parameters  map[string]Parameter `yaml:"parameters"`
	Template    string               `yaml:"template"`
	Metadata    TemplateMetadata     `yaml:"metadata"`
}

// Parameter represents a template parameter
type Parameter struct {
	Type        string      `yaml:"type"`
	Description string      `yaml:"description"`
	Default     interface{} `yaml:"default,omitempty"`
	Required    bool        `yaml:"required,omitempty"`
	Options     []string    `yaml:"options,omitempty"`
	Examples    []string    `yaml:"examples,omitempty"`
	Pattern     string      `yaml:"pattern,omitempty"`
	Range       []int       `yaml:"range,omitempty"`
	MinLength   int         `yaml:"min_length,omitempty"`
	MaxLength   int         `yaml:"max_length,omitempty"`
	Validation  string      `yaml:"validation,omitempty"`
}

// TemplateMetadata contains template metadata
type TemplateMetadata struct {
	Tags            []string `yaml:"tags"`
	Complexity      string   `yaml:"complexity"`
	EstimatedTokens int      `yaml:"estimated_tokens"`
	LastUpdated     string   `yaml:"last_updated"`
	Author          string   `yaml:"author"`
}

// TemplateLoader manages prompt template loading and versioning
type TemplateLoader struct {
	templates map[string]*PromptTemplate
	versions  map[string][]string // template name -> versions
}

// NewTemplateLoader creates a new template loader
func NewTemplateLoader() (*TemplateLoader, error) {
	loader := &TemplateLoader{
		templates: make(map[string]*PromptTemplate),
		versions:  make(map[string][]string),
	}

	if err := loader.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return loader, nil
}

// loadTemplates loads all embedded templates
func (tl *TemplateLoader) loadTemplates() error {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			if err := tl.loadTemplate(entry.Name()); err != nil {
				return fmt.Errorf("failed to load template %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// loadTemplate loads a single template file
func (tl *TemplateLoader) loadTemplate(filename string) error {
	data, err := templateFS.ReadFile(filepath.Join("templates", filename))
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	var promptTemplate PromptTemplate
	if err := yaml.Unmarshal(data, &promptTemplate); err != nil {
		return fmt.Errorf("failed to unmarshal template: %w", err)
	}

	// Validate template
	if err := tl.validateTemplate(&promptTemplate); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Store template with version key
	key := fmt.Sprintf("%s@%s", promptTemplate.Name, promptTemplate.Version)
	tl.templates[key] = &promptTemplate

	// Track versions
	versions := tl.versions[promptTemplate.Name]
	versions = append(versions, promptTemplate.Version)
	tl.versions[promptTemplate.Name] = versions

	return nil
}

// validateTemplate validates template structure and content
func (tl *TemplateLoader) validateTemplate(pt *PromptTemplate) error {
	if pt.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if pt.Version == "" {
		return fmt.Errorf("template version is required")
	}
	if pt.Description == "" {
		return fmt.Errorf("template description is required")
	}
	if pt.Template == "" {
		return fmt.Errorf("template content is required")
	}

	// Validate template syntax
	tmpl := template.New("test").Funcs(getTemplateFuncs())
	_, err := tmpl.Parse(pt.Template)
	if err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}

	return nil
}

// GetTemplate retrieves a template by name and version
func (tl *TemplateLoader) GetTemplate(name, version string) (*PromptTemplate, error) {
	if version == "" {
		version = tl.GetLatestVersion(name)
	}

	key := fmt.Sprintf("%s@%s", name, version)
	template, exists := tl.templates[key]
	if !exists {
		return nil, fmt.Errorf("template %s version %s not found", name, version)
	}

	return template, nil
}

// GetLatestVersion returns the latest version of a template
func (tl *TemplateLoader) GetLatestVersion(name string) string {
	versions := tl.versions[name]
	if len(versions) == 0 {
		return ""
	}

	// For simplicity, return the last version (assumes sorted)
	// In production, implement proper semantic version comparison
	return versions[len(versions)-1]
}

// ListTemplates returns all available templates
func (tl *TemplateLoader) ListTemplates() map[string][]string {
	return tl.versions
}

// RenderTemplate renders a template with provided parameters
func (tl *TemplateLoader) RenderTemplate(name, version string, params map[string]interface{}) (string, error) {
	promptTemplate, err := tl.GetTemplate(name, version)
	if err != nil {
		return "", err
	}

	// Validate parameters
	if err := tl.validateParameters(promptTemplate, params); err != nil {
		return "", fmt.Errorf("parameter validation failed: %w", err)
	}

	// Fill in defaults for missing parameters
	mergedParams := tl.mergeWithDefaults(promptTemplate, params)

	// Create and execute template
	tmpl := template.New(name).Funcs(getTemplateFuncs())
	if _, err = tmpl.Parse(promptTemplate.Template); err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, mergedParams); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// validateParameters validates provided parameters against template definition
func (tl *TemplateLoader) validateParameters(pt *PromptTemplate, params map[string]interface{}) error {
	for paramName, paramDef := range pt.Parameters {
		value, exists := params[paramName]

		// Check required parameters
		if paramDef.Required && !exists {
			return fmt.Errorf("required parameter '%s' is missing", paramName)
		}

		if !exists {
			continue // Will be filled with default
		}

		// Type validation (basic)
		if err := tl.validateParameterType(paramName, paramDef, value); err != nil {
			return err
		}

		// Range validation for numbers
		if paramDef.Type == "number" && len(paramDef.Range) == 2 {
			if num, ok := value.(int); ok {
				if num < paramDef.Range[0] || num > paramDef.Range[1] {
					return fmt.Errorf("parameter '%s' value %d is out of range [%d, %d]",
						paramName, num, paramDef.Range[0], paramDef.Range[1])
				}
			}
		}

		// Length validation for strings
		if paramDef.Type == "string" {
			if str, ok := value.(string); ok {
				if paramDef.MinLength > 0 && len(str) < paramDef.MinLength {
					return fmt.Errorf("parameter '%s' is too short (min %d chars)",
						paramName, paramDef.MinLength)
				}
				if paramDef.MaxLength > 0 && len(str) > paramDef.MaxLength {
					return fmt.Errorf("parameter '%s' is too long (max %d chars)",
						paramName, paramDef.MaxLength)
				}
			}
		}

		// Pattern validation
		if paramDef.Pattern != "" && paramDef.Type == "string" {
			if str, ok := value.(string); ok {
				// Simple pattern check - in real implementation would use regexp
				if paramDef.Pattern == "^[a-z0-9-]+$" {
					// Check for lowercase alphanumeric and hyphens only
					for _, ch := range str {
						if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
							return fmt.Errorf("parameter '%s' value '%s' does not match pattern %s",
								paramName, str, paramDef.Pattern)
						}
					}
				}
			}
		}

		// Options validation
		if len(paramDef.Options) > 0 {
			if str, ok := value.(string); ok {
				valid := false
				for _, option := range paramDef.Options {
					if str == option {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("parameter '%s' value '%s' is not in allowed options: %v",
						paramName, str, paramDef.Options)
				}
			}
		}
	}

	return nil
}

// validateParameterType performs basic type validation
func (tl *TemplateLoader) validateParameterType(name string, def Parameter, value interface{}) error {
	switch def.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter '%s' must be a string", name)
		}
	case "number":
		switch value.(type) {
		case int, int64, float64:
			// OK
		default:
			return fmt.Errorf("parameter '%s' must be a number", name)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter '%s' must be a boolean", name)
		}
	case "array":
		// Basic array check
		switch value.(type) {
		case []interface{}, []string:
			// OK
		default:
			return fmt.Errorf("parameter '%s' must be an array", name)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("parameter '%s' must be an object", name)
		}
	}
	return nil
}

// mergeWithDefaults merges provided parameters with template defaults
func (tl *TemplateLoader) mergeWithDefaults(pt *PromptTemplate, params map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Start with defaults
	for paramName, paramDef := range pt.Parameters {
		if paramDef.Default != nil {
			result[paramName] = paramDef.Default
		}
	}

	// Override with provided parameters
	for key, value := range params {
		result[key] = value
	}

	return result
}

// GetTemplateInfo returns template information without rendering
func (tl *TemplateLoader) GetTemplateInfo(name, version string) (*TemplateInfo, error) {
	template, err := tl.GetTemplate(name, version)
	if err != nil {
		return nil, err
	}

	return &TemplateInfo{
		Name:              template.Name,
		Version:           template.Version,
		Description:       template.Description,
		Category:          template.Category,
		Parameters:        template.Parameters,
		Metadata:          template.Metadata,
		AvailableVersions: tl.versions[name],
	}, nil
}

// TemplateInfo provides template information
type TemplateInfo struct {
	Name              string               `json:"name"`
	Version           string               `json:"version"`
	Description       string               `json:"description"`
	Category          string               `json:"category"`
	Parameters        map[string]Parameter `json:"parameters"`
	Metadata          TemplateMetadata     `json:"metadata"`
	AvailableVersions []string             `json:"available_versions"`
}

// getTemplateFuncs returns custom template functions
func getTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"contains": func(slice interface{}, item string) bool {
			switch s := slice.(type) {
			case []string:
				for _, v := range s {
					if v == item {
						return true
					}
				}
			case []interface{}:
				for _, v := range s {
					if str, ok := v.(string); ok && str == item {
						return true
					}
				}
			}
			return false
		},
	}
}
