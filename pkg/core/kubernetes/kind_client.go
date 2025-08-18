package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/containerization-assist/pkg/common/errors"
	"github.com/Azure/containerization-assist/pkg/common/logger"
	"github.com/Azure/containerization-assist/pkg/common/runner"
	"github.com/Azure/containerization-assist/pkg/core/docker"
	"github.com/Azure/containerization-assist/pkg/core/kind"
)

// ValidateKindInstalled checks if 'kind' is installed, installs it if missing based on OS.
func ValidateKindInstalled(ctx context.Context, kindRunner kind.KindRunner) error {
	if _, err := kindRunner.Version(ctx); err != nil {
		logger.Info("kind is not installed.")

		fmt.Print("Would you like to install kind? (y/n): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.ToLower(scanner.Text())
		if response != "y" {
			return errors.New(errors.CodeOperationFailed, "kind", "kind installation aborted", nil)
		}

		logger.Info("Attempting to install kind now for you...")
		if output, err := kindRunner.Install(ctx); err != nil {
			return errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to install kind: %s, error: %v", output, err), err)
		}
	}
	return nil
}

// checkDockerRunning verifies if the Docker daemon is running.
func checkDockerRunning(ctx context.Context, docker docker.DockerClient) error {
	if output, err := docker.Info(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running. Please start Docker and try again. Error details: %s", string(output))
	}
	return nil
}

// SetupLocalRegistryCluster sets up a local Docker registry for kind, supporting multiple OS types.
func SetupLocalRegistryCluster(ctx context.Context, kindRunner kind.KindRunner, docker docker.DockerClient) error {
	if err := checkDockerRunning(ctx, docker); err != nil {
		return err
	}

	if output, err := kindRunner.SetupRegistry(ctx); err != nil {
		return errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to set up local registry: %s, error: %v", output, err), err)
	}

	return nil
}

func GetKindCluster(ctx context.Context, kindRunner kind.KindRunner, docker docker.DockerClient) (string, error) {
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
			logger.Infof("found existing 'containerization-assist' cluster")
			break
		}
	}

	if exists {
		logger.Warn("Deleting existing kind cluster 'containerization-assist'")
		if output, err = kindRunner.DeleteCluster(ctx, "containerization-assist"); err != nil {
			return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to delete existing kind cluster: %s, error: %v", output, err), err)
		}
	}
	logger.Info("Creating kind cluster 'containerization-assist'")
	if err := SetupLocalRegistryCluster(ctx, kindRunner, docker); err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("setting up local registry cluster: %v", err), err)
	}

	logger.Info("Setting kubectl context to 'kind-containerization-assist'")
	// Create a kube runner to set context
	kubeRunner := NewKubeCmdRunner(&runner.DefaultCommandRunner{})
	if output, err = kubeRunner.SetKubeContext(ctx, "kind-containerization-assist"); err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to set kubectl context: %s, error: %v", output, err), err)
	}

	return "localhost:5001", nil
}
