# Deprecated Code Removal Priority List

## Overview
Based on the comprehensive deprecated code inventory, this document prioritizes removal order by impact, complexity, and dependencies.

## Priority Classification

### PRIORITY 1: Critical Impact (Remove Days 11-12)
**High business value, breaking changes, must remove first**

1. **Manager Interface Removals** (EPSILON workstream)
   - Status: Already documented as removed in interfaces.go
   - Impact: Breaking changes to public APIs
   - Action: Verify no remaining references, complete cleanup

2. **Validation System Core Deprecations**
   - `pkg/common/validation/unified_validator.go` (reflection-based)
   - Impact: Performance bottleneck, security concerns with reflection
   - Action: Replace with unified validation system from GAMMA Day 4

### PRIORITY 2: High Impact (Remove Days 13-14)
**Functional deprecations affecting core features**

3. **State Management Service Deprecations**
   - `pkg/mcp/application/state/context_enrichers.go`
   - `pkg/mcp/application/state/integration.go`
   - Impact: State management consistency
   - Dependencies: ServiceContainer must be fully implemented

4. **Service Interface Transport/Retry Deprecations**
   - `pkg/mcp/application/services/transport.go`
   - `pkg/mcp/application/services/retry.go`
   - Impact: Breaking changes to service layer
   - Dependencies: api.Transport and api.RetryCoordinator must be complete

### PRIORITY 3: Medium Impact (Remove Days 15-16)
**Tool and schema deprecations with moderate impact**

5. **Tool Validation Error Deprecations**
   - `pkg/mcp/application/tools/tool_validation.go` - 2 deprecated functions
   - Impact: Error reporting quality
   - Action: Replace with NewRichValidationError functions

6. **Schema Generation Deprecations (Batch 1)**
   - `ToMap`, `FromMap`, `GenerateSchemaAsMap` functions
   - Impact: Schema generation API consistency
   - Action: Update all callers to new function names

### PRIORITY 4: Low Impact (Remove Days 17-18)
**Remaining schema deprecations**

7. **Schema Type Deprecations (Batch 2)**
   - `StringSchema`, `NumberSchema`, `ArraySchema` functions
   - Impact: Type schema generation
   - Action: Replace with typed schema functions

8. **Schema Internal Deprecations (Batch 3)**
   - `applyValidationConstraintsTyped`, `getJSONType`
   - Impact: Internal schema processing
   - Action: Refactor internal schema logic

### PRIORITY 5: Cleanup (Remove Days 19-20)
**Final validation and cleanup**

9. **Final Validation and Testing**
   - Comprehensive testing of all replacements
   - Performance benchmarking
   - Documentation updates

## Removal Strategy by Priority

### Priority 1 & 2: Critical Path Dependencies
```bash
# Days 11-12: Critical removals
1. Verify Manager interfaces completely removed
2. Replace reflection-based validation
3. Update state management to ServiceContainer
4. Replace transport/retry services with api versions
```

### Priority 3 & 4: Feature Deprecations
```bash
# Days 13-16: Feature deprecations
1. Update error functions to RichError variants
2. Batch update schema generation functions
3. Update schema type functions
4. Refactor internal schema processing
```

### Priority 5: Quality Assurance
```bash
# Days 17-20: Final cleanup and validation
1. Run comprehensive test suite
2. Performance benchmarking
3. Update documentation
4. Final verification of no deprecated code usage
```

## Dependencies and Risks

### High-Risk Removals
1. **Validation System** - Core business logic, extensive testing required
2. **Manager Interfaces** - Breaking changes, coordinated removal needed
3. **State Management** - Session integrity, data consistency critical

### Medium-Risk Removals
4. **Service Interfaces** - Transport layer changes, network implications
5. **Tool Validation** - Error reporting quality impact

### Low-Risk Removals
6. **Schema Functions** - Internal API changes, limited external impact

## Success Metrics

### Day 12 Checkpoint (Priority 1-2 Complete)
- ✅ Manager interfaces completely removed
- ✅ Reflection-based validation eliminated  
- ✅ State management using ServiceContainer
- ✅ Service layer using api interfaces

### Day 16 Checkpoint (Priority 3-4 Complete)
- ✅ RichError functions implemented
- ✅ Schema generation APIs unified
- ✅ Type schema functions updated

### Day 20 Final Verification
- ✅ Zero deprecated code remaining
- ✅ All tests passing
- ✅ Performance benchmarks improved
- ✅ Documentation updated

## Rollback Strategy

Each priority level should be implemented with rollback capability:

1. **Feature flags** for validation system changes
2. **Gradual migration** for service interfaces
3. **Backward compatibility** during schema updates
4. **Automated testing** at each removal step

This prioritized approach ensures systematic removal while maintaining system stability and functionality throughout the GAMMA workstream.