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
2. ✅ Create tool in appropriate domain package under `pkg/mcp/domain/`
3. ✅ Implement `ExecuteWithContext()` method with proper signatures
4. ✅ Use `types.BaseToolArgs` and `types.BaseToolResponse`
5. ✅ Return actual errors (never use `success=false` patterns)
6. ✅ Integrate session management using domain/session patterns
7. ✅ Register tool in `pkg/mcp/application/core/server_impl.go` using unified interface system
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

Our current tools follow the domain-based structure:
- **Analyze domain**: `analyze_repository_atomic`, `generate_dockerfile`, `validate_dockerfile_atomic`
- **Deploy domain**: `generate_manifests_atomic`, `deploy_kubernetes_atomic`, `check_health_atomic`
- **Scan domain**: `scan_image_security_atomic`, `scan_secrets_atomic`
- **Session domain**: `list_sessions`, `delete_session`, `manage_session_labels`

## Tool Standards Reference

**⚠️ Critical**: All new tools MUST follow the patterns defined in [MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md).

### Required Method Signature

```go
func (t *YourTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
```

### Required Argument Structure

```go
type YourToolArgs struct {
    types.BaseToolArgs                    // REQUIRED: Embedded base args
    ToolSpecificField string `json:"tool_specific_field" jsonschema:"required"`
    OptionalField     string `json:"optional_field,omitempty"`
}
```

### Required Result Structure

```go
type YourToolResult struct {
    types.BaseToolResponse              // REQUIRED: Embedded base response
    core.BaseAIContextResult           // REQUIRED: For AI context methods
    Success         bool   `json:"success"`          // REQUIRED: Success indicator
    ToolSpecificData string `json:"tool_specific_data"`
}
```

## Step-by-Step Implementation

### 1. Create Tool Structure

Create your tool in the appropriate domain package:

```go
// pkg/mcp/domain/yourdomain/your_tool_atomic.go
package yourdomain

import (
    "context"
    "fmt"
    "time"

    "github.com/Azure/container-kit/pkg/mcp/core"
    "github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
    "github.com/Azure/container-kit/pkg/mcp/internal/types"
    "github.com/localrivet/gomcp/server"
    "github.com/rs/zerolog"
)

// AtomicYourToolArgs defines arguments for your tool
type AtomicYourToolArgs struct {
    types.BaseToolArgs                           // REQUIRED: Embedded base args
    ResourcePath string `json:"resource_path" jsonschema:"required,description=Path to the resource"`
    Options      string `json:"options,omitempty" description:"Optional tool-specific options"`
}

// AtomicYourToolResult defines the response from your tool
type AtomicYourToolResult struct {
    types.BaseToolResponse                       // REQUIRED: Embedded base response
    core.BaseAIContextResult                     // REQUIRED: For AI context methods
    Success      bool   `json:"success"`                 // REQUIRED: Success indicator
    SessionID    string `json:"session_id"`             // Session context
    WorkspaceDir string `json:"workspace_dir"`          // Workspace directory
    ResultData   string `json:"result_data"`            // Tool-specific results
    Duration     time.Duration `json:"duration"`        // Execution duration
}

// AtomicYourTool implements your tool with standardized patterns
type AtomicYourTool struct {
    sessionManager core.ToolSessionManager               // Session management
    logger         zerolog.Logger                       // Structured logging
    sessionMixin   *utils.StandardizedSessionValidationMixin // REQUIRED: Session validation
}
```

### 2. Implement Constructor

```go
// NewAtomicYourTool creates a new tool instance
func NewAtomicYourTool(sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicYourTool {
    toolLogger := logger.With().Str("tool", "atomic_your_tool").Logger()

    // REQUIRED: Initialize standardized session management
    sessionMixin := utils.NewStandardizedSessionValidationMixin(sessionManager, toolLogger, "atomic_your_tool")

    return &AtomicYourTool{
        sessionManager: sessionManager,
        logger:         toolLogger,
        sessionMixin:   sessionMixin,
    }
}
```

### 3. Implement ExecuteWithContext Method

```go
// ExecuteWithContext executes the tool with standardized patterns
func (t *AtomicYourTool) ExecuteWithContext(ctx *server.Context, args *AtomicYourToolArgs) (*AtomicYourToolResult, error) {
    startTime := time.Now()

    t.logger.Info().
        Str("resource_path", args.ResourcePath).
        Str("session_id", args.SessionID).
        Msg("Starting tool execution")

    // Step 1: Handle session management using standardized pattern
    sessionResult, err := t.sessionMixin.GetOrCreateSessionForTool(args.SessionID)
    if err != nil {
        return nil, fmt.Errorf("failed to get or create session: %w", err)
    }

    session := sessionResult.Session
    if sessionResult.IsNew {
        t.logger.Info().
            Str("session_id", session.SessionID).
            Bool("is_resumed", sessionResult.IsResumed).
            Msg("Created new session for tool execution")
    }

    // Step 2: Create result object early for consistent response
    result := &AtomicYourToolResult{
        BaseToolResponse:    types.NewBaseResponse("atomic_your_tool", session.SessionID, args.DryRun),
        BaseAIContextResult: core.NewBaseAIContextResult("your_operation", false, 0), // Duration updated later
        SessionID:           session.SessionID,
        WorkspaceDir:        session.WorkspaceDir,
        Success:             false, // Will be set to true on success
    }

    // Step 3: Implement your tool logic here
    toolResult, err := t.performYourOperation(args, session)
    if err != nil {
        t.logger.Error().Err(err).
            Str("session_id", session.SessionID).
            Dur("duration", time.Since(startTime)).
            Msg("Tool execution failed")
        // IMPORTANT: Return actual error, not success=false
        return result, err
    }

    // Step 4: Update result with success data
    result.Success = true
    result.ResultData = toolResult
    result.Duration = time.Since(startTime)

    // Step 5: Update session metadata with execution result
    if err := t.sessionMixin.UpdateToolExecutionMetadata(session, result); err != nil {
        t.logger.Warn().Err(err).Msg("Failed to update session metadata")
    }

    t.logger.Info().
        Str("session_id", session.SessionID).
        Dur("duration", result.Duration).
        Msg("Tool execution completed successfully")

    return result, nil
}
```

### 4. Implement Standard Interface Methods

```go
// GetMetadata returns comprehensive tool metadata
func (t *AtomicYourTool) GetMetadata() core.ToolMetadata {
    return core.ToolMetadata{
        Name:         "atomic_your_tool",
        Description:  "Your tool description mentioning session context management",
        Version:      "1.0.0",
        Category:     "your_category",
        Dependencies: []string{"dependency1", "dependency2"},
        Capabilities: []string{"supports_dry_run", "supports_streaming"},
        Parameters: map[string]string{
            "resource_path": "required - Path to the resource",
            "options":       "optional - Tool-specific options",
        },
        Examples: []core.ToolExample{
            {
                Name:        "basic_usage",
                Description: "Basic usage example",
                Input: map[string]interface{}{
                    "session_id":    "session-123",
                    "resource_path": "/path/to/resource",
                },
                Output: map[string]interface{}{
                    "success":     true,
                    "result_data": "operation completed",
                },
            },
        },
    }
}

// Validate validates the tool arguments using standardized patterns
func (t *AtomicYourTool) Validate(ctx context.Context, args interface{}) error {
    toolArgs, ok := args.(AtomicYourToolArgs)
    if !ok {
        return fmt.Errorf("invalid argument type for atomic_your_tool: expected AtomicYourToolArgs, got %T", args)
    }

    // Validate required fields
    if toolArgs.ResourcePath == "" {
        return fmt.Errorf("validation error for field resource_path: resource path is required")
    }

    // REQUIRED: Use standardized session validation
    if err := t.sessionMixin.ValidateSessionInArgs(toolArgs.SessionID, true); err != nil {
        return err
    }

    return nil
}

// Execute implements unified Tool interface (delegates to ExecuteWithContext)
func (t *AtomicYourTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    toolArgs, ok := args.(AtomicYourToolArgs)
    if !ok {
        return nil, fmt.Errorf("invalid argument type for atomic_your_tool: expected AtomicYourToolArgs, got %T", args)
    }
    // Execute with nil server context (no progress tracking)
    return t.ExecuteWithContext(nil, &toolArgs)
}
```

### 5. Implement Tool-Specific Logic

```go
// performYourOperation implements your core tool logic
func (t *AtomicYourTool) performYourOperation(args *AtomicYourToolArgs, session *core.SessionState) (string, error) {
    // Implement your specific tool logic here
    // Use session.WorkspaceDir for file operations
    // Respect args.DryRun flag for testing

    if args.DryRun {
        t.logger.Info().Msg("Dry run mode - skipping actual operations")
        return "dry run completed", nil
    }

    // Your actual implementation here
    result := "operation completed successfully"

    return result, nil
}
```

## Session Management

All tools MUST use standardized session management:

### Required Session Integration

```go
// REQUIRED: Include in your tool struct
sessionMixin *utils.StandardizedSessionValidationMixin

// REQUIRED: Initialize in constructor
sessionMixin := utils.NewStandardizedSessionValidationMixin(sessionManager, toolLogger, "your_tool_name")

// REQUIRED: Use in ExecuteWithContext
sessionResult, err := t.sessionMixin.GetOrCreateSessionForTool(args.SessionID)
if err != nil {
    return nil, fmt.Errorf("failed to get or create session: %w", err)
}

// REQUIRED: Update session metadata
if err := t.sessionMixin.UpdateToolExecutionMetadata(session, result); err != nil {
    t.logger.Warn().Err(err).Msg("Failed to update session metadata")
}

// REQUIRED: Validate session in Validate method
if err := t.sessionMixin.ValidateSessionInArgs(toolArgs.SessionID, true); err != nil {
    return err
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

Register your tool in `pkg/mcp/application/core/server_impl.go`:

```go
// In registerEssentialContainerizationTools function
func (s *SimplifiedGoMCPManager) registerEssentialContainerizationTools() error {
    // ... existing tool registrations ...

    // Register your new tool using ExecuteWithContext pattern
    yourTool := yourdomain.NewAtomicYourTool(s.sessionManager, s.logger)
    s.server.Tool("atomic_your_tool", "Your tool description mentioning session context management",
        func(ctx *server.Context, args *yourdomain.AtomicYourToolArgs) (*yourdomain.AtomicYourToolResult, error) {
            return yourTool.ExecuteWithContext(ctx, args)
        })

    s.logger.Debug().Msg("Registered atomic_your_tool")
    return nil
}
```

## Best Practices

### 1. Follow Standards Document
- **Always** reference [MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md) for canonical patterns
- Use `ExecuteWithContext` method signature
- Include `types.BaseToolArgs` and `types.BaseToolResponse`
- Return actual errors, never use `success=false` patterns

### 2. Session Management
- Always use `StandardizedSessionValidationMixin`
- Update session metadata after execution
- Validate session IDs in the `Validate` method
- Use session workspace directory for file operations

### 3. Error Handling
- Return actual Go errors from `ExecuteWithContext`
- Provide meaningful error messages with context
- Log errors with structured information
- Never use result-based error patterns

### 4. Testing
- Include comprehensive unit tests
- Test both success and error scenarios
- Mock dependencies properly
- Add integration tests through MCP protocol

### 5. Performance
- Keep execution time under 300μs for simple operations
- Use progress reporting for long-running operations
- Log execution duration
- Monitor resource usage

## Troubleshooting

### Common Issues

1. **Tool Not Registered**
   - Ensure you added registration in `core.go`
   - Check for compilation errors
   - Verify the `ExecuteWithContext` signature matches exactly

2. **Session Management Errors**
   - Ensure `StandardizedSessionValidationMixin` is properly initialized
   - Check session validation in the `Validate` method
   - Verify session ID handling in arguments

3. **Schema Validation Failures**
   - Use proper JSON tags with `omitempty` for optional fields
   - Include `jsonschema` tags for validation rules
   - Test with `types.BaseToolArgs` embedding

4. **Type Assertion Errors**
   - Ensure argument types match exactly
   - Use pointer types for argument structs
   - Check the registration signature in `core.go`

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
   # Tool should appear in logs: "Registered atomic_your_tool"
   ```

2. **Test Schema Generation**
   ```go
   // Temporarily add to your test
   schema := core.GenerateSchema(&AtomicYourToolArgs{})
   t.Logf("Schema: %s", schema)
   ```

3. **Validate MCP Protocol**
   ```bash
   # Use MCP test client to verify protocol compliance
   echo '{"method":"tools/list"}' | ./container-kit-mcp
   ```

## Next Steps

1. Review [MCP_TOOL_STANDARDS.md](./MCP_TOOL_STANDARDS.md) for complete reference
2. Study existing tools in `pkg/mcp/internal/` for examples
3. Check integration tests for MCP protocol patterns
4. Run the complete test suite to ensure compliance

Remember: Our standardized patterns handle complexity and ensure consistency. Focus on implementing clean tool logic, and the framework provides session management, error handling, and MCP protocol integration automatically.
