package version

import (
	"os"
	"path/filepath"
	"regexp"
)

// detectDotNetVersion detects .NET version from project files
func (d *Detector) detectDotNetVersion(repoPath string) string {
	// Check for .csproj files
	csprojFiles, err := filepath.Glob(filepath.Join(repoPath, "*.csproj"))
	if err == nil && len(csprojFiles) > 0 {
		content, err := os.ReadFile(csprojFiles[0])
		if err == nil {
			targetRegex := regexp.MustCompile(`<TargetFramework>([^<]+)</TargetFramework>`)
			if matches := targetRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
				version := matches[1]
				d.logger.Debug("Found .NET version in .csproj", "version", version)
				return version
			}
		}
	}

	// Check global.json
	globalJsonPath := filepath.Join(repoPath, "global.json")
	if content, err := d.parseJSONFile(globalJsonPath); err == nil {
		if sdk, ok := content["sdk"].(map[string]interface{}); ok {
			if version, ok := sdk["version"].(string); ok && version != "" {
				d.logger.Debug("Found .NET SDK version in global.json", "version", version)
				return version
			}
		}
	}

	return ""
}
