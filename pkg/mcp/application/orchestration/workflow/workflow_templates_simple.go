package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/infra/templates"
	"github.com/rs/zerolog"
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
	logger    zerolog.Logger
	templates map[string]*SimpleWorkflowTemplate
}

// NewSimpleTemplateManager creates a new template manager
func NewSimpleTemplateManager(logger zerolog.Logger) *SimpleTemplateManager {
	return &SimpleTemplateManager{
		logger:    logger.With().Str("component", "template_manager").Logger(),
		templates: make(map[string]*SimpleWorkflowTemplate),
	}
}

// LoadTemplate loads a template from the embedded filesystem
func (tm *SimpleTemplateManager) LoadTemplate(name string) (*WorkflowTemplate, error) {
	// Check cache first
	if template, exists := tm.templates[name]; exists {
		return tm.convertToWorkflowTemplate(template), nil
	}

	// Load from filesystem
	path := fmt.Sprintf("workflows/%s.yaml", name)
	content, err := templates.LoadTemplate(path)
	if err != nil {
		return nil, errors.NewError().Message("failed to load template " + name).Cause(err).WithLocation(

		// Parse YAML
		).Build()
	}

	var template SimpleWorkflowTemplate
	if err := yaml.Unmarshal([]byte(content), &template); err != nil {
		return nil, errors.NewError().Message("failed to parse template " + name).Cause(err).WithLocation(

		// Set ID if not specified
		).Build()
	}

	if template.ID == "" {
		template.ID = name
	}

	// Cache for future use
	tm.templates[name] = &template

	tm.logger.Debug().
		Str("template", name).
		Str("version", template.Version).
		Msg("Template loaded successfully")

	return tm.convertToWorkflowTemplate(&template), nil
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
) (*execution.WorkflowSpec, error) {
	template, err := tm.LoadTemplate(templateName)
	if err != nil {
		return nil, err
	}

	// Validate required parameters
	for _, param := range template.Parameters {
		if param.Required {
			if _, exists := parameters[param.Name]; !exists {
				return nil, errors.NewError().Messagef("required parameter %s not provided", param.Name).WithLocation().

					// Apply defaults
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
	stages := make([]execution.ExecutionStage, 0, len(template.Stages))
	for _, ts := range template.Stages {
		stage := execution.ExecutionStage{
			ID:        ts.ID,
			Name:      ts.Name,
			Type:      "tool",
			Tools:     []string{ts.ToolName},
			Variables: ts.Parameters,
			DependsOn: ts.DependsOn,
		}
		stages = append(stages, stage)
	}

	spec := &execution.WorkflowSpec{
		ID:        fmt.Sprintf("%s-%d", template.ID, time.Now().Unix()),
		Name:      template.Name,
		Version:   template.Version,
		Stages:    stages,
		Variables: parameters,
		Metadata: execution.WorkflowMetadata{
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
func (tm *SimpleTemplateManager) GetTemplate(name string) (*WorkflowTemplate, error) {
	return tm.LoadTemplate(name)
}

// ClearCache clears the template cache
func (tm *SimpleTemplateManager) ClearCache() {
	tm.templates = make(map[string]*SimpleWorkflowTemplate)
	tm.logger.Debug().Msg("Template cache cleared")
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
