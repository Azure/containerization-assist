package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// validateKindInstalled checks if 'kind' is installed, installs it if missing.
func validateKindInstalled() error {
	cmd := exec.Command("kind", "version")
	if err := cmd.Run(); err != nil {
		fmt.Println("kind is not installed, attempting to install...")
		installCmd := exec.Command("sh", "-c", "curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/")
		if output, err := installCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to install kind: %s, error: %w", string(output), err)
		}
	}
	return nil
}

// setupLocalRegistry sets up a local Docker registry for kind.
func setupLocalRegistry() error {
	cmd := exec.Command("sh", "-c", `
	docker network inspect kind >/dev/null 2>&1 || docker network create kind
	docker ps | grep "kind-registry" || \
	docker run -d --restart=always -p 5001:5001 --name kind-registry registry:2
	`)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set up local registry: %s, error: %w", string(output), err)
	}
	return nil
}

// getKindCluster ensures a 'container-copilot' kind cluster exists and sets kubectl context.
func getKindCluster() (string, error) {
	if err := validateKindInstalled(); err != nil {
		return "", err
	}

	cmd := exec.Command("kind", "get", "clusters")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get kind clusters: %s, error: %w", string(output), err)
	}

	clusters := strings.Split(string(output), "\n")
	exists := false
	for _, cluster := range clusters {
		if strings.TrimSpace(cluster) == "container-copilot" {
			exists = true
			break
		}
	}

	if !exists {
		cmd = exec.Command("kind", "create", "cluster", "--name", "container-copilot")
		if output, err = cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to create kind cluster: %s, error: %w", string(output), err)
		}
	}

	cmd = exec.Command("kubectl", "config", "use-context", "kind-container-copilot")
	if output, err = cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to set kubectl context: %s, error: %w", string(output), err)
	}

	if err := setupLocalRegistry(); err != nil {
		return "", err
	}

	return "localhost:5001", nil
}
