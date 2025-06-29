# Workstream B: High-Impact Adapter Elimination - Detailed Implementation Plan

## Executive Summary

**Objective**: Eliminate the 4 highest-impact adapter files (646 lines total) that provide the most architectural value when removed.

**Duration**: 4 days (can start immediately after Workstream A Day 1)  
**Team Size**: 2 developers  
**Expected Reduction**: -646 lines, -4 adapter files

## Target Adapters (Priority Order)

| Priority | Adapter File | Lines | Complexity | Impact |
|----------|-------------|--------|------------|---------|
| **1** | `repository_analyzer_adapter.go` | 357 | HIGH | Breaks analyze↔build import cycle |
| **2** | `auto_registration_adapter.go` | 176 | HIGH | Migration debt, blocks interface unification |
| **3** | `transport_adapter.go` | 73 | MEDIUM | Architectural cleanup |
| **4** | `dockerfile_adapter.go` | 40 | LOW | Quick win, stub implementation |

## Day-by-Day Implementation Plan

### **Day 1: Repository Analyzer Adapter Elimination**

**Target**: `pkg/mcp/internal/orchestration/repository_analyzer_adapter.go` (357 lines)

#### **Morning (4 hours): Analysis and Preparation**

**Step 1.1: Current State Analysis**
```bash
# Analyze current adapter usage
grep -r "RepositoryAnalyzerAdapter" pkg/mcp/internal/ 
grep -r "repositoryAnalyzerAdapter" pkg/mcp/internal/

# Expected usage:
# - pkg/mcp/internal/orchestration/analyzer_helper.go
# - pkg/mcp/internal/build/analyzer_integration.go
```

**Step 1.2: Understand Import Cycle Problem**
```bash
# Current problematic pattern:
# pkg/mcp/internal/analyze ↔ pkg/mcp/internal/build
# 
# Adapter exists because:
# - analyze package can't import build (circular)
# - build package needs repository analysis
# - 200+ lines of conversion logic bridge the gap
```

**Step 1.3: Review Adapter Implementation**
```go
// Current adapter pattern (to be eliminated):
type RepositoryAnalyzerAdapter struct {
    toolFactory    ToolFactoryInterface
    sessionManager mcp.ToolSessionManager
    logger         zerolog.Logger
}

func (r *RepositoryAnalyzerAdapter) convertToRepositoryInfo(result *analyze.AtomicAnalysisResult) *build.RepositoryInfo {
    // 200+ lines of conversion logic - THIS IS THE PROBLEM
}
```

#### **Afternoon (4 hours): Implementation**

**Step 1.4: Use Core Interface for Repository Analysis**
```go
// Update build package to use core interface directly
// File: pkg/mcp/internal/build/analyzer_integration.go

package build

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/core"
)

type EnhancedBuildAnalyzer struct {
    repositoryAnalyzer core.RepositoryAnalyzer // Direct dependency injection
    logger            zerolog.Logger
}

func NewEnhancedBuildAnalyzer(analyzer core.RepositoryAnalyzer) *EnhancedBuildAnalyzer {
    return &EnhancedBuildAnalyzer{
        repositoryAnalyzer: analyzer,
        logger:            zerolog.New(os.Stderr),
    }
}

func (eba *EnhancedBuildAnalyzer) AnalyzeForBuild(ctx context.Context, repoPath string) (*BuildAnalysisResult, error) {
    // Direct usage - no conversion needed
    repoInfo, err := eba.repositoryAnalyzer.AnalyzeStructure(ctx, repoPath)
    if err != nil {
        return nil, err
    }
    
    // Use core.RepositoryInfo directly
    return &BuildAnalysisResult{
        RepositoryInfo: repoInfo, // Already core.RepositoryInfo type
        // ... other fields
    }, nil
}
```

**Step 1.5: Update Analyze Package to Implement Core Interface**
```go
// Update analyze package to implement core interface
// File: pkg/mcp/internal/analyze/repository_analyzer.go

package analyze

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/core"
)

type repositoryAnalyzer struct {
    logger zerolog.Logger
}

// NewRepositoryAnalyzer creates analyzer that implements core interface
func NewRepositoryAnalyzer() core.RepositoryAnalyzer {
    return &repositoryAnalyzer{
        logger: zerolog.New(os.Stderr),
    }
}

func (r *repositoryAnalyzer) AnalyzeStructure(ctx context.Context, path string) (*core.RepositoryInfo, error) {
    // Direct implementation using core types - no conversion
    result := &core.RepositoryInfo{
        Path:     path,
        Type:     "detected_type",
        Language: "detected_language",
        // ... populate directly with core types
    }
    return result, nil
}

func (r *repositoryAnalyzer) AnalyzeDockerfile(ctx context.Context, path string) (*core.DockerfileInfo, error) {
    // Implementation using core types
    // ...
}

func (r *repositoryAnalyzer) GetBuildRecommendations(ctx context.Context, repo *core.RepositoryInfo) (*core.BuildRecommendations, error) {
    // Implementation using core types
    // ...
}
```

**Step 1.6: Update Dependency Injection**
```go
// Update main initialization to inject analyzer
// This breaks the import cycle by using dependency injection

// In server initialization:
repoAnalyzer := analyze.NewRepositoryAnalyzer()
buildAnalyzer := build.NewEnhancedBuildAnalyzer(repoAnalyzer)

// Register with tool orchestrator
orchestrator.RegisterBuildAnalyzer(buildAnalyzer)
```

**Step 1.7: Remove Adapter File**
```bash
# Verify no references remain
grep -r "RepositoryAnalyzerAdapter" pkg/mcp/internal/
grep -r "repositoryAnalyzerAdapter" pkg/mcp/internal/

# Remove the adapter file
rm pkg/mcp/internal/orchestration/repository_analyzer_adapter.go

# Verify build still works
go build -tags mcp ./pkg/mcp/internal/build/...
go build -tags mcp ./pkg/mcp/internal/analyze/...
```

**Expected Day 1 Results:**
- ✅ Repository analyzer adapter eliminated (-357 lines)
- ✅ Import cycle between analyze↔build resolved
- ✅ Direct interface usage instead of conversion
- ✅ Cleaner dependency injection pattern

---

### **Day 2: Auto Registration Adapter Elimination**

**Target**: `pkg/mcp/internal/runtime/auto_registration_adapter.go` (176 lines)

#### **Morning (4 hours): Migration Strategy**

**Step 2.1: Audit Remaining Non-Unified Tools**
```bash
# Find tools still using old patterns
grep -r "runtime\.UnifiedTool" pkg/mcp/internal/ | wc -l
grep -r "interface{}" pkg/mcp/internal/*/atomic*.go | grep Execute

# Expected: 5-10 tools still using old registration patterns
```

**Step 2.2: Understand Auto Registration Problem**
```go
// Current problematic pattern (to be eliminated):
type AutoRegistrationAdapter struct {
    registry map[string]interface{} // Type erasure problem
}

type OrchestratorRegistryAdapter struct {
    orchestratorRegistry interface {
        RegisterTool(name string, tool interface{}) error // interface{} abuse
    }
}

// Problem: Complex type assertions and conversions
func (a *AutoRegistrationAdapter) adaptTool(tool interface{}) core.Tool {
    // 50+ lines of type assertion complexity - ELIMINATE THIS
}
```

#### **Afternoon (4 hours): Implementation**

**Step 2.3: Update Remaining Tools to Core Interface**
```go
// Example: Update a tool that hasn't been migrated yet
// Before (using auto registration adapter):
type SomeTool struct{}

func (t *SomeTool) Execute(args interface{}) (interface{}, error) // Old pattern

// After (using core interface directly):
import "github.com/Azure/container-kit/pkg/mcp/core"

type SomeTool struct{}

func (t *SomeTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Updated signature matches core.Tool interface
}

func (t *SomeTool) GetMetadata() core.ToolMetadata {
    return core.ToolMetadata{
        Name:        "some_tool",
        Description: "Tool description",
        Version:     "1.0.0",
        Category:    "utility",
    }
}

func (t *SomeTool) Validate(ctx context.Context, args interface{}) error {
    // Validation logic
    return nil
}
```

**Step 2.4: Update Tool Registration to Use Core Registry**
```go
// Update registry to use core interfaces directly
// File: pkg/mcp/internal/runtime/registry.go

package runtime

import "github.com/Azure/container-kit/pkg/mcp/core"

type Registry struct {
    tools map[string]core.Tool
    mutex sync.RWMutex
}

func (r *Registry) Register(tool core.Tool) {
    r.mutex.Lock()
    defer r.mutex.Unlock()
    
    metadata := tool.GetMetadata()
    r.tools[metadata.Name] = tool
}

func (r *Registry) Get(name string) (core.Tool, bool) {
    r.mutex.RLock()
    defer r.mutex.RUnlock()
    
    tool, exists := r.tools[name]
    return tool, exists
}

func (r *Registry) GetTool(name string) (core.Tool, error) {
    if tool, exists := r.Get(name); exists {
        return tool, nil
    }
    return nil, fmt.Errorf("tool %s not found", name)
}
```

**Step 2.5: Remove Auto Registration Adapter**
```bash
# Verify no references remain
grep -r "AutoRegistrationAdapter" pkg/mcp/internal/
grep -r "OrchestratorRegistryAdapter" pkg/mcp/internal/

# Remove the adapter file
rm pkg/mcp/internal/runtime/auto_registration_adapter.go

# Update imports to remove adapter-specific references
find pkg/mcp/internal -name "*.go" -exec sed -i '/auto_registration_adapter/d' {} \;
```

**Expected Day 2 Results:**
- ✅ Auto registration adapter eliminated (-176 lines)
- ✅ All tools use unified core.Tool interface
- ✅ Direct tool registration without type erasure
- ✅ Simplified registry implementation

---

### **Day 3: Transport Adapter Elimination**

**Target**: `pkg/mcp/internal/core/transport_adapter.go` (73 lines)

#### **Morning (2 hours): Transport Standardization**

**Step 3.1: Analyze Transport Implementations**
```bash
# Find all transport implementations
find pkg/mcp/internal/transport -name "*.go" -exec grep -l "Serve\|Stop\|SetHandler" {} \;

# Expected implementations:
# - stdio transport
# - HTTP transport
```

**Step 3.2: Update Transports to Use Core Interface**
```go
// Update stdio transport to implement core.Transport directly
// File: pkg/mcp/internal/transport/stdio.go

package transport

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/core"
)

type StdioTransport struct {
    handler core.RequestHandler
    // ... other fields
}

func NewStdioTransport() core.Transport {
    return &StdioTransport{}
}

func (s *StdioTransport) Serve(ctx context.Context) error {
    // Implementation
    return nil
}

func (s *StdioTransport) Stop(ctx context.Context) error {
    // Implementation  
    return nil
}

func (s *StdioTransport) SetHandler(handler core.RequestHandler) {
    s.handler = handler
}

func (s *StdioTransport) Name() string {
    return "stdio"
}
```

#### **Afternoon (2 hours): Server Integration**

**Step 3.3: Update Server to Use Direct Transport**
```go
// Update server to use transports directly without adapter
// File: pkg/mcp/internal/core/server.go

package core

import "github.com/Azure/container-kit/pkg/mcp/core"

type Server struct {
    transport core.Transport // Direct usage, no adapter needed
}

func (s *Server) SetTransport(t core.Transport) {
    s.transport = t
    s.transport.SetHandler(s) // Server implements RequestHandler
}

func (s *Server) HandleRequest(ctx context.Context, req *core.MCPRequest) (*core.MCPResponse, error) {
    // Direct implementation using core types
    return &core.MCPResponse{
        ID:     req.ID,
        Result: "handled",
    }, nil
}
```

**Step 3.4: Remove Transport Adapter**
```bash
# Remove the adapter file
rm pkg/mcp/internal/core/transport_adapter.go

# Update server initialization
# Remove adapter-specific code from server setup
```

**Expected Day 3 Results:**
- ✅ Transport adapter eliminated (-73 lines)
- ✅ Direct transport interface usage
- ✅ Simplified server architecture
- ✅ Cleaner transport abstraction

---

### **Day 4: Dockerfile Adapter Elimination + Integration Testing**

**Target**: `pkg/mcp/internal/analyze/dockerfile_adapter.go` (40 lines)

#### **Morning (2 hours): Quick Dockerfile Adapter Removal**

**Step 4.1: Analyze Dockerfile Adapter**
```bash
# This is a stub adapter - should be simple to remove
grep -r "DockerfileAdapter" pkg/mcp/internal/
# Expected: Only used in validate_dockerfile_atomic.go
```

**Step 4.2: Direct Integration**
```go
// Move validation logic directly to the atomic tool
// File: pkg/mcp/internal/analyze/validate_dockerfile_atomic.go

// Before (using adapter):
adapter := NewDockerfileAdapter()
result := adapter.ValidateDockerfile(path)

// After (direct implementation):
func (t *AtomicValidateDockerfileTool) validateDockerfile(path string) (*ValidationResult, error) {
    // Direct validation logic - no adapter needed
    // Move the actual validation code here
    return &ValidationResult{
        Valid:  true,
        Issues: []string{},
    }, nil
}
```

**Step 4.3: Remove Dockerfile Adapter**
```bash
rm pkg/mcp/internal/analyze/dockerfile_adapter.go
```

#### **Afternoon (6 hours): Integration Testing and Validation**

**Step 4.4: Comprehensive Build Testing**
```bash
# Test all modified packages build successfully
go build -tags mcp ./pkg/mcp/internal/analyze/...
go build -tags mcp ./pkg/mcp/internal/build/...
go build -tags mcp ./pkg/mcp/internal/orchestration/...
go build -tags mcp ./pkg/mcp/internal/runtime/...
go build -tags mcp ./pkg/mcp/internal/core/...
go build -tags mcp ./pkg/mcp/internal/transport/...

# Test entire MCP package
go build -tags mcp ./pkg/mcp/...
```

**Step 4.5: Functional Testing**
```bash
# Test tool execution works
go test -tags mcp ./pkg/mcp/internal/analyze/...
go test -tags mcp ./pkg/mcp/internal/build/...

# Test transport layer works
go test -tags mcp ./pkg/mcp/internal/transport/...
go test -tags mcp ./pkg/mcp/internal/core/...
```

**Step 4.6: Adapter Elimination Validation**
```bash
# Verify no adapter files remain
find pkg/mcp -name "*adapter*.go" | grep -E "(repository_analyzer|auto_registration|transport|dockerfile)" | wc -l
# Expected: 0

# Verify no adapter imports remain
grep -r "adapter" pkg/mcp/internal/ | grep -E "(repository_analyzer|auto_registration|transport|dockerfile)"
# Expected: No results

# Count total lines eliminated
echo "Repository Analyzer: -357 lines"
echo "Auto Registration: -176 lines"  
echo "Transport: -73 lines"
echo "Dockerfile: -40 lines"
echo "Total: -646 lines eliminated"
```

**Expected Day 4 Results:**
- ✅ Dockerfile adapter eliminated (-40 lines)
- ✅ All 4 target adapters removed (-646 lines total)
- ✅ Comprehensive testing passes
- ✅ No adapter-related imports remain

## Coordination with Workstream C

### **Shared Resources**
- **Day 1-2**: B works on repository/registration, C works on progress adapters (no conflicts)
- **Day 3-4**: B works on transport/dockerfile, C works on operation wrappers (no conflicts)

### **Communication Points**
- **Daily standup**: Share interface changes
- **End of Day 2**: Sync on any core interface additions needed
- **End of Day 4**: Coordinate integration testing

## Risk Mitigation

### **Import Cycle Prevention**
- Use dependency injection consistently
- Never let internal packages import each other directly
- All shared interfaces go through core package

### **Rollback Plan**
- Tag each day's completion: `workstream-b-day-1`, etc.
- Keep adapter files in version control until final validation
- Have fallback commits ready for each step

### **Testing Strategy**
- Test after each adapter elimination
- Maintain existing test coverage
- Add integration tests for new dependency injection patterns

## Success Criteria

### **Functional Requirements**
- [ ] All affected tools execute without errors
- [ ] Repository analysis works without adapter conversion
- [ ] Tool registration works without type erasure
- [ ] Transport layer functions correctly
- [ ] Dockerfile validation works without adapter

### **Architectural Requirements**
- [ ] Zero adapter files for target adapters
- [ ] No import cycles between analyze↔build packages
- [ ] Direct usage of core interfaces throughout
- [ ] Dependency injection replaces adapter patterns

### **Quality Requirements**
- [ ] All tests pass: `make test-mcp`
- [ ] Build succeeds: `go build -tags mcp ./pkg/mcp/...`
- [ ] No performance regression
- [ ] Code coverage maintained

## Expected Final State

```bash
# File elimination verification
find pkg/mcp -name "*adapter*.go" | grep -E "(repository_analyzer|auto_registration|transport|dockerfile)" | wc -l
# Result: 0

# Line count reduction verification  
echo "Total lines eliminated: 646"
echo "Percentage of total adapters: 44% (646/1474)"

# Architecture verification
echo "Import cycles resolved: analyze ↔ build"
echo "Direct interface usage: 100% for target adapters"
echo "Dependency injection patterns: Implemented"
```

**Workstream B will deliver a 44% reduction in adapter complexity while establishing clean dependency injection patterns for the remaining workstreams!**