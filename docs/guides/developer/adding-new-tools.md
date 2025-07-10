# Developer Guide: Adding New MCP Tools

This guide provides a comprehensive walkthrough for adding new tools to the Container Kit MCP server using our standardized architecture and patterns.

## Table of Contents
1. [Quick Start](#quick-start)
2. [Architecture Overview](#architecture-overview)
3. [Tool Standards Reference](#tool-standards-reference)
4. [Step-by-Step Implementation](#step-by-step-implementation)
5. [Session Management](#session-management)
6. [Testing Your Tool](#testing-your-tool)
7. [Registration Process](#registration-process)
8. [Best Practices](#best-practices)
9. [Troubleshooting](#troubleshooting)

## Quick Start

For experienced developers, here's the minimal checklist:

1. ✅ Read [MCP Tool Standards](./MCP_TOOL_STANDARDS.md) for canonical patterns
2. ✅ Create tool in `pkg/mcp/application/commands/tool_registration.go`
3. ✅ Implement lazy tool creation pattern with service injection
4. ✅ Use FileAccessService for any file operations (secure by default)
5. ✅ Use session-based workspace isolation
6. ✅ Follow consolidated command pattern
7. ✅ Register tool in init() function using MCP server Tool() method
8. ✅ Add unit tests and run integration tests

> **📖 Important**: All patterns in this guide follow the canonical standards defined in [MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md). When in doubt, reference that document.

## Architecture Overview

The MCP server uses a **unified interface system** with standardized patterns:

### Key Components

- **Standard Interface**: All tools implement `ExecuteWithContext(*server.Context, *Args) (*Result, error)`
- **Session Management**: `StandardizedSessionValidationMixin` provides consistent session handling
- **Error Handling**: Actual Go errors instead of result-based error handling
- **Auto-Registration**: Zero-configuration tool discovery in `core.go`
- **Progress Tracking**: Unified progress reporting through `observability.NewUnifiedProgressReporter`

### Current Tool Registry

Our current **12 production tools** follow the consolidated pattern:
- **Containerization**: `analyze_repository`, `generate_dockerfile`, `build_image`, `push_image`, `generate_manifests`, `scan_image`
- **File Access**: `read_file`, `list_directory`, `file_exists` (with FileAccessService)
- **Session Management**: `list_sessions`
- **Diagnostics**: `ping`, `server_status`

## Tool Standards Reference

**⚠️ Critical**: All new tools MUST follow the patterns defined in [MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md).

### Required Method Signature

```go
// Lazy tool creation pattern for MCP registration
func() LazyTool {
    return func(serviceContainer services.ServiceContainer) (string, string, interface{}) {
        return toolName, description, toolFunc
    }
}
```

### Tool Implementation Pattern

Tools are implemented as lazy-initialized functions using the consolidated pattern:

```go
// In pkg/mcp/application/commands/tool_registration.go
type YourToolArgs struct {
    SessionID       string `json:"session_id,omitempty"`
    ToolSpecificField string `json:"tool_specific_field" jsonschema:"required"`
    OptionalField     string `json:"optional_field,omitempty"`
}

type YourToolResult struct {
    Success         bool   `json:"success"`
    SessionID       string `json:"session_id"`
    ToolSpecificData string `json:"tool_specific_data"`
    // FileAccessService results if applicable
    Content         string `json:"content,omitempty"`
    Files          []string `json:"files,omitempty"`
}
```

## Step-by-Step Implementation

### 1. Create Tool Structure

Add your tool to the consolidated tool registration file:

```go
// pkg/mcp/application/commands/tool_registration.go
package commands

import (
    "context"
    "fmt"
    "log/slog"

    "github.com/Azure/container-kit/pkg/mcp/application/services"
    "github.com/localrivet/gomcp/server"
)

// YourToolArgs defines arguments for your tool
type YourToolArgs struct {
    SessionID    string `json:"session_id,omitempty"`
    ResourcePath string `json:"resource_path" jsonschema:"required,description=Path to the resource"`
    Options      string `json:"options,omitempty"`
}

// YourToolResult defines the response from your tool
type YourToolResult struct {
    Success      bool   `json:"success"`
    SessionID    string `json:"session_id"`
    WorkspaceDir string `json:"workspace_dir,omitempty"`
    ResultData   string `json:"result_data"`
    // Include FileAccessService results if needed
    Files        []string `json:"files,omitempty"`
    Content      string   `json:"content,omitempty"`
}
```

### 2. Implement Lazy Tool Creation

```go
// LazyYourTool creates your tool with dependency injection
func LazyYourTool() LazyTool {
    return func(serviceContainer services.ServiceContainer) (string, string, interface{}) {
        // Access required services
        fileAccess := serviceContainer.FileAccessService()
        sessionState := serviceContainer.SessionState()
        
        // Tool function implementation
        toolFunc := func(ctx *server.Context, args *YourToolArgs) (*YourToolResult, error) {
            return executeYourTool(ctx, args, fileAccess, sessionState)
        }
        
        return "your_tool", "Description of your tool with session support", toolFunc
    }
}
```

### 3. Implement Tool Function

```go
// executeYourTool implements the actual tool logic
func executeYourTool(ctx *server.Context, args *YourToolArgs, fileAccess services.FileAccessService, sessionState services.SessionState) (*YourToolResult, error) {
    // Session management
    sessionID := args.SessionID
    if sessionID == "" {
        sessionID = generateSessionID() // Generate if not provided
    }
    
    // Get workspace directory
    workspaceDir, err := sessionState.GetWorkspaceDir(context.Background(), sessionID)
    if err != nil {
        return nil, fmt.Errorf("failed to get workspace directory: %w", err)
    }
    
    result := &YourToolResult{
        SessionID:    sessionID,
        WorkspaceDir: workspaceDir,
        Success:      false,
    }
    
    // Use FileAccessService for any file operations
    if args.ResourcePath != "" {
        // Example: Check if file exists
        exists, err := fileAccess.FileExists(context.Background(), sessionID, args.ResourcePath)
        if err != nil {
            return result, fmt.Errorf("failed to check file existence: %w", err)
        }
        
        if exists {
            // Example: Read file content
            content, err := fileAccess.ReadFile(context.Background(), sessionID, args.ResourcePath)
            if err != nil {
                return result, fmt.Errorf("failed to read file: %w", err)
            }
            result.Content = content
        }
    }
    
    // Your tool-specific logic here
    result.ResultData = "Tool operation completed"
    result.Success = true
    
    return result, nil
}
```

### 4. Add Tool Registration

```go
// Add to init() function in tool_registration.go
func init() {
    // Register your tool
    lazyTools = append(lazyTools, LazyYourTool())
}
```

### 5. FileAccessService Integration

Always use FileAccessService for file operations:

```go
func performFileOperations(fileAccess services.FileAccessService, sessionID string) error {
    // Read file securely
    content, err := fileAccess.ReadFile(context.Background(), sessionID, "path/to/file")
    if err != nil {
        return fmt.Errorf("failed to read file: %w", err)
    }
    
    // List directory contents
    files, err := fileAccess.ListDirectory(context.Background(), sessionID, "path/to/dir")
    if err != nil {
        return fmt.Errorf("failed to list directory: %w", err)
    }
    
    // Check file existence
    exists, err := fileAccess.FileExists(context.Background(), sessionID, "path/to/check")
    if err != nil {
        return fmt.Errorf("failed to check file existence: %w", err)
    }
    
    return nil
}
```

## Session Management

All tools should use session management through the service container:

### Session Management Pattern

```go
// Access session services through service container
sessionState := serviceContainer.SessionState()
sessionStore := serviceContainer.SessionStore()

// Get or create workspace
workspaceDir, err := sessionState.GetWorkspaceDir(context.Background(), sessionID)
if err != nil {
    return nil, fmt.Errorf("failed to get workspace: %w", err)
}

// All FileAccessService operations are automatically session-scoped
// No manual session validation needed - FileAccessService handles it
content, err := fileAccess.ReadFile(context.Background(), sessionID, relativePath)
if err != nil {
    return nil, fmt.Errorf("failed to read file: %w", err)
}
```

## Testing Your Tool

### 1. Unit Tests

```go
// pkg/mcp/internal/yourdomain/your_tool_test.go
package yourdomain

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/rs/zerolog"
)

func TestAtomicYourTool_ExecuteWithContext(t *testing.T) {
    // Create test logger
    logger := zerolog.New(zerolog.NewTestWriter(t))

    // Create mock session manager
    sessionManager := &MockSessionManager{} // Implement mock

    // Create tool instance
    tool := NewAtomicYourTool(sessionManager, logger)

    tests := []struct {
        name    string
        args    *AtomicYourToolArgs
        wantErr bool
    }{
        {
            name: "successful execution",
            args: &AtomicYourToolArgs{
                BaseToolArgs: types.BaseToolArgs{
                    SessionID: "test-session",
                },
                ResourcePath: "/valid/path",
            },
            wantErr: false,
        },
        // Add more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := tool.ExecuteWithContext(nil, tt.args)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.True(t, result.Success)
        })
    }
}
```

### 2. Integration Tests

Add your tool to the integration test suite:

```go
func TestYourToolIntegration(t *testing.T) {
    // Test your tool through the MCP protocol
    client := setupMCPTestClient(t)

    result, err := client.CallTool("atomic_your_tool", map[string]interface{}{
        "session_id":    "test-session",
        "resource_path": "/test/path",
    })

    require.NoError(t, err)
    assert.Equal(t, true, result["success"])
}
```

## Registration Process

Tools are automatically registered through the lazy tool pattern:

```go
// In pkg/mcp/application/commands/tool_registration.go

// Your lazy tool function is automatically called during server initialization
// No manual registration needed in core server code

// The service container handles dependency injection
// MCP server.Tool() method handles schema generation and registration

// Example of automatic registration in init():
func init() {
    lazyTools = append(lazyTools, 
        LazyReadFileTool(),
        LazyListDirectoryTool(),
        LazyFileExistsTool(),
        LazyAnalyzeTool(),
        LazyYourTool(), // Your tool gets registered here
    )
}
```

## Best Practices

### 1. Follow Consolidated Pattern
- Use lazy tool creation pattern from tool_registration.go
- Inject services through service container
- Use FileAccessService for all file operations
- Follow session-based workspace isolation

### 2. Security Best Practices
- **Always** use FileAccessService instead of direct file operations
- All file paths are validated for path traversal attacks
- File operations are automatically scoped to session workspace
- File type and size validation is handled by FileAccessService

### 3. Service Integration
- Access required services through service container dependency injection
- Use sessionState for workspace management
- Use fileAccess for secure file operations
- Use other services as needed (docker, k8s, etc.)

### 4. Testing
- Test tools through the lazy tool pattern
- Mock FileAccessService for unit tests
- Test session workspace isolation
- Verify security validation works correctly

### 5. FileAccessService Benefits
- **Security**: Automatic path traversal protection
- **Isolation**: Session-based workspace boundaries
- **Validation**: File type and size checking
- **Consistency**: Standardized file operation patterns
- **Error Handling**: Rich error messages with context

## Troubleshooting

### Common Issues

1. **Tool Not Registered**
   - Ensure your LazyTool function is added to lazyTools slice in init()
   - Check for compilation errors in tool_registration.go
   - Verify the lazy tool pattern is followed correctly

2. **FileAccessService Errors**
   - Always use sessionID parameter for file operations
   - Check that file paths are relative, not absolute
   - Ensure session workspace exists before file operations
   - Verify file access permissions within session boundaries

3. **Service Container Issues**
   - Ensure services are properly accessed through service container
   - Check that required services are available
   - Verify dependency injection is working correctly

4. **Session Management**
   - Use sessionState service for workspace management
   - Allow empty sessionID (will be generated automatically)
   - FileAccessService handles session validation automatically

### Testing Commands

```bash
# Build with new tool
make mcp

# Run MCP tests
make test-mcp

# Run integration tests
go test -tags mcp -race ./pkg/mcp/internal/test/integration/... -v

# Test tool discovery
go test -tags mcp -race ./pkg/mcp/internal/test/integration/... -v -run="Schema"
```

### Debugging Tips

1. **Check Tool Registration**
   ```bash
   # Verify your tool is discovered
   ./container-kit-mcp &
   # Tool should appear in tools/list response
   ```

2. **Test FileAccessService**
   ```go
   // Test file operations in your tool
   fileAccess := serviceContainer.FileAccessService()
   content, err := fileAccess.ReadFile(ctx, sessionID, "test-file.txt")
   if err != nil {
       t.Errorf("FileAccessService error: %v", err)
   }
   ```

3. **Validate Session Isolation**
   ```bash
   # Test that different sessions have isolated workspaces
   # Each session should have its own workspace directory
   ```

## Next Steps

1. Review [MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md) for complete reference
2. Study existing tools in `pkg/mcp/internal/` for examples
3. Check integration tests for MCP protocol patterns
4. Run the complete test suite to ensure compliance

Remember: The consolidated pattern with FileAccessService provides security by default. Focus on implementing your tool logic, and the framework provides session isolation, file security, error handling, and MCP protocol integration automatically. Always use FileAccessService for file operations to maintain security and session boundaries.
