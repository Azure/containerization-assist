// Package main provides performance regression detection for Container Kit
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

// PerformanceBaseline represents the expected performance characteristics
type PerformanceBaseline struct {
	PerformanceBaseline struct {
		Date            string `json:"date"`
		Version         string `json:"version"`
		Description     string `json:"description"`
		TargetP95Micros int    `json:"target_p95_latency_microseconds"`
		Measurements    struct {
			CoreSamplingPerformance struct {
				SmallPrompt  BenchmarkResult `json:"small_prompt"`
				MediumPrompt BenchmarkResult `json:"medium_prompt"`
				LargePrompt  BenchmarkResult `json:"large_prompt"`
			} `json:"core_sampling_performance"`
		} `json:"measurements"`
		RegressionThresholds struct {
			P95LatencyIncreaseMaxPercent int `json:"p95_latency_increase_max_percent"`
			MemoryIncreaseMaxPercent     int `json:"memory_increase_max_percent"`
			AllocationIncreaseMaxPercent int `json:"allocation_increase_max_percent"`
		} `json:"regression_thresholds"`
	} `json:"performance_baseline"`
}

// BenchmarkResult represents a single benchmark measurement
type BenchmarkResult struct {
	P95LatencyNs     int     `json:"p95_latency_ns"`
	P95LatencyMicros float64 `json:"p95_latency_microseconds"`
	MemoryPerOpBytes int     `json:"memory_per_op_bytes"`
	AllocationsPerOp int     `json:"allocations_per_op"`
	MeetsTarget      bool    `json:"meets_target"`
	Notes            string  `json:"notes"`
}

// CurrentPerformance represents current benchmark results
type CurrentPerformance struct {
	SmallPrompt  BenchmarkResult
	MediumPrompt BenchmarkResult
	LargePrompt  BenchmarkResult
}

func main() {
	fmt.Println("🚀 Container Kit Performance Monitor")
	fmt.Println("====================================")

	// Load baseline
	baseline, err := loadBaseline("performance-baseline.json")
	if err != nil {
		log.Fatalf("Failed to load performance baseline: %v", err)
	}

	fmt.Printf("📊 Baseline: %s (%s)\n", baseline.PerformanceBaseline.Version, baseline.PerformanceBaseline.Date)
	fmt.Printf("🎯 Target: P95 < %dμs\n", baseline.PerformanceBaseline.TargetP95Micros)

	// Run current benchmarks
	fmt.Println("\n⏱️  Running performance benchmarks...")
	current, err := runBenchmarks()
	if err != nil {
		log.Fatalf("Failed to run benchmarks: %v", err)
	}

	// Compare and detect regressions
	fmt.Println("\n📈 Performance Analysis")
	fmt.Println("=======================")

	hasRegression := false
	thresholds := baseline.PerformanceBaseline.RegressionThresholds

	// Check each benchmark
	benchmarks := map[string]struct {
		baseline BenchmarkResult
		current  BenchmarkResult
	}{
		"SmallPrompt":  {baseline.PerformanceBaseline.Measurements.CoreSamplingPerformance.SmallPrompt, current.SmallPrompt},
		"MediumPrompt": {baseline.PerformanceBaseline.Measurements.CoreSamplingPerformance.MediumPrompt, current.MediumPrompt},
		"LargePrompt":  {baseline.PerformanceBaseline.Measurements.CoreSamplingPerformance.LargePrompt, current.LargePrompt},
	}

	for name, bench := range benchmarks {
		fmt.Printf("\n🔍 %s:\n", name)

		// Check P95 latency
		latencyIncrease := calculatePercentIncrease(bench.baseline.P95LatencyNs, bench.current.P95LatencyNs)
		fmt.Printf("   P95 Latency: %dns → %dns (%+.1f%%)\n",
			bench.baseline.P95LatencyNs, bench.current.P95LatencyNs, latencyIncrease)

		if latencyIncrease > float64(thresholds.P95LatencyIncreaseMaxPercent) {
			fmt.Printf("   ❌ REGRESSION: P95 latency increased by %.1f%% (threshold: %d%%)\n",
				latencyIncrease, thresholds.P95LatencyIncreaseMaxPercent)
			hasRegression = true
		} else {
			fmt.Printf("   ✅ P95 latency within acceptable range\n")
		}

		// Check memory usage
		memoryIncrease := calculatePercentIncrease(bench.baseline.MemoryPerOpBytes, bench.current.MemoryPerOpBytes)
		fmt.Printf("   Memory: %dB → %dB (%+.1f%%)\n",
			bench.baseline.MemoryPerOpBytes, bench.current.MemoryPerOpBytes, memoryIncrease)

		if memoryIncrease > float64(thresholds.MemoryIncreaseMaxPercent) {
			fmt.Printf("   ❌ REGRESSION: Memory usage increased by %.1f%% (threshold: %d%%)\n",
				memoryIncrease, thresholds.MemoryIncreaseMaxPercent)
			hasRegression = true
		} else {
			fmt.Printf("   ✅ Memory usage within acceptable range\n")
		}

		// Check allocations
		allocIncrease := calculatePercentIncrease(bench.baseline.AllocationsPerOp, bench.current.AllocationsPerOp)
		fmt.Printf("   Allocations: %d → %d (%+.1f%%)\n",
			bench.baseline.AllocationsPerOp, bench.current.AllocationsPerOp, allocIncrease)

		if allocIncrease > float64(thresholds.AllocationIncreaseMaxPercent) {
			fmt.Printf("   ❌ REGRESSION: Allocation count increased by %.1f%% (threshold: %d%%)\n",
				allocIncrease, thresholds.AllocationIncreaseMaxPercent)
			hasRegression = true
		} else {
			fmt.Printf("   ✅ Allocation count within acceptable range\n")
		}
	}

	fmt.Println("\n📋 Summary")
	fmt.Println("===========")
	if hasRegression {
		fmt.Println("❌ Performance regressions detected!")
		fmt.Println("💡 Consider optimizing the affected code paths before merging.")
		os.Exit(1)
	} else {
		fmt.Println("✅ All performance metrics within acceptable thresholds!")
		fmt.Println("🚀 Performance is stable - safe to proceed.")
	}
}

func loadBaseline(filename string) (*PerformanceBaseline, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading baseline file: %w", err)
	}

	var baseline PerformanceBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("parsing baseline JSON: %w", err)
	}

	return &baseline, nil
}

func runBenchmarks() (*CurrentPerformance, error) {
	// Run the core sampling performance benchmark
	cmd := exec.Command("go", "test", "-bench=BenchmarkCoreSamplingPerformance",
		"./pkg/mcp/infrastructure/ai_ml/sampling/", "-benchmem", "-run=^$", "-timeout=30s")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running benchmarks: %w\nOutput: %s", err, output)
	}

	return parseBenchmarkOutput(string(output))
}

func parseBenchmarkOutput(output string) (*CurrentPerformance, error) {
	// Parse benchmark output with regex
	// Example: BenchmarkCoreSamplingPerformance/SmallPrompt-20         	 6495940	       196.6 ns/op	       138.0 p95_ns	         0 p95_μs	      48 B/op	       2 allocs/op
	re := regexp.MustCompile(`BenchmarkCoreSamplingPerformance/(\w+)-\d+\s+\d+\s+[\d.]+\s+ns/op\s+([\d.]+)\s+p95_ns\s+[\d.]+\s+p95_μs\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)

	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) < 3 {
		return nil, fmt.Errorf("could not parse benchmark output - expected 3 benchmarks, found %d", len(matches))
	}

	current := &CurrentPerformance{}

	for _, match := range matches {
		if len(match) != 5 {
			continue
		}

		name := match[1]
		p95Ns, _ := strconv.ParseFloat(match[2], 64)
		memoryBytes, _ := strconv.Atoi(match[3])
		allocations, _ := strconv.Atoi(match[4])

		result := BenchmarkResult{
			P95LatencyNs:     int(p95Ns),
			P95LatencyMicros: p95Ns / 1000.0,
			MemoryPerOpBytes: memoryBytes,
			AllocationsPerOp: allocations,
			MeetsTarget:      p95Ns < 300000, // 300μs in nanoseconds
		}

		switch name {
		case "SmallPrompt":
			current.SmallPrompt = result
		case "MediumPrompt":
			current.MediumPrompt = result
		case "LargePrompt":
			current.LargePrompt = result
		}
	}

	return current, nil
}

func calculatePercentIncrease(baseline, current int) float64 {
	if baseline == 0 {
		if current == 0 {
			return 0
		}
		return 100 // 100% increase from 0
	}
	return (float64(current-baseline) / float64(baseline)) * 100
}
