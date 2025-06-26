package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ToolInfoV2 contains enhanced information about a tool
type ToolInfoV2 struct {
	Name          string
	StructName    string
	Package       string
	FilePath      string
	Constructor   ConstructorInfo
	ExecuteMethod MethodInfo
	ArgsType      TypeInfo
	ResultType    TypeInfo
	Category      string
	Description   string
}

// ConstructorInfo contains information about tool constructor
type ConstructorInfo struct {
	Name       string
	Parameters []ParameterInfo
}

// ParameterInfo contains information about a function parameter
type ParameterInfo struct {
	Name string
	Type string
}

// MethodInfo contains information about a method
type MethodInfo struct {
	Name       string
	Receiver   string
	Parameters []ParameterInfo
	Returns    []TypeInfo
}

// TypeInfo contains detailed type information
type TypeInfo struct {
	Name      string
	Package   string
	IsPointer bool
	Fields    []FieldInfo
}

// FieldInfo contains information about a struct field
type FieldInfo struct {
	Name     string
	Type     string
	JSONTag  string
	Required bool
}

// AnalyzeToolsV2 performs enhanced analysis of tools
func (g *Generator) AnalyzeToolsV2() ([]ToolInfoV2, error) {
	var tools []ToolInfoV2

	// Enhanced tool information
	toolData := map[string]struct {
		structName    string
		executeMethod string
		category      string
		description   string
		fileName      string
	}{
		"analyze_repository_atomic": {
			structName:    "AtomicAnalyzeRepositoryTool",
			executeMethod: "ExecuteRepositoryAnalysis",
			category:      "analysis",
			description:   "Analyzes repository structure and dependencies",
			fileName:      "analyze_repository_atomic.go",
		},
		"generate_dockerfile": {
			structName:    "GenerateDockerfileTool",
			executeMethod: "ExecuteGeneration",
			category:      "generation",
			description:   "Generates optimized Dockerfile from repository analysis",
			fileName:      "generate_dockerfile.go",
		},
		"validate_dockerfile_atomic": {
			structName:    "AtomicValidateDockerfileTool",
			executeMethod: "ExecuteValidation",
			category:      "validation",
			description:   "Validates Dockerfile syntax and best practices",
			fileName:      "validate_dockerfile_atomic.go",
		},
		"build_image_atomic": {
			structName:    "AtomicBuildImageTool",
			executeMethod: "ExecuteBuild",
			category:      "build",
			description:   "Builds Docker image from Dockerfile",
			fileName:      "build_image_atomic.go",
		},
		"push_image_atomic": {
			structName:    "AtomicPushImageTool",
			executeMethod: "ExecutePush",
			category:      "registry",
			description:   "Pushes Docker image to registry",
			fileName:      "push_image_atomic.go",
		},
		"pull_image_atomic": {
			structName:    "AtomicPullImageTool",
			executeMethod: "ExecutePull",
			category:      "registry",
			description:   "Pulls Docker image from registry",
			fileName:      "pull_image_atomic.go",
		},
		"tag_image_atomic": {
			structName:    "AtomicTagImageTool",
			executeMethod: "ExecuteTag",
			category:      "registry",
			description:   "Tags Docker image with specified tags",
			fileName:      "tag_image_atomic.go",
		},
		"scan_image_security_atomic": {
			structName:    "AtomicScanImageSecurityTool",
			executeMethod: "ExecuteScan",
			category:      "security",
			description:   "Performs security scanning on Docker image",
			fileName:      "scan_image_security_atomic.go",
		},
		"scan_secrets_atomic": {
			structName:    "AtomicScanSecretsTool",
			executeMethod: "ExecuteScan",
			category:      "security",
			description:   "Scans for secrets and sensitive information",
			fileName:      "scan_secrets_atomic.go",
		},
		"generate_manifests_atomic": {
			structName:    "AtomicGenerateManifestsTool",
			executeMethod: "ExecuteGeneration",
			category:      "kubernetes",
			description:   "Generates Kubernetes manifests for deployment",
			fileName:      "generate_manifests_atomic.go",
		},
		"deploy_kubernetes_atomic": {
			structName:    "AtomicDeployKubernetesTool",
			executeMethod: "ExecuteDeployment",
			category:      "kubernetes",
			description:   "Deploys application to Kubernetes cluster",
			fileName:      "deploy_kubernetes_atomic.go",
		},
		"check_health_atomic": {
			structName:    "AtomicCheckHealthTool",
			executeMethod: "ExecuteHealthCheck",
			category:      "monitoring",
			description:   "Checks health and readiness of deployed application",
			fileName:      "check_health_atomic.go",
		},
	}

	// Analyze each tool file
	for toolName, info := range toolData {
		filePath := filepath.Join(g.inputDir, info.fileName)

		tool, err := g.analyzeToolFile(filePath, toolName, info)
		if err != nil {
			if g.verbose {
				g.logger.Printf("Warning: Failed to analyze %s: %v", toolName, err)
			}
			continue
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

// analyzeToolFile analyzes a single tool file
func (g *Generator) analyzeToolFile(filePath string, toolName string, info struct {
	structName    string
	executeMethod string
	category      string
	description   string
	fileName      string
}) (ToolInfoV2, error) {

	fileSet := token.NewFileSet()
	node, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return ToolInfoV2{}, fmt.Errorf("failed to parse file: %w", err)
	}

	tool := ToolInfoV2{
		Name:        toolName,
		StructName:  info.structName,
		Package:     "tools",
		FilePath:    filePath,
		Category:    info.category,
		Description: info.description,
	}

	// Find constructor, execute method, and types
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Check for constructor
			if strings.HasPrefix(x.Name.Name, "New") && strings.Contains(x.Name.Name, info.structName) {
				tool.Constructor = g.extractConstructorInfo(x)
			}

			// Check for execute method
			if x.Recv != nil && x.Name.Name == info.executeMethod {
				tool.ExecuteMethod = g.extractMethodInfo(x)
			}

		case *ast.TypeSpec:
			// Find args and result types
			typeName := x.Name.Name
			if strings.Contains(typeName, "Args") && strings.Contains(typeName, strings.ReplaceAll(info.structName, "Tool", "")) {
				tool.ArgsType = g.extractTypeInfo(x)
			}
			if strings.Contains(typeName, "Result") && strings.Contains(typeName, strings.ReplaceAll(info.structName, "Tool", "")) {
				tool.ResultType = g.extractTypeInfo(x)
			}
		}
		return true
	})

	return tool, nil
}

// extractConstructorInfo extracts constructor information
func (g *Generator) extractConstructorInfo(fn *ast.FuncDecl) ConstructorInfo {
	info := ConstructorInfo{
		Name: fn.Name.Name,
	}

	// Extract parameters
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			param := ParameterInfo{
				Type: g.typeToString(field.Type),
			}
			if len(field.Names) > 0 {
				param.Name = field.Names[0].Name
			}
			info.Parameters = append(info.Parameters, param)
		}
	}

	return info
}

// extractMethodInfo extracts method information
func (g *Generator) extractMethodInfo(fn *ast.FuncDecl) MethodInfo {
	info := MethodInfo{
		Name: fn.Name.Name,
	}

	// Extract receiver
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		info.Receiver = g.typeToString(fn.Recv.List[0].Type)
	}

	// Extract parameters
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			param := ParameterInfo{
				Type: g.typeToString(field.Type),
			}
			if len(field.Names) > 0 {
				param.Name = field.Names[0].Name
			}
			info.Parameters = append(info.Parameters, param)
		}
	}

	// Extract returns
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			returnType := TypeInfo{
				Name: g.typeToString(field.Type),
			}
			info.Returns = append(info.Returns, returnType)
		}
	}

	return info
}

// extractTypeInfo extracts type information from a type spec
func (g *Generator) extractTypeInfo(spec *ast.TypeSpec) TypeInfo {
	info := TypeInfo{
		Name: spec.Name.Name,
	}

	// Extract struct fields if it's a struct
	if structType, ok := spec.Type.(*ast.StructType); ok {
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue // Skip embedded fields for now
			}

			fieldInfo := FieldInfo{
				Name: field.Names[0].Name,
				Type: g.typeToString(field.Type),
			}

			// Extract JSON tag
			if field.Tag != nil {
				tag := field.Tag.Value
				if strings.Contains(tag, `json:"`) {
					start := strings.Index(tag, `json:"`) + 6
					end := strings.Index(tag[start:], `"`)
					jsonTag := tag[start : start+end]
					parts := strings.Split(jsonTag, ",")
					fieldInfo.JSONTag = parts[0]
					fieldInfo.Required = !strings.Contains(jsonTag, "omitempty")
				}
			}

			info.Fields = append(info.Fields, fieldInfo)
		}
	}

	return info
}

// typeToString converts an AST expression to a string representation
func (g *Generator) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + g.typeToString(t.X)
	case *ast.SelectorExpr:
		return g.typeToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + g.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + g.typeToString(t.Key) + "]" + g.typeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "unknown"
	}
}
