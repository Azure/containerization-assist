// Package ml provides build optimization integration for Container Kit.
package ml

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// BuildOptimizer integrates resource prediction with Docker build operations
type BuildOptimizer struct {
	predictor *ResourcePredictor
	logger    *slog.Logger
}

// NewBuildOptimizer creates a new build optimizer
func NewBuildOptimizer(predictor *ResourcePredictor, logger *slog.Logger) *BuildOptimizer {
	return &BuildOptimizer{
		predictor: predictor,
		logger:    logger.With("component", "build_optimizer"),
	}
}

// OptimizeBuildCommand generates an optimized docker build command based on predictions
func (o *BuildOptimizer) OptimizeBuildCommand(
	ctx context.Context,
	baseCommand string,
	analysis RepositoryAnalysis,
	dockerfilePath string,
	contextPath string,
	tags []string,
) (string, *ResourcePrediction, error) {

	// Get resource predictions
	prediction, err := o.predictor.PredictResources(ctx, analysis)
	if err != nil {
		o.logger.Error("Failed to predict resources", "error", err)
		// Return base command on prediction failure
		return baseCommand, nil, nil
	}

	// Build optimized command
	optimizedCmd := o.buildOptimizedCommand(baseCommand, prediction, dockerfilePath, contextPath, tags)

	o.logger.Info("Generated optimized build command",
		"cpu_cores", prediction.CPU.Cores,
		"memory_mb", prediction.Memory.RecommendedMB,
		"cache_mounts", len(prediction.Cache.MountCaches),
		"confidence", prediction.Confidence)

	return optimizedCmd, prediction, nil
}

// buildOptimizedCommand constructs the docker build command with optimizations
func (o *BuildOptimizer) buildOptimizedCommand(
	baseCommand string,
	prediction *ResourcePrediction,
	dockerfilePath string,
	contextPath string,
	tags []string,
) string {

	var parts []string

	// Start with docker buildx build for advanced features
	parts = append(parts, "docker", "buildx", "build")

	// Add platform if specified
	if prediction.CPU.Architecture != "" {
		parts = append(parts, "--platform", fmt.Sprintf("linux/%s", prediction.CPU.Architecture))
	}

	// Add CPU limits
	if prediction.CPU.Cores > 0 {
		parts = append(parts, "--cpuset-cpus", fmt.Sprintf("0-%d", prediction.CPU.Cores-1))
	}

	// Add memory limits
	if prediction.Memory.RecommendedMB > 0 {
		parts = append(parts, "--memory", fmt.Sprintf("%dm", prediction.Memory.RecommendedMB))
	}

	// Add cache configuration
	if prediction.Cache.UseCache {
		// Cache from
		for _, cacheFrom := range prediction.Cache.CacheFrom {
			parts = append(parts, "--cache-from", cacheFrom)
		}

		// Cache to
		for _, cacheTo := range prediction.Cache.CacheTo {
			parts = append(parts, "--cache-to", cacheTo)
		}

		// Mount caches
		for _, mount := range prediction.Cache.MountCaches {
			mountStr := fmt.Sprintf("type=%s,target=%s", mount.Type, mount.Target)
			if mount.ID != "" {
				mountStr += fmt.Sprintf(",id=%s", mount.ID)
			}
			if mount.Sharing != "" {
				mountStr += fmt.Sprintf(",sharing=%s", mount.Sharing)
			}
			if mount.ReadOnly {
				mountStr += ",ro=true"
			}
			parts = append(parts, "--mount", mountStr)
		}
	}

	// Add build args for optimization
	parts = append(parts, "--build-arg", fmt.Sprintf("BUILDKIT_CPU_LIMIT=%d", prediction.CPU.Cores))
	parts = append(parts, "--build-arg", fmt.Sprintf("GOMAXPROCS=%d", prediction.CPU.ParallelismLevel))

	// Add progress output
	parts = append(parts, "--progress=plain")

	// Add tags
	for _, tag := range tags {
		parts = append(parts, "-t", tag)
	}

	// Add Dockerfile path
	if dockerfilePath != "" && dockerfilePath != "Dockerfile" {
		parts = append(parts, "-f", dockerfilePath)
	}

	// Add context path
	parts = append(parts, contextPath)

	return strings.Join(parts, " ")
}

// GenerateBuildkitConfig creates a buildkit configuration for the build
func (o *BuildOptimizer) GenerateBuildkitConfig(prediction *ResourcePrediction) string {
	var config strings.Builder

	config.WriteString("# syntax=docker/dockerfile:1\n")
	config.WriteString("# Buildkit optimization configuration\n\n")

	// Add cache mount examples based on language
	if len(prediction.Cache.MountCaches) > 0 {
		config.WriteString("# Recommended cache mounts:\n")
		for _, mount := range prediction.Cache.MountCaches {
			config.WriteString(fmt.Sprintf("# RUN --mount=%s \\\n", o.formatMountString(mount)))
		}
		config.WriteString("\n")
	}

	// Add parallelism hints
	if prediction.CPU.ParallelismLevel > 1 {
		config.WriteString(fmt.Sprintf("# Build with parallelism: %d\n", prediction.CPU.ParallelismLevel))
		config.WriteString("# ENV MAKEFLAGS=-j${PARALLELISM:-4}\n\n")
	}

	return config.String()
}

// formatMountString formats a cache mount for Dockerfile RUN command
func (o *BuildOptimizer) formatMountString(mount CacheMount) string {
	parts := []string{
		fmt.Sprintf("type=%s", mount.Type),
		fmt.Sprintf("target=%s", mount.Target),
	}

	if mount.ID != "" {
		parts = append(parts, fmt.Sprintf("id=%s", mount.ID))
	}
	if mount.Sharing != "" {
		parts = append(parts, fmt.Sprintf("sharing=%s", mount.Sharing))
	}
	if mount.ReadOnly {
		parts = append(parts, "ro")
	}

	return strings.Join(parts, ",")
}

// MonitorBuildPerformance monitors actual build performance for learning
func (o *BuildOptimizer) MonitorBuildPerformance(
	ctx context.Context,
	buildID string,
	profile BuildProfile,
	startTime time.Time,
) *BuildRecord {

	duration := time.Since(startTime)

	// Create build record (in production, would collect actual metrics)
	record := &BuildRecord{
		ID:        buildID,
		Profile:   profile,
		Duration:  duration,
		Success:   true, // Would be determined by build result
		Timestamp: startTime,
		Resources: ResourceUsage{
			// These would be collected from Docker stats API
			PeakCPU:      75.0, // Placeholder
			PeakMemoryMB: 1024, // Placeholder
			DiskIOMB:     500,  // Placeholder
			NetworkMB:    200,  // Placeholder
		},
		CacheHitRate: 0.8, // Would be calculated from build output
	}

	// Record for future predictions
	o.predictor.historyStore.RecordBuild(record)

	o.logger.Info("Build performance recorded",
		"build_id", buildID,
		"duration", duration,
		"peak_cpu", record.Resources.PeakCPU,
		"peak_memory_mb", record.Resources.PeakMemoryMB)

	return record
}

// GetOptimizationSummary provides a summary of optimizations applied
func (o *BuildOptimizer) GetOptimizationSummary(prediction *ResourcePrediction) string {
	var summary strings.Builder

	summary.WriteString("🚀 Build Optimization Summary\n")
	summary.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Resource allocation
	summary.WriteString(fmt.Sprintf("📊 Resource Allocation:\n"))
	summary.WriteString(fmt.Sprintf("   • CPU Cores: %d\n", prediction.CPU.Cores))
	summary.WriteString(fmt.Sprintf("   • Memory: %d MB\n", prediction.Memory.RecommendedMB))
	summary.WriteString(fmt.Sprintf("   • Storage: ~%d MB\n", prediction.Storage.TotalRequiredMB))
	summary.WriteString(fmt.Sprintf("   • Estimated Time: %v\n\n", prediction.BuildTime.Round(time.Second)))

	// Cache optimization
	if len(prediction.Cache.MountCaches) > 0 {
		summary.WriteString("💾 Cache Optimization:\n")
		for _, mount := range prediction.Cache.MountCaches {
			summary.WriteString(fmt.Sprintf("   • %s → %s\n", mount.ID, mount.Target))
		}
		summary.WriteString("\n")
	}

	// Recommendations
	if len(prediction.Recommendations) > 0 {
		summary.WriteString("💡 Recommendations:\n")
		for _, rec := range prediction.Recommendations {
			summary.WriteString(fmt.Sprintf("   • %s\n", rec))
		}
		summary.WriteString("\n")
	}

	// Confidence
	summary.WriteString(fmt.Sprintf("📈 Prediction Confidence: %.0f%%\n", prediction.Confidence*100))

	if prediction.Reasoning != "" {
		summary.WriteString(fmt.Sprintf("\n📝 Reasoning: %s\n", prediction.Reasoning))
	}

	return summary.String()
}
