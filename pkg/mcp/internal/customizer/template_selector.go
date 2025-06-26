package customizer

import (
	"strings"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/rs/zerolog"
)

// Selector handles template selection logic
type Selector struct {
	logger zerolog.Logger
}

// NewSelector creates a new template selector
func NewSelector(logger zerolog.Logger) *Selector {
	return &Selector{
		logger: logger.With().Str("component", "template_selector").Logger(),
	}
}

// SelectDockerfileTemplate selects the best Dockerfile template based on analysis
func (s *Selector) SelectDockerfileTemplate(repoAnalysis *analysis.AnalysisResult) string {
	if repoAnalysis == nil {
		return "generic"
	}

	language := strings.ToLower(repoAnalysis.Language)
	framework := strings.ToLower(repoAnalysis.Framework)

	// Framework-specific templates take precedence
	if framework != "" {
		template := s.getFrameworkTemplate(language, framework)
		if template != "" {
			s.logger.Debug().
				Str("language", language).
				Str("framework", framework).
				Str("template", template).
				Msg("Selected framework-specific template")
			return template
		}
	}

	// Language-specific templates
	template := s.getLanguageTemplate(language)
	if template != "" {
		s.logger.Debug().
			Str("language", language).
			Str("template", template).
			Msg("Selected language-specific template")
		return template
	}

	// Default to generic template
	s.logger.Debug().Msg("Selected generic template")
	return "generic"
}

// getFrameworkTemplate returns framework-specific template name
func (s *Selector) getFrameworkTemplate(language, framework string) string {
	// Mapping of language+framework to template names
	templateMap := map[string]map[string]string{
		"javascript": {
			"express": "node-express",
			"next.js": "nextjs",
			"nextjs":  "nextjs",
			"react":   "react-spa",
			"vue":     "vue-spa",
			"angular": "angular-spa",
		},
		"typescript": {
			"express": "node-express",
			"next.js": "nextjs",
			"nextjs":  "nextjs",
			"react":   "react-spa",
			"vue":     "vue-spa",
			"angular": "angular-spa",
		},
		"python": {
			"django":  "python-django",
			"flask":   "python-flask",
			"fastapi": "python-fastapi",
		},
		"java": {
			"spring":      "java-spring",
			"spring boot": "java-spring",
			"springboot":  "java-spring",
		},
		"go": {
			"gin":   "go-gin",
			"echo":  "go-echo",
			"fiber": "go-fiber",
		},
		"c#": {
			"asp.net":      "dotnet-aspnet",
			"asp.net core": "dotnet-aspnet",
		},
		"csharp": {
			"asp.net":      "dotnet-aspnet",
			"asp.net core": "dotnet-aspnet",
		},
	}

	if langMap, exists := templateMap[language]; exists {
		if template, exists := langMap[framework]; exists {
			return template
		}
	}

	return ""
}

// getLanguageTemplate returns language-specific template name
func (s *Selector) getLanguageTemplate(language string) string {
	languageTemplates := map[string]string{
		"go":         "go-generic",
		"python":     "python-generic",
		"javascript": "node-generic",
		"typescript": "node-generic",
		"java":       "java-generic",
		"c#":         "dotnet-generic",
		"csharp":     "dotnet-generic",
		"ruby":       "ruby-generic",
		"php":        "php-generic",
		"rust":       "rust-generic",
	}

	if template, exists := languageTemplates[language]; exists {
		return template
	}

	return ""
}

// CreateTemplateContext creates a template context from repository analysis
func (s *Selector) CreateTemplateContext(repoAnalysis *analysis.AnalysisResult) *TemplateContext {
	// Convert dependencies to string array
	deps := make([]string, len(repoAnalysis.Dependencies))
	for i, dep := range repoAnalysis.Dependencies {
		deps[i] = dep.Name
	}

	ctx := &TemplateContext{
		Language:     repoAnalysis.Language,
		Framework:    repoAnalysis.Framework,
		Dependencies: deps,
	}

	// Analyze repository characteristics
	for _, configFile := range repoAnalysis.ConfigFiles {
		path := strings.ToLower(configFile.Path)

		// Check for test files
		if strings.Contains(path, "test") || strings.Contains(path, "spec") {
			ctx.HasTests = true
		}

		// Check for database configuration
		if strings.Contains(path, "database") || strings.Contains(path, "db") ||
			strings.Contains(path, "postgres") || strings.Contains(path, "mysql") ||
			strings.Contains(path, "mongo") {
			ctx.HasDatabase = true
		}
	}

	// Check for web application indicators
	ctx.IsWebApp = s.isWebApplication(repoAnalysis)

	// Check for static files
	ctx.HasStaticFiles = s.hasStaticFiles(repoAnalysis)

	return ctx
}

// isWebApplication determines if the repository is a web application
func (s *Selector) isWebApplication(analysis *analysis.AnalysisResult) bool {
	// Framework indicators
	webFrameworks := []string{
		"express", "flask", "django", "fastapi", "spring", "asp.net",
		"rails", "laravel", "next.js", "react", "vue", "angular",
	}

	framework := strings.ToLower(analysis.Framework)
	for _, wf := range webFrameworks {
		if strings.Contains(framework, wf) {
			return true
		}
	}

	// Port indicator
	if analysis.Port > 0 {
		return true
	}

	// File indicators
	for _, configFile := range analysis.ConfigFiles {
		path := strings.ToLower(configFile.Path)
		if strings.Contains(path, "routes") || strings.Contains(path, "controllers") ||
			strings.Contains(path, "views") || strings.Contains(path, "templates") {
			return true
		}
	}

	return false
}

// hasStaticFiles checks if the repository has static files
func (s *Selector) hasStaticFiles(analysis *analysis.AnalysisResult) bool {
	for _, configFile := range analysis.ConfigFiles {
		path := strings.ToLower(configFile.Path)
		if strings.Contains(path, "static") || strings.Contains(path, "public") ||
			strings.Contains(path, "assets") || strings.Contains(path, "dist") {
			return true
		}
	}
	return false
}
