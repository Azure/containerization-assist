package workflow

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/templates"
	"gopkg.in/yaml.v3"
)

// SimpleWorkflowTemplate represents a workflow template loaded from YAML
type SimpleWorkflowTemplate struct {
	ID          string                    `yaml:"id" json:"id"`
	Name        string                    `yaml:"name" json:"name"`
	Description string                    `yaml:"description" json:"description"`
	Version     string                    `yaml:"version" json:"version"`
	Parameters  []SimpleTemplateParameter `yaml:"parameters" json:"parameters"`
	Stages      []SimpleTemplateStage     `yaml:"stages" json:"stages"`
	Variables   map[string]interface{}    `yaml:"variables" json:"variables"`
	Timeout     time.Duration             `yaml:"timeout" json:"timeout"`
}

// SimpleTemplateParameter defines a template parameter
type SimpleTemplateParameter struct {
	Name         string      `yaml:"name" json:"name"`
	Type         string      `yaml:"type" json:"type"`
	Description  string      `yaml:"description" json:"description"`
	Required     bool        `yaml:"required" json:"required"`
	DefaultValue interface{} `yaml:"default,omitempty" json:"default_value,omitempty"`
}

// SimpleTemplateStage represents a stage in a workflow template
type SimpleTemplateStage struct {
	ID         string                 `yaml:"id" json:"id"`
	Name       string                 `yaml:"name" json:"name"`
	Tool       string                 `yaml:"tool" json:"tool"`
	Parameters map[string]interface{} `yaml:"parameters" json:"parameters"`
	DependsOn  []string               `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Optional   bool                   `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// SimpleTemplateManager manages workflow templates from external files
type SimpleTemplateManager struct {
	logger    *slog.Logger
	templates map[string]*SimpleWorkflowTemplate
}

// NewSimpleTemplateManager creates a new template manager
func NewSimpleTemplateManager(logger *slog.Logger) *SimpleTemplateManager {
	return &SimpleTemplateManager{
		logger:    logger.With("component", "template_manager"),
		templates: make(map[string]*SimpleWorkflowTemplate),
	}
}

// LoadTemplate loads a template from the embedded filesystem
func (tm *SimpleTemplateManager) LoadTemplate(name string) (*SimpleWorkflowTemplate, error) {
	// Check cache first
	if template, exists := tm.templates[name]; exists {
		return template, nil
	}

	// Load from filesystem
	path := fmt.Sprintf("workflows/%s.yaml", name)
	content, err := templates.LoadTemplate(path)
	if err != nil {
		return nil, errors.NewError().
			Message("failed to load template "+name).
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Cause(err).
			Context("template_name", name).
			Context("path", path).
			WithLocation().
			Build()
	}

	var template SimpleWorkflowTemplate
	if err := yaml.Unmarshal([]byte(content), &template); err != nil {
		return nil, errors.NewError().
			Message("failed to parse template "+name).
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Cause(err).
			Context("template_name", name).
			Context("path", path).
			WithLocation().
			Build()
	}

	if template.ID == "" {
		template.ID = name
	}

	// Cache for future use
	tm.templates[name] = &template

	tm.logger.Debug("Template loaded successfully",
		"template", name,
		"version", template.Version)

	return &template, nil
}

// ListTemplates lists available templates
func (tm *SimpleTemplateManager) ListTemplates() ([]string, error) {
	allTemplates, err := templates.ListTemplates()
	if err != nil {
		return nil, err
	}

	// Filter to workflows only
	var workflows []string
	for _, t := range allTemplates {
		if len(t) > 10 && t[:9] == "workflows" {
			// Extract name without path and extension
			name := t[10 : len(t)-5]
			workflows = append(workflows, name)
		}
	}

	return workflows, nil
}

// InstantiateWorkflow creates a workflow instance from a template
func (tm *SimpleTemplateManager) InstantiateWorkflow(
	ctx context.Context,
	templateName string,
	parameters map[string]interface{},
) (*WorkflowSpec, error) {
	template, err := tm.LoadTemplate(templateName)
	if err != nil {
		return nil, err
	}

	// Validate required parameters
	for _, param := range template.Parameters {
		if param.Required {
			if _, exists := parameters[param.Name]; !exists {
				return nil, errors.NewError().
					Messagef("required parameter %s not provided", param.Name).
					Code(errors.CodeMissingParameter).
					Type(errors.ErrTypeValidation).
					Context("parameter_name", param.Name).
					Context("template_name", templateName).
					Suggestion(fmt.Sprintf("Provide a value for the required parameter '%s'", param.Name)).
					WithLocation().
					Build()
			}
		}
	}

	for _, param := range template.Parameters {
		if _, exists := parameters[param.Name]; !exists && param.DefaultValue != nil {
			parameters[param.Name] = param.DefaultValue
		}
	}

	// Create workflow spec
	stages := make([]WorkflowStage, 0, len(template.Stages))
	for _, ts := range template.Stages {
		stage := WorkflowStage{
			ID:           ts.ID,
			Name:         ts.Name,
			Tools:        []string{ts.Tool},
			Parameters:   ts.Parameters,
			Dependencies: ts.DependsOn,
		}
		stages = append(stages, stage)
	}

	spec := &WorkflowSpec{
		ID:        fmt.Sprintf("%s-%d", template.ID, time.Now().Unix()),
		Name:      template.Name,
		Version:   template.Version,
		Stages:    stages,
		Variables: parameters,
		Metadata: WorkflowMetadata{
			Name:        template.Name,
			Description: template.Description,
			Version:     template.Version,
			Labels: map[string]string{
				"template": templateName,
			},
		},
	}

	return spec, nil
}

// GetTemplate retrieves a cached template
func (tm *SimpleTemplateManager) GetTemplate(name string) (*SimpleWorkflowTemplate, error) {
	return tm.LoadTemplate(name)
}

// ClearCache clears the template cache
func (tm *SimpleTemplateManager) ClearCache() {
	tm.templates = make(map[string]*SimpleWorkflowTemplate)
	tm.logger.Debug("Template cache cleared")
}

// convertToWorkflowTemplate converts a SimpleWorkflowTemplate to WorkflowTemplate
func (tm *SimpleTemplateManager) convertToWorkflowTemplate(simple *SimpleWorkflowTemplate) *WorkflowTemplate {
	// Convert parameters
	params := make([]TemplateParameter, len(simple.Parameters))
	for i, p := range simple.Parameters {
		params[i] = TemplateParameter{
			Name:         p.Name,
			Type:         p.Type,
			Description:  p.Description,
			Required:     p.Required,
			DefaultValue: p.DefaultValue,
		}
	}

	// Convert stages
	stages := make([]TemplateStage, len(simple.Stages))
	for i, s := range simple.Stages {
		stages[i] = TemplateStage{
			ID:         s.ID,
			Name:       s.Name,
			Type:       "tool",
			ToolName:   s.Tool,
			Parameters: s.Parameters,
			DependsOn:  s.DependsOn,
			Optional:   s.Optional,
		}
	}

	return &WorkflowTemplate{
		ID:          simple.ID,
		Name:        simple.Name,
		Description: simple.Description,
		Version:     simple.Version,
		Parameters:  params,
		Stages:      stages,
		Variables:   simple.Variables,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
