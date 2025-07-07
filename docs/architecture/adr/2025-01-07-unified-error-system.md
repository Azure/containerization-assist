# ADR-004: Unified Rich Error System

Date: 2025-01-07
Status: Accepted
Context: 3 competing error systems create inconsistency and poor error experience

## Decision
Standardize on single rich-error framework with context and metadata

## Problem
The codebase had multiple competing error handling approaches:
1. Standard Go `fmt.Errorf` scattered throughout code
2. Legacy MCPError system in domain/errors
3. Application-level error wrappers

This led to:
- Inconsistent error messages and handling
- Difficulty in error categorization and debugging
- Poor error context for troubleshooting
- Multiple import paths for error handling

## Solution
Consolidate all error handling to use the unified RichError system from `pkg/mcp/domain/errors/rich.go`:

### Features
- **Structured error codes**: Unique error identifiers for each error type
- **Error categorization**: Validation, Network, Internal, etc.
- **Severity levels**: Low, Medium, High, Critical
- **Rich context**: Key-value pairs for debugging information
- **Source location**: Automatic capture of file/line where error occurred
- **Error chaining**: Proper cause tracking with `Unwrap()` support
- **Suggestions**: Human-readable resolution guidance

### Builder Pattern
```go
return errors.NewError().
    Code(errors.CodeValidationFailed).
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Message("invalid input parameter").
    Context("parameter", paramName).
    Context("value", paramValue).
    Suggestion("Check parameter format and try again").
    WithLocation().
    Build()
```

## Consequences

### Easier
- Consistent error handling across all components
- Better debugging with structured context and location info
- Categorized error handling for retry logic and user feedback
- Machine-readable error codes for automation
- Standardized error responses for API consumers

### Harder
- Migration effort from existing fmt.Errorf calls
- Learning curve for rich error patterns
- Slightly more verbose error creation code
- Need to maintain error code consistency

## Implementation Status
- ✅ RichError framework established in `pkg/mcp/domain/errors/rich.go`
- ✅ Removed competing error handling in `pkg/mcp/application/internal/common/error_handling.go`
- 🔄 Migrating remaining `fmt.Errorf` calls to RichError pattern
- ✅ Error codes and types standardized
- ✅ Builder pattern for fluent error construction

## Migration Guidelines
1. Replace `fmt.Errorf` with appropriate error builders
2. Add relevant context fields for debugging
3. Use appropriate error codes and categories
4. Include actionable suggestions where possible
5. Capture source location with `WithLocation()`

## Success Metrics
- Target: 0 `fmt.Errorf` calls in production code
- All errors provide structured context
- Consistent error handling patterns across modules
- Improved debugging and troubleshooting experience
