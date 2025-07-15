package caching

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
)

// WorkflowCache provides caching for workflow operations
type WorkflowCache struct {
	analysisCache    Cache
	dockerfileCache  Cache
	manifestCache    Cache
	metricsCollector func(operation string, hit bool)
}

// NewWorkflowCache creates a new workflow-specific cache
func NewWorkflowCache() *WorkflowCache {
	// Create layered caches for different types of data
	// L1: Small, fast cache for hot data
	// L2: Larger cache for warm data

	analysisL1 := NewMemoryCache(50, 1*time.Minute)
	analysisL2 := NewMemoryCache(200, 5*time.Minute)

	dockerfileL1 := NewMemoryCache(100, 1*time.Minute)
	dockerfileL2 := NewMemoryCache(500, 5*time.Minute)

	manifestL1 := NewMemoryCache(50, 1*time.Minute)
	manifestL2 := NewMemoryCache(200, 5*time.Minute)

	return &WorkflowCache{
		analysisCache:   NewLayeredCache(analysisL1, analysisL2),
		dockerfileCache: NewLayeredCache(dockerfileL1, dockerfileL2),
		manifestCache:   NewLayeredCache(manifestL1, manifestL2),
		metricsCollector: func(operation string, hit bool) {
			// Default no-op metrics collector
		},
	}
}

// SetMetricsCollector sets the metrics collection function
func (wc *WorkflowCache) SetMetricsCollector(collector func(operation string, hit bool)) {
	wc.metricsCollector = collector
}

// AnalysisResult represents cached repository analysis
type AnalysisResult struct {
	Language     string                 `json:"language"`
	Framework    string                 `json:"framework"`
	Dependencies []string               `json:"dependencies"`
	BuildSystem  string                 `json:"build_system"`
	HasTests     bool                   `json:"has_tests"`
	Metadata     map[string]interface{} `json:"metadata"`
	AnalyzedAt   time.Time              `json:"analyzed_at"`
	CacheKey     string                 `json:"cache_key"`
}

// GetAnalysis retrieves cached repository analysis
func (wc *WorkflowCache) GetAnalysis(ctx context.Context, repoPath string, commitHash string) (*AnalysisResult, bool) {
	key := wc.analysisKey(repoPath, commitHash)

	value, found := wc.analysisCache.Get(ctx, key)
	if !found {
		wc.metricsCollector("analysis_get", false)
		return nil, false
	}

	wc.metricsCollector("analysis_get", true)

	if result, ok := value.(*AnalysisResult); ok {
		return result, true
	}

	// Try to deserialize if stored as JSON
	cache := NewSerializableCache(wc.analysisCache)
	var result AnalysisResult
	if err := cache.Get(ctx, key, &result); err == nil {
		return &result, true
	}

	return nil, false
}

// SetAnalysis caches repository analysis results
func (wc *WorkflowCache) SetAnalysis(ctx context.Context, repoPath string, commitHash string, result *AnalysisResult) {
	key := wc.analysisKey(repoPath, commitHash)
	result.CacheKey = key
	result.AnalyzedAt = time.Now()

	// Cache for 1 hour - analysis results are stable for a given commit
	wc.analysisCache.Set(ctx, key, result, 1*time.Hour)
	wc.metricsCollector("analysis_set", true)
}

// DockerfileContent represents cached Dockerfile content
type DockerfileContent struct {
	Content       string                 `json:"content"`
	BaseImage     string                 `json:"base_image"`
	Optimizations []string               `json:"optimizations"`
	Metadata      map[string]interface{} `json:"metadata"`
	GeneratedAt   time.Time              `json:"generated_at"`
	CacheKey      string                 `json:"cache_key"`
}

// GetDockerfile retrieves cached Dockerfile content
func (wc *WorkflowCache) GetDockerfile(ctx context.Context, language, framework string, dependencies []string) (*DockerfileContent, bool) {
	key := wc.dockerfileKey(language, framework, dependencies)

	value, found := wc.dockerfileCache.Get(ctx, key)
	if !found {
		wc.metricsCollector("dockerfile_get", false)
		return nil, false
	}

	wc.metricsCollector("dockerfile_get", true)

	if result, ok := value.(*DockerfileContent); ok {
		return result, true
	}

	// Try to deserialize if stored as JSON
	cache := NewSerializableCache(wc.dockerfileCache)
	var result DockerfileContent
	if err := cache.Get(ctx, key, &result); err == nil {
		return &result, true
	}

	return nil, false
}

// SetDockerfile caches Dockerfile content
func (wc *WorkflowCache) SetDockerfile(ctx context.Context, language, framework string, dependencies []string, content *DockerfileContent) {
	key := wc.dockerfileKey(language, framework, dependencies)
	content.CacheKey = key
	content.GeneratedAt = time.Now()

	// Cache for 24 hours - Dockerfiles for same tech stack are relatively stable
	wc.dockerfileCache.Set(ctx, key, content, 24*time.Hour)
	wc.metricsCollector("dockerfile_set", true)
}

// ManifestContent represents cached Kubernetes manifests
type ManifestContent struct {
	Manifests   map[string]string      `json:"manifests"`
	Namespace   string                 `json:"namespace"`
	ServiceType string                 `json:"service_type"`
	Metadata    map[string]interface{} `json:"metadata"`
	GeneratedAt time.Time              `json:"generated_at"`
	CacheKey    string                 `json:"cache_key"`
}

// GetManifest retrieves cached Kubernetes manifests
func (wc *WorkflowCache) GetManifest(ctx context.Context, appName, imageRef string, port int) (*ManifestContent, bool) {
	key := wc.manifestKey(appName, imageRef, port)

	value, found := wc.manifestCache.Get(ctx, key)
	if !found {
		wc.metricsCollector("manifest_get", false)
		return nil, false
	}

	wc.metricsCollector("manifest_get", true)

	if result, ok := value.(*ManifestContent); ok {
		return result, true
	}

	// Try to deserialize if stored as JSON
	cache := NewSerializableCache(wc.manifestCache)
	var result ManifestContent
	if err := cache.Get(ctx, key, &result); err == nil {
		return &result, true
	}

	return nil, false
}

// SetManifest caches Kubernetes manifest content
func (wc *WorkflowCache) SetManifest(ctx context.Context, appName, imageRef string, port int, content *ManifestContent) {
	key := wc.manifestKey(appName, imageRef, port)
	content.CacheKey = key
	content.GeneratedAt = time.Now()

	// Cache for 12 hours - manifests are semi-stable
	wc.manifestCache.Set(ctx, key, content, 12*time.Hour)
	wc.metricsCollector("manifest_set", true)
}

// InvalidateAnalysis removes cached analysis for a repository
func (wc *WorkflowCache) InvalidateAnalysis(ctx context.Context, repoPath string, commitHash string) {
	key := wc.analysisKey(repoPath, commitHash)
	wc.analysisCache.Delete(ctx, key)
}

// InvalidateAll clears all caches
func (wc *WorkflowCache) InvalidateAll(ctx context.Context) {
	wc.analysisCache.Clear(ctx)
	wc.dockerfileCache.Clear(ctx)
	wc.manifestCache.Clear(ctx)
}

// Stats returns aggregated cache statistics
func (wc *WorkflowCache) Stats() map[string]CacheStats {
	return map[string]CacheStats{
		"analysis":   wc.analysisCache.Stats(),
		"dockerfile": wc.dockerfileCache.Stats(),
		"manifest":   wc.manifestCache.Stats(),
	}
}

// Helper methods for generating cache keys

func (wc *WorkflowCache) analysisKey(repoPath, commitHash string) string {
	return CacheKey("analysis", hashString(repoPath), commitHash)
}

func (wc *WorkflowCache) dockerfileKey(language, framework string, dependencies []string) string {
	depHash := hashStringSlice(dependencies)
	return CacheKey("dockerfile", language, framework, depHash)
}

func (wc *WorkflowCache) manifestKey(appName, imageRef string, port int) string {
	portStr := fmt.Sprintf("%d", port)
	return CacheKey("manifest", hashString(appName), hashString(imageRef), portStr)
}

// hashString creates a short hash of a string
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8]) // Use first 8 bytes for shorter keys
}

// hashStringSlice creates a hash of a string slice
func hashStringSlice(slice []string) string {
	h := sha256.New()
	for _, s := range slice {
		h.Write([]byte(s))
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8])
}

// WorkflowCacheDecorator decorates a workflow orchestrator with caching
type WorkflowCacheDecorator struct {
	base  workflow.WorkflowOrchestrator
	cache *WorkflowCache
}

// NewWorkflowCacheDecorator creates a new caching decorator
func NewWorkflowCacheDecorator(base workflow.WorkflowOrchestrator, cache *WorkflowCache) *WorkflowCacheDecorator {
	return &WorkflowCacheDecorator{
		base:  base,
		cache: cache,
	}
}

// Execute runs the complete containerization workflow with caching
func (d *WorkflowCacheDecorator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	// For now, just pass through to base orchestrator
	// In a full implementation, individual steps would check cache
	// and skip expensive operations when possible

	// Example of how caching would be integrated:
	// 1. Check if analysis is cached for this repo/commit
	// 2. Check if Dockerfile is cached for detected tech stack
	// 3. Check if manifests are cached for this configuration
	// 4. Skip cached steps and use cached results

	result, err := d.base.Execute(ctx, req, args)

	// Cache successful results for future use
	if err == nil && result.Success {
		// Cache would be populated by individual steps
		// This is just a placeholder for the pattern
	}

	return result, err
}
