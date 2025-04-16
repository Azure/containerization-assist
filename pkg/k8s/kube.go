package k8s

import (
	"fmt"
	"os/exec"

	"github.com/Azure/container-copilot/pkg/runner"
)

type KubeRunner interface {
	Apply(manifestPath string) (string, error)
	GetPods(namespace string, labelSelector string) (string, error)
	SetKubeContext(name string) (string, error)
}

type KubeCmdRunner struct {
	runner runner.CommandRunner
}

var _ KubeRunner = &KubeCmdRunner{}

func NewKubeCmdRunner(runner runner.CommandRunner) KubeRunner {
	return &KubeCmdRunner{
		runner: runner,
	}
}

func (k *KubeCmdRunner) Apply(manifestPath string) (string, error) {
	return k.runner.RunCommand("kubectl", "apply", "-f", manifestPath)
}

func (k *KubeCmdRunner) GetPods(namespace string, labelSelector string) (string, error) {
	if labelSelector != "" {
		return k.runner.RunCommand("kubectl", "get", "pods", "-n", namespace, "-l", labelSelector)
	}
	return k.runner.RunCommand("kubectl", "get", "pods", "-n", namespace)
}

func (k *KubeCmdRunner) SetKubeContext(name string) (string, error) {
	return k.runner.RunCommand("kubectl", "config", "use-context", name)
}

func CheckKubectlInstalled() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl executable not found in PATH. Please install kubectl or ensure it's available in your PATH")
	}
	return nil
}
