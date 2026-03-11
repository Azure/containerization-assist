---
layout: doc
---

# Getting Started

Containerization Assist is an AI-powered MCP server that helps you build, scan, and deploy Docker containers and Kubernetes applications — with security-first OPA Rego policies built in.

## Install

One-click install for VS Code:

[![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_Containerization_Assist_MCP-0098FF?style=flat-square&logo=visualstudiocode&logoColor=ffffff)](https://insiders.vscode.dev/redirect/mcp/install?name=ca&config=%7B%22type%22%3A%22stdio%22%2C%22command%22%3A%22npx%22%2C%22args%22%3A%5B%22-y%22%2C%22containerization-assist-mcp%22%2C%22start%22%5D%7D)

[![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Containerization_Assist_MCP-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=ffffff)](https://insiders.vscode.dev/redirect/mcp/install?name=ca&config=%7B%22type%22%3A%22stdio%22%2C%22command%22%3A%22npx%22%2C%22args%22%3A%5B%22-y%22%2C%22containerization-assist-mcp%22%2C%22start%22%5D%7D&quality=insiders)

Or add the following to `.vscode/mcp.json` in your project:

```json
{
  "servers": {
    "ca": {
      "command": "npx",
      "args": ["-y", "containerization-assist-mcp", "start"],
      "env": {
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

Restart VS Code to enable the server in GitHub Copilot.

## What it does

- **Docker Integration** — Build, scan, and deploy container images with intelligent Dockerfile generation
- **Kubernetes Support** — Generate manifests and deploy to your cluster with built-in verification
- **Policy-Driven Security** — Full control through OPA Rego policies for security and compliance
- **AI-Powered Analysis** — Context-aware recommendations with security best practices

## Prerequisites

- Node.js 20+
- Docker or Docker Desktop
- Optional: [Trivy](https://aquasecurity.github.io/trivy/latest/getting-started/installation/) for security scanning
- Optional: Kubernetes cluster for deployment features

## Prompt Loops

Containerization Assist includes two interactive prompt loops, available as `/` slash commands in VS Code Copilot Chat. Each loop walks you through a full containerize-and-deploy workflow step by step.

### `kind-loop` — Local Development

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

### `aks-loop` — Azure Kubernetes Service

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
## Next steps

- [Policy Getting Started](./guides/policy-getting-started.md) — Quick start with the policy system
- [Policy Authoring](./guides/policy-authoring.md) — Write custom OPA Rego policies
- [SDK Integration Examples](./examples/README.md) — Use the SDK without MCP protocol
