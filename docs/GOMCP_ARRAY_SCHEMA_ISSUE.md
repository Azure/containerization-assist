# GoMCP Array Schema Issue (RESOLVED)

## Problem Description

GoMCP (github.com/localrivet/gomcp v1.6.5) had a bug in its JSON Schema generation where it failed to include the `items` property for array fields. This caused validation failures in MCP clients that strictly validate JSON schemas (like GitHub Copilot).

**UPDATE**: This issue has been resolved by using a forked version of GoMCP with the array schema generation fix applied. The fork is configured in `go.mod`:

```go
replace github.com/localrivet/gomcp => github.com/gambtho/gomcp v0.0.0-20250626062144-6c7ed4c9d536
```

### Example of the Issue

When a tool has an array parameter like:
```go
type Args struct {
    Labels []string `json:"labels"`
}
```

GoMCP generates:
```json
{
  "type": "object",
  "properties": {
    "labels": {
      "type": "array"
      // Missing: "items": {"type": "string"}
    }
  }
}
```

But it should generate:
```json
{
  "type": "object", 
  "properties": {
    "labels": {
      "type": "array",
      "items": {"type": "string"}
    }
  }
}
```

## Affected Tools

The following tools in Container Kit have array parameters and are affected by this issue:

1. **update_session_labels** - `labels` field (array of strings)
2. **generate_manifests** - Multiple array fields:
   - `secrets` (array of objects)
   - `registry_secrets` (array of objects)
   - `ingress_tls` (array of objects)
   - `ingress_hosts` (array of objects)
   - `service_ports` (array of objects)
3. **validate_dockerfile** - Array fields:
   - `ignore_rules` (array of strings)
   - `trusted_registries` (array of strings)
4. **scan_image_security** - `vuln_types` field (array of strings)
5. **scan_secrets** - Array fields:
   - `exclude_patterns` (array of strings)
   - `file_patterns` (array of strings)

## Resolution

The issue has been resolved by using a forked version of GoMCP that includes the array schema fix. The `RegisterSimpleToolWithFixedSchema` function now simply delegates to the standard registration since the fork generates correct schemas.

## Attempted Solutions

We investigated several approaches to fix this issue:

1. **Schema Interception**: Attempted to intercept and modify schemas after GoMCP generates them, but GoMCP doesn't expose any hooks for this.

2. **Server Wrapper**: Created `SchemaFixingServerWrapper` to wrap the GoMCP server, but discovered that GoMCP handles the tools/list request internally without any way to intercept it.

3. **Runtime Patching**: Explored using reflection to patch GoMCP's schema generator at runtime, but this would be fragile and could break with GoMCP updates.

4. **Transport-level Interception**: Considered intercepting at the JSON-RPC transport level, but GoMCP handles all protocol communication internally.

## Root Cause

The issue is in GoMCP's schema generation code. When it uses reflection to generate JSON schemas from Go structs, it correctly identifies array types but fails to generate the `items` property that describes the array element type.

## Permanent Solutions

There are only a few ways to permanently fix this issue:

1. **Fix GoMCP Upstream**: Submit a PR to GoMCP to fix their schema generation for arrays. This is the ideal solution.

2. **Fork GoMCP**: Create and maintain a fork of GoMCP with the array schema fix applied. This requires ongoing maintenance.

3. **Use Alternative MCP Library**: Switch to a different MCP server implementation that properly generates schemas or allows custom schema definitions.

4. **Implement JSON-RPC Proxy**: Create a separate process that intercepts stdio communication and modifies the tools/list response. This is complex and fragile.

## Impact

Until this is fixed:

1. GitHub Copilot and other strict MCP clients may reject tools with array parameters
2. The tools still function correctly when called, but may not appear in the tool list
3. Clients that don't strictly validate schemas (more lenient parsers) will work fine

## Internal Registry

Our internal tool registry (`pkg/mcp/internal/runtime/registry.go`) uses invopop/jsonschema for internal validation and documentation. With the fixed fork of GoMCP, both internal and external schemas now correctly include array items properties.