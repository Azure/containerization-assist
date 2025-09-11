# Getting Started

This guide will help you install, configure, and use the Containerization Assistant MCP Server.

## Prerequisites

- **Node.js** 20 or higher
- **Docker** 20.10 or higher  
- **kubectl** (optional, for Kubernetes deployments)

## Installation

```bash
npm install -g @azure/containerization-assist-mcp
```

> For development setup, see the [Development Setup Guide](./development-setup.md)

## Configuration

### With VS Code / GitHub Copilot (Recommended)

After installing the package globally, configure VS Code to use the MCP server. Create `.vscode/mcp.json` in your project:

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

For Windows users, use:
```json
"DOCKER_SOCKET": "//./pipe/docker_engine"
```

Simply restart VS Code to enable the MCP server in GitHub Copilot.

### With Claude Desktop

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

### With MCP Inspector (For Testing)

```bash
npx @modelcontextprotocol/inspector containerization-assist-mcp start
```

## First Containerization

Once configured, you can use natural language commands with GitHub Copilot or Claude to containerize your applications.

### Quick Start Commands

Simply ask your AI assistant:

1. **"Analyze my Node.js application for containerization"**
   - The assistant will analyze your repository structure and dependencies

2. **"Generate a Dockerfile for this project"**
   - Creates an optimized Dockerfile based on the analysis

3. **"Build a Docker image with tag myapp:latest"**
   - Builds the Docker image with progress tracking

4. **"Scan the image for security vulnerabilities"**
   - Runs Trivy security scanning on the built image

5. **"Deploy to Kubernetes"**
   - Generates manifests and deploys to your cluster

### Complete Workflow

For a complete containerization workflow, simply say:

**"Start a containerization workflow for my application"**

This will automatically:
- Analyze your repository
- Generate an optimized Dockerfile
- Build the Docker image
- Scan for vulnerabilities
- Optionally deploy to Kubernetes

### Programmatic Usage

For developers who want to integrate directly, see the [examples](./examples/) directory for code samples using the MCP client libraries.

## Available Tools

The MCP server provides 14 tools that work together seamlessly:

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

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DOCKER_SOCKET` | Docker socket path | `/var/run/docker.sock` |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `MCP_MODE` | Enable MCP mode | `true` |
| `MCP_QUIET` | Suppress non-MCP output | `true` |
| `NODE_ENV` | Environment (development, production) | `production` |

## Configuration File

Create `.containerization-config.json` in your project root:

```json
{
  "ai": {
    "enabled": true,
    "model": "gpt-4"
  },
  "docker": {
    "registry": "docker.io",
    "timeout": 300,
    "buildkit": true
  },
  "kubernetes": {
    "context": "default",
    "namespace": "default"
  },
  "security": {
    "scanOnBuild": true,
    "blockOnCritical": false
  }
}
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

### Build Issues

```bash
# Clean build
npm run clean
npm run build

# Check TypeScript compilation
npm run typecheck

# Run tests
npm test
```

## Next Steps

- Explore the [Usage Examples](./examples/) for integration patterns
- Review the [Architecture Guide](./architecture.md) to understand the system design
- For contributing, see the [Development Setup Guide](./development-setup.md)
- Check the [Main README](../README.md) for complete feature overview