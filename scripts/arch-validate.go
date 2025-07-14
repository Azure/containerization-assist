// Package main provides a command-line architecture validation tool for Container Kit MCP
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ArchRule represents an architecture validation rule
type ArchRule struct {
	Name         string
	Description  string
	Layer        string
	ForbiddenRE  *regexp.Regexp
	CheckImports bool
	CheckCode    bool
}

// ValidationResult represents the result of architecture validation
type ValidationResult struct {
	Passed     bool
	Layer      string
	Package    string
	File       string
	Line       int
	Rule       string
	Violation  string
	Suggestion string
}

func main() {
	fmt.Println("🏗️  Container Kit MCP Architecture Validator")
	fmt.Println("============================================")

	rules := getArchitectureRules()
	results := []ValidationResult{}

	// Validate each layer
	for _, rule := range rules {
		fmt.Printf("\n📋 Validating %s...\n", rule.Name)
		layerResults := validateLayer(rule)
		results = append(results, layerResults...)
	}

	// Print summary
	printSummary(results)

	// Exit with appropriate code
	hasFailures := false
	for _, result := range results {
		if !result.Passed {
			hasFailures = true
			break
		}
	}

	if hasFailures {
		os.Exit(1)
	}
	fmt.Println("\n✅ All architecture validation checks passed!")
}

func getArchitectureRules() []ArchRule {
	return []ArchRule{
		{
			Name:         "Domain Layer Isolation",
			Description:  "Domain layer must not import infrastructure or application packages",
			Layer:        "pkg/mcp/domain",
			ForbiddenRE:  regexp.MustCompile(`"[^"]*/(infrastructure|application)/`),
			CheckImports: true,
		},
		{
			Name:         "Application Layer Boundary",
			Description:  "Application layer must not directly import infrastructure packages",
			Layer:        "pkg/mcp/application",
			ForbiddenRE:  regexp.MustCompile(`"[^"]*/infrastructure/`),
			CheckImports: true,
		},
		{
			Name:         "API Layer Isolation",
			Description:  "API layer should only import domain interfaces",
			Layer:        "pkg/mcp/api",
			ForbiddenRE:  regexp.MustCompile(`"[^"]*/(infrastructure|application)/`),
			CheckImports: true,
		},
		{
			Name:         "Infrastructure Layer Direction",
			Description:  "Infrastructure layer must not import from application or api layers",
			Layer:        "pkg/mcp/infrastructure",
			ForbiddenRE:  regexp.MustCompile(`"[^"]*/(application|api)/`),
			CheckImports: true,
		},
		{
			Name:        "Domain Purity",
			Description: "Domain layer must not use external services directly",
			Layer:       "pkg/mcp/domain",
			ForbiddenRE: regexp.MustCompile(`(os\.WriteFile|os\.ReadFile|exec\.Command|http\.Get|sql\.Open)`),
			CheckCode:   true,
		},
		{
			Name:         "Wire Isolation",
			Description:  "Wire dependency injection should only be in wiring package",
			Layer:        "pkg/mcp",
			ForbiddenRE:  regexp.MustCompile(`"github\.com/google/wire"`),
			CheckImports: true,
		},
	}
}

func validateLayer(rule ArchRule) []ValidationResult {
	results := []ValidationResult{}
	baseDir := "."

	// Find the layer directory
	layerDir := filepath.Join(baseDir, rule.Layer)

	err := filepath.Walk(layerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible files
		}

		// Only check .go files, skip test files for most rules
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip wire.go files for wire isolation rule
		if rule.Name == "Wire Isolation" && strings.HasSuffix(path, "wire.go") {
			return nil
		}

		// Skip wiring directory for wire isolation rule
		if rule.Name == "Wire Isolation" && strings.Contains(path, "/wiring/") {
			return nil
		}

		// Read and validate file
		fileResults := validateFile(path, rule)
		results = append(results, fileResults...)

		return nil
	})

	if err != nil {
		fmt.Printf("⚠️  Warning: Could not walk directory %s: %v\n", layerDir, err)
	}

	return results
}

func validateFile(filePath string, rule ArchRule) []ValidationResult {
	results := []ValidationResult{}

	file, err := os.Open(filePath)
	if err != nil {
		return results
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	inImportBlock := false

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Track import blocks
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		// Check imports
		if rule.CheckImports {
			isImportLine := strings.HasPrefix(line, "import ") || inImportBlock
			if isImportLine && rule.ForbiddenRE.MatchString(line) {
				results = append(results, ValidationResult{
					Passed:     false,
					Layer:      rule.Layer,
					Package:    getPackageFromPath(filePath),
					File:       filePath,
					Line:       lineNum,
					Rule:       rule.Name,
					Violation:  line,
					Suggestion: getSuggestionForRule(rule.Name),
				})
			}
		}

		// Check code patterns
		if rule.CheckCode && rule.ForbiddenRE.MatchString(line) {
			results = append(results, ValidationResult{
				Passed:     false,
				Layer:      rule.Layer,
				Package:    getPackageFromPath(filePath),
				File:       filePath,
				Line:       lineNum,
				Rule:       rule.Name,
				Violation:  line,
				Suggestion: getSuggestionForRule(rule.Name),
			})
		}
	}

	return results
}

func getPackageFromPath(filePath string) string {
	dir := filepath.Dir(filePath)
	return strings.ReplaceAll(dir, string(os.PathSeparator), "/")
}

func getSuggestionForRule(ruleName string) string {
	suggestions := map[string]string{
		"Domain Layer Isolation":         "Move infrastructure dependencies to domain interfaces",
		"Application Layer Boundary":     "Use dependency injection to access infrastructure services",
		"API Layer Isolation":            "Only import domain interfaces in API layer",
		"Infrastructure Layer Direction": "Infrastructure should implement domain interfaces, not import from higher layers",
		"Domain Purity":                  "Use domain interfaces for external operations, implement in infrastructure",
		"Wire Isolation":                 "Move dependency injection to pkg/mcp/api/wiring",
	}

	if suggestion, exists := suggestions[ruleName]; exists {
		return suggestion
	}
	return "Review architecture documentation for guidance"
}

func printSummary(results []ValidationResult) {
	fmt.Println("\n📊 Architecture Validation Summary")
	fmt.Println("==================================")

	violations := []ValidationResult{}
	for _, result := range results {
		if !result.Passed {
			violations = append(violations, result)
		}
	}

	if len(violations) == 0 {
		fmt.Println("✅ No architecture violations found!")
		return
	}

	fmt.Printf("❌ Found %d architecture violations:\n\n", len(violations))

	// Group by rule
	violationsByRule := make(map[string][]ValidationResult)
	for _, violation := range violations {
		violationsByRule[violation.Rule] = append(violationsByRule[violation.Rule], violation)
	}

	for rule, ruleViolations := range violationsByRule {
		fmt.Printf("🚫 %s (%d violations):\n", rule, len(ruleViolations))
		for _, violation := range ruleViolations {
			fmt.Printf("   📁 %s:%d\n", violation.File, violation.Line)
			fmt.Printf("      Violation: %s\n", violation.Violation)
			fmt.Printf("      💡 %s\n\n", violation.Suggestion)
		}
	}

	fmt.Println("🔧 Fix these violations to ensure clean architecture compliance.")
}
