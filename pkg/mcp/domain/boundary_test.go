// Package domain_test provides architecture boundary tests to ensure clean architecture principles
package domain_test

import (
	"go/build"
	"strings"
	"testing"
)

// TestNoDomainInfrastructureDependencies ensures domain layer doesn't depend on infrastructure
func TestNoDomainInfrastructureDependencies(t *testing.T) {
	// List of domain packages to check
	domainPackages := []string{
		"github.com/Azure/container-kit/pkg/mcp/domain/workflow",
		"github.com/Azure/container-kit/pkg/mcp/domain/events",
		"github.com/Azure/container-kit/pkg/mcp/domain/saga",
		"github.com/Azure/container-kit/pkg/mcp/domain/progress",
		"github.com/Azure/container-kit/pkg/mcp/domain/sampling",
		"github.com/Azure/container-kit/pkg/mcp/domain/session",
		"github.com/Azure/container-kit/pkg/mcp/domain/errors",
	}

	// Forbidden imports in domain layer
	forbiddenImports := []string{
		"/infrastructure/",
		"/application/",
		"os/exec",
		"database/sql",
		"net/http",
	}

	for _, pkgPath := range domainPackages {
		t.Run(pkgPath, func(t *testing.T) {
			pkg, err := build.Import(pkgPath, "", build.IgnoreVendor)
			if err != nil {
				t.Skipf("Skipping %s: %v", pkgPath, err)
				return
			}

			// Check all imports
			allImports := append(pkg.Imports, pkg.TestImports...)
			for _, imp := range allImports {
				for _, forbidden := range forbiddenImports {
					if strings.Contains(imp, forbidden) {
						t.Errorf("Domain package %s imports forbidden dependency: %s", pkgPath, imp)
					}
				}
			}
		})
	}
}

// TestNoApplicationInfrastructureDependencies ensures application layer doesn't depend on infrastructure directly
func TestNoApplicationInfrastructureDependencies(t *testing.T) {
	// List of application packages to check
	applicationPackages := []string{
		"github.com/Azure/container-kit/pkg/mcp/application/commands",
		"github.com/Azure/container-kit/pkg/mcp/application/queries",
		"github.com/Azure/container-kit/pkg/mcp/application/workflow",
	}

	// Application layer can use domain, but not infrastructure directly
	forbiddenImports := []string{
		"/infrastructure/",
		"os/exec",
		"database/sql",
	}

	for _, pkgPath := range applicationPackages {
		t.Run(pkgPath, func(t *testing.T) {
			pkg, err := build.Import(pkgPath, "", build.IgnoreVendor)
			if err != nil {
				t.Skipf("Skipping %s: %v", pkgPath, err)
				return
			}

			// Check all imports
			for _, imp := range pkg.Imports {
				for _, forbidden := range forbiddenImports {
					if strings.Contains(imp, forbidden) {
						t.Errorf("Application package %s imports forbidden dependency: %s", pkgPath, imp)
					}
				}
			}
		})
	}
}

// TestLayerDependencyDirection ensures dependencies only flow in the correct direction
func TestLayerDependencyDirection(t *testing.T) {
	// Infrastructure should not import from application or api
	infrastructurePackages := []string{
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/container",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/kubernetes",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/persistence",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/steps",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/ml",
	}

	forbiddenForInfra := []string{
		"/application/",
		"/api/",
	}

	for _, pkgPath := range infrastructurePackages {
		t.Run(pkgPath, func(t *testing.T) {
			pkg, err := build.Import(pkgPath, "", build.IgnoreVendor)
			if err != nil {
				t.Skipf("Skipping %s: %v", pkgPath, err)
				return
			}

			// Check all imports
			for _, imp := range pkg.Imports {
				for _, forbidden := range forbiddenForInfra {
					if strings.Contains(imp, forbidden) {
						t.Errorf("Infrastructure package %s imports from higher layer: %s", pkgPath, imp)
					}
				}
			}
		})
	}
}
