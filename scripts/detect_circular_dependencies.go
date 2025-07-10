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

// Package represents a Go package with its dependencies
type Package struct {
	Path         string
	Dependencies []string
}

// Cycle represents a circular dependency
type Cycle struct {
	Packages []string
	Length   int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run detect_circular_dependencies.go <directory>")
		os.Exit(1)
	}

	rootDir := os.Args[1]

	fmt.Printf("Detecting circular dependencies in %s\n\n", rootDir)

	// Build dependency graph
	packages, err := buildDependencyGraph(rootDir)
	if err != nil {
		fmt.Printf("Error building dependency graph: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Analyzed %d packages\n\n", len(packages))

	// Detect cycles
	cycles := detectCycles(packages)

	if len(cycles) > 0 {
		fmt.Printf("❌ Found %d circular dependencies:\n\n", len(cycles))

		// Sort cycles by length (shorter cycles first, they're usually more critical)
		sort.Slice(cycles, func(i, j int) bool {
			return cycles[i].Length < cycles[j].Length
		})

		for i, cycle := range cycles {
			fmt.Printf("%d. Circular dependency (length %d):\n", i+1, cycle.Length)
			for j, pkg := range cycle.Packages {
				if j > 0 {
					fmt.Printf("   ↓\n")
				}
				fmt.Printf("   %s\n", pkg)
			}
			fmt.Printf("   ↓\n   %s (back to start)\n\n", cycle.Packages[0])
		}

		fmt.Printf("❌ Circular dependency check FAILED\n")
		fmt.Printf("\nRecommendations:\n")
		fmt.Printf("1. Extract common interfaces to a shared package\n")
		fmt.Printf("2. Use dependency injection to break cycles\n")
		fmt.Printf("3. Move shared types to a common package\n")
		fmt.Printf("4. Consider if some packages should be merged\n")

		os.Exit(1)
	} else {
		fmt.Printf("✅ No circular dependencies found!\n")
		fmt.Printf("✅ Circular dependency check PASSED\n")

		// Show some statistics
		fmt.Printf("\nDependency Statistics:\n")

		totalDeps := 0
		maxDeps := 0
		maxDepsPackage := ""

		for _, pkg := range packages {
			depCount := len(pkg.Dependencies)
			totalDeps += depCount
			if depCount > maxDeps {
				maxDeps = depCount
				maxDepsPackage = pkg.Path
			}
		}

		avgDeps := float64(totalDeps) / float64(len(packages))
		fmt.Printf("- Average dependencies per package: %.1f\n", avgDeps)
		fmt.Printf("- Package with most dependencies: %s (%d deps)\n", maxDepsPackage, maxDeps)

		// Find packages with no dependencies (leaf packages)
		leafPackages := []string{}
		for _, pkg := range packages {
			if len(pkg.Dependencies) == 0 {
				leafPackages = append(leafPackages, pkg.Path)
			}
		}

		if len(leafPackages) > 0 {
			fmt.Printf("- Leaf packages (no dependencies): %d\n", len(leafPackages))
			sort.Strings(leafPackages)
			for _, leaf := range leafPackages {
				fmt.Printf("  - %s\n", leaf)
			}
		}
	}
}

func buildDependencyGraph(rootDir string) (map[string]*Package, error) {
	packages := make(map[string]*Package)

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
			return nil // Skip files that can't be parsed
		}

		// Get package path from file location
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		packagePath := filepath.Dir(relPath)

		// Initialize package if not exists
		if _, exists := packages[packagePath]; !exists {
			packages[packagePath] = &Package{
				Path:         packagePath,
				Dependencies: []string{},
			}
		}

		// Collect dependencies
		deps := make(map[string]bool)
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Only track internal dependencies
			if strings.Contains(importPath, "github.com/Azure/container-kit") {
				// Convert to relative path
				relativePath := strings.TrimPrefix(importPath, "github.com/Azure/container-kit/")

				// Skip self-dependencies
				if relativePath != packagePath {
					deps[relativePath] = true
				}
			}
		}

		// Add unique dependencies
		for dep := range deps {
			// Check if already in dependencies
			found := false
			for _, existing := range packages[packagePath].Dependencies {
				if existing == dep {
					found = true
					break
				}
			}
			if !found {
				packages[packagePath].Dependencies = append(packages[packagePath].Dependencies, dep)
			}
		}

		return nil
	})

	return packages, err
}

func detectCycles(packages map[string]*Package) []Cycle {
	var cycles []Cycle
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	// DFS to detect cycles
	var dfs func(string, []string) bool
	dfs = func(pkg string, path []string) bool {
		if recStack[pkg] {
			// Found a cycle - extract it
			cycleStart := -1
			for i, p := range path {
				if p == pkg {
					cycleStart = i
					break
				}
			}

			if cycleStart >= 0 {
				cyclePath := append(path[cycleStart:], pkg)
				cycles = append(cycles, Cycle{
					Packages: cyclePath,
					Length:   len(cyclePath) - 1,
				})
			}
			return true
		}

		if visited[pkg] {
			return false
		}

		visited[pkg] = true
		recStack[pkg] = true

		// Visit all dependencies
		if packageInfo, exists := packages[pkg]; exists {
			for _, dep := range packageInfo.Dependencies {
				if dfs(dep, append(path, pkg)) {
					return true
				}
			}
		}

		recStack[pkg] = false
		return false
	}

	// Check each package
	for pkgPath := range packages {
		if !visited[pkgPath] {
			dfs(pkgPath, []string{})
		}
	}

	return removeDuplicateCycles(cycles)
}

func removeDuplicateCycles(cycles []Cycle) []Cycle {
	seen := make(map[string]bool)
	var unique []Cycle

	for _, cycle := range cycles {
		// Create a canonical representation of the cycle
		canonical := createCanonicalCycle(cycle.Packages)

		if !seen[canonical] {
			seen[canonical] = true
			unique = append(unique, cycle)
		}
	}

	return unique
}

func createCanonicalCycle(packages []string) string {
	if len(packages) == 0 {
		return ""
	}

	// Find the lexicographically smallest package to start the cycle
	minIdx := 0
	for i, pkg := range packages {
		if pkg < packages[minIdx] {
			minIdx = i
		}
	}

	// Create canonical form starting from the minimum package
	canonical := make([]string, len(packages))
	for i := 0; i < len(packages); i++ {
		canonical[i] = packages[(minIdx+i)%len(packages)]
	}

	return strings.Join(canonical, " -> ")
}
