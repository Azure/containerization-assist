# Containerization Assist MCP Server

An AI-powered containerization assistant that helps you build, scan, and deploy Docker containers through VS Code and other MCP-compatible tools.

## Features

- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications  
- 🤖 **AI-Powered**: Intelligent Dockerfile generation and optimization
- 🔄 **Workflow Orchestration**: Complete containerization pipelines
- 📊 **Progress Tracking**: Real-time progress updates via MCP
- 🔒 **Security Scanning**: Built-in vulnerability scanning with Trivy

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
- **"Start a complete containerization workflow"**

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

## Available Tools

| Tool | Description |
|------|-------------|
| `analyze_repository` | Analyze repository structure and detect language/framework |
| `resolve_base_images` | Find optimal base images for applications |
| `generate_dockerfile` | Create optimized Dockerfiles |
| `fix_dockerfile` | Fix and optimize existing Dockerfiles |
| `build_image` | Build Docker images with progress tracking |
| `scan_image` | Security vulnerability scanning with Trivy |
| `tag_image` | Tag Docker images |
| `push_image` | Push images to registry |
| `generate_k8s_manifests` | Create Kubernetes deployment configurations |
| `prepare_cluster` | Prepare Kubernetes cluster for deployment |
| `deploy_application` | Deploy applications to Kubernetes |
| `verify_deployment` | Verify deployment health and status |
| `start_workflow` | Start complete containerization workflow |
| `ops` | Operational tools (ping, health, registry) |

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
- **[Architecture Guide](./docs/architecture.md)** - System design and components
- **[Development Guide](./docs/development-setup.md)** - Contributing and development setup
- **[Documentation Index](./docs/README.md)** - All available documentation

## For Developers

If you want to contribute or run from source, see the [Development Setup Guide](./docs/development-setup.md).

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Support

- GitHub Issues: https://github.com/azure/container-assist/issues
- Documentation: https://github.com/azure/container-assist/tree/main/docs