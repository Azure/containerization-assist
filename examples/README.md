# Container Kit MCP Examples

This directory contains examples demonstrating how to use the unified interface system in Container Kit v2.0.

## Examples Overview

### Basic Tool Implementation
- `basic-tool/` - Simple tool implementing the unified interface
- `tool-with-validation/` - Tool with comprehensive validation
- `tool-with-progress/` - Tool with progress reporting

### Domain-Specific Examples
- `build-tool-example/` - Example build domain tool
- `deploy-tool-example/` - Example deployment tool
- `scan-tool-example/` - Example security scanning tool
- `analyze-tool-example/` - Example analysis tool

### Integration Examples
- `orchestrator-usage/` - Using the tool orchestrator
- `session-management/` - Working with sessions
- `error-handling/` - Rich error handling patterns

### Migration Examples
- `v1-to-v2-migration/` - Migrating a v1 tool to v2
- `compatibility-adapter/` - Using the compatibility layer

## Running the Examples

Each example includes a README with specific instructions. Generally:

```bash
# Navigate to an example
cd basic-tool/

# Run the example
go run main.go

# Run tests
go test ./...
```

## Quick Start

For a basic tool implementation:

```go
package main

import (
    "context"
    "fmt"
    "github.com/Azure/container-copilot/pkg/mcp"
)

type MyTool struct{}

func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    return map[string]string{"status": "success"}, nil
}

func (t *MyTool) GetMetadata() mcp.ToolMetadata {
    return mcp.ToolMetadata{
        Name:        "my_tool",
        Description: "A simple example tool",
        Version:     "1.0.0",
        Category:    "example",
    }
}

func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    return nil
}

// Ensure interface compliance
var _ mcp.Tool = (*MyTool)(nil)
```

See individual examples for more detailed implementations.