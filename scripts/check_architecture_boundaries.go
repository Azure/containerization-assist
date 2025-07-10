package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ArchitectureRule defines an architecture boundary rule
type ArchitectureRule struct {
	Name          string
	Pattern       string
	AllowedDeps   []string
	ForbiddenDeps []string
	Description   string
}

// Violation represents an architecture boundary violation
type Violation struct {
	File        string
	Package     string
	Import      string
	Rule        string
	Description string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_architecture_boundaries.go <directory>")
		os.Exit(1)
	}

	rootDir := os.Args[1]

	fmt.Printf("Checking architecture boundaries in %s\n\n", rootDir)

	// Define architecture rules based on our clean architecture
	rules := []ArchitectureRule{
		{
			Name:    "Domain Layer Isolation",
			Pattern: "pkg/mcp/domain/**",
			ForbiddenDeps: []string{
				"pkg/mcp/application/**",
				"pkg/mcp/infra/**",
			},
			Description: "Domain layer should not depend on application or infrastructure layers",
		},
		{
			Name:    "Application Layer Dependencies",
			Pattern: "pkg/mcp/application/**",
			ForbiddenDeps: []string{
				"pkg/mcp/infra/**",
			},
			AllowedDeps: []string{
				"pkg/mcp/domain/**",
				"pkg/mcp/api/**",
				"pkg/mcp/errors/**",
				"pkg/mcp/services/**",
				"pkg/mcp/session/**",
				"pkg/mcp/shared/**",
				"pkg/mcp/logging/**",
				"pkg/mcp/config/**",
				"pkg/mcp/tools/**",
			},
			Description: "Application layer should only depend on domain layer and flattened packages",
		},
		{
			Name:    "Infrastructure Layer Dependencies",
			Pattern: "pkg/mcp/infra/**",
			AllowedDeps: []string{
				"pkg/mcp/domain/**",
				"pkg/mcp/application/**",
				"pkg/mcp/api/**",
				"pkg/mcp/errors/**",
				"pkg/mcp/services/**",
				"pkg/mcp/session/**",
				"pkg/mcp/shared/**",
				"pkg/mcp/logging/**",
				"pkg/mcp/config/**",
				"pkg/mcp/tools/**",
			},
			Description: "Infrastructure layer can depend on application and domain layers",
		},
		{
			Name:        "Maximum Import Depth",
			Pattern:     "pkg/mcp/**",
			Description: "No imports should exceed depth 3 (pkg/mcp/X/Y/Z)",
		},
		{
			Name:    "No Internal Package Access",
			Pattern: "**",
			ForbiddenDeps: []string{
				"**/internal/**",
			},
			Description: "Internal packages should not be imported across package boundaries",
		},
		{
			Name:    "Flattened Package Structure",
			Pattern: "pkg/mcp/**",
			ForbiddenDeps: []string{
				"pkg/mcp/domain/errors/**",
				"pkg/mcp/domain/session/**",
				"pkg/mcp/domain/shared/**",
				"pkg/mcp/domain/config/**",
				"pkg/mcp/domain/tools/**",
				"pkg/mcp/domain/containerization/**",
				"pkg/mcp/application/api/**",
				"pkg/mcp/application/services/**",
				"pkg/mcp/application/logging/**",
			},
			Description: "Should use flattened packages instead of old deep nested ones",
		},
	}

	violations := []Violation{}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") || strings.Contains(path, ".git/") {
			return nil
		}

		// Skip test files for now
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			fmt.Printf("Error parsing %s: %v\n", path, err)
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			relPath = path
		}

		// Get package path from file location
		packagePath := filepath.Dir(relPath)

		// Check each import against architecture rules
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Skip standard library and external imports
			if !strings.Contains(importPath, "github.com/Azure/container-kit") {
				continue
			}

			// Check against each rule
			for _, rule := range rules {
				violation := checkRule(relPath, packagePath, importPath, rule)
				if violation != nil {
					violations = append(violations, *violation)
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Print results
	if len(violations) > 0 {
		fmt.Printf("❌ Found %d architecture boundary violations:\n\n", len(violations))

		// Group by rule
		byRule := make(map[string][]Violation)
		for _, v := range violations {
			byRule[v.Rule] = append(byRule[v.Rule], v)
		}

		// Sort rule names
		ruleNames := make([]string, 0, len(byRule))
		for rule := range byRule {
			ruleNames = append(ruleNames, rule)
		}
		sort.Strings(ruleNames)

		for _, ruleName := range ruleNames {
			vList := byRule[ruleName]
			fmt.Printf("🚫 %s (%d violations)\n", ruleName, len(vList))

			// Show first 5 violations
			for i, v := range vList {
				if i >= 5 {
					fmt.Printf("   ... and %d more violations\n", len(vList)-5)
					break
				}
				fmt.Printf("   - %s imports %s\n", v.File, v.Import)
			}
			fmt.Println()
		}

		fmt.Printf("❌ Architecture boundary check FAILED\n")
		os.Exit(1)
	} else {
		fmt.Printf("✅ All architecture boundaries are properly maintained!\n")
		fmt.Printf("✅ Architecture boundary check PASSED\n")
	}
}

func checkRule(filePath, packagePath, importPath string, rule ArchitectureRule) *Violation {
	// Check if this file matches the rule pattern
	if !matchesPattern(packagePath, rule.Pattern) {
		return nil
	}

	// Special handling for import depth rule
	if rule.Name == "Maximum Import Depth" {
		if calculateDepth(importPath) > 3 {
			return &Violation{
				File:        filePath,
				Package:     packagePath,
				Import:      importPath,
				Rule:        rule.Name,
				Description: fmt.Sprintf("Import depth %d exceeds maximum of 3", calculateDepth(importPath)),
			}
		}
		return nil
	}

	// Check forbidden dependencies
	for _, forbidden := range rule.ForbiddenDeps {
		if matchesImportPattern(importPath, forbidden) {
			return &Violation{
				File:        filePath,
				Package:     packagePath,
				Import:      importPath,
				Rule:        rule.Name,
				Description: rule.Description,
			}
		}
	}

	// If there are allowed dependencies, check that import is in the allowed list
	if len(rule.AllowedDeps) > 0 {
		allowed := false
		for _, allowedPattern := range rule.AllowedDeps {
			if matchesImportPattern(importPath, allowedPattern) {
				allowed = true
				break
			}
		}

		// Also allow standard library and external packages
		if !strings.Contains(importPath, "github.com/Azure/container-kit") {
			allowed = true
		}

		if !allowed {
			return &Violation{
				File:        filePath,
				Package:     packagePath,
				Import:      importPath,
				Rule:        rule.Name,
				Description: fmt.Sprintf("%s - import not in allowed list", rule.Description),
			}
		}
	}

	return nil
}

func matchesPattern(path, pattern string) bool {
	// Simple pattern matching - supports /** wildcards
	pattern = strings.ReplaceAll(pattern, "/**", "")
	return strings.HasPrefix(path, pattern)
}

func matchesImportPattern(importPath, pattern string) bool {
	// Convert import path to relative path for matching
	relativePath := strings.TrimPrefix(importPath, "github.com/Azure/container-kit/")

	// Remove /** suffix for matching
	cleanPattern := strings.TrimSuffix(pattern, "/**")

	if strings.HasSuffix(pattern, "/**") {
		return strings.HasPrefix(relativePath, cleanPattern)
	}

	return relativePath == cleanPattern
}

func calculateDepth(importPath string) int {
	// Remove the base module path
	path := strings.TrimPrefix(importPath, "github.com/Azure/container-kit/")

	// Count the number of path segments
	if path == "" {
		return 0
	}

	segments := strings.Split(path, "/")
	return len(segments)
}
