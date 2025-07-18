package version

import (
	"os"
	"path/filepath"
	"regexp"
)

// detectRustVersion detects Rust version from Cargo.toml
func (d *Detector) detectRustVersion(repoPath string) string {
	cargoPath := filepath.Join(repoPath, "Cargo.toml")
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return ""
	}

	rustRegex := regexp.MustCompile(`rust-version\s*=\s*"([^"]+)"`)
	if matches := rustRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
		version := matches[1]
		d.logger.Debug("Found Rust version in Cargo.toml", "version", version)
		return version
	}

	return ""
}
