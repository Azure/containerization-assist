// Package docker provides core Docker operations extracted from the pipeline
// without AI dependencies. These are mechanical operations that can be used
// by atomic MCP tools.
package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/clients"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// Builder provides mechanical Docker build operations without AI
type Builder struct {
	clients *clients.Clients
	logger  *slog.Logger
}

// NewBuilder creates a new Docker builder
func NewBuilder(clients *clients.Clients, logger *slog.Logger) *Builder {
	return &Builder{
		clients: clients,
		logger:  logger.With("component", "docker_builder"),
	}
}

// BuildResult contains the result of a Docker build operation
type BuildResult struct {
	Success  bool                   `json:"success"`
	ImageID  string                 `json:"image_id,omitempty"`
	ImageRef string                 `json:"image_ref"`
	Logs     []string               `json:"logs"`
	Duration time.Duration          `json:"duration"`
	Error    *BuildError            `json:"error,omitempty"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// BuildError provides detailed error information for external AI to analyze
type BuildError struct {
	Type       string                 `json:"type"` // "dockerfile_error", "build_error", "network_error"
	Message    string                 `json:"message"`
	ExitCode   int                    `json:"exit_code"`
	Command    string                 `json:"command"`
	Dockerfile string                 `json:"dockerfile"` // For Claude to analyze
	BuildLogs  string                 `json:"build_logs"`
	Context    map[string]interface{} `json:"context"`
}

// BuildOptions contains options for Docker builds
type BuildOptions struct {
	ImageName    string
	Registry     string
	NoCache      bool
	Platform     string
	BuildArgs    map[string]string
	Tags         []string
	BuildTimeout time.Duration
}

// BuildImage performs a Docker build using the provided Dockerfile content
// This is a mechanical operation that returns detailed error context for external AI analysis
func (b *Builder) BuildImage(ctx context.Context, dockerfileContent string, contextPath string, options BuildOptions) (*BuildResult, error) {
	startTime := time.Now()

	result := &BuildResult{
		ImageRef: b.normalizeImageRef(options),
		Logs:     make([]string, 0),
		Context:  make(map[string]interface{}),
	}

	b.logger.Info("Starting Docker build",
		"image_ref", result.ImageRef,
		"context_path", contextPath)

	// Validate inputs
	if err := b.validateInputs(dockerfileContent, contextPath, options); err != nil {
		result.Error = &BuildError{
			Type:       "validation_error",
			Message:    err.Error(),
			Dockerfile: dockerfileContent,
			Context: map[string]interface{}{
				"context_path": contextPath,
				"options":      options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Check Docker installation
	if err := b.checkDockerInstalled(); err != nil {
		result.Error = &BuildError{
			Type:    "docker_not_available",
			Message: err.Error(),
			Context: map[string]interface{}{
				"suggestion": "Install Docker or ensure it's running",
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Create temporary Dockerfile
	tmpDir, dockerfilePath, err := b.createTempDockerfile(dockerfileContent)
	if err != nil {
		result.Error = &BuildError{
			Type:       "filesystem_error",
			Message:    fmt.Sprintf("Failed to create temporary Dockerfile: %v", err),
			Dockerfile: dockerfileContent,
			Context: map[string]interface{}{
				"context_path": contextPath,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}
	defer os.RemoveAll(tmpDir)

	// Perform the actual Docker build
	buildOutput, err := b.clients.Docker.Build(ctx, dockerfilePath, result.ImageRef, contextPath)
	if err != nil {
		b.logger.Error("Docker build failed", "error", err, "build_output", buildOutput)

		result.Error = &BuildError{
			Type:       "build_error",
			Message:    fmt.Sprintf("Docker build failed: %v", err),
			Command:    b.buildDockerCommand(dockerfilePath, result.ImageRef, contextPath),
			Dockerfile: dockerfileContent,
			BuildLogs:  buildOutput,
			Context: map[string]interface{}{
				"context_path":    contextPath,
				"image_ref":       result.ImageRef,
				"build_options":   options,
				"dockerfile_path": dockerfilePath,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Build succeeded
	result.Success = true
	result.ImageID = b.extractImageID(buildOutput)
	result.Logs = b.parseBuildLogs(buildOutput)
	result.Duration = time.Since(startTime)

	// Add success context
	result.Context = map[string]interface{}{
		"context_path":    contextPath,
		"build_time":      result.Duration.Seconds(),
		"dockerfile_size": len(dockerfileContent),
	}

	b.logger.Info("Docker build completed successfully",
		"image_id", result.ImageID,
		"image_ref", result.ImageRef,
		"duration", result.Duration)

	return result, nil
}

// ValidateDockerfile performs basic validation of Dockerfile content
func (b *Builder) ValidateDockerfile(dockerfileContent string) error {
	if strings.TrimSpace(dockerfileContent) == "" {
		return mcperrors.NewError().Messagef("dockerfile is empty").WithLocation(

		// Check for FROM instruction
		).Build()
	}

	if !strings.Contains(strings.ToUpper(dockerfileContent), "FROM") {
		return mcperrors.NewError().Messagef("dockerfile missing FROM instruction").WithLocation(

		// Basic syntax validation could be added here
		).Build()
	}

	return nil
}

// PushImage pushes a Docker image to a registry
func (b *Builder) PushImage(ctx context.Context, imageRef string) (*PushResult, error) {
	b.logger.Info("Starting Docker push", "image_ref", imageRef)

	output, err := b.clients.Docker.Push(ctx, imageRef)
	if err != nil {
		return &PushResult{
			Success: false,
			Error: &PushError{
				Type:     "push_error",
				Message:  fmt.Sprintf("Docker push failed: %v", err),
				ImageRef: imageRef,
				Output:   output,
				Context: map[string]interface{}{
					"image_ref": imageRef,
				},
			},
		}, nil
	}

	return &PushResult{
		Success:  true,
		ImageRef: imageRef,
		Output:   output,
	}, nil
}

// PushResult contains the result of a Docker push operation
type PushResult struct {
	Success  bool       `json:"success"`
	ImageRef string     `json:"image_ref"`
	Output   string     `json:"output"`
	Error    *PushError `json:"error,omitempty"`
}

// PushError provides detailed push error information
type PushError struct {
	Type     string                 `json:"type"`
	Message  string                 `json:"message"`
	ImageRef string                 `json:"image_ref"`
	Output   string                 `json:"output"`
	Context  map[string]interface{} `json:"context"`
}

// Helper methods

func (b *Builder) validateInputs(dockerfileContent string, contextPath string, options BuildOptions) error {
	if err := b.ValidateDockerfile(dockerfileContent); err != nil {
		return err
	}

	if contextPath == "" {
		return fmt.Errorf("context path is required")
	}

	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		return fmt.Errorf("context path does not exist: %s", contextPath)
	}

	if options.ImageName == "" {
		return fmt.Errorf("image name is required")
	}

	return nil
}

func (b *Builder) checkDockerInstalled() error {
	// Use the same check as the original pipeline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := b.clients.Docker.Info(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running or not accessible: %v", err)
	}

	return nil
}

func (b *Builder) createTempDockerfile(content string) (string, string, error) {
	tmpDir, err := os.MkdirTemp("", "docker-build-*")
	if err != nil {
		return "", "", err
	}

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", err
	}

	return tmpDir, dockerfilePath, nil
}

func (b *Builder) normalizeImageRef(options BuildOptions) string {
	imageName := options.ImageName
	if imageName == "" {
		imageName = "app"
	}

	if options.Registry == "" {
		return fmt.Sprintf("%s:latest", imageName)
	}

	return fmt.Sprintf("%s/%s:latest", options.Registry, imageName)
}

func (b *Builder) buildDockerCommand(dockerfilePath, imageRef, contextPath string) string {
	return fmt.Sprintf("docker build -q -f %s -t %s %s", dockerfilePath, imageRef, contextPath)
}

func (b *Builder) extractImageID(buildOutput string) string {
	// Extract image ID from build output if available
	lines := strings.Split(buildOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "sha256:") || (len(line) == 64 && strings.ToLower(line) == line) {
			return line
		}
	}
	return ""
}

func (b *Builder) parseBuildLogs(buildOutput string) []string {
	logs := make([]string, 0)
	lines := strings.Split(buildOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			logs = append(logs, line)
		}
	}
	return logs
}
