package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/Azure/container-kit/pkg/runner"
)

type DockerClient interface {
	Version(ctx context.Context) (string, error)
	Info(ctx context.Context) (string, error)
	Build(ctx context.Context, dockerfilePath, imageTag, contextPath string) (string, error)
	Push(ctx context.Context, imageTag string) (string, error)
	Pull(ctx context.Context, imageRef string) (string, error)
	Tag(ctx context.Context, sourceRef, targetRef string) (string, error)
	Login(ctx context.Context, registry, username, password string) (string, error)
}

type DockerCmdRunner struct {
	runner runner.CommandRunner
}

var _ DockerClient = &DockerCmdRunner{}

func NewDockerCmdRunner(runner runner.CommandRunner) DockerClient {
	return &DockerCmdRunner{
		runner: runner,
	}
}

func (d *DockerCmdRunner) Info(ctx context.Context) (string, error) {
	return d.runner.RunCommand("docker", "info")
}

func (d *DockerCmdRunner) Version(ctx context.Context) (string, error) {
	return d.runner.RunCommand("docker", "version")
}

func (d *DockerCmdRunner) Build(ctx context.Context, dockerfilePath, imageTag, contextPath string) (string, error) {
	return d.runner.RunCommandStderr("docker", "build", "-q", "-f", dockerfilePath, "-t", imageTag, contextPath)
}

func (d *DockerCmdRunner) Push(ctx context.Context, image string) (string, error) {
	return d.runner.RunCommand("docker", "push", image)
}

func (d *DockerCmdRunner) Pull(ctx context.Context, imageRef string) (string, error) {
	return d.runner.RunCommand("docker", "pull", imageRef)
}

func (d *DockerCmdRunner) Tag(ctx context.Context, sourceRef, targetRef string) (string, error) {
	return d.runner.RunCommand("docker", "tag", sourceRef, targetRef)
}

func (d *DockerCmdRunner) Login(ctx context.Context, registry, username, password string) (string, error) {
	// For now, use environment variables to pass credentials
	// This avoids exposing passwords in command line but requires proper cleanup
	oldUser := os.Getenv("DOCKER_USER")
	oldPass := os.Getenv("DOCKER_PASSWORD")
	defer func() {
		os.Setenv("DOCKER_USER", oldUser)
		os.Setenv("DOCKER_PASSWORD", oldPass)
	}()

	os.Setenv("DOCKER_USER", username)
	os.Setenv("DOCKER_PASSWORD", password)

	if registry == "" {
		return d.runner.RunCommand("docker", "login", "--username", username, "--password", password)
	}
	return d.runner.RunCommand("docker", "login", registry, "--username", username, "--password", password)
}

func CheckDockerInstalled() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker executable not found in PATH. Please install Docker or ensure it's available in your PATH")
	}
	return nil
}
