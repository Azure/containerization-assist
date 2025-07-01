# AI Assistant Prompt: Workstream Beta - Technical Debt Resolution

## 🎯 Mission Brief
You are the **Lead Developer for Workstream Beta** in the Container Kit MCP architecture completion project. Your mission is to **resolve all identified technical debt and TODO items** over **2 days**.

## 📋 Project Context
- **Repository**: Container Kit MCP server (`pkg/mcp/` directory)
- **Goal**: Clean up critical technical debt from comprehensive codebase scan
  - **155 files** with TODO/FIXME patterns identified
  - **49 files** with placeholder/stub implementations
  - **18 critical items** prioritized for immediate resolution
- **Team**: 3 parallel workstreams (you are Beta - technical debt specialist)
- **Timeline**: 2 days (coordinated with Alpha and Gamma workstreams)
- **Impact**: Clean, production-ready codebase with resolved technical debt

## 🚨 Critical Success Factors

### Must-Do Items
1. **Resolve Interface Issues**: Fix ExecuteWithProgress method call and runtime dependencies
2. **Restore External Services**: Complete registry functionality restoration
3. **Implement Missing Analyzers**: Complete scan and deploy analyzer implementations
4. **Update Test Configurations**: Fix broken test configurations for new systems

### Must-Not-Do Items
- ❌ **Do NOT modify auto-fixing logic** (that's Workstream Alpha)
- ❌ **Do NOT write new tests** (Workstream Gamma handles testing)
- ❌ **Do NOT change core architecture** (focus on completing existing patterns)
- ❌ **Do NOT defer critical TODO items** (they block other functionality)
- ❌ **Do NOT add new TODO comments or placeholders** (resolve existing ones only)
- ❌ **Do NOT create stub implementations** (complete proper implementations)

## 📂 Your File Ownership (You Own These)

### Primary Targets (25+ TODO Items Identified Across Multiple Files)

#### CRITICAL Issues (Must Fix Day 1)
```
pkg/mcp/internal/build/push_image_atomic.go:line_unknown     # CRITICAL: Fix progress adapter TODO
pkg/mcp/internal/build/push_image_atomic.go:line_unknown     # CRITICAL: Implement CalculateScore method
pkg/mcp/internal/orchestration/tool_factory.go:89           # CRITICAL: Implement scan analyzer
pkg/mcp/internal/orchestration/tool_factory.go:102          # CRITICAL: Implement deploy analyzer (manifests)
pkg/mcp/internal/orchestration/tool_factory.go:110          # CRITICAL: Implement deploy analyzer (kubernetes)
```

#### HIGH Priority Issues (Day 1-2)
```
pkg/mcp/internal/analyze/analyze_repository_atomic.go:381   # Check metadata for previous analysis
pkg/mcp/internal/observability/preflight_checker.go:107     # Restore registry functionality
pkg/mcp/internal/observability/preflight_checker.go:569     # Restore registry credential functionality
```

#### Comprehensive TODO Audit Results (54 Files With TODOs)
Based on comprehensive scan, found TODOs in: pipeline/operations.go, deploy/deploy_kubernetes_validate.go, scan/scan_image_security_atomic.go, orchestration/no_reflect_orchestrator.go, build/build_executor.go, build/tag_image_atomic.go, build/pull_image_atomic.go, build/push_image_atomic.go, build/build_fixer.go, state/migrators.go, core/server_lifecycle.go, observability/error_metrics_test.go, observability/preflight_checker.go, orchestration/checkpoint_manager.go, orchestration/tool_factory.go, orchestration/types.go, runtime/conversation/conversation_handler.go, runtime/conversation/conversation_types.go, and many others.

### Do NOT Touch (Other Workstreams)
```
pkg/mcp/internal/runtime/conversation/                       # Workstream Alpha (auto-fixing)
pkg/mcp/internal/build/atomic_tool_mixin.go                 # Workstream Alpha (AI integration)
*_test.go files                                              # Workstream Gamma (testing) - except analyzer_test.go
```

## 📅 2-Day Implementation Plan

### Day 1: High Priority TODO Resolution (8 hours)

#### Morning (4 hours): Interface Migration Fixes
```bash
# CRITICAL TASK 1: Fix ExecuteWithProgress method call
# File: pkg/mcp/internal/build/push_image_atomic.go:127
# Priority: HIGH - Critical path issue blocking functionality

# Current issue: ExecuteWithProgress method not found
# Investigation needed:
# 1. Check what interface operation implements
# 2. Find correct method name in the interface
# 3. Update method call to use correct interface method
# 4. Ensure all required parameters are passed correctly
```

**Implementation Steps**:
1. **Investigate Interface**: Check what interface `operation` implements
2. **Find Correct Method**: Look for similar progress-based execution methods
3. **Fix Method Call**: Replace `ExecuteWithProgress` with correct method name
4. **Test Compilation**: Ensure fix resolves the compilation error

```bash
# TASK 2: Remove runtime dependency
# File: pkg/mcp/internal/build/service.go:40
# Priority: MEDIUM - Clean architecture requirement

# Current: ValidateSessionID method has runtime dependency
# Goal: Implement without runtime dependency

# Implementation approach:
# 1. Analyze current ValidateSessionID implementation
# 2. Identify the runtime dependency being used
# 3. Replace with direct session validation logic
# 4. Ensure session validation still works correctly
```

#### Afternoon (4 hours): External Service Integration
```bash
# TASK 3: Restore registry functionality
# Files: pkg/mcp/internal/observability/preflight_checker.go:107,569
# Priority: MEDIUM - Important for production readiness

# Two TODO items to resolve:
# Line 107: Restore registry functionality with simplified interface
# Line 569: Restore registry credential functionality with simplified interface

# Implementation strategy:
# 1. Analyze what registry functionality was removed
# 2. Design simplified interface for registry operations
# 3. Implement basic registry connectivity checks
# 4. Implement registry credential validation
# 5. Ensure integration with existing preflight checks
```

### Day 2: Orchestration & Testing Cleanup (8 hours)

#### Morning (4 hours): Orchestration Analyzers
```bash
# TASK 4: Implement scan analyzer
# File: pkg/mcp/internal/orchestration/tool_factory.go:89
# Priority: MEDIUM - Required for security scanning integration

# Current: Placeholder implementation
# Goal: Proper scan analyzer for security scanning tools

# Implementation approach:
# 1. Study existing analyzer implementations in the codebase
# 2. Create scan analyzer following established patterns
# 3. Integrate with security scanning tools (Trivy/Grype)
# 4. Ensure proper error handling and result processing
```

```bash
# TASK 5: Implement deploy analyzer (2 instances)
# File: pkg/mcp/internal/orchestration/tool_factory.go:102,110
# Priority: MEDIUM - Required for deployment tool integration

# Current: Placeholder implementations (duplicate TODOs)
# Goal: Proper deploy analyzer for deployment tools

# Implementation approach:
# 1. Study build analyzer implementation as reference
# 2. Create deploy analyzer with Kubernetes-specific logic
# 3. Handle manifest analysis and deployment validation
# 4. Integrate with deployment failure analysis
# 5. Remove duplicate TODO (lines 102 and 110 are the same)
```

#### Afternoon (4 hours): Test & Config Updates
```bash
# TASK 6: Update analyzer config test
# File: pkg/mcp/internal/analyze/analyzer_test.go:117
# Priority: MEDIUM - Critical for test suite reliability

# Current: Test doesn't work with new config system
# Goal: Update test to work with ConfigManager

# Implementation approach:
# 1. Analyze what changed in the config system
# 2. Update test to use new ConfigManager patterns
# 3. Ensure test covers critical analyzer configuration scenarios
# 4. Validate test passes with new configuration system
```

```bash
# TASK 7: Docker optimization version fix
# File: pkg/mcp/internal/customizer/docker_optimization.go:137
# Priority: MEDIUM - Production readiness requirement

# Current: Placeholder version tag
# Goal: Replace with actual version pinning

# Implementation approach:
# 1. Determine appropriate version pinning strategy
# 2. Replace placeholder with actual version handling
# 3. Ensure security optimization uses specific versions
# 4. Document version update process
```

## 🎯 Detailed Task Instructions

### CRITICAL Task 1: Fix ExecuteWithProgress Method Call

**File**: `pkg/mcp/internal/build/push_image_atomic.go:127`  
**Error**: `ExecuteWithProgress method not found`  
**Priority**: HIGH - This blocks functionality

**Investigation Steps**:
```bash
# 1. Check the operation interface
grep -r "type.*Operation.*interface" pkg/mcp/internal/build/
grep -r "Execute.*Progress" pkg/mcp/internal/build/

# 2. Find similar method patterns
grep -r "func.*Execute" pkg/mcp/internal/build/ | grep -i progress

# 3. Check what methods are actually available
# Look at the operation interface definition
```

**Likely Solution**:
The method name may have changed during interface cleanup. Common alternatives:
- `Execute(ctx, progressCallback)`
- `ExecuteWithCallback(ctx, progress)`
- `Run(ctx, progressHandler)`

### Task 2: Remove Runtime Dependency

**File**: `pkg/mcp/internal/build/service.go:40`  
**Issue**: `ValidateSessionID` has runtime dependency  
**Goal**: Clean architecture without runtime coupling

**Implementation Strategy**:
```go
// Current approach likely uses runtime for session access
// Replace with direct session validation

func (s *Service) ValidateSessionID(sessionID string) error {
    // Instead of runtime dependency, use direct session validation
    if sessionID == "" {
        return errors.New("session ID cannot be empty")
    }
    
    // Add direct session validation logic
    if !isValidSessionFormat(sessionID) {
        return errors.New("invalid session ID format")
    }
    
    // Use session manager directly instead of runtime
    return s.sessionManager.ValidateSession(sessionID)
}
```

### Task 3: Restore Registry Functionality

**Files**: `pkg/mcp/internal/observability/preflight_checker.go:107,569`  
**Goal**: Simplified registry interface for connectivity and credentials

**Implementation Approach**:
```go
// Line 107: Registry connectivity check
func (pc *PreflightChecker) checkRegistryConnectivity() error {
    // Implement basic registry ping/connectivity test
    // Use simplified interface that doesn't require complex setup
    registry := pc.getSimpleRegistryClient()
    return registry.Ping(context.Background())
}

// Line 569: Registry credential validation  
func (pc *PreflightChecker) validateRegistryCredentials() error {
    // Implement basic credential validation
    // Check if credentials exist and are properly formatted
    creds := pc.getRegistryCredentials()
    return creds.Validate()
}
```

## 📊 Success Criteria Validation

### After Day 1
```bash
# Interface method call fix validation
method_call_errors=$(go build ./pkg/mcp/internal/build/... 2>&1 | grep -c "ExecuteWithProgress")
[ $method_call_errors -eq 0 ] && echo "✅ Method call fixed" || echo "❌ Method call still broken"

# Runtime dependency check
runtime_deps=$(grep -c "runtime\." pkg/mcp/internal/build/service.go)
echo "Runtime dependencies: $runtime_deps (target: 0)"

# Registry functionality restoration
registry_todos=$(grep -c "TODO.*registry" pkg/mcp/internal/observability/preflight_checker.go)
echo "Registry TODOs: $registry_todos (target: 0)"
```

### After Day 2
```bash
# Orchestration analyzer implementation
analyzer_todos=$(grep -c "TODO.*analyzer" pkg/mcp/internal/orchestration/tool_factory.go)
echo "Analyzer TODOs: $analyzer_todos (target: 0)"

# Test configuration update
test_config_issues=$(go test ./pkg/mcp/internal/analyze/... -v 2>&1 | grep -c "config.*error")
echo "Test config issues: $test_config_issues (target: 0)"

# Version placeholder fix
version_placeholders=$(grep -c "TODO.*version" pkg/mcp/internal/customizer/docker_optimization.go)
echo "Version placeholders: $version_placeholders (target: 0)"
```

### Final Validation
```bash
# Complete TODO resolution check
total_todos=$(grep -r "TODO.*implement\|TODO.*add\|TODO.*fix\|TODO.*remove" pkg/mcp/ --include="*.go" | wc -l)
echo "Remaining high/medium TODOs: $total_todos (target: ≤8 low-priority items)"

# Compilation check
go build ./pkg/mcp/... && echo "✅ Clean compilation" || echo "❌ Compilation errors remain"
```

## 🚨 Common Pitfalls & How to Avoid

### Pitfall 1: Method Call Fix Creates New Issues
**Problem**: Fixing ExecuteWithProgress breaks other functionality
**Solution**: Carefully analyze interface usage patterns before changing

### Pitfall 2: Registry Implementation Too Complex
**Problem**: Attempting to restore full registry functionality
**Solution**: Focus on "simplified interface" - basic connectivity and credential validation only

### Pitfall 3: Analyzer Implementation Scope Creep
**Problem**: Over-engineering the scan/deploy analyzers
**Solution**: Follow existing analyzer patterns, implement minimal viable functionality

### Pitfall 4: Test Update Breaking More Tests
**Problem**: Config test fix breaks other analyzer tests
**Solution**: Limit changes to specific test mentioned in TODO

## 🤝 Coordination with Other Workstreams

### File Coordination Notes
- **Push Image Atomic**: Your method call fix may affect Alpha's auto-fixing integration
- **Registry Functionality**: May impact Gamma's integration tests
- **Orchestration Analyzers**: Should integrate with Alpha's conversation workflow

### Daily Coordination
```bash
# Create daily summary for coordination
cat > day_X_beta_summary.txt << EOF
WORKSTREAM BETA - DAY X SUMMARY
===============================
Progress: X% complete
Critical TODOs resolved: X/18
Method call fixes: ✅/❌
Registry restoration: ✅/❌

Files modified:
- push_image_atomic.go: [specific changes]
- preflight_checker.go: [specific changes]
- tool_factory.go: [specific changes]

Issues encountered:
- [any blockers or concerns]

Tomorrow's focus:
- [next priorities]

Coordination notes:
- [any Alpha/Gamma dependencies]
EOF
```

## 📊 Comprehensive Technical Debt Analysis

### Codebase Scan Results Summary
**Identified during systematic code review:**

#### TODO/FIXME/XXX/HACK Comments (155 files)
**Critical Priority:**
- `pkg/mcp/internal/scan/scan_image_security_atomic.go:504` - Fix session manager interface
- `pkg/mcp/internal/observability/preflight_checker.go:107,569,612,626` - Restore registry functionality
- `pkg/mcp/internal/orchestration/tool_factory.go:89,102,110` - Implement scan/deploy analyzers

**High Priority:**
- `pkg/mcp/internal/build/pull_image_atomic.go:326,331,338` - Complete AI context implementations
- `pkg/mcp/internal/build/service.go:40` - Remove runtime dependency
- `pkg/mcp/internal/customizer/docker_optimization.go:137` - Fix version placeholders

#### Placeholder/Stub Implementations (49 files)
**Critical Stubs:**
- `pkg/mcp/internal/context/integration.go:22` - toolFactory placeholder
- `pkg/mcp/internal/core/server_lifecycle.go:73` - direct request handling not implemented
- `pkg/mcp/internal/runtime/conversation/conversation_handler.go` - retry logic placeholders
- `pkg/mcp/internal/orchestration/workflow_orchestrator.go:924` - executeTool placeholder

**Provider Placeholders:**
- `pkg/mcp/internal/context/providers.go:194,200` - environment/resource detection stubs
- `pkg/mcp/internal/core/communication_manager.go:254` - tool execution simulation

#### Legacy/Compatibility Patterns (58 files)
**Backward Compatibility Issues:**
- Multiple files using deprecated interface patterns
- Import cycle avoidance shims in orchestration
- Legacy session management patterns
- Old analyzer interface implementations

### Impact Assessment
**Production Blockers (Must Fix):** 12 items  
**Core Functionality (Should Fix):** 15 items  
**Technical Debt (Nice to Fix):** 35+ items  
**Total Effort Estimate:** 32-40 hours across 2 days

## 📋 Low Priority TODO Items (Defer if Needed)

If time is limited, these items can be deferred to future iterations:

### AI Context Implementation (3 items)
- `pkg/mcp/internal/build/pull_image_atomic.go:326,331,338`
- **Reason**: Waiting for Recommendation/AI context types to be fully defined
- **Effort**: 6-12 hours total
- **Can defer**: Yes - framework exists, just missing implementations

### Session Metadata Check (1 item)  
- `pkg/mcp/internal/analyze/analyze_repository_atomic.go:381`
- **Reason**: Enhancement, not critical functionality
- **Effort**: 1-2 hours
- **Can defer**: Yes - current functionality works without it

## 🎯 Success Metrics

### Quantitative Targets
- **Critical TODOs**: 100% of high priority items resolved (7 items)
- **Method Call Errors**: 0 compilation errors
- **Registry Functionality**: Basic connectivity and credential validation working
- **Analyzer Implementation**: Scan and deploy analyzers functional

### Qualitative Goals
- **Clean Compilation**: All TODO fixes result in successful builds
- **Production Readiness**: Registry and analyzer functionality suitable for production
- **Test Reliability**: Updated test configurations work with new systems
- **Code Quality**: Resolved TODOs follow established patterns and conventions

## 📚 Reference Materials

- **Build Operations**: `/pkg/mcp/internal/build/` - Study existing operation patterns
- **Analyzer Patterns**: `/pkg/mcp/internal/analyze/` - Reference for new analyzer implementations
- **Registry Operations**: Look for existing registry-related code for patterns
- **Session Management**: `/pkg/mcp/internal/session/` - For runtime dependency removal
- **CLAUDE.md**: Project build commands and testing procedures

## 🔄 End-of-Day Process

```bash
# At the end of each day - MANDATORY VALIDATION STEPS:

# 1. CRITICAL: Full compilation check
echo "=== STEP 1: COMPILATION VALIDATION ==="
go build ./pkg/mcp/...
if [ $? -ne 0 ]; then
    echo "❌ CRITICAL FAILURE: Compilation errors detected"
    echo "🚨 DO NOT PROCEED - Fix compilation errors before ending sprint"
    exit 1
fi
echo "✅ Compilation: PASSED"

# 2. CRITICAL: Lint validation 
echo "=== STEP 2: LINT VALIDATION ==="
make lint
if [ $? -ne 0 ]; then
    echo "❌ CRITICAL FAILURE: Lint errors detected"
    echo "🚨 DO NOT PROCEED - Fix lint errors before ending sprint"
    exit 1
fi
echo "✅ Lint: PASSED"

# 3. CRITICAL: Test validation
echo "=== STEP 3: TEST VALIDATION ==="
make test-mcp
if [ $? -ne 0 ]; then
    echo "❌ CRITICAL FAILURE: Test failures detected"
    echo "🚨 DO NOT PROCEED - Fix test failures before ending sprint"
    exit 1
fi
echo "✅ Tests: PASSED"

# 4. CRITICAL: Full test suite validation
echo "=== STEP 4: FULL TEST SUITE ==="
make test-all
if [ $? -ne 0 ]; then
    echo "❌ CRITICAL FAILURE: Full test suite failures detected"
    echo "🚨 DO NOT PROCEED - Fix test failures before ending sprint"
    exit 1
fi
echo "✅ Full Test Suite: PASSED"

# 5. CRITICAL: TODO resolution validation
echo "=== STEP 5: TODO RESOLUTION VALIDATION ==="
remaining_critical_todos=$(grep -r "TODO.*implement\|TODO.*add\|TODO.*fix\|TODO.*remove" pkg/mcp/ --include="*.go" | grep -v "low priority\|defer" | wc -l)
echo "Critical TODOs remaining: $remaining_critical_todos"
if [ $remaining_critical_todos -gt 8 ]; then
    echo "❌ WARNING: High number of critical TODOs still remain"
    echo "⚠️ Consider extending sprint to complete more TODO items"
fi

# 6. Only proceed if ALL validations pass
echo "=== ALL CRITICAL VALIDATIONS PASSED ==="

# 7. Commit your progress
git add -A
git commit -m "feat(beta): day X technical debt resolution

- Fixed ExecuteWithProgress method call in push_image_atomic.go
- Restored registry functionality in preflight_checker.go  
- Implemented [specific analyzers] in tool_factory.go
- [other specific achievements]

✅ Validated: compilation, lint, tests all passing
📊 TODO Progress: X/18 critical items resolved"

# 8. Create summary for coordination
# (Create day_X_beta_summary.txt as shown above)

# 9. Stop and wait for external merge coordination
echo "✅ Day X Beta work complete - ready for Gamma workstream validation"
echo "✅ ALL QUALITY GATES PASSED: compilation ✓ lint ✓ tests ✓"
```

---

**Remember**: You are cleaning up technical debt that accumulates during rapid development. Focus on fixing what's broken and completing what's partial. Your work makes the codebase production-ready and maintainable! 🚀