package clients

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// buildDockerfileContent builds a Docker image from a string containing Dockerfile contents
func (c *Clients) BuildDockerfileContent(ctx context.Context, dockerfileContent string, targetDir string, registry string, imageName string) (string, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "docker-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up

	// Create temporary Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %v", err)
	}

	registryPrefix := ""
	if registry != "" {
		registryPrefix = registry + "/"
	}

	// Build the image using the temporary Dockerfile
	fmt.Printf("building docker image with tag '%s%s:latest'\n", registryPrefix, imageName)
	buildErrors, err := c.Docker.Build(ctx, dockerfilePath, registryPrefix+imageName+":latest", targetDir)

	if err != nil {
		return buildErrors, fmt.Errorf("docker build failed: %v", err)
	}

	fmt.Printf("built docker image")
	return buildErrors, nil
}

// checkDockerRunning verifies if the Docker daemon is running.
func (c *Clients) checkDockerRunning(ctx context.Context) error {
	if output, err := c.Docker.Info(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running. Please start Docker and try again. Error details: %s", string(output))
	}
	return nil
}

func (c *Clients) PushDockerImage(ctx context.Context, image string) error {
	output, err := c.Docker.Push(ctx, image)
	fmt.Println("Output: ", output)

	if err != nil {
		fmt.Println("Registry push failed with error:", err)
		return fmt.Errorf("error pushing to registry: %v", err)
	}

	return nil
}
