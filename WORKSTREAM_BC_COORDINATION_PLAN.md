# Workstream B & C Coordination Plan

## Executive Summary

This document defines the coordination strategy between Workstream B (High-Impact Adapters) and Workstream C (Progress & Operations) to ensure smooth parallel execution with minimal conflicts and maximum efficiency.

## Parallel Execution Timeline

| Day | Workstream B | Workstream C | Coordination Points |
|-----|-------------|-------------|-------------------|
| **1** | Repository Analyzer Adapter | Progress Adapter Analysis | 🔄 Share core interface needs |
| **2** | Auto Registration Adapter | Progress Adapter Replacement | 🔄 Sync on progress interface |
| **3** | Transport Adapter | Operation Wrapper Analysis | 🔄 Build package coordination |
| **4** | Dockerfile + Integration | Operation Wrapper Replacement | 🔄 Joint integration testing |

## Resource Allocation

### **File-Level Conflict Matrix**

| Package | Workstream B Files | Workstream C Files | Conflict Risk |
|---------|-------------------|-------------------|---------------|
| **analyze** | `repository_analyzer_adapter.go`, `dockerfile_adapter.go` | None | ✅ **NONE** |
| **build** | `analyzer_integration.go` | `*operation_wrapper.go`, `*atomic.go` | 🟡 **LOW** |
| **orchestration** | `repository_analyzer_adapter.go` | None | ✅ **NONE** |
| **runtime** | `auto_registration_adapter.go` | `gomcp_progress_adapter.go` | ✅ **NONE** |
| **core** | `transport_adapter.go` | None | ✅ **NONE** |
| **transport** | Transport implementations | None | ✅ **NONE** |
| **observability** | None | `progress.go` (new) | ✅ **NONE** |

**Result**: Only 1 package (`build`) has potential conflicts, and they're in different files.

### **Team Assignment Strategy**

#### **Workstream B Team (2 developers)**
- **Developer B1**: Repository analyzer, transport adapters
- **Developer B2**: Auto registration, dockerfile adapters

#### **Workstream C Team (2 developers)**
- **Developer C1**: Progress adapter consolidation
- **Developer C2**: Operation wrapper consolidation

### **Shared Resources Management**
- **Git Branches**: `workstream-b` and `workstream-c` (parallel development)
- **Core Package**: Workstream A foundation (read-only for B & C)
- **Build Package**: Coordination required (different files)

## Daily Coordination Protocol

### **Daily Standup (15 minutes)**
**Time**: Start of each day
**Participants**: All 4 developers + leads

**Agenda**:
1. **Previous day completion status**
2. **Current day targets**
3. **Blocked dependencies**
4. **Interface changes needed**
5. **Merge conflicts preview**

**Standup Template**:
```
Workstream B:
- Completed: [specific adapter eliminated]
- Today: [target adapter]
- Blockers: [any dependencies on C or core changes]
- Interface needs: [any core interface additions]

Workstream C:
- Completed: [progress/operation milestone]
- Today: [target consolidation]
- Blockers: [any dependencies on B or core changes]
- Interface needs: [any core interface additions]

Coordination:
- Merge readiness: [ready/not ready]
- Integration testing: [schedule]
- Conflict resolution: [action items]
```

### **Mid-Day Sync (5 minutes)**
**Time**: Midday
**Method**: Slack check-in

**Purpose**:
- Quick status update
- Early conflict detection
- Resource reallocation if needed

### **End-of-Day Integration (30 minutes)**
**Time**: End of each day
**Participants**: Technical leads

**Activities**:
1. **Code review** of completed work
2. **Integration test** preparation
3. **Next day planning** adjustment
4. **Conflict resolution** if any

## Interface Change Management

### **Core Interface Additions Protocol**

**If Workstream B needs core interface changes:**
1. **Notify C team immediately**
2. **Create PR against core package**
3. **Wait for C team approval** (impacts their progress interface)
4. **Merge after both teams agree**

**If Workstream C needs core interface changes:**
1. **Notify B team immediately**
2. **Create PR against core package**
3. **Wait for B team approval** (impacts their tool interfaces)
4. **Merge after both teams agree**

### **Example Interface Change Flow**
```
Day 2 - Workstream C adds progress interface method:

1. C1 (Developer): "Need to add GetCurrentStage() to core.ProgressReporter"
2. C1 → B team: "Adding progress method, affects your tools?"
3. B1/B2: "No impact on our adapters, proceed"
4. C1: Creates PR for core interface
5. B team: Reviews and approves
6. C1: Merges after approval
7. Both teams: Update their code accordingly
```

## Build Package Coordination

### **Conflict Prevention Strategy**

Since both workstreams modify the `build` package:

**Workstream B Files**:
- `analyzer_integration.go` (repository analyzer integration)

**Workstream C Files**:
- `*_atomic.go` files (docker operation usage)
- `pull_operation_wrapper.go` (eliminate)
- `push_operation_wrapper.go` (eliminate)
- `tag_operation_wrapper.go` (eliminate)
- `docker_operation.go` (new)

**Prevention Measures**:
1. **File-level separation**: Different files = no merge conflicts
2. **Interface coordination**: Use same core interfaces
3. **Commit frequently**: Small commits prevent big conflicts
4. **Cross-review**: B reviews C's build changes, C reviews B's

### **Build Package Integration Protocol**

**Day 2 Checkpoint**:
- B team: Repository analyzer integration complete
- C team: Progress adapters done, starting operations
- **Action**: Sync on build package interface usage

**Day 3 Checkpoint**:
- B team: Starting transport adapters (no build impact)
- C team: Working on operation wrappers (high build impact)
- **Action**: C team leads build package changes

**Day 4 Checkpoint**:
- B team: Dockerfile adapter (minimal build impact)
- C team: Operation wrapper completion (high build impact)
- **Action**: Joint integration testing

## Merge Strategy

### **Daily Merge Protocol**

**End of Day 1**:
```bash
# Workstream B
git checkout workstream-b
git add -A
git commit -m "Day 1: Repository analyzer adapter eliminated"
git push origin workstream-b

# Workstream C
git checkout workstream-c
git add -A
git commit -m "Day 1: Progress adapter analysis and unified design"
git push origin workstream-c
```

**End of Day 2**:
```bash
# Both workstreams merge to integration branch for testing
git checkout integration-bc
git merge workstream-b
git merge workstream-c
# Resolve any conflicts
# Run integration tests
# If tests pass, both teams proceed
```

**Continue pattern for Days 3-4**

### **Conflict Resolution Hierarchy**

**Level 1 - Developer Level**:
- Developers communicate directly
- Quick fixes and minor conflicts
- File-level coordination

**Level 2 - Team Lead Level**:
- Interface change decisions
- Architecture alignment
- Resource reallocation

**Level 3 - Technical Lead Level**:
- Major architectural conflicts
- Timeline adjustments
- Scope changes

## Integration Testing Strategy

### **Continuous Integration**

**Automated Testing** (after each merge):
```bash
# Build verification
go build -tags mcp ./pkg/mcp/...

# Unit tests
go test -tags mcp ./pkg/mcp/internal/analyze/...
go test -tags mcp ./pkg/mcp/internal/build/...
go test -tags mcp ./pkg/mcp/internal/orchestration/...
go test -tags mcp ./pkg/mcp/internal/runtime/...

# Adapter elimination verification
./scripts/validate_adapter_elimination.sh
```

**Integration Test Milestones**:

**Day 2 Integration**:
- Repository analyzer + progress adapters work together
- Build tools use new progress interface
- No regressions in tool execution

**Day 4 Final Integration**:
- All adapters eliminated
- All consolidations complete
- Full end-to-end tool execution
- Performance benchmarking

### **Integration Test Scenarios**

**Scenario 1: Build Image with Progress**
```bash
# Test that build tools use unified progress reporting
# Validates both B's analyzer and C's progress work together
```

**Scenario 2: Tool Registration and Execution**
```bash
# Test that unified tool registry works
# Validates B's auto-registration elimination
```

**Scenario 3: Docker Operations with Retry**
```bash
# Test that docker operations use unified wrapper
# Validates C's operation wrapper consolidation
```

**Scenario 4: Transport Integration**
```bash
# Test that transport works without adapters
# Validates B's transport adapter elimination
```

## Risk Mitigation

### **High Risk Scenarios**

**Risk 1**: Core interface incompatibility between B & C
- **Probability**: Low
- **Impact**: High
- **Mitigation**: Daily interface sync, early prototyping

**Risk 2**: Build package merge conflicts
- **Probability**: Medium
- **Impact**: Medium
- **Mitigation**: File-level separation, frequent commits

**Risk 3**: Integration test failures
- **Probability**: Medium
- **Impact**: High
- **Mitigation**: Continuous testing, rollback plan

### **Rollback Strategy**

**Per-Day Rollback**:
```bash
# Each day tagged for rollback
git tag workstream-b-day-1-complete
git tag workstream-c-day-1-complete

# Rollback if needed
git reset --hard workstream-b-day-X-complete
```

**Cross-Workstream Rollback**:
- If one workstream blocks the other
- Reset both to last known good state
- Reassess coordination strategy

### **Communication Escalation**

**Level 1**: Developer-to-developer (< 30 minutes)
**Level 2**: Team lead intervention (< 2 hours)
**Level 3**: Technical lead + scope adjustment (< 1 day)

## Success Metrics

### **Daily Success Criteria**

**Day 1**:
- [ ] B: Repository analyzer adapter eliminated
- [ ] C: Unified progress design complete
- [ ] Both: No merge conflicts
- [ ] Integration: Basic build test passes

**Day 2**:
- [ ] B: Auto registration adapter eliminated
- [ ] C: Progress adapters consolidated
- [ ] Both: Core interface alignment maintained
- [ ] Integration: Tool execution tests pass

**Day 3**:
- [ ] B: Transport adapter eliminated
- [ ] C: Operation wrapper analysis complete
- [ ] Both: Build package coordination successful
- [ ] Integration: Docker operations test pass

**Day 4**:
- [ ] B: All 4 adapters eliminated (-646 lines)
- [ ] C: All consolidations complete (-457 lines)
- [ ] Both: Joint integration testing passes
- [ ] Final: No adapter files remain

### **Coordination Success Metrics**

- [ ] **Zero blocking conflicts** between workstreams
- [ ] **Same-day conflict resolution** for any issues
- [ ] **Aligned core interface usage** across both workstreams
- [ ] **Successful joint integration** testing
- [ ] **Combined line reduction** of 1,103 lines

### **Quality Gates**

**After Each Day**:
```bash
# Build success
go build -tags mcp ./pkg/mcp/... # Must pass

# Test success
make test-mcp # Must pass

# No adapter regression
./scripts/validate_no_new_adapters.sh # Must pass

# Performance check
./scripts/benchmark_adapter_elimination.sh # No regression
```

## Communication Tools

### **Slack Channels**
- `#workstream-b-adapters`: B team coordination
- `#workstream-c-operations`: C team coordination
- `#bc-integration`: Cross-workstream coordination
- `#adapter-elimination`: Overall project updates

### **Documentation**
- **Daily progress**: Update in shared doc
- **Interface changes**: Document in core package
- **Decisions log**: Track architectural decisions
- **Conflict resolution**: Document resolution patterns

### **Meetings**
- **Daily standup**: 15 min, all teams
- **Mid-day sync**: 5 min, async Slack
- **End-of-day review**: 30 min, leads only
- **Weekly retrospective**: 1 hour, full team

## Expected Combined Results

### **Line Elimination**
- **Workstream B**: -646 lines (4 adapter files)
- **Workstream C**: -457 lines (net after consolidation)
- **Combined**: -1,103 lines total
- **Percentage**: 75% of total adapter complexity

### **Architectural Improvements**
- **Import cycles resolved**: analyze ↔ build
- **Unified interfaces**: Progress, operations, tools
- **Direct dependencies**: No adapter layers
- **Dependency injection**: Clean separation patterns

### **Development Benefits**
- **Faster tool development**: Unified patterns
- **Easier testing**: Mock core interfaces
- **Better maintainability**: Single implementations
- **Clearer architecture**: Well-defined boundaries

**The coordinated execution of Workstreams B & C will eliminate 75% of adapter complexity while establishing clean architectural patterns for future development!**
