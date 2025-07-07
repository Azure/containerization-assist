package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	all     = flag.Bool("all", false, "Update all files in the project")
	file    = flag.String("file", "", "Update specific file")
	dryRun  = flag.Bool("dry-run", false, "Show changes without applying them")
	verbose = flag.Bool("verbose", false, "Verbose output")
	mcpOnly = flag.Bool("mcp-only", true, "Only update MCP package files (default: true)")
)

// Import path mappings based on the new package structure
var importMappings = map[string]string{
	// Package restructuring - flattened structure
	"github.com/Azure/container-kit/pkg/mcp/internal/engine":               "github.com/Azure/container-kit/pkg/mcp/internal/runtime",
	"github.com/Azure/container-kit/pkg/mcp/internal/runtime/conversation": "github.com/Azure/container-kit/pkg/mcp/internal/runtime/conversation",
	"github.com/Azure/container-kit/pkg/mcp/internal/tools/security":       "github.com/Azure/container-kit/pkg/mcp/internal/scan",
	"github.com/Azure/container-kit/pkg/mcp/internal/tools/analysis":       "github.com/Azure/container-kit/pkg/mcp/internal/analyze",

	// Session consolidation
	"github.com/Azure/container-kit/pkg/mcp/internal/store/session": "github.com/Azure/container-kit/pkg/mcp/internal/session",
	"github.com/Azure/container-kit/pkg/mcp/internal/types/session": "github.com/Azure/container-kit/pkg/mcp/internal/session",

	// Workflow simplification
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration/workflow": "github.com/Azure/container-kit/pkg/mcp/internal/workflow",

	// Observability package
	"github.com/Azure/container-kit/pkg/logger":           "github.com/Azure/container-kit/pkg/mcp/internal/observability",
	"github.com/Azure/container-kit/pkg/mcp/internal/ops": "github.com/Azure/container-kit/pkg/mcp/internal/observability",

	// Validation package
	"github.com/Azure/container-kit/pkg/mcp/internal/validate": "github.com/Azure/container-kit/pkg/mcp/internal/validate",
}

func main() {
	flag.Parse()

	fmt.Println("MCP Import Path Update Tool")
	fmt.Println("===========================")

	if *dryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be applied")
		fmt.Println()
	}

	var filesToProcess []string
	var err error

	if *all {
		filesToProcess, err = findAllGoFiles()
		if err != nil {
			log.Fatalf("Failed to find Go files: %v", err)
		}
	} else if *file != "" {
		filesToProcess = []string{*file}
	} else {
		fmt.Println("Usage:")
		fmt.Println("  --all: Update all Go files in the project")
		fmt.Println("  --file <path>: Update specific file")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Printf("📄 Processing %d files...\n", len(filesToProcess))
	fmt.Println()

	totalChanges := 0
	changedFiles := 0

	for _, filePath := range filesToProcess {
		changes, err := processFile(filePath)
		if err != nil {
			log.Printf("⚠️  Failed to process %s: %v", filePath, err)
			continue
		}

		if changes > 0 {
			changedFiles++
			totalChanges += changes

			if *verbose {
				fmt.Printf("✅ %s: %d imports updated\n", filePath, changes)
			}
		}
	}

	fmt.Printf("\n🎉 Summary:\n")
	fmt.Printf("   Files processed: %d\n", len(filesToProcess))
	fmt.Printf("   Files changed: %d\n", changedFiles)
	fmt.Printf("   Total import updates: %d\n", totalChanges)

	if !*dryRun && totalChanges > 0 {
		fmt.Println("\n📝 Next steps:")
		fmt.Println("   1. Run 'go mod tidy' to clean up dependencies")
		fmt.Println("   2. Run 'go build ./...' to verify imports")
		fmt.Println("   3. Run tests to validate changes")
	}
}

func findAllGoFiles() ([]string, error) {
	var files []string

	// Start from pkg/mcp if mcp-only is enabled
	startPath := "."
	if *mcpOnly && !*all {
		startPath = "pkg/mcp"
	}

	err := filepath.WalkDir(startPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and .git directories
		if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git") {
			return filepath.SkipDir
		}

		// If mcp-only is enabled and -all is not set, only include MCP files
		if *mcpOnly && !*all && !strings.Contains(path, "pkg/mcp") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func processFile(filePath string) (int, error) {
	fset := token.NewFileSet()

	// Parse the file
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return 0, fmt.Errorf("failed to parse file: %w", err)
	}

	changes := 0
	modified := false

	// Process import declarations
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

			// Get the import path (without quotes)
			importPath := strings.Trim(importSpec.Path.Value, `"`)

			// Check if this import needs to be updated
			if newPath, exists := importMappings[importPath]; exists {
				if *verbose {
					fmt.Printf("📝 %s: %s -> %s\n", filePath, importPath, newPath)
				}

				if !*dryRun {
					importSpec.Path.Value = `"` + newPath + `"`
					modified = true
				}
				changes++
			}
		}
	}

	// Also check for string-based imports in comments or other contexts
	if changes == 0 {
		// Read file content and check for import paths in comments or strings
		content, err := os.ReadFile(filePath)
		if err != nil {
			return 0, fmt.Errorf("failed to read file: %w", err)
		}

		stringChanges, newContent := updateStringImports(string(content), filePath)
		if stringChanges > 0 && !*dryRun {
			err = os.WriteFile(filePath, []byte(newContent), 0644)
			if err != nil {
				return 0, fmt.Errorf("failed to write file: %w", err)
			}
		}
		changes += stringChanges
	}

	// Write the modified AST back to file
	if modified && !*dryRun {
		var buf strings.Builder
		if err := format.Node(&buf, fset, file); err != nil {
			return 0, fmt.Errorf("failed to format file: %w", err)
		}

		if err := os.WriteFile(filePath, []byte(buf.String()), 0644); err != nil {
			return 0, fmt.Errorf("failed to write file: %w", err)
		}
	}

	return changes, nil
}

func updateStringImports(content, filePath string) (int, string) {
	changes := 0
	newContent := content

	// Create regex patterns for each import mapping
	for oldPath, newPath := range importMappings {
		// Match import paths in strings, comments, and other contexts
		patterns := []string{
			// In strings
			`"` + regexp.QuoteMeta(oldPath) + `"`,
			`'` + regexp.QuoteMeta(oldPath) + `'`,
			// In comments
			regexp.QuoteMeta(oldPath),
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllString(newContent, -1)

			if len(matches) > 0 {
				if *verbose {
					fmt.Printf("📝 %s: Found %d string references to %s\n", filePath, len(matches), oldPath)
				}

				// Replace the old path with new path, preserving quotes
				replacement := strings.ReplaceAll(pattern, regexp.QuoteMeta(oldPath), newPath)
				newContent = re.ReplaceAllString(newContent, replacement)
				changes += len(matches)
			}
		}
	}

	return changes, newContent
}

// validateImports checks if the updated imports are valid
func validateImports(filePath string) error {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	return err
}
