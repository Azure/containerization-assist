package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// This is the fmt.Errorf counter from WORKSTREAM_GAMMA_PROMPT.md
// It tracks progress toward the <10 fmt.Errorf goal

func countFmtErrorf() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <directory> <max-fmt-errorf>\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Args[1]
	maxFmtErrorf, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid max-fmt-errorf value: %v\n", err)
		os.Exit(1)
	}

	count := 0
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files with parse errors
		}

		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok {
						if id.Name == "fmt" && sel.Sel.Name == "Errorf" {
							count++
							pos := fset.Position(call.Pos())
							fmt.Printf("%s:%d: fmt.Errorf usage found\n", pos.Filename, pos.Line)
						}
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total fmt.Errorf usage: %d (max allowed: %d)\n", count, maxFmtErrorf)

	if count > maxFmtErrorf {
		fmt.Printf("❌ fmt.Errorf usage exceeds limit\n")
		os.Exit(1)
	}

	fmt.Printf("✅ fmt.Errorf usage within limit\n")
}
