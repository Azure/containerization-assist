package utils

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// PatternAnalyzer analyzes validation patterns across the codebase
type PatternAnalyzer struct {
	// Pattern statistics
	patternStats map[string]*PatternStat
	// Validation types found
	validationTypes map[string]int
	// Common validation functions
	validationFuncs map[string]*FunctionStat
	// Import analysis
	importStats map[string]int
	// Type definitions
	typeDefinitions map[string]*TypeDef
}

// PatternStat tracks statistics for a validation pattern
type PatternStat struct {
	Pattern     string
	Count       int
	Files       []string
	LineNumbers map[string][]int
}

// FunctionStat tracks validation function usage
type FunctionStat struct {
	Name       string
	Count      int
	Signatures []string
	Files      []string
}

// TypeDef tracks type definitions related to validation
type TypeDef struct {
	Name       string
	Package    string
	File       string
	Definition string
	Fields     []string
}

// NewPatternAnalyzer creates a new pattern analyzer
func NewPatternAnalyzer() *PatternAnalyzer {
	return &PatternAnalyzer{
		patternStats:    make(map[string]*PatternStat),
		validationTypes: make(map[string]int),
		validationFuncs: make(map[string]*FunctionStat),
		importStats:     make(map[string]int),
		typeDefinitions: make(map[string]*TypeDef),
	}
}

// AnalyzeDirectory analyzes all Go files in a directory for validation patterns
func (p *PatternAnalyzer) AnalyzeDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files if needed
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Analyze the file
		if err := p.analyzeFile(path); err != nil {
			fmt.Printf("Error analyzing %s: %v\n", path, err)
		}

		return nil
	})
}

// analyzeFile analyzes a single Go file
func (p *PatternAnalyzer) analyzeFile(filePath string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return err
	}

	// Analyze imports
	p.analyzeImports(node, filePath)

	// Analyze type definitions
	p.analyzeTypes(node, filePath, fset)

	// Analyze functions
	p.analyzeFunctions(node, filePath, fset)

	// Analyze validation patterns in content
	p.analyzePatterns(string(content), filePath)

	return nil
}

// analyzeImports analyzes import statements
func (p *PatternAnalyzer) analyzeImports(node *ast.File, filePath string) {
	ast.Inspect(node, func(n ast.Node) bool {
		if importSpec, ok := n.(*ast.ImportSpec); ok && importSpec.Path != nil {
			importPath := strings.Trim(importSpec.Path.Value, `"`)

			// Track validation-related imports
			if strings.Contains(importPath, "validation") ||
				strings.Contains(importPath, "validate") ||
				strings.Contains(importPath, "validator") {
				p.importStats[importPath]++
			}
		}
		return true
	})
}

// analyzeTypes analyzes type definitions
func (p *PatternAnalyzer) analyzeTypes(node *ast.File, filePath string, fset *token.FileSet) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			typeName := x.Name.Name

			// Check if it's validation-related
			if strings.Contains(typeName, "Validation") ||
				strings.Contains(typeName, "Validator") ||
				strings.Contains(typeName, "Valid") {

				typeDef := &TypeDef{
					Name:    typeName,
					Package: node.Name.Name,
					File:    filePath,
				}

				// Extract type definition
				switch t := x.Type.(type) {
				case *ast.StructType:
					typeDef.Definition = "struct"
					typeDef.Fields = p.extractStructFields(t)
				case *ast.InterfaceType:
					typeDef.Definition = "interface"
					typeDef.Fields = p.extractInterfaceMethods(t)
				default:
					pos := fset.Position(x.Pos())
					typeDef.Definition = fmt.Sprintf("type at line %d", pos.Line)
				}

				p.typeDefinitions[typeName] = typeDef
				p.validationTypes[typeName]++
			}
		}
		return true
	})
}

// extractStructFields extracts field names from a struct
func (p *PatternAnalyzer) extractStructFields(structType *ast.StructType) []string {
	fields := []string{}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			fields = append(fields, name.Name)
		}
	}
	return fields
}

// extractInterfaceMethods extracts method names from an interface
func (p *PatternAnalyzer) extractInterfaceMethods(interfaceType *ast.InterfaceType) []string {
	methods := []string{}
	for _, method := range interfaceType.Methods.List {
		switch m := method.Type.(type) {
		case *ast.FuncType:
			if len(method.Names) > 0 {
				methods = append(methods, method.Names[0].Name)
			}
		case *ast.Ident:
			// Embedded interface
			methods = append(methods, "embedded:"+m.Name)
		}
	}
	return methods
}

// analyzeFunctions analyzes function declarations
func (p *PatternAnalyzer) analyzeFunctions(node *ast.File, filePath string, fset *token.FileSet) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			funcName := x.Name.Name

			// Check if it's validation-related
			if p.isValidationFunction(funcName) {
				if _, exists := p.validationFuncs[funcName]; !exists {
					p.validationFuncs[funcName] = &FunctionStat{
						Name:       funcName,
						Count:      0,
						Signatures: []string{},
						Files:      []string{},
					}
				}

				stat := p.validationFuncs[funcName]
				stat.Count++

				// Extract function signature
				sig := p.extractFunctionSignature(x)
				if !contains(stat.Signatures, sig) {
					stat.Signatures = append(stat.Signatures, sig)
				}

				if !contains(stat.Files, filePath) {
					stat.Files = append(stat.Files, filePath)
				}
			}
		}
		return true
	})
}

// isValidationFunction checks if a function name is validation-related
func (p *PatternAnalyzer) isValidationFunction(name string) bool {
	validationPrefixes := []string{
		"Validate", "Valid", "Check", "Verify", "Ensure",
		"validate", "valid", "check", "verify", "ensure",
		"Is", "is", "Has", "has",
	}

	validationSuffixes := []string{
		"Validation", "Validator", "Valid", "Check",
		"validation", "validator", "valid", "check",
	}

	// Check prefixes
	for _, prefix := range validationPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	// Check suffixes
	for _, suffix := range validationSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	// Check if contains validation keywords
	nameLower := strings.ToLower(name)
	return strings.Contains(nameLower, "validat") || strings.Contains(nameLower, "check")
}

// extractFunctionSignature extracts a simplified function signature
func (p *PatternAnalyzer) extractFunctionSignature(funcDecl *ast.FuncDecl) string {
	var sig strings.Builder
	sig.WriteString(funcDecl.Name.Name)
	sig.WriteString("(")

	// Parameters
	if funcDecl.Type.Params != nil {
		params := []string{}
		for _, field := range funcDecl.Type.Params.List {
			paramType := "interface{}"
			if field.Type != nil {
				paramType = formatType(field.Type)
			}
			if len(field.Names) > 0 {
				for range field.Names {
					params = append(params, paramType)
				}
			} else {
				params = append(params, paramType)
			}
		}
		sig.WriteString(strings.Join(params, ", "))
	}
	sig.WriteString(")")

	// Return types
	if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
		sig.WriteString(" ")
		results := []string{}
		for _, field := range funcDecl.Type.Results.List {
			resultType := "interface{}"
			if field.Type != nil {
				resultType = formatType(field.Type)
			}
			results = append(results, resultType)
		}
		if len(results) == 1 {
			sig.WriteString(results[0])
		} else {
			sig.WriteString("(")
			sig.WriteString(strings.Join(results, ", "))
			sig.WriteString(")")
		}
	}

	return sig.String()
}

// formatType formats an AST type expression as a string
func formatType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatType(t.X)
	case *ast.ArrayType:
		return "[]" + formatType(t.Elt)
	case *ast.SelectorExpr:
		return formatType(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "interface{}"
	}
}

// analyzePatterns analyzes validation patterns in file content
func (p *PatternAnalyzer) analyzePatterns(content, filePath string) {
	// Common validation patterns to look for
	patterns := map[string]*regexp.Regexp{
		"ValidationResult":     regexp.MustCompile(`\bValidationResult\b`),
		"ValidationError":      regexp.MustCompile(`\bValidationError\b`),
		"ValidationOptions":    regexp.MustCompile(`\bValidationOptions\b`),
		"Validator interface":  regexp.MustCompile(`\bValidator\s+interface\b`),
		"validate method":      regexp.MustCompile(`\bfunc\s+.*\s+[Vv]alidate\s*\(`),
		"IsValid pattern":      regexp.MustCompile(`\bIsValid\s*\(`),
		"error returns":        regexp.MustCompile(`\breturn\s+.*,\s*(err|error)\b`),
		"validation tags":      regexp.MustCompile(`validate:"[^"]+"`),
		"required fields":      regexp.MustCompile(`\brequired\b`),
		"validation constants": regexp.MustCompile(`\bconst\s+.*Valid`),
	}

	for patternName, regex := range patterns {
		matches := regex.FindAllStringIndex(content, -1)
		if len(matches) > 0 {
			if _, exists := p.patternStats[patternName]; !exists {
				p.patternStats[patternName] = &PatternStat{
					Pattern:     patternName,
					Count:       0,
					Files:       []string{},
					LineNumbers: make(map[string][]int),
				}
			}

			stat := p.patternStats[patternName]
			stat.Count += len(matches)

			if !contains(stat.Files, filePath) {
				stat.Files = append(stat.Files, filePath)
			}

			// Find line numbers for matches
			lineNumbers := []int{}
			for _, match := range matches {
				lineNum := p.getLineNumber(content, match[0])
				lineNumbers = append(lineNumbers, lineNum)
			}
			stat.LineNumbers[filePath] = lineNumbers
		}
	}
}

// getLineNumber gets the line number for a given position in content
func (p *PatternAnalyzer) getLineNumber(content string, position int) int {
	line := 1
	for i := 0; i < position && i < len(content); i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}

// GenerateReport generates a comprehensive analysis report
func (p *PatternAnalyzer) GenerateReport() string {
	var report strings.Builder

	report.WriteString("=== Validation Pattern Analysis Report ===\n\n")

	// Import statistics
	report.WriteString("## Import Statistics\n")
	if len(p.importStats) > 0 {
		sortedImports := p.getSortedKeys(p.importStats)
		for _, imp := range sortedImports {
			report.WriteString(fmt.Sprintf("- %s: %d occurrences\n", imp, p.importStats[imp]))
		}
	} else {
		report.WriteString("No validation-related imports found.\n")
	}
	report.WriteString("\n")

	// Type definitions
	report.WriteString("## Validation Type Definitions\n")
	if len(p.typeDefinitions) > 0 {
		typeNames := []string{}
		for name := range p.typeDefinitions {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)

		for _, name := range typeNames {
			typeDef := p.typeDefinitions[name]
			report.WriteString(fmt.Sprintf("- %s (%s)\n", name, typeDef.Definition))
			report.WriteString(fmt.Sprintf("  Package: %s\n", typeDef.Package))
			report.WriteString(fmt.Sprintf("  File: %s\n", typeDef.File))
			if len(typeDef.Fields) > 0 {
				report.WriteString(fmt.Sprintf("  Fields/Methods: %s\n", strings.Join(typeDef.Fields, ", ")))
			}
		}
	} else {
		report.WriteString("No validation type definitions found.\n")
	}
	report.WriteString("\n")

	// Function statistics
	report.WriteString("## Validation Functions\n")
	if len(p.validationFuncs) > 0 {
		funcNames := []string{}
		for name := range p.validationFuncs {
			funcNames = append(funcNames, name)
		}
		sort.Strings(funcNames)

		for _, name := range funcNames {
			stat := p.validationFuncs[name]
			report.WriteString(fmt.Sprintf("- %s: %d occurrences in %d files\n",
				name, stat.Count, len(stat.Files)))
			if len(stat.Signatures) > 1 {
				report.WriteString("  Multiple signatures found:\n")
				for _, sig := range stat.Signatures {
					report.WriteString(fmt.Sprintf("    %s\n", sig))
				}
			}
		}
	} else {
		report.WriteString("No validation functions found.\n")
	}
	report.WriteString("\n")

	// Pattern statistics
	report.WriteString("## Pattern Usage Statistics\n")
	if len(p.patternStats) > 0 {
		patternNames := []string{}
		for name := range p.patternStats {
			patternNames = append(patternNames, name)
		}
		sort.Strings(patternNames)

		for _, name := range patternNames {
			stat := p.patternStats[name]
			report.WriteString(fmt.Sprintf("- %s: %d occurrences in %d files\n",
				name, stat.Count, len(stat.Files)))
		}
	} else {
		report.WriteString("No validation patterns found.\n")
	}
	report.WriteString("\n")

	// Summary
	report.WriteString("## Summary\n")
	report.WriteString(fmt.Sprintf("- Total validation types: %d\n", len(p.typeDefinitions)))
	report.WriteString(fmt.Sprintf("- Total validation functions: %d\n", len(p.validationFuncs)))
	report.WriteString(fmt.Sprintf("- Total validation patterns: %d\n", len(p.patternStats)))
	report.WriteString(fmt.Sprintf("- Files with validation imports: %d\n", p.countTotalFiles()))

	return report.String()
}

// getSortedKeys returns sorted keys from a map[string]int
func (p *PatternAnalyzer) getSortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// countTotalFiles counts total unique files analyzed
func (p *PatternAnalyzer) countTotalFiles() int {
	fileSet := make(map[string]bool)

	for _, stat := range p.patternStats {
		for _, file := range stat.Files {
			fileSet[file] = true
		}
	}

	for _, stat := range p.validationFuncs {
		for _, file := range stat.Files {
			fileSet[file] = true
		}
	}

	return len(fileSet)
}

// GetMigrationPriorities suggests migration priorities based on analysis
func (p *PatternAnalyzer) GetMigrationPriorities() []MigrationPriority {
	priorities := []MigrationPriority{}

	// Analyze type usage to determine priorities
	for typeName, typeDef := range p.typeDefinitions {
		priority := MigrationPriority{
			Item:     fmt.Sprintf("Type: %s", typeName),
			Priority: p.calculateTypePriority(typeName, typeDef),
			Reason:   p.getTypePriorityReason(typeName, typeDef),
		}
		priorities = append(priorities, priority)
	}

	// Sort by priority
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].Priority > priorities[j].Priority
	})

	return priorities
}

// MigrationPriority represents an item with its migration priority
type MigrationPriority struct {
	Item     string
	Priority int
	Reason   string
}

// calculateTypePriority calculates priority for a type
func (p *PatternAnalyzer) calculateTypePriority(typeName string, typeDef *TypeDef) int {
	priority := 50

	// Core types get higher priority
	if typeName == "ValidationResult" || typeName == "ValidationError" {
		priority += 40
	}

	// Interfaces get high priority
	if typeDef.Definition == "interface" {
		priority += 20
	}

	// Check usage count
	if count, exists := p.validationTypes[typeName]; exists {
		priority += count * 2
	}

	return priority
}

// getTypePriorityReason explains why a type has its priority
func (p *PatternAnalyzer) getTypePriorityReason(typeName string, typeDef *TypeDef) string {
	reasons := []string{}

	if typeName == "ValidationResult" || typeName == "ValidationError" {
		reasons = append(reasons, "Core validation type")
	}

	if typeDef.Definition == "interface" {
		reasons = append(reasons, "Interface definition")
	}

	if count, exists := p.validationTypes[typeName]; exists && count > 1 {
		reasons = append(reasons, fmt.Sprintf("Used %d times", count))
	}

	if len(reasons) == 0 {
		return "Standard validation type"
	}

	return strings.Join(reasons, ", ")
}
