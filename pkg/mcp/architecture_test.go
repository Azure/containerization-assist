package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Custom architecture validation using Go AST parsing
func TestArchitecturalBoundaries(t *testing.T) {
	violations := []string{}

	// Check domain layer isolation
	domainViolations := checkDomainLayerIsolation()
	violations = append(violations, domainViolations...)

	// Check service interface pattern
	serviceViolations := checkServiceInterfacePattern()
	violations = append(violations, serviceViolations...)

	// Check session management migration
	sessionViolations := checkSessionManagementMigration()
	violations = append(violations, sessionViolations...)

	// Assert no violations
	if len(violations) > 0 {
		t.Logf("Architecture violations found:")
		for _, violation := range violations {
			t.Logf("- %s", violation)
		}
		t.Fail()
	} else {
		t.Logf("Architecture validation passed: 3-bounded-context pattern is properly implemented")
	}
}

func checkDomainLayerIsolation() []string {
	violations := []string{}

	// Parse domain packages and check their imports
	domainPaths := []string{
		"pkg/mcp/domain/containerization/analyze",
		"pkg/mcp/domain/containerization/build",
		"pkg/mcp/domain/containerization/deploy",
		"pkg/mcp/domain/containerization/scan",
		"pkg/mcp/domain/validation",
	}

	for _, domainPath := range domainPaths {
		files, err := filepath.Glob(domainPath + "/*.go")
		if err != nil {
			continue // Skip if path doesn't exist
		}

		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue // Skip test files
			}

			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
			if err != nil {
				continue
			}

			for _, imp := range node.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)

				// Check for violations: domain should not import application or infra
				if strings.Contains(importPath, "/application/") {
					violations = append(violations,
						"Domain layer violation: "+file+" imports application layer: "+importPath)
				}

				if strings.Contains(importPath, "/infra/") {
					violations = append(violations,
						"Domain layer violation: "+file+" imports infrastructure layer: "+importPath)
				}
			}
		}
	}

	return violations
}

func checkServiceInterfacePattern() []string {
	violations := []string{}

	// Check that domain tools use service interfaces, not implementations
	domainPaths := []string{
		"pkg/mcp/domain/containerization/analyze",
		"pkg/mcp/domain/containerization/build",
		"pkg/mcp/domain/containerization/deploy",
		"pkg/mcp/domain/containerization/scan",
	}

	for _, domainPath := range domainPaths {
		files, err := filepath.Glob(domainPath + "/*.go")
		if err != nil {
			continue
		}

		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}

			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
			if err != nil {
				continue
			}

			for _, imp := range node.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)

				// Check for violations: should not import session implementations
				if strings.Contains(importPath, "/infra/session") {
					violations = append(violations,
						"Service interface violation: "+file+" imports session implementation: "+importPath)
				}
			}
		}
	}

	return violations
}

func checkSessionManagementMigration() []string {
	violations := []string{}

	// Check that old session manager is not used
	allPaths := []string{
		"pkg/mcp/domain",
		"pkg/mcp/application",
	}

	for _, basePath := range allPaths {
		files, err := filepath.Glob(basePath + "/**/*.go")
		if err != nil {
			continue
		}

		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}

			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
			if err != nil {
				continue
			}

			// Check for old session manager usage in AST
			ast.Inspect(node, func(n ast.Node) bool {
				if ident, ok := n.(*ast.Ident); ok {
					if ident.Name == "UnifiedSessionManager" {
						violations = append(violations,
							"Session management violation: "+file+" uses old UnifiedSessionManager")
					}
				}
				return true
			})
		}
	}

	return violations
}

func TestComplexityReduction(t *testing.T) {
	// Verify that complexity reduction was successful by checking
	// that the refactored methods exist and follow expected patterns

	// Check that ScanTool uses command pattern (check specific file relative to project root)
	scanToolFile := "domain/containerization/scan/tools.go"
	content, err := parser.ParseFile(token.NewFileSet(), scanToolFile, nil, parser.ParseComments)
	assert.NoError(t, err, "Should parse scan tools file")

	foundCommandPattern := false
	if content != nil {
		// Look for command pattern methods
		ast.Inspect(content, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if fn.Name.Name == "parseAndValidateInput" ||
					fn.Name.Name == "executeScan" ||
					fn.Name.Name == "formatScanResponse" {
					foundCommandPattern = true
				}
			}
			return true
		})
	}

	assert.True(t, foundCommandPattern, "ScanTool should use command pattern with broken down methods")

	// Check that GenerateManifestsTool uses command pattern (check specific file)
	manifestToolFile := "domain/containerization/deploy/generate_manifests.go"
	manifestContent, err := parser.ParseFile(token.NewFileSet(), manifestToolFile, nil, parser.ParseComments)
	assert.NoError(t, err, "Should parse manifest generation file")

	foundManifestPattern := false
	if manifestContent != nil {
		// Look for manifest generation methods
		ast.Inspect(manifestContent, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if strings.Contains(fn.Name.Name, "generateDeploymentManifest") ||
					strings.Contains(fn.Name.Name, "generateServiceManifest") {
					foundManifestPattern = true
				}
			}
			return true
		})
	}

	assert.True(t, foundManifestPattern, "GenerateManifestsTool should use command pattern")

	// Check that validation engine uses strategy pattern (check specific file)
	validationEngineFile := "domain/validation/engine.go"
	validationContent, err := parser.ParseFile(token.NewFileSet(), validationEngineFile, nil, parser.ParseComments)
	assert.NoError(t, err, "Should parse validation engine file")

	foundStrategyPattern := false
	if validationContent != nil {
		// Look for strategy pattern (ConstraintValidator interface)
		ast.Inspect(validationContent, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				if intf, ok := ts.Type.(*ast.InterfaceType); ok {
					if ts.Name.Name == "ConstraintValidator" && len(intf.Methods.List) > 0 {
						foundStrategyPattern = true
					}
				}
			}
			return true
		})
	}

	assert.True(t, foundStrategyPattern, "Validation engine should use strategy pattern with ConstraintValidator interface")

	t.Logf("Complexity reduction validation passed: Command and Strategy patterns properly implemented")
}
