# Workstream C: Progress & Operation Consolidation - Detailed Implementation Plan

## Executive Summary

**Objective**: Consolidate 6 adapter/wrapper files (657 lines total) into 2 unified implementations, eliminating duplication while preserving functionality.

**Duration**: 4 days (can start immediately after Workstream A Day 1)  
**Team Size**: 2 developers  
**Expected Reduction**: -657 lines, -6 files → +~200 lines, +2 files = **Net -457 lines**

## Target Files (Priority Order)

| Priority | File Type | Files | Lines | Strategy |
|----------|-----------|-------|--------|----------|
| **1** | Progress Adapters | 3 files | 370 lines | Consolidate → 1 unified implementation |
| **2** | Operation Wrappers | 3 files | 287 lines | Consolidate → 1 generic wrapper |

### **Progress Adapters (Priority 1)**
- `pkg/mcp/types/progress_adapter.go` (138 lines)
- `pkg/mcp/internal/gomcp_progress_adapter.go` (132 lines)  
- `pkg/mcp/internal/runtime/gomcp_progress_adapter.go` (100 lines)

### **Operation Wrappers (Priority 2)**
- `pkg/mcp/internal/build/pull_operation_wrapper.go` (96 lines)
- `pkg/mcp/internal/build/push_operation_wrapper.go` (93 lines)
- `pkg/mcp/internal/build/tag_operation_wrapper.go` (98 lines)

## Day-by-Day Implementation Plan

### **Day 1: Progress Adapter Analysis and Design**

#### **Morning (4 hours): Current State Analysis**

**Step 1.1: Analyze Progress Adapter Duplication**
```bash
# Identify all progress adapter files
find pkg/mcp -name "*progress_adapter*.go" | grep -v test

# Expected results:
# pkg/mcp/types/progress_adapter.go
# pkg/mcp/internal/gomcp_progress_adapter.go  
# pkg/mcp/internal/runtime/gomcp_progress_adapter.go

# Analyze usage patterns
grep -r "GoMCPProgressAdapter" pkg/mcp/internal/
grep -r "ProgressAdapter" pkg/mcp/internal/

# Expected: Used in 12+ atomic tool files
```

**Step 1.2: Compare Implementation Patterns**
```go
// Current Pattern 1: types/progress_adapter.go
type GoMCPProgressAdapter struct {
    serverCtx *server.Context
    token     string
    stages    []LocalProgressStage
    current   int
}

// Current Pattern 2: internal/gomcp_progress_adapter.go  
type GoMCPProgressAdapter struct {
    serverCtx *server.Context
    stages    []mcp.ProgressStage
    current   int
    logger    zerolog.Logger
}

// Current Pattern 3: runtime/gomcp_progress_adapter.go
type GoMCPProgressAdapter struct {
    reporter  ProgressReporter
    stages    []LocalProgressStage
    progress  map[string]float64
}

// PROBLEM: 3 different implementations doing similar work!
```

**Step 1.3: Design Unified Progress Implementation**
```go
// NEW: Single unified implementation using core interfaces
// File: pkg/mcp/internal/observability/progress.go

package observability

import (
    "context"
    "fmt"
    "time"
    "github.com/Azure/container-kit/pkg/mcp/core"
    "github.com/rs/zerolog"
)

type UnifiedProgressReporter struct {
    serverCtx *server.Context // GoMCP integration
    stages    map[core.ProgressToken]*progressState
    logger    zerolog.Logger
    mutex     sync.RWMutex
}

type progressState struct {
    stage       *core.ProgressStage
    startTime   time.Time
    lastUpdate  time.Time
    progress    int
}

func NewUnifiedProgressReporter(serverCtx *server.Context) core.ProgressReporter {
    return &UnifiedProgressReporter{
        serverCtx: serverCtx,
        stages:    make(map[core.ProgressToken]*progressState),
        logger:    zerolog.New(os.Stderr).With().Str("component", "progress").Logger(),
    }
}
```

#### **Afternoon (4 hours): Core Interface Implementation**

**Step 1.4: Implement Core ProgressReporter Interface**
```go
// Implement all core.ProgressReporter methods
func (p *UnifiedProgressReporter) StartStage(stage string) core.ProgressToken {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    token := core.ProgressToken(fmt.Sprintf("stage_%d_%s", time.Now().UnixNano(), stage))
    
    progressStage := &core.ProgressStage{
        Name:        stage,
        Description: stage,
        Status:      "running",
        Progress:    0,
        Message:     fmt.Sprintf("Starting %s", stage),
        Weight:      1.0, // Default weight
    }
    
    p.stages[token] = &progressState{
        stage:     progressStage,
        startTime: time.Now(),
        progress:  0,
    }
    
    // Direct GoMCP integration - no adapter needed
    if p.serverCtx != nil {
        p.serverCtx.NotifyProgress(string(token), &server.Progress{
            Token: string(token),
            Title: stage,
            Message: progressStage.Message,
            Progress: 0,
        })
    }
    
    p.logger.Info().
        Str("token", string(token)).
        Str("stage", stage).
        Msg("Progress stage started")
    
    return token
}

func (p *UnifiedProgressReporter) UpdateProgress(token core.ProgressToken, message string, percent int) {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    state, exists := p.stages[token]
    if !exists {
        p.logger.Warn().Str("token", string(token)).Msg("Progress token not found")
        return
    }
    
    state.stage.Message = message
    state.stage.Progress = percent
    state.lastUpdate = time.Now()
    state.progress = percent
    
    // GoMCP integration
    if p.serverCtx != nil {
        p.serverCtx.NotifyProgress(string(token), &server.Progress{
            Token: string(token),
            Title: state.stage.Name,
            Message: message,
            Progress: percent,
        })
    }
    
    p.logger.Debug().
        Str("token", string(token)).
        Str("message", message).
        Int("percent", percent).
        Msg("Progress updated")
}

func (p *UnifiedProgressReporter) CompleteStage(token core.ProgressToken, success bool, message string) {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    state, exists := p.stages[token]
    if !exists {
        return
    }
    
    if success {
        state.stage.Status = "completed"
        state.stage.Progress = 100
    } else {
        state.stage.Status = "failed"
    }
    state.stage.Message = message
    
    // GoMCP integration
    if p.serverCtx != nil {
        p.serverCtx.NotifyProgress(string(token), &server.Progress{
            Token: string(token),
            Title: state.stage.Name,
            Message: message,
            Progress: state.stage.Progress,
            Completed: true,
            Error: !success,
        })
    }
    
    duration := time.Since(state.startTime)
    p.logger.Info().
        Str("token", string(token)).
        Bool("success", success).
        Dur("duration", duration).
        Str("message", message).
        Msg("Progress stage completed")
    
    // Clean up completed stages after a delay
    go func() {
        time.Sleep(5 * time.Minute)
        p.mutex.Lock()
        delete(p.stages, token)
        p.mutex.Unlock()
    }()
}
```

**Expected Day 1 Results:**
- ✅ Unified progress implementation designed
- ✅ Core interface fully implemented  
- ✅ GoMCP integration without adapters
- ✅ Foundation ready for atomic tool updates

---

### **Day 2: Progress Adapter Replacement**

#### **Morning (4 hours): Update Atomic Tools**

**Step 2.1: Identify All Tools Using Progress Adapters**
```bash
# Find all atomic tools using progress adapters
grep -r "GoMCPProgressAdapter\|ProgressAdapter" pkg/mcp/internal/*/atomic*.go

# Expected tools (~12 files):
# - build atomic tools
# - analyze atomic tools  
# - deploy atomic tools
# - scan atomic tools
```

**Step 2.2: Update Tool Constructors**
```go
// Example: Update atomic build tool
// File: pkg/mcp/internal/build/build_image_atomic.go

package build

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/core"
    "github.com/Azure/container-kit/pkg/mcp/internal/observability"
)

type AtomicBuildImageTool struct {
    progress       core.ProgressReporter
    sessionManager core.SessionManager
    dockerClient   interface{} // Docker client
    logger         zerolog.Logger
}

func NewAtomicBuildImageTool(
    progress core.ProgressReporter,
    sessionManager core.SessionManager,
    dockerClient interface{},
) *AtomicBuildImageTool {
    return &AtomicBuildImageTool{
        progress:       progress,
        sessionManager: sessionManager,
        dockerClient:   dockerClient,
        logger:         zerolog.New(os.Stderr).With().Str("tool", "build_image").Logger(),
    }
}
```

**Step 2.3: Update Tool Execution to Use Unified Progress**
```go
func (t *AtomicBuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    buildArgs, ok := args.(*BuildImageArgs)
    if !ok {
        return nil, fmt.Errorf("invalid arguments for build_image")
    }
    
    // Use unified progress reporting - no adapter needed
    token := t.progress.StartStage("build_image")
    
    t.progress.UpdateProgress(token, "Analyzing Dockerfile", 10)
    // ... dockerfile analysis
    
    t.progress.UpdateProgress(token, "Starting image build", 30)
    // ... image build
    
    t.progress.UpdateProgress(token, "Finalizing build", 90)
    // ... finalization
    
    t.progress.CompleteStage(token, true, "Image built successfully")
    
    return &BuildImageResult{
        BaseToolResponse: core.BaseToolResponse{
            Success:   true,
            Message:   "Image built successfully",
            Timestamp: time.Now(),
        },
        ImageID:  "built_image_id",
        ImageRef: buildArgs.ImageRef,
    }, nil
}
```

#### **Afternoon (4 hours): Batch Update and Testing**

**Step 2.4: Batch Update All Atomic Tools**
```bash
# Update imports in all atomic tools
find pkg/mcp/internal -name "*atomic*.go" -exec sed -i \
    's|progress_adapter|observability|g; s|GoMCPProgressAdapter|UnifiedProgressReporter|g' {} \;

# Update constructor calls throughout codebase
find pkg/mcp/internal -name "*.go" -exec sed -i \
    's|NewGoMCPProgressAdapter|observability.NewUnifiedProgressReporter|g' {} \;
```

**Step 2.5: Remove All Progress Adapter Files**
```bash
# Verify no more references exist
grep -r "progress_adapter" pkg/mcp/internal/ | grep -v observability

# Remove the 3 adapter files
rm pkg/mcp/types/progress_adapter.go
rm pkg/mcp/internal/gomcp_progress_adapter.go
rm pkg/mcp/internal/runtime/gomcp_progress_adapter.go

# Test builds
go build -tags mcp ./pkg/mcp/internal/build/...
go build -tags mcp ./pkg/mcp/internal/analyze/...
```

**Expected Day 2 Results:**
- ✅ All atomic tools use unified progress (-370 lines from adapters)
- ✅ 3 progress adapter files eliminated
- ✅ Direct core interface usage throughout
- ✅ Consistent progress reporting across all tools

---

### **Day 3: Operation Wrapper Analysis and Design**

#### **Morning (4 hours): Operation Wrapper Analysis**

**Step 3.1: Analyze Operation Wrapper Duplication**
```bash
# Find all operation wrapper files
find pkg/mcp/internal/build -name "*operation_wrapper*.go"

# Expected results:
# pkg/mcp/internal/build/pull_operation_wrapper.go  
# pkg/mcp/internal/build/push_operation_wrapper.go
# pkg/mcp/internal/build/tag_operation_wrapper.go

# Analyze usage patterns
grep -r "OperationWrapper" pkg/mcp/internal/build/
```

**Step 3.2: Compare Wrapper Implementations**
```go
// Current Pattern 1: pull_operation_wrapper.go
type PullOperationWrapper struct {
    operation func(context.Context) error
    analyzer  func() error
    preparer  func() error
    lastError error
}

// Current Pattern 2: push_operation_wrapper.go  
type PushOperationWrapper struct {
    operation func(context.Context) error
    analyzer  func() error
    preparer  func() error  
    validator func() error
    lastError error
}

// Current Pattern 3: tag_operation_wrapper.go
type TagOperationWrapper struct {
    operation func(context.Context) error
    preparer  func() error
    validator func() error
    lastError error
}

// PROBLEM: 95% duplicate code with slight variations!
```

**Step 3.3: Design Generic Operation Wrapper**
```go
// NEW: Single configurable operation wrapper
// File: pkg/mcp/internal/build/docker_operation.go

package build

import (
    "context"
    "fmt"
    "time"
    "github.com/Azure/container-kit/pkg/mcp/core"
)

type OperationType string

const (
    OperationPull OperationType = "pull"
    OperationPush OperationType = "push"
    OperationTag  OperationType = "tag"
)

type DockerOperation struct {
    // Operation identification
    Type         OperationType
    Name         string
    
    // Configuration
    RetryAttempts int
    Timeout      time.Duration
    
    // Dependencies
    Progress     core.ProgressReporter
    Logger       zerolog.Logger
    
    // Operation-specific functions (configurable)
    Execute  func(ctx context.Context) error
    Analyze  func() error
    Prepare  func() error
    Validate func() error
    
    // State
    lastError error
    attempt   int
}

type DockerOperationConfig struct {
    Type          OperationType
    Name          string
    RetryAttempts int
    Timeout       time.Duration
    
    ExecuteFunc  func(ctx context.Context) error
    AnalyzeFunc  func() error
    PrepareFunc  func() error
    ValidateFunc func() error
}

func NewDockerOperation(config DockerOperationConfig, progress core.ProgressReporter) *DockerOperation {
    return &DockerOperation{
        Type:         config.Type,
        Name:         config.Name,
        RetryAttempts: config.RetryAttempts,
        Timeout:      config.Timeout,
        Progress:     progress,
        Logger:       zerolog.New(os.Stderr).With().Str("operation", string(config.Type)).Logger(),
        Execute:      config.ExecuteFunc,
        Analyze:      config.AnalyzeFunc,
        Prepare:      config.PrepareFunc,
        Validate:     config.ValidateFunc,
    }
}
```

#### **Afternoon (4 hours): Generic Wrapper Implementation**

**Step 3.4: Implement Generic Operation Logic**
```go
func (op *DockerOperation) Run(ctx context.Context) error {
    token := op.Progress.StartStage(fmt.Sprintf("%s_%s", op.Type, op.Name))
    
    // Pre-operation steps
    if err := op.runPreOperation(ctx, token); err != nil {
        op.Progress.CompleteStage(token, false, err.Error())
        return err
    }
    
    // Main operation with retry logic
    var lastError error
    for op.attempt = 1; op.attempt <= op.RetryAttempts; op.attempt++ {
        op.Progress.UpdateProgress(token, 
            fmt.Sprintf("Attempt %d/%d: %s", op.attempt, op.RetryAttempts, op.Name), 
            30 + (60 * op.attempt / op.RetryAttempts))
            
        if err := op.executeWithTimeout(ctx); err == nil {
            op.Progress.CompleteStage(token, true, "Operation completed successfully")
            return nil
        } else {
            lastError = err
            op.lastError = err
            
            if op.attempt < op.RetryAttempts {
                op.Logger.Warn().Err(err).Int("attempt", op.attempt).Msg("Operation failed, retrying")
                time.Sleep(time.Duration(op.attempt) * time.Second) // Exponential backoff
            }
        }
    }
    
    op.Progress.CompleteStage(token, false, fmt.Sprintf("Operation failed after %d attempts: %v", op.RetryAttempts, lastError))
    return fmt.Errorf("operation failed after %d attempts: %w", op.RetryAttempts, lastError)
}

func (op *DockerOperation) runPreOperation(ctx context.Context, token core.ProgressToken) error {
    if op.Analyze != nil {
        op.Progress.UpdateProgress(token, "Analyzing operation", 10)
        if err := op.Analyze(); err != nil {
            return fmt.Errorf("analysis failed: %w", err)
        }
    }
    
    if op.Prepare != nil {
        op.Progress.UpdateProgress(token, "Preparing operation", 20)
        if err := op.Prepare(); err != nil {
            return fmt.Errorf("preparation failed: %w", err)
        }
    }
    
    if op.Validate != nil {
        op.Progress.UpdateProgress(token, "Validating operation", 25)
        if err := op.Validate(); err != nil {
            return fmt.Errorf("validation failed: %w", err)
        }
    }
    
    return nil
}

func (op *DockerOperation) executeWithTimeout(ctx context.Context) error {
    if op.Execute == nil {
        return fmt.Errorf("no execute function configured")
    }
    
    // Create timeout context
    timeoutCtx, cancel := context.WithTimeout(ctx, op.Timeout)
    defer cancel()
    
    // Execute operation
    return op.Execute(timeoutCtx)
}
```

**Expected Day 3 Results:**
- ✅ Generic docker operation wrapper designed
- ✅ Configurable retry and timeout logic
- ✅ Unified progress reporting integration
- ✅ Foundation ready for atomic tool updates

---

### **Day 4: Operation Wrapper Replacement + Integration Testing**

#### **Morning (4 hours): Update Atomic Tools to Use Generic Wrapper**

**Step 4.1: Update Pull Image Atomic Tool**
```go
// File: pkg/mcp/internal/build/pull_image_atomic.go
func (t *AtomicPullImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    pullArgs := args.(*PullImageArgs)
    
    // Configure operation using generic wrapper
    operation := NewDockerOperation(DockerOperationConfig{
        Type:          OperationPull,
        Name:          pullArgs.ImageRef,
        RetryAttempts: 3,
        Timeout:       5 * time.Minute,
        
        ExecuteFunc: func(ctx context.Context) error {
            return t.dockerClient.ImagePull(ctx, pullArgs.ImageRef, types.ImagePullOptions{})
        },
        
        ValidateFunc: func() error {
            return validateImageRef(pullArgs.ImageRef)
        },
        
        PrepareFunc: func() error {
            return t.ensureDockerConnection()
        },
    }, t.progress)
    
    // Execute using generic wrapper
    if err := operation.Run(ctx); err != nil {
        return &PullImageResult{
            BaseToolResponse: core.BaseToolResponse{
                Success: false,
                Error:   err.Error(),
            },
        }, err
    }
    
    return &PullImageResult{
        BaseToolResponse: core.BaseToolResponse{
            Success: true,
            Message: "Image pulled successfully",
        },
        ImageRef: pullArgs.ImageRef,
    }, nil
}
```

**Step 4.2: Update Push and Tag Tools Similarly**
```go
// Similar updates for push_image_atomic.go and tag_image_atomic.go
// Each uses NewDockerOperation with appropriate configuration
```

**Step 4.3: Remove Operation Wrapper Files**
```bash
# Verify no references remain
grep -r "PullOperationWrapper\|PushOperationWrapper\|TagOperationWrapper" pkg/mcp/internal/

# Remove the 3 wrapper files
rm pkg/mcp/internal/build/pull_operation_wrapper.go
rm pkg/mcp/internal/build/push_operation_wrapper.go
rm pkg/mcp/internal/build/tag_operation_wrapper.go
```

#### **Afternoon (4 hours): Integration Testing and Validation**

**Step 4.4: Comprehensive Build Testing**
```bash
# Test all affected packages
go build -tags mcp ./pkg/mcp/internal/build/...
go build -tags mcp ./pkg/mcp/internal/observability/...
go build -tags mcp ./pkg/mcp/internal/analyze/...
go build -tags mcp ./pkg/mcp/internal/deploy/...
go build -tags mcp ./pkg/mcp/internal/scan/...

# Test entire MCP package
go build -tags mcp ./pkg/mcp/...
```

**Step 4.5: Functional Testing**
```bash
# Test progress reporting works
go test -tags mcp ./pkg/mcp/internal/observability/...

# Test docker operations work
go test -tags mcp ./pkg/mcp/internal/build/...

# Test atomic tools execute properly
go test -tags mcp -run TestAtomic ./pkg/mcp/internal/*/...
```

**Step 4.6: Consolidation Validation**
```bash
# Verify consolidation achieved
echo "Progress Adapters eliminated:"
find pkg/mcp -name "*progress_adapter*.go" | wc -l  # Expected: 0

echo "Operation Wrappers consolidated:"
find pkg/mcp/internal/build -name "*operation_wrapper*.go" | wc -l  # Expected: 0
find pkg/mcp/internal/build -name "docker_operation.go" | wc -l     # Expected: 1

echo "New unified implementations:"
ls -la pkg/mcp/internal/observability/progress.go                  # Expected: ~150 lines
ls -la pkg/mcp/internal/build/docker_operation.go                  # Expected: ~200 lines

# Calculate line reduction
echo "Lines eliminated: 657 (from 6 files)"
echo "Lines added: ~350 (2 unified implementations)" 
echo "Net reduction: ~307 lines"
echo "File reduction: 6 → 2 files (-4 files)"
```

**Expected Day 4 Results:**
- ✅ All operation wrappers consolidated (-287 lines)
- ✅ Generic docker operation wrapper implemented (+~150 lines)
- ✅ All atomic tools use unified implementations
- ✅ Net reduction of ~457 lines achieved

## Coordination with Workstream B

### **Shared Dependencies**
- **Core interfaces**: Both use `core.ProgressReporter`, `core.Tool`
- **Build package**: B modifies build analyzer, C modifies docker operations
- **No conflicts**: Different files within build package

### **Communication Points**
- **Daily standup**: Share core interface additions
- **Day 2 sync**: Ensure progress interface compatibility
- **Day 4 sync**: Coordinate final integration testing

### **Handoff Points**
- **Day 1**: C provides progress interface, B can use it
- **Day 4**: Both workstreams test integration together

## Risk Mitigation

### **Functionality Preservation**
- Maintain exact same progress reporting behavior
- Preserve all docker operation retry logic
- Keep same error handling patterns

### **Performance Considerations**
- Generic wrapper should have minimal overhead
- Progress reporting should be efficient
- No blocking operations in progress updates

### **Rollback Strategy**
- Keep original files until final validation
- Tag progress: `workstream-c-day-1`, etc.
- Test fallback for each consolidated implementation

## Success Criteria

### **Functional Requirements**
- [ ] All atomic tools report progress correctly
- [ ] Docker operations (pull/push/tag) work with retry logic
- [ ] GoMCP integration maintains same behavior
- [ ] No performance regression in tool execution

### **Consolidation Requirements**
- [ ] 3 progress adapters → 1 unified implementation
- [ ] 3 operation wrappers → 1 generic wrapper
- [ ] Net ~457 line reduction achieved
- [ ] Same functionality with simplified architecture

### **Quality Requirements**
- [ ] All tests pass: `make test-mcp`
- [ ] Build succeeds: `go build -tags mcp ./pkg/mcp/...`
- [ ] Progress reporting validated in atomic tools
- [ ] Docker operations validated with retry scenarios

## Expected Final State

```bash
# Consolidation verification
echo "Progress Adapters: 3 → 1 (-370 lines)"
echo "Operation Wrappers: 3 → 1 (-287 lines)"
echo "Total elimination: 657 lines from 6 files"
echo "Total addition: ~350 lines in 2 files"
echo "Net reduction: ~457 lines (-4 files)"

# Architecture verification  
echo "Unified progress: pkg/mcp/internal/observability/progress.go"
echo "Generic operations: pkg/mcp/internal/build/docker_operation.go"
echo "Core interface usage: 100% for progress and operations"
echo "GoMCP integration: Direct, no adapters"
```

## Benefits Achieved

### **Code Quality**
- **Eliminated duplication**: 95% duplicate code removed
- **Consistent behavior**: Same progress/retry patterns everywhere
- **Maintainable**: Changes in one place instead of 6
- **Type safety**: Core interfaces provide compile-time safety

### **Architecture**
- **Unified interfaces**: All tools use core.ProgressReporter
- **Configurable operations**: Generic wrapper handles all Docker ops
- **Direct integration**: No adapter layers for progress/operations
- **Dependency injection**: Clean separation of concerns

### **Development Experience**
- **Simpler tool creation**: Use unified progress/operations
- **Consistent patterns**: Same approach across all atomic tools
- **Better testing**: Mock core interfaces instead of adapters
- **Clear abstractions**: Well-defined operation configuration

**Workstream C will deliver a 31% reduction in progress/operation complexity while establishing unified patterns for all atomic tools!**