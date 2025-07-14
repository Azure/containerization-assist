// Package main provides architectural boundary validation for the MCP server
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

// Layer represents an architectural layer
type Layer int

const (
	API Layer = iota
	Application
	Domain
	Infrastructure
)

func (l Layer) String() string {
	switch l {
	case API:
		return "API"
	case Application:
		return "Application"
	case Domain:
		return "Domain"
	case Infrastructure:
		return "Infrastructure"
	default:
		return "Unknown"
	}
}

// ArchRule defines allowed dependencies between layers
type ArchRule struct {
	From    Layer
	To      []Layer
	Allowed bool
}

// Violation represents an architectural boundary violation
type Violation struct {
	File          string
	Layer         Layer
	ImportedPkg   string
	ImportedLayer Layer
	Line          int
}

// ArchValidator validates architectural boundaries
type ArchValidator struct {
	rules      []ArchRule
	violations []Violation
	fileSet    *token.FileSet
	debug      bool
}

// NewArchValidator creates a new architectural validator
func NewArchValidator() *ArchValidator {
	return &ArchValidator{
		rules: []ArchRule{
			// API layer can only import external packages (no MCP dependencies)
			{From: API, To: []Layer{}, Allowed: true},

			// Application layer can import Domain, API, and other Application packages
			{From: Application, To: []Layer{Domain, API, Application}, Allowed: true},

			// Domain layer can import other Domain packages, API interfaces, and external packages
			{From: Domain, To: []Layer{Domain, API}, Allowed: true},

			// Infrastructure layer can import Domain and other Infrastructure packages
			{From: Infrastructure, To: []Layer{Domain, Infrastructure}, Allowed: true},
		},
		fileSet: token.NewFileSet(),
	}
}

// getPackageLayer determines which architectural layer a package belongs to
func (av *ArchValidator) getPackageLayer(pkgPath string) (Layer, bool) {
	// For file paths, we need to check the directory structure
	// For import paths, we check the full module path

	switch {
	case strings.Contains(pkgPath, "/pkg/mcp/api") || strings.Contains(pkgPath, "pkg/mcp/api"):
		return API, true
	case strings.Contains(pkgPath, "/pkg/mcp/application") || strings.Contains(pkgPath, "pkg/mcp/application"):
		return Application, true
	case strings.Contains(pkgPath, "/pkg/mcp/domain") || strings.Contains(pkgPath, "pkg/mcp/domain"):
		return Domain, true
	case strings.Contains(pkgPath, "/pkg/mcp/infrastructure") || strings.Contains(pkgPath, "pkg/mcp/infrastructure"):
		return Infrastructure, true
	case strings.Contains(pkgPath, "github.com/Azure/container-kit/pkg/mcp/"):
		// For import paths that don't match above patterns
		if strings.Contains(pkgPath, "/api") {
			return API, true
		} else if strings.Contains(pkgPath, "/application") {
			return Application, true
		} else if strings.Contains(pkgPath, "/domain") {
			return Domain, true
		} else if strings.Contains(pkgPath, "/infrastructure") {
			return Infrastructure, true
		}
		return API, false // Other MCP packages (like /pkg/mcp/di)
	default:
		return API, false // External package
	}
}

// isAllowedDependency checks if an import is allowed according to architectural rules
func (av *ArchValidator) isAllowedDependency(fromLayer, toLayer Layer) bool {
	for _, rule := range av.rules {
		if rule.From == fromLayer {
			for _, allowedLayer := range rule.To {
				if allowedLayer == toLayer {
					return true
				}
			}
			return false
		}
	}
	return false
}

// analyzeFile analyzes a single Go file for architectural violations
func (av *ArchValidator) analyzeFile(filePath string) error {
	// Parse the Go file
	node, err := parser.ParseFile(av.fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	// Determine the layer of this file
	fileLayer, isMCPPackage := av.getPackageLayer(filePath)
	if av.debug {
		fmt.Printf("File: %s, Layer: %s, isMCP: %t\n", filePath, fileLayer, isMCPPackage)
	}
	if !isMCPPackage {
		return nil // Skip non-MCP packages
	}

	// Debug output
	if av.debug {
		fmt.Printf("Analyzing file: %s (Layer: %s)\n", filePath, fileLayer)
	}

	// Analyze imports
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		if av.debug {
			fmt.Printf("  Checking import: %s\n", importPath)
		}

		// Only check MCP internal imports
		if !strings.Contains(importPath, "github.com/Azure/container-kit/pkg/mcp/") {
			if av.debug {
				fmt.Printf("    Skipping non-MCP import\n")
			}
			continue
		}

		importedLayer, isImportedMCP := av.getPackageLayer(importPath)
		if !isImportedMCP {
			if av.debug {
				fmt.Printf("    Skipping non-MCP layer import\n")
			}
			continue
		}

		if av.debug {
			fmt.Printf("  Import: %s (Layer: %s)\n", importPath, importedLayer)
		}

		// Check if this dependency is allowed
		if !av.isAllowedDependency(fileLayer, importedLayer) {
			position := av.fileSet.Position(imp.Pos())
			violation := Violation{
				File:          filePath,
				Layer:         fileLayer,
				ImportedPkg:   importPath,
				ImportedLayer: importedLayer,
				Line:          position.Line,
			}
			av.violations = append(av.violations, violation)

			if av.debug {
				fmt.Printf("    ❌ VIOLATION: %s → %s\n", fileLayer, importedLayer)
			}
		} else if av.debug {
			fmt.Printf("    ✅ OK: %s → %s\n", fileLayer, importedLayer)
		}
	}

	return nil
}

// ValidateDirectory recursively validates all Go files in a directory
func (av *ArchValidator) ValidateDirectory(rootDir string) error {
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if av.debug {
			fmt.Printf("Walking: %s\n", path)
		}

		// Skip wiring/DI directory - it needs to import from all layers
		if strings.Contains(path, "/wiring/") {
			if av.debug {
				fmt.Printf("  Skipping wiring directory: %s\n", path)
			}
			return nil
		}

		// Only process .go files, skip test files and generated files
		if !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") ||
			strings.Contains(path, "wire_gen.go") {
			if av.debug && strings.HasSuffix(path, ".go") {
				fmt.Printf("  Skipping: %s\n", path)
			}
			return nil
		}

		if av.debug {
			fmt.Printf("  Processing: %s\n", path)
		}

		return av.analyzeFile(path)
	})
}

// GetViolations returns all found violations
func (av *ArchValidator) GetViolations() []Violation {
	return av.violations
}

// PrintReport prints a detailed violation report
func (av *ArchValidator) PrintReport() {
	if len(av.violations) == 0 {
		fmt.Println("✅ No architectural violations found!")
		return
	}

	fmt.Printf("❌ Found %d architectural violations:\n\n", len(av.violations))

	// Group violations by layer
	violationsByLayer := make(map[Layer][]Violation)
	for _, v := range av.violations {
		violationsByLayer[v.Layer] = append(violationsByLayer[v.Layer], v)
	}

	// Sort layers for consistent output
	var layers []Layer
	for layer := range violationsByLayer {
		layers = append(layers, layer)
	}
	sort.Slice(layers, func(i, j int) bool {
		return layers[i] < layers[j]
	})

	for _, layer := range layers {
		violations := violationsByLayer[layer]
		fmt.Printf("## %s Layer Violations (%d)\n", layer, len(violations))

		for _, v := range violations {
			relPath := strings.TrimPrefix(v.File, "/home/tng/workspace/container-kit/")
			fmt.Printf("  • %s:%d\n", relPath, v.Line)
			fmt.Printf("    %s layer importing from %s layer\n", v.Layer, v.ImportedLayer)
			fmt.Printf("    Import: %s\n", v.ImportedPkg)
			fmt.Println()
		}
	}

	// Print summary and recommendations
	fmt.Println("## Recommendations")
	fmt.Println()
	fmt.Println("1. **Apply Dependency Inversion Principle**")
	fmt.Println("   - Create domain interfaces for imported infrastructure services")
	fmt.Println("   - Use dependency injection to provide implementations")
	fmt.Println()
	fmt.Println("2. **Refactor Application Layer**")
	fmt.Println("   - Move concrete type references to dependency injection providers")
	fmt.Println("   - Application should only depend on domain interfaces")
	fmt.Println()
	fmt.Println("3. **Update Wire Configuration**")
	fmt.Println("   - Infrastructure creation should happen in DI layer")
	fmt.Println("   - Application should receive interfaces, not concrete types")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: arch-validate <directory>")
		fmt.Println("Example: arch-validate ./pkg/mcp")
		os.Exit(1)
	}

	rootDir := os.Args[1]
	validator := NewArchValidator()

	fmt.Printf("🔍 Analyzing architectural boundaries in %s...\n", rootDir)
	fmt.Println("   (Note: wiring/DI directories are excluded as they need to import from all layers)")

	// Add debug mode
	debug := len(os.Args) > 2 && os.Args[2] == "--debug"
	validator.debug = debug
	if debug {
		fmt.Println("Debug mode enabled")
	}

	err := validator.ValidateDirectory(rootDir)
	if err != nil {
		fmt.Printf("Error during validation: %v\n", err)
		os.Exit(1)
	}

	if debug {
		fmt.Printf("Total violations found: %d\n", len(validator.GetViolations()))
	}

	validator.PrintReport()

	// Exit with error code if violations found
	if len(validator.GetViolations()) > 0 {
		os.Exit(1)
	}
}
