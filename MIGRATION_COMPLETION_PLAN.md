# Container Kit Migration Completion Plan

## Overview
This plan outlines the remaining steps to complete the three-layer architecture migration for Container Kit. Current progress: **75-80% complete**.

## Phase 1: Fix Critical Build Issues (Day 1)
### 1.1 Fix Import Path Misalignment
- [ ] Update `pkg/mcp/core/conversation/handler.go` import from `services` to `application/services`
- [ ] Fix duplicate function declarations in commands package:
  - [ ] Remove duplicate `ValidationError` type
  - [ ] Remove duplicate `contains`, `getStringParam`, `getIntParam`, `getBoolParam` functions
  - [ ] Create `pkg/mcp/application/commands/common.go` for shared utilities

### 1.2 Resolve Session Package Issues
- [ ] Complete migration from `core.SessionState` to `domain/session.SessionState`
- [ ] Remove legacy session manager interfaces
- [ ] Update all references to use domain session types

## Phase 2: Complete Core Migration (Days 2-3)
### 2.1 Move Conversation Package
- [ ] Move `pkg/mcp/core/conversation/` → `pkg/mcp/application/conversation/`
- [ ] Update all import paths
- [ ] Ensure no circular dependencies

### 2.2 Migrate State Management
- [ ] Move `pkg/mcp/core/state/` → `pkg/mcp/application/state/`
- [ ] Separate domain state from application state logic
- [ ] Update dependent packages

### 2.3 Remove Core Package
- [ ] Migrate remaining files from `pkg/mcp/core/`:
  - Config types → `application/config/`
  - Interfaces → Remove (use `application/api/interfaces.go`)
  - Server types → `application/core/`
- [ ] Delete empty `pkg/mcp/core/` directory

## Phase 3: Eliminate Internal Package (Days 4-5)
### 3.1 Distribute Internal Components
```
pkg/mcp/internal/ → Target locations:
├── config/        → application/config/
├── runtime/       → application/runtime/
├── pipeline/      → application/orchestration/pipeline/
├── server/        → application/core/server/
├── types/         → domain/types/ or application/types/
└── utils/         → infra/utils/ or cross-cutting/
```

### 3.2 Update Import Paths
- [ ] Use automated script to update all import paths
- [ ] Verify no broken imports

## Phase 4: Clean Up Legacy Code (Days 6-7)
### 4.1 Remove Duplicate Packages
- [ ] Delete `pkg/mcp/api/` (replaced by `application/api/`)
- [ ] Delete `pkg/mcp/session/` (use `domain/session/`)
- [ ] Delete `pkg/mcp/templates/` (use `infra/templates/`)
- [ ] Remove empty tool subdirectories in `application/tools/`

### 4.2 Consolidate Security Package
- [ ] Move security domain logic → `domain/security/`
- [ ] Move security infrastructure → `infra/security/`
- [ ] Update security validation to use new paths

## Phase 5: Final Architecture Validation (Day 8)
### 5.1 Verify Three-Layer Compliance
```
✓ Domain Layer (pkg/mcp/domain/)
  - Pure business logic
  - No external dependencies
  - Domain entities and value objects

✓ Application Layer (pkg/mcp/application/)
  - Use cases and orchestration
  - Service interfaces (DI)
  - DTOs and API types

✓ Infrastructure Layer (pkg/mcp/infra/)
  - External integrations
  - Transport protocols
  - Persistence implementations
```

### 5.2 Dependency Direction Validation
- [ ] Ensure dependencies flow: Infrastructure → Application → Domain
- [ ] No reverse dependencies
- [ ] No circular dependencies

## Phase 6: Testing and Documentation (Days 9-10)
### 6.1 Update Tests
- [ ] Fix broken tests due to import changes
- [ ] Add integration tests for new structure
- [ ] Ensure >80% test coverage

### 6.2 Update Documentation
- [ ] Update architecture diagrams
- [ ] Update README with new structure
- [ ] Document migration decisions in ADRs
- [ ] Update API documentation

## Implementation Commands

### Phase 1 Commands
```bash
# Fix import paths
find . -name "*.go" -exec sed -i 's|github.com/Azure/container-kit/pkg/mcp/services|github.com/Azure/container-kit/pkg/mcp/application/services|g' {} +

# Create common utilities file
cat > pkg/mcp/application/commands/common.go << 'EOF'
package commands

// Common utilities for all commands
// (Move shared functions here)
EOF
```

### Phase 2 Commands
```bash
# Move conversation package
mkdir -p pkg/mcp/application/conversation
mv pkg/mcp/core/conversation/* pkg/mcp/application/conversation/
find . -name "*.go" -exec sed -i 's|pkg/mcp/core/conversation|pkg/mcp/application/conversation|g' {} +
```

### Phase 3 Commands
```bash
# Distribute internal packages
mv pkg/mcp/internal/config/* pkg/mcp/application/config/
mv pkg/mcp/internal/runtime/* pkg/mcp/application/runtime/
mv pkg/mcp/internal/pipeline/* pkg/mcp/application/orchestration/pipeline/
```

## Success Criteria
1. ✅ All packages follow three-layer architecture
2. ✅ No circular dependencies
3. ✅ Build completes successfully: `go build ./...`
4. ✅ All tests pass: `make test-all`
5. ✅ No legacy packages remain
6. ✅ Clear separation of concerns

## Risk Mitigation
1. **Import Cycles**: Use `go list -f '{{.ImportPath}} -> {{join .Imports " "}}' ./...` to detect
2. **Breaking Changes**: Run tests after each phase
3. **Missing Functionality**: Keep detailed logs of all moves
4. **Performance Impact**: Benchmark before and after migration

## Timeline
- **Total Duration**: 10 working days
- **Critical Path**: Days 1-3 (fixing build and core migration)
- **Buffer**: 2 days for unexpected issues

## Next Steps
1. Start with Phase 1.1 - Fix import path issues
2. Run build after each step to ensure no regressions
3. Commit after each successful phase