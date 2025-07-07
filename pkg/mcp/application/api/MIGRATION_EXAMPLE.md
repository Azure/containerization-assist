# API Package Migration Example

This document demonstrates how to migrate from the old scattered interfaces to the new canonical API package.

## Example 1: Migrating Tool Interface

### Before (Multiple Definitions)
```go
// In pkg/mcp/interfaces.go
type Tool interface {
    Name() string
    Description() string
    Execute(input ToolInput) (ToolOutput, error)
    GetSchema() ToolSchema
}

// In pkg/mcp/core/tool_types.go
type Tool interface {
    Name() string
    Description() string
    Schema() interface{}
    Execute(input ToolInput) (ToolOutput, error)
}

// In pkg/mcp/tools/types.go
type Tool interface {
    Name() string
    Description() string
    Execute(input ToolInput) (ToolOutput, error)
    GetSchema() ToolSchema
}

// In pkg/mcp/registry/interfaces.go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input interface{}) (interface{}, error)
}
```

### After (Single Canonical Definition)
```go
// In pkg/mcp/api/tool.go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
    Schema() ToolSchema
}
```

## Example 2: Creating Type Aliases for Gradual Migration

### Step 1: Add Type Alias in Your Package
```go
// pkg/mcp/core/aliases.go
package core

import "github.com/Azure/container-kit/pkg/mcp/api"

// Type aliases for backward compatibility
type Tool = api.Tool
type ToolInput = api.ToolInput
type ToolOutput = api.ToolOutput
type ToolSchema = api.ToolSchema
```

### Step 2: Update Imports Gradually
```go
// Old code continues to work
func RegisterTool(tool core.Tool) error {
    // Implementation
}

// New code uses api package directly
func RegisterNewTool(tool api.Tool) error {
    // Implementation
}
```

## Example 3: Migrating Tool Implementation

### Before
```go
package analyze

import (
    "github.com/Azure/container-kit/pkg/mcp/core"
)

type AnalyzeTool struct {
    // fields
}

func (t *AnalyzeTool) Execute(input core.ToolInput) (core.ToolOutput, error) {
    // No context support
    return core.ToolOutput{}, nil
}
```

### After
```go
package analyze

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/api"
)

type AnalyzeTool struct {
    // fields
}

func (t *AnalyzeTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Now with context support
    return api.ToolOutput{
        Success: true,
        Data: map[string]interface{}{
            "result": "analyzed",
        },
    }, nil
}

func (t *AnalyzeTool) Schema() api.ToolSchema {
    return api.ToolSchema{
        Name:        t.Name(),
        Description: t.Description(),
        Version:     "1.0.0",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "repo_url": map[string]interface{}{
                    "type": "string",
                },
            },
        },
    }
}
```

## Example 4: Migrating Registry Usage

### Before
```go
import (
    "github.com/Azure/container-kit/pkg/mcp/core/orchestration"
    "github.com/Azure/container-kit/pkg/mcp/registry"
)

// Multiple registry types
var toolRegistry orchestration.Registry
var validatorRegistry registry.ValidatorRegistry
var unifiedRegistry registry.UnifiedRegistry
```

### After
```go
import (
    "github.com/Azure/container-kit/pkg/mcp/api"
)

// Single registry interface
var registry api.Registry

// Register tools with options
err := registry.Register(tool,
    api.WithNamespace("analyze"),
    api.WithTags("repository", "analysis"),
    api.WithTimeout(30 * time.Second),
)
```

## Example 5: Using Advanced Tool Features

### Context-Aware Tool
```go
type MyContextTool struct {
    timeout time.Duration
}

// Implement api.ContextTool
func (t *MyContextTool) Validate(ctx context.Context, input api.ToolInput) error {
    if input.SessionID == "" {
        return fmt.Errorf("session ID required")
    }
    return nil
}

func (t *MyContextTool) GetTimeout() time.Duration {
    return t.timeout
}

func (t *MyContextTool) GetRetryPolicy() api.RetryPolicy {
    return api.DefaultRetryPolicy()
}
```

### Streaming Tool
```go
type MyStreamTool struct{}

// Implement api.StreamTool
func (t *MyStreamTool) Stream(ctx context.Context, input api.ToolInput) (<-chan api.ToolOutput, <-chan error) {
    outputCh := make(chan api.ToolOutput)
    errorCh := make(chan error)

    go func() {
        defer close(outputCh)
        defer close(errorCh)

        // Stream results
        for i := 0; i < 10; i++ {
            select {
            case <-ctx.Done():
                errorCh <- ctx.Err()
                return
            case outputCh <- api.ToolOutput{
                Success: true,
                Data: map[string]interface{}{
                    "progress": i * 10,
                },
            }:
            }
        }
    }()

    return outputCh, errorCh
}

func (t *MyStreamTool) SupportsStreaming() bool {
    return true
}
```

## Migration Checklist

1. [ ] Identify all files importing old interfaces
2. [ ] Create type aliases in transition packages
3. [ ] Update tool implementations to use context
4. [ ] Replace interface{} with proper types
5. [ ] Update registry usage to unified interface
6. [ ] Remove old interface definitions
7. [ ] Update tests to use new interfaces
8. [ ] Remove type aliases after full migration

## Common Issues and Solutions

### Import Cycles
If you encounter import cycles:
1. Ensure you're importing from `pkg/mcp/api` only
2. Move shared types to `api` package
3. Use interfaces instead of concrete types

### Missing Context Parameter
Old tools don't have context support:
```go
// Adapter for legacy tools
type LegacyToolAdapter struct {
    legacy OldTool
}

func (a *LegacyToolAdapter) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Ignore context for now
    return a.legacy.Execute(input)
}
```

### Type Mismatches
Use conversion functions during transition:
```go
func ConvertToAPIInput(old OldInput) api.ToolInput {
    return api.ToolInput{
        SessionID: old.SessionID,
        Data:      old.Data,
        Context:   make(map[string]interface{}),
    }
}
```
