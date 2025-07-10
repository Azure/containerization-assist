# Error Propagation Analysis

## Current State Summary
- **Total fmt.Errorf usage**: 135 instances (vs 622 expected)
- **Total RichError usage**: 717 instances  
- **Files with mixed usage**: 7 files

## Error Pattern Distribution

### Top Error Hotspots
1. `pkg/mcp/domain/errors/root_errors.go` - 25 instances
2. `pkg/mcp/infra/infra.go` - 17 instances
3. `pkg/mcp/infra/retry/coordinator_test.go` - 11 instances
4. `pkg/mcp/domain/errors/errors_test.go` - 11 instances

### Layer Distribution
- **Domain Layer**: ~30 fmt.Errorf calls (needs 100% RichError)
- **Application Layer**: ~40 fmt.Errorf calls
- **Infrastructure Layer**: ~65 fmt.Errorf calls (acceptable for some cases)

## Error Propagation Patterns Observed

### Pattern 1: Simple Error Wrapping
```go
fmt.Errorf("failed to %s: %w", operation, err)
```
Should be converted to:
```go
errors.NewInternalError(operation, err)
```

### Pattern 2: Validation Errors
```go
fmt.Errorf("missing %s", fieldName)
fmt.Errorf("invalid %s", fieldName)
```
Should use existing constructors:
```go
errors.MissingParameterError(fieldName)
errors.ToolValidationError(toolName, field, message, code, value)
```

### Pattern 3: Multi-error Aggregation
```go
fmt.Errorf("errors stopping workers: %v", errors)
```
Needs new constructor for multi-error scenarios.

### Pattern 4: Panic Recovery
```go
fmt.Errorf("worker panicked: %v", r)
```
Needs special handling for panic recovery with stack trace.

## Mixed Usage Files Analysis
Files that use both fmt.Errorf and RichError:
1. `conversation_handler.go` - Transitioning to RichError
2. `background_workers.go` - Error aggregation patterns
3. `service_bridges.go` - Service layer boundaries

## Migration Priority
1. **High Priority**: Domain layer (must be 100% RichError)
2. **Medium Priority**: Application layer service interfaces
3. **Low Priority**: Test files and infrastructure hot paths

## Recommendations
1. Create additional error constructors for common patterns
2. Focus on domain layer first for 100% compliance
3. Identify performance-critical paths for grandfathering
4. Establish clear patterns for error aggregation