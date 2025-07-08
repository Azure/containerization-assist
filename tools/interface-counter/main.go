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

type InterfaceCounter struct {
	interfaces map[string][]InterfaceInfo
	total      int
}

type InterfaceInfo struct {
	Name    string
	File    string
	Line    int
	Methods int
	Package string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <directory>\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Args[1]
	counter := &InterfaceCounter{
		interfaces: make(map[string][]InterfaceInfo),
	}

	err := counter.CountInterfaces(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	counter.PrintReport()

	// Exit with error code if over limit
	if counter.total > 50 {
		fmt.Fprintf(os.Stderr, "ERROR: Interface count %d exceeds limit of 50\n", counter.total)
		os.Exit(1)
	}
}

func (c *InterfaceCounter) CountInterfaces(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		packageName := node.Name.Name

		ast.Inspect(node, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok {
				if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					position := fset.Position(typeSpec.Pos())

					info := InterfaceInfo{
						Name:    typeSpec.Name.Name,
						File:    path,
						Line:    position.Line,
						Methods: len(interfaceType.Methods.List),
						Package: packageName,
					}

					c.interfaces[packageName] = append(c.interfaces[packageName], info)
					c.total++
				}
			}
			return true
		})

		return nil
	})
}

func (c *InterfaceCounter) PrintReport() {
	fmt.Printf("=== INTERFACE COUNT REPORT ===\n")
	fmt.Printf("Total interfaces: %d\n", c.total)
	fmt.Printf("Target limit: 50\n")

	if c.total > 50 {
		fmt.Printf("Status: ❌ OVER LIMIT by %d interfaces\n", c.total-50)
	} else {
		fmt.Printf("Status: ✅ WITHIN LIMIT (%d remaining)\n", 50-c.total)
	}

	fmt.Printf("\nPer-package breakdown:\n")
	for pkg, interfaces := range c.interfaces {
		fmt.Printf("  %s: %d interfaces\n", pkg, len(interfaces))
		for _, iface := range interfaces {
			fmt.Printf("    - %s (%d methods) at %s:%d\n",
				iface.Name, iface.Methods, iface.File, iface.Line)
		}
	}

	if c.total > 40 {
		fmt.Printf("\n⚠️  WARNING: Approaching interface limit (40+)\n")
		fmt.Printf("Consider consolidating interfaces or removing unused ones.\n")
	}
}