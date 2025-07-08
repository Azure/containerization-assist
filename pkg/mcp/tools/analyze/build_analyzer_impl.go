package analyze

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// analyzeBuildSystems detects different build systems
func (b *BuildAnalyzer) analyzeBuildSystems(config AnalysisConfig, result *EngineAnalysisResult) {
	buildSystems := map[string]string{
		"Makefile":            "make",
		"makefile":            "make",
		"build.gradle":        "gradle",
		"pom.xml":             "maven",
		"package.json":        "npm",
		"Dockerfile":          "docker",
		"docker-compose.yml":  "docker-compose",
		"docker-compose.yaml": "docker-compose",
		"CMakeLists.txt":      "cmake",
		"meson.build":         "meson",
		"SConstruct":          "scons",
		"setup.py":            "python-setuptools",
		"pyproject.toml":      "python-modern",
		"Cargo.toml":          "cargo",
		"go.mod":              "go-modules",
	}

	for _, file := range config.RepoData.Files {
		for buildFile, system := range buildSystems {
			if strings.HasSuffix(file.Path, buildFile) || filepath.Base(file.Path) == buildFile {
				b.addBuildSystemFinding(file, system, result)
				break
			}
		}
	}
}

// analyzeEntryPoints finds main application entry points
func (b *BuildAnalyzer) analyzeEntryPoints(config AnalysisConfig, result *EngineAnalysisResult) {
	entryPointPatterns := map[*regexp.Regexp]string{
		regexp.MustCompile(`(?i)if\s+__name__\s*==\s*["']__main__["']`): "python",
		regexp.MustCompile(`(?i)func\s+main\s*\(\s*\)`):                 "go",
		regexp.MustCompile(`(?i)public\s+static\s+void\s+main`):         "java",
		regexp.MustCompile(`(?i)int\s+main\s*\(`):                       "c_cpp",
	}

	for _, file := range config.RepoData.Files {
		for pattern, language := range entryPointPatterns {
			if pattern.MatchString(file.Content) {
				b.addEntryPointFinding(file, language, result)
			}
		}
	}

	// Check for common entry point files
	entryPointFiles := []string{
		"main.go", "main.py", "main.js", "main.ts", "index.js", "index.ts",
		"app.py", "app.js", "server.js", "server.py", "run.py",
	}

	for _, file := range config.RepoData.Files {
		baseName := filepath.Base(file.Path)
		for _, entryFile := range entryPointFiles {
			if baseName == entryFile {
				b.addEntryPointFileFinding(file, result)
				break
			}
		}
	}
}

// analyzeBuildScripts analyzes build and deployment scripts
func (b *BuildAnalyzer) analyzeBuildScripts(config AnalysisConfig, result *EngineAnalysisResult) {
	scriptPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(build|compile|deploy|install|test)\.sh`),
		regexp.MustCompile(`(?i)(build|compile|deploy|install|test)\.bat`),
		regexp.MustCompile(`(?i)(build|compile|deploy|install|test)\.ps1`),
	}

	for _, file := range config.RepoData.Files {
		for _, pattern := range scriptPatterns {
			if pattern.MatchString(file.Path) {
				b.addBuildScriptFinding(file, result)
				b.analyzeBuildScriptContent(file, result)
				break
			}
		}
	}
}

// analyzeCICDConfiguration detects CI/CD configuration files
func (b *BuildAnalyzer) analyzeCICDConfiguration(config AnalysisConfig, result *EngineAnalysisResult) {
	cicdFiles := map[string]string{
		".github/workflows":    "github-actions",
		".gitlab-ci.yml":       "gitlab-ci",
		"azure-pipelines.yml":  "azure-devops",
		"Jenkinsfile":          "jenkins",
		".travis.yml":          "travis-ci",
		"circle.yml":           "circleci",
		".circleci/config.yml": "circleci",
		"buildkite.yml":        "buildkite",
	}

	for _, file := range config.RepoData.Files {
		for cicdPath, system := range cicdFiles {
			if strings.Contains(file.Path, cicdPath) {
				b.addCICDFinding(file, system, result)
				break
			}
		}
	}
}

// Helper methods for adding findings

func (b *BuildAnalyzer) addBuildSystemFinding(file FileData, system string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeBuild,
		Category:    "build_system",
		Title:       fmt.Sprintf("Build System: %s", strings.Title(system)),
		Description: fmt.Sprintf("Found %s build system configuration", system),
		Confidence:  0.9,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"build_system": system,
			"file_name":    filepath.Base(file.Path),
			"file_size":    file.Size,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (b *BuildAnalyzer) addEntryPointFinding(file FileData, language string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeEntrypoint,
		Category:    "entry_point_pattern",
		Title:       fmt.Sprintf("Entry Point Pattern: %s", strings.Title(language)),
		Description: fmt.Sprintf("Found %s main function pattern", language),
		Confidence:  0.8,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"language":     language,
			"pattern_type": "main_function",
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (b *BuildAnalyzer) addEntryPointFileFinding(file FileData, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeEntrypoint,
		Category:    "entry_point_file",
		Title:       fmt.Sprintf("Entry Point File: %s", filepath.Base(file.Path)),
		Description: "Found potential application entry point file",
		Confidence:  0.75,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"file_name": filepath.Base(file.Path),
			"file_ext":  filepath.Ext(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (b *BuildAnalyzer) addBuildScriptFinding(file FileData, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeBuild,
		Category:    "build_script",
		Title:       fmt.Sprintf("Build Script: %s", filepath.Base(file.Path)),
		Description: "Found build or deployment script",
		Confidence:  0.85,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"script_name": filepath.Base(file.Path),
			"script_type": filepath.Ext(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (b *BuildAnalyzer) analyzeBuildScriptContent(file FileData, result *EngineAnalysisResult) {
	// Look for common build commands
	buildCommands := []string{
		"npm install", "npm run build", "npm test",
		"go build", "go test", "go mod",
		"make", "cmake", "gradle build",
		"mvn compile", "mvn package", "mvn test",
		"python setup.py", "pip install",
		"docker build", "docker run",
	}

	commandsFound := []string{}
	for _, command := range buildCommands {
		if strings.Contains(file.Content, command) {
			commandsFound = append(commandsFound, command)
		}
	}

	if len(commandsFound) > 0 {
		finding := Finding{
			Type:        FindingTypeBuild,
			Category:    "build_commands",
			Title:       fmt.Sprintf("Build Commands Found: %d", len(commandsFound)),
			Description: "Analyzed build commands in script",
			Confidence:  0.8,
			Severity:    SeverityInfo,
			Location: &Location{
				Path: file.Path,
			},
			Metadata: map[string]interface{}{
				"commands_found": commandsFound,
				"command_count":  len(commandsFound),
			},
		}
		result.Findings = append(result.Findings, finding)
	}
}

func (b *BuildAnalyzer) addCICDFinding(file FileData, system string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeBuild,
		Category:    "cicd_configuration",
		Title:       fmt.Sprintf("CI/CD Configuration: %s", strings.Title(system)),
		Description: fmt.Sprintf("Found %s CI/CD configuration", system),
		Confidence:  0.9,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"cicd_system": system,
			"file_path":   file.Path,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (b *BuildAnalyzer) countBuildFiles(config AnalysisConfig) int {
	buildFiles := []string{
		"Makefile", "makefile", "build.gradle", "pom.xml", "package.json",
		"Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		"CMakeLists.txt", "meson.build", "SConstruct", "setup.py",
		"pyproject.toml", "Cargo.toml", "go.mod",
	}

	count := 0
	for _, file := range config.RepoData.Files {
		baseName := filepath.Base(file.Path)
		for _, buildFile := range buildFiles {
			if baseName == buildFile {
				count++
				break
			}
		}
	}
	return count
}
