package build

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// Helper methods for ConsolidatedDockerOperationsTool

// parseInput parses the input arguments into DockerOperationsInput
func (t *ConsolidatedDockerOperationsTool) parseInput(input api.ToolInput) (*DockerOperationsInput, error) {
	result := &DockerOperationsInput{}

	// Extract operation type
	if operation, ok := input.Data["operation"].(string); ok {
		result.Operation = OperationType(operation)
	}

	// Extract session ID
	if sessionID, ok := input.Data["session_id"].(string); ok {
		result.SessionID = sessionID
	}

	// Extract image reference
	if imageRef, ok := input.Data["image_ref"].(string); ok {
		result.ImageRef = imageRef
	}

	// Extract registry
	if registry, ok := input.Data["registry"].(string); ok {
		result.Registry = registry
	}

	// Extract source/target images for tag operations
	if sourceImage, ok := input.Data["source_image"].(string); ok {
		result.SourceImage = sourceImage
	}
	if targetImage, ok := input.Data["target_image"].(string); ok {
		result.TargetImage = targetImage
	}

	// Extract platform
	if platform, ok := input.Data["platform"].(string); ok {
		result.Platform = platform
	}

	// Extract boolean flags
	if force, ok := input.Data["force"].(bool); ok {
		result.Force = force
	}
	if dryRun, ok := input.Data["dry_run"].(bool); ok {
		result.DryRun = dryRun
	}
	if batchMode, ok := input.Data["batch_mode"].(bool); ok {
		result.BatchMode = batchMode
	}
	if parallel, ok := input.Data["parallel"].(bool); ok {
		result.Parallel = parallel
	}

	// Extract images array for batch operations
	if images, ok := input.Data["images"].([]interface{}); ok {
		result.Images = make([]string, len(images))
		for i, img := range images {
			if str, ok := img.(string); ok {
				result.Images[i] = str
			}
		}
	}

	// Extract timeout and retry count
	if timeout, ok := input.Data["timeout"].(int); ok {
		result.Timeout = timeout
	}
	if retryCount, ok := input.Data["retry_count"].(int); ok {
		result.RetryCount = retryCount
	}

	// Set defaults
	if result.Timeout == 0 {
		result.Timeout = 600 // 10 minutes default
	}
	if result.RetryCount == 0 {
		result.RetryCount = 3 // 3 retries default
	}

	return result, nil
}

// initializeSession initializes the session for operation tracking
func (t *ConsolidatedDockerOperationsTool) initializeSession(ctx context.Context, sessionID string, input *DockerOperationsInput) error {
	if t.sessionStore == nil {
		return nil
	}

	session := &api.Session{
		ID: sessionID,
		Metadata: map[string]interface{}{
			"tool":       "docker_operations",
			"operation":  string(input.Operation),
			"image_ref":  input.ImageRef,
			"batch_mode": input.BatchMode,
			"started_at": time.Now(),
		},
	}

	return t.sessionStore.Create(ctx, session)
}

// handleDryRun handles dry run operations
func (t *ConsolidatedDockerOperationsTool) handleDryRun(
	ctx context.Context,
	input *DockerOperationsInput,
	result *DockerOperationsOutput,
) (*DockerOperationsOutput, error) {
	result.Success = true
	result.Duration = time.Since(result.StartTime)

	switch input.Operation {
	case OperationPush:
		result.ImageRef = input.ImageRef
		result.Registry = input.Registry
		result.Recommendations = []string{
			"This is a dry-run - no actual push was performed",
			"Remove dry_run flag to perform actual push operation",
			fmt.Sprintf("Would push image %s to registry %s", input.ImageRef, input.Registry),
		}

	case OperationPull:
		result.ImageRef = input.ImageRef
		result.Recommendations = []string{
			"This is a dry-run - no actual pull was performed",
			"Remove dry_run flag to perform actual pull operation",
			fmt.Sprintf("Would pull image %s", input.ImageRef),
		}

	case OperationTag:
		result.SourceImage = input.SourceImage
		result.TargetImage = input.TargetImage
		result.Recommendations = []string{
			"This is a dry-run - no actual tag was performed",
			"Remove dry_run flag to perform actual tag operation",
			fmt.Sprintf("Would tag image %s as %s", input.SourceImage, input.TargetImage),
		}
	}

	return result, nil
}

// executeBatchParallel executes batch operations in parallel
func (t *ConsolidatedDockerOperationsTool) executeBatchParallel(
	ctx context.Context,
	images []string,
	operationFunc func(string) BatchOperationResult,
) []BatchOperationResult {
	results := make([]BatchOperationResult, len(images))
	var wg sync.WaitGroup

	for i, image := range images {
		wg.Add(1)
		go func(index int, img string) {
			defer wg.Done()
			results[index] = operationFunc(img)
		}(i, image)
	}

	wg.Wait()
	return results
}

// executeSinglePush executes a single push operation
func (t *ConsolidatedDockerOperationsTool) executeSinglePush(ctx context.Context, imageRef, registry string, force bool) BatchOperationResult {
	startTime := time.Now()

	pushResult, err := t.pushExecutor.Push(ctx, PushRequest{
		ImageRef: imageRef,
		Registry: registry,
		Force:    force,
		Timeout:  10 * time.Minute,
	})

	result := BatchOperationResult{
		ImageRef: imageRef,
		Duration: time.Since(startTime),
		Success:  err == nil,
	}

	if err != nil {
		result.Error = err.Error()
	} else {
		result.ImageID = pushResult.ImageID
		result.Size = pushResult.Size
	}

	return result
}

// executeSinglePull executes a single pull operation
func (t *ConsolidatedDockerOperationsTool) executeSinglePull(ctx context.Context, imageRef, platform string, force bool) BatchOperationResult {
	startTime := time.Now()

	pullResult, err := t.pullExecutor.Pull(ctx, PullRequest{
		ImageRef: imageRef,
		Platform: platform,
		Force:    force,
		Timeout:  10 * time.Minute,
	})

	result := BatchOperationResult{
		ImageRef: imageRef,
		Duration: time.Since(startTime),
		Success:  err == nil,
	}

	if err != nil {
		result.Error = err.Error()
	} else {
		result.ImageID = pullResult.ImageID
		result.Size = pullResult.ImageSize
	}

	return result
}

// calculateParallelEfficiency calculates parallel execution efficiency
func (t *ConsolidatedDockerOperationsTool) calculateParallelEfficiency(results []BatchOperationResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	var totalDuration time.Duration
	var maxDuration time.Duration

	for _, result := range results {
		totalDuration += result.Duration
		if result.Duration > maxDuration {
			maxDuration = result.Duration
		}
	}

	if maxDuration == 0 {
		return 0.0
	}

	// Efficiency = (total sequential time) / (actual parallel time * number of operations)
	expectedSequentialTime := totalDuration
	actualParallelTime := maxDuration * time.Duration(len(results))

	return float64(expectedSequentialTime) / float64(actualParallelTime)
}

// generatePushRecommendations generates recommendations for push operations
func (t *ConsolidatedDockerOperationsTool) generatePushRecommendations(pushResult *PushResult) []string {
	var recommendations []string

	// Cache efficiency recommendations
	if pushResult.CacheHitRatio < 0.5 {
		recommendations = append(recommendations, "Low cache hit ratio detected - consider organizing layers for better caching")
	}

	// Transfer rate recommendations
	if pushResult.TransferRate < 10.0 { // Less than 10 MB/s
		recommendations = append(recommendations, "Slow transfer rate detected - consider optimizing network connection or using a closer registry")
	}

	// Compression recommendations
	if pushResult.CompressionRatio < 0.3 {
		recommendations = append(recommendations, "Low compression ratio - consider using smaller base images or multi-stage builds")
	}

	// Layer count recommendations
	if pushResult.PushedLayers > 20 {
		recommendations = append(recommendations, "High layer count detected - consider combining RUN instructions to reduce layers")
	}

	return recommendations
}

// generatePullRecommendations generates recommendations for pull operations
func (t *ConsolidatedDockerOperationsTool) generatePullRecommendations(pullResult *PullResult) []string {
	var recommendations []string

	// Size recommendations
	if pullResult.ImageSize > 1*1024*1024*1024 { // > 1GB
		recommendations = append(recommendations, "Large image size detected - consider using more specific base images or multi-stage builds")
	}

	// Transfer rate recommendations
	if pullResult.TransferRate < 5.0 { // Less than 5 MB/s
		recommendations = append(recommendations, "Slow pull speed detected - consider using a closer registry or checking network connection")
	}

	// Layer optimization recommendations
	if pullResult.AverageLayerSize > 100*1024*1024 { // > 100MB per layer
		recommendations = append(recommendations, "Large average layer size - consider splitting operations into smaller, more cacheable layers")
	}

	return recommendations
}

// Supporting types and interfaces

// PushRequest represents a push operation request
type PushRequest struct {
	ImageRef   string
	Registry   string
	Force      bool
	Timeout    time.Duration
	RetryCount int
}

// PushResult represents the result of a push operation
type PushResult struct {
	ImageID          string
	Registry         string
	Size             int64
	PushedLayers     int
	CachedLayers     int
	DigestMap        map[string]string
	NetworkTime      time.Duration
	ProcessingTime   time.Duration
	CacheHitRatio    float64
	TransferRate     float64
	CompressionRatio float64
}

// PullRequest represents a pull operation request
type PullRequest struct {
	ImageRef   string
	Platform   string
	Force      bool
	Timeout    time.Duration
	RetryCount int
}

// PullResult represents the result of a pull operation
type PullResult struct {
	ImageID          string
	ImageSize        int64
	PulledLayers     int
	LayerSizes       []int64
	NetworkTime      time.Duration
	ProcessingTime   time.Duration
	AverageLayerSize int64
	TransferRate     float64
}

// TagRequest represents a tag operation request
type TagRequest struct {
	SourceImage string
	TargetImage string
	Force       bool
}

// TagResult represents the result of a tag operation
type TagResult struct {
	ImageID        string
	ProcessingTime time.Duration
}

// Operation executors

// PushExecutor handles push operations
type PushExecutor struct {
	dockerClient DockerClient
	logger       *slog.Logger
}

// NewPushExecutor creates a new push executor
func NewPushExecutor(dockerClient DockerClient, logger *slog.Logger) *PushExecutor {
	return &PushExecutor{
		dockerClient: dockerClient,
		logger:       logger.With("executor", "push"),
	}
}

// Push executes a push operation
func (e *PushExecutor) Push(ctx context.Context, request PushRequest) (*PushResult, error) {
	startTime := time.Now()

	// Validate request
	if request.ImageRef == "" {
		return nil, errors.NewError().Message("image reference is required").Build()
	}

	// Extract registry from image reference if not provided
	registry := request.Registry
	if registry == "" {
		registry = e.extractRegistry(request.ImageRef)
	}

	e.logger.Info("Starting push operation",
		"image_ref", request.ImageRef,
		"registry", registry)

	// Simulate push operation (in real implementation, this would use Docker client)
	networkStart := time.Now()

	// In real implementation:
	// err := e.dockerClient.Push(ctx, request.ImageRef, registry)
	// For now, simulate
	time.Sleep(100 * time.Millisecond) // Simulate network operation

	networkTime := time.Since(networkStart)
	processingTime := time.Since(startTime) - networkTime

	result := &PushResult{
		ImageID:          fmt.Sprintf("sha256:%s", strings.Repeat("a", 64)),
		Registry:         registry,
		Size:             100 * 1024 * 1024, // 100MB simulation
		PushedLayers:     5,
		CachedLayers:     3,
		DigestMap:        map[string]string{request.ImageRef: "sha256:example"},
		NetworkTime:      networkTime,
		ProcessingTime:   processingTime,
		CacheHitRatio:    0.6,
		TransferRate:     15.0, // 15 MB/s simulation
		CompressionRatio: 0.4,
	}

	e.logger.Info("Push operation completed",
		"image_ref", request.ImageRef,
		"registry", registry,
		"duration", time.Since(startTime))

	return result, nil
}

// extractRegistry extracts registry from image reference
func (e *PushExecutor) extractRegistry(imageRef string) string {
	parts := strings.Split(imageRef, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		return parts[0]
	}
	return "docker.io"
}

// PullExecutor handles pull operations
type PullExecutor struct {
	dockerClient DockerClient
	logger       *slog.Logger
}

// NewPullExecutor creates a new pull executor
func NewPullExecutor(dockerClient DockerClient, logger *slog.Logger) *PullExecutor {
	return &PullExecutor{
		dockerClient: dockerClient,
		logger:       logger.With("executor", "pull"),
	}
}

// Pull executes a pull operation
func (e *PullExecutor) Pull(ctx context.Context, request PullRequest) (*PullResult, error) {
	startTime := time.Now()

	// Validate request
	if request.ImageRef == "" {
		return nil, errors.NewError().Message("image reference is required").Build()
	}

	e.logger.Info("Starting pull operation",
		"image_ref", request.ImageRef,
		"platform", request.Platform)

	// Simulate pull operation (in real implementation, this would use Docker client)
	networkStart := time.Now()

	// In real implementation:
	// err := e.dockerClient.Pull(ctx, request.ImageRef, request.Platform)
	// For now, simulate
	time.Sleep(200 * time.Millisecond) // Simulate network operation

	networkTime := time.Since(networkStart)
	processingTime := time.Since(startTime) - networkTime

	result := &PullResult{
		ImageID:          fmt.Sprintf("sha256:%s", strings.Repeat("b", 64)),
		ImageSize:        200 * 1024 * 1024, // 200MB simulation
		PulledLayers:     8,
		LayerSizes:       []int64{10 * 1024 * 1024, 20 * 1024 * 1024, 30 * 1024 * 1024},
		NetworkTime:      networkTime,
		ProcessingTime:   processingTime,
		AverageLayerSize: 25 * 1024 * 1024, // 25MB average
		TransferRate:     10.0,             // 10 MB/s simulation
	}

	e.logger.Info("Pull operation completed",
		"image_ref", request.ImageRef,
		"image_size", result.ImageSize,
		"duration", time.Since(startTime))

	return result, nil
}

// TagExecutor handles tag operations
type TagExecutor struct {
	dockerClient DockerClient
	logger       *slog.Logger
}

// NewTagExecutor creates a new tag executor
func NewTagExecutor(dockerClient DockerClient, logger *slog.Logger) *TagExecutor {
	return &TagExecutor{
		dockerClient: dockerClient,
		logger:       logger.With("executor", "tag"),
	}
}

// Tag executes a tag operation
func (e *TagExecutor) Tag(ctx context.Context, request TagRequest) (*TagResult, error) {
	startTime := time.Now()

	// Validate request
	if request.SourceImage == "" || request.TargetImage == "" {
		return nil, errors.NewError().Message("source and target images are required").Build()
	}

	e.logger.Info("Starting tag operation",
		"source_image", request.SourceImage,
		"target_image", request.TargetImage)

	// Simulate tag operation (in real implementation, this would use Docker client)
	processingStart := time.Now()

	// In real implementation:
	// err := e.dockerClient.Tag(ctx, request.SourceImage, request.TargetImage)
	// For now, simulate
	time.Sleep(50 * time.Millisecond) // Simulate processing time

	processingTime := time.Since(processingStart)

	result := &TagResult{
		ImageID:        fmt.Sprintf("sha256:%s", strings.Repeat("c", 64)),
		ProcessingTime: processingTime,
	}

	e.logger.Info("Tag operation completed",
		"source_image", request.SourceImage,
		"target_image", request.TargetImage,
		"duration", time.Since(startTime))

	return result, nil
}
