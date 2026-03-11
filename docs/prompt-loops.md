---
layout: doc
---

# Prompt Loops

Containerization Assist includes two interactive prompt loops, available as `/` slash commands in VS Code Copilot Chat. Each loop walks you through a full containerize-and-deploy workflow step by step.

## `kind-loop` — Local Development

Runs the full cycle locally using a [Kind](https://kind.sigs.k8s.io/) cluster:

1. Analyze your repository
2. Generate a Dockerfile
3. Build the image
4. Scan for vulnerabilities
5. Set up a local Kind cluster with a registry
6. Tag and push to the local registry
7. Generate Kubernetes manifests
8. Deploy to Kind
9. Verify the deployment

| Input | Required | Description |
| --- | --- | --- |
| `namespace` | No | Kubernetes namespace (defaults to `default`) |
| `imageName` | No | Image name (auto-detected from repo) |

## `aks-loop` — Azure Kubernetes Service

Same workflow, targeting a remote AKS cluster with Azure Container Registry:

1. Analyze your repository
2. Generate a Dockerfile
3. Build the image
4. Scan for vulnerabilities
5. Configure AKS credentials
6. Tag and push to ACR
7. Generate Kubernetes manifests
8. Deploy to AKS
9. Verify the deployment

| Input | Required | Description |
| --- | --- | --- |
| `registry` | **Yes** | ACR URL (e.g. `myregistry.azurecr.io`) |
| `resourceGroup` | **Yes** | Azure resource group containing the cluster |
| `clusterName` | **Yes** | AKS cluster name |
| `namespace` | No | Kubernetes namespace (defaults to `default`) |
| `imageName` | No | Image name (auto-detected from repo) |
