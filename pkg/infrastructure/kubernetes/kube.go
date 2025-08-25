package kubernetes

import (
	"context"

	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
)

type KubeRunner interface {
	Apply(ctx context.Context, manifestPath string) (string, error)
	GetPods(ctx context.Context, namespace string, labelSelector string) (string, error)
	GetPodsJSON(ctx context.Context, namespace string, labelSelector string) (string, error)
	DescribePod(ctx context.Context, podName string, namespace string) (string, error)
	GetEvents(ctx context.Context, namespace string) (string, error)
	GetNodes(ctx context.Context) (string, error)
	SetKubeContext(ctx context.Context, name string) (string, error)
	DeleteDeployment(ctx context.Context, manifestPath string) (string, error)
	RolloutStatus(ctx context.Context, resourceType string, resourceName string, namespace string, timeout string) (string, error)
}

type KubeCmdRunner struct {
	runner core.CommandRunner
}

var _ KubeRunner = &KubeCmdRunner{}

func NewKubeCmdRunner(runner core.CommandRunner) KubeRunner {
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

func (k *KubeCmdRunner) DescribePod(ctx context.Context, podName string, namespace string) (string, error) {
	return k.runner.RunCommand("kubectl", "describe", "pod", podName, "-n", namespace)
}

func (k *KubeCmdRunner) GetEvents(ctx context.Context, namespace string) (string, error) {
	return k.runner.RunCommand("kubectl", "get", "events", "-n", namespace, "--sort-by='.lastTimestamp'")
}

func (k *KubeCmdRunner) GetNodes(ctx context.Context) (string, error) {
	return k.runner.RunCommand("kubectl", "get", "nodes")
}

func (k *KubeCmdRunner) SetKubeContext(ctx context.Context, name string) (string, error) {
	return k.runner.RunCommand("kubectl", "config", "use-context", name)
}

func (k *KubeCmdRunner) DeleteDeployment(ctx context.Context, manifestPath string) (string, error) {
	return k.runner.RunCommand("kubectl", "delete", "-f", manifestPath, "--ignore-not-found=true")
}

func (k *KubeCmdRunner) RolloutStatus(ctx context.Context, resourceType string, resourceName string, namespace string, timeout string) (string, error) {
	args := []string{"rollout", "status", resourceType + "/" + resourceName, "-n", namespace}
	if timeout != "" {
		args = append(args, "--timeout="+timeout)
	}
	return k.runner.RunCommand("kubectl", args...)
}

// CheckKubectlInstalled function removed as dead code
