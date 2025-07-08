package k8s

import (
	"context"
	"os/exec"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/runner"
)

type KubeRunner interface {
	Apply(ctx context.Context, manifestPath string) (string, error)
	GetPods(ctx context.Context, namespace string, labelSelector string) (string, error)
	GetPodsJSON(ctx context.Context, namespace string, labelSelector string) (string, error)
	SetKubeContext(ctx context.Context, name string) (string, error)
	DeleteDeployment(ctx context.Context, manifestPath string) (string, error)
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

func (k *KubeCmdRunner) Apply(ctx context.Context, manifestPath string) (string, error) {
	return k.runner.RunCommand("kubectl", "apply", "-f", manifestPath)
}

func (k *KubeCmdRunner) GetPods(ctx context.Context, namespace string, labelSelector string) (string, error) {
	if labelSelector != "" {
		return k.runner.RunCommand("kubectl", "get", "pods", "-n", namespace, "-l", labelSelector)
	}
	return k.runner.RunCommand("kubectl", "get", "pods", "-n", namespace)
}

func (k *KubeCmdRunner) GetPodsJSON(ctx context.Context, namespace string, labelSelector string) (string, error) {
	if labelSelector != "" {
		return k.runner.RunCommand("kubectl", "get", "pods", "-n", namespace, "-l", labelSelector, "-o", "json")
	}
	return k.runner.RunCommand("kubectl", "get", "pods", "-n", namespace, "-o", "json")
}

func (k *KubeCmdRunner) SetKubeContext(ctx context.Context, name string) (string, error) {
	return k.runner.RunCommand("kubectl", "config", "use-context", name)
}

func (k *KubeCmdRunner) DeleteDeployment(ctx context.Context, manifestPath string) (string, error) {
	return k.runner.RunCommand("kubectl", "delete", "-f", manifestPath, "--ignore-not-found=true")
}

func CheckKubectlInstalled() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return mcperrors.NewError().Messagef("kubectl executable not found in PATH. Please install kubectl or ensure it's available in your PATH").WithLocation().Build()
	}
	return nil
}
