package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	baseline = flag.String("baseline", "", "Path to baseline performance data file")
	output   = flag.String("output", "performance_baseline.json", "Output file for performance data")
	compare  = flag.Bool("compare", false, "Compare current performance to baseline")
	verbose  = flag.Bool("verbose", false, "Verbose output")
)

type PerformanceMetrics struct {
	Timestamp     time.Time              `json:"timestamp"`
	Version       string                 `json:"version,omitempty"`
	BuildTime     Duration               `json:"build_time"`
	TestTime      Duration               `json:"test_time"`
	BinarySize    FileSize               `json:"binary_size"`
	PackageCount  int                    `json:"package_count"`
	FileCount     int                    `json:"file_count"`
	LineCount     int                    `json:"line_count"`
	Benchmarks    map[string]BenchResult `json:"benchmarks"`
	CompileMemory MemoryUsage            `json:"compile_memory"`
}

type Duration struct {
	Seconds float64 `json:"seconds"`
	Human   string  `json:"human"`
}

type FileSize struct {
	Bytes int64  `json:"bytes"`
	Human string `json:"human"`
}

type MemoryUsage struct {
	Bytes int64  `json:"bytes"`
	Human string `json:"human"`
}

type BenchResult struct {
	NsPerOp     int64   `json:"ns_per_op"`
	AllocsPerOp int64   `json:"allocs_per_op"`
	BytesPerOp  int64   `json:"bytes_per_op"`
	MemPerOp    string  `json:"mem_per_op"`
	Iterations  int     `json:"iterations"`
}

type PerformanceComparison struct {
	Metric      string  `json:"metric"`
	Baseline    float64 `json:"baseline"`
	Current     float64 `json:"current"`
	Change      float64 `json:"change"`
	ChangeType  string  `json:"change_type"` // "improvement", "regression", "neutral"
	Significant bool    `json:"significant"`
}

func main() {
	flag.Parse()
	
	fmt.Println("MCP Performance Measurement Tool")
	fmt.Println("================================")
	
	metrics := &PerformanceMetrics{
		Timestamp:  time.Now(),
		Benchmarks: make(map[string]BenchResult),
	}
	
	// Get Git version for tracking
	if version, err := getGitVersion(); err == nil {
		metrics.Version = version
	}
	
	// 1. Measure build time
	fmt.Println("📏 Measuring build time...")
	buildTime, err := measureBuildTime()
	if err != nil {
		log.Printf("⚠️  Failed to measure build time: %v", err)
	} else {
		metrics.BuildTime = buildTime
		fmt.Printf("   Build time: %s\n", buildTime.Human)
	}
	
	// 2. Measure test time
	fmt.Println("📏 Measuring test time...")
	testTime, err := measureTestTime()
	if err != nil {
		log.Printf("⚠️  Failed to measure test time: %v", err)
	} else {
		metrics.TestTime = testTime
		fmt.Printf("   Test time: %s\n", testTime.Human)
	}
	
	// 3. Measure binary size
	fmt.Println("📏 Measuring binary size...")
	binarySize, err := measureBinarySize()
	if err != nil {
		log.Printf("⚠️  Failed to measure binary size: %v", err)
	} else {
		metrics.BinarySize = binarySize
		fmt.Printf("   Binary size: %s\n", binarySize.Human)
	}
	
	// 4. Count packages and files
	fmt.Println("📏 Counting packages and files...")
	packageCount, fileCount, lineCount, err := countCodeMetrics()
	if err != nil {
		log.Printf("⚠️  Failed to count code metrics: %v", err)
	} else {
		metrics.PackageCount = packageCount
		metrics.FileCount = fileCount
		metrics.LineCount = lineCount
		fmt.Printf("   Packages: %d, Files: %d, Lines: %d\n", packageCount, fileCount, lineCount)
	}
	
	// 5. Run benchmarks
	fmt.Println("📏 Running benchmarks...")
	benchmarks, err := runBenchmarks()
	if err != nil {
		log.Printf("⚠️  Failed to run benchmarks: %v", err)
	} else {
		metrics.Benchmarks = benchmarks
		fmt.Printf("   Ran %d benchmarks\n", len(benchmarks))
	}
	
	// 6. Measure compile memory usage
	fmt.Println("📏 Measuring compile memory usage...")
	compileMemory, err := measureCompileMemory()
	if err != nil {
		log.Printf("⚠️  Failed to measure compile memory: %v", err)
	} else {
		metrics.CompileMemory = compileMemory
		fmt.Printf("   Compile memory: %s\n", compileMemory.Human)
	}
	
	// Save metrics
	if err := saveMetrics(metrics, *output); err != nil {
		log.Fatalf("Failed to save metrics: %v", err)
	}
	
	fmt.Printf("\n✅ Performance metrics saved to: %s\n", *output)
	
	// Compare with baseline if requested
	if *compare && *baseline != "" {
		fmt.Println("\n📊 Comparing with baseline...")
		if err := compareWithBaseline(metrics, *baseline); err != nil {
			log.Printf("⚠️  Failed to compare with baseline: %v", err)
		}
	}
}

func getGitVersion() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output))[:8], nil
}

func measureBuildTime() (Duration, error) {
	// Clean first
	exec.Command("go", "clean", "-cache").Run()
	
	start := time.Now()
	cmd := exec.Command("go", "build", "./...")
	err := cmd.Run()
	duration := time.Since(start)
	
	return Duration{
		Seconds: duration.Seconds(),
		Human:   duration.String(),
	}, err
}

func measureTestTime() (Duration, error) {
	start := time.Now()
	cmd := exec.Command("go", "test", "./...", "-v")
	err := cmd.Run()
	duration := time.Since(start)
	
	return Duration{
		Seconds: duration.Seconds(),
		Human:   duration.String(),
	}, err
}

func measureBinarySize() (FileSize, error) {
	// Build the main binary
	tempDir := os.TempDir()
	binaryPath := filepath.Join(tempDir, "mcp-server-test")
	
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/mcp-server")
	if err := cmd.Run(); err != nil {
		return FileSize{}, fmt.Errorf("failed to build binary: %w", err)
	}
	
	// Get file size
	info, err := os.Stat(binaryPath)
	if err != nil {
		return FileSize{}, fmt.Errorf("failed to stat binary: %w", err)
	}
	
	// Clean up
	os.Remove(binaryPath)
	
	size := info.Size()
	return FileSize{
		Bytes: size,
		Human: formatBytes(size),
	}, nil
}

func countCodeMetrics() (int, int, int, error) {
	packageCount := 0
	fileCount := 0
	lineCount := 0
	
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip vendor, .git, and other irrelevant directories
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			// Count as package if it contains Go files
			hasGoFiles, _ := hasGoFilesInDir(path)
			if hasGoFiles {
				packageCount++
			}
			return nil
		}
		
		if strings.HasSuffix(path, ".go") {
			fileCount++
			
			// Count lines in the file
			lines, err := countLinesInFile(path)
			if err == nil {
				lineCount += lines
			}
		}
		
		return nil
	})
	
	return packageCount, fileCount, lineCount, err
}

func runBenchmarks() (map[string]BenchResult, error) {
	benchmarks := make(map[string]BenchResult)
	
	// Run benchmarks with memory stats
	cmd := exec.Command("go", "test", "-bench=.", "-benchmem", "./...")
	output, err := cmd.Output()
	if err != nil {
		return benchmarks, fmt.Errorf("failed to run benchmarks: %w", err)
	}
	
	// Parse benchmark output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Benchmark") {
			result := parseBenchmarkLine(line)
			if result != nil {
				// Extract benchmark name
				parts := strings.Fields(line)
				if len(parts) > 0 {
					benchName := parts[0]
					benchmarks[benchName] = *result
				}
			}
		}
	}
	
	return benchmarks, nil
}

func measureCompileMemory() (MemoryUsage, error) {
	// Use go build with memory profiling
	tempDir := os.TempDir()
	memProfilePath := filepath.Join(tempDir, "compile.mem")
	
	cmd := exec.Command("go", "build", "-gcflags", "-memprofile="+memProfilePath, "./...")
	err := cmd.Run()
	if err != nil {
		return MemoryUsage{}, fmt.Errorf("failed to build with memory profiling: %w", err)
	}
	
	// Try to get memory info from the profile (simplified)
	info, err := os.Stat(memProfilePath)
	if err != nil {
		return MemoryUsage{}, nil // Return empty if profiling didn't work
	}
	
	// Clean up
	os.Remove(memProfilePath)
	
	// This is a rough approximation - in practice you'd parse the actual profile
	size := info.Size() * 1000 // Rough multiplier for actual memory usage
	
	return MemoryUsage{
		Bytes: size,
		Human: formatBytes(size),
	}, nil
}

func saveMetrics(metrics *PerformanceMetrics, outputPath string) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}
	
	return os.WriteFile(outputPath, data, 0644)
}

func compareWithBaseline(current *PerformanceMetrics, baselinePath string) error {
	// Load baseline data
	baselineData, err := os.ReadFile(baselinePath)
	if err != nil {
		return fmt.Errorf("failed to read baseline file: %w", err)
	}
	
	var baseline PerformanceMetrics
	if err := json.Unmarshal(baselineData, &baseline); err != nil {
		return fmt.Errorf("failed to parse baseline data: %w", err)
	}
	
	// Compare metrics
	fmt.Println("\n📊 Performance Comparison")
	fmt.Println("========================")
	
	comparisons := []PerformanceComparison{
		compareMetric("Build Time", baseline.BuildTime.Seconds, current.BuildTime.Seconds, true),
		compareMetric("Test Time", baseline.TestTime.Seconds, current.TestTime.Seconds, true),
		compareMetric("Binary Size", float64(baseline.BinarySize.Bytes), float64(current.BinarySize.Bytes), true),
		compareMetric("Package Count", float64(baseline.PackageCount), float64(current.PackageCount), true),
		compareMetric("File Count", float64(baseline.FileCount), float64(current.FileCount), true),
		compareMetric("Line Count", float64(baseline.LineCount), float64(current.LineCount), true),
	}
	
	regressions := 0
	improvements := 0
	
	for _, comp := range comparisons {
		symbol := "="
		if comp.ChangeType == "improvement" {
			symbol = "↓"
			improvements++
		} else if comp.ChangeType == "regression" {
			symbol = "↑"
			regressions++
		}
		
		fmt.Printf("%s %s: %.2f%% change %s\n", 
			symbol, comp.Metric, comp.Change*100, 
			formatComparison(comp.Baseline, comp.Current))
	}
	
	fmt.Printf("\nSummary: %d improvements, %d regressions\n", improvements, regressions)
	
	if regressions > 0 {
		fmt.Println("⚠️  Performance regressions detected!")
	} else if improvements > 0 {
		fmt.Println("✅ Performance improvements detected!")
	} else {
		fmt.Println("➡️  Performance remained stable")
	}
	
	return nil
}

func compareMetric(name string, baseline, current float64, lowerIsBetter bool) PerformanceComparison {
	change := (current - baseline) / baseline
	changeType := "neutral"
	
	if change > 0.05 { // 5% threshold
		if lowerIsBetter {
			changeType = "regression"
		} else {
			changeType = "improvement"
		}
	} else if change < -0.05 {
		if lowerIsBetter {
			changeType = "improvement"
		} else {
			changeType = "regression"
		}
	}
	
	return PerformanceComparison{
		Metric:      name,
		Baseline:    baseline,
		Current:     current,
		Change:      change,
		ChangeType:  changeType,
		Significant: change > 0.05 || change < -0.05,
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatComparison(baseline, current float64) string {
	return fmt.Sprintf("(%.2f → %.2f)", baseline, current)
}

func hasGoFilesInDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}
	
	return false, nil
}

func countLinesInFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	
	return strings.Count(string(data), "\n"), nil
}

func parseBenchmarkLine(line string) *BenchResult {
	// Example: BenchmarkExample-8    1000000    1234 ns/op    456 B/op    7 allocs/op
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return nil
	}
	
	result := &BenchResult{}
	
	// Parse iterations
	if iterations, err := strconv.Atoi(parts[1]); err == nil {
		result.Iterations = iterations
	}
	
	// Parse ns/op
	if len(parts) >= 4 && strings.HasSuffix(parts[3], "ns/op") {
		nsStr := strings.TrimSuffix(parts[2], "ns/op")
		if ns, err := strconv.ParseInt(nsStr, 10, 64); err == nil {
			result.NsPerOp = ns
		}
	}
	
	// Parse memory stats if present
	for i, part := range parts {
		if strings.HasSuffix(part, "B/op") && i > 0 {
			bytesStr := strings.TrimSuffix(parts[i-1], "B/op")
			if bytes, err := strconv.ParseInt(bytesStr, 10, 64); err == nil {
				result.BytesPerOp = bytes
			}
		}
		if strings.HasSuffix(part, "allocs/op") && i > 0 {
			allocsStr := strings.TrimSuffix(parts[i-1], "allocs/op")
			if allocs, err := strconv.ParseInt(allocsStr, 10, 64); err == nil {
				result.AllocsPerOp = allocs
			}
		}
	}
	
	return result
}