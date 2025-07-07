package deploy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/templates"
)

// Writer handles writing manifest files
type Writer struct {
	logger *slog.Logger
}

// NewWriter creates a new manifest writer
func NewWriter(logger *slog.Logger) *Writer {
	return &Writer{
		logger: logger.With("component", "manifest_writer"),
	}
}

// EnsureDirectory creates the manifest directory if it doesn't exist
func (w *Writer) EnsureDirectory(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return errors.NewError().Messagef("failed to create directory %s: %v", path, err).Build()
	}
	w.logger.Debug("Ensured manifest directory exists", "path", path)
	return nil
}

// WriteFile writes content to a file
func (w *Writer) WriteFile(filePath string, content []byte) error {
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return errors.NewError().Messagef("error").Build()
	}
	w.logger.Debug("Wrote manifest file", "file", filePath)
	return nil
}

// WriteDeploymentTemplate writes a deployment manifest template
func (w *Writer) WriteDeploymentTemplate(manifestPath string, opts GenerationOptions) error {
	deploymentPath := filepath.Join(manifestPath, "deployment.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "deployment.yaml"))
	if err != nil {
		return errors.NewError().Messagef("failed to read deployment template: %v", err).WithLocation(

		// Apply template substitutions
		).Build()
	}

	processed, err := w.processTemplate("deployment", string(templateContent), opts)
	if err != nil {
		return errors.NewError().Messagef("failed to process deployment template: %v", err).Build()
	}

	if err := w.WriteFile(deploymentPath, []byte(processed)); err != nil {
		return errors.NewError().Messagef("failed to write deployment template: %v", err).Build()
	}

	w.logger.Debug("Wrote deployment template",
		"path", deploymentPath,
		"image", opts.ImageRef.String())

	return nil
}

// WriteServiceTemplate writes a service manifest template
func (w *Writer) WriteServiceTemplate(manifestPath string, opts GenerationOptions) error {
	servicePath := filepath.Join(manifestPath, "service.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "service.yaml"))
	if err != nil {
		return errors.NewError().Messagef("failed to read service template: %v", err).WithLocation(

		// Apply template substitutions
		).Build()
	}

	processed, err := w.processTemplate("service", string(templateContent), opts)
	if err != nil {
		return errors.NewError().Messagef("failed to process service template: %v", err).Build()
	}

	if err := w.WriteFile(servicePath, []byte(processed)); err != nil {
		return errors.NewError().Messagef("failed to write service template: %v", err).Build()
	}

	w.logger.Debug("Wrote service template",
		"path", servicePath,
		"service_type", opts.ServiceType)

	return nil
}

// WriteConfigMapTemplate writes a configmap manifest template
func (w *Writer) WriteConfigMapTemplate(manifestPath string, opts GenerationOptions) error {
	configMapPath := filepath.Join(manifestPath, "configmap.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "configmap.yaml"))
	if err != nil {
		return errors.NewError().Messagef("failed to read configmap template: %v", err).WithLocation(

		// Apply template substitutions
		).Build()
	}

	processed, err := w.processTemplate("configmap", string(templateContent), opts)
	if err != nil {
		return errors.NewError().Messagef("failed to process configmap template: %v", err).Build()
	}

	if err := w.WriteFile(configMapPath, []byte(processed)); err != nil {
		return errors.NewError().Messagef("failed to write configmap template: %v", err).Build()
	}

	w.logger.Debug("Wrote configmap template",
		"path", configMapPath,
		"env_vars", len(opts.Environment))

	return nil
}

// WriteIngressTemplate writes an ingress manifest template
func (w *Writer) WriteIngressTemplate(manifestPath string, opts GenerationOptions) error {
	ingressPath := filepath.Join(manifestPath, "ingress.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "ingress.yaml"))
	if err != nil {
		return errors.NewError().Messagef("failed to read ingress template: %v", err).WithLocation(

		// Apply template substitutions
		).Build()
	}

	processed, err := w.processTemplate("ingress", string(templateContent), opts)
	if err != nil {
		return errors.NewError().Messagef("failed to process ingress template: %v", err).Build()
	}

	if err := w.WriteFile(ingressPath, []byte(processed)); err != nil {
		return errors.NewError().Messagef("failed to write ingress template: %v", err).Build()
	}

	w.logger.Debug("Wrote ingress template",
		"path", ingressPath,
		"hosts", len(opts.IngressHosts))

	return nil
}

// WriteSecretTemplate writes a secret manifest template
func (w *Writer) WriteSecretTemplate(manifestPath string, opts GenerationOptions) error {
	secretPath := filepath.Join(manifestPath, "secret.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "secret.yaml"))
	if err != nil {
		return errors.NewError().Messagef("failed to read secret template: %v", err).WithLocation(

		// Apply template substitutions
		).Build()
	}

	processed, err := w.processTemplate("secret", string(templateContent), opts)
	if err != nil {
		return errors.NewError().Messagef("failed to process secret template: %v", err).Build()
	}

	if err := w.WriteFile(secretPath, []byte(processed)); err != nil {
		return errors.NewError().Messagef("failed to write secret template: %v", err).Build()
	}

	w.logger.Debug("Wrote secret template",
		"path", secretPath,
		"secrets", len(opts.Secrets))

	return nil
}

// WriteManifestFromTemplate writes a manifest using a specific template
func (w *Writer) WriteManifestFromTemplate(filePath, templatePath string, data interface{}) error {
	templateContent, err := templates.Templates.ReadFile(templatePath)
	if err != nil {
		return errors.NewError().Messagef("failed to read template %s: %v", templatePath, err).WithLocation(

		// If data is GenerationOptions, use processTemplate
		).Build()
	}

	if opts, ok := data.(GenerationOptions); ok {
		processed, err := w.processTemplate(filepath.Base(templatePath), string(templateContent), opts)
		if err != nil {
			return errors.NewError().Messagef("failed to process template: %v", err).Build()
		}
		return w.WriteFile(filePath, []byte(processed))
	}

	// Otherwise, use Go templates directly
	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(templateContent))
	if err != nil {
		return errors.NewError().Messagef("failed to parse template: %v", err).Build()
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return errors.NewError().Messagef("failed to execute template: %v", err).Build()
	}

	return w.WriteFile(filePath, buf.Bytes())
}

// processTemplate processes a template with the given options
func (w *Writer) processTemplate(name string, templateContent string, opts GenerationOptions) (string, error) {
	// Create template functions
	funcMap := template.FuncMap{
		"default": func(def interface{}, val interface{}) interface{} {
			if val == nil || val == "" || val == 0 {
				return def
			}
			return val
		},
		"quote": func(s string) string {
			return fmt.Sprintf("%q", s)
		},
	}

	// Parse the template
	tmpl, err := template.New(name).Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return "", errors.NewError().Messagef("failed to parse template: %v", err).WithLocation(

		// Prepare template data
		).Build()
	}

	data := map[string]interface{}{
		"Name":            "app", // Default app name
		"Namespace":       opts.Namespace,
		"Image":           opts.ImageRef.String(),
		"Replicas":        opts.Replicas,
		"ServiceType":     opts.ServiceType,
		"Environment":     opts.Environment,
		"Resources":       opts.Resources,
		"ServicePorts":    opts.ServicePorts,
		"IngressHosts":    opts.IngressHosts,
		"IngressClass":    opts.IngressClass,
		"IngressTLS":      opts.IngressTLS,
		"ConfigMapData":   opts.ConfigMapData,
		"LoadBalancerIP":  opts.LoadBalancerIP,
		"SessionAffinity": opts.SessionAffinity,
		"WorkflowLabels":  opts.WorkflowLabels,
	}

	// Set defaults
	if data["Namespace"] == "" {
		data["Namespace"] = "default"
	}
	if data["Replicas"] == 0 {
		data["Replicas"] = 1
	}
	if data["ServiceType"] == "" {
		data["ServiceType"] = "ClusterIP"
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.NewError().Messagef("failed to execute template: %v", err).WithLocation().Build()
	}

	return buf.String(), nil
}
