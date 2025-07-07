package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
)

// BoundaryInfo holds information about whether a location is a boundary
type BoundaryInfo struct {
	Location string `json:"location"`
	Type     string `json:"type"` // "BOUNDARY" or "INTERNAL"
	Function string `json:"function,omitempty"`
	Package  string `json:"package,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func runBoundaries(outputFile string) error {

	// Read error inventory CSV
	locations, err := readErrorLocations("/tmp/error_inventory.csv")
	if err != nil {
		return fmt.Errorf("reading error inventory: %w", err)
	}

	fmt.Printf("🔍 Analyzing %d error locations for boundary detection...\n", len(locations))

	boundaries := make(map[string]*BoundaryInfo)

	for _, location := range locations {
		info, err := analyzeBoundary(location)
		if err != nil {
			fmt.Printf("Warning: failed to analyze %s: %v\n", location, err)
			continue
		}
		boundaries[location] = info
	}

	// Write results to JSON
	if err := writeBoundariesJSON(boundaries, outputFile); err != nil {
		return fmt.Errorf("writing boundaries JSON: %w", err)
	}

	// Print summary
	printSummary(boundaries)
	fmt.Printf("✅ Boundary analysis saved to %s\n", outputFile)
	return nil
}

func readErrorLocations(csvFile string) ([]string, error) {
	file, err := os.Open(csvFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var locations []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			locations = append(locations, line)
		}
	}

	return locations, scanner.Err()
}

func analyzeBoundary(location string) (*BoundaryInfo, error) {
	// Parse location format: file:line
	parts := strings.Split(location, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid location format: %s", location)
	}

	fileName := parts[0]
	lineNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid line number: %s", parts[1])
	}

	// Parse the Go file
	fset := token.NewFileSet()
	fileAst, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file %s: %w", fileName, err)
	}

	// Find the function containing the error location
	funcInfo := findContainingFunction(fileAst, fset, lineNum)
	if funcInfo == nil {
		return &BoundaryInfo{
			Location: location,
			Type:     "INTERNAL",
			Reason:   "not in function",
		}, nil
	}

	// Determine if this is a boundary
	isBoundary, reason := isBoundaryFunction(funcInfo, fileName)

	info := &BoundaryInfo{
		Location: location,
		Function: funcInfo.Name,
		Package:  fileAst.Name.Name,
		Reason:   reason,
	}

	if isBoundary {
		info.Type = "BOUNDARY"
	} else {
		info.Type = "INTERNAL"
	}

	return info, nil
}

type FunctionInfo struct {
	Name       string
	IsExported bool
	Doc        string
}

func findContainingFunction(fileAst *ast.File, fset *token.FileSet, targetLine int) *FunctionInfo {
	var result *FunctionInfo

	ast.Inspect(fileAst, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			startPos := fset.Position(fn.Pos())
			endPos := fset.Position(fn.End())

			if targetLine >= startPos.Line && targetLine <= endPos.Line {
				result = &FunctionInfo{
					Name:       fn.Name.Name,
					IsExported: ast.IsExported(fn.Name.Name),
				}
				if fn.Doc != nil {
					result.Doc = fn.Doc.Text()
				}
				return false // Found it, stop searching
			}
		}
		return true
	})

	return result
}

func isBoundaryFunction(funcInfo *FunctionInfo, fileName string) (bool, string) {
	// Rule 1: Exported functions are boundaries
	if funcInfo.IsExported {
		return true, "exported function"
	}

	// Rule 2: Functions in transport, api, handler, server, rpc packages
	if strings.Contains(fileName, "/transport/") {
		return true, "transport package"
	}
	if strings.Contains(fileName, "/api/") {
		return true, "api package"
	}
	if strings.Contains(fileName, "/handler/") {
		return true, "handler package"
	}
	if strings.Contains(fileName, "/server/") {
		return true, "server package"
	}
	if strings.Contains(fileName, "/rpc/") {
		return true, "rpc package"
	}

	// Rule 3: MCP root package functions
	if strings.Contains(fileName, "/mcp/") && !strings.Contains(fileName, "/internal/") {
		return true, "mcp public package"
	}

	// Rule 4: Functions that appear to handle stdio errors
	if strings.Contains(funcInfo.Name, "StdioError") || strings.Contains(funcInfo.Name, "ErrorHandler") {
		return true, "stdio error handler"
	}

	// Rule 5: Tool interface implementations (common pattern)
	if strings.Contains(funcInfo.Name, "Execute") || strings.Contains(funcInfo.Name, "Call") || strings.Contains(funcInfo.Name, "Invoke") {
		return true, "tool interface method"
	}

	return false, "internal helper"
}

func writeBoundariesJSON(boundaries map[string]*BoundaryInfo, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(boundaries)
}

func printSummary(boundaries map[string]*BoundaryInfo) {
	var boundaryCount, internalCount int
	boundaryReasons := make(map[string]int)

	for _, info := range boundaries {
		if info.Type == "BOUNDARY" {
			boundaryCount++
			boundaryReasons[info.Reason]++
		} else {
			internalCount++
		}
	}

	fmt.Printf("\n📊 Boundary Analysis Summary:\n")
	fmt.Printf("  Total locations: %d\n", len(boundaries))
	fmt.Printf("  Boundary functions: %d\n", boundaryCount)
	fmt.Printf("  Internal functions: %d\n", internalCount)

	if len(boundaryReasons) > 0 {
		fmt.Printf("\n🎯 Boundary reasons:\n")
		for reason, count := range boundaryReasons {
			fmt.Printf("  %s: %d\n", reason, count)
		}
	}
}
