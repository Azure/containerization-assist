package runner

type DockerRunner interface {
	Version() (string, error)
	Info() (string, error)
	Build(dockerfilePath, imageTag, contextPath string) (string, error)
	Push(imageTag string) (string, error)
}

type DockerCmdRunner struct {
	runner CommandRunner
}

var _ DockerRunner = &DockerCmdRunner{}

func NewDockerCmdRunner(runner CommandRunner) DockerRunner {
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
	return d.runner.RunCommandStderr("docker", "build", "-f", dockerfilePath, "-t", imageTag, contextPath)
}

func (d *DockerCmdRunner) Push(image string) (string, error) {
	return d.runner.RunCommand("docker", "push", image)
}
