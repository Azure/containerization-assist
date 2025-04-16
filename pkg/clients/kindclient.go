package clients

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// validateKindInstalled checks if 'kind' is installed, installs it if missing based on OS.
func (c *Clients) ValidateKindInstalled() error {
	if _, err := c.Kind.Version(); err != nil {
		fmt.Println("kind is not installed.")
		fmt.Print("Would you like to install kind? (y/n): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.ToLower(scanner.Text())
		if response != "y" {
			return fmt.Errorf("kind installation aborted")
		}
		fmt.Println("Attempting to install kind now for you...")
		if output, err := c.Kind.Install(); err != nil {
			return fmt.Errorf("failed to install kind: %s, error: %w", output, err)
		}
	}
	return nil
}

// setupLocalRegistry sets up a local Docker registry for kind, supporting multiple OS types.
func (c *Clients) SetupLocalRegistryCluster() error {
	if err := c.checkDockerRunning(); err != nil {
		return err
	}

	if output, err := c.Kind.SetupRegistry(); err != nil {
		return fmt.Errorf("failed to set up local registry: %s, error: %w", output, err)
	}

	return nil
}

// getKindCluster ensures a 'container-copilot' kind cluster exists and sets kubectl context.
func (c *Clients) GetKindCluster() (string, error) {
	if err := c.checkDockerRunning(); err != nil {
		return "", err
	}
	if err := c.ValidateKindInstalled(); err != nil {
		return "", err
	}

	output, err := c.Kind.GetClusters()
	if err != nil {
		return "", fmt.Errorf("failed to get kind clusters: %s, error: %w", output, err)
	}

	clusters := strings.Split(string(output), "\n")
	exists := false
	for _, cluster := range clusters {
		if strings.TrimSpace(cluster) == "container-copilot" {
			exists = true
			fmt.Println("found existing 'container-copilot' cluster")
			break
		}
	}

	if exists {
		fmt.Println("Deleting existing kind cluster 'container-copilot'")
		if output, err = c.Kind.DeleteCluster("container-copilot"); err != nil {
			return "", fmt.Errorf("failed to delete existing kind cluster: %s, error: %w", output, err)
		}
	}
	fmt.Println("Creating kind cluster 'container-copilot'")
	if err := c.SetupLocalRegistryCluster(); err != nil {
		return "", fmt.Errorf("setting up local registry cluster: %w", err)
	}

	fmt.Println("Setting kubectl context to 'kind-container-copilot'")
	if output, err = c.Kube.SetKubeContext("kind-container-copilot"); err != nil {
		return "", fmt.Errorf("failed to set kubectl context: %s, error: %w", string(output), err)
	}

	return "localhost:5001", nil
}
