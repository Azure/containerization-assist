package utils

import (
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

// FormatBytes function has been moved to string_utils.go - use utils.FormatBytes instead
