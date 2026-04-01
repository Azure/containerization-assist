---
layout: doc
---

# Workflow Tools

Containerization Assist includes three interactive workflow tools. Each returns a step-by-step plan that the AI walks you through. Tool output is collapsed by default in VS Code Copilot Chat, keeping the conversation clean.

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

## `create-containerization-policy` — Policy Authoring

Guided workflow for creating a custom OPA Rego policy. Returns a step-by-step plan with recommended defaults for each decision:

1. Choose policy scope (Dockerfile or Kubernetes manifests)
2. Choose policy location (project-local or global)
3. Select severity level
4. Define rules from suggested examples
5. Write and validate the policy

No inputs required — the tool provides an interactive plan the AI walks you through.
