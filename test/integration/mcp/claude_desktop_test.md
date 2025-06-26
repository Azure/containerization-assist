# Claude Desktop Integration Testing Guide

This guide provides instructions for testing the Container Kit MCP Server with Claude Desktop.

## Prerequisites

1. **Claude Desktop** installed on your system
2. **Container Kit MCP Server** built and ready
3. **Docker** daemon running
4. **kubectl** configured (optional, for K8s testing)
5. **Kind** installed (optional, for local K8s testing)

## Setup Instructions

### 1. Build the MCP Server

```bash
# From the project root
go build -o container-kit-mcp ./cmd/mcp-server
```

### 2. Configure Claude Desktop

Add the following to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
**Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "container-kit": {
      "command": "/path/to/container-kit/container-kit-mcp",
      "env": {
        "CONTAINER_KIT_WORKSPACE_DIR": "/tmp/container-kit-workspaces",
        "CONTAINER_KIT_LOG_LEVEL": "info"
      }
    }
  }
}
```

Replace `/path/to/container-kit/container-kit-mcp` with the actual path to your built binary.

### 3. Restart Claude Desktop

After updating the configuration, restart Claude Desktop to load the MCP server.

## Test Scenarios

### Scenario 1: Conversation Mode - Complete Workflow

1. Open Claude Desktop
2. Start a new conversation
3. Ask Claude to guide you through containerization:

```
I want to containerize my Go application. Can you help me through the process step by step?
```

**Expected Result**:
- Claude should start a conversation flow using the `chat` tool
- You should see a guided workflow with pre-flight checks
- Each stage should offer clear options and next steps

### Scenario 2: Atomic Tools - Direct Repository Analysis

1. Ask Claude to use a specific tool directly:

```
Use the analyze_repository tool to analyze https://github.com/golang/example
```

**Expected Result**:
- Claude should use the `analyze_repository` atomic tool
- You should see detailed repository analysis results
- Results should include language, framework, dependencies, and containerization suggestions

### Scenario 3: Conversation Mode - Repository Analysis

1. In conversation mode, provide a repository:

```
My repository is at https://github.com/golang/example. Please analyze it and create a containerization plan.
```

**Expected Result**:
- The conversation should progress through analysis stage
- You should see repository analysis results
- Claude should offer next steps for Dockerfile generation

### Scenario 4: Dry Run Operations

1. Ask for a build preview:

```
I want to see what the Docker build would look like before actually building. Can you show me a dry run?
```

**Expected Result**:
- Claude should use dry-run functionality
- You should see estimated build information without actual building
- Preview should include estimated size, layers, and build steps

### Scenario 5: Session Persistence

1. Start a containerization workflow:

```
Help me containerize my Python Flask application at https://github.com/user/flask-app
```

2. In a new conversation (or later), continue:

```
Continue working on my Flask application containerization from the previous session
```

**Expected Result**:
- Sessions should persist across conversations
- Claude should resume from where you left off
- Previous analysis and state should be remembered

### Scenario 6: Error Handling and Recovery

1. Try analyzing a non-existent repository:

```
Analyze this repository: https://github.com/nonexistent/repo-does-not-exist
```

**Expected Result**:
- Tools should handle errors gracefully
- Claude should explain the error clearly
- Suggestions for resolution should be provided
- The conversation should continue despite the error

### Scenario 7: Preference Learning

1. Set preferences during conversation:

```
I always prefer size-optimized Docker images and want to deploy to the 'production' namespace
```

2. In a later session:

```
Containerize another application for me
```

**Expected Result**:
- Preferences should be remembered across sessions
- Size optimization should be applied automatically
- Production namespace should be suggested
- User shouldn't need to re-specify preferences

## Debugging Tips

### 1. Check MCP Server Logs

The MCP server logs to stderr. You can see these in Claude Desktop's developer console:
- Open Developer Tools: `Cmd+Option+I` (macOS) or `Ctrl+Shift+I` (Windows/Linux)
- Look for MCP server output in the console

### 2. Verify Tool Registration

Ask Claude:
```
What tools do you have available for containerization?
```

Claude should list the available tools:
- **Conversation Mode**: `chat` tool for guided workflows
- **Atomic Tools**: `analyze_repository`, `generate_dockerfile`, `build_image`, `push_image`, `generate_manifests`, `deploy_kubernetes_atomic`, `check_health_atomic`

### 3. Test Individual Tools

Test atomic tools directly:
```
Use the analyze_repository tool with these parameters:
{
  "repo_url": "https://github.com/golang/example"
}
```

Test conversation mode:
```
Use the chat tool with this message: "Hello, I want to containerize my application"
```

### 4. Check Session State

In conversation mode, ask about sessions:
```
Can you show me information about my current containerization session?
```

For atomic tools:
```
Use the list_sessions tool to show all active sessions
```

## Common Issues and Solutions

### Issue: "Tool not found"
**Solution**: Ensure the MCP server is properly configured and running. Check the developer console for errors.

### Issue: "Session not found"
**Solution**: Sessions may have expired. Start a new analysis workflow.

### Issue: "Docker daemon not accessible"
**Solution**: Ensure Docker is running and the MCP server has access to the Docker socket.

### Issue: "Kind cluster not found"
**Solution**: Kind is optional. The server should work without it, but K8s deployment testing will be limited.

## Performance Testing

### 1. Large Repository Test
Test with a larger repository to check performance:
```
Analyze and containerize https://github.com/kubernetes/kubernetes
```

### 2. Concurrent Session Test
Create multiple sessions simultaneously:
```
Start three containerization workflows for different repositories at the same time
```

### 3. Error Recovery Loop Test
Test the error recovery with a problematic Dockerfile:
```
Create a Dockerfile that will fail to build and see how many iterations it takes to fix
```

## Reporting Issues

When reporting issues, please include:
1. The exact conversation/prompts used
2. MCP server logs from developer console
3. The session ID if applicable
4. Your claude_desktop_config.json (sanitized)
5. Version of Claude Desktop and Container Kit
