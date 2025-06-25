package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// LanguageAnalyzer analyzes programming languages and frameworks
type LanguageAnalyzer struct {
	logger zerolog.Logger
}

// NewLanguageAnalyzer creates a new language analyzer
func NewLanguageAnalyzer(logger zerolog.Logger) *LanguageAnalyzer {
	return &LanguageAnalyzer{
		logger: logger.With().Str("engine", "language").Logger(),
	}
}

// GetName returns the name of this engine
func (l *LanguageAnalyzer) GetName() string {
	return "language_analyzer"
}

// GetCapabilities returns what this engine can analyze
func (l *LanguageAnalyzer) GetCapabilities() []string {
	return []string{
		"programming_languages",
		"web_frameworks",
		"runtime_detection",
		"technology_stack",
		"version_analysis",
	}
}

// IsApplicable determines if this engine should run
func (l *LanguageAnalyzer) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	// Always applicable - every repo has some language/framework
	return true
}

// Analyze performs language and framework analysis
func (l *LanguageAnalyzer) Analyze(ctx context.Context, config AnalysisConfig) (*AnalysisResult, error) {
	startTime := time.Now()
	result := &AnalysisResult{
		Engine:   l.GetName(),
		Findings: make([]Finding, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Analyze primary languages
	if err := l.analyzePrimaryLanguages(config, result); err != nil {
		result.Errors = append(result.Errors, err)
	}

	// Analyze frameworks if enabled
	if config.Options.IncludeFrameworks {
		if err := l.analyzeFrameworks(config, result); err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	// Analyze runtime requirements
	if err := l.analyzeRuntimeRequirements(config, result); err != nil {
		result.Errors = append(result.Errors, err)
	}

	// Analyze technology stack
	if err := l.analyzeTechnologyStack(config, result); err != nil {
		result.Errors = append(result.Errors, err)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Confidence = l.calculateConfidence(result)

	return result, nil
}

// analyzePrimaryLanguages identifies the primary programming languages
func (l *LanguageAnalyzer) analyzePrimaryLanguages(config AnalysisConfig, result *AnalysisResult) error {
	repoData := config.RepoData

	// Get language percentages from core analysis
	languages := repoData.Languages
	if len(languages) == 0 {
		l.logger.Warn().Msg("No languages detected in repository")
		return nil
	}

	// Find primary language (highest percentage)
	var primaryLang string
	var primaryPercent float64
	for lang, percent := range languages {
		if percent > primaryPercent {
			primaryLang = lang
			primaryPercent = percent
		}
	}

	// Create finding for primary language
	finding := Finding{
		Type:        FindingTypeLanguage,
		Category:    "primary_language",
		Title:       "Primary Programming Language",
		Description: l.generateLanguageDescription(primaryLang, primaryPercent),
		Confidence:  l.getLanguageConfidence(primaryPercent),
		Severity:    SeverityInfo,
		Metadata: map[string]interface{}{
			"language":      primaryLang,
			"percentage":    primaryPercent,
			"all_languages": languages,
		},
		Evidence: []Evidence{
			{
				Type:        "language_detection",
				Description: "Detected through file extension analysis",
				Value:       languages,
			},
		},
	}

	result.Findings = append(result.Findings, finding)

	// Add secondary languages if significant
	for lang, percent := range languages {
		if lang != primaryLang && percent > 10.0 {
			secondaryFinding := Finding{
				Type:        FindingTypeLanguage,
				Category:    "secondary_language",
				Title:       "Secondary Programming Language",
				Description: l.generateLanguageDescription(lang, percent),
				Confidence:  l.getLanguageConfidence(percent),
				Severity:    SeverityInfo,
				Metadata: map[string]interface{}{
					"language":   lang,
					"percentage": percent,
				},
			}
			result.Findings = append(result.Findings, secondaryFinding)
		}
	}

	return nil
}

// analyzeFrameworks identifies web frameworks and libraries
func (l *LanguageAnalyzer) analyzeFrameworks(config AnalysisConfig, result *AnalysisResult) error {
	repoData := config.RepoData

	// Check for framework indicators in files
	frameworkIndicators := map[string][]string{
		"React":         {"package.json:react", "src/App.js", "src/App.jsx", "public/index.html"},
		"Vue.js":        {"package.json:vue", "src/main.js", "src/App.vue"},
		"Angular":       {"package.json:@angular", "angular.json", "src/app/app.module.ts"},
		"Express.js":    {"package.json:express", "app.js", "server.js"},
		"Next.js":       {"package.json:next", "next.config.js", "pages/"},
		"Nuxt.js":       {"package.json:nuxt", "nuxt.config.js"},
		"Django":        {"requirements.txt:django", "manage.py", "settings.py"},
		"Flask":         {"requirements.txt:flask", "app.py"},
		"FastAPI":       {"requirements.txt:fastapi", "main.py"},
		"Spring Boot":   {"pom.xml:spring-boot", "build.gradle:spring-boot"},
		"Laravel":       {"composer.json:laravel", "artisan"},
		"Ruby on Rails": {"Gemfile:rails", "config/application.rb"},
		"ASP.NET Core":  {"*.csproj", "Program.cs", "Startup.cs"},
	}

	for framework, indicators := range frameworkIndicators {
		confidence := l.checkFrameworkIndicators(repoData, indicators)
		if confidence > 0.3 {
			finding := Finding{
				Type:        FindingTypeFramework,
				Category:    "web_framework",
				Title:       framework + " Framework Detected",
				Description: l.generateFrameworkDescription(framework, confidence),
				Confidence:  confidence,
				Severity:    SeverityInfo,
				Metadata: map[string]interface{}{
					"framework":  framework,
					"indicators": indicators,
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzeRuntimeRequirements identifies runtime and version requirements
func (l *LanguageAnalyzer) analyzeRuntimeRequirements(config AnalysisConfig, result *AnalysisResult) error {
	repoData := config.RepoData

	// Check for runtime version files
	runtimeFiles := map[string]string{
		".node-version":      "Node.js",
		".nvmrc":             "Node.js",
		".python-version":    "Python",
		".ruby-version":      "Ruby",
		".java-version":      "Java",
		"runtime.txt":        "Python/Heroku",
		"Dockerfile":         "Docker",
		"docker-compose.yml": "Docker Compose",
	}

	for file, runtime := range runtimeFiles {
		if l.fileExists(repoData, file) {
			finding := Finding{
				Type:        FindingTypeLanguage,
				Category:    "runtime_requirement",
				Title:       runtime + " Runtime Configuration",
				Description: "Runtime version configuration detected",
				Confidence:  0.9,
				Severity:    SeverityInfo,
				Location: &Location{
					Path: file,
				},
				Metadata: map[string]interface{}{
					"runtime": runtime,
					"file":    file,
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzeTechnologyStack provides overall technology stack assessment
func (l *LanguageAnalyzer) analyzeTechnologyStack(config AnalysisConfig, result *AnalysisResult) error {
	// Aggregate findings to determine technology stack
	languages := make(map[string]float64)
	frameworks := make([]string, 0)
	runtimes := make([]string, 0)

	for _, finding := range result.Findings {
		switch finding.Category {
		case "primary_language", "secondary_language":
			if lang, ok := finding.Metadata["language"].(string); ok {
				if percent, ok := finding.Metadata["percentage"].(float64); ok {
					languages[lang] = percent
				}
			}
		case "web_framework":
			if framework, ok := finding.Metadata["framework"].(string); ok {
				frameworks = append(frameworks, framework)
			}
		case "runtime_requirement":
			if runtime, ok := finding.Metadata["runtime"].(string); ok {
				runtimes = append(runtimes, runtime)
			}
		}
	}

	// Create technology stack summary
	stackFinding := Finding{
		Type:        FindingTypeLanguage,
		Category:    "technology_stack",
		Title:       "Technology Stack Summary",
		Description: l.generateStackDescription(languages, frameworks, runtimes),
		Confidence:  0.95,
		Severity:    SeverityInfo,
		Metadata: map[string]interface{}{
			"languages":  languages,
			"frameworks": frameworks,
			"runtimes":   runtimes,
			"stack_type": l.classifyStackType(languages, frameworks),
		},
	}

	result.Findings = append(result.Findings, stackFinding)
	return nil
}

// Helper methods

func (l *LanguageAnalyzer) generateLanguageDescription(language string, percentage float64) string {
	return fmt.Sprintf("Primary language %s detected (%.1f%% of codebase)", language, percentage)
}

func (l *LanguageAnalyzer) generateFrameworkDescription(framework string, confidence float64) string {
	return fmt.Sprintf("%s framework detected with %.0f%% confidence", framework, confidence*100)
}

func (l *LanguageAnalyzer) generateStackDescription(languages map[string]float64, frameworks, runtimes []string) string {
	var primary string
	var maxPercent float64
	for lang, percent := range languages {
		if percent > maxPercent {
			primary = lang
			maxPercent = percent
		}
	}

	desc := fmt.Sprintf("Technology stack: %s", primary)
	if len(frameworks) > 0 {
		desc += fmt.Sprintf(" with %s", strings.Join(frameworks, ", "))
	}
	if len(runtimes) > 0 {
		desc += fmt.Sprintf(" (runtimes: %s)", strings.Join(runtimes, ", "))
	}
	return desc
}

func (l *LanguageAnalyzer) getLanguageConfidence(percentage float64) float64 {
	if percentage > 80 {
		return 0.95
	} else if percentage > 60 {
		return 0.85
	} else if percentage > 40 {
		return 0.75
	} else if percentage > 20 {
		return 0.65
	}
	return 0.5
}

func (l *LanguageAnalyzer) checkFrameworkIndicators(repoData *RepoData, indicators []string) float64 {
	matches := 0
	total := len(indicators)

	for _, indicator := range indicators {
		if strings.Contains(indicator, ":") {
			// File content check (e.g., "package.json:react")
			parts := strings.Split(indicator, ":")
			if len(parts) == 2 && l.fileContains(repoData, parts[0], parts[1]) {
				matches++
			}
		} else {
			// File existence check
			if l.fileExists(repoData, indicator) {
				matches++
			}
		}
	}

	return float64(matches) / float64(total)
}

func (l *LanguageAnalyzer) fileExists(repoData *RepoData, filename string) bool {
	for _, file := range repoData.Files {
		if strings.HasSuffix(file.Path, filename) ||
			filepath.Base(file.Path) == filename ||
			strings.Contains(file.Path, filename) {
			return true
		}
	}
	return false
}

func (l *LanguageAnalyzer) fileContains(repoData *RepoData, filename, content string) bool {
	for _, file := range repoData.Files {
		if strings.HasSuffix(file.Path, filename) || filepath.Base(file.Path) == filename {
			return strings.Contains(strings.ToLower(file.Content), strings.ToLower(content))
		}
	}
	return false
}

func (l *LanguageAnalyzer) classifyStackType(languages map[string]float64, frameworks []string) string {
	// Determine if it's frontend, backend, or fullstack
	hasBackend := false
	hasFrontend := false

	for lang := range languages {
		switch strings.ToLower(lang) {
		case "javascript", "typescript", "html", "css":
			hasFrontend = true
		case "go", "python", "java", "c#", "ruby", "php":
			hasBackend = true
		}
	}

	for _, framework := range frameworks {
		switch framework {
		case "React", "Vue.js", "Angular":
			hasFrontend = true
		case "Express.js", "Django", "Flask", "FastAPI", "Spring Boot", "Laravel", "Ruby on Rails", "ASP.NET Core":
			hasBackend = true
		case "Next.js", "Nuxt.js":
			hasFrontend = true
			hasBackend = true // These can do SSR
		}
	}

	if hasFrontend && hasBackend {
		return "fullstack"
	} else if hasFrontend {
		return "frontend"
	} else if hasBackend {
		return "backend"
	}
	return "unknown"
}

func (l *LanguageAnalyzer) calculateConfidence(result *AnalysisResult) float64 {
	if len(result.Findings) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, finding := range result.Findings {
		totalConfidence += finding.Confidence
	}

	return totalConfidence / float64(len(result.Findings))
}
