// Package prompts provides external prompt template management
package prompts

import (
	"fmt"
	"strings"
	"text/template"
	"time"
)

// Template represents a prompt template with metadata
type Template struct {
	ID           string            `yaml:"id" json:"id"`
	Name         string            `yaml:"name" json:"name"`
	Description  string            `yaml:"description" json:"description"`
	Version      string            `yaml:"version" json:"version"`
	Category     string            `yaml:"category" json:"category"`
	Template     string            `yaml:"template" json:"template"`
	SystemPrompt string            `yaml:"system_prompt" json:"system_prompt"`
	Parameters   []Parameter       `yaml:"parameters" json:"parameters"`
	Defaults     map[string]string `yaml:"defaults" json:"defaults"`
	MaxTokens    int32             `yaml:"max_tokens" json:"max_tokens"`
	Temperature  float32           `yaml:"temperature" json:"temperature"`
	Tags         []string          `yaml:"tags" json:"tags"`
	CreatedAt    time.Time         `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time         `yaml:"updated_at" json:"updated_at"`
}

// Parameter describes a template parameter
type Parameter struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"` // string, int, bool, etc.
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required" json:"required"`
	Default     string `yaml:"default" json:"default"`
	Example     string `yaml:"example" json:"example"`
}

// TemplateData represents data to be used with a template
type TemplateData map[string]interface{}

// RenderedPrompt represents a rendered template with metadata
type RenderedPrompt struct {
	ID           string
	Content      string
	SystemPrompt string
	MaxTokens    int32
	Temperature  float32
	Parameters   TemplateData
	Metadata     map[string]interface{}
}

// TemplateError represents template-related errors
type TemplateError struct {
	TemplateID string
	Operation  string
	Cause      error
}

func (e TemplateError) Error() string {
	return fmt.Sprintf("template error [%s] during %s: %v", e.TemplateID, e.Operation, e.Cause)
}

// ValidationError represents parameter validation errors
type ValidationError struct {
	Parameter string
	Message   string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for parameter '%s': %s", e.Parameter, e.Message)
}

// Render executes the template with the provided data
func (t *Template) Render(data TemplateData) (*RenderedPrompt, error) {
	// Apply defaults first
	enrichedData := t.applyDefaults(data)

	// Validate required parameters after defaults are applied
	if err := t.ValidateParameters(enrichedData); err != nil {
		return nil, err
	}

	// Parse and execute template
	tmpl, err := template.New(t.ID).Parse(t.Template)
	if err != nil {
		return nil, TemplateError{
			TemplateID: t.ID,
			Operation:  "parse",
			Cause:      err,
		}
	}

	var content strings.Builder
	if err := tmpl.Execute(&content, enrichedData); err != nil {
		return nil, TemplateError{
			TemplateID: t.ID,
			Operation:  "execute",
			Cause:      err,
		}
	}

	return &RenderedPrompt{
		ID:           t.ID,
		Content:      content.String(),
		SystemPrompt: t.SystemPrompt,
		MaxTokens:    t.MaxTokens,
		Temperature:  t.Temperature,
		Parameters:   enrichedData,
		Metadata: map[string]interface{}{
			"template_version": t.Version,
			"category":         t.Category,
			"rendered_at":      time.Now(),
		},
	}, nil
}

// ValidateParameters checks if all required parameters are provided
func (t *Template) ValidateParameters(data TemplateData) error {
	for _, param := range t.Parameters {
		if param.Required {
			if _, exists := data[param.Name]; !exists {
				return ValidationError{
					Parameter: param.Name,
					Message:   "required parameter missing",
				}
			}
		}
	}
	return nil
}

// applyDefaults merges provided data with defaults
func (t *Template) applyDefaults(data TemplateData) TemplateData {
	enriched := make(TemplateData)

	// Start with defaults
	for key, value := range t.Defaults {
		enriched[key] = value
	}

	// Override with provided data
	for key, value := range data {
		enriched[key] = value
	}

	return enriched
}
