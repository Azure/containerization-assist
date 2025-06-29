# Adapter Elimination Workstream Division Plan

## Executive Summary

The adapter elimination work can be effectively divided into **4 parallel workstreams** that minimize dependencies and maximize parallel execution. This approach reduces the timeline from 14 days to **8 days** with proper coordination.

## Workstream Division Strategy

### Dependency Analysis

Based on the adapter analysis, here's how we can parallelize the work:

```
Workstream A (Foundation)     →  Workstream B (High-Impact Adapters)
     ↓                            ↓
Workstream C (Progress/Ops)  →  Workstream D (Integration & Validation)
```

**Key Insight**: The core interface package (Workstream A) must complete first, but the other workstreams can work in parallel on different adapter types.

## Workstream Definitions

### **Workstream A: Foundation & Core Interfaces**
**Duration**: 2 days
**Team Size**: 1 senior developer
**Dependency**: Must complete first - enables all other workstreams

**Scope**:
- Create `pkg/mcp/core/interfaces.go` package ✅ (Already started)
- Migrate interface definitions from main package
- Update internal package imports to use core interfaces
- Establish dependency injection foundation

**Deliverables**:
- Single source of truth for all interfaces
- Import cycle elimination framework
- Foundation for adapter removal

**Files Modified**: ~15-20 files (interface definitions and imports)

---

### **Workstream B: High-Impact Adapter Elimination**
**Duration**: 4 days (can start after Workstream A Day 1)
**Team Size**: 2 developers
**Dependencies**: Requires core interfaces from Workstream A

**Scope**:
- Eliminate Repository Analyzer Adapter (357 lines)
- Eliminate Auto Registration Adapter (176 lines)
- Remove Transport Adapter (73 lines)
- Remove Dockerfile Adapter (40 lines)

**Target Files**:
- `pkg/mcp/internal/orchestration/repository_analyzer_adapter.go`
- `pkg/mcp/internal/runtime/auto_registration_adapter.go`
- `pkg/mcp/internal/core/transport_adapter.go`
- `pkg/mcp/internal/analyze/dockerfile_adapter.go`

**Expected Reduction**: -646 lines, -4 adapter files

---

### **Workstream C: Progress & Operation Consolidation**
**Duration**: 4 days (can start after Workstream A Day 1)
**Team Size**: 2 developers
**Dependencies**: Requires core interfaces from Workstream A

**Scope**:
- Consolidate 3 Progress Adapters into single implementation (370 lines)
- Consolidate 3 Operation Wrappers into generic wrapper (287 lines)
- Update all atomic tools to use unified progress interface
- Simplify Docker operation retry logic

**Target Files**:
- `pkg/mcp/types/progress_adapter.go`
- `pkg/mcp/internal/gomcp_progress_adapter.go`
- `pkg/mcp/internal/runtime/gomcp_progress_adapter.go`
- `pkg/mcp/internal/build/pull_operation_wrapper.go`
- `pkg/mcp/internal/build/push_operation_wrapper.go`
- `pkg/mcp/internal/build/tag_operation_wrapper.go`

**Expected Reduction**: -657 lines, -6 files (net ~400 lines after consolidation)

---

### **Workstream D: Integration & Validation**
**Duration**: 2 days (depends on B & C completion)
**Team Size**: 1 developer + QA support
**Dependencies**: Requires completion of Workstreams B & C

**Scope**:
- Integration testing across all changes
- Validation of adapter elimination
- Performance testing and optimization
- Documentation updates
- CI/CD pipeline updates

**Deliverables**:
- Comprehensive test suite validation
- Performance metrics comparison
- Updated architecture documentation
- Zero adapter files confirmed

## Parallel Execution Timeline

| Day | Workstream A | Workstream B | Workstream C | Workstream D |
|-----|-------------|-------------|-------------|-------------|
| **1** | ✅ Core interfaces | *Waiting* | *Waiting* | *Waiting* |
| **2** | Import migration | Repository adapter | Progress analysis | *Waiting* |
| **3** | Complete foundation | Auto-reg adapter | Progress consolidation | *Waiting* |
| **4** | Support others | Transport adapter | Operation wrappers | *Waiting* |
| **5** | Support others | Dockerfile adapter | Tool updates | *Waiting* |
| **6** | Code review | Complete & test | Complete & test | Integration start |
| **7** | Code review | Documentation | Documentation | Testing & validation |
| **8** | Final review | Final review | Final review | Final integration |

## Workstream Coordination Points

### **Daily Standups** (15 min)
- Dependency status updates
- Blocker identification
- Interface change notifications
- Conflict resolution

### **Integration Points**
1. **Day 1**: Workstream A publishes core interface package
2. **Day 3**: Workstreams B & C sync on shared interface usage
3. **Day 5**: All workstreams provide status for integration planning
4. **Day 7**: Final integration and validation

### **Shared Resources**
- **Git Strategy**: Each workstream uses feature branch
  - `workstream-a-foundation`
  - `workstream-b-adapters`
  - `workstream-c-progress`
  - `workstream-d-integration`
- **Communication**: Shared Slack channel for interface changes
- **Documentation**: Shared doc for interface evolution tracking

## Risk Mitigation

### **Interface Change Management**
- Workstream A must notify others of interface changes immediately
- All workstreams use same core interface package
- Version core interface package if breaking changes needed

### **Merge Conflict Prevention**
- Workstreams work on different file sets (minimal overlap)
- Regular rebase from main branch
- Coordinate changes to shared files

### **Quality Gates**
- Each workstream must pass `make test-mcp` before integration
- No workstream proceeds if core interfaces are unstable
- Integration testing before final merge

## Resource Requirements

| Workstream | Developers | Skill Level | Key Skills |
|-----------|------------|-------------|------------|
| **A** | 1 | Senior | Go interfaces, dependency injection |
| **B** | 2 | Mid-Senior | Adapter patterns, refactoring |
| **C** | 2 | Mid-Senior | Progress reporting, Docker ops |
| **D** | 1 + QA | Senior | Integration testing, performance |

**Total Team**: 6 developers + QA support

## Success Metrics Per Workstream

### **Workstream A Success Criteria**
- [ ] Single `pkg/mcp/core/interfaces.go` with all core interfaces
- [ ] Zero import cycles in core package
- [ ] All internal packages import from core (not each other)
- [ ] Foundation supports adapter elimination patterns

### **Workstream B Success Criteria**
- [ ] 4 adapter files eliminated (-646 lines)
- [ ] Repository analysis uses dependency injection
- [ ] Tool registration simplified (no auto-registration adapter)
- [ ] Transport layer uses direct interfaces

### **Workstream C Success Criteria**
- [ ] Single progress implementation (from 3 adapters)
- [ ] Single operation wrapper (from 3 wrappers)
- [ ] All atomic tools use unified progress interface
- [ ] Docker operations have consistent retry logic

### **Workstream D Success Criteria**
- [ ] Zero adapter files in codebase (`find pkg/mcp -name "*adapter*.go" | wc -l = 0`)
- [ ] All tests pass (`make test-mcp`)
- [ ] Build time improved by 15%+
- [ ] Documentation reflects new architecture

## Expected Combined Results

**Total Elimination**: 1,303+ lines of adapter code
**File Reduction**: 10 adapter files → 0
**Timeline Improvement**: 14 days → 8 days
**Architecture Improvement**: Clean dependency injection, zero import cycles

## Next Steps

1. **Assign Workstream Leads**: Identify developers for each workstream
2. **Create Feature Branches**: Set up parallel development branches
3. **Kickoff Meeting**: Align all workstreams on interfaces and dependencies
4. **Start Workstream A**: Begin foundation work immediately
5. **Schedule Daily Standups**: Ensure coordination throughout execution

This parallel approach maximizes development velocity while maintaining code quality and architectural integrity.
