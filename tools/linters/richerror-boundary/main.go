package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Violation represents a linting violation
type Violation struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Function string `json:"function"`
	Message  string `json:"message"`
}

// Result represents the linting result
type Result struct {
	ViolationsFound int         `json:"violations_found"`
	Results         []Violation `json:"results"`
}

type lintRichErrorBoundary struct {
	fileSet    *token.FileSet
	filename   string
	violations []Violation
}

func (w *lintRichErrorBoundary) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		// Only check exported functions (boundary functions)
		if n.Name.IsExported() {
			w.checkFunctionForSimpleErrors(n)
		}
	}
	return w
}

func (w *lintRichErrorBoundary) checkFunctionForSimpleErrors(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if w.isSimpleErrorCall(x) {
				pos := w.fileSet.Position(x.Pos())
				w.violations = append(w.violations, Violation{
					File:     w.filename,
					Line:     pos.Line,
					Column:   pos.Column,
					Function: funcDecl.Name.Name,
					Message: fmt.Sprintf("exported function '%s' should use RichError instead of simple error patterns (fmt.Errorf/errors.New) for better observability and debugging",
						funcDecl.Name.Name),
				})
			}
		}
		return true
	})
}

func (w *lintRichErrorBoundary) isSimpleErrorCall(call *ast.CallExpr) bool {
	// Check for fmt.Errorf calls
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok {
			if pkg.Name == "fmt" && sel.Sel.Name == "Errorf" {
				return true
			}
			if pkg.Name == "mcperrors" && sel.Sel.Name == "New" {
				return true
			}
		}
	}
	return false
}

func main() {
	// Check if we should run fmt.Errorf counter mode
	if len(os.Args) == 3 && !strings.HasPrefix(os.Args[1], "-") {
		// Run fmt.Errorf counter from WORKSTREAM_GAMMA_PROMPT.md
		countFmtErrorf()
		return
	}

	var (
		packagePath     = flag.String("package", "pkg/mcp", "Package path to audit (defaults to pkg/mcp)")
		jsonOutput      = flag.Bool("json", false, "Output results as JSON")
		failOnViolation = flag.Bool("fail", false, "Exit with non-zero code if violations found")
	)
	flag.Parse()

	// Ensure we only run on MCP packages for architectural consistency
	if !strings.Contains(*packagePath, "pkg/mcp") {
		log.Printf("Warning: RichError boundary linter is intended for MCP packages only. Running on: %s", *packagePath)
	}

	result, err := auditPackage(*packagePath)
	if err != nil {
		log.Fatalf("Error auditing package: %v", err)
	}

	if *jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling result: %v", err)
		}
		fmt.Println(string(data))
	} else {
		for _, violation := range result.Results {
			fmt.Printf("%s:%d:%d: %s\n", violation.File, violation.Line, violation.Column, violation.Message)
		}
		fmt.Printf("Found %d violations\n", result.ViolationsFound)
	}

	if *failOnViolation && result.ViolationsFound > 0 {
		os.Exit(1)
	}
}

func auditPackage(packagePath string) (*Result, error) {
	result := &Result{
		Results: []Violation{},
	}

	fileSet := token.NewFileSet()

	err := filepath.Walk(packagePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		violations, err := auditFile(fileSet, path)
		if err != nil {
			return fmt.Errorf("error auditing file %s: %w", path, err)
		}

		result.Results = append(result.Results, violations...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	result.ViolationsFound = len(result.Results)
	return result, nil
}

func auditFile(fileSet *token.FileSet, filename string) ([]Violation, error) {
	node, err := parser.ParseFile(fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("error parsing file: %w", err)
	}

	w := &lintRichErrorBoundary{
		fileSet:    fileSet,
		filename:   filename,
		violations: []Violation{},
	}

	ast.Walk(w, node)
	return w.violations, nil
}
