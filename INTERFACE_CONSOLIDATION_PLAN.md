# Interface Consolidation Implementation Plan

## Current Situation Analysis

We have 16 duplicate interface definitions across the codebase that need consolidation. These duplicates violate the "single source of truth" principle and can lead to maintenance issues and confusion.

## Duplicate Interface Categories

### 1. **HIGH RISK - Core Architecture Interfaces (5 interfaces)**
These are fundamental to the system architecture and require careful handling:

- **ServiceContainer** (services vs state packages)
  - `pkg/mcp/application/services/interfaces.go` - Full interface (10+ methods)
  - `pkg/mcp/application/state/integration.go` - Minimal interface (2 methods)
  - **Risk**: Circular dependency concerns, widely used

- **ToolRegistry** (core vs services packages)
  - `pkg/mcp/application/core/tool_registry.go` - Implementation-focused
  - `pkg/mcp/application/services/interfaces.go` - Service-focused
  - **Risk**: Core infrastructure, used by many components

- **SessionState** / **SessionStore** (services vs state packages)
  - `pkg/mcp/application/services/interfaces.go` - Service layer
  - `pkg/mcp/application/state/context_enrichers.go` - State layer
  - **Risk**: Session management is critical

- **PromptService** (conversation vs services packages)
  - `pkg/mcp/application/internal/conversation/prompt_service_core.go` - Implementation
  - `pkg/mcp/application/services/interfaces.go` - Service contract
  - **Risk**: AI conversation functionality

### 2. **MEDIUM RISK - Retry System Interfaces (3 interfaces)**
Related to error handling and resilience:

- **RetryCoordinator** (api vs services packages)
- **RetryService** (api vs conversation vs infra packages)
- **FixProvider** (api vs internal vs infra packages)
  - **Risk**: Resilience system, but more isolated

### 3. **LOW RISK - Domain/Common Interfaces (8 interfaces)**
More isolated interfaces with fewer dependencies:

- **Transport** (4 locations)
- **Server** (2 locations)
- **RequestHandler** (2 locations)
- **Services** (3 locations - generic name)
- **Lifecycle** (2 locations)
- **FailureAnalyzer** (2 locations)
- **StateProvider** (2 locations)
- **Analyzer** (2 locations)

## Implementation Strategy

### Phase 1: Low-Risk Duplicates (Quick Wins)
**Goal**: Reduce error count with minimal risk
**Target**: Fix 8 low-risk duplicates

1. **Domain/Common interfaces** - Move to canonical locations
2. **Generic "Services" interfaces** - Rename or consolidate
3. **Simple utility interfaces** - Choose single location

**Approach**:
- Identify the most complete/used version
- Remove or rename duplicates
- Add deprecation comments for transition

### Phase 2: Retry System Consolidation
**Goal**: Unify retry/resilience interfaces
**Target**: Fix 3 retry-related duplicates

1. **Choose canonical location**: `pkg/mcp/application/api/` (most complete)
2. **Create type aliases** in other packages for backward compatibility
3. **Update implementations** to use canonical interfaces

### Phase 3: Core Architecture Interfaces
**Goal**: Consolidate critical service interfaces
**Target**: Fix 5 core duplicates

1. **ToolRegistry** - Merge into services package
2. **ServiceContainer** - Create adapter pattern for state package
3. **Session interfaces** - Consolidate in services package
4. **PromptService** - Use services as canonical location

**Approach**:
- Careful dependency analysis
- Create adapters where needed to avoid circular dependencies
- Phased migration with backward compatibility

### Phase 4: Validation and Cleanup
**Goal**: Ensure zero duplicate interfaces
**Target**: Clean validation results

1. Run interface validation
2. Fix any remaining issues
3. Update architecture documentation
4. Remove deprecated code

## Risk Mitigation Strategies

### 1. **Backward Compatibility**
```go
// In transitional packages
type OldInterface = canonical.NewInterface
```

### 2. **Adapter Pattern for Circular Dependencies**
```go
// Where direct import would create cycles
type AdapterInterface interface {
    Method1() Type1
    Method2() Type2
}

type adapter struct {
    impl canonical.Interface
}
```

### 3. **Gradual Migration**
- Keep old interfaces as deprecated aliases
- Update implementations one by one
- Remove aliases after all references updated

## Execution Plan

### Step 1: Analysis and Documentation (10 minutes)
- [x] Analyze all 16 duplicates
- [x] Categorize by risk level
- [x] Document strategy

### Step 2: Phase 1 - Low Risk (20 minutes)
- [ ] Fix domain/common interface duplicates
- [ ] Target: 8 duplicates → 0 duplicates

### Step 3: Phase 2 - Retry System (15 minutes)
- [ ] Consolidate retry interfaces
- [ ] Target: 3 duplicates → 0 duplicates

### Step 4: Phase 3 - Core Architecture (25 minutes)
- [ ] Consolidate service container interfaces
- [ ] Target: 5 duplicates → 0 duplicates

### Step 5: Phase 4 - Validation (10 minutes)
- [ ] Run validation
- [ ] Fix any remaining issues
- [ ] Update documentation

**Total Estimated Time**: 80 minutes

## Success Criteria

1. ✅ **Zero duplicate interface errors** in validation
2. ✅ **Clean compilation** - no broken builds
3. ✅ **Backward compatibility** maintained during transition
4. ✅ **Documentation updated** to reflect canonical locations
5. ✅ **Architecture validation passes** without warnings

## Canonical Interface Locations

Based on architecture analysis:

- **Tool interfaces**: `pkg/mcp/application/api/interfaces.go`
- **Service interfaces**: `pkg/mcp/application/services/interfaces.go`
- **Domain interfaces**: `pkg/mcp/domain/*/interfaces.go` (domain-specific)
- **Infrastructure interfaces**: `pkg/mcp/infra/*/interfaces.go` (infra-specific)

## Next Steps

1. **Approval**: Confirm this strategy is acceptable
2. **Execute Phase 1**: Start with low-risk duplicates
3. **Progressive implementation**: Move through phases systematically
4. **Continuous validation**: Test after each phase
