# Team A: Interface Unification Plan

## Overview
Team A is responsible for replacing all 11 interface files with a single unified interface pattern across the MCP codebase. This is the foundation that enables Teams B and C to proceed with their restructuring work.

## Timeline: 2 Weeks

### Week 1: Create & Implement New Interfaces
**Goal**: Establish unified interface pattern and begin migration

#### Task 1: Create Unified Interface File
- **File**: `pkg/mcp/interfaces.go`
- **Content**: Single source of truth with 4 core interfaces:
  - `Tool` interface (Execute, GetMetadata, Validate methods)
  - `Session` interface (ID, GetWorkspace, UpdateState methods)
  - `Transport` interface (Serve, Stop methods)
  - `Orchestrator` interface (ExecuteTool, RegisterTool methods)

#### Task 2: Update Tool Implementations
- Convert all atomic tools to new unified `Tool` interface
- Target tools: AtomicBuildImageTool, AtomicDeployKubernetesTool, etc.
- Remove dependencies on old interface files
- Standardize method signatures across all tools

#### Task 3: Update Orchestration Components
- Convert `MCPToolOrchestrator` to new `Orchestrator` interface
- Remove adapter layers entirely
- Implement direct tool registration pattern

### Week 2: Complete Migration
**Goal**: Remove all old interfaces and complete codebase migration

#### Task 4: Update Remaining Packages
- Migrate all remaining packages to unified interfaces
- Ensure consistent interface usage across entire codebase

#### Task 5: Remove Old Interface Files
- Delete all 11 legacy interface files:
  - `dispatch/interfaces.go`
  - `tools/interfaces.go`
  - `base/atomic_tool.go`
  - 8 additional interface files
- Clean up unused interface definitions

#### Task 6: Update Import Statements
- Update all import statements across codebase
- Replace old interface imports with unified interface imports
- Ensure no broken imports remain

#### Task 7: Add Interface Conformance Tests
- Create tests to validate interface implementations
- Ensure all tools properly implement unified interfaces
- Add automated validation for future interface compliance

## Dependencies
- **None** - Team A can start immediately
- **Blocks**: Team B (Package Restructuring), Team C (Tool System Rewrite)

## Risk Level: Medium
- Requires coordinated updates across entire codebase
- Must maintain functionality while changing interfaces
- Critical path for other teams

## Success Criteria
- Single `interfaces.go` file replaces all 11 interface files
- All tools implement unified `Tool` interface
- Zero old interface file references remain
- All tests pass with new interface pattern
- Codebase ready for Teams B & C to proceed

## Quality Gates
- `go build ./...` - All packages compile successfully
- `go test ./...` - All existing tests pass
- `go vet ./...` - No static analysis issues
- Interface conformance tests pass
- No circular dependencies introduced

## Execution Order
This plan follows the execution timeline from REORG.md:
- **Week 1**: Focus on creating unified interfaces and tool implementation updates
- **Week 2**: Complete migration and cleanup of old interfaces
- Teams B & C can begin their work once Week 1 deliverables are complete