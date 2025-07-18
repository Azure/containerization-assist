package version

import (
	"path/filepath"
)

// detectPHPVersion detects PHP version from composer.json
func (d *Detector) detectPHPVersion(repoPath string) string {
	composerPath := filepath.Join(repoPath, "composer.json")
	if content, err := d.parseJSONFile(composerPath); err == nil {
		if require, ok := content["require"].(map[string]interface{}); ok {
			if phpVersion, ok := require["php"].(string); ok && phpVersion != "" {
				d.logger.Debug("Found PHP version in composer.json", "version", phpVersion)
				return phpVersion
			}
		}
	}
	return ""
}
