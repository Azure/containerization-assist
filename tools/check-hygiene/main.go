package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	verbose = flag.Bool("verbose", false, "Verbose output")
	fix     = flag.Bool("fix", false, "Attempt to fix issues automatically")
)

type DependencyIssue struct {
	Type        string
	Description string
	File        string
	Severity    string
	Command     string // Command to fix the issue
}

func main() {
	flag.Parse()
	
	fmt.Println("MCP Dependency Hygiene Check")
	fmt.Println("============================")
	
	var issues []DependencyIssue
	
	// 1. Check go.mod tidy status
	fmt.Println("🔍 Checking go mod tidy status...")
	tidyIssues := checkGoModTidy()
	issues = append(issues, tidyIssues...)
	
	// 2. Check for circular dependencies
	fmt.Println("🔍 Checking for circular dependencies...")
	circularIssues := checkCircularDependencies()
	issues = append(issues, circularIssues...)
	
	// 3. Check for unused dependencies
	fmt.Println("🔍 Checking for unused dependencies...")
	unusedIssues := checkUnusedDependencies()
	issues = append(issues, unusedIssues...)
	
	// 4. Check for version conflicts
	fmt.Println("🔍 Checking for version conflicts...")
	versionIssues := checkVersionConflicts()
	issues = append(issues, versionIssues...)
	
	// 5. Check for vulnerable dependencies
	fmt.Println("🔍 Checking for vulnerable dependencies...")
	vulnIssues := checkVulnerabilities()
	issues = append(issues, vulnIssues...)
	
	// 6. Check for duplicate dependencies
	fmt.Println("🔍 Checking for duplicate dependencies...")
	duplicateIssues := checkDuplicateDependencies()
	issues = append(issues, duplicateIssues...)
	
	// Report results
	fmt.Println("\n📊 Dependency Hygiene Results")
	fmt.Println("=============================")
	
	errors := 0
	warnings := 0
	
	for _, issue := range issues {
		switch issue.Severity {
		case "error":
			fmt.Printf("❌ ERROR: %s\n", issue.Description)
			errors++
		case "warning":
			fmt.Printf("⚠️  WARNING: %s\n", issue.Description)
			warnings++
		case "info":
			if *verbose {
				fmt.Printf("ℹ️  INFO: %s\n", issue.Description)
			}
		}
		
		if *verbose && issue.File != "" {
			fmt.Printf("   File: %s\n", issue.File)
		}
		
		if *fix && issue.Command != "" {
			fmt.Printf("   🔧 Fixing: %s\n", issue.Command)
			if err := runCommand(issue.Command); err != nil {
				fmt.Printf("   ❌ Fix failed: %v\n", err)
			} else {
				fmt.Printf("   ✅ Fixed\n")
			}
		}
		fmt.Println()
	}
	
	fmt.Printf("Summary: %d errors, %d warnings\n", errors, warnings)
	
	if errors > 0 {
		fmt.Println("\n❌ Dependency hygiene check failed!")
		fmt.Println("   Fix the errors above before proceeding.")
		
		if !*fix {
			fmt.Println("   Use --fix flag to attempt automatic fixes.")
		}
		
		os.Exit(1)
	} else if warnings > 0 {
		fmt.Println("\n⚠️  Dependency hygiene check passed with warnings.")
		fmt.Println("   Consider addressing the warnings above.")
	} else {
		fmt.Println("\n✅ Dependency hygiene check passed!")
	}
}

func checkGoModTidy() []DependencyIssue {
	var issues []DependencyIssue
	
	// Run go mod tidy and check if it makes changes
	cmd := exec.Command("go", "mod", "tidy")
	output, err := cmd.CombinedOutput()
	if err != nil {
		issues = append(issues, DependencyIssue{
			Type:        "mod_tidy",
			Description: fmt.Sprintf("go mod tidy failed: %v", err),
			Severity:    "error",
			Command:     "go mod tidy",
		})
		return issues
	}
	
	// Check if go.mod or go.sum changed
	if len(output) > 0 {
		issues = append(issues, DependencyIssue{
			Type:        "mod_tidy",
			Description: "go.mod/go.sum not tidy - dependencies need to be cleaned up",
			Severity:    "warning",
			Command:     "go mod tidy",
		})
	}
	
	// Verify the module
	cmd = exec.Command("go", "mod", "verify")
	output, err = cmd.CombinedOutput()
	if err != nil {
		issues = append(issues, DependencyIssue{
			Type:        "mod_verify",
			Description: fmt.Sprintf("go mod verify failed: %v\nOutput: %s", err, string(output)),
			Severity:    "error",
			Command:     "go mod download",
		})
	}
	
	return issues
}

func checkCircularDependencies() []DependencyIssue {
	var issues []DependencyIssue
	
	// Use go mod graph to check for cycles
	cmd := exec.Command("go", "mod", "graph")
	output, err := cmd.Output()
	if err != nil {
		issues = append(issues, DependencyIssue{
			Type:        "mod_graph",
			Description: fmt.Sprintf("Failed to get dependency graph: %v", err),
			Severity:    "error",
		})
		return issues
	}
	
	// Parse the dependency graph
	dependencies := make(map[string][]string)
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			from := parts[0]
			to := parts[1]
			dependencies[from] = append(dependencies[from], to)
		}
	}
	
	// Check for cycles (simplified check)
	cycles := findCycles(dependencies)
	for _, cycle := range cycles {
		issues = append(issues, DependencyIssue{
			Type:        "circular_dependency",
			Description: fmt.Sprintf("Circular dependency detected: %s", strings.Join(cycle, " -> ")),
			Severity:    "error",
		})
	}
	
	return issues
}

func checkUnusedDependencies() []DependencyIssue {
	var issues []DependencyIssue
	
	// Check if go mod why works for our dependencies
	cmd := exec.Command("go", "list", "-m", "all")
	output, err := cmd.Output()
	if err != nil {
		issues = append(issues, DependencyIssue{
			Type:        "list_modules",
			Description: fmt.Sprintf("Failed to list modules: %v", err),
			Severity:    "warning",
		})
		return issues
	}
	
	// Parse module list
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "github.com/tng/workspace/prod") {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			module := parts[0]
			
			// Check if this module is actually used
			whyCmd := exec.Command("go", "mod", "why", module)
			whyOutput, err := whyCmd.Output()
			if err != nil {
				continue
			}
			
			// If "go mod why" returns no explanation, the module might be unused
			if strings.Contains(string(whyOutput), "(main module does not need") {
				issues = append(issues, DependencyIssue{
					Type:        "unused_dependency",
					Description: fmt.Sprintf("Potentially unused dependency: %s", module),
					Severity:    "info",
				})
			}
		}
	}
	
	return issues
}

func checkVersionConflicts() []DependencyIssue {
	var issues []DependencyIssue
	
	// Get dependency list with versions
	cmd := exec.Command("go", "list", "-m", "-versions", "all")
	output, err := cmd.Output()
	if err != nil {
		issues = append(issues, DependencyIssue{
			Type:        "version_check",
			Description: fmt.Sprintf("Failed to check versions: %v", err),
			Severity:    "warning",
		})
		return issues
	}
	
	// Parse for potential version conflicts
	lines := strings.Split(string(output), "\n")
	moduleVersions := make(map[string][]string)
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			module := parts[0]
			version := parts[1]
			
			// Skip our own module
			if strings.HasPrefix(module, "github.com/tng/workspace/prod") {
				continue
			}
			
			moduleVersions[module] = append(moduleVersions[module], version)
		}
	}
	
	// Check for modules with multiple versions (potential conflict)
	for module, versions := range moduleVersions {
		if len(versions) > 1 {
			issues = append(issues, DependencyIssue{
				Type:        "version_conflict",
				Description: fmt.Sprintf("Multiple versions of %s: %v", module, versions),
				Severity:    "warning",
			})
		}
	}
	
	return issues
}

func checkVulnerabilities() []DependencyIssue {
	var issues []DependencyIssue
	
	// Check if govulncheck is available
	cmd := exec.Command("govulncheck", "--version")
	if err := cmd.Run(); err != nil {
		issues = append(issues, DependencyIssue{
			Type:        "vuln_tool",
			Description: "govulncheck not available - install with: go install golang.org/x/vuln/cmd/govulncheck@latest",
			Severity:    "info",
			Command:     "go install golang.org/x/vuln/cmd/govulncheck@latest",
		})
		return issues
	}
	
	// Run vulnerability check
	cmd = exec.Command("govulncheck", "./...")
	output, err := cmd.CombinedOutput()
	
	// govulncheck returns non-zero exit code when vulnerabilities are found
	if err != nil {
		// Parse the output to understand the vulnerabilities
		outputStr := string(output)
		if strings.Contains(outputStr, "Vulnerability") {
			issues = append(issues, DependencyIssue{
				Type:        "vulnerability",
				Description: fmt.Sprintf("Vulnerabilities found in dependencies:\n%s", outputStr),
				Severity:    "error",
			})
		} else {
			issues = append(issues, DependencyIssue{
				Type:        "vuln_check",
				Description: fmt.Sprintf("Vulnerability check failed: %v\nOutput: %s", err, outputStr),
				Severity:    "warning",
			})
		}
	}
	
	return issues
}

func checkDuplicateDependencies() []DependencyIssue {
	var issues []DependencyIssue
	
	// Read go.mod file
	goModContent, err := os.ReadFile("go.mod")
	if err != nil {
		issues = append(issues, DependencyIssue{
			Type:        "read_gomod",
			Description: fmt.Sprintf("Failed to read go.mod: %v", err),
			Severity:    "error",
		})
		return issues
	}
	
	// Parse go.mod for duplicate entries
	scanner := bufio.NewScanner(strings.NewReader(string(goModContent)))
	seenDeps := make(map[string]int)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Look for require statements
		if strings.HasPrefix(line, "require") && !strings.HasPrefix(line, "require (") {
			// Single require statement
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				module := parts[1]
				seenDeps[module]++
			}
		} else if strings.Contains(line, " v") && !strings.HasPrefix(line, "//") {
			// Inside require block
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				module := parts[0]
				seenDeps[module]++
			}
		}
	}
	
	// Check for duplicates
	for module, count := range seenDeps {
		if count > 1 {
			issues = append(issues, DependencyIssue{
				Type:        "duplicate_dependency",
				Description: fmt.Sprintf("Duplicate dependency in go.mod: %s (appears %d times)", module, count),
				Severity:    "error",
				File:        "go.mod",
				Command:     "go mod tidy",
			})
		}
	}
	
	return issues
}

func findCycles(dependencies map[string][]string) [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	for module := range dependencies {
		if !visited[module] {
			if cycle := dfs(module, dependencies, visited, recStack, []string{}); len(cycle) > 0 {
				cycles = append(cycles, cycle)
			}
		}
	}
	
	return cycles
}

func dfs(module string, dependencies map[string][]string, visited, recStack map[string]bool, path []string) []string {
	visited[module] = true
	recStack[module] = true
	path = append(path, module)
	
	for _, dep := range dependencies[module] {
		if !visited[dep] {
			if cycle := dfs(dep, dependencies, visited, recStack, path); len(cycle) > 0 {
				return cycle
			}
		} else if recStack[dep] {
			// Found a cycle
			cycleStart := -1
			for i, p := range path {
				if p == dep {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				return append(path[cycleStart:], dep)
			}
		}
	}
	
	recStack[module] = false
	return nil
}

func runCommand(command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}
	
	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %v\nOutput: %s", err, string(output))
	}
	
	return nil
}