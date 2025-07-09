# Migration Plan: gomcp Library to Consolidated Commands

## Overview

This plan migrates from the current gomcp library pattern to consolidated commands using the `api.Tool` interface while maintaining full compatibility with the existing gomcp server infrastructure.

## Key Insight: No Adapter Layer Needed

The migration uses **direct integration** - gomcp handlers call consolidated commands directly, converting input/output formats as needed. This approach:

- ✅ Reuses existing consolidated command logic
- ✅ Maintains gomcp server compatibility  
- ✅ No complex adapter layer
- ✅ Can migrate tools incrementally
- ✅ Easy rollback per tool

## Phase 1: Infrastructure (Days 1-2)

### 1.1 Create Integration Layer
**Files**: `pkg/mcp/application/core/tool_integration.go`

```go
// Helper functions for gomcp ↔ api.Tool conversion
func convertGomcpToToolInput(input map[string]interface{}) api.ToolInput
func convertToolOutputToGomcp(output api.ToolOutput) map[string]interface{}
func createToolWrapper(tool api.Tool) func(*server.Context, map[string]interface{}) (map[string]interface{}, error)
```

### 1.2 Set Up Dependency Management
**Files**: `pkg/mcp/application/core/consolidated_tools.go`

```go
// Initialize consolidated tools with proper dependencies
func (s *serverImpl) initializeConsolidatedTools() []api.Tool
func (s *serverImpl) registerAllConsolidatedTools()
```

### 1.3 Service Container Integration
**Goal**: Inject dependencies into consolidated commands

**Current State**: Dependencies are created ad-hoc in each tool
**Target State**: Dependencies injected via service container

```go
// Service container provides all dependencies
type ServiceContainer interface {
    SessionStore() SessionStore
    SessionState() SessionState
    DockerClient() DockerClient
    AnalysisEngine() *analysis.Engine
    // ... other services
}
```

## Phase 2: Tool-by-Tool Migration (Days 3-7)

### 2.1 Migration Pattern

Each tool follows this pattern:

```go
// Before: gomcp function with typed parameters
s.server.Tool("analyze_repository", "description", 
    func(_ *server.Context, args *AnalyzeArgs) (*AnalyzeResponse, error) {
        // Direct implementation
    })

// After: gomcp function calling consolidated command
s.server.Tool("analyze_repository", "description",
    func(_ *server.Context, input map[string]interface{}) (map[string]interface{}, error) {
        // Convert input
        toolInput := convertGomcpToToolInput(input)
        
        // Call consolidated command
        output, err := s.analyzeCmd.Execute(context.Background(), toolInput)
        
        // Convert output
        return convertToolOutputToGomcp(output), err
    })
```

### 2.2 Tool Migration Order

**Priority 1 (Core Tools)**:
1. `analyze_repository` - Most complex, good test case
2. `build_image` - High usage, critical path
3. `generate_manifests` - Complex parameters

**Priority 2 (Secondary Tools)**:
4. `push_image` - Depends on build_image
5. `scan_image` - Independent functionality  
6. `generate_dockerfile` - May be merged with analyze

**Priority 3 (Simple Tools)**:
7. `list_sessions` - Simple, good for validation
8. `ping` - Trivial, easy win
9. `server_status` - Simple, low risk

### 2.3 Migration Steps Per Tool

For each tool:

1. **Verify consolidated command exists**
   - Check `pkg/mcp/application/commands/{tool}_consolidated.go`
   - Ensure it implements `api.Tool` interface

2. **Create dependencies**
   - Identify what services the consolidated command needs
   - Wire them through service container

3. **Replace gomcp registration**
   - Keep the same tool name and description
   - Replace typed function with generic map function
   - Call consolidated command inside wrapper

4. **Test compatibility**
   - Ensure same input/output format
   - Run existing tests
   - Validate MCP protocol compliance

5. **Performance validation**
   - Check execution time matches or improves
   - Monitor memory usage
   - Validate error handling

## Phase 3: Service Container Integration (Days 8-9)

### 3.1 Replace Ad-hoc Dependencies

**Current Pattern**:
```go
// Each tool creates its own dependencies
analyzer := analysis.NewRepositoryAnalyzer(logger)
```

**Target Pattern**:
```go
// Service container provides dependencies
analyzeCmd := commands.NewConsolidatedAnalyzeCommand(
    container.SessionStore(),
    container.SessionState(), 
    container.Logger(),
    container.AnalysisEngine(),
)
```

### 3.2 Implement Service Container

**Files**: `pkg/mcp/application/services/container.go`

```go
type ServiceContainer struct {
    sessionStore   SessionStore
    sessionState   SessionState
    dockerClient   DockerClient
    analysisEngine *analysis.Engine
    logger         *slog.Logger
}

func NewServiceContainer(config Config) *ServiceContainer {
    return &ServiceContainer{
        sessionStore:   NewSessionStore(config.SessionConfig),
        sessionState:   NewSessionState(config.StateConfig),
        dockerClient:   NewDockerClient(config.DockerConfig),
        analysisEngine: analysis.NewRepositoryAnalyzer(logger),
        logger:         logger,
    }
}
```

### 3.3 Update Server Initialization

**Files**: `pkg/mcp/application/core/server_impl.go`

```go
func NewServer(config ServerConfig) *serverImpl {
    // Create service container
    container := services.NewServiceContainer(config)
    
    // Create server with container
    server := &serverImpl{
        container: container,
        // ... other fields
    }
    
    // Register tools using container
    server.registerAllConsolidatedTools()
    
    return server
}
```

## Phase 4: Testing & Validation (Days 10-12)

### 4.1 Test Coverage

**Unit Tests**:
- Each consolidated command works independently
- Service container provides correct dependencies
- Input/output conversion is accurate

**Integration Tests**:
- Full MCP protocol compliance
- All tools work end-to-end
- Error handling matches expectations

**Performance Tests**:
- Tool execution time < 300μs P95
- Memory usage within bounds
- No performance regression

### 4.2 Validation Checklist

For each migrated tool:

- [ ] Tool registration succeeds
- [ ] Input parameters match original format
- [ ] Output format matches original
- [ ] Error handling works correctly
- [ ] Session management works
- [ ] Performance meets targets
- [ ] Tests pass
- [ ] Documentation updated

## Phase 5: Cleanup (Days 13-14)

### 5.1 Remove Legacy Code

**Remove**:
- Old gomcp function implementations
- Duplicate parameter validation
- Ad-hoc dependency creation

**Keep**:
- Consolidated commands (they're the new implementation)
- Service container and dependencies
- Integration layer (for future tools)

### 5.2 Update Documentation

**Update**:
- `MCP_TOOL_STANDARDS.md` - Document new pattern
- `TOOL_GUIDE.md` - Update usage examples
- `ADDING_NEW_TOOLS.md` - Show consolidated command pattern

## Benefits of This Approach

### 1. **Incremental Migration**
- Migrate one tool at a time
- Easy rollback if issues occur
- No big-bang deployment

### 2. **No Breaking Changes**
- Same tool names and descriptions
- Same input/output format
- Same MCP protocol compliance

### 3. **Better Architecture**
- Consolidated commands are more testable
- Service container enables proper DI
- Cleaner separation of concerns

### 4. **Future-Proof**
- Easy to add new tools using consolidated pattern
- Service container enables middleware, validation, etc.
- Foundation for advanced features

## Risk Mitigation

### 1. **Rollback Plan**
Each tool can be rolled back independently:
```go
// Easy to switch back to original implementation
s.server.Tool("analyze_repository", "description", originalFunction)
```

### 2. **Gradual Deployment**
- Deploy one tool at a time
- Monitor performance and errors
- Validate compatibility before proceeding

### 3. **Testing Strategy**
- Comprehensive test suite for each tool
- Integration tests for full workflow
- Performance benchmarks

### 4. **Monitoring**
- Track tool execution time
- Monitor error rates
- Watch memory usage

## Implementation Timeline

| Phase | Duration | Key Deliverables |
|-------|----------|------------------|
| 1. Infrastructure | 2 days | Integration layer, service container |
| 2. Tool Migration | 5 days | All 9 tools migrated |
| 3. Service Container | 2 days | Full dependency injection |
| 4. Testing | 3 days | Comprehensive validation |
| 5. Cleanup | 2 days | Legacy code removal |

**Total: 14 days**

## Success Criteria

- [ ] All 9 tools migrated to consolidated commands
- [ ] No breaking changes to MCP protocol
- [ ] Performance targets met (<300μs P95)
- [ ] Test coverage maintained or improved
- [ ] Documentation updated
- [ ] Service container fully integrated
- [ ] Legacy code removed

This migration plan provides a safe, incremental path to the consolidated command architecture while maintaining full compatibility with the existing gomcp infrastructure.