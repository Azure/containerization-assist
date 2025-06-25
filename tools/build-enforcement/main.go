package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

var (
	verbose = flag.Bool("verbose", false, "Verbose output")
	fix     = flag.Bool("fix", false, "Attempt to fix violations automatically")
)

type EnforcementCheck struct {
	Name        string
	Description string
	Command     string
	Critical    bool // If true, failure stops the build
}

func main() {
	flag.Parse()

	fmt.Println("MCP Build-Time Enforcement")
	fmt.Println("==========================")

	checks := []EnforcementCheck{
		{
			Name:        "Package Boundaries",
			Description: "Validate package boundary rules",
			Command:     "go run tools/check-boundaries/main.go",
			Critical:    true,
		},
		{
			Name:        "Interface Conformance",
			Description: "Check unified interface compliance",
			Command:     "go run tools/validate-interfaces/main.go",
			Critical:    true,
		},
		{
			Name:        "Dependency Hygiene",
			Description: "Check dependency cleanliness",
			Command:     "go run tools/check-hygiene/main.go",
			Critical:    false, // Some warnings are acceptable
		},
		{
			Name:        "Go Vet",
			Description: "Static analysis with go vet",
			Command:     "go vet ./...",
			Critical:    true,
		},
		{
			Name:        "Go Fmt",
			Description: "Code formatting check",
			Command:     "gofmt -l .",
			Critical:    true,
		},
		{
			Name:        "Module Tidiness",
			Description: "Check go.mod/go.sum tidiness",
			Command:     "go mod tidy && git diff --exit-code go.mod go.sum",
			Critical:    true,
		},
		{
			Name:        "Build Check",
			Description: "Ensure all packages build",
			Command:     "go build ./...",
			Critical:    true,
		},
		{
			Name:        "Test Check",
			Description: "Run all tests",
			Command:     "go test ./...",
			Critical:    true,
		},
	}

	passed := 0
	failed := 0
	skipped := 0

	for i, check := range checks {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(checks), check.Name)
		fmt.Printf("📋 %s\n", check.Description)

		if *verbose {
			fmt.Printf("🔧 Command: %s\n", check.Command)
		}

		success := runCheck(check)

		if success {
			fmt.Println("✅ PASSED")
			passed++
		} else if check.Critical {
			fmt.Println("❌ FAILED (Critical)")
			failed++

			if !*fix {
				fmt.Printf("\n❌ Build enforcement failed on critical check: %s\n", check.Name)
				fmt.Println("   Fix the issue above or use --fix flag for automatic fixes.")
				os.Exit(1)
			} else {
				fmt.Println("🔧 Attempting auto-fix...")
				if runFix(check) {
					fmt.Println("✅ Auto-fix successful")
					passed++
					failed--
				} else {
					fmt.Println("❌ Auto-fix failed")
					os.Exit(1)
				}
			}
		} else {
			fmt.Println("⚠️  FAILED (Non-critical)")
			skipped++
		}
	}

	fmt.Printf("\n📊 Build Enforcement Summary\n")
	fmt.Printf("============================\n")
	fmt.Printf("✅ Passed: %d\n", passed)
	fmt.Printf("❌ Failed: %d\n", failed)
	fmt.Printf("⚠️  Skipped: %d\n", skipped)

	if failed > 0 {
		fmt.Println("\n❌ Build enforcement failed!")
		os.Exit(1)
	} else if skipped > 0 {
		fmt.Println("\n⚠️  Build enforcement passed with warnings.")
	} else {
		fmt.Println("\n✅ All build enforcement checks passed!")
	}
}

func runCheck(check EnforcementCheck) bool {
	cmd := exec.Command("bash", "-c", check.Command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if *verbose {
			fmt.Printf("❌ Output: %s\n", string(output))
		}
		return false
	}

	// Special handling for gofmt - it succeeds but outputs files if formatting is needed
	if check.Name == "Go Fmt" && len(output) > 0 {
		if *verbose {
			fmt.Printf("❌ Files need formatting: %s\n", string(output))
		}
		return false
	}

	return true
}

func runFix(check EnforcementCheck) bool {
	var fixCommand string

	switch check.Name {
	case "Go Fmt":
		fixCommand = "gofmt -w ."
	case "Module Tidiness":
		fixCommand = "go mod tidy"
	case "Package Boundaries":
		fixCommand = "go run tools/check-boundaries/main.go --fix"
	case "Interface Conformance":
		fixCommand = "go run tools/validate-interfaces/main.go --fix"
	case "Dependency Hygiene":
		fixCommand = "go run tools/check-hygiene/main.go --fix"
	default:
		fmt.Printf("   No auto-fix available for %s\n", check.Name)
		return false
	}

	if *verbose {
		fmt.Printf("🔧 Fix command: %s\n", fixCommand)
	}

	cmd := exec.Command("bash", "-c", fixCommand)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if *verbose {
			fmt.Printf("❌ Fix output: %s\n", string(output))
		}
		return false
	}

	// Re-run the original check to verify the fix
	return runCheck(check)
}
