package version

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectPythonVersion detects Python version from various sources
func (d *Detector) detectPythonVersion(repoPath string) string {
	// Check .python-version file
	pythonVersionPath := filepath.Join(repoPath, ".python-version")
	if content, err := os.ReadFile(pythonVersionPath); err == nil {
		version := strings.TrimSpace(string(content))
		if version != "" {
			d.logger.Debug("Found Python version in .python-version", "version", version)
			return version
		}
	}

	// Check pyproject.toml
	pyprojectPath := filepath.Join(repoPath, "pyproject.toml")
	if content, err := os.ReadFile(pyprojectPath); err == nil {
		pythonRegex := regexp.MustCompile(`python\s*=\s*"([^"]+)"`)
		if matches := pythonRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Python version in pyproject.toml", "version", version)
			return version
		}
	}

	// Check runtime.txt (common in Heroku deployments)
	runtimePath := filepath.Join(repoPath, "runtime.txt")
	if content, err := os.ReadFile(runtimePath); err == nil {
		pythonRegex := regexp.MustCompile(`python-(\d+\.\d+\.\d+)`)
		if matches := pythonRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Python version in runtime.txt", "version", version)
			return version
		}
	}

	// Check Dockerfile for Python version
	dockerfilePath := filepath.Join(repoPath, "Dockerfile")
	if content, err := os.ReadFile(dockerfilePath); err == nil {
		pythonRegex := regexp.MustCompile(`FROM\s+python:([^\s]+)`)
		if matches := pythonRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Python version in Dockerfile", "version", version)
			return version
		}
	}

	return ""
}

// detectPythonFrameworkVersion detects version of Python frameworks
func (d *Detector) detectPythonFrameworkVersion(repoPath, framework string) string {
	// Check requirements.txt first
	reqPath := filepath.Join(repoPath, "requirements.txt")
	if content, err := os.ReadFile(reqPath); err == nil {
		frameworkRegex := regexp.MustCompile(fmt.Sprintf(`%s[>=<~!]*([^\s\n]+)`, regexp.QuoteMeta(framework)))
		if matches := frameworkRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Python framework version in requirements.txt", "framework", framework, "version", version)
			return version
		}
	}

	// Check pyproject.toml
	pyprojectPath := filepath.Join(repoPath, "pyproject.toml")
	if content, err := os.ReadFile(pyprojectPath); err == nil {
		frameworkRegex := regexp.MustCompile(fmt.Sprintf(`%s\s*=\s*"([^"]+)"`, regexp.QuoteMeta(framework)))
		if matches := frameworkRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Python framework version in pyproject.toml", "framework", framework, "version", version)
			return version
		}
	}

	return ""
}
