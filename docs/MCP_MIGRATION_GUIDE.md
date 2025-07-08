# MCP Package Structure Migration Guide

## Overview

Container Kit MCP has undergone a major architectural simplification, reducing from 86 packages to 10 focused packages with clear boundaries and shallow import paths. This guide helps you migrate existing code to the new structure.

## Key Changes

### Package Count Reduction
- **Before**: 86 packages with deep nesting
- **After**: 10 top-level packages (27 total including subdirectories)
- **Benefit**: 69% reduction in complexity

### Import Depth
- **Before**: Up to 5 levels deep (`pkg/mcp/domain/containerization/build/strategies`)
- **After**: Maximum 3 levels (`pkg/mcp/tools/build`)
- **Benefit**: Faster builds, easier navigation

### Distributed Complexity Removal
- Removed ~1,800 lines of over-engineered distributed system code
- Eliminated inappropriate features: distributed caching, auto-scaling, complex recovery
- Focused on single-node container tool requirements

## Import Path Changes

### Core Mappings

| Old Import Path | New Import Path |
|----------------|-----------------|
| `pkg/mcp/application/api` | `pkg/mcp/api` |
| `pkg/mcp/application/core` | `pkg/mcp/core` |
| `pkg/mcp/domain/containerization/analyze` | `pkg/mcp/tools/analyze` |
| `pkg/mcp/domain/containerization/build` | `pkg/mcp/tools/build` |
| `pkg/mcp/domain/containerization/deploy` | `pkg/mcp/tools/deploy` |
| `pkg/mcp/domain/containerization/scan` | `pkg/mcp/tools/scan` |
| `pkg/mcp/domain/session` | `pkg/mcp/session` |
| `pkg/mcp/application/orchestration/workflow` | `pkg/mcp/workflow` |
| `pkg/mcp/infra/transport` | `pkg/mcp/transport` |
| `pkg/mcp/infra/persistence` | `pkg/mcp/storage` |
| `pkg/mcp/infra/templates` | `pkg/mcp/templates` |
| `pkg/mcp/domain/security` | `pkg/mcp/security` |
| `pkg/mcp/domain/validation` | `pkg/mcp/security/validation` |
| `pkg/mcp/domain/errors` | `pkg/mcp/internal/errors` |
| `pkg/mcp/domain/types` | `pkg/mcp/internal/types` |

### Registry Consolidation

Multiple registry packages consolidated:
```go
// Old
import "github.com/Azure/container-kit/pkg/mcp/app/registry"
import "github.com/Azure/container-kit/pkg/mcp/application/orchestration/registry"
import "github.com/Azure/container-kit/pkg/mcp/services/registry"

// New
import "github.com/Azure/container-kit/pkg/mcp/core/registry"
```

### Service Imports

Services distributed by function:
```go
// Old
import "github.com/Azure/container-kit/pkg/mcp/services/session"
import "github.com/Azure/container-kit/pkg/mcp/services/workflow"
import "github.com/Azure/container-kit/pkg/mcp/services/validation"

// New
import "github.com/Azure/container-kit/pkg/mcp/session"
import "github.com/Azure/container-kit/pkg/mcp/workflow"
import "github.com/Azure/container-kit/pkg/mcp/security/validation"
```

## API Changes

### Removed Features

The following over-engineered features have been removed:

1. **Distributed Caching** (`DistributedCacheManager`)
   - Replace with simple in-memory caching if needed
   - Most operations don't need caching

2. **Auto-scaling** (`AutoScaler`)
   - Not applicable for container build operations
   - Remove all auto-scaling configuration

3. **Complex Recovery** (`RecoveryManager`, `ErrorRecoveryPromptBuilder`)
   - Use simple error handling with retry
   - Leverage unified error system

4. **Performance Monitoring** (`PerformanceMonitor`, `BuildMonitor`)
   - Premature optimization removed
   - Add metrics only when proven necessary

### Interface Location

All interfaces now in `pkg/mcp/api/interfaces.go`:
```go
// Old: Scattered across packages
import "github.com/Azure/container-kit/pkg/mcp/application/api"
import "github.com/Azure/container-kit/pkg/mcp/domain/tools"

// New: Single source of truth
import "github.com/Azure/container-kit/pkg/mcp/api"
```

## Migration Steps

### 1. Update Import Paths

Use sed or your IDE to update imports:
```bash
# Example for Linux/Mac
find . -name "*.go" -exec sed -i 's|pkg/mcp/application/api|pkg/mcp/api|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/domain/containerization|pkg/mcp/tools|g' {} \;
# ... repeat for other mappings
```

### 2. Remove Distributed Features

Search and remove usage of:
- `DistributedCacheManager`
- `DistributedOperationManager`
- `AutoScaler`
- `PerformanceMonitor`
- `RecoveryManager`

### 3. Update Registry Usage

```go
// Old: Multiple registry types
registry := orchestration.NewFederatedRegistry()
registry := app.NewTypedToolRegistry()

// New: Single registry
registry := core.NewRegistry()
```

### 4. Fix Package References

Update any hardcoded paths in tests:
```go
// Old
content, _ := ioutil.ReadFile("domain/containerization/build/Dockerfile")

// New
content, _ := ioutil.ReadFile("tools/build/Dockerfile")
```

### 5. Validate Boundaries

Run boundary checker:
```bash
tools/check-boundaries/check-boundaries -strict ./pkg/mcp
```

## Common Issues and Solutions

### Import Cycles

**Problem**: `import cycle not allowed`

**Solution**:
- Check if you're importing implementation instead of interface
- Use interfaces from `pkg/mcp/api`
- Move shared types to `pkg/mcp/internal/types`

### Missing Types

**Problem**: Type not found after migration

**Solution**:
- Error types: `pkg/mcp/internal/errors`
- Common types: `pkg/mcp/internal/types`
- Domain types: Check specific tool package

### Build Failures

**Problem**: Package not found

**Solution**:
1. Verify new import path from mapping table
2. Check if feature was removed (distributed features)
3. Run `go mod tidy` to clean up dependencies

## Validation Checklist

After migration:
- [ ] All imports updated to new paths
- [ ] No references to removed distributed features
- [ ] Build succeeds: `go build ./pkg/mcp/...`
- [ ] Tests pass: `make test-mcp`
- [ ] No boundary violations: `tools/check-boundaries/check-boundaries -strict ./pkg/mcp`
- [ ] No deep imports (>3 levels)

## Benefits After Migration

1. **Faster Builds**: Simplified dependency graph
2. **Easier Navigation**: Maximum 3-level imports
3. **Clear Architecture**: Enforced boundaries
4. **Reduced Complexity**: No over-engineering
5. **Better Maintainability**: Focused packages

## Getting Help

- Architecture documentation: [ARCHITECTURE.md](./ARCHITECTURE.md)
- Package guide: [MCP_PACKAGE_GUIDE.md](./MCP_PACKAGE_GUIDE.md)
- Boundary rules: [check-boundaries](../tools/check-boundaries/)

## Example Migration

### Before
```go
package myservice

import (
    "github.com/Azure/container-kit/pkg/mcp/application/api"
    "github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
    "github.com/Azure/container-kit/pkg/mcp/infra/persistence/boltdb"
    "github.com/Azure/container-kit/pkg/mcp/services/registry"
)

func BuildImage() {
    registry := registry.NewFederatedRegistry()
    cache := build.NewDistributedCacheManager()
    monitor := build.NewPerformanceMonitor()
    // ...
}
```

### After
```go
package myservice

import (
    "github.com/Azure/container-kit/pkg/mcp/api"
    "github.com/Azure/container-kit/pkg/mcp/tools/build"
    "github.com/Azure/container-kit/pkg/mcp/storage/boltdb"
    "github.com/Azure/container-kit/pkg/mcp/core/registry"
)

func BuildImage() {
    registry := registry.New()
    // Removed: distributed cache and performance monitor
    // ...
}
```
