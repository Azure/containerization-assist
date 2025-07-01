# AI Assistant Prompt: Container Kit MCP Critical Cleanup

## TASK OVERVIEW

You are tasked with executing critical cleanup of the Container Kit MCP codebase to remove dead code, over-engineering, and structural issues. This is a **high-impact, low-risk** cleanup focused on removing unused code and simplifying over-engineered components.

## CRITICAL SAFETY REQUIREMENTS

1. **NEVER remove code without verification** - Always check usage with search tools first
2. **Run tests after each major deletion** to ensure no breakage
3. **Make incremental changes** - Don't delete multiple large files simultaneously
4. **Preserve all functional behavior** - Only remove truly unused code
5. **Document any breaking changes** in commit messages

## PHASE 1: DEAD CODE REMOVAL (CRITICAL PRIORITY)

### 1.1 Verify and Remove Massive Dead Files

**Before removing each file, MUST verify it's truly unused:**

```bash
# For each target file, run these verification commands:
rg "docker_retry" --type go pkg/mcp/ | grep -v "_test.go" | grep -v "docker_retry.go"
rg "ResourceMonitor" --type go pkg/mcp/ | grep -v "_test.go" | grep -v "resource_monitor.go"
rg "PerformanceOptimizer" --type go pkg/mcp/ | grep -v "_test.go" | grep -v "performance_optimizer.go"
rg "SagaManager" --type go pkg/mcp/ | grep -v "_test.go" | grep -v "saga_manager.go"
```

**Target Files for Removal (6,700+ lines):**
- `pkg/mcp/internal/pipeline/docker_retry.go` (1,244 lines)
- `pkg/mcp/internal/session/resource_monitor.go` (1,190 lines)
- `pkg/mcp/internal/orchestration/performance_optimizer.go` (1,179 lines)
- `pkg/mcp/internal/orchestration/saga_manager.go` (1,091 lines)
- `pkg/mcp/internal/orchestration/workflow_orchestrator.go` (991 lines)
- `pkg/mcp/internal/orchestration/circuit_breaker.go` (385 lines)
- `pkg/mcp/utils/workspace.go` (1,087 lines) - if verification shows minimal usage

**Process for each file:**
1. Search for external usage beyond tests
2. If no external usage found, delete the file
3. Delete corresponding test files
4. Remove imports from other files
5. Run `go test -short ./...` to verify no breakage

### 1.2 Remove Orphaned Test Infrastructure

**Target files (1,000+ lines):**
- `pkg/mcp/internal/testutil/profiling_profiling_test_suite.go`
- `pkg/mcp/internal/testutil/pipeline_pipeline_test_helpers.go`
- `pkg/mcp/internal/testutil/profiling_performance_assertions.go`

**Verification process:**
```bash
# Check if test utilities are actually used
rg "MockProfiler|TestAnalysisConverter|PerformanceAssertion" --type go pkg/mcp/
```

## PHASE 2: STRUCTURAL FIXES (HIGH PRIORITY)

### 2.1 Fix Binary Location
```bash
# Move binary from package to correct location
mkdir -p cmd/mcp-server
mv pkg/mcp/cmd/mcp-server/* cmd/mcp-server/
rm -rf pkg/mcp/cmd/
# Update go.mod and any references to the old path
```

### 2.2 Fix File Naming Issues

**Rename files with redundant prefixes:**
```bash
# In pkg/mcp/internal/transport/
mv llm_llm_stdio.go llm_stdio.go
mv llm_llm_http.go llm_http.go
mv llm_llm_mock.go llm_mock.go
mv llm_llm_stdio_test.go llm_stdio_test.go

# In pkg/mcp/internal/testutil/
mv profiling_profiling_test_suite.go profiling_test_suite.go
mv pipeline_pipeline_test_helpers.go pipeline_test_helpers.go
```

### 2.3 Remove Panic Usage in Library Code

**Target: `pkg/mcp/client_factory.go`**
```go
// REPLACE panic-based methods:
func (b *BaseInjectableClients) GetDockerClient() docker.DockerClient {
    if b.clientFactory == nil {
        panic("client factory not injected - call SetClientFactory first")
    }
    return b.clientFactory.CreateDockerClient()
}

// WITH error-returning versions:
func (b *BaseInjectableClients) GetDockerClient() (docker.DockerClient, error) {
    if b.clientFactory == nil {
        return nil, fmt.Errorf("client factory not injected - call SetClientFactory first")
    }
    return b.clientFactory.CreateDockerClient(), nil
}
```

**IMPORTANT:** This is a breaking change. Update all call sites to handle errors.

## PHASE 3: INTERFACE CONSOLIDATION (MEDIUM PRIORITY)

### 3.1 Reduce Interface Complexity

**Target: `pkg/mcp/core/interfaces.go` (626 lines)**

**Review and consolidate:**
- Identify interfaces with only one implementation
- Merge related interfaces
- Remove interfaces that just wrap simple functions
- Keep only interfaces with multiple implementations or clear abstraction value

### 3.2 Remove Legacy Compatibility Code

**Target patterns to remove:**
```go
// Remove backward compatibility type aliases
type WorkflowSession = ExecutionSession
type WorkflowStage = ExecutionStage

// Remove legacy conversion functions
func ConvertRepositoryInfoToScanSummary(...) {...}
func ConvertScanSummaryToRepositoryInfo(...) {...}
```

## EXECUTION COMMANDS

### Setup and Verification
```bash
# 1. Create a cleanup branch
git checkout -b cleanup/remove-dead-code

# 2. Run initial tests to establish baseline
make test-all

# 3. Check current code metrics
find pkg/mcp -name "*.go" | wc -l
find pkg/mcp -name "*.go" | xargs wc -l | tail -1
```

### Dead Code Verification Template
```bash
# For each target file, use this verification process:
TARGET_FILE="pkg/mcp/internal/pipeline/docker_retry.go"
BASE_NAME=$(basename "$TARGET_FILE" .go)
STRUCT_NAME="DockerRetryManager"  # Adjust per file

echo "Checking usage of $TARGET_FILE..."
rg "$BASE_NAME|$STRUCT_NAME" --type go pkg/mcp/ | grep -v "$TARGET_FILE" | grep -v "_test.go"

# If no results, safe to delete:
rm "$TARGET_FILE"
rm "${TARGET_FILE%%.go}_test.go" 2>/dev/null || true

# Test after each deletion
go test ./...
```

### Post-Cleanup Validation
```bash
# After each phase, run:
go test ./...
go fmt ./...

# Check metrics improvement
find pkg/mcp -name "*.go" | wc -l
find pkg/mcp -name "*.go" | xargs wc -l | tail -1

# Commit progress
git add .
git commit -m "cleanup: remove dead code files

- Removed docker_retry.go (1,244 lines) - only used in tests
- Removed resource_monitor.go (1,190 lines) - no external usage
- Removed performance_optimizer.go (1,179 lines) - unused optimization engine

Estimated reduction: 3,600+ lines of dead code"
```

## SUCCESS CRITERIA

✅ **Phase 1 Complete:**
- 6,000+ lines of dead code removed
- All tests still pass
- No functional behavior changed
- Clean commit history with explanatory messages

✅ **Phase 2 Complete:**
- Binary moved to correct location (`cmd/mcp-server/`)
- File naming conventions fixed
- Panic usage replaced with error returns
- All breaking changes documented

✅ **Phase 3 Complete:**
- Interface count reduced by 50%+
- Legacy compatibility code removed
- Code complexity significantly reduced

## ROLLBACK PLAN

If issues arise:
```bash
# Rollback to previous state
git reset --hard HEAD~1

# Or cherry-pick successful changes
git log --oneline | head -10  # Find good commits
git cherry-pick <good-commit-hash>
```

## MONITORING PROGRESS

Track these metrics throughout cleanup:
- **Total Go files:** `find pkg/mcp -name "*.go" | wc -l`
- **Total lines of code:** `find pkg/mcp -name "*.go" | xargs wc -l | tail -1`
- **Large files (>500 lines):** `find pkg/mcp -name "*.go" -exec wc -l {} + | awk '$1 > 500' | wc -l`
- **Test success:** `make test-all && echo "✅ PASS" || echo "❌ FAIL"`

## COMMUNICATION

After completing each phase:
1. **Document changes** in commit messages with line count savings
2. **Note any breaking changes** in the commit description
3. **Run full test suite** and include results in final report
4. **Update metrics** in cleanup summary

This cleanup should result in **30-40% code reduction** while maintaining all functional behavior and improving maintainability.
