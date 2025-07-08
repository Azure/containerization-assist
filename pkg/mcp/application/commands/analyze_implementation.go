package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze"
)

// Language detection implementations

// detectLanguageByExtension detects language based on file extensions
func (cmd *ConsolidatedAnalyzeCommand) detectLanguageByExtension(workspaceDir string, languageMap map[string]int) error {
	extensionMap := map[string]string{
		".go":     "go",
		".js":     "javascript",
		".ts":     "typescript",
		".py":     "python",
		".java":   "java",
		".cs":     "csharp",
		".cpp":    "cpp",
		".c":      "c",
		".rb":     "ruby",
		".php":    "php",
		".rs":     "rust",
		".kt":     "kotlin",
		".swift":  "swift",
		".scala":  "scala",
		".sh":     "shell",
		".ps1":    "powershell",
		".yaml":   "yaml",
		".yml":    "yaml",
		".json":   "json",
		".xml":    "xml",
		".html":   "html",
		".css":    "css",
		".scss":   "scss",
		".sass":   "sass",
		".less":   "less",
		".sql":    "sql",
		".md":     "markdown",
		".dockerfile": "dockerfile",
		".Dockerfile": "dockerfile",
	}

	return filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if info.IsDir() {
			// Skip common directories
			dirName := info.Name()
			if dirName == ".git" || dirName == "node_modules" || dirName == "vendor" || dirName == ".vscode" || dirName == ".idea" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(info.Name())
		if lang, exists := extensionMap[ext]; exists {
			languageMap[lang]++
		}

		// Special cases for files without extensions
		fileName := info.Name()
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

		return nil
	})
}

// detectLanguageByContent detects language based on file content patterns
func (cmd *ConsolidatedAnalyzeCommand) detectLanguageByContent(workspaceDir string, languageMap map[string]int) error {
	patterns := map[string]*regexp.Regexp{
		"go":         regexp.MustCompile(`(?m)^package\s+\w+`),
		"javascript": regexp.MustCompile(`(?m)(require\(|import\s+.*from|export\s+.*=)`),
		"typescript": regexp.MustCompile(`(?m)(interface\s+\w+|type\s+\w+\s*=|import.*\.ts)`),
		"python":     regexp.MustCompile(`(?m)(import\s+\w+|from\s+\w+\s+import|def\s+\w+\()`),
		"java":       regexp.MustCompile(`(?m)(public\s+class|import\s+java\.)`),
		"csharp":     regexp.MustCompile(`(?m)(using\s+System|namespace\s+\w+|public\s+class)`),
	}

	return filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Only check text files
		if !cmd.isTextFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		for lang, pattern := range patterns {
			if pattern.MatchString(contentStr) {
				languageMap[lang] += 2 // Weight content-based detection higher
			}
		}

		return nil
	})
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
func (cmd *ConsolidatedAnalyzeCommand) detectGoFramework(result *analyze.AnalysisResult, workspaceDir string) error {
	// Check go.mod for framework dependencies
	goModPath := filepath.Join(workspaceDir, "go.mod")
	if !fileExists(goModPath) {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	contentStr := string(content)
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
func (cmd *ConsolidatedAnalyzeCommand) detectJSFramework(result *analyze.AnalysisResult, workspaceDir string) error {
	packageJSONPath := filepath.Join(workspaceDir, "package.json")
	if !fileExists(packageJSONPath) {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	content, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	contentStr := string(content)
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
func (cmd *ConsolidatedAnalyzeCommand) detectPythonFramework(result *analyze.AnalysisResult, workspaceDir string) error {
	// Check requirements.txt, setup.py, pyproject.toml
	files := []string{"requirements.txt", "setup.py", "pyproject.toml", "Pipfile"}
	
	var content string
	for _, file := range files {
		filePath := filepath.Join(workspaceDir, file)
		if fileExists(filePath) {
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			content += string(fileContent) + "\n"
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
func (cmd *ConsolidatedAnalyzeCommand) detectJavaFramework(result *analyze.AnalysisResult, workspaceDir string) error {
	// Check pom.xml, build.gradle, build.gradle.kts
	files := []string{"pom.xml", "build.gradle", "build.gradle.kts"}
	
	var content string
	for _, file := range files {
		filePath := filepath.Join(workspaceDir, file)
		if fileExists(filePath) {
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			content += string(fileContent) + "\n"
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
func (cmd *ConsolidatedAnalyzeCommand) detectDotNetFramework(result *analyze.AnalysisResult, workspaceDir string) error {
	// Check for .csproj, .vbproj, .fsproj files
	var csprojFiles []string
	filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".csproj") || strings.HasSuffix(info.Name(), ".vbproj") || strings.HasSuffix(info.Name(), ".fsproj") {
			csprojFiles = append(csprojFiles, path)
		}
		return nil
	})

	if len(csprojFiles) == 0 {
		result.Framework = analyze.Framework{
			Name:       "none",
			Type:       analyze.FrameworkTypeNone,
			Confidence: analyze.ConfidenceHigh,
		}
		return nil
	}

	var content string
	for _, file := range csprojFiles {
		fileContent, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		content += string(fileContent) + "\n"
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
func (cmd *ConsolidatedAnalyzeCommand) analyzeGoDependencies(workspaceDir string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency
	
	goModPath := filepath.Join(workspaceDir, "go.mod")
	if !fileExists(goModPath) {
		return dependencies, nil
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
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
					Manager: "go",
				})
			}
		}
	}

	return dependencies, nil
}

// analyzeNodeDependencies analyzes Node.js dependencies
func (cmd *ConsolidatedAnalyzeCommand) analyzeNodeDependencies(workspaceDir string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency
	
	packageJSONPath := filepath.Join(workspaceDir, "package.json")
	if !fileExists(packageJSONPath) {
		return dependencies, nil
	}

	// This is a simplified implementation - in a real system you'd parse JSON properly
	content, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	contentStr := string(content)
	
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
				Manager: "npm",
			})
		}
	}

	return dependencies, nil
}

// analyzePythonDependencies analyzes Python dependencies
func (cmd *ConsolidatedAnalyzeCommand) analyzePythonDependencies(workspaceDir string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency
	
	reqPath := filepath.Join(workspaceDir, "requirements.txt")
	if !fileExists(reqPath) {
		return dependencies, nil
	}

	content, err := os.ReadFile(reqPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read requirements.txt: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
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
				Manager: "pip",
			})
		}
	}

	return dependencies, nil
}

// analyzeJavaDependencies analyzes Java dependencies
func (cmd *ConsolidatedAnalyzeCommand) analyzeJavaDependencies(workspaceDir string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency
	
	// Check for Maven dependencies (pom.xml)
	pomPath := filepath.Join(workspaceDir, "pom.xml")
	if fileExists(pomPath) {
		deps, err := cmd.parseMavenDependencies(pomPath)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, deps...)
	}
	
	// Check for Gradle dependencies (build.gradle)
	gradlePath := filepath.Join(workspaceDir, "build.gradle")
	if fileExists(gradlePath) {
		deps, err := cmd.parseGradleDependencies(gradlePath)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, deps...)
	}

	return dependencies, nil
}

// parseMavenDependencies parses Maven dependencies from pom.xml
func (cmd *ConsolidatedAnalyzeCommand) parseMavenDependencies(pomPath string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency
	
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pom.xml: %w", err)
	}

	// Simple regex-based parsing - in a real system you'd use XML parser
	depPattern := regexp.MustCompile(`<groupId>([^<]+)</groupId>\s*<artifactId>([^<]+)</artifactId>\s*<version>([^<]+)</version>`)
	matches := depPattern.FindAllStringSubmatch(string(content), -1)
	
	for _, match := range matches {
		if len(match) == 4 {
			groupId := match[1]
			artifactId := match[2]
			version := match[3]
			
			dependencies = append(dependencies, analyze.Dependency{
				Name:    fmt.Sprintf("%s:%s", groupId, artifactId),
				Version: version,
				Type:    analyze.DependencyTypeDirect,
				Manager: "maven",
			})
		}
	}

	return dependencies, nil
}

// parseGradleDependencies parses Gradle dependencies from build.gradle
func (cmd *ConsolidatedAnalyzeCommand) parseGradleDependencies(gradlePath string) ([]analyze.Dependency, error) {
	var dependencies []analyze.Dependency
	
	content, err := os.ReadFile(gradlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read build.gradle: %w", err)
	}

	// Simple regex-based parsing for Gradle dependencies
	depPattern := regexp.MustCompile(`(?:implementation|compile|testImplementation|testCompile)\s+['"]([^'"]+)['"]`)
	matches := depPattern.FindAllStringSubmatch(string(content), -1)
	
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
					Manager: "gradle",
				})
			}
		}
	}

	return dependencies, nil
}

// Additional analysis implementations

// analyzeSecrets performs secrets analysis
func (cmd *ConsolidatedAnalyzeCommand) analyzeSecrets(ctx context.Context, result *analyze.AnalysisResult, workspaceDir string) error {
	// Implement secrets analysis using patterns
	secretPatterns := map[string]*regexp.Regexp{
		"AWS_ACCESS_KEY":    regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"AWS_SECRET_KEY":    regexp.MustCompile(`[0-9a-zA-Z/+]{40}`),
		"GITHUB_TOKEN":      regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36}`),
		"PRIVATE_KEY":       regexp.MustCompile(`-----BEGIN [A-Z ]+PRIVATE KEY-----`),
		"API_KEY":           regexp.MustCompile(`[aA][pP][iI]_?[kK][eE][yY].*['\"'][0-9a-zA-Z]{32,45}['\"']`),
		"PASSWORD":          regexp.MustCompile(`[pP][aA][sS][sS][wW][oO][rR][dD].*['\"'][^'\"]{8,}['\"']`),
		"DATABASE_URL":      regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s]*`),
		"JWT_SECRET":        regexp.MustCompile(`[jJ][wW][tT].*['\"'][A-Za-z0-9_-]{20,}['\"']`),
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
		lines := strings.Split(contentStr, "\n")

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
					Rule:        secretType,
				})
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("secrets analysis failed: %w", err)
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
					Rule:        vulnType,
				})
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("vulnerability analysis failed: %w", err)
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
			Rule:        "LICENSE_REQUIRED",
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
			Rule:        "SECURITY_POLICY_REQUIRED",
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
				Name:     "go test",
				Type:     analyze.TestFrameworkTypeUnit,
				Version:  "builtin",
				Coverage: cmd.calculateGoCoverage(workspaceDir),
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
		return fmt.Errorf("metrics analysis failed: %w", err)
	}
	
	// Store metrics in analysis metadata
	result.AnalysisMetadata.Metrics = map[string]interface{}{
		"total_files": totalFiles,
		"total_lines": totalLines,
		"avg_lines_per_file": func() float64 {
			if totalFiles == 0 {
				return 0
			}
			return float64(totalLines) / float64(totalFiles)
		}(),
	}
	
	return nil
}

// Dockerfile analysis implementations

// parseDockerfile parses a Dockerfile
func (cmd *ConsolidatedAnalyzeCommand) parseDockerfile(path string) (*DockerfileInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Dockerfile: %w", err)
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
					Type:        analyze.SecurityIssueTypeConfiguration,
					Severity:    analyze.SeverityHigh,
					Title:       "Running as root user",
					Description: "Container should not run as root user",
					File:        dockerfile.Path,
					Line:        instruction.Line,
					Rule:        "DOCKERFILE_USER_ROOT",
				})
			}
		case "ADD":
			if strings.Contains(instruction.Args, "http://") {
				issues = append(issues, analyze.SecurityIssue{
					Type:        analyze.SecurityIssueTypeConfiguration,
					Severity:    analyze.SeverityMedium,
					Title:       "Using HTTP in ADD instruction",
					Description: "ADD instruction should use HTTPS instead of HTTP",
					File:        dockerfile.Path,
					Line:        instruction.Line,
					Rule:        "DOCKERFILE_ADD_HTTP",
				})
			}
		case "RUN":
			if strings.Contains(instruction.Args, "curl") && strings.Contains(instruction.Args, "sudo") {
				issues = append(issues, analyze.SecurityIssue{
					Type:        analyze.SecurityIssueTypeConfiguration,
					Severity:    analyze.SeverityMedium,
					Title:       "Using sudo in RUN instruction",
					Description: "Avoid using sudo in RUN instructions",
					File:        dockerfile.Path,
					Line:        instruction.Line,
					Rule:        "DOCKERFILE_RUN_SUDO",
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
			Category:    "dockerfile",
		})
	}
	
	if !hasHealthcheck {
		recommendations = append(recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypeOperational,
			Priority:    analyze.PriorityMedium,
			Title:       "Add HEALTHCHECK instruction",
			Description: "Add HEALTHCHECK instruction to monitor container health",
			Category:    "dockerfile",
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
		float64(result.Framework.Confidence) / 3.0, // Convert to 0-1 scale
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
			Category:    "framework",
		})
	}
	
	// Security recommendations
	if len(result.SecurityIssues) > 0 {
		result.Recommendations = append(result.Recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypeSecurity,
			Priority:    analyze.PriorityHigh,
			Title:       "Address security issues",
			Description: fmt.Sprintf("Found %d security issues that should be addressed", len(result.SecurityIssues)),
			Category:    "security",
		})
	}
	
	// Testing recommendations
	if len(result.TestFrameworks) == 0 {
		result.Recommendations = append(result.Recommendations, analyze.Recommendation{
			Type:        analyze.RecommendationTypeQuality,
			Priority:    analyze.PriorityMedium,
			Title:       "Add automated tests",
			Description: "Consider adding unit tests and integration tests",
			Category:    "testing",
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

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

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
		ftype   analyze.TestFrameworkType
	}{
		{"jest", "\"jest\":", analyze.TestFrameworkTypeUnit},
		{"mocha", "\"mocha\":", analyze.TestFrameworkTypeUnit},
		{"jasmine", "\"jasmine\":", analyze.TestFrameworkTypeUnit},
		{"cypress", "\"cypress\":", analyze.TestFrameworkTypeE2E},
		{"playwright", "\"playwright\":", analyze.TestFrameworkTypeE2E},
		{"puppeteer", "\"puppeteer\":", analyze.TestFrameworkTypeE2E},
	}
	
	for _, fw := range testFrameworks {
		if strings.Contains(contentStr, fw.pattern) {
			frameworks = append(frameworks, analyze.TestFramework{
				Name:     fw.name,
				Type:     fw.ftype,
				Version:  "unknown",
				Coverage: 0.0,
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
				ftype   analyze.TestFrameworkType
			}{
				{"pytest", "pytest", analyze.TestFrameworkTypeUnit},
				{"unittest", "unittest", analyze.TestFrameworkTypeUnit},
				{"nose", "nose", analyze.TestFrameworkTypeUnit},
				{"selenium", "selenium", analyze.TestFrameworkTypeE2E},
			}
			
			for _, fw := range testFrameworks {
				if strings.Contains(contentStr, fw.pattern) {
					frameworks = append(frameworks, analyze.TestFramework{
						Name:     fw.name,
						Type:     fw.ftype,
						Version:  "unknown",
						Coverage: 0.0,
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
				ftype   analyze.TestFrameworkType
			}{
				{"junit", "junit", analyze.TestFrameworkTypeUnit},
				{"testng", "testng", analyze.TestFrameworkTypeUnit},
				{"mockito", "mockito", analyze.TestFrameworkTypeUnit},
				{"selenium", "selenium", analyze.TestFrameworkTypeE2E},
			}
			
			for _, fw := range testFrameworks {
				if strings.Contains(contentStr, fw.pattern) {
					frameworks = append(frameworks, analyze.TestFramework{
						Name:     fw.name,
						Type:     fw.ftype,
						Version:  "unknown",
						Coverage: 0.0,
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

// getStringSliceParam extracts string slice parameter from input data
func getStringSliceParam(params map[string]interface{}, key string) []string {
	if val, exists := params[key]; exists {
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, len(slice))
			for i, item := range slice {
				if str, ok := item.(string); ok {
					result[i] = str
				}
			}
			return result
		}
	}
	return []string{}
}