# Test Coverage Improvement Plan

## Current Status (January 2025)

### Coverage Summary
- **Overall Coverage**: ~15% (estimated from working packages)
- **Target Coverage**: 55% global baseline, 80% for new code
- **Current Best**: Domain containerization packages (~60-70%)

### Package Analysis

#### High Coverage (✅ Good - >55%)
- `domain/containerization/analyze`: 68.2%
- `domain/containerization/build`: 69.5%
- `domain/containerization/deploy`: 55.7%

#### Medium Coverage (⚠️ Needs Work - 30-55%)
- `domain/containerization/scan`: 52.5%
- `application/internal/retry`: 44.8%
- `domain/security`: 34.2%

#### Low Coverage (❌ Poor - <30%)
- `application/internal/conversation`: 14.6%
- `application/internal/runtime`: 20.8%
- `domain/errors`: 4.6%
- `domain/session`: 14.3%
- `infra/retry`: 8.6%

#### No Coverage (❌ No tests)
- Most `application/*` packages
- Most `infra/*` packages
- Many `domain/*` utility packages

### Build Issues
- 1 package with build failures preventing testing
- Several packages have missing imports

## Improvement Strategy

### Phase 1: Foundation (Week 1)
**Target**: Fix build issues and add basic tests

**High Priority Packages**:
1. `domain/errors` - Critical business logic (currently 4.6%)
   - Add comprehensive RichError tests
   - Test error wrapping and context
   - Test error code and severity handling

2. `domain/security` - Security validation (currently 34.2%)
   - Test validation rules
   - Test policy enforcement
   - Test security edge cases

3. `application/internal/runtime` - Core functionality (currently 20.8%)
   - Test tool registry operations
   - Test tool execution paths
   - Test error handling

**Actions**:
```bash
# Generate tests for priority packages
scripts/quality/generate_tests.sh pkg/mcp/domain/errors unit
scripts/quality/generate_tests.sh pkg/mcp/domain/security unit
scripts/quality/generate_tests.sh pkg/mcp/application/internal/runtime unit
```

### Phase 2: Core Domain (Week 2-3)
**Target**: 70%+ coverage for domain layer

**Focus Areas**:
- Complete domain/containerization/* testing
- Add comprehensive domain/session tests
- Test domain/config validation

**Key Packages**:
- `domain/session` → Target: 80%
- `domain/config` → Target: 70%
- `domain/types` → Target: 60%

### Phase 3: Application Layer (Week 4-5)
**Target**: 50%+ coverage for application layer

**Focus Areas**:
- `application/core` - Server lifecycle
- `application/commands` - Command implementations
- `application/orchestration` - Pipeline logic

**Integration Tests**:
- Full workflow testing
- Multi-tool scenarios
- Error recovery testing

### Phase 4: Infrastructure (Week 6)
**Target**: 40%+ coverage for infrastructure

**Focus Areas**:
- `infra/persistence` - Storage operations
- `infra/transport` - Protocol handling
- `infra/docker` - Container operations

## Test Types by Layer

### Domain Layer Tests
- **Unit Tests**: Pure business logic, no dependencies
- **Property-Based Tests**: Validation rules
- **Error Scenario Tests**: Rich error handling

Example:
```go
func TestRichError_WithContext(t *testing.T) {
    err := errors.NewError().
        Code(errors.CodeValidationFailed).
        Message("test error").
        Context("field", "value").
        Build()
    
    assert.Equal(t, errors.CodeValidationFailed, err.Code())
    assert.Equal(t, "value", err.Context()["field"])
}
```

### Application Layer Tests
- **Integration Tests**: Service interactions
- **Mock-Based Tests**: External dependencies
- **Workflow Tests**: End-to-end scenarios

Example:
```go
func TestToolRegistry_Execute(t *testing.T) {
    registry := NewToolRegistry()
    mockTool := &MockTool{}
    
    registry.Register(mockTool)
    
    result, err := registry.Execute(ctx, "mock-tool", args)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Infrastructure Layer Tests
- **Integration Tests**: Real external systems
- **Contract Tests**: Interface compliance
- **Performance Tests**: Resource usage

## Test Standards

### Test Structure
```go
func TestFunction_Scenario(t *testing.T) {
    // Arrange
    // Act  
    // Assert
}
```

### Coverage Requirements
- **New Code**: 80% minimum
- **Modified Code**: Maintain or improve existing coverage
- **Critical Paths**: 90%+ coverage
- **Error Paths**: 100% coverage

### Test Categories
1. **Happy Path**: Normal operation scenarios
2. **Edge Cases**: Boundary conditions
3. **Error Cases**: Failure scenarios
4. **Performance**: Benchmark critical paths
5. **Integration**: Cross-component interaction

## Implementation Plan

### Week 1: Infrastructure Setup ✅
- [x] Coverage tracking system
- [x] Test templates
- [x] Test generation tools
- [x] Baseline measurement

### Week 2: Critical Domain Tests
- [ ] Fix build issues blocking tests
- [ ] Implement domain/errors comprehensive tests
- [ ] Implement domain/security validation tests  
- [ ] Implement domain/session tests

### Week 3: Domain Layer Completion
- [ ] Complete containerization package tests
- [ ] Add domain/config tests
- [ ] Add domain/types tests
- [ ] Add missing utility tests

### Week 4: Application Layer Core
- [ ] Implement application/core tests
- [ ] Implement application/commands tests
- [ ] Add application/internal/* tests
- [ ] Integration test infrastructure

### Week 5: Application Layer Integration
- [ ] Pipeline orchestration tests
- [ ] Workflow engine tests
- [ ] Service integration tests
- [ ] End-to-end scenarios

### Week 6: Infrastructure & Polish
- [ ] Infrastructure layer tests
- [ ] Performance benchmark tests
- [ ] Test cleanup and optimization
- [ ] Final coverage validation

## Success Metrics

### Coverage Targets
- **Overall**: 55% (from current ~15%)
- **Domain**: 70% (from current ~40%)
- **Application**: 50% (from current ~10%)
- **Infrastructure**: 40% (from current ~20%)

### Quality Metrics
- **All new code**: 80%+ coverage
- **Critical paths**: 90%+ coverage
- **Zero critical bugs**: From test coverage gaps
- **Performance**: No regression from tests

### Process Metrics
- **Test generation**: <5 minutes per package
- **Test execution**: <2 minutes for full suite
- **Coverage reporting**: Automated in CI/CD
- **Test maintenance**: <10% effort overhead

## Tools and Resources

### Generated Tools
- `scripts/quality/coverage_tracker.sh` - Coverage analysis
- `scripts/quality/generate_tests.sh` - Test generation
- `test/templates/` - Test templates

### Manual Commands
```bash
# Run coverage analysis
scripts/quality/coverage_tracker.sh

# Generate unit tests for a package
scripts/quality/generate_tests.sh pkg/mcp/domain/errors unit

# Generate integration tests
scripts/quality/generate_tests.sh pkg/mcp/application/core integration

# Run tests with coverage
go test -cover ./pkg/mcp/...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./pkg/mcp/...
go tool cover -html=coverage.out -o coverage.html
```

### Integration with CI/CD
- Coverage gates in pull requests
- Automatic test generation suggestions
- Performance regression detection
- Test result reporting

## Risk Mitigation

### Test Maintenance Overhead
- **Mitigation**: Use generated templates and standard patterns
- **Monitoring**: Track test-to-code ratio

### Performance Impact
- **Mitigation**: Separate unit vs integration tests
- **Monitoring**: Benchmark test execution time

### Coverage Gaming
- **Mitigation**: Focus on meaningful tests, not just coverage numbers
- **Review**: Manual review of critical test scenarios

### Coordination with Refactoring
- **Strategy**: Add tests before refactoring
- **Priority**: Test stable interfaces first
- **Flexibility**: Update tests as interfaces evolve

---

This plan provides a systematic approach to achieving the 55% coverage target while maintaining code quality and supporting the ongoing architectural refactoring.