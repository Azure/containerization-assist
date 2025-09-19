# Error Taxonomy

## Overview
This document defines the error codes and categorization system used throughout the containerization-assist MCP server. All errors follow a structured format with specific codes for better debugging and monitoring.

## Error Code Categories

### Planning Errors (E_PLAN_*)
Errors that occur during tool execution planning and dependency resolution.

| Code | Description | Recovery Action |
|------|-------------|----------------|
| `E_PLAN_CYCLE` | Circular dependency detected in tool graph | Review tool dependencies, break circular references |
| `E_MISSING_TOOL` | Referenced tool not found in registry | Verify tool name, check tool availability |
| `E_INVALID_DEPS` | Invalid or malformed dependencies | Validate dependency configuration |

### Enrichment Errors (E_PARAM_*)
Errors during parameter enrichment and validation phase.

| Code | Description | Recovery Action |
|------|-------------|----------------|
| `E_PARAM_INFER` | Failed to infer missing parameters | Provide explicit parameter values |
| `E_PARAM_VALIDATION` | Parameter validation failed | Check parameter types and constraints |
| `E_SESSION_ERROR` | Session management failure | Retry with new session or check session store |

### Execution Errors (E_TOOL_* / E_POLICY_*)
Errors that occur during actual tool execution.

| Code | Description | Recovery Action |
|------|-------------|----------------|
| `E_TOOL_EXEC` | Tool execution failed | Check tool logs, verify preconditions |
| `E_POLICY_CLAMP` | Policy constraint violation | Adjust request to comply with policies |
| `E_COST_LIMIT` | Cost limit exceeded | Reduce scope or increase cost limit |
| `E_TIMEOUT` | Execution timeout | Increase timeout or optimize operation |

### System Errors (E_*)
General system-level errors.

| Code | Description | Recovery Action |
|------|-------------|----------------|
| `E_UNKNOWN` | Unknown/unexpected error | Check logs for details |

## Error Object Structure

All errors follow this structure:

```typescript
interface RouterError {
  code: ErrorCode;        // Specific error code
  message: string;         // Human-readable message
  details?: unknown;       // Additional context
}
```

## Usage Examples

### Planning Error
```typescript
{
  code: "E_PLAN_CYCLE",
  message: "Circular dependency detected: analyze-repo -> generate-dockerfile -> analyze-repo",
  details: {
    toolName: "generate-dockerfile",
    cycle: ["analyze-repo", "generate-dockerfile", "analyze-repo"]
  }
}
```

### Parameter Error
```typescript
{
  code: "E_PARAM_VALIDATION",
  message: "Parameter 'path' is required but was not provided",
  details: {
    toolName: "analyze-repo",
    missingParams: ["path"],
    providedParams: ["sessionId"]
  }
}
```

### Execution Error
```typescript
{
  code: "E_COST_LIMIT",
  message: "Cost limit exceeded: $10.24 > $10.00",
  details: {
    totalUsd: 10.24,
    maxCostUsd: 10.00,
    toolsExecuted: ["analyze-repo", "generate-dockerfile"]
  }
}
```

## Telemetry Integration

Errors are tracked in step telemetry:

```typescript
interface StepTelemetry {
  step: string;
  tool: string;
  durationMs: number;
  tokensIn: number;
  tokensOut: number;
  usd: number;
  success: boolean;
  errType?: string;  // Error code if failed
}
```

## Monitoring & Alerting

### Key Metrics
- Error rate by code
- Cost limit violations
- Timeout occurrences
- Policy clamp frequency

### Alert Thresholds
- `E_COST_LIMIT` > 5/hour: Review cost policies
- `E_TIMEOUT` > 10/hour: Check system performance
- `E_PLAN_CYCLE` > 1/day: Review tool dependencies
- `E_POLICY_CLAMP` > 20/hour: Review policy settings

## Best Practices

1. **Always use specific error codes** - Avoid generic errors
2. **Include actionable details** - Provide context for debugging
3. **Log at appropriate levels** - Errors vs warnings vs info
4. **Track error patterns** - Monitor for systemic issues
5. **Document recovery paths** - Help users resolve issues

## Migration Guide

When migrating from unstructured errors:

### Before
```typescript
return Failure("Tool not found");
```

### After
```typescript
return Failure(
  createError(
    ErrorCode.E_MISSING_TOOL,
    `Tool not found: ${toolName}`,
    { toolName, availableTools }
  ).message
);
```

## HTTP Status Mapping

For API responses:

| Error Code | HTTP Status |
|------------|-------------|
| `E_MISSING_TOOL` | 404 Not Found |
| `E_PARAM_*` | 400 Bad Request |
| `E_POLICY_*` | 403 Forbidden |
| `E_COST_LIMIT` | 402 Payment Required |
| `E_TIMEOUT` | 408 Request Timeout |
| `E_TOOL_EXEC` | 500 Internal Server Error |
| `E_UNKNOWN` | 500 Internal Server Error |