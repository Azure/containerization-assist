package runner

type KubeRunner interface {
	Apply(manifestPath string) (string, error)
	GetPods(namespace string) (string, error)
	SetKubeContext(name string) (string, error)
}

type KubeCmdRunner struct {
	runner CommandRunner
}

var _ KubeRunner = &KubeCmdRunner{}

func NewKubeCmdRunner(runner CommandRunner) KubeRunner {
	return &KubeCmdRunner{
		runner: runner,
	}
}

func (k *KubeCmdRunner) Apply(manifestPath string) (string, error) {
	return k.runner.RunCommand("kubectl", "apply", "-f", manifestPath)
}

func (k *KubeCmdRunner) GetPods(namespace string) (string, error) {
	return k.runner.RunCommand("kubectl", "get", "pods", "-n", namespace, "-o", "json")
}

func (k *KubeCmdRunner) SetKubeContext(name string) (string, error) {
	return k.runner.RunCommand("kubectl", "config", "use-context", name)
}
