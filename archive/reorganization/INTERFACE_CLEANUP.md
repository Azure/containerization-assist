# Tool Interface Cleanup Documentation

## Overview
This document describes the interface cleanup performed as part of Workstream 3, Task 4.

## Changes Made

### 1. Created Simplified Interface Structure
- Created `interfaces_simplified.go` with cleaner, consolidated interfaces
- Removed generic complexity from ExecutableTool
- Consolidated duplicate session-related structures

### 2. Interface Consolidations

#### SimpleTool (replaces ExecutableTool[TArgs, TResult])
- Removed generic type parameters for simplicity
- Uses `interface{}` for args/results with type assertions
- Maintains same functionality with less complexity

#### PipelineOperations (replaces PipelineAdapter)
- Clearer naming convention
- Grouped operations by category (Repository, Docker, Kubernetes, Session)
- Removed context management methods (not widely used)

#### SessionOperations (replaces SessionManager)
- Clearer naming
- Combined with SessionData structure

#### SessionData (consolidates Session + SessionState)
- Single structure instead of nested composition
- Clearer field organization by category
- Simplified security scan structure

### 3. Removed Unnecessary Complexity

#### Removed Unused Interfaces
- LongRunningTool (no implementations found)
- Removed context management from PipelineAdapter

#### Simplified Data Structures
- RepositoryInfo (replaces RepositoryScanSummary)
- FileStructure for clearer file organization
- VulnerabilityCount with explicit fields

### 4. Backward Compatibility
- Added type aliases for smooth migration
- Existing code continues to work
- Can be removed after full migration

## Migration Plan

### Phase 1: Introduction (Current)
- New simplified interfaces created alongside existing ones
- Type aliases provide backward compatibility

### Phase 2: Gradual Migration
1. Update atomic tools to use simplified interfaces
2. Update core systems (registry, adapters)
3. Remove legacy adapter patterns

### Phase 3: Cleanup
1. Remove type aliases
2. Delete original interfaces.go
3. Rename interfaces_simplified.go to interfaces.go

## Benefits

1. **Reduced Complexity**: Removed unnecessary generic types
2. **Better Organization**: Interfaces grouped by functionality
3. **Clearer Naming**: Operations-based naming convention
4. **Simplified Structures**: Flattened nested structures
5. **Maintainability**: Easier to understand and modify

## Code Examples

### Before (Complex Generic)
```go
type ExecutableTool[TArgs, TResult any] interface {
    Tool
    Execute(ctx context.Context, args TArgs) (*TResult, error)
    PreValidate(ctx context.Context, args TArgs) error
}
```

### After (Simple Interface)
```go
type SimpleTool interface {
    GetName() string
    GetDescription() string
    GetVersion() string
    GetCapabilities() ToolCapabilities
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    Validate(ctx context.Context, args interface{}) error
}
```

## Next Steps

1. Begin migrating tools to use new interfaces
2. Update registry to support SimpleTool
3. Gradually phase out legacy patterns
4. Complete migration in next sprint