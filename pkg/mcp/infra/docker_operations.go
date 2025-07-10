//go:build docker

package infra

import (
	"context"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
)

// DockerOperations handles Docker-specific operations
// This is only compiled when -tags docker is used
type DockerOperations struct {
	client docker.Client
	logger *slog.Logger
}

// NewDockerOperations creates a new Docker operations handler
func NewDockerOperations(client docker.Client, logger *slog.Logger) *DockerOperations {
	return &DockerOperations{
		client: client,
		logger: logger,
	}
}

// BuildImageParams represents parameters for building Docker images
type BuildImageParams struct {
	ContextPath    string
	DockerfilePath string
	Tags           []string
	BuildArgs      map[string]string
	Labels         map[string]string
	Target         string
	NoCache        bool
	PullParent     bool
	Platform       string
	Timeout        time.Duration
}

// BuildImageResult represents the result of building a Docker image
type BuildImageResult struct {
	ImageID     string
	ImageDigest string
	Tags        []string
	Size        int64
	BuildTime   time.Duration
	Success     bool
	Error       string
	BuildLogs   []string
}

// BuildImage builds a Docker image
func (d *DockerOperations) BuildImage(ctx context.Context, params BuildImageParams) (*BuildImageResult, error) {
	startTime := time.Now()

	d.logger.Info("Starting Docker image build",
		"context_path", params.ContextPath,
		"dockerfile_path", params.DockerfilePath,
		"tags", params.Tags)

	// Create Docker build request
	buildRequest := docker.BuildRequest{
		ContextPath:    params.ContextPath,
		DockerfilePath: params.DockerfilePath,
		Tags:           params.Tags,
		BuildArgs:      params.BuildArgs,
		Labels:         params.Labels,
		Target:         params.Target,
		NoCache:        params.NoCache,
		PullParent:     params.PullParent,
		Platform:       params.Platform,
	}

	// Set timeout context
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Execute build
	buildResult, err := d.client.BuildImage(ctx, buildRequest)
	if err != nil {
		return &BuildImageResult{
			Success:   false,
			Error:     err.Error(),
			BuildTime: time.Since(startTime),
		}, nil
	}

	// Convert to our result format
	result := &BuildImageResult{
		ImageID:     buildResult.ImageID,
		ImageDigest: buildResult.ImageDigest,
		Tags:        buildResult.Tags,
		Size:        buildResult.Size,
		BuildTime:   time.Since(startTime),
		Success:     true,
		BuildLogs:   buildResult.BuildLogs,
	}

	d.logger.Info("Docker image build completed",
		"image_id", result.ImageID,
		"tags", result.Tags,
		"size", result.Size,
		"build_time", result.BuildTime)

	return result, nil
}

// PushImageParams represents parameters for pushing Docker images
type PushImageParams struct {
	ImageRef      string
	Registry      string
	Username      string
	Password      string
	SkipTLSVerify bool
	Timeout       time.Duration
}

// PushImageResult represents the result of pushing a Docker image
type PushImageResult struct {
	ImageRef string
	Digest   string
	Size     int64
	PushTime time.Duration
	Success  bool
	Error    string
	PushLogs []string
}

// PushImage pushes a Docker image to a registry
func (d *DockerOperations) PushImage(ctx context.Context, params PushImageParams) (*PushImageResult, error) {
	startTime := time.Now()

	d.logger.Info("Starting Docker image push",
		"image_ref", params.ImageRef,
		"registry", params.Registry)

	// Create Docker push request
	pushRequest := docker.PushRequest{
		ImageRef:      params.ImageRef,
		Registry:      params.Registry,
		Username:      params.Username,
		Password:      params.Password,
		SkipTLSVerify: params.SkipTLSVerify,
	}

	// Set timeout context
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Execute push
	pushResult, err := d.client.PushImage(ctx, pushRequest)
	if err != nil {
		return &PushImageResult{
			ImageRef: params.ImageRef,
			Success:  false,
			Error:    err.Error(),
			PushTime: time.Since(startTime),
		}, nil
	}

	// Convert to our result format
	result := &PushImageResult{
		ImageRef: params.ImageRef,
		Digest:   pushResult.Digest,
		Size:     pushResult.Size,
		PushTime: time.Since(startTime),
		Success:  true,
		PushLogs: pushResult.PushLogs,
	}

	d.logger.Info("Docker image push completed",
		"image_ref", result.ImageRef,
		"digest", result.Digest,
		"size", result.Size,
		"push_time", result.PushTime)

	return result, nil
}

// PullImageParams represents parameters for pulling Docker images
type PullImageParams struct {
	ImageRef      string
	Registry      string
	Username      string
	Password      string
	SkipTLSVerify bool
	Timeout       time.Duration
}

// PullImageResult represents the result of pulling a Docker image
type PullImageResult struct {
	ImageRef string
	ImageID  string
	Digest   string
	Size     int64
	PullTime time.Duration
	Success  bool
	Error    string
	PullLogs []string
}

// PullImage pulls a Docker image from a registry
func (d *DockerOperations) PullImage(ctx context.Context, params PullImageParams) (*PullImageResult, error) {
	startTime := time.Now()

	d.logger.Info("Starting Docker image pull",
		"image_ref", params.ImageRef,
		"registry", params.Registry)

	// Create Docker pull request
	pullRequest := docker.PullRequest{
		ImageRef:      params.ImageRef,
		Registry:      params.Registry,
		Username:      params.Username,
		Password:      params.Password,
		SkipTLSVerify: params.SkipTLSVerify,
	}

	// Set timeout context
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Execute pull
	pullResult, err := d.client.PullImage(ctx, pullRequest)
	if err != nil {
		return &PullImageResult{
			ImageRef: params.ImageRef,
			Success:  false,
			Error:    err.Error(),
			PullTime: time.Since(startTime),
		}, nil
	}

	// Convert to our result format
	result := &PullImageResult{
		ImageRef: params.ImageRef,
		ImageID:  pullResult.ImageID,
		Digest:   pullResult.Digest,
		Size:     pullResult.Size,
		PullTime: time.Since(startTime),
		Success:  true,
		PullLogs: pullResult.PullLogs,
	}

	d.logger.Info("Docker image pull completed",
		"image_ref", result.ImageRef,
		"image_id", result.ImageID,
		"digest", result.Digest,
		"size", result.Size,
		"pull_time", result.PullTime)

	return result, nil
}

// ScanImageParams represents parameters for scanning Docker images
type ScanImageParams struct {
	ImageRef          string
	ScannerType       string
	SeverityThreshold string
	IncludeSecrets    bool
	Timeout           time.Duration
}

// ScanImageResult represents the result of scanning a Docker image
type ScanImageResult struct {
	ImageRef        string
	ScannerType     string
	Vulnerabilities []scan.VulnerabilityInfo
	Secrets         []scan.SecretInfo
	ScanTime        time.Duration
	Success         bool
	Error           string
	ScanLogs        []string
}

// ScanImage scans a Docker image for vulnerabilities and secrets
func (d *DockerOperations) ScanImage(ctx context.Context, params ScanImageParams) (*ScanImageResult, error) {
	startTime := time.Now()

	d.logger.Info("Starting Docker image scan",
		"image_ref", params.ImageRef,
		"scanner_type", params.ScannerType,
		"severity_threshold", params.SeverityThreshold)

	// Create Docker scan request
	scanRequest := docker.ScanRequest{
		ImageRef:          params.ImageRef,
		ScannerType:       params.ScannerType,
		SeverityThreshold: params.SeverityThreshold,
		IncludeSecrets:    params.IncludeSecrets,
	}

	// Set timeout context
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Execute scan
	scanResult, err := d.client.ScanImage(ctx, scanRequest)
	if err != nil {
		return &ScanImageResult{
			ImageRef:    params.ImageRef,
			ScannerType: params.ScannerType,
			Success:     false,
			Error:       err.Error(),
			ScanTime:    time.Since(startTime),
		}, nil
	}

	// Convert vulnerabilities
	vulnerabilities := make([]scan.VulnerabilityInfo, len(scanResult.Vulnerabilities))
	for i, vuln := range scanResult.Vulnerabilities {
		vulnerabilities[i] = scan.VulnerabilityInfo{
			ID:          vuln.ID,
			Severity:    vuln.Severity,
			Title:       vuln.Title,
			Description: vuln.Description,
			Package:     vuln.Package,
			Version:     vuln.Version,
			FixedIn:     vuln.FixedIn,
			CVSS:        vuln.CVSS,
			References:  vuln.References,
			Metadata:    vuln.Metadata,
		}
	}

	// Convert secrets
	secrets := make([]scan.SecretInfo, len(scanResult.Secrets))
	for i, secret := range scanResult.Secrets {
		secrets[i] = scan.SecretInfo{
			Type:       secret.Type,
			File:       secret.File,
			Line:       secret.Line,
			Pattern:    secret.Pattern,
			Value:      secret.Value,
			Severity:   secret.Severity,
			Confidence: secret.Confidence,
			Metadata:   secret.Metadata,
		}
	}

	// Convert to our result format
	result := &ScanImageResult{
		ImageRef:        params.ImageRef,
		ScannerType:     params.ScannerType,
		Vulnerabilities: vulnerabilities,
		Secrets:         secrets,
		ScanTime:        time.Since(startTime),
		Success:         true,
		ScanLogs:        scanResult.ScanLogs,
	}

	d.logger.Info("Docker image scan completed",
		"image_ref", result.ImageRef,
		"vulnerabilities", len(result.Vulnerabilities),
		"secrets", len(result.Secrets),
		"scan_time", result.ScanTime)

	return result, nil
}

// TagImageParams represents parameters for tagging Docker images
type TagImageParams struct {
	SourceImageRef string
	TargetImageRef string
}

// TagImageResult represents the result of tagging a Docker image
type TagImageResult struct {
	SourceImageRef string
	TargetImageRef string
	Success        bool
	Error          string
}

// TagImage tags a Docker image
func (d *DockerOperations) TagImage(ctx context.Context, params TagImageParams) (*TagImageResult, error) {
	d.logger.Info("Starting Docker image tag",
		"source_image_ref", params.SourceImageRef,
		"target_image_ref", params.TargetImageRef)

	// Execute tag
	err := d.client.TagImage(ctx, params.SourceImageRef, params.TargetImageRef)
	if err != nil {
		return &TagImageResult{
			SourceImageRef: params.SourceImageRef,
			TargetImageRef: params.TargetImageRef,
			Success:        false,
			Error:          err.Error(),
		}, nil
	}

	result := &TagImageResult{
		SourceImageRef: params.SourceImageRef,
		TargetImageRef: params.TargetImageRef,
		Success:        true,
	}

	d.logger.Info("Docker image tag completed",
		"source_image_ref", result.SourceImageRef,
		"target_image_ref", result.TargetImageRef)

	return result, nil
}

// GetImageInfoParams represents parameters for getting Docker image information
type GetImageInfoParams struct {
	ImageRef string
}

// GetImageInfoResult represents Docker image information
type GetImageInfoResult struct {
	ImageRef     string
	ImageID      string
	Digest       string
	Size         int64
	Created      time.Time
	Architecture string
	OS           string
	Tags         []string
	Labels       map[string]string
	Success      bool
	Error        string
}

// GetImageInfo gets information about a Docker image
func (d *DockerOperations) GetImageInfo(ctx context.Context, params GetImageInfoParams) (*GetImageInfoResult, error) {
	d.logger.Info("Getting Docker image info", "image_ref", params.ImageRef)

	// Execute get info
	imageInfo, err := d.client.GetImageInfo(ctx, params.ImageRef)
	if err != nil {
		return &GetImageInfoResult{
			ImageRef: params.ImageRef,
			Success:  false,
			Error:    err.Error(),
		}, nil
	}

	result := &GetImageInfoResult{
		ImageRef:     params.ImageRef,
		ImageID:      imageInfo.ImageID,
		Digest:       imageInfo.Digest,
		Size:         imageInfo.Size,
		Created:      imageInfo.Created,
		Architecture: imageInfo.Architecture,
		OS:           imageInfo.OS,
		Tags:         imageInfo.Tags,
		Labels:       imageInfo.Labels,
		Success:      true,
	}

	d.logger.Info("Docker image info retrieved",
		"image_ref", result.ImageRef,
		"image_id", result.ImageID,
		"size", result.Size,
		"architecture", result.Architecture,
		"os", result.OS)

	return result, nil
}

// ConvertBuildRequest converts domain build request to Docker build request
func (d *DockerOperations) ConvertBuildRequest(domainRequest *build.BuildRequest) BuildImageParams {
	return BuildImageParams{
		ContextPath:    domainRequest.ContextPath,
		DockerfilePath: domainRequest.DockerfilePath,
		Tags:           domainRequest.Tags,
		BuildArgs:      domainRequest.BuildArgs,
		Labels:         domainRequest.Labels,
		Target:         domainRequest.Target,
		NoCache:        domainRequest.NoCache,
		PullParent:     domainRequest.PullParent,
		Platform:       domainRequest.Platform,
		Timeout:        domainRequest.Timeout,
	}
}

// ConvertBuildResult converts Docker build result to domain build result
func (d *DockerOperations) ConvertBuildResult(dockerResult *BuildImageResult) *build.BuildResult {
	return &build.BuildResult{
		ImageID:     dockerResult.ImageID,
		ImageDigest: dockerResult.ImageDigest,
		Tags:        dockerResult.Tags,
		Size:        dockerResult.Size,
		BuildTime:   dockerResult.BuildTime,
		Success:     dockerResult.Success,
		Error:       dockerResult.Error,
		BuildLogs:   dockerResult.BuildLogs,
	}
}
