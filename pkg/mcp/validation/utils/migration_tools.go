package utils

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// MigrationTool provides utilities for automated migration detection and conversion
type MigrationTool struct {
	// Map of old validation patterns to new validator types
	patternMap map[string]string
	// Files that need migration
	migrationCandidates []MigrationCandidate
}

// MigrationCandidate represents a file that needs migration
type MigrationCandidate struct {
	FilePath        string
	ValidationType  string
	OldPatterns     []string
	SuggestedAction string
	Priority        int
}

// NewMigrationTool creates a new migration tool instance
func NewMigrationTool() *MigrationTool {
	return &MigrationTool{
		patternMap: map[string]string{
			"ValidationResult":   "core.ValidationResult",
			"ValidateImage":      "ImageValidator",
			"ValidateDockerfile": "DockerValidator",
			"ValidateSecurity":   "SecurityValidator",
			"ValidateNetwork":    "NetworkValidator",
			"ValidateFormat":     "FormatValidator",
			"ValidateContext":    "ContextValidator",
			"ValidateManifest":   "ManifestValidator",
			"ValidateHealth":     "HealthValidator",
			"ValidateDeployment": "DeploymentValidator",
			"CheckSecrets":       "SecurityValidator",
			"CheckPermissions":   "SecurityValidator",
			"CheckCompliance":    "SecurityValidator",
			"ValidateSyntax":     "SyntaxValidator",
			"ValidateYAML":       "FormatValidator",
			"ValidateJSON":       "FormatValidator",
		},
		migrationCandidates: []MigrationCandidate{},
	}
}

// ScanDirectory scans a directory for files that need migration
func (m *MigrationTool) ScanDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip already migrated files
		if strings.Contains(path, "pkg/mcp/validation") {
			return nil
		}

		// Analyze the file
		if candidate := m.analyzeFile(path); candidate != nil {
			m.migrationCandidates = append(m.migrationCandidates, *candidate)
		}

		return nil
	})
}

// analyzeFile analyzes a single Go file for migration patterns
func (m *MigrationTool) analyzeFile(filePath string) *MigrationCandidate {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil
	}

	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil
	}

	candidate := &MigrationCandidate{
		FilePath:    filePath,
		OldPatterns: []string{},
	}

	// Check for validation-related imports
	hasValidationImports := false
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			if x.Path != nil {
				path := strings.Trim(x.Path.Value, `"`)
				if strings.Contains(path, "validation") || strings.Contains(path, "validate") {
					hasValidationImports = true
				}
			}
		}
		return true
	})

	if !hasValidationImports && !m.containsValidationPatterns(string(content)) {
		return nil
	}

	// Look for validation patterns in the code
	patterns := m.findValidationPatterns(node, string(content))
	if len(patterns) == 0 {
		return nil
	}

	candidate.OldPatterns = patterns
	candidate.ValidationType = m.determineValidationType(patterns, filePath)
	candidate.SuggestedAction = m.generateMigrationAction(candidate)
	candidate.Priority = m.calculatePriority(candidate)

	return candidate
}

// containsValidationPatterns does a quick check for validation-related content
func (m *MigrationTool) containsValidationPatterns(content string) bool {
	validationKeywords := []string{
		"Validate", "validate", "Validation", "validation",
		"Valid", "valid", "Check", "check",
		"Verify", "verify", "SecurityCheck", "Compliance",
	}

	for _, keyword := range validationKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

// findValidationPatterns finds specific validation patterns in the AST
func (m *MigrationTool) findValidationPatterns(node *ast.File, content string) []string {
	patterns := []string{}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Check function names
			if x.Name != nil {
				funcName := x.Name.Name
				for pattern := range m.patternMap {
					if strings.Contains(funcName, pattern) {
						patterns = append(patterns, funcName)
					}
				}
			}
		case *ast.TypeSpec:
			// Check type names
			if x.Name != nil {
				typeName := x.Name.Name
				if strings.Contains(typeName, "Validation") || strings.Contains(typeName, "Validator") {
					patterns = append(patterns, typeName)
				}
			}
		case *ast.CallExpr:
			// Check function calls
			if ident, ok := x.Fun.(*ast.Ident); ok {
				for pattern := range m.patternMap {
					if strings.Contains(ident.Name, pattern) {
						patterns = append(patterns, ident.Name)
					}
				}
			}
		}
		return true
	})

	// Also check for string patterns in content
	for pattern := range m.patternMap {
		if strings.Contains(content, pattern) && !contains(patterns, pattern) {
			patterns = append(patterns, pattern)
		}
	}

	return unique(patterns)
}

// determineValidationType determines what type of validation is being performed
func (m *MigrationTool) determineValidationType(patterns []string, filePath string) string {
	// Check file path for hints
	pathLower := strings.ToLower(filePath)
	if strings.Contains(pathLower, "security") {
		return "security"
	}
	if strings.Contains(pathLower, "build") {
		return "build"
	}
	if strings.Contains(pathLower, "deploy") {
		return "deployment"
	}
	if strings.Contains(pathLower, "scan") {
		return "scan"
	}
	if strings.Contains(pathLower, "network") {
		return "network"
	}
	if strings.Contains(pathLower, "format") {
		return "format"
	}

	// Check patterns for type hints
	for _, pattern := range patterns {
		patternLower := strings.ToLower(pattern)
		if strings.Contains(patternLower, "security") || strings.Contains(patternLower, "secret") {
			return "security"
		}
		if strings.Contains(patternLower, "docker") || strings.Contains(patternLower, "image") {
			return "docker"
		}
		if strings.Contains(patternLower, "network") || strings.Contains(patternLower, "port") {
			return "network"
		}
		if strings.Contains(patternLower, "format") || strings.Contains(patternLower, "json") || strings.Contains(patternLower, "yaml") {
			return "format"
		}
		if strings.Contains(patternLower, "manifest") || strings.Contains(patternLower, "kubernetes") {
			return "kubernetes"
		}
	}

	return "general"
}

// generateMigrationAction generates suggested migration action
func (m *MigrationTool) generateMigrationAction(candidate *MigrationCandidate) string {
	actions := []string{}

	// Import changes
	actions = append(actions, "1. Add import: \"github.com/Azure/container-kit/pkg/mcp/validation/core\"")

	// Based on validation type, suggest appropriate validator
	switch candidate.ValidationType {
	case "security":
		actions = append(actions, "2. Replace with SecurityValidator from validators package")
	case "docker":
		actions = append(actions, "2. Replace with DockerValidator from validators package")
	case "network":
		actions = append(actions, "2. Replace with NetworkValidator from validators package")
	case "format":
		actions = append(actions, "2. Replace with FormatValidator from validators package")
	case "kubernetes":
		actions = append(actions, "2. Create new KubernetesValidator in validators package")
	default:
		actions = append(actions, "2. Create appropriate validator in validators package")
	}

	// Type replacements
	for _, pattern := range candidate.OldPatterns {
		if newType, ok := m.patternMap[pattern]; ok {
			actions = append(actions, fmt.Sprintf("3. Replace '%s' with '%s'", pattern, newType))
		}
	}

	return strings.Join(actions, "\n")
}

// calculatePriority calculates migration priority based on various factors
func (m *MigrationTool) calculatePriority(candidate *MigrationCandidate) int {
	priority := 50 // Base priority

	// Increase priority for core packages
	if strings.Contains(candidate.FilePath, "internal/build") {
		priority += 30
	}
	if strings.Contains(candidate.FilePath, "internal/deploy") {
		priority += 25
	}
	if strings.Contains(candidate.FilePath, "internal/scan") {
		priority += 20
	}
	if strings.Contains(candidate.FilePath, "internal/runtime") {
		priority += 20
	}

	// Increase priority based on pattern count
	priority += len(candidate.OldPatterns) * 5

	// Cap priority at 100
	if priority > 100 {
		priority = 100
	}

	return priority
}

// GetMigrationReport generates a migration report
func (m *MigrationTool) GetMigrationReport() string {
	if len(m.migrationCandidates) == 0 {
		return "No files need migration"
	}

	// Sort by priority
	sortByPriority(m.migrationCandidates)

	report := fmt.Sprintf("Found %d files needing migration:\n\n", len(m.migrationCandidates))

	for i, candidate := range m.migrationCandidates {
		report += fmt.Sprintf("%d. %s (Priority: %d)\n", i+1, candidate.FilePath, candidate.Priority)
		report += fmt.Sprintf("   Type: %s\n", candidate.ValidationType)
		report += fmt.Sprintf("   Patterns found: %s\n", strings.Join(candidate.OldPatterns, ", "))
		report += fmt.Sprintf("   Suggested actions:\n%s\n\n", indent(candidate.SuggestedAction, "   "))
	}

	return report
}

// GenerateMigrationScript generates a shell script to help with migration
func (m *MigrationTool) GenerateMigrationScript() string {
	script := "#!/bin/bash\n\n"
	script += "# Auto-generated migration script for validation framework\n"
	script += "# Review each change before applying\n\n"

	for _, candidate := range m.migrationCandidates {
		script += fmt.Sprintf("echo \"Migrating %s...\"\n", candidate.FilePath)

		// Generate sed commands for simple replacements
		for oldPattern, newPattern := range m.patternMap {
			if contains(candidate.OldPatterns, oldPattern) {
				script += fmt.Sprintf("sed -i 's/%s/%s/g' %s\n", oldPattern, newPattern, candidate.FilePath)
			}
		}

		script += "\n"
	}

	script += "echo \"Migration script completed. Please review changes and run tests.\"\n"
	return script
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func unique(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func sortByPriority(candidates []MigrationCandidate) {
	// Simple bubble sort for priority
	for i := 0; i < len(candidates)-1; i++ {
		for j := 0; j < len(candidates)-i-1; j++ {
			if candidates[j].Priority < candidates[j+1].Priority {
				candidates[j], candidates[j+1] = candidates[j+1], candidates[j]
			}
		}
	}
}

func indent(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		if lines[i] != "" {
			lines[i] = prefix + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}
