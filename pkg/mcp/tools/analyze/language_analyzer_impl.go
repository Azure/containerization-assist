package analyze

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// analyzeProgrammingLanguages detects programming languages by file extensions and content
func (l *LanguageAnalyzer) analyzeProgrammingLanguages(config AnalysisConfig, result *EngineAnalysisResult) {
	languageMap := map[string]string{
		".go":    "Go",
		".py":    "Python",
		".js":    "JavaScript",
		".ts":    "TypeScript",
		".java":  "Java",
		".c":     "C",
		".cpp":   "C++",
		".cs":    "C#",
		".rb":    "Ruby",
		".php":   "PHP",
		".rs":    "Rust",
		".swift": "Swift",
		".kt":    "Kotlin",
		".scala": "Scala",
		".r":     "R",
		".m":     "Objective-C",
		".pl":    "Perl",
		".sh":    "Shell",
		".ps1":   "PowerShell",
	}

	languageCounts := make(map[string]int)
	languageFiles := make(map[string][]string)

	for _, file := range config.RepoData.Files {
		ext := strings.ToLower(filepath.Ext(file.Path))
		if language, exists := languageMap[ext]; exists {
			languageCounts[language]++
			if languageFiles[language] == nil {
				languageFiles[language] = []string{}
			}
			languageFiles[language] = append(languageFiles[language], file.Path)
		}
	}

	for language, count := range languageCounts {
		l.addLanguageFinding(language, count, languageFiles[language], result)
	}
}

// analyzeWebFrameworks detects web frameworks based on file patterns and content
func (l *LanguageAnalyzer) analyzeWebFrameworks(config AnalysisConfig, result *EngineAnalysisResult) {
	frameworkPatterns := map[*regexp.Regexp]string{
		// JavaScript/TypeScript frameworks
		regexp.MustCompile(`(?i)import.*react`):   "React",
		regexp.MustCompile(`(?i)import.*vue`):     "Vue.js",
		regexp.MustCompile(`(?i)import.*angular`): "Angular",
		regexp.MustCompile(`(?i)import.*express`): "Express.js",
		regexp.MustCompile(`(?i)import.*next`):    "Next.js",
		regexp.MustCompile(`(?i)import.*nuxt`):    "Nuxt.js",

		// Python frameworks
		regexp.MustCompile(`(?i)from\s+django`):    "Django",
		regexp.MustCompile(`(?i)from\s+flask`):     "Flask",
		regexp.MustCompile(`(?i)from\s+fastapi`):   "FastAPI",
		regexp.MustCompile(`(?i)import\s+tornado`): "Tornado",

		// Go frameworks
		regexp.MustCompile(`(?i)github\.com/gin-gonic`):     "Gin",
		regexp.MustCompile(`(?i)github\.com/gorilla`):       "Gorilla",
		regexp.MustCompile(`(?i)github\.com/labstack/echo`): "Echo",
		regexp.MustCompile(`(?i)github\.com/gofiber`):       "Fiber",

		// Java frameworks
		regexp.MustCompile(`(?i)springframework`): "Spring",
		regexp.MustCompile(`(?i)import.*jakarta`): "Jakarta EE",

		// Other frameworks
		regexp.MustCompile(`(?i)require.*rails`): "Ruby on Rails",
		regexp.MustCompile(`(?i)use.*laravel`):   "Laravel",
	}

	frameworkFiles := make(map[string][]string)

	for _, file := range config.RepoData.Files {
		for pattern, framework := range frameworkPatterns {
			if pattern.MatchString(file.Content) {
				if frameworkFiles[framework] == nil {
					frameworkFiles[framework] = []string{}
				}
				frameworkFiles[framework] = append(frameworkFiles[framework], file.Path)
			}
		}
	}

	for framework, files := range frameworkFiles {
		l.addFrameworkFinding(framework, files, result)
	}
}

// analyzeRuntimeDetection detects runtime requirements and versions
func (l *LanguageAnalyzer) analyzeRuntimeDetection(config AnalysisConfig, result *EngineAnalysisResult) {
	versionPatterns := map[*regexp.Regexp]string{
		regexp.MustCompile(`(?i)node.*version.*['"]([\d\.]+)['"]`): "Node.js",
		regexp.MustCompile(`(?i)python.*['"]([\d\.]+)['"]`):        "Python",
		regexp.MustCompile(`(?i)go\s+([\d\.]+)`):                   "Go",
		regexp.MustCompile(`(?i)java.*['"]([\d\.]+)['"]`):          "Java",
		regexp.MustCompile(`(?i)ruby.*['"]([\d\.]+)['"]`):          "Ruby",
	}

	for _, file := range config.RepoData.Files {
		for pattern, runtime := range versionPatterns {
			matches := pattern.FindAllStringSubmatch(file.Content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					l.addRuntimeVersionFinding(runtime, match[1], file, result)
				}
			}
		}
	}

	// Check for runtime specification files
	runtimeFiles := map[string]string{
		".nvmrc":          "Node.js",
		".python-version": "Python",
		".ruby-version":   "Ruby",
		".go-version":     "Go",
	}

	for _, file := range config.RepoData.Files {
		baseName := filepath.Base(file.Path)
		if runtime, exists := runtimeFiles[baseName]; exists {
			l.addRuntimeFileFinding(runtime, file, result)
		}
	}
}

// analyzeTechnologyStack detects overall technology stack patterns
func (l *LanguageAnalyzer) analyzeTechnologyStack(config AnalysisConfig, result *EngineAnalysisResult) {
	stackPatterns := map[string][]string{
		"MEAN":     {"angular", "express", "mongodb", "node"},
		"MERN":     {"react", "express", "mongodb", "node"},
		"LAMP":     {"linux", "apache", "mysql", "php"},
		"Django":   {"python", "django", "postgresql"},
		"Rails":    {"ruby", "rails", "postgresql"},
		"JAMstack": {"javascript", "api", "markup"},
	}

	content := ""
	for _, file := range config.RepoData.Files {
		content += strings.ToLower(file.Content) + " "
	}

	for stack, technologies := range stackPatterns {
		matchCount := 0
		for _, tech := range technologies {
			if strings.Contains(content, tech) {
				matchCount++
			}
		}

		if matchCount >= len(technologies)/2 { // At least half the technologies match
			confidence := float64(matchCount) / float64(len(technologies))
			l.addTechnologyStackFinding(stack, technologies, confidence, result)
		}
	}
}

// Helper methods for adding findings

func (l *LanguageAnalyzer) addLanguageFinding(language string, count int, files []string, result *EngineAnalysisResult) {
	severity := SeverityInfo
	if count > 10 {
		severity = SeverityMedium // Primary language
	}

	finding := Finding{
		Type:        FindingTypeLanguage,
		Category:    "programming_language",
		Title:       fmt.Sprintf("Programming Language: %s", language),
		Description: fmt.Sprintf("Found %d %s files", count, language),
		Confidence:  0.95,
		Severity:    severity,
		Location: &Location{
			Path: files[0], // Reference first file
		},
		Metadata: map[string]interface{}{
			"language":   language,
			"file_count": count,
			"files":      files,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (l *LanguageAnalyzer) addFrameworkFinding(framework string, files []string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeFramework,
		Category:    "web_framework",
		Title:       fmt.Sprintf("Framework: %s", framework),
		Description: fmt.Sprintf("Detected %s framework usage", framework),
		Confidence:  0.8,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: files[0],
		},
		Metadata: map[string]interface{}{
			"framework":  framework,
			"file_count": len(files),
			"files":      files,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (l *LanguageAnalyzer) addRuntimeVersionFinding(runtime, version string, file FileData, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeLanguage,
		Category:    "runtime_version",
		Title:       fmt.Sprintf("Runtime Version: %s %s", runtime, version),
		Description: "Found runtime version specification",
		Confidence:  0.7,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"runtime": runtime,
			"version": version,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (l *LanguageAnalyzer) addRuntimeFileFinding(runtime string, file FileData, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeLanguage,
		Category:    "runtime_file",
		Title:       fmt.Sprintf("Runtime Configuration: %s", runtime),
		Description: "Found runtime version configuration file",
		Confidence:  0.9,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"runtime":   runtime,
			"file_name": filepath.Base(file.Path),
			"content":   strings.TrimSpace(file.Content),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (l *LanguageAnalyzer) addTechnologyStackFinding(stack string, technologies []string, confidence float64, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeFramework,
		Category:    "technology_stack",
		Title:       fmt.Sprintf("Technology Stack: %s", stack),
		Description: fmt.Sprintf("Detected %s technology stack pattern", stack),
		Confidence:  confidence * 0.7, // Reduce confidence as this is pattern-based
		Severity:    SeverityInfo,
		Metadata: map[string]interface{}{
			"stack":        stack,
			"technologies": technologies,
			"match_ratio":  confidence,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (l *LanguageAnalyzer) countLanguages(config AnalysisConfig) int {
	languages := make(map[string]bool)
	languageMap := map[string]string{
		".go": "Go", ".py": "Python", ".js": "JavaScript", ".ts": "TypeScript",
		".java": "Java", ".c": "C", ".cpp": "C++", ".cs": "C#", ".rb": "Ruby",
		".php": "PHP", ".rs": "Rust", ".swift": "Swift", ".kt": "Kotlin",
	}

	for _, file := range config.RepoData.Files {
		ext := strings.ToLower(filepath.Ext(file.Path))
		if language, exists := languageMap[ext]; exists {
			languages[language] = true
		}
	}

	return len(languages)
}

func (l *LanguageAnalyzer) countFrameworks(result *EngineAnalysisResult) int {
	count := 0
	for _, finding := range result.Findings {
		if finding.Type == FindingTypeFramework {
			count++
		}
	}
	return count
}
