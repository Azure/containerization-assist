# MCP Integration Testing Implementation Plan

## Problem Analysis

Our recent debugging revealed critical gaps in test coverage that allowed multiple production issues to slip through:

1. **Session Type Assertion Failures**: Runtime type mismatches not caught by unit tests
2. **Session Continuity Issues**: Tools creating new sessions instead of reusing existing ones
3. **Tool Description Gaps**: AI clients not receiving proper workflow instructions
4. **Cross-Tool State Management**: Session state not properly shared between workflow tools
5. **Import Consistency**: Type import conflicts causing runtime failures despite clean builds

**Root Cause**: Current tests focus on individual tool functionality but miss the critical integration points and real MCP protocol interactions.

## Current Test Coverage Gaps

### What We Have
- Unit tests for individual tool components
- Mocked dependencies and interfaces  
- Basic server lifecycle tests
- Session manager functionality tests

### What We're Missing
- **End-to-End MCP Protocol Tests**: Real gomcp server/client interactions
- **Multi-Tool Workflow Tests**: Session continuity across tool chains
- **Session State Validation**: Actual session sharing and state persistence
- **Tool Description Parsing**: How AI clients interpret our tool schemas
- **Type System Integration**: Runtime type assertion validation
- **Real Tool Execution**: Actual tool invocation through MCP protocol

## Implementation Plan

### Phase 1: MCP Protocol Integration Test Framework

#### 1.1 Real MCP Client Test Infrastructure
```go
// pkg/mcp/internal/test/integration/mcp_client.go
type MCPIntegrationTestSuite struct {
    server     *core.Server
    client     *gomcp.Client
    tempDir    string
    serverAddr string
}

func (suite *MCPIntegrationTestSuite) SetupSuite() {
    // Start real MCP server with HTTP transport
    // Create real gomcp client 
    // Setup test workspace directories
}
```

**Key Features:**
- Real HTTP transport (not mocked stdio)
- Actual gomcp client library usage
- Real session persistence (BoltDB)
- Temporary but persistent workspaces
- Server startup/shutdown lifecycle

#### 1.2 Tool Schema Validation Tests
```go
func TestToolSchemaIntegration(t *testing.T) {
    // Test that tool descriptions contain session management instructions
    // Validate parameter schemas match expected types
    // Ensure required fields are properly marked
    // Test tool discovery through MCP protocol
}
```

### Phase 2: Multi-Tool Workflow Integration Tests

#### 2.1 Complete Containerization Workflow Test
```go
func TestCompleteContainerizationWorkflow(t *testing.T) {
    client := setupMCPClient(t)
    
    // Step 1: analyze_repository
    analyzeResult := client.CallTool("analyze_repository", map[string]interface{}{
        "repo_url": "https://github.com/example/java-app",
        "branch": "main",
    })
    
    // Validate session_id is returned
    sessionID := analyzeResult["session_id"].(string)
    require.NotEmpty(t, sessionID)
    
    // Step 2: generate_dockerfile with session continuity
    dockerfileResult := client.CallTool("generate_dockerfile", map[string]interface{}{
        "session_id": sessionID,  // Critical: use same session
        "template": "java",
    })
    
    // Validate same session used
    assert.Equal(t, sessionID, dockerfileResult["session_id"])
    
    // Step 3: build_image with session continuity
    buildResult := client.CallTool("build_image", map[string]interface{}{
        "session_id": sessionID,
        "image_name": "test-app",
        "tag": "latest",
    })
    
    // Validate session continuity and state sharing
    assert.Equal(t, sessionID, buildResult["session_id"])
    assert.True(t, buildResult["success"].(bool))
    
    // Validate workspace persistence
    workspace := getSessionWorkspace(t, sessionID)
    assert.FileExists(t, filepath.Join(workspace, "Dockerfile"))
    
    // Step 4: generate_manifests
    manifestResult := client.CallTool("generate_manifests", map[string]interface{}{
        "session_id": sessionID,
        "app_name": "test-app",
        "port": 8080,
    })
    
    // Validate full workflow state
    validateWorkflowCompletion(t, sessionID, manifestResult)
}
```

#### 2.2 Session State Validation Tests
```go
func TestSessionStateSharing(t *testing.T) {
    // Test that repository analysis results are available to dockerfile generation
    // Test that dockerfile path is available to build step
    // Test that image reference is available to manifest generation
    // Test that session metadata persists across tool calls
}

func TestSessionWorkspaceManagement(t *testing.T) {
    // Test workspace creation and persistence
    // Test file sharing between tools in same session
    // Test workspace cleanup on session deletion
    // Test workspace isolation between different sessions
}
```

#### 2.3 Error Handling and Recovery Tests
```go
func TestWorkflowErrorRecovery(t *testing.T) {
    // Test workflow continuation after non-fatal errors
    // Test session state recovery after server restart
    // Test handling of invalid session IDs
    // Test tool execution with missing dependencies
}
```

### Phase 3: Session Management Integration Tests

#### 3.1 Session Type System Tests
```go
func TestSessionTypeConsistency(t *testing.T) {
    // Test that GetOrCreateSession returns correct type
    // Test type assertions don't fail at runtime
    // Test session interface implementations
    // Test import consistency across packages
}

func TestSessionManagerIntegration(t *testing.T) {
    // Test session creation through MCP tools
    // Test session retrieval and updates
    // Test session persistence and loading
    // Test concurrent session access
}
```

#### 3.2 Cross-Package Type Integration
```go
func TestTypeImportConsistency(t *testing.T) {
    // Test that all packages use consistent session types
    // Test interface implementations across package boundaries
    // Test type alias resolution
    // Test reflection-based type inspection
}
```

### Phase 4: AI Client Behavior Simulation Tests

#### 4.1 Tool Description Interpretation Tests
```go
func TestAIClientBehaviorSimulation(t *testing.T) {
    // Simulate how Claude would interpret tool descriptions
    // Test that session_id requirements are clear
    // Test workflow instruction parsing
    // Test parameter requirement understanding
}

func TestToolDiscoveryAndUsage(t *testing.T) {
    // Test tool listing through MCP protocol
    // Test parameter schema interpretation
    // Test required vs optional parameter handling
    // Test error message clarity for AI clients
}
```

### Phase 5: Performance and Reliability Tests

#### 5.1 Session Performance Tests
```go
func TestSessionPerformance(t *testing.T) {
    // Test session creation/retrieval performance
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
```

### Phase 6: Real-World Scenario Tests

#### 6.1 Repository Integration Tests
```go
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
            // Validate generated Dockerfile
            // Validate successful build
        })
    }
}
```

## Test Implementation Structure

### Directory Structure
```
pkg/mcp/internal/test/
├── integration/
│   ├── mcp_client_test.go          # Real MCP client tests
│   ├── workflow_integration_test.go # Multi-tool workflow tests
│   ├── session_integration_test.go  # Session management tests
│   ├── type_integration_test.go     # Type system tests
│   └── testutil/
│       ├── mcp_test_client.go      # Test client utilities
│       ├── test_repositories.go    # Test repo fixtures
│       └── workflow_helpers.go     # Workflow test helpers
├── e2e/
│   ├── real_repository_test.go     # Real repo integration
│   ├── performance_test.go         # Performance benchmarks
│   └── reliability_test.go         # Long-running tests
└── fixtures/
    ├── repositories/               # Test repository fixtures
    ├── expected_results/          # Expected tool outputs
    └── test_configs/              # Test server configurations
```

### Test Categories and Tagging

```go
// Build tags for different test types
//go:build integration
//go:build e2e
//go:build performance

// Test categories
func TestWorkflowIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    // ... test implementation
}
```

### Continuous Integration Integration

#### Makefile Targets
```makefile
# Add to existing Makefile
.PHONY: test-integration test-e2e test-performance

test-integration:
	go test -tags=integration ./pkg/mcp/internal/test/integration/... -v

test-e2e:
	go test -tags=e2e ./pkg/mcp/internal/test/e2e/... -v -timeout=30m

test-performance:
	go test -tags=performance ./pkg/mcp/internal/test/e2e/... -v -bench=. -timeout=60m

test-all-integration: test-integration test-e2e

# Update existing test target
test: test-unit test-integration
```

#### GitHub Actions Integration
```yaml
# Add to .github/workflows/test.yml
- name: Run Integration Tests
  run: make test-integration
  
- name: Run E2E Tests (on main branch)
  if: github.ref == 'refs/heads/main'
  run: make test-e2e
```

## Success Metrics

### Coverage Goals
- **Integration Test Coverage**: 80% of MCP tool interactions
- **Workflow Coverage**: 100% of documented user workflows
- **Session Management**: 95% of session lifecycle scenarios
- **Error Handling**: 90% of error conditions and recovery paths

### Quality Gates
1. **No Runtime Type Assertion Failures**: All type assertions must be validated by tests
2. **Session Continuity**: All multi-tool workflows must maintain session state
3. **Tool Description Completeness**: All workflow tools must have clear session requirements
4. **Performance Benchmarks**: Session operations must meet performance targets (<300μs P95)

### Test Execution Strategy
- **Pre-commit**: Fast unit tests + critical integration tests
- **CI Pipeline**: Full integration test suite
- **Nightly**: E2E tests with real repositories
- **Release**: Performance and reliability test suite

## Implementation Timeline

### Week 1-2: Foundation
- Create MCP client test infrastructure
- Implement basic workflow integration tests
- Set up test data fixtures and utilities

### Week 3-4: Core Integration Tests
- Multi-tool workflow tests
- Session state validation tests
- Type system integration tests

### Week 5-6: Advanced Scenarios
- Error handling and recovery tests
- Performance and reliability tests
- AI client behavior simulation

### Week 7-8: Real-World Integration
- Real repository integration tests
- Performance benchmarking
- CI/CD integration and documentation

## Risk Mitigation

### Potential Issues
1. **Test Environment Complexity**: Real MCP client/server setup
   - **Mitigation**: Docker-based test environments, test utilities
2. **Test Data Management**: Repository fixtures and expected results
   - **Mitigation**: Version-controlled fixtures, automated result validation
3. **Test Execution Time**: E2E tests may be slow
   - **Mitigation**: Parallel execution, smart test selection, caching

### Success Validation
- Run new tests against current codebase to verify they catch known issues
- Validate tests catch the specific problems we recently fixed
- Ensure tests provide clear failure diagnostics for future debugging

This comprehensive testing strategy would have caught all the issues we recently encountered and will prevent similar problems in the future by testing the actual MCP protocol interactions and multi-tool workflows that our users depend on.