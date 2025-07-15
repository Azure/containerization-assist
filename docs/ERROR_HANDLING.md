# Error Handling Guidelines

## Overview

Container Kit uses a unified error handling system based on Rich errors to provide structured, traceable, and AI-friendly error context.

## Error Types

### Rich Errors (`pkg/common/errors`)
The base error type that provides:
- Structured error codes with severity levels
- Location tracking (file:line)
- Retryable flags
- Domain classification
- Custom context fields

```go
// Basic usage
err := errors.New(errors.CodeValidationFailed, "workflow", "invalid input", cause)

// With context
err := errors.New(errors.CodeValidationFailed, "workflow", "invalid input", cause).
    With("field", "repo_url").
    With("value", repoURL)
```

### Workflow Errors (`pkg/common/errors/workflow_context.go`)
Extended Rich errors for workflow operations:
- Step tracking
- Attempt counting
- Fix history for AI-assisted recovery
- Workflow ID association

```go
// Create workflow error
wfErr := errors.NewWorkflowError(
    errors.CodeImageBuildFailed,
    "docker",
    "build",
    "Docker build failed",
    cause,
).WithWorkflowID(workflowID).
  WithStepContext("dockerfile_path", path)

// Add fix attempts
wfErr.AddFixAttempt("Updated base image to alpine:3.18")
```

## Migration from fmt.Errorf

### ❌ Don't Use
```go
return fmt.Errorf("failed to build image: %w", err)
```

### ✅ Use Instead
```go
return errors.NewWorkflowError(
    errors.CodeImageBuildFailed,
    "docker",
    "build", 
    "failed to build image",
    err,
)
```

## Error Code Selection

Choose appropriate error codes based on the failure type:

- **CodeValidationFailed**: Input validation errors
- **CodeImageBuildFailed**: Docker build failures
- **CodeDeploymentFailed**: Kubernetes deployment failures
- **CodeOperationFailed**: Generic operation failures
- **CodeResourceNotFound**: Missing resources
- **CodePermissionDenied**: Authorization failures
- **CodeTimeoutError**: Timeout-related failures

## Domain Error Conversion

Use the converter utilities for existing domain errors:

```go
converter := errors.NewDomainErrorConverter("kubernetes")
richErr := converter.ConvertError(domainErr, errors.CodeDeploymentFailed)
```

## Linting Rules

The project enforces error handling standards via golangci-lint:

1. **No fmt.Errorf** outside test files
2. **Use Rich/Workflow errors** for all error creation
3. **Include context** for debugging and AI recovery

## Testing

In test files, you can still use `fmt.Errorf` for simple test assertions:

```go
// OK in *_test.go files
if result == nil {
    t.Errorf("expected result, got nil")
}

// Better approach
assert.NotNil(t, result, "result should not be nil")
```

## Error Recovery

The workflow error system supports AI-assisted error recovery:

```go
// Error history tracking
history := errors.NewWorkflowErrorHistory(10)
history.AddError(workflowError)

// Get AI-friendly summary
summary := history.GetAISummary()
```

## Best Practices

1. **Always use structured errors** for runtime failures
2. **Include relevant context** for debugging
3. **Choose appropriate error codes** for the failure type
4. **Add workflow context** for step-based operations
5. **Record fix attempts** for AI learning
6. **Test error paths** with structured assertions

## Example: Complete Error Handling

```go
func deployToKubernetes(ctx context.Context, manifest string) error {
    // Attempt deployment
    if err := k8sClient.Apply(ctx, manifest); err != nil {
        // Create workflow error with context
        wfErr := errors.NewWorkflowError(
            errors.CodeDeploymentFailed,
            "kubernetes", 
            "deploy",
            "failed to apply Kubernetes manifest", 
            err,
        ).WithStepContext("manifest_size", len(manifest)).
          WithStepContext("namespace", namespace)
        
        // Add to error history for AI recovery
        errorHistory.AddError(wfErr)
        
        return wfErr
    }
    
    return nil
}
```

This approach provides comprehensive error context for debugging, monitoring, and AI-assisted problem resolution.