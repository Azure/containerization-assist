# Day 5: Import Depth Analysis Report

## Summary
Created an import depth checker tool that analyzes package structure. Found significant violations of the 3-level depth limit in the MCP package.

## Statistics
- **Depth 2**: 11 imports (✓ OK)
- **Depth 3**: 42 imports (✓ OK) 
- **Depth 4**: 267 imports (❌ VIOLATION)
- **Depth 5**: 33 imports (❌ VIOLATION)

**Total Violations**: 300 imports exceed the 3-level depth limit

## Major Violation Categories

### 1. Application Layer (depth 4-5)
Most violations come from the application layer's deep nesting:
- `pkg/mcp/application/api` (37 files)
- `pkg/mcp/application/services` (15 files)
- `pkg/mcp/application/logging` (23 files)
- `pkg/mcp/application/commands` (2 files)
- `pkg/mcp/application/orchestration/pipeline` (1 file, depth 5)

### 2. Domain Layer (depth 4-5)
Domain layer also has significant depth issues:
- `pkg/mcp/domain/errors` (100 files - most used!)
- `pkg/mcp/domain/containerization/*` (analyze, build, deploy, scan - all depth 5)
- `pkg/mcp/domain/session` (30 files)
- `pkg/mcp/domain/shared` (21 files)

### 3. Infrastructure Layer (depth 4)
- `pkg/mcp/infra/retry` (1 file)

## Flattening Strategy for Week 2

### Phase 1: Flatten Most Used Packages
1. **domain/errors** → **errors** (saves 1 level for 100 files!)
2. **application/api** → **api** (saves 1 level for 37 files)
3. **domain/session** → **session** (saves 1 level for 30 files)

### Phase 2: Consolidate Application Services
1. Merge related services:
   - `application/services` + `application/state` → `services`
   - `application/commands` → Top level commands in `application`
   - `application/logging` → `logging` (top level)

### Phase 3: Flatten Domain Containerization
1. Change structure from:
   ```
   domain/containerization/analyze
   domain/containerization/build
   domain/containerization/deploy
   domain/containerization/scan
   ```
   To:
   ```
   domain/analyze
   domain/build
   domain/deploy
   domain/scan
   ```

### Phase 4: Eliminate Deep Nesting
1. `application/orchestration/pipeline` → `orchestration`
2. `application/internal/*` → Merge into parent packages
3. `domain/internal/types` → `domain/types`

## Expected Results After Flattening

```
pkg/mcp/
├── api/           # Was: application/api
├── commands/      # Was: application/commands  
├── core/          # Application core (unchanged)
├── domain/        # Domain logic
│   ├── analyze/   # Was: domain/containerization/analyze
│   ├── build/     # Was: domain/containerization/build
│   ├── deploy/    # Was: domain/containerization/deploy
│   ├── scan/      # Was: domain/containerization/scan
│   └── types/     # Was: domain/internal/types
├── errors/        # Was: domain/errors
├── infra/         # Infrastructure (unchanged)
├── logging/       # Was: application/logging
├── orchestration/ # Was: application/orchestration/pipeline
├── services/      # Merged application services
├── session/       # Was: domain/session
└── shared/        # Was: domain/shared
```

## Benefits
1. **All packages at depth ≤ 3**
2. **Simpler import paths**
3. **Clearer architecture**
4. **Easier navigation**
5. **Reduced cognitive load**

## Import Depth Checker Tool

Created at: `scripts/check_import_depth.go`

Usage:
```bash
go run scripts/check_import_depth.go <directory>
```

Features:
- Analyzes all Go files (excluding vendor and tests)
- Reports depth statistics
- Groups violations by import path
- Shows which files use deep imports
- Configurable max depth limit