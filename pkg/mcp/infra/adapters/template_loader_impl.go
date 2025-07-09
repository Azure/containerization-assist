package adapters

import (
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/infra/templates"
)

// TemplateLoaderImpl implements the TemplateLoader interface
type TemplateLoaderImpl struct{}

// NewTemplateLoader creates a new template loader
func NewTemplateLoader() services.TemplateLoader {
	return &TemplateLoaderImpl{}
}

// LoadTemplate loads a template by name
func (t *TemplateLoaderImpl) LoadTemplate(name string) (string, error) {
	return templates.LoadTemplate(name)
}

// ListTemplates lists all available templates
func (t *TemplateLoaderImpl) ListTemplates() ([]string, error) {
	return templates.ListTemplates()
}
