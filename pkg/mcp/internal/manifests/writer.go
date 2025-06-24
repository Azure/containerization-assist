package manifests

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Azure/container-copilot/templates"
	"github.com/rs/zerolog"
)

// Writer handles writing manifest files
type Writer struct {
	logger zerolog.Logger
}

// NewWriter creates a new manifest writer
func NewWriter(logger zerolog.Logger) *Writer {
	return &Writer{
		logger: logger.With().Str("component", "manifest_writer").Logger(),
	}
}

// EnsureDirectory creates the manifest directory if it doesn't exist
func (w *Writer) EnsureDirectory(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	w.logger.Debug().Str("path", path).Msg("Ensured manifest directory exists")
	return nil
}

// WriteFile writes content to a file
func (w *Writer) WriteFile(filePath string, content []byte) error {
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}
	w.logger.Debug().Str("file", filePath).Msg("Wrote manifest file")
	return nil
}

// WriteDeploymentTemplate writes a deployment manifest template
func (w *Writer) WriteDeploymentTemplate(manifestPath string, opts GenerationOptions) error {
	deploymentPath := filepath.Join(manifestPath, "deployment.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "deployment.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read deployment template: %w", err)
	}

	// Apply template substitutions
	processed, err := w.processTemplate("deployment", string(templateContent), opts)
	if err != nil {
		return fmt.Errorf("failed to process deployment template: %w", err)
	}

	if err := w.WriteFile(deploymentPath, []byte(processed)); err != nil {
		return fmt.Errorf("failed to write deployment template: %w", err)
	}

	w.logger.Debug().
		Str("path", deploymentPath).
		Str("image", opts.ImageRef.String()).
		Msg("Wrote deployment template")

	return nil
}

// WriteServiceTemplate writes a service manifest template
func (w *Writer) WriteServiceTemplate(manifestPath string, opts GenerationOptions) error {
	servicePath := filepath.Join(manifestPath, "service.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "service.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read service template: %w", err)
	}

	// Apply template substitutions
	processed, err := w.processTemplate("service", string(templateContent), opts)
	if err != nil {
		return fmt.Errorf("failed to process service template: %w", err)
	}

	if err := w.WriteFile(servicePath, []byte(processed)); err != nil {
		return fmt.Errorf("failed to write service template: %w", err)
	}

	w.logger.Debug().
		Str("path", servicePath).
		Str("service_type", opts.ServiceType).
		Msg("Wrote service template")

	return nil
}

// WriteConfigMapTemplate writes a configmap manifest template
func (w *Writer) WriteConfigMapTemplate(manifestPath string, opts GenerationOptions) error {
	configMapPath := filepath.Join(manifestPath, "configmap.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "configmap.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read configmap template: %w", err)
	}

	// Apply template substitutions
	processed, err := w.processTemplate("configmap", string(templateContent), opts)
	if err != nil {
		return fmt.Errorf("failed to process configmap template: %w", err)
	}

	if err := w.WriteFile(configMapPath, []byte(processed)); err != nil {
		return fmt.Errorf("failed to write configmap template: %w", err)
	}

	w.logger.Debug().
		Str("path", configMapPath).
		Int("env_vars", len(opts.Environment)).
		Msg("Wrote configmap template")

	return nil
}

// WriteIngressTemplate writes an ingress manifest template
func (w *Writer) WriteIngressTemplate(manifestPath string, opts GenerationOptions) error {
	ingressPath := filepath.Join(manifestPath, "ingress.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "ingress.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read ingress template: %w", err)
	}

	// Apply template substitutions
	processed, err := w.processTemplate("ingress", string(templateContent), opts)
	if err != nil {
		return fmt.Errorf("failed to process ingress template: %w", err)
	}

	if err := w.WriteFile(ingressPath, []byte(processed)); err != nil {
		return fmt.Errorf("failed to write ingress template: %w", err)
	}

	w.logger.Debug().
		Str("path", ingressPath).
		Int("hosts", len(opts.IngressHosts)).
		Msg("Wrote ingress template")

	return nil
}

// WriteSecretTemplate writes a secret manifest template
func (w *Writer) WriteSecretTemplate(manifestPath string, opts GenerationOptions) error {
	secretPath := filepath.Join(manifestPath, "secret.yaml")

	// Use the embedded template system
	templateContent, err := templates.Templates.ReadFile(filepath.Join("manifests", "manifest-basic", "secret.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read secret template: %w", err)
	}

	// Apply template substitutions
	processed, err := w.processTemplate("secret", string(templateContent), opts)
	if err != nil {
		return fmt.Errorf("failed to process secret template: %w", err)
	}

	if err := w.WriteFile(secretPath, []byte(processed)); err != nil {
		return fmt.Errorf("failed to write secret template: %w", err)
	}

	w.logger.Debug().
		Str("path", secretPath).
		Int("secrets", len(opts.Secrets)).
		Msg("Wrote secret template")

	return nil
}

// WriteManifestFromTemplate writes a manifest using a specific template
func (w *Writer) WriteManifestFromTemplate(filePath, templatePath string, data interface{}) error {
	templateContent, err := templates.Templates.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// If data is GenerationOptions, use processTemplate
	if opts, ok := data.(GenerationOptions); ok {
		processed, err := w.processTemplate(filepath.Base(templatePath), string(templateContent), opts)
		if err != nil {
			return fmt.Errorf("failed to process template: %w", err)
		}
		return w.WriteFile(filePath, []byte(processed))
	}

	// Otherwise, use Go templates directly
	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
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
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare template data
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
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
