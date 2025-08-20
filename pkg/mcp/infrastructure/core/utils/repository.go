// Package utils provides consolidated utility functions for the MCP package.
//
// This package includes:
// - ExtractRepoName: Extract repository name from a Git URL
// - ExtractDeploymentName: Extract Kubernetes-compatible deployment name from an image reference
// - RepositoryAnalyzer: Perform mechanical repository analysis without AI
// - MaskSensitiveData: Mask sensitive information in strings
// - AI-powered retry operations with progressive error context
package utils

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/common/filesystem"
)

// ExtractRepoName extracts repository name from Git URL
func ExtractRepoName(repoURL string) string {
	if repoURL == "" {
		return "app"
	}

	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return "app"
	}

	name := parts[len(parts)-1]
	return strings.TrimSuffix(name, ".git")
}

// ExtractDeploymentName extracts deployment name from image reference
func ExtractDeploymentName(imageRef string) string {
	if imageRef == "" {
		return "unknown-deployment"
	}

	// Extract the image name part after the last '/'
	parts := strings.Split(imageRef, "/")
	imageName := parts[len(parts)-1]

	// Remove tag if present
	if idx := strings.Index(imageName, ":"); idx != -1 {
		imageName = imageName[:idx]
	}

	// Replace underscores and dots with hyphens for valid K8s names
	deploymentName := strings.ReplaceAll(imageName, "_", "-")
	deploymentName = strings.ReplaceAll(deploymentName, ".", "-")

	return deploymentName
}

// RepositoryAnalyzer provides mechanical repository analysis without AI
type RepositoryAnalyzer struct {
	logger *slog.Logger
}

// NewRepositoryAnalyzer creates a new repository analyzer
func NewRepositoryAnalyzer(logger *slog.Logger) *RepositoryAnalyzer { //TODO: Refactor -  we are just wrapping the logger
	return &RepositoryAnalyzer{
		logger: logger.With("component", "repository_analyzer"),
	}
}

// AnalysisResult contains the result of repository analysis
type AnalysisResult struct {
	Success      bool                   `json:"success"`
	Language     string                 `json:"language"`
	Framework    string                 `json:"framework,omitempty"`
	Dependencies []Dependency           `json:"dependencies"`
	ConfigFiles  []ConfigFile           `json:"config_files"`
	Structure    map[string]interface{} `json:"structure"`
	EntryPoints  []string               `json:"entry_points"`
	BuildFiles   []string               `json:"build_files"`
	Port         int                    `json:"port,omitempty"`
	DatabaseInfo *DatabaseInfo          `json:"database_info,omitempty"`
	Suggestions  []string               `json:"suggestions"`
	Context      map[string]interface{} `json:"context"`
	Error        *AnalysisError         `json:"error,omitempty"`
}

// Dependency represents a project dependency
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Type    string `json:"type"`    // "runtime", "dev", "build"
	Manager string `json:"manager"` // "npm", "pip", "maven", etc.
}

// ConfigFile represents a configuration file found in the repository
type ConfigFile struct {
	Path     string                 `json:"path"`
	Type     string                 `json:"type"` // "package", "build", "env", "docker"
	Content  map[string]interface{} `json:"content,omitempty"`
	Relevant bool                   `json:"relevant"`
}

// DatabaseInfo contains information about database usage
type DatabaseInfo struct {
	Detected    bool     `json:"detected"`
	Types       []string `json:"types"`        // "mysql", "postgres", "mongodb", etc.
	Libraries   []string `json:"libraries"`    // Detected database libraries
	ConfigFiles []string `json:"config_files"` // Files with DB config
}

// AnalysisError provides detailed analysis error information
type AnalysisError struct {
	Type    string                 `json:"type"`
	Message string                 `json:"message"`
	Path    string                 `json:"path,omitempty"`
	Context map[string]interface{} `json:"context"`
}

// AnalyzeRepository performs comprehensive mechanical analysis of a repository
func (ra *RepositoryAnalyzer) AnalyzeRepository(repoPath string) (*AnalysisResult, error) {
	startTime := time.Now()

	result := &AnalysisResult{
		Dependencies: make([]Dependency, 0),
		ConfigFiles:  make([]ConfigFile, 0),
		EntryPoints:  make([]string, 0),
		BuildFiles:   make([]string, 0),
		Suggestions:  make([]string, 0),
		Context:      make(map[string]interface{}),
	}

	ra.logger.Info("Starting repository analysis", "repo_path", repoPath)

	// Validate input
	if err := ra.validateInput(repoPath); err != nil {
		result.Error = &AnalysisError{
			Type:    "validation_error",
			Message: err.Error(),
			Path:    repoPath,
		}
		return result, nil
	}

	// Generate structured file tree using existing filesystem function
	fileTreeMap, err := filesystem.GenerateFileTreeMap(repoPath, 4)
	if err != nil {
		result.Error = &AnalysisError{
			Type:    "filesystem_error",
			Message: fmt.Sprintf("Failed to read file tree: %v", err),
			Path:    repoPath,
		}
		return result, nil
	}

	result.Structure = fileTreeMap

	// Detect language and framework
	result.Language, result.Framework = ra.detectLanguageAndFramework(repoPath)

	// Analyze configuration files
	result.ConfigFiles = ra.analyzeConfigFiles(repoPath)

	// Extract dependencies
	result.Dependencies = ra.extractDependencies(repoPath, result.ConfigFiles)

	// Find entry points
	result.EntryPoints = ra.findEntryPoints(repoPath, result.Language, result.Framework)

	// Find build files
	result.BuildFiles = ra.findBuildFiles(repoPath)

	// Detect port
	result.Port = ra.detectPort(repoPath, result.ConfigFiles)

	// Analyze database usage
	result.DatabaseInfo = ra.analyzeDatabase(repoPath, result.Dependencies, result.ConfigFiles)

	// Generate suggestions
	result.Suggestions = ra.generateSuggestions(result)

	// Set context
	result.Context = map[string]interface{}{
		"analysis_time":     time.Since(startTime).Seconds(),
		"files_analyzed":    len(result.ConfigFiles),
		"dependencies":      len(result.Dependencies),
		"entry_points":      len(result.EntryPoints),
		"database_detected": result.DatabaseInfo.Detected,
	}

	result.Success = true

	ra.logger.Info("Repository analysis completed",
		"language", result.Language,
		"framework", result.Framework,
		"dependencies", len(result.Dependencies),
		"database", result.DatabaseInfo.Detected)

	return result, nil
}

// detectLanguageAndFramework detects the primary language and framework
// This is intentionally simple - complex logic belongs in the AI enhancement layer
func (ra *RepositoryAnalyzer) detectLanguageAndFramework(repoPath string) (string, string) {
	// Check for specific files that indicate language/framework
	checks := []struct {
		file      string
		language  string
		framework string
	}{
		{"package.json", "javascript", ""},
		{"go.mod", "go", ""},
		{"requirements.txt", "python", ""},
		{"Pipfile", "python", ""},
		{"pyproject.toml", "python", ""},
		{"pom.xml", "java", "maven"},
		{"build.gradle", "java", "gradle"},
		{"build.gradle.kts", "java", "gradle"},
		{"build.xml", "java", "ant"},
		{"ivy.xml", "java", "ant"},
		{"server.xml", "java", "tomcat"},
		{"context.xml", "java", "tomcat"},
		{"jboss-web.xml", "java", "jboss"},
		{"wildfly.xml", "java", "wildfly"},
		{"Cargo.toml", "rust", ""},
		{"composer.json", "php", ""},
		{"Gemfile", "ruby", ""},
		{"mix.exs", "elixir", ""},
		{"project.clj", "clojure", ""},
		{"*.csproj", "csharp", "dotnet"},
		{"project.json", "csharp", "dotnet"},
	}

	for _, check := range checks {
		if strings.Contains(check.file, "*") {
			if ra.findFilesByPattern(repoPath, check.file) {
				return check.language, check.framework
			}
		} else {
			filePath := filepath.Join(repoPath, check.file)
			if _, err := os.Stat(filePath); err == nil {
				framework := check.framework

				if check.language == "javascript" {
					framework = ra.detectJavaScriptFramework(filePath)
				}

				return check.language, framework
			}
		}
	}

	// If no config files found, try file extension counting
	return ra.detectLanguageByExtensions(repoPath), ""
}

// detectJavaScriptFramework detects specific JavaScript frameworks
func (ra *RepositoryAnalyzer) detectJavaScriptFramework(packageJsonPath string) string {
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return ""
	}

	var packageJson map[string]interface{}
	if err := json.Unmarshal(content, &packageJson); err != nil {
		return ""
	}

	// Check dependencies for framework indicators
	deps := ra.extractJSONDependencies(packageJson, "dependencies")
	devDeps := ra.extractJSONDependencies(packageJson, "devDependencies")
	allDeps := append(deps, devDeps...)

	for _, dep := range allDeps {
		switch dep.Name {
		case "next":
			return "nextjs"
		case "react":
			return "react"
		case "vue":
			return "vue"
		case "angular", "@angular/core":
			return "angular"
		case "express":
			return "express"
		case "koa":
			return "koa"
		case "fastify":
			return "fastify"
		case "nuxt":
			return "nuxt"
		case "gatsby":
			return "gatsby"
		}
	}

	return "nodejs"
}

// detectLanguageByExtensions detects language by counting file extensions
func (ra *RepositoryAnalyzer) detectLanguageByExtensions(repoPath string) string {
	extensionCounts := make(map[string]int)

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != "" {
			extensionCounts[ext]++
		}

		return nil
	})

	if err != nil {
		return "unknown"
	}

	// Language mappings
	langMap := map[string]string{
		".js":    "javascript",
		".ts":    "typescript",
		".py":    "python",
		".go":    "go",
		".java":  "java",
		".rs":    "rust",
		".php":   "php",
		".rb":    "ruby",
		".cs":    "csharp",
		".cpp":   "cpp",
		".c":     "c",
		".kt":    "kotlin",
		".scala": "scala",
	}

	// Find the most common language extension
	maxCount := 0
	detectedLang := "unknown"

	for ext, count := range extensionCounts {
		if lang, exists := langMap[ext]; exists && count > maxCount {
			maxCount = count
			detectedLang = lang
		}
	}

	return detectedLang
}

// analyzeConfigFiles finds and analyzes configuration files with graceful error handling
func (ra *RepositoryAnalyzer) analyzeConfigFiles(repoPath string) []ConfigFile {
	configFiles := make([]ConfigFile, 0)

	// Define important configuration files
	configPatterns := map[string]string{
		"package.json":           "package",
		"requirements.txt":       "package",
		"Pipfile":                "package",
		"pyproject.toml":         "package",
		"pom.xml":                "build",
		"build.gradle":           "build",
		"build.gradle.kts":       "build",
		"Cargo.toml":             "package",
		"composer.json":          "package",
		"Gemfile":                "package",
		"go.mod":                 "package",
		".env":                   "env",
		".env.example":           "env",
		"config.json":            "config",
		"appsettings.json":       "config",
		"application.properties": "config",
		"application.yml":        "config",
		"docker-compose.yml":     "docker",
		"docker-compose.yaml":    "docker",
		"Dockerfile":             "docker",
		"Makefile":               "build",
		"tsconfig.json":          "config",
		"webpack.config.js":      "build",
	}

	var filesChecked, filesFound int

	for fileName, fileType := range configPatterns {
		filesChecked++
		filePath := filepath.Join(repoPath, fileName)

		info, err := os.Stat(filePath)
		if err != nil {
			if !os.IsNotExist(err) {
				// Log actual errors (not just missing files)
				ra.logger.Debug("Failed to stat config file",
					"file", fileName,
					"error", err,
					"path", filePath)
			}
			continue
		}

		// Skip directories
		if info.IsDir() {
			ra.logger.Debug("Skipping directory with config name", "path", fileName)
			continue
		}

		filesFound++
		configFile := ConfigFile{
			Path:     fileName,
			Type:     fileType,
			Relevant: true,
		}

		// Try to parse content for JSON files with error handling
		if strings.HasSuffix(fileName, ".json") {
			content, err := ra.parseJSONFile(filePath)
			if err != nil {
				ra.logger.Warn("Failed to parse JSON config file",
					"file", fileName,
					"error", err,
					"continuing", "yes")
				// Still include the file even if parsing failed
			} else {
				configFile.Content = content
			}
		}

		configFiles = append(configFiles, configFile)
	}

	ra.logger.Debug("Config file analysis complete",
		"files_checked", filesChecked,
		"files_found", filesFound,
		"config_files", len(configFiles))

	return configFiles
}

// extractDependencies extracts dependencies from configuration files
func (ra *RepositoryAnalyzer) extractDependencies(repoPath string, configFiles []ConfigFile) []Dependency {
	dependencies := make([]Dependency, 0)

	for _, configFile := range configFiles {
		filePath := filepath.Join(repoPath, configFile.Path)

		switch configFile.Path {
		case "package.json":
			deps := ra.extractNpmDependencies(filePath)
			dependencies = append(dependencies, deps...)
		case "requirements.txt":
			deps := ra.extractPipDependencies(filePath)
			dependencies = append(dependencies, deps...)
		case "pom.xml":
			deps := ra.extractMavenDependencies(filePath)
			dependencies = append(dependencies, deps...)
		case "go.mod":
			deps := ra.extractGoDependencies(filePath)
			dependencies = append(dependencies, deps...)
		}
	}

	return dependencies
}

// findEntryPoints finds application entry points
func (ra *RepositoryAnalyzer) findEntryPoints(repoPath, language, framework string) []string {
	entryPoints := make([]string, 0)

	// Common entry point patterns by language
	patterns := map[string][]string{
		"javascript": {"index.js", "app.js", "server.js", "main.js", "src/index.js", "src/app.js"},
		"typescript": {"index.ts", "app.ts", "server.ts", "main.ts", "src/index.ts", "src/app.ts"},
		"python":     {"main.py", "app.py", "server.py", "__main__.py", "run.py", "wsgi.py", "manage.py"},
		"go":         {"main.go", "cmd/main.go", "cmd/*/main.go"},
		"java":       {"src/main/java/**/Application.java", "src/main/java/**/Main.java"},
		"rust":       {"src/main.rs", "main.rs"},
		"php":        {"index.php", "app.php", "public/index.php"},
	}

	if langPatterns, exists := patterns[language]; exists {
		for _, pattern := range langPatterns {
			if strings.Contains(pattern, "**") || strings.Contains(pattern, "*") {
				// Handle wildcard patterns with basic glob support
				matches, err := filepath.Glob(filepath.Join(repoPath, pattern))
				if err != nil {
					ra.logger.Debug("Error in glob pattern", "pattern", pattern, "error", err)
					continue
				}
				for _, match := range matches {
					relPath, err := filepath.Rel(repoPath, match)
					if err != nil {
						ra.logger.Debug("Failed to get relative path", "path", match, "error", err)
						continue
					}
					entryPoints = append(entryPoints, relPath)
				}
			} else {
				filePath := filepath.Join(repoPath, pattern)
				info, err := os.Stat(filePath)
				if err != nil {
					if !os.IsNotExist(err) {
						ra.logger.Debug("Error checking entry point", "file", pattern, "error", err)
					}
					continue
				}
				if !info.IsDir() {
					entryPoints = append(entryPoints, pattern)
				}
			}
		}
	}

	// If no entry points found, log a warning
	if len(entryPoints) == 0 {
		ra.logger.Debug("No standard entry points found", "language", language, "framework", framework)
	}

	return entryPoints
}

// findBuildFiles finds build-related files
func (ra *RepositoryAnalyzer) findBuildFiles(repoPath string) []string {
	buildFiles := make([]string, 0)

	buildPatterns := []string{
		"Makefile", "makefile",
		"build.sh", "build.py",
		"webpack.config.js",
		"rollup.config.js",
		"vite.config.js",
		"tsconfig.json",
		".github/workflows/*.yml",
		"Jenkinsfile",
		"azure-pipelines.yml",
	}

	for _, pattern := range buildPatterns {
		if strings.Contains(pattern, "*") {
			if ra.findFilesByPattern(repoPath, pattern) {
				buildFiles = append(buildFiles, pattern)
			}
		} else {
			filePath := filepath.Join(repoPath, pattern)
			if _, err := os.Stat(filePath); err == nil {
				buildFiles = append(buildFiles, pattern)
			}
		}
	}

	return buildFiles
}

// detectPort detects the port the application runs on
func (ra *RepositoryAnalyzer) detectPort(repoPath string, configFiles []ConfigFile) int {
	// Check common environment files for PORT
	envFiles := []string{".env", ".env.example", "config.json"}

	for _, envFile := range envFiles {
		filePath := filepath.Join(repoPath, envFile)
		if port := ra.extractPortFromFile(filePath); port > 0 {
			return port
		}
	}

	// Check package.json scripts for port references
	packageJsonPath := filepath.Join(repoPath, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		if port := ra.extractPortFromPackageJson(packageJsonPath); port > 0 {
			return port
		}
	}

	// Default ports by framework (currently unused, but kept for future use)
	_ = map[string]int{
		"express": 3000,
		"nextjs":  3000,
		"react":   3000,
		"vue":     8080,
		"angular": 4200,
		"flask":   5000,
		"django":  8000,
		"spring":  8080,
	}

	// Try to find the port in common entry files
	entryFiles := []string{"index.js", "app.js", "server.js", "main.py", "app.py"}
	for _, entryFile := range entryFiles {
		filePath := filepath.Join(repoPath, entryFile)
		if port := ra.extractPortFromFile(filePath); port > 0 {
			return port
		}
	}

	return 0
}

// analyzeDatabase analyzes database usage in the repository
func (ra *RepositoryAnalyzer) analyzeDatabase(repoPath string, dependencies []Dependency, configFiles []ConfigFile) *DatabaseInfo {
	dbInfo := &DatabaseInfo{
		Types:       make([]string, 0),
		Libraries:   make([]string, 0),
		ConfigFiles: make([]string, 0),
	}

	// Database libraries by language
	dbLibraries := map[string]string{
		// JavaScript/Node.js
		"mysql":     "mysql",
		"mysql2":    "mysql",
		"pg":        "postgres",
		"postgres":  "postgres",
		"mongodb":   "mongodb",
		"mongoose":  "mongodb",
		"redis":     "redis",
		"sqlite3":   "sqlite",
		"sequelize": "sql",
		"prisma":    "sql",
		"typeorm":   "sql",

		// Python
		"pymongo":     "mongodb",
		"psycopg2":    "postgres",
		"mysqlclient": "mysql",
		"sqlite":      "sqlite",
		"sqlalchemy":  "sql",
		"django":      "sql",

		// Java
		"mysql-connector-java": "mysql",
		"postgresql":           "postgres",
		"mongo-java-driver":    "mongodb",
		"jedis":                "redis",

		// Go
		"github.com/lib/pq":              "postgres",
		"github.com/go-sql-driver/mysql": "mysql",
		"go.mongodb.org/mongo-driver":    "mongodb",
		"github.com/go-redis/redis":      "redis",
	}

	// Check dependencies for database libraries
	for _, dep := range dependencies {
		if dbType, exists := dbLibraries[dep.Name]; exists {
			if !slices.Contains(dbInfo.Types, dbType) {
				dbInfo.Types = append(dbInfo.Types, dbType)
			}
			dbInfo.Libraries = append(dbInfo.Libraries, dep.Name)
			dbInfo.Detected = true
		}
	}

	// Check configuration files for database connections
	for _, configFile := range configFiles {
		filePath := filepath.Join(repoPath, configFile.Path)
		if ra.containsDatabaseConfig(filePath) {
			dbInfo.ConfigFiles = append(dbInfo.ConfigFiles, configFile.Path)
			dbInfo.Detected = true
		}
	}

	return dbInfo
}

// Helper methods

func (ra *RepositoryAnalyzer) validateInput(repoPath string) error {
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	return nil
}

func (ra *RepositoryAnalyzer) findFilesByPattern(repoPath, pattern string) bool {
	// Convert simple glob pattern to regex for basic support
	regexPattern := strings.ReplaceAll(pattern, ".", "\\.")
	regexPattern = strings.ReplaceAll(regexPattern, "*", ".*")

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		ra.logger.Debug("Invalid pattern", "pattern", pattern, "error", err)
		return false
	}

	var found bool
	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}

		if regex.MatchString(relPath) || regex.MatchString(info.Name()) {
			found = true
			return filepath.SkipDir // Stop walking once we find a match
		}

		return nil
	})

	return found
}

func (ra *RepositoryAnalyzer) parseJSONFile(filePath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		ra.logger.Debug("Failed to read JSON file", "path", filePath, "error", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check if file is empty
	if len(content) == 0 {
		ra.logger.Debug("Empty JSON file", "path", filePath)
		return make(map[string]interface{}), nil
	}

	var result map[string]interface{}
	err = json.Unmarshal(content, &result)
	if err != nil {
		// Try to provide more context about the JSON error
		preview := string(content)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		ra.logger.Debug("Failed to parse JSON",
			"path", filePath,
			"error", err,
			"preview", preview)
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return result, nil
}

func (ra *RepositoryAnalyzer) extractJSONDependencies(packageJson map[string]interface{}, key string) []Dependency {
	deps := make([]Dependency, 0)

	if depsInterface, exists := packageJson[key]; exists {
		if depsMap, ok := depsInterface.(map[string]interface{}); ok {
			for name, versionInterface := range depsMap {
				version := ""
				if v, ok := versionInterface.(string); ok {
					version = v
				}
				deps = append(deps, Dependency{
					Name:    name,
					Version: version,
					Type:    key,
					Manager: "npm",
				})
			}
		}
	}

	return deps
}

func (ra *RepositoryAnalyzer) extractNpmDependencies(filePath string) []Dependency {
	content, err := ra.parseJSONFile(filePath)
	if err != nil {
		return nil
	}

	deps := make([]Dependency, 0)
	deps = append(deps, ra.extractJSONDependencies(content, "dependencies")...)
	deps = append(deps, ra.extractJSONDependencies(content, "devDependencies")...)

	return deps
}

func (ra *RepositoryAnalyzer) extractPipDependencies(filePath string) []Dependency {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	deps := make([]Dependency, 0)
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse requirement line (simple implementation)
		parts := regexp.MustCompile(`[>=<~!]`).Split(line, 2)
		name := strings.TrimSpace(parts[0])
		version := ""
		if len(parts) > 1 {
			version = strings.TrimSpace(parts[1])
		}

		deps = append(deps, Dependency{
			Name:    name,
			Version: version,
			Type:    "runtime",
			Manager: "pip",
		})
	}

	return deps
}

// extractMavenDependencies parses Maven POM.xml files for dependencies
func (ra *RepositoryAnalyzer) extractMavenDependencies(filePath string) []Dependency {
	content, err := os.ReadFile(filePath)
	if err != nil {
		ra.logger.Debug("Failed to read Maven POM file", "error", err, "file", filePath)
		return nil
	}

	var pom struct {
		Dependencies struct {
			Dependency []struct {
				GroupID    string `xml:"groupId"`
				ArtifactID string `xml:"artifactId"`
				Version    string `xml:"version"`
				Scope      string `xml:"scope"`
			} `xml:"dependency"`
		} `xml:"dependencies"`
	}

	if err := xml.Unmarshal(content, &pom); err != nil {
		ra.logger.Debug("Failed to parse Maven POM XML", "error", err, "file", filePath)
		return nil
	}

	var deps []Dependency
	for _, dep := range pom.Dependencies.Dependency {
		if dep.GroupID != "" && dep.ArtifactID != "" {
			deps = append(deps, Dependency{
				Name:    fmt.Sprintf("%s:%s", dep.GroupID, dep.ArtifactID),
				Version: dep.Version,
				Type:    dep.Scope, // Use scope as type (compile, test, runtime, etc.)
				Manager: "maven",
			})
		}
	}

	return deps
}

// extractGoDependencies parses Go go.mod files for dependencies
func (ra *RepositoryAnalyzer) extractGoDependencies(filePath string) []Dependency {
	content, err := os.ReadFile(filePath)
	if err != nil {
		ra.logger.Debug("Failed to read Go mod file", "error", err, "file", filePath)
		return nil
	}

	lines := strings.Split(string(content), "\n")
	var deps []Dependency
	inRequireBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Handle require block
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// Parse single require statement
		if strings.HasPrefix(line, "require ") {
			parts := strings.Fields(line[8:]) // Remove "require "
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]
				deps = append(deps, Dependency{
					Name:    name,
					Version: version,
					Type:    "go",
				})
			}
			continue
		}

		// Parse dependencies within require block
		if inRequireBlock && line != "" && !strings.HasPrefix(line, "//") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]
				// Remove trailing comment if present
				if idx := strings.Index(version, " //"); idx > 0 {
					version = version[:idx]
				}
				deps = append(deps, Dependency{
					Name:    name,
					Version: version,
					Type:    "go",
				})
			}
		}
	}

	return deps
}

func (ra *RepositoryAnalyzer) extractPortFromFile(filePath string) int {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}

	// Look for PORT environment variable or common port patterns
	portRegex := regexp.MustCompile(`(?i)port[^\d]*(\d+)`)
	matches := portRegex.FindStringSubmatch(string(content))
	if len(matches) > 1 {
		if port := parseInt(matches[1]); port > 0 && port < 65536 {
			return port
		}
	}

	return 0
}

func (ra *RepositoryAnalyzer) extractPortFromPackageJson(filePath string) int {
	content, err := os.ReadFile(filePath)
	if err != nil {
		ra.logger.Debug("Failed to read package.json file", "error", err, "file", filePath)
		return 0
	}

	var pkg struct {
		Scripts map[string]string      `json:"scripts"`
		Config  map[string]interface{} `json:"config"`
	}

	if err := json.Unmarshal(content, &pkg); err != nil {
		ra.logger.Debug("Failed to parse package.json", "error", err, "file", filePath)
		return 0
	}

	// Check scripts for port references
	for _, script := range pkg.Scripts {
		// Look for --port, -p, PORT= patterns in scripts
		portRegex := regexp.MustCompile(`(?:--port|PORT=|:)[\s=]*(\d+)`)
		matches := portRegex.FindStringSubmatch(script)
		if len(matches) > 1 {
			if port := parseInt(matches[1]); port > 0 && port < 65536 {
				return port
			}
		}
	}

	// Check config section for port
	if port, ok := pkg.Config["port"]; ok {
		switch v := port.(type) {
		case float64:
			if int(v) > 0 && int(v) < 65536 {
				return int(v)
			}
		case string:
			if port := parseInt(v); port > 0 && port < 65536 {
				return port
			}
		}
	}

	return 0
}

func (ra *RepositoryAnalyzer) containsDatabaseConfig(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	contentStr := strings.ToLower(string(content))
	dbKeywords := []string{"database", "mongodb", "mysql", "postgres", "redis", "connection", "db_"}

	for _, keyword := range dbKeywords {
		if strings.Contains(contentStr, keyword) {
			return true
		}
	}

	return false
}

func (ra *RepositoryAnalyzer) generateSuggestions(result *AnalysisResult) []string {
	suggestions := make([]string, 0)

	if result.Language != "" {
		suggestions = append(suggestions, fmt.Sprintf("Detected %s project", result.Language))
	}

	if result.Framework != "" {
		suggestions = append(suggestions, fmt.Sprintf("Using %s framework", result.Framework))
	}

	if len(result.Dependencies) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Found %d dependencies", len(result.Dependencies)))
	}

	if result.DatabaseInfo.Detected {
		suggestions = append(suggestions, "Database usage detected - ensure proper connection configuration")
	}

	if result.Port > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Application appears to run on port %d", result.Port))
	}

	suggestions = append(suggestions, "Review the analysis results and verify accuracy")
	suggestions = append(suggestions, "Consider the detected configuration for containerization")

	return suggestions
}

// parseInt safely parses a string to int
func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}
