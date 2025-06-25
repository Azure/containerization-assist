package manifests

import (
	"fmt"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// TemplateProcessor handles template selection and processing
type TemplateProcessor struct {
	templateCache map[string]TemplateInfo
	logger        zerolog.Logger
}

// TemplateInfo contains information about a template
type TemplateInfo struct {
	Name        string
	Description string
	Languages   []string
	Frameworks  []string
	Features    []string
	Priority    int
}

// NewTemplateProcessor creates a new template processor
func NewTemplateProcessor(logger zerolog.Logger) *TemplateProcessor {
	tp := &TemplateProcessor{
		templateCache: make(map[string]TemplateInfo),
		logger:        logger.With().Str("component", "template_processor").Logger(),
	}
	tp.initializeTemplates()
	return tp
}

// initializeTemplates sets up the available templates
func (tp *TemplateProcessor) initializeTemplates() {
	templates := []TemplateInfo{
		{
			Name:        "microservice-basic",
			Description: "Basic microservice deployment",
			Languages:   []string{"go", "java", "python", "node", "dotnet"},
			Frameworks:  []string{},
			Features:    []string{"service", "deployment", "basic"},
			Priority:    1,
		},
		{
			Name:        "microservice-advanced",
			Description: "Advanced microservice with monitoring and scaling",
			Languages:   []string{"go", "java", "python", "node", "dotnet"},
			Frameworks:  []string{},
			Features:    []string{"service", "deployment", "hpa", "monitoring", "health-checks"},
			Priority:    2,
		},
		{
			Name:        "web-application",
			Description: "Web application with ingress",
			Languages:   []string{"python", "node", "ruby", "php"},
			Frameworks:  []string{"django", "flask", "express", "rails", "laravel"},
			Features:    []string{"service", "deployment", "ingress", "web"},
			Priority:    2,
		},
		{
			Name:        "stateful-application",
			Description: "Application with persistent storage",
			Languages:   []string{"*"},
			Frameworks:  []string{},
			Features:    []string{"service", "deployment", "pvc", "statefulset"},
			Priority:    3,
		},
		{
			Name:        "job-batch",
			Description: "Batch job processing",
			Languages:   []string{"*"},
			Frameworks:  []string{},
			Features:    []string{"job", "cronjob", "batch"},
			Priority:    1,
		},
		{
			Name:        "api-gateway",
			Description: "API Gateway pattern",
			Languages:   []string{"go", "java", "node"},
			Frameworks:  []string{"kong", "zuul", "express-gateway"},
			Features:    []string{"service", "deployment", "ingress", "api", "gateway"},
			Priority:    3,
		},
	}

	for _, template := range templates {
		tp.templateCache[template.Name] = template
	}
}

// SelectTemplate selects the best template based on session context
func (tp *TemplateProcessor) SelectTemplate(session *session.SessionState, args GenerateManifestsRequest) (string, string, error) {
	tp.logger.Info().
		Str("session_id", args.SessionID).
		Msg("Selecting template for manifest generation")

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

		tp.logger.Debug().
			Str("template", name).
			Int("score", score).
			Msg("Template scored")

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

	tp.logger.Info().
		Str("template", bestTemplate).
		Int("score", bestScore).
		Str("info", strings.Join(selectionInfo, ", ")).
		Msg("Template selected")

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
	tp.logger.Info().
		Str("template", templateName).
		Msg("Processing template")

	// In a real implementation, this would use a proper template engine
	// For now, we just return a placeholder
	return fmt.Sprintf("# Template: %s\n# Processed with data\n", templateName), nil
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
		return fmt.Errorf("template '%s' not found", templateName)
	}
	return nil
}
