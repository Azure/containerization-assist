# Migration Guide for External Users

This guide helps external users migrate to the new unified interface system introduced in Container Kit v2.0.

## Overview of Changes

The Container Kit MCP system has undergone a significant reorganization to improve maintainability and provide cleaner interfaces. This guide covers the breaking changes and migration steps for external tool developers and API consumers.

## Breaking Changes

### 1. Interface Location Changes

**Before (v1.x)**:
```go
import (
    "github.com/Azure/container-copilot/pkg/mcp"
    "github.com/Azure/container-copilot/pkg/mcp/tools"
)

// Tools implemented various interfaces from different packages
type MyTool struct{}

func (t *MyTool) Run(ctx context.Context, params tools.RunParams) error {
    // Old implementation
}
```

**After (v2.0)**:
```go
import "github.com/Azure/container-copilot/pkg/mcp"

// All tools now implement the unified Tool interface
type MyTool struct{}

func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // New implementation
}

func (t *MyTool) GetMetadata() mcp.ToolMetadata {
    return mcp.ToolMetadata{
        Name: "my_tool",
        // ... metadata
    }
}

func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    // Validation logic
}
```

### 2. Method Signature Changes

| Old Method | New Method | Notes |
|------------|------------|--------|
| `Run(ctx, params RunParams) error` | `Execute(ctx, args interface{}) (interface{}, error)` | Now returns results |
| `GetName() string` | `GetMetadata() ToolMetadata` | Returns full metadata |
| `GetArgs() ToolArgs` | Removed | Args defined separately |
| N/A | `Validate(ctx, args interface{}) error` | New validation method |

### 3. Tool Registration Changes

**Before**:
```go
// Manual registration in init()
func init() {
    tools.Register("my_tool", &MyTool{})
}
```

**After**:
```go
// Automatic registration via code generation
// Add annotation above your tool struct:

// MyTool performs specific functionality
// +tool:name=my_tool
// +tool:category=build
// +tool:description=Does something useful
type MyTool struct {
    // fields
}

// Then run: go generate ./...
```

### 4. Error Handling Changes

**Before**:
```go
return fmt.Errorf("operation failed: %v", err)
```

**After**:
```go
import "github.com/Azure/container-copilot/pkg/mcp/types"

return types.NewRichError(
    "OPERATION_FAILED",
    "Operation failed",
    err,
).WithContext(map[string]interface{}{
    "tool": "my_tool",
    "phase": "execution",
})
```

### 5. Result Type Changes

**Before**:
```go
// Tools returned errors only
func (t *MyTool) Run(ctx context.Context, params RunParams) error {
    // Do work
    return nil
}
```

**After**:
```go
// Tools now return results and errors
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Do work
    result := &MyToolResult{
        Status: "success",
        Data: processedData,
    }
    return result, nil
}
```

## Migration Steps

### Step 1: Update Import Paths

Replace all old import paths with the new unified imports:

```go
// Replace these:
import (
    "github.com/Azure/container-copilot/pkg/mcp/tools"
    "github.com/Azure/container-copilot/pkg/mcp/tools/interfaces"
    "github.com/Azure/container-copilot/pkg/mcp/common"
)

// With this:
import "github.com/Azure/container-copilot/pkg/mcp"
```

### Step 2: Implement the New Interface

Update your tool to implement the unified `Tool` interface:

```go
package mytool

import (
    "context"
    "github.com/Azure/container-copilot/pkg/mcp"
)

// Ensure compliance at compile time
var _ mcp.Tool = (*MyTool)(nil)

type MyTool struct {
    // your fields
}

// Execute replaces the old Run method
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Convert args to your expected type
    myArgs, ok := args.(*MyToolArgs)
    if !ok {
        return nil, fmt.Errorf("invalid arguments type: expected *MyToolArgs, got %T", args)
    }
    
    // Your tool logic here
    result := t.performWork(myArgs)
    
    // Return results instead of just error
    return result, nil
}

// GetMetadata replaces GetName and provides richer information
func (t *MyTool) GetMetadata() mcp.ToolMetadata {
    return mcp.ToolMetadata{
        Name:        "my_tool",
        Description: "Clear description of what your tool does",
        Version:     "1.0.0",
        Category:    "build", // or "deploy", "scan", "analyze"
        Capabilities: []string{
            "capability-1",
            "capability-2",
        },
        Requirements: []string{
            "docker",
            "git",
        },
        Parameters: map[string]string{
            "input_file": "required - Path to input file",
            "output_dir": "optional - Output directory (default: ./output)",
        },
        Examples: []mcp.ToolExample{
            {
                Description: "Basic usage",
                Args: map[string]interface{}{
                    "input_file": "/path/to/file",
                },
            },
        },
    }
}

// Validate is a new required method
func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    myArgs, ok := args.(*MyToolArgs)
    if !ok {
        return fmt.Errorf("invalid arguments type")
    }
    
    // Validate your arguments
    if myArgs.InputFile == "" {
        return fmt.Errorf("input_file is required")
    }
    
    return nil
}
```

### Step 3: Update Argument Structures

Define your argument structures separately:

```go
// Define args as a separate struct
type MyToolArgs struct {
    SessionID  string `json:"session_id" description:"Session ID for tracking"`
    InputFile  string `json:"input_file" description:"Path to input file"`
    OutputDir  string `json:"output_dir,omitempty" description:"Output directory"`
    Verbose    bool   `json:"verbose,omitempty" description:"Enable verbose output"`
}

// Define result structure
type MyToolResult struct {
    Status     string   `json:"status"`
    OutputPath string   `json:"output_path"`
    Files      []string `json:"files"`
    Duration   string   `json:"duration"`
}
```

### Step 4: Update Error Handling

Migrate to rich error types:

```go
import "github.com/Azure/container-copilot/pkg/mcp/types"

// Before
if err != nil {
    return fmt.Errorf("failed to read file: %v", err)
}

// After
if err != nil {
    return types.NewRichError(
        "FILE_READ_ERROR",
        "Failed to read input file",
        err,
    ).WithContext(map[string]interface{}{
        "file": myArgs.InputFile,
        "tool": "my_tool",
    })
}
```

### Step 5: Add Tool Annotations

Add code generation annotations for automatic registration:

```go
// MyTool processes input files and generates output
// +tool:name=my_tool
// +tool:category=build
// +tool:description=Processes input files and generates optimized output
type MyTool struct {
    logger zerolog.Logger
}
```

### Step 6: Update Tests

Update your tests to match the new interface:

```go
func TestMyTool_Execute(t *testing.T) {
    tool := &MyTool{}
    
    args := &MyToolArgs{
        SessionID: "test-session",
        InputFile: "test.txt",
    }
    
    // Test validation
    err := tool.Validate(context.Background(), args)
    assert.NoError(t, err)
    
    // Test execution
    result, err := tool.Execute(context.Background(), args)
    assert.NoError(t, err)
    
    // Check result type
    myResult, ok := result.(*MyToolResult)
    assert.True(t, ok)
    assert.Equal(t, "success", myResult.Status)
}
```

## Common Migration Patterns

### Pattern 1: Async Operations

If your tool performed async operations:

**Before**:
```go
func (t *MyTool) Run(ctx context.Context, params RunParams) error {
    go t.doAsyncWork(params)
    return nil // Fire and forget
}
```

**After**:
```go
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Return a job ID for tracking
    jobID := t.startAsyncWork(args)
    return &AsyncJobResult{
        JobID: jobID,
        Status: "started",
    }, nil
}
```

### Pattern 2: Progress Reporting

For long-running operations:

```go
func (t *MyTool) ExecuteWithProgress(
    ctx context.Context, 
    args interface{}, 
    reporter mcp.ProgressReporter,
) (interface{}, error) {
    reporter.ReportStage(0.0, "Starting operation")
    
    // Do work...
    reporter.ReportStage(0.5, "Halfway complete")
    
    // More work...
    reporter.ReportStage(1.0, "Operation complete")
    
    return result, nil
}
```

### Pattern 3: Configuration

If your tool used global configuration:

**Before**:
```go
var config = LoadConfig()

func (t *MyTool) Run(ctx context.Context, params RunParams) error {
    url := config.Get("api.url")
    // Use config
}
```

**After**:
```go
type MyTool struct {
    config *Config
}

func NewMyTool(config *Config) *MyTool {
    return &MyTool{config: config}
}

func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    url := t.config.Get("api.url")
    // Use config
}
```

## Compatibility Layer

For gradual migration, you can create a compatibility wrapper:

```go
// LegacyToolAdapter wraps old tools to work with new interface
type LegacyToolAdapter struct {
    legacyTool OldToolInterface
    name       string
}

func (a *LegacyToolAdapter) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Convert new args to old format
    oldParams := convertToOldParams(args)
    
    // Call old method
    err := a.legacyTool.Run(ctx, oldParams)
    
    // Return nil result for compatibility
    return nil, err
}

func (a *LegacyToolAdapter) GetMetadata() mcp.ToolMetadata {
    return mcp.ToolMetadata{
        Name: a.name,
        Description: "Legacy tool - please migrate",
        Version: "0.0.0",
        Category: "legacy",
    }
}

func (a *LegacyToolAdapter) Validate(ctx context.Context, args interface{}) error {
    // Basic validation
    return nil
}
```

## Testing Your Migration

### 1. Compile-Time Checks

Ensure your tool implements the interface:

```go
var _ mcp.Tool = (*MyTool)(nil)
```

### 2. Integration Test

Test with the MCP system:

```go
func TestToolIntegration(t *testing.T) {
    // Create orchestrator
    orchestrator := mcp.NewOrchestrator()
    
    // Register your tool
    tool := NewMyTool()
    err := orchestrator.RegisterTool(tool)
    assert.NoError(t, err)
    
    // Execute through orchestrator
    result, err := orchestrator.ExecuteTool(
        context.Background(),
        "my_tool",
        &MyToolArgs{
            InputFile: "test.txt",
        },
    )
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### 3. Validation Test

Run the interface validator:

```bash
go run tools/validate-interfaces/main.go
```

## Troubleshooting

### Common Issues

1. **Import Cycle Errors**
   - If you're implementing tools inside `pkg/mcp/internal/`, use `mcptypes.InternalTool` instead
   - External tools should use `mcp.Tool`

2. **Type Assertion Failures**
   ```go
   // Always check type assertions
   myArgs, ok := args.(*MyToolArgs)
   if !ok {
       return nil, fmt.Errorf("invalid arguments type: expected *MyToolArgs, got %T", args)
   }
   ```

3. **Missing Methods**
   - Ensure you implement all three methods: Execute, GetMetadata, Validate
   - Use compile-time checks to catch missing methods

4. **Registration Issues**
   - Check that annotations are correctly formatted
   - Run `go generate ./...` after adding annotations
   - Verify tool appears in generated registration code

## Support and Resources

- **Documentation**: See `docs/adding-new-tools.md` for detailed examples
- **Examples**: Check `pkg/mcp/examples/` for reference implementations
- **Issues**: Report migration issues at https://github.com/Azure/container-copilot/issues
- **Interface Reference**: See `pkg/mcp/interfaces.go` for complete interface definitions

## Version Compatibility

| Container Kit Version | Interface Version | Migration Required |
|---------------------|-------------------|-------------------|
| v1.0 - v1.9 | Legacy | Yes |
| v2.0+ | Unified | No |

The new unified interface system is designed to be stable and extensible. Future additions will maintain backward compatibility through interface composition rather than breaking changes.