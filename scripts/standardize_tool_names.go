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

// ToolInfo represents information about a tool
type ToolInfo struct {
	FilePath   string
	StructName string
	Package    string
}

// ToolNamingInfo represents naming analysis results
type ToolNamingInfo struct {
	FilePath           string
	CurrentFileName    string
	CurrentStructName  string
	ExpectedFileName   string
	ExpectedStructName string
	NeedsRename        bool
}

// ToolNamingStandard defines naming conventions for tools
type ToolNamingStandard struct {
	UseAtomicPrefix   bool
	FileNamePattern   string
	StructNamePattern string
}

func main() {
	// Define the packages to scan
	packages := []string{
		"pkg/mcp/internal/analyze",
		"pkg/mcp/internal/build",
		"pkg/mcp/internal/deploy",
		"pkg/mcp/internal/scan",
		"pkg/mcp/internal/observability",
		"pkg/mcp/internal/server",
		"pkg/mcp/internal/session",
	}

	// Find all tools
	var tools []*ToolInfo
	for _, pkg := range packages {
		pkgPath := filepath.Join("/home/tng/workspace/tools", pkg)
		pkgTools, err := findToolsInPackage(pkgPath)
		if err != nil {
			fmt.Printf("Error scanning package %s: %v\n", pkg, err)
			continue
		}
		tools = append(tools, pkgTools...)
	}

	// Analyze naming
	standard := &ToolNamingStandard{
		UseAtomicPrefix:   true,
		FileNamePattern:   "%s_atomic.go",
		StructNamePattern: "Atomic%sTool",
	}
	categories := []string{"analyze", "build", "deploy", "scan"}

	var needsRename []*ToolNamingInfo
	fmt.Println("=== Tool Naming Analysis ===")

	for _, tool := range tools {
		// Determine if this tool should use Atomic pattern
		toolName := extractToolNameFromStruct(tool.StructName)
		shouldBeAtomic := shouldUseAtomicPattern(toolName, categories)

		// Adjust standard based on tool category
		toolStandard := *standard
		if !shouldBeAtomic {
			toolStandard.UseAtomicPrefix = false
		}

		info := analyzeToolNaming(tool.FilePath, tool.StructName, &toolStandard)

		if info.NeedsRename {
			needsRename = append(needsRename, info)
			fmt.Printf("❌ %s\n", tool.FilePath)
			fmt.Printf("   Current:  %s -> %s\n", info.CurrentFileName, info.CurrentStructName)
			fmt.Printf("   Expected: %s -> %s\n", info.ExpectedFileName, info.ExpectedStructName)
			fmt.Printf("   Category: %s\n\n", getToolCategory(toolName, categories))
		} else {
			fmt.Printf("✅ %s (correctly named)\n", tool.FilePath)
		}
	}

	// Generate rename commands
	if len(needsRename) > 0 {
		fmt.Printf("\n=== Rename Commands ===\n")
		fmt.Println("# Run these commands to standardize tool names:")
		fmt.Println("# WARNING: Review each command before running!")
		fmt.Println()

		commands := generateRenameCommands(needsRename)
		for _, cmd := range commands {
			fmt.Println(cmd)
		}

		// Generate a script file
		scriptPath := "/home/tng/workspace/tools/scripts/rename_tools.sh"
		err := generateRenameScript(scriptPath, commands)
		if err != nil {
			fmt.Printf("\nError generating script: %v\n", err)
		} else {
			fmt.Printf("\n# Or run the generated script:\n")
			fmt.Printf("chmod +x %s\n", scriptPath)
			fmt.Printf("%s\n", scriptPath)
		}
	}

	// Summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total tools found: %d\n", len(tools))
	fmt.Printf("Tools needing rename: %d\n", len(needsRename))
	fmt.Printf("Correctly named: %d\n", len(tools)-len(needsRename))
}

// findToolsInPackage finds all tool structs in a package
func findToolsInPackage(pkgPath string) ([]*ToolInfo, error) {
	var tools []*ToolInfo

	err := filepath.Walk(pkgPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse the file
		fileTools, err := findToolsInFile(path)
		if err != nil {
			fmt.Printf("Error parsing %s: %v\n", path, err)
			return nil
		}

		tools = append(tools, fileTools...)
		return nil
	})

	return tools, err
}

// findToolsInFile finds tool structs in a single file
func findToolsInFile(filePath string) ([]*ToolInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var tools []*ToolInfo

	// Look for struct declarations that end with "Tool"
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if _, ok := x.Type.(*ast.StructType); ok {
				name := x.Name.Name
				if strings.HasSuffix(name, "Tool") {
					tools = append(tools, &ToolInfo{
						FilePath:   filePath,
						StructName: name,
						Package:    node.Name.Name,
					})
				}
			}
		}
		return true
	})

	return tools, nil
}

// extractToolNameFromStruct extracts the base tool name from a struct name
func extractToolNameFromStruct(structName string) string {
	name := strings.TrimSuffix(structName, "Tool")
	name = strings.TrimPrefix(name, "Atomic")
	// Convert to snake_case
	return toSnakeCase(name)
}

// toSnakeCase converts CamelCase to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// getToolCategory returns the category of a tool
func getToolCategory(toolName string, categories []string) string {
	for _, cat := range categories {
		if strings.Contains(toolName, cat) {
			return cat
		}
	}
	return "Unknown"
}

// shouldUseAtomicPattern determines if a tool should use the Atomic pattern
func shouldUseAtomicPattern(toolName string, _ []string) bool {
	// Core tools should use Atomic pattern
	coreTools := []string{"analyze", "build", "deploy", "scan"}
	for _, core := range coreTools {
		if strings.Contains(toolName, core) {
			return true
		}
	}
	return false
}

// analyzeToolNaming analyzes a tool's naming
func analyzeToolNaming(filePath, structName string, standard *ToolNamingStandard) *ToolNamingInfo {
	baseName := filepath.Base(filePath)
	toolName := extractToolNameFromStruct(structName)

	var expectedFileName, expectedStructName string
	if standard.UseAtomicPrefix {
		expectedFileName = fmt.Sprintf(standard.FileNamePattern, toolName)
		expectedStructName = fmt.Sprintf(standard.StructNamePattern, toCamelCase(toolName))
	} else {
		expectedFileName = toolName + ".go"
		expectedStructName = toCamelCase(toolName) + "Tool"
	}

	return &ToolNamingInfo{
		FilePath:           filePath,
		CurrentFileName:    baseName,
		CurrentStructName:  structName,
		ExpectedFileName:   expectedFileName,
		ExpectedStructName: expectedStructName,
		NeedsRename:        baseName != expectedFileName || structName != expectedStructName,
	}
}

// toCamelCase converts snake_case to CamelCase
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// generateRenameCommands generates rename commands for tools
func generateRenameCommands(needsRename []*ToolNamingInfo) []string {
	var commands []string

	for _, info := range needsRename {
		dir := filepath.Dir(info.FilePath)
		newPath := filepath.Join(dir, info.ExpectedFileName)

		// File rename command
		if info.CurrentFileName != info.ExpectedFileName {
			commands = append(commands, fmt.Sprintf("git mv %s %s", info.FilePath, newPath))
		}

		// Struct rename command (as a comment)
		if info.CurrentStructName != info.ExpectedStructName {
			commands = append(commands, fmt.Sprintf("# In %s: rename struct %s to %s",
				newPath, info.CurrentStructName, info.ExpectedStructName))
		}
	}

	return commands
}

// generateRenameScript generates a shell script with rename commands
func generateRenameScript(scriptPath string, commands []string) error {
	content := []string{
		"#!/bin/bash",
		"# Tool Naming Standardization Script",
		"# Generated by standardize_tool_names.go",
		"",
		"set -e",
		"",
		"echo 'Starting tool naming standardization...'",
		"",
	}

	content = append(content, commands...)
	content = append(content, "", "echo 'Tool naming standardization complete!'")

	return os.WriteFile(scriptPath, []byte(strings.Join(content, "\n")), 0600)
}
