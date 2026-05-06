---
layout: doc
---

# Getting Started

Containerization Assist is an AI-powered MCP server that helps you build, scan, and deploy Docker containers and Kubernetes applications — with security-first OPA Rego policies built in.

## Install

One-click install for VS Code:

[![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_Containerization_Assist_MCP-0098FF?style=flat-square&logo=visualstudiocode&logoColor=ffffff)](https://azure.github.io/containerization-assist/vscode-mcp-install-redirect.html)

[![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Containerization_Assist_MCP-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=ffffff)](https://azure.github.io/containerization-assist/vscode-insiders-mcp-install-redirect.html)


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

## Workflow Tools

Three built-in workflow tools return step-by-step plans for common containerization tasks:

- **`kind-loop`** — Local dev loop using a Kind cluster (no required inputs)
- **`aks-loop`** — Remote deployment to AKS with Azure Container Registry
- **`create-containerization-policy`** — Guided policy authoring with recommended defaults

See [Workflow Tools](./workflow-tools.md) for details and input reference.

## Next steps

- [Policy Getting Started](./guides/policy-getting-started.md) — Quick start with the policy system
- [Policy Authoring](./guides/policy-authoring.md) — Write custom OPA Rego policies
- [SDK Integration Examples](./examples/README.md) — Use the SDK without MCP protocol
