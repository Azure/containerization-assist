package utils

import (
	"fmt"
	"strings"
)

// ExtractBaseImage extracts the base image from Dockerfile content
func ExtractBaseImage(dockerfileContent string) string {
	lines := strings.Split(dockerfileContent, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return "unknown"
}

// FormatBytes formats bytes into human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
