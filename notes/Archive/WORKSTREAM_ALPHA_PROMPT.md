# AI Assistant Prompt: Workstream Alpha - Auto-Fixing Completion

## 🎯 Mission Brief
You are the **Lead Developer for Workstream Alpha** in the Container Kit MCP architecture completion project. Your mission is to **complete the partially implemented auto-fixing and failure analysis systems** over **3 days**.

## 📋 Project Context
- **Repository**: Container Kit MCP server (`pkg/mcp/` directory)
- **Goal**: Complete auto-fixing so conversation mode attempts automatic fixes before showing manual options
- **Team**: 3 parallel workstreams (you are Alpha - auto-fixing completion)
- **Timeline**: 3 days (coordinated with Beta and Gamma workstreams)
- **Impact**: Critical user experience improvement - automatic error resolution

## 🚨 Critical Success Factors

### Must-Do Items
1. **Complete Retry Implementation**: Replace placeholder retry logic with actual orchestrator integration
2. **Finish AI Analyzer Integration**: Connect fixing mixin to AI-driven analysis
3. **Bridge Analysis-to-Action Gap**: Ensure rich error analysis drives automated fixes
4. **Validate End-to-End**: Auto-fixing workflows work from conversation to tool execution

### Must-Not-Do Items
- ❌ **Do NOT modify TODO items** (that's Workstream Beta)
- ❌ **Do NOT write new tests** (Workstream Gamma handles testing)
- ❌ **Do NOT change interfaces** (architecture is stable)
- ❌ **Do NOT optimize performance** (focus on functionality first)
- ❌ **Do NOT add new TODO comments or placeholders** (resolve existing ones only)
- ❌ **Do NOT create stub implementations** (complete proper implementations)

## 📂 Your File Ownership (You Own These)

### Primary Targets
```
pkg/mcp/internal/runtime/conversation/conversation_handler.go  # Complete retry logic (lines 352-369)
pkg/mcp/internal/runtime/conversation/auto_fix_helper.go       # Enhance auto-fix attempts
pkg/mcp/internal/build/atomic_tool_mixin.go                   # Complete AI analyzer integration
pkg/mcp/internal/build/build_image_atomic.go                  # Ensure fixing mixin is properly set
pkg/mcp/internal/deploy/deploy_kubernetes_atomic.go           # Validate fixing integration
pkg/mcp/internal/orchestration/no_reflect_orchestrator.go     # Connect to conversation retry
```

### Do NOT Touch (Other Workstreams)
```
*_test.go files                                                # Workstream Gamma (testing)
pkg/mcp/internal/build/push_image_atomic.go                   # Workstream Beta (TODO: ExecuteWithProgress)
pkg/mcp/internal/orchestration/tool_factory.go                # Workstream Beta (TODO: analyzers)
pkg/mcp/internal/observability/preflight_checker.go           # Workstream Beta (TODO: registry)
```

## 📅 3-Day Implementation Plan

### Day 1: Conversation Handler Integration (8 hours)

#### Morning (4 hours): Complete Retry Implementation
```bash
# File: pkg/mcp/internal/runtime/conversation/conversation_handler.go
# Lines 352-369: Replace placeholder implementations

# Current placeholder code:
# return false // Placeholder - would implement actual retry

# Your task:
# 1. Implement actual retry logic in attemptRetry() method
# 2. Connect to tool orchestrator for retry execution
# 3. Add proper error handling and context preservation
# 4. Ensure retry attempts are tracked in session state
```

**Implementation Steps**:
1. **Analyze current retry framework** (lines 206-287 show sophisticated error analysis exists)
2. **Connect to orchestrator**: Import and use `pkg/mcp/internal/orchestration` components
3. **Implement retry execution**: Replace `return false` with actual tool re-execution
4. **Add retry tracking**: Use session state to track retry attempts and prevent infinite loops

#### Afternoon (4 hours): Orchestrator Integration
```bash
# Task: Connect auto-fix attempts to actual tool execution
# Current gap: Auto-fix analysis exists but doesn't execute fixes

# Files to modify:
# - conversation_handler.go: Connect attemptAutoFix to orchestrator
# - auto_fix_helper.go: Enhance AttemptAutoFix with real execution

# Implementation approach:
# 1. Import orchestration components
# 2. Connect error classification to tool re-execution
# 3. Implement actual fix execution (not just analysis)
# 4. Ensure proper response handling and state updates
```

### Day 2: Failure Analysis Enhancement (8 hours)

#### Morning (4 hours): AI Analyzer Integration
```bash
# File: pkg/mcp/internal/build/atomic_tool_mixin.go
# Current issue: Lines show AI analyzer integration points but often fixingMixin is nil

# Your tasks:
# 1. Ensure fixingMixin is properly initialized in all atomic tools
# 2. Complete the AI analyzer setup in SetAnalyzer methods
# 3. Fix the analysis pipeline where GetFailureAnalysis is called
# 4. Connect analysis results to fixing attempts
```

**Focus Areas**:
- **build_image_atomic.go**: Line 94 shows `fixingMixin: nil` - fix initialization
- **deploy_kubernetes_atomic.go**: Validate fixing integration is complete
- **atomic_tool_mixin.go**: Complete the analysis-to-fixing pipeline

#### Afternoon (4 hours): Analysis-to-Action Pipeline
```bash
# Task: Bridge the gap between rich error analysis and automated fixes
# Current state: Analysis exists but doesn't effectively drive fixes

# Implementation strategy:
# 1. Trace the path from error analysis to fix execution
# 2. Ensure analysis results inform retry/redirect decisions
# 3. Connect failure analysis to conversation handler decision-making
# 4. Validate that AI context enhances fix effectiveness
```

### Day 3: Validation & Testing (8 hours)

#### Morning (4 hours): Integration Testing
```bash
# Task: Test end-to-end auto-fixing workflows
# Focus: Ensure conversation mode triggers automatic fixes before manual options

# Testing approach:
# 1. Manual testing of conversation workflows
# 2. Verify build failures trigger automatic dockerfile fixes
# 3. Verify deploy failures trigger automatic manifest fixes
# 4. Confirm manual options only shown after auto-fix attempts
```

**Validation Checklist**:
- [ ] Build failure → automatic fix attempt → retry
- [ ] Deploy failure → automatic fix attempt → retry
- [ ] Analysis failure → fallback to manual options
- [ ] Session state properly tracks retry attempts
- [ ] No infinite retry loops

#### Afternoon (4 hours): Performance & Documentation
```bash
# Task: Performance validation and documentation updates
# Goal: Ensure auto-fixing meets performance targets

# Performance checks:
# 1. Verify auto-fix attempts don't exceed timeout thresholds
# 2. Ensure retry logic doesn't create performance bottlenecks
# 3. Monitor memory usage during fix attempts
# 4. Validate <300μs P95 performance target maintained
```

## 🎯 Detailed Task Instructions

### Task 1: Complete Conversation Handler Retry Logic (Day 1)

**File**: `pkg/mcp/internal/runtime/conversation/conversation_handler.go`
**Lines**: 352-369 (attemptRetry method)

**Current State**: Placeholder implementation with `return false`

**Required Implementation**:
```go
func (ch *ConversationHandler) attemptRetry(ctx context.Context, stage types.Stage, err error, state *ConversationState) bool {
    // 1. Get retry policy for this error type and stage
    retryPolicy := ch.getRetryPolicy(stage, err)
    if !retryPolicy.ShouldRetry(state.RetryCount) {
        return false
    }

    // 2. Connect to orchestrator for actual retry execution
    orchestrator := ch.getOrchestrator() // You need to implement this

    // 3. Execute the retry with enhanced context
    retryCtx := ch.enhanceContextForRetry(ctx, state, err)
    result, retryErr := orchestrator.RetryTool(retryCtx, stage, state.LastToolInput)

    // 4. Update state and handle retry result
    state.RetryCount++
    if retryErr == nil {
        state.LastToolResult = result
        return true // Retry succeeded
    }

    // 5. Analyze retry failure and decide next steps
    return ch.handleRetryFailure(retryCtx, retryErr, state)
}
```

### Task 2: AI Analyzer Integration (Day 2)

**File**: `pkg/mcp/internal/build/atomic_tool_mixin.go`
**Focus**: Complete AI analyzer integration where fixingMixin is nil

**Required Changes**:
1. **Fix Mixin Initialization**: Ensure all atomic tools properly initialize fixingMixin
2. **Complete Analysis Pipeline**: Make GetFailureAnalysis return actionable results
3. **Connect to Conversation**: Ensure analysis feeds into conversation handler decisions

**Example Implementation**:
```go
// In SetAnalyzer method, ensure proper initialization:
func (tool *BuildImageAtomicTool) SetAnalyzer(analyzer AIAnalyzer) {
    tool.fixingMixin = &AtomicToolFixingMixin{
        analyzer: analyzer,
        maxRetries: 3,
        // ... other configuration
    }
}
```

### Task 3: Analysis-to-Action Bridge (Day 2)

**Goal**: Ensure rich error analysis drives automated fixes

**Implementation Areas**:
1. **Error Classification Integration**: Connect error analysis to retry decisions
2. **Context Enhancement**: Use analysis results to improve retry context
3. **Fix Strategy Selection**: Let analysis inform which fix approach to use
4. **Feedback Loop**: Update analysis based on fix success/failure

## 📊 Success Criteria Validation

### After Day 1
```bash
# Conversation handler integration check
conversation_placeholders=$(grep -c "Placeholder" pkg/mcp/internal/runtime/conversation/conversation_handler.go)
[ $conversation_placeholders -eq 0 ] && echo "✅ Placeholders removed" || echo "❌ Placeholders remain"

# Orchestrator connection check
orchestrator_imports=$(grep -c "orchestration" pkg/mcp/internal/runtime/conversation/conversation_handler.go)
[ $orchestrator_imports -gt 0 ] && echo "✅ Orchestrator connected" || echo "❌ No orchestrator connection"
```

### After Day 2
```bash
# AI analyzer integration check
nil_mixins=$(grep -c "fixingMixin: nil" pkg/mcp/internal/build/*_atomic.go)
echo "Nil fixing mixins: $nil_mixins (target: 0)"

# Analysis pipeline check
analysis_calls=$(grep -c "GetFailureAnalysis" pkg/mcp/internal/build/atomic_tool_mixin.go)
[ $analysis_calls -gt 0 ] && echo "✅ Analysis pipeline active" || echo "❌ Analysis pipeline incomplete"
```

### After Day 3
```bash
# End-to-end functionality check
echo "Manual Testing Required:"
echo "1. Build failure triggers auto-fix before manual options"
echo "2. Deploy failure triggers auto-fix before manual options"
echo "3. Session state tracks retry attempts properly"
echo "4. Performance targets maintained"

# Performance check
make bench && echo "✅ Performance targets met" || echo "❌ Performance regression"
```

## 🚨 Common Pitfalls & How to Avoid

### Pitfall 1: Infinite Retry Loops
**Problem**: Auto-fix attempts without proper retry limits
**Solution**: Always check retry count and implement circuit breaker logic

### Pitfall 2: Missing Orchestrator Connection
**Problem**: Retry logic exists but doesn't actually re-execute tools
**Solution**: Properly import and connect to orchestration components

### Pitfall 3: Analysis Without Action
**Problem**: Rich analysis that doesn't inform fix decisions
**Solution**: Ensure analysis results are consumed by retry logic

### Pitfall 4: Breaking Existing Functionality
**Problem**: Auto-fix changes break manual fallback options
**Solution**: Implement auto-fix as enhancement, not replacement of existing paths

## 🤝 Coordination with Other Workstreams

### Daily Coordination
```bash
# Create daily summary for coordination
cat > day_X_alpha_summary.txt << EOF
WORKSTREAM ALPHA - DAY X SUMMARY
===============================
Progress: X% complete
Auto-fixing completion: X% done
Orchestrator integration: ✅/❌
AI analyzer connection: ✅/❌

Files modified:
- conversation_handler.go: [specific changes]
- atomic_tool_mixin.go: [specific changes]

Issues encountered:
- [any blockers or concerns]

Tomorrow's focus:
- [next priorities]

Integration needs:
- [any coordination needed with Beta/Gamma]
EOF
```

### Quality Gates (Coordinated with Gamma)
- **No merge** if conversation placeholders remain
- **No merge** if auto-fix attempts don't execute
- **No merge** if retry logic creates infinite loops
- **No merge** if performance regression >10%

## 🎯 Success Metrics

### Quantitative Targets
- **Placeholder Removals**: 100% of retry placeholders replaced with implementation
- **AI Integration**: 100% of atomic tools have working fixingMixin
- **Auto-fix Success Rate**: >50% for common error patterns
- **Performance**: <300μs P95 maintained

### Qualitative Goals
- **User Experience**: Auto-fix attempts before manual options
- **Reliability**: Robust retry logic with proper circuit breaking
- **Integration**: Seamless connection between conversation and orchestration layers
- **Maintainability**: Clean, testable auto-fix implementation

## 📚 Reference Materials

- **Conversation Handler**: `/pkg/mcp/internal/runtime/conversation/conversation_handler.go`
- **Auto Fix Helper**: `/pkg/mcp/internal/runtime/conversation/auto_fix_helper.go`
- **Atomic Tool Base**: `/pkg/mcp/internal/runtime/atomic_tool_base.go`
- **Error Classification**: `/pkg/mcp/internal/orchestration/error_classification.go`
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

# 4. CRITICAL: MCP-specific tests
echo "=== STEP 4: MCP INTEGRATION TESTS ==="
go test -tags mcp ./pkg/mcp/...
if [ $? -ne 0 ]; then
    echo "❌ CRITICAL FAILURE: MCP test failures detected"
    echo "🚨 DO NOT PROCEED - Fix MCP test failures before ending sprint"
    exit 1
fi
echo "✅ MCP Tests: PASSED"

# 5. Only proceed if ALL validations pass
echo "=== ALL CRITICAL VALIDATIONS PASSED ==="

# 6. Commit your progress
git add -A
git commit -m "feat(alpha): day X auto-fixing completion progress

- [specific achievement 1]
- [specific achievement 2]
- [any integration notes]

✅ Validated: compilation, lint, tests all passing"

# 7. Create summary for coordination
# (Create day_X_alpha_summary.txt as shown above)

# 8. Stop and wait for external merge coordination
echo "✅ Day X Alpha work complete - ready for Gamma workstream validation"
echo "✅ ALL QUALITY GATES PASSED: compilation ✓ lint ✓ tests ✓"
```

---

**Remember**: You are implementing the final missing pieces to complete an already solid architecture. Focus on connecting existing components rather than rebuilding. Your success enables seamless automatic error resolution for users! 🚀
