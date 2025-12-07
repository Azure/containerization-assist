# Containerization Assist MCP Server

[![Test Pipeline](https://github.com/Azure/containerization-assist/workflows/Test%20Pipeline/badge.svg)](https://github.com/Azure/containerization-assist/actions/workflows/test-pipeline.yml)
[![Version](https://img.shields.io/badge/version-0.0.1-orange.svg)](https://github.com/Azure/containerization-assist/blob/main/package.json)
[![MCP SDK](https://img.shields.io/badge/MCP%20SDK-1.17.3-blueviolet.svg)](https://github.com/modelcontextprotocol/sdk)
[![Node Version](https://img.shields.io/node/v/containerization-assist-mcp?color=brightgreen)](https://nodejs.org)

<!-- When repo is public, use dynamic version: [![Version](https://img.shields.io/github/package-json/v/Azure/containerization-assist?color=orange)](https://github.com/Azure/containerization-assist/blob/main/package.json) -->

An AI-powered containerization assistant that helps you build, scan, and deploy Docker containers through VS Code and other MCP-compatible tools.

## Features

### Core Capabilities

- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications
- 🤖 **AI-Powered**: Intelligent Dockerfile generation and optimization
- 🧠 **Knowledge Enhanced**: AI-driven content improvement with security and performance best practices
- 🔄 **Intelligent Tool Routing**: Automatic dependency resolution and execution
- 📊 **Progress Tracking**: Real-time progress updates via MCP notifications
- 🔒 **Security Scanning**: Built-in vulnerability scanning with AI-powered suggestions
- ✨ **Smart Analysis**: Context-aware recommendations
- **Policy-Driven System (v3.0)** 🆕
  - Pre-generation configuration
  - Knowledge filtering and weighting
  - Template injection
  - Semantic validation
  - Cross-tool consistency

### Policy System (v3.0) 🆕

Full control over containerization through Rego policies:

- **Configure Before Generation**: Set defaults for resources, base images, build strategy
- **Guide During Generation**: Filter knowledge base, inject templates automatically
- **Validate After Generation**: Semantic checks, security scoring, cross-tool consistency

**Example Policies Included**:
- Environment-based strategy (dev/staging/prod)
- Cost control by team tier
- Security-first organization
- Multi-cloud registry governance
- Speed-optimized development

See [Policy Authoring Guide](docs/guides/policy-authoring.md) for details.

## System Requirements

- Node.js 20+
- Docker or Docker Desktop
- Optional: [Trivy](https://aquasecurity.github.io/trivy/latest/getting-started/installation/) (for security scanning features)
- Optional: Kubernetes (for deployment features)

## VS Code Setup

Add the following to your VS Code settings or create `.vscode/mcp.json` in your project:

```json
{
  "servers": {
    "containerization-assist": {
      "command": "npx",
      "args": ["-y", "containerization-assist-mcp", "start"],
      "env": {
        "DOCKER_SOCKET": "/var/run/docker.sock",
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

Restart VS Code to enable the MCP server in GitHub Copilot.

### SDK Usage (Without MCP)

For direct tool usage without MCP protocol (e.g., VS Code extensions, programmatic access):

```typescript
import { analyzeRepo, buildImage, scanImage } from 'containerization-assist-mcp/sdk';

// Simple function calls - no MCP server needed
const analysis = await analyzeRepo({ repositoryPath: './myapp' });
if (analysis.ok) {
  console.log('Detected:', analysis.value.modules);
}

const build = await buildImage({ path: './myapp', imageName: 'myapp:v1' });
const scan = await scanImage({ imageId: 'myapp:v1' });
```

See [CLAUDE.md](./CLAUDE.md#sdk-usage-non-mcp) for full SDK documentation.

### Windows Users

For Windows, use the Windows Docker pipe:
```json
"DOCKER_SOCKET": "//./pipe/docker_engine"
```

## Quick Start

The easiest way to understand the containerization workflow is through an end-to-end example:

### Single-App Containerization Journey

This MCP server guides you through a complete containerization workflow for a single application. The journey follows this sequence:

1. **Analyze Repository** → Understand your application's language, framework, and dependencies
2. **Generate Dockerfile** → Create an optimized, security-hardened container configuration
3. **Build Image** → Compile your application into a Docker image
4. **Scan Image** → Identify security vulnerabilities and get remediation guidance
5. **Tag Image** → Apply appropriate version tags to your image
6. **Generate K8s Manifests** → Create deployment configurations for Kubernetes
7. **Prepare Cluster** → Set up namespace and prerequisites (if needed)
8. **Deploy** → Deploy your application to Kubernetes
9. **Verify** → Confirm deployment health and readiness

### Prerequisites

Before starting, ensure you have:

- **Docker**: Running Docker daemon with accessible socket (`docker ps` should work)
  - Linux/Mac: `/var/run/docker.sock` accessible
  - Windows: Docker Desktop with `//./pipe/docker_engine` accessible
- **Kubernetes** (optional, for deployment features):
  - Valid kubeconfig at `~/.kube/config`
  - Cluster connectivity (`kubectl cluster-info` should work)
  - Appropriate RBAC permissions for deployments, services, namespaces
- **Node.js**: Version 20 or higher
- **MCP Client**: VS Code with Copilot, Claude Desktop, or another MCP-compatible client

### Example Workflow with Natural Language

Once configured in your MCP client (VS Code Copilot, Claude Desktop, etc.), use natural language:

**Starting the Journey:**
```
"Analyze my Java application for containerization"
```

**Building the Container:**
```
"Generate an optimized Dockerfile with security best practices"
"Build a Docker image tagged myapp:v1.0.0"
"Scan the image for vulnerabilities"
```

**Deploying to Kubernetes:**
```
"Generate Kubernetes manifests for this application"
"Prepare my cluster and deploy to the default namespace"
"Verify the deployment is healthy"
```

### Single-Operator Model

This server is optimized for **one engineer containerizing one application at a time**. Key characteristics:

- **Sequential execution**: Each tool builds on the results of previous steps
- **Fast-fail validation**: Clear, actionable error messages if Docker/Kubernetes are unavailable
- **Deterministic AI generation**: Tools provide reproducible outputs through built-in prompt engineering
- **Real-time progress**: MCP notifications surface progress updates to clients during long-running operations

### Multi-Module/Monorepo Support

The server detects and supports monorepo structures with multiple independently deployable services:

- **Automatic Detection**: `analyze-repo` identifies monorepo patterns (npm workspaces, services/, apps/ directories)
- **Automated Multi-Module Generation**: `generate-dockerfile` and `generate-k8s-manifests` support multi-module workflows
- **Conservative Safeguards**: Excludes shared libraries and utility folders from containerization

**Multi-Module Workflow Example:**
```
1. "Analyze my monorepo at ./my-monorepo"
   → Detects 3 modules: api-gateway, user-service, notification-service

2. "Generate Dockerfiles"
   → Automatically creates Dockerfiles for all 3 modules:
     - services/api-gateway/Dockerfile
     - services/user-service/Dockerfile
     - services/notification-service/Dockerfile

3. "Generate K8s manifests"
   → Automatically creates manifests for all 3 modules

4. Optional: "Generate Dockerfile for user-service module"
   → Creates module-specific deployment manifests
```

**Detection Criteria:**
- Workspace configurations (npm, yarn, pnpm workspaces, lerna, nx, turborepo, cargo workspace)
- Separate package.json, pom.xml, go.mod, Cargo.toml per service
- Independent entry points and build configs
- EXCLUDES: shared/, common/, lib/, packages/utils directories

## Available Tools

The server provides 13 MCP tools organized by functionality:

### Analysis & Planning
| Tool | Description |
|------|-------------|
| `analyze-repo` | Analyze repository structure and detect technologies by parsing config files |

### Dockerfile Operations
| Tool | Description |
|------|-------------|
| `generate-dockerfile` | Gather insights from knowledge base and return requirements for Dockerfile creation |
| `fix-dockerfile` | Analyze Dockerfile for issues including organizational policy validation and return knowledge-based fix recommendations |

### Image Operations
| Tool | Description |
|------|-------------|
| `build-image` | Build Docker images from Dockerfiles with security analysis |
| `scan-image` | Scan Docker images for security vulnerabilities with remediation guidance (uses Trivy CLI) |
| `tag-image` | Tag Docker images with version and registry information |
| `push-image` | Push Docker images to a registry |

### Kubernetes Operations
| Tool | Description |
|------|-------------|
| `generate-k8s-manifests` | Gather insights and return requirements for Kubernetes/Helm/ACA/Kustomize manifest creation |
| `prepare-cluster` | Prepare Kubernetes cluster for deployment |
| `verify-deploy` | Verify Kubernetes deployment status |

### Utilities
| Tool | Description |
|------|-------------|
| `ops` | Operational utilities for ping and server status |

## Supported Technologies

### Languages & Frameworks
- **Java**: Spring Boot, Quarkus, Micronaut (Java 8-21)
- **.NET**: ASP.NET Core, Blazor (.NET 6.0+)

### Build Systems
- Maven, Gradle (Java)
- dotnet CLI (.NET)

## Configuration

### Environment Variables

The following environment variables control server behavior:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DOCKER_SOCKET` | Docker socket path | `/var/run/docker.sock` (Linux/Mac)<br>`//./pipe/docker_engine` (Windows) | Yes (for Docker features) |
| `DOCKER_TIMEOUT` | Docker operation timeout in milliseconds | `60000` (60s) | No |
| `KUBECONFIG` | Path to Kubernetes config file | `~/.kube/config` | No |
| `K8S_NAMESPACE` | Default Kubernetes namespace | `default` | No |
| `LOG_LEVEL` | Logging level | `info` | No |
| `WORKSPACE_DIR` | Working directory for operations | Current directory | No |
| `MCP_MODE` | Enable MCP protocol mode (logs to stderr) | `false` | No |
| `MCP_QUIET` | Suppress non-essential output in MCP mode | `false` | No |
| `CONTAINERIZATION_ASSIST_TOOL_LOGS_DIR_PATH` | Directory path for tool execution logs (JSON format) | Disabled | No |
| `CUSTOM_POLICY_PATH` | Directory path for custom policies (highest priority) | Not set | No |

**Progress Notifications:**
Long-running operations (build, deploy, scan-image) emit real-time progress updates via MCP notifications. MCP clients can subscribe to these notifications to display progress to users.

### Tool Execution Logging

Enable detailed logging of all tool executions to JSON files for debugging and auditing:

```bash
export CONTAINERIZATION_ASSIST_TOOL_LOGS_DIR_PATH=/path/to/logs
```

**Log File Format:**
- Filename: `ca-tool-logs-${timestamp}.jsonl`
- Example: `ca-tool-logs-2025-10-13T14-30-15-123Z.jsonl`

**Log Contents:**
```json
{
  "timestamp": "2025-10-13T14:30:15.123Z",
  "toolName": "analyze-repo",
  "input": { "path": "/workspace/myapp" },
  "output": { "language": "typescript", "framework": "express" },
  "success": true,
  "durationMs": 245,
  "error": "Error message if failed",
  "errorGuidance": {
    "hint": "Suggested fix",
    "resolution": "Step-by-step instructions"
  }
}
```

The logging directory is validated at startup to ensure it's writable.


### Policy System

The policy system uses **OPA Rego** for security, quality, and compliance enforcement. Rego is the industry-standard policy language from Open Policy Agent, providing expressive rules with rich built-in functions.

**Default Behavior (No Configuration Needed):**
By default, all policies in the `policies/` directory are automatically discovered and merged:
- `policies/security-baseline.rego` - Essential security rules (root user prevention, secrets detection, privileged containers)
- `policies/base-images.rego` - Base image governance (Microsoft Azure Linux recommendation, no :latest tag, deprecated versions)
- `policies/container-best-practices.rego` - Docker best practices (HEALTHCHECK, multi-stage builds, layer optimization)

This provides comprehensive out-of-the-box security and quality enforcement.

### Policy Customization

The policy system supports three priority-ordered search paths for easy customization:

**Priority Order (highest to lowest):**
1. **Custom directory** via `CUSTOM_POLICY_PATH` environment variable (NPM users)
2. **`policies.user/` directory** in your repository (source installation users)
3. **Built-in `policies/`** (baseline policies)

Later policies override earlier policies during merging by package namespace.

#### Quick Start: Source Installation (10 seconds)

```bash
# Copy example policy to policies.user/
mkdir -p policies.user
cp policies.user.examples/allow-all-registries.rego policies.user/

# Restart your MCP client (VS Code, Claude Desktop, etc.)
```

#### Quick Start: NPM Installation (30 seconds)

```bash
# 1. Create custom policy directory
mkdir -p ~/.config/containerization-assist/policies

# 2. Copy example policy
cp node_modules/containerization-assist-mcp/policies.user.examples/allow-all-registries.rego \
   ~/.config/containerization-assist/policies/

# 3. Configure environment variable in .vscode/mcp.json
{
  "servers": {
    "containerization-assist": {
      "env": {
        "CUSTOM_POLICY_PATH": "${env:HOME}/.config/containerization-assist/policies"
      }
    }
  }
}

# 4. Restart VS Code
```

#### Pre-Built Example Policies

The `policies.user.examples/` directory includes three ready-to-use examples:

| Example | Purpose | Use Case |
|---------|---------|----------|
| `allow-all-registries.rego` | Override MCR preference | Docker Hub, GCR, ECR, private registries |
| `warn-only-mode.rego` | Advisory-only enforcement | Testing, gradual adoption, dev environments |
| `custom-organization-template.rego` | Organization template | Custom labels, registries, compliance |

See [policies.user.examples/README.md](policies.user.examples/README.md) for detailed usage.

#### Built-In Policies

Three production-ready Rego policies are included by default:

- **`policies/security-baseline.rego`** - Essential security rules (root user prevention, secrets detection, privileged containers)
- **`policies/base-images.rego`** - Base image governance (Microsoft Azure Linux recommendation, no :latest tag, deprecated versions)
- **`policies/container-best-practices.rego`** - Docker best practices (HEALTHCHECK, multi-stage builds, layer optimization)

User policies override built-in policies by package namespace.

**Policy File Format (Rego):**

```rego
package containerization.custom_policy

# Blocking violations
violations contains result if {
  input_type == "dockerfile"
  regex.match(`FROM\s+[^:]+:latest`, input.content)

  result := {
    "rule": "block-latest-tag",
    "category": "quality",
    "priority": 80,
    "severity": "block",
    "message": "Using :latest tag is not allowed. Specify explicit version tags.",
    "description": "Prevent :latest for reproducibility",
  }
}

# Non-blocking warnings
warnings contains result if {
  input_type == "dockerfile"
  not regex.match(`HEALTHCHECK`, input.content)

  result := {
    "rule": "suggest-healthcheck",
    "category": "quality",
    "priority": 70,
    "severity": "warn",
    "message": "Consider adding HEALTHCHECK instruction for container monitoring",
    "description": "HEALTHCHECK improves container lifecycle management",
  }
}

# Policy decision
default allow := false
allow if count(violations) == 0

# Result structure
result := {
  "allow": allow,
  "violations": violations,
  "warnings": warnings,
  "suggestions": [],
  "summary": {
    "total_violations": count(violations),
    "total_warnings": count(warnings),
    "total_suggestions": 0,
  },
}
```

**Priority Levels:**
- **90-100**: Security rules (highest priority)
- **70-89**: Quality rules
- **50-69**: Performance rules
- **30-49**: Compliance rules

**Using Policies:**

```bash
# List discovered policies
npx containerization-assist-mcp list-policies

# List policies and show merged result
npx containerization-assist-mcp list-policies --show-merged

# Validate Dockerfile with policies (automatic discovery)
npx containerization-assist-mcp fix-dockerfile --path ./Dockerfile
```

**Creating Custom Policies:**

See [Policy Customization Guide](docs/guides/policy-getting-started.md) and existing policies in `policies/` for examples.

**Testing Policies:**

```bash
# Validate policy syntax
opa check policies.user/my-policy.rego

# Run policy tests
opa test policies.user/

# Test with MCP Inspector
npx @modelcontextprotocol/inspector containerization-assist-mcp start
```

### MCP Inspector (Testing)

```bash
npx @modelcontextprotocol/inspector containerization-assist-mcp start
```

## Troubleshooting

### Docker Connection Issues

```bash
# Check Docker is running
docker ps

# Check socket permissions (Linux/Mac)
ls -la /var/run/docker.sock

# For Windows, ensure Docker Desktop is running
```

### MCP Connection Issues

```bash
# Test with MCP Inspector
npx @modelcontextprotocol/inspector containerization-assist-mcp start

# Check logs with debug level
npx -y containerization-assist-mcp start --log-level debug
```

### Kubernetes Connection Issues

The server performs fast-fail validation when Kubernetes tools are used. If you encounter Kubernetes errors:

**Kubeconfig Not Found**
```bash
# Check if kubeconfig exists
ls -la ~/.kube/config

# Verify kubectl can connect
kubectl cluster-info

# If using cloud providers, update kubeconfig:
# AWS EKS
aws eks update-kubeconfig --name <cluster-name> --region <region>

# Google GKE
gcloud container clusters get-credentials <cluster-name> --zone <zone>

# Azure AKS
az aks get-credentials --resource-group <rg> --name <cluster-name>
```

**Connection Timeout or Refused**
```bash
# Verify cluster is running
kubectl get nodes

# Check API server address
kubectl config view

# Test connectivity to API server
kubectl cluster-info dump

# Verify firewall rules allow connection to API server port (typically 6443)
```

**Authentication or Authorization Errors**
```bash
# Check current context and user
kubectl config current-context
kubectl config view --minify

# Test permissions
kubectl auth can-i create deployments --namespace default
kubectl auth can-i create services --namespace default

# If using cloud providers, refresh credentials:
# AWS EKS: re-run update-kubeconfig
# GKE: run gcloud auth login
# AKS: run az login
```

**Invalid or Missing Context**
```bash
# List available contexts
kubectl config get-contexts

# Set a context
kubectl config use-context <context-name>

# View current configuration
kubectl config view
```

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Support

See [SUPPORT.md](SUPPORT.md) for information on how to get help with this project.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft trademarks or logos is subject to and must [follow Microsoft’s Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks). Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship. Any use of third-party trademarks or logos are subject to those third-party’s policies.
