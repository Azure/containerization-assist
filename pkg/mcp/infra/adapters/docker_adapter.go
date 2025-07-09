package adapters

import (
	"context"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
)

// DockerClientAdapter adapts the core docker client to the application interface
type DockerClientAdapter struct {
	client docker.Client
}

// NewDockerClient creates a new Docker client adapter
func NewDockerClient(client docker.Client) services.DockerClient {
	return &DockerClientAdapter{
		client: client,
	}
}

// Build builds a Docker image
func (d *DockerClientAdapter) Build(ctx context.Context, options services.DockerBuildOptions) (*services.DockerBuildResult, error) {
	buildOptions := docker.BuildOptions{
		Context:    options.Context,
		Dockerfile: options.Dockerfile,
		Tags:       options.Tags,
		BuildArgs:  options.BuildArgs,
		Target:     options.Target,
		Platform:   options.Platform,
		NoCache:    options.NoCache,
		PullParent: options.PullParent,
		Labels:     options.Labels,
	}

	result, err := d.client.Build(ctx, buildOptions)
	if err != nil {
		return nil, err
	}

	return &services.DockerBuildResult{
		ImageID:     result.ImageID,
		Size:        result.Size,
		Logs:        result.Logs,
		Layers:      result.Layers,
		CacheHits:   result.CacheHits,
		CacheMisses: result.CacheMisses,
	}, nil
}

// Push pushes an image to a registry
func (d *DockerClientAdapter) Push(ctx context.Context, imageRef string, options services.DockerPushOptions) error {
	pushOptions := docker.PushOptions{
		Registry: options.Registry,
		Tag:      options.Tag,
	}

	return d.client.Push(ctx, imageRef, pushOptions)
}

// Pull pulls an image from a registry
func (d *DockerClientAdapter) Pull(ctx context.Context, imageRef string, options services.DockerPullOptions) (*services.DockerPullResult, error) {
	pullOptions := docker.PullOptions{
		Registry: options.Registry,
		Tag:      options.Tag,
	}

	result, err := d.client.Pull(ctx, imageRef, pullOptions)
	if err != nil {
		return nil, err
	}

	return &services.DockerPullResult{
		ImageID: result.ImageID,
		Size:    result.Size,
	}, nil
}

// Tag tags an image
func (d *DockerClientAdapter) Tag(ctx context.Context, sourceImage, targetImage string) error {
	return d.client.Tag(ctx, sourceImage, targetImage)
}

// ImageExists checks if an image exists locally
func (d *DockerClientAdapter) ImageExists(ctx context.Context, imageRef string) (bool, error) {
	return d.client.ImageExists(ctx, imageRef)
}

// GetImageInfo retrieves image information
func (d *DockerClientAdapter) GetImageInfo(ctx context.Context, imageRef string) (*services.DockerImageInfo, error) {
	info, err := d.client.GetImageInfo(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	return &services.DockerImageInfo{
		ID:       info.ID,
		RepoTags: info.RepoTags,
		Size:     info.Size,
		Created:  info.Created,
		Labels:   info.Labels,
	}, nil
}
