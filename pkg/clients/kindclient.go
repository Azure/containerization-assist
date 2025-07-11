package clients

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/container-kit/pkg/logger"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// validateKindInstalled checks if 'kind' is installed, installs it if missing based on OS.
func (c *Clients) ValidateKindInstalled(ctx context.Context) error {
	if _, err := c.Kind.Version(ctx); err != nil {
		logger.Info("kind is not installed.")

		fmt.Print("Would you like to install kind? (y/n): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.ToLower(scanner.Text())
		if response != "y" {
			return errors.New(errors.CodeOperationFailed, "kind", "kind installation aborted", nil)
		}

		logger.Info("Attempting to install kind now for you...")
		if output, err := c.Kind.Install(ctx); err != nil {
			return errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to install kind: %s, error: %v", output, err), err)
		}
	}
	return nil
}

// setupLocalRegistry sets up a local Docker registry for kind, supporting multiple OS types.
func (c *Clients) SetupLocalRegistryCluster(ctx context.Context) error {
	if err := c.checkDockerRunning(ctx); err != nil {
		return err
	}

	if output, err := c.Kind.SetupRegistry(ctx); err != nil {
		return errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to set up local registry: %s, error: %v", output, err), err)
	}

	return nil
}

func (c *Clients) GetKindCluster(ctx context.Context) (string, error) {
	if err := c.checkDockerRunning(ctx); err != nil {
		return "", err
	}
	if err := c.ValidateKindInstalled(ctx); err != nil {
		return "", err
	}

	output, err := c.Kind.GetClusters(ctx)
	if err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to get kind clusters: %s, error: %v", output, err), err)
	}

	clusters := strings.Split(string(output), "\n")
	exists := false
	for _, cluster := range clusters {
		if strings.TrimSpace(cluster) == "container-kit" {
			exists = true
			logger.Infof("found existing 'container-kit' cluster")
			break
		}
	}

	if exists {
		logger.Warn("Deleting existing kind cluster 'container-kit'")
		if output, err = c.Kind.DeleteCluster(ctx, "container-kit"); err != nil {
			return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to delete existing kind cluster: %s, error: %v", output, err), err)
		}
	}
	logger.Info("Creating kind cluster 'container-kit'")
	if err := c.SetupLocalRegistryCluster(ctx); err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("setting up local registry cluster: %v", err), err)
	}

	logger.Info("Setting kubectl context to 'kind-container-kit'")
	if output, err = c.Kube.SetKubeContext(ctx, "kind-container-kit"); err != nil {
		return "", errors.New(errors.CodeOperationFailed, "kind", fmt.Sprintf("failed to set kubectl context: %s, error: %v", output, err), err)
	}

	return "localhost:5001", nil
}
