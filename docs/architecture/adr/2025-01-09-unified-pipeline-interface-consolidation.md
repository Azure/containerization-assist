# ADR-009: Unified Pipeline Interface Consolidation

## Status
**Accepted** - January 9, 2025

## Context

### Current State
Container Kit currently has three separate pipeline implementations that evolved independently, creating maintenance overhead and inconsistent behavior patterns:

#### 1. Orchestration Pipeline
```go
// pkg/mcp/application/orchestration/pipeline.go
type OrchestrationPipeline struct {
    stages []OrchestrationStage
    config OrchestrationConfig
    // 15+ fields for various concerns
}

func (p *OrchestrationPipeline) Execute(ctx context.Context, input interface{}) (interface{}, error) {
    // Complex orchestration logic with timeout handling
    // Error recovery and retry mechanisms
    // Metrics collection and reporting
}
```

#### 2. Atomic Pipeline
```go
// pkg/mcp/application/orchestration/pipeline/atomic/pipeline.go
type AtomicPipeline struct {
    operations []AtomicOperation
    transaction TransactionManager
    // 8+ fields for atomic semantics
}

func (p *AtomicPipeline) RunAtomic(ctx context.Context, ops []Operation) error {
    // Atomic execution with rollback capability
    // All-or-nothing semantics
    // Transaction management
}
```

#### 3. Workflow Pipeline
```go
// pkg/mcp/application/workflows/pipeline.go
type WorkflowPipeline struct {
    steps []WorkflowStep
    parallel bool
    // 6+ fields for workflow management
}

func (p *WorkflowPipeline) RunWorkflow(ctx context.Context, def WorkflowDefinition) (*WorkflowResult, error) {
    // Sequential or parallel step execution
    // Workflow state management
    // Step dependency resolution
}
```

### Problems with Current Approach

#### 1. Interface Fragmentation
- **Different Method Names**: `Execute()`, `RunAtomic()`, `RunWorkflow()`
- **Inconsistent Parameters**: Different input/output types
- **Separate Error Handling**: 3 different error patterns
- **Configuration Chaos**: 29+ total configuration fields across implementations

#### 2. Code Duplication
- **Retry Logic**: Similar retry patterns in all 3 implementations
- **Timeout Handling**: Duplicated timeout management
- **Context Propagation**: Inconsistent context handling
- **Metrics Collection**: 3 separate metrics implementations

#### 3. Testing Complexity
- **Mock Generation**: 3 separate mock interfaces needed
- **Test Patterns**: Different testing approaches for each pipeline
- **Integration Testing**: Complex cross-pipeline integration scenarios

#### 4. Developer Confusion
- **API Selection**: Unclear which pipeline to use when
- **Feature Overlap**: Similar capabilities with different APIs
- **Documentation Burden**: 3 separate documentation sets

### Usage Analysis
```bash
# Current pipeline usage across codebase
$ grep -r "OrchestrationPipeline\|AtomicPipeline\|WorkflowPipeline" pkg/mcp/
# Results: 47 files using orchestration, 23 using atomic, 31 using workflow
# Total: 101 usage points with inconsistent patterns
```

## Decision

**We will consolidate the 3 pipeline implementations into a single unified Pipeline interface** with specialized implementations that share common behavior patterns.

### Unified Pipeline Interface

#### Core Interface Definition
```go
// pkg/mcp/application/api/interfaces.go
type Pipeline interface {
    // Execute runs the pipeline with unified semantics
    Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error)

    // Builder methods for fluent API
    AddStage(stage PipelineStage) Pipeline
    WithTimeout(timeout time.Duration) Pipeline
    WithRetry(policy RetryPolicy) Pipeline
    WithMetrics(collector MetricsCollector) Pipeline
}

// Unified request/response types
type PipelineRequest struct {
    Input    interface{}
    Metadata map[string]interface{}
    Options  *PipelineOptions
}

type PipelineResponse struct {
    Output   interface{}
    Metadata map[string]interface{}
    Metrics  *PipelineMetrics
}

// Stage abstraction for all pipeline types
type PipelineStage interface {
    Name() string
    Execute(ctx context.Context, input interface{}) (interface{}, error)
    Validate(input interface{}) error
}
```

#### Implementation Strategy

##### 1. Atomic Pipeline (Transactional Semantics)
```go
// pkg/mcp/application/pipeline/atomic.go
type AtomicPipeline struct {
    stages []PipelineStage
    transaction TransactionManager
}

func (p *AtomicPipeline) Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error) {
    // Begin transaction
    tx := p.transaction.Begin()
    defer tx.Rollback() // Auto-rollback on error

    var result interface{} = request.Input

    // Execute stages atomically
    for _, stage := range p.stages {
        var err error
        result, err = stage.Execute(ctx, result)
        if err != nil {
            return nil, err // Automatic rollback
        }
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return nil, err
    }

    return &PipelineResponse{
        Output: result,
        Metadata: map[string]interface{}{
            "type": "atomic",
            "transactional": true,
        },
    }, nil
}
```

##### 2. Workflow Pipeline (Sequential/Parallel Semantics)
```go
// pkg/mcp/application/pipeline/workflow.go
type WorkflowPipeline struct {
    stages []PipelineStage
    parallel bool
}

func (p *WorkflowPipeline) Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error) {
    if p.parallel {
        return p.executeParallel(ctx, request)
    }
    return p.executeSequential(ctx, request)
}

func (p *WorkflowPipeline) executeParallel(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error) {
    // Parallel execution with sync.WaitGroup
    var wg sync.WaitGroup
    results := make([]interface{}, len(p.stages))
    errors := make([]error, len(p.stages))

    for i, stage := range p.stages {
        wg.Add(1)
        go func(idx int, s PipelineStage) {
            defer wg.Done()
            result, err := s.Execute(ctx, request.Input)
            results[idx] = result
            errors[idx] = err
        }(i, stage)
    }

    wg.Wait()

    // Collect results and check for errors
    for _, err := range errors {
        if err != nil {
            return nil, err
        }
    }

    return &PipelineResponse{
        Output: results,
        Metadata: map[string]interface{}{
            "type": "workflow",
            "parallel": true,
        },
    }, nil
}
```

##### 3. Orchestration Pipeline (Full-Featured Semantics)
```go
// pkg/mcp/application/pipeline/orchestration.go
type OrchestrationPipeline struct {
    stages []PipelineStage
    timeout time.Duration
    retryPolicy RetryPolicy
    metrics MetricsCollector
}

func (p *OrchestrationPipeline) Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error) {
    // Apply timeout
    if p.timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, p.timeout)
        defer cancel()
    }

    var result interface{} = request.Input

    // Execute stages with retry and metrics
    for _, stage := range p.stages {
        start := time.Now()
        var err error

        // Execute with retry policy
        for attempt := 0; attempt <= p.retryPolicy.MaxAttempts; attempt++ {
            result, err = stage.Execute(ctx, result)
            if err == nil {
                break
            }

            if attempt < p.retryPolicy.MaxAttempts {
                time.Sleep(p.retryPolicy.BackoffDuration)
            }
        }

        // Record metrics
        if p.metrics != nil {
            p.metrics.RecordStageExecution(stage.Name(), time.Since(start), err)
        }

        if err != nil {
            return nil, err
        }
    }

    return &PipelineResponse{
        Output: result,
        Metadata: map[string]interface{}{
            "type": "orchestration",
            "timeout": p.timeout.String(),
            "retries": p.retryPolicy.MaxAttempts,
        },
    }, nil
}
```

### Builder Pattern Implementation

#### Fluent API for Pipeline Construction
```go
// pkg/mcp/application/pipeline/builder.go
type PipelineBuilder interface {
    // Pipeline type selection
    Atomic() PipelineBuilder
    Workflow(parallel bool) PipelineBuilder
    Orchestration() PipelineBuilder

    // Common configuration
    WithStages(stages ...PipelineStage) PipelineBuilder
    WithTimeout(timeout time.Duration) PipelineBuilder
    WithRetry(policy RetryPolicy) PipelineBuilder
    WithMetrics(collector MetricsCollector) PipelineBuilder

    // Build final pipeline
    Build() Pipeline
}

// Usage example
pipeline := pipeline.New().
    Orchestration().
    WithStages(validateStage, transformStage, executeStage).
    WithTimeout(30 * time.Second).
    WithRetry(RetryPolicy{MaxAttempts: 3, BackoffDuration: 1 * time.Second}).
    WithMetrics(metricsCollector).
    Build()
```

### Migration Strategy

#### Phase 1: Interface Unification (Week 4, Days 1-5)
- Create unified Pipeline interface
- Implement basic atomic, workflow, and orchestration variants
- Establish builder pattern for pipeline construction

#### Phase 2: Legacy Migration (Week 5, Days 6-10)
- Migrate existing OrchestrationPipeline usage
- Migrate existing AtomicPipeline usage
- Migrate existing WorkflowPipeline usage

#### Phase 3: Code Removal (Week 5, Days 11-15)
- Remove old pipeline implementations
- Clean up unused imports and types
- Update documentation

#### Phase 4: Optimization (Week 6, Days 16-20)
- Performance tuning for unified interface
- Advanced features (caching, stage composition)
- Comprehensive testing

## Consequences

### Positive Outcomes

#### 1. Simplified API Surface
- **Single Interface**: `Pipeline` instead of 3 separate interfaces
- **Consistent Methods**: Unified `Execute()` method signature
- **Fluent API**: Builder pattern for intuitive pipeline construction
- **Reduced Complexity**: 101 usage points → Single consistent pattern

#### 2. Code Consolidation
- **Shared Logic**: Common retry, timeout, and metrics handling
- **Reduced Duplication**: 29+ configuration fields → Single configuration approach
- **Unified Testing**: Single test pattern for all pipeline types
- **Maintenance Reduction**: 3 implementations → 1 interface + 3 specializations

#### 3. Developer Experience
- **Clear API**: Obvious pipeline type selection through builder
- **Consistent Behavior**: Predictable error handling and metrics
- **Better Documentation**: Single comprehensive documentation set
- **Easier Testing**: Unified mocking and testing patterns

#### 4. Performance Benefits
- **Reduced Overhead**: Single interface dispatch instead of 3 separate systems
- **Shared Resources**: Common connection pools and caches
- **Optimized Execution**: Specialized implementations for specific use cases

### Negative Outcomes

#### 1. Migration Complexity
- **Breaking Changes**: All existing pipeline usage must be updated
- **Testing Overhead**: 101 usage points need test updates
- **Risk of Regression**: Behavioral changes during migration
- **Timeline Impact**: 4-week migration effort

#### 2. Abstraction Overhead
- **Interface Indirection**: Slight performance overhead from interface dispatch
- **Complexity Hidden**: Specialized behavior less obvious
- **Learning Curve**: Developers need to understand new builder pattern

#### 3. Behavioral Changes
- **Subtle Differences**: Unified interface may change edge case behavior
- **Configuration Migration**: Existing configuration needs translation
- **Error Handling**: Unified error patterns may differ from legacy

### Mitigation Strategies

#### 1. Gradual Migration
```go
// Compatibility layer during migration
type LegacyOrchestrationPipeline struct {
    unified Pipeline
}

func (l *LegacyOrchestrationPipeline) Execute(ctx context.Context, input interface{}) (interface{}, error) {
    request := &PipelineRequest{Input: input}
    response, err := l.unified.Execute(ctx, request)
    if err != nil {
        return nil, err
    }
    return response.Output, nil
}
```

#### 2. Comprehensive Testing
```go
// Migration test suite
func TestPipelineMigration(t *testing.T) {
    // Test atomic pipeline migration
    t.Run("atomic_migration", func(t *testing.T) {
        // Compare legacy vs unified behavior
    })

    // Test workflow pipeline migration
    t.Run("workflow_migration", func(t *testing.T) {
        // Compare legacy vs unified behavior
    })

    // Test orchestration pipeline migration
    t.Run("orchestration_migration", func(t *testing.T) {
        // Compare legacy vs unified behavior
    })
}
```

#### 3. Performance Monitoring
```go
// Performance regression testing
func BenchmarkPipelinePerformance(b *testing.B) {
    // Baseline: legacy pipeline performance
    // Target: unified pipeline performance
    // Ensure no regression during migration
}
```

## Success Metrics

### API Simplification
- **Interface Reduction**: 3 pipeline interfaces → 1 unified interface
- **Method Consolidation**: 3 different execute methods → 1 `Execute()` method
- **Configuration Reduction**: 29+ fields → Single configuration approach
- **Usage Consistency**: 101 usage points → Consistent pattern

### Performance Targets
- **Execution Time**: <300μs P95 for pipeline operations (no regression)
- **Memory Usage**: ≤10% increase for interface overhead
- **Throughput**: Maintain or improve current pipeline throughput

### Quality Metrics
- **Test Coverage**: >80% for unified pipeline system
- **Bug Reports**: <5 migration-related issues
- **Documentation**: Single comprehensive pipeline documentation set

## Alternatives Considered

### Option 1: Keep Separate Implementations
- **Pros**: No migration effort, preserved behavior
- **Cons**: Continued maintenance overhead, developer confusion

### Option 2: Single Implementation with Modes
- **Pros**: Maximum code sharing, single implementation
- **Cons**: Complex configuration, unclear behavior modes

### Option 3: Unified Interface with Specialized Implementations (Chosen)
- **Pros**: Clear API, specialized behavior, shared patterns
- **Cons**: Migration effort, abstraction overhead

### Option 4: Complete Rewrite
- **Pros**: Clean slate, optimal design
- **Cons**: Extreme risk, potential behavior changes

## Implementation Timeline

### Week 4: Interface Design (Days 1-5)
- [ ] Create unified Pipeline interface
- [ ] Implement builder pattern
- [ ] Create specialized implementations
- [ ] Basic integration testing

### Week 5: Migration (Days 6-15)
- [ ] Migrate OrchestrationPipeline usage
- [ ] Migrate AtomicPipeline usage
- [ ] Migrate WorkflowPipeline usage
- [ ] Remove legacy implementations

### Week 6: Optimization (Days 16-20)
- [ ] Performance tuning
- [ ] Advanced features
- [ ] Comprehensive testing
- [ ] Documentation updates

## References

- [Pipeline Design Patterns](https://martinfowler.com/articles/collection-pipeline/)
- [Builder Pattern in Go](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
- [Interface Segregation Principle](https://en.wikipedia.org/wiki/Interface_segregation_principle)
- [Container Kit Three-Layer Architecture](../THREE_LAYER_ARCHITECTURE.md)

## Revision History

| Date | Author | Changes |
|------|--------|---------|
| 2025-01-09 | Claude | Initial ADR creation |

---

**Note**: This ADR represents a significant API consolidation that will simplify the Container Kit pipeline system while maintaining specialized behavior for different use cases. The unified interface approach balances simplicity with functionality.
