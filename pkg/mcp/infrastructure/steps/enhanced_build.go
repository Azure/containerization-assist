package steps

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	infraprogress "github.com/Azure/container-kit/pkg/mcp/infrastructure/progress"
	"github.com/mark3labs/mcp-go/mcp"
)

// EnhancedBuildStep wraps the standard build functionality with hierarchical progress tracking
type EnhancedBuildStep struct {
	buildFunc   func(ctx context.Context, dockerfileResult *DockerfileResult, imageName, imageTag, buildContext string, logger *slog.Logger) (*BuildResult, error)
	logger      *slog.Logger
	sinkFactory *infraprogress.SinkFactory
}

// NewEnhancedBuildStep creates a new enhanced build step with sub-progress tracking
func NewEnhancedBuildStep(logger *slog.Logger) *EnhancedBuildStep {
	return &EnhancedBuildStep{
		buildFunc:   BuildImage, // Use the existing BuildImage function
		logger:      logger.With("component", "enhanced_build_step"),
		sinkFactory: infraprogress.NewSinkFactory(logger),
	}
}

// BuildImageWithProgress wraps BuildImage with hierarchical progress tracking
func (e *EnhancedBuildStep) BuildImageWithProgress(
	ctx context.Context,
	req *mcp.CallToolRequest,
	dockerfileResult *DockerfileResult,
	imageName, imageTag, buildContext string,
) (*BuildResult, error) {
	e.logger.Info("Starting enhanced Docker image build with progress tracking",
		"image_name", imageName,
		"image_tag", imageTag,
		"build_context", buildContext)

	// Estimate number of Docker layers from Dockerfile content
	layerCount := estimateDockerLayers(dockerfileResult.Content)
	if layerCount < 3 {
		layerCount = 3 // Minimum reasonable estimate
	}

	// Create sub-tracker for Docker layer progress
	traceID := fmt.Sprintf("build-%d", time.Now().UnixNano())
	subTracker := e.sinkFactory.CreateSubTracker(ctx, req, layerCount, traceID, "docker_build")

	// Begin sub-tracking
	subTracker.Begin("Building Docker image layers")
	defer subTracker.Finish()

	// Simulate layer-by-layer progress during build
	go e.simulateLayerProgress(ctx, subTracker, layerCount, dockerfileResult.Content)

	// Execute the actual build
	result, err := e.buildFunc(ctx, dockerfileResult, imageName, imageTag, buildContext, e.logger)

	if err != nil {
		subTracker.Error(layerCount, "Docker build failed", err)
		return nil, err
	}

	// Complete sub-tracking
	subTracker.Complete("Docker image build completed successfully")

	e.logger.Info("Enhanced Docker build completed successfully",
		"image_id", result.ImageID,
		"layers_processed", layerCount)

	return result, nil
}

// simulateLayerProgress simulates Docker layer-by-layer build progress
func (e *EnhancedBuildStep) simulateLayerProgress(ctx context.Context, tracker *progress.Tracker, totalLayers int, dockerfileContent string) {
	layers := extractDockerLayers(dockerfileContent)

	for i := 0; i < totalLayers; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Determine layer type and simulate appropriate timing
		layerType := "RUN"
		layerDesc := fmt.Sprintf("Layer %d/%d", i+1, totalLayers)

		if i < len(layers) {
			layerType = layers[i].Command
			layerDesc = fmt.Sprintf("Layer %d/%d: %s", i+1, totalLayers, layers[i].Preview)
		}

		// Emit layer start
		tracker.Update(i, layerDesc, map[string]interface{}{
			"substep_name": fmt.Sprintf("layer %d/%d", i+1, totalLayers),
			"layer_type":   layerType,
			"layer_index":  i + 1,
			"total_layers": totalLayers,
			"status":       "building",
		})

		// Simulate build time based on layer type
		buildTime := getLayerBuildTime(layerType)
		time.Sleep(buildTime)

		// Emit layer completion
		tracker.Update(i+1, fmt.Sprintf("Completed %s", layerDesc), map[string]interface{}{
			"substep_name": fmt.Sprintf("layer %d/%d", i+1, totalLayers),
			"layer_type":   layerType,
			"status":       "completed",
		})
	}
}

// DockerLayer represents a Docker instruction layer
type DockerLayer struct {
	Command string
	Preview string
}

// extractDockerLayers parses Dockerfile content to extract layer information
func extractDockerLayers(dockerfileContent string) []DockerLayer {
	lines := strings.Split(dockerfileContent, "\n")
	layers := make([]DockerLayer, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		command := strings.ToUpper(parts[0])
		preview := parts[1]
		if len(preview) > 50 {
			preview = preview[:47] + "..."
		}

		// Only include layer-creating commands
		if isLayerCreatingCommand(command) {
			layers = append(layers, DockerLayer{
				Command: command,
				Preview: preview,
			})
		}
	}

	return layers
}

// estimateDockerLayers estimates the number of layers from Dockerfile content
func estimateDockerLayers(dockerfileContent string) int {
	layers := extractDockerLayers(dockerfileContent)
	count := len(layers)

	// Add some buffer for base image layers
	count += 2

	if count > 20 {
		count = 20 // Cap at reasonable maximum
	}

	return count
}

// isLayerCreatingCommand checks if a Docker command creates a new layer
func isLayerCreatingCommand(command string) bool {
	layerCreatingCommands := map[string]bool{
		"FROM":       true,
		"RUN":        true,
		"COPY":       true,
		"ADD":        true,
		"WORKDIR":    false, // Metadata only
		"ENV":        false, // Metadata only
		"EXPOSE":     false, // Metadata only
		"CMD":        false, // Metadata only
		"ENTRYPOINT": false, // Metadata only
	}

	creates, exists := layerCreatingCommands[command]
	return exists && creates
}

// getLayerBuildTime returns simulated build time based on layer type
func getLayerBuildTime(layerType string) time.Duration {
	switch layerType {
	case "FROM":
		return 500 * time.Millisecond // Base image pull
	case "RUN":
		return 2 * time.Second // Execution time
	case "COPY", "ADD":
		return 300 * time.Millisecond // File operations
	default:
		return 200 * time.Millisecond // Metadata operations
	}
}
