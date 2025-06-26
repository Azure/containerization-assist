package utils

import (
	"fmt"
	"path/filepath"
	"strings"

	utils "github.com/Azure/container-kit/pkg/mcp/utils"
)

// ToolNamingStandard defines the standard naming conventions for tools
type ToolNamingStandard struct {
	// UseAtomicPrefix indicates if tools should use the "Atomic" prefix
	UseAtomicPrefix bool

	// FileNamePattern defines the file naming pattern
	// If UseAtomicPrefix is true: "tool_name_atomic.go"
	// If UseAtomicPrefix is false: "tool_name.go"
	FileNamePattern string

	// StructNamePattern defines the struct naming pattern
	// If UseAtomicPrefix is true: "AtomicToolNameTool"
	// If UseAtomicPrefix is false: "ToolNameTool"
	StructNamePattern string
}

// DefaultNamingStandard returns the default naming standard
// We'll standardize on using the Atomic prefix for all tools
func DefaultNamingStandard() *ToolNamingStandard {
	return &ToolNamingStandard{
		UseAtomicPrefix:   true,
		FileNamePattern:   "%s_atomic.go",
		StructNamePattern: "Atomic%sTool",
	}
}

// ToolNamingInfo contains information about a tool's naming
type ToolNamingInfo struct {
	FilePath           string
	CurrentFileName    string
	CurrentStructName  string
	ExpectedFileName   string
	ExpectedStructName string
	NeedsRename        bool
}

// AnalyzeToolNaming analyzes a tool's naming against the standard
func AnalyzeToolNaming(filePath string, structName string, standard *ToolNamingStandard) *ToolNamingInfo {
	fileName := filepath.Base(filePath)
	baseName := strings.TrimSuffix(fileName, ".go")

	// Extract the tool name from various patterns
	toolName := extractToolName(baseName, structName)

	// Generate expected names
	var expectedFileName, expectedStructName string
	if standard.UseAtomicPrefix {
		expectedFileName = fmt.Sprintf(standard.FileNamePattern, toolName)
		expectedStructName = fmt.Sprintf(standard.StructNamePattern, toCamelCase(toolName))
	} else {
		expectedFileName = toolName + ".go"
		expectedStructName = toCamelCase(toolName) + "Tool"
	}

	needsRename := fileName != expectedFileName || structName != expectedStructName

	return &ToolNamingInfo{
		FilePath:           filePath,
		CurrentFileName:    fileName,
		CurrentStructName:  structName,
		ExpectedFileName:   expectedFileName,
		ExpectedStructName: expectedStructName,
		NeedsRename:        needsRename,
	}
}

// extractToolName extracts the base tool name from file or struct name
func extractToolName(fileName string, structName string) string {
	// Remove common suffixes/prefixes from file name
	name := strings.TrimSuffix(fileName, "_atomic")
	name = strings.TrimSuffix(name, "_tool")

	// If the file name doesn't give us enough info, use the struct name
	if name == "" || name == "build" || name == "deploy" {
		// Convert struct name to snake_case
		name = toSnakeCaseNaming(structName)
		name = strings.TrimPrefix(name, "atomic_")
		name = strings.TrimSuffix(name, "_tool")
	}

	return name
}

// Use consolidated string utilities
// toCamelCase converts snake_case to CamelCase
func toCamelCase(s string) string {
	return utils.ToCamelCase(s)
}

// toSnakeCaseNaming converts CamelCase to snake_case (specific to tool naming)
func toSnakeCaseNaming(s string) string {
	return utils.ToSnakeCase(s)
}

// GenerateRenameCommands generates shell commands to rename files
func GenerateRenameCommands(infos []*ToolNamingInfo) []string {
	var commands []string

	for _, info := range infos {
		if !info.NeedsRename {
			continue
		}

		dir := filepath.Dir(info.FilePath)
		newPath := filepath.Join(dir, info.ExpectedFileName)

		// Add file rename command
		if info.CurrentFileName != info.ExpectedFileName {
			commands = append(commands, fmt.Sprintf("mv %s %s", info.FilePath, newPath))
		}

		// Add sed command to rename struct
		if info.CurrentStructName != info.ExpectedStructName {
			sedCmd := fmt.Sprintf("sed -i 's/%s/%s/g' %s",
				info.CurrentStructName, info.ExpectedStructName, newPath)
			commands = append(commands, sedCmd)
		}
	}

	return commands
}

// ToolCategories defines different categories of tools
type ToolCategories struct {
	// Core tools that should always use Atomic pattern
	CoreTools []string

	// Utility tools that might not need Atomic pattern
	UtilityTools []string

	// Management tools for server/session management
	ManagementTools []string
}

// DefaultToolCategories returns the default tool categorization
func DefaultToolCategories() *ToolCategories {
	return &ToolCategories{
		CoreTools: []string{
			"analyze_repository",
			"build_image",
			"push_image",
			"pull_image",
			"tag_image",
			"generate_dockerfile",
			"validate_dockerfile",
			"generate_manifests",
			"deploy_kubernetes",
			"check_health",
			"scan_secrets",
			"scan_image_security",
		},
		UtilityTools: []string{
			"chat",
			"validate_deployment",
		},
		ManagementTools: []string{
			"list_sessions",
			"delete_session",
			"add_session_label",
			"remove_session_label",
			"get_server_health",
			"get_logs",
		},
	}
}

// ShouldUseAtomicPattern determines if a tool should use the Atomic pattern
func ShouldUseAtomicPattern(toolName string, categories *ToolCategories) bool {
	// Check if it's a core tool
	for _, core := range categories.CoreTools {
		if strings.Contains(toolName, core) {
			return true
		}
	}

	// Utility and management tools don't need Atomic pattern
	for _, util := range categories.UtilityTools {
		if strings.Contains(toolName, util) {
			return false
		}
	}

	for _, mgmt := range categories.ManagementTools {
		if strings.Contains(toolName, mgmt) {
			return false
		}
	}

	// Default to using Atomic pattern for unknown tools
	return true
}
