// Package version provides version detection functionality for various programming languages and frameworks.
package version

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// ConfigFile represents a configuration file found in the repository
type ConfigFile struct {
	Path     string                 `json:"path"`
	Type     string                 `json:"type"` // "package", "build", "env", "docker"
	Content  map[string]interface{} `json:"content,omitempty"`
	Relevant bool                   `json:"relevant"`
}

// Detector provides language and framework version detection capabilities
type Detector struct {
	logger *slog.Logger
}

// NewDetector creates a new version detector
func NewDetector(logger *slog.Logger) *Detector {
	return &Detector{
		logger: logger.With("component", "version_detector"),
	}
}

// DetectLanguageVersion detects the version of the primary language
func (d *Detector) DetectLanguageVersion(repoPath, language string) string {
	switch language {
	case "javascript", "typescript":
		return d.detectNodeVersion(repoPath)
	case "python":
		return d.detectPythonVersion(repoPath)
	case "go":
		return d.detectGoVersion(repoPath)
	case "java":
		return d.detectJavaVersion(repoPath)
	case "rust":
		return d.detectRustVersion(repoPath)
	case "php":
		return d.detectPHPVersion(repoPath)
	case "ruby":
		return d.detectRubyVersion(repoPath)
	case "csharp":
		return d.detectDotNetVersion(repoPath)
	}
	return ""
}

// DetectFrameworkVersion detects the version of the primary framework
func (d *Detector) DetectFrameworkVersion(repoPath, framework string) string {
	switch framework {
	case "nextjs", "react", "vue", "angular", "express", "koa", "fastify", "nuxt", "gatsby":
		return d.detectNpmFrameworkVersion(repoPath, framework)
	case "maven", "gradle", "maven-servlet", "gradle-servlet", "maven-war", "gradle-war":
		return d.detectJavaFrameworkVersion(repoPath, framework)
	case "spring", "spring-boot":
		return d.detectSpringVersion(repoPath)
	case "django", "flask":
		return d.detectPythonFrameworkVersion(repoPath, framework)
	case "gin", "echo", "fiber":
		return d.detectGoFrameworkVersion(repoPath, framework)
	}
	return ""
}

// DetectGoFrameworkFromMod detects Go frameworks from go.mod dependencies
func (d *Detector) DetectGoFrameworkFromMod(repoPath string) string {
	return d.detectGoFrameworkFromMod(repoPath)
}

// parseJSONFile parses a JSON file and returns the content as a map
func (d *Detector) parseJSONFile(filePath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		d.logger.Debug("Failed to read JSON file", "path", filePath, "error", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check if file is empty
	if len(content) == 0 {
		d.logger.Debug("Empty JSON file", "path", filePath)
		return make(map[string]interface{}), nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		// Try to provide more context about the JSON error
		preview := string(content)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		d.logger.Debug("Failed to parse JSON",
			"path", filePath,
			"error", err,
			"preview", preview)
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return result, nil
}
