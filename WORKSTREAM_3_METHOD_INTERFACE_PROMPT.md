# Workstream 3: Fix Method Signature and Interface Compatibility Issues

## Objective
Fix all method signature mismatches and interface compatibility issues identified in the pre-commit checks. These are issues where types don't implement expected interfaces or have wrong method signatures.

## Scope
Focus exclusively on fixing method signatures, interface implementations, and type compatibility without changing core business logic.

## Affected Areas and Issues

### 1. Context Interface Implementation Issues

**File**: `pkg/mcp/server/core.go`

**Issues**:
- Line 196: `buildImageTool.ExecuteWithContext(ctx, args)` - Context type mismatch
- Line 202: `pushImageTool.ExecuteWithContext(ctx, args)` - Context type mismatch

**Problem**: 
```go
// Current: gomcp server.Context doesn't implement context.Context
cannot use ctx (variable of type *"github.com/localrivet/gomcp/server".Context) as context.Context value in argument to ExecuteWithContext: 
*"github.com/localrivet/gomcp/server".Context does not implement context.Context (wrong type for method Deadline)
    have Deadline() (interface{}, bool)
    want Deadline() (time.Time, bool)
```

### 2. Logger Type Compatibility Issues

**File**: `pkg/mcp/server/core.go`

**Issue**:
- Line 353: `runtime.NewToolRegistry(logger.With("component", "tool_registry"))`

**Problem**:
```go
// Current: slog.Logger being passed where zerolog.Logger expected
cannot use logger.With("component", "tool_registry") (value of type *slog.Logger) as zerolog.Logger value in argument to runtime.NewToolRegistry
```

### 3. Struct Field Type Mismatches

**File**: `pkg/mcp/tools/deploy/typesafe_deploy_tool_simple.go`

**Issues with local DeployResult type vs core.DeployResult**:
- Lines 457, 461, 469, 470, 472, 477, 478, 480, 485, 486, 488: Missing fields in local DeployResult
- Local DeployResult doesn't have: `DeploymentTime`, `Data`, `Errors`, `Warnings` fields

### 4. Health Check Interface Issues

**File**: `pkg/mcp/tools/deploy/validate_deployment.go`

**Issues with local HealthCheckResult vs core.HealthCheckResult**:
- Lines 213, 216, 236, 366, 367, 372, 373: Missing fields in local HealthCheckResult
- Local HealthCheckResult doesn't have: `Healthy`, `Error`, `StatusCode`, `Checked`, `Endpoint` fields

### 5. Type Assertion and Usage Issues

**File**: `pkg/mcp/internal/pipeline/interface_implementations_test.go`

**Issue**:
- Line 299: `invalid argument: result.SecretsFound (variable of type int) for built-in len`

**Problem**: Using `len()` on an integer field instead of a slice/array

## Instructions

1. **Fix Context Interface Issues**:
   - Create adapter functions to convert between gomcp.Context and context.Context
   - Or modify method signatures to accept the correct context type

2. **Fix Logger Type Issues**:
   - Create adapter functions to convert between slog.Logger and zerolog.Logger
   - Or update method signatures to accept the correct logger type

3. **Align Struct Definitions**:
   - Ensure local struct types match core struct types
   - Use core types instead of local redefinitions where possible
   - Add missing fields to local structs if they need to diverge

4. **Fix Type Usage Issues**:
   - Replace incorrect usage patterns (like using len() on integers)
   - Ensure proper type assertions and conversions

5. **Maintain Interface Contracts**:
   - Ensure all interface implementations have correct method signatures
   - Update interface definitions if needed to match actual usage

## Success Criteria
- All method signature mismatch errors are resolved
- All interface compatibility issues are fixed
- All type assertion errors are resolved
- No new compilation errors are introduced
- Existing functionality is preserved

## Example Patterns

### Context Adapter Pattern
```go
// Create adapter function for context conversion
func adaptContext(mcpCtx *gomcp.Context) context.Context {
    // Implementation to convert between context types
    return context.WithValue(context.Background(), "mcp_context", mcpCtx)
}

// Usage
return buildImageTool.ExecuteWithContext(adaptContext(ctx), args)
```

### Logger Adapter Pattern
```go
// Create adapter for logger types
func adaptLogger(slogLogger *slog.Logger) zerolog.Logger {
    // Implementation to convert or wrap logger
    return zerolog.New(os.Stderr).With().Logger()
}

// Usage
toolRegistry := runtime.NewToolRegistry(adaptLogger(logger.With("component", "tool_registry")))
```

### Struct Alignment Pattern
```go
// Use core types instead of local redefinitions
import "github.com/Azure/container-kit/pkg/mcp/core"

// Instead of local type, use core type
func someFunction() *core.DeployResult {
    return &core.DeployResult{
        BaseToolResponse: core.BaseToolResponse{},
        DeploymentTime:   time.Now(),
        Data:            make(map[string]interface{}),
        Errors:          []string{},
        Warnings:        []string{},
    }
}
```