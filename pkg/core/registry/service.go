package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// Service provides a unified interface to container registry operations
type Service interface {
	// Registry operations
	Login(ctx context.Context, registry string, auth AuthConfig) error
	Logout(ctx context.Context, registry string) error

	// Image operations
	Push(ctx context.Context, image string, options PushOptions) (*PushResult, error)
	Pull(ctx context.Context, image string, options PullOptions) (*PullResult, error)
	Tag(ctx context.Context, source, target string) error

	// Repository operations
	ListRepositories(ctx context.Context, registry string) ([]Repository, error)
	GetRepository(ctx context.Context, registry, name string) (*Repository, error)
	CreateRepository(ctx context.Context, registry string, repo CreateRepositoryOptions) error
	DeleteRepository(ctx context.Context, registry, name string) error

	// Tag operations
	ListTags(ctx context.Context, registry, repository string) ([]Tag, error)
	GetTagInfo(ctx context.Context, registry, repository, tag string) (*TagInfo, error)
	DeleteTag(ctx context.Context, registry, repository, tag string) error

	// Registry info
	GetRegistryInfo(ctx context.Context, registry string) (*Info, error)
	TestConnection(ctx context.Context, registry string) error
}

// ServiceImpl implements the Registry Service interface
type ServiceImpl struct {
	logger      *slog.Logger
	authCache   map[string]*AuthConfig
	authMutex   sync.RWMutex
	connections map[string]*Connection
	connMutex   sync.RWMutex
}

// NewRegistryService creates a new Registry service
func NewRegistryService(logger *slog.Logger) Service {
	return &ServiceImpl{
		logger:      logger.With("component", "registry_service"),
		authCache:   make(map[string]*AuthConfig),
		connections: make(map[string]*Connection),
	}
}

// Supporting types

// AuthConfig contains registry authentication
type AuthConfig struct {
	Username      string
	Password      string
	Token         string
	IdentityToken string
	ServerAddress string
}

// PushOptions contains options for push operations
type PushOptions struct {
	Registry            string
	Repository          string
	Tag                 string
	Auth                *AuthConfig
	AllTags             bool
	DisableContentTrust bool
}

// PullOptions contains options for pull operations
type PullOptions struct {
	Registry            string
	Platform            string
	Auth                *AuthConfig
	AllTags             bool
	DisableContentTrust bool
}

// PushResult contains push operation results
type PushResult struct {
	Digest   string
	Size     int64
	Duration time.Duration
	Tags     []string
}

// PullResult contains pull operation results
type PullResult struct {
	ImageID  string
	Size     int64
	Duration time.Duration
	Digest   string
}

// Repository represents a container repository
type Repository struct {
	Name        string
	Description string
	Private     bool
	TagCount    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Stars       int
	Pulls       int64
}

// Tag represents a container image tag
type Tag struct {
	Name         string
	Digest       string
	Size         int64
	CreatedAt    time.Time
	Architecture string
	OS           string
}

// TagInfo contains detailed tag information
type TagInfo struct {
	Tag             Tag
	Manifest        map[string]interface{}
	Config          map[string]interface{}
	History         []LayerHistory
	Vulnerabilities []Vulnerability
}

// LayerHistory represents a layer in the image history
type LayerHistory struct {
	Created    time.Time
	CreatedBy  string
	Size       int64
	EmptyLayer bool
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string
	Severity    string
	Package     string
	Version     string
	FixVersion  string
	Description string
}

// CreateRepositoryOptions contains options for creating repositories
type CreateRepositoryOptions struct {
	Name        string
	Description string
	Private     bool
	AutoInit    bool
}

// Info contains registry information
type Info struct {
	Name          string
	URL           string
	Type          string
	Version       string
	Features      []string
	StorageDriver string
	AuthSupported bool
}

// Type aliases for backward compatibility
//
//nolint:revive // These aliases are needed for backward compatibility
type RegistryInfo = Info

// Connection represents a registry connection
type Connection struct {
	Registry  string
	Auth      *AuthConfig
	Connected bool
	LastTest  time.Time
}

// Login authenticates with a registry
func (s *ServiceImpl) Login(ctx context.Context, registry string, auth AuthConfig) error {
	s.logger.Info("Logging into registry", "registry", registry)

	s.authMutex.Lock()
	defer s.authMutex.Unlock()

	// Test the connection first
	if err := s.testConnection(ctx, registry, &auth); err != nil {
		return errors.New(errors.CodePermissionDenied, "registry", fmt.Sprintf("failed to login to registry: %s", registry), err)
	}

	// Store the auth config
	s.authCache[registry] = &auth

	// Update connection status
	s.connMutex.Lock()
	s.connections[registry] = &Connection{
		Registry:  registry,
		Auth:      &auth,
		Connected: true,
		LastTest:  time.Now(),
	}
	s.connMutex.Unlock()

	s.logger.Info("Successfully logged into registry", "registry", registry)
	return nil
}

// Logout removes authentication for a registry
func (s *ServiceImpl) Logout(_ context.Context, registry string) error {
	s.logger.Info("Logging out of registry", "registry", registry)

	s.authMutex.Lock()
	delete(s.authCache, registry)
	s.authMutex.Unlock()

	s.connMutex.Lock()
	delete(s.connections, registry)
	s.connMutex.Unlock()

	return nil
}

// Push pushes an image to a registry
func (s *ServiceImpl) Push(_ context.Context, image string, options PushOptions) (*PushResult, error) {
	s.logger.Info("Pushing image to registry", "image", image, "registry", options.Registry)

	startTime := time.Now()

	// Validate auth
	if err := s.validateAuth(options.Registry, options.Auth); err != nil {
		return nil, err
	}

	// Simulate push operation (in real implementation, this would use Docker API)
	result := &PushResult{
		Digest:   fmt.Sprintf("sha256:%x", time.Now().UnixNano()),
		Size:     1024 * 1024 * 50, // 50MB
		Duration: time.Since(startTime),
		Tags:     []string{options.Tag},
	}

	s.logger.Info("Successfully pushed image", "image", image, "digest", result.Digest)
	return result, nil
}

// Pull pulls an image from a registry
func (s *ServiceImpl) Pull(_ context.Context, image string, options PullOptions) (*PullResult, error) {
	s.logger.Info("Pulling image from registry", "image", image, "registry", options.Registry)

	startTime := time.Now()

	// Validate auth if provided
	if options.Auth != nil {
		if err := s.validateAuth(options.Registry, options.Auth); err != nil {
			return nil, err
		}
	}

	// Simulate pull operation (in real implementation, this would use Docker API)
	result := &PullResult{
		ImageID:  fmt.Sprintf("sha256:%x", time.Now().UnixNano()),
		Size:     1024 * 1024 * 45, // 45MB
		Duration: time.Since(startTime),
		Digest:   fmt.Sprintf("sha256:%x", time.Now().UnixNano()),
	}

	s.logger.Info("Successfully pulled image", "image", image, "imageID", result.ImageID)
	return result, nil
}

// Tag creates a new tag for an image
func (s *ServiceImpl) Tag(_ context.Context, source, target string) error {
	s.logger.Info("Tagging image", "source", source, "target", target)

	// Simulate tagging (in real implementation, this would use Docker API)
	s.logger.Info("Successfully tagged image", "source", source, "target", target)
	return nil
}

// ListRepositories lists repositories in a registry
func (s *ServiceImpl) ListRepositories(ctx context.Context, registry string) ([]Repository, error) {
	s.logger.Info("Listing repositories", "registry", registry)

	// Validate connection
	if err := s.TestConnection(ctx, registry); err != nil {
		return nil, err
	}

	// Simulate repository listing
	repos := []Repository{
		{
			Name:        "my-app",
			Description: "My application container",
			Private:     false,
			TagCount:    5,
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-1 * time.Hour),
			Stars:       3,
			Pulls:       150,
		},
		{
			Name:        "backend-service",
			Description: "Backend microservice",
			Private:     true,
			TagCount:    12,
			CreatedAt:   time.Now().Add(-60 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-2 * time.Hour),
			Stars:       0,
			Pulls:       75,
		},
	}

	return repos, nil
}

// GetRepository gets information about a specific repository
func (s *ServiceImpl) GetRepository(ctx context.Context, registry, name string) (*Repository, error) {
	s.logger.Info("Getting repository info", "registry", registry, "repository", name)

	repos, err := s.ListRepositories(ctx, registry)
	if err != nil {
		return nil, err
	}

	for _, repo := range repos {
		if repo.Name == name {
			return &repo, nil
		}
	}

	return nil, errors.New(errors.CodeNotFound, "registry", fmt.Sprintf("repository not found: %s/%s", registry, name), nil)
}

// CreateRepository creates a new repository
func (s *ServiceImpl) CreateRepository(ctx context.Context, registry string, options CreateRepositoryOptions) error {
	s.logger.Info("Creating repository", "registry", registry, "name", options.Name)

	// Validate connection
	if err := s.TestConnection(ctx, registry); err != nil {
		return err
	}

	// Simulate repository creation
	s.logger.Info("Successfully created repository", "registry", registry, "name", options.Name)
	return nil
}

// DeleteRepository deletes a repository
func (s *ServiceImpl) DeleteRepository(ctx context.Context, registry, name string) error {
	s.logger.Info("Deleting repository", "registry", registry, "repository", name)

	// Validate connection
	if err := s.TestConnection(ctx, registry); err != nil {
		return err
	}

	// Simulate repository deletion
	s.logger.Info("Successfully deleted repository", "registry", registry, "repository", name)
	return nil
}

// ListTags lists tags in a repository
func (s *ServiceImpl) ListTags(ctx context.Context, registry, repository string) ([]Tag, error) {
	s.logger.Info("Listing tags", "registry", registry, "repository", repository)

	// Validate connection
	if err := s.TestConnection(ctx, registry); err != nil {
		return nil, err
	}

	// Simulate tag listing
	tags := []Tag{
		{
			Name:         "latest",
			Digest:       "sha256:abcd1234",
			Size:         50 * 1024 * 1024,
			CreatedAt:    time.Now().Add(-1 * time.Hour),
			Architecture: "amd64",
			OS:           "linux",
		},
		{
			Name:         "v1.0.0",
			Digest:       "sha256:efgh5678",
			Size:         48 * 1024 * 1024,
			CreatedAt:    time.Now().Add(-24 * time.Hour),
			Architecture: "amd64",
			OS:           "linux",
		},
	}

	return tags, nil
}

// GetTagInfo gets detailed information about a tag
func (s *ServiceImpl) GetTagInfo(ctx context.Context, registry, repository, tag string) (*TagInfo, error) {
	s.logger.Info("Getting tag info", "registry", registry, "repository", repository, "tag", tag)

	tags, err := s.ListTags(ctx, registry, repository)
	if err != nil {
		return nil, err
	}

	for _, t := range tags {
		if t.Name == tag {
			return &TagInfo{
				Tag: t,
				Manifest: map[string]interface{}{
					"mediaType":     "application/vnd.docker.distribution.manifest.v2+json",
					"schemaVersion": 2,
				},
				Config: map[string]interface{}{
					"architecture": t.Architecture,
					"os":           t.OS,
				},
				History: []LayerHistory{
					{
						Created:   t.CreatedAt,
						CreatedBy: "ADD . /app",
						Size:      t.Size,
					},
				},
				Vulnerabilities: []Vulnerability{},
			}, nil
		}
	}

	return nil, errors.New(errors.CodeNotFound, "registry", fmt.Sprintf("tag not found: %s/%s:%s", registry, repository, tag), nil)
}

// DeleteTag deletes a tag
func (s *ServiceImpl) DeleteTag(ctx context.Context, registry, repository, tag string) error {
	s.logger.Info("Deleting tag", "registry", registry, "repository", repository, "tag", tag)

	// Validate connection
	if err := s.TestConnection(ctx, registry); err != nil {
		return err
	}

	// Simulate tag deletion
	s.logger.Info("Successfully deleted tag", "registry", registry, "repository", repository, "tag", tag)
	return nil
}

// GetRegistryInfo gets information about the registry
func (s *ServiceImpl) GetRegistryInfo(ctx context.Context, registry string) (*Info, error) {
	s.logger.Info("Getting registry info", "registry", registry)

	// Validate connection
	if err := s.TestConnection(ctx, registry); err != nil {
		return nil, err
	}

	// Simulate registry info retrieval
	info := &Info{
		Name:          "Docker Hub",
		URL:           registry,
		Type:          "docker-registry",
		Version:       "2.8.1",
		Features:      []string{"push", "pull", "delete"},
		StorageDriver: "s3",
		AuthSupported: true,
	}

	return info, nil
}

// TestConnection tests connectivity to a registry
func (s *ServiceImpl) TestConnection(ctx context.Context, registry string) error {
	return s.testConnection(ctx, registry, nil)
}

// testConnection tests connectivity with optional auth
func (s *ServiceImpl) testConnection(_ context.Context, registry string, auth *AuthConfig) error {
	s.logger.Debug("Testing registry connection", "registry", registry)

	// If no auth provided, try to get from cache
	if auth == nil {
		s.authMutex.RLock()
		cachedAuth, exists := s.authCache[registry]
		s.authMutex.RUnlock()

		if exists {
			auth = cachedAuth
			_ = auth // Use auth to suppress unused variable warning
		}
	}

	// Simulate connection test (in real implementation, this would make HTTP request)
	// For now, just return success
	return nil
}

// validateAuth validates authentication for a registry
func (s *ServiceImpl) validateAuth(registry string, auth *AuthConfig) error {
	if auth == nil {
		s.authMutex.RLock()
		cachedAuth, exists := s.authCache[registry]
		s.authMutex.RUnlock()

		if !exists {
			return errors.New(errors.CodePermissionDenied, "registry", fmt.Sprintf("no authentication configured for registry: %s", registry), nil)
		}
		auth = cachedAuth
	}

	if auth.Username == "" && auth.Token == "" && auth.IdentityToken == "" {
		return errors.New(errors.CodePermissionDenied, "registry", fmt.Sprintf("invalid authentication configuration for registry: %s", registry), nil)
	}

	return nil
}
