package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// validateKindInstalled checks if 'kind' is installed, installs it if missing based on OS.
func validateKindInstalled() error {
	cmd := exec.Command("kind", "version")
	if err := cmd.Run(); err != nil {
		fmt.Println("kind is not installed.")
		fmt.Print("Would you like to install kind? (y/n): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.ToLower(scanner.Text())
		if response != "y" {
			return fmt.Errorf("kind installation aborted")
		}
		fmt.Println("Attempting to install kind now for you...")
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
func setupLocalRegistryCluster() error {
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
		cmd = exec.Command("sh", "-c", kindSetupScript)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set up local registry: %s, error: %w", string(output), err)
	}
	return nil
}

// getKindCluster ensures a 'container-copilot' kind cluster exists and sets kubectl context.
func getKindCluster() (string, error) {
	if err := checkDockerRunning(); err != nil {
		return "", err
	}
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
			fmt.Println("found existing 'container-copilot' cluster")
			break
		}
	}

	if exists {
		fmt.Println("Deleting existing kind cluster 'container-copilot'")
		cmd = exec.Command("kind", "delete", "cluster", "--name", "container-copilot")
		if output, err = cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to delete existing kind cluster: %s, error: %w", string(output), err)
		}
	}
	fmt.Println("Creating kind cluster 'container-copilot'")
	if err := setupLocalRegistryCluster(); err != nil {
		return "", fmt.Errorf("setting up local registry cluster: %w", err)
	}

	fmt.Println("Setting kubectl context to 'kind-container-copilot'")
	cmd = exec.Command("kubectl", "config", "use-context", "kind-container-copilot")
	if output, err = cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to set kubectl context: %s, error: %w", string(output), err)
	}

	return "localhost:5001", nil
}

const kindSetupScript = `
#!/bin/sh
set -o errexit

# 1. Create registry container unless it already exists
reg_name='kind-registry'
reg_port='5001'
if [ "$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
  docker run \
    -d --restart=always -p "127.0.0.1:${reg_port}:5000" --network bridge --name "${reg_name}" \
    registry:2
fi

# 2. Create kind cluster with containerd registry config dir enabled
#
# NOTE: the containerd config patch is not necessary with images from kind v0.27.0+
# It may enable some older images to work similarly.
# If you're only supporting newer relases, you can just use kind create cluster here.
#
# See:
# https://github.com/kubernetes-sigs/kind/issues/2875
# https://github.com/containerd/containerd/blob/main/docs/cri/config.md#registry-configuration
# See: https://github.com/containerd/containerd/blob/main/docs/hosts.md
cat <<EOF | kind create cluster -n container-copilot --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
EOF

# 3. Add the registry config to the nodes
#
# This is necessary because localhost resolves to loopback addresses that are
# network-namespace local.
# In other words: localhost in the container is not localhost on the host.
#
# We want a consistent name that works from both ends, so we tell containerd to
# alias localhost:${reg_port} to the registry container when pulling images
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${reg_port}"
for node in $(kind get nodes -n container-copilot); do
  docker exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | docker exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${reg_name}:5000"]
EOF
done

# 4. Connect the registry to the cluster network if not already connected
# This allows kind to bootstrap the network but ensures they're on the same network
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
  docker network connect "kind" "${reg_name}"
fi

# 5. Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
`
