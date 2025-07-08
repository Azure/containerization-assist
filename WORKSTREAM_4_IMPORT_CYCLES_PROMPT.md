# Workstream 4: Fix Import Cycles and Package Dependencies

## Objective
Resolve import cycle issues and package dependency problems identified in the pre-commit checks. These are issues where packages have circular dependencies or incorrect import paths.

## Scope
Focus exclusively on fixing import cycles, missing imports, and package structure issues without changing core functionality.

## Affected Areas and Issues

### 1. Import Cycle Issues

**Packages with potential import cycles**:
- `pkg/mcp/server` importing `pkg/mcp/tools/analyze` 
- Various tools packages cross-referencing each other
- Core packages importing from internal packages that import back

### 2. Missing Import Statements

**Files with compilation errors due to missing imports**:
- Various files missing imports for `time`, `context`, `fmt`, etc.
- Missing imports for custom types and packages

### 3. Incorrect Import Paths

**Issues with import path resolution**:
- Some files importing packages that have been moved or renamed
- Relative import issues in test files
- Package alias conflicts

### 4. Package Structure Issues

**Files that may need to be moved or restructured**:
- Types defined in wrong packages causing import cycles
- Interfaces defined in packages that create circular dependencies
- Shared types that should be in a common package

## Common Import Cycle Patterns to Fix

### 1. Server → Tools → Core → Server Cycle
```
pkg/mcp/server → pkg/mcp/tools/analyze → pkg/mcp/core → pkg/mcp/server
```

### 2. Tools Cross-Dependencies
```
pkg/mcp/tools/analyze → pkg/mcp/tools/build → pkg/mcp/tools/analyze
```

### 3. Core → Internal → Core Cycle
```
pkg/mcp/core → pkg/mcp/internal/something → pkg/mcp/core
```

## Instructions

1. **Identify Import Cycles**:
   - Use `go mod graph` and tools to identify circular dependencies
   - Map out the dependency graph for affected packages
   - Document which imports are causing cycles

2. **Break Import Cycles**:
   - Move shared types to common packages
   - Use dependency inversion (interfaces in consumer packages)
   - Extract common interfaces to separate packages
   - Move concrete implementations away from interface definitions

3. **Fix Missing Imports**:
   - Add missing standard library imports
   - Add missing custom package imports
   - Ensure all used types and functions are properly imported

4. **Correct Import Paths**:
   - Update import paths for moved packages
   - Fix relative import issues
   - Resolve package alias conflicts

5. **Restructure if Necessary**:
   - Move types to appropriate packages to break cycles
   - Create common/shared packages for widely used types
   - Ensure package boundaries respect the dependency hierarchy

## Success Criteria
- All import cycle errors are resolved
- All missing import errors are fixed
- All packages compile without import-related errors
- Package structure follows clean architecture principles
- No new import cycles are introduced

## Example Patterns

### Breaking Import Cycles with Interfaces
```go
// Instead of concrete dependency causing cycle
// core/types.go - move interface here
type Analyzer interface {
    Analyze(path string) (*Result, error)
}

// tools/analyze/analyzer.go - implement interface
type concreteAnalyzer struct {}
func (a *concreteAnalyzer) Analyze(path string) (*Result, error) { ... }

// server/core.go - depend on interface, not concrete type
import "github.com/Azure/container-kit/pkg/mcp/core"
func NewServer(analyzer core.Analyzer) *Server { ... }
```

### Moving Shared Types
```go
// Move from tools/analyze/types.go to core/types.go
type AnalysisResult struct {
    // Common fields used by multiple packages
}

// Update imports in all consuming packages
import "github.com/Azure/container-kit/pkg/mcp/core"
// Use core.AnalysisResult instead of local type
```

### Dependency Injection Pattern
```go
// Instead of importing concrete implementations
// Define interfaces in the consuming package
type ToolRegistry interface {
    Register(tool Tool) error
    Get(name string) Tool
}

// Let concrete implementations be injected
func NewServer(registry ToolRegistry) *Server {
    return &Server{registry: registry}
}
```

## Tools for Analysis
```bash
# Identify import cycles
go mod graph | grep cycle

# Build specific packages to see import errors
go build ./pkg/mcp/server/...
go build ./pkg/mcp/tools/...

# Use go list to analyze dependencies
go list -deps ./pkg/mcp/server/...
```