# Error Taxonomy and Context Standardization Guide

## Error Code Taxonomy

### General Errors (1000-1999)
- `UNKNOWN` - Unknown error type
- `INTERNAL_ERROR` - Internal system errors
- `NOT_IMPLEMENTED` - Feature not yet implemented
- `DISABLED` - Feature or service disabled

### Validation Errors (2000-2999)
- `VALIDATION_FAILED` - General validation failure
- `INVALID_PARAMETER` - Invalid parameter value
- `MISSING_PARAMETER` - Required parameter missing
- `TYPE_CONVERSION_FAILED` - Type conversion error
- `VALIDATION_ERROR` - General validation error

### Resource Errors (3000-3999)
- `NOT_FOUND` - Resource not found
- `ALREADY_EXISTS` - Resource already exists
- `RESOURCE_NOT_FOUND` - Specific resource not found
- `RESOURCE_ALREADY_EXISTS` - Specific resource exists
- `RESOURCE_EXHAUSTED` - Resource limits exceeded

### Network/IO Errors (4000-4999)
- `NETWORK_TIMEOUT` - Network operation timeout
- `NETWORK_ERROR` - General network error
- `IO_ERROR` - Input/output error
- `FILE_NOT_FOUND` - File not found
- `TIMEOUT_ERROR` - Operation timeout

### Security/Permission Errors (5000-5999)
- `PERMISSION_DENIED` - Access permission denied
- `SECURITY_ERROR` - General security error
- `SECURITY_VIOLATION` - Security policy violation
- `VULNERABILITY_FOUND` - Security vulnerability detected

### Container/Docker Errors (6000-6999)
- `DOCKERFILE_SYNTAX_ERROR` - Dockerfile syntax error
- `IMAGE_BUILD_FAILED` - Image build failure
- `IMAGE_PUSH_FAILED` - Image push failure
- `IMAGE_PULL_FAILED` - Image pull failure
- `CONTAINER_START_FAILED` - Container start failure

### Kubernetes Errors (7000-7999)
- `KUBERNETES_API_ERROR` - Kubernetes API error
- `MANIFEST_INVALID` - Invalid K8s manifest
- `DEPLOYMENT_FAILED` - Deployment failure
- `NAMESPACE_NOT_FOUND` - Namespace not found

### Tool/Registry Errors (8000-8999)
- `TOOL_NOT_FOUND` - Tool not found
- `TOOL_EXECUTION_FAILED` - Tool execution failure
- `TOOL_ALREADY_REGISTERED` - Tool already registered
- `VERSION_MISMATCH` - Version incompatibility

### Configuration Errors (9000-9999)
- `CONFIGURATION_INVALID` - Invalid configuration
- `INVALID_STATE` - Invalid state
- `OPERATION_FAILED` - Operation failure

## Error Type Categories

### Core Types
- `internal` - Internal system errors
- `validation` - Input validation errors
- `network` - Network-related errors
- `io` - Input/output errors
- `timeout` - Timeout errors
- `not_found` - Resource not found errors
- `conflict` - Resource conflict errors

### Domain Types
- `container` - Container/Docker errors
- `kubernetes` - Kubernetes errors
- `tool` - Tool-related errors
- `security` - Security errors
- `session` - Session management errors
- `resource` - Resource management errors
- `business` - Business logic errors
- `system` - System-level errors
- `permission` - Permission errors
- `configuration` - Configuration errors
- `operation` - Operation errors
- `external` - External service errors

## Context Standardization

### Required Context Fields

1. **For Resource Operations**:
   - `resource` - Resource type (e.g., "tool", "session", "image")
   - `identifier` - Resource identifier

2. **For Operations**:
   - `operation` - Operation name
   - `phase` - Operation phase (optional)

3. **For Validation**:
   - `field` - Field name
   - `value` - Invalid value (if safe to log)
   - `constraint` - Constraint violated

4. **For Network/IO**:
   - `endpoint` - Network endpoint
   - `path` - File/resource path
   - `duration` - Timeout duration

5. **For Security**:
   - `violation` - Type of violation
   - `user` - User identifier (if applicable)
   - `action` - Attempted action

### Optional Context Fields
- `request_id` - Request tracking ID
- `session_id` - Session identifier
- `timestamp` - Error timestamp
- `retry_count` - Number of retries
- `suggestion` - User-friendly suggestion

## Error Severity Guidelines

### Critical
- Security violations
- Data corruption risks
- System-wide failures

### High
- Service failures
- Operation failures
- Configuration errors

### Medium
- Validation errors
- Not found errors
- Permission denials

### Low
- Warnings
- Deprecation notices
- Performance issues

## Constructor Usage Examples

```go
// Missing parameter
NewMissingParam("image_name")

// Validation failure
NewValidationFailed("port", "must be between 1-65535")

// Internal error with cause
NewInternalError("database_query", err)

// Configuration error
NewConfigurationError("database", "connection string invalid")

// Not found
NewNotFoundError("tool", "build_image")

// Permission denied
NewPermissionDeniedError("registry", "push")

// Timeout
NewTimeoutError("docker_build", "5m")

// Network error
NewNetworkError("registry_push", err)

// Already exists
NewAlreadyExistsError("session", "sess_123")

// Multi-error aggregation
NewMultiError("deployment", []error{err1, err2, err3})
```

## Best Practices

1. **Use specific constructors** over generic NewError() when available
2. **Include relevant context** but avoid sensitive data
3. **Provide actionable suggestions** in error messages
4. **Use appropriate severity** based on impact
5. **Wrap underlying errors** to preserve stack traces
6. **Use consistent field names** in context
7. **Keep error messages concise** but informative