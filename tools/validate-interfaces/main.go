package main

import (
	"encoding/json"
	"flag"
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

var (
	verbose       = flag.Bool("verbose", false, "Verbose output")
	fix           = flag.Bool("fix", false, "Attempt to fix violations automatically")
	metrics       = flag.Bool("metrics", false, "Generate interface adoption metrics report")
	metricsOutput = flag.String("metrics-output", "interface_metrics.json", "Output file for metrics report")
	errorBudget   = flag.Int("error-budget", 0, "Allow up to N interface validation errors before failing")
	warningBudget = flag.Int("warning-budget", -1, "Allow up to N interface validation warnings before failing (-1 = unlimited)")
)

// Expected unified interfaces that should exist after Team A's work
var expectedInterfaces = map[string][]string{
	"Tool": {
		"Execute(ctx context.Context, args interface{}) (interface{}, error)",
		"GetMetadata() ToolMetadata",
		"Validate(ctx context.Context, args interface{}) error",
	},
	"Session": {
		"ID() string",
		"GetWorkspace() string",
		"UpdateState(func(*SessionState))",
	},
	"Transport": {
		"Serve(ctx context.Context) error",
		"Stop() error",
	},
	"Orchestrator": {
		"ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)",
		"RegisterTool(name string, tool Tool) error",
	},
}

// Legacy interfaces that should be removed
var legacyInterfaces = []string{
	"pkg/mcp/internal/interfaces/",
	"pkg/mcp/internal/adapter/interfaces.go",
	"pkg/mcp/internal/tools/interfaces.go",
	"pkg/mcp/internal/tools/base/atomic_tool.go",
	"pkg/mcp/internal/dispatch/interfaces.go",
	"pkg/mcp/internal/analyzer/interfaces.go",
	"pkg/mcp/internal/ai_context/interfaces.go",
	"pkg/mcp/internal/fixing/interfaces.go",
	"pkg/mcp/internal/manifests/interfaces.go",
}

type ValidationResult struct {
	File      string
	Interface string
	Issue     string
	Severity  string
}

// InterfaceMetrics tracks interface usage and adoption patterns
type InterfaceMetrics struct {
	Timestamp          time.Time                       `json:"timestamp"`
	TotalInterfaces    int                             `json:"total_interfaces"`
	TotalImplementors  int                             `json:"total_implementors"`
	AdoptionRate       float64                         `json:"adoption_rate"`
	InterfaceStats     map[string]*InterfaceUsageStats `json:"interface_stats"`
	ImplementorStats   map[string]*ImplementorStats    `json:"implementor_stats"`
	PatternAnalysis    *InterfacePatternAnalysis       `json:"pattern_analysis"`
	ComplianceReport   *ComplianceReport               `json:"compliance_report"`
	RecommendationList []string                        `json:"recommendations"`
}

// InterfaceUsageStats tracks usage statistics for a specific interface
type InterfaceUsageStats struct {
	InterfaceName    string          `json:"interface_name"`
	ImplementorCount int             `json:"implementor_count"`
	Implementors     []string        `json:"implementors"`
	Methods          []string        `json:"methods"`
	PackageDistrib   map[string]int  `json:"package_distribution"`
	AdoptionTrend    []AdoptionPoint `json:"adoption_trend"`
	MostUsedMethods  []MethodUsage   `json:"most_used_methods"`
}

// ImplementorStats tracks statistics for types that implement interfaces
type ImplementorStats struct {
	TypeName            string   `json:"type_name"`
	Package             string   `json:"package"`
	InterfacesImpl      []string `json:"interfaces_implemented"`
	MethodCount         int      `json:"method_count"`
	InterfaceCompliance float64  `json:"interface_compliance"`
	PatternType         string   `json:"pattern_type"` // "unified", "legacy", "mixed"
}

// InterfacePatternAnalysis provides insights into interface usage patterns
type InterfacePatternAnalysis struct {
	UnifiedPatternUsage  int            `json:"unified_pattern_usage"`
	LegacyPatternUsage   int            `json:"legacy_pattern_usage"`
	MixedPatternUsage    int            `json:"mixed_pattern_usage"`
	PatternMigrationRate float64        `json:"pattern_migration_rate"`
	TopPatterns          []PatternUsage `json:"top_patterns"`
	AntiPatterns         []AntiPattern  `json:"anti_patterns"`
}

// ComplianceReport tracks compliance with interface standards
type ComplianceReport struct {
	OverallCompliance    float64            `json:"overall_compliance"`
	InterfaceCompliance  map[string]float64 `json:"interface_compliance"`
	MissingInterfaces    []string           `json:"missing_interfaces"`
	OrphanedImplementors []string           `json:"orphaned_implementors"`
	NonCompliantTools    []string           `json:"non_compliant_tools"`
}

// AdoptionPoint tracks adoption over time
type AdoptionPoint struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}

// MethodUsage tracks usage of specific interface methods
type MethodUsage struct {
	MethodName string `json:"method_name"`
	UsageCount int    `json:"usage_count"`
}

// PatternUsage tracks common patterns in interface usage
type PatternUsage struct {
	PatternName string   `json:"pattern_name"`
	Count       int      `json:"count"`
	Examples    []string `json:"examples"`
}

// AntiPattern identifies problematic interface usage patterns
type AntiPattern struct {
	Pattern     string   `json:"pattern"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Severity    string   `json:"severity"`
}

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

func validateUnifiedInterfaces() []ValidationResult {
	var results []ValidationResult

	// Check if pkg/mcp/interfaces.go exists
	interfacesFile := "pkg/mcp/interfaces.go"
	if _, err := os.Stat(interfacesFile); os.IsNotExist(err) {
		results = append(results, ValidationResult{
			File:     interfacesFile,
			Issue:    "Unified interfaces file does not exist - Team A work not complete",
			Severity: "error",
		})
		return results
	}

	// Parse the interfaces file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, interfacesFile, nil, parser.ParseComments)
	if err != nil {
		results = append(results, ValidationResult{
			File:     interfacesFile,
			Issue:    fmt.Sprintf("Failed to parse interfaces file: %v", err),
			Severity: "error",
		})
		return results
	}

	// Check for expected interfaces
	foundInterfaces := make(map[string]bool)

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

			interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			interfaceName := typeSpec.Name.Name
			foundInterfaces[interfaceName] = true

			// Validate interface methods
			if expectedMethods, exists := expectedInterfaces[interfaceName]; exists {
				actualMethods := getInterfaceMethods(interfaceType)
				if err := validateMethods(interfaceName, expectedMethods, actualMethods); err != nil {
					results = append(results, ValidationResult{
						File:      interfacesFile,
						Interface: interfaceName,
						Issue:     err.Error(),
						Severity:  "error",
					})
				}
			}
		}
	}

	// Check for missing interfaces
	for interfaceName := range expectedInterfaces {
		if !foundInterfaces[interfaceName] {
			results = append(results, ValidationResult{
				File:      interfacesFile,
				Interface: interfaceName,
				Issue:     fmt.Sprintf("Missing expected interface: %s", interfaceName),
				Severity:  "error",
			})
		}
	}

	return results
}

func validateLegacyInterfaces() []ValidationResult {
	var results []ValidationResult

	for _, legacyPath := range legacyInterfaces {
		if _, err := os.Stat(legacyPath); err == nil {
			results = append(results, ValidationResult{
				File:     legacyPath,
				Issue:    "Legacy interface file still exists - should be removed",
				Severity: "error",
			})
		}
	}

	return results
}

func validateInterfaceConformance() []ValidationResult {
	var results []ValidationResult

	// Find all tool implementations
	toolFiles, err := findToolImplementations()
	if err != nil {
		results = append(results, ValidationResult{
			Issue:    fmt.Sprintf("Failed to find tool implementations: %v", err),
			Severity: "error",
		})
		return results
	}

	for _, toolFile := range toolFiles {
		conformanceResults := validateToolConformance(toolFile)
		results = append(results, conformanceResults...)
	}

	return results
}

func validateDuplicateInterfaces() []ValidationResult {
	var results []ValidationResult

	// Find all interface definitions across the codebase
	interfaceDefinitions := make(map[string][]string)

	err := filepath.WalkDir("pkg/mcp", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files that can't be parsed
		}

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

				if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					interfaceName := typeSpec.Name.Name
					interfaceDefinitions[interfaceName] = append(interfaceDefinitions[interfaceName], path)
				}
			}
		}

		return nil
	})

	if err != nil {
		results = append(results, ValidationResult{
			Issue:    fmt.Sprintf("Failed to scan for duplicate interfaces: %v", err),
			Severity: "error",
		})
		return results
	}

	// Check for duplicates
	for interfaceName, files := range interfaceDefinitions {
		if len(files) > 1 {
			results = append(results, ValidationResult{
				Interface: interfaceName,
				Issue:     fmt.Sprintf("Interface %s defined in multiple files: %v", interfaceName, files),
				Severity:  "error",
			})
		}
	}

	return results
}

func getInterfaceMethods(interfaceType *ast.InterfaceType) []string {
	var methods []string

	for _, method := range interfaceType.Methods.List {
		if len(method.Names) > 0 {
			// Regular method
			methodName := method.Names[0].Name
			methods = append(methods, methodName)
		}
	}

	return methods
}

func validateMethods(interfaceName string, expected []string, actual []string) error {
	actualSet := make(map[string]bool)
	for _, method := range actual {
		actualSet[method] = true
	}

	var missing []string
	for _, expectedMethod := range expected {
		// Extract just the method name (before the opening parenthesis)
		methodName := strings.Split(expectedMethod, "(")[0]
		if !actualSet[methodName] {
			missing = append(missing, methodName)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("interface %s missing methods: %v", interfaceName, missing)
	}

	return nil
}

func findToolImplementations() ([]string, error) {
	var toolFiles []string

	err := filepath.WalkDir("pkg/mcp/internal", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Look for files that likely contain tool implementations
		if strings.Contains(path, "tool") || strings.Contains(path, "atomic") {
			toolFiles = append(toolFiles, path)
		}

		return nil
	})

	return toolFiles, err
}

func validateToolConformance(filePath string) []ValidationResult {
	var results []ValidationResult

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		results = append(results, ValidationResult{
			File:     filePath,
			Issue:    fmt.Sprintf("Failed to parse file: %v", err),
			Severity: "warning",
		})
		return results
	}

	// Look for struct types that should implement Tool interface
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

			if _, ok := typeSpec.Type.(*ast.StructType); ok {
				structName := typeSpec.Name.Name
				if strings.HasSuffix(structName, "Tool") {
					// This should implement the Tool interface
					// Check if it has the required methods
					if !hasRequiredMethods(file, structName, expectedInterfaces["Tool"]) {
						results = append(results, ValidationResult{
							File:     filePath,
							Issue:    fmt.Sprintf("Struct %s should implement Tool interface but missing methods", structName),
							Severity: "error",
						})
					}
				}
			}
		}
	}

	return results
}

func hasRequiredMethods(file *ast.File, structName string, requiredMethods []string) bool {
	// This is a simplified check - in practice, you'd want to check method signatures
	// For now, just check if methods with the right names exist

	methodSet := make(map[string]bool)

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		// Check if this method belongs to our struct
		recvType := getReceiverType(funcDecl.Recv)
		if recvType == structName || recvType == "*"+structName {
			methodSet[funcDecl.Name.Name] = true
		}
	}

	// Check if all required methods are present
	for _, requiredMethod := range requiredMethods {
		methodName := strings.Split(requiredMethod, "(")[0]
		if !methodSet[methodName] {
			return false
		}
	}

	return true
}

func getReceiverType(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}

	field := recv.List[0]
	switch expr := field.Type.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	}

	return ""
}

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
func analyzeFileForMetrics(file *ast.File, filePath, packageName string,
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
func determinePatternType(interfaces []string, pkg string) string {
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
func countMethodsForImplementor(implementor string, interfaceMap map[string]*ast.InterfaceType) int {
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
