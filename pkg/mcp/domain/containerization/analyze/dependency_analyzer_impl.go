package analyze

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// analyzePackageFiles analyzes different types of dependency files
func (d *DependencyAnalyzer) analyzePackageFiles(config AnalysisConfig, result *EngineAnalysisResult) {
	dependencyFiles := map[string]string{
		"package.json":      "node",
		"yarn.lock":         "node",
		"package-lock.json": "node",
		"requirements.txt":  "python",
		"Pipfile":           "python",
		"poetry.lock":       "python",
		"pyproject.toml":    "python",
		"go.mod":            "go",
		"go.sum":            "go",
		"pom.xml":           "java",
		"build.gradle":      "java",
		"Gemfile":           "ruby",
		"composer.json":     "php",
		"Cargo.toml":        "rust",
		"Cargo.lock":        "rust",
	}

	for _, file := range config.RepoData.Files {
		for depFile, ecosystem := range dependencyFiles {
			if strings.HasSuffix(file.Path, depFile) || filepath.Base(file.Path) == depFile {
				d.addDependencyFileFinding(file, ecosystem, result)
				d.analyzeDependencyFileContent(file, ecosystem, result)
				break
			}
		}
	}
}

// analyzeDependencyVersions analyzes dependency versions for outdated packages
func (d *DependencyAnalyzer) analyzeDependencyVersions(config AnalysisConfig, result *EngineAnalysisResult) {
	versionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`"([^"]+)":\s*"([^"]+)"`), // JSON format
		regexp.MustCompile(`([^=\s]+)==([^\s]+)`),    // requirements.txt format
		regexp.MustCompile(`([^@\s]+)@([^\s]+)`),     // package@version format
	}

	for _, file := range config.RepoData.Files {
		if d.isDependencyFile(file.Path) {
			for _, pattern := range versionPatterns {
				matches := pattern.FindAllStringSubmatch(file.Content, -1)
				for _, match := range matches {
					if len(match) > 2 {
						d.addVersionFinding(file, match[1], match[2], result)
					}
				}
			}
		}
	}
}

// analyzeSecurityVulnerabilities looks for known vulnerable dependency patterns
func (d *DependencyAnalyzer) analyzeSecurityVulnerabilities(config AnalysisConfig, result *EngineAnalysisResult) {
	// Common vulnerable package patterns
	vulnerablePatterns := map[string][]string{
		"node": {
			"lodash.*<4.17.12",
			"minimist.*<1.2.3",
			"serialize-javascript.*<3.1.0",
		},
		"python": {
			"requests.*<2.25.0",
			"pyyaml.*<5.4.0",
			"django.*<3.2.0",
		},
	}

	for _, file := range config.RepoData.Files {
		ecosystem := d.getEcosystem(file.Path)
		if patterns, ok := vulnerablePatterns[ecosystem]; ok {
			for _, pattern := range patterns {
				re := regexp.MustCompile(pattern)
				if re.MatchString(file.Content) {
					d.addVulnerabilityFinding(file, pattern, result)
				}
			}
		}
	}
}

// Helper methods

func (d *DependencyAnalyzer) addDependencyFileFinding(file FileData, ecosystem string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeDependency,
		Category:    "dependency_file",
		Title:       fmt.Sprintf("%s Dependency File: %s", strings.Title(ecosystem), filepath.Base(file.Path)),
		Description: fmt.Sprintf("Found %s dependency management file", ecosystem),
		Confidence:  0.95,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"ecosystem": ecosystem,
			"file_type": filepath.Ext(file.Path),
			"file_size": file.Size,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (d *DependencyAnalyzer) analyzeDependencyFileContent(file FileData, ecosystem string, result *EngineAnalysisResult) {
	// Count dependencies
	dependencyCount := 0
	devDependencyCount := 0

	switch ecosystem {
	case "node":
		dependencyCount += strings.Count(file.Content, `"dependencies"`)
		devDependencyCount += strings.Count(file.Content, `"devDependencies"`)
	case "python":
		if strings.HasSuffix(file.Path, "requirements.txt") {
			lines := strings.Split(file.Content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					dependencyCount++
				}
			}
		}
	}

	if dependencyCount > 0 || devDependencyCount > 0 {
		finding := Finding{
			Type:        FindingTypeDependency,
			Category:    "dependency_count",
			Title:       fmt.Sprintf("Dependencies Found: %d production, %d development", dependencyCount, devDependencyCount),
			Description: "Analyzed dependency counts in file",
			Confidence:  0.8,
			Severity:    SeverityInfo,
			Location: &Location{
				Path: file.Path,
			},
			Metadata: map[string]interface{}{
				"ecosystem":               ecosystem,
				"production_dependencies": dependencyCount,
				"dev_dependencies":        devDependencyCount,
				"total_dependencies":      dependencyCount + devDependencyCount,
			},
		}
		result.Findings = append(result.Findings, finding)
	}
}

func (d *DependencyAnalyzer) addVersionFinding(file FileData, packageName, version string, result *EngineAnalysisResult) {
	// Skip if it looks like a variable or placeholder
	if strings.Contains(version, "$") || strings.Contains(version, "{") {
		return
	}

	confidence := 0.7
	severity := SeverityInfo

	// Check for potentially outdated version patterns
	if strings.Contains(version, "^") || strings.Contains(version, "~") {
		severity = SeverityLow
		confidence = 0.6
	}

	finding := Finding{
		Type:        FindingTypeDependency,
		Category:    "dependency_version",
		Title:       fmt.Sprintf("Dependency: %s@%s", packageName, version),
		Description: "Found dependency with version specification",
		Confidence:  confidence,
		Severity:    severity,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"package_name": packageName,
			"version":      version,
			"ecosystem":    d.getEcosystem(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (d *DependencyAnalyzer) addVulnerabilityFinding(file FileData, pattern string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeSecurity,
		Category:    "vulnerable_dependency",
		Title:       "Potentially Vulnerable Dependency",
		Description: fmt.Sprintf("Found dependency pattern that may be vulnerable: %s", pattern),
		Confidence:  0.6, // Lower confidence as this is pattern-based
		Severity:    SeverityMedium,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"pattern":   pattern,
			"ecosystem": d.getEcosystem(file.Path),
			"file_type": filepath.Ext(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (d *DependencyAnalyzer) isDependencyFile(path string) bool {
	dependencyFiles := []string{
		"package.json", "yarn.lock", "package-lock.json",
		"requirements.txt", "Pipfile", "poetry.lock", "pyproject.toml",
		"go.mod", "go.sum", "pom.xml", "build.gradle", "Gemfile",
		"composer.json", "Cargo.toml", "Cargo.lock",
	}

	baseName := filepath.Base(path)
	for _, depFile := range dependencyFiles {
		if baseName == depFile {
			return true
		}
	}
	return false
}

func (d *DependencyAnalyzer) getEcosystem(path string) string {
	baseName := filepath.Base(path)
	switch {
	case strings.Contains(baseName, "package") && strings.HasSuffix(baseName, ".json"):
		return "node"
	case baseName == "yarn.lock" || baseName == "package-lock.json":
		return "node"
	case strings.Contains(baseName, "requirements") || baseName == "Pipfile" || strings.Contains(baseName, "poetry"):
		return "python"
	case baseName == "pyproject.toml":
		return "python"
	case strings.HasPrefix(baseName, "go."):
		return "go"
	case strings.Contains(baseName, "pom") || strings.Contains(baseName, "gradle"):
		return "java"
	case baseName == "Gemfile":
		return "ruby"
	case baseName == "composer.json":
		return "php"
	case strings.Contains(baseName, "Cargo"):
		return "rust"
	default:
		return "unknown"
	}
}

func (d *DependencyAnalyzer) countDependencyFiles(config AnalysisConfig) int {
	count := 0
	for _, file := range config.RepoData.Files {
		if d.isDependencyFile(file.Path) {
			count++
		}
	}
	return count
}
