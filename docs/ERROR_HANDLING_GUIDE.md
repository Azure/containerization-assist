# Container Kit MCP Error Handling Guide

## Overview

Container Kit MCP uses a unified Rich Error System that provides comprehensive error information with domain-specific handling, context enrichment, and actionable suggestions. This guide explains how to use the error system effectively.

> **📋 Boundary Compliance**: For information about RichError requirements in exported functions, see the [RichError Boundary Compliance Guide](./development/error-handling.md).

## Error System Architecture

### Core Components

1. **Rich Error Structure** (`pkg/mcp/domain/errors/rich.go`)
   - Comprehensive error information with code, message, type, and severity
   - Context and metadata for debugging
   - Error chaining for proper error propagation
   - Suggestions for resolution

2. **Domain-Specific Factories** (`pkg/mcp/domain/errors/factories.go`)
   - `BuildError()` - Build and container operations
   - `DeployError()` - Kubernetes deployment operations
   - `SecurityError()` - Security scanning and vulnerabilities
   - `ValidationError()` - Input and configuration validation
   - `NetworkError()` - Network and connectivity issues
   - `SystemError()` - System-level errors

3. **Error Codes** (`pkg/mcp/domain/errors/codes/`)
   - Centralized error code definitions by domain
   - Consistent error identification across the system

4. **Error Classification** (`pkg/mcp/domain/errors/classification.go`)
   - Automatic error classification for retryability
   - Severity-based retry strategies
   - User-facing error determination

## Using the Error System

### Creating Errors

#### Using Domain-Specific Factories

```go
import (
    "github.com/Azure/container-kit/pkg/mcp/domain/errors"
    "github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
)

// Build domain error
err := errors.BuildError(
    codes.BUILD_DOCKERFILE_INVALID,
    "Invalid Dockerfile syntax at line 10",
    originalErr,
).WithContext("line", 10).
  WithContext("file", "Dockerfile").
  WithSuggestion("Check for missing FROM statement")

// Deploy domain error
err := errors.DeployError(
    codes.DEPLOY_MANIFEST_INVALID,
    "Invalid Kubernetes manifest",
    nil,
).WithContext("manifest", "deployment.yaml").
  WithSeverity(errors.SeverityHigh)

// Security domain error
err := errors.SecurityError(
    codes.SECURITY_VULNERABILITY_CRITICAL,
    "Critical vulnerability CVE-2024-1234 found",
    nil,
).WithContext("cve", "CVE-2024-1234").
  WithContext("package", "openssl")
```

#### Using the Error Builder

```go
// For more complex error construction
err := errors.NewError().
    Code(errors.CodeValidationFailed).
    Message("Configuration validation failed").
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Context("field", "port").
    Context("value", -1).
    Suggestion("Port must be between 1 and 65535").
    WithLocation().  // Captures source location
    Build()
```

#### Using Specialized Constructors

```go
// Build failures
err := errors.BuildFailedError("docker-build", "Out of disk space")

// Deployment failures
err := errors.DeploymentError("nginx-deployment", "ImagePullBackOff")

// Security vulnerabilities
err := errors.VulnerabilityError("critical", 5, "Multiple CVEs found")

// Session errors
err := errors.SessionExpiredError("session-123")
```

### Error Context and Metadata

Always provide relevant context to help with debugging:

```go
err := errors.BuildError(code, message, cause).
    WithContext("docker_version", "20.10.17").
    WithContext("build_context", "/app").
    WithContext("stage", "base-image").
    WithContext("attempt", 3)
```

### Error Suggestions

Provide actionable suggestions for error resolution:

```go
err := errors.NetworkError(
    codes.NETWORK_TIMEOUT,
    "Connection timeout to registry",
    nil,
).WithSuggestion("Check network connectivity").
  WithSuggestion("Verify registry URL is correct").
  WithSuggestion("Try increasing timeout value")
```

### Error Classification and Retry Logic

The error system automatically classifies errors for retry handling:

```go
// Check if error should be retried
if errors.ShouldRetry(err, attemptNumber) {
    delay := errors.GetRetryDelay(err, attemptNumber)
    time.Sleep(delay)
    // Retry operation
}

// Check error characteristics
if errors.IsUserFacing(err) {
    // Show error to user with suggestions
}

if errors.RequiresAuth(err) {
    // Trigger re-authentication
}
```

## Best Practices

### 1. Always Use Domain-Specific Factories

❌ **Don't:**
```go
return fmt.Errorf("build failed: %v", err)
```

✅ **Do:**
```go
return errors.BuildError(
    codes.BUILD_EXECUTION_FAILED,
    "Docker build failed",
    err,
).WithContext("image", imageName)
```

### 2. Preserve Error Chains

Always wrap underlying errors to maintain the error chain:

```go
if err != nil {
    return errors.BuildError(
        codes.BUILD_LAYER_FAILED,
        "Failed to process build layer",
        err,  // Preserve original error
    ).WithContext("layer", layerID)
}
```

### 3. Use Appropriate Error Codes

Select specific error codes from the predefined constants:

```go
// Use specific codes
errors.BuildError(codes.BUILD_DOCKERFILE_MISSING, ...)  // ✅
errors.BuildError(codes.BUILD_CACHE_FAILED, ...)       // ✅

// Avoid generic codes when specific ones exist
errors.BuildError(errors.CodeUnknown, ...)             // ❌
```

### 4. Include Relevant Context

Add context that will help debugging:

```go
err := errors.DeployError(code, message, cause).
    WithContext("namespace", namespace).
    WithContext("deployment", deploymentName).
    WithContext("replicas", replicaCount).
    WithContext("cluster", clusterName)
```

### 5. Provide Helpful Suggestions

Include actionable suggestions for common errors:

```go
if strings.Contains(err.Error(), "permission denied") {
    return errors.BuildError(code, message, err).
        WithSuggestion("Run with elevated permissions (sudo)").
        WithSuggestion("Check file ownership and permissions").
        WithSuggestion("Ensure Docker daemon is accessible")
}
```

## Migration from Legacy Patterns

### From fmt.Errorf

```go
// Old pattern
return fmt.Errorf("failed to build image %s: %v", imageName, err)

// New pattern
return errors.BuildError(
    codes.BUILD_IMAGE_FAILED,
    fmt.Sprintf("Failed to build image %s", imageName),
    err,
)
```

### From errors.New

```go
// Old pattern
return errors.New("validation failed")

// New pattern
return errors.ValidationError(
    codes.VALIDATION_FAILED,
    "Validation failed",
    nil,
)
```

### From Simple Error Wrapping

```go
// Old pattern
if err != nil {
    return fmt.Errorf("deploy failed: %w", err)
}

// New pattern
if err != nil {
    return errors.DeployError(
        codes.DEPLOY_EXECUTION_FAILED,
        "Deployment failed",
        err,
    )
}
```

## Error Handling Patterns

### Tool Implementation

```go
func (t *BuildTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Validate arguments
    buildArgs, ok := args.(BuildArgs)
    if !ok {
        return nil, errors.ValidationError(
            codes.VALIDATION_FAILED,
            "Invalid argument type for build tool",
            nil,
        ).WithContext("expected", "BuildArgs").
          WithContext("received", fmt.Sprintf("%T", args))
    }

    // Perform operation
    result, err := t.performBuild(ctx, buildArgs)
    if err != nil {
        return nil, errors.BuildError(
            codes.BUILD_EXECUTION_FAILED,
            "Build operation failed",
            err,
        ).WithContext("dockerfile", buildArgs.DockerfilePath).
          WithContext("context", buildArgs.BuildContext)
    }

    return result, nil
}
```

### Error Recovery

```go
func (s *Service) executeWithRetry(ctx context.Context, operation func() error) error {
    var lastErr error

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }

        lastErr = err

        // Check if error is retryable
        if !errors.ShouldRetry(err, attempt) {
            return err
        }

        // Calculate retry delay
        delay := errors.GetRetryDelay(err, attempt)

        select {
        case <-ctx.Done():
            return errors.SystemError(
                codes.TIMEOUT_DEADLINE_EXCEEDED,
                "Operation cancelled",
                ctx.Err(),
            )
        case <-time.After(delay):
            // Continue to next attempt
        }
    }

    return errors.SystemError(
        codes.SYSTEM_ERROR,
        fmt.Sprintf("Operation failed after %d attempts", maxAttempts),
        lastErr,
    )
}
```

## Testing with Errors

### Creating Test Errors

```go
func TestBuildOperation(t *testing.T) {
    // Create a test error
    testErr := errors.BuildError(
        codes.BUILD_DOCKERFILE_INVALID,
        "Test error",
        nil,
    )

    // Test error properties
    assert.Equal(t, codes.BUILD_DOCKERFILE_INVALID, testErr.Code)
    assert.Equal(t, errors.ErrTypeContainer, testErr.Type)

    // Test error classification
    classification := errors.ClassifyError(testErr)
    assert.True(t, classification.Retryable)
}
```

### Mocking Errors

```go
type mockBuilder struct {
    shouldFail bool
    failureErr error
}

func (m *mockBuilder) Build() error {
    if m.shouldFail {
        if m.failureErr != nil {
            return m.failureErr
        }
        return errors.BuildError(
            codes.BUILD_EXECUTION_FAILED,
            "Mock build failure",
            nil,
        )
    }
    return nil
}
```

## Monitoring and Logging

### Structured Logging

```go
logger.Error().
    Str("error_code", string(err.Code)).
    Str("error_type", string(err.Type)).
    Str("severity", string(err.Severity)).
    Interface("context", err.Context).
    Err(err).
    Msg("Operation failed")
```

### Metrics Collection

```go
// Track errors by type and severity
errorCounter.WithLabelValues(
    string(err.Type),
    string(err.Severity),
    string(err.Code),
).Inc()
```

## Appendix: Error Code Reference

See the following files for complete error code listings:
- `pkg/mcp/domain/errors/codes/build_codes.go` - Build domain error codes
- `pkg/mcp/domain/errors/codes/deploy_codes.go` - Deploy domain error codes
- `pkg/mcp/domain/errors/codes/security_codes.go` - Security domain error codes
- `pkg/mcp/domain/errors/codes/common_codes.go` - Common error codes

## Support

For questions or issues with the error handling system, please refer to:
- The error system source code in `pkg/mcp/domain/errors/`
- The Container Kit MCP documentation
- The project issue tracker
