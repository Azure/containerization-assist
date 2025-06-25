# Next Steps for MCP Reorganization

## Overview
The MCP reorganization has achieved 90% completion with all core objectives met. This document outlines the remaining cleanup tasks and recommendations for completing the reorganization and improving the codebase.

## Immediate Priorities (1-2 weeks)

### 1. Interface Cleanup (Team A & C)
**Current**: 5 interface validation errors remain  
**Target**: 0 errors

**Actions**:
- Apply consistent "Internal" prefix strategy across all remaining interfaces
- Update tools that still reference old interface types
- Remove or consolidate duplicate interface definitions
- Run `go run tools/validate-interfaces/main.go` until it passes

**Files to update**:
- Review `pkg/mcp/types/interfaces.go` for remaining duplicates
- Check tool implementations in domain packages for interface compliance

### 2. Directory Structure Flattening (Team B)
**Current**: 58 directories  
**Target**: ~15 directories

**Actions**:
```bash
# Identify deeply nested directories
find pkg/mcp/internal -type d | awk -F/ '{print NF-1, $0}' | sort -n

# Flatten session/session/ structure
mv pkg/mcp/internal/session/session/* pkg/mcp/internal/session/
rmdir pkg/mcp/internal/session/session

# Remove empty intermediate directories
find pkg/mcp/internal -type d -empty -delete
```

### 3. Error Handling Standardization (Team C)
**Current**: 28% adoption (237 proper types vs 619 fmt.Errorf)  
**Target**: 80% adoption

**Priority files** (highest fmt.Errorf usage):
1. `pkg/mcp/internal/validate/health_validator.go`
2. `pkg/mcp/internal/workflow/stage_*.go` files
3. `pkg/mcp/internal/build/` tool implementations

**Pattern to follow**:
```go
// Replace:
return fmt.Errorf("failed to build image: %w", err)

// With:
return types.NewRichError("BUILD001", "failed to build image").
    WithError(err).
    WithContext("dockerfile", dockerfilePath)
```

## Code Quality Improvements (2-4 weeks)

### 1. Naming Conventions

**Tool Naming**:
- Atomic tools should consistently use `_atomic` suffix
- Consider removing redundant prefixes (e.g., `BuildImageTool` → `ImageBuilder`)

**Package Names**:
- Ensure singular form for packages (e.g., `build` not `builds`)
- Remove redundant package prefixes in type names

### 2. Documentation Updates

**Required Documentation**:

1. **Architecture Diagram** (`docs/mcp-architecture.md`)
   - Show new package structure
   - Illustrate auto-registration flow
   - Document interface relationships

2. **Tool Development Guide** (`docs/adding-new-tools.md`)
   ```markdown
   ## Adding a New MCP Tool
   
   1. Choose appropriate domain package:
      - `/build` - Image building operations
      - `/deploy` - Deployment operations
      - `/scan` - Security scanning
      - `/analyze` - Code analysis
   
   2. Implement the Tool interface:
      ```go
      type MyTool struct {
          // fields
      }
      
      func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error)
      func (t *MyTool) GetMetadata() mcptypes.ToolMetadata
      func (t *MyTool) Validate(ctx context.Context, args interface{}) error
      ```
   
   3. Register with auto-generation:
      - Add to appropriate `tools.go` file
      - Run `go generate ./...`
   ```

3. **Migration Guide** (`docs/mcp-migration.md`)
   - For users of the old adapter pattern
   - Interface change documentation
   - Breaking changes and how to update

### 3. Comment Standardization

**Package Comments**:
```go
// Package build provides tools for Docker image building operations.
// It includes atomic tools for building, tagging, and pushing images,
// with automatic error recovery through the fixer integration.
package build
```

**Interface Comments**:
```go
// Tool represents a single MCP tool that can be executed via the protocol.
// All tools must implement this interface to be registered with the server.
type Tool interface {
    // Execute runs the tool with the provided arguments.
    // It returns the tool's output or an error if execution fails.
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    
    // GetMetadata returns descriptive information about the tool.
    GetMetadata() ToolMetadata
    
    // Validate checks if the provided arguments are valid for this tool.
    Validate(ctx context.Context, args interface{}) error
}
```

## Testing Improvements

### 1. Fix Hanging Tests
**Immediate**:
- Fix `TestServerTransportError` and `TestServerCleanupOnFailure`
- Add proper timeout handling in server lifecycle
- Consider using context with timeout for server.Start()

### 2. Integration Test Coverage
**Add tests for**:
- Auto-registration system
- Cross-package tool interactions
- Error recovery (fixer) mechanisms

### 3. Validation Tool Tests
**Ensure validation tools have tests**:
- Interface validation accuracy
- Package boundary detection
- Auto-registration discovery

## Performance Optimizations

### 1. Tool Registration
- Consider lazy loading for rarely used tools
- Implement tool caching where appropriate

### 2. Session Management
- Review cleanup routine performance
- Optimize BoltDB operations

## Long-term Considerations (1-3 months)

### 1. Interface Evolution
- Plan for v2 interfaces if breaking changes needed
- Consider interface versioning strategy

### 2. Plugin Architecture
- Evaluate if tools should be loadable as plugins
- Design plugin interface if needed

### 3. Observability
- Enhance telemetry with tool-specific metrics
- Add distributed tracing support

## Success Metrics

Track these metrics to measure completion:

1. **Code Quality**
   - Interface validation errors: 0
   - Package boundary violations: 0
   - Error handling adoption: >80%
   - Test coverage: >60%

2. **Architecture**
   - Directory count: <20
   - Circular dependencies: 0
   - Auto-registration coverage: 100%

3. **Developer Experience**
   - Time to add new tool: <30 minutes
   - Build time: <2 minutes
   - Test execution time: <5 minutes

## Conclusion

The MCP reorganization has successfully achieved its primary goals of:
- ✅ Unified interfaces
- ✅ Clean package boundaries
- ✅ Auto-registration system
- ✅ Proper domain organization

The remaining 10% consists of cleanup tasks that will improve code quality and developer experience but are not blocking for functionality. Teams should prioritize based on their current work while gradually completing these improvements.