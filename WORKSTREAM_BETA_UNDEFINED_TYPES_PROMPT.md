# WORKSTREAM BETA: Undefined Types & Interfaces Implementation Guide

## 🎯 Mission
Define and implement all missing interfaces and types that are causing "undefined" compilation errors across the codebase, focusing on session management, client interfaces, and core types.

## 📋 Context
- **Project**: Container Kit - Three-layer architecture pre-commit fixes
- **Your Role**: Interface architect - defining contracts that other layers will implement
- **Timeline**: Week 1-2, Days 2-7 (6 days)
- **Dependencies**: ALPHA's shared types (available Day 2)
- **Deliverables**: SessionManager interfaces, client interfaces (analysis.Engine, kubernetes.Client), type constants

## 🎯 Success Metrics
- Undefined type errors: 50+ → 0
- Session interfaces: Fully defined with all methods
- Client interfaces: analysis.Engine, kubernetes.Client defined
- Type constants: JobStatus values implemented
- Interface compliance: All implementations satisfy contracts

## 📁 File Ownership
You have exclusive ownership of these files/directories:
```
pkg/mcp/application/services/session.go (create)
pkg/mcp/application/services/kubernetes.go (create)
pkg/mcp/application/services/types.go (create)
pkg/mcp/domain/session/types.go (create)
pkg/mcp/domain/containerization/analyze/interfaces.go
pkg/mcp/domain/containerization/scan/types.go
```

Shared files requiring coordination:
```
pkg/mcp/application/commands/* (they depend on your interfaces)
pkg/mcp/application/workflows/* (they use session types)
pkg/mcp/application/orchestration/pipeline/* (they use MCPClients)
```

## 🗓️ Implementation Schedule

### Day 2-3: Core Interface Definitions

#### Day 2 Morning: Session Manager Interfaces
**Task: Define SessionManager and UnifiedSessionManager**

First, analyze current usage to understand requirements:
```bash
# Find all SessionManager usage
grep -r "session.SessionManager" pkg/mcp --include="*.go" | grep -v "_test"
grep -r "session.UnifiedSessionManager" pkg/mcp --include="*.go"

# Understand method calls
grep -r "sessionManager\." pkg/mcp --include="*.go" | cut -d'.' -f2 | cut -d'(' -f1 | sort | uniq
```

**Create session interfaces**:
```go
// pkg/mcp/application/services/session.go
package services

import (
    "context"
    "time"
)

// SessionManager handles session lifecycle and persistence
type SessionManager interface {
    // Create a new session
    CreateSession(ctx context.Context, config SessionConfig) (*Session, error)
    
    // Get existing session
    GetSession(ctx context.Context, sessionID string) (*Session, error)
    
    // Update session state
    UpdateSession(ctx context.Context, sessionID string, update SessionUpdate) error
    
    // Delete session
    DeleteSession(ctx context.Context, sessionID string) error
    
    // List sessions with filters
    ListSessions(ctx context.Context, filter SessionFilter) ([]*Session, error)
    
    // Add more methods based on grep results
}

// UnifiedSessionManager extends SessionManager with additional capabilities
type UnifiedSessionManager interface {
    SessionManager
    
    // Additional unified methods based on usage analysis
}
```

#### Day 2 Afternoon: Session Types
**Task: Define session-related types**

```go
// pkg/mcp/domain/session/types.go
package session

import "time"

// Session represents a work session
type Session struct {
    ID          string
    WorkspaceID string
    Status      JobStatus
    CreatedAt   time.Time
    UpdatedAt   time.Time
    // Add fields based on usage
}

// JobStatus represents the status of a job/session
type JobStatus string

const (
    JobStatusPending   JobStatus = "pending"
    JobStatusRunning   JobStatus = "running"  
    JobStatusCompleted JobStatus = "completed"
    JobStatusFailed    JobStatus = "failed"
)

// SessionConfig for creating new sessions
type SessionConfig struct {
    // Add based on CreateSession usage
}
```

**Validation Commands**:
```bash
# Test compilation
go build ./pkg/mcp/application/services/...
go build ./pkg/mcp/domain/session/...

# Verify job status usage
grep -r "JobStatus" pkg/mcp/application/workflows/
```

#### Day 3 Morning: Analysis Engine Interface
**Task: Define analysis.Engine interface**

```bash
# Understand Engine usage
grep -r "analysis.Engine" pkg/mcp --include="*.go"
grep -r "Engine\." pkg/mcp/application/commands/ | grep -v "//" 
```

```go
// pkg/mcp/domain/containerization/analyze/interfaces.go
package analyze

import "context"

// Engine performs repository analysis
type Engine interface {
    // Analyze repository at given path
    Analyze(ctx context.Context, path string, options AnalysisOptions) (*AnalysisResult, error)
    
    // ValidateRepository checks if path is valid repo
    ValidateRepository(ctx context.Context, path string) error
    
    // Add methods based on usage in analyze_consolidated.go
}

// AnalysisOptions for configuring analysis
type AnalysisOptions struct {
    // Define based on usage
}
```

#### Day 3 Afternoon: Kubernetes Client Interface
**Task: Define kubernetes.Client interface**

```bash
# Understand kubernetes client usage
grep -r "kubernetes.Client" pkg/mcp --include="*.go"
grep -B5 -A5 "kubernetes.Client" pkg/mcp/application/commands/deploy_consolidated.go
```

```go
// pkg/mcp/application/services/kubernetes.go  
package services

import (
    "context"
)

// KubernetesClient interface for Kubernetes operations
type KubernetesClient interface {
    // Deploy manifest to cluster
    Deploy(ctx context.Context, manifest []byte, namespace string) error
    
    // Get deployment status
    GetDeploymentStatus(ctx context.Context, name, namespace string) (*DeploymentStatus, error)
    
    // Add based on deploy_consolidated.go usage
}

type DeploymentStatus struct {
    Ready   bool
    Replicas int
    // Add fields based on usage
}
```

### Day 4-5: Type Implementations

#### Day 4: Docker and Scan Types
**Task: Define docker.Vulnerability and related types**

```bash
# Find Vulnerability usage
grep -r "docker.Vulnerability" pkg/mcp --include="*.go"
sed -n '700,710p' pkg/mcp/application/commands/scan_implementation.go
```

```go
// pkg/mcp/domain/containerization/scan/types.go
package scan

// Vulnerability represents a security vulnerability
type Vulnerability struct {
    ID          string
    Severity    string
    Package     string
    Version     string
    FixedVersion string
    Description string
    // Add fields based on scan_implementation.go line 705
}
```

#### Day 5: Core Types and MCPClients
**Task: Define remaining core types**

```bash
# Find MCPClients usage
grep -r "MCPClients" pkg/mcp --include="*.go"
grep -r "NewMCPClients" pkg/mcp/application/core/
```

```go
// pkg/mcp/application/services/types.go
package services

// MCPClients holds all client instances
type MCPClients struct {
    Docker     DockerClient
    Kubernetes KubernetesClient
    Registry   RegistryClient
    // Add based on NewMCPClients usage
}

// ConsolidatedConversationConfig for conversation mode
type ConsolidatedConversationConfig struct {
    // Define based on EnableConversationMode usage
}
```

### Day 6-7: Integration and Polish

#### Day 6: Fix Pipeline Types
**Task: Resolve pipeline package undefined types**

```bash
# Analyze pipeline type errors
grep -r "undefined:" pkg/mcp/application/orchestration/pipeline/
```

Focus on:
- sessionsvc.SessionManager → services.SessionManager
- mcptypes.MCPClients → services.MCPClients  
- common types → create in shared location

#### Day 7: Final Integration
**Task: Ensure all undefined errors resolved**

```bash
# Final validation
/usr/bin/make pre-commit 2>&1 | grep "undefined" | wc -l
# Should be approaching 0

# Test each package compiles
go build ./pkg/mcp/application/services/...
go build ./pkg/mcp/application/commands/...
go build ./pkg/mcp/application/workflows/...
```

## 🔧 Technical Guidelines

### Interface Design Principles
- Keep interfaces small and focused (ISP)
- Use interface composition for complex contracts
- Define interfaces where they're used (consumer side)
- Return concrete types, accept interfaces

### Naming Conventions
- Interfaces: End with "-er" when possible (Runner, Manager)
- Types: Use clear, domain-specific names
- Constants: ALL_CAPS for exported, camelCase for internal

### Documentation Requirements
- Every exported interface needs package-level doc
- Method comments should explain contract, not implementation
- Include usage examples in package doc

## 🤝 Coordination Points

### Dependencies FROM Other Workstreams
| Workstream | What You Need | When | Contact |
|------------|---------------|------|---------|
| ALPHA | BaseToolArgs/Response types | Day 2 PM | Check domain/shared |

### Dependencies TO Other Workstreams  
| Workstream | What They Need | When | Format |
|------------|----------------|------|--------|
| GAMMA | Session interfaces | Day 3 | Complete interfaces |
| GAMMA | Client interfaces | Day 4 | For registry impl |
| DELTA | All service interfaces | Day 5 | For adapter impl |
| EPSILON | All types defined | Day 7 | For test fixes |

## 📊 Progress Tracking

### Key Metrics to Track
```bash
# Count undefined errors by type
/usr/bin/make pre-commit 2>&1 | grep "undefined" | cut -d: -f3 | sort | uniq -c | sort -rn

# Track progress by package
for pkg in services commands workflows orchestration; do
    echo "=== $pkg ==="
    go build ./pkg/mcp/application/$pkg/... 2>&1 | grep -c "undefined" || echo "0 errors"
done
```

### Daily Status Template
```markdown
## WORKSTREAM BETA - Day [X] Status

### Completed Today:
- Defined SessionManager interface with X methods
- Created JobStatus constants
- Fixed X undefined type errors

### Blockers:
- Need clarification on Engine.Analyze signature
- Waiting for ALPHA BaseToolResponse type

### Metrics:
- Undefined errors: X → Y (target: 0)
- Interfaces defined: X/Y
- Packages compiling: [list]

### Tomorrow's Focus:
- Complete kubernetes.Client interface
- Start docker types implementation
```

## 🚨 Common Issues & Solutions

### Issue 1: Circular import when defining interface
**Symptoms**: Import cycle detected
**Solution**: Move interface to services package, not domain
```bash
# Domain imports nothing
# Application imports domain
# Infra imports both
```

### Issue 2: Method signature unclear from usage
**Symptoms**: Can't determine parameters/returns
**Solution**: Check multiple usage points
```bash
# Find all calls to the method
grep -r "methodName(" pkg/mcp --include="*.go" -B2 -A2

# Check error handling to understand return type
grep -r "err := .*methodName" pkg/mcp --include="*.go"
```

### Issue 3: Interface too large
**Symptoms**: Interface has >10 methods
**Solution**: Split using interface composition
```go
type SessionReader interface {
    GetSession(...) 
    ListSessions(...)
}

type SessionWriter interface {
    CreateSession(...)
    UpdateSession(...)
}

type SessionManager interface {
    SessionReader
    SessionWriter
}
```

## 📞 Escalation Path

1. **Method signature ambiguity**: Check with GAMMA team who implements
2. **Domain modeling questions**: Consult architecture docs
3. **Breaking changes**: If interface change affects many files, coordinate in standup

## ✅ Definition of Done

Your workstream is complete when:
- [ ] Zero "undefined" errors in compilation
- [ ] All session interfaces defined and documented
- [ ] Client interfaces (analysis, k8s, docker) complete
- [ ] Type constants (JobStatus, etc.) implemented
- [ ] MCPClients and core types defined
- [ ] All service package compiles clean
- [ ] Interface documentation complete
- [ ] Handoff to implementation teams done

## 📚 Resources

- Interface design: Effective Go interfaces section
- Domain modeling: `docs/architecture/THREE_LAYER_ARCHITECTURE.md`
- Session patterns: Check git history for original SessionManager
- Client patterns: Look at existing client implementations in infra/

---

**Remember**: You're defining the contracts that hold the system together. Well-designed interfaces make implementation easy. Poorly designed interfaces cause endless pain. When in doubt, keep interfaces small and compose them. The implementation teams are counting on clear, well-documented interfaces from you.