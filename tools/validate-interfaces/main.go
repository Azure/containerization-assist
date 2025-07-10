package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	verbose       = flag.Bool("verbose", false, "Verbose output")
	metrics       = flag.Bool("metrics", false, "Generate interface adoption metrics report")
	metricsOutput = flag.String("metrics-output", "interface_metrics.json", "Output file for metrics report")
	errorBudget   = flag.Int("error-budget", 0, "Allow up to N interface validation errors before failing")
	warningBudget = flag.Int("warning-budget", -1, "Allow up to N interface validation warnings before failing (-1 = unlimited)")
)

func main() {
	flag.Parse()

	fmt.Println("MCP Interface Validation Tool")
	fmt.Println("=============================")

	var results []ValidationResult

	// 1. Check for unified interfaces in the main package
	fmt.Println("🔍 Checking for unified interfaces...")
	unifiedResults := validateUnifiedInterfaces()
	results = append(results, unifiedResults...)

	// 2. Check for legacy interface files
	fmt.Println("🔍 Checking for legacy interface files...")
	legacyResults := validateLegacyInterfaces()
	results = append(results, legacyResults...)

	// 3. Check interface conformance across all tools
	fmt.Println("🔍 Checking interface conformance...")
	conformanceResults := validateInterfaceConformance()
	results = append(results, conformanceResults...)

	// 4. Check for duplicate interface definitions
	fmt.Println("🔍 Checking for duplicate interface definitions...")
	duplicateResults := validateDuplicateInterfaces()
	results = append(results, duplicateResults...)

	// Generate metrics if requested
	if *metrics {
		fmt.Println("\n📈 Generating interface adoption metrics...")
		metricsReport := generateInterfaceMetrics()
		if err := saveMetricsReport(metricsReport, *metricsOutput); err != nil {
			fmt.Printf("⚠️  Failed to save metrics report: %v\n", err)
		} else {
			fmt.Printf("   Metrics saved to: %s\n", *metricsOutput)
			printMetricsSummary(metricsReport)
		}
	}

	// Report results
	fmt.Println("\n📊 Validation Results")
	fmt.Println("=====================")

	errors := 0
	warnings := 0

	for _, result := range results {
		switch result.Severity {
		case "error":
			fmt.Printf("❌ ERROR: %s\n", result.Issue)
			errors++
		case "warning":
			fmt.Printf("⚠️  WARNING: %s\n", result.Issue)
			warnings++
		}

		if *verbose {
			fmt.Printf("   File: %s\n", result.File)
			if result.Interface != "" {
				fmt.Printf("   Interface: %s\n", result.Interface)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Summary: %d errors, %d warnings\n", errors, warnings)

	// Check error budget
	if errors > *errorBudget {
		fmt.Printf("\n❌ Interface validation failed! (%d errors > %d allowed)\n", errors, *errorBudget)
		fmt.Println("   Fix the errors above or increase the error budget.")
		os.Exit(1)
	} else if *warningBudget >= 0 && warnings > *warningBudget {
		fmt.Printf("\n❌ Interface validation failed! (%d warnings > %d allowed)\n", warnings, *warningBudget)
		fmt.Println("   Fix the warnings above or increase the warning budget.")
		os.Exit(1)
	} else if errors > 0 {
		fmt.Printf("\n⚠️  Interface validation passed with %d errors (within budget of %d).\n", errors, *errorBudget)
		fmt.Println("   Consider fixing the errors above.")
	} else if warnings > 0 {
		fmt.Println("\n⚠️  Interface validation passed with warnings.")
		fmt.Println("   Consider addressing the warnings above.")
	} else {
		fmt.Println("\n✅ Interface validation passed!")
	}
}
