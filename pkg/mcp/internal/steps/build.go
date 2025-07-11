package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/Azure/container-kit/pkg/clients"
	"github.com/Azure/container-kit/pkg/core/docker"
	dockerpkg "github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	"github.com/Azure/container-kit/pkg/runner"
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

	// Initialize clients with proper Docker client
	dockerClients := &clients.Clients{
		Docker: dockerpkg.NewDockerCmdRunner(&runner.DefaultCommandRunner{}),
		Kind:   kind.NewKindCmdRunner(&runner.DefaultCommandRunner{}),
		Kube:   k8s.NewKubeCmdRunner(&runner.DefaultCommandRunner{}),
	}

	// Create Docker service using pkg/core/docker
	dockerService := docker.NewService(dockerClients, logger.With("component", "docker_service"))

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
	logger.Info("Starting Docker image push",
		"image_name", buildResult.ImageName,
		"image_tag", buildResult.ImageTag,
		"registry", registry)

	if buildResult == nil {
		return "", fmt.Errorf("build result is required")
	}

	// Default registry if not provided
	if registry == "" {
		registry = "localhost:5001" // Local registry for kind clusters
	}

	// Initialize clients with proper Docker client
	dockerClients := &clients.Clients{
		Docker: dockerpkg.NewDockerCmdRunner(&runner.DefaultCommandRunner{}),
		Kind:   kind.NewKindCmdRunner(&runner.DefaultCommandRunner{}),
		Kube:   k8s.NewKubeCmdRunner(&runner.DefaultCommandRunner{}),
	}
	dockerService := docker.NewService(dockerClients, logger.With("component", "docker_service"))

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

// LoadImageToKind loads a Docker image directly into a kind cluster using real kind operations
func LoadImageToKind(ctx context.Context, buildResult *BuildResult, clusterName string, logger *slog.Logger) error {
	logger.Info("Loading image into kind cluster",
		"image_name", buildResult.ImageName,
		"image_tag", buildResult.ImageTag,
		"cluster", clusterName)

	if buildResult == nil {
		return fmt.Errorf("build result is required")
	}

	// Default cluster name
	if clusterName == "" {
		clusterName = "container-kit"
	}

	// Construct image reference
	imageRef := fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag)

	logger.Info("Loading image into kind cluster using kind load command", "image_ref", imageRef, "cluster", clusterName)

	// Use kind load docker-image command directly
	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", imageRef, "--name", clusterName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("Failed to load image to kind cluster",
			"error", err,
			"output", string(output),
			"image_ref", imageRef,
			"cluster", clusterName)
		return fmt.Errorf("failed to load image to kind: %v, output: %s", err, string(output))
	}

	logger.Info("Image loaded into kind cluster successfully",
		"image_ref", imageRef,
		"cluster", clusterName,
		"output", string(output))
	return nil
}

// SetupKindCluster creates or ensures a kind cluster with local registry exists
func SetupKindCluster(ctx context.Context, clusterName string, logger *slog.Logger) (string, error) {
	logger.Info("Setting up kind cluster with local registry", "cluster", clusterName)

	// Initialize clients with proper Docker client
	dockerClients := &clients.Clients{
		Docker: dockerpkg.NewDockerCmdRunner(&runner.DefaultCommandRunner{}),
		Kind:   kind.NewKindCmdRunner(&runner.DefaultCommandRunner{}),
		Kube:   k8s.NewKubeCmdRunner(&runner.DefaultCommandRunner{}),
	}

	// Use the real GetKindCluster method that handles cluster creation and registry setup
	registryURL, err := dockerClients.GetKindCluster(ctx)
	if err != nil {
		logger.Error("Failed to setup kind cluster", "error", err, "cluster", clusterName)
		return "", fmt.Errorf("failed to setup kind cluster: %v", err)
	}

	logger.Info("Kind cluster with local registry setup completed",
		"cluster", clusterName,
		"registry_url", registryURL)

	return registryURL, nil
}
