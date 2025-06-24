package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	execute = flag.Bool("execute", false, "Execute test migration (default: dry-run)")
	verbose = flag.Bool("verbose", false, "Verbose output")
	target  = flag.Float64("target", 70.0, "Target test coverage percentage")
)

type TestMigration struct {
	TestFile        string
	NewLocation     string
	UpdatedImports  []string
	UpdatedContent  string
	RequiresChanges bool
}

type CoverageReport struct {
	Package    string
	Coverage   float64
	Statements int
	Missing    int
	Files      map[string]float64
}

func main() {
	flag.Parse()
	
	fmt.Println("MCP Test Migration Tool")
	fmt.Println("=======================")
	
	if !*execute {
		fmt.Println("🔍 DRY RUN MODE - Use --execute to perform actual migration")
		fmt.Println()
	}
	
	// 1. Analyze current test structure
	fmt.Println("🔍 Analyzing current test structure...")
	currentTests, err := analyzeCurrentTests()
	if err != nil {
		fmt.Printf("❌ Failed to analyze tests: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("📊 Found %d test files\n", len(currentTests))
	
	// 2. Check current coverage
	fmt.Println("\n📏 Measuring current test coverage...")
	currentCoverage, err := measureCoverage()
	if err != nil {
		fmt.Printf("❌ Failed to measure coverage: %v\n", err)
	} else {
		displayCoverageReport(currentCoverage)
	}
	
	// 3. Plan test migrations based on new package structure
	fmt.Println("\n📋 Planning test migrations...")
	migrations := planTestMigrations(currentTests)
	
	if len(migrations) == 0 {
		fmt.Println("✅ No test migrations needed!")
		return
	}
	
	fmt.Printf("📝 Planned %d test migrations\n\n", len(migrations))
	
	// 4. Execute migrations if requested
	if *execute {
		fmt.Println("🔄 Executing test migrations...")
		for i, migration := range migrations {
			fmt.Printf("[%d/%d] Migrating %s\n", i+1, len(migrations), migration.TestFile)
			if err := executeMigration(migration); err != nil {
				fmt.Printf("❌ Migration failed: %v\n", err)
				continue
			}
			fmt.Println("  ✅ Completed")
		}
		
		// 5. Update test imports
		fmt.Println("\n🔄 Updating test imports...")
		if err := updateTestImports(); err != nil {
			fmt.Printf("❌ Failed to update imports: %v\n", err)
		}
		
		// 6. Verify tests still pass
		fmt.Println("\n🧪 Verifying migrated tests...")
		if err := runAllTests(); err != nil {
			fmt.Printf("❌ Tests failed after migration: %v\n", err)
			os.Exit(1)
		}
		
		// 7. Check final coverage
		fmt.Println("\n📏 Measuring final test coverage...")
		finalCoverage, err := measureCoverage()
		if err != nil {
			fmt.Printf("⚠️  Failed to measure final coverage: %v\n", err)
		} else {
			displayCoverageReport(finalCoverage)
			
			// Compare coverage
			if currentCoverage != nil && finalCoverage != nil {
				compareCoverage(currentCoverage, finalCoverage)
			}
		}
		
		fmt.Println("\n✅ Test migration completed successfully!")
	} else {
		// Just show the plan
		for i, migration := range migrations {
			fmt.Printf("%d. %s\n", i+1, migration.TestFile)
			fmt.Printf("   → %s\n", migration.NewLocation)
			if *verbose && len(migration.UpdatedImports) > 0 {
				fmt.Printf("   Imports: %v\n", migration.UpdatedImports)
			}
		}
		
		fmt.Println("\n📋 Migration plan complete. Use --execute to run.")
	}
}

func analyzeCurrentTests() ([]string, error) {
	var testFiles []string
	
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip vendor and other irrelevant directories
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" || name == "tools" {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Find test files
		if strings.HasSuffix(path, "_test.go") {
			testFiles = append(testFiles, path)
		}
		
		return nil
	})
	
	return testFiles, err
}

func measureCoverage() (map[string]*CoverageReport, error) {
	// Run tests with coverage
	cmd := exec.Command("go", "test", "-coverprofile=coverage.out", "./...")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run tests with coverage: %v\nOutput: %s", err, string(output))
	}
	
	// Parse coverage output
	cmd = exec.Command("go", "tool", "cover", "-func=coverage.out")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to parse coverage: %v", err)
	}
	
	coverage := make(map[string]*CoverageReport)
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			// Parse function coverage line
			// Format: file:line.col,line.col function coverage%
			if strings.Contains(parts[0], ":") && strings.HasSuffix(parts[len(parts)-1], "%") {
				coverageStr := strings.TrimSuffix(parts[len(parts)-1], "%")
				if coverageVal, err := strconv.ParseFloat(coverageStr, 64); err == nil {
					// Extract package from file path
					pkg := extractPackageFromPath(parts[0])
					if pkg != "" {
						if coverage[pkg] == nil {
							coverage[pkg] = &CoverageReport{
								Package: pkg,
								Files:   make(map[string]float64),
							}
						}
						coverage[pkg].Files[parts[0]] = coverageVal
					}
				}
			}
		}
	}
	
	// Calculate package-level coverage
	for _, report := range coverage {
		total := 0.0
		count := 0
		for _, fileCoverage := range report.Files {
			total += fileCoverage
			count++
		}
		if count > 0 {
			report.Coverage = total / float64(count)
		}
	}
	
	// Clean up
	os.Remove("coverage.out")
	
	return coverage, nil
}

func planTestMigrations(testFiles []string) []TestMigration {
	var migrations []TestMigration
	
	// Define migration mappings based on new package structure
	migrationMap := map[string]string{
		"pkg/mcp/internal/engine/":           "pkg/mcp/internal/runtime/",
		"pkg/mcp/internal/tools/atomic/":     "pkg/mcp/internal/",
		"pkg/mcp/internal/tools/security/":   "pkg/mcp/internal/scan/",
		"pkg/mcp/internal/tools/analysis/":   "pkg/mcp/internal/analyze/",
		"pkg/mcp/internal/store/session/":    "pkg/mcp/internal/session/",
		"pkg/mcp/internal/types/session/":    "pkg/mcp/internal/session/",
		"pkg/mcp/internal/orchestration/workflow/": "pkg/mcp/internal/workflow/",
		"pkg/logger/":                        "pkg/mcp/internal/observability/",
		"pkg/mcp/internal/ops/":              "pkg/mcp/internal/observability/",
	}
	
	for _, testFile := range testFiles {
		var newLocation string
		requiresChanges := false
		
		// Check if this test needs to be migrated
		for oldPath, newPath := range migrationMap {
			if strings.HasPrefix(testFile, oldPath) {
				newLocation = strings.Replace(testFile, oldPath, newPath, 1)
				requiresChanges = true
				break
			}
		}
		
		// Check if test imports need updating (even if file doesn't move)
		updatedImports := getUpdatedImports(testFile)
		if len(updatedImports) > 0 {
			requiresChanges = true
		}
		
		if requiresChanges {
			if newLocation == "" {
				newLocation = testFile // Same location, just import updates
			}
			
			migrations = append(migrations, TestMigration{
				TestFile:        testFile,
				NewLocation:     newLocation,
				UpdatedImports:  updatedImports,
				RequiresChanges: requiresChanges,
			})
		}
	}
	
	return migrations
}

func getUpdatedImports(testFile string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, testFile, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	
	var updatedImports []string
	
	// Import mappings from Team A's work
	importMappings := map[string]string{
		"github.com/tng/workspace/prod/pkg/mcp/internal/engine":           "github.com/tng/workspace/prod/pkg/mcp/internal/runtime",
		"github.com/tng/workspace/prod/pkg/mcp/internal/tools/atomic":     "github.com/tng/workspace/prod/pkg/mcp/internal",
		"github.com/tng/workspace/prod/pkg/mcp/internal/tools/security":   "github.com/tng/workspace/prod/pkg/mcp/internal/scan",
		"github.com/tng/workspace/prod/pkg/mcp/internal/tools/analysis":   "github.com/tng/workspace/prod/pkg/mcp/internal/analyze",
		"github.com/tng/workspace/prod/pkg/mcp/internal/store/session":    "github.com/tng/workspace/prod/pkg/mcp/internal/session",
		"github.com/tng/workspace/prod/pkg/mcp/internal/types/session":    "github.com/tng/workspace/prod/pkg/mcp/internal/session",
		"github.com/tng/workspace/prod/pkg/logger":                       "github.com/tng/workspace/prod/pkg/mcp/internal/observability",
		"github.com/tng/workspace/prod/pkg/mcp/internal/ops":             "github.com/tng/workspace/prod/pkg/mcp/internal/observability",
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
			
			for oldPath, newPath := range importMappings {
				if strings.Contains(importPath, oldPath) {
					newImport := strings.Replace(importPath, oldPath, newPath, 1)
					updatedImports = append(updatedImports, fmt.Sprintf("%s → %s", importPath, newImport))
					break
				}
			}
		}
	}
	
	return updatedImports
}

func executeMigration(migration TestMigration) error {
	// 1. Move file if location changed
	if migration.TestFile != migration.NewLocation {
		// Create destination directory
		destDir := filepath.Dir(migration.NewLocation)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", destDir, err)
		}
		
		// Use git mv to preserve history
		cmd := exec.Command("git", "mv", migration.TestFile, migration.NewLocation)
		if output, err := cmd.CombinedOutput(); err != nil {
			// If git mv fails, try regular move
			if err := os.Rename(migration.TestFile, migration.NewLocation); err != nil {
				return fmt.Errorf("failed to move %s -> %s: %v\nOutput: %s", 
					migration.TestFile, migration.NewLocation, err, string(output))
			}
		}
	}
	
	// 2. Update imports in the test file
	if len(migration.UpdatedImports) > 0 {
		targetFile := migration.NewLocation
		if targetFile == "" {
			targetFile = migration.TestFile
		}
		
		if err := updateFileImports(targetFile); err != nil {
			return fmt.Errorf("failed to update imports in %s: %w", targetFile, err)
		}
	}
	
	return nil
}

func updateTestImports() error {
	// Use our existing import update tool
	cmd := exec.Command("go", "run", "tools/update-imports/main.go", "--all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update imports: %v\nOutput: %s", err, string(output))
	}
	
	if *verbose {
		fmt.Printf("Import update output: %s\n", string(output))
	}
	
	return nil
}

func updateFileImports(filename string) error {
	// This is a simplified version - the full implementation would parse and update AST
	// For now, we'll rely on the global import update tool
	return nil
}

func runAllTests() error {
	cmd := exec.Command("go", "test", "./...")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tests failed: %v\nOutput: %s", err, string(output))
	}
	
	if *verbose {
		fmt.Printf("Test output: %s\n", string(output))
	}
	
	return nil
}

func displayCoverageReport(coverage map[string]*CoverageReport) {
	if len(coverage) == 0 {
		fmt.Println("   No coverage data available")
		return
	}
	
	fmt.Printf("📊 Test Coverage Report\n")
	fmt.Printf("   Target: %.1f%%\n\n", *target)
	
	totalCoverage := 0.0
	count := 0
	
	for _, report := range coverage {
		status := "✅"
		if report.Coverage < *target {
			status = "❌"
		} else if report.Coverage < *target+10 {
			status = "⚠️ "
		}
		
		fmt.Printf("   %s %s: %.1f%%\n", status, report.Package, report.Coverage)
		totalCoverage += report.Coverage
		count++
	}
	
	if count > 0 {
		avgCoverage := totalCoverage / float64(count)
		fmt.Printf("\n   📊 Average coverage: %.1f%%\n", avgCoverage)
		
		if avgCoverage >= *target {
			fmt.Printf("   ✅ Coverage target met!\n")
		} else {
			fmt.Printf("   ❌ Coverage below target (%.1f%% < %.1f%%)\n", avgCoverage, *target)
		}
	}
}

func compareCoverage(before, after map[string]*CoverageReport) {
	fmt.Printf("\n📊 Coverage Comparison\n")
	fmt.Printf("======================\n")
	
	for pkgName := range before {
		beforeCov := before[pkgName].Coverage
		afterCov := 0.0
		if after[pkgName] != nil {
			afterCov = after[pkgName].Coverage
		}
		
		change := afterCov - beforeCov
		symbol := "="
		if change > 1 {
			symbol = "↑"
		} else if change < -1 {
			symbol = "↓"
		}
		
		fmt.Printf("   %s %s: %.1f%% → %.1f%% (%.1f%%)\n", 
			symbol, pkgName, beforeCov, afterCov, change)
	}
}

func extractPackageFromPath(path string) string {
	// Extract package name from file path
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return ""
}