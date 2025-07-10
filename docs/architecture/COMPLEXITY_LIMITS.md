# Function Complexity Limits

This document explains the cyclomatic complexity limits enforced by the CI system and the rationale behind them.

## Overview

Cyclomatic complexity is a software metric that measures the number of linearly independent paths through a program's source code. Lower complexity generally means easier to understand, test, and maintain code.

## Default Limits

- **Default Maximum Complexity**: 20
- **Target Complexity**: ≤15 for most functions
- **Warning Threshold**: 20
- **Fail Threshold**: 25 (without exceptions)

## Functions with Custom Limits

Some functions are inherently complex due to their nature and have been granted custom complexity limits:

| Function | Allowed Complexity | Current Complexity | File | Rationale |
|----------|-------------------|-------------------|------|-----------|
| `registerCommonFixes` | 45 | 40 | `pkg/mcp/application/conversation/auto_fix_helper.go:74` | Comprehensive fix registration with many conditions |
| `chainMatches` | 25 | 21 | `pkg/mcp/application/conversation/fix_strategy_chaining.go:284` | Complex pattern matching logic |
| `RegisterTools` | 30 | 27 | `pkg/mcp/application/core/server_impl.go:168` | Tool registration with extensive validation |

## Complexity Categories

### Simple Functions (1-10)
- Basic getters/setters
- Simple data transformations
- Straightforward business logic

### Moderate Functions (11-15)
- Functions with conditional logic
- Loops with simple conditions
- Basic error handling patterns

### Complex Functions (16-20)
- Functions with multiple conditional paths
- Nested loops or conditions
- Complex error handling
- **This is the default maximum limit**

### Highly Complex Functions (21+)
- Only allowed with explicit exceptions
- Require strong justification
- Should be candidates for refactoring when possible

## Adding New Exceptions

To add a function to the allowed exceptions list:

1. **Identify the function**: Get the exact function name and complexity
2. **Justify the complexity**: Explain why the function needs to be complex
3. **Update the checker**: Modify `tools/complexity-checker/main.go`
4. **Set reasonable limit**: Allow some headroom above current complexity
5. **Document the decision**: Update this file with the rationale

Example modification in `tools/complexity-checker/main.go`:
```go
allowedFunctions: map[string]int{
    "yourFunctionName": 35,  // Current: 32, justified by [reason]
}
```

## Refactoring Guidelines

For functions approaching or exceeding complexity limits:

### Techniques to Reduce Complexity
1. **Extract Methods**: Break large functions into smaller, focused helper methods
2. **Early Returns**: Use guard clauses to reduce nesting
3. **Strategy Pattern**: Replace complex conditionals with strategy objects
4. **Table-Driven Logic**: Replace switch statements with lookup tables
5. **State Machines**: Use explicit state machines for complex state logic

### When to Consider Exceptions
- **Domain-specific algorithms**: Complex but well-understood business logic
- **Legacy integration**: Code that interfaces with complex external systems  
- **Configuration/registration**: Functions that handle many similar cases
- **Performance-critical paths**: Where refactoring would impact performance

### When to Refactor Instead
- **Functions with unclear responsibilities**: Should be split by concern
- **Deeply nested logic**: Usually indicates missing abstractions
- **Repeated patterns**: Can often be extracted into shared utilities
- **Complex conditionals**: May benefit from polymorphism or strategy pattern

## Monitoring and Trends

The complexity checker runs on every CI build and tracks:
- Total number of complex functions
- Trend over time (improvement/regression)
- New complex functions introduced
- Functions that have been successfully refactored

## Tools

- **Complexity Checker**: `tools/complexity-checker/main.go`
- **Manual Check**: `scripts/complexity-checker pkg/mcp/`
- **Baseline Generation**: `scripts/complexity-baseline.sh baseline`
- **Trending**: `scripts/complexity-baseline.sh report`

## Quality Gate Integration

The complexity checker is integrated with the CI quality gates and will:
- ✅ **Pass**: All functions within their respective limits
- ⚠️ **Warn**: Functions approaching their limits (not implemented yet)
- ❌ **Fail**: Functions exceeding their allowed complexity

Current status in CI: **Enforcement enabled** - builds will fail if limits are exceeded.