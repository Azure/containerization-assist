package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// validateKindInstalled checks if 'kind' is installed, installs it if missing based on OS.
func validateKindInstalled() error {
	cmd := exec.Command("kind", "version")
	if err := cmd.Run(); err != nil {
		fmt.Println("kind is not installed, attempting to install...")
		switch runtime.GOOS {
		case "linux":
			installCmd := exec.Command("sh", "-c", "curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/")
			if output, err := installCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to install kind: %s, error: %w", string(output), err)
			}
		case "darwin":
			// Check if brew is installed
			if _, err := exec.LookPath("brew"); err != nil {
				return fmt.Errorf("Homebrew is not installed. Please install Homebrew first: https://brew.sh/")
			}
			installCmd := exec.Command("brew", "install", "kind")
			if output, err := installCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to install kind on macOS: %s, error: %w", string(output), err)
			}
		case "windows":
			installCmd := exec.Command("powershell", "-Command", "winget install kind")
			if output, err := installCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to install kind on Windows: %s, error: %w", string(output), err)
			}
		default:
			return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
		}
	}
	return nil
}

// setupLocalRegistry sets up a local Docker registry for kind, supporting multiple OS types.
func setupLocalRegistry() error {
	if err := checkDockerRunning(); err != nil {
		return err
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", `
		if (-Not (docker network inspect kind -ErrorAction SilentlyContinue)) { docker network create kind }
		if (-Not (docker ps -q -f name=kind-registry)) { docker run -d --restart=always -p 5001:5001 --name kind-registry registry:2 }
		if (-Not (docker network inspect kind | Select-String kind-registry)) { docker network connect kind kind-registry }
		kubectl apply -f - <<EOF
		apiVersion: v1
		kind: ConfigMap
		metadata:
		  name: local-registry-hosting
		  namespace: kube-public
		data:
		  localRegistryHosting.v1: |
		    host: "localhost:5001"
		    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
		EOF
		`)
	} else {
		cmd = exec.Command("sh", "-c", `
		docker network inspect kind >/dev/null 2>&1 || docker network create kind
		docker ps | grep "kind-registry" || \
		docker run -d --restart=always -p 5001:5001 --name kind-registry registry:2

		# Connect the registry to the cluster network if not already connected
		if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' kind-registry)" = "null" ]; then
		  docker network connect "kind" "kind-registry"
		fi

		# Document the local registry
		cat <<EOF | kubectl apply -f -
		apiVersion: v1
		kind: ConfigMap
		metadata:
		  name: local-registry-hosting
		  namespace: kube-public
		data:
		  localRegistryHosting.v1: |
		    host: "localhost:5001"
		    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
		EOF
		`)
	}

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

	cmd = exec.Command("kubectl", "config", "use-context", "container-copilot")
	if output, err = cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to set kubectl context: %s, error: %w", string(output), err)
	}

	if err := setupLocalRegistry(); err != nil {
		return "", err
	}

	return "localhost:5001", nil
}
