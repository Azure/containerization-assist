package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Language detection implementations

// detectLanguageByExtension detects language based on file extensions
func (cmd *ConsolidatedAnalyzeCommand) detectLanguageByExtension(ctx context.Context, sessionID string, languageMap map[string]int) error {
	extensionMap := map[string]string{
		".go":         "go",
		".js":         "javascript",
		".ts":         "typescript",
		".py":         "python",
		".java":       "java",
		".cs":         "csharp",
		".cpp":        "cpp",
		".c":          "c",
		".rb":         "ruby",
		".php":        "php",
		".rs":         "rust",
		".kt":         "kotlin",
		".swift":      "swift",
		".scala":      "scala",
		".sh":         "shell",
		".ps1":        "powershell",
		".yaml":       "yaml",
		".yml":        "yaml",
		".json":       "json",
		".xml":        "xml",
		".html":       "html",
		".css":        "css",
		".scss":       "scss",
		".sass":       "sass",
		".less":       "less",
		".sql":        "sql",
		".md":         "markdown",
		".dockerfile": "dockerfile",
		".Dockerfile": "dockerfile",
	}

	// Use FileAccessService to search for all files
	files, err := cmd.fileAccess.SearchFiles(ctx, sessionID, "*")
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeIO).
			Messagef("failed to search files: %w", err).
			WithLocation().
			Build()
	}

	for _, file := range files {
		// Skip directories
		if file.IsDir {
			continue
		}

		ext := filepath.Ext(file.Name)
		if lang, exists := extensionMap[ext]; exists {
			languageMap[lang]++
		}

		// Special cases for files without extensions
		fileName := file.Name
		if fileName == "Dockerfile" || fileName == "dockerfile" {
			languageMap["dockerfile"]++
		} else if fileName == "Makefile" || fileName == "makefile" {
			languageMap["make"]++
		} else if fileName == "Gemfile" {
			languageMap["ruby"]++
		} else if fileName == "requirements.txt" || fileName == "setup.py" {
			languageMap["python"]++
		} else if fileName == "package.json" {
			languageMap["javascript"]++
		} else if fileName == "pom.xml" || fileName == "build.gradle" {
			languageMap["java"]++
		} else if fileName == "go.mod" || fileName == "go.sum" {
			languageMap["go"]++
		} else if fileName == "Cargo.toml" {
			languageMap["rust"]++
		}
	}

	return nil
}

// detectLanguageByContent detects language based on file content patterns
func (cmd *ConsolidatedAnalyzeCommand) detectLanguageByContent(ctx context.Context, sessionID string, languageMap map[string]int) error {
	patterns := map[string]*regexp.Regexp{
		"go":         regexp.MustCompile(`(?m)^package\s+\w+`),
		"javascript": regexp.MustCompile(`(?m)(require\(|import\s+.*from|export\s+.*=)`),
		"typescript": regexp.MustCompile(`(?m)(interface\s+\w+|type\s+\w+\s*=|import.*\.ts)`),
		"python":     regexp.MustCompile(`(?m)(import\s+\w+|from\s+\w+\s+import|def\s+\w+\()`),
		"java":       regexp.MustCompile(`(?m)(public\s+class|import\s+java\.)`),
		"csharp":     regexp.MustCompile(`(?m)(using\s+System|namespace\s+\w+|public\s+class)`),
	}

	// Get all files for content analysis
	files, err := cmd.fileAccess.SearchFiles(ctx, sessionID, "*")
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeIO).
			Messagef("failed to search files: %w", err).
			WithLocation().
			Build()
	}

	for _, file := range files {
		// Skip directories
		if file.IsDir {
			continue
		}

		// Only check text files
		if !cmd.isTextFile(file.Path) {
			continue
		}

		// Read file content using FileAccessService
		content, err := cmd.fileAccess.ReadFile(ctx, sessionID, file.Path)
		if err != nil {
			continue // Skip files we can't read
		}

		for lang, pattern := range patterns {
			if pattern.MatchString(content) {
				languageMap[lang] += 2 // Weight content-based detection higher
			}
		}
	}

	return nil
}

// determinePrimaryLanguage determines the primary language from detection results
func (cmd *ConsolidatedAnalyzeCommand) determinePrimaryLanguage(languageMap map[string]int) (string, float64) {
	if len(languageMap) == 0 {
		return "unknown", 0.0
	}

	var maxLang string
	var maxCount int
	var totalCount int

	for lang, count := range languageMap {
		totalCount += count
		if count > maxCount {
			maxCount = count
			maxLang = lang
		}
	}

	confidence := float64(maxCount) / float64(totalCount)
	return maxLang, confidence
}

// calculateLanguagePercentage calculates the percentage of a language in the codebase
func (cmd *ConsolidatedAnalyzeCommand) calculateLanguagePercentage(language string, languageMap map[string]int) float64 {
	if len(languageMap) == 0 {
		return 0.0
	}

	totalCount := 0
	for _, count := range languageMap {
		totalCount += count
	}

	if totalCount == 0 {
		return 0.0
	}

	return float64(languageMap[language]) / float64(totalCount) * 100.0
}

// Framework detection implementations

// detectGoFramework detects Go frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectGoFramework(ctx context.Context, sessionID string, result *analyze.AnalysisResult) error {
	// Check if go.mod exists using FileAccessService
	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "go.mod")
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeIO).
			Messagef("failed to check go.mod existence: %w", err).
			WithLocation().
			Build()
	}

	if !exists {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	// Read go.mod content using FileAccessService
	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "go.mod")
	if err != nil {
		return errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read go.mod: %w", err).
			WithLocation().
			Build()
	}

	contentStr := content
	frameworks := []struct {
		name    string
		pattern string
		ftype   analyze.FrameworkType
	}{
		{"gin", "github.com/gin-gonic/gin", analyze.FrameworkTypeWeb},
		{"echo", "github.com/labstack/echo", analyze.FrameworkTypeWeb},
		{"fiber", "github.com/gofiber/fiber", analyze.FrameworkTypeWeb},
		{"chi", "github.com/go-chi/chi", analyze.FrameworkTypeWeb},
		{"gorilla", "github.com/gorilla/mux", analyze.FrameworkTypeWeb},
		{"beego", "github.com/beego/beego", analyze.FrameworkTypeWeb},
		{"revel", "github.com/revel/revel", analyze.FrameworkTypeWeb},
		{"gorm", "gorm.io/gorm", analyze.FrameworkTypeORM},
		{"xorm", "xorm.io/xorm", analyze.FrameworkTypeORM},
		{"cobra", "github.com/spf13/cobra", analyze.FrameworkTypeCLI},
		{"viper", "github.com/spf13/viper", analyze.FrameworkTypeConfig},
		{"testify", "github.com/stretchr/testify", analyze.FrameworkTypeTest},
	}

	for _, fw := range frameworks {
		if strings.Contains(contentStr, fw.pattern) {
			result.Framework = analyze.Framework{
				Name:       fw.name,
				Type:       fw.ftype,
				Confidence: analyze.ConfidenceHigh,
			}
			return nil
		}
	}

	result.Framework = analyze.Framework{
		Name:       "standard",
		Type:       analyze.FrameworkTypeStandard,
		Confidence: analyze.ConfidenceHigh,
	}

	return nil
}

// detectJSFramework detects JavaScript/TypeScript frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectJSFramework(ctx context.Context, sessionID string, result *analyze.AnalysisResult) error {
	// Check if package.json exists using FileAccessService
	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "package.json")
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeIO).
			Messagef("failed to check package.json existence: %w", err).
			WithLocation().
			Build()
	}

	if !exists {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	// Read package.json content using FileAccessService
	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "package.json")
	if err != nil {
		return errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read package.json: %w", err).
			WithLocation().
			Build()
	}

	contentStr := content
	frameworks := []struct {
		name    string
		pattern string
		ftype   analyze.FrameworkType
	}{
		{"react", "\"react\":", analyze.FrameworkTypeWeb},
		{"vue", "\"vue\":", analyze.FrameworkTypeWeb},
		{"angular", "\"@angular/", analyze.FrameworkTypeWeb},
		{"svelte", "\"svelte\":", analyze.FrameworkTypeWeb},
		{"express", "\"express\":", analyze.FrameworkTypeWeb},
		{"fastify", "\"fastify\":", analyze.FrameworkTypeWeb},
		{"koa", "\"koa\":", analyze.FrameworkTypeWeb},
		{"next", "\"next\":", analyze.FrameworkTypeWeb},
		{"nuxt", "\"nuxt\":", analyze.FrameworkTypeWeb},
		{"gatsby", "\"gatsby\":", analyze.FrameworkTypeWeb},
		{"nestjs", "\"@nestjs/", analyze.FrameworkTypeWeb},
		{"electron", "\"electron\":", analyze.FrameworkTypeDesktop},
		{"jest", "\"jest\":", analyze.FrameworkTypeTest},
		{"mocha", "\"mocha\":", analyze.FrameworkTypeTest},
		{"cypress", "\"cypress\":", analyze.FrameworkTypeTest},
	}

	for _, fw := range frameworks {
		if strings.Contains(contentStr, fw.pattern) {
			result.Framework = analyze.Framework{
				Name:       fw.name,
				Type:       fw.ftype,
				Confidence: analyze.ConfidenceHigh,
			}
			return nil
		}
	}

	result.Framework = analyze.Framework{
		Name:       "nodejs",
		Type:       analyze.FrameworkTypeRuntime,
		Confidence: analyze.ConfidenceMedium,
	}

	return nil
}

// detectPythonFramework detects Python frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectPythonFramework(ctx context.Context, sessionID string, result *analyze.AnalysisResult) error {
	// Check requirements.txt, setup.py, pyproject.toml
	files := []string{"requirements.txt", "setup.py", "pyproject.toml", "Pipfile"}

	var content string
	for _, file := range files {
		// Check if file exists using FileAccessService
		exists, err := cmd.fileAccess.FileExists(ctx, sessionID, file)
		if err != nil {
			continue
		}
		if exists {
			// Read file content using FileAccessService
			fileContent, err := cmd.fileAccess.ReadFile(ctx, sessionID, file)
			if err != nil {
				continue
			}
			content += fileContent + "\n"
		}
	}

	if content == "" {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	frameworks := []struct {
		name    string
		pattern string
		ftype   analyze.FrameworkType
	}{
		{"django", "django", analyze.FrameworkTypeWeb},
		{"flask", "flask", analyze.FrameworkTypeWeb},
		{"fastapi", "fastapi", analyze.FrameworkTypeWeb},
		{"tornado", "tornado", analyze.FrameworkTypeWeb},
		{"bottle", "bottle", analyze.FrameworkTypeWeb},
		{"pyramid", "pyramid", analyze.FrameworkTypeWeb},
		{"pandas", "pandas", analyze.FrameworkTypeData},
		{"numpy", "numpy", analyze.FrameworkTypeData},
		{"scipy", "scipy", analyze.FrameworkTypeData},
		{"scikit-learn", "scikit-learn", analyze.FrameworkTypeML},
		{"tensorflow", "tensorflow", analyze.FrameworkTypeML},
		{"pytorch", "torch", analyze.FrameworkTypeML},
		{"pytest", "pytest", analyze.FrameworkTypeTest},
		{"unittest", "unittest", analyze.FrameworkTypeTest},
	}

	for _, fw := range frameworks {
		if strings.Contains(strings.ToLower(content), fw.pattern) {
			result.Framework = analyze.Framework{
				Name:       fw.name,
				Type:       fw.ftype,
				Confidence: analyze.ConfidenceHigh,
			}
			return nil
		}
	}

	result.Framework = analyze.Framework{
		Name:       "standard",
		Type:       analyze.FrameworkTypeStandard,
		Confidence: analyze.ConfidenceMedium,
	}

	return nil
}

// detectJavaFramework detects Java frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectJavaFramework(ctx context.Context, sessionID string, result *analyze.AnalysisResult) error {
	// Check pom.xml, build.gradle, build.gradle.kts
	files := []string{"pom.xml", "build.gradle", "build.gradle.kts"}

	var content string
	for _, file := range files {
		// Check if file exists using FileAccessService
		exists, err := cmd.fileAccess.FileExists(ctx, sessionID, file)
		if err != nil {
			continue
		}
		if exists {
			// Read file content using FileAccessService
			fileContent, err := cmd.fileAccess.ReadFile(ctx, sessionID, file)
			if err != nil {
				continue
			}
			content += fileContent + "\n"
		}
	}

	if content == "" {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	frameworks := []struct {
		name    string
		pattern string
		ftype   analyze.FrameworkType
	}{
		{"spring", "spring", analyze.FrameworkTypeWeb},
		{"springboot", "spring-boot", analyze.FrameworkTypeWeb},
		{"quarkus", "quarkus", analyze.FrameworkTypeWeb},
		{"micronaut", "micronaut", analyze.FrameworkTypeWeb},
		{"vertx", "vertx", analyze.FrameworkTypeWeb},
		{"jersey", "jersey", analyze.FrameworkTypeWeb},
		{"hibernate", "hibernate", analyze.FrameworkTypeORM},
		{"mybatis", "mybatis", analyze.FrameworkTypeORM},
		{"junit", "junit", analyze.FrameworkTypeTest},
		{"testng", "testng", analyze.FrameworkTypeTest},
		{"mockito", "mockito", analyze.FrameworkTypeTest},
	}

	for _, fw := range frameworks {
		if strings.Contains(strings.ToLower(content), fw.pattern) {
			result.Framework = analyze.Framework{
				Name:       fw.name,
				Type:       fw.ftype,
				Confidence: analyze.ConfidenceHigh,
			}
			return nil
		}
	}

	result.Framework = analyze.Framework{
		Name:       "standard",
		Type:       analyze.FrameworkTypeStandard,
		Confidence: analyze.ConfidenceMedium,
	}

	return nil
}

// detectDotNetFramework detects .NET frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectDotNetFramework(ctx context.Context, sessionID string, result *analyze.AnalysisResult) error {
	// Search for .csproj, .vbproj, .fsproj files using FileAccessService
	patterns := []string{"*.csproj", "*.vbproj", "*.fsproj"}
	var projectFiles []services.FileInfo

	for _, pattern := range patterns {
		files, err := cmd.fileAccess.SearchFiles(ctx, sessionID, pattern)
		if err != nil {
			continue
		}
		projectFiles = append(projectFiles, files...)
	}

	if len(projectFiles) == 0 {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	var content string
	for _, file := range projectFiles {
		// Read project file content using FileAccessService
		fileContent, err := cmd.fileAccess.ReadFile(ctx, sessionID, file.Path)
		if err != nil {
			continue
		}
		content += fileContent + "\n"
	}

	frameworks := []struct {
		name    string
		pattern string
		ftype   analyze.FrameworkType
	}{
		{"aspnet", "Microsoft.AspNetCore", analyze.FrameworkTypeWeb},
		{"blazor", "Microsoft.AspNetCore.Blazor", analyze.FrameworkTypeWeb},
		{"mvc", "Microsoft.AspNetCore.Mvc", analyze.FrameworkTypeWeb},
		{"webapi", "Microsoft.AspNetCore.WebApi", analyze.FrameworkTypeWeb},
		{"entityframework", "Microsoft.EntityFrameworkCore", analyze.FrameworkTypeORM},
		{"wpf", "Microsoft.WindowsDesktop.App", analyze.FrameworkTypeDesktop},
		{"winforms", "System.Windows.Forms", analyze.FrameworkTypeDesktop},
		{"xamarin", "Xamarin", analyze.FrameworkTypeMobile},
		{"maui", "Microsoft.Maui", analyze.FrameworkTypeMobile},
		{"xunit", "xunit", analyze.FrameworkTypeTest},
		{"nunit", "NUnit", analyze.FrameworkTypeTest},
		{"mstest", "MSTest", analyze.FrameworkTypeTest},
	}

	for _, fw := range frameworks {
		if strings.Contains(content, fw.pattern) {
			result.Framework = analyze.Framework{
				Name:       fw.name,
				Type:       fw.ftype,
				Confidence: analyze.ConfidenceHigh,
			}
			return nil
		}
	}

	result.Framework = analyze.Framework{
		Name:       "dotnet",
		Type:       analyze.FrameworkTypeRuntime,
		Confidence: analyze.ConfidenceMedium,
	}

	return nil
}

// Dependency analysis implementations

// analyzeGoDependencies analyzes Go dependencies
func (cmd *ConsolidatedAnalyzeCommand) analyzeGoDependencies(ctx context.Context, sessionID string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency

	// Check if go.mod exists using FileAccessService
	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "go.mod")
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeIO).
			Messagef("failed to check go.mod existence: %w", err).
			WithLocation().
			Build()
	}

	if !exists {
		return dependencies, nil
	}

	// Read go.mod content using FileAccessService
	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "go.mod")
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read go.mod: %w", err).
			WithLocation().
			Build()
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	inRequire := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "require (" {
			inRequire = true
			continue
		}

		if line == ")" {
			inRequire = false
			continue
		}

		if inRequire || strings.HasPrefix(line, "require ") {
			// Parse dependency line
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[0]
				if strings.HasPrefix(name, "require") {
					if len(parts) >= 3 {
						name = parts[1]
					} else {
						continue
					}
				}

				version := parts[1]
				if len(parts) >= 3 {
					version = parts[2]
				}

				dependencies = append(dependencies, analyze.Dependency{
					Name:    name,
					Version: version,
					Type:    analyze.DependencyTypeDirect,
					Source:  "go.mod",
				})
			}
		}
	}

	return dependencies, nil
}

// analyzeNodeDependencies analyzes Node.js dependencies
func (cmd *ConsolidatedAnalyzeCommand) analyzeNodeDependencies(ctx context.Context, sessionID string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency

	// Check if package.json exists
	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "package.json")
	if err != nil {
		return nil, err
	}
	if !exists {
		return dependencies, nil
	}

	// Read package.json content using FileAccessService
	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "package.json")
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read package.json: %w", err).
			WithLocation().
			Build()
	}

	contentStr := content

	// Simple regex-based parsing for dependencies
	depPattern := regexp.MustCompile(`"([^"]+)":\s*"([^"]+)"`)
	matches := depPattern.FindAllStringSubmatch(contentStr, -1)

	for _, match := range matches {
		if len(match) == 3 {
			name := match[1]
			version := match[2]

			// Skip non-dependency entries
			if name == "name" || name == "version" || name == "description" || name == "main" || name == "scripts" {
				continue
			}

			dependencies = append(dependencies, analyze.Dependency{
				Name:    name,
				Version: version,
				Type:    analyze.DependencyTypeDirect,
				Source:  "package.json",
			})
		}
	}

	return dependencies, nil
}

// analyzePythonDependencies analyzes Python dependencies
func (cmd *ConsolidatedAnalyzeCommand) analyzePythonDependencies(ctx context.Context, sessionID string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency

	// Check if requirements.txt exists
	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "requirements.txt")
	if err != nil {
		return nil, err
	}
	if !exists {
		return dependencies, nil
	}

	// Read requirements.txt content using FileAccessService
	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "requirements.txt")
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read requirements.txt: %w", err).
			WithLocation().
			Build()
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse requirement line (package==version or package>=version)
		parts := regexp.MustCompile(`[>=<!=]+`).Split(line, 2)
		if len(parts) >= 1 {
			name := strings.TrimSpace(parts[0])
			version := ""
			if len(parts) >= 2 {
				version = strings.TrimSpace(parts[1])
			}

			dependencies = append(dependencies, analyze.Dependency{
				Name:    name,
				Version: version,
				Type:    analyze.DependencyTypeDirect,
				Source:  "requirements.txt",
			})
		}
	}

	return dependencies, nil
}

// analyzeJavaDependencies analyzes Java dependencies
func (cmd *ConsolidatedAnalyzeCommand) analyzeJavaDependencies(ctx context.Context, sessionID string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency

	// Check for Maven dependencies (pom.xml)
	pomExists, err := cmd.fileAccess.FileExists(ctx, sessionID, "pom.xml")
	if err != nil {
		return nil, err
	}
	if pomExists {
		deps, err := cmd.parseMavenDependencies(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, deps...)
	}

	// Check for Gradle dependencies (build.gradle)
	gradleExists, err := cmd.fileAccess.FileExists(ctx, sessionID, "build.gradle")
	if err != nil {
		return nil, err
	}
	if gradleExists {
		deps, err := cmd.parseGradleDependencies(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, deps...)
	}

	return dependencies, nil
}

// parseMavenDependencies parses Maven dependencies from pom.xml
func (cmd *ConsolidatedAnalyzeCommand) parseMavenDependencies(ctx context.Context, sessionID string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency

	// Read pom.xml content using FileAccessService
	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "pom.xml")
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read pom.xml: %w", err).
			WithLocation().
			Build()
	}

	// Simple regex-based parsing - in a real system you'd use XML parser
	depPattern := regexp.MustCompile(`<groupId>([^<]+)</groupId>\s*<artifactId>([^<]+)</artifactId>\s*<version>([^<]+)</version>`)
	matches := depPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) == 4 {
			groupId := match[1]
			artifactId := match[2]
			version := match[3]

			dependencies = append(dependencies, analyze.Dependency{
				Name:    fmt.Sprintf("%s:%s", groupId, artifactId),
				Version: version,
				Type:    analyze.DependencyTypeDirect,
				Source:  "pom.xml",
			})
		}
	}

	return dependencies, nil
}

// parseGradleDependencies parses Gradle dependencies from build.gradle
func (cmd *ConsolidatedAnalyzeCommand) parseGradleDependencies(ctx context.Context, sessionID string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency

	// Read build.gradle content using FileAccessService
	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "build.gradle")
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read build.gradle: %w", err).
			WithLocation().
			Build()
	}

	// Simple regex-based parsing for Gradle dependencies
	depPattern := regexp.MustCompile(`(?:implementation|compile|testImplementation|testCompile)\s+['"]([^'"]+)['"]`)
	matches := depPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) == 2 {
			depString := match[1]
			parts := strings.Split(depString, ":")

			if len(parts) >= 2 {
				name := strings.Join(parts[:2], ":")
				version := ""
				if len(parts) >= 3 {
					version = parts[2]
				}

				dependencies = append(dependencies, analyze.Dependency{
					Name:    name,
					Version: version,
					Type:    analyze.DependencyTypeDirect,
					Source:  "build.gradle",
				})
			}
		}
	}

	return dependencies, nil
}

// Database detection implementations

// analyzeDatabases detects database usage and configuration
func (cmd *ConsolidatedAnalyzeCommand) analyzeDatabases(ctx context.Context, sessionID string, result *analyze.AnalysisResult) error {
	var databases []analyze.Database

	// Database patterns to search for
	dbPatterns := map[string]*regexp.Regexp{
		"PostgreSQL":    regexp.MustCompile(`postgres://|postgresql://|host.*dbname.*user|DATABASE_URL.*postgres`),
		"MySQL":         regexp.MustCompile(`mysql://|jdbc:mysql|host.*database.*username|DATABASE_URL.*mysql`),
		"MongoDB":       regexp.MustCompile(`mongodb://|mongodb\+srv://|mongoose\.connect|MongoClient`),
		"Redis":         regexp.MustCompile(`redis://|redis-server|RedisClient|redis\.createClient`),
		"SQLite":        regexp.MustCompile(`sqlite://|\.db$|\.sqlite$|sqlite3|database\.sqlite`),
		"Oracle":        regexp.MustCompile(`oracle://|jdbc:oracle|OracleClient`),
		"Cassandra":     regexp.MustCompile(`cassandra://|CassandraClient|contact_points`),
		"DynamoDB":      regexp.MustCompile(`dynamodb|aws-sdk.*dynamodb|DynamoDBClient`),
		"Elasticsearch": regexp.MustCompile(`elasticsearch://|ElasticsearchClient|@elastic/elasticsearch`),
	}

	// Check dependency files for database connections
	databases = append(databases, cmd.detectDatabasesFromDependencies(ctx, sessionID, result.Language.Name)...)

	// Check configuration files for database strings
	databases = append(databases, cmd.detectDatabasesFromConfig(ctx, sessionID, dbPatterns)...)

	// Check source code for database usage
	databases = append(databases, cmd.detectDatabasesFromSource(ctx, sessionID, dbPatterns)...)

	result.Databases = databases
	return nil
}

// detectDatabasesFromDependencies detects databases from dependency files
func (cmd *ConsolidatedAnalyzeCommand) detectDatabasesFromDependencies(ctx context.Context, sessionID, language string) []analyze.Database {
	var databases []analyze.Database

	switch language {
	case "javascript", "typescript":
		databases = append(databases, cmd.detectNodeDatabases(ctx, sessionID)...)
	case "python":
		databases = append(databases, cmd.detectPythonDatabases(ctx, sessionID)...)
	case "go":
		databases = append(databases, cmd.detectGoDatabases(ctx, sessionID)...)
	case "java":
		databases = append(databases, cmd.detectJavaDatabases(ctx, sessionID)...)
	}

	return databases
}

// detectNodeDatabases detects Node.js database dependencies
func (cmd *ConsolidatedAnalyzeCommand) detectNodeDatabases(ctx context.Context, sessionID string) []analyze.Database {
	var databases []analyze.Database

	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "package.json")
	if err != nil || !exists {
		return databases
	}

	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "package.json")
	if err != nil {
		return databases
	}

	// Database dependency mapping
	dbMapping := map[string]analyze.Database{
		"pg":             {Type: analyze.DatabaseTypePostgreSQL, Name: "pg", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"postgres":       {Type: analyze.DatabaseTypePostgreSQL, Name: "postgres", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"mysql":          {Type: analyze.DatabaseTypeMySQL, Name: "mysql", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"mysql2":         {Type: analyze.DatabaseTypeMySQL, Name: "mysql2", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"mongoose":       {Type: analyze.DatabaseTypeMongoDB, Name: "mongoose", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"mongodb":        {Type: analyze.DatabaseTypeMongoDB, Name: "mongodb", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"redis":          {Type: analyze.DatabaseTypeRedis, Name: "redis", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"ioredis":        {Type: analyze.DatabaseTypeRedis, Name: "ioredis", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"sqlite3":        {Type: analyze.DatabaseTypeSQLite, Name: "sqlite3", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"better-sqlite3": {Type: analyze.DatabaseTypeSQLite, Name: "better-sqlite3", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"prisma":         {Type: analyze.DatabaseTypePostgreSQL, Name: "prisma", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"typeorm":        {Type: analyze.DatabaseTypePostgreSQL, Name: "typeorm", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
		"sequelize":      {Type: analyze.DatabaseTypePostgreSQL, Name: "sequelize", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: "package.json", Type: "dependency"}}},
	}

	for depName, db := range dbMapping {
		if strings.Contains(content, fmt.Sprintf("\"%s\"", depName)) {
			databases = append(databases, db)
		}
	}

	return databases
}

// detectPythonDatabases detects Python database dependencies
func (cmd *ConsolidatedAnalyzeCommand) detectPythonDatabases(ctx context.Context, sessionID string) []analyze.Database {
	var databases []analyze.Database

	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "requirements.txt")
	if err != nil || !exists {
		return databases
	}

	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "requirements.txt")
	if err != nil {
		return databases
	}

	// Python database package mapping
	dbMapping := map[string]analyze.Database{
		"psycopg2":        {Type: analyze.DatabaseTypePostgreSQL, Name: "psycopg2", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
		"asyncpg":         {Type: analyze.DatabaseTypePostgreSQL, Name: "asyncpg", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
		"PyMySQL":         {Type: analyze.DatabaseTypeMySQL, Name: "PyMySQL", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
		"mysql-connector": {Type: analyze.DatabaseTypeMySQL, Name: "mysql-connector", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
		"pymongo":         {Type: analyze.DatabaseTypeMongoDB, Name: "pymongo", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
		"redis":           {Type: analyze.DatabaseTypeRedis, Name: "redis", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
		"sqlalchemy":      {Type: analyze.DatabaseTypePostgreSQL, Name: "sqlalchemy", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
		"django":          {Type: analyze.DatabaseTypePostgreSQL, Name: "django", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: "requirements.txt", Type: "dependency"}}},
	}

	for depName, db := range dbMapping {
		if strings.Contains(content, depName) {
			databases = append(databases, db)
		}
	}

	return databases
}

// detectGoDatabases detects Go database dependencies
func (cmd *ConsolidatedAnalyzeCommand) detectGoDatabases(ctx context.Context, sessionID string) []analyze.Database {
	var databases []analyze.Database

	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "go.mod")
	if err != nil || !exists {
		return databases
	}

	content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "go.mod")
	if err != nil {
		return databases
	}

	// Go database package mapping
	dbMapping := map[string]analyze.Database{
		"github.com/lib/pq":              {Type: analyze.DatabaseTypePostgreSQL, Name: "pq", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "go.mod", Type: "dependency"}}},
		"github.com/jackc/pgx":           {Type: analyze.DatabaseTypePostgreSQL, Name: "pgx", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "go.mod", Type: "dependency"}}},
		"github.com/go-sql-driver/mysql": {Type: analyze.DatabaseTypeMySQL, Name: "mysql", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "go.mod", Type: "dependency"}}},
		"go.mongodb.org/mongo-driver":    {Type: analyze.DatabaseTypeMongoDB, Name: "mongo-driver", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "go.mod", Type: "dependency"}}},
		"github.com/go-redis/redis":      {Type: analyze.DatabaseTypeRedis, Name: "redis", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: "go.mod", Type: "dependency"}}},
		"gorm.io/gorm":                   {Type: analyze.DatabaseTypePostgreSQL, Name: "gorm", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: "go.mod", Type: "dependency"}}},
		"github.com/jmoiron/sqlx":        {Type: analyze.DatabaseTypePostgreSQL, Name: "sqlx", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: "go.mod", Type: "dependency"}}},
	}

	for depName, db := range dbMapping {
		if strings.Contains(content, depName) {
			databases = append(databases, db)
		}
	}

	return databases
}

// detectJavaDatabases detects Java database dependencies
func (cmd *ConsolidatedAnalyzeCommand) detectJavaDatabases(ctx context.Context, sessionID string) []analyze.Database {
	var databases []analyze.Database

	// Check pom.xml
	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "pom.xml")
	if err == nil && exists {
		content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "pom.xml")
		if err == nil {
			databases = append(databases, cmd.parseJavaDbDependencies(content, "pom.xml")...)
		}
	}

	// Check build.gradle
	exists, err = cmd.fileAccess.FileExists(ctx, sessionID, "build.gradle")
	if err == nil && exists {
		content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "build.gradle")
		if err == nil {
			databases = append(databases, cmd.parseJavaDbDependencies(content, "build.gradle")...)
		}
	}

	return databases
}

// parseJavaDbDependencies parses Java database dependencies from build files
func (cmd *ConsolidatedAnalyzeCommand) parseJavaDbDependencies(content, source string) []analyze.Database {
	var databases []analyze.Database

	// Java database artifact mapping
	dbMapping := map[string]analyze.Database{
		"postgresql":                   {Type: analyze.DatabaseTypePostgreSQL, Name: "postgresql", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: source, Type: "dependency"}}},
		"mysql-connector-java":         {Type: analyze.DatabaseTypeMySQL, Name: "mysql-connector", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: source, Type: "dependency"}}},
		"mongo-java-driver":            {Type: analyze.DatabaseTypeMongoDB, Name: "mongo-java-driver", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: source, Type: "dependency"}}},
		"jedis":                        {Type: analyze.DatabaseTypeRedis, Name: "jedis", Confidence: analyze.ConfidenceHigh, Evidence: []analyze.Evidence{{Source: source, Type: "dependency"}}},
		"spring-boot-starter-data-jpa": {Type: analyze.DatabaseTypePostgreSQL, Name: "spring-data-jpa", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: source, Type: "dependency"}}},
		"hibernate-core":               {Type: analyze.DatabaseTypePostgreSQL, Name: "hibernate", Confidence: analyze.ConfidenceMedium, Evidence: []analyze.Evidence{{Source: source, Type: "dependency"}}},
	}

	for artifact, db := range dbMapping {
		if strings.Contains(content, artifact) {
			databases = append(databases, db)
		}
	}

	return databases
}

// detectDatabasesFromConfig detects databases from configuration files
func (cmd *ConsolidatedAnalyzeCommand) detectDatabasesFromConfig(ctx context.Context, sessionID string, patterns map[string]*regexp.Regexp) []analyze.Database {
	var databases []analyze.Database

	// Configuration files to check
	configFiles := []string{
		".env",
		".env.local",
		".env.example",
		"config.json",
		"database.yml",
		"application.yml",
		"application.properties",
		"docker-compose.yml",
		"docker-compose.yaml",
	}

	for _, configFile := range configFiles {
		exists, err := cmd.fileAccess.FileExists(ctx, sessionID, configFile)
		if err != nil || !exists {
			continue
		}

		content, err := cmd.fileAccess.ReadFile(ctx, sessionID, configFile)
		if err != nil {
			continue
		}

		for dbType, pattern := range patterns {
			if pattern.MatchString(content) {
				// Map string to DatabaseType
				var dbTypeEnum analyze.DatabaseType
				switch strings.ToLower(dbType) {
				case "postgresql":
					dbTypeEnum = analyze.DatabaseTypePostgreSQL
				case "mysql":
					dbTypeEnum = analyze.DatabaseTypeMySQL
				case "mongodb":
					dbTypeEnum = analyze.DatabaseTypeMongoDB
				case "redis":
					dbTypeEnum = analyze.DatabaseTypeRedis
				case "sqlite":
					dbTypeEnum = analyze.DatabaseTypeSQLite
				case "oracle":
					dbTypeEnum = analyze.DatabaseTypeOracle
				case "cassandra":
					dbTypeEnum = analyze.DatabaseTypeCassandra
				case "elasticsearch":
					dbTypeEnum = analyze.DatabaseTypeElastic
				default:
					dbTypeEnum = analyze.DatabaseTypePostgreSQL // fallback
				}

				connStr := cmd.extractConnectionString(content, pattern)
				databases = append(databases, analyze.Database{
					Type:       dbTypeEnum,
					Name:       dbType,
					Confidence: analyze.ConfidenceMedium,
					Evidence: []analyze.Evidence{{
						Source:  configFile,
						Type:    analyze.EvidenceTypeConfiguration,
						Content: connStr,
					}},
				})
			}
		}
	}

	return databases
}

// detectDatabasesFromSource detects databases from source code files
func (cmd *ConsolidatedAnalyzeCommand) detectDatabasesFromSource(ctx context.Context, sessionID string, patterns map[string]*regexp.Regexp) []analyze.Database {
	var databases []analyze.Database

	// Search for database patterns in source files
	sourcePatterns := []string{"*.js", "*.ts", "*.py", "*.go", "*.java"}

	for _, pattern := range sourcePatterns {
		files, err := cmd.fileAccess.SearchFiles(ctx, sessionID, pattern)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir {
				continue
			}

			content, err := cmd.fileAccess.ReadFile(ctx, sessionID, file.Path)
			if err != nil {
				continue
			}

			for dbType, dbPattern := range patterns {
				if dbPattern.MatchString(content) {
					// Map string to DatabaseType
					var dbTypeEnum analyze.DatabaseType
					switch strings.ToLower(dbType) {
					case "postgresql":
						dbTypeEnum = analyze.DatabaseTypePostgreSQL
					case "mysql":
						dbTypeEnum = analyze.DatabaseTypeMySQL
					case "mongodb":
						dbTypeEnum = analyze.DatabaseTypeMongoDB
					case "redis":
						dbTypeEnum = analyze.DatabaseTypeRedis
					case "sqlite":
						dbTypeEnum = analyze.DatabaseTypeSQLite
					case "oracle":
						dbTypeEnum = analyze.DatabaseTypeOracle
					case "cassandra":
						dbTypeEnum = analyze.DatabaseTypeCassandra
					case "elasticsearch":
						dbTypeEnum = analyze.DatabaseTypeElastic
					default:
						dbTypeEnum = analyze.DatabaseTypePostgreSQL // fallback
					}

					databases = append(databases, analyze.Database{
						Type:       dbTypeEnum,
						Name:       dbType,
						Confidence: analyze.ConfidenceLow,
						Evidence: []analyze.Evidence{{
							Source: file.Path,
							Type:   analyze.EvidenceTypeContent,
						}},
					})
				}
			}
		}
	}

	return databases
}

// extractConnectionString attempts to extract database connection string
func (cmd *ConsolidatedAnalyzeCommand) extractConnectionString(content string, pattern *regexp.Regexp) string {
	matches := pattern.FindAllString(content, 1)
	if len(matches) > 0 {
		// Mask sensitive parts of connection string
		connStr := matches[0]
		// Simple masking - replace password with ***
		masked := regexp.MustCompile(`://([^:]+):([^@]+)@`).ReplaceAllString(connStr, "://$1:***@")
		return masked
	}
	return ""
}

// Port detection implementations

// analyzePorts detects port configuration from various sources
func (cmd *ConsolidatedAnalyzeCommand) analyzePorts(ctx context.Context, sessionID string, result *analyze.AnalysisResult) error {
	var ports []analyze.Port

	// Detect ports from different sources
	ports = append(ports, cmd.detectPortsFromConfig(ctx, sessionID)...)
	ports = append(ports, cmd.detectPortsFromSource(ctx, sessionID, result.Language.Name)...)
	ports = append(ports, cmd.detectPortsFromDocker(ctx, sessionID)...)

	// Deduplicate ports
	uniquePorts := make(map[int]analyze.Port)
	for _, port := range ports {
		if existing, exists := uniquePorts[port.Number]; exists {
			// Merge sources
			existing.Sources = append(existing.Sources, port.Sources...)
			uniquePorts[port.Number] = existing
		} else {
			uniquePorts[port.Number] = port
		}
	}

	// Convert back to slice
	result.Ports = make([]analyze.Port, 0, len(uniquePorts))
	for _, port := range uniquePorts {
		result.Ports = append(result.Ports, port)
	}

	return nil
}

// detectPortsFromConfig detects ports from configuration files
func (cmd *ConsolidatedAnalyzeCommand) detectPortsFromConfig(ctx context.Context, sessionID string) []analyze.Port {
	var ports []analyze.Port

	// Configuration files to check
	configFiles := []string{
		".env",
		".env.local",
		"config.json",
		"package.json",
		"application.yml",
		"application.properties",
		"docker-compose.yml",
		"docker-compose.yaml",
	}

	// Port patterns
	portPatterns := []*regexp.Regexp{
		regexp.MustCompile(`PORT\s*=\s*(\d+)`),
		regexp.MustCompile(`port\s*:\s*(\d+)`),
		regexp.MustCompile(`"port"\s*:\s*(\d+)`),
		regexp.MustCompile(`listen\s*:\s*(\d+)`),
		regexp.MustCompile(`server\.port\s*=\s*(\d+)`),
		regexp.MustCompile(`-\s*"(\d+):\d+"`), // Docker port mapping
	}

	for _, configFile := range configFiles {
		exists, err := cmd.fileAccess.FileExists(ctx, sessionID, configFile)
		if err != nil || !exists {
			continue
		}

		content, err := cmd.fileAccess.ReadFile(ctx, sessionID, configFile)
		if err != nil {
			continue
		}

		for _, pattern := range portPatterns {
			matches := pattern.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					if portNum := cmd.parsePort(match[1]); portNum > 0 {
						ports = append(ports, analyze.Port{
							Number:  portNum,
							Type:    cmd.detectPortType(portNum),
							Sources: []string{configFile},
						})
					}
				}
			}
		}
	}

	return ports
}

// detectPortsFromSource detects ports from source code
func (cmd *ConsolidatedAnalyzeCommand) detectPortsFromSource(ctx context.Context, sessionID, language string) []analyze.Port {
	var ports []analyze.Port

	// Language-specific port detection patterns
	var patterns []*regexp.Regexp
	var filePattern string

	switch language {
	case "javascript", "typescript":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`listen\(\s*(\d+)`),
			regexp.MustCompile(`\.listen\(\s*process\.env\.PORT\s*\|\|\s*(\d+)`),
			regexp.MustCompile(`port:\s*(\d+)`),
		}
		filePattern = "*.js"
	case "python":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`app\.run\([^)]*port\s*=\s*(\d+)`),
			regexp.MustCompile(`listen\(\s*(\d+)`),
			regexp.MustCompile(`bind\s*=\s*.*:(\d+)`),
		}
		filePattern = "*.py"
	case "go":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`ListenAndServe\(\s*":(\d+)"`),
			regexp.MustCompile(`Addr:\s*":(\d+)"`),
		}
		filePattern = "*.go"
	case "java":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`server\.port\s*=\s*(\d+)`),
			regexp.MustCompile(`@Value\("\$\{server\.port:(\d+)\}"\)`),
		}
		filePattern = "*.java"
	default:
		return ports
	}

	files, err := cmd.fileAccess.SearchFiles(ctx, sessionID, filePattern)
	if err != nil {
		return ports
	}

	for _, file := range files {
		if file.IsDir {
			continue
		}

		content, err := cmd.fileAccess.ReadFile(ctx, sessionID, file.Path)
		if err != nil {
			continue
		}

		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					if portNum := cmd.parsePort(match[1]); portNum > 0 {
						ports = append(ports, analyze.Port{
							Number:  portNum,
							Type:    cmd.detectPortType(portNum),
							Sources: []string{file.Path},
						})
					}
				}
			}
		}
	}

	return ports
}

// detectPortsFromDocker detects ports from Docker files
func (cmd *ConsolidatedAnalyzeCommand) detectPortsFromDocker(ctx context.Context, sessionID string) []analyze.Port {
	var ports []analyze.Port

	// Check Dockerfile
	exists, err := cmd.fileAccess.FileExists(ctx, sessionID, "Dockerfile")
	if err == nil && exists {
		content, err := cmd.fileAccess.ReadFile(ctx, sessionID, "Dockerfile")
		if err == nil {
			exposePattern := regexp.MustCompile(`EXPOSE\s+(\d+)`)
			matches := exposePattern.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					if portNum := cmd.parsePort(match[1]); portNum > 0 {
						ports = append(ports, analyze.Port{
							Number:  portNum,
							Type:    cmd.detectPortType(portNum),
							Sources: []string{"Dockerfile"},
						})
					}
				}
			}
		}
	}

	return ports
}

// Helper methods for port and database detection

func (cmd *ConsolidatedAnalyzeCommand) parsePort(portStr string) int {
	if portNum, err := strconv.Atoi(portStr); err == nil && portNum > 0 && portNum <= 65535 {
		return portNum
	}
	return 0
}

func (cmd *ConsolidatedAnalyzeCommand) detectPortType(port int) string {
	switch {
	case port == 80 || port == 8080:
		return "HTTP"
	case port == 443 || port == 8443:
		return "HTTPS"
	case port == 3000:
		return "Development"
	case port == 5000:
		return "Flask/Development"
	case port == 8000:
		return "Django/Development"
	case port == 9000:
		return "Application"
	case port >= 3000 && port <= 9999:
		return "Application"
	default:
		return "Custom"
	}
}

// Additional analysis implementations

// analyzeSecrets performs secrets analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeSecrets(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Implement secrets analysis using patterns
	secretPatterns := map[string]*regexp.Regexp{
		"AWS_ACCESS_KEY": regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"AWS_SECRET_KEY": regexp.MustCompile(`[0-9a-zA-Z/+]{40}`),
		"GITHUB_TOKEN":   regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36}`),
		"PRIVATE_KEY":    regexp.MustCompile(`-----BEGIN [A-Z ]+PRIVATE KEY-----`),
		"API_KEY":        regexp.MustCompile(`[aA][pP][iI]_?[kK][eE][yY].*['\"'][0-9a-zA-Z]{32,45}['\"']`),
		"PASSWORD":       regexp.MustCompile(`[pP][aA][sS][sS][wW][oO][rR][dD].*['\"'][^'\"]{8,}['\"']`),
		"DATABASE_URL":   regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s]*`),
		"JWT_SECRET":     regexp.MustCompile(`[jJ][wW][tT].*['\"'][A-Za-z0-9_-]{20,}['\"']`),
	}

	var secretsFound []analyze.SecurityIssue

	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip binary files and certain directories
		if !cmd.isTextFile(path) || cmd.shouldSkipFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)

		for secretType, pattern := range secretPatterns {
			matches := pattern.FindAllStringIndex(contentStr, -1)
			for _, match := range matches {
				// Find line number
				lineNum := cmd.findLineNumber(contentStr, match[0])

				secretsFound = append(secretsFound, analyze.SecurityIssue{
					Type:        analyze.SecurityIssueTypeSecret,
					Severity:    analyze.SeverityHigh,
					Title:       fmt.Sprintf("Potential %s found", secretType),
					Description: fmt.Sprintf("Potential secret detected in %s", path),
					File:        path,
					Line:        lineNum,
					// Rule: secretType (stored in Type field)
				})
			}
		}

		return nil
	})

	if err != nil {
		return errors.NewError().
			Code(errors.CodeSecurityViolation).
			Type(errors.ErrTypeSecurity).
			Messagef("secrets analysis failed: %w", err).
			WithLocation().
			Build()
	}

	result.SecurityIssues = append(result.SecurityIssues, secretsFound...)
	return nil
}

// analyzeVulnerabilities performs vulnerability analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeVulnerabilities(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Implement basic vulnerability analysis
	// This would typically integrate with vulnerability databases

	var vulns []analyze.SecurityIssue

	// Check for known vulnerable patterns
	vulnPatterns := map[string]struct {
		pattern *regexp.Regexp
		desc    string
	}{
		"SQL_INJECTION": {
			regexp.MustCompile(`(SELECT|INSERT|UPDATE|DELETE).*\+.*['"]`),
			"Potential SQL injection vulnerability",
		},
		"XSS": {
			regexp.MustCompile(`innerHTML\s*=\s*[^'"]`),
			"Potential XSS vulnerability",
		},
		"HARDCODED_CRYPTO": {
			regexp.MustCompile(`(DES|MD5|SHA1)\s*\(`),
			"Use of weak cryptographic algorithm",
		},
	}

	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !cmd.isTextFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)

		for vulnType, vuln := range vulnPatterns {
			if vuln.pattern.MatchString(contentStr) {
				vulns = append(vulns, analyze.SecurityIssue{
					Type:        analyze.SecurityIssueTypeVulnerability,
					Severity:    analyze.SeverityMedium,
					Title:       vulnType,
					Description: vuln.desc,
					File:        path,
					// Rule: vulnType (stored in Type field)
				})
			}
		}

		return nil
	})

	if err != nil {
		return errors.NewError().
			Code(errors.CodeSecurityViolation).
			Type(errors.ErrTypeSecurity).
			Messagef("vulnerability analysis failed: %w", err).
			WithLocation().
			Build()
	}

	result.SecurityIssues = append(result.SecurityIssues, vulns...)
	return nil
}

// analyzeCompliance performs compliance analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeCompliance(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Implement compliance checks (OWASP, NIST, etc.)
	// This is a simplified implementation

	complianceItems := []analyze.SecurityIssue{}

	// Check for license files
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING"}
	hasLicense := false
	for _, file := range licenseFiles {
		if fileExists(filepath.Join(workspaceDir, file)) {
			hasLicense = true
			break
		}
	}

	if !hasLicense {
		complianceItems = append(complianceItems, analyze.SecurityIssue{
			Type:        analyze.SecurityIssueTypeCompliance,
			Severity:    analyze.SeverityMedium,
			Title:       "Missing License File",
			Description: "Repository should include a license file",
			// Rule: "LICENSE_REQUIRED" (compliance check)
		})
	}

	// Check for security policy
	securityFiles := []string{"SECURITY.md", "SECURITY.txt", ".github/SECURITY.md"}
	hasSecurity := false
	for _, file := range securityFiles {
		if fileExists(filepath.Join(workspaceDir, file)) {
			hasSecurity = true
			break
		}
	}

	if !hasSecurity {
		complianceItems = append(complianceItems, analyze.SecurityIssue{
			Type:        analyze.SecurityIssueTypeCompliance,
			Severity:    analyze.SeverityLow,
			Title:       "Missing Security Policy",
			Description: "Repository should include a security policy",
			// Rule: "SECURITY_POLICY_REQUIRED" (compliance check)
		})
	}

	result.SecurityIssues = append(result.SecurityIssues, complianceItems...)
	return nil
}

// analyzeTests performs test analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeTests(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Implement test analysis
	var testFrameworks []analyze.TestFramework

	// Language-specific test framework detection
	switch result.Language.Name {
	case "go":
		if cmd.hasGoTests(workspaceDir) {
			testFrameworks = append(testFrameworks, analyze.TestFramework{
				Name:       "go test",
				Type:       analyze.TestTypeUnit,
				Confidence: analyze.ConfidenceHigh,
			})
		}
	case "javascript", "typescript":
		testFrameworks = append(testFrameworks, cmd.detectJSTestFrameworks(workspaceDir)...)
	case "python":
		testFrameworks = append(testFrameworks, cmd.detectPythonTestFrameworks(workspaceDir)...)
	case "java":
		testFrameworks = append(testFrameworks, cmd.detectJavaTestFrameworks(workspaceDir)...)
	}

	result.TestFrameworks = testFrameworks
	return nil
}

// analyzeMetrics performs code metrics analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeMetrics(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Implement code metrics analysis
	// This would calculate complexity, maintainability index, etc.

	// For now, just add basic file metrics
	var totalFiles, totalLines int

	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !cmd.isSourceFile(path) {
			return nil
		}

		totalFiles++

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		totalLines += len(lines)

		return nil
	})

	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Messagef("metrics analysis failed: %w", err).
			WithLocation().
			Build()
	}

	// Store metrics in analysis metadata
	// Store metrics in options field
	result.AnalysisMetadata.Options = map[string]interface{}{
		"metrics": map[string]interface{}{
			"total_files": totalFiles,
			"total_lines": totalLines,
			"avg_lines_per_file": func() float64 {
				if totalFiles == 0 {
					return 0
				}
				return float64(totalLines) / float64(totalFiles)
			}(),
		},
	}

	return nil
}

// Dockerfile analysis implementations

// parseDockerfile parses a Dockerfile
func (cmd *ConsolidatedAnalyzeCommand) parseDockerfile(path string) (*DockerfileInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeIO).
			Messagef("failed to read Dockerfile: %w", err).
			WithLocation().
			Build()
	}

	dockerfile := &DockerfileInfo{
		Path:         path,
		Instructions: []DockerfileInstruction{},
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		instruction := DockerfileInstruction{
			Command: strings.ToUpper(parts[0]),
			Args:    parts[1],
			Line:    lineNum,
		}

		dockerfile.Instructions = append(dockerfile.Instructions, instruction)
	}

	return dockerfile, nil
}

// analyzeDockerfileSecurity analyzes Dockerfile for security issues
func (cmd *ConsolidatedAnalyzeCommand) analyzeDockerfileSecurity(dockerfile *DockerfileInfo) ([]analyze.SecurityIssue, error) {
	var issues []analyze.SecurityIssue

	for _, instruction := range dockerfile.Instructions {
		switch instruction.Command {
		case "USER":
			if instruction.Args == "root" || instruction.Args == "0" {
				issues = append(issues, analyze.SecurityIssue{
					Type:        analyze.SecurityTypePermission,
					Severity:    analyze.SeverityHigh,
					Title:       "Running as root user",
					Description: "Container should not run as root user",
					File:        dockerfile.Path,
					Line:        instruction.Line,
					// Rule: "DOCKERFILE_USER_ROOT" (security best practice)
				})
			}
		case "ADD":
			if strings.Contains(instruction.Args, "http://") {
				issues = append(issues, analyze.SecurityIssue{
					Type:        analyze.SecurityTypePermission,
					Severity:    analyze.SeverityMedium,
					Title:       "Using HTTP in ADD instruction",
					Description: "ADD instruction should use HTTPS instead of HTTP",
					File:        dockerfile.Path,
					Line:        instruction.Line,
					// Rule: "DOCKERFILE_ADD_HTTP" (security best practice)
				})
			}
		case "RUN":
			if strings.Contains(instruction.Args, "curl") && strings.Contains(instruction.Args, "sudo") {
				issues = append(issues, analyze.SecurityIssue{
					Type:        analyze.SecurityTypePermission,
					Severity:    analyze.SeverityMedium,
					Title:       "Using sudo in RUN instruction",
					Description: "Avoid using sudo in RUN instructions",
					File:        dockerfile.Path,
					Line:        instruction.Line,
					// Rule: "DOCKERFILE_RUN_SUDO" (security best practice)
				})
			}
		}
	}

	return issues, nil
}

// generateDockerfileRecommendations generates recommendations for Dockerfile
func (cmd *ConsolidatedAnalyzeCommand) generateDockerfileRecommendations(dockerfile *DockerfileInfo) ([]analyze.Recommendation, error) {
	var recommendations []analyze.Recommendation

	hasUser := false
	hasHealthcheck := false

	for _, instruction := range dockerfile.Instructions {
		switch instruction.Command {
		case "USER":
			hasUser = true
		case "HEALTHCHECK":
			hasHealthcheck = true
		}
	}

	if !hasUser {
		recommendations = append(recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypeSecurity,
			Priority:    analyze.PriorityHigh,
			Title:       "Add USER instruction",
			Description: "Add USER instruction to run container as non-root user",
			Action:      "Add 'USER <non-root-user>' instruction to Dockerfile",
			// Category: "dockerfile",
		})
	}

	if !hasHealthcheck {
		recommendations = append(recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypePerformance,
			Priority:    analyze.PriorityMedium,
			Title:       "Add HEALTHCHECK instruction",
			Description: "Add HEALTHCHECK instruction to monitor container health",
			Action:      "Add 'HEALTHCHECK' instruction to Dockerfile",
			// Category: "dockerfile",
		})
	}

	return recommendations, nil
}

// Final analysis methods

// calculateConfidence calculates overall analysis confidence
func (cmd *ConsolidatedAnalyzeCommand) calculateConfidence(result *analyze.AnalysisResult) {
	// Calculate confidence based on various factors
	factors := []float64{
		result.Language.Confidence,
		func() float64 {
			switch result.Framework.Confidence {
			case analyze.ConfidenceHigh:
				return 1.0
			case analyze.ConfidenceMedium:
				return 0.66
			case analyze.ConfidenceLow:
				return 0.33
			default:
				return 0.5
			}
		}(),
	}

	// Add factors for completeness
	if len(result.Dependencies) > 0 {
		factors = append(factors, 0.8)
	}
	if len(result.TestFrameworks) > 0 {
		factors = append(factors, 0.7)
	}
	if len(result.SecurityIssues) == 0 {
		factors = append(factors, 0.9)
	}

	// Calculate average
	sum := 0.0
	for _, factor := range factors {
		sum += factor
	}
	avg := sum / float64(len(factors))

	// Convert to confidence level
	if avg >= 0.8 {
		result.Confidence = analyze.ConfidenceHigh
	} else if avg >= 0.6 {
		result.Confidence = analyze.ConfidenceMedium
	} else {
		result.Confidence = analyze.ConfidenceLow
	}
}

// generateRecommendations generates recommendations based on analysis
func (cmd *ConsolidatedAnalyzeCommand) generateRecommendations(result *analyze.AnalysisResult) {
	// Generate recommendations based on analysis results

	// Language-specific recommendations
	if result.Language.Name == "go" && result.Framework.Name == "none" {
		result.Recommendations = append(result.Recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypeArchitecture,
			Priority:    analyze.PriorityMedium,
			Title:       "Consider using a Go web framework",
			Description: "For web applications, consider using Gin, Echo, or Fiber",
			Action:      "Add a web framework dependency to go.mod",
			// Category: "framework",
		})
	}

	// Security recommendations
	if len(result.SecurityIssues) > 0 {
		result.Recommendations = append(result.Recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypeSecurity,
			Priority:    analyze.PriorityHigh,
			Title:       "Address security issues",
			Description: fmt.Sprintf("Found %d security issues that should be addressed", len(result.SecurityIssues)),
			Action:      "Review and fix identified security issues",
			// Category: "security",
		})
	}

	// Testing recommendations
	if len(result.TestFrameworks) == 0 {
		result.Recommendations = append(result.Recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypeMaintenance,
			Priority:    analyze.PriorityMedium,
			Title:       "Add automated tests",
			Description: "Consider adding unit tests and integration tests",
			Action:      "Implement test coverage for critical functionality",
			// Category: "testing",
		})
	}
}

// Helper methods

// isTextFile checks if a file is a text file
func (cmd *ConsolidatedAnalyzeCommand) isTextFile(path string) bool {
	// Simple heuristic for text files
	ext := strings.ToLower(filepath.Ext(path))
	textExts := []string{".go", ".js", ".ts", ".py", ".java", ".cs", ".cpp", ".c", ".rb", ".php", ".rs", ".kt", ".swift", ".scala", ".sh", ".ps1", ".yaml", ".yml", ".json", ".xml", ".html", ".css", ".scss", ".sass", ".less", ".sql", ".md", ".txt", ".dockerfile", ".gitignore", ".gitattributes"}

	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}

	// Check for files without extension
	fileName := strings.ToLower(filepath.Base(path))
	specialFiles := []string{"dockerfile", "makefile", "gemfile", "rakefile", "requirements.txt", "setup.py", "package.json", "pom.xml", "build.gradle", "cargo.toml", "go.mod", "go.sum"}

	for _, specialFile := range specialFiles {
		if fileName == specialFile {
			return true
		}
	}

	return false
}

// isSourceFile checks if a file is a source code file
func (cmd *ConsolidatedAnalyzeCommand) isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	sourceExts := []string{".go", ".js", ".ts", ".py", ".java", ".cs", ".cpp", ".c", ".rb", ".php", ".rs", ".kt", ".swift", ".scala"}

	for _, sourceExt := range sourceExts {
		if ext == sourceExt {
			return true
		}
	}

	return false
}

// shouldSkipFile checks if a file should be skipped during analysis
func (cmd *ConsolidatedAnalyzeCommand) shouldSkipFile(path string) bool {
	skipPatterns := []string{
		"/.git/",
		"/node_modules/",
		"/vendor/",
		"/.vscode/",
		"/.idea/",
		"/target/",
		"/build/",
		"/dist/",
		"/.cache/",
		"/tmp/",
		"/temp/",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// findLineNumber finds the line number for a given character position
func (cmd *ConsolidatedAnalyzeCommand) findLineNumber(content string, pos int) int {
	lines := strings.Split(content[:pos], "\n")
	return len(lines)
}

// Note: fileExists is defined in common.go

// Note: Use slices.Contains from standard library

// Helper methods for test framework detection

// hasGoTests checks if Go tests exist
func (cmd *ConsolidatedAnalyzeCommand) hasGoTests(workspaceDir string) bool {
	var hasTests bool
	filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "_test.go") {
			hasTests = true
			return filepath.SkipDir
		}
		return nil
	})
	return hasTests
}

// calculateGoCoverage calculates Go test coverage (simplified)
func (cmd *ConsolidatedAnalyzeCommand) calculateGoCoverage(workspaceDir string) float64 {
	// This is a simplified implementation
	// In a real system, you'd run go test -cover
	return 0.0
}

// detectJSTestFrameworks detects JavaScript test frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectJSTestFrameworks(workspaceDir string) []analyze.TestFramework {
	var frameworks []analyze.TestFramework

	packageJSONPath := filepath.Join(workspaceDir, "package.json")
	if !fileExists(packageJSONPath) {
		return frameworks
	}

	content, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return frameworks
	}

	contentStr := string(content)

	testFrameworks := []struct {
		name    string
		pattern string
		ftype   analyze.TestType
	}{
		{"jest", "\"jest\":", analyze.TestTypeUnit},
		{"mocha", "\"mocha\":", analyze.TestTypeUnit},
		{"jasmine", "\"jasmine\":", analyze.TestTypeUnit},
		{"cypress", "\"cypress\":", analyze.TestTypeEnd2End},
		{"playwright", "\"playwright\":", analyze.TestTypeEnd2End},
		{"puppeteer", "\"puppeteer\":", analyze.TestTypeEnd2End},
	}

	for _, fw := range testFrameworks {
		if strings.Contains(contentStr, fw.pattern) {
			frameworks = append(frameworks, analyze.TestFramework{
				Name:       fw.name,
				Type:       fw.ftype,
				Confidence: analyze.ConfidenceMedium,
			})
		}
	}

	return frameworks
}

// detectPythonTestFrameworks detects Python test frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectPythonTestFrameworks(workspaceDir string) []analyze.TestFramework {
	var frameworks []analyze.TestFramework

	// Check requirements.txt
	reqPath := filepath.Join(workspaceDir, "requirements.txt")
	if fileExists(reqPath) {
		content, err := os.ReadFile(reqPath)
		if err == nil {
			contentStr := strings.ToLower(string(content))

			testFrameworks := []struct {
				name    string
				pattern string
				ftype   analyze.TestType
			}{
				{"pytest", "pytest", analyze.TestTypeUnit},
				{"unittest", "unittest", analyze.TestTypeUnit},
				{"nose", "nose", analyze.TestTypeUnit},
				{"selenium", "selenium", analyze.TestTypeEnd2End},
			}

			for _, fw := range testFrameworks {
				if strings.Contains(contentStr, fw.pattern) {
					frameworks = append(frameworks, analyze.TestFramework{
						Name:       fw.name,
						Type:       fw.ftype,
						Confidence: analyze.ConfidenceMedium,
					})
				}
			}
		}
	}

	return frameworks
}

// detectJavaTestFrameworks detects Java test frameworks
func (cmd *ConsolidatedAnalyzeCommand) detectJavaTestFrameworks(workspaceDir string) []analyze.TestFramework {
	var frameworks []analyze.TestFramework

	// Check pom.xml and build.gradle
	files := []string{"pom.xml", "build.gradle"}

	for _, file := range files {
		filePath := filepath.Join(workspaceDir, file)
		if fileExists(filePath) {
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			contentStr := strings.ToLower(string(content))

			testFrameworks := []struct {
				name    string
				pattern string
				ftype   analyze.TestType
			}{
				{"junit", "junit", analyze.TestTypeUnit},
				{"testng", "testng", analyze.TestTypeUnit},
				{"mockito", "mockito", analyze.TestTypeUnit},
				{"selenium", "selenium", analyze.TestTypeEnd2End},
			}

			for _, fw := range testFrameworks {
				if strings.Contains(contentStr, fw.pattern) {
					frameworks = append(frameworks, analyze.TestFramework{
						Name:       fw.name,
						Type:       fw.ftype,
						Confidence: analyze.ConfidenceMedium,
					})
				}
			}
		}
	}

	return frameworks
}

// Helper types for implementation

// DockerfileInfo represents parsed Dockerfile information
type DockerfileInfo struct {
	Path         string
	Instructions []DockerfileInstruction
}

// DockerfileInstruction represents a single Dockerfile instruction
type DockerfileInstruction struct {
	Command string
	Args    string
	Line    int
}

// Note: getStringSliceParam is defined in common.go
