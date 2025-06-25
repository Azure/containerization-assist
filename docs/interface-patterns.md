# Interface Patterns Documentation

## Overview

This document explains the dual-interface strategy used in the Container Kit MCP system to maintain clean architecture while avoiding Go import cycles.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            Public API Layer                              │
│                                                                         │
│  ┌─────────────────────────────────────────┐                          │
│  │         pkg/mcp/interfaces.go            │                          │
│  │                                          │                          │
│  │  • Tool                                  │ ← External tools import  │
│  │  • Session                               │                          │
│  │  • Transport                             │                          │
│  │  • Orchestrator                          │                          │
│  │  • RequestHandler                        │                          │
│  └─────────────────────────────────────────┘                          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↑
                          ❌ IMPORT CYCLE if internal
                             packages import this
                                    ↓
┌─────────────────────────────────────────────────────────────────────────┐
│                           Internal Layer                                 │
│                                                                         │
│  ┌─────────────────────────────────────────┐                          │
│  │      pkg/mcp/types/interfaces.go         │                          │
│  │                                          │                          │
│  │  • InternalTool                          │ ← Internal packages use  │
│  │  • InternalSession                       │                          │
│  │  • InternalTransport                     │                          │
│  │  • InternalOrchestrator                  │                          │
│  │  • InternalRequestHandler                │                          │
│  └─────────────────────────────────────────┘                          │
│                                    ↑                                    │
│                                    │                                    │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐       │
│  │ internal/build/ │  │ internal/deploy │  │ internal/scan/  │       │
│  │                 │  │                 │  │                 │       │
│  │ Implements      │  │ Implements      │  │ Implements      │       │
│  │ InternalTool    │  │ InternalTool    │  │ InternalTool    │       │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

                   Interface Bridging at Boundaries
┌─────────────────────────────────────────────────────────────────────────┐
│                      Orchestration/Adapter Layer                         │
│                                                                         │
│   func RegisterTool(tool interface{}) {                                │
│       switch t := tool.(type) {                                        │
│       case mcp.Tool:           // External tool                        │
│       case mcptypes.InternalTool:  // Internal tool                    │
│       }                                                                │
│   }                                                                    │
└─────────────────────────────────────────────────────────────────────────┘
```

## The Dual-Interface Strategy

### Problem Statement

In Go, import cycles are not allowed. When creating a unified interface system, we face a fundamental challenge:

```
pkg/mcp/interfaces.go (defines Tool interface)
    ↓ (wants to import)
pkg/mcp/internal/tools/ (implements Tool interface)
    ↓ (wants to import)
pkg/mcp/ (for Tool interface)
    ↑ IMPORT CYCLE!
```

### Solution: Internal + Public Interface Pattern

We solve this with a dual-interface approach:

1. **Public Interfaces** (`pkg/mcp/interfaces.go`) - Single source of truth
2. **Internal Interfaces** (`pkg/mcp/types/interfaces.go`) - Lightweight versions for internal use

## Interface Mapping

### Core Tool Interface

**Public Interface** (`pkg/mcp/interfaces.go`):
```go
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}
```

**Internal Interface** (`pkg/mcp/types/interfaces.go`):
```go
type InternalTool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata  // Same return type
    Validate(ctx context.Context, args interface{}) error
}
```

**Key Principle**: Identical method signatures ensure type compatibility.

### Transport Interface

**Public Interface**:
```go
type Transport interface {
    Serve(ctx context.Context) error
    Stop() error
    Name() string
    SetHandler(handler RequestHandler)
}
```

**Internal Interface**:
```go
type InternalTransport interface {
    Serve(ctx context.Context) error
    Stop() error
    Name() string
    SetHandler(handler InternalRequestHandler)
}
```

### Request Handler Interface

**Public Interface**:
```go
type RequestHandler interface {
    HandleRequest(ctx context.Context, req interface{}) (interface{}, error)
}
```

**Internal Interface**:
```go
type InternalRequestHandler interface {
    HandleRequest(ctx context.Context, req interface{}) (interface{}, error)
}
```

## Usage Patterns

### For Internal Packages

Internal packages (under `pkg/mcp/internal/`) use the Internal prefixed interfaces:

```go
// pkg/mcp/internal/build/build_tool.go
package build

import mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"

type BuildImageTool struct {
    // tool implementation
}

// Implements mcptypes.InternalTool interface
func (t *BuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // implementation
}

func (t *BuildImageTool) GetMetadata() mcptypes.ToolMetadata {
    return mcptypes.ToolMetadata{
        Name: "build_image",
        // ... metadata
    }
}

func (t *BuildImageTool) Validate(ctx context.Context, args interface{}) error {
    // validation
}
```

### For External Packages

External packages and public APIs use the main interfaces:

```go
// External tool implementation
package mytool

import "github.com/Azure/container-copilot/pkg/mcp"

type MyTool struct {
    // tool implementation
}

// Implements mcp.Tool interface
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // implementation
}

func (t *MyTool) GetMetadata() mcp.ToolMetadata {
    return mcp.ToolMetadata{
        Name: "my_tool",
        // ... metadata
    }
}
```

### Interface Bridging

When internal and external systems need to interact, type assertions handle the conversion:

```go
// In orchestrator or adapter layer
func registerTool(tool interface{}) error {
    // Handle both internal and external tool types
    switch t := tool.(type) {
    case mcp.Tool:
        // External tool - use directly
        return registry.Register(t)
    case mcptypes.InternalTool:
        // Internal tool - wrap if needed
        adapter := &toolAdapter{internal: t}
        return registry.Register(adapter)
    default:
        return fmt.Errorf("unsupported tool type")
    }
}
```

## Return Type Strategy

### Interface{} Returns

To maintain compatibility between internal and external interfaces, we use `interface{}` for return types that might vary:

```go
// Both interfaces return interface{} for flexibility
type Tool interface {
    GetMetadata() ToolMetadata  // Concrete type
    Execute(ctx context.Context, args interface{}) (interface{}, error)  // Generic
}

type InternalTool interface {
    GetMetadata() ToolMetadata  // Same concrete type
    Execute(ctx context.Context, args interface{}) (interface{}, error)  // Same generic
}
```

### Type Assertions

When working with `interface{}` returns, use type assertions:

```go
result, err := tool.Execute(ctx, args)
if err != nil {
    return err
}

// Assert to expected type
if buildResult, ok := result.(*BuildResult); ok {
    // Handle build result
    return processBuildResult(buildResult)
}
```

## Naming Conventions

### Interface Names

1. **Public Interfaces**: Use clear, domain-specific names
   - `Tool`, `Transport`, `Session`, `Orchestrator`

2. **Internal Interfaces**: Prefix with "Internal"
   - `InternalTool`, `InternalTransport`, `InternalRequestHandler`

3. **Domain Interfaces**: Use domain prefix to avoid conflicts
   - `DockerfileValidator`, `RuntimeAnalyzer`, `KubernetesDeployer`

### Method Names

- Use consistent method names across interfaces
- Follow Go conventions (exported methods start with capital letters)
- Provide clear intent through naming

```go
// Good - Clear intent
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}

// Avoid - Unclear or inconsistent naming
type Tool interface {
    Run(ctx context.Context, args interface{}) (interface{}, error)      // Inconsistent
    Meta() ToolMetadata                                                  // Abbreviated
    Check(ctx context.Context, args interface{}) error                  // Unclear
}
```

## Best Practices

### 1. Maintain Method Signature Compatibility

Always ensure internal and public interfaces have identical method signatures:

```go
// ✅ Good - Identical signatures
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
}

type InternalTool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
}

// ❌ Bad - Different signatures
type Tool interface {
    Execute(ctx context.Context, args ToolArgs) (ToolResult, error)
}

type InternalTool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
}
```

### 2. Use Interface{} for Flexible Types

When types might vary between implementations, use `interface{}`:

```go
// Flexible for different tool result types
Execute(ctx context.Context, args interface{}) (interface{}, error)

// Specific for consistent metadata
GetMetadata() ToolMetadata
```

### 3. Document Interface Relationships

Always document which internal interface corresponds to which public interface:

```go
// InternalTool provides the core tool interface for internal use
// This interface mirrors mcp.Tool to avoid import cycles
type InternalTool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}
```

### 4. Validate Interface Compliance

Use the validation tool to ensure no duplicate interface definitions:

```bash
go run tools/validate-interfaces/main.go
```

### 5. Prefer Composition Over Large Interfaces

Break large interfaces into smaller, focused ones:

```go
// ✅ Good - Focused interfaces
type ToolExecutor interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
}

type ToolValidator interface {
    Validate(ctx context.Context, args interface{}) error
}

type ToolMetadataProvider interface {
    GetMetadata() ToolMetadata
}

// Compose them
type Tool interface {
    ToolExecutor
    ToolValidator
    ToolMetadataProvider
}

// ❌ Avoid - Monolithic interfaces
type MegaTool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    Validate(ctx context.Context, args interface{}) error
    GetMetadata() ToolMetadata
    Start() error
    Stop() error
    Configure(config Config) error
    GetStatus() Status
    // ... 20+ more methods
}
```

## Testing Interface Implementations

### Mock Objects

Create mocks that implement both internal and external interfaces:

```go
type MockTool struct {
    ExecuteFunc    func(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadataFunc func() ToolMetadata
    ValidateFunc   func(ctx context.Context, args interface{}) error
}

// Implement both interfaces
func (m *MockTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    if m.ExecuteFunc != nil {
        return m.ExecuteFunc(ctx, args)
    }
    return nil, nil
}

func (m *MockTool) GetMetadata() ToolMetadata {
    if m.GetMetadataFunc != nil {
        return m.GetMetadataFunc()
    }
    return ToolMetadata{Name: "mock"}
}

func (m *MockTool) Validate(ctx context.Context, args interface{}) error {
    if m.ValidateFunc != nil {
        return m.ValidateFunc(ctx, args)
    }
    return nil
}

// Ensure it implements both interfaces
var _ mcp.Tool = (*MockTool)(nil)
var _ mcptypes.InternalTool = (*MockTool)(nil)
```

### Interface Compliance Tests

Test that your implementations satisfy both interfaces:

```go
func TestToolImplementsInterfaces(t *testing.T) {
    tool := &MyTool{}
    
    // Should implement external interface
    var _ mcp.Tool = tool
    
    // Should implement internal interface
    var _ mcptypes.InternalTool = tool
    
    // Test actual functionality
    ctx := context.Background()
    result, err := tool.Execute(ctx, nil)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

## Migration Guide

### Adding New Methods

When adding new methods to interfaces:

1. Add to the public interface first
2. Add the identical method signature to the internal interface
3. Update all implementations
4. Run validation to ensure consistency

```go
// Step 1: Add to public interface
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
    // New method
    GetCapabilities() []string
}

// Step 2: Add to internal interface
type InternalTool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
    // New method with identical signature
    GetCapabilities() []string
}
```

### Deprecating Methods

When deprecating methods:

1. Add deprecation comments
2. Provide alternative implementations
3. Update documentation
4. Plan removal timeline

```go
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
    
    // Deprecated: Use GetMetadata().Version instead
    // Will be removed in v2.0
    GetVersion() string
}
```

This dual-interface pattern ensures we maintain clean architecture, avoid import cycles, and provide a clear separation between internal implementation details and public APIs while keeping the system maintainable and extensible.