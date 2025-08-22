package kind

import (
	"context"
	"fmt"
	"runtime"

	"github.com/Azure/containerization-assist/pkg/infrastructure/core/runner"
)

type KindRunner interface {
	Version(ctx context.Context) (string, error)
	GetClusters(ctx context.Context) (string, error)
	DeleteCluster(ctx context.Context, name string) (string, error)
	Install(ctx context.Context) (string, error)
	SetupRegistry(ctx context.Context) (string, error)
}

type KindCmdRunner struct {
	runner runner.CommandRunner
}

var _ KindRunner = &KindCmdRunner{}

func NewKindCmdRunner(runner runner.CommandRunner) KindRunner {
	return &KindCmdRunner{
		runner: runner,
	}
}

func (k *KindCmdRunner) Version(ctx context.Context) (string, error) {
	return k.runner.RunCommand("kind", "version")
}

func (k *KindCmdRunner) GetClusters(ctx context.Context) (string, error) {
	return k.runner.RunCommand("kind", "get", "clusters")
}

func (k *KindCmdRunner) DeleteCluster(ctx context.Context, name string) (string, error) {
	return k.runner.RunCommand("kind", "delete", "cluster", "--name", name)
}

func (k *KindCmdRunner) Install(ctx context.Context) (string, error) {
	switch runtime.GOOS {
	case "linux":
		return k.runner.RunCommand("sh", "-c", "curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/")
	case "darwin":
		return k.runner.RunCommand("brew", "install", "kind")
	case "windows":
		return k.runner.RunCommand("powershell", "-Command", "winget install kind")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (k *KindCmdRunner) SetupRegistry(ctx context.Context) (string, error) {
	if runtime.GOOS == "windows" {
		return k.runner.RunCommand("powershell", "-Command", `
        if (-Not (docker network inspect kind -ErrorAction SilentlyContinue)) { docker network create kind }
        if (-Not (docker ps -q -f name=kind-registry)) { docker run -d --restart=always -p 5001:5000 --name kind-registry registry:2 }
        if (-Not (docker network inspect kind | Select-String kind-registry)) { docker network connect kind kind-registry }
        kind create cluster --name containerization-assist
		$configMap = @"
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
"@
        $configMap | Out-File -FilePath local-registry-hosting.yaml -Encoding ascii
        kubectl apply -f local-registry-hosting.yaml
        Remove-Item local-registry-hosting.yaml
        `)
	} else {
		return k.runner.RunCommand("sh", "-c", kindSetupScript)
	}
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
cat <<EOF | kind create cluster -n containerization-assist --config=-
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
for node in $(kind get nodes -n containerization-assist); do
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
