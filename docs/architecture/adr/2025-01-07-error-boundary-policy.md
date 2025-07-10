# ADR-006: Error Handling at Package Boundaries

Date: 2025-01-07
Status: Accepted
Context: Balance between pragmatism and error quality across package boundaries

## Decision
Use RichError for errors that cross package boundaries; allow fmt.Errorf for internal implementation

## Problem
Complete migration from `fmt.Errorf` to RichError throughout the entire codebase would be:
- Time-consuming with 575+ instances to convert
- Potentially over-engineered for simple internal error cases
- Disruptive to ongoing development work

However, errors that cross package boundaries need rich context for:
- Better debugging when errors propagate through layers
- Consistent error handling in API responses
- Structured logging and monitoring
- Clear error categorization for retry logic

## Solution
Implement a **boundary-based error policy** that matches existing CI rules:

### Boundary Functions (MUST use RichError)
Functions that qualify as package boundaries include:
1. **Exported functions** - Any function with a capitalized name
2. **API/Transport packages** - Functions in `/api/`, `/transport/`, `/handler/`, `/server/`, `/rpc/`
3. **Public MCP packages** - Functions in `/mcp/` but not `/internal/`
4. **Error handlers** - Functions handling stdio errors or error responses
5. **Tool interfaces** - Methods like `Execute()`, `Call()`, `Invoke()`

### Internal Functions (MAY use fmt.Errorf)
- Private functions (lowercase names)
- Functions in `/internal/` packages
- Helper functions within a package
- Test code

### Examples

**Boundary Function (Exported) - MUST use RichError:**
```go
// ValidateDockerImage validates a Docker image configuration
func ValidateDockerImage(config DockerConfig) error {
    if config.Image == "" {
        return errors.NewError().
            Code(errors.CodeValidationFailed).
            Type(errors.ErrTypeValidation).
            Message("Docker image name is required").
            Context("config_field", "Image").
            Suggestion("Provide a valid Docker image name").
            WithLocation().
            Build()
    }
    return nil
}
```

**Internal Function - MAY use fmt.Errorf:**
```go
// parseImageTag is an internal helper
func parseImageTag(image string) (name, tag string, err error) {
    parts := strings.Split(image, ":")
    if len(parts) != 2 {
        return "", "", fmt.Errorf("invalid image format: %s", image)
    }
    return parts[0], parts[1], nil
}
```

**Tool Interface - MUST use RichError:**
```go
func (t *BuildTool) Execute(ctx context.Context, req *BuildRequest) (*BuildResponse, error) {
    if err := t.validate(req); err != nil {
        return nil, errors.NewError().
            Code(errors.CodeToolExecutionFailed).
            Type(errors.ErrTypeTool).
            Message("build tool validation failed").
            Cause(err).
            Context("tool", "build").
            WithLocation().
            Build()
    }
    // implementation...
}
```

## Implementation

### CI Enforcement
The existing CI pipeline enforces this policy through:
1. **Pre-commit hooks** - Warns about new `fmt.Errorf` usage (non-blocking)
2. **Rich-audit tool** - Checks boundary compliance in CI (blocking)
3. **Boundary detection** - Automated identification of boundary functions

### Boundary Detection Rules
The `mcp-richify` tool identifies boundaries using these rules:
```go
// From cmd/mcp-richify/boundaries.go
1. ast.IsExported(functionName)        // Exported functions
2. strings.Contains(path, "/api/")      // API packages
3. strings.Contains(path, "/transport/") // Transport layer
4. !strings.Contains(path, "/internal/") // Public packages
5. method names: Execute, Call, Invoke   // Tool interfaces
```

### Migration Strategy
1. **Immediate**: Fix any boundary violations caught by CI
2. **Ongoing**: Convert `fmt.Errorf` to RichError when modifying boundary functions
3. **Optional**: Convert internal `fmt.Errorf` when it improves debugging
4. **Never required**: Test code can continue using `fmt.Errorf`

## Consequences

### Benefits
- **Pragmatic approach** - Focuses effort where it matters most
- **Better API errors** - Rich context for errors crossing boundaries
- **Faster adoption** - Less disruptive than full migration
- **CI enforcement** - Automated checking prevents regression
- **Clear guidelines** - Developers know when to use each pattern

### Trade-offs
- **Mixed patterns** - Both error styles exist in codebase
- **Conversion overhead** - Internal errors may need wrapping at boundaries
- **Learning curve** - Developers must understand boundary concept

### Best Practices
1. **Wrap at boundaries** - Convert internal errors to RichError when returning from exported functions
2. **Preserve context** - Use `.Cause(err)` to maintain error chains
3. **Add value** - Include relevant context when creating RichError
4. **Test both paths** - Ensure error conversion works correctly

## Metrics
- **Boundary compliance**: 100% of boundary functions use RichError
- **Internal usage**: fmt.Errorf allowed, no target for reduction
- **CI pass rate**: Rich-audit checks pass on all PRs
- **Developer velocity**: No significant slowdown from error handling

## Future Considerations
- **Tooling improvements**: Enhanced boundary detection and auto-conversion
- **Gradual migration**: Convert high-value internal errors over time
- **Performance monitoring**: Ensure RichError overhead is acceptable
- **Error analytics**: Leverage rich context for better observability

## References
- CI Configuration: `.github/actions/rich-error-audit/action.yml`
- Boundary Detection: `cmd/mcp-richify/boundaries.go`
- Error Inventory: `scripts/error_inventory.sh`
- RichError Framework: `pkg/mcp/domain/errors/rich.go`
