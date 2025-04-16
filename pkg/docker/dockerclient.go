package docker

import (
	"fmt"
	"os/exec"

	"github.com/Azure/container-copilot/pkg/runner"
)

type DockerClient interface {
	Version() (string, error)
	Info() (string, error)
	Build(dockerfilePath, imageTag, contextPath string) (string, error)
	Push(imageTag string) (string, error)
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

func (d *DockerCmdRunner) Info() (string, error) {
	return d.runner.RunCommand("docker", "info")
}

func (d *DockerCmdRunner) Version() (string, error) {
	return d.runner.RunCommand("docker", "version")
}

func (d *DockerCmdRunner) Build(dockerfilePath, imageTag, contextPath string) (string, error) {
	return d.runner.RunCommandStderr("docker", "build", "-q", "-f", dockerfilePath, "-t", imageTag, contextPath)
}

func (d *DockerCmdRunner) Push(image string) (string, error) {
	return d.runner.RunCommand("docker", "push", image)
}
func CheckDockerInstalled() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker executable not found in PATH. Please install Docker or ensure it's available in your PATH")
	}
	return nil
}
