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
		"github.com/Azure/container-kit/pkg/mcp/domain/progress",
		"github.com/Azure/container-kit/pkg/mcp/domain/sampling",
		"github.com/Azure/container-kit/pkg/mcp/domain/session",
		"github.com/Azure/container-kit/pkg/mcp/domain/ml",
		"github.com/Azure/container-kit/pkg/mcp/domain/prompts",
		"github.com/Azure/container-kit/pkg/mcp/domain/resources",
	}

	// Forbidden imports in domain layer
	forbiddenImports := []string{
		"/infrastructure/",
		"/service/",
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

// TestNoServiceInfrastructureDependencies ensures service layer doesn't depend on infrastructure directly
func TestNoServiceInfrastructureDependencies(t *testing.T) {
	// List of service packages to check
	servicePackages := []string{
		"github.com/Azure/container-kit/pkg/mcp/service/commands",
		"github.com/Azure/container-kit/pkg/mcp/service/queries",
	}

	// Service layer can use domain, but not infrastructure directly
	forbiddenImports := []string{
		"/infrastructure/",
		"os/exec",
		"database/sql",
	}

	for _, pkgPath := range servicePackages {
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
						t.Errorf("Service package %s imports forbidden dependency: %s", pkgPath, imp)
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
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging",
		"github.com/Azure/container-kit/pkg/mcp/infrastructure/persistence/session",
	}

	forbiddenForInfra := []string{
		"/service/",
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
