# Orphaned Test Files Analysis - Sprint D

**Generated:** 2025-06-26
**Total Test Files Analyzed:** 55
**Orphaned Files Found:** 28 (50.9%)
**Valid Test Files:** 27 (49.1%)

## Executive Summary

The analysis confirms the Sprint D plan finding of "26+ orphaned test files" under `pkg/mcp`. **28 orphaned test files** were identified, representing 50.9% of all test files in the MCP package structure. This indicates significant test debt that needs immediate attention to achieve the 80% coverage target.

## Orphaned Test Files Catalog (28 Total)

### Critical Infrastructure Tests (8 files) 🚨
**Priority: IMMEDIATE** - These test core MCP functionality

1. **`pkg/mcp/auto_advance_test.go`**
   - Missing: `pkg/mcp/auto_advance.go`
   - Tests: Conversation auto-advance functionality
   - **Action:** Implement or delete (likely needed for Sprint A)

2. **`pkg/mcp/no_external_ai_test.go`**
   - Missing: `pkg/mcp/no_external_ai.go`
   - Tests: AI-free mode functionality
   - **Action:** Investigate requirement, implement or delete

3. **`pkg/mcp/internal/core/mcp_server_test.go`**
   - Missing: `pkg/mcp/internal/core/mcp_server.go`
   - Tests: MCP server basics and session management
   - **Action:** HIGH PRIORITY - Core server functionality

4. **`pkg/mcp/internal/core/conversation_test.go`**
   - Missing: `pkg/mcp/internal/core/conversation.go`
   - **Action:** Likely needs integration with existing conversation code

5. **`pkg/mcp/internal/core/server_shutdown_test.go`**
   - Missing: `pkg/mcp/internal/core/server_shutdown.go`
   - **Action:** Critical for proper resource cleanup

6. **`pkg/mcp/internal/core/tool_integration_test.go`**
   - Missing: `pkg/mcp/internal/core/tool_integration.go`
   - **Action:** Essential for Sprint A tool orchestration

7. **`pkg/mcp/internal/core/tool_argument_mapping_test.go`**
   - Missing: `pkg/mcp/internal/core/tool_argument_mapping.go`
   - **Action:** Required for MCP protocol compliance

8. **`pkg/mcp/internal/core/schema_regression_test.go`**
   - Missing: `pkg/mcp/internal/core/schema_regression.go`
   - **Action:** Important for API stability

### Build & Deploy Tests (3 files) 🔧
**Priority: HIGH** - Sprint A dependencies

9. **`pkg/mcp/internal/build/integration_test.go`**
   - Missing: `pkg/mcp/internal/build/integration.go`
   - **Sprint A Relevance:** Build tool iterative fixing

10. **`pkg/mcp/internal/deploy/manifests_test.go`**
    - Missing: `pkg/mcp/internal/deploy/manifests.go`
    - **Sprint A Relevance:** Deploy tool iterative fixing

11. **`pkg/mcp/internal/analyze/repository_test.go`**
    - Missing: `pkg/mcp/internal/analyze/repository.go`
    - **Action:** May need integration with existing analyzer

### Orchestration Tests (3 files) 🔄
**Priority: HIGH** - Core workflow functionality

12. **`pkg/mcp/internal/orchestration/orchestration_test.go`**
    - Missing: `pkg/mcp/internal/orchestration/orchestration.go`
    - **Action:** Critical for tool coordination

13. **`pkg/mcp/internal/orchestration/benchmark_test.go`**
    - Missing: `pkg/mcp/internal/orchestration/benchmark.go`
    - **Action:** Performance testing for orchestration

### Conversation & Runtime Tests (4 files) 💬
**Priority: MEDIUM** - User experience features

14. **`pkg/mcp/internal/runtime/conversation/integration_test.go`**
15. **`pkg/mcp/internal/runtime/conversation/preflight_autorun_test.go`**
16. **`pkg/mcp/internal/runtime/conversation/prompt_manager_test.go`**
17. **`pkg/mcp/internal/runtime/conversation/welcome_stage_simple_test.go`**

### Transport & Protocol Tests (4 files) 🌐
**Priority: MEDIUM** - Communication infrastructure

18. **`pkg/mcp/internal/transport/http_logging_test.go`**
19. **`pkg/mcp/internal/transport/llm_e2e_test.go`**
20. **`pkg/mcp/internal/transport/stdio_error_test.go`**
21. **`pkg/mcp/internal/transport/stdio_mapping_test.go`**

### Observability Tests (3 files) 📊
**Priority: MEDIUM** - Monitoring and debugging

22. **`pkg/mcp/internal/observability/profiling_test.go`**
23. **`pkg/mcp/internal/observability/registry_integration_test.go`**
24. **`pkg/mcp/internal/observability/telemetry_token_internal_test.go`**

### Utility & Support Tests (3 files) 🛠️
**Priority: LOW** - Supporting functionality

25. **`pkg/mcp/internal/customizer/k8s_customizer_test.go`**
26. **`pkg/mcp/internal/testutil/example_test.go`**
27. **`pkg/mcp/internal/utils/integration_test.go`**
28. **`pkg/mcp/internal/types/types_test.go`** ⚠️

⚠️ **Special Case:** `types_test.go` - Multiple implementation files exist in directory, needs investigation

## Resolution Strategy

### Phase 1: Critical Infrastructure (Days 1-2)
**Target:** Files 1-8 (Core MCP functionality)
- **Implement:** mcp_server.go, conversation.go, tool_integration.go
- **Delete:** Obsolete tests that are no longer relevant
- **Relocate:** Tests that belong in different packages

### Phase 2: Build/Deploy Integration (Day 3)
**Target:** Files 9-11 (Sprint A coordination)
- Review against Sprint A requirements
- Implement missing integration code
- Ensure compatibility with automatic fixing features

### Phase 3: Orchestration (Day 4)
**Target:** Files 12-13 (Workflow coordination)
- Critical for cross-tool error escalation
- Implement orchestration patterns

### Phase 4: Communication & UX (Days 5-6)
**Target:** Files 14-21 (Transport and conversation)
- Focus on user-facing functionality
- Ensure robust MCP protocol handling

### Phase 5: Supporting Infrastructure (Day 7)
**Target:** Files 22-28 (Observability and utilities)
- Complete remaining test gaps
- Clean up utility functions

## Immediate Actions Required

### Delete Candidates 🗑️
Files that appear to test non-existent features:
- `no_external_ai_test.go` (if AI-free mode not required)
- `profiling_test.go` (if profiling not implemented)

### Implement Candidates ⚡
Critical missing implementations:
- `mcp_server.go` - Core server functionality
- `tool_integration.go` - Essential for Sprint A
- `orchestration.go` - Core workflow management

### Investigate Candidates 🔍
Files needing detailed analysis:
- `types_test.go` - Check if tests match existing implementations
- Integration tests - Determine if they should be moved to `/test/integration`

## Sprint Coordination

### Sprint A Dependencies
- `build/integration_test.go` - Build tool iterative fixing
- `deploy/manifests_test.go` - Deploy tool iterative fixing
- `core/tool_integration_test.go` - Cross-tool error escalation

### Sprint B Dependencies
- Security scanning tests should coordinate with scan package development

## Success Metrics
- [ ] 28 orphaned test files resolved (implement, delete, or relocate)
- [ ] All critical infrastructure tests (files 1-8) resolved
- [ ] Sprint A coordination tests functional
- [ ] No orphaned test files in final coverage report
- [ ] Test resolution contributes to 80% coverage target

**Next Steps:** Begin with Phase 1 critical infrastructure files and coordinate with Sprint A team on build/deploy test requirements.
