# Team C Week 3 Execution Plan

## Current Status: 60% Complete → Target: 100% Complete

According to REORG.md, Team C is significantly behind with critical deliverables missing. This plan addresses the remaining 40% of work to achieve full completion.

## Execution Order (Per REORG.md Timeline)

### Week 3 Critical Tasks (High Priority)
Team C must complete these core deliverables that are currently incomplete:

#### 1. **CRITICAL BLOCKER: Fix Interface Validation Errors** 
- **Status**: 7 duplicate interface definitions blocking CI/CD
- **Action**: Remove duplicates from `pkg/mcp/types/interfaces.go`, keep canonical versions in `pkg/mcp/interfaces.go`
- **Priority**: IMMEDIATE - blocks all other teams
- **Files**: `pkg/mcp/types/interfaces.go` cleanup

#### 2. **Complete Unified Pattern Standardization**
- **Status**: Only 60% done, 10 interface validation errors remain
- **Action**: Ensure ALL tools implement unified `mcptypes.Tool` interface
- **Priority**: HIGH - core Team C deliverable
- **Target**: Interface validation shows 0 errors

#### 3. **Sub-package Restructuring** 
- **Status**: NOT STARTED despite claiming complete
- **Action**: Move 31 tool files from `/pkg/mcp/internal/tools/` to domain packages
- **Priority**: HIGH - major deliverable
- **Structure**:
  - `internal/build/`: `build_image.go`, `tag_image.go`, `push_image.go`, `pull_image.go`
  - `internal/deploy/`: `deploy_kubernetes.go`, `generate_manifests.go`, `check_health.go`
  - `internal/scan/`: `scan_image_security.go`, `scan_secrets.go`
  - `internal/analyze/`: `analyze_repository.go`, `validate_dockerfile.go`, `generate_dockerfile.go`
  - `internal/session/`: `list_sessions.go`, `delete_session.go`, etc.
  - `internal/server/`: `get_server_health.go`, `get_telemetry_metrics.go`

#### 4. **Fix Non-functional Fixer Integration**
- **Status**: StubAnalyzer always returns errors
- **Action**: Implement working analyzer integration for conversation mode
- **Priority**: HIGH - functional requirement

### Dependencies & Coordination

**Team A Dependency**: Interface unification must be complete before we can finalize unified patterns
- Team A Status: 95% complete (technical solution achieved)
- **We can proceed** - Team A unblocked CI/CD pipeline

**Team B Coordination**: Package restructuring affects our sub-package work
- Team B Status: 85% complete (core structure done)
- **We can proceed** - domain packages already exist

### Post-Task Validation

After each major task:
1. Run `go build` - must pass
2. Run `go vet ./...` - must pass  
3. Run `go test -short ./...` - must pass (fast local tests)
4. Run interface validation: `go run tools/validate-interfaces/main.go` - must show 0 errors
5. Git commit with descriptive message

### Success Criteria

**Team C 100% Complete When**:
- ✅ Interface validation: 0 errors (currently 7)
- ✅ All tools implement unified patterns consistently
- ✅ All 31 tool files moved to proper domain packages
- ✅ Fixer integration works with real analyzer (not StubAnalyzer)
- ✅ Auto-registration discovers all tools correctly
- ✅ Build/test/vet passes cleanly

### Execution Timeline

**Immediate (Next 2-3 commits)**:
1. Fix duplicate interface definitions (CRITICAL BLOCKER)
2. Complete unified pattern standardization
3. Execute sub-package restructuring

**Next Phase**:
4. Fix fixer integration
5. Final validation and cleanup

This plan addresses the specific gaps identified in REORG.md validation and gets Team C to 100% completion.