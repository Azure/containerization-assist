package analyze

import (
	"log/slog"
	"strings"
	"sync"
)

// TemplateService provides template selection without global state
type TemplateService struct {
	mu                       sync.RWMutex
	languageCompatibilityMap map[string][]string
	configFileScores         map[string]map[string]int
	logger                   *slog.Logger
}

// NewTemplateService creates a new template service
func NewTemplateService(logger *slog.Logger) *TemplateService {
	return &TemplateService{
		languageCompatibilityMap: getDefaultLanguageCompatibilityMap(),
		configFileScores:         getDefaultConfigFileScores(),
		logger:                   logger,
	}
}

// getDefaultLanguageCompatibilityMap returns the default language compatibility mapping
func getDefaultLanguageCompatibilityMap() map[string][]string {
	return map[string][]string{
		"go":     {"go", "golang"},
		"node":   {"javascript", "typescript", "node", "nodejs"},
		"python": {"python"},
		"java":   {"java"},
		"rust":   {"rust"},
		"php":    {"php"},
		"ruby":   {"ruby"},
		"dotnet": {"c#", "csharp", "f#", "vb"},
	}
}

// getDefaultConfigFileScores returns the default config file scoring
func getDefaultConfigFileScores() map[string]map[string]int {
	return map[string]map[string]int{
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
			"pom.xml":      20,
			"build.gradle": 20,
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
			"*.csproj":        20,
			"*.sln":           20,
			"packages.config": 20,
		},
	}
}

// SelectTemplate selects the best template based on repository analysis
func (ts *TemplateService) SelectTemplate(repoAnalysis map[string]interface{}) (string, error) {
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
		return "ruby", nil
	case "c#", "csharp", "f#", "vb":
		return "dotnet", nil
	default:
		return "generic", nil
	}
}

// ScoreLanguageMatch scores language compatibility
func (ts *TemplateService) ScoreLanguageMatch(templateType, language string) int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if compatibleLanguages, exists := ts.languageCompatibilityMap[templateType]; exists {
		if ts.isLanguageCompatible(language, compatibleLanguages) {
			return 100
		}
	}
	return 0
}

// ScoreConfigFiles scores based on config file presence
func (ts *TemplateService) ScoreConfigFiles(templateType string, configFiles []string) int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	scores, exists := ts.configFileScores[templateType]
	if !exists {
		return 0
	}

	totalScore := 0
	for _, file := range configFiles {
		if score, exists := scores[file]; exists {
			totalScore += score
		}
	}

	return totalScore
}

// isLanguageCompatible checks if a language is compatible with the template
func (ts *TemplateService) isLanguageCompatible(language string, compatibleLanguages []string) bool {
	language = strings.ToLower(language)
	for _, compatible := range compatibleLanguages {
		if language == strings.ToLower(compatible) {
			return true
		}
	}
	return false
}

// SetLanguageCompatibility sets custom language compatibility (for testing or customization)
func (ts *TemplateService) SetLanguageCompatibility(templateType string, languages []string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.languageCompatibilityMap[templateType] = languages
}

// SetConfigFileScores sets custom config file scores (for testing or customization)
func (ts *TemplateService) SetConfigFileScores(templateType string, scores map[string]int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.configFileScores[templateType] = scores
}
