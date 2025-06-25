package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type QualityMetrics struct {
	Timestamp          time.Time            `json:"timestamp"`
	ErrorHandling      ErrorHandlingMetrics `json:"error_handling"`
	DirectoryStructure DirectoryMetrics     `json:"directory_structure"`
	TestCoverage       TestCoverageMetrics  `json:"test_coverage"`
	BuildMetrics       BuildMetrics         `json:"build_metrics"`
	CodeQuality        CodeQualityMetrics   `json:"code_quality"`
	Recommendations    []string             `json:"recommendations"`
}

type ErrorHandlingMetrics struct {
	TotalErrors       int                   `json:"total_errors"`
	RichErrors        int                   `json:"rich_errors"`
	StandardErrors    int                   `json:"standard_errors"`
	AdoptionRate      float64               `json:"adoption_rate"`
	FileBreakdown     map[string]ErrorStats `json:"file_breakdown"`
	PackageBreakdown  map[string]ErrorStats `json:"package_breakdown"`
	TopFilesToMigrate []FileErrorInfo       `json:"top_files_to_migrate"`
}

type ErrorStats struct {
	Total    int `json:"total"`
	Rich     int `json:"rich"`
	Standard int `json:"standard"`
}

type FileErrorInfo struct {
	Path           string  `json:"path"`
	StandardErrors int     `json:"standard_errors"`
	AdoptionRate   float64 `json:"adoption_rate"`
}

type DirectoryMetrics struct {
	TotalDirectories   int                    `json:"total_directories"`
	MaxDepth           int                    `json:"max_depth"`
	EmptyDirectories   int                    `json:"empty_directories"`
	DirectoriesByDepth map[int]int            `json:"directories_by_depth"`
	PackageStructure   map[string]PackageInfo `json:"package_structure"`
	Violations         []string               `json:"violations"`
}

type PackageInfo struct {
	FileCount      int      `json:"file_count"`
	Subdirectories []string `json:"subdirectories"`
	Depth          int      `json:"depth"`
}

type TestCoverageMetrics struct {
	OverallCoverage   float64                 `json:"overall_coverage"`
	PackageCoverage   map[string]CoverageInfo `json:"package_coverage"`
	UncoveredPackages []string                `json:"uncovered_packages"`
	TopCoverage       []PackageCoverageInfo   `json:"top_coverage"`
	BottomCoverage    []PackageCoverageInfo   `json:"bottom_coverage"`
}

type CoverageInfo struct {
	Coverage   float64 `json:"coverage"`
	Statements int     `json:"statements"`
	Covered    int     `json:"covered"`
}

type PackageCoverageInfo struct {
	Package  string  `json:"package"`
	Coverage float64 `json:"coverage"`
}

type BuildMetrics struct {
	BuildTime    time.Duration `json:"build_time"`
	TestTime     time.Duration `json:"test_time"`
	BinarySize   int64         `json:"binary_size"`
	Dependencies int           `json:"dependencies"`
	BuildHistory []BuildRecord `json:"build_history,omitempty"`
}

type BuildRecord struct {
	Timestamp time.Time     `json:"timestamp"`
	BuildTime time.Duration `json:"build_time"`
	Success   bool          `json:"success"`
}

type CodeQualityMetrics struct {
	CyclomaticComplexity map[string]int  `json:"cyclomatic_complexity"`
	LongFunctions        []FunctionInfo  `json:"long_functions"`
	DuplicateCode        []DuplicateInfo `json:"duplicate_code,omitempty"`
	TODOComments         int             `json:"todo_comments"`
}

type FunctionInfo struct {
	Package  string `json:"package"`
	Function string `json:"function"`
	Lines    int    `json:"lines"`
}

type DuplicateInfo struct {
	File1      string  `json:"file1"`
	File2      string  `json:"file2"`
	Similarity float64 `json:"similarity"`
}

var (
	rootDir       = flag.String("root", ".", "Root directory to analyze")
	outputFile    = flag.String("output", "quality-metrics.json", "Output file for metrics")
	outputFormat  = flag.String("format", "json", "Output format: json, text, or html")
	watch         = flag.Bool("watch", false, "Watch mode - update metrics continuously")
	watchInterval = flag.Duration("interval", 5*time.Minute, "Watch interval")
	historyFile   = flag.String("history", "", "File to store historical metrics")
)

func main() {
	flag.Parse()

	if *watch {
		runWatchMode()
	} else {
		if err := runOnce(); err != nil {
			log.Fatal(err)
		}
	}
}

func runOnce() error {
	metrics, err := collectMetrics(*rootDir)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Add recommendations based on metrics
	metrics.Recommendations = generateRecommendations(metrics)

	// Output metrics
	switch *outputFormat {
	case "json":
		return outputJSON(metrics)
	case "text":
		return outputText(metrics)
	case "html":
		return outputHTML(metrics)
	default:
		return fmt.Errorf("unknown output format: %s", *outputFormat)
	}
}

func runWatchMode() {
	log.Printf("Starting quality dashboard in watch mode (interval: %v)\n", *watchInterval)

	ticker := time.NewTicker(*watchInterval)
	defer ticker.Stop()

	// Run immediately
	if err := runOnce(); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Then run periodically
	for range ticker.C {
		if err := runOnce(); err != nil {
			log.Printf("Error: %v\n", err)
		}
	}
}

func collectMetrics(rootDir string) (*QualityMetrics, error) {
	metrics := &QualityMetrics{
		Timestamp: time.Now(),
	}

	// Collect error handling metrics
	errorMetrics, err := collectErrorHandlingMetrics(rootDir)
	if err != nil {
		return nil, fmt.Errorf("error handling metrics: %w", err)
	}
	metrics.ErrorHandling = *errorMetrics

	// Collect directory structure metrics
	dirMetrics, err := collectDirectoryMetrics(rootDir)
	if err != nil {
		return nil, fmt.Errorf("directory metrics: %w", err)
	}
	metrics.DirectoryStructure = *dirMetrics

	// Collect test coverage metrics
	coverageMetrics, err := collectTestCoverageMetrics(rootDir)
	if err != nil {
		log.Printf("Warning: failed to collect coverage metrics: %v", err)
		// Don't fail completely if coverage collection fails
	} else {
		metrics.TestCoverage = *coverageMetrics
	}

	// Collect build metrics
	buildMetrics, err := collectBuildMetrics(rootDir)
	if err != nil {
		log.Printf("Warning: failed to collect build metrics: %v", err)
	} else {
		metrics.BuildMetrics = *buildMetrics
	}

	// Collect code quality metrics
	qualityMetrics, err := collectCodeQualityMetrics(rootDir)
	if err != nil {
		log.Printf("Warning: failed to collect code quality metrics: %v", err)
	} else {
		metrics.CodeQuality = *qualityMetrics
	}

	return metrics, nil
}

func collectErrorHandlingMetrics(rootDir string) (*ErrorHandlingMetrics, error) {
	metrics := &ErrorHandlingMetrics{
		FileBreakdown:     make(map[string]ErrorStats),
		PackageBreakdown:  make(map[string]ErrorStats),
		TopFilesToMigrate: []FileErrorInfo{},
	}

	richErrorPattern := regexp.MustCompile(`types\.NewRichError|NewRichError|RichError`)
	standardErrorPattern := regexp.MustCompile(`fmt\.Errorf|errors\.New|errors\.Wrap`)

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "test.go") {
			return nil
		}

		// Skip vendor and test directories
		if strings.Contains(path, "/vendor/") || strings.Contains(path, "/.git/") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		richCount := len(richErrorPattern.FindAll(content, -1))
		standardCount := len(standardErrorPattern.FindAll(content, -1))

		if richCount > 0 || standardCount > 0 {
			relPath, _ := filepath.Rel(rootDir, path)
			metrics.FileBreakdown[relPath] = ErrorStats{
				Total:    richCount + standardCount,
				Rich:     richCount,
				Standard: standardCount,
			}

			// Update package breakdown
			pkg := filepath.Dir(relPath)
			pkgStats := metrics.PackageBreakdown[pkg]
			pkgStats.Total += richCount + standardCount
			pkgStats.Rich += richCount
			pkgStats.Standard += standardCount
			metrics.PackageBreakdown[pkg] = pkgStats

			metrics.TotalErrors += richCount + standardCount
			metrics.RichErrors += richCount
			metrics.StandardErrors += standardCount
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Calculate adoption rate
	if metrics.TotalErrors > 0 {
		metrics.AdoptionRate = float64(metrics.RichErrors) / float64(metrics.TotalErrors) * 100
	}

	// Find top files to migrate
	type fileError struct {
		path     string
		standard int
		rate     float64
	}

	var files []fileError
	for path, stats := range metrics.FileBreakdown {
		if stats.Standard > 0 {
			rate := float64(stats.Rich) / float64(stats.Total) * 100
			files = append(files, fileError{path, stats.Standard, rate})
		}
	}

	// Sort by standard errors (descending)
	sort.Slice(files, func(i, j int) bool {
		return files[i].standard > files[j].standard
	})

	// Take top 10
	for i, f := range files {
		if i >= 10 {
			break
		}
		metrics.TopFilesToMigrate = append(metrics.TopFilesToMigrate, FileErrorInfo{
			Path:           f.path,
			StandardErrors: f.standard,
			AdoptionRate:   f.rate,
		})
	}

	return metrics, nil
}

func collectDirectoryMetrics(rootDir string) (*DirectoryMetrics, error) {
	metrics := &DirectoryMetrics{
		DirectoriesByDepth: make(map[int]int),
		PackageStructure:   make(map[string]PackageInfo),
		Violations:         []string{},
	}

	// Calculate base depth
	baseDepth := strings.Count(rootDir, string(os.PathSeparator))

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git and vendor
		if strings.Contains(path, "/.git") || strings.Contains(path, "/vendor/") {
			return filepath.SkipDir
		}

		if d.IsDir() {
			metrics.TotalDirectories++

			// Calculate depth
			depth := strings.Count(path, string(os.PathSeparator)) - baseDepth
			metrics.DirectoriesByDepth[depth]++

			if depth > metrics.MaxDepth {
				metrics.MaxDepth = depth
			}

			// Check if directory is empty
			entries, err := os.ReadDir(path)
			if err == nil && len(entries) == 0 {
				metrics.EmptyDirectories++
				relPath, _ := filepath.Rel(rootDir, path)
				metrics.Violations = append(metrics.Violations, fmt.Sprintf("Empty directory: %s", relPath))
			}

			// Analyze package structure
			if strings.Contains(path, "/pkg/") || strings.Contains(path, "/cmd/") {
				relPath, _ := filepath.Rel(rootDir, path)

				// Count Go files
				goFiles := 0
				subdirs := []string{}

				if entries, err := os.ReadDir(path); err == nil {
					for _, entry := range entries {
						if strings.HasSuffix(entry.Name(), ".go") {
							goFiles++
						} else if entry.IsDir() {
							subdirs = append(subdirs, entry.Name())
						}
					}
				}

				metrics.PackageStructure[relPath] = PackageInfo{
					FileCount:      goFiles,
					Subdirectories: subdirs,
					Depth:          depth,
				}

				// Check for violations
				if depth > 5 {
					metrics.Violations = append(metrics.Violations,
						fmt.Sprintf("Directory too deep (%d levels): %s", depth, relPath))
				}
			}
		}

		return nil
	})

	return metrics, err
}

func collectTestCoverageMetrics(rootDir string) (*TestCoverageMetrics, error) {
	metrics := &TestCoverageMetrics{
		PackageCoverage:   make(map[string]CoverageInfo),
		UncoveredPackages: []string{},
		TopCoverage:       []PackageCoverageInfo{},
		BottomCoverage:    []PackageCoverageInfo{},
	}

	// Run go test with coverage
	cmd := exec.Command("go", "test", "-coverprofile=coverage.tmp", "./...")
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to parse partial output even if tests fail
		log.Printf("Warning: go test failed: %v\n%s", err, output)
	}

	// Parse coverage profile
	if _, err := os.Stat(filepath.Join(rootDir, "coverage.tmp")); err == nil {
		defer os.Remove(filepath.Join(rootDir, "coverage.tmp"))

		// Get coverage by package
		cmd = exec.Command("go", "tool", "cover", "-func=coverage.tmp")
		cmd.Dir = rootDir
		output, err = cmd.Output()
		if err != nil {
			return metrics, fmt.Errorf("failed to parse coverage: %w", err)
		}

		lines := strings.Split(string(output), "\n")

		for _, line := range lines {
			if line == "" {
				continue
			}

			// Parse coverage line
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if parts[len(parts)-1] == "%" {
					continue
				}

				// Extract package and coverage
				pkg := parts[0]
				coverageStr := strings.TrimSuffix(parts[len(parts)-1], "%")

				if coverage, err := parseFloat(coverageStr); err == nil {
					// For the total line
					if strings.HasPrefix(line, "total:") {
						metrics.OverallCoverage = coverage
					} else {
						// Extract package name from file path
						pkgName := extractPackageName(pkg)
						if info, exists := metrics.PackageCoverage[pkgName]; exists {
							// Aggregate coverage for the package
							info.Coverage = (info.Coverage + coverage) / 2
							metrics.PackageCoverage[pkgName] = info
						} else {
							metrics.PackageCoverage[pkgName] = CoverageInfo{
								Coverage: coverage,
							}
						}
					}
				}
			}
		}
	}

	// Find uncovered packages
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}

		// Skip non-Go directories
		if strings.Contains(path, "/.git") || strings.Contains(path, "/vendor/") {
			return filepath.SkipDir
		}

		// Check if directory contains Go files
		hasGoFiles := false
		entries, _ := os.ReadDir(path)
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				hasGoFiles = true
				break
			}
		}

		if hasGoFiles {
			relPath, _ := filepath.Rel(rootDir, path)
			if _, hasCoverage := metrics.PackageCoverage[relPath]; !hasCoverage {
				metrics.UncoveredPackages = append(metrics.UncoveredPackages, relPath)
			}
		}

		return nil
	})

	// Sort packages by coverage
	type pkgCov struct {
		pkg string
		cov float64
	}

	var packages []pkgCov
	for pkg, info := range metrics.PackageCoverage {
		packages = append(packages, pkgCov{pkg, info.Coverage})
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].cov > packages[j].cov
	})

	// Get top and bottom 5
	for i, p := range packages {
		if i < 5 {
			metrics.TopCoverage = append(metrics.TopCoverage, PackageCoverageInfo{
				Package:  p.pkg,
				Coverage: p.cov,
			})
		}
		if i >= len(packages)-5 {
			metrics.BottomCoverage = append(metrics.BottomCoverage, PackageCoverageInfo{
				Package:  p.pkg,
				Coverage: p.cov,
			})
		}
	}

	return metrics, nil
}

func collectBuildMetrics(rootDir string) (*BuildMetrics, error) {
	metrics := &BuildMetrics{}

	// Measure build time
	start := time.Now()
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = rootDir
	err := cmd.Run()
	metrics.BuildTime = time.Since(start)

	if err != nil {
		return metrics, fmt.Errorf("build failed: %w", err)
	}

	// Measure test time
	start = time.Now()
	cmd = exec.Command("go", "test", "./...")
	cmd.Dir = rootDir
	_ = cmd.Run() // Don't fail if tests fail
	metrics.TestTime = time.Since(start)

	// Count dependencies
	cmd = exec.Command("go", "list", "-m", "all")
	cmd.Dir = rootDir
	output, err := cmd.Output()
	if err == nil {
		metrics.Dependencies = len(strings.Split(string(output), "\n")) - 1
	}

	// Check binary size if main package exists
	if _, err := os.Stat(filepath.Join(rootDir, "main.go")); err == nil {
		cmd = exec.Command("go", "build", "-o", "temp-binary", ".")
		cmd.Dir = rootDir
		if err := cmd.Run(); err == nil {
			if info, err := os.Stat(filepath.Join(rootDir, "temp-binary")); err == nil {
				metrics.BinarySize = info.Size()
			}
			os.Remove(filepath.Join(rootDir, "temp-binary"))
		}
	}

	// Load build history if specified
	if *historyFile != "" {
		if data, err := os.ReadFile(*historyFile); err == nil {
			var history []BuildRecord
			if err := json.Unmarshal(data, &history); err == nil {
				metrics.BuildHistory = history
			}
		}

		// Add current build to history
		metrics.BuildHistory = append(metrics.BuildHistory, BuildRecord{
			Timestamp: time.Now(),
			BuildTime: metrics.BuildTime,
			Success:   true,
		})

		// Keep only last 100 records
		if len(metrics.BuildHistory) > 100 {
			metrics.BuildHistory = metrics.BuildHistory[len(metrics.BuildHistory)-100:]
		}

		// Save updated history
		if data, err := json.Marshal(metrics.BuildHistory); err == nil {
			os.WriteFile(*historyFile, data, 0644)
		}
	}

	return metrics, nil
}

func collectCodeQualityMetrics(rootDir string) (*CodeQualityMetrics, error) {
	metrics := &CodeQualityMetrics{
		CyclomaticComplexity: make(map[string]int),
		LongFunctions:        []FunctionInfo{},
	}

	todoPattern := regexp.MustCompile(`(?i)\b(todo|fixme|hack|xxx)\b`)

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") {
			return err
		}

		// Skip vendor and test files
		if strings.Contains(path, "/vendor/") || strings.Contains(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Count TODO comments
		metrics.TODOComments += len(todoPattern.FindAll(content, -1))

		// Parse Go file for function analysis
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return nil // Skip files that don't parse
		}

		// Analyze functions
		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				// Calculate cyclomatic complexity (simplified)
				complexity := calculateCyclomaticComplexity(fn)
				relPath, _ := filepath.Rel(rootDir, path)
				funcName := fmt.Sprintf("%s:%s", relPath, fn.Name.Name)
				metrics.CyclomaticComplexity[funcName] = complexity

				// Check function length
				if fn.Body != nil {
					start := fset.Position(fn.Body.Lbrace).Line
					end := fset.Position(fn.Body.Rbrace).Line
					lines := end - start

					if lines > 50 { // Functions longer than 50 lines
						metrics.LongFunctions = append(metrics.LongFunctions, FunctionInfo{
							Package:  relPath,
							Function: fn.Name.Name,
							Lines:    lines,
						})
					}
				}
			}
			return true
		})

		return nil
	})

	// Sort long functions by length
	sort.Slice(metrics.LongFunctions, func(i, j int) bool {
		return metrics.LongFunctions[i].Lines > metrics.LongFunctions[j].Lines
	})

	// Keep only top 20 longest functions
	if len(metrics.LongFunctions) > 20 {
		metrics.LongFunctions = metrics.LongFunctions[:20]
	}

	return metrics, err
}

func calculateCyclomaticComplexity(fn *ast.FuncDecl) int {
	complexity := 1 // Base complexity

	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})

	return complexity
}

func generateRecommendations(metrics *QualityMetrics) []string {
	var recommendations []string

	// Error handling recommendations
	if metrics.ErrorHandling.AdoptionRate < 80 {
		recommendations = append(recommendations,
			fmt.Sprintf("🔴 Error Handling: Only %.1f%% adoption of RichError. Target: 80%%",
				metrics.ErrorHandling.AdoptionRate))

		if len(metrics.ErrorHandling.TopFilesToMigrate) > 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("   Start with: %s (%d standard errors)",
					metrics.ErrorHandling.TopFilesToMigrate[0].Path,
					metrics.ErrorHandling.TopFilesToMigrate[0].StandardErrors))
		}
	}

	// Directory structure recommendations
	if metrics.DirectoryStructure.MaxDepth > 5 {
		recommendations = append(recommendations,
			fmt.Sprintf("🟡 Directory Structure: Max depth is %d (recommended: ≤5)",
				metrics.DirectoryStructure.MaxDepth))
	}

	if metrics.DirectoryStructure.EmptyDirectories > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("🟡 Clean up %d empty directories",
				metrics.DirectoryStructure.EmptyDirectories))
	}

	// Test coverage recommendations
	if metrics.TestCoverage.OverallCoverage < 70 {
		recommendations = append(recommendations,
			fmt.Sprintf("🔴 Test Coverage: %.1f%% (target: 70%%)",
				metrics.TestCoverage.OverallCoverage))

		if len(metrics.TestCoverage.BottomCoverage) > 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("   Lowest coverage: %s (%.1f%%)",
					metrics.TestCoverage.BottomCoverage[0].Package,
					metrics.TestCoverage.BottomCoverage[0].Coverage))
		}
	}

	if len(metrics.TestCoverage.UncoveredPackages) > 5 {
		recommendations = append(recommendations,
			fmt.Sprintf("🟡 Add tests for %d uncovered packages",
				len(metrics.TestCoverage.UncoveredPackages)))
	}

	// Build performance recommendations
	if metrics.BuildMetrics.BuildTime > 30*time.Second {
		recommendations = append(recommendations,
			fmt.Sprintf("🟡 Build Time: %v (consider optimization for times >30s)",
				metrics.BuildMetrics.BuildTime.Round(time.Second)))
	}

	// Code quality recommendations
	if metrics.CodeQuality.TODOComments > 50 {
		recommendations = append(recommendations,
			fmt.Sprintf("🟡 Address %d TODO/FIXME comments",
				metrics.CodeQuality.TODOComments))
	}

	if len(metrics.CodeQuality.LongFunctions) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("🟡 Refactor %d functions with >50 lines",
				len(metrics.CodeQuality.LongFunctions)))
	}

	// High complexity functions
	highComplexity := 0
	for _, c := range metrics.CodeQuality.CyclomaticComplexity {
		if c > 10 {
			highComplexity++
		}
	}
	if highComplexity > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("🟡 Simplify %d functions with complexity >10",
				highComplexity))
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "✅ All quality metrics are within acceptable ranges!")
	}

	return recommendations
}

func outputJSON(metrics *QualityMetrics) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}

	if *outputFile == "-" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(*outputFile, data, 0644); err != nil {
			return err
		}
		fmt.Printf("Metrics written to %s\n", *outputFile)
	}

	return nil
}

func outputText(metrics *QualityMetrics) error {
	fmt.Printf("Quality Dashboard Report\n")
	fmt.Printf("Generated: %s\n\n", metrics.Timestamp.Format(time.RFC3339))

	fmt.Printf("ERROR HANDLING METRICS\n")
	fmt.Printf("=====================\n")
	fmt.Printf("Total Errors: %d\n", metrics.ErrorHandling.TotalErrors)
	fmt.Printf("Rich Errors: %d (%.1f%%)\n",
		metrics.ErrorHandling.RichErrors,
		metrics.ErrorHandling.AdoptionRate)
	fmt.Printf("Standard Errors: %d\n\n", metrics.ErrorHandling.StandardErrors)

	fmt.Printf("DIRECTORY STRUCTURE\n")
	fmt.Printf("==================\n")
	fmt.Printf("Total Directories: %d\n", metrics.DirectoryStructure.TotalDirectories)
	fmt.Printf("Max Depth: %d\n", metrics.DirectoryStructure.MaxDepth)
	fmt.Printf("Empty Directories: %d\n\n", metrics.DirectoryStructure.EmptyDirectories)

	fmt.Printf("TEST COVERAGE\n")
	fmt.Printf("=============\n")
	fmt.Printf("Overall Coverage: %.1f%%\n", metrics.TestCoverage.OverallCoverage)
	fmt.Printf("Uncovered Packages: %d\n\n", len(metrics.TestCoverage.UncoveredPackages))

	fmt.Printf("BUILD METRICS\n")
	fmt.Printf("=============\n")
	fmt.Printf("Build Time: %v\n", metrics.BuildMetrics.BuildTime.Round(time.Millisecond))
	fmt.Printf("Test Time: %v\n", metrics.BuildMetrics.TestTime.Round(time.Millisecond))
	fmt.Printf("Dependencies: %d\n\n", metrics.BuildMetrics.Dependencies)

	fmt.Printf("CODE QUALITY\n")
	fmt.Printf("============\n")
	fmt.Printf("TODO Comments: %d\n", metrics.CodeQuality.TODOComments)
	fmt.Printf("Long Functions (>50 lines): %d\n\n", len(metrics.CodeQuality.LongFunctions))

	fmt.Printf("RECOMMENDATIONS\n")
	fmt.Printf("===============\n")
	for _, rec := range metrics.Recommendations {
		fmt.Printf("%s\n", rec)
	}

	return nil
}

func outputHTML(metrics *QualityMetrics) error {
	// Simple HTML dashboard
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Quality Dashboard - %s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .metric { background: #f0f0f0; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .good { color: green; }
        .warning { color: orange; }
        .bad { color: red; }
        h2 { color: #333; }
        .chart { margin: 20px 0; }
        table { border-collapse: collapse; width: 100%%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>Quality Dashboard</h1>
    <p>Generated: %s</p>
    
    <div class="metric">
        <h2>Error Handling</h2>
        <p>Adoption Rate: <span class="%s">%.1f%%</span></p>
        <p>Total Errors: %d (Rich: %d, Standard: %d)</p>
    </div>
    
    <div class="metric">
        <h2>Test Coverage</h2>
        <p>Overall Coverage: <span class="%s">%.1f%%</span></p>
        <p>Uncovered Packages: %d</p>
    </div>
    
    <div class="metric">
        <h2>Build Performance</h2>
        <p>Build Time: %v</p>
        <p>Test Time: %v</p>
    </div>
    
    <div class="metric">
        <h2>Recommendations</h2>
        <ul>
        %s
        </ul>
    </div>
</body>
</html>`,
		metrics.Timestamp.Format("2006-01-02 15:04:05"),
		metrics.Timestamp.Format(time.RFC3339),
		getColorClass(metrics.ErrorHandling.AdoptionRate, 80, 60),
		metrics.ErrorHandling.AdoptionRate,
		metrics.ErrorHandling.TotalErrors,
		metrics.ErrorHandling.RichErrors,
		metrics.ErrorHandling.StandardErrors,
		getColorClass(metrics.TestCoverage.OverallCoverage, 70, 50),
		metrics.TestCoverage.OverallCoverage,
		len(metrics.TestCoverage.UncoveredPackages),
		metrics.BuildMetrics.BuildTime.Round(time.Millisecond),
		metrics.BuildMetrics.TestTime.Round(time.Millisecond),
		formatRecommendationsHTML(metrics.Recommendations))

	if *outputFile == "-" {
		fmt.Println(html)
	} else {
		outputPath := strings.TrimSuffix(*outputFile, filepath.Ext(*outputFile)) + ".html"
		if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
			return err
		}
		fmt.Printf("HTML dashboard written to %s\n", outputPath)
	}

	return nil
}

func getColorClass(value, goodThreshold, warningThreshold float64) string {
	if value >= goodThreshold {
		return "good"
	} else if value >= warningThreshold {
		return "warning"
	}
	return "bad"
}

func formatRecommendationsHTML(recommendations []string) string {
	var items []string
	for _, rec := range recommendations {
		// Convert emoji to HTML entities
		rec = strings.ReplaceAll(rec, "🔴", "&#x1F534;")
		rec = strings.ReplaceAll(rec, "🟡", "&#x1F7E1;")
		rec = strings.ReplaceAll(rec, "✅", "&#x2705;")
		items = append(items, fmt.Sprintf("<li>%s</li>", rec))
	}
	return strings.Join(items, "\n")
}

func extractPackageName(filePath string) string {
	// Extract package name from file path
	parts := strings.Split(filePath, "/")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return "."
}

func parseFloat(s string) (float64, error) {
	if s == "---" || s == "" {
		return 0, fmt.Errorf("invalid coverage value")
	}
	return strconv.ParseFloat(s, 64)
}
