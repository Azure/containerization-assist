// Package utils provides consolidated utility functions for the MCP package.
//
// This package includes:
// - ExtractRepoName: Extract repository name from a Git URL
// - RepositoryAnalyzer: Perform mechanical repository analysis without AI
// - MaskSensitiveData: Mask sensitive information in strings
// - AI-powered retry operations with progressive error context
package utils

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/containerization-assist/pkg/infrastructure/core/filesystem"
)

const (
	maxPortNumber      = 65535
	maxFileSize        = 10 * 1024 * 1024 // 10MB max file size for parsing
	maxPathDepth       = 10               // Maximum directory depth to search
	defaultJavaVersion = "21"             // Default to latest LTS
)

// Pre-compiled regex patterns for performance
var (
	portRegexOnce sync.Once
	portRegex     *regexp.Regexp

	javaVersionPatternsOnce sync.Once
	javaVersionPatterns     map[string][]*regexp.Regexp

	legacyJavaVersionsOnce sync.Once
	legacyJavaVersions     map[string]string
)

// Application server detection constants - optimized with merged indicators
var (
	// Pre-merged server indicators for better performance
	allServerIndicators     map[string]string
	allServerIndicatorsOnce sync.Once

	// Common server indicators that work across Maven and Gradle
	serverIndicators = map[string]string{
		// JBoss/WildFly indicators
		"org.jboss":                  "jboss",
		"jboss-as":                   "jboss",
		"jboss-eap":                  "jboss",
		"jboss-web":                  "jboss",
		"wildfly":                    "wildfly",
		"org.wildfly":                "wildfly",
		"wildfly-":                   "wildfly",
		"jboss-deployment-structure": "jboss",
		"wildfly-config":             "wildfly",

		// Tomcat indicators
		"tomcat":            "tomcat",
		"org.apache.tomcat": "tomcat",

		// Other servers
		"jetty":             "jetty",
		"org.eclipse.jetty": "jetty",
		"undertow":          "undertow",
		"io.undertow":       "undertow",
		"glassfish":         "glassfish",
		"weblogic":          "weblogic",
		"liberty":           "liberty",
		"websphere":         "liberty",
		"micronaut":         "micronaut",
		"quarkus":           "quarkus",
	}

	// Dependency-specific server indicators (for Spring Boot embedded servers)
	dependencyServerIndicators = map[string]string{
		"spring-boot-starter-web":      "embedded-tomcat",
		"spring-boot-starter-jetty":    "embedded-jetty",
		"spring-boot-starter-undertow": "embedded-undertow",
	}

	// Maven-specific indicators
	mavenServerIndicators = map[string]string{
		"wildfly-maven-plugin":       "wildfly",
		"jboss-maven-plugin":         "jboss",
		"jboss-as-maven-plugin":      "jboss",
		"<packaging>ear</packaging>": "jboss",
		"<packaging>war</packaging>": "servlet",
	}

	// Gradle-specific indicators
	gradleServerIndicators = map[string]string{
		"wildfly-gradle-plugin": "wildfly",
		"org.wildfly.plugins":   "wildfly",
		"jboss-gradle-plugin":   "jboss",
		"gradle-jboss-plugin":   "jboss",
		"jbossas-gradle-plugin": "jboss",
		"apply plugin: 'ear'":   "jboss",
		"id 'ear'":              "jboss",
		"apply plugin: 'war'":   "servlet",
		"id 'war'":              "servlet",
		"jboss.home":            "jboss",
		"wildfly.home":          "wildfly",
	}

	// Priority-ordered server config files for efficient detection
	serverConfigFiles = []struct {
		filename string
		server   string
		priority int // Lower number = higher priority
	}{
		{"server.xml", "tomcat", 1},
		{"context.xml", "tomcat", 1},
		{"jboss-web.xml", "jboss", 2},
		{"wildfly.xml", "wildfly", 2},
		{"standalone.xml", "wildfly", 2},
		{"weblogic.xml", "weblogic", 3},
		{"glassfish-web.xml", "glassfish", 3},
		{"jetty.xml", "jetty", 4},
		{"liberty-web.xml", "liberty", 4},
		{"undertow.xml", "undertow", 5},
	}

	// Common config directories (reduced from 35+ to most common ones)
	commonConfigDirs = []string{
		"WEB-INF",
		"src/main/webapp/WEB-INF",
		"src/main/resources",
		"src/main/resources/META-INF",
		"conf",
		"config",
		"META-INF",
	}
)

// Performance optimization: Initialize all cached patterns and data structures at startup
func init() {
	getPortRegex()
	getJavaVersionPatterns()
	getLegacyJavaVersions()
	getAllServerIndicators()
}

// getPortRegex returns a cached, pre-compiled regex for port detection
func getPortRegex() *regexp.Regexp {
	portRegexOnce.Do(func() {
		portRegex = regexp.MustCompile(`(?i)port[^\d]*(\d+)`)
	})
	return portRegex
}

// getJavaVersionPatterns returns cached, pre-compiled regex patterns for Java version detection
// organized by build file type for efficient lookup
func getJavaVersionPatterns() map[string][]*regexp.Regexp {
	javaVersionPatternsOnce.Do(func() {
		javaVersionPatterns = map[string][]*regexp.Regexp{
			"pom.xml": compilePatterns([]string{
				`<maven\.compiler\.target>([^<]+)</maven\.compiler\.target>`,
				`<maven\.compiler\.source>([^<]+)</maven\.compiler\.source>`,
				`<java\.version>([^<]+)</java\.version>`,
				`<version\.java>([^<]+)</version\.java>`,
				`<maven\.compiler\.release>([^<]+)</maven\.compiler\.release>`,
			}),
			"build.gradle": compilePatterns([]string{
				`sourceCompatibility\s*=\s*['"]?([^'"\s,)]+)`,
				`targetCompatibility\s*=\s*['"]?([^'"\s,)]+)`,
				`JavaVersion\.VERSION_(\d+)`,
				`javaVersion\s*=\s*['"]?([^'"\s,)]+)`,
			}),
			"build.gradle.kts": compilePatterns([]string{
				`sourceCompatibility\s*=\s*JavaVersion\.VERSION_(\d+)`,
				`targetCompatibility\s*=\s*JavaVersion\.VERSION_(\d+)`,
				`javaVersion\.set\("([^"]+)"\)`,
			}),
			"gradle.properties": compilePatterns([]string{
				`javaVersion\s*=\s*([^\s#]+)`,
				`java\.version\s*=\s*([^\s#]+)`,
				`sourceCompatibility\s*=\s*([^\s#]+)`,
			}),
		}
	})
	return javaVersionPatterns
}

// compilePatterns pre-compiles regex patterns with case-insensitive flag
func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if re, err := regexp.Compile(`(?i)` + pattern); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// getLegacyJavaVersions returns cached mappings for legacy Java version formats (1.x → x)
func getLegacyJavaVersions() map[string]string {
	legacyJavaVersionsOnce.Do(func() {
		legacyJavaVersions = map[string]string{
			"1.5": "5", "1.6": "6", "1.7": "7", "1.8": "8", "1.9": "9",
			"5.0": "5", "6.0": "6", "7.0": "7", "8.0": "8", "9.0": "9",
			"10.0": "10", "11.0": "11", "12.0": "12", "13.0": "13", "14.0": "14",
			"15.0": "15", "16.0": "16", "17.0": "17", "18.0": "18", "19.0": "19",
			"20.0": "20", "21.0": "21", "22.0": "22", "23.0": "23",
		}
	})
	return legacyJavaVersions
}

// getAllServerIndicators returns a pre-merged map of all server indicators for efficient lookup
func getAllServerIndicators() map[string]string {
	allServerIndicatorsOnce.Do(func() {
		allServerIndicators = make(map[string]string)
		for k, v := range serverIndicators {
			allServerIndicators[k] = v
		}
		for k, v := range dependencyServerIndicators {
			allServerIndicators[k] = v
		}
		for k, v := range mavenServerIndicators {
			allServerIndicators[k] = v
		}
		for k, v := range gradleServerIndicators {
			allServerIndicators[k] = v
		}
	})
	return allServerIndicators
}

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

// RepositoryAnalyzer provides mechanical repository analysis without AI
type RepositoryAnalyzer struct {
	logger *slog.Logger
}

// NewRepositoryAnalyzer creates a new repository analyzer
func NewRepositoryAnalyzer(logger *slog.Logger) *RepositoryAnalyzer { //TODO: Refactor -  we are just wrapping the logger
	return &RepositoryAnalyzer{}
}

// AnalysisResult contains the result of repository analysis
type AnalysisResult struct {
	Success         bool                   `json:"success"`
	Language        string                 `json:"language"`
	LanguageVersion string                 `json:"language_version,omitempty"`
	Framework       string                 `json:"framework,omitempty"`
	Dependencies    []Dependency           `json:"dependencies"`
	ConfigFiles     []ConfigFile           `json:"config_files"`
	Structure       map[string]interface{} `json:"structure"`
	EntryPoints     []string               `json:"entry_points"`
	BuildFiles      []string               `json:"build_files"`
	Port            int                    `json:"port,omitempty"`
	DatabaseInfo    *DatabaseInfo          `json:"database_info,omitempty"`
	Suggestions     []string               `json:"suggestions"`
	Context         map[string]interface{} `json:"context"`
	Error           *AnalysisError         `json:"error,omitempty"`
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

	// Detect language version
	result.LanguageVersion = ra.detectLanguageVersion(repoPath, result.Language)

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

	// Detect application server
	if appServer := ra.detectApplicationServer(repoPath, result.Language, result.Framework, result.Dependencies); appServer != "" {
		if result.Framework == "" {
			result.Framework = appServer
		} else if !strings.Contains(result.Framework, appServer) {
			result.Framework = result.Framework + "-" + appServer
		}
	}

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

// detectApplicationServer detects Java application servers
func (ra *RepositoryAnalyzer) detectApplicationServer(repoPath, language, framework string, dependencies []Dependency) string {
	// Only detect application servers for Java projects
	if language == "java" {
		return ra.detectJavaApplicationServer(repoPath, framework, dependencies)
	}

	return ""
}

// detectJavaApplicationServer detects Java application servers using priority-based detection:
// 1. Dependencies (fastest) 2. Spring Boot detection 3. Config files 4. Build files
func (ra *RepositoryAnalyzer) detectJavaApplicationServer(repoPath, framework string, dependencies []Dependency) string {
	if server := ra.checkDependenciesForServer(dependencies); server != "" {
		return server
	}

	if strings.Contains(framework, "spring") {
		if server := ra.detectSpringBootServer(dependencies); server != "" {
			return server
		}
		return "embedded-tomcat"
	}

	if server := ra.findServerConfigFiles(repoPath); server != "" {
		return server
	}

	if server := ra.checkBuildFilesForServer(repoPath); server != "" {
		return server
	}

	if ra.hasSpringBootIndicators(repoPath) {
		return "embedded-tomcat"
	}

	return ""
}

// findServerConfigFiles searches for server config files in root and common directories
// using priority ordering to check most likely servers first
func (ra *RepositoryAnalyzer) findServerConfigFiles(repoPath string) string {
	for _, config := range serverConfigFiles {
		filePath := filepath.Join(repoPath, config.filename)
		if ra.fileExists(filePath) {
			ra.logger.Info("Detected application server from config file",
				"server", config.server, "file", config.filename)
			return config.server
		}
	}

	for _, config := range serverConfigFiles {
		for _, dir := range commonConfigDirs {
			filePath := filepath.Join(repoPath, dir, config.filename)
			if ra.fileExists(filePath) {
				ra.logger.Info("Detected application server from config file",
					"server", config.server, "file", filepath.Join(dir, config.filename))
				return config.server
			}
		}
	}

	return ""
}

// detectSpringBootServer identifies the embedded server type from Spring Boot dependencies
func (ra *RepositoryAnalyzer) detectSpringBootServer(dependencies []Dependency) string {
	for _, dep := range dependencies {
		switch {
		case strings.Contains(dep.Name, "spring-boot-starter-jetty"):
			return "embedded-jetty"
		case strings.Contains(dep.Name, "spring-boot-starter-undertow"):
			return "embedded-undertow"
		case strings.Contains(dep.Name, "spring-boot-starter-web"):
			return "embedded-tomcat"
		}
	}
	return ""
}

// fileExists efficiently checks if a path exists and is a file (not directory)
func (ra *RepositoryAnalyzer) fileExists(path string) bool {
	if info, err := os.Stat(path); err == nil {
		return !info.IsDir()
	}
	return false
}

// checkBuildFilesForServer scans Maven/Gradle build files for server indicators
func (ra *RepositoryAnalyzer) checkBuildFilesForServer(repoPath string) string {
	buildFiles := []string{"pom.xml", "build.gradle", "build.gradle.kts"}

	for _, buildFile := range buildFiles {
		if server := ra.scanFileForServerIndicators(repoPath, buildFile); server != "" {
			return server
		}
	}
	return ""
}

// scanFileForServerIndicators performs size-aware scanning of build files for server keywords
func (ra *RepositoryAnalyzer) scanFileForServerIndicators(repoPath, fileName string) string {
	filePath := filepath.Join(repoPath, fileName)

	info, err := os.Stat(filePath)
	if err != nil {
		return ""
	}

	if info.Size() > maxFileSize {
		ra.logger.Debug("Skipping large build file", "file", fileName, "size", info.Size())
		return ""
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	contentStr := strings.ToLower(string(content))
	allIndicators := getAllServerIndicators()

	for indicator, serverType := range allIndicators {
		if strings.Contains(contentStr, strings.ToLower(indicator)) {
			ra.logger.Info("Detected application server from build file",
				"server", serverType, "file", fileName, "indicator", indicator)
			return serverType
		}
	}
	return ""
}

// checkDependenciesForServer performs two-phase lookup: exact match then substring matching
func (ra *RepositoryAnalyzer) checkDependenciesForServer(dependencies []Dependency) string {
	allIndicators := getAllServerIndicators()

	for _, dep := range dependencies {
		depName := strings.ToLower(dep.Name)

		if serverType, exists := allIndicators[depName]; exists {
			ra.logger.Info("Detected application server from dependency (exact match)",
				"server", serverType, "dependency", dep.Name)
			return serverType
		}

		for indicator, serverType := range allIndicators {
			if indicator != depName && strings.Contains(depName, strings.ToLower(indicator)) {
				ra.logger.Info("Detected application server from dependency (substring match)",
					"server", serverType, "dependency", dep.Name, "indicator", indicator)
				return serverType
			}
		}
	}
	return ""
}

// hasSpringBootIndicators uses two-phase detection: config files then main class scanning
func (ra *RepositoryAnalyzer) hasSpringBootIndicators(repoPath string) bool {
	configFiles := []string{
		"application.properties", "application.yml", "application.yaml",
		"src/main/resources/application.properties",
		"src/main/resources/application.yml",
		"src/main/resources/application.yaml",
	}

	for _, configFile := range configFiles {
		if ra.fileExists(filepath.Join(repoPath, configFile)) {
			ra.logger.Debug("Found Spring Boot config file", "file", configFile)
			return true
		}
	}

	javaSourceDir := filepath.Join(repoPath, "src/main/java")
	if _, err := os.Stat(javaSourceDir); err != nil {
		return false
	}

	found := ra.findSpringBootMainClass(javaSourceDir)
	if found {
		ra.logger.Debug("Found Spring Boot main class")
	}
	return found
}

// findSpringBootMainClass uses WalkDir with depth limiting to find @SpringBootApplication
func (ra *RepositoryAnalyzer) findSpringBootMainClass(javaDir string) bool {
	found := false
	depth := 0

	err := filepath.WalkDir(javaDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if depth > maxPathDepth {
				return filepath.SkipDir
			}
			depth++
			return nil
		}

		if !strings.HasSuffix(path, "Application.java") {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize {
			return nil
		}

		if ra.hasSpringBootAnnotation(path) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		ra.logger.Debug("Error walking Java source directory", "error", err)
	}
	return found
}

// hasSpringBootAnnotation checks for Spring Boot annotations in Java files
func (ra *RepositoryAnalyzer) hasSpringBootAnnotation(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	contentStr := string(content)
	return strings.Contains(contentStr, "@SpringBootApplication") ||
		strings.Contains(contentStr, "@EnableAutoConfiguration")
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
			}
			continue
		}

		// Skip directories
		if info.IsDir() {
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
				// Still include the file even if parsing failed
			} else {
				configFile.Content = content
			}
		}

		configFiles = append(configFiles, configFile)
	}

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
					continue
				}
				for _, match := range matches {
					relPath, err := filepath.Rel(repoPath, match)
					if err != nil {
						continue
					}
					entryPoints = append(entryPoints, relPath)
				}
			} else {
				filePath := filepath.Join(repoPath, pattern)
				info, err := os.Stat(filePath)
				if err != nil {
					if !os.IsNotExist(err) {
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
	}

	return entryPoints
}

// findBuildFiles finds build-related files
func (ra *RepositoryAnalyzer) findBuildFiles(repoPath string) []string {
	buildFiles := make([]string, 0)

	buildPatterns := []string{
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
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check if file is empty
	if len(content) == 0 {
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

// extractPortFromFile searches for port numbers using cached regex with size validation
func (ra *RepositoryAnalyzer) extractPortFromFile(filePath string) int {
	info, err := os.Stat(filePath)
	if err != nil || info.Size() > maxFileSize {
		return 0
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}

	portRegex := getPortRegex()
	if matches := portRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
		if port := parseInt(matches[1]); port > 0 {
			return port
		}
	}
	return 0
}

// extractPortFromPackageJson parses package.json for port configuration in scripts and config
func (ra *RepositoryAnalyzer) extractPortFromPackageJson(filePath string) int {
	info, err := os.Stat(filePath)
	if err != nil {
		ra.logger.Debug("Failed to stat package.json file", "error", err)
		return 0
	}
	if info.Size() > maxFileSize {
		ra.logger.Debug("Skipping large package.json for port detection", "size", info.Size())
		return 0
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
		Config  map[string]any    `json:"config"`
	}

	if err := json.Unmarshal(content, &pkg); err != nil {
		return 0
	}

	portRegex := regexp.MustCompile(`(?:--port|PORT=|:)[\s=]*(\d+)`)

	for _, script := range pkg.Scripts {
		if matches := portRegex.FindStringSubmatch(script); len(matches) > 1 {
			if port := parseInt(matches[1]); port > 0 {
				return port
			}
		}
	}

	if port, ok := pkg.Config["port"]; ok {
		switch v := port.(type) {
		case float64:
			if portInt := int(v); portInt > 0 && portInt <= maxPortNumber {
				return portInt
			}
		case string:
			if port := parseInt(v); port > 0 {
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

// detectLanguageVersion detects the version of the primary language
func (ra *RepositoryAnalyzer) detectLanguageVersion(repoPath, language string) string {
	if strings.ToLower(language) == "java" {
		return ra.detectJavaVersion(repoPath)
	}
	return ""
}

// detectJavaVersion scans build files in priority order using pre-compiled regex patterns
func (ra *RepositoryAnalyzer) detectJavaVersion(repoPath string) string {
	patterns := getJavaVersionPatterns()
	fileOrder := []string{"pom.xml", "gradle.properties", "build.gradle", "build.gradle.kts"}

	for _, fileName := range fileOrder {
		if compiledPatterns, exists := patterns[fileName]; exists {
			if version := ra.extractJavaVersionFromFileOptimized(repoPath, fileName, compiledPatterns); version != "" {
				return ra.normalizeJavaVersionOptimized(version)
			}
		}
	}
	return defaultJavaVersion
}

// extractJavaVersionFromFileOptimized applies pre-compiled patterns with size validation
func (ra *RepositoryAnalyzer) extractJavaVersionFromFileOptimized(repoPath, fileName string, patterns []*regexp.Regexp) string {
	filePath := filepath.Join(repoPath, fileName)

	info, err := os.Stat(filePath)
	if err != nil {
		return ""
	}

	if info.Size() > maxFileSize {
		ra.logger.Debug("Skipping large build file for version detection",
			"file", fileName, "size", info.Size())
		return ""
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	contentStr := string(content)
	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(contentStr); len(matches) >= 2 {
			version := strings.TrimSpace(matches[1])
			if version != "" {
				ra.logger.Debug("Found Java version", "file", fileName, "version", version)
				return version
			}
		}
	}
	return ""
}

// normalizeJavaVersionOptimized converts legacy formats (1.x → x) using cached mappings
func (ra *RepositoryAnalyzer) normalizeJavaVersionOptimized(version string) string {
	version = strings.TrimSpace(version)
	version = strings.Trim(version, `"'`)

	legacyMappings := getLegacyJavaVersions()
	if normalized, exists := legacyMappings[version]; exists {
		return normalized
	}

	if parts := strings.Split(version, "."); len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return version
}

// parseInt converts string to int with port range validation
func parseInt(s string) int {
	if s == "" {
		return 0
	}

	s = strings.TrimSpace(s)

	// Find the end of the numeric prefix
	end := 0
	for i, r := range s {
		if r < '0' || r > '9' {
			break
		}
		end = i + 1
	}

	if end == 0 {
		return 0
	}

	result, err := strconv.Atoi(s[:end])
	if err != nil || result < 0 || result > maxPortNumber {
		return 0
	}
	return result
}
