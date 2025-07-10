package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// runConvert performs the automated error conversion
func runConvert(boundariesFile string) error {
	fmt.Printf("🔧 Loading boundary analysis from %s...\n", boundariesFile)

	// Load boundaries
	boundaries, err := loadBoundaries(boundariesFile)
	if err != nil {
		return fmt.Errorf("loading boundaries: %w", err)
	}

	// Filter for only BOUNDARY locations
	boundaryFiles := make(map[string]bool)
	boundaryCount := 0
	for _, info := range boundaries {
		if info.Type == "BOUNDARY" {
			// Extract file path from location (format: file:line)
			parts := strings.Split(info.Location, ":")
			if len(parts) >= 1 {
				filePath := parts[0]
				boundaryFiles[filePath] = true
				boundaryCount++
			}
		}
	}

	fmt.Printf("📝 Converting %d boundary locations in %d files...\n", boundaryCount, len(boundaryFiles))

	// Process each boundary file
	filesChanged := 0
	totalReplacements := 0

	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") || strings.HasPrefix(path, "vendor/") {
			return err
		}

		// Normalize path to match boundary file format (add ./ prefix)
		normalizedPath := "./" + path

		// Skip files that don't have boundary errors
		if !boundaryFiles[normalizedPath] {
			return nil
		}

		changed, replacements, err := processFile(normalizedPath, boundaries)
		if err != nil {
			return fmt.Errorf("processing file %s: %w", path, err)
		}

		if changed {
			filesChanged++
			totalReplacements += replacements
			fmt.Printf("  ✅ %s (%d replacements)\n", path, replacements)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	fmt.Printf("\n🎉 Conversion complete!\n")
	fmt.Printf("  Files changed: %d\n", filesChanged)
	fmt.Printf("  Total replacements: %d\n", totalReplacements)
	fmt.Printf("  Boundary functions converted to RichError\n")

	return nil
}

func loadBoundaries(path string) (map[string]*BoundaryInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var boundaries map[string]*BoundaryInfo
	if err := json.NewDecoder(file).Decode(&boundaries); err != nil {
		return nil, err
	}

	return boundaries, nil
}

func processFile(filePath string, boundaries map[string]*BoundaryInfo) (bool, int, error) {

	fset := token.NewFileSet()
	fileAst, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, 0, err
	}

	changed := false
	replacements := 0

	// Walk the AST and replace fmt.Errorf/errors.New calls in boundary functions
	ast.Inspect(fileAst, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if this is fmt.Errorf or errors.New
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		pkgIdent, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}

		var isFmtErrorf, isErrorsNew bool
		if pkgIdent.Name == "fmt" && selector.Sel.Name == "Errorf" {
			isFmtErrorf = true
		} else if pkgIdent.Name == "mcperrors" && selector.Sel.Name == "New" {
			isErrorsNew = true
		} else {
			return true
		}

		// Check if this location is in a boundary function
		pos := fset.Position(call.Pos())
		locationKey := fmt.Sprintf("%s:%d", filePath, pos.Line)

		boundaryInfo, exists := boundaries[locationKey]
		if !exists || boundaryInfo.Type != "BOUNDARY" {
			return true
		}

		// Replace the call
		if isFmtErrorf {
			replaceWithRichErrorf(call)
		} else if isErrorsNew {
			replaceWithRichError(call)
		}

		changed = true
		replacements++
		return true
	})

	if !changed {
		return false, 0, nil
	}

	// Ensure errors package is imported
	if err := ensureErrorsImport(fileAst); err != nil {
		return false, 0, fmt.Errorf("ensuring errors import: %w", err)
	}

	// Write the modified file
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, fileAst); err != nil {
		return false, 0, fmt.Errorf("formatting AST: %w", err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return false, 0, fmt.Errorf("writing file: %w", err)
	}

	return true, replacements, nil
}

func replaceWithRichErrorf(call *ast.CallExpr) {
	// Transform fmt.Errorf("format", args...) to
	// errors.NewError().Messagef("format", args...).Build()

	newBuilder := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "mcperrors"},
			Sel: &ast.Ident{Name: "NewError"},
		},
	}

	msgCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   newBuilder,
			Sel: &ast.Ident{Name: "Messagef"},
		},
		Args: call.Args, // Use original arguments
	}

	withLoc := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   msgCall,
			Sel: &ast.Ident{Name: "WithLocation"},
		},
	}

	build := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   withLoc,
			Sel: &ast.Ident{Name: "Build"},
		},
	}

	// Replace the original call
	*call = *build
}

func replaceWithRichError(call *ast.CallExpr) {
	// Transform errors.New("message") to
	// errors.NewError().Message("message").Build()

	newBuilder := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "mcperrors"},
			Sel: &ast.Ident{Name: "NewError"},
		},
	}

	msgCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   newBuilder,
			Sel: &ast.Ident{Name: "Message"},
		},
		Args: call.Args, // Use original message argument
	}

	withLoc := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   msgCall,
			Sel: &ast.Ident{Name: "WithLocation"},
		},
	}

	build := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   withLoc,
			Sel: &ast.Ident{Name: "Build"},
		},
	}

	// Replace the original call
	*call = *build
}

func ensureErrorsImport(fileAst *ast.File) error {
	// Check if errors package is already imported
	mcpErrorsImported := false

	for _, imp := range fileAst.Imports {
		if imp.Path.Value == `"github.com/Azure/container-kit/pkg/mcp/domain/errors"` {
			mcpErrorsImported = true
		}
	}

	// We need the mcp/errors package, not the standard errors package
	if !mcpErrorsImported {
		// Add import for mcp/errors
		newImport := &ast.ImportSpec{
			Name: &ast.Ident{Name: "mcperrors"},
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: `"github.com/Azure/container-kit/pkg/mcp/domain/errors"`,
			},
		}

		// Add to imports
		if fileAst.Decls[0] == nil {
			// Create import declaration
			fileAst.Decls = append([]ast.Decl{&ast.GenDecl{
				Tok:   token.IMPORT,
				Specs: []ast.Spec{newImport},
			}}, fileAst.Decls...)
		} else if genDecl, ok := fileAst.Decls[0].(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			// Add to existing import declaration
			genDecl.Specs = append(genDecl.Specs, newImport)
		} else {
			// Insert new import declaration before the first declaration
			fileAst.Decls = append([]ast.Decl{&ast.GenDecl{
				Tok:   token.IMPORT,
				Specs: []ast.Spec{newImport},
			}}, fileAst.Decls...)
		}
	}

	return nil
}
