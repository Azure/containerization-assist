# Interface Pattern Migration Guide

## Overview
This guide helps developers migrate from deprecated patterns to the unified interface pattern used throughout the codebase.

## Table of Contents
1. [Deprecated Map Utility Functions](#deprecated-map-utility-functions)
2. [Error Handling Patterns](#error-handling-patterns)
3. [Tool Registration Patterns](#tool-registration-patterns)
4. [Analyzer Interface Patterns](#analyzer-interface-patterns)
5. [Mock/Test Interface Patterns](#mocktest-interface-patterns)

---

## Deprecated Map Utility Functions

### ❌ Old Pattern (REMOVED)
```go
import "github.com/Azure/container-copilot/pkg/mcp/utils"

// These functions have been REMOVED
name := utils.GetStringFromMap(data, "name")
count := utils.GetIntFromMap(data, "count")
enabled := utils.GetBoolFromMap(data, "enabled")
```

### ✅ New Pattern (Required)
```go
import "github.com/Azure/container-copilot/pkg/genericutils"

// Use generic type-safe functions
name := genericutils.MapGetWithDefault[string](data, "name", "")
count := genericutils.MapGetWithDefault[int](data, "count", 0)
enabled := genericutils.MapGetWithDefault[bool](data, "enabled", false)
```

### Special Case: Integer Conversion with JSON Support
For the special case where you need to handle JSON number conversion (int, float64, int64), use this pattern:

```go
// Create a helper function for JSON-compatible int extraction
func getIntFromMap(m map[string]interface{}, key string) int {
    // Try direct int first
    if val, ok := genericutils.MapGet[int](m, key); ok {
        return val
    }
    // Try float64 (common in JSON)
    if val, ok := genericutils.MapGet[float64](m, key); ok {
        return int(val)
    }
    // Try int64
    if val, ok := genericutils.MapGet[int64](m, key); ok {
        return int(val)
    }
    return 0
}
```

### Migration Steps
1. **Replace imports**: Change `utils` to `genericutils`
2. **Update function calls**: Use `MapGetWithDefault[T]` pattern
3. **Handle JSON conversion**: Use helper function for complex int conversion
4. **Verify types**: Ensure type safety with explicit generic parameters

---

## Error Handling Patterns

### ❌ Old Pattern (REMOVED)
```go
// These panic-prone methods have been REMOVED
result := someResult.Unwrap()           // REMOVED - would panic on error
err := someResult.UnwrapErr()           // REMOVED - would panic on success
```

### ✅ New Pattern (Required)
```go
import "github.com/Azure/container-copilot/pkg/genericutils"

// Safe error handling patterns
value := result.UnwrapOr(defaultValue)  // Safe - returns default on error
value, ok := result.Get()               // Safe - returns value and boolean
if result.IsOk() {
    value := result.UnwrapOr(zero)      // Safe when checked
}

// For debugging/testing with context
value := result.Expect("operation should succeed")  // Panics with context
```

### Best Practices
1. **Always use `UnwrapOr`** instead of `Unwrap()`
2. **Check state first** with `IsOk()` or `IsErr()`
3. **Use `Expect()`** only in tests or with clear error context
4. **Handle both cases** explicitly rather than assuming success

---

## Tool Registration Patterns

### ✅ Current Pattern (Already Implemented)
The auto-registration system automatically discovers and registers tools:

```go
// Tools are auto-discovered by the registration generator
// File: pkg/mcp/internal/build/build_image.go
type BuildImageTool struct {
    // Implementation
}

// Auto-registered as "build_image" in the registry
```

### Auto-Registration Process
1. **Tool Discovery**: Generator scans standard directories
2. **Name Conversion**: `CamelCase` → `snake_case`
3. **Registry Generation**: Creates `auto_registration.go`
4. **Runtime Registration**: Registers with GoMCP server

### Tool Implementation Interface
```go
// Standard tool interface
type Tool interface {
    Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
    Validate(args map[string]interface{}) error
    GetMetadata() ToolMetadata
}
```

### Migration Steps for New Tools
1. **Follow naming convention**: `*Tool` suffix
2. **Implement required interface**: `Execute`, `Validate`, `GetMetadata`
3. **Place in correct directory**: `pkg/mcp/internal/{category}/`
4. **Run generator**: `go run tools/register-tools/main.go`

---

## Analyzer Interface Patterns

### ✅ Production Pattern (Factory-Based)
```go
import "github.com/Azure/container-copilot/pkg/mcp/internal/analyze"

// Use factory pattern for analyzer creation
factory := analyze.NewAnalyzerFactory(logger, enableAI, transport)
analyzer := factory.CreateAnalyzer()

// Returns CallerAnalyzer when AI enabled, StubAnalyzer when disabled
```

### Direct Instantiation (When Needed)
```go
// For AI-enabled production use
analyzer := analyze.NewCallerAnalyzer(transport, analyze.CallerAnalyzerOpts{
    ToolName:       "chat",
    SystemPrompt:   "You are an AI assistant...",
    PerCallTimeout: 60 * time.Second,
})

// For testing or AI-disabled mode
analyzer := analyze.NewStubAnalyzer()
```

### Configuration-Driven Selection
```bash
# Environment variable controls analyzer type
export MCP_ENABLE_AI_ANALYZER=true  # Uses CallerAnalyzer
export MCP_ENABLE_AI_ANALYZER=false # Uses StubAnalyzer (default)
```

### Interface Implementation
```go
// All analyzers implement this interface
type AIAnalyzer interface {
    Analyze(ctx context.Context, prompt string) (string, error)
    AnalyzeWithFileTools(ctx context.Context, prompt string, tools []string) (string, error)
    AnalyzeWithFormat(ctx context.Context, prompt string, format AnalysisFormat) (string, error)
    GetTokenUsage() TokenUsage
    ResetTokenUsage()
}
```

---

## Mock/Test Interface Patterns

### ❌ Old Pattern (REMOVED)
```go
// Removed deprecated constructor
mock := transport.NewMockToolInvokerTransport(logger)  // REMOVED
```

### ✅ New Pattern (Required)
```go
import "github.com/Azure/container-copilot/pkg/mcp/internal/transport"

// Use the standard constructor
mock := transport.NewMockLLMTransport(logger)

// Configure responses
mock.SetResponse("build_image", transport.ToolInvocationResponse{
    Content: "Build completed successfully",
    Error:   "",
})

// Set default response for all tools
mock.SetDefaultResponse(transport.ToolInvocationResponse{
    Content: "Default response",
    Error:   "",
})
```

### Test Configuration Patterns
```go
// Configure mock behavior
mock.SetSimulateDelay(100 * time.Millisecond)  // Simulate network delay
mock.SetPromptResponse("analysis", &types.LLMResponse{
    ResponseMessage: "Analysis complete",
    Confidence:      0.9,
})

// Verify calls
calls := mock.GetCallHistory()
assert.Len(t, calls, 2)
assert.Equal(t, "build_image", calls[0].ToolName)
```

---

## General Migration Principles

### 1. Type Safety
- Use generic types with explicit type parameters
- Avoid `interface{}` when possible
- Leverage compile-time type checking

### 2. Error Handling
- Always handle both success and error cases
- Use descriptive error messages
- Avoid panic-prone patterns in production code

### 3. Interface Consistency
- Implement complete interfaces, not partial ones
- Use factory patterns for complex object creation
- Follow established naming conventions

### 4. Testing Patterns
- Use mock implementations for external dependencies
- Test both success and error paths
- Verify interface compliance in tests

### 5. Documentation
- Document interface expectations clearly
- Provide examples of correct usage
- Mark deprecated patterns clearly

---

## Code Review Checklist

### ❌ Red Flags (Block PR)
- [ ] Uses removed deprecated functions
- [ ] Direct use of `Unwrap()` or `UnwrapErr()`
- [ ] Hardcoded StubAnalyzer when factory pattern available
- [ ] Missing error handling
- [ ] Uses `interface{}` without justification

### ✅ Good Patterns (Approve)
- [ ] Uses `genericutils.MapGetWithDefault[T]`
- [ ] Proper error handling with `UnwrapOr` or explicit checking
- [ ] Factory pattern for analyzer creation
- [ ] Complete interface implementation
- [ ] Type-safe generic usage
- [ ] Clear error messages and documentation

---

## Getting Help

### Resources
- **Interface Documentation**: See individual package README files
- **Examples**: Check test files for usage patterns
- **Migration Issues**: Create GitHub issue with "migration" label

### Contact
- **Sprint C Team**: Interface modernization questions
- **Architecture Team**: Design pattern questions
- **Testing Team**: Mock implementation questions

---

## Summary of Changes

| Old Pattern | New Pattern | Status |
|-------------|-------------|---------|
| `utils.GetStringFromMap()` | `genericutils.MapGetWithDefault[string]()` | ✅ Migrated |
| `result.Unwrap()` | `result.UnwrapOr()` or `result.Get()` | ✅ Migrated |
| `NewMockToolInvokerTransport()` | `NewMockLLMTransport()` | ✅ Migrated |
| Direct StubAnalyzer usage | Factory pattern | ✅ Verified |
| Manual tool registration | Auto-registration | ✅ Verified |

All deprecated patterns have been successfully removed from the codebase. This guide serves as reference for future development and for understanding the migration that was completed.
