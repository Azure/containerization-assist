// Package ml provides AI-powered resource prediction for Container Kit builds.
package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// RepositoryAnalysis defines the interface for repository analysis results
type RepositoryAnalysis interface {
	GetLanguage() string
	GetFramework() string
	GetDependencies() []string
	GetBuildCommand() string
	GetStartCommand() string
}

// SimpleAnalysis provides a basic implementation of RepositoryAnalysis
type SimpleAnalysis struct {
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	Dependencies []string `json:"dependencies"`
	BuildCommand string   `json:"build_command"`
	StartCommand string   `json:"start_command"`
}

func (s *SimpleAnalysis) GetLanguage() string       { return s.Language }
func (s *SimpleAnalysis) GetFramework() string      { return s.Framework }
func (s *SimpleAnalysis) GetDependencies() []string { return s.Dependencies }
func (s *SimpleAnalysis) GetBuildCommand() string   { return s.BuildCommand }
func (s *SimpleAnalysis) GetStartCommand() string   { return s.StartCommand }

// ResourcePrediction represents predicted resource requirements for a build
type ResourcePrediction struct {
	CPU             CPURequirements     `json:"cpu"`
	Memory          MemoryRequirements  `json:"memory"`
	Storage         StorageRequirements `json:"storage"`
	Cache           CacheStrategy       `json:"cache"`
	BuildTime       time.Duration       `json:"estimated_build_time"`
	Confidence      float64             `json:"confidence"`
	Reasoning       string              `json:"reasoning"`
	Recommendations []string            `json:"recommendations"`
}

// CPURequirements represents CPU resource predictions
type CPURequirements struct {
	Cores            int    `json:"cores"`
	Architecture     string `json:"architecture"`
	ParallelismLevel int    `json:"parallelism_level"`
	CPUShares        int    `json:"cpu_shares"`
	CPUQuota         int    `json:"cpu_quota"`
}

// MemoryRequirements represents memory resource predictions
type MemoryRequirements struct {
	MinMemoryMB      int `json:"min_memory_mb"`
	RecommendedMB    int `json:"recommended_mb"`
	MaxMemoryMB      int `json:"max_memory_mb"`
	SwapMB           int `json:"swap_mb"`
	MemorySwappiness int `json:"memory_swappiness"`
}

// StorageRequirements represents storage resource predictions
type StorageRequirements struct {
	BuildContextMB  int `json:"build_context_mb"`
	ImageSizeMB     int `json:"image_size_mb"`
	LayerCacheMB    int `json:"layer_cache_mb"`
	TotalRequiredMB int `json:"total_required_mb"`
	IOPSRequired    int `json:"iops_required"`
}

// CacheStrategy represents caching recommendations
type CacheStrategy struct {
	UseCache       bool         `json:"use_cache"`
	CacheFrom      []string     `json:"cache_from"`
	CacheTo        []string     `json:"cache_to"`
	InlineCache    bool         `json:"inline_cache"`
	MountCaches    []CacheMount `json:"mount_caches"`
	InvalidateKeys []string     `json:"invalidate_keys"`
}

// CacheMount represents a cache mount configuration
type CacheMount struct {
	Target   string `json:"target"`
	Type     string `json:"type"`
	Sharing  string `json:"sharing"`
	ID       string `json:"id"`
	ReadOnly bool   `json:"readonly"`
}

// BuildProfile represents characteristics that affect resource requirements
type BuildProfile struct {
	Language         string  `json:"language"`
	Framework        string  `json:"framework"`
	BuildSystem      string  `json:"build_system"`
	Dependencies     int     `json:"dependency_count"`
	CodeSizeMB       float64 `json:"code_size_mb"`
	AssetSizeMB      float64 `json:"asset_size_mb"`
	TestSuite        bool    `json:"has_test_suite"`
	NativeExtensions bool    `json:"has_native_extensions"`
	Containerized    bool    `json:"already_containerized"`
	MultiStage       bool    `json:"multi_stage_build"`
	BuildSteps       int     `json:"build_step_count"`
}

// ResourcePredictor provides AI-powered resource prediction for builds
type ResourcePredictor struct {
	samplingClient domainsampling.Sampler
	historyStore   *BuildHistoryStore
	logger         *slog.Logger
}

// NewResourcePredictor creates a new resource predictor
func NewResourcePredictor(samplingClient domainsampling.Sampler, logger *slog.Logger) *ResourcePredictor {
	return &ResourcePredictor{
		samplingClient: samplingClient,
		historyStore:   NewBuildHistoryStore(),
		logger:         logger.With("component", "resource_predictor"),
	}
}

// PredictResources predicts optimal build resources based on repository analysis
func (p *ResourcePredictor) PredictResources(ctx context.Context, analysis RepositoryAnalysis) (*ResourcePrediction, error) {
	p.logger.Info("Predicting build resources",
		"language", analysis.GetLanguage(),
		"framework", analysis.GetFramework(),
		"dependencies", len(analysis.GetDependencies()))

	// Build profile from analysis
	profile := p.buildProfile(analysis)

	// Get AI-powered prediction
	prediction, err := p.getAIPrediction(ctx, profile)
	if err != nil {
		p.logger.Error("AI prediction failed, using fallback", "error", err)
		prediction = p.fallbackPrediction(profile)
	}

	// Enhance with historical data
	p.enhanceWithHistory(prediction, profile)

	// Validate and adjust predictions
	p.validatePrediction(prediction)

	p.logger.Info("Resource prediction completed",
		"cpu_cores", prediction.CPU.Cores,
		"memory_mb", prediction.Memory.RecommendedMB,
		"confidence", prediction.Confidence)

	return prediction, nil
}

// buildProfile creates a build profile from repository analysis
func (p *ResourcePredictor) buildProfile(analysis RepositoryAnalysis) BuildProfile {
	profile := BuildProfile{
		Language:     analysis.GetLanguage(),
		Framework:    analysis.GetFramework(),
		Dependencies: len(analysis.GetDependencies()),
	}

	// Detect build system
	profile.BuildSystem = p.detectBuildSystem(analysis)

	// Estimate sizes (would be enhanced with actual file analysis)
	profile.CodeSizeMB = p.estimateCodeSize(analysis)
	profile.AssetSizeMB = p.estimateAssetSize(analysis)

	// Detect characteristics
	profile.TestSuite = p.hasTestSuite(analysis)
	profile.NativeExtensions = p.hasNativeExtensions(analysis)
	profile.MultiStage = p.shouldUseMultiStage(analysis)
	profile.BuildSteps = p.estimateBuildSteps(analysis)

	return profile
}

// getAIPrediction gets resource prediction from AI
func (p *ResourcePredictor) getAIPrediction(ctx context.Context, profile BuildProfile) (*ResourcePrediction, error) {
	prompt := p.buildPredictionPrompt(profile)

	response, err := p.samplingClient.Sample(ctx, domainsampling.Request{
		Prompt:      prompt,
		Temperature: 0.2, // Lower temperature for more consistent predictions
		MaxTokens:   1500,
	})

	if err != nil {
		return nil, fmt.Errorf("AI sampling failed: %w", err)
	}

	return p.parsePredictionResponse(response.Content, profile)
}

// buildPredictionPrompt creates a detailed prompt for resource prediction
func (p *ResourcePredictor) buildPredictionPrompt(profile BuildProfile) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert in optimizing Docker build resources. Analyze this project and predict optimal build resources.\n\n")

	// Project profile
	prompt.WriteString("PROJECT PROFILE:\n")
	prompt.WriteString(fmt.Sprintf("- Language: %s\n", profile.Language))
	prompt.WriteString(fmt.Sprintf("- Framework: %s\n", profile.Framework))
	prompt.WriteString(fmt.Sprintf("- Build System: %s\n", profile.BuildSystem))
	prompt.WriteString(fmt.Sprintf("- Dependencies: %d\n", profile.Dependencies))
	prompt.WriteString(fmt.Sprintf("- Code Size: %.2f MB\n", profile.CodeSizeMB))
	prompt.WriteString(fmt.Sprintf("- Asset Size: %.2f MB\n", profile.AssetSizeMB))
	prompt.WriteString(fmt.Sprintf("- Has Tests: %v\n", profile.TestSuite))
	prompt.WriteString(fmt.Sprintf("- Native Extensions: %v\n", profile.NativeExtensions))
	prompt.WriteString(fmt.Sprintf("- Multi-Stage Build: %v\n", profile.MultiStage))
	prompt.WriteString(fmt.Sprintf("- Estimated Build Steps: %d\n", profile.BuildSteps))

	// Request structured response
	prompt.WriteString(`
PROVIDE A JSON RESPONSE WITH THIS STRUCTURE:
{
  "cpu": {
    "cores": 2-8,
    "architecture": "amd64|arm64",
    "parallelism_level": 1-16,
    "cpu_shares": 1024,
    "cpu_quota": 100000
  },
  "memory": {
    "min_memory_mb": 512-4096,
    "recommended_mb": 1024-8192,
    "max_memory_mb": 2048-16384,
    "swap_mb": 0-4096,
    "memory_swappiness": 0-100
  },
  "storage": {
    "build_context_mb": estimated,
    "image_size_mb": estimated,
    "layer_cache_mb": estimated,
    "total_required_mb": total,
    "iops_required": 100-10000
  },
  "cache": {
    "use_cache": true/false,
    "cache_from": ["registry/image:tag"],
    "cache_to": ["type=local,dest=path"],
    "inline_cache": true/false,
    "mount_caches": [
      {
        "target": "/path/to/cache",
        "type": "cache",
        "sharing": "shared|private|locked",
        "id": "cache-id"
      }
    ]
  },
  "estimated_build_time_seconds": 60-3600,
  "confidence": 0.0-1.0,
  "reasoning": "explanation of predictions",
  "recommendations": [
    "specific optimization tips"
  ]
}

OPTIMIZATION GUIDELINES:
- Balance resource allocation with build speed
- Consider dependency installation requirements
- Account for compilation memory spikes
- Optimize cache strategy for the technology stack
- Suggest mount caches for package managers
- Consider multi-stage build benefits
`)

	return prompt.String()
}

// parsePredictionResponse parses the AI response into ResourcePrediction
func (p *ResourcePredictor) parsePredictionResponse(response string, profile BuildProfile) (*ResourcePrediction, error) {
	// Extract JSON from response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}") + 1

	if start == -1 || end <= start {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := response[start:end]

	// Parse into temporary structure
	var rawPrediction struct {
		CPU                       CPURequirements     `json:"cpu"`
		Memory                    MemoryRequirements  `json:"memory"`
		Storage                   StorageRequirements `json:"storage"`
		Cache                     CacheStrategy       `json:"cache"`
		EstimatedBuildTimeSeconds int                 `json:"estimated_build_time_seconds"`
		Confidence                float64             `json:"confidence"`
		Reasoning                 string              `json:"reasoning"`
		Recommendations           []string            `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawPrediction); err != nil {
		return nil, fmt.Errorf("failed to parse prediction JSON: %w", err)
	}

	// Convert to final structure
	prediction := &ResourcePrediction{
		CPU:             rawPrediction.CPU,
		Memory:          rawPrediction.Memory,
		Storage:         rawPrediction.Storage,
		Cache:           rawPrediction.Cache,
		BuildTime:       time.Duration(rawPrediction.EstimatedBuildTimeSeconds) * time.Second,
		Confidence:      rawPrediction.Confidence,
		Reasoning:       rawPrediction.Reasoning,
		Recommendations: rawPrediction.Recommendations,
	}

	return prediction, nil
}

// fallbackPrediction provides rule-based prediction when AI fails
func (p *ResourcePredictor) fallbackPrediction(profile BuildProfile) *ResourcePrediction {
	prediction := &ResourcePrediction{
		Confidence: 0.5,
		Reasoning:  "Rule-based prediction (AI unavailable)",
	}

	// CPU prediction based on language and dependencies
	prediction.CPU = p.predictCPUFallback(profile)

	// Memory prediction based on build complexity
	prediction.Memory = p.predictMemoryFallback(profile)

	// Storage prediction based on sizes
	prediction.Storage = p.predictStorageFallback(profile)

	// Cache strategy based on technology
	prediction.Cache = p.predictCacheFallback(profile)

	// Build time estimation
	prediction.BuildTime = p.estimateBuildTime(profile)

	// General recommendations
	prediction.Recommendations = p.getGeneralRecommendations(profile)

	return prediction
}

// predictCPUFallback provides CPU prediction based on rules
func (p *ResourcePredictor) predictCPUFallback(profile BuildProfile) CPURequirements {
	cpu := CPURequirements{
		Architecture: "amd64",
		CPUShares:    1024,
		CPUQuota:     100000,
	}

	// Base CPU cores on language and build system
	switch profile.Language {
	case "go", "rust":
		cpu.Cores = 4 // Parallel compilation benefits
		cpu.ParallelismLevel = 8
	case "java", "scala":
		cpu.Cores = 4 // JVM and build tools need resources
		cpu.ParallelismLevel = 4
	case "node", "javascript", "typescript":
		cpu.Cores = 2 // Less CPU intensive
		cpu.ParallelismLevel = 4
	case "python":
		if profile.NativeExtensions {
			cpu.Cores = 4 // Compiling C extensions
			cpu.ParallelismLevel = 4
		} else {
			cpu.Cores = 2
			cpu.ParallelismLevel = 2
		}
	default:
		cpu.Cores = 2
		cpu.ParallelismLevel = 2
	}

	// Adjust for dependency count
	if profile.Dependencies > 100 {
		cpu.Cores = int(math.Min(float64(cpu.Cores+2), 8))
	}

	return cpu
}

// predictMemoryFallback provides memory prediction based on rules
func (p *ResourcePredictor) predictMemoryFallback(profile BuildProfile) MemoryRequirements {
	mem := MemoryRequirements{
		SwapMB:           1024,
		MemorySwappiness: 10,
	}

	// Base memory on language and framework
	baseMemory := 1024 // 1GB default

	switch profile.Language {
	case "java", "scala":
		baseMemory = 2048 // JVM needs more memory
	case "node", "javascript", "typescript":
		if strings.Contains(profile.Framework, "next") || strings.Contains(profile.Framework, "gatsby") {
			baseMemory = 2048 // Heavy build tools
		}
	case "rust":
		baseMemory = 2048 // Rust compiler is memory intensive
	case "go":
		baseMemory = 1024 // Go is efficient
	case "python":
		if profile.NativeExtensions {
			baseMemory = 1536
		}
	}

	// Adjust for dependencies
	extraMemory := (profile.Dependencies / 50) * 256

	mem.MinMemoryMB = baseMemory
	mem.RecommendedMB = baseMemory + extraMemory
	mem.MaxMemoryMB = mem.RecommendedMB * 2

	return mem
}

// predictStorageFallback provides storage prediction based on rules
func (p *ResourcePredictor) predictStorageFallback(profile BuildProfile) StorageRequirements {
	storage := StorageRequirements{
		BuildContextMB: int(profile.CodeSizeMB + profile.AssetSizeMB),
	}

	// Estimate layer cache based on dependencies
	storage.LayerCacheMB = profile.Dependencies * 5 // Rough estimate: 5MB per dependency

	// Estimate final image size
	baseImageSize := 100 // Base image size
	switch profile.Language {
	case "java":
		baseImageSize = 200 // JDK images are larger
	case "python":
		baseImageSize = 150
	case "node":
		baseImageSize = 130
	case "go":
		if profile.MultiStage {
			baseImageSize = 20 // Distroless final image
		} else {
			baseImageSize = 300 // Full Go image
		}
	}

	storage.ImageSizeMB = baseImageSize + storage.BuildContextMB + (profile.Dependencies * 2)
	storage.TotalRequiredMB = storage.BuildContextMB + storage.ImageSizeMB + storage.LayerCacheMB
	storage.IOPSRequired = 1000 // Standard IOPS requirement

	return storage
}

// predictCacheFallback provides cache strategy based on technology
func (p *ResourcePredictor) predictCacheFallback(profile BuildProfile) CacheStrategy {
	cache := CacheStrategy{
		UseCache:    true,
		InlineCache: true,
		CacheFrom:   []string{},
		CacheTo:     []string{"type=inline"},
		MountCaches: []CacheMount{},
	}

	// Add mount caches based on language
	switch profile.Language {
	case "go":
		cache.MountCaches = append(cache.MountCaches, CacheMount{
			Target:  "/go/pkg/mod",
			Type:    "cache",
			Sharing: "shared",
			ID:      "go-mod-cache",
		})
	case "node", "javascript", "typescript":
		cache.MountCaches = append(cache.MountCaches, CacheMount{
			Target:  "/root/.npm",
			Type:    "cache",
			Sharing: "shared",
			ID:      "npm-cache",
		})
		// Add yarn cache if using yarn
		if strings.Contains(profile.BuildSystem, "yarn") {
			cache.MountCaches = append(cache.MountCaches, CacheMount{
				Target:  "/usr/local/share/.cache/yarn",
				Type:    "cache",
				Sharing: "shared",
				ID:      "yarn-cache",
			})
		}
	case "python":
		cache.MountCaches = append(cache.MountCaches, CacheMount{
			Target:  "/root/.cache/pip",
			Type:    "cache",
			Sharing: "shared",
			ID:      "pip-cache",
		})
	case "java":
		cache.MountCaches = append(cache.MountCaches, CacheMount{
			Target:  "/root/.m2",
			Type:    "cache",
			Sharing: "shared",
			ID:      "maven-cache",
		})
		if strings.Contains(profile.BuildSystem, "gradle") {
			cache.MountCaches = append(cache.MountCaches, CacheMount{
				Target:  "/root/.gradle",
				Type:    "cache",
				Sharing: "shared",
				ID:      "gradle-cache",
			})
		}
	case "rust":
		cache.MountCaches = append(cache.MountCaches,
			CacheMount{
				Target:  "/usr/local/cargo/registry",
				Type:    "cache",
				Sharing: "shared",
				ID:      "cargo-registry",
			},
			CacheMount{
				Target:  "/usr/local/cargo/git",
				Type:    "cache",
				Sharing: "shared",
				ID:      "cargo-git",
			},
		)
	}

	return cache
}

// Helper methods

func (p *ResourcePredictor) detectBuildSystem(analysis RepositoryAnalysis) string {
	// Map common patterns
	buildSystems := map[string][]string{
		"go":     {"go mod", "go build"},
		"node":   {"npm", "yarn", "pnpm"},
		"python": {"pip", "poetry", "pipenv"},
		"java":   {"maven", "gradle"},
		"rust":   {"cargo"},
		"ruby":   {"bundler", "rake"},
		"php":    {"composer"},
		"dotnet": {"dotnet", "msbuild"},
	}

	if systems, ok := buildSystems[analysis.GetLanguage()]; ok && len(systems) > 0 {
		return systems[0] // Return first/most common
	}

	return "make" // Generic fallback
}

func (p *ResourcePredictor) estimateCodeSize(analysis RepositoryAnalysis) float64 {
	// Base estimate on language and dependencies
	baseSize := 10.0 // 10MB base

	// Add size based on dependencies
	depSize := float64(len(analysis.GetDependencies())) * 0.5

	return baseSize + depSize
}

func (p *ResourcePredictor) estimateAssetSize(analysis RepositoryAnalysis) float64 {
	// Estimate based on framework
	framework := analysis.GetFramework()
	if strings.Contains(framework, "react") ||
		strings.Contains(framework, "angular") ||
		strings.Contains(framework, "vue") {
		return 50.0 // Frontend frameworks often have assets
	}
	return 5.0 // Minimal assets for backend services
}

func (p *ResourcePredictor) hasTestSuite(analysis RepositoryAnalysis) bool {
	// Check for test dependencies
	for _, dep := range analysis.GetDependencies() {
		dep = strings.ToLower(dep)
		if strings.Contains(dep, "test") ||
			strings.Contains(dep, "jest") ||
			strings.Contains(dep, "mocha") ||
			strings.Contains(dep, "pytest") ||
			strings.Contains(dep, "junit") {
			return true
		}
	}
	return false
}

func (p *ResourcePredictor) hasNativeExtensions(analysis RepositoryAnalysis) bool {
	// Check for native dependencies
	indicators := []string{"numpy", "scipy", "pandas", "opencv", "tensorflow", "grpc", "protobuf"}
	for _, dep := range analysis.GetDependencies() {
		dep = strings.ToLower(dep)
		for _, indicator := range indicators {
			if strings.Contains(dep, indicator) {
				return true
			}
		}
	}
	return false
}

func (p *ResourcePredictor) shouldUseMultiStage(analysis RepositoryAnalysis) bool {
	// Languages that benefit from multi-stage builds
	language := analysis.GetLanguage()
	return language == "go" ||
		language == "rust" ||
		language == "java" ||
		language == "dotnet"
}

func (p *ResourcePredictor) estimateBuildSteps(analysis RepositoryAnalysis) int {
	steps := 3 // Base: FROM, WORKDIR, CMD

	// Add dependency installation step
	if len(analysis.GetDependencies()) > 0 {
		steps++
	}

	// Add build step for compiled languages
	switch analysis.GetLanguage() {
	case "go", "rust", "java", "dotnet", "c", "cpp":
		steps++
	}

	// Add test step if test suite exists
	if p.hasTestSuite(analysis) {
		steps++
	}

	// Multi-stage builds have more steps
	if p.shouldUseMultiStage(analysis) {
		steps += 2
	}

	return steps
}

func (p *ResourcePredictor) estimateBuildTime(profile BuildProfile) time.Duration {
	// Base time in seconds
	baseTime := 60

	// Add time for dependencies
	depTime := profile.Dependencies * 2 // 2 seconds per dependency

	// Add compilation time
	compileTime := 0
	switch profile.Language {
	case "rust":
		compileTime = 180 // Rust is slow to compile
	case "go":
		compileTime = 60
	case "java", "scala":
		compileTime = 120
	case "c", "cpp":
		compileTime = 90
	}

	// Add test time
	testTime := 0
	if profile.TestSuite {
		testTime = 60
	}

	totalSeconds := baseTime + depTime + compileTime + testTime
	return time.Duration(totalSeconds) * time.Second
}

func (p *ResourcePredictor) getGeneralRecommendations(profile BuildProfile) []string {
	recommendations := []string{}

	// Language-specific recommendations
	switch profile.Language {
	case "go":
		recommendations = append(recommendations,
			"Use multi-stage build to reduce final image size",
			"Cache Go modules with --mount=type=cache",
			"Consider using distroless or scratch as final base")
	case "node":
		recommendations = append(recommendations,
			"Use npm ci instead of npm install for faster builds",
			"Add .dockerignore to exclude node_modules",
			"Consider using Alpine base for smaller images")
	case "python":
		recommendations = append(recommendations,
			"Use pip --no-cache-dir to reduce layer size",
			"Consider using slim Python base images",
			"Install dependencies before copying code for better caching")
	case "java":
		recommendations = append(recommendations,
			"Use JDK for build stage, JRE for runtime",
			"Consider using jlink to create custom JRE",
			"Use .dockerignore to exclude build artifacts")
	}

	// General recommendations
	if profile.Dependencies > 50 {
		recommendations = append(recommendations,
			"Consider splitting dependencies into separate layers for better caching")
	}

	// Create a simple analysis to check if multi-stage should be used
	simpleAnalysis := &SimpleAnalysis{
		Language: profile.Language,
	}
	if !profile.MultiStage && p.shouldUseMultiStage(simpleAnalysis) {
		recommendations = append(recommendations,
			"Use multi-stage build to separate build and runtime dependencies")
	}

	return recommendations
}

// enhanceWithHistory enhances prediction with historical build data
func (p *ResourcePredictor) enhanceWithHistory(prediction *ResourcePrediction, profile BuildProfile) {
	// Look for similar builds in history
	similarBuilds := p.historyStore.FindSimilarBuilds(profile)

	if len(similarBuilds) > 0 {
		// Adjust confidence based on historical success
		successRate := p.calculateSuccessRate(similarBuilds)
		prediction.Confidence = (prediction.Confidence + successRate) / 2

		// Add historical insights to recommendations
		if insight := p.extractHistoricalInsights(similarBuilds); insight != "" {
			prediction.Recommendations = append(prediction.Recommendations, insight)
		}

		p.logger.Info("Enhanced prediction with history",
			"similar_builds", len(similarBuilds),
			"adjusted_confidence", prediction.Confidence)
	}
}

// validatePrediction ensures predictions are within reasonable bounds
func (p *ResourcePredictor) validatePrediction(prediction *ResourcePrediction) {
	// CPU validation
	if prediction.CPU.Cores < 1 {
		prediction.CPU.Cores = 1
	} else if prediction.CPU.Cores > 16 {
		prediction.CPU.Cores = 16
	}

	// Memory validation
	if prediction.Memory.MinMemoryMB < 256 {
		prediction.Memory.MinMemoryMB = 256
	}
	if prediction.Memory.RecommendedMB < prediction.Memory.MinMemoryMB {
		prediction.Memory.RecommendedMB = prediction.Memory.MinMemoryMB * 2
	}
	if prediction.Memory.MaxMemoryMB < prediction.Memory.RecommendedMB {
		prediction.Memory.MaxMemoryMB = prediction.Memory.RecommendedMB * 2
	}

	// Storage validation
	if prediction.Storage.TotalRequiredMB < 100 {
		prediction.Storage.TotalRequiredMB = 100
	}

	// Build time validation
	if prediction.BuildTime < 30*time.Second {
		prediction.BuildTime = 30 * time.Second
	} else if prediction.BuildTime > 2*time.Hour {
		prediction.BuildTime = 2 * time.Hour
	}

	// Confidence validation
	if prediction.Confidence < 0 {
		prediction.Confidence = 0
	} else if prediction.Confidence > 1 {
		prediction.Confidence = 1
	}
}

func (p *ResourcePredictor) calculateSuccessRate(builds []BuildRecord) float64 {
	if len(builds) == 0 {
		return 0.5
	}

	successful := 0
	for _, build := range builds {
		if build.Success {
			successful++
		}
	}

	return float64(successful) / float64(len(builds))
}

func (p *ResourcePredictor) extractHistoricalInsights(builds []BuildRecord) string {
	// Analyze patterns in historical builds
	avgBuildTime := time.Duration(0)
	for _, build := range builds {
		avgBuildTime += build.Duration
	}
	avgBuildTime /= time.Duration(len(builds))

	return fmt.Sprintf("Based on %d similar builds, average build time is %v",
		len(builds), avgBuildTime.Round(time.Second))
}
