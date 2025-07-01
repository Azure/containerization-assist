# HTTP Transport Tool Schema Access

This document explains how to access tool schemas and parameters through the HTTP transport in the MCP server.

## Overview

The HTTP transport now provides endpoints to retrieve detailed tool schemas, including parameter information generated using reflection and JSON schema generation.

## Available Endpoints

### 1. List Tools with Optional Schema
```
GET /api/v1/tools?include_schema=true
```

Lists all available tools. When `include_schema=true` is passed as a query parameter, includes detailed schema information for each tool.

**Response Example:**
```json
{
  "tools": [
    {
      "name": "analyze_repository",
      "description": "Analyze a repository to detect language, framework, and containerization requirements",
      "endpoint": "/api/v1/tools/analyze_repository",
      "parameters": {
        "repo_url": "Repository URL to analyze",
        "session_id": "Session ID for tracking"
      },
      "category": "analysis",
      "version": "1.0.0"
    }
  ],
  "count": 1
}
```

### 2. Get All Tool Schemas
```
GET /api/v1/tools/schemas
```

Returns comprehensive schema information for all registered tools, including:
- Parameter schemas with types and descriptions
- Output schemas
- Tool metadata (category, version, dependencies, capabilities)
- Examples

**Response Example:**
```json
{
  "schemas": {
    "analyze_repository": {
      "name": "analyze_repository",
      "description": "Analyze a repository...",
      "category": "analysis",
      "version": "1.0.0",
      "dependencies": [],
      "capabilities": ["language_detection", "framework_analysis"],
      "parameters": {
        "repo_url": "string",
        "session_id": "string"
      },
      "output": {
        "type": "object",
        "properties": {
          "success": {"type": "boolean"},
          "language": {"type": "string"},
          "framework": {"type": "string"}
        }
      }
    }
  },
  "count": 1
}
```

### 3. Get Specific Tool Schema
```
GET /api/v1/tools/{toolName}/schema
```

Returns detailed schema information for a specific tool.

**Response Example:**
```json
{
  "name": "analyze_repository",
  "description": "Analyze a repository...",
  "metadata": {
    "category": "analysis",
    "version": "1.0.0",
    "parameters": {
      "repo_url": "Repository URL to analyze",
      "session_id": "Session ID"
    }
  },
  "schema": {
    "parameters": {
      "type": "object",
      "properties": {
        "repo_url": {
          "type": "string",
          "description": "Repository URL to analyze"
        },
        "session_id": {
          "type": "string",
          "description": "Session ID for tracking"
        }
      },
      "required": ["repo_url"]
    },
    "output": {
      "type": "object",
      "properties": {
        "success": {"type": "boolean"},
        "analysis_result": {"type": "object"}
      }
    }
  }
}
```

## Implementation Details

The tool schema functionality leverages:

1. **Tool Registry**: The `MCPToolRegistry` maintains metadata about all registered tools
2. **Reflection**: Uses Go reflection to analyze tool parameter and result types
3. **JSON Schema Generation**: Uses the `invopop/jsonschema` library to generate proper JSON schemas
4. **Server Reference**: The HTTP transport maintains a reference to the MCP server to access the tool orchestrator and registry

## Usage Example

Here's a simple Go client example:

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    // Get all tool schemas
    resp, err := http.Get("http://localhost:8080/api/v1/tools/schemas")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    var schemas map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&schemas)

    // Pretty print the schemas
    data, _ := json.MarshalIndent(schemas, "", "  ")
    fmt.Println(string(data))
}
```

## Testing

Use the provided test script to verify the endpoints:

```bash
go run test_tool_schemas.go
```

This will test all three endpoints and display the responses.

## Notes

- Tool schemas are generated dynamically based on the registered tools
- The schema includes both input parameters and output types
- Parameter descriptions are extracted from struct tags when available
- The system uses the unified interface pattern to avoid import cycles
