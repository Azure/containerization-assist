package repoanalysispipeline

import (
	"fmt"
	"strings"
)

// FormatFileOperationLogs formats the file operations list for better readability
func FormatFileOperationLogs(calls []string) string {
	if len(calls) == 0 {
		return "No file operations detected."
	}

	// Group by operation type
	fileReads := []string{}
	dirLists := []string{}
	fileChecks := []string{}

	for _, call := range calls {
		if strings.Contains(call, "reading file") {
			path := strings.TrimPrefix(call, "📄 LLM reading file: ")
			fileReads = append(fileReads, path)
		} else if strings.Contains(call, "listing directory") {
			path := strings.TrimPrefix(call, "📂 LLM listing directory: ")
			dirLists = append(dirLists, path)
		} else if strings.Contains(call, "checking if file exists") {
			path := strings.TrimPrefix(call, "🔍 LLM checking if file exists: ")
			fileChecks = append(fileChecks, path)
		}
	}

	var result strings.Builder

	result.WriteString(fmt.Sprintf("📄 Files Read (%d):\n", len(fileReads)))
	for _, file := range fileReads {
		result.WriteString(fmt.Sprintf("  - %s\n", file))
	}

	result.WriteString(fmt.Sprintf("\n📂 Directories Listed (%d):\n", len(dirLists)))
	for _, dir := range dirLists {
		result.WriteString(fmt.Sprintf("  - %s\n", dir))
	}

	result.WriteString(fmt.Sprintf("\n🔍 Files Checked (%d):\n", len(fileChecks)))
	for _, check := range fileChecks {
		result.WriteString(fmt.Sprintf("  - %s\n", check))
	}

	return result.String()
}
