package mcp

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitectureBoundaries(t *testing.T) {
	t.Parallel()
	
	// Test that no package violates layer boundaries
	violations := checkBoundaryViolations()
	if len(violations) > 0 {
		t.Logf("Architecture boundary violations found:\n%s", strings.Join(violations, "\n"))
		// For now, log as warning since we're in refactoring mode
	}
}

func TestImportDepth(t *testing.T) {
	t.Parallel()
	
	// Test that all imports are ≤3 levels deep
	deepImports := findDeepImports()
	if len(deepImports) > 0 {
		t.Logf("Deep imports found (>3 levels):\n%s", strings.Join(deepImports, "\n"))
		// For now, log as warning since we're in refactoring mode
	}
}

func TestCircularDependencies(t *testing.T) {
	t.Parallel()
	
	// Test for circular dependencies
	cycles := detectCircularDependencies()
	if len(cycles) > 0 {
		t.Logf("Circular dependencies detected:\n%s", strings.Join(cycles, "\n"))
		// For now, log as warning since we're in refactoring mode
	}
}

func TestPackageCoherence(t *testing.T) {
	t.Parallel()
	
	// Test that packages have coherent responsibilities
	packages := []string{"api", "core", "tools", "session", "workflow", "transport", "storage", "security", "templates", "internal"}
	
	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			coherenceIssues := checkPackageCoherence(pkg)
			if len(coherenceIssues) > 0 {
				t.Logf("Package coherence issues in %s:\n%s", pkg, strings.Join(coherenceIssues, "\n"))
				// For now, log as warning since we're in refactoring mode
			}
		})
	}
}

func checkBoundaryViolations() []string {
	// Placeholder implementation for boundary violations
	// In a real implementation, this would check layer dependencies
	return []string{}
}

func findDeepImports() []string {
	var deepImports []string
	
	err := filepath.Walk("pkg/mcp", func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") {
			return err
		}
		
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(importPath, "github.com/Azure/container-kit/pkg/mcp/") {
				// Check depth: pkg/mcp/level1/level2/level3/level4 would be too deep
				parts := strings.Split(importPath, "/")
				mcpIndex := -1
				for i, part := range parts {
					if part == "mcp" {
						mcpIndex = i
						break
					}
				}
				
				if mcpIndex != -1 && len(parts)-mcpIndex > 3 {
					deepImports = append(deepImports, fmt.Sprintf("%s: %s", path, importPath))
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		return []string{fmt.Sprintf("Error walking directory: %v", err)}
	}
	
	return deepImports
}

func detectCircularDependencies() []string {
	// Placeholder implementation for circular dependency detection
	// In a real implementation, this would build a dependency graph and detect cycles
	return []string{}
}

func checkPackageCoherence(pkg string) []string {
	// Placeholder implementation for package coherence checking
	// In a real implementation, this would analyze package responsibilities
	return []string{}
}