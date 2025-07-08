package build

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// Register consolidated docker operations tool
func init() {
	core.RegisterTool("docker_operations", func() api.Tool {
		return &ConsolidatedDockerOperationsTool{}
	})
}

// OperationType defined in docker_operation.go

// DockerOperationsInput represents unified input for Docker operations
type DockerOperationsInput struct {
	// Core parameters
	SessionID string        `json:"session_id,omitempty" validate:"omitempty,session_id" description:"Session ID for state correlation"`
	Operation OperationType `json:"operation" validate:"required,oneof=push pull tag" description:"Type of operation (push, pull, tag)"`

	// Push/Pull parameters
	ImageRef string `json:"image_ref,omitempty" validate:"required_if=Operation push,required_if=Operation pull,docker_image" description:"Image reference for push/pull operations"`
	Registry string `json:"registry,omitempty" validate:"omitempty,registry_url" description:"Registry URL (optional for pull, extracted from image_ref if not provided)"`

	// Tag parameters
	SourceImage string `json:"source_image,omitempty" validate:"required_if=Operation tag,docker_image" description:"Source image for tag operation"`
	TargetImage string `json:"target_image,omitempty" validate:"required_if=Operation tag,docker_image" description:"Target image for tag operation"`

	// Common options
	Platform string `json:"platform,omitempty" validate:"omitempty,platform" description:"Target platform (for pull operations)"`
	Force    bool   `json:"force,omitempty" description:"Force operation even if conflicts exist"`
	DryRun   bool   `json:"dry_run,omitempty" description:"Preview operation without executing"`

	// Batch operations
	BatchMode bool     `json:"batch_mode,omitempty" description:"Enable batch operation mode"`
	Images    []string `json:"images,omitempty" description:"List of images for batch operations"`

	// Advanced options
	Timeout    int  `json:"timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Operation timeout in seconds (default: 600)"`
	RetryCount int  `json:"retry_count,omitempty" validate:"omitempty,min=0,max=10" description:"Number of retry attempts (default: 3)"`
	Parallel   bool `json:"parallel,omitempty" description:"Enable parallel execution for batch operations"`
}

// Validate implements validation using tag-based validation
func (d DockerOperationsInput) Validate() error {
	return validation.ValidateTaggedStruct(d)
}

// DockerOperationsOutput represents unified output for Docker operations
type DockerOperationsOutput struct {
	// Status
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Operation string `json:"operation"`
	Error     string `json:"error,omitempty"`

	// Operation results
	ImageRef  string        `json:"image_ref,omitempty"`
	ImageID   string        `json:"image_id,omitempty"`
	ImageSize int64         `json:"image_size,omitempty"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"start_time"`

	// Push-specific results
	Registry     string            `json:"registry,omitempty"`
	PushedLayers int               `json:"pushed_layers,omitempty"`
	CachedLayers int               `json:"cached_layers,omitempty"`
	DigestMap    map[string]string `json:"digest_map,omitempty"`

	// Pull-specific results
	PulledLayers int     `json:"pulled_layers,omitempty"`
	LayerSizes   []int64 `json:"layer_sizes,omitempty"`

	// Tag-specific results
	SourceImage string `json:"source_image,omitempty"`
	TargetImage string `json:"target_image,omitempty"`

	// Batch operation results
	BatchResults []BatchOperationResult `json:"batch_results,omitempty"`

	// Analysis and insights
	PerformanceMetrics *OperationMetrics `json:"performance_metrics,omitempty"`
	Recommendations    []string          `json:"recommendations,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// BatchOperationResult represents result of individual batch operation
type BatchOperationResult struct {
	ImageRef string        `json:"image_ref"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
	ImageID  string        `json:"image_id,omitempty"`
	Size     int64         `json:"size,omitempty"`
}

// OperationMetrics represents performance metrics for operations
type OperationMetrics struct {
	TotalDuration      time.Duration `json:"total_duration"`
	NetworkTime        time.Duration `json:"network_time"`
	ProcessingTime     time.Duration `json:"processing_time"`
	CacheHitRatio      float64       `json:"cache_hit_ratio"`
	AverageLayerSize   int64         `json:"average_layer_size"`
	CompressionRatio   float64       `json:"compression_ratio"`
	TransferRate       float64       `json:"transfer_rate_mbps"`
	ParallelEfficiency float64       `json:"parallel_efficiency,omitempty"`
}

// ConsolidatedDockerOperationsTool - Unified Docker operations tool
type ConsolidatedDockerOperationsTool struct {
	// Service dependencies
	sessionStore  services.SessionStore
	sessionState  services.SessionState
	buildExecutor services.BuildExecutor
	logger        *slog.Logger

	// Core components
	dockerClient DockerClient

	// Operation state management
	state      map[string]interface{}
	stateMutex sync.RWMutex

	// Performance tracking
	metrics *OperationMetrics

	// Operation executors
	pushExecutor *PushExecutor
	pullExecutor *PullExecutor
	tagExecutor  *TagExecutor
}

// NewConsolidatedDockerOperationsTool creates a new consolidated Docker operations tool
func NewConsolidatedDockerOperationsTool(
	serviceContainer services.ServiceContainer,
	dockerClient DockerClient,
	logger *slog.Logger,
) *ConsolidatedDockerOperationsTool {
	toolLogger := logger.With("tool", "docker_operations_consolidated")

	tool := &ConsolidatedDockerOperationsTool{
		sessionStore:  serviceContainer.SessionStore(),
		sessionState:  serviceContainer.SessionState(),
		buildExecutor: serviceContainer.BuildExecutor(),
		logger:        toolLogger,
		dockerClient:  dockerClient,
		state:         make(map[string]interface{}),
		metrics:       &OperationMetrics{},
	}

	// Initialize operation executors
	tool.pushExecutor = NewPushExecutor(dockerClient, toolLogger)
	tool.pullExecutor = NewPullExecutor(dockerClient, toolLogger)
	tool.tagExecutor = NewTagExecutor(dockerClient, toolLogger)

	return tool
}

// Execute implements api.Tool interface
func (t *ConsolidatedDockerOperationsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Parse input
	opsInput, err := t.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Validate input
	if err := opsInput.Validate(); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Input validation failed: %v", err),
		}, err
	}

	// Generate session ID if not provided
	sessionID := opsInput.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("ops_%d", time.Now().Unix())
	}

	// Execute operation based on type
	result, err := t.executeOperation(ctx, opsInput, sessionID, startTime)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Operation failed: %v", err),
		}, err
	}

	return api.ToolOutput{
		Success: result.Success,
		Data:    map[string]interface{}{"result": result},
	}, nil
}

// executeOperation executes the specified Docker operation
func (t *ConsolidatedDockerOperationsTool) executeOperation(
	ctx context.Context,
	input *DockerOperationsInput,
	sessionID string,
	startTime time.Time,
) (*DockerOperationsOutput, error) {
	result := &DockerOperationsOutput{
		Success:   false,
		SessionID: sessionID,
		Operation: string(input.Operation),
		StartTime: startTime,
		Metadata:  make(map[string]interface{}),
	}

	// Initialize session
	if err := t.initializeSession(ctx, sessionID, input); err != nil {
		t.logger.Warn("Failed to initialize session", "error", err)
	}

	// Handle dry run
	if input.DryRun {
		return t.handleDryRun(ctx, input, result)
	}

	// Execute based on operation type
	switch input.Operation {
	case OperationPush:
		return t.executePush(ctx, input, result)
	case OperationPull:
		return t.executePull(ctx, input, result)
	case OperationTag:
		return t.executeTag(ctx, input, result)
	default:
		result.Error = fmt.Sprintf("Unknown operation: %s", input.Operation)
		return result, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

// executePush executes Docker push operation
func (t *ConsolidatedDockerOperationsTool) executePush(
	ctx context.Context,
	input *DockerOperationsInput,
	result *DockerOperationsOutput,
) (*DockerOperationsOutput, error) {
	t.logger.Info("Executing Docker push operation",
		"image_ref", input.ImageRef,
		"registry", input.Registry,
		"session_id", result.SessionID)

	// Batch mode
	if input.BatchMode && len(input.Images) > 0 {
		return t.executeBatchPush(ctx, input, result)
	}

	// Single image push
	pushResult, err := t.pushExecutor.Push(ctx, PushRequest{
		ImageRef:   input.ImageRef,
		Registry:   input.Registry,
		Force:      input.Force,
		Timeout:    time.Duration(input.Timeout) * time.Second,
		RetryCount: input.RetryCount,
	})

	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Update result with push details
	result.Success = true
	result.ImageRef = input.ImageRef
	result.ImageID = pushResult.ImageID
	result.Registry = pushResult.Registry
	result.PushedLayers = pushResult.PushedLayers
	result.CachedLayers = pushResult.CachedLayers
	result.DigestMap = pushResult.DigestMap
	result.Duration = time.Since(result.StartTime)

	// Add performance metrics
	result.PerformanceMetrics = &OperationMetrics{
		TotalDuration:    result.Duration,
		NetworkTime:      pushResult.NetworkTime,
		ProcessingTime:   pushResult.ProcessingTime,
		CacheHitRatio:    pushResult.CacheHitRatio,
		TransferRate:     pushResult.TransferRate,
		CompressionRatio: pushResult.CompressionRatio,
	}

	// Add recommendations
	result.Recommendations = t.generatePushRecommendations(pushResult)

	t.logger.Info("Docker push operation completed",
		"image_ref", result.ImageRef,
		"duration", result.Duration,
		"success", result.Success)

	return result, nil
}

// executePull executes Docker pull operation
func (t *ConsolidatedDockerOperationsTool) executePull(
	ctx context.Context,
	input *DockerOperationsInput,
	result *DockerOperationsOutput,
) (*DockerOperationsOutput, error) {
	t.logger.Info("Executing Docker pull operation",
		"image_ref", input.ImageRef,
		"platform", input.Platform,
		"session_id", result.SessionID)

	// Batch mode
	if input.BatchMode && len(input.Images) > 0 {
		return t.executeBatchPull(ctx, input, result)
	}

	// Single image pull
	pullResult, err := t.pullExecutor.Pull(ctx, PullRequest{
		ImageRef:   input.ImageRef,
		Platform:   input.Platform,
		Force:      input.Force,
		Timeout:    time.Duration(input.Timeout) * time.Second,
		RetryCount: input.RetryCount,
	})

	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Update result with pull details
	result.Success = true
	result.ImageRef = input.ImageRef
	result.ImageID = pullResult.ImageID
	result.ImageSize = pullResult.ImageSize
	result.PulledLayers = pullResult.PulledLayers
	result.LayerSizes = pullResult.LayerSizes
	result.Duration = time.Since(result.StartTime)

	// Add performance metrics
	result.PerformanceMetrics = &OperationMetrics{
		TotalDuration:    result.Duration,
		NetworkTime:      pullResult.NetworkTime,
		ProcessingTime:   pullResult.ProcessingTime,
		AverageLayerSize: pullResult.AverageLayerSize,
		TransferRate:     pullResult.TransferRate,
	}

	// Add recommendations
	result.Recommendations = t.generatePullRecommendations(pullResult)

	t.logger.Info("Docker pull operation completed",
		"image_ref", result.ImageRef,
		"image_size", result.ImageSize,
		"duration", result.Duration,
		"success", result.Success)

	return result, nil
}

// executeTag executes Docker tag operation
func (t *ConsolidatedDockerOperationsTool) executeTag(
	ctx context.Context,
	input *DockerOperationsInput,
	result *DockerOperationsOutput,
) (*DockerOperationsOutput, error) {
	t.logger.Info("Executing Docker tag operation",
		"source_image", input.SourceImage,
		"target_image", input.TargetImage,
		"session_id", result.SessionID)

	// Single image tag
	tagResult, err := t.tagExecutor.Tag(ctx, TagRequest{
		SourceImage: input.SourceImage,
		TargetImage: input.TargetImage,
		Force:       input.Force,
	})

	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Update result with tag details
	result.Success = true
	result.SourceImage = input.SourceImage
	result.TargetImage = input.TargetImage
	result.ImageID = tagResult.ImageID
	result.Duration = time.Since(result.StartTime)

	// Add performance metrics
	result.PerformanceMetrics = &OperationMetrics{
		TotalDuration:  result.Duration,
		ProcessingTime: tagResult.ProcessingTime,
	}

	t.logger.Info("Docker tag operation completed",
		"source_image", result.SourceImage,
		"target_image", result.TargetImage,
		"duration", result.Duration,
		"success", result.Success)

	return result, nil
}

// executeBatchPush executes batch push operations
func (t *ConsolidatedDockerOperationsTool) executeBatchPush(
	ctx context.Context,
	input *DockerOperationsInput,
	result *DockerOperationsOutput,
) (*DockerOperationsOutput, error) {
	results := make([]BatchOperationResult, len(input.Images))

	// Execute operations in parallel or sequential based on configuration
	if input.Parallel {
		results = t.executeBatchParallel(ctx, input.Images, func(imageRef string) BatchOperationResult {
			return t.executeSinglePush(ctx, imageRef, input.Registry, input.Force)
		})
	} else {
		for i, imageRef := range input.Images {
			results[i] = t.executeSinglePush(ctx, imageRef, input.Registry, input.Force)
		}
	}

	// Aggregate results
	successCount := 0
	for _, batchResult := range results {
		if batchResult.Success {
			successCount++
		}
	}

	result.Success = successCount == len(input.Images)
	result.BatchResults = results
	result.Duration = time.Since(result.StartTime)

	// Calculate parallel efficiency if parallel execution was used
	if input.Parallel {
		result.PerformanceMetrics = &OperationMetrics{
			TotalDuration:      result.Duration,
			ParallelEfficiency: t.calculateParallelEfficiency(results),
		}
	}

	t.logger.Info("Batch push operation completed",
		"total_images", len(input.Images),
		"successful", successCount,
		"failed", len(input.Images)-successCount,
		"duration", result.Duration)

	return result, nil
}

// executeBatchPull executes batch pull operations
func (t *ConsolidatedDockerOperationsTool) executeBatchPull(
	ctx context.Context,
	input *DockerOperationsInput,
	result *DockerOperationsOutput,
) (*DockerOperationsOutput, error) {
	results := make([]BatchOperationResult, len(input.Images))

	// Execute operations in parallel or sequential based on configuration
	if input.Parallel {
		results = t.executeBatchParallel(ctx, input.Images, func(imageRef string) BatchOperationResult {
			return t.executeSinglePull(ctx, imageRef, input.Platform, input.Force)
		})
	} else {
		for i, imageRef := range input.Images {
			results[i] = t.executeSinglePull(ctx, imageRef, input.Platform, input.Force)
		}
	}

	// Aggregate results
	successCount := 0
	for _, batchResult := range results {
		if batchResult.Success {
			successCount++
		}
	}

	result.Success = successCount == len(input.Images)
	result.BatchResults = results
	result.Duration = time.Since(result.StartTime)

	// Calculate parallel efficiency if parallel execution was used
	if input.Parallel {
		result.PerformanceMetrics = &OperationMetrics{
			TotalDuration:      result.Duration,
			ParallelEfficiency: t.calculateParallelEfficiency(results),
		}
	}

	t.logger.Info("Batch pull operation completed",
		"total_images", len(input.Images),
		"successful", successCount,
		"failed", len(input.Images)-successCount,
		"duration", result.Duration)

	return result, nil
}

// Implement api.Tool interface methods

func (t *ConsolidatedDockerOperationsTool) Name() string {
	return "docker_operations"
}

func (t *ConsolidatedDockerOperationsTool) Description() string {
	return "Unified Docker operations tool for push, pull, and tag operations with batch processing and performance optimization"
}

func (t *ConsolidatedDockerOperationsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "docker_operations",
		Description: "Unified Docker operations tool for push, pull, and tag operations with batch processing and performance optimization",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Type of operation (push, pull, tag)",
					"enum":        []string{"push", "pull", "tag"},
				},
				"image_ref": map[string]interface{}{
					"type":        "string",
					"description": "Image reference for push/pull operations",
				},
				"source_image": map[string]interface{}{
					"type":        "string",
					"description": "Source image for tag operation",
				},
				"target_image": map[string]interface{}{
					"type":        "string",
					"description": "Target image for tag operation",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Registry URL (optional)",
				},
				"platform": map[string]interface{}{
					"type":        "string",
					"description": "Target platform (for pull operations)",
				},
				"batch_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable batch operation mode",
				},
				"images": map[string]interface{}{
					"type":        "array",
					"description": "List of images for batch operations",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"parallel": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable parallel execution for batch operations",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Force operation even if conflicts exist",
				},
				"dry_run": map[string]interface{}{
					"type":        "boolean",
					"description": "Preview operation without executing",
				},
			},
			"required": []string{"operation"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether operation was successful",
				},
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Type of operation performed",
				},
				"image_ref": map[string]interface{}{
					"type":        "string",
					"description": "Image reference processed",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Operation duration",
				},
				"batch_results": map[string]interface{}{
					"type":        "array",
					"description": "Results of batch operations",
				},
				"performance_metrics": map[string]interface{}{
					"type":        "object",
					"description": "Performance metrics for the operation",
				},
				"recommendations": map[string]interface{}{
					"type":        "array",
					"description": "Optimization recommendations",
				},
			},
		},
	}
}

// Helper methods and types will be implemented in the next file...
