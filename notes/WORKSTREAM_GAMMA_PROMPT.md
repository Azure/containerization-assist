# WORKSTREAM GAMMA: MCP Testing Infrastructure
**AI Assistant Prompt - Container Kit MCP Cleanup**

## 🎯 MISSION OVERVIEW

You are the **Integration Testing Specialist** responsible for creating comprehensive MCP testing infrastructure that catches the critical integration issues we recently discovered. Your tests will validate real MCP protocol interactions and multi-tool workflows.

**Duration**: Week 3-4 (10 days)
**Dependencies**: WORKSTREAM ALPHA + BETA completion (validation + type safety)
**Critical Success**: Production-ready testing framework preventing integration failures

## 📋 YOUR SPECIFIC RESPONSIBILITIES

### Week 3 (Days 11-15): Foundation & Integration Tests

#### Day 11-12: MCP Client Test Infrastructure (CRITICAL)
```bash
# WAIT: Until WORKSTREAM ALPHA complete AND WORKSTREAM BETA Week 2 merged

# Create real MCP test framework (not mocked):
mkdir -p pkg/mcp/internal/test/{integration,e2e,fixtures}

# File 1: pkg/mcp/internal/test/integration/mcp_client_test.go
# Real MCP client integration:
type MCPIntegrationTestSuite struct {
    server     *core.Server
    client     *gomcp.Client      // Real gomcp client, not mock
    tempDir    string
    serverAddr string
    httpServer *httptest.Server   // Real HTTP transport
}

func (suite *MCPIntegrationTestSuite) SetupSuite() {
    // Start real MCP server with HTTP transport
    // Create real gomcp client connection
    // Setup test workspace directories with BoltDB
    // Validate server startup and client connection
}

# File 2: pkg/mcp/internal/test/integration/tool_schema_test.go
# Tool schema validation:
func TestToolSchemaIntegration(t *testing.T) {
    // Test that tool descriptions contain session management instructions
    // Validate parameter schemas match RichError types (from BETA)
    // Ensure required fields are properly marked
    // Test tool discovery through MCP protocol
}

# File 3: pkg/mcp/internal/test/testutil/mcp_test_client.go
# Test client utilities:
# - Real MCP client connection helpers
# - Session state inspection utilities
# - Workspace management for tests
# - Tool execution helpers with error validation

# VALIDATION REQUIRED:
go test -tags=integration ./pkg/mcp/internal/test/integration/... && echo "✅ MCP test foundation ready"
go fmt ./pkg/mcp/internal/test/...
```

#### Day 13-14: Multi-Tool Workflow Tests (HIGH PRIORITY)
```bash
# Complete containerization workflow tests:

# File 1: pkg/mcp/internal/test/integration/workflow_integration_test.go
# End-to-end workflow validation:
func TestCompleteContainerizationWorkflow(t *testing.T) {
    client := setupMCPClient(t)

    // Step 1: analyze_repository - MUST return session_id
    analyzeResult := client.CallTool("analyze_repository", map[string]interface{}{
        "repo_url": "https://github.com/example/java-app",
        "branch": "main",
    })

    sessionID := analyzeResult["session_id"].(string)
    require.NotEmpty(t, sessionID, "analyze_repository must return session_id")

    // Step 2: generate_dockerfile - MUST use same session
    dockerfileResult := client.CallTool("generate_dockerfile", map[string]interface{}{
        "session_id": sessionID,  // CRITICAL: session continuity
        "template": "java",
    })

    assert.Equal(t, sessionID, dockerfileResult["session_id"], "session_id must be preserved")

    // Step 3: build_image - MUST use same session
    buildResult := client.CallTool("build_image", map[string]interface{}{
        "session_id": sessionID,
        "image_name": "test-app",
        "tag": "latest",
    })

    // CRITICAL: Validate session continuity AND state sharing
    assert.Equal(t, sessionID, buildResult["session_id"])
    assert.True(t, buildResult["success"].(bool))

    // Validate workspace persistence (files exist across tools)
    workspace := getSessionWorkspace(t, sessionID)
    assert.FileExists(t, filepath.Join(workspace, "Dockerfile"))

    // Step 4: generate_manifests - complete workflow
    manifestResult := client.CallTool("generate_manifests", map[string]interface{}{
        "session_id": sessionID,
        "app_name": "test-app",
        "port": 8080,
    })

    validateWorkflowCompletion(t, sessionID, manifestResult)
}

# File 2: pkg/mcp/internal/test/integration/session_integration_test.go
# Session state validation:
func TestSessionStateSharing(t *testing.T) {
    // Test repository analysis results available to dockerfile generation
    // Test dockerfile path available to build step
    // Test image reference available to manifest generation
    // Test session metadata persists across tool calls
}

func TestSessionWorkspaceManagement(t *testing.T) {
    // Test workspace creation and persistence
    // Test file sharing between tools in same session
    // Test workspace cleanup on session deletion
    // Test workspace isolation between different sessions
}

# VALIDATION REQUIRED:
go test -tags=integration ./pkg/mcp/internal/test/integration/... && echo "✅ Workflow tests complete"
```

#### Day 15: Session Management & Type Integration
```bash
# Session type system tests:

# File 1: pkg/mcp/internal/test/integration/session_type_test.go
# Type consistency validation:
func TestSessionTypeConsistency(t *testing.T) {
    // Test GetOrCreateSession returns correct type (no interface{})
    // Test type assertions don't fail at runtime (use BETA's strong types)
    // Test session interface implementations
    // Test import consistency across packages
}

func TestSessionManagerIntegration(t *testing.T) {
    // Test session creation through MCP tools
    // Test session retrieval and updates
    // Test session persistence and loading (BoltDB)
    // Test concurrent session access
}

# File 2: pkg/mcp/internal/test/integration/type_integration_test.go
# Cross-package type integration:
func TestTypeImportConsistency(t *testing.T) {
    // Test all packages use consistent session types
    // Test interface implementations across package boundaries
    // Test type alias resolution
    // Test RichError integration (from BETA) in tool responses
}

# CHECKPOINT VALIDATION:
go test -tags=integration ./pkg/mcp/internal/test/integration/...

# COMMIT AND PAUSE:
git add .
git commit -m "feat(testing): implement MCP integration testing framework

- Created real MCP client/server test infrastructure with HTTP transport
- Added comprehensive multi-tool workflow integration tests
- Implemented session management and type consistency validation
- Created test utilities for MCP protocol interactions

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"

# PAUSE POINT: Wait for external merge before Week 4
```

### Week 4 (Days 16-20): Advanced Testing & CI Integration

#### Day 16-17: AI Client Behavior & Error Recovery
```bash
# WAIT: Until Week 3 changes merged and WORKSTREAM BETA complete

# AI client simulation and error handling tests:

# File 1: pkg/mcp/internal/test/e2e/ai_client_simulation_test.go
# AI client behavior tests:
func TestAIClientBehaviorSimulation(t *testing.T) {
    // Simulate how Claude interprets tool descriptions
    // Test session_id requirements are clear in tool descriptions
    // Test workflow instruction parsing and understanding
    // Test parameter requirement comprehension
}

func TestToolDiscoveryAndUsage(t *testing.T) {
    // Test tool listing through MCP protocol
    // Test parameter schema interpretation (use BETA's generic types)
    // Test required vs optional parameter handling
    // Test error message clarity for AI clients (use BETA's RichError)
}

# File 2: pkg/mcp/internal/test/e2e/error_recovery_test.go
# Error handling and recovery:
func TestWorkflowErrorRecovery(t *testing.T) {
    // Test workflow continuation after non-fatal errors
    // Test session state recovery after server restart
    // Test handling of invalid session IDs
    // Test tool execution with missing dependencies
    // VALIDATE: RichError context helps with recovery
}

# File 3: pkg/mcp/internal/test/e2e/session_recovery_test.go
# Session persistence and recovery:
func TestSessionPersistenceRecovery(t *testing.T) {
    // Test session state survives server restart
    // Test workspace files persist across server cycles
    // Test in-progress workflows can be resumed
    // Test session cleanup after failures
}

# VALIDATION REQUIRED:
go test -tags=e2e ./pkg/mcp/internal/test/e2e/... && echo "✅ E2E tests complete"
```

#### Day 18-19: Performance & Real-World Scenarios
```bash
# Performance and real repository tests:

# File 1: pkg/mcp/internal/test/e2e/performance_test.go
# Performance benchmarks:
func TestSessionPerformance(t *testing.T) {
    // Test session creation/retrieval performance (<300μs P95)
    // Test concurrent session handling
    // Test session cleanup performance
    // Test workspace disk usage limits
}

func TestLongRunningWorkflows(t *testing.T) {
    // Test workflows with multiple tools over time
    // Test session TTL and expiration handling
    // Test session persistence across server restarts
    // Test memory usage over long sessions
}

# File 2: pkg/mcp/internal/test/e2e/real_repository_test.go
# Real repository integration:
func TestRealRepositoryIntegration(t *testing.T) {
    testCases := []struct{
        name string
        repoURL string
        expectedLanguage string
        expectedFramework string
    }{
        {"Java Maven", "https://github.com/spring-projects/spring-petclinic", "java", "maven"},
        {"Node.js Express", "https://github.com/expressjs/express", "javascript", "npm"},
        {"Python Flask", "https://github.com/pallets/flask", "python", "pip"},
        {"Go Module", "https://github.com/gin-gonic/gin", "go", "go-modules"},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Run complete workflow on real repository
            // Validate analysis accuracy
            // Validate generated Dockerfile works
            // Validate successful build
        })
    }
}

# VALIDATION REQUIRED:
go test -tags=performance -bench=. ./pkg/mcp/internal/test/e2e/...
```

#### Day 20: CI/CD Integration & Documentation
```bash
# Complete testing infrastructure:

# File 1: Update Makefile
# Add new test targets:
.PHONY: test-integration test-e2e test-performance test-all-integration

test-integration:
	go test -tags=integration ./pkg/mcp/internal/test/integration/... -v

test-e2e:
	go test -tags=e2e ./pkg/mcp/internal/test/e2e/... -v -timeout=30m

test-performance:
	go test -tags=performance ./pkg/mcp/internal/test/e2e/... -v -bench=. -timeout=60m

test-all-integration: test-integration test-e2e

# Update existing test target
test: test-unit test-integration

# File 2: .github/workflows/test.yml updates
# GitHub Actions integration:
- name: Run Integration Tests
  run: make test-integration

- name: Run E2E Tests (on main branch)
  if: github.ref == 'refs/heads/main'
  run: make test-e2e

# File 3: pkg/mcp/internal/test/README.md
# Testing documentation:
# - Test execution strategy
# - How to run different test types
# - Test data management
# - CI/CD integration guide

# File 4: Performance baseline establishment:
# Create performance baseline data for future comparisons

# FINAL VALIDATION:
make test-all-integration && echo "✅ GAMMA WORKSTREAM COMPLETE"

# FINAL COMMIT:
git add .
git commit -m "feat(testing): complete MCP testing infrastructure

- Implemented comprehensive AI client behavior simulation
- Added performance benchmarking framework
- Created real repository integration tests
- Integrated CI/CD pipeline with new test categories
- Established performance baselines and monitoring

GAMMA WORKSTREAM COMPLETE ✅

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"
```

## 🎯 SUCCESS CRITERIA

### Must Achieve (100% Required):
- ✅ **80% MCP protocol interaction coverage** - Real MCP client/server tests
- ✅ **100% documented user workflow testing** - All workflows have integration tests
- ✅ **95% session lifecycle scenario coverage** - Session creation → cleanup
- ✅ **Real-world repository integration** - Tests work with actual GitHub repos
- ✅ **Performance benchmarking framework** - <300μs P95 session operations
- ✅ **CI/CD integration** - Automated test execution in GitHub Actions

### Quality Gates (Enforce Strictly):
```bash
# REQUIRED before each commit:
go test -tags=integration ./pkg/mcp/internal/test/integration/...  # Integration tests
go test -tags=e2e ./pkg/mcp/internal/test/e2e/...                 # E2E tests
go test -short ./pkg/mcp/...                                      # Unit tests still pass
go fmt ./pkg/mcp/internal/test/...                                # Code formatting
go build ./pkg/mcp/...                                            # Must compile

# PERFORMANCE validation:
go test -tags=performance -bench=. ./pkg/mcp/internal/test/e2e/... | grep "ns/op"
# Session operations must be <300μs P95
```

### Daily Validation Commands
```bash
# Morning startup:
go test -short ./pkg/mcp/... && echo "✅ Unit tests still pass"

# After MCP client implementation:
go test -tags=integration ./pkg/mcp/internal/test/integration/... && echo "✅ MCP integration working"

# After workflow tests:
go test -tags=integration -run="TestCompleteContainerizationWorkflow" ./... && echo "✅ End-to-end workflows working"

# After session tests:
go test -tags=integration -run="TestSession" ./... && echo "✅ Session management working"

# After performance tests:
go test -tags=performance -bench=. ./pkg/mcp/internal/test/e2e/... && echo "✅ Performance benchmarks working"

# End of day:
make test-all-integration && echo "✅ All integration systems functional"
```

## 🚨 CRITICAL COORDINATION POINTS

### Dependencies You Need:
- **WORKSTREAM ALPHA** unified validation system - MUST be complete for error testing
- **WORKSTREAM BETA** RichError + generics - MUST be complete for type-safe testing
- External merge of ALPHA + BETA changes - Wait for clean merged branch

### Dependencies on Your Work:
- **All other workstreams** depend on your tests to validate their changes work correctly
- **Production deployment** depends on your integration tests passing
- **Future development** depends on your testing framework

### Files You Own (Full Authority):
- `pkg/mcp/internal/test/` (entire directory tree) - You create the testing infrastructure
- `Makefile` test targets - You add new test categories
- `.github/workflows/test.yml` - You update CI/CD for integration tests
- Performance baselines - You establish and maintain benchmarks

### Files to Coordinate On:
- Any file with tool implementations - You need to test their integration
- Session management files - You need to validate their behavior
- Transport layer files - You need to test MCP protocol compliance

## 📊 PROGRESS TRACKING

### Daily Metrics to Track:
```bash
# Integration test coverage:
go test -tags=integration -cover ./pkg/mcp/internal/test/integration/... | grep "coverage:"

# E2E test coverage:
go test -tags=e2e -cover ./pkg/mcp/internal/test/e2e/... | grep "coverage:"

# Test execution performance:
go test -tags=integration ./pkg/mcp/internal/test/integration/... | grep "PASS"

# Session workflow validation:
go test -tags=integration -run="TestCompleteContainerizationWorkflow" ./... -v

# Real repository test success:
go test -tags=e2e -run="TestRealRepositoryIntegration" ./... -v
```

### Daily Summary Format:
```
WORKSTREAM GAMMA - DAY X SUMMARY
================================
Progress: X% complete
Integration test coverage: X%
E2E test coverage: X%
Performance benchmarks: X tests implemented

Tests implemented today:
- TestCompleteContainerizationWorkflow (session continuity)
- TestSessionTypeConsistency (type safety validation)
- [other tests]

Critical issues caught:
- [any integration failures discovered]
- [session continuity problems found]
- [type safety issues identified]

Test categories completed:
- ✅ MCP protocol integration
- ✅ Multi-tool workflows
- ✅ Session management
- ✅ Type system validation
- ✅ Performance benchmarking

Issues encountered:
- [any blockers or test failures]

Coordination needed:
- [validation requests for other workstreams]

Tomorrow's focus:
- [next test categories]

Quality status: X/Y test suites passing ✅
Performance status: Session ops <Xμs (target: <300μs)
```

## 🛡️ ERROR HANDLING & ROLLBACK

### If Things Go Wrong:
1. **Tests fail consistently**: Check real MCP server/client setup
2. **Session tests fail**: Validate BoltDB persistence setup
3. **Performance regression**: Review test environment and baseline data
4. **Integration conflicts**: Coordinate with other workstreams on fixes

### Rollback Procedure:
```bash
# Emergency rollback:
git checkout HEAD~1 -- pkg/mcp/internal/test/
git checkout HEAD~1 -- Makefile
git checkout HEAD~1 -- .github/workflows/test.yml

# Selective rollback:
git checkout HEAD~1 -- <specific-problematic-test-file>
```

## 🎯 KEY IMPLEMENTATION PATTERNS

### Real MCP Client Test Pattern:
```go
func TestWithRealMCPClient(t *testing.T) {
    // Setup real HTTP server
    server := httptest.NewServer(/* real MCP handler */)
    defer server.Close()

    // Create real gomcp client
    client, err := gomcp.NewClient(server.URL)
    require.NoError(t, err)
    defer client.Close()

    // Test actual MCP protocol
    result, err := client.CallTool("analyze_repository", params)
    require.NoError(t, err)

    // Validate result using strong types from BETA
    assert.IsType(t, &analyze.AnalyzeResult{}, result)
}
```

### Session Continuity Test Pattern:
```go
func TestSessionContinuity(t *testing.T) {
    client := setupMCPClient(t)

    // Step 1: Get session ID from first tool
    result1 := client.CallTool("analyze_repository", params)
    sessionID := extractSessionID(t, result1)

    // Step 2: Use same session ID in second tool
    params2 := addSessionID(params2, sessionID)
    result2 := client.CallTool("generate_dockerfile", params2)

    // CRITICAL: Validate session continuity
    assert.Equal(t, sessionID, extractSessionID(t, result2))

    // Validate shared state between tools
    validateSharedWorkspace(t, sessionID)
}
```

### Performance Benchmark Pattern:
```go
func BenchmarkSessionOperations(b *testing.B) {
    client := setupMCPClient(b)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Time session creation
        start := time.Now()
        result := client.CallTool("analyze_repository", params)
        duration := time.Since(start)

        // Validate performance target (<300μs P95)
        if duration > 300*time.Microsecond {
            b.Errorf("Session operation took %v, exceeds 300μs target", duration)
        }
    }
}
```

## 🎯 FINAL DELIVERABLES

At completion, you must deliver:

1. **Real MCP testing framework** (`pkg/mcp/internal/test/`) with HTTP transport
2. **Comprehensive workflow integration tests** validating session continuity
3. **Session management validation** ensuring state sharing works
4. **Type system integration tests** validating BETA's strong typing
5. **Performance benchmarking framework** with <300μs P95 targets
6. **Real repository integration tests** with multiple language/framework combinations
7. **CI/CD integration** with automated test execution
8. **Performance baselines** for future regression detection

**Remember**: Your testing framework is the safety net for all other workstreams. Focus on catching the real integration issues that slip through unit tests, especially session continuity and MCP protocol compliance! 🚀

---

**CRITICAL**: Stop work and create summary at end of each day. Do not attempt merges - external coordination will handle branch management. Your job is to create comprehensive integration tests that catch real-world MCP usage issues.
