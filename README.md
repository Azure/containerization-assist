# Containerization Assist MCP Server

An AI-powered containerization assistant that helps you build, scan, and deploy Docker containers through VS Code and other MCP-compatible tools.

## Features

- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications
- 🤖 **AI-Powered**: Intelligent Dockerfile generation and optimization with deterministic sampling
- 🧠 **Knowledge Enhanced**: AI-driven content improvement with security and performance best practices
- 🔄 **Intelligent Tool Routing**: Automatic dependency resolution and execution
- 📊 **Progress Tracking**: Real-time progress updates via MCP notifications
- 🔒 **Security Scanning**: Built-in vulnerability scanning with AI-powered suggestions
- ✨ **Smart Analysis**: Context-aware recommendations

## Installation

### Install from npm

```bash
npm install -g containerization-assist-mcp
```

### System Requirements

- Node.js 20+
- Docker or Docker Desktop
- Optional: Kubernetes (for deployment features)

## VS Code Setup

### Using the npm Package

1. Install the MCP server globally:
   ```bash
   npm install -g containerization-assist-mcp
   ```

2. Configure VS Code to use the MCP server. Add to your VS Code settings or create `.vscode/mcp.json` in your project:
   ```json
   {
     "servers": {
       "containerization-assist": {
         "command": "containerization-assist-mcp",
         "args": ["start"],
         "env": {
           "DOCKER_SOCKET": "/var/run/docker.sock",
           "LOG_LEVEL": "info"
         }
       }
     }
   }
   ```

3. Restart VS Code to enable the MCP server in GitHub Copilot.

### Windows Users

For Windows, use the Windows Docker pipe:
```json
"DOCKER_SOCKET": "//./pipe/docker_engine"
```

## Quick Start

The easiest way to understand the containerization workflow is through an end-to-end example:

### Single-App Containerization Journey

This MCP server guides you through a complete containerization workflow for a single application. The journey follows this sequence:

1. **Analyze** → Understand your application's language, framework, and dependencies
2. **Generate Dockerfile** → Create an optimized, security-hardened container configuration
3. **Build Image** → Compile your application into a Docker image
4. **Scan** → Identify security vulnerabilities and get remediation guidance
5. **Tag** → Apply appropriate version tags to your image
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
- **Deterministic AI generation**: All AI-powered tools use single-candidate sampling with scoring for quality validation
- **Real-time progress**: MCP notifications surface progress updates to clients during long-running operations

### Multi-Module/Monorepo Support

The server detects and supports monorepo structures with multiple independently deployable services:

- **Automatic Detection**: `analyze-repo` identifies monorepo patterns (npm workspaces, services/, apps/ directories)
- **Automated Multi-Module Generation**: `generate-dockerfile-plan` and `generate-k8s-manifests-plan` support multi-module workflows
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

| Tool | Description |
|------|-------------|
| `analyze-repo` | Analyze repository structure and detect language/framework |
| `generate-dockerfile-plan` | Plan Dockerfile generation strategy |
| `validate-dockerfile` | Validate Dockerfile syntax and best practices |
| `generate-manifest-plan` | Plan Kubernetes manifest generation strategy |

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
| `TEMP_DIR` | Temporary directory for build artifacts | System temp dir | No |
| `MCP_MODE` | Enable MCP protocol mode (logs to stderr) | `false` | No |
| `MCP_QUIET` | Suppress non-essential output in MCP mode | `false` | No |
| `CONTAINERIZATION_ASSIST_IMAGE_ALLOWLIST` | Comma-separated list of allowed base images | Empty | No |
| `CONTAINERIZATION_ASSIST_IMAGE_DENYLIST` | Comma-separated list of denied base images | Empty | No |
| `CONTAINERIZATION_ASSIST_TOOL_LOGS_DIR_PATH` | Directory path for tool execution logs (JSON format) | Disabled | No |

**Progress Notifications:**
Long-running operations (build, deploy, scan) emit real-time progress updates via MCP notifications. MCP clients can subscribe to these notifications to display progress to users.

**Deterministic AI Sampling:**
All AI-powered tools use deterministic sampling with `count: 1` to ensure reproducible outputs. Each generation includes scoring metadata for quality validation and diagnostics.

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


```bash
# Set policy file path
export POLICY_PATH=/path/to/policy.yaml

# Set policy environment (development, staging, production)
export POLICY_ENVIRONMENT=production
```

See `src/config/policy-schemas.ts`, `src/config/policy-io.ts`, `src/config/policy-eval.ts`, and `src/config/policy-data.ts` for implementation details.

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

# Check logs
containerization-assist-mcp start --log-level debug
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

## Documentation

- **[Design Document](./DESIGN_DOCUMENT.md)** - Architecture and system design
- **[Contributing Guidelines](./CONTRIBUTING.md)** - How to contribute
- **[Examples](./docs/examples/)** - Code examples and usage patterns

## External Telemetry Integration

Container Assist tools expose a clean, idiomatic API for external telemetry wrapping. This enables teams to track tool usage, performance metrics, and user behavior without modifying the core tool implementations.

### Tool Interface Properties

Each tool exposes the following properties for telemetry integration:

```typescript
import { buildImageTool } from 'containerization-assist-mcp';

// Access tool properties for telemetry
const {
  name,         // string: Tool identifier
  description,  // string: Human-readable description
  inputSchema,  // ZodRawShape: For MCP SDK registration
  parse,        // (args: unknown) => TypedInput: Zod validation
  handler,      // (input: TypedInput, context: ToolContext) => Promise<Result>
  metadata      // ToolMetadata: Knowledge enhancement info
} = buildImageTool;
```

### Telemetry Wrapper Pattern

Wrap tools with your telemetry layer while maintaining type safety:

```typescript
import { Server as McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { buildImageTool } from 'containerization-assist-mcp';

// Wrap with your telemetry
server.tool(
  buildImageTool.name,
  buildImageTool.description,
  buildImageTool.inputSchema,
  async (args, extra) => {
    const startTime = Date.now();

    try {
      // Step 1: Parse & validate with Zod
      const typedInput = buildImageTool.parse(args);

      // Step 2: Record telemetry with typed properties
      recordTelemetry({
        tool: buildImageTool.name,
        parameters: typedInput,
        timestamp: startTime
      });

      // Step 3: Execute tool handler
      const result = await buildImageTool.handler(typedInput, context);

      // Step 4: Record result metrics
      recordResult({
        tool: buildImageTool.name,
        success: result.ok,
        duration: Date.now() - startTime
      });

      return formatResponse(result);
    } catch (error) {
      recordError(error);
      throw error;
    }
  }
);
```

### Complete Example

See [docs/examples/app-mod-telemetry.ts](./docs/examples/app-mod-telemetry.ts) for a complete example showing:
- Full telemetry wrapper implementation
- Error tracking and metrics collection
- Type-safe parameter extraction
- Integration with all Container Assist tools

### Benefits

- **Type Safety**: Parse method provides strongly-typed input for handlers
- **Clean Separation**: Validation and execution are clearly separated
- **Flexible Integration**: Wrap with any telemetry system (Application Insights, DataDog, etc.)
- **No Modifications**: Zero changes needed to core tool implementations
- **Metadata Access**: Tool capabilities and enhancement info available for categorization

## For Developers

If you want to contribute or run from source:

### Development Setup

1. Clone the repository
2. Install dependencies: `npm install`
3. Build the project: `npm run build` (ESM + CJS dual build)
4. Run validation: `npm run validate` (lint + typecheck + unit tests)
5. Auto-fix issues: `npm run fix` (lint:fix + format)

**Essential Commands:**
- `npm run validate` - **Single validation command** for lint, typecheck, and tests (used locally and in CI)
- `npm run fix` - Auto-fix linting issues and format code
- `npm run build` - Full clean + ESM + CJS build
- `npm run build:esm` - ESM-only build (dist/)
- `npm run build:cjs` - CJS-only build (dist-cjs/)
- `npm run build:watch` - Watch mode for development
- `npm test` - Run all tests
- `npm run test:unit` - Run unit tests only

### Testing the Full Workflow

To verify the end-to-end containerization workflow works correctly, run the smoke test:

```bash
npm run smoke:journey
```

**What it does:**
This command executes the complete single-app workflow using real tool implementations:

- **Analyze repository** - Detects language and framework
- **Generate Dockerfile** - Creates optimized container configuration
- **Build Docker image** - Compiles the application
- **Tag image** - Applies version tags
- **Prepare Kubernetes cluster** - Sets up namespace (if K8s available)
- **Deploy to Kubernetes** - Deploys the application (if K8s available)
- **Verify deployment** - Confirms health and readiness (if K8s available)

**Requirements for smoke test:**
- Docker daemon running
- Kubernetes cluster optional (K8s steps will be skipped if unavailable)
- Test fixture: `test/__support__/fixtures/python-flask` directory

The smoke test validates that the sequential workflow functions as expected and provides logs for debugging.

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Support

See [SUPPORT.md](SUPPORT.md) for information on how to get help with this project.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft trademarks or logos is subject to and must [follow Microsoft’s Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks). Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship. Any use of third-party trademarks or logos are subject to those third-party’s policies.
