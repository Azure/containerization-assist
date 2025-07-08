package security_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// InterfaceStats holds interface analysis results
type InterfaceStats struct {
	InterfacesByPackage    map[string]int
	InterfacesByFile       map[string][]string
	InterfaceNames         map[string][]string
	SingleMethodInterfaces int
	EmptyInterfaces        int
}

// analyzeInterfaces performs interface analysis across the codebase
func analyzeInterfaces(t *testing.T) *InterfaceStats {
	stats := &InterfaceStats{
		InterfacesByPackage: make(map[string]int),
		InterfacesByFile:    make(map[string][]string),
		InterfaceNames:      make(map[string][]string),
	}

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip test files and vendor
		if strings.Contains(path, "_test.go") || strings.Contains(path, "vendor") {
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") || d.IsDir() {
			return nil
		}

		return stats.analyzeFile(path)
	})

	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	return stats
}

// analyzeFile analyzes interfaces in a single Go file
func (s *InterfaceStats) analyzeFile(path string) error {
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil // Skip files we can't parse
	}

	// Get package directory
	pkgDir := filepath.Dir(path)
	fileInterfaces := []string{}

	ast.Inspect(node, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok {
			s.processGenDecl(genDecl, path, pkgDir, &fileInterfaces)
		}
		return true
	})

	if len(fileInterfaces) > 0 {
		s.InterfacesByFile[path] = fileInterfaces
	}

	return nil
}

// processGenDecl processes a general declaration for interfaces
func (s *InterfaceStats) processGenDecl(genDecl *ast.GenDecl, path, pkgDir string, fileInterfaces *[]string) {
	for _, spec := range genDecl.Specs {
		if typeSpec, ok := spec.(*ast.TypeSpec); ok {
			if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				s.processInterface(typeSpec, interfaceType, path, pkgDir, fileInterfaces)
			}
		}
	}
}

// processInterface processes a single interface declaration
func (s *InterfaceStats) processInterface(typeSpec *ast.TypeSpec, interfaceType *ast.InterfaceType, path, pkgDir string, fileInterfaces *[]string) {
	interfaceName := typeSpec.Name.Name

	// Track interface
	s.InterfacesByPackage[pkgDir]++
	*fileInterfaces = append(*fileInterfaces, interfaceName)
	s.InterfaceNames[interfaceName] = append(s.InterfaceNames[interfaceName], path)

	// Count methods
	methodCount := s.countMethods(interfaceType)

	if methodCount == 0 {
		s.EmptyInterfaces++
	} else if methodCount == 1 {
		s.SingleMethodInterfaces++
	}
}

// countMethods counts the number of methods in an interface
func (s *InterfaceStats) countMethods(interfaceType *ast.InterfaceType) int {
	methodCount := 0
	if interfaceType.Methods != nil {
		for _, method := range interfaceType.Methods.List {
			if len(method.Names) > 0 {
				methodCount++
			}
		}
	}
	return methodCount
}

// TestInterfaceConsolidation validates that interface over-engineering has been resolved
func TestInterfaceConsolidation(t *testing.T) {
	stats := analyzeInterfaces(t)

	t.Run("PackagesWithManyInterfaces", func(t *testing.T) {
		testPackagesWithManyInterfaces(t, stats)
	})

	t.Run("DuplicateInterfaces", func(t *testing.T) {
		testDuplicateInterfaces(t, stats)
	})

	t.Run("FilesWithManyInterfaces", func(t *testing.T) {
		testFilesWithManyInterfaces(t, stats)
	})

	t.Run("SingleMethodAndEmptyInterfaces", func(t *testing.T) {
		testSingleMethodAndEmptyInterfaces(t, stats)
	})
}

// testPackagesWithManyInterfaces tests that interface consolidation has reduced excessive interfaces
func testPackagesWithManyInterfaces(t *testing.T, stats *InterfaceStats) {
	packagesWithManyInterfaces := 0
	for pkg, count := range stats.InterfacesByPackage {
		if count > 5 {
			packagesWithManyInterfaces++
			t.Logf("Package %s has %d interfaces (>5)", pkg, count)
		}
	}

	// After consolidation, we should have fewer packages with excessive interfaces
	if packagesWithManyInterfaces > 5 {
		t.Errorf("Expected fewer than 5 packages with >5 interfaces after consolidation, found %d", packagesWithManyInterfaces)
	}
	t.Logf("Successfully consolidated: only %d packages have >5 interfaces", packagesWithManyInterfaces)
}

// testDuplicateInterfaces tests that interface consolidation has reduced duplicates
func testDuplicateInterfaces(t *testing.T, stats *InterfaceStats) {
	duplicateInterfaces := 0
	for name, locations := range stats.InterfaceNames {
		if len(locations) > 1 {
			duplicateInterfaces++
			t.Logf("Interface '%s' defined in %d locations: %v", name, len(locations), locations)
		}
	}

	// After consolidation, we should have fewer duplicate interface names
	if duplicateInterfaces > 3 {
		t.Errorf("Expected fewer than 3 duplicate interface names after consolidation, found %d", duplicateInterfaces)
	}
	t.Logf("Successfully consolidated: only %d duplicate interface names", duplicateInterfaces)
}

// testFilesWithManyInterfaces tests that interface consolidation has reduced files with excessive interfaces
func testFilesWithManyInterfaces(t *testing.T, stats *InterfaceStats) {
	filesWithManyInterfaces := 0
	for file, interfaces := range stats.InterfacesByFile {
		if len(interfaces) > 5 {
			filesWithManyInterfaces++
			t.Logf("File %s has %d interfaces (>5): %v", file, len(interfaces), interfaces)
		}
	}

	// After consolidation, we should have fewer files with excessive interfaces
	if filesWithManyInterfaces > 2 {
		t.Errorf("Expected fewer than 2 files with >5 interfaces after consolidation, found %d", filesWithManyInterfaces)
	}
	t.Logf("Successfully consolidated: only %d files have >5 interfaces", filesWithManyInterfaces)
}

// testSingleMethodAndEmptyInterfaces tests that interface consolidation has reduced problematic patterns
func testSingleMethodAndEmptyInterfaces(t *testing.T, stats *InterfaceStats) {
	t.Logf("Found %d single-method interfaces", stats.SingleMethodInterfaces)
	t.Logf("Found %d empty interfaces", stats.EmptyInterfaces)

	// After consolidation, we should have fewer single-method interfaces
	if stats.SingleMethodInterfaces > 15 {
		t.Errorf("Expected fewer than 15 single-method interfaces after consolidation, found %d", stats.SingleMethodInterfaces)
	}
	t.Logf("Successfully consolidated: only %d single-method interfaces", stats.SingleMethodInterfaces)

	t.Run("FactoryPatterns", func(t *testing.T) {
		testFactoryPatterns(t)
	})

	t.Run("ConsolidatedTypes", func(t *testing.T) {
		testConsolidatedTypes(t)
	})

	t.Run("Summary", func(t *testing.T) {
		printSummary(t, stats)
	})
}

// testFactoryPatterns tests that factory patterns have been consolidated
func testFactoryPatterns(t *testing.T) {
	factoryFiles := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if strings.Contains(path, "factory") && strings.HasSuffix(path, ".go") {
			factoryFiles++
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	t.Logf("Found %d factory-related files", factoryFiles)

	// After consolidation, we should have fewer factory files
	if factoryFiles > 2 {
		t.Errorf("Expected fewer than 2 factory files after consolidation, found %d", factoryFiles)
	}
	t.Logf("Successfully consolidated: only %d factory files", factoryFiles)
}

// testConsolidatedTypes tests for consolidated type patterns
func testConsolidatedTypes(t *testing.T) {
	consolidatedTypes := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") || d.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if strings.Contains(string(content), "Consolidated") {
			consolidatedTypes++
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	t.Logf("Found %d files with 'Consolidated' types", consolidatedTypes)
}

// printSummary prints the interface over-engineering summary
func printSummary(t *testing.T, stats *InterfaceStats) {
	packagesWithManyInterfaces := 0
	for _, count := range stats.InterfacesByPackage {
		if count > 5 {
			packagesWithManyInterfaces++
		}
	}

	duplicateInterfaces := 0
	for _, locations := range stats.InterfaceNames {
		if len(locations) > 1 {
			duplicateInterfaces++
		}
	}

	filesWithManyInterfaces := 0
	for _, interfaces := range stats.InterfacesByFile {
		if len(interfaces) > 5 {
			filesWithManyInterfaces++
		}
	}

	// Summary
	t.Logf("\n=== Interface Over-Engineering Summary ===")
	t.Logf("Packages with >5 interfaces: %d", packagesWithManyInterfaces)
	t.Logf("Duplicate interface names: %d", duplicateInterfaces)
	t.Logf("Files with >5 interfaces: %d", filesWithManyInterfaces)
	t.Logf("Single-method interfaces: %d", stats.SingleMethodInterfaces)
	t.Logf("Empty interfaces: %d", stats.EmptyInterfaces)
}

// TestSpecificInterfacePatterns checks for specific problematic patterns
func TestSpecificInterfacePatterns(t *testing.T) {
	// Check for the dual interface system
	publicInterfacesFile := "interfaces.go"
	internalInterfacesFile := "types/interfaces.go"

	publicExists := false
	internalExists := false

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if strings.HasSuffix(path, publicInterfacesFile) {
			publicExists = true
		}
		if strings.HasSuffix(path, internalInterfacesFile) {
			internalExists = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	if publicExists && internalExists {
		t.Log("Confirmed: Dual interface system exists (public and internal interfaces)")
	}

	// Check for wrapper/adapter patterns
	wrapperCount := 0
	adapterCount := 0

	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") || d.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		wrapperCount += strings.Count(contentStr, "Wrapper")
		wrapperCount += strings.Count(contentStr, "wrapper")
		adapterCount += strings.Count(contentStr, "Adapter")
		adapterCount += strings.Count(contentStr, "adapter")

		return nil
	})

	t.Logf("Found %d wrapper references", wrapperCount)
	t.Logf("Found %d adapter references", adapterCount)

	// High counts indicate over-use of wrapper/adapter patterns
	if wrapperCount > 20 {
		t.Logf("WARNING: High wrapper count (%d) indicates potential over-engineering", wrapperCount)
	}
	if adapterCount > 20 {
		t.Logf("WARNING: High adapter count (%d) indicates potential over-engineering", adapterCount)
	}
}
