package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// generateInterfaceMetrics analyzes the codebase and generates comprehensive interface metrics
func generateInterfaceMetrics() *InterfaceMetrics {
	metrics := &InterfaceMetrics{
		Timestamp:        time.Now(),
		InterfaceStats:   make(map[string]*InterfaceUsageStats),
		ImplementorStats: make(map[string]*ImplementorStats),
		PatternAnalysis:  &InterfacePatternAnalysis{},
		ComplianceReport: &ComplianceReport{
			InterfaceCompliance: make(map[string]float64),
		},
	}

	// Scan all Go files in the project
	interfaceMap := make(map[string]*ast.InterfaceType)
	implementorMap := make(map[string][]string) // implementor -> interfaces
	packageMap := make(map[string]string)       // type -> package

	err := filepath.WalkDir("pkg/mcp", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return nil // Skip files that can't be parsed
		}

		packageName := file.Name.Name

		// Find interfaces and implementors in this file
		analyzeFileForMetrics(file, path, packageName, interfaceMap, implementorMap, packageMap)

		return nil
	})

	if err != nil {
		fmt.Printf("Warning: Error scanning files for metrics: %v\n", err)
	}

	// Build interface usage stats
	for interfaceName := range expectedInterfaces {
		stats := &InterfaceUsageStats{
			InterfaceName:   interfaceName,
			Methods:         expectedInterfaces[interfaceName],
			Implementors:    []string{},
			PackageDistrib:  make(map[string]int),
			MostUsedMethods: []MethodUsage{},
		}

		// Count implementors for this interface
		for implementor, interfaces := range implementorMap {
			for _, impl := range interfaces {
				if impl == interfaceName {
					stats.Implementors = append(stats.Implementors, implementor)
					stats.ImplementorCount++

					if pkg, exists := packageMap[implementor]; exists {
						stats.PackageDistrib[pkg]++
					}
				}
			}
		}

		metrics.InterfaceStats[interfaceName] = stats
	}

	// Build implementor stats
	for implementor, interfaces := range implementorMap {
		pkg := packageMap[implementor]
		compliance := calculateCompliance(interfaces, expectedInterfaces)
		patternType := determinePatternType(interfaces, pkg)

		stats := &ImplementorStats{
			TypeName:            implementor,
			Package:             pkg,
			InterfacesImpl:      interfaces,
			MethodCount:         countMethodsForImplementor(implementor, interfaceMap),
			InterfaceCompliance: compliance,
			PatternType:         patternType,
		}

		metrics.ImplementorStats[implementor] = stats
	}

	// Calculate overall metrics
	metrics.TotalInterfaces = len(interfaceMap)
	metrics.TotalImplementors = len(implementorMap)
	if metrics.TotalImplementors > 0 {
		metrics.AdoptionRate = float64(len(implementorMap)) / float64(metrics.TotalImplementors) * 100
	}

	// Generate pattern analysis
	metrics.PatternAnalysis = analyzeInterfacePatterns(metrics.ImplementorStats)

	// Generate compliance report
	metrics.ComplianceReport = generateComplianceReport(metrics.InterfaceStats, metrics.ImplementorStats)

	// Generate recommendations
	metrics.RecommendationList = generateRecommendations(metrics)

	return metrics
}

// analyzeFileForMetrics extracts interface and implementor information from a single file
func analyzeFileForMetrics(file *ast.File, _, packageName string,
	interfaceMap map[string]*ast.InterfaceType,
	implementorMap map[string][]string,
	packageMap map[string]string) {

	// Find interface definitions
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			typeName := typeSpec.Name.Name
			packageMap[typeName] = packageName

			// Check if it's an interface
			if interfaceType, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
				interfaceMap[typeName] = interfaceType
			}

			// Check if it's a struct that might implement interfaces
			if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
				interfaces := findImplementedInterfaces(file, typeName)
				if len(interfaces) > 0 {
					implementorMap[typeName] = interfaces
				}
			}
		}
	}
}

// findImplementedInterfaces determines which interfaces a struct implements
func findImplementedInterfaces(file *ast.File, structName string) []string {
	var interfaces []string

	// Look for methods that match expected interface methods
	for interfaceName, expectedMethods := range expectedInterfaces {
		if implementsInterface(file, structName, expectedMethods) {
			interfaces = append(interfaces, interfaceName)
		}
	}

	return interfaces
}

// implementsInterface checks if a struct implements all methods of an interface
func implementsInterface(file *ast.File, structName string, expectedMethods []string) bool {
	foundMethods := make(map[string]bool)

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		// Check if this method belongs to our struct
		recvType := getReceiverType(funcDecl.Recv)
		if recvType == structName || recvType == "*"+structName {
			foundMethods[funcDecl.Name.Name] = true
		}
	}

	// Check if all expected methods are present
	for _, expectedMethod := range expectedMethods {
		methodName := strings.Split(expectedMethod, "(")[0]
		if !foundMethods[methodName] {
			return false
		}
	}

	return len(expectedMethods) > 0 // Only return true if there are methods to check
}

// calculateCompliance calculates interface compliance percentage
func calculateCompliance(implementedInterfaces []string, expectedInterfaces map[string][]string) float64 {
	if len(expectedInterfaces) == 0 {
		return 100.0
	}

	implemented := len(implementedInterfaces)
	expected := len(expectedInterfaces)

	return float64(implemented) / float64(expected) * 100.0
}

// determinePatternType determines the pattern type based on implemented interfaces and package
func determinePatternType(interfaces []string, _ string) string {
	hasUnified := false
	hasLegacy := false

	for _, iface := range interfaces {
		if _, exists := expectedInterfaces[iface]; exists {
			hasUnified = true
		} else {
			hasLegacy = true
		}
	}

	if hasUnified && hasLegacy {
		return "mixed"
	} else if hasUnified {
		return "unified"
	} else if hasLegacy {
		return "legacy"
	}

	return "unknown"
}

// countMethodsForImplementor counts methods for a specific implementor
func countMethodsForImplementor(_ string, _ map[string]*ast.InterfaceType) int {
	// This is a simplified implementation
	// In practice, you'd want to count actual methods on the implementor
	return 0
}

// analyzeInterfacePatterns analyzes interface usage patterns
func analyzeInterfacePatterns(implementorStats map[string]*ImplementorStats) *InterfacePatternAnalysis {
	analysis := &InterfacePatternAnalysis{
		TopPatterns:  []PatternUsage{},
		AntiPatterns: []AntiPattern{},
	}

	unifiedCount := 0
	legacyCount := 0
	mixedCount := 0

	patternCounts := make(map[string]int)

	for _, stats := range implementorStats {
		switch stats.PatternType {
		case "unified":
			unifiedCount++
		case "legacy":
			legacyCount++
		case "mixed":
			mixedCount++
		}

		patternCounts[stats.PatternType]++
	}

	analysis.UnifiedPatternUsage = unifiedCount
	analysis.LegacyPatternUsage = legacyCount
	analysis.MixedPatternUsage = mixedCount

	totalPatterns := unifiedCount + legacyCount + mixedCount
	if totalPatterns > 0 {
		analysis.PatternMigrationRate = float64(unifiedCount) / float64(totalPatterns) * 100.0
	}

	// Convert pattern counts to sorted list
	type patternCount struct {
		name  string
		count int
	}

	var patterns []patternCount
	for pattern, count := range patternCounts {
		patterns = append(patterns, patternCount{name: pattern, count: count})
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].count > patterns[j].count
	})

	for _, pattern := range patterns {
		analysis.TopPatterns = append(analysis.TopPatterns, PatternUsage{
			PatternName: pattern.name,
			Count:       pattern.count,
			Examples:    []string{}, // Would be populated with actual examples
		})
	}

	// Identify anti-patterns
	if mixedCount > 0 {
		analysis.AntiPatterns = append(analysis.AntiPatterns, AntiPattern{
			Pattern:     "mixed_interface_patterns",
			Description: "Types implementing both unified and legacy interfaces",
			Examples:    []string{}, // Would be populated with actual examples
			Severity:    "warning",
		})
	}

	return analysis
}

// generateComplianceReport generates a compliance report
func generateComplianceReport(interfaceStats map[string]*InterfaceUsageStats, implementorStats map[string]*ImplementorStats) *ComplianceReport {
	report := &ComplianceReport{
		InterfaceCompliance:  make(map[string]float64),
		MissingInterfaces:    []string{},
		OrphanedImplementors: []string{},
		NonCompliantTools:    []string{},
	}

	totalCompliance := 0.0
	compliantCount := 0

	// Calculate interface-specific compliance
	for interfaceName, stats := range interfaceStats {
		if stats.ImplementorCount > 0 {
			compliance := float64(stats.ImplementorCount) / float64(len(implementorStats)) * 100.0
			report.InterfaceCompliance[interfaceName] = compliance
			totalCompliance += compliance
			compliantCount++
		} else {
			report.MissingInterfaces = append(report.MissingInterfaces, interfaceName)
		}
	}

	if compliantCount > 0 {
		report.OverallCompliance = totalCompliance / float64(compliantCount)
	}

	// Find orphaned implementors (no interfaces implemented)
	for implementorName, stats := range implementorStats {
		if len(stats.InterfacesImpl) == 0 {
			report.OrphanedImplementors = append(report.OrphanedImplementors, implementorName)
		}

		if stats.InterfaceCompliance < 50.0 {
			report.NonCompliantTools = append(report.NonCompliantTools, implementorName)
		}
	}

	return report
}

// generateRecommendations generates actionable recommendations
func generateRecommendations(metrics *InterfaceMetrics) []string {
	var recommendations []string

	if metrics.ComplianceReport.OverallCompliance < 70.0 {
		recommendations = append(recommendations,
			"Overall interface compliance is below 70%. Focus on implementing missing interfaces.")
	}

	if metrics.PatternAnalysis.LegacyPatternUsage > metrics.PatternAnalysis.UnifiedPatternUsage {
		recommendations = append(recommendations,
			"Legacy patterns outnumber unified patterns. Prioritize migration to unified interfaces.")
	}

	if metrics.PatternAnalysis.MixedPatternUsage > 0 {
		recommendations = append(recommendations,
			"Some types use mixed interface patterns. Standardize on unified interfaces.")
	}

	if len(metrics.ComplianceReport.OrphanedImplementors) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Found %d orphaned implementors with no interface implementations. Consider adding interface compliance.",
				len(metrics.ComplianceReport.OrphanedImplementors)))
	}

	if len(metrics.ComplianceReport.MissingInterfaces) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Found %d interfaces with no implementations. Consider creating implementations or removing unused interfaces.",
				len(metrics.ComplianceReport.MissingInterfaces)))
	}

	return recommendations
}

// saveMetricsReport saves the metrics report to a JSON file
func saveMetricsReport(metrics *InterfaceMetrics, outputPath string) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	return os.WriteFile(outputPath, data, 0644)
}

// printMetricsSummary prints a summary of the metrics to the console
func printMetricsSummary(metrics *InterfaceMetrics) {
	fmt.Println("\n📊 Interface Adoption Metrics Summary")
	fmt.Println("====================================")

	fmt.Printf("Total Interfaces: %d\n", metrics.TotalInterfaces)
	fmt.Printf("Total Implementors: %d\n", metrics.TotalImplementors)
	fmt.Printf("Overall Adoption Rate: %.1f%%\n", metrics.AdoptionRate)
	fmt.Printf("Overall Compliance: %.1f%%\n", metrics.ComplianceReport.OverallCompliance)

	fmt.Println("\n🎯 Pattern Analysis:")
	fmt.Printf("  Unified Pattern Usage: %d\n", metrics.PatternAnalysis.UnifiedPatternUsage)
	fmt.Printf("  Legacy Pattern Usage: %d\n", metrics.PatternAnalysis.LegacyPatternUsage)
	fmt.Printf("  Mixed Pattern Usage: %d\n", metrics.PatternAnalysis.MixedPatternUsage)
	fmt.Printf("  Migration Rate: %.1f%%\n", metrics.PatternAnalysis.PatternMigrationRate)

	fmt.Println("\n📈 Top Interfaces by Implementation Count:")
	type interfaceCount struct {
		name  string
		count int
	}

	var interfaces []interfaceCount
	for name, stats := range metrics.InterfaceStats {
		interfaces = append(interfaces, interfaceCount{name: name, count: stats.ImplementorCount})
	}

	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].count > interfaces[j].count
	})

	for i, iface := range interfaces {
		if i < 5 { // Show top 5
			fmt.Printf("  %s: %d implementations\n", iface.name, iface.count)
		}
	}

	if len(metrics.RecommendationList) > 0 {
		fmt.Println("\n💡 Recommendations:")
		for _, rec := range metrics.RecommendationList {
			fmt.Printf("  • %s\n", rec)
		}
	}

	if len(metrics.PatternAnalysis.AntiPatterns) > 0 {
		fmt.Println("\n⚠️  Anti-patterns Detected:")
		for _, ap := range metrics.PatternAnalysis.AntiPatterns {
			fmt.Printf("  • %s: %s\n", ap.Pattern, ap.Description)
		}
	}
}
