# Getting Started & Development Guide

This guide covers installation, configuration, usage, and development setup for the Containerization Assistant MCP Server.

## Prerequisites

- **Node.js** 20 or higher
- **Docker** 20.10 or higher
- **kubectl** (optional, for Kubernetes deployments)
- **Git** (for development)

## Installation

### For Users
```bash
npm install -g @thgamble/containerization-assist-mcp
```

### For Development
```bash
git clone https://github.com/Azure/containerization-assist
cd containerization-assist
npm install
npm run build
```

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

## Development Workflow

### Development Commands
```bash
# Build & Development
npm run build          # Full build (ESM + CJS)
npm run dev            # Development server with auto-reload
npm start              # Start production server

# Code Quality
npm run lint           # ESLint code linting
npm run typecheck      # TypeScript type checking
npm run validate       # Run lint + typecheck + test
npm run fix            # Auto-fix lint + format issues

# Testing
npm test                   # Run all tests
npm run test:unit          # Unit tests only
npm run test:integration   # Integration tests via MCP Inspector
npm run mcp:inspect        # Start MCP inspector for testing
```

### Development with Hot Reload

The project includes `.vscode/mcp.json` for development:

```json
{
  "servers": {
    "containerization-assist-dev": {
      "command": "npx",
      "args": ["tsx", "./src/cli/cli.ts"],
      "env": {
        "MCP_MODE": "true",
        "MCP_QUIET": "true",
        "NODE_ENV": "development"
      }
    }
  }
}
```

Simply restart VS Code to enable the development MCP server.

## Available Tools

The MCP server provides 17 tools that work together seamlessly:

| Tool | Description |
|------|-------------|
| `analyze-repo` | Analyze repository structure and detect language/framework |
| `resolve-base-images` | Find optimal base images for applications |
| `generate-dockerfile` | Create optimized Dockerfiles |
| `fix-dockerfile` | Fix and optimize existing Dockerfiles |
| `build-image` | Build Docker images with progress tracking |
| `scan` | Security vulnerability scanning with Trivy |
| `tag-image` | Tag Docker images |
| `push-image` | Push images to registry |
| `generate-k8s-manifests` | Create Kubernetes deployment configurations |
| `prepare-cluster` | Prepare Kubernetes cluster for deployment |
| `deploy` | Deploy applications to Kubernetes |
| `verify-deploy` | Verify deployment health and status |
| `generate-aca-manifests` | Generate Azure Container Apps manifests |
| `convert-aca-to-k8s` | Convert ACA manifests to Kubernetes |
| `generate-helm-charts` | Generate Helm charts for deployments |
| `inspect-session` | Inspect session data for debugging |
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

## Development Best Practices

### Before Making Changes
1. **Check TypeScript compilation**: `npm run typecheck`
2. **Run linting**: `npm run lint`
3. **Ensure tests pass**: `npm test`

### After Making Changes
1. **Run quick validation**: `npm run validate`
2. **Fix any issues**: `npm run fix`
3. **Full validation**: `npm run validate:pr`

### Import Conventions
Use TypeScript path aliases instead of relative imports:

```typescript
// ✅ Correct - Use path aliases
import { createLogger } from '@lib/logger';
import type { ToolContext } from '@mcp/context';
import { Success, Failure } from '@types';

// ❌ Incorrect - Don't use relative imports
import { createLogger } from '../../lib/logger';
```

### Architecture Notes
- **Error Handling**: All functions use `Result<T>` pattern (no thrown exceptions)
- **Tool Pattern**: Each tool has co-located `tool.ts`, `schema.ts`, `index.ts`
- **Session Management**: Persistent state across tool executions

## Troubleshooting

### Common Development Issues

**TypeScript compilation errors:**
```bash
npm run clean && npm run build
```

**ESLint issues:**
```bash
npm run lint:fix
```

**Docker connection issues in tests:**
```bash
# Ensure Docker is running
docker ps

# Check mock mode
USE_MOCK_DOCKER=true npm test
```

**MCP connection issues:**
```bash
# Test with MCP Inspector
npx @modelcontextprotocol/inspector containerization-assist-mcp start

# Check logs
containerization-assist-mcp start --log-level debug
```

## Next Steps

- Explore the [Usage Examples](./examples/) for integration patterns
- Review the [Architecture Guide](../reference/architecture.md) to understand the system design
- Check the [External Usage Guide](./external-usage.md) for API integration
- See the [Session Management Guide](./sessions.md) for state management patterns
- Read the [Main README](../README.md) for complete feature overview