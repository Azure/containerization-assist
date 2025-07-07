package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// validateUnifiedInterfaces checks if unified interfaces exist and are properly defined
func validateUnifiedInterfaces() []ValidationResult {
	var results []ValidationResult

	// Check if pkg/mcp/interfaces.go exists
	interfacesFile := "pkg/mcp/interfaces.go"
	if _, err := os.Stat(interfacesFile); os.IsNotExist(err) {
		results = append(results, ValidationResult{
			File:     interfacesFile,
			Issue:    "Unified interfaces file does not exist - Team A work not complete",
			Severity: "error",
		})
		return results
	}

	// Parse the interfaces file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, interfacesFile, nil, parser.ParseComments)
	if err != nil {
		results = append(results, ValidationResult{
			File:     interfacesFile,
			Issue:    fmt.Sprintf("Failed to parse interfaces file: %v", err),
			Severity: "error",
		})
		return results
	}

	// Check for expected interfaces
	foundInterfaces := make(map[string]bool)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			interfaceName := typeSpec.Name.Name
			foundInterfaces[interfaceName] = true

			// Validate interface methods
			if expectedMethods, exists := expectedInterfaces[interfaceName]; exists {
				actualMethods := getInterfaceMethods(interfaceType)
				if err := validateMethods(interfaceName, expectedMethods, actualMethods); err != nil {
					results = append(results, ValidationResult{
						File:      interfacesFile,
						Interface: interfaceName,
						Issue:     err.Error(),
						Severity:  "error",
					})
				}
			}
		}
	}

	// Check for missing interfaces
	for interfaceName := range expectedInterfaces {
		if !foundInterfaces[interfaceName] {
			results = append(results, ValidationResult{
				File:      interfacesFile,
				Interface: interfaceName,
				Issue:     fmt.Sprintf("Missing expected interface: %s", interfaceName),
				Severity:  "error",
			})
		}
	}

	return results
}

// validateLegacyInterfaces checks if legacy interface files have been removed
func validateLegacyInterfaces() []ValidationResult {
	var results []ValidationResult

	for _, legacyPath := range legacyInterfaces {
		if _, err := os.Stat(legacyPath); err == nil {
			results = append(results, ValidationResult{
				File:     legacyPath,
				Issue:    "Legacy interface file still exists - should be removed",
				Severity: "error",
			})
		}
	}

	return results
}

// validateInterfaceConformance checks if tools properly implement interfaces
func validateInterfaceConformance() []ValidationResult {
	var results []ValidationResult

	// Find all tool implementations
	toolFiles, err := findToolImplementations()
	if err != nil {
		results = append(results, ValidationResult{
			Issue:    fmt.Sprintf("Failed to find tool implementations: %v", err),
			Severity: "error",
		})
		return results
	}

	for _, toolFile := range toolFiles {
		conformanceResults := validateToolConformance(toolFile)
		results = append(results, conformanceResults...)
	}

	return results
}

// validateDuplicateInterfaces checks for duplicate interface definitions
func validateDuplicateInterfaces() []ValidationResult {
	var results []ValidationResult

	// Find all interface definitions across the codebase
	interfaceDefinitions := make(map[string][]string)

	err := filepath.WalkDir("pkg/mcp", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files that can't be parsed
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					interfaceName := typeSpec.Name.Name
					interfaceDefinitions[interfaceName] = append(interfaceDefinitions[interfaceName], path)
				}
			}
		}

		return nil
	})

	if err != nil {
		results = append(results, ValidationResult{
			Issue:    fmt.Sprintf("Failed to scan for duplicate interfaces: %v", err),
			Severity: "error",
		})
		return results
	}

	// Check for duplicates
	for interfaceName, files := range interfaceDefinitions {
		if len(files) > 1 {
			results = append(results, ValidationResult{
				Interface: interfaceName,
				Issue:     fmt.Sprintf("Interface %s defined in multiple files: %v", interfaceName, files),
				Severity:  "error",
			})
		}
	}

	return results
}

// validateToolConformance validates that a tool file properly implements interfaces
func validateToolConformance(filePath string) []ValidationResult {
	var results []ValidationResult

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		results = append(results, ValidationResult{
			File:     filePath,
			Issue:    fmt.Sprintf("Failed to parse file: %v", err),
			Severity: "warning",
		})
		return results
	}

	// Look for struct types that should implement Tool interface
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if _, ok := typeSpec.Type.(*ast.StructType); ok {
				structName := typeSpec.Name.Name
				if strings.HasSuffix(structName, "Tool") {
					// This should implement the Tool interface
					// Check if it has the required methods
					if !hasRequiredMethods(file, structName, expectedInterfaces["Tool"]) {
						results = append(results, ValidationResult{
							File:     filePath,
							Issue:    fmt.Sprintf("Struct %s should implement Tool interface but missing methods", structName),
							Severity: "error",
						})
					}
				}
			}
		}
	}

	return results
}

// Helper functions

func getInterfaceMethods(interfaceType *ast.InterfaceType) []string {
	var methods []string

	for _, method := range interfaceType.Methods.List {
		if len(method.Names) > 0 {
			// Regular method
			methodName := method.Names[0].Name
			methods = append(methods, methodName)
		}
	}

	return methods
}

func validateMethods(interfaceName string, expected []string, actual []string) error {
	actualSet := make(map[string]bool)
	for _, method := range actual {
		actualSet[method] = true
	}

	var missing []string
	for _, expectedMethod := range expected {
		// Extract just the method name (before the opening parenthesis)
		methodName := strings.Split(expectedMethod, "(")[0]
		if !actualSet[methodName] {
			missing = append(missing, methodName)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("interface %s missing methods: %v", interfaceName, missing)
	}

	return nil
}

func findToolImplementations() ([]string, error) {
	var toolFiles []string

	err := filepath.WalkDir("pkg/mcp/internal", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Look for files that likely contain tool implementations
		if strings.Contains(path, "tool") || strings.Contains(path, "atomic") {
			toolFiles = append(toolFiles, path)
		}

		return nil
	})

	return toolFiles, err
}

func hasRequiredMethods(file *ast.File, structName string, requiredMethods []string) bool {
	// This is a simplified check - in practice, you'd want to check method signatures
	// For now, just check if methods with the right names exist

	methodSet := make(map[string]bool)

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		// Check if this method belongs to our struct
		recvType := getReceiverType(funcDecl.Recv)
		if recvType == structName || recvType == "*"+structName {
			methodSet[funcDecl.Name.Name] = true
		}
	}

	// Check if all required methods are present
	for _, requiredMethod := range requiredMethods {
		methodName := strings.Split(requiredMethod, "(")[0]
		if !methodSet[methodName] {
			return false
		}
	}

	return true
}

func getReceiverType(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}

	field := recv.List[0]
	switch expr := field.Type.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	}

	return ""
}
