package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	verbose = flag.Bool("verbose", false, "Verbose output")
	strict  = flag.Bool("strict", false, "Enable strict boundary checking")
)

// Package boundary rules based on REORG.md
type BoundaryRule struct {
	Package     string
	AllowedDeps []string
	Forbidden   []string
	Description string
}

var boundaryRules = []BoundaryRule{
	{
		Package:     "pkg/mcp/internal/runtime",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/session", "pkg/mcp/internal/transport", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/build", "pkg/mcp/internal/deploy", "pkg/mcp/internal/scan", "pkg/mcp/internal/analyze"},
		Description: "Runtime core should not depend on specific tool domains",
	},
	{
		Package:     "pkg/mcp/internal/build",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/validate", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/deploy", "pkg/mcp/internal/scan", "pkg/mcp/internal/analyze", "pkg/mcp/internal/runtime"},
		Description: "Build tools should be independent of other tool domains",
	},
	{
		Package:     "pkg/mcp/internal/deploy",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/validate", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/build", "pkg/mcp/internal/scan", "pkg/mcp/internal/analyze", "pkg/mcp/internal/runtime"},
		Description: "Deploy tools should be independent of other tool domains",
	},
	{
		Package:     "pkg/mcp/internal/scan",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/validate", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/build", "pkg/mcp/internal/deploy", "pkg/mcp/internal/analyze", "pkg/mcp/internal/runtime"},
		Description: "Security scanning tools should be independent of other tool domains",
	},
	{
		Package:     "pkg/mcp/internal/analyze",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/validate", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/build", "pkg/mcp/internal/deploy", "pkg/mcp/internal/scan", "pkg/mcp/internal/runtime"},
		Description: "Analysis tools should be independent of other tool domains",
	},
	{
		Package:     "pkg/mcp/internal/session",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/build", "pkg/mcp/internal/deploy", "pkg/mcp/internal/scan", "pkg/mcp/internal/analyze"},
		Description: "Session management should not depend on tool domains",
	},
	{
		Package:     "pkg/mcp/internal/transport",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/build", "pkg/mcp/internal/deploy", "pkg/mcp/internal/scan", "pkg/mcp/internal/analyze", "pkg/mcp/internal/session"},
		Description: "Transport layer should be independent of business logic",
	},
	{
		Package:     "pkg/mcp/internal/workflow",
		AllowedDeps: []string{"pkg/mcp", "pkg/mcp/internal/session", "pkg/mcp/internal/observability"},
		Forbidden:   []string{"pkg/mcp/internal/build", "pkg/mcp/internal/deploy", "pkg/mcp/internal/scan", "pkg/mcp/internal/analyze"},
		Description: "Workflow orchestration should not depend on specific tool domains",
	},
	{
		Package:     "pkg/mcp/internal/validate",
		AllowedDeps: []string{"pkg/mcp"},
		Forbidden:   []string{},
		Description: "Validation package should be minimal and reusable",
	},
	{
		Package:     "pkg/mcp/internal/observability",
		AllowedDeps: []string{},
		Forbidden:   []string{},
		Description: "Observability should have minimal dependencies",
	},
}

type BoundaryViolation struct {
	Package    string
	File       string
	Import     string
	Rule       BoundaryRule
	Severity   string
	LineNumber int
}

func main() {
	flag.Parse()

	fmt.Println("MCP Package Boundary Validation Tool")
	fmt.Println("====================================")

	violations := []BoundaryViolation{}

	// Check each package boundary rule
	for _, rule := range boundaryRules {
		fmt.Printf("🔍 Checking package: %s\n", rule.Package)

		packageViolations, err := checkPackageBoundaries(rule)
		if err != nil {
			log.Printf("⚠️  Failed to check package %s: %v", rule.Package, err)
			continue
		}

		violations = append(violations, packageViolations...)

		if *verbose {
			fmt.Printf("   Found %d violations\n", len(packageViolations))
		}
	}

	// Check for circular dependencies
	fmt.Println("🔍 Checking for circular dependencies...")
	circularViolations, err := checkCircularDependencies()
	if err != nil {
		log.Printf("⚠️  Failed to check circular dependencies: %v", err)
	} else {
		violations = append(violations, circularViolations...)
	}

	// Report results
	fmt.Println("\n📊 Package Boundary Validation Results")
	fmt.Println("======================================")

	errors := 0
	warnings := 0

	for _, violation := range violations {
		switch violation.Severity {
		case "error":
			fmt.Printf("❌ ERROR: %s\n", formatViolation(violation))
			errors++
		case "warning":
			fmt.Printf("⚠️  WARNING: %s\n", formatViolation(violation))
			warnings++
		}

		if *verbose {
			fmt.Printf("   File: %s:%d\n", violation.File, violation.LineNumber)
			fmt.Printf("   Rule: %s\n", violation.Rule.Description)
		}
		fmt.Println()
	}

	fmt.Printf("Summary: %d errors, %d warnings\n", errors, warnings)

	if errors > 0 {
		fmt.Println("\n❌ Package boundary validation failed!")
		fmt.Println("   Fix the boundary violations above before proceeding.")
		os.Exit(1)
	} else if warnings > 0 {
		fmt.Println("\n⚠️  Package boundary validation passed with warnings.")
		fmt.Println("   Consider addressing the warnings above.")
	} else {
		fmt.Println("\n✅ Package boundary validation passed!")
	}
}

func checkPackageBoundaries(rule BoundaryRule) ([]BoundaryViolation, error) {
	var violations []BoundaryViolation

	// Check if package exists
	if _, err := os.Stat(rule.Package); os.IsNotExist(err) {
		// Package doesn't exist yet - this is expected during migration
		if *verbose {
			fmt.Printf("   Package %s does not exist yet (expected during migration)\n", rule.Package)
		}
		return violations, nil
	}

	// Find all Go files in the package
	err := filepath.WalkDir(rule.Package, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		fileViolations, err := checkFileImports(path, rule)
		if err != nil {
			return err
		}

		violations = append(violations, fileViolations...)
		return nil
	})

	return violations, err
}

func checkFileImports(filePath string, rule BoundaryRule) ([]BoundaryViolation, error) {
	var violations []BoundaryViolation

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return violations, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	// Check each import
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}

		for _, spec := range genDecl.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}

			importPath := strings.Trim(importSpec.Path.Value, `"`)

			// Skip standard library and external dependencies
			if !strings.Contains(importPath, "github.com/tng/workspace/prod/pkg/mcp") {
				continue
			}

			violation := checkImportViolation(filePath, importPath, rule, fset.Position(importSpec.Pos()).Line)
			if violation != nil {
				violations = append(violations, *violation)
			}
		}
	}

	return violations, nil
}

func checkImportViolation(filePath, importPath string, rule BoundaryRule, lineNumber int) *BoundaryViolation {
	// Check forbidden imports
	for _, forbidden := range rule.Forbidden {
		if strings.Contains(importPath, forbidden) {
			return &BoundaryViolation{
				Package:    rule.Package,
				File:       filePath,
				Import:     importPath,
				Rule:       rule,
				Severity:   "error",
				LineNumber: lineNumber,
			}
		}
	}

	// In strict mode, check if import is in allowed list
	if *strict && len(rule.AllowedDeps) > 0 {
		allowed := false
		for _, allowedDep := range rule.AllowedDeps {
			if strings.Contains(importPath, allowedDep) {
				allowed = true
				break
			}
		}

		// Also allow standard library and external deps
		if !allowed && strings.Contains(importPath, "github.com/tng/workspace/prod/pkg/mcp") {
			return &BoundaryViolation{
				Package:    rule.Package,
				File:       filePath,
				Import:     importPath,
				Rule:       rule,
				Severity:   "warning",
				LineNumber: lineNumber,
			}
		}
	}

	return nil
}

func checkCircularDependencies() ([]BoundaryViolation, error) {
	var violations []BoundaryViolation

	// Build dependency graph
	depGraph := make(map[string][]string)

	err := filepath.WalkDir("pkg/mcp", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Get package name from path
		packagePath := filepath.Dir(path)

		// Parse file to get imports
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files that can't be parsed
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.IMPORT {
				continue
			}

			for _, spec := range genDecl.Specs {
				importSpec, ok := spec.(*ast.ImportSpec)
				if !ok {
					continue
				}

				importPath := strings.Trim(importSpec.Path.Value, `"`)

				// Only consider internal MCP imports
				if strings.Contains(importPath, "github.com/tng/workspace/prod/pkg/mcp/internal") {
					// Convert import path to local package path
					localPath := strings.Replace(importPath, "github.com/tng/workspace/prod/", "", 1)
					depGraph[packagePath] = append(depGraph[packagePath], localPath)
				}
			}
		}

		return nil
	})

	if err != nil {
		return violations, err
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for pkg := range depGraph {
		if !visited[pkg] {
			if cycle := findCycle(pkg, depGraph, visited, recStack, []string{}); len(cycle) > 0 {
				violations = append(violations, BoundaryViolation{
					Package:  pkg,
					File:     pkg,
					Import:   strings.Join(cycle, " -> "),
					Severity: "error",
					Rule: BoundaryRule{
						Description: "Circular dependency detected",
					},
				})
			}
		}
	}

	return violations, nil
}

func findCycle(pkg string, depGraph map[string][]string, visited, recStack map[string]bool, path []string) []string {
	visited[pkg] = true
	recStack[pkg] = true
	path = append(path, pkg)

	for _, dep := range depGraph[pkg] {
		if !visited[dep] {
			if cycle := findCycle(dep, depGraph, visited, recStack, path); len(cycle) > 0 {
				return cycle
			}
		} else if recStack[dep] {
			// Found a cycle
			cycleStart := -1
			for i, p := range path {
				if p == dep {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				return append(path[cycleStart:], dep)
			}
		}
	}

	recStack[pkg] = false
	return nil
}

func formatViolation(violation BoundaryViolation) string {
	if violation.Rule.Description == "Circular dependency detected" {
		return fmt.Sprintf("Circular dependency: %s", violation.Import)
	}

	return fmt.Sprintf("Package %s imports forbidden dependency: %s",
		violation.Package, violation.Import)
}
