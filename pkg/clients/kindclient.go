package clients

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/container-copilot/pkg/logger"
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
			return fmt.Errorf("kind installation aborted")
		}

		logger.Info("Attempting to install kind now for you...")
		if output, err := c.Kind.Install(ctx); err != nil {
			return fmt.Errorf("failed to install kind: %s, error: %w", output, err)
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
		return fmt.Errorf("failed to set up local registry: %s, error: %w", output, err)
	}

	return nil
}

// getKindCluster ensures a 'container-copilot' kind cluster exists and sets kubectl context.
func (c *Clients) GetKindCluster(ctx context.Context) (string, error) {
	if err := c.checkDockerRunning(ctx); err != nil {
		return "", err
	}
	if err := c.ValidateKindInstalled(ctx); err != nil {
		return "", err
	}

	output, err := c.Kind.GetClusters(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get kind clusters: %s, error: %w", output, err)
	}

	clusters := strings.Split(string(output), "\n")
	exists := false
	for _, cluster := range clusters {
		if strings.TrimSpace(cluster) == "container-copilot" {
			exists = true
			logger.Infof("found existing 'container-copilot' cluster")
			break
		}
	}

	if exists {
		logger.Warn("Deleting existing kind cluster 'container-copilot'")
		if output, err = c.Kind.DeleteCluster(ctx, "container-copilot"); err != nil {
			return "", fmt.Errorf("failed to delete existing kind cluster: %s, error: %w", output, err)
		}
	}
	logger.Info("Creating kind cluster 'container-copilot'")
	if err := c.SetupLocalRegistryCluster(ctx); err != nil {
		return "", fmt.Errorf("setting up local registry cluster: %w", err)
	}

	logger.Info("Setting kubectl context to 'kind-container-copilot'")
	if output, err = c.Kube.SetKubeContext(ctx, "kind-container-copilot"); err != nil {
		return "", fmt.Errorf("failed to set kubectl context: %s, error: %w", output, err)
	}

	return "localhost:5001", nil
}
