package build

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// BuildOptimizer provides intelligent build optimization strategies
type BuildOptimizer struct {
	logger           *slog.Logger
	cacheManager     *CacheManager
	layerOptimizer   *LayerOptimizer
	contextOptimizer *ContextOptimizer
}

// NewBuildOptimizer creates a new build optimizer
func NewBuildOptimizer(logger *slog.Logger) *BuildOptimizer {
	return &BuildOptimizer{
		logger:           logger.With("component", "build_optimizer"),
		cacheManager:     NewCacheManager(logger),
		layerOptimizer:   NewLayerOptimizer(logger),
		contextOptimizer: NewContextOptimizer(logger),
	}
}

// OptimizeBuild analyzes and optimizes the build process
func (o *BuildOptimizer) OptimizeBuild(ctx context.Context, dockerfilePath string, buildContext string) (*OptimizationResult, error) {
	o.logger.Info("Starting build optimization analysis",
		"dockerfile", dockerfilePath,
		"context", buildContext)

	result := &OptimizationResult{
		Recommendations: []OptimizationRecommendation{},
		CacheStrategy:   CacheStrategy{},
		LayerStrategy:   LayerStrategy{},
		ContextStrategy: ContextStrategy{},
	}

	// Analyze Dockerfile for optimization opportunities
	dockerfileOptimizations, err := o.analyzeDockerfile(dockerfilePath)
	if err != nil {
		o.logger.Warn("Failed to analyze Dockerfile", "error", err)
	} else {
		result.Recommendations = append(result.Recommendations, dockerfileOptimizations...)
	}

	// Analyze build context
	contextOptimizations, err := o.contextOptimizer.Analyze(buildContext)
	if err != nil {
		o.logger.Warn("Failed to analyze build context", "error", err)
	} else {
		result.ContextStrategy = contextOptimizations
		result.Recommendations = append(result.Recommendations, contextOptimizations.Recommendations...)
	}

	// Generate cache strategy
	result.CacheStrategy = o.cacheManager.GenerateStrategy(dockerfilePath, buildContext)

	// Generate layer optimization strategy
	result.LayerStrategy = o.layerOptimizer.GenerateStrategy(dockerfilePath)

	// Calculate potential improvements
	result.EstimatedImprovements = o.calculateImprovements(result)

	return result, nil
}

// analyzeDockerfile analyzes Dockerfile for optimization opportunities
func (o *BuildOptimizer) analyzeDockerfile(dockerfilePath string) ([]OptimizationRecommendation, error) {
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return nil, errors.NewError().Message("failed to read Dockerfile").Cause(err).WithLocation().Build()
	}

	recommendations := []OptimizationRecommendation{}
	lines := strings.Split(string(content), "\n")

	// Check for inefficient layer ordering
	if rec := o.checkLayerOrdering(lines); rec != nil {
		recommendations = append(recommendations, *rec)
	}

	// Check for cache-busting commands
	if rec := o.checkCacheBusting(lines); rec != nil {
		recommendations = append(recommendations, *rec)
	}

	// Check for multi-stage optimization opportunities
	if rec := o.checkMultiStageOpportunities(lines); rec != nil {
		recommendations = append(recommendations, *rec)
	}

	// Check for package manager optimizations
	if rec := o.checkPackageManagerOptimizations(lines); rec != nil {
		recommendations = append(recommendations, *rec)
	}

	return recommendations, nil
}

// checkLayerOrdering checks if layers are ordered for optimal caching
func (o *BuildOptimizer) checkLayerOrdering(lines []string) *OptimizationRecommendation {
	copyAllIndex := -1
	installIndex := -1
	hasPackageManager := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmedUpper := strings.ToUpper(trimmed)

		// Check for COPY . . or COPY . /path
		if strings.HasPrefix(trimmedUpper, "COPY") && (strings.Contains(trimmed, " . ") || strings.HasSuffix(trimmed, " .")) {
			copyAllIndex = i
		}

		// Check for package manager install commands
		if strings.HasPrefix(trimmedUpper, "RUN") &&
			(strings.Contains(line, "npm install") ||
				strings.Contains(line, "npm ci") ||
				strings.Contains(line, "yarn install") ||
				strings.Contains(line, "pip install") ||
				strings.Contains(line, "go mod download") ||
				strings.Contains(line, "bundle install")) {
			installIndex = i
			hasPackageManager = true
		}
	}

	// If we found COPY . . before package installation, that's inefficient
	if hasPackageManager && copyAllIndex != -1 && installIndex != -1 && copyAllIndex < installIndex {
		return &OptimizationRecommendation{
			Type:        "layer_ordering",
			Priority:    "high",
			Title:       "Suboptimal layer ordering detected",
			Description: "Source code is copied before dependency installation",
			Impact:      "Major impact on build time - dependencies reinstalled on every code change",
			Solution:    "Move COPY commands for source code after dependency installation",
			Example: `# Better ordering:
COPY package*.json ./
RUN npm ci --only=production
COPY . .`,
		}
	}

	return nil
}

// checkCacheBusting checks for commands that unnecessarily bust the cache
func (o *BuildOptimizer) checkCacheBusting(lines []string) *OptimizationRecommendation {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for ADD with remote URLs
		if strings.HasPrefix(strings.ToUpper(trimmed), "ADD") && strings.Contains(trimmed, "http") {
			return &OptimizationRecommendation{
				Type:        "cache_busting",
				Priority:    "medium",
				Title:       "ADD with remote URL prevents caching",
				Description: fmt.Sprintf("Line %d: ADD with remote URL always invalidates cache", i+1),
				Impact:      "Moderate impact - layer and all subsequent layers rebuilt",
				Solution:    "Use RUN with curl/wget and cache the download",
				Example:     "RUN curl -fsSL https://example.com/file -o file && echo 'checksum  file' | sha256sum -c",
			}
		}

		// Check for non-deterministic commands
		if strings.Contains(trimmed, "apt-get update") && !strings.Contains(trimmed, "&&") {
			return &OptimizationRecommendation{
				Type:        "cache_busting",
				Priority:    "medium",
				Title:       "Separate apt-get update prevents effective caching",
				Description: fmt.Sprintf("Line %d: apt-get update in separate RUN instruction", i+1),
				Impact:      "Moderate impact - package cache may be stale",
				Solution:    "Combine apt-get update with install in same RUN instruction",
				Example:     "RUN apt-get update && apt-get install -y package && rm -rf /var/lib/apt/lists/*",
			}
		}
	}

	return nil
}

// checkMultiStageOpportunities checks if multi-stage build would help
func (o *BuildOptimizer) checkMultiStageOpportunities(lines []string) *OptimizationRecommendation {
	hasBuildTools := false
	notUsingMultiStage := false
	buildCommands := 0

	for _, line := range lines {
		if strings.Contains(line, "gcc") || strings.Contains(line, "make") || strings.Contains(line, "build-essential") ||
			strings.Contains(line, "g++") || strings.Contains(line, "python-dev") || strings.Contains(line, "node-gyp") {
			hasBuildTools = true
		}
		if strings.Contains(line, "npm run build") || strings.Contains(line, "go build") ||
			strings.Contains(line, "cargo build") || strings.Contains(line, "make build") ||
			strings.Contains(line, "mvn package") || strings.Contains(line, "gradle build") {
			buildCommands++
		}
		if strings.Contains(line, "FROM") && !strings.Contains(line, " AS ") {
			notUsingMultiStage = true
		}
	}

	if hasBuildTools && notUsingMultiStage && buildCommands > 0 {
		return &OptimizationRecommendation{
			Type:        "multi_stage",
			Priority:    "high",
			Title:       "Multi-stage build opportunity detected",
			Description: "Build tools present in final image, increasing size",
			Impact:      "Major impact on image size - could reduce by 50-80%",
			Solution:    "Use multi-stage build to separate build and runtime",
			Example: `# Build stage
FROM node:16 AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Runtime stage
FROM node:16-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
CMD ["node", "dist/index.js"]`,
		}
	}

	return nil
}

// checkPackageManagerOptimizations checks for package manager specific optimizations
func (o *BuildOptimizer) checkPackageManagerOptimizations(lines []string) *OptimizationRecommendation {
	for _, line := range lines {
		// Check for npm install instead of npm ci
		if strings.Contains(line, "npm install") && !strings.Contains(line, "npm ci") {
			return &OptimizationRecommendation{
				Type:        "package_manager",
				Priority:    "medium",
				Title:       "Use npm ci instead of npm install",
				Description: "npm install is slower and less deterministic than npm ci",
				Impact:      "Moderate impact on build time and reproducibility",
				Solution:    "Replace 'npm install' with 'npm ci' for faster, more reliable builds",
				Example:     "RUN npm ci --only=production",
			}
		}

		// Check for missing --no-cache-dir with pip
		if strings.Contains(line, "pip install") && !strings.Contains(line, "--no-cache-dir") {
			return &OptimizationRecommendation{
				Type:        "package_manager",
				Priority:    "low",
				Title:       "pip install without --no-cache-dir",
				Description: "pip caches packages in the image, increasing size",
				Impact:      "Minor impact on image size",
				Solution:    "Add --no-cache-dir flag to pip install",
				Example:     "RUN pip install --no-cache-dir -r requirements.txt",
			}
		}
	}

	return nil
}

// calculateImprovements estimates potential improvements
func (o *BuildOptimizer) calculateImprovements(result *OptimizationResult) EstimatedImprovements {
	improvements := EstimatedImprovements{}

	// Estimate build time improvement
	for _, rec := range result.Recommendations {
		switch rec.Priority {
		case "high":
			improvements.BuildTimeReduction += 30
		case "medium":
			improvements.BuildTimeReduction += 15
		case "low":
			improvements.BuildTimeReduction += 5
		}
	}

	// Cap at realistic maximum
	if improvements.BuildTimeReduction > 70 {
		improvements.BuildTimeReduction = 70
	}

	// Estimate size reduction
	if result.ContextStrategy.EstimatedSizeReduction > 0 {
		improvements.ImageSizeReduction = result.ContextStrategy.EstimatedSizeReduction
	}

	// Estimate cache efficiency
	improvements.CacheHitRateIncrease = len(result.Recommendations) * 10
	if improvements.CacheHitRateIncrease > 40 {
		improvements.CacheHitRateIncrease = 40
	}

	return improvements
}

// CacheManager handles Docker build cache optimization
type CacheManager struct {
	logger *slog.Logger
}

func NewCacheManager(logger *slog.Logger) *CacheManager {
	return &CacheManager{
		logger: logger.With("component", "cache_manager"),
	}
}

// GenerateStrategy generates an optimal cache strategy
func (m *CacheManager) GenerateStrategy(dockerfilePath string, buildContext string) CacheStrategy {
	strategy := CacheStrategy{
		CacheFrom:        []string{},
		CacheTo:          []string{},
		CacheMode:        "max",
		LayerCaching:     true,
		BuildKitFeatures: []string{},
	}

	// Recommend BuildKit features
	strategy.BuildKitFeatures = []string{
		"inline-cache",
		"registry-cache",
		"cache-mounts",
	}

	// Generate cache key based on dependencies
	strategy.CacheKey = m.generateCacheKey(dockerfilePath, buildContext)

	// Recommend cache sources
	strategy.CacheFrom = []string{
		"type=registry,ref=myregistry/myapp:buildcache",
		"type=local,src=/tmp/buildkit-cache",
	}

	// Recommend cache destinations
	strategy.CacheTo = []string{
		"type=registry,ref=myregistry/myapp:buildcache,mode=max",
		"type=local,dest=/tmp/buildkit-cache",
	}

	return strategy
}

// generateCacheKey generates a cache key based on dependencies
func (m *CacheManager) generateCacheKey(dockerfilePath string, buildContext string) string {
	h := sha256.New()

	// Hash Dockerfile
	if file, err := os.Open(dockerfilePath); err == nil {
		defer file.Close()
		io.Copy(h, file)
	}

	// Hash dependency files
	depFiles := []string{"package.json", "package-lock.json", "requirements.txt", "go.mod", "go.sum"}
	for _, depFile := range depFiles {
		path := filepath.Join(buildContext, depFile)
		if file, err := os.Open(path); err == nil {
			defer file.Close()
			io.Copy(h, file)
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

// LayerOptimizer optimizes Docker image layers
type LayerOptimizer struct {
	logger *slog.Logger
}

func NewLayerOptimizer(logger *slog.Logger) *LayerOptimizer {
	return &LayerOptimizer{
		logger: logger.With("component", "layer_optimizer"),
	}
}

// GenerateStrategy generates layer optimization strategy
func (o *LayerOptimizer) GenerateStrategy(dockerfilePath string) LayerStrategy {
	strategy := LayerStrategy{
		OptimalOrder:    []string{},
		CombineCommands: true,
		MinimizeLayers:  true,
		CleanupCommands: true,
	}

	// Recommend optimal layer order
	strategy.OptimalOrder = []string{
		"FROM base_image",
		"RUN install_system_dependencies",
		"COPY dependency_files",
		"RUN install_app_dependencies",
		"COPY source_code",
		"RUN build_application",
		"COPY configuration",
		"EXPOSE ports",
		"CMD start_application",
	}

	// Calculate squash points
	strategy.SquashPoints = []int{3, 6} // After dependencies and after build

	return strategy
}

// ContextOptimizer optimizes build context
type ContextOptimizer struct {
	logger *slog.Logger
}

func NewContextOptimizer(logger *slog.Logger) *ContextOptimizer {
	return &ContextOptimizer{
		logger: logger.With("component", "context_optimizer"),
	}
}

// Analyze analyzes build context for optimization
func (o *ContextOptimizer) Analyze(buildContext string) (ContextStrategy, error) {
	strategy := ContextStrategy{
		ExcludePatterns:        []string{},
		IncludeOnlyNeeded:      true,
		EstimatedSizeReduction: 0,
		Recommendations:        []OptimizationRecommendation{},
	}

	// Check for .dockerignore
	dockerignorePath := filepath.Join(buildContext, ".dockerignore")
	hasDockerignore := false
	if _, err := os.Stat(dockerignorePath); err == nil {
		hasDockerignore = true
	}

	// Analyze context size and contents
	totalSize := int64(0)
	unnecessarySize := int64(0)
	fileCount := 0
	largeFiles := []string{}

	err := filepath.Walk(buildContext, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			fileCount++
			totalSize += info.Size()

			// Check for unnecessary files
			relPath, _ := filepath.Rel(buildContext, path)
			if o.isUnnecessaryFile(relPath) {
				unnecessarySize += info.Size()
				strategy.ExcludePatterns = append(strategy.ExcludePatterns, relPath)
			}

			// Track large files
			if info.Size() > 10*1024*1024 { // 10MB
				largeFiles = append(largeFiles, fmt.Sprintf("%s (%.1f MB)", relPath, float64(info.Size())/(1024*1024)))
			}
		}

		return nil
	})

	if err != nil {
		return strategy, err
	}

	// Calculate potential size reduction
	if totalSize > 0 {
		strategy.EstimatedSizeReduction = int(unnecessarySize * 100 / totalSize)
	}

	// Generate recommendations
	if !hasDockerignore && fileCount > 100 {
		strategy.Recommendations = append(strategy.Recommendations, OptimizationRecommendation{
			Type:        "context_size",
			Priority:    "high",
			Title:       "Missing .dockerignore file",
			Description: fmt.Sprintf("Build context contains %d files (%.1f MB) without .dockerignore", fileCount, float64(totalSize)/(1024*1024)),
			Impact:      "Major impact on build time - all files sent to Docker daemon",
			Solution:    "Create .dockerignore file to exclude unnecessary files",
			Example: `.git
node_modules
*.log
.env
dist
coverage
.vscode`,
		})
	}

	if len(largeFiles) > 0 {
		strategy.Recommendations = append(strategy.Recommendations, OptimizationRecommendation{
			Type:        "context_size",
			Priority:    "medium",
			Title:       "Large files in build context",
			Description: fmt.Sprintf("Found %d large files: %s", len(largeFiles), strings.Join(largeFiles[:min(3, len(largeFiles))], ", ")),
			Impact:      "Moderate impact on build time",
			Solution:    "Exclude large files not needed for build or use .dockerignore",
		})
	}

	return strategy, nil
}

// isUnnecessaryFile checks if a file is typically unnecessary in build context
func (o *ContextOptimizer) isUnnecessaryFile(path string) bool {
	unnecessaryPatterns := []string{
		".git", ".svn", ".hg",
		"node_modules", "vendor", "__pycache__",
		".pytest_cache", ".coverage", "htmlcov",
		".log", ".tmp", ".temp",
		".DS_Store", "Thumbs.db",
		".env", ".env.local",
		"test", "tests", "spec",
		"docs", "documentation",
	}

	for _, pattern := range unnecessaryPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// Types for optimization results

// OptimizationResult contains build optimization analysis results
type OptimizationResult struct {
	Recommendations       []OptimizationRecommendation `json:"recommendations"`
	CacheStrategy         CacheStrategy                `json:"cache_strategy"`
	LayerStrategy         LayerStrategy                `json:"layer_strategy"`
	ContextStrategy       ContextStrategy              `json:"context_strategy"`
	EstimatedImprovements EstimatedImprovements        `json:"estimated_improvements"`
}

// OptimizationRecommendation represents a specific optimization recommendation
type OptimizationRecommendation struct {
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Solution    string `json:"solution"`
	Example     string `json:"example,omitempty"`
}

// CacheStrategy defines caching optimization strategy
type CacheStrategy struct {
	CacheFrom        []string `json:"cache_from"`
	CacheTo          []string `json:"cache_to"`
	CacheMode        string   `json:"cache_mode"`
	CacheKey         string   `json:"cache_key"`
	LayerCaching     bool     `json:"layer_caching"`
	BuildKitFeatures []string `json:"buildkit_features"`
}

// LayerStrategy defines layer optimization strategy
type LayerStrategy struct {
	OptimalOrder    []string `json:"optimal_order"`
	CombineCommands bool     `json:"combine_commands"`
	MinimizeLayers  bool     `json:"minimize_layers"`
	CleanupCommands bool     `json:"cleanup_commands"`
	SquashPoints    []int    `json:"squash_points"`
}

// ContextStrategy defines build context optimization strategy
type ContextStrategy struct {
	ExcludePatterns        []string                     `json:"exclude_patterns"`
	IncludeOnlyNeeded      bool                         `json:"include_only_needed"`
	EstimatedSizeReduction int                          `json:"estimated_size_reduction"`
	Recommendations        []OptimizationRecommendation `json:"recommendations"`
}

// EstimatedImprovements contains estimated improvement metrics
type EstimatedImprovements struct {
	BuildTimeReduction   int `json:"build_time_reduction_percent"`
	ImageSizeReduction   int `json:"image_size_reduction_percent"`
	CacheHitRateIncrease int `json:"cache_hit_rate_increase_percent"`
}

// Helper function
