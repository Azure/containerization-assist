package analyze

import (
	"fmt"
	"log/slog"
	"strings"
)

// TemplateSelector handles Dockerfile template selection and scoring
type TemplateSelector struct {
	logger *slog.Logger
}

// NewTemplateSelector creates a new template selector
func NewTemplateSelector(logger *slog.Logger) *TemplateSelector {
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
	ts.logger.Warn("No specific template found, using generic template",
		"language", language,
		"framework", framework)

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

// languageCompatibilityMap defines language compatibility for template types
// languageCompatibilityMap is deprecated - use TemplateService instead
// var languageCompatibilityMap = map[string][]string{ ... } // REMOVED: Global state eliminated

// scoreLanguageMatch scores language compatibility
// Deprecated: Use TemplateService.ScoreLanguageMatch instead
func (ts *TemplateSelector) scoreLanguageMatch(templateType, language string) int {
	// NOTE: This method is deprecated. Use TemplateService for template selection without global state.
	panic("scoreLanguageMatch is deprecated - use TemplateService.ScoreLanguageMatch instead")
}

// isLanguageCompatible checks if a language is in the compatible languages list
func (ts *TemplateSelector) isLanguageCompatible(language string, compatibleLanguages []string) bool {
	for _, compatibleLang := range compatibleLanguages {
		if language == compatibleLang {
			return true
		}
	}
	return false
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

// configFileScores is deprecated - use TemplateService instead
// var configFileScores = map[string]map[string]int{ // REMOVED: Global state eliminated
/*
	"go": {
		"go.mod": 20,
		"go.sum": 20,
	},
	"node": {
		"package.json":      20,
		"package-lock.json": 20,
		"yarn.lock":         20,
		"pnpm-lock.yaml":    20,
	},
	"python": {
		"requirements.txt": 20,
		"setup.py":         20,
		"Pipfile":          20,
		"pyproject.toml":   20,
		"poetry.lock":      20,
	},
	"java": {
		"pom.xml":          20,
		"build.gradle":     20,
		"build.gradle.kts": 20,
	},
	"rust": {
		"Cargo.toml": 20,
		"Cargo.lock": 20,
	},
	"php": {
		"composer.json": 20,
		"composer.lock": 20,
	},
	"ruby": {
		"Gemfile":      20,
		"Gemfile.lock": 20,
	},
	"dotnet": {
		"global.json": 20,
	},
}
*/

// getConfigFileScore returns score for specific config file
// This function is deprecated - use TemplateService instead
func (ts *TemplateSelector) getConfigFileScore(templateType, file string) int {
	// Return 0 as this function is deprecated
	return 0
}

// isDotNetProjectFile checks if file is a .NET project file
func (ts *TemplateSelector) isDotNetProjectFile(file string) bool {
	return strings.HasSuffix(file, ".csproj") ||
		strings.HasSuffix(file, ".fsproj") ||
		strings.HasSuffix(file, ".vbproj")
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
