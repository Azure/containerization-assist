# Containerization Assist MCP Server

An AI-powered containerization assistant that helps you build, scan, and deploy Docker containers through VS Code and other MCP-compatible tools.

## Features

- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications
- 🤖 **AI-First Architecture**: Intelligent decision-making powered by prompts and knowledge bases
- 🔄 **Unified Tool Pattern**: Consistent, composable tool execution with explicit dependencies
- 📊 **Progress Tracking**: Real-time progress updates via MCP
- 🔒 **Security Scanning**: Built-in vulnerability scanning with Trivy
- 📋 **Session Management**: Persistent context across tool executions
- 🎯 **Prompt-Backed Intelligence**: All business logic driven by YAML prompts and knowledge packs

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

## Available Tools

The server provides 17 tools organized into two categories:

### AI-Powered Tools (11)
| Tool | Description |
|------|-------------|
| `analyze-repo` | AI-powered repository analysis and framework detection |
| `generate-dockerfile` | Intelligent Dockerfile generation with best practices |
| `fix-dockerfile` | AI-assisted Dockerfile optimization and fixes |
| `build-image` | Docker image building with AI error handling |
| `scan` | Security scanning with AI vulnerability analysis |
| `deploy` | Kubernetes deployment with AI strategy selection |
| `generate-k8s-manifests` | AI-driven Kubernetes manifest generation |
| `resolve-base-images` | AI-powered base image recommendations |
| `generate-aca-manifests` | Azure Container Apps manifest generation |
| `convert-aca-to-k8s` | Convert ACA manifests to Kubernetes |
| `generate-helm-charts` | AI-powered Helm chart generation |

### Infrastructure Tools (6)
| Tool | Description |
|------|-------------|
| `ops` | Server status and ping utilities |
| `tag-image` | Docker image tagging operations |
| `push-image` | Push images to Docker registry |
| `prepare-cluster` | Kubernetes cluster preparation |
| `inspect-session` | Session data inspection for debugging |
| `verify-deploy` | Kubernetes deployment verification |

## Architecture Patterns

### AI-First Design (Migration Complete ✅)
This project has successfully completed its migration to an **AI-First Architecture** achieving:
- **74% reduction** in core complexity (prompt-backed-tool.ts: 901 → 235 lines)
- **100% tool consistency** (all 17 tools use standardized patterns)
- **100% prompt standardization** (all YAML format, no JSON)
- **100% AI provenance tracking** (every AI call tracked with hash)

Key architectural principles:
- **Business logic** lives in YAML prompts and knowledge bases
- **TypeScript code** handles only deterministic operations (Docker calls, file I/O, K8s operations)
- **Decision-making** is delegated to AI via structured prompts
- **Context** is maintained through sessions and passed explicitly between tools

### Unified Tool Pattern
All tools follow a consistent pattern:
```typescript
export const tool = {
  name: 'tool_name',
  description: 'Tool description',
  inputSchema: zodSchema,
  execute: async (params, deps, context) => {
    // AI-powered decision making via prompts
    // Deterministic side effects only
    // Session management for context
  }
};
```

### Key Principles
- **Explicit Dependencies**: No global dependency injection
- **Prompt-Driven**: All heuristics and analysis via YAML prompts
- **Deterministic**: Same inputs always produce same outputs
- **Composable**: Tools can be chained and combined
- **Traceable**: Full provenance tracking for AI decisions

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

- **[Getting Started Guide](./docs/guides/getting-started.md)** - Detailed setup and first use
- **[Architecture Guide](./docs/reference/architecture.md)** - System design and components
- **[Development Guide](./docs/development/language-framework-guide.md)** - Language support and development
- **[Documentation Index](./docs/README.md)** - All available documentation

## For Developers

### Development Setup

```bash
# Clone and install dependencies
git clone https://github.com/Azure/containerization-assist.git
cd containerization-assist
npm install
```

### Build System

The project uses a dual-build system to support both ESM and CommonJS:

```bash
# Full build (ESM + CJS)
npm run build

# ESM build only
npm run build:esm

# CJS build only
npm run build:cjs

# Development with hot reload
npm run dev
```

### Available Scripts

```bash
# Development
npm run dev              # Development server with watch
npm run validate         # Quick validation (lint, typecheck, test)

# Testing
npm test                 # All tests
npm run test:unit        # Unit tests only
npm run mcp:inspect      # Test with MCP Inspector

# Quality
npm run lint             # Check linting
npm run lint:fix         # Fix linting issues
npm run typecheck        # TypeScript type checking
npm run format           # Format code with Prettier
```

### Architecture

This project follows an **AI-First Architecture** where:
- Business logic lives in YAML prompts and knowledge bases
- TypeScript handles deterministic operations (Docker, file I/O, K8s)
- All tools follow a unified pattern with explicit dependencies

For detailed development information, see the [Getting Started Guide](./docs/guides/getting-started.md).

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Support

- GitHub Issues: https://github.com/Azure/containerization-assist/issues
- Documentation: https://github.com/Azure/containerization-assist/tree/main/docs

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft trademarks or logos is subject to and must [follow Microsoft’s Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks). Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship. Any use of third-party trademarks or logos are subject to those third-party’s policies.
