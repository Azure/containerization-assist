package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ToolInfo contains analyzed information about a tool
type ToolInfo struct {
	Name          string
	StructName    string
	Package       string
	FilePath      string
	ArgsType      string
	ResultType    string
	ExecuteMethod string
	Category      string
	Description   string
}

// AnalyzeTools finds and analyzes all atomic tools
func (g *Generator) AnalyzeTools() ([]ToolInfo, error) {
	var tools []ToolInfo

	// Known atomic tools based on the registry
	knownTools := map[string]struct {
		structName    string
		executeMethod string
		category      string
		description   string
	}{
		"analyze_repository_atomic": {
			structName:    "AtomicAnalyzeRepositoryTool",
			executeMethod: "ExecuteRepositoryAnalysis",
			category:      "analysis",
			description:   "Analyzes repository structure and dependencies",
		},
		"generate_dockerfile": {
			structName:    "GenerateDockerfileTool",
			executeMethod: "ExecuteGeneration",
			category:      "generation",
			description:   "Generates optimized Dockerfile from repository analysis",
		},
		"validate_dockerfile_atomic": {
			structName:    "AtomicValidateDockerfileTool",
			executeMethod: "ExecuteValidation",
			category:      "validation",
			description:   "Validates Dockerfile syntax and best practices",
		},
		"build_image_atomic": {
			structName:    "AtomicBuildImageTool",
			executeMethod: "ExecuteBuild",
			category:      "build",
			description:   "Builds Docker image from Dockerfile",
		},
		"push_image_atomic": {
			structName:    "AtomicPushImageTool",
			executeMethod: "ExecutePush",
			category:      "registry",
			description:   "Pushes Docker image to registry",
		},
		"pull_image_atomic": {
			structName:    "AtomicPullImageTool",
			executeMethod: "ExecutePull",
			category:      "registry",
			description:   "Pulls Docker image from registry",
		},
		"tag_image_atomic": {
			structName:    "AtomicTagImageTool",
			executeMethod: "ExecuteTag",
			category:      "registry",
			description:   "Tags Docker image with specified tags",
		},
		"scan_image_security_atomic": {
			structName:    "AtomicScanImageSecurityTool",
			executeMethod: "ExecuteScan",
			category:      "security",
			description:   "Performs security scanning on Docker image",
		},
		"scan_secrets_atomic": {
			structName:    "AtomicScanSecretsTool",
			executeMethod: "ExecuteScan",
			category:      "security",
			description:   "Scans for secrets and sensitive information",
		},
		"generate_manifests_atomic": {
			structName:    "AtomicGenerateManifestsTool",
			executeMethod: "ExecuteGeneration",
			category:      "kubernetes",
			description:   "Generates Kubernetes manifests for deployment",
		},
		"deploy_kubernetes_atomic": {
			structName:    "AtomicDeployKubernetesTool",
			executeMethod: "ExecuteDeployment",
			category:      "kubernetes",
			description:   "Deploys application to Kubernetes cluster",
		},
		"check_health_atomic": {
			structName:    "AtomicCheckHealthTool",
			executeMethod: "ExecuteHealthCheck",
			category:      "monitoring",
			description:   "Checks health and readiness of deployed application",
		},
	}

	// For each known tool, analyze its implementation
	for toolName, info := range knownTools {
		// Try to find the file
		fileName := strings.ReplaceAll(toolName, "_atomic", "_atomic.go")
		if toolName == "generate_dockerfile" {
			fileName = "generate_dockerfile.go"
		}

		filePath := filepath.Join(g.inputDir, fileName)

		// Parse the file to get type information
		fileSet := token.NewFileSet()
		node, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
		if err != nil {
			if g.verbose {
				g.logger.Printf("Warning: Could not parse %s: %v", filePath, err)
			}
			continue
		}

		// Find the args and result types
		argsType := ""
		resultType := ""

		// Look for type declarations
		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSpec:
				name := x.Name.Name
				if strings.HasSuffix(name, "Args") && strings.Contains(name, strings.ReplaceAll(info.structName, "Tool", "")) {
					argsType = name
				}
				if strings.HasSuffix(name, "Result") && strings.Contains(name, strings.ReplaceAll(info.structName, "Tool", "")) {
					resultType = name
				}
			}
			return true
		})

		// Create tool info
		tool := ToolInfo{
			Name:          toolName,
			StructName:    info.structName,
			Package:       "tools",
			FilePath:      filePath,
			ArgsType:      argsType,
			ResultType:    resultType,
			ExecuteMethod: info.executeMethod,
			Category:      info.category,
			Description:   info.description,
		}

		// If we couldn't find the types, use generic names
		if tool.ArgsType == "" {
			tool.ArgsType = "map[string]interface{}"
		}
		if tool.ResultType == "" {
			tool.ResultType = "interface{}"
		}

		tools = append(tools, tool)

		if g.verbose {
			g.logger.Printf("Found tool: %s (struct: %s, args: %s, result: %s)",
				tool.Name, tool.StructName, tool.ArgsType, tool.ResultType)
		}
	}

	return tools, nil
}
