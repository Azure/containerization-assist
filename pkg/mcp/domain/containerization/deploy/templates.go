package deploy

import (
	"path/filepath"

	"fmt"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/templates"
)

// TemplateManager handles template operations for manifest generation
type TemplateManager struct {
	logger *slog.Logger
}

// NewTemplateManager creates a new template manager
func NewTemplateManager(logger *slog.Logger) *TemplateManager {
	return &TemplateManager{
		logger: logger.With("component", "template_manager"),
	}
}

// GetTemplate retrieves a template by name
func (tm *TemplateManager) GetTemplate(templateName string) ([]byte, error) {
	templatePath := filepath.Join("k8s", templateName+".yaml")

	content, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", templateName+".yaml"))
	if err != nil {
		return nil, errors.NewError().Message(fmt.Sprintf("failed to read template %s", templateName)).Cause(err).Build()
	}

	tm.logger.Debug("Retrieved template",
		"template", templateName,
		"path", templatePath)

	return content, nil
}

// ListAvailableTemplates returns a list of available templates
func (tm *TemplateManager) ListAvailableTemplates() ([]string, error) {
	templates := []string{
		"deployment",
		"service",
		"ingress",
		"configmap",
		"secret",
		"namespace",
		"serviceaccount",
		"pvc",
		"hpa",
	}

	return templates, nil
}

// GetTemplateForResource returns the appropriate template for a given resource type
func (tm *TemplateManager) GetTemplateForResource(resourceType string) ([]byte, error) {
	// Map resource types to template names
	templateMap := map[string]string{
		"Deployment":              "deployment",
		"Service":                 "service",
		"Ingress":                 "ingress",
		"ConfigMap":               "configmap",
		"Secret":                  "secret",
		"Namespace":               "namespace",
		"ServiceAccount":          "serviceaccount",
		"PersistentVolumeClaim":   "pvc",
		"HorizontalPodAutoscaler": "hpa",
	}

	templateName, exists := templateMap[resourceType]
	if !exists {
		return nil, errors.NewError().Messagef("no template available for resource type: %s", resourceType).Build()
	}

	return tm.GetTemplate(templateName)
}

// ValidateTemplate validates that a template exists and is readable
func (tm *TemplateManager) ValidateTemplate(templateName string) error {
	_, err := tm.GetTemplate(templateName)
	return err
}
