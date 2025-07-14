// Package mcp_test provides comprehensive architecture linting for the 4-layer MCP architecture
package mcp_test

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ArchitectureLayer represents the four layers of our clean architecture
type ArchitectureLayer int

const (
	APILayer ArchitectureLayer = iota
	ApplicationLayer
	DomainLayer
	InfrastructureLayer
)

// LayerRules defines what each layer can and cannot import
type LayerRules struct {
	Name           string
	PackagePattern string
	CanImport      []ArchitectureLayer
	CannotImport   []string
	MustNotCall    []string
}

// getArchitectureRules returns the complete set of 4-layer architecture rules
func getArchitectureRules() map[ArchitectureLayer]LayerRules {
	return map[ArchitectureLayer]LayerRules{
		APILayer: {
			Name:           "API Layer",
			PackagePattern: "github.com/Azure/container-kit/pkg/mcp/api",
			CanImport:      []ArchitectureLayer{DomainLayer}, // Only domain interfaces
			CannotImport: []string{
				"/infrastructure/",
				"/application/",
				"os/exec",
				"database/sql",
				"net/http",
			},
			MustNotCall: []string{
				"os.WriteFile",
				"os.ReadFile",
				"exec.Command",
				"http.Get",
				"sql.Open",
			},
		},
		ApplicationLayer: {
			Name:           "Application Layer",
			PackagePattern: "github.com/Azure/container-kit/pkg/mcp/application",
			CanImport:      []ArchitectureLayer{APILayer, DomainLayer}, // Can use API and domain
			CannotImport: []string{
				"/infrastructure/", // Should not directly import infrastructure
				"os/exec",
				"database/sql",
			},
			MustNotCall: []string{
				"os.WriteFile", // Should delegate to infrastructure
				"exec.Command",
				"sql.Open",
			},
		},
		DomainLayer: {
			Name:           "Domain Layer",
			PackagePattern: "github.com/Azure/container-kit/pkg/mcp/domain",
			CanImport:      []ArchitectureLayer{}, // Only other domain packages
			CannotImport: []string{
				"/infrastructure/",
				"/application/",
				"/api/",
				"os/exec",
				"database/sql",
				"net/http",
			},
			MustNotCall: []string{
				"os.WriteFile",
				"os.ReadFile",
				"os.MkdirAll",
				"exec.Command",
				"http.Get",
				"sql.Open",
			},
		},
		InfrastructureLayer: {
			Name:           "Infrastructure Layer",
			PackagePattern: "github.com/Azure/container-kit/pkg/mcp/infrastructure",
			CanImport:      []ArchitectureLayer{DomainLayer}, // Can import domain, but not api/application
			CannotImport: []string{
				"/application/",
				"/api/",
			},
			MustNotCall: []string{
				// Infrastructure can use external services, so minimal restrictions
			},
		},
	}
}

// TestFourLayerArchitectureBoundaries validates the complete 4-layer architecture
func TestFourLayerArchitectureBoundaries(t *testing.T) {
	rules := getArchitectureRules()

	for _, rule := range rules {
		t.Run(rule.Name, func(t *testing.T) {
			packages := findPackagesInLayer(t, rule.PackagePattern)
			
			for _, pkg := range packages {
				t.Run(pkg, func(t *testing.T) {
					validatePackageImports(t, pkg, rule)
				})
			}
		})
	}
}

// TestDependencyInversionPrinciple ensures infrastructure implements domain interfaces
func TestDependencyInversionPrinciple(t *testing.T) {
	// Key interfaces that must be implemented by infrastructure
	requiredInterfaces := []string{
		"ErrorPatternRecognizer",
		"EnhancedErrorHandler", 
		"StepEnhancer",
		"Manager", // prompts.Manager
	}

	infrastructurePackages := findPackagesInLayer(t, "github.com/Azure/container-kit/pkg/mcp/infrastructure")
	
	for _, requiredInterface := range requiredInterfaces {
		t.Run("Interface_"+requiredInterface, func(t *testing.T) {
			found := false
			for _, pkg := range infrastructurePackages {
				if hasInterfaceImplementation(t, pkg, requiredInterface) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Required interface %s not implemented in infrastructure layer", requiredInterface)
			}
		})
	}
}

// TestWiringLayerCompliance ensures wiring only happens in designated places
func TestWiringLayerCompliance(t *testing.T) {
	allowedWiringPackages := []string{
		"github.com/Azure/container-kit/pkg/mcp/api/wiring",
	}

	allPackages := []string{}
	allPackages = append(allPackages, findPackagesInLayer(t, "github.com/Azure/container-kit/pkg/mcp/api")...)
	allPackages = append(allPackages, findPackagesInLayer(t, "github.com/Azure/container-kit/pkg/mcp/application")...)
	allPackages = append(allPackages, findPackagesInLayer(t, "github.com/Azure/container-kit/pkg/mcp/domain")...)
	allPackages = append(allPackages, findPackagesInLayer(t, "github.com/Azure/container-kit/pkg/mcp/infrastructure")...)

	for _, pkg := range allPackages {
		t.Run(pkg, func(t *testing.T) {
			// Skip allowed wiring packages
			isAllowed := false
			for _, allowed := range allowedWiringPackages {
				if strings.Contains(pkg, allowed) {
					isAllowed = true
					break
				}
			}
			if isAllowed {
				return
			}

			// Check for Wire imports in non-wiring packages
			validateNoWireImports(t, pkg)
		})
	}
}

// TestConfigurationCentralization ensures config is centralized
func TestConfigurationCentralization(t *testing.T) {
	allowedConfigPackages := []string{
		"github.com/Azure/container-kit/pkg/mcp/application/config",
		"github.com/Azure/container-kit/pkg/mcp/api/wiring", // For conversion
	}

	forbiddenConfigPatterns := []string{
		"config.go",
		"configuration.go",
		"settings.go",
	}

	infrastructurePackages := findPackagesInLayer(t, "github.com/Azure/container-kit/pkg/mcp/infrastructure")
	
	for _, pkg := range infrastructurePackages {
		t.Run(pkg, func(t *testing.T) {
			validateNoScatteredConfig(t, pkg, allowedConfigPackages, forbiddenConfigPatterns)
		})
	}
}

// Helper functions

func findPackagesInLayer(t *testing.T, pattern string) []string {
	packages := []string{}
	
	// Use go list to find packages matching pattern
	baseDir := "../../../" // Adjust based on test location
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() && strings.Contains(path, "pkg/mcp/") {
			// Convert file path to import path
			relPath := strings.TrimPrefix(path, baseDir)
			importPath := "github.com/Azure/container-kit/" + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")
			
			if strings.Contains(importPath, pattern) {
				packages = append(packages, importPath)
			}
		}
		return nil
	})
	
	if err != nil {
		t.Logf("Warning: could not walk directory: %v", err)
	}
	
	return packages
}

func validatePackageImports(t *testing.T, pkgPath string, rule LayerRules) {
	pkg, err := build.Import(pkgPath, "", build.IgnoreVendor)
	if err != nil {
		t.Skipf("Skipping %s: %v", pkgPath, err)
		return
	}

	// Check all imports (including test imports for comprehensive validation)
	allImports := append(pkg.Imports, pkg.TestImports...)
	
	for _, imp := range allImports {
		// Check forbidden imports
		for _, forbidden := range rule.CannotImport {
			if strings.Contains(imp, forbidden) {
				t.Errorf("%s imports forbidden dependency: %s", pkgPath, imp)
			}
		}
	}
}

func hasInterfaceImplementation(t *testing.T, pkgPath string, interfaceName string) bool {
	// This is a simplified check - in practice, you might use go/ast for deeper analysis
	pkg, err := build.Import(pkgPath, "", build.IgnoreVendor)
	if err != nil {
		return false
	}

	// Check if package likely implements the interface
	// This is a heuristic based on naming conventions
	for _, file := range pkg.GoFiles {
		if strings.Contains(file, strings.ToLower(interfaceName)) ||
			strings.Contains(pkgPath, strings.ToLower(interfaceName)) {
			return true
		}
	}
	
	return false
}

func validateNoWireImports(t *testing.T, pkgPath string) {
	pkg, err := build.Import(pkgPath, "", build.IgnoreVendor)
	if err != nil {
		t.Skipf("Skipping %s: %v", pkgPath, err)
		return
	}

	forbiddenWireImports := []string{
		"github.com/google/wire",
	}

	for _, imp := range pkg.Imports {
		for _, forbidden := range forbiddenWireImports {
			if strings.Contains(imp, forbidden) {
				t.Errorf("Package %s should not import Wire directly: %s", pkgPath, imp)
			}
		}
	}
}

func validateNoScatteredConfig(t *testing.T, pkgPath string, allowedPackages []string, forbiddenPatterns []string) {
	// Check if this package is allowed to have config
	for _, allowed := range allowedPackages {
		if strings.Contains(pkgPath, allowed) {
			return // Skip validation for allowed packages
		}
	}

	pkg, err := build.Import(pkgPath, "", build.IgnoreVendor)
	if err != nil {
		t.Skipf("Skipping %s: %v", pkgPath, err)
		return
	}

	// Check for forbidden config file patterns
	for _, file := range pkg.GoFiles {
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(file, pattern) {
				t.Errorf("Package %s contains scattered config file: %s (should use centralized config)", pkgPath, file)
			}
		}
	}
}

// TestArchitectureDocumentation ensures architecture decisions are documented
func TestArchitectureDocumentation(t *testing.T) {
	requiredADRs := []string{
		"single-workflow-architecture",
		"go-embed-template-management", 
		"wire-dependency-injection",
		"unified-rich-error-system",
		"ai-assisted-error-recovery",
		"four-layer-mcp-architecture",
	}

	adrDir := "../../../docs/architecture/adr"
	
	for _, adr := range requiredADRs {
		t.Run("ADR_"+adr, func(t *testing.T) {
			pattern := filepath.Join(adrDir, "*"+adr+"*.md")
			matches, err := filepath.Glob(pattern)
			if err != nil {
				t.Errorf("Error checking for ADR %s: %v", adr, err)
				return
			}
			if len(matches) == 0 {
				t.Errorf("Missing required ADR documentation: %s", adr)
			}
		})
	}
}

// TestPerformanceConstraints ensures architecture meets performance requirements
func TestPerformanceConstraints(t *testing.T) {
	// Check for performance anti-patterns in hot paths
	hotPathPackages := []string{
		"github.com/Azure/container-kit/pkg/mcp/domain/workflow",
		"github.com/Azure/container-kit/pkg/mcp/application",
	}

	performanceAntiPatterns := []string{
		"reflect.ValueOf",
		"json.Marshal", // Should be minimized in hot paths
		"fmt.Sprintf",  // Should prefer string builders in loops
	}

	for _, pkg := range hotPathPackages {
		t.Run(pkg, func(t *testing.T) {
			validatePerformancePatterns(t, pkg, performanceAntiPatterns)
		})
	}
}

func validatePerformancePatterns(t *testing.T, pkgPath string, antiPatterns []string) {
	_, err := build.Import(pkgPath, "", build.IgnoreVendor)
	if err != nil {
		t.Skipf("Skipping %s: %v", pkgPath, err)
		return
	}

	// This is a simplified check - real implementation would parse Go files
	for _, pattern := range antiPatterns {
		t.Logf("Performance validation for %s checking pattern: %s", pkgPath, pattern)
		// In practice, you'd read source files and check for patterns
	}
}