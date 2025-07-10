package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type ComplexityChecker struct {
	violations       []ComplexityViolation
	maxComplexity    int
	allowedFunctions map[string]int // Function name -> allowed complexity
}

type ComplexityViolation struct {
	Function   string
	File       string
	Line       int
	Complexity int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <directory>\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Args[1]
	checker := &ComplexityChecker{
		maxComplexity: 20,
		allowedFunctions: map[string]int{
			// Known complex functions that are allowed higher complexity
			"registerCommonFixes": 45, // Current: 40, allow some headroom
			"chainMatches":        25, // Current: 21, allow some headroom
			"RegisterTools":       30, // Current: 27, allow some headroom
		},
	}

	err := checker.CheckComplexity(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	checker.PrintReport()

	if len(checker.violations) > 0 {
		os.Exit(1)
	}
}

func (c *ComplexityChecker) CheckComplexity(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Body != nil {
				complexity := c.calculateComplexity(fn.Body)

				// Check if this function has a custom allowed complexity
				allowedComplexity := c.maxComplexity
				if customLimit, exists := c.allowedFunctions[fn.Name.Name]; exists {
					allowedComplexity = customLimit
				}

				if complexity > allowedComplexity {
					position := fset.Position(fn.Pos())

					violation := ComplexityViolation{
						Function:   fn.Name.Name,
						File:       path,
						Line:       position.Line,
						Complexity: complexity,
					}

					c.violations = append(c.violations, violation)
				}
			}
			return true
		})

		return nil
	})
}

func (c *ComplexityChecker) calculateComplexity(body *ast.BlockStmt) int {
	complexity := 1 // Base complexity

	ast.Inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})

	return complexity
}

func (c *ComplexityChecker) PrintReport() {
	fmt.Printf("=== COMPLEXITY CHECKER REPORT ===\n")
	fmt.Printf("Maximum allowed complexity: %d\n", c.maxComplexity)

	// Show allowed exceptions
	if len(c.allowedFunctions) > 0 {
		fmt.Printf("Functions with custom complexity limits:\n")
		for funcName, limit := range c.allowedFunctions {
			fmt.Printf("  - %s(): %d\n", funcName, limit)
		}
	}

	fmt.Printf("Violations found: %d\n", len(c.violations))

	if len(c.violations) == 0 {
		fmt.Printf("✅ PASS: All functions within complexity limits\n")
		return
	}

	fmt.Printf("❌ FAIL: Functions exceeding complexity limit:\n")
	for _, violation := range c.violations {
		fmt.Printf("  %s() in %s:%d (complexity: %d)\n",
			violation.Function, violation.File, violation.Line, violation.Complexity)
	}

	fmt.Printf("\nSuggestions:\n")
	fmt.Printf("- Break complex functions into smaller helper functions\n")
	fmt.Printf("- Extract nested logic into separate methods\n")
	fmt.Printf("- Use early returns to reduce nesting\n")
	fmt.Printf("- Consider using function objects for complex state management\n")
}
