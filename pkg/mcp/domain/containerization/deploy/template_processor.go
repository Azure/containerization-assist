package deploy

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// TemplateProcessor handles template selection and processing
type TemplateProcessor struct {
	templateCache map[string]TemplateInfo
	logger        *slog.Logger
}

// TemplateInfo is now defined in manifests_common.go with Content field

// NewTemplateProcessor creates a new template processor
func NewTemplateProcessor(logger *slog.Logger) *TemplateProcessor {
	tp := &TemplateProcessor{
		templateCache: make(map[string]TemplateInfo),
		logger:        logger.With("component", "template_processor"),
	}
	tp.initializeTemplates()
	return tp
}

// initializeTemplates sets up the available templates
func (tp *TemplateProcessor) initializeTemplates() {
	templates := []TemplateInfo{
		{
			Name:        "microservice-basic",
			Path:        "/templates/microservice-basic.yaml",
			Content:     "# Basic microservice deployment template",
			Description: "Basic microservice deployment",
			Languages:   []string{"go", "java", "python", "node", "dotnet"},
			Frameworks:  []string{},
			Features:    []string{"service", "deployment", "basic"},
			Priority:    1,
			Metadata:    map[string]interface{}{},
		},
		{
			Name:        "microservice-advanced",
			Path:        "/templates/microservice-advanced.yaml",
			Content:     "# Advanced microservice with monitoring and scaling template",
			Description: "Advanced microservice with monitoring and scaling",
			Languages:   []string{"go", "java", "python", "node", "dotnet"},
			Frameworks:  []string{},
			Features:    []string{"service", "deployment", "hpa", "monitoring", "health-checks"},
			Priority:    2,
			Metadata:    map[string]interface{}{},
		},
		{
			Name:        "web-application",
			Path:        "/templates/web-application.yaml",
			Content:     "# Web application with ingress template",
			Description: "Web application with ingress",
			Languages:   []string{"python", "node", "ruby", "php"},
			Frameworks:  []string{"django", "flask", "express", "rails", "laravel"},
			Features:    []string{"service", "deployment", "ingress", "web"},
			Priority:    2,
			Metadata:    map[string]interface{}{},
		},
		{
			Name:        "stateful-application",
			Path:        "/templates/stateful-application.yaml",
			Content:     "# Application with persistent storage template",
			Description: "Application with persistent storage",
			Languages:   []string{"*"},
			Frameworks:  []string{},
			Features:    []string{"service", "deployment", "pvc", "statefulset"},
			Priority:    3,
			Metadata:    map[string]interface{}{},
		},
		{
			Name:        "job-batch",
			Path:        "/templates/job-batch.yaml",
			Content:     "# Batch job processing template",
			Description: "Batch job processing",
			Languages:   []string{"*"},
			Frameworks:  []string{},
			Features:    []string{"job", "cronjob", "batch"},
			Priority:    1,
			Metadata:    map[string]interface{}{},
		},
		{
			Name:        "api-gateway",
			Path:        "/templates/api-gateway.yaml",
			Content:     "# API Gateway pattern template",
			Description: "API Gateway pattern",
			Languages:   []string{"go", "java", "node"},
			Frameworks:  []string{"kong", "zuul", "express-gateway"},
			Features:    []string{"service", "deployment", "ingress", "api", "gateway"},
			Priority:    3,
			Metadata:    map[string]interface{}{},
		},
	}

	for _, template := range templates {
		tp.templateCache[template.Name] = template
	}
}

// SelectTemplate selects the best template based on session context
func (tp *TemplateProcessor) SelectTemplate(session *session.SessionState, args GenerateManifestsRequest) (string, string, error) {
	tp.logger.Info("Selecting template for manifest generation",
		"session_id", args.SessionID)

	// Get repository context from session
	var language, framework string
	var features []string

	if session != nil && session.ScanSummary != nil {
		language = strings.ToLower(session.ScanSummary.Language)
		framework = strings.ToLower(session.ScanSummary.Framework)

		// Extract features from repository info
		if len(session.ScanSummary.DatabaseFiles) > 0 {
			features = append(features, "database", "stateful")
		}
		// Simple heuristics for web app and API detection
		if framework != "" && (strings.Contains(framework, "django") || strings.Contains(framework, "flask") ||
			strings.Contains(framework, "express") || strings.Contains(framework, "rails")) {
			features = append(features, "web", "ingress")
		}
		// Check for API patterns in entry points
		for _, entryPoint := range session.ScanSummary.EntryPointsFound {
			if strings.Contains(strings.ToLower(entryPoint), "api") ||
				strings.Contains(strings.ToLower(entryPoint), "server") {
				features = append(features, "api")
				break
			}
		}
	}

	// Score templates
	bestTemplate := "microservice-basic"
	bestScore := 0
	var selectionInfo []string

	for name, template := range tp.templateCache {
		score := tp.scoreTemplate(template, language, framework, features, args)

		tp.logger.Debug("Template scored",
			"template", name,
			"score", score)

		if score > bestScore {
			bestScore = score
			bestTemplate = name
		}
	}

	// Build selection info
	if language != "" {
		selectionInfo = append(selectionInfo, fmt.Sprintf("language=%s", language))
	}
	if framework != "" {
		selectionInfo = append(selectionInfo, fmt.Sprintf("framework=%s", framework))
	}
	if len(features) > 0 {
		selectionInfo = append(selectionInfo, fmt.Sprintf("features=%s", strings.Join(features, ",")))
	}
	selectionInfo = append(selectionInfo, fmt.Sprintf("selected=%s", bestTemplate))
	selectionInfo = append(selectionInfo, fmt.Sprintf("score=%d", bestScore))

	tp.logger.Info("Template selected",
		"template", bestTemplate,
		"score", bestScore,
		"info", strings.Join(selectionInfo, ", "))

	return bestTemplate, strings.Join(selectionInfo, ", "), nil
}

// scoreTemplate scores a template based on matching criteria
func (tp *TemplateProcessor) scoreTemplate(template TemplateInfo, language, framework string, features []string, args GenerateManifestsRequest) int {
	score := 0

	// Language match (high weight)
	if tp.matchesLanguage(template, language) {
		score += 10
	}

	// Framework match (very high weight)
	if framework != "" && tp.matchesFramework(template, framework) {
		score += 20
	}

	// Feature matches (medium weight)
	for _, feature := range features {
		if tp.hasFeature(template, feature) {
			score += 5
		}
	}

	// Specific requirements
	if args.IncludeIngress && tp.hasFeature(template, "ingress") {
		score += 5
	}

	// Priority bonus
	score += template.Priority

	return score
}

// matchesLanguage checks if template supports the language
func (tp *TemplateProcessor) matchesLanguage(template TemplateInfo, language string) bool {
	if language == "" {
		return false
	}

	for _, lang := range template.Languages {
		if lang == "*" || lang == language {
			return true
		}
	}
	return false
}

// matchesFramework checks if template supports the framework
func (tp *TemplateProcessor) matchesFramework(template TemplateInfo, framework string) bool {
	if framework == "" {
		return false
	}

	for _, fw := range template.Frameworks {
		if strings.Contains(framework, fw) || strings.Contains(fw, framework) {
			return true
		}
	}
	return false
}

// hasFeature checks if template has a specific feature
func (tp *TemplateProcessor) hasFeature(template TemplateInfo, feature string) bool {
	for _, f := range template.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// ProcessTemplate processes a template with the given data
func (tp *TemplateProcessor) ProcessTemplate(templateName string, data interface{}) (string, error) {
	tp.logger.Info("Processing template",
		"template", templateName)

	// Check if template is cached
	templateInfo, exists := tp.GetTemplateInfo(templateName)
	if !exists {
		return "", errors.NewError().Messagef("template %s not found", templateName).WithLocation(

		// Parse template content
		).Build()
	}

	tmpl, err := template.New(templateName).Parse(templateInfo.Content)
	if err != nil {
		tp.logger.Error("Failed to parse template", "error", err, "template", templateName)
		return "", errors.NewError().Message("parsing template " + templateName).Cause(err).WithLocation(

		// Execute template with provided data
		).Build()
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		tp.logger.Error("Failed to execute template", "error", err, "template", templateName)
		return "", errors.NewError().Message("executing template " + templateName).Cause(err).WithLocation().Build()
	}

	result := buf.String()
	tp.logger.Debug("Template processed successfully",
		"template", templateName,
		"output_length", len(result))

	return result, nil
}

// GetTemplateInfo returns information about a specific template
func (tp *TemplateProcessor) GetTemplateInfo(templateName string) (TemplateInfo, bool) {
	info, exists := tp.templateCache[templateName]
	return info, exists
}

// ListTemplates returns all available templates
func (tp *TemplateProcessor) ListTemplates() []TemplateInfo {
	var templates []TemplateInfo
	for _, template := range tp.templateCache {
		templates = append(templates, template)
	}
	return templates
}

// ValidateTemplate checks if a template name is valid
func (tp *TemplateProcessor) ValidateTemplate(templateName string) error {
	if _, exists := tp.templateCache[templateName]; !exists {
		return errors.NewError().Messagef("template '%s' not found", templateName).Build()
	}
	return nil
}
