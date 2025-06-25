# Basic Tool Example

This example demonstrates a simple tool implementation using the Container Kit v2.0 unified interface.

## Overview

The `SimpleTool` shows:
- Basic implementation of the `Tool` interface
- Input validation
- Structured arguments and results
- Metadata definition
- Interface compliance checking

## Key Concepts

### 1. Tool Interface Implementation

All tools must implement three methods:
```go
Execute(ctx context.Context, args interface{}) (interface{}, error)
GetMetadata() mcp.ToolMetadata
Validate(ctx context.Context, args interface{}) error
```

### 2. Type Safety

Use type assertions to handle the `interface{}` parameters:
```go
toolArgs, ok := args.(*SimpleToolArgs)
if !ok {
    return nil, fmt.Errorf("invalid arguments type")
}
```

### 3. Validation

Implement comprehensive validation in the `Validate` method:
- Check required fields
- Validate field constraints
- Return clear error messages

### 4. Metadata

Provide rich metadata for tool discovery and documentation:
- Name and description
- Version and category
- Capabilities and requirements
- Parameter documentation
- Usage examples

## Running the Example

```bash
# Run the example
go run main.go

# Expected output:
# Tool: simple_tool v1.0.0
# Description: A simple example tool that echoes messages
# Category: example
# 
# Example 1: Valid execution
# ✓ Validation passed
# ✓ Execution successful: Hello, Alice! You said: Hello from the unified interface!
#   Status: success
#   Echo: Hello from the unified interface!
# 
# Example 2: Invalid arguments (missing name)
# ✓ Validation correctly failed: name is required
# 
# Example 3: Invalid arguments (name too long)
# ✓ Validation correctly failed: name must be 50 characters or less
```

## Integration with MCP

To use this tool in the MCP system:

1. Add the tool annotation for auto-registration:
```go
// SimpleTool demonstrates a basic tool implementation
// +tool:name=simple_tool
// +tool:category=example
// +tool:description=A simple example tool that echoes messages
type SimpleTool struct {
    name string
}
```

2. Run code generation:
```bash
go generate ./...
```

3. The tool will be automatically registered and available through the orchestrator.

## Next Steps

- See `../tool-with-validation/` for advanced validation examples
- See `../tool-with-progress/` for progress reporting
- See `../orchestrator-usage/` for using tools through the orchestrator