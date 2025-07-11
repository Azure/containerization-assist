package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/core/kind"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
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
func BuildImage(ctx context.Context, dockerfileResult *DockerfileResult, imageName, imageTag, buildContext string, logger *slog.Logger) (*BuildResult, error) {
	logger.Info("Starting Docker image build",
		"image_name", imageName,
		"image_tag", imageTag,
		"build_context", buildContext)

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
	dockerClient := docker.NewDockerCmdRunner(&runner.DefaultCommandRunner{})

	// Create Docker service using pkg/core/docker
	dockerService := docker.NewService(dockerClient, logger.With("component", "docker_service"))

	// Build the image using the core Docker functionality
	buildOptions := docker.BuildOptions{
		ImageName: imageName,
		Registry:  "", // Use default registry
		NoCache:   false,
		Platform:  "", // Use default platform
		BuildArgs: dockerfileResult.BuildArgs,
		Tags:      []string{imageTag},
	}

	logger.Info("Building Docker image with QuickBuild",
		"dockerfile_length", len(dockerfileResult.Content),
		"build_context", buildContext)

	buildResult, err := dockerService.QuickBuild(ctx, dockerfileResult.Content, buildContext, buildOptions)
	if err != nil {
		logger.Error("Docker build failed", "error", err, "image_name", imageName)
		return nil, fmt.Errorf("docker build failed: %v", err)
	}

	if !buildResult.Success {
		logger.Error("Docker build unsuccessful", "error", buildResult.Error)
		return nil, fmt.Errorf("docker build unsuccessful: %v", buildResult.Error)
	}

	buildDuration := time.Since(startTime)
	logger.Info("Docker build completed successfully",
		"image_id", buildResult.ImageID,
		"duration", buildDuration,
		"image_ref", buildResult.ImageRef)

	return &BuildResult{
		ImageName: imageName,
		ImageTag:  imageTag,
		ImageID:   buildResult.ImageID,
		BuildTime: startTime,
	}, nil
}

// PushImage pushes a Docker image to a registry using real Docker operations
func PushImage(ctx context.Context, buildResult *BuildResult, registry string, logger *slog.Logger) (string, error) {
	if buildResult == nil {
		return "", fmt.Errorf("build result is required")
	}

	logger.Info("Starting Docker image push",
		"image_name", buildResult.ImageName,
		"image_tag", buildResult.ImageTag,
		"registry", registry)

	// Default registry if not provided
	if registry == "" {
		registry = "localhost:5001" // Local registry for kind clusters
	}

	// Initialize Docker client and service
	dockerClient := docker.NewDockerCmdRunner(&runner.DefaultCommandRunner{})
	dockerService := docker.NewService(dockerClient, logger.With("component", "docker_service"))

	// Construct image reference for the target registry
	imageRef := fmt.Sprintf("%s/%s:%s", registry, buildResult.ImageName, buildResult.ImageTag)

	logger.Info("Pushing Docker image with QuickPush", "image_ref", imageRef)

	// Use QuickPush for pushing the image
	pushOptions := docker.PushOptions{
		Registry:   registry,
		RetryCount: 3,
		Timeout:    5 * time.Minute,
	}

	pushResult, err := dockerService.QuickPush(ctx, imageRef, pushOptions)
	if err != nil {
		logger.Error("Docker push failed", "error", err, "image_ref", imageRef)
		return "", fmt.Errorf("docker push failed: %v", err)
	}

	if !pushResult.Success {
		logger.Error("Docker push unsuccessful", "error", pushResult.Error)
		return "", fmt.Errorf("docker push unsuccessful: %v", pushResult.Error)
	}

	logger.Info("Image pushed successfully", "image_ref", imageRef)
	return imageRef, nil
}

// LoadImageToKind tags and pushes a Docker image to the kind cluster's local registry
func LoadImageToKind(ctx context.Context, buildResult *BuildResult, clusterName string, logger *slog.Logger) error {
	if buildResult == nil {
		return fmt.Errorf("build result is required")
	}

	logger.Info("Loading image into kind cluster registry",
		"image_name", buildResult.ImageName,
		"image_tag", buildResult.ImageTag,
		"cluster", clusterName)

	// Default cluster name
	if clusterName == "" {
		clusterName = "container-kit"
	}

	// Construct image references
	sourceImageRef := fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag)
	targetImageRef := fmt.Sprintf("localhost:5001/%s:%s", buildResult.ImageName, buildResult.ImageTag)

	logger.Info("Tagging image for local registry", "source", sourceImageRef, "target", targetImageRef)

	// First, tag the image for the local registry
	tagCmd := exec.CommandContext(ctx, "docker", "tag", sourceImageRef, targetImageRef)
	if output, err := tagCmd.CombinedOutput(); err != nil {
		logger.Error("Failed to tag image for local registry",
			"error", err,
			"output", string(output),
			"source", sourceImageRef,
			"target", targetImageRef)
		return fmt.Errorf("failed to tag image: %v, output: %s", err, string(output))
	}

	logger.Info("Pushing image to local registry", "image_ref", targetImageRef)

	// Push to the local registry
	pushCmd := exec.CommandContext(ctx, "docker", "push", targetImageRef)
	output, err := pushCmd.CombinedOutput()

	if err != nil {
		logger.Error("Failed to push image to local registry",
			"error", err,
			"output", string(output),
			"image_ref", targetImageRef)
		return fmt.Errorf("failed to push image to local registry: %v, output: %s", err, string(output))
	}

	logger.Info("Image pushed to local registry successfully",
		"image_ref", targetImageRef,
		"output", string(output))
	return nil
}

// SetupKindCluster creates or ensures a kind cluster with local registry exists
func SetupKindCluster(ctx context.Context, clusterName string, logger *slog.Logger) (string, error) {
	logger.Info("Setting up kind cluster with local registry", "cluster", clusterName)

	// Initialize clients
	dockerClient := docker.NewDockerCmdRunner(&runner.DefaultCommandRunner{})
	kindRunner := kind.NewKindCmdRunner(&runner.DefaultCommandRunner{})

	// Use the GetKindCluster function that handles cluster creation and registry setup
	registryURL, err := kubernetes.GetKindCluster(ctx, kindRunner, dockerClient)
	if err != nil {
		logger.Error("Failed to setup kind cluster", "error", err, "cluster", clusterName)
		return "", fmt.Errorf("failed to setup kind cluster: %v", err)
	}

	logger.Info("Kind cluster with local registry setup completed",
		"cluster", clusterName,
		"registry_url", registryURL)

	return registryURL, nil
}
