package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/infrastructure/container"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
	"github.com/Azure/containerization-assist/pkg/infrastructure/kubernetes"
	"github.com/Azure/containerization-assist/pkg/infrastructure/kubernetes/kind"
)

// BuildResult contains the results of a Docker build operation
type BuildResult struct {
	ImageName string    `json:"image_name"`
	ImageTag  string    `json:"image_tag"`
	ImageID   string    `json:"image_id"`
	BuildTime time.Time `json:"build_time"`
	Size      int64     `json:"size,omitempty"`
}

// BuildImage builds a Docker image from a Dockerfile using real Docker operations
func BuildImage(ctx context.Context, dockerfileResult *workflow.DockerfileResult, imageName, imageTag, buildContext string, logger *slog.Logger) (*BuildResult, error) {

	if dockerfileResult == nil {
		return nil, fmt.Errorf("dockerfile result is required")
	}

	if imageName == "" {
		return nil, fmt.Errorf("image name is required")
	}

	// Default tag if not provided
	if imageTag == "" {
		imageTag = "latest"
	}

	// Default build context if not provided
	if buildContext == "" {
		buildContext = "."
	}

	startTime := time.Now()

	// Initialize Docker client
	dockerClient := container.NewDockerCmdRunner(&core.DefaultCommandRunner{})

	// Create Docker service using container infrastructure
	dockerService := container.NewService(dockerClient, logger)

	// Build the image using the core Docker functionality
	buildOptions := container.BuildOptions{
		ImageName: imageName,
		Registry:  "", // Use default registry
		NoCache:   false,
		Platform:  "",  // Use default platform
		BuildArgs: nil, // Build args not provided in Dockerfile generation
		Tags:      []string{imageTag},
	}

	buildResult, err := dockerService.QuickBuild(ctx, dockerfileResult.Content, buildContext, buildOptions)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %v", err)
	}

	if !buildResult.Success {

		// Debug: Log what we received
		if buildResult.Error != nil {
		} else {
		}

		// Extract detailed error information from BuildError if available
		if buildResult.Error != nil {
			errorMsg := fmt.Sprintf("Docker build failed: %s", buildResult.Error.Message)
			if buildResult.Error.BuildLogs != "" {
				errorMsg += fmt.Sprintf("\nBuild output:\n%s", buildResult.Error.BuildLogs)
			}
			if buildResult.Error.Command != "" {
				errorMsg += fmt.Sprintf("\nCommand: %s", buildResult.Error.Command)
			}
			if buildResult.Error.ExitCode != 0 {
				errorMsg += fmt.Sprintf("\nExit code: %d", buildResult.Error.ExitCode)
			}
			return nil, fmt.Errorf("%s", errorMsg)
		}
		return nil, fmt.Errorf("docker build unsuccessful: %v", buildResult.Error)
	}

	_ = time.Since(startTime)

	// Get image size using docker inspect
	imageSize := int64(0)
	if buildResult.ImageID == "" {
	} else {
		dockerClient := container.NewDockerCmdRunner(&core.DefaultCommandRunner{})
		inspectOutput, err := dockerClient.Inspect(ctx, buildResult.ImageID)
		if err != nil {
		} else {
			imageSize = extractImageSizeFromInspect(inspectOutput, logger)
		}
	}

	return &BuildResult{
		ImageName: imageName,
		ImageTag:  imageTag,
		ImageID:   buildResult.ImageID,
		BuildTime: startTime,
		Size:      imageSize,
	}, nil
}

// PushImage pushes a Docker image to a registry using real Docker operations
func PushImage(ctx context.Context, buildResult *BuildResult, registry string, logger *slog.Logger) (string, error) {
	if buildResult == nil {
		return "", fmt.Errorf("build result is required")
	}

	// Default registry if not provided
	if registry == "" {
		registry = "localhost:5001" // Local registry for kind clusters
	}

	// Initialize Docker client and service
	dockerClient := container.NewDockerCmdRunner(&core.DefaultCommandRunner{})
	dockerService := container.NewService(dockerClient, logger)

	// Construct image reference for the target registry
	imageRef := fmt.Sprintf("%s/%s:%s", registry, buildResult.ImageName, buildResult.ImageTag)

	// Use QuickPush for pushing the image
	pushOptions := container.PushOptions{
		Registry:   registry,
		RetryCount: 3,
		Timeout:    5 * time.Minute,
	}

	pushResult, err := dockerService.QuickPush(ctx, imageRef, pushOptions)
	if err != nil {
		return "", fmt.Errorf("docker push failed: %v", err)
	}

	if !pushResult.Success {
		return "", fmt.Errorf("docker push unsuccessful: %v", pushResult.Error)
	}

	return imageRef, nil
}

// LoadImageToKind tags and pushes a Docker image to the kind cluster's local registry
func LoadImageToKind(ctx context.Context, buildResult *BuildResult, clusterName string, logger *slog.Logger) error {
	if buildResult == nil {
		return fmt.Errorf("build result is required")
	}

	// Default cluster name
	if clusterName == "" {
		clusterName = "containerization-assist"
	}

	// Construct image references
	sourceImageRef := fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag)
	targetImageRef := fmt.Sprintf("localhost:5001/%s:%s", buildResult.ImageName, buildResult.ImageTag)

	// First, tag the image for the local registry
	tagCmd := exec.CommandContext(ctx, "docker", "tag", sourceImageRef, targetImageRef)
	if output, err := tagCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to tag image: %v, output: %s", err, string(output))
	}

	// Push to the local registry
	pushCmd := exec.CommandContext(ctx, "docker", "push", targetImageRef)
	output, err := pushCmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to push image to local registry: %v, output: %s", err, string(output))
	}

	return nil
}

// SetupKindCluster creates or ensures a kind cluster with local registry exists
func SetupKindCluster(ctx context.Context, clusterName string, logger *slog.Logger) (string, error) {

	// Initialize clients
	dockerClient := container.NewDockerCmdRunner(&core.DefaultCommandRunner{})
	kindRunner := kind.NewKindCmdRunner(&core.DefaultCommandRunner{})

	// Use the GetKindCluster function that handles cluster creation and registry setup
	registryURL, err := kubernetes.GetKindCluster(ctx, kindRunner, dockerClient)
	if err != nil {
		return "", fmt.Errorf("failed to setup kind cluster: %v", err)
	}

	return registryURL, nil
}

// extractImageSizeFromInspect extracts the image size from docker inspect JSON output
func extractImageSizeFromInspect(inspectOutput string, logger *slog.Logger) int64 {
	var inspectData []struct {
		Size int64 `json:"Size"`
	}

	if err := json.Unmarshal([]byte(inspectOutput), &inspectData); err != nil {
		return 0
	}

	if len(inspectData) > 0 {
		return inspectData[0].Size
	}

	return 0
}

// formatBytes converts bytes to human-readable format (B, KB, MB, GB)
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
