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

type ImportInfo struct {
	File       string
	Package    string
	ImportPath string
	Depth      int
}

type DepthViolation struct {
	File       string
	Package    string
	ImportPath string
	Depth      int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_import_depth.go <directory>")
		os.Exit(1)
	}

	rootDir := os.Args[1]
	maxDepth := 3 // Maximum allowed package depth

	fmt.Printf("Checking import depth in %s (max depth: %d)\n\n", rootDir, maxDepth)

	violations := []DepthViolation{}
	stats := make(map[int]int) // depth -> count

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

		// Check each import
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Skip standard library and external imports
			if !strings.Contains(importPath, "github.com/Azure/container-kit") {
				continue
			}

			// Calculate depth of import path
			depth := calculateDepth(importPath)
			stats[depth]++

			if depth > maxDepth {
				violations = append(violations, DepthViolation{
					File:       relPath,
					Package:    node.Name.Name,
					ImportPath: importPath,
					Depth:      depth,
				})
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Print statistics
	fmt.Println("=== Import Depth Statistics ===")
	depths := make([]int, 0, len(stats))
	for d := range stats {
		depths = append(depths, d)
	}
	sort.Ints(depths)

	for _, d := range depths {
		fmt.Printf("Depth %d: %d imports\n", d, stats[d])
	}

	// Print violations
	if len(violations) > 0 {
		fmt.Printf("\n=== Violations (imports deeper than %d levels) ===\n", maxDepth)
		fmt.Printf("Found %d violations:\n\n", len(violations))

		// Group by import path
		byImport := make(map[string][]DepthViolation)
		for _, v := range violations {
			byImport[v.ImportPath] = append(byImport[v.ImportPath], v)
		}

		// Sort import paths
		importPaths := make([]string, 0, len(byImport))
		for imp := range byImport {
			importPaths = append(importPaths, imp)
		}
		sort.Strings(importPaths)

		for _, imp := range importPaths {
			vList := byImport[imp]
			fmt.Printf("Import: %s (depth: %d)\n", imp, vList[0].Depth)
			fmt.Printf("Used in %d files:\n", len(vList))

			// Show first 5 files
			for i, v := range vList {
				if i >= 5 {
					fmt.Printf("  ... and %d more files\n", len(vList)-5)
					break
				}
				fmt.Printf("  - %s\n", v.File)
			}
			fmt.Println()
		}

		fmt.Printf("\nAction Required: Flatten these packages to %d levels or less\n", maxDepth)
	} else {
		fmt.Printf("\n✓ All imports are within the %d level depth limit!\n", maxDepth)
	}
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
