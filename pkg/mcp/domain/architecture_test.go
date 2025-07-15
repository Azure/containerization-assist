package domain_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDomainLayerImports checks that domain packages don't import infrastructure or application packages
func TestDomainLayerImports(t *testing.T) {
	// Start from the domain directory
	domainRoot := "."

	// Forbidden import patterns for domain layer
	forbiddenImports := []string{
		"pkg/mcp/infrastructure/",
		"pkg/mcp/application/",
		"os/exec",
		"database/sql",
		"net/http",
	}

	err := filepath.WalkDir(domainRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only check .go files (excluding test files)
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", path, err)
			return nil
		}

		// Check for forbidden imports
		lines := strings.Split(string(content), "\n")
		inImportBlock := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Detect import block
			if strings.HasPrefix(trimmed, "import (") {
				inImportBlock = true
				continue
			}
			if inImportBlock && trimmed == ")" {
				inImportBlock = false
				continue
			}

			// Check single imports
			if strings.HasPrefix(trimmed, "import ") {
				for _, forbidden := range forbiddenImports {
					if strings.Contains(trimmed, forbidden) {
						t.Errorf("File %s line %d: Domain layer imports forbidden dependency: %s", path, i+1, trimmed)
					}
				}
			}

			// Check imports in block
			if inImportBlock && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				for _, forbidden := range forbiddenImports {
					if strings.Contains(trimmed, forbidden) {
						t.Errorf("File %s line %d: Domain layer imports forbidden dependency: %s", path, i+1, trimmed)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk domain directory: %v", err)
	}
}

// TestNoDirectFileOperations checks that domain layer doesn't use direct file operations
func TestNoDirectFileOperations(t *testing.T) {
	domainRoot := "."

	// Forbidden function calls in domain layer
	forbiddenCalls := []string{
		"os.WriteFile",
		"os.ReadFile",
		"os.MkdirAll",
		"os.Remove",
		"exec.Command",
		"http.Get",
		"http.Post",
	}

	err := filepath.WalkDir(domainRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only check .go files (excluding test files and this test)
		if !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") ||
			strings.Contains(path, "architecture_test.go") {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", path, err)
			return nil
		}

		// Check for forbidden function calls
		contentStr := string(content)
		for _, forbidden := range forbiddenCalls {
			if strings.Contains(contentStr, forbidden) {
				// Count occurrences
				count := strings.Count(contentStr, forbidden)
				t.Errorf("File %s contains %d occurrences of forbidden call: %s", path, count, forbidden)
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk domain directory: %v", err)
	}
}
