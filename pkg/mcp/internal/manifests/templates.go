package manifests

import (
	"fmt"
	"path/filepath"

	"github.com/Azure/container-copilot/templates"
	"github.com/rs/zerolog"
)

// TemplateManager handles template operations for manifest generation
type TemplateManager struct {
	logger zerolog.Logger
}

// NewTemplateManager creates a new template manager
func NewTemplateManager(logger zerolog.Logger) *TemplateManager {
	return &TemplateManager{
		logger: logger.With().Str("component", "template_manager").Logger(),
	}
}

// GetTemplate retrieves a template by name
func (tm *TemplateManager) GetTemplate(templateName string) ([]byte, error) {
	templatePath := filepath.Join("k8s", templateName+".yaml")

	content, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", templateName+".yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", templateName, err)
	}

	tm.logger.Debug().
		Str("template", templateName).
		Str("path", templatePath).
		Msg("Retrieved template")

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
		return nil, fmt.Errorf("no template available for resource type: %s", resourceType)
	}

	return tm.GetTemplate(templateName)
}

// ValidateTemplate validates that a template exists and is readable
func (tm *TemplateManager) ValidateTemplate(templateName string) error {
	_, err := tm.GetTemplate(templateName)
	return err
}
