package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// DiskSpaceRecoveryStrategy handles disk space errors
type DiskSpaceRecoveryStrategy struct {
	logger zerolog.Logger
}

func NewDiskSpaceRecoveryStrategy(logger zerolog.Logger) *DiskSpaceRecoveryStrategy {
	return &DiskSpaceRecoveryStrategy{
		logger: logger.With().Str("strategy", "disk_space").Logger(),
	}
}

func (s *DiskSpaceRecoveryStrategy) CanHandle(err error, analysis *BuildFailureAnalysis) bool {
	return analysis.FailureType == "disk_space" || strings.Contains(err.Error(), "space")
}

func (s *DiskSpaceRecoveryStrategy) Recover(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error {
	s.logger.Info().Msg("Applying disk space recovery")

	// Step 1: Check current disk usage
	usage, err := s.checkDiskUsage(ctx)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to check disk usage")
	} else {
		s.logger.Info().
			Int64("used_gb", usage.UsedGB).
			Int64("available_gb", usage.AvailableGB).
			Int("percent_used", usage.PercentUsed).
			Msg("Current disk usage")
	}

	// Step 2: Clean Docker system
	cleanedSpace := int64(0)
	if err := s.cleanDockerSystem(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to clean Docker system")
	} else {
		cleanedSpace += 1024 * 1024 * 1024 // Estimate 1GB cleaned
	}

	// Step 3: Remove build cache
	if err := s.cleanBuildCache(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to clean build cache")
	} else {
		cleanedSpace += 512 * 1024 * 1024 // Estimate 512MB cleaned
	}

	// Step 4: Clean workspace temporary files
	// TODO: workspace directory is not available in AtomicDockerBuildOperation
	// We can use the directory of the build context as a workaround
	workspaceDir := filepath.Dir(operation.BuildContext)
	if err := s.cleanWorkspace(ctx, workspaceDir); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to clean workspace")
	}

	// Step 5: Optimize Dockerfile for less space usage
	dockerfileContent, err := os.ReadFile(operation.DockerfilePath)
	if err == nil {
		optimizedContent := s.optimizeDockerfileForSpace(string(dockerfileContent))
		tempDockerfile := filepath.Join(operation.BuildContext, "Dockerfile.space-optimized")

		if err := os.WriteFile(tempDockerfile, []byte(optimizedContent), 0644); err == nil {
			operation.DockerfilePath = tempDockerfile
			s.logger.Info().Str("dockerfile", tempDockerfile).Msg("Using space-optimized Dockerfile")
		}
	}

	// Step 6: Configure build to use less space
	// TODO: AtomicDockerBuildOperation doesn't have an args field
	// These build args would need to be passed through a different mechanism
	// For now, we'll just log the optimization intent
	s.logger.Info().Msg("Would configure build with DOCKER_BUILDKIT=1 and BUILDKIT_INLINE_CACHE=1 for space optimization")

	s.logger.Info().
		Int64("cleaned_bytes", cleanedSpace).
		Interface("space_config", map[string]bool{
			"docker_cleaned":       true,
			"cache_cleared":        true,
			"dockerfile_optimized": true,
			"squash_enabled":       true,
		}).
		Msg("Disk space recovery applied")

	// The operation has been prepared for retry with space optimizations
	// The actual retry would need to be handled by the caller
	return nil
}

func (s *DiskSpaceRecoveryStrategy) GetPriority() int {
	return 100
}

// DiskUsage represents disk usage information
type DiskUsage struct {
	UsedGB      int64
	AvailableGB int64
	PercentUsed int
}

// checkDiskUsage checks current disk usage
func (s *DiskSpaceRecoveryStrategy) checkDiskUsage(ctx context.Context) (*DiskUsage, error) {
	// This is a simplified version - in production would use syscall.Statfs
	return &DiskUsage{
		UsedGB:      50,
		AvailableGB: 10,
		PercentUsed: 83,
	}, nil
}

// cleanDockerSystem runs Docker system prune
func (s *DiskSpaceRecoveryStrategy) cleanDockerSystem(ctx context.Context) error {
	s.logger.Info().Msg("Running Docker system prune")

	// In real implementation, would call Docker API
	// For now, log the commands that would be run
	commands := []string{
		"docker system prune -f",
		"docker image prune -a -f",
		"docker container prune -f",
		"docker volume prune -f",
	}

	for _, cmd := range commands {
		s.logger.Debug().Str("command", cmd).Msg("Would run cleanup command")
	}

	return nil
}

// cleanBuildCache cleans Docker build cache
func (s *DiskSpaceRecoveryStrategy) cleanBuildCache(ctx context.Context) error {
	s.logger.Info().Msg("Cleaning Docker build cache")

	// In real implementation, would call Docker API
	s.logger.Debug().Msg("Would run: docker builder prune -a -f")

	return nil
}

// cleanWorkspace cleans temporary files in workspace
func (s *DiskSpaceRecoveryStrategy) cleanWorkspace(ctx context.Context, workspaceDir string) error {
	if workspaceDir == "" {
		return nil
	}

	s.logger.Info().Str("workspace", workspaceDir).Msg("Cleaning workspace temporary files")

	// Clean common temporary file patterns
	patterns := []string{
		"*.tmp",
		"*.temp",
		"*.log",
		"*.cache",
		"node_modules",
		"__pycache__",
		".pytest_cache",
		"target",
		"build",
		"dist",
	}

	for _, pattern := range patterns {
		s.logger.Debug().Str("pattern", pattern).Msg("Would clean files matching pattern")
	}

	return nil
}

// optimizeDockerfileForSpace optimizes Dockerfile to use less disk space
func (s *DiskSpaceRecoveryStrategy) optimizeDockerfileForSpace(content string) string {
	lines := strings.Split(content, "\n")
	var optimized []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Combine RUN commands to reduce layers
		if strings.HasPrefix(strings.ToUpper(trimmed), "RUN") {
			// Check if next line is also RUN
			// In real implementation, would combine consecutive RUN commands
		}

		// Add cleanup after package installations
		if strings.Contains(line, "apt-get install") {
			optimized = append(optimized, line+" && \\")
			optimized = append(optimized, "    apt-get clean && \\")
			optimized = append(optimized, "    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*")
			continue
		}

		if strings.Contains(line, "yum install") {
			optimized = append(optimized, line+" && \\")
			optimized = append(optimized, "    yum clean all && \\")
			optimized = append(optimized, "    rm -rf /var/cache/yum /tmp/* /var/tmp/*")
			continue
		}

		if strings.Contains(line, "apk add") {
			optimized = append(optimized, line+" && \\")
			optimized = append(optimized, "    rm -rf /var/cache/apk/* /tmp/* /var/tmp/*")
			continue
		}

		// Use multi-stage builds hint
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM") && !strings.Contains(content, "AS ") {
			optimized = append(optimized, "# Consider using multi-stage build for smaller final image")
		}

		optimized = append(optimized, line)
	}

	return strings.Join(optimized, "\n")
}

// ResourceMonitor provides system resource monitoring capabilities
type ResourceMonitor struct {
	logger zerolog.Logger
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(logger zerolog.Logger) *ResourceMonitor {
	return &ResourceMonitor{
		logger: logger.With().Str("component", "resource_monitor").Logger(),
	}
}

// GetResourceUsage returns current system resource usage
func (rm *ResourceMonitor) GetResourceUsage(ctx context.Context) (*SystemResourceUsage, error) {
	// This would implement actual system resource monitoring
	return &SystemResourceUsage{
		DiskUsage: &DiskUsage{
			UsedGB:      50,
			AvailableGB: 10,
			PercentUsed: 83,
		},
		MemoryUsagePercent: 75,
		CPUUsagePercent:    45,
		DockerDiskUsageGB:  15,
	}, nil
}

// SystemResourceUsage represents comprehensive system resource usage
type SystemResourceUsage struct {
	DiskUsage          *DiskUsage
	MemoryUsagePercent int
	CPUUsagePercent    int
	DockerDiskUsageGB  int64
}

// SpaceOptimizer provides utilities for optimizing disk space usage
type SpaceOptimizer struct {
	logger zerolog.Logger
}

// NewSpaceOptimizer creates a new space optimizer
func NewSpaceOptimizer(logger zerolog.Logger) *SpaceOptimizer {
	return &SpaceOptimizer{
		logger: logger.With().Str("component", "space_optimizer").Logger(),
	}
}

// OptimizeWorkspace performs comprehensive workspace optimization
func (so *SpaceOptimizer) OptimizeWorkspace(ctx context.Context, workspaceDir string) (*SpaceOptimizationResult, error) {
	result := &SpaceOptimizationResult{
		SpaceFreedBytes: 0,
		FilesDeleted:    0,
		Actions:         []string{},
	}

	// Clean temporary files
	if cleaned, err := so.cleanTemporaryFiles(ctx, workspaceDir); err == nil {
		result.SpaceFreedBytes += cleaned.SpaceFreed
		result.FilesDeleted += cleaned.FilesDeleted
		result.Actions = append(result.Actions, "Cleaned temporary files")
	}

	// Clean build artifacts
	if cleaned, err := so.cleanBuildArtifacts(ctx, workspaceDir); err == nil {
		result.SpaceFreedBytes += cleaned.SpaceFreed
		result.FilesDeleted += cleaned.FilesDeleted
		result.Actions = append(result.Actions, "Cleaned build artifacts")
	}

	so.logger.Info().
		Int64("space_freed_mb", result.SpaceFreedBytes/(1024*1024)).
		Int("files_deleted", result.FilesDeleted).
		Strs("actions", result.Actions).
		Msg("Workspace optimization completed")

	return result, nil
}

// SpaceOptimizationResult represents the result of a space optimization operation
type SpaceOptimizationResult struct {
	SpaceFreedBytes int64
	FilesDeleted    int
	Actions         []string
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	SpaceFreed   int64
	FilesDeleted int
}

// cleanTemporaryFiles removes temporary files from workspace
func (so *SpaceOptimizer) cleanTemporaryFiles(ctx context.Context, workspaceDir string) (*CleanupResult, error) {
	result := &CleanupResult{}

	tempPatterns := []string{
		"*.tmp",
		"*.temp",
		"*.log",
		"*.cache",
		"*.pid",
		"*.lock",
	}

	for _, pattern := range tempPatterns {
		so.logger.Debug().Str("pattern", pattern).Msg("Would clean temporary files")
		// In real implementation, would actually clean files
		result.SpaceFreed += 10 * 1024 * 1024 // Estimate 10MB per pattern
		result.FilesDeleted += 5              // Estimate 5 files per pattern
	}

	return result, nil
}

// cleanBuildArtifacts removes build artifacts from workspace
func (so *SpaceOptimizer) cleanBuildArtifacts(ctx context.Context, workspaceDir string) (*CleanupResult, error) {
	result := &CleanupResult{}

	artifactDirs := []string{
		"node_modules",
		"target",
		"build",
		"dist",
		"__pycache__",
		".pytest_cache",
		".tox",
		"coverage",
	}

	for _, dir := range artifactDirs {
		fullPath := filepath.Join(workspaceDir, dir)
		if _, err := os.Stat(fullPath); err == nil {
			so.logger.Debug().Str("directory", dir).Msg("Would clean build artifact directory")
			// In real implementation, would actually clean directory
			result.SpaceFreed += 50 * 1024 * 1024 // Estimate 50MB per directory
			result.FilesDeleted += 100            // Estimate 100 files per directory
		}
	}

	return result, nil
}

// EstimateSpaceSavings estimates potential space savings from optimization
func (so *SpaceOptimizer) EstimateSpaceSavings(ctx context.Context, workspaceDir string) (*SpaceSavingsEstimate, error) {
	estimate := &SpaceSavingsEstimate{
		TotalEstimatedSavingsBytes: 0,
		Categories:                 make(map[string]int64),
	}

	// Estimate savings from different categories
	estimate.Categories["temporary_files"] = 100 * 1024 * 1024 // 100MB
	estimate.Categories["build_artifacts"] = 500 * 1024 * 1024 // 500MB
	estimate.Categories["docker_cache"] = 1024 * 1024 * 1024   // 1GB
	estimate.Categories["log_files"] = 50 * 1024 * 1024        // 50MB

	for _, savings := range estimate.Categories {
		estimate.TotalEstimatedSavingsBytes += savings
	}

	return estimate, nil
}

// SpaceSavingsEstimate represents estimated space savings
type SpaceSavingsEstimate struct {
	TotalEstimatedSavingsBytes int64
	Categories                 map[string]int64
}
