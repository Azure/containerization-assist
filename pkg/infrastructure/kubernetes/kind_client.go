package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/infrastructure/container"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
	"github.com/Azure/containerization-assist/pkg/infrastructure/kubernetes/kind"
)

// ValidateKindInstalled checks if 'kind' is installed
func ValidateKindInstalled(ctx context.Context, kindRunner kind.KindRunner) error {
	if _, err := kindRunner.Version(ctx); err != nil {
		// In MCP server mode, we don't interactively install kind
		// Return an error indicating kind needs to be installed
		return errors.New(errors.CodeOperationFailed, "kind", "kind is not installed. Please install kind (https://kind.sigs.k8s.io/) to use local Kubernetes features", nil)
	}
	return nil
}

// checkDockerRunning verifies if the Docker daemon is running.
func checkDockerRunning(ctx context.Context, docker container.DockerClient) error {
	if output, err := docker.Info(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running. Please start Docker and try again. Error details: %s", string(output))
	}
	return nil
}

// SetupLocalRegistryCluster sets up a local Docker registry for kind, supporting multiple OS types.
func SetupLocalRegistryCluster(ctx context.Context, kindRunner kind.KindRunner, docker container.DockerClient) error {
	if err := checkDockerRunning(ctx, docker); err != nil {
		return err
	}

	if output, err := kindRunner.SetupRegistry(ctx); err != nil {
		return errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to set up local registry: %s, error: %v", output, err), err)
	}

	return nil
}

func GetKindCluster(ctx context.Context, kindRunner kind.KindRunner, docker container.DockerClient) (string, error) {
	if err := checkDockerRunning(ctx, docker); err != nil {
		return "", err
	}
	if err := ValidateKindInstalled(ctx, kindRunner); err != nil {
		return "", err
	}

	output, err := kindRunner.GetClusters(ctx)
	if err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to get kind clusters: %s, error: %v", output, err), err)
	}

	clusters := strings.Split(string(output), "\n")
	exists := false
	for _, cluster := range clusters {
		if strings.TrimSpace(cluster) == "containerization-assist" {
			exists = true
			break
		}
	}

	if exists {
		if output, err = kindRunner.DeleteCluster(ctx, "containerization-assist"); err != nil {
			return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to delete existing kind cluster: %s, error: %v", output, err), err)
		}
	}
	if err := SetupLocalRegistryCluster(ctx, kindRunner, docker); err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("setting up local registry cluster: %v", err), err)
	}

	// Create a kube runner to set context
	kubeRunner := NewKubeCmdRunner(&core.DefaultCommandRunner{})
	if output, err = kubeRunner.SetKubeContext(ctx, "kind-containerization-assist"); err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to set kubectl context: %s, error: %v", output, err), err)
	}

	return "localhost:5001", nil
}
