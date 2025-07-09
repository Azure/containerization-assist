package services

import (
	"context"
)

// DockerClient provides Docker operations interface
type DockerClient interface {
	// Build builds a Docker image
	Build(ctx context.Context, options DockerBuildOptions) (*DockerBuildResult, error)

	// Push pushes an image to a registry
	Push(ctx context.Context, imageRef string, options DockerPushOptions) error

	// Pull pulls an image from a registry
	Pull(ctx context.Context, imageRef string, options DockerPullOptions) (*DockerPullResult, error)

	// Tag tags an image
	Tag(ctx context.Context, sourceImage, targetImage string) error

	// ImageExists checks if an image exists locally
	ImageExists(ctx context.Context, imageRef string) (bool, error)

	// GetImageInfo retrieves image information
	GetImageInfo(ctx context.Context, imageRef string) (*DockerImageInfo, error)
}

// DockerBuildOptions represents options for building Docker images
type DockerBuildOptions struct {
	Context    string
	Dockerfile string
	Tags       []string
	BuildArgs  map[string]string
	Target     string
	Platform   string
	NoCache    bool
	PullParent bool
	Labels     map[string]string
}

// DockerBuildResult represents the result of a Docker build
type DockerBuildResult struct {
	ImageID     string
	Size        int64
	Logs        []string
	Layers      int
	CacheHits   int
	CacheMisses int
}

// DockerPushOptions represents options for pushing images
type DockerPushOptions struct {
	Registry string
	Tag      string
}

// DockerPullOptions represents options for pulling images
type DockerPullOptions struct {
	Registry string
	Tag      string
}

// DockerPullResult represents the result of a Docker pull
type DockerPullResult struct {
	ImageID string
	Size    int64
}

// DockerImageInfo represents Docker image information
type DockerImageInfo struct {
	ID       string
	RepoTags []string
	Size     int64
	Created  int64
	Labels   map[string]string
}
