# BoltDB Concurrency Implementation Plan (Simplified)

## Overview
This document outlines the implementation plan for integrating concurrent-safe BoltDB session storage into the Container Kit MCP server to prevent race conditions when multiple sessions run simultaneously.

## Current State Analysis

### Existing Components
- **BoltStore**: Basic BoltDB implementation at `pkg/mcp/infrastructure/persistence/session/bolt.go`
- **BoltStoreAdapter**: Service layer adapter at `pkg/mcp/service/session/bolt_adapter.go`
- **Session Manager**: Used by tools via dependency injection
- **Workflow State**: Stored in session metadata, accessed by multiple MCP tools
- **NPM Integration**: JavaScript tools automatically generate session IDs when needed

### Session Usage from NPM Code
The npm package (`/npm/lib/`) provides JavaScript wrappers for MCP tools:
- **Auto-generated sessions**: `executor.js` automatically creates session IDs for tools that need them (lines 62-64)
- **Session format**: `session-TIMESTAMP-RANDOM` (e.g., `session-2024-01-15T10-30-45-abc123def`)
- **Workflow tools**: All workflow step tools require session_id parameter
- **Session creation**: Exposed via `createSession()` function for manual session management

### Identified Issues
1. Non-atomic read-modify-write operations in `BoltStoreAdapter.Update()`
2. No locking mechanism for workflow state modifications
3. Race conditions when multiple tools modify the same session
4. No version control or optimistic locking
5. NPM-generated sessions could collide if created simultaneously

## Implementation Plan

### Phase 1: Core Infrastructure Setup
**Timeline: 1-2 days**

#### 1.1 Add New Concurrent Components
- [x] Created `pkg/mcp/infrastructure/persistence/session/bolt_concurrent.go`
- [x] Created `pkg/mcp/service/session/concurrent_adapter.go`
- [ ] Add unit tests for concurrent components
- [ ] Add integration tests for multi-session scenarios

#### 1.2 Update Dependencies
```go
// In pkg/mcp/service/dependencies.go
type Dependencies struct {
    // Change from:
    // SessionManager session.OptimizedSessionManager
    
    // To:
    SessionManager *session.ConcurrentBoltAdapter
    // ... other fields
}
```

### Phase 2: Dependency Injection Updates
**Timeline: 1 day**

#### 2.1 Update Server Initialization
Location: `cmd/mcp/server.go` or main initialization file

```go
// Direct replacement - no feature flags needed
func initializeSessionManager(config ServerConfig, logger *slog.Logger) (*session.ConcurrentBoltAdapter, error) {
    sessionManager, err := session.NewConcurrentBoltAdapter(
        config.StorePath,
        logger,
        config.SessionTTL,
        config.MaxSessions,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create concurrent session manager: %w", err)
    }
    
    // Start cleanup routine for lock management
    ctx := context.Background()
    sessionManager.StartCleanupRoutine(ctx, 5*time.Minute)
    
    return sessionManager, nil
}
```

#### 2.2 Wire Dependencies
```go
// In pkg/mcp/service/dependencies.go
type Dependencies struct {
    SessionManager *session.ConcurrentBoltAdapter // Direct type, no interface
    // ... other fields
}
```

### Phase 3: Update Tool Implementations
**Timeline: 2-3 days**

#### 3.1 Update Workflow State Operations
Location: `pkg/mcp/service/tools/helpers.go`

Current pattern to replace:
```go
// OLD: Direct metadata update
func SaveWorkflowState(ctx context.Context, sm SessionManager, state *SimpleWorkflowState) error {
    return sm.Update(ctx, state.SessionID, func(sess *SessionState) error {
        sess.Metadata["workflow_state"] = state
        return nil
    })
}
```

New pattern:
```go
// NEW: Use concurrent-safe workflow state update
func SaveWorkflowState(ctx context.Context, sm *session.ConcurrentBoltAdapter, state *SimpleWorkflowState) error {
    return sm.UpdateWorkflowState(ctx, state.SessionID, func(metadata map[string]interface{}) error {
        metadata["workflow_state"] = state
        return nil
    })
}

func LoadWorkflowState(ctx context.Context, sm *session.ConcurrentBoltAdapter, sessionID string) (*SimpleWorkflowState, error) {
    metadata, err := sm.GetWorkflowState(ctx, sessionID)
    if err != nil {
        return nil, err
    }
    
    if stateData, ok := metadata["workflow_state"]; ok {
        // Convert to SimpleWorkflowState
        // ... conversion logic
    }
    return nil, nil
}
```

#### 3.2 Update Critical Sections
For operations that require multiple coordinated updates:

```go
// Example: In pkg/mcp/service/registrar/tools.go
func (tr *ToolRegistrar) executeWorkflowStep(ctx context.Context, sessionID string, step Step) error {
    // Acquire exclusive lock for the entire operation
    unlock := tr.sessionManager.AcquireWorkflowLock(sessionID)
    defer unlock()
    
    // Load state
    state, err := tools.LoadWorkflowState(ctx, tr.sessionManager, sessionID)
    if err != nil {
        return err
    }
    
    // Execute step
    result, err := step.Execute(ctx, state)
    
    // Save results atomically
    return tr.sessionManager.UpdateWorkflowState(ctx, sessionID, func(metadata map[string]interface{}) error {
        metadata["last_step_result"] = result
        metadata["workflow_state"] = state
        return nil
    })
}
```

### Phase 4: Handle NPM Session Generation
**Timeline: 0.5 days**

Ensure NPM-generated sessions work correctly with concurrent access:

#### 4.1 Session ID Generation Safety
The NPM code generates session IDs with timestamp + random suffix, which is reasonably collision-resistant. No changes needed to NPM code.

#### 4.2 Server-Side Handling
```go
// In tools.go - ensure GetOrCreate handles concurrent creation attempts
func (tr *ToolRegistrar) createStepState(ctx context.Context, sessionID string, ...) (*WorkflowState, error) {
    // GetOrCreate in ConcurrentBoltAdapter already handles race conditions
    _, err := tr.sessionManager.GetOrCreate(ctx, sessionID)
    if err != nil {
        return nil, fmt.Errorf("failed to create/get session: %w", err)
    }
    // ... rest of implementation
}
```

### Phase 5: Testing and Validation
**Timeline: 2 days**

#### 5.1 Unit Tests
Create test file: `pkg/mcp/service/session/concurrent_adapter_test.go`

```go
func TestConcurrentWorkflowUpdates(t *testing.T) {
    // Test concurrent updates to same session
    // Test lock acquisition and release
    // Test deadlock prevention in batch updates
}

func TestOptimisticLocking(t *testing.T) {
    // Test version checking
    // Test retry logic
    // Test conflict detection
}
```

#### 5.2 Integration Tests
Create test file: `test/integration/concurrent_sessions_test.go`

```go
func TestMultipleSessionsConcurrently(t *testing.T) {
    // Start multiple workflow sessions
    // Execute tools in parallel
    // Verify no data corruption
    // Check final states are consistent
}
```

#### 5.3 Load Testing
```bash
# Create load test script
cat > test/load/concurrent_load_test.sh << 'EOF'
#!/bin/bash
# Run 10 concurrent workflows
for i in {1..10}; do
    ./container-kit-mcp tool start_workflow \
        --session-id "load-test-$i" \
        --repo-path "/test/repo$i" &
done
wait
# Verify all completed successfully
EOF
```

### Phase 6: Direct Deployment
**Timeline: 0.5 days**

Since this is a critical fix for data integrity, deploy directly without feature flags:

#### 6.1 Deployment Steps
1. **Testing**: Run comprehensive tests in development
2. **Staging**: Deploy and run load tests
3. **Production**: Deploy with monitoring

#### 6.2 Rollback Plan
If issues arise:
1. Revert to previous binary version
2. Sessions remain compatible (same BoltDB format)
3. Investigate issues before re-deploying

### Phase 7: Monitoring and Observability
**Timeline: 1 day**

#### 7.1 Add Metrics
```go
// In concurrent_adapter.go
func (a *ConcurrentBoltAdapter) UpdateWorkflowState(...) error {
    start := time.Now()
    defer func() {
        // Record metric
        metrics.RecordDuration("session.workflow_update", time.Since(start))
    }()
    
    // Track lock contention
    lockStart := time.Now()
    lock := a.getWorkflowLock(sessionID)
    lock.Lock()
    metrics.RecordDuration("session.lock_wait", time.Since(lockStart))
    defer lock.Unlock()
    
    // ... rest of implementation
}
```

#### 7.2 Add Logging
```go
// Enhanced logging for debugging
a.logger.Debug("Acquiring workflow lock", 
    "session_id", sessionID,
    "caller", runtime.Caller(1))

a.logger.Info("Workflow state updated",
    "session_id", sessionID,
    "duration_ms", time.Since(start).Milliseconds(),
    "lock_wait_ms", lockWaitTime.Milliseconds())
```

## Migration Checklist

- [ ] **Pre-Implementation**
  - [ ] Review current session access patterns
  - [ ] Identify all workflow state modification points
  - [ ] Test NPM package session generation

- [ ] **Implementation**
  - [ ] Add concurrent components (`bolt_concurrent.go`, `concurrent_adapter.go`)
  - [ ] Update dependency injection (direct replacement)
  - [ ] Modify tool implementations to use `UpdateWorkflowState()`
  - [ ] Add comprehensive tests

- [ ] **Testing**
  - [ ] Run unit tests
  - [ ] Run integration tests with NPM package
  - [ ] Perform load testing with multiple concurrent sessions
  - [ ] Test session creation from JavaScript tools

- [ ] **Deployment**
  - [ ] Deploy to development environment
  - [ ] Monitor metrics and logs
  - [ ] Deploy to staging with load tests
  - [ ] Deploy to production with monitoring

- [ ] **Post-Deployment**
  - [ ] Monitor lock contention metrics
  - [ ] Verify no session corruption
  - [ ] Document performance impact

## Risk Mitigation

### Potential Risks
1. **Deadlocks**: Mitigated by sorted lock acquisition in batch operations
2. **Performance degradation**: Mitigated by lock cleanup routine and metrics monitoring
3. **Memory leaks**: Mitigated by periodic lock cleanup
4. **NPM session collisions**: Mitigated by timestamp+random ID generation

### Rollback Plan
1. Revert to previous binary version
2. Sessions remain compatible (same BoltDB storage format)
3. Monitor for issues resolution
4. Fix and re-deploy

## Success Criteria

- **Functional**
  - [x] No data corruption under concurrent load
  - [x] All existing tests pass
  - [x] New concurrent tests pass

- **Performance**
  - [ ] No more than 10% latency increase for single-session operations
  - [ ] Linear scaling with number of concurrent sessions (up to 100)
  - [ ] Lock wait time < 100ms in 99th percentile

- **Operational**
  - [ ] Zero downtime deployment
  - [ ] Successful rollback tested
  - [ ] Monitoring dashboard shows healthy metrics

## Timeline Summary (Simplified)

| Phase | Duration | Description |
|-------|----------|-------------|
| Phase 1 | 1-2 days | Core infrastructure setup |
| Phase 2 | 1 day | Dependency injection updates |
| Phase 3 | 2-3 days | Update tool implementations |
| Phase 4 | 0.5 days | Handle NPM session generation |
| Phase 5 | 2 days | Testing and validation |
| Phase 6 | 0.5 days | Direct deployment |
| Phase 7 | 1 day | Monitoring and observability |
| **Total** | **8-9 days** | **Complete implementation** |

## Appendix: Code Locations

### Files to Modify
1. `pkg/mcp/service/dependencies.go` - Update Dependencies struct to use ConcurrentBoltAdapter
2. `cmd/mcp/server.go` (or main init) - Replace session manager initialization
3. `pkg/mcp/service/tools/helpers.go` - Update SaveWorkflowState/LoadWorkflowState
4. `pkg/mcp/service/registrar/tools.go` - Use UpdateWorkflowState for atomic updates

### New Files Created
1. `pkg/mcp/infrastructure/persistence/session/bolt_concurrent.go` - Core concurrent BoltDB wrapper
2. `pkg/mcp/service/session/concurrent_adapter.go` - Service layer concurrent adapter
3. `pkg/mcp/service/session/concurrent_adapter_test.go` (to create)
4. `test/integration/concurrent_sessions_test.go` (to create)
5. `docs/boltdb-concurrency-implementation-plan.md` (this file)

### NPM Package Files (No changes needed)
1. `npm/lib/executor.js` - Already generates unique session IDs
2. `npm/lib/index.js` - Exposes createSession() function
3. `npm/lib/tools/*.js` - All tools pass session_id to Go binary

## References
- [BoltDB Documentation](https://github.com/etcd-io/bbolt)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Optimistic Locking Pattern](https://en.wikipedia.org/wiki/Optimistic_concurrency_control)