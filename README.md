# Containerization Assist MCP Server

An AI-powered containerization assistant that helps you build, scan, and deploy Docker containers through VS Code and other MCP-compatible tools.

## Features

- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications
- 🤖 **AI-Powered**: Intelligent Dockerfile generation and optimization with N-best sampling
- 🧠 **Knowledge Enhanced**: AI-driven content improvement with security and performance best practices
- 🔄 **Intelligent Tool Routing**: Automatic dependency resolution and execution
- 📊 **Progress Tracking**: Real-time progress updates via MCP
- 🔒 **Security Scanning**: Built-in vulnerability scanning with AI-powered suggestions
- ✨ **Smart Analysis**: Context-aware recommendations and optimization across 14 AI-enhanced tools

## Installation

### Install from npm

```bash
npm install -g @thgamble/containerization-assist-mcp
```

### System Requirements

- Node.js 20+
- Docker or Docker Desktop
- Optional: Kubernetes (for deployment features)

## VS Code Setup

### Using the npm Package

1. Install the MCP server globally:
   ```bash
   npm install -g @thgamble/containerization-assist-mcp
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

## Usage Examples

Once installed and configured, you can use natural language commands with GitHub Copilot or Claude Desktop:

### Basic Commands

- **"Analyze my Node.js application for containerization"**
- **"Generate a Dockerfile for this Python project"**
- **"Build and scan a Docker image"**
- **"Create Kubernetes deployment manifests"**
- **"Analyze and containerize my application"**

### Step-by-Step Containerization

1. **Analyze your project:**
   ```
   "Analyze the repository at /path/to/my-app"
   ```

2. **Generate Dockerfile:**
   ```
   "Create an optimized Dockerfile for this Node.js app"
   ```

3. **Build image:**
   ```
   "Build a Docker image with tag myapp:latest"
   ```

4. **Scan for vulnerabilities:**
   ```
   "Scan the image for security issues"
   ```

5. **Deploy to Kubernetes:**
   ```
   "Generate Kubernetes manifests and deploy the application"
   ```

### AI-Enhanced Features

Take advantage of AI-powered insights and optimizations:

- **"Generate an optimized Dockerfile with security best practices"**
- **"Scan my image and provide AI-powered remediation suggestions"**
- **"Analyze my deployment and suggest performance optimizations"**
- **"Fix this Dockerfile with knowledge-enhanced improvements"**
- **"Deploy with intelligent resource optimization"**

## Available Tools

| Tool | Description | AI Enhanced |
|------|-------------|-------------|
| `analyze-repo` | Analyze repository structure and detect language/framework | ✅ |
| `resolve-base-images` | Find optimal base images for applications | ✅ |
| `generate-dockerfile` | Create optimized Dockerfiles with knowledge enhancement | ✅ |
| `fix-dockerfile` | Fix and optimize existing Dockerfiles | ✅ |
| `build-image` | Build Docker images with optimization suggestions | ✅ |
| `scan` | Security vulnerability scanning with AI-powered recommendations | ✅ |
| `tag-image` | Tag Docker images with intelligent strategies | ✅ |
| `push-image` | Push images to registry with optimization guidance | ✅ |
| `generate-k8s-manifests` | Create Kubernetes deployment configurations | ✅ |
| `generate-helm-charts` | Generate Helm charts with template optimization | ✅ |
| `generate-aca-manifests` | Create Azure Container Apps manifests | ✅ |
| `convert-aca-to-k8s` | Convert Azure Container Apps to Kubernetes | ✅ |
| `prepare-cluster` | Prepare Kubernetes cluster with optimization advice | ✅ |
| `deploy` | Deploy applications with intelligent analysis | ✅ |
| `verify-deployment` | Verify deployment health with AI diagnostics | ✅ |
| `ops` | Operational tools with intelligent insights | ✅ |
| `generate-kustomize` | Generate Kustomize configurations | ❌ |
| `inspect-session` | Debug and analyze tool execution sessions | ❌ |

## Supported Technologies

### Languages & Frameworks
- **Java**: Spring Boot, Quarkus, Micronaut (Java 8-21)
- **Node.js**: Express, NestJS, Fastify, Next.js
- **Python**: FastAPI, Django, Flask (Python 3.8+)
- **Go**: Gin, Echo, Fiber (Go 1.19+)
- **.NET**: ASP.NET Core, Blazor (.NET 6.0+)
- **Others**: Ruby, PHP, Rust

### Build Systems
- Maven, Gradle (Java)
- npm, yarn, pnpm (Node.js)
- pip, poetry, pipenv (Python)
- go mod (Go)
- dotnet CLI (.NET)

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DOCKER_SOCKET` | Docker socket path | `/var/run/docker.sock` |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `MCP_MODE` | Enable MCP mode | `true` |
| `MCP_QUIET` | Suppress non-MCP output | `true` |
| `AI_ENHANCEMENT_ENABLED` | Enable AI enhancement features | `true` |
| `AI_ENHANCEMENT_CONFIDENCE` | Default confidence threshold for AI suggestions | `0.8` |
| `AI_ENHANCEMENT_MAX_SUGGESTIONS` | Maximum AI suggestions per request | `5` |

### Project Configuration

Create `.containerization-config.json` in your project root for custom settings:

```json
{
  "docker": {
    "registry": "docker.io",
    "buildkit": true
  },
  "security": {
    "scanOnBuild": true
  },
  "kubernetes": {
    "namespace": "default"
  },
  "aiEnhancement": {
    "enabled": true,
    "confidence": 0.8,
    "focus": "security",
    "includeExamples": true
  }
}
```

## Alternative MCP Clients

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
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

## Documentation

- **[Getting Started Guide](./docs/getting-started.md)** - Detailed setup and first use
- **[AI Enhancement System](./docs/ai-enhancement.md)** - AI-powered features and capabilities
- **[Tool Capabilities Reference](./docs/tool-capabilities.md)** - Complete tool reference with AI enhancement details
- **[Architecture Guide](./docs/architecture.md)** - System design and components
- **[Development Guide](./docs/development-setup.md)** - Contributing and development setup
- **[Documentation Index](./docs/README.md)** - All available documentation

## For Developers

If you want to contribute or run from source, see the [Development Setup Guide](./docs/development-setup.md).

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Support

- GitHub Issues: https://github.com/azure/containerization-assist/issues
- Documentation: https://github.com/azure/containerization-assist/tree/main/docs

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft trademarks or logos is subject to and must [follow Microsoft’s Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks). Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship. Any use of third-party trademarks or logos are subject to those third-party’s policies.
