package version

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectRubyVersion detects Ruby version from various sources
func (d *Detector) detectRubyVersion(repoPath string) string {
	// Check .ruby-version file
	rubyVersionPath := filepath.Join(repoPath, ".ruby-version")
	if content, err := os.ReadFile(rubyVersionPath); err == nil {
		version := strings.TrimSpace(string(content))
		if version != "" {
			d.logger.Debug("Found Ruby version in .ruby-version", "version", version)
			return version
		}
	}

	// Check Gemfile for ruby version
	gemfilePath := filepath.Join(repoPath, "Gemfile")
	if content, err := os.ReadFile(gemfilePath); err == nil {
		rubyRegex := regexp.MustCompile(`ruby\s+['"]*([^'"\s]+)['"]*`)
		if matches := rubyRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Ruby version in Gemfile", "version", version)
			return version
		}
	}

	return ""
}
