# Team B: Package Restructuring Plan

## Overview
Team B is responsible for package restructuring and module boundaries as part of the MCP Consolidated Reorganization Plan. Our goal is to flatten the directory structure from 62 directories to 15 focused packages, reducing the codebase by 75%.

## Timeline
- **Duration**: 2 weeks (Weeks 2-3)
- **Dependencies**: Team A must complete interface unification first
- **Blocks**: None (Team C works in parallel on tools)

## Week 2 Tasks: Execute Restructuring

### Priority Task 1: Implement Final Package Structure
Target structure (flattened single-module):
```
pkg/mcp/
├── go.mod                 # Single module for entire MCP system
├── mcp.go                 # Public API
├── interfaces.go          # Unified interfaces (from Team A)
├── internal/
│   ├── runtime/          # Core server (was engine/)
│   ├── build/            # Build tools (flattened - no extra "tools" level)
│   ├── deploy/           # Deploy tools (flattened)
│   ├── scan/             # Security tools (flattened)
│   ├── analyze/          # Analysis tools (flattened)
│   ├── session/          # Unified session management
│   ├── transport/        # Transport implementations
│   ├── workflow/         # Orchestration (simplified)
│   ├── observability/    # Logging, metrics, tracing (early creation)
│   └── validate/         # Shared validation (exported)
```

### Priority Task 2: Consolidate Session Management
- **Current**: 3 packages (`internal/store/session/`, `internal/types/session/`, etc.)
- **Target**: Single `internal/session/` package
- **Actions**: Move all session-related code to unified location

### Priority Task 3: Create Observability Package Early
- **Purpose**: Prevent "shared" from becoming junk drawer
- **Contents**: Logging, metrics, tracing from scattered locations
- **Benefit**: Makes cross-cutting concerns discoverable

## Week 3 Tasks: Import Path Updates & Cleanup

### Priority Task 1: Update All Import Paths
- Update import statements across entire codebase
- Use automated tooling where possible
- Validate all imports resolve correctly

### Priority Task 2: Remove Empty Directories
- **Target**: Reduce from 62 to 15 directories
- **Method**: Systematic removal after file migration
- **Validation**: Ensure no orphaned files

### Priority Task 3: Validate Package Boundaries
- Implement automated boundary checks
- Ensure clean module boundaries
- Prevent circular dependencies

### Priority Task 4: Remove Duplicate Files
- **Target**: Remove duplicate `types.go` and `common.go` files
- **Current**: 7 each → 1 each
- **Method**: Consolidate into single authoritative versions

## Execution Strategy

### Dependencies
- **BLOCKED ON**: Team A completion of interface unification
- **WAITING FOR**: `pkg/mcp/interfaces.go` from Team A
- **COORDINATION**: Daily standups with Team A for handoff timing

### Risk Mitigation
- **High Risk**: Mass file movement
- **Mitigation**: Git history preservation + automated import updates
- **Validation**: Automated testing at each step
- **Rollback**: Daily snapshots with full rollback capability

### Quality Gates
- All imports resolve correctly
- No circular dependencies
- Package boundaries validated
- Build/test/vet passes cleanly
- Performance regression < 5%

## Success Metrics
- **Directory Reduction**: 62 → 15 directories (-75%)
- **Import Path Simplification**: Shorter, cleaner paths
- **Build Time Improvement**: Target -20%
- **Developer Experience**: Better IDE support, faster navigation

## Deliverables
1. Flattened package structure implemented
2. All import paths updated
3. Empty directories removed
4. Package boundaries validated
5. Automated checks in place
6. Clean build/test/vet results
7. Git commit with all changes

## Notes
- This plan assumes Team A completes interface unification on schedule
- Coordination with Team D for automation tooling
- Team C works in parallel on tool system rewrite
- Final validation before Week 3 completion