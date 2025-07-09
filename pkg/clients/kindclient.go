package clients

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/container-kit/pkg/logger"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
			return mcperrors.NewError().Messagef("kind installation aborted").WithLocation().Build()
		}

		logger.Info("Attempting to install kind now for you...")
		if output, err := c.Kind.Install(ctx); err != nil {
			return mcperrors.NewError().Messagef("failed to install kind: %s, error: %w", output, err).WithLocation().Build()
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
		return mcperrors.NewError().Messagef("failed to set up local registry: %s, error: %w", output, err).WithLocation().Build(

		// getKindCluster ensures a 'container-kit' kind cluster exists and sets kubectl context.
		)
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
		return "", mcperrors.NewError().Messagef("failed to get kind clusters: %s, error: %w", output, err).WithLocation().Build()
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
			return "", mcperrors.NewError().Messagef("failed to delete existing kind cluster: %s, error: %w", output, err).WithLocation().Build()
		}
	}
	logger.Info("Creating kind cluster 'container-kit'")
	if err := c.SetupLocalRegistryCluster(ctx); err != nil {
		return "", mcperrors.NewError().Messagef("setting up local registry cluster: %w", err).WithLocation().Build()
	}

	logger.Info("Setting kubectl context to 'kind-container-kit'")
	if output, err = c.Kube.SetKubeContext(ctx, "kind-container-kit"); err != nil {
		return "", mcperrors.NewError().Messagef("failed to set kubectl context: %s, error: %w", output, err).WithLocation().Build()
	}

	return "localhost:5001", nil
}
