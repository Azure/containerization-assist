# Testing Guide

Container Kit maintains comprehensive testing standards to ensure reliability and quality. This guide covers testing practices, tools, and patterns used throughout the codebase.

## Testing Architecture

### Test Structure
```
pkg/mcp/
├── domain/             # Domain logic tests
│   ├── *_test.go      # Unit tests for business logic
│   └── integration/   # Domain integration tests
├── application/        # Application layer tests
│   ├── *_test.go      # Service orchestration tests
│   └── mocks/         # Service mocks for testing
└── infra/             # Infrastructure tests
    ├── *_test.go      # External integration tests
    └── testdata/      # Test fixtures and data
```

### Testing Commands

```bash
# Run all tests
make test-all

# Run MCP package tests only
make test

# Run MCP tests with build tags
make test-mcp

# Run performance benchmarks
make bench

# Generate coverage report
make coverage-html
```

## Test Categories

### 1. Unit Tests
**Purpose**: Test individual components in isolation

**Patterns**:
```go
func TestAnalyzeRepository(t *testing.T) {
    // Arrange
    mockFileAccess := &mocks.FileAccessService{}
    mockFileAccess.On("ReadFile", mock.Anything, "session-1", "go.mod").
        Return("module example.com/app", nil)
    
    // Act
    result, err := analyzeRepository(context.Background(), args, mockFileAccess)
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, "go", result.Language)
    mockFileAccess.AssertExpectations(t)
}
```

### 2. Integration Tests
**Purpose**: Test component interactions and workflows

**Service Integration**:
```go
func TestContainerizationWorkflow(t *testing.T) {
    // Setup service container with real implementations
    container := setupTestServiceContainer(t)
    
    // Test complete workflow
    analyzeResult := testAnalyzeStep(t, container)
    dockerfileResult := testGenerateStep(t, container, analyzeResult)
    buildResult := testBuildStep(t, container, dockerfileResult)
    
    // Verify end-to-end workflow
    assert.True(t, buildResult.Success)
}
```

### 3. MCP Protocol Tests
**Purpose**: Test MCP protocol compliance and tool behavior

**Tool Testing**:
```go
func TestMCPToolExecution(t *testing.T) {
    server := setupMCPServer(t)
    
    // Test tool registration
    tools := server.GetTools()
    assert.Contains(t, tools, "analyze_repository")
    
    // Test tool execution
    result, err := server.ExecuteTool("analyze_repository", args)
    require.NoError(t, err)
    assert.True(t, result.Success)
}
```

### 4. Performance Tests
**Purpose**: Validate performance requirements (<300μs P95)

**Benchmarks**:
```go
func BenchmarkAnalyzeRepository(b *testing.B) {
    container := setupBenchmarkContainer(b)
    args := &AnalyzeArgs{SessionID: "bench-session"}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := container.AnalyzeRepository(context.Background(), args)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Mock Strategy

### Service Mocking
**FileAccessService Mock**:
```go
type MockFileAccessService struct {
    mock.Mock
}

func (m *MockFileAccessService) ReadFile(ctx context.Context, sessionID, path string) (string, error) {
    args := m.Called(ctx, sessionID, path)
    return args.String(0), args.Error(1)
}

func (m *MockFileAccessService) ListDirectory(ctx context.Context, sessionID, path string) ([]FileInfo, error) {
    args := m.Called(ctx, sessionID, path)
    return args.Get(0).([]FileInfo), args.Error(1)
}
```

### Service Container Mock
```go
type MockServiceContainer struct {
    fileAccess FileAccessService
    sessionStore SessionStore
    // ... other services
}

func NewMockServiceContainer() *MockServiceContainer {
    return &MockServiceContainer{
        fileAccess: &MockFileAccessService{},
        sessionStore: &MockSessionStore{},
    }
}
```

## Test Data Management

### Test Fixtures
```go
// testdata/repositories/golang-simple/
├── go.mod
├── main.go
├── Dockerfile.expected
└── analysis.json

func loadTestRepository(t *testing.T, name string) *TestRepository {
    dataPath := filepath.Join("testdata", "repositories", name)
    return &TestRepository{
        Path: dataPath,
        Expected: loadExpectedResults(t, dataPath),
    }
}
```

### Session Management in Tests
```go
func setupTestSession(t *testing.T) (string, func()) {
    sessionID := "test-session-" + uuid.New().String()
    
    // Create isolated workspace
    workspaceDir := createTempWorkspace(t, sessionID)
    
    // Return cleanup function
    cleanup := func() {
        os.RemoveAll(workspaceDir)
    }
    
    return sessionID, cleanup
}
```

## Testing Patterns

### 1. Table-Driven Tests
```go
func TestLanguageDetection(t *testing.T) {
    testCases := []struct {
        name     string
        files    map[string]string
        expected string
    }{
        {
            name: "golang project",
            files: map[string]string{
                "go.mod": "module example.com/app",
                "main.go": "package main",
            },
            expected: "go",
        },
        {
            name: "nodejs project",
            files: map[string]string{
                "package.json": `{"name": "app"}`,
                "index.js": "console.log('hello')",
            },
            expected: "nodejs",
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 2. Error Testing
```go
func TestErrorHandling(t *testing.T) {
    mockService := &MockFileAccessService{}
    mockService.On("ReadFile", mock.Anything, mock.Anything, mock.Anything).
        Return("", errors.New("file not found"))
    
    result, err := analyzeRepository(context.Background(), args, mockService)
    
    // Test error is properly handled
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "file not found")
    assert.Nil(t, result)
}
```

### 3. Concurrent Testing
```go
func TestConcurrentSessions(t *testing.T) {
    container := setupTestServiceContainer(t)
    
    // Create multiple sessions concurrently
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(sessionID string) {
            defer wg.Done()
            
            // Test concurrent operations
            result, err := container.AnalyzeRepository(context.Background(), &AnalyzeArgs{
                SessionID: sessionID,
            })
            
            assert.NoError(t, err)
            assert.True(t, result.Success)
        }(fmt.Sprintf("session-%d", i))
    }
    
    wg.Wait()
}
```

## Test Configuration

### Environment Setup
```go
func setupTestEnvironment(t *testing.T) {
    // Set test environment variables
    os.Setenv("CONTAINER_KIT_ENV", "test")
    os.Setenv("CONTAINER_KIT_LOG_LEVEL", "error")
    
    // Cleanup after test
    t.Cleanup(func() {
        os.Unsetenv("CONTAINER_KIT_ENV")
        os.Unsetenv("CONTAINER_KIT_LOG_LEVEL")
    })
}
```

### Test Database Setup
```go
func setupTestDB(t *testing.T) (*bolt.DB, func()) {
    tmpFile, err := os.CreateTemp("", "test-*.db")
    require.NoError(t, err)
    tmpFile.Close()
    
    db, err := bolt.Open(tmpFile.Name(), 0600, nil)
    require.NoError(t, err)
    
    cleanup := func() {
        db.Close()
        os.Remove(tmpFile.Name())
    }
    
    return db, cleanup
}
```

## Test Quality Standards

### Coverage Requirements
- **Minimum Coverage**: 80% for new code
- **Critical Paths**: 100% coverage for core workflows
- **Integration Points**: Full coverage of service boundaries

### Performance Benchmarks
```bash
# Set performance baseline
make bench-baseline

# Run benchmarks with target
make bench

# Expected targets:
# - Tool execution: <300μs P95
# - File operations: <100μs P95
# - Session operations: <50μs P95
```

### Test Reliability
- **Flaky Test Policy**: Zero tolerance for flaky tests
- **Test Isolation**: Each test must be independent
- **Cleanup**: Proper cleanup of resources and state

## Continuous Integration

### GitHub Actions Integration
```yaml
name: Test Suite
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24.1'
      - run: make test-all
      - run: make bench
      - run: make coverage-html
```

### Quality Gates
- **All tests pass**: Required for merge
- **Coverage maintained**: No decrease in coverage
- **Performance benchmarks**: Must meet P95 targets
- **Lint compliance**: <100 issues maximum

## Debugging Tests

### Test Debugging
```go
func TestWithDebugLogging(t *testing.T) {
    // Enable debug logging for test
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))
    
    ctx := context.WithValue(context.Background(), "logger", logger)
    
    // Run test with debug context
    result, err := analyzeRepository(ctx, args, mockService)
    
    // Debug output will show detailed execution
    require.NoError(t, err)
}
```

### Test Data Inspection
```go
func TestDataInspection(t *testing.T) {
    result, err := analyzeRepository(context.Background(), args, mockService)
    require.NoError(t, err)
    
    // Pretty print result for inspection
    resultJSON, _ := json.MarshalIndent(result, "", "  ")
    t.Logf("Analysis result: %s", resultJSON)
}
```

## Common Issues and Solutions

### 1. Test Isolation
**Problem**: Tests interfere with each other
**Solution**: Use unique session IDs and proper cleanup

### 2. Mock Complexity
**Problem**: Complex mocking requirements
**Solution**: Use test builders and factory patterns

### 3. Performance Test Variability
**Problem**: Inconsistent benchmark results
**Solution**: Use proper benchmark setup and statistical analysis

### 4. Integration Test Reliability
**Problem**: Tests fail due to external dependencies
**Solution**: Use test containers and proper mocking

## Best Practices

1. **Test Names**: Use descriptive names that explain what is being tested
2. **Test Structure**: Follow Arrange-Act-Assert pattern
3. **Mock Usage**: Mock external dependencies, not internal logic
4. **Test Data**: Use realistic test data and edge cases
5. **Performance**: Include performance tests for critical paths
6. **Cleanup**: Always clean up resources and state
7. **Documentation**: Document complex test scenarios

## Related Documentation

- [Quality Standards](../../contributing/code-standards.md)
- [Error Handling](../developer/error-handling.md)
- [Service Container](../../architecture/service-container.md)
- [Tool Development](../developer/adding-new-tools.md)

Container Kit's testing strategy ensures high quality, reliability, and performance through comprehensive coverage and rigorous standards.