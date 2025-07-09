package infra

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"text/template"
)

// Template filesystem embeddings
//
//go:embed templates/workflows/*.yaml
var workflowTemplates embed.FS

//go:embed templates/manifests/*.yaml
var manifestTemplates embed.FS

//go:embed templates/stages/*.yaml
var stageTemplates embed.FS

//go:embed templates/pipelines/*.yaml
var pipelineTemplates embed.FS

//go:embed templates/components/*.yaml
var componentTemplates embed.FS

//go:embed templates/*.tmpl templates/Dockerfile
var dockerfileTemplates embed.FS

// TemplateManager manages template operations for infrastructure
type TemplateManager struct {
	logger *slog.Logger
	cache  map[string]*template.Template
}

// NewTemplateManager creates a new template manager
func NewTemplateManager(logger *slog.Logger) *TemplateManager {
	return &TemplateManager{
		logger: logger,
		cache:  make(map[string]*template.Template),
	}
}

// TemplateType represents different template types
type TemplateType string

const (
	TemplateTypeWorkflow   TemplateType = "workflow"
	TemplateTypeManifest   TemplateType = "manifest"
	TemplateTypeStage      TemplateType = "stage"
	TemplateTypePipeline   TemplateType = "pipeline"
	TemplateTypeComponent  TemplateType = "component"
	TemplateTypeDockerfile TemplateType = "dockerfile"
)

// TemplateRenderParams represents template rendering parameters
type TemplateRenderParams struct {
	Name       string
	Type       TemplateType
	Variables  map[string]interface{}
	OutputPath string
}

// TemplateRenderResult represents template rendering result
type TemplateRenderResult struct {
	Name     string
	Type     TemplateType
	Content  string
	FilePath string
	Success  bool
	Error    string
}

// RenderTemplate renders a template with variables
func (tm *TemplateManager) RenderTemplate(params TemplateRenderParams) (*TemplateRenderResult, error) {
	tm.logger.Info("Rendering template",
		"name", params.Name,
		"type", params.Type)

	// Get template content
	templateContent, err := tm.getTemplateContent(params.Type, params.Name)
	if err != nil {
		return &TemplateRenderResult{
			Name:    params.Name,
			Type:    params.Type,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Parse and execute template
	tmpl, err := template.New(params.Name).Parse(templateContent)
	if err != nil {
		return &TemplateRenderResult{
			Name:    params.Name,
			Type:    params.Type,
			Success: false,
			Error:   fmt.Sprintf("template parsing failed: %v", err),
		}, nil
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, params.Variables)
	if err != nil {
		return &TemplateRenderResult{
			Name:    params.Name,
			Type:    params.Type,
			Success: false,
			Error:   fmt.Sprintf("template execution failed: %v", err),
		}, nil
	}

	result := &TemplateRenderResult{
		Name:     params.Name,
		Type:     params.Type,
		Content:  buf.String(),
		FilePath: params.OutputPath,
		Success:  true,
	}

	tm.logger.Info("Template rendered successfully",
		"name", result.Name,
		"type", result.Type,
		"size", len(result.Content))

	return result, nil
}

// getTemplateContent gets template content from embedded filesystem
func (tm *TemplateManager) getTemplateContent(templateType TemplateType, name string) (string, error) {
	var fs embed.FS
	var basePath string

	switch templateType {
	case TemplateTypeWorkflow:
		fs = workflowTemplates
		basePath = "templates/workflows"
	case TemplateTypeManifest:
		fs = manifestTemplates
		basePath = "templates/manifests"
	case TemplateTypeStage:
		fs = stageTemplates
		basePath = "templates/stages"
	case TemplateTypePipeline:
		fs = pipelineTemplates
		basePath = "templates/pipelines"
	case TemplateTypeComponent:
		fs = componentTemplates
		basePath = "templates/components"
	case TemplateTypeDockerfile:
		fs = dockerfileTemplates
		basePath = "templates/dockerfiles"
	default:
		return "", fmt.Errorf("unsupported template type: %s", templateType)
	}

	// Construct file path
	filePath := filepath.Join(basePath, name)
	if !strings.HasSuffix(filePath, ".yaml") && !strings.HasSuffix(filePath, ".yml") && templateType != TemplateTypeDockerfile {
		filePath += ".yaml"
	}

	// Read template content
	content, err := fs.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", filePath, err)
	}

	return string(content), nil
}

// ListTemplates lists available templates by type
func (tm *TemplateManager) ListTemplates(templateType TemplateType) ([]string, error) {
	var fs embed.FS
	var basePath string

	switch templateType {
	case TemplateTypeWorkflow:
		fs = workflowTemplates
		basePath = "templates/workflows"
	case TemplateTypeManifest:
		fs = manifestTemplates
		basePath = "templates/manifests"
	case TemplateTypeStage:
		fs = stageTemplates
		basePath = "templates/stages"
	case TemplateTypePipeline:
		fs = pipelineTemplates
		basePath = "templates/pipelines"
	case TemplateTypeComponent:
		fs = componentTemplates
		basePath = "templates/components"
	case TemplateTypeDockerfile:
		fs = dockerfileTemplates
		basePath = "templates/dockerfiles"
	default:
		return nil, fmt.Errorf("unsupported template type: %s", templateType)
	}

	// Read directory entries
	entries, err := fs.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template directory %s: %w", basePath, err)
	}

	var templates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			// Remove file extension for non-dockerfile templates
			if templateType != TemplateTypeDockerfile {
				name = strings.TrimSuffix(name, ".yaml")
				name = strings.TrimSuffix(name, ".yml")
			}
			templates = append(templates, name)
		}
	}

	return templates, nil
}

// GetTemplateMetadata gets metadata about a template
func (tm *TemplateManager) GetTemplateMetadata(templateType TemplateType, name string) (*TemplateMetadata, error) {
	content, err := tm.getTemplateContent(templateType, name)
	if err != nil {
		return nil, err
	}

	metadata := &TemplateMetadata{
		Name:        name,
		Type:        templateType,
		Size:        len(content),
		Variables:   tm.extractTemplateVariables(content),
		Description: tm.extractTemplateDescription(content),
	}

	return metadata, nil
}

// TemplateMetadata represents template metadata
type TemplateMetadata struct {
	Name        string            `json:"name"`
	Type        TemplateType      `json:"type"`
	Size        int               `json:"size"`
	Variables   []string          `json:"variables"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Version     string            `json:"version"`
	Metadata    map[string]string `json:"metadata"`
}

// extractTemplateVariables extracts template variables from content
func (tm *TemplateManager) extractTemplateVariables(content string) []string {
	var variables []string

	// Simple regex-based extraction for Go template variables
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
			// Extract variable names between {{ and }}
			start := strings.Index(line, "{{")
			end := strings.Index(line, "}}")
			if start != -1 && end != -1 && end > start {
				varExpr := strings.TrimSpace(line[start+2 : end])
				if strings.HasPrefix(varExpr, ".") {
					varName := strings.TrimPrefix(varExpr, ".")
					varName = strings.Split(varName, " ")[0] // Take first word
					if varName != "" && !contains(variables, varName) {
						variables = append(variables, varName)
					}
				}
			}
		}
	}

	return variables
}

// extractTemplateDescription extracts description from template comments
func (tm *TemplateManager) extractTemplateDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# Description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# Description:"))
		}
		if strings.HasPrefix(line, "# ") && strings.Contains(line, "template") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// DockerfileGenerator generates Dockerfiles using language-specific templates
type DockerfileGenerator struct {
	templateManager *TemplateManager
	logger          *slog.Logger
}

// NewDockerfileGenerator creates a new Dockerfile generator
func NewDockerfileGenerator(logger *slog.Logger) *DockerfileGenerator {
	return &DockerfileGenerator{
		templateManager: NewTemplateManager(logger),
		logger:          logger,
	}
}

// GenerateDockerfileParams represents Dockerfile generation parameters
type GenerateDockerfileParams struct {
	Language   string
	Framework  string
	BaseImage  string
	WorkingDir string
	Ports      []int
	Commands   []string
	Variables  map[string]interface{}
	OutputPath string
}

// GenerateDockerfileResult represents Dockerfile generation result
type GenerateDockerfileResult struct {
	Language  string
	Framework string
	Content   string
	FilePath  string
	Success   bool
	Error     string
	Template  string
}

// GenerateDockerfile generates a Dockerfile based on language and framework
func (dg *DockerfileGenerator) GenerateDockerfile(params GenerateDockerfileParams) (*GenerateDockerfileResult, error) {
	dg.logger.Info("Generating Dockerfile",
		"language", params.Language,
		"framework", params.Framework)

	// Determine template name based on language and framework
	templateName := dg.getDockerfileTemplateName(params.Language, params.Framework)

	// Prepare template variables
	templateVars := map[string]interface{}{
		"BaseImage":  params.BaseImage,
		"WorkingDir": params.WorkingDir,
		"Ports":      params.Ports,
		"Commands":   params.Commands,
		"Language":   params.Language,
		"Framework":  params.Framework,
	}

	// Merge with custom variables
	for k, v := range params.Variables {
		templateVars[k] = v
	}

	// Render template
	renderResult, err := dg.templateManager.RenderTemplate(TemplateRenderParams{
		Name:       templateName,
		Type:       TemplateTypeDockerfile,
		Variables:  templateVars,
		OutputPath: params.OutputPath,
	})
	if err != nil {
		return &GenerateDockerfileResult{
			Language:  params.Language,
			Framework: params.Framework,
			Success:   false,
			Error:     err.Error(),
		}, nil
	}

	if !renderResult.Success {
		return &GenerateDockerfileResult{
			Language:  params.Language,
			Framework: params.Framework,
			Success:   false,
			Error:     renderResult.Error,
		}, nil
	}

	result := &GenerateDockerfileResult{
		Language:  params.Language,
		Framework: params.Framework,
		Content:   renderResult.Content,
		FilePath:  renderResult.FilePath,
		Success:   true,
		Template:  templateName,
	}

	dg.logger.Info("Dockerfile generated successfully",
		"language", result.Language,
		"framework", result.Framework,
		"template", result.Template)

	return result, nil
}

// getDockerfileTemplateName determines the template name based on language and framework
func (dg *DockerfileGenerator) getDockerfileTemplateName(language, framework string) string {
	language = strings.ToLower(language)
	framework = strings.ToLower(framework)

	// Language-specific templates
	switch language {
	case "go", "golang":
		return "dockerfile-go"
	case "java":
		if framework == "tomcat" {
			return "dockerfile-java-tomcat"
		}
		if framework == "jboss" {
			return "dockerfile-java-jboss"
		}
		return "dockerfile-java"
	case "javascript", "js", "node", "nodejs":
		return "dockerfile-javascript"
	case "python", "py":
		return "dockerfile-python"
	case "csharp", "c#", "dotnet":
		return "dockerfile-csharp"
	case "ruby", "rb":
		return "dockerfile-ruby"
	case "php":
		return "dockerfile-php"
	case "rust":
		return "dockerfile-rust"
	case "swift":
		return "dockerfile-swift"
	case "clojure":
		return "dockerfile-clojure"
	case "erlang":
		return "dockerfile-erlang"
	case "maven":
		return "dockerfile-maven"
	case "gradle":
		return "dockerfile-gradle"
	case "gradlew":
		return "dockerfile-gradlew"
	default:
		return "dockerfile-go" // Default to Go template
	}
}

// ManifestGenerator generates Kubernetes manifests using templates
type ManifestGenerator struct {
	templateManager *TemplateManager
	logger          *slog.Logger
}

// NewManifestGenerator creates a new manifest generator
func NewManifestGenerator(logger *slog.Logger) *ManifestGenerator {
	return &ManifestGenerator{
		templateManager: NewTemplateManager(logger),
		logger:          logger,
	}
}

// GenerateManifestParams represents manifest generation parameters
type GenerateManifestParams struct {
	Type       string // deployment, service, ingress, configmap, secret, etc.
	Name       string
	Namespace  string
	Variables  map[string]interface{}
	OutputPath string
}

// GenerateManifestResult represents manifest generation result
type GenerateManifestResult struct {
	Type     string
	Name     string
	Content  string
	FilePath string
	Success  bool
	Error    string
}

// GenerateManifest generates a Kubernetes manifest
func (mg *ManifestGenerator) GenerateManifest(params GenerateManifestParams) (*GenerateManifestResult, error) {
	mg.logger.Info("Generating Kubernetes manifest",
		"type", params.Type,
		"name", params.Name,
		"namespace", params.Namespace)

	// Prepare template variables
	templateVars := map[string]interface{}{
		"Name":      params.Name,
		"Namespace": params.Namespace,
		"Type":      params.Type,
	}

	// Merge with custom variables
	for k, v := range params.Variables {
		templateVars[k] = v
	}

	// Render template
	renderResult, err := mg.templateManager.RenderTemplate(TemplateRenderParams{
		Name:       params.Type,
		Type:       TemplateTypeManifest,
		Variables:  templateVars,
		OutputPath: params.OutputPath,
	})
	if err != nil {
		return &GenerateManifestResult{
			Type:    params.Type,
			Name:    params.Name,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if !renderResult.Success {
		return &GenerateManifestResult{
			Type:    params.Type,
			Name:    params.Name,
			Success: false,
			Error:   renderResult.Error,
		}, nil
	}

	result := &GenerateManifestResult{
		Type:     params.Type,
		Name:     params.Name,
		Content:  renderResult.Content,
		FilePath: renderResult.FilePath,
		Success:  true,
	}

	mg.logger.Info("Kubernetes manifest generated successfully",
		"type", result.Type,
		"name", result.Name)

	return result, nil
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
