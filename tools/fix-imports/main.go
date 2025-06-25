package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Fix import mappings based on actual migrations
var fixMappings = map[string]string{
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session":        "github.com/Azure/container-copilot/pkg/mcp/internal/session",
	"github.com/Azure/container-copilot/pkg/mcp/internal/types/session":        "github.com/Azure/container-copilot/pkg/mcp/internal/session",
	"github.com/Azure/container-copilot/pkg/mcp/internal/analyze":              "github.com/Azure/container-copilot/pkg/mcp/internal/analyze",
	"github.com/Azure/container-copilot/pkg/mcp/internal/scan":                 "github.com/Azure/container-copilot/pkg/mcp/internal/scan",
	"github.com/Azure/container-copilot/pkg/mcp/internal/scan/scanners":        "github.com/Azure/container-copilot/pkg/mcp/internal/scan/scanners",
	"github.com/Azure/container-copilot/pkg/mcp/internal/runtime/conversation": "github.com/Azure/container-copilot/pkg/mcp/internal/runtime/conversation",
	"github.com/Azure/container-copilot/pkg/mcp/internal/observability":        "github.com/Azure/container-copilot/pkg/mcp/internal/observability",
	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow":             "github.com/Azure/container-copilot/pkg/mcp/internal/workflow",
	"github.com/Azure/container-copilot/pkg/mcp/internal/validate":             "github.com/Azure/container-copilot/pkg/mcp/internal/validate",
}

func main() {
	fmt.Println("Fixing imports after migration...")

	// Get all Go files
	var files []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".go") && !strings.Contains(path, "vendor/") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking files: %v\n", err)
		os.Exit(1)
	}

	// Fix imports in each file
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", file, err)
			continue
		}

		modified := false
		contentStr := string(content)

		for oldImport, newImport := range fixMappings {
			if strings.Contains(contentStr, oldImport) {
				contentStr = strings.ReplaceAll(contentStr, oldImport, newImport)
				modified = true
				fmt.Printf("Fixed import in %s: %s -> %s\n", file, oldImport, newImport)
			}
		}

		if modified {
			err = os.WriteFile(file, []byte(contentStr), 0644)
			if err != nil {
				fmt.Printf("Error writing %s: %v\n", file, err)
			}
		}
	}

	fmt.Println("Import fixing complete!")
}
