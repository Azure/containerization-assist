# Container Kit MCP Server

[![npm version](https://badge.fury.io/js/@container-assist%2Fmcp-server.svg)](https://www.npmjs.com/package/@container-assist/mcp-server)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![MCP Version](https://img.shields.io/badge/MCP-1.0.0-blue)](https://modelcontextprotocol.io)

AI-powered containerization workflow tools for the Model Context Protocol (MCP). Container Kit provides a comprehensive set of tools to analyze, containerize, and deploy applications using AI assistance.

## 🚀 Quick Start

### Installation

```bash
# Install globally
npm install -g @container-assist/mcp-server

# Or use with npx (no installation required)
npx @container-assist/mcp-server --help
```

### Basic Usage

```bash
# Check version
container-assist-mcp --version

# Or use the short alias
ckmcp --version

# Start the MCP server (for stdio transport)
container-assist-mcp

# With custom configuration
container-assist-mcp --workspace-dir ./my-project --log-level debug
```

## 📦 What's Included

Container Kit MCP Server provides **15 specialized tools** for containerization workflows:

### Workflow Step Tools (10)
- `analyze_repository` - Analyze repository structure and detect technologies
- `generate_dockerfile` - Generate optimized Dockerfile based on analysis
- `build_image` - Build Docker image from Dockerfile
- `scan_image` - Scan image for security vulnerabilities
- `tag_image` - Tag image with version information
- `push_image` - Push image to container registry
- `generate_k8s_manifests` - Generate Kubernetes deployment manifests
- `prepare_cluster` - Prepare Kubernetes cluster for deployment
- `deploy_application` - Deploy application to Kubernetes
- `verify_deployment` - Verify deployment health and readiness

### Orchestration Tools (2)
- `start_workflow` - Start complete containerization workflow
- `workflow_status` - Check workflow progress and status

### Utility Tools (3)
- `list_tools` - List all available tools and their descriptions
- `ping` - Test MCP server connectivity
- `server_status` - Get server status and health information

## 🔧 Configuration

### Environment Variables

```bash
# MCP Server Configuration
export MCP_WORKSPACE_DIR="/path/to/workspace"  # Working directory for operations
export MCP_LOG_LEVEL="info"                    # debug, info, warn, error
export MCP_SESSION_TTL="24h"                   # Session timeout duration
export MCP_MAX_SESSIONS="100"                  # Maximum concurrent sessions
export MCP_STORE_PATH="./mcp-store.db"         # Session store database path
export MCP_WORKFLOW_MODE="interactive"         # automated or interactive

# Optional: Go environment (if needed)
export GOSUMDB="sum.golang.org"
export GOPROXY=""
```

### Configuration File

Create a `.env` file in your project root:

```env
MCP_WORKSPACE_DIR=./workspace
MCP_LOG_LEVEL=info
MCP_SESSION_TTL=24h
MCP_MAX_SESSIONS=100
MCP_STORE_PATH=./mcp-store.db
MCP_WORKFLOW_MODE=interactive
```

Then use with:
```bash
container-assist-mcp --config .env
```

## 🔌 MCP Client Integration

### VS Code Integration

Container Kit works with any MCP-compatible client. For VS Code:

1. Install an MCP client extension
2. Configure the MCP server in your settings:

```json
{
  "mcp.servers": {
    "container-assist": {
      "command": "npx",
      "args": ["@container-assist/mcp-server"],
      "env": {
        "MCP_WORKSPACE_DIR": "./workspace",
        "MCP_LOG_LEVEL": "info"
      }
    }
  }
}
```

### Programmatic Usage

```javascript
const { spawn } = require('child_process');

// Start the MCP server
const server = spawn('npx', ['@container-assist/mcp-server'], {
  stdio: ['pipe', 'pipe', 'inherit'],
  env: {
    ...process.env,
    MCP_WORKSPACE_DIR: './workspace',
    MCP_LOG_LEVEL: 'info'
  }
});

// MCP communication via stdio
server.stdin.write(JSON.stringify({
  jsonrpc: '2.0',
  method: 'tools/call',
  params: {
    name: 'analyze_repository',
    arguments: {
      path: './my-app'
    }
  },
  id: 1
}));

server.stdout.on('data', (data) => {
  const response = JSON.parse(data);
  console.log('Response:', response);
});
```

## 🎯 Use Cases

### Complete Containerization Workflow
```bash
# Start an interactive containerization session
container-assist-mcp --workflow-mode interactive
```

### CI/CD Integration
```yaml
# GitHub Actions example
- name: Containerize Application
  run: |
    npx @container-assist/mcp-server \
      --workspace-dir ${{ github.workspace }} \
      --workflow-mode automated
```

### Custom Tool Integration
Use individual tools programmatically through the MCP protocol for custom workflows.

## 🏗️ Platform Support

| Platform | Architecture | Support Status |
|----------|-------------|----------------|
| macOS    | Apple Silicon (M1/M2) | ✅ Full Support |
| macOS    | Intel (x64) | ✅ Full Support |
| Linux    | x64 | ✅ Full Support |
| Linux    | ARM64 | ✅ Full Support |
| Windows  | x64 | ✅ Full Support |
| Windows  | ARM64 | ⚠️ Experimental |

## 🐛 Troubleshooting

### Binary Not Found
If you see "Binary not found" error:
```bash
# Rebuild for your platform
cd $(npm root -g)/@container-assist/mcp-server
npm run build:current
```

### Permission Denied
On Unix-like systems:
```bash
chmod +x $(npm root -g)/@container-assist/mcp-server/bin/mcp-server
```

### Debug Mode
Enable debug output:
```bash
MCP_DEBUG=true container-assist-mcp --debug
```

## 📊 Performance

- **Startup Time**: < 100ms
- **Memory Usage**: ~50MB idle, ~200MB during operations
- **Binary Size**: ~20MB per platform
- **Protocol**: STDIO-based MCP transport

## 🔒 Security

- All binaries are built from audited source code
- No telemetry or data collection
- Supports air-gapped environments
- Compatible with corporate proxies

## 📚 Documentation

- [Full Documentation](https://github.com/Azure/container-kit)
- [MCP Protocol Specification](https://modelcontextprotocol.io)
- [API Reference](https://github.com/Azure/container-kit/blob/main/docs/API.md)

## 🤝 Contributing

Contributions are welcome! Please see our [Contributing Guide](https://github.com/Azure/container-kit/blob/main/CONTRIBUTING.md).

## 📄 License

MIT License - see [LICENSE](https://github.com/Azure/container-kit/blob/main/LICENSE) for details.

## 🙏 Acknowledgments

Built with:
- [Model Context Protocol](https://modelcontextprotocol.io)
- [mcp-go](https://github.com/mark3labs/mcp-go) - Go implementation of MCP
- Go 1.24.4
- Docker & Kubernetes APIs

---

**Need help?** Open an issue on [GitHub](https://github.com/Azure/container-kit/issues)