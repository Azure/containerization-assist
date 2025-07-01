package analyze

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// TemplateSelector handles Dockerfile template selection and scoring
type TemplateSelector struct {
	logger zerolog.Logger
}

// NewTemplateSelector creates a new template selector
func NewTemplateSelector(logger zerolog.Logger) *TemplateSelector {
	return &TemplateSelector{
		logger: logger,
	}
}

// SelectTemplate selects the best template based on repository analysis
func (ts *TemplateSelector) SelectTemplate(repoAnalysis map[string]interface{}) (string, error) {
	// Extract language and framework information
	language := ""
	framework := ""

	if lang, ok := repoAnalysis["primary_language"].(string); ok {
		language = strings.ToLower(lang)
	}

	if fw, ok := repoAnalysis["framework"].(string); ok {
		framework = strings.ToLower(fw)
	}

	// Direct language mapping
	switch language {
	case "go", "golang":
		return "go", nil
	case "javascript", "typescript":
		if framework == "react" || framework == "vue" || framework == "angular" {
			return "node", nil
		}
		return "node", nil
	case "python":
		if framework == "django" || framework == "flask" {
			return "python", nil
		}
		return "python", nil
	case "java":
		if framework == "spring" || framework == "springboot" {
			return "java", nil
		}
		return "java", nil
	case "rust":
		return "rust", nil
	case "php":
		return "php", nil
	case "ruby":
		if framework == "rails" {
			return "ruby", nil
		}
		return "ruby", nil
	case "c#", "csharp":
		return "dotnet", nil
	}

	// Fallback to generic template
	ts.logger.Warn().
		Str("language", language).
		Str("framework", framework).
		Msg("No specific template found, using generic template")

	return "generic", nil
}

// GetRecommendedBaseImage returns the recommended base image for a language/framework
func (ts *TemplateSelector) GetRecommendedBaseImage(language, framework string) string {
	// Language-specific base images
	switch language {
	case "go", "golang":
		return "golang:1.21-alpine"
	case "python":
		if framework == "django" || framework == "flask" {
			return "python:3.11-slim"
		}
		return "python:3.11-alpine"
	case "node", "nodejs", "javascript", "typescript":
		return "node:20-alpine"
	case "java":
		if framework == "spring" || framework == "springboot" {
			return "eclipse-temurin:17-jre-alpine"
		}
		return "openjdk:17-alpine"
	case "rust":
		return "rust:1.70-alpine"
	case "ruby":
		if framework == "rails" {
			return "ruby:3.2-slim"
		}
		return "ruby:3.2-alpine"
	case "php":
		return "php:8.2-fpm-alpine"
	case "dotnet", "c#", "csharp":
		return "mcr.microsoft.com/dotnet/sdk:7.0-alpine"
	default:
		return "alpine:latest"
	}
}

// GenerateTemplateSelectionContext creates a rich context for template selection
func (ts *TemplateSelector) GenerateTemplateSelectionContext(language, framework string, dependencies, configFiles []string) *TemplateSelectionContext {
	options := ts.getTemplateOptions(language, framework, dependencies, configFiles)
	alternatives := ts.getAlternativeTemplates(language, framework, dependencies)

	// Find best match
	var recommended string
	maxScore := 0
	for _, opt := range options {
		if opt.MatchScore > maxScore {
			maxScore = opt.MatchScore
			recommended = opt.Name
		}
	}

	reasoning := []string{
		fmt.Sprintf("Detected language: %s", language),
	}
	if framework != "" {
		reasoning = append(reasoning, fmt.Sprintf("Detected framework: %s", framework))
	}
	if len(dependencies) > 0 {
		reasoning = append(reasoning, fmt.Sprintf("Found %d dependencies", len(dependencies)))
	}
	if len(configFiles) > 0 {
		reasoning = append(reasoning, fmt.Sprintf("Found configuration files: %v", configFiles))
	}

	return &TemplateSelectionContext{
		DetectedLanguage:    language,
		DetectedFramework:   framework,
		AvailableTemplates:  options,
		RecommendedTemplate: recommended,
		SelectionReasoning:  reasoning,
		AlternativeOptions:  alternatives,
	}
}

// getTemplateOptions returns available template options with scores
func (ts *TemplateSelector) getTemplateOptions(language, framework string, dependencies, configFiles []string) []TemplateOption {
	options := []TemplateOption{
		{
			Name:        "generic",
			Description: "Generic multi-purpose Dockerfile",
			BestFor:     []string{"simple applications", "static files", "compiled binaries"},
			Limitations: []string{"no language-specific optimizations", "manual dependency management"},
			MatchScore:  10, // Base score
		},
	}

	// Add language-specific templates
	templateTypes := []string{"go", "node", "python", "java", "rust", "php", "ruby", "dotnet"}

	for _, tmplType := range templateTypes {
		option := TemplateOption{
			Name:        tmplType,
			Description: ts.getTemplateDescription(tmplType),
			BestFor:     ts.getTemplateBestFor(tmplType),
			Limitations: ts.getTemplateLimitations(tmplType),
			MatchScore:  ts.calculateMatchScore(tmplType, language, framework, configFiles),
		}
		options = append(options, option)
	}

	return options
}

// calculateMatchScore calculates how well a template matches the project
func (ts *TemplateSelector) calculateMatchScore(templateType, language, framework string, configFiles []string) int {
	score := 0

	// Language match
	score += ts.scoreLanguageMatch(templateType, language)

	// Framework match
	score += ts.scoreFrameworkMatch(templateType, framework)

	// Config file match
	score += ts.scoreConfigFileMatch(templateType, configFiles)

	return score
}

// scoreLanguageMatch scores language compatibility
func (ts *TemplateSelector) scoreLanguageMatch(templateType, language string) int {
	switch templateType {
	case "go":
		if language == "go" || language == "golang" {
			return 100
		}
	case "node":
		if language == "javascript" || language == "typescript" || language == "node" || language == "nodejs" {
			return 100
		}
	case "python":
		if language == "python" {
			return 100
		}
	case "java":
		if language == "java" {
			return 100
		}
	case "rust":
		if language == "rust" {
			return 100
		}
	case "php":
		if language == "php" {
			return 100
		}
	case "ruby":
		if language == "ruby" {
			return 100
		}
	case "dotnet":
		if language == "c#" || language == "csharp" || language == "f#" || language == "vb" {
			return 100
		}
	}
	return 0
}

// scoreFrameworkMatch scores framework compatibility
func (ts *TemplateSelector) scoreFrameworkMatch(templateType, framework string) int {
	if framework == "" {
		return 0
	}

	switch templateType {
	case "node":
		if framework == "express" || framework == "react" || framework == "vue" || framework == "angular" || framework == "next" || framework == "nuxt" {
			return 50
		}
	case "python":
		if framework == "django" || framework == "flask" || framework == "fastapi" {
			return 50
		}
	case "java":
		if framework == "spring" || framework == "springboot" {
			return 50
		}
	case "ruby":
		if framework == "rails" || framework == "sinatra" {
			return 50
		}
	}
	return 0
}

// scoreConfigFileMatch scores based on configuration files
func (ts *TemplateSelector) scoreConfigFileMatch(templateType string, configFiles []string) int {
	score := 0
	for _, file := range configFiles {
		score += ts.getConfigFileScore(templateType, file)
	}
	return score
}

// getConfigFileScore returns score for specific config file
func (ts *TemplateSelector) getConfigFileScore(templateType, file string) int {
	switch templateType {
	case "go":
		if file == "go.mod" || file == "go.sum" {
			return 20
		}
	case "node":
		if file == "package.json" || file == "package-lock.json" || file == "yarn.lock" || file == "pnpm-lock.yaml" {
			return 20
		}
	case "python":
		if file == "requirements.txt" || file == "setup.py" || file == "Pipfile" || file == "pyproject.toml" || file == "poetry.lock" {
			return 20
		}
	case "java":
		if file == "pom.xml" || file == "build.gradle" || file == "build.gradle.kts" {
			return 20
		}
	case "rust":
		if file == "Cargo.toml" || file == "Cargo.lock" {
			return 20
		}
	case "php":
		if file == "composer.json" || file == "composer.lock" {
			return 20
		}
	case "ruby":
		if file == "Gemfile" || file == "Gemfile.lock" {
			return 20
		}
	case "dotnet":
		if strings.HasSuffix(file, ".csproj") || strings.HasSuffix(file, ".fsproj") || strings.HasSuffix(file, ".vbproj") || file == "global.json" {
			return 20
		}
	}
	return 0
}

// getAlternativeTemplates returns alternative template suggestions
func (ts *TemplateSelector) getAlternativeTemplates(language, framework string, dependencies []string) []AlternativeTemplate {
	var alternatives []AlternativeTemplate

	// Suggest multi-stage builds for compiled languages
	if language == "go" || language == "rust" || language == "java" || language == "c#" {
		alternatives = append(alternatives, AlternativeTemplate{
			Template: "multi-stage",
			Reason:   "Reduces final image size by separating build and runtime",
			TradeOffs: []string{
				"Longer build times",
				"More complex Dockerfile",
			},
			UseCases: []string{
				"Production deployments",
				"Security-conscious environments",
				"Size-constrained deployments",
			},
		})
	}

	// Suggest distroless for security
	alternatives = append(alternatives, AlternativeTemplate{
		Template: "distroless",
		Reason:   "Minimal attack surface with only application runtime",
		TradeOffs: []string{
			"No shell access for debugging",
			"Limited runtime tools",
			"May require additional configuration",
		},
		UseCases: []string{
			"High-security environments",
			"Production microservices",
			"Compliance requirements",
		},
	})

	return alternatives
}

// getTemplateDescription returns template description
func (ts *TemplateSelector) getTemplateDescription(templateType string) string {
	descriptions := map[string]string{
		"go":     "Optimized for Go applications with module support",
		"node":   "Node.js template with npm/yarn support",
		"python": "Python template with pip/poetry support",
		"java":   "Java template with Maven/Gradle support",
		"rust":   "Rust template with Cargo support",
		"php":    "PHP template with Composer support",
		"ruby":   "Ruby template with Bundler support",
		"dotnet": ".NET template with NuGet support",
	}

	if desc, ok := descriptions[templateType]; ok {
		return desc
	}
	return "Generic template"
}

// getTemplateBestFor returns what the template is best for
func (ts *TemplateSelector) getTemplateBestFor(templateType string) []string {
	bestFor := map[string][]string{
		"go":     {"microservices", "CLI tools", "web APIs"},
		"node":   {"web applications", "REST APIs", "frontend builds"},
		"python": {"data science", "web applications", "scripts"},
		"java":   {"enterprise applications", "Spring Boot", "microservices"},
		"rust":   {"system programming", "performance-critical apps", "WebAssembly"},
		"php":    {"web applications", "WordPress", "Laravel"},
		"ruby":   {"Rails applications", "web APIs", "scripts"},
		"dotnet": {"enterprise applications", "web APIs", "microservices"},
	}

	if bf, ok := bestFor[templateType]; ok {
		return bf
	}
	return []string{"general purpose"}
}

// getTemplateLimitations returns template limitations
func (ts *TemplateSelector) getTemplateLimitations(templateType string) []string {
	limitations := map[string][]string{
		"go":     {"requires Go modules", "single binary output"},
		"node":   {"large node_modules", "requires build step for TypeScript"},
		"python": {"global package conflicts", "virtual environment setup"},
		"java":   {"JVM overhead", "longer startup times"},
		"rust":   {"long compilation times", "large build cache"},
		"php":    {"requires web server", "extension management"},
		"ruby":   {"gem conflicts", "version management"},
		"dotnet": {"runtime size", "Windows-specific features"},
	}

	if lim, ok := limitations[templateType]; ok {
		return lim
	}
	return []string{"generic limitations"}
}

// MapCommonTemplateNames maps common template name variations
func (ts *TemplateSelector) MapCommonTemplateNames(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	// Handle common variations
	switch name {
	case "golang":
		return "go"
	case "nodejs", "js", "javascript", "typescript", "ts":
		return "node"
	case "py":
		return "python"
	case "cs", "csharp", "c#", "net", ".net", "dotnetcore":
		return "dotnet"
	default:
		return name
	}
}
