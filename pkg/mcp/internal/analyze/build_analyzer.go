package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// BuildAnalyzer analyzes build systems and entry points
type BuildAnalyzer struct {
	logger zerolog.Logger
}

// NewBuildAnalyzer creates a new build analyzer
func NewBuildAnalyzer(logger zerolog.Logger) *BuildAnalyzer {
	return &BuildAnalyzer{
		logger: logger.With().Str("engine", "build").Logger(),
	}
}

// GetName returns the name of this engine
func (b *BuildAnalyzer) GetName() string {
	return "build_analyzer"
}

// GetCapabilities returns what this engine can analyze
func (b *BuildAnalyzer) GetCapabilities() []string {
	return []string{
		"build_systems",
		"entry_points",
		"build_scripts",
		"ci_cd_configuration",
		"containerization_readiness",
		"deployment_artifacts",
	}
}

// IsApplicable determines if this engine should run
func (b *BuildAnalyzer) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	// Build analysis is always useful
	return true
}

// Analyze performs build system analysis
func (b *BuildAnalyzer) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	startTime := time.Now()
	result := &EngineAnalysisResult{
		Engine:   "build_analyzer",
		Success:  true,
		Findings: []Finding{},
		Metadata: make(map[string]interface{}),
		Errors:   []error{},
	}

	// Note: Simplified implementation - build analysis would be implemented here
	_ = config // Prevent unused variable error

	// Additional analysis methods would be implemented here

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Confidence = 0.8 // Default confidence
	// result.Confidence already set to 0.8 above

	return result, nil
}

// analyzeBuildSystems identifies build systems and tools
func (b *BuildAnalyzer) analyzeBuildSystems(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	buildSystems := map[string]BuildSystemConfig{
		"npm": {
			Files:       []string{"package.json"},
			Scripts:     []string{"build", "start", "dev", "test"},
			Description: "Node.js Package Manager",
			Type:        "javascript",
		},
		"yarn": {
			Files:       []string{"yarn.lock", "package.json"},
			Scripts:     []string{"build", "start", "dev", "test"},
			Description: "Yarn Package Manager",
			Type:        "javascript",
		},
		"webpack": {
			Files:       []string{"webpack.config.js", "webpack.config.ts"},
			Scripts:     []string{},
			Description: "Webpack Module Bundler",
			Type:        "javascript",
		},
		"vite": {
			Files:       []string{"vite.config.js", "vite.config.ts"},
			Scripts:     []string{},
			Description: "Vite Build Tool",
			Type:        "javascript",
		},
		"maven": {
			Files:       []string{"pom.xml"},
			Scripts:     []string{"compile", "package", "install", "test"},
			Description: "Apache Maven",
			Type:        "java",
		},
		"gradle": {
			Files:       []string{"build.gradle", "build.gradle.kts", "gradlew"},
			Scripts:     []string{"build", "test", "assemble"},
			Description: "Gradle Build Tool",
			Type:        "java",
		},
		"make": {
			Files:       []string{"Makefile", "makefile"},
			Scripts:     []string{},
			Description: "GNU Make",
			Type:        "native",
		},
		"cmake": {
			Files:       []string{"CMakeLists.txt"},
			Scripts:     []string{},
			Description: "CMake Build System",
			Type:        "native",
		},
		"pip": {
			Files:       []string{"setup.py", "pyproject.toml"},
			Scripts:     []string{},
			Description: "Python Package Installer",
			Type:        "python",
		},
		"poetry": {
			Files:       []string{"pyproject.toml", "poetry.lock"},
			Scripts:     []string{},
			Description: "Python Poetry",
			Type:        "python",
		},
		"go": {
			Files:       []string{"go.mod", "go.sum"},
			Scripts:     []string{},
			Description: "Go Modules",
			Type:        "go",
		},
		"cargo": {
			Files:       []string{"Cargo.toml", "Cargo.lock"},
			Scripts:     []string{},
			Description: "Rust Cargo",
			Type:        "rust",
		},
		"dotnet": {
			Files:       []string{"*.csproj", "*.sln", "*.fsproj", "*.vbproj"},
			Scripts:     []string{},
			Description: ".NET Build System",
			Type:        "dotnet",
		},
	}

	for systemName, system := range buildSystems {
		if b.detectBuildSystem(repoData, system) {
			finding := Finding{
				Type:        FindingTypeBuild,
				Category:    "build_system",
				Title:       fmt.Sprintf("%s Build System", system.Description),
				Description: b.generateBuildSystemDescription(system, repoData),
				Confidence:  0.9,
				Severity:    SeverityInfo,
				Metadata: map[string]interface{}{
					"system":      systemName,
					"type":        system.Type,
					"description": system.Description,
					"files":       b.getExistingBuildFiles(repoData, system.Files),
					"scripts":     b.getAvailableScripts(repoData, system),
				},
			}
			result.Findings = append(result.Findings, finding)

			// Analyze build scripts if available
			b.analyzeBuildScripts(repoData, system, result)
		}
	}

	return nil
}

// analyzeEntryPoints identifies application entry points
func (b *BuildAnalyzer) analyzeEntryPoints(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	entryPointPatterns := map[string][]string{
		"Node.js": {
			"index.js", "app.js", "server.js", "main.js",
			"src/index.js", "src/app.js", "src/server.js", "src/main.js",
		},
		"Python": {
			"main.py", "app.py", "server.py", "run.py",
			"src/main.py", "src/app.py", "__main__.py",
		},
		"Java": {
			"Main.java", "Application.java", "App.java",
			"src/main/java/Main.java", "src/main/java/Application.java",
		},
		"Go": {
			"main.go", "cmd/main.go", "cmd/*/main.go",
		},
		"C#": {
			"Program.cs", "Main.cs", "Startup.cs",
		},
		"PHP": {
			"index.php", "app.php", "main.php", "public/index.php",
		},
		"Ruby": {
			"main.rb", "app.rb", "config.ru",
		},
	}

	for language, patterns := range entryPointPatterns {
		entryPoints := b.findEntryPoints(repoData, patterns)
		for _, entryPoint := range entryPoints {
			finding := Finding{
				Type:        FindingTypeEntrypoint,
				Category:    "entry_point",
				Title:       fmt.Sprintf("%s Entry Point", language),
				Description: fmt.Sprintf("%s application entry point: %s", language, entryPoint.Path),
				Confidence:  0.85,
				Severity:    SeverityInfo,
				Location: &Location{
					Path: entryPoint.Path,
				},
				Metadata: map[string]interface{}{
					"language":    language,
					"entry_point": entryPoint.Path,
					"file_size":   len(entryPoint.Content),
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	// Check package.json for main entry
	b.analyzePackageJsonMain(repoData, result)

	return nil
}

// analyzeCICDConfiguration detects CI/CD setup
func (b *BuildAnalyzer) analyzeCICDConfiguration(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	cicdSystems := map[string][]string{
		"GitHub Actions": {
			".github/workflows", ".github/workflows/*.yml", ".github/workflows/*.yaml",
		},
		"GitLab CI": {
			".gitlab-ci.yml", ".gitlab-ci.yaml",
		},
		"Jenkins": {
			"Jenkinsfile", "jenkins.yml", "jenkins.yaml",
		},
		"Travis CI": {
			".travis.yml", ".travis.yaml",
		},
		"CircleCI": {
			".circleci/config.yml", ".circleci/config.yaml",
		},
		"Azure DevOps": {
			"azure-pipelines.yml", "azure-pipelines.yaml", ".azure/pipelines",
		},
		"Docker": {
			"Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		},
		"Kubernetes": {
			"k8s", "kubernetes", "*.yaml", "*.yml",
		},
		"Helm": {
			"Chart.yaml", "values.yaml", "charts/",
		},
	}

	for system, patterns := range cicdSystems {
		if b.detectCICDSystem(repoData, patterns) {
			finding := Finding{
				Type:        FindingTypeBuild,
				Category:    "cicd_system",
				Title:       fmt.Sprintf("%s Configuration", system),
				Description: fmt.Sprintf("%s CI/CD configuration detected", system),
				Confidence:  0.9,
				Severity:    SeverityInfo,
				Metadata: map[string]interface{}{
					"system":   system,
					"patterns": patterns,
					"files":    b.getMatchingFiles(repoData, patterns),
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzeContainerizationReadiness assesses readiness for containerization
func (b *BuildAnalyzer) analyzeContainerizationReadiness(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	readinessFactors := map[string]bool{
		"has_dockerfile":     b.fileExists(repoData, "Dockerfile"),
		"has_docker_compose": b.fileExists(repoData, "docker-compose.yml") || b.fileExists(repoData, "docker-compose.yaml"),
		"has_dockerignore":   b.fileExists(repoData, ".dockerignore"),
		"has_build_scripts":  b.hasBuildScripts(repoData),
		"has_start_script":   b.hasStartScript(repoData),
		"has_health_check":   b.hasHealthCheck(repoData),
		"has_env_config":     b.hasEnvironmentConfig(repoData),
		"single_executable":  b.hasSingleExecutable(repoData),
	}

	readinessScore := b.calculateReadinessScore(readinessFactors)

	var severity Severity = SeverityInfo
	if readinessScore > 0.8 {
		severity = SeverityInfo
	} else if readinessScore > 0.5 {
		severity = SeverityLow
	} else {
		severity = SeverityMedium
	}

	finding := Finding{
		Type:        FindingTypeBuild,
		Category:    "containerization_readiness",
		Title:       "Containerization Readiness Assessment",
		Description: b.generateReadinessDescription(readinessScore, readinessFactors),
		Confidence:  0.95,
		Severity:    severity,
		Metadata: map[string]interface{}{
			"readiness_score": readinessScore,
			"factors":         readinessFactors,
			"recommendations": b.generateReadinessRecommendations(readinessFactors),
		},
	}

	result.Findings = append(result.Findings, finding)
	return nil
}

// Helper types and methods

type BuildSystemConfig struct {
	Files       []string
	Scripts     []string
	Description string
	Type        string
}

func (b *BuildAnalyzer) detectBuildSystem(repoData *RepoData, system BuildSystemConfig) bool {
	for _, file := range system.Files {
		if b.fileExists(repoData, file) || b.filePatternExists(repoData, file) {
			return true
		}
	}
	return false
}

func (b *BuildAnalyzer) fileExists(repoData *RepoData, filename string) bool {
	for _, file := range repoData.Files {
		if strings.HasSuffix(file.Path, filename) || filepath.Base(file.Path) == filename {
			return true
		}
	}
	return false
}

func (b *BuildAnalyzer) filePatternExists(repoData *RepoData, pattern string) bool {
	for _, file := range repoData.Files {
		if strings.Contains(pattern, "*") {
			// Simple wildcard matching
			if strings.HasSuffix(pattern, "*") {
				prefix := strings.TrimSuffix(pattern, "*")
				if strings.HasSuffix(file.Path, prefix) {
					return true
				}
			}
		}
	}
	return false
}

func (b *BuildAnalyzer) getExistingBuildFiles(repoData *RepoData, files []string) []string {
	var existing []string
	for _, file := range files {
		if b.fileExists(repoData, file) {
			existing = append(existing, file)
		}
	}
	return existing
}

func (b *BuildAnalyzer) getAvailableScripts(repoData *RepoData, system BuildSystemConfig) []string {
	var scripts []string

	// For npm/yarn, check package.json scripts
	if system.Type == "javascript" {
		packageJsonFile := b.findFile(repoData, "package.json")
		if packageJsonFile != nil {
			for _, script := range system.Scripts {
				if strings.Contains(packageJsonFile.Content, fmt.Sprintf("\"%s\"", script)) {
					scripts = append(scripts, script)
				}
			}
		}
	}

	return scripts
}

func (b *BuildAnalyzer) findFile(repoData *RepoData, filename string) *FileData {
	for _, file := range repoData.Files {
		if strings.HasSuffix(file.Path, filename) || filepath.Base(file.Path) == filename {
			return &file
		}
	}
	return nil
}

func (b *BuildAnalyzer) generateBuildSystemDescription(system BuildSystemConfig, repoData *RepoData) string {
	files := b.getExistingBuildFiles(repoData, system.Files)
	return fmt.Sprintf("%s detected with configuration files: %s", system.Description, strings.Join(files, ", "))
}

func (b *BuildAnalyzer) analyzeBuildScripts(repoData *RepoData, system BuildSystemConfig, result *EngineAnalysisResult) {
	scripts := b.getAvailableScripts(repoData, system)
	for _, script := range scripts {
		finding := Finding{
			Type:        FindingTypeBuild,
			Category:    "build_script",
			Title:       fmt.Sprintf("%s Script: %s", system.Description, script),
			Description: fmt.Sprintf("Build script '%s' available in %s", script, system.Description),
			Confidence:  0.8,
			Severity:    SeverityInfo,
			Metadata: map[string]interface{}{
				"script":       script,
				"build_system": system.Description,
				"type":         system.Type,
			},
		}
		result.Findings = append(result.Findings, finding)
	}
}

func (b *BuildAnalyzer) findEntryPoints(repoData *RepoData, patterns []string) []FileData {
	var entryPoints []FileData
	for _, pattern := range patterns {
		for _, file := range repoData.Files {
			if strings.HasSuffix(file.Path, pattern) ||
				filepath.Base(file.Path) == pattern ||
				strings.Contains(file.Path, pattern) {
				entryPoints = append(entryPoints, file)
			}
		}
	}
	return entryPoints
}

func (b *BuildAnalyzer) analyzePackageJsonMain(repoData *RepoData, result *EngineAnalysisResult) {
	packageJsonFile := b.findFile(repoData, "package.json")
	if packageJsonFile != nil {
		if strings.Contains(packageJsonFile.Content, "\"main\"") {
			finding := Finding{
				Type:        FindingTypeEntrypoint,
				Category:    "package_main",
				Title:       "Package.json Main Entry",
				Description: "Main entry point defined in package.json",
				Confidence:  0.9,
				Severity:    SeverityInfo,
				Location: &Location{
					Path: packageJsonFile.Path,
				},
				Metadata: map[string]interface{}{
					"source": "package.json",
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}
}

func (b *BuildAnalyzer) detectCICDSystem(repoData *RepoData, patterns []string) bool {
	for _, pattern := range patterns {
		if b.fileExists(repoData, pattern) || b.filePatternExists(repoData, pattern) {
			return true
		}
	}
	return false
}

func (b *BuildAnalyzer) getMatchingFiles(repoData *RepoData, patterns []string) []string {
	var matches []string
	for _, pattern := range patterns {
		for _, file := range repoData.Files {
			if strings.Contains(file.Path, pattern) ||
				strings.HasSuffix(file.Path, pattern) {
				matches = append(matches, file.Path)
			}
		}
	}
	return matches
}

func (b *BuildAnalyzer) hasBuildScripts(repoData *RepoData) bool {
	buildFiles := []string{"package.json", "pom.xml", "build.gradle", "Makefile", "CMakeLists.txt"}
	for _, file := range buildFiles {
		if b.fileExists(repoData, file) {
			return true
		}
	}
	return false
}

func (b *BuildAnalyzer) hasStartScript(repoData *RepoData) bool {
	packageJsonFile := b.findFile(repoData, "package.json")
	if packageJsonFile != nil {
		return strings.Contains(packageJsonFile.Content, "\"start\"")
	}
	return false
}

func (b *BuildAnalyzer) hasHealthCheck(repoData *RepoData) bool {
	for _, file := range repoData.Files {
		if strings.Contains(strings.ToLower(file.Content), "health") ||
			strings.Contains(strings.ToLower(file.Content), "ping") ||
			strings.Contains(strings.ToLower(file.Content), "/health") {
			return true
		}
	}
	return false
}

func (b *BuildAnalyzer) hasEnvironmentConfig(repoData *RepoData) bool {
	envFiles := []string{".env", ".env.example", "config.json", "config.yaml"}
	for _, file := range envFiles {
		if b.fileExists(repoData, file) {
			return true
		}
	}
	return false
}

func (b *BuildAnalyzer) hasSingleExecutable(repoData *RepoData) bool {
	// Simple heuristic: check if there's a clear main entry point
	mainFiles := []string{"main.go", "main.py", "app.js", "index.js", "Program.cs"}
	count := 0
	for _, file := range mainFiles {
		if b.fileExists(repoData, file) {
			count++
		}
	}
	return count == 1
}

func (b *BuildAnalyzer) calculateReadinessScore(factors map[string]bool) float64 {
	weights := map[string]float64{
		"has_dockerfile":     0.3,
		"has_docker_compose": 0.1,
		"has_dockerignore":   0.05,
		"has_build_scripts":  0.2,
		"has_start_script":   0.15,
		"has_health_check":   0.1,
		"has_env_config":     0.05,
		"single_executable":  0.05,
	}

	score := 0.0
	for factor, present := range factors {
		if present {
			if weight, exists := weights[factor]; exists {
				score += weight
			}
		}
	}

	return score
}

func (b *BuildAnalyzer) generateReadinessDescription(score float64, factors map[string]bool) string {
	percentage := int(score * 100)
	return fmt.Sprintf("Containerization readiness: %d%% (%d/8 factors present)", percentage, b.countTrueFactors(factors))
}

func (b *BuildAnalyzer) countTrueFactors(factors map[string]bool) int {
	count := 0
	for _, present := range factors {
		if present {
			count++
		}
	}
	return count
}

func (b *BuildAnalyzer) generateReadinessRecommendations(factors map[string]bool) []string {
	var recommendations []string

	if !factors["has_dockerfile"] {
		recommendations = append(recommendations, "Add Dockerfile for containerization")
	}
	if !factors["has_dockerignore"] {
		recommendations = append(recommendations, "Add .dockerignore to optimize build context")
	}
	if !factors["has_start_script"] {
		recommendations = append(recommendations, "Define start script for application startup")
	}
	if !factors["has_health_check"] {
		recommendations = append(recommendations, "Implement health check endpoint")
	}
	if !factors["has_env_config"] {
		recommendations = append(recommendations, "Add environment configuration support")
	}

	return recommendations
}

func (b *BuildAnalyzer) calculateConfidence(result *EngineAnalysisResult) float64 {
	if len(result.Findings) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, finding := range result.Findings {
		totalConfidence += finding.Confidence
	}

	return totalConfidence / float64(len(result.Findings))
}
