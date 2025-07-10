# Schema Generation Guide

## Overview

The Container Kit MCP Server uses automated schema generation to create JSON schemas from the canonical Tool interface. This ensures consistency between tool implementations and their schemas while reducing manual maintenance.

## Architecture

### Canonical Tool Interface
All tools implement the standard interface defined in `pkg/mcp/domain/tools/interface.go`:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() *json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (*ExecutionResult, error)
    Category() string
    Tags() []string
    Version() string
}
```

### Schema Generator
The schema generator (`cmd/mcp-schema-gen`) creates:
- JSON schemas from Go type definitions
- Canonical tool implementations with proper schema methods
- Validation code for input parameters

## Usage

### 1. Generate Schemas (Automated)

Run the make target to generate all schemas:
```bash
make mcp-schema-gen
```

This will:
1. Build the schema generator binary at `cmd/mcp-schema-gen`
2. Generate schemas for tools in domain packages
3. Create JSON schema files for each tool

### 2. Validate Schemas

Ensure all generated schemas are valid JSON:
```bash
make schema-validate
```

### 3. Add Schema Generation to a Tool

Add a go:generate directive to your tool file:
```go
//go:generate go run ../../../../../cmd/mcp-schema-gen/main.go -tool <tool_name> -domain <domain> -output <output_file>

package mypackage
```

Example:
```go
//go:generate go run ../../../../../cmd/mcp-schema-gen/main.go -tool analyze_repository -domain analyze -output analyze_schema.json
```

### 4. Manual Schema Generation

For individual tool schema generation:
```bash
go run cmd/mcp-schema-gen/main.go \
  -tool analyze_repository \
  -domain analyze \
  -desc "Analyzes repository for containerization" \
  -output pkg/mcp/domain/containerization/analyze/analyze_schema.json
```

## Schema Format

Generated schemas follow the JSON Schema Draft 7 specification:

```json
{
  "type": "object",
  "properties": {
    "session_id": {
      "type": "string",
      "description": "Session ID for the operation"
    },
    "repo_url": {
      "type": "string",
      "description": "Repository URL to analyze"
    },
    "branch": {
      "type": "string",
      "description": "Branch to analyze (optional)"
    }
  },
  "required": ["session_id", "repo_url"]
}
```

## Tool Implementation

### Canonical Tool Example

```go
package analyze

//go:generate go run ../../../../../cmd/mcp-schema-gen/main.go -tool analyze_repository -domain analyze -output analyze_schema.json

import (
    "context"
    "encoding/json"
    "github.com/Azure/container-kit/pkg/mcp/domain/tools"
)

type AnalyzeTool struct {
    // tool fields
}

func (t *AnalyzeTool) InputSchema() *json.RawMessage {
    schema := json.RawMessage(`{
        "type": "object",
        "properties": {
            "session_id": {"type": "string"},
            "repo_url": {"type": "string"}
        },
        "required": ["session_id", "repo_url"]
    }`)
    return &schema
}

func (t *AnalyzeTool) Execute(ctx context.Context, input json.RawMessage) (*tools.ExecutionResult, error) {
    // Parse input according to schema
    var params struct {
        SessionID string `json:"session_id"`
        RepoURL   string `json:"repo_url"`
    }

    if err := json.Unmarshal(input, &params); err != nil {
        return &tools.ExecutionResult{
            IsError: true,
            Content: []tools.ContentBlock{{
                Type: "text",
                Text: "Invalid input",
            }},
        }, err
    }

    // Execute tool logic
    // ...
}
```

## Custom Schema Hints

Use struct tags to provide schema generation hints:

```go
type ToolParams struct {
    SessionID string `json:"session_id" schema:"required,description=Session identifier"`
    RepoURL   string `json:"repo_url" schema:"required,description=Repository URL,format=uri"`
    Branch    string `json:"branch,omitempty" schema:"description=Git branch,default=main"`
    Timeout   int    `json:"timeout,omitempty" schema:"minimum=0,maximum=3600,default=300"`
}
```

Supported schema tags:
- `required`: Mark field as required
- `description`: Field description
- `format`: JSON Schema format (uri, email, date-time, etc.)
- `minimum`/`maximum`: Numeric constraints
- `minLength`/`maxLength`: String length constraints
- `pattern`: Regex pattern for validation
- `default`: Default value
- `enum`: Comma-separated list of allowed values

## Integration with MCP Protocol

The generated schemas are used by:
1. **Tool Discovery**: MCP clients can discover available tools and their schemas
2. **Validation**: Input validation before tool execution
3. **Documentation**: Auto-generated API documentation
4. **Type Safety**: Compile-time type checking for tool implementations

## Best Practices

1. **Keep Schemas Simple**: Use basic types (string, number, boolean, array, object)
2. **Provide Descriptions**: Always include descriptions for parameters
3. **Mark Required Fields**: Explicitly mark required fields
4. **Use Consistent Naming**: Follow the naming conventions (snake_case for JSON)
5. **Version Your Schemas**: Include version in tool metadata
6. **Test Schema Generation**: Validate generated schemas in CI/CD

## Troubleshooting

### Schema Generation Fails
- Ensure the schema generator is built: `go build ./cmd/mcp-schema-gen`
- Check go:generate directive syntax
- Verify relative paths in the directive

### Invalid JSON Schema
- Run `make schema-validate` to check all schemas
- Use a JSON validator to identify syntax errors
- Ensure proper escaping in embedded JSON strings

### Schema Not Updated
- Run `go generate ./...` to regenerate all schemas
- Check if go:generate directive is present
- Verify file permissions

## CI/CD Integration

Add to your CI pipeline:

```yaml
# .github/workflows/schema-check.yml
name: Schema Validation
on: [push, pull_request]

jobs:
  validate-schemas:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Generate Schemas
        run: make schema-gen

      - name: Validate Schemas
        run: make schema-validate

      - name: Check for Changes
        run: |
          git diff --exit-code || (echo "Schemas need regeneration" && exit 1)
```

## Future Enhancements

1. **OpenAPI Generation**: Generate OpenAPI specs from tool schemas
2. **Client SDK Generation**: Generate typed clients from schemas
3. **Schema Versioning**: Support multiple schema versions
4. **Runtime Validation**: Enhanced runtime validation with detailed errors
5. **Schema Registry**: Central schema registry for tool discovery
