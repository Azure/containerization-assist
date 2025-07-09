package infra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/infra/retry"
)

// DockerServiceFactory creates Docker service implementations
// This eliminates the adapter pattern by providing direct implementations
type DockerServiceFactory struct {
	coreService docker.Service
}

// NewDockerServiceFactory creates a factory for Docker services
func NewDockerServiceFactory(coreService docker.Service) *DockerServiceFactory {
	return &DockerServiceFactory{
		coreService: coreService,
	}
}

// CreateDockerService creates a Docker service implementation
// Returns an interface that can be used directly without adapters
func (f *DockerServiceFactory) CreateDockerService() DockerService {
	return &dockerServiceImpl{
		coreService: f.coreService,
	}
}

// DockerService defines the interface that eliminates the need for adapters
type DockerService interface {
	Build(ctx context.Context, options DockerBuildOptions) (*DockerBuildResult, error)
	Push(ctx context.Context, imageRef string, options DockerPushOptions) error
	Pull(ctx context.Context, imageRef string, options DockerPullOptions) (*DockerPullResult, error)
	Tag(ctx context.Context, sourceImage, targetImage string) error
	ImageExists(ctx context.Context, imageRef string) (bool, error)
	GetImageInfo(ctx context.Context, imageRef string) (*DockerImageInfo, error)
	CheckPrerequisites(ctx context.Context) error
}

// DockerBuildOptions represents Docker build options
type DockerBuildOptions struct {
	Context    string
	Dockerfile string
	Tags       []string
	BuildArgs  map[string]string
	Platform   string
	NoCache    bool
	Target     string
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
	Duration    time.Duration
}

// DockerPushOptions represents Docker push options
type DockerPushOptions struct {
	Registry string
	Tag      string
	Auth     map[string]string
}

// DockerPullOptions represents Docker pull options
type DockerPullOptions struct {
	Registry string
	Auth     map[string]string
	Platform string
}

// DockerPullResult represents the result of a Docker pull
type DockerPullResult struct {
	ImageID string
	Size    int64
	Layers  []string
}

// DockerImageInfo represents Docker image information
type DockerImageInfo struct {
	ID       string
	RepoTags []string
	Size     int64
	Created  int64
	Labels   map[string]string
	Config   map[string]interface{}
}

// dockerServiceImpl implements DockerService
type dockerServiceImpl struct {
	coreService docker.Service
}

// Build implements DockerService
func (d *dockerServiceImpl) Build(ctx context.Context, options DockerBuildOptions) (*DockerBuildResult, error) {
	if options.Dockerfile == "" {
		return nil, fmt.Errorf("dockerfile content is required")
	}

	coreOptions := docker.BuildOptions{
		ImageName: extractImageName(options.Tags),
		BuildArgs: options.BuildArgs,
		Platform:  options.Platform,
		NoCache:   options.NoCache,
		Tags:      options.Tags,
	}

	start := time.Now()
	result, err := d.coreService.QuickBuild(ctx, options.Dockerfile, options.Context, coreOptions)
	if err != nil {
		return nil, err
	}

	return &DockerBuildResult{
		ImageID:  result.ImageID,
		Size:     0, // Not available in core result
		Logs:     result.Logs,
		Duration: time.Since(start),
	}, nil
}

// Push implements DockerService
func (d *dockerServiceImpl) Push(ctx context.Context, imageRef string, options DockerPushOptions) error {
	coreOptions := docker.PushOptions{
		Registry: options.Registry,
		Tag:      options.Tag,
	}

	result, err := d.coreService.QuickPush(ctx, imageRef, coreOptions)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("push failed: %s", result.Error.Message)
	}

	return nil
}

// Pull implements DockerService
func (d *dockerServiceImpl) Pull(ctx context.Context, imageRef string, _ DockerPullOptions) (*DockerPullResult, error) {
	result, err := d.coreService.QuickPull(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("pull failed: %s", result.Error.Message)
	}

	return &DockerPullResult{
		ImageID: result.ImageRef,
		Size:    0, // Not available in core result
	}, nil
}

// Tag implements DockerService
func (d *dockerServiceImpl) Tag(_ context.Context, _, _ string) error {
	return fmt.Errorf("tag operation not yet implemented in core service")
}

// ImageExists implements DockerService
func (d *dockerServiceImpl) ImageExists(_ context.Context, _ string) (bool, error) {
	return false, fmt.Errorf("image exists check not yet implemented in core service")
}

// GetImageInfo implements DockerService
func (d *dockerServiceImpl) GetImageInfo(_ context.Context, imageRef string) (*DockerImageInfo, error) {
	return &DockerImageInfo{
		ID:       "",
		RepoTags: []string{imageRef},
		Size:     0,
		Created:  time.Now().Unix(),
		Labels:   make(map[string]string),
	}, nil
}

// CheckPrerequisites implements DockerService
func (d *dockerServiceImpl) CheckPrerequisites(ctx context.Context) error {
	return d.coreService.CheckPrerequisites(ctx)
}

// RetryServiceFactory creates retry service implementations
type RetryServiceFactory struct{}

// NewRetryServiceFactory creates a factory for retry services
func NewRetryServiceFactory() *RetryServiceFactory {
	return &RetryServiceFactory{}
}

// CreateRetryService creates a retry service implementation
func (f *RetryServiceFactory) CreateRetryService() RetryService {
	return &retryServiceImpl{
		coordinator: retry.New(),
		policies:    make(map[string]*RetryPolicy),
	}
}

// RetryService defines the interface that eliminates the need for adapters
type RetryService interface {
	SetPolicy(operationType string, policy *RetryPolicy)
	GetPolicy(operationType string) *RetryPolicy
	ExecuteWithRetry(ctx context.Context, operationType string, operation func() error) error
	ShouldRetry(err error, operationType string) bool
}

// RetryPolicy defines retry behavior for operations
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffStrategy string
	Multiplier      float64
	Jitter          bool
	ErrorPatterns   []string
}

// retryServiceImpl implements RetryService
type retryServiceImpl struct {
	coordinator *retry.Coordinator
	policies    map[string]*RetryPolicy
}

// SetPolicy implements RetryService
func (r *retryServiceImpl) SetPolicy(operationType string, policy *RetryPolicy) {
	r.policies[operationType] = policy

	corePolicy := &retry.Policy{
		MaxAttempts:     policy.MaxAttempts,
		InitialDelay:    policy.InitialDelay,
		MaxDelay:        policy.MaxDelay,
		BackoffStrategy: retry.BackoffStrategy(policy.BackoffStrategy),
		Multiplier:      policy.Multiplier,
		Jitter:          policy.Jitter,
		ErrorPatterns:   policy.ErrorPatterns,
	}

	r.coordinator.SetPolicy(operationType, corePolicy)
}

// GetPolicy implements RetryService
func (r *retryServiceImpl) GetPolicy(operationType string) *RetryPolicy {
	if policy, exists := r.policies[operationType]; exists {
		return policy
	}

	return &RetryPolicy{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        10 * time.Second,
		BackoffStrategy: "exponential",
		Multiplier:      2.0,
		Jitter:          true,
		ErrorPatterns:   []string{"timeout", "connection refused", "service unavailable"},
	}
}

// ExecuteWithRetry implements RetryService
func (r *retryServiceImpl) ExecuteWithRetry(ctx context.Context, operationType string, operation func() error) error {
	return r.coordinator.Execute(ctx, operationType, func(_ context.Context) error {
		return operation()
	})
}

// ShouldRetry implements RetryService
func (r *retryServiceImpl) ShouldRetry(err error, _ string) bool {
	return r.coordinator.IsRetryable(err)
}

// extractImageName extracts the image name from a list of tags
func extractImageName(tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	tag := tags[0]
	if colonIndex := strings.LastIndex(tag, ":"); colonIndex != -1 {
		return tag[:colonIndex]
	}
	return tag
}
