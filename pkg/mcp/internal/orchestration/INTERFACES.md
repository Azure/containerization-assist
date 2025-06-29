# Orchestration Package Interfaces

## Overview

The orchestration package defines two local interfaces to avoid import cycles with the main `pkg/mcp` package:

1. **ToolInstanceRegistry** - Provides access to tool instances
2. **ToolOrchestrationExecutor** - Coordinates tool execution

## Why Local Interfaces?

These interfaces exist for several important reasons:

### 1. Import Cycle Prevention
The main `pkg/mcp/interfaces.go` defines comprehensive interfaces that depend on types from various packages. If the orchestration package imported these interfaces directly, it would create circular dependencies.

### 2. Different Semantics
- **Main ToolRegistry**: Returns `ToolFactory` instances (functions that create tools)
- **ToolInstanceRegistry**: Returns actual tool instances (`interface{}`)

The orchestration package needs direct access to tool instances for execution, not factories.

### 3. Minimal Surface Area
The orchestration interfaces are intentionally minimal, exposing only what's needed for tool coordination without bringing in the full complexity of the main interfaces.

## Interface Comparison

### Main Package (pkg/mcp/interfaces.go)
```go
type ToolRegistry interface {
    Register(name string, factory ToolFactory) error
    Get(name string) (ToolFactory, error)
    List() []string
    GetMetadata() map[string]ToolMetadata
}
```

### Orchestration Package (local)
```go
type ToolInstanceRegistry interface {
    GetTool(name string) (interface{}, error)
}
```

## Implementation

The `MCPToolRegistry` in this package implements `ToolInstanceRegistry` and stores actual tool instances. This is different from the main registry pattern which uses factories.

## Best Practices

1. Keep these interfaces minimal and focused
2. Document why they differ from main interfaces
3. Consider them internal implementation details
4. Don't expose them outside the orchestration package
