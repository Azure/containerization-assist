package version

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectNodeVersion detects Node.js version from various sources
func (d *Detector) detectNodeVersion(repoPath string) string {
	// Check .nvmrc file
	nvmrcPath := filepath.Join(repoPath, ".nvmrc")
	if content, err := os.ReadFile(nvmrcPath); err == nil {
		version := strings.TrimSpace(string(content))
		if version != "" {
			d.logger.Debug("Found Node version in .nvmrc", "version", version)
			return version
		}
	}

	// Check package.json engines field
	packageJsonPath := filepath.Join(repoPath, "package.json")
	if content, err := d.parseJSONFile(packageJsonPath); err == nil {
		if engines, ok := content["engines"].(map[string]interface{}); ok {
			if nodeVersion, ok := engines["node"].(string); ok && nodeVersion != "" {
				d.logger.Debug("Found Node version in package.json engines", "version", nodeVersion)
				return nodeVersion
			}
		}
	}

	// Check Dockerfile for Node version
	dockerfilePath := filepath.Join(repoPath, "Dockerfile")
	if content, err := os.ReadFile(dockerfilePath); err == nil {
		nodeRegex := regexp.MustCompile(`FROM\s+node:([^\s]+)`)
		if matches := nodeRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Node version in Dockerfile", "version", version)
			return version
		}
	}

	return ""
}

// detectNpmFrameworkVersion detects version of npm-based frameworks
func (d *Detector) detectNpmFrameworkVersion(repoPath, framework string) string {
	packageJsonPath := filepath.Join(repoPath, "package.json")
	content, err := d.parseJSONFile(packageJsonPath)
	if err != nil {
		return ""
	}

	// Check dependencies and devDependencies
	depSections := []string{"dependencies", "devDependencies"}
	frameworkNames := map[string][]string{
		"nextjs":  {"next"},
		"react":   {"react"},
		"vue":     {"vue", "@vue/cli"},
		"angular": {"@angular/core", "angular"},
		"express": {"express"},
		"koa":     {"koa"},
		"fastify": {"fastify"},
		"nuxt":    {"nuxt"},
		"gatsby":  {"gatsby"},
	}

	if names, exists := frameworkNames[framework]; exists {
		for _, section := range depSections {
			if deps, ok := content[section].(map[string]interface{}); ok {
				for _, name := range names {
					if version, ok := deps[name].(string); ok && version != "" {
						d.logger.Debug("Found framework version", "framework", framework, "version", version)
						return version
					}
				}
			}
		}
	}

	return ""
}
