# Development Guide

This guide is for developers working on the Container Kit npm package.

## 🛠️ Development Setup

### Prerequisites
- Node.js >= 16.0.0
- Go >= 1.19 (for building binaries)
- Access to the Container Kit repository

### Initial Setup
```bash
# Install dependencies
npm install zod

# Build the Go binary for your platform
npm run build:current

# Or build for all platforms
npm run build
```

## 🧪 Testing with the Development Server

The `test-server.js` script provides a comprehensive testing environment for the npm package.

### Basic Usage

```bash
# Run with mock server (no dependencies needed)
node test-server.js

# Test with custom tool names
node test-server.js --custom-names

# Test specific tools only
node test-server.js --tools=ping,list_tools,server_status

# Test with real MCP SDK (if installed)
npm install @modelcontextprotocol/sdk
node test-server.js --real
```

### What the Test Server Does

1. **Registers all tools** - Verifies tool registration works correctly
2. **Lists registered tools** - Shows all tools with their descriptions
3. **Tests utility tools** - Executes ping, list_tools, and server_status
4. **Validates structure** - Ensures each tool has required properties

### How Tools Work

All Container Kit tools are executed through the Go binary with full server infrastructure initialized. This ensures:
- **Session persistence** - BoltDB stores sessions across tool invocations
- **Consistent state** - Workflow state is maintained between tool calls
- **Full dependencies** - All tools have access to required services

#### Session Management

- Sessions are automatically created when needed
- Sessions persist in BoltDB at `/tmp/mcp-store/sessions.db` by default
- If no `session_id` is provided, one is auto-generated for workflow tools
- **The session ID is always included in the tool output** so you can reuse it
- Sessions can be reused across multiple tool invocations

#### Performance Note

Currently, each tool invocation initializes the full server infrastructure. This ensures reliability but has some overhead. Sessions and state persist via BoltDB between invocations, so workflow continuity is maintained.

**Potential Future Enhancement**: A persistent server daemon could be implemented to:
- Start on first tool invocation and keep running
- Reduce initialization overhead for subsequent tool calls
- Maintain in-memory caches and connections

However, the current approach is simpler and more reliable, avoiding daemon management complexity.

#### Tool Execution

```bash
# Tools can be executed individually (session ID is auto-generated)
TOOL_PARAMS='{"repo_path":"/path/to/repo"}' containerization-assist-mcp tool analyze_repository
# Output includes: {"data":{"session_id":"session-20250819-193756-533387",...}}

# The session ID from the response can be used for subsequent tools
TOOL_PARAMS='{"session_id":"session-20250819-193756-533387"}' containerization-assist-mcp tool generate_dockerfile

# Or start a full workflow
TOOL_PARAMS='{"repo_path":"/path/to/repo"}' containerization-assist-mcp tool start_workflow
# Output includes: {"data":{"session_id":"wf_xxx",...}}
```

### Available Tools

**Workflow Tools** - Perform containerization steps:
- `analyze_repository` - Analyze code repository
- `generate_dockerfile` - Generate optimized Dockerfile
- `build_image` - Build Docker image
- `scan_image` - Security vulnerability scanning
- `tag_image` - Tag Docker images
- `push_image` - Push to registry
- `generate_k8s_manifests` - Generate K8s manifests
- `prepare_cluster` - Prepare K8s cluster
- `deploy_application` - Deploy to K8s
- `verify_deployment` - Verify deployment health

**Orchestration Tools** - Manage workflows:
- `start_workflow` - Start full containerization workflow
- `workflow_status` - Check workflow progress

**Utility Tools** - System utilities:
- `ping` - Test connectivity
- `list_tools` - List available tools
- `server_status` - Get server status
5. **Tests helpers** - Verifies registerTool() and registerAllTools() work

### Test Output Example

```
🚀 Container Kit MCP Tools - Development Test Server
============================================================

📦 Creating MCP Server: containeization-assist-test v1.0.0

📚 Registering Container Kit Tools
============================================================
  ✅ Registering tool: analyze_repository
  ✅ Registering tool: generate_dockerfile
  ...

📋 Registered tools (13):
   • analyze_repository: Analyze repository to detect...
   • generate_dockerfile: Generate an optimized Dockerfile...
   ...

🧪 Running Tool Tests
============================================================
🔧 Calling tool: ping
   Parameters: { message: 'test from dev server' }
   ✅ Success
   Result: {"response":"pong: test from dev server",...}
```

## 🔄 Development Workflow

### 1. Making Changes to Tools

Edit tool definitions in `lib/tools/`:
```javascript
// lib/tools/my-tool.js
const { createTool, z } = require('./_tool-factory');

module.exports = createTool({
  name: 'my_tool',
  title: 'My Tool',
  description: 'Description of what the tool does',
  inputSchema: {
    param1: z.string().describe('Parameter description'),
    param2: z.number().optional().describe('Optional parameter')
  }
});
```

### 2. Testing Your Changes

```bash
# Rebuild if Go code changed
npm run build:current

# Test registration and execution
node test-server.js

# Test specific tool
node test-server.js --tools=my_tool
```

### 3. Adding Tool Logic in Go

For workflow tools, implement the logic in the Go binary:
1. Add tool handler in `cmd/mcp-server/tool_mode.go`
2. Update tool registry in `pkg/mcp/service/tools/registry.go`
3. Rebuild: `npm run build:current`

### 4. Testing with Real MCP SDK

```bash
# Install MCP SDK
npm install @modelcontextprotocol/sdk

# Test with real server
node test-server.js --real
```

## 📁 Project Structure

```
npm/
├── lib/
│   ├── index.js           # Main exports and helpers
│   ├── executor.js        # Subprocess bridge to Go binary
│   └── tools/            # Individual tool definitions
│       ├── _tool-factory.js
│       ├── analyze-repository.js
│       └── ...
├── bin/                  # Platform-specific binaries
│   ├── darwin-x64/
│   ├── darwin-arm64/
│   ├── linux-x64/
│   ├── linux-arm64/
│   └── win32-x64/
├── scripts/
│   ├── build-all.sh      # Build for all platforms
│   └── build-current.sh  # Build for current platform
├── test-server.js        # Development test server
└── package.json
```

## 🐛 Debugging

### Common Issues

**Binary not found error:**
```bash
# Rebuild for your platform
npm run build:current
```

**Tool execution fails:**
```bash
# Check if binary has tool mode
./bin/linux-x64/container-kit-mcp tool list_tools
# Should output JSON with tool list
```

**Registration fails:**
```bash
# Test with verbose output
node test-server.js 2>&1 | less
```

### Testing Binary Directly

```bash
# Test tool mode
TOOL_PARAMS='{"message":"test"}' ./bin/linux-x64/container-kit-mcp tool ping

# Test list_tools
TOOL_PARAMS='{}' ./bin/linux-x64/container-kit-mcp tool list_tools
```

## 🚀 Building for Release

### Build All Platforms
```bash
# Build binaries for all supported platforms
npm run build

# Verify all binaries exist
ls -la bin/*/container-kit-mcp*
```

### Test Package Locally
```bash
# Pack the package
npm pack

# Test in another directory
cd /tmp
npm init -y
npm install /path/to/thgamble-containerization-assist-mcp-*.tgz
node -e "const ck = require('@thgamble/containerization-assist-mcp'); console.log(Object.keys(ck.tools))"
```

## 📝 Adding New Tools

1. **Define the tool** in `lib/tools/new-tool.js`
2. **Add Go implementation** if it's a workflow tool
3. **Update TypeScript definitions** in `index.d.ts`
4. **Test with test-server.js**
5. **Update README** if it's a major feature

## 🧹 Cleanup

```bash
# Remove test files
rm -f test-*.js example-*.js

# Clean build artifacts
rm -rf bin/

# Clean node_modules
rm -rf node_modules/
```

## 💡 Tips

- Use `createTool()` factory for consistent tool definitions
- Keep tool names in snake_case for MCP compatibility
- Always test with `test-server.js` after changes
- Run `npm run build:current` for quick local testing
- Run `npm run build` before publishing

## 🔗 Resources

- [Container Kit Repository](https://github.com/Azure/containerization-assist)
- [Model Context Protocol Docs](https://modelcontextprotocol.io)
- [MCP SDK](https://github.com/modelcontextprotocol/typescript-sdk)