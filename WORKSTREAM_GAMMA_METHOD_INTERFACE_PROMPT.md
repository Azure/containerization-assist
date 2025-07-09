# WORKSTREAM GAMMA: Method & Interface Alignment Implementation Guide

## 🎯 Mission
Resolve all redeclared types, align method signatures with interfaces, and ensure the core application package compiles successfully by implementing the interfaces defined by BETA team.

## 📋 Context
- **Project**: Container Kit - Three-layer architecture pre-commit fixes
- **Your Role**: Implementation specialist - making interfaces concrete and fixing conflicts
- **Timeline**: Week 1-2, Days 3-8 (6 days)
- **Dependencies**: ALPHA's types (Day 2), BETA's interfaces (Day 3+)
- **Deliverables**: No redeclared types, aligned method signatures, fully compiling core package

## 🎯 Success Metrics
- Redeclared types: 6 → 0
- Method mismatches: All resolved
- pkg/mcp/application/core: Fully compiles
- Registry using domain/shared types: 100%
- Server implementation: All methods properly typed

## 📁 File Ownership
You have exclusive ownership of these files/directories:
```
pkg/mcp/application/core/server.go
pkg/mcp/application/core/server_impl.go
pkg/mcp/application/core/server_core.go
pkg/mcp/application/core/registry.go
pkg/mcp/application/core/config.go
pkg/mcp/application/core/types.go
pkg/mcp/application/core/mcp.go
```

Shared files requiring coordination:
```
pkg/mcp/application/commands/* (they call your implementations)
pkg/mcp/application/api/interfaces.go (source of truth for interfaces)
```

## 🗓️ Implementation Schedule

### Day 3-4: Resolve Redeclarations

#### Day 3 Morning: Fix Type Redeclarations
**Task: Resolve Server, ServerConfig, DefaultServerConfig conflicts**

First, understand the conflicts:
```bash
# See exact redeclaration errors
/usr/bin/make pre-commit 2>&1 | grep "redeclared" -A2 -B2

# Find all Server type definitions
grep -n "type Server" pkg/mcp/application/core/*.go
grep -n "type ServerConfig" pkg/mcp/application/core/*.go  
grep -n "func DefaultServerConfig" pkg/mcp/application/core/*.go
```

**Resolution strategy**:
```go
// pkg/mcp/application/core/server.go - KEEP THIS (interface)
type Server interface {
    // Keep all interface methods
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
    // etc...
}

// pkg/mcp/application/core/server_impl.go - RENAME THIS
type serverImpl struct {  // Changed from Server to serverImpl
    // struct fields
}

// pkg/mcp/application/core/config.go - REMOVE duplicates
// Delete ServerConfig struct definition here
// Delete DefaultServerConfig function here

// pkg/mcp/application/core/types.go - USE ALIASES
type ServerConfig = config.ServerConfig  // Alias to domain config

// pkg/mcp/application/core/mcp.go - DELEGATE
func DefaultServerConfig() config.ServerConfig {
    return config.DefaultServerConfig()  // Delegate to domain
}
```

#### Day 3 Afternoon: Fix Import Dependencies
**Task: Update all type references after renaming**

```bash
# Update struct references
sed -i 's/\*Server/\*serverImpl/g' pkg/mcp/application/core/server_impl.go
sed -i 's/&Server{/&serverImpl{/g' pkg/mcp/application/core/server_impl.go

# Verify no more redeclarations
go build ./pkg/mcp/application/core/... 2>&1 | grep "redeclared"
```

#### Day 4 Morning: Registry Type Alignment
**Task: Import domain/shared types in registry**

```go
// pkg/mcp/application/core/registry.go
package core

import (
    // Add this import
    "github.com/Azure/container-kit/pkg/mcp/domain/shared"
    // ... other imports
)

// Update all type references
// Change: types.BaseToolResponse
// To:     shared.BaseToolResponse

// Change: types.BaseToolArgs  
// To:     shared.BaseToolArgs
```

**Validation**:
```bash
# Should show no undefined types
go build ./pkg/mcp/application/core/registry.go
```

#### Day 4 Afternoon: Type Assertion Fixes
**Task: Fix type assertions and embedded types**

Search and fix patterns like:
```go
// Before
response := types.BaseToolResponse{}

// After  
response := shared.BaseToolResponse{}

// Before - embedded field
type SomeResponse struct {
    types.BaseToolResponse
}

// After
type SomeResponse struct {
    shared.BaseToolResponse
}
```

### Day 5-6: Method Implementation

#### Day 5: Server Method Implementation
**Task: Implement missing interface methods**

First, identify missing methods:
```bash
# Check what Server interface requires
grep -A50 "type Server interface" pkg/mcp/application/core/server.go

# Check what methods serverImpl has
grep "^func (s \*serverImpl)" pkg/mcp/application/core/server_impl.go | cut -d' ' -f3
```

Implement missing methods:
```go
// pkg/mcp/application/core/server_impl.go

// EnableConversationMode implements Server interface
func (s *serverImpl) EnableConversationMode(config ConsolidatedConversationConfig) error {
    // Implementation based on existing patterns
    return nil
}

// GetSessionManager implements Server interface  
func (s *serverImpl) GetSessionManager() session.UnifiedSessionManager {
    return s.sessionManager
}

// Add other missing methods...
```

#### Day 6: Tool Constructor Fixes
**Task: Fix tool registration calls**

The undefined tool constructors need proper imports:
```go
// pkg/mcp/application/core/server_impl.go
import (
    // Add tool imports
    "github.com/Azure/container-kit/pkg/mcp/application/commands/analyze"
    "github.com/Azure/container-kit/pkg/mcp/application/commands/build"
    "github.com/Azure/container-kit/pkg/mcp/application/commands/deploy"
    "github.com/Azure/container-kit/pkg/mcp/application/commands/scan"
)

// Fix constructor calls
analyzeRepoTool := analyze.NewAtomicAnalyzeRepositoryTool(...)
buildImageTool := build.NewAtomicBuildImageTool(...)
// etc...
```

### Day 7-8: Final Integration

#### Day 7: Handler Function Signatures
**Task: Align handler signatures with tool requirements**

```go
// Ensure handler functions match expected signatures
srv.analyzeHandler = func(ctx *server.Context, args *analyze.AtomicAnalyzeRepositoryArgs) (*analyze.AtomicAnalysisResult, error) {
    // Make sure types match what the tool expects
    return analyzeRepoTool.Run(ctx, args)
}
```

#### Day 8: Final Compilation
**Task: Ensure entire core package compiles**

```bash
# Full build test
go build ./pkg/mcp/application/core/...

# Check no compilation errors
echo $?  # Should be 0

# Run any existing tests
go test ./pkg/mcp/application/core/...
```

## 🔧 Technical Guidelines

### Naming Conventions
- Interfaces: Exported (Server)
- Implementations: Unexported (serverImpl)
- Factory functions: NewXXX returns interface type

### Import Organization
```go
import (
    // Standard library
    "context"
    "fmt"
    
    // Internal packages (domain layer)
    "github.com/Azure/container-kit/pkg/mcp/domain/config"
    "github.com/Azure/container-kit/pkg/mcp/domain/shared"
    
    // Internal packages (same layer)
    "github.com/Azure/container-kit/pkg/mcp/application/services"
    
    // External packages
    "github.com/some/external"
)
```

### Method Receivers
- Always use pointer receivers for struct methods
- Keep receiver name consistent (s for server, r for registry)

## 🤝 Coordination Points

### Dependencies FROM Other Workstreams
| Workstream | What You Need | When | Contact |
|------------|---------------|------|---------|
| ALPHA | shared.BaseToolArgs/Response | Day 4 | Must be ready |
| BETA | Session interfaces | Day 5 | For GetSessionManager |
| BETA | Client interfaces | Day 6 | For tool setup |

### Dependencies TO Other Workstreams  
| Workstream | What They Need | When | Format |
|------------|----------------|------|--------|
| DELTA | Compiling core package | Day 7 | For transport layer |
| EPSILON | All methods implemented | Day 8 | For testing |

## 📊 Progress Tracking

### Daily Validation Commands
```bash
# Check redeclaration count
/usr/bin/make pre-commit 2>&1 | grep -c "redeclared"

# Check method implementation completeness  
go build -o /dev/null ./pkg/mcp/application/core 2>&1 | grep -c "missing method"

# Check undefined types in core
go build ./pkg/mcp/application/core/... 2>&1 | grep -c "undefined"

# Full validation
go build ./pkg/mcp/application/core/... && echo "✅ Core package compiles!"
```

### Daily Status Template
```markdown
## WORKSTREAM GAMMA - Day [X] Status

### Completed Today:
- Resolved Server/serverImpl redeclaration
- Fixed X method signatures
- Imported shared types in registry
- Core package compilation: X% complete

### Blockers:
- Waiting for BETA session interface final version
- Need clarification on EnableConversationMode parameters

### Metrics:
- Redeclared types: X → Y (target: 0)
- Undefined methods: X → Y (target: 0)  
- Core build status: [compiling/failing]

### Tomorrow's Focus:
- Complete tool constructor imports
- Implement remaining server methods
```

## 🚨 Common Issues & Solutions

### Issue 1: Interface method signature mismatch
**Symptoms**: "does not implement" error
**Solution**: Copy exact signature from interface
```bash
# Get exact interface method
grep -A2 "MethodName(" pkg/mcp/application/core/server.go

# Ensure implementation matches exactly (including return types)
```

### Issue 2: Import cycle after adding imports
**Symptoms**: "import cycle not allowed"
**Solution**: Check dependency direction
```
✅ core can import services (same layer)
✅ core can import domain (lower layer)  
❌ core cannot import commands (depends on core)
```

### Issue 3: Type assertion fails at runtime
**Symptoms**: "interface conversion: X is not Y"
**Solution**: Check actual type being passed
```go
// Add debug logging
fmt.Printf("Type: %T\n", actualValue)

// Ensure type matches interface
var _ ExpectedInterface = actualValue // Compile-time check
```

## 📞 Escalation Path

1. **Interface changes needed**: Coordinate with BETA team
2. **Import cycles**: May need DELTA team to create adapter
3. **Missing types**: Check with BETA if interface is defined

## ✅ Definition of Done

Your workstream is complete when:
- [ ] Zero redeclared type errors
- [ ] All server methods implemented
- [ ] Registry using shared types throughout
- [ ] No undefined type errors in core package
- [ ] go build ./pkg/mcp/application/core/... succeeds
- [ ] All tool constructors properly imported
- [ ] Handler functions correctly typed
- [ ] Basic smoke test passes

## 📚 Resources

- Interface implementation: Check git history for original implementations
- Method signatures: pkg/mcp/application/api/interfaces.go is source of truth
- Import management: Use goimports tool
- Architecture: docs/architecture/THREE_LAYER_ARCHITECTURE.md

---

**Remember**: You're the bridge between interfaces and implementation. Your work makes the abstract concrete. A clean core package is the heart of the application - when it compiles cleanly, the whole system has a better chance of working. Pay attention to types and signatures - the compiler is your friend here.