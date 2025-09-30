# Error Guidance System

Phase B-B1 enhancement providing actionable error guidance to operators.

## Overview

The error guidance system enriches failure messages with:
- **Hint**: What went wrong in user-friendly terms
- **Resolution**: Specific steps to fix the issue
- **Details**: Additional context for debugging

## Architecture

### Core Types

```typescript
// src/types/core.ts
export interface ErrorGuidance {
  message: string;
  hint?: string;
  resolution?: string;
  details?: Record<string, unknown>;
}

export type Result<T> =
  | { ok: true; value: T }
  | { ok: false; error: string; guidance?: ErrorGuidance };
```

### Error Extraction Functions

**Docker Errors** (`src/infra/docker/errors.ts`):
```typescript
export function extractDockerErrorGuidance(error: unknown): ErrorGuidance
```

**Kubernetes Errors** (`src/infra/kubernetes/errors.ts`):
```typescript
export function extractK8sErrorGuidance(error: unknown, operation?: string): ErrorGuidance
```

## Error Examples

### Docker Connection Refused

```typescript
{
  message: "Docker daemon is not available",
  hint: "Connection to Docker daemon was refused",
  resolution: "Ensure Docker is installed and running: `docker ps` should succeed. Check Docker daemon logs if the service is running."
}
```

### Docker Authentication Failed

```typescript
{
  message: "Docker registry authentication failed",
  hint: "Invalid or missing registry credentials",
  resolution: "Run `docker login <registry>` to authenticate, or verify credentials in your Docker config (~/.docker/config.json)."
}
```

### Kubernetes Configuration Missing

```typescript
{
  message: "Kubernetes configuration not found",
  hint: "Unable to locate or read kubeconfig file",
  resolution: "Set KUBECONFIG environment variable or ensure ~/.kube/config exists. Run `kubectl config view` to verify."
}
```

### Kubernetes Authorization Failed

```typescript
{
  message: "Kubernetes authorization failed",
  hint: "Your user/service account lacks required permissions",
  resolution: "Verify RBAC permissions with `kubectl auth can-i <verb> <resource>`. Contact cluster administrator to grant necessary roles."
}
```

## MCP Error Formatting

The MCP server automatically formats errors with guidance for the AI:

```
Failed to push image: Docker daemon is not available

💡 Connection to Docker daemon was refused

🔧 Resolution:
Ensure Docker is installed and running: `docker ps` should succeed. Check Docker daemon logs if the service is running.
```

## Usage in Tools

Tools propagate guidance from infrastructure clients:

```typescript
// src/tools/push-image/tool.ts
const pushResult = await dockerClient.pushImage(repository, tag);
if (!pushResult.ok) {
  // Use the guidance from the Docker client if available
  return Failure(`Failed to push image: ${pushResult.error}`, pushResult.guidance);
}
```

## Testing

Comprehensive tests in `test/unit/infra/error-guidance.test.ts`:
- 25+ test cases covering Docker and Kubernetes errors
- Validation of guidance structure and content
- Ensures no internal stack traces leak
- Verifies operator-friendly resolutions

## Backward Compatibility

The system maintains backward compatibility:
- `Failure(error)` still works without guidance
- Existing `extractDockerErrorMessage()` delegates to guidance function
- All 887 existing tests pass

## Guidelines for New Errors

When adding error guidance:

1. **Message**: Concise description (< 300 chars)
2. **Hint**: Explain what went wrong in user terms (< 200 chars)
3. **Resolution**: Provide specific commands and steps
4. **No Leaks**: Never include stack traces or internal paths
5. **Actionable**: Include commands in backticks

Example:
```typescript
return Failure(
  'Operation failed',
  createErrorGuidance(
    'Operation failed',
    'Brief explanation of what went wrong',
    'Step 1: Run `command1`\nStep 2: Check configuration with `command2`'
  )
);
```