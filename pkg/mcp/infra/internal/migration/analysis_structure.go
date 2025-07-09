package migration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// initializeAnalyzers sets up structural analyzers for migration detection
func (md *Detector) initializeAnalyzers() {
	md.analyzers = map[string]AnalyzerFunc{
		"complexity": func(file *ast.File, fset *token.FileSet) []Opportunity {
			return md.analyzeComplexityOpportunities(file, fset)
		},
		"interface_segregation": func(file *ast.File, fset *token.FileSet) []Opportunity {
			return md.analyzeInterfaceSegregation(file, fset)
		},
		"dependency_inversion": func(file *ast.File, fset *token.FileSet) []Opportunity {
			return md.analyzeDependencyInversion(file, fset)
		},
		"single_responsibility": func(file *ast.File, fset *token.FileSet) []Opportunity {
			return md.analyzeSingleResponsibility(file, fset)
		},
		"error_handling": func(file *ast.File, fset *token.FileSet) []Opportunity {
			return md.analyzeErrorHandling(file, fset)
		},
		"test_coverage": func(file *ast.File, fset *token.FileSet) []Opportunity {
			return md.analyzeTestCoverage(file, fset)
		},
		"naming_conventions": func(file *ast.File, fset *token.FileSet) []Opportunity {
			return md.analyzeNamingConventions(file, fset)
		},
	}
}

// analyzeStructure performs structural analysis on a Go file
func (md *Detector) analyzeStructure(filePath string) ([]Opportunity, error) {
	file, err := parser.ParseFile(md.fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var opportunities []Opportunity

	// Run all structural analyzers
	for name, analyzer := range md.analyzers {
		if results := analyzer(file, md.fileSet); len(results) > 0 {
			// Add file path and analyzer context to each opportunity
			for i := range results {
				results[i].File = filePath
				if results[i].Context == nil {
					results[i].Context = make(map[string]interface{})
				}
				results[i].Context["analyzer"] = name
			}
			opportunities = append(opportunities, results...)
		}
	}

	return opportunities, nil
}

// analyzeComplexityOpportunities identifies high complexity functions
func (md *Detector) analyzeComplexityOpportunities(file *ast.File, fset *token.FileSet) []Opportunity {
	var opportunities []Opportunity

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			complexity := md.calculateCyclomaticComplexity(fn)
			if complexity > 10 {
				pos := fset.Position(fn.Pos())
				opportunities = append(opportunities, Opportunity{
					Type:        "high_complexity",
					Priority:    md.getComplexityPriority(complexity),
					Confidence:  0.9,
					Line:        pos.Line,
					Column:      pos.Column,
					Description: "High cyclomatic complexity detected",
					Suggestion:  "Consider breaking down this function into smaller, more focused functions",
					Context: map[string]interface{}{
						"function_name": fn.Name.Name,
						"complexity":    complexity,
					},
					EstimatedEffort: md.getComplexityEffort(complexity),
				})
			}
		}
		return true
	})

	return opportunities
}

// analyzeInterfaceSegregation checks for interface segregation principle violations
func (md *Detector) analyzeInterfaceSegregation(file *ast.File, fset *token.FileSet) []Opportunity {
	var opportunities []Opportunity

	ast.Inspect(file, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				methodCount := len(interfaceType.Methods.List)
				if methodCount > 5 {
					pos := fset.Position(typeSpec.Pos())
					opportunities = append(opportunities, Opportunity{
						Type:        "large_interface",
						Priority:    "HIGH",
						Confidence:  0.85,
						Line:        pos.Line,
						Column:      pos.Column,
						Description: "Interface has too many methods",
						Suggestion:  "Split this interface into smaller, role-specific interfaces",
						Context: map[string]interface{}{
							"interface_name": typeSpec.Name.Name,
							"method_count":   methodCount,
						},
						EstimatedEffort: "MAJOR",
					})
				}
			}
		}
		return true
	})

	return opportunities
}

// analyzeDependencyInversion checks for dependency inversion principle violations
func (md *Detector) analyzeDependencyInversion(file *ast.File, fset *token.FileSet) []Opportunity {
	var opportunities []Opportunity

	// Look for concrete type dependencies in struct fields
	ast.Inspect(file, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				for _, field := range structType.Fields.List {
					if md.isConcreteTypeDependency(field.Type) {
						pos := fset.Position(field.Pos())
						opportunities = append(opportunities, Opportunity{
							Type:        "concrete_dependency",
							Priority:    "MEDIUM",
							Confidence:  0.7,
							Line:        pos.Line,
							Column:      pos.Column,
							Description: "Struct field depends on concrete type instead of interface",
							Suggestion:  "Consider using an interface instead of concrete type",
							Context: map[string]interface{}{
								"struct_name": typeSpec.Name.Name,
								"field_type":  md.getTypeName(field.Type),
							},
							EstimatedEffort: "MINOR",
						})
					}
				}
			}
		}
		return true
	})

	return opportunities
}

// analyzeSingleResponsibility checks for single responsibility principle violations
func (md *Detector) analyzeSingleResponsibility(file *ast.File, _ *token.FileSet) []Opportunity {
	var opportunities []Opportunity

	// Count methods per type
	typeMethods := make(map[string]int)

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			if len(fn.Recv.List) > 0 {
				typeName := md.getReceiverTypeName(fn.Recv.List[0].Type)
				typeMethods[typeName]++
			}
		}
	}

	// Check for types with too many methods
	for typeName, count := range typeMethods {
		if count > 10 {
			opportunities = append(opportunities, Opportunity{
				Type:        "too_many_methods",
				Priority:    "MEDIUM",
				Confidence:  0.8,
				Description: "Type has too many methods",
				Suggestion:  "Consider splitting responsibilities into separate types",
				Context: map[string]interface{}{
					"type_name":    typeName,
					"method_count": count,
				},
				EstimatedEffort: "MAJOR",
			})
		}
	}

	return opportunities
}

// analyzeErrorHandling checks for error handling patterns
func (md *Detector) analyzeErrorHandling(file *ast.File, fset *token.FileSet) []Opportunity {
	var opportunities []Opportunity

	ast.Inspect(file, func(n ast.Node) bool {
		// Check for functions returning error without proper handling
		if fn, ok := n.(*ast.FuncDecl); ok {
			if md.returnsError(fn) && !md.hasProperErrorHandling(fn) {
				pos := fset.Position(fn.Pos())
				opportunities = append(opportunities, Opportunity{
					Type:        "insufficient_error_handling",
					Priority:    "HIGH",
					Confidence:  0.75,
					Line:        pos.Line,
					Column:      pos.Column,
					Description: "Function returns error but lacks comprehensive error handling",
					Suggestion:  "Add proper error wrapping and context",
					Context: map[string]interface{}{
						"function_name": fn.Name.Name,
					},
					EstimatedEffort: "MINOR",
				})
			}
		}
		return true
	})

	return opportunities
}

// analyzeTestCoverage checks for missing tests
func (md *Detector) analyzeTestCoverage(file *ast.File, fset *token.FileSet) []Opportunity {
	var opportunities []Opportunity

	// Skip test files
	if strings.HasSuffix(file.Name.Name, "_test") {
		return opportunities
	}

	// Look for exported functions without corresponding tests
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.IsExported() {
			pos := fset.Position(fn.Pos())
			opportunities = append(opportunities, Opportunity{
				Type:        "missing_test",
				Priority:    "LOW",
				Confidence:  0.6,
				Line:        pos.Line,
				Column:      pos.Column,
				Description: "Exported function may lack test coverage",
				Suggestion:  "Add unit tests for this function",
				Context: map[string]interface{}{
					"function_name": fn.Name.Name,
				},
				EstimatedEffort: "MINOR",
			})
		}
	}

	return opportunities
}

// analyzeNamingConventions checks for naming convention violations
func (md *Detector) analyzeNamingConventions(file *ast.File, fset *token.FileSet) []Opportunity {
	var opportunities []Opportunity

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			// Check constant and variable naming
			if node.Tok == token.CONST || node.Tok == token.VAR {
				for _, spec := range node.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if !md.isProperNaming(name.Name, node.Tok) {
								pos := fset.Position(name.Pos())
								opportunities = append(opportunities, Opportunity{
									Type:        "naming_convention",
									Priority:    "LOW",
									Confidence:  0.9,
									Line:        pos.Line,
									Column:      pos.Column,
									Description: "Naming convention violation",
									Suggestion:  md.getNamingSuggestion(name.Name, node.Tok),
									Context: map[string]interface{}{
										"name": name.Name,
										"type": node.Tok.String(),
									},
									EstimatedEffort: "TRIVIAL",
								})
							}
						}
					}
				}
			}
		}
		return true
	})

	return opportunities
}

// Helper methods

func (md *Detector) calculateCyclomaticComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})
	return complexity
}

func (md *Detector) getComplexityPriority(complexity int) string {
	if complexity > 20 {
		return "HIGH"
	} else if complexity > 15 {
		return "MEDIUM"
	}
	return "LOW"
}

func (md *Detector) getComplexityEffort(complexity int) string {
	if complexity > 20 {
		return "MAJOR"
	} else if complexity > 15 {
		return "MINOR"
	}
	return "TRIVIAL"
}

func (md *Detector) isConcreteTypeDependency(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return md.isConcreteTypeDependency(t.X)
	case *ast.Ident:
		// Check if it's a known concrete type (not an interface)
		name := t.Name
		return !strings.HasSuffix(name, "er") && !strings.HasSuffix(name, "Interface")
	}
	return false
}

func (md *Detector) getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + md.getTypeName(t.X)
	case *ast.SelectorExpr:
		return md.getTypeName(t.X) + "." + t.Sel.Name
	}
	return "unknown"
}

func (md *Detector) getReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return md.getReceiverTypeName(t.X)
	}
	return ""
}

func (md *Detector) returnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, result := range fn.Type.Results.List {
		if ident, ok := result.Type.(*ast.Ident); ok && ident.Name == "error" {
			return true
		}
	}
	return false
}

func (md *Detector) hasProperErrorHandling(fn *ast.FuncDecl) bool {
	hasErrorWrap := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Errorf" || sel.Sel.Name == "Wrap" {
					hasErrorWrap = true
				}
			}
		}
		return true
	})
	return hasErrorWrap
}

func (md *Detector) isProperNaming(name string, tok token.Token) bool {
	switch tok {
	case token.CONST:
		// Constants should be CamelCase or ALL_CAPS
		return isCapitalized(name) || isAllCaps(name)
	case token.VAR:
		// Package-level vars should be lowercase or CamelCase
		return true
	}
	return true
}

func (md *Detector) getNamingSuggestion(name string, tok token.Token) string {
	switch tok {
	case token.CONST:
		if !isCapitalized(name) && !isAllCaps(name) {
			return "Consider using CamelCase or ALL_CAPS for constants"
		}
	}
	return "Follow Go naming conventions"
}

func isCapitalized(s string) bool {
	return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z'
}

func isAllCaps(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return false
		}
	}
	return true
}
