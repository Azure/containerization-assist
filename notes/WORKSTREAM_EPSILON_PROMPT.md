# WORKSTREAM EPSILON: Non-Idiomatic Go Cleanup
**AI Assistant Prompt - Container Kit MCP Cleanup**

## 🎯 MISSION OVERVIEW

You are the **Go Idioms Specialist** responsible for eliminating 300+ interface{} instances (85% reduction), replacing panic() usage with proper error handling, and enforcing idiomatic Go patterns throughout the codebase.

**Duration**: Week 2-4 (parallel, after BETA foundation)  
**Dependencies**: WORKSTREAM BETA completion (type safety foundation)  
**Critical Success**: Idiomatic, type-safe Go code with compile-time checking

## 📋 YOUR SPECIFIC RESPONSIBILITIES

### Week 2 (Days 6-10): Foundation & Analysis

#### Day 6-7: Interface{} Analysis & Planning  
```bash
# WAIT: Until WORKSTREAM BETA RichError foundation complete

# Comprehensive interface{} analysis:
rg "interface{}" pkg/mcp/ --type go > interface_usage_inventory.txt
echo "Total interface{} instances: $(wc -l < interface_usage_inventory.txt)"

# Categorize interface{} usage:
echo "# Interface{} Usage Analysis

## Critical Path Usage (HIGH PRIORITY):
$(rg "interface{}" pkg/mcp/internal/orchestration/ --type go | wc -l) instances in orchestration
$(rg "interface{}" pkg/mcp/internal/core/ --type go | wc -l) instances in core

## Configuration Usage (MEDIUM PRIORITY):  
$(rg "interface{}" pkg/mcp/internal/build/ --type go | wc -l) instances in build
$(rg "interface{}" pkg/mcp/internal/deploy/ --type go | wc -l) instances in deploy

## Utility Usage (LOW PRIORITY):
$(rg "interface{}" pkg/mcp/utils/ --type go | wc -l) instances in utils
" > docs/interface_usage_analysis.md

# Analyze type assertions:
rg "\.(" pkg/mcp/ --type go > type_assertions_inventory.txt  
echo "Type assertions found: $(wc -l < type_assertions_inventory.txt)"

# Plan strongly-typed replacements using BETA's generic types
```

#### Day 8-10: Begin Safe Replacements
```bash
# Start with NON-CRITICAL interface{} replacements:

# Target 1: Simple map[string]interface{} → typed structs
# Example: Configuration maps that can be proper structs
find pkg/mcp/utils -name "*.go" -exec grep -l "map\[string\]interface{}" {} \;

# Target 2: Type assertions without error checking → safe assertions  
# Example: obj.(Type) → obj.(Type), ok
find pkg/mcp/utils -name "*.go" -exec grep -l "\\.(" {} \;

# Target 3: Configuration interface{} → typed configuration
# Replace generic config maps with specific config structs

# CREATE: pkg/mcp/types/config/
mkdir -p pkg/mcp/types/config
touch pkg/mcp/types/config/build.go      # BuildConfig struct
touch pkg/mcp/types/config/deploy.go     # DeployConfig struct  
touch pkg/mcp/types/config/scan.go       # ScanConfig struct

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/utils/... && echo "✅ Safe replacements complete"
go test -short ./pkg/mcp/types/config/... && echo "✅ Typed configs working"

# COMMIT:
git add .
git commit -m "refactor(types): begin interface{} elimination with safe replacements

- Replaced 20+ simple map[string]interface{} with typed structs
- Added error handling to 15+ type assertions  
- Created typed configuration structs
- Improved type safety in utility packages

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"
```

### Week 3 (Days 11-15): Core System Type Safety

#### Day 11-12: Tool Registry Type Safety (COORDINATE WITH BETA)
```bash
# WAIT: Until WORKSTREAM BETA generic registry completion

# COORDINATE: Use BETA's strongly-typed registry system

# Target: pkg/mcp/internal/orchestration/ 
# Replace ALL interface{} with BETA's generic types

# File 1: pkg/mcp/internal/orchestration/tool_orchestrator.go
# - Replace interface{} parameters with TParams from BETA
# - Replace interface{} results with TResult from BETA  
# - Use BETA's Tool[TParams, TResult] interface
# - Remove ALL type assertions from tool execution

# File 2: pkg/mcp/internal/orchestration/tool_registry.go
# - Use BETA's GenericRegistry[T, TParams, TResult]
# - Replace map[string]interface{} with strongly-typed tool storage
# - Remove runtime type checking where possible

# File 3: pkg/mcp/internal/orchestration/tool_factory.go
# - Use BETA's strongly-typed tool creation
# - Replace interface{} tool parameters with specific types
# - Remove type casting in tool creation

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/internal/orchestration/... && echo "✅ Orchestration type-safe"

# Count progress:
echo "interface{} instances in orchestration: $(rg "interface{}" pkg/mcp/internal/orchestration/ | wc -l)"
echo "Type assertions in orchestration: $(rg "\.(" pkg/mcp/internal/orchestration/ | wc -l)"
```

#### Day 13-15: Transport Layer Type Safety
```bash
# HTTP/gRPC interface{} elimination:

# File 1: pkg/mcp/internal/transport/http.go  
# - Replace JSON interface{} with specific request/response types
# - Define typed HTTP request structures
# - Define typed HTTP response structures  
# - Remove runtime JSON type checking

# File 2: pkg/mcp/internal/transport/stdio.go
# - Replace interface{} in stdio communication
# - Use BETA's RichError for transport errors
# - Add compile-time type checking for messages

# File 3: pkg/mcp/internal/core/gomcp_tools.go  
# - Replace the massive interface{} usage (identified in analysis)
# - Use BETA's strongly-typed tool definitions
# - Remove type assertions from tool execution
# - Integrate with BETA's RichError system

# CREATE typed request/response structures:
mkdir -p pkg/mcp/types/transport
touch pkg/mcp/types/transport/requests.go
touch pkg/mcp/types/transport/responses.go

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/internal/transport/... && echo "✅ Transport layer type-safe"
go test -short ./pkg/mcp/internal/core/... && echo "✅ Core tools type-safe"
```

### Week 4 (Days 16-20): Final Cleanup & Constants

#### Day 16-17: Magic Numbers & Constants
```bash
# Define constants for all magic numbers:

# CREATE: pkg/mcp/constants/
mkdir -p pkg/mcp/constants
  
# File 1: pkg/mcp/constants/timeouts.go
# Define timeout constants:
const (
    DefaultTimeout        = 30 * time.Second  // Used 10+ times
    BuildTimeout         = 300 * time.Second // Docker build operations
    DeployTimeout        = 120 * time.Second // K8s deployment operations
    ValidationTimeout    = 10 * time.Second  // Validation operations
)

# File 2: pkg/mcp/constants/limits.go  
# Define limit constants:
const (
    MaxErrors            = 100               // Validation error limit
    MaxSessions          = 1000              // Session manager limit
    MaxDiskPerSession    = 1 << 30           // 1GB per session
    TotalDiskLimit       = 10 << 30          // 10GB total
    MaxBodyLogSize       = 1 << 20           // 1MB log limit
    MaxWorkers           = 10                // Worker pool limit
)

# File 3: pkg/mcp/constants/buffers.go
# Define buffer size constants:
const (
    SmallBufferSize      = 1024              // 1KB for small operations
    MediumBufferSize     = 4096              // 4KB for medium operations  
    LargeBufferSize      = 65536             // 64KB for large operations
)

# Replace magic numbers throughout codebase:
find pkg/mcp -name "*.go" -exec grep -l "30.*time\.Second" {} \; | head -5
# Replace with constants.DefaultTimeout

find pkg/mcp -name "*.go" -exec grep -l "100," {} \; | head -5  
# Replace with constants.MaxErrors where appropriate
```

#### Day 18-19: Panic Usage Elimination (BREAKING CHANGES)
```bash
# Replace panic() with error returns:

# CRITICAL FILE: pkg/mcp/client_factory.go
# Current panic usage in library code:

# BEFORE:
func (b *BaseInjectableClients) GetDockerClient() docker.DockerClient {
    if b.clientFactory == nil {
        panic("client factory not injected - call SetClientFactory first")
    }
    return b.clientFactory.CreateDockerClient()
}

# AFTER:
func (b *BaseInjectableClients) GetDockerClient() (docker.DockerClient, error) {
    if b.clientFactory == nil {
        return nil, rich.NewError().
            Code("CLIENT_FACTORY_NOT_INJECTED").
            Message("Client factory not injected").
            Type(rich.ErrTypeConfiguration).
            Severity(rich.SeverityHigh).
            Suggestion("Call SetClientFactory() before GetDockerClient()").
            Build()
    }
    return b.clientFactory.CreateDockerClient(), nil
}

# Update ALL call sites to handle errors:
find pkg/mcp -name "*.go" -exec grep -l "GetDockerClient()" {} \;
# Add error handling: client, err := deps.GetDockerClient()

# File 2: pkg/mcp/internal/config/global.go
# Replace MustGet() panic with proper error handling

# BREAKING CHANGE WARNING: Document all API changes
echo "# BREAKING CHANGES - Panic Elimination

## Modified Functions (now return errors):
- BaseInjectableClients.GetDockerClient() → (docker.DockerClient, error)
- BaseInjectableClients.GetKindClient() → (kind.KindClient, error)
- BaseInjectableClients.GetKubeClient() → (k8s.KubeClient, error)
- ConfigManager.MustGet() → Get() (ConfigManager, error)

## Migration Guide:
[Provide migration examples for each changed function]
" > docs/BREAKING_CHANGES.md
```

#### Day 20: Final Validation & Documentation
```bash
# Final idiomatic Go validation:

# Count remaining interface{} instances:
echo "Remaining interface{} instances: $(rg "interface{}" pkg/mcp/ | wc -l) (target: <50)"

# Count type assertions without error checking:
echo "Unsafe type assertions: $(rg "\.(" pkg/mcp/ | grep -v ", ok" | wc -l) (target: 0)"

# Validate magic numbers replaced:
echo "Magic number 30: $(rg "30.*time\.Second" pkg/mcp/ | wc -l) (should be 0)"
echo "Magic number 100: $(rg "100," pkg/mcp/ | wc -l) (should be minimal)"

# Document remaining non-idiomatic patterns:
echo "# Remaining Non-Idiomatic Patterns

## Acceptable interface{} Usage:
- JSON marshaling/unmarshaling where type flexibility needed
- Plugin systems requiring runtime type flexibility  
- [Other justified cases]

## Acceptable Type Assertions:
- [List remaining type assertions with justification]

## Performance Considerations:
- [Any performance trade-offs made for type safety]
" > docs/remaining_patterns.md

# Update coding standards:
echo "# Container Kit Go Coding Standards

## Type Safety Requirements:
- Use specific types instead of interface{} where possible
- All type assertions must include error checking
- Prefer compile-time type checking over runtime checks

## Error Handling:
- Library code must never use panic()
- Use RichError for enhanced error context
- Provide actionable error messages

## Constants:
- No magic numbers in code
- Define constants with clear documentation
- Group related constants in const blocks
" > docs/go_coding_standards.md

# FINAL VALIDATION:
go test ./... && echo "✅ EPSILON WORKSTREAM COMPLETE"

# FINAL COMMIT:
git add .
git commit -m "refactor(idioms): complete Go idioms cleanup

- Eliminated 250+ interface{} instances (83% reduction: 300+ → <50)
- Replaced panic() usage with proper error handling (BREAKING CHANGES)
- Added 50+ constants replacing magic numbers
- Achieved 95% compile-time type checking
- Updated coding standards and documentation

EPSILON WORKSTREAM COMPLETE ✅

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"
```

## 🎯 SUCCESS CRITERIA

### Must Achieve (100% Required):
- ✅ **300+ interface{} instances → <50 instances** (85% reduction)
- ✅ **Type assertions with proper error handling** (no unsafe assertions)
- ✅ **Magic numbers replaced with documented constants**
- ✅ **Panic usage eliminated from library code** (BREAKING CHANGES)
- ✅ **95% compile-time type checking** throughout system
- ✅ **All tests pass** with improved type safety

### Quality Gates (Enforce Strictly):
```bash
# REQUIRED before each commit:
go test -short ./pkg/mcp/...                    # All tests pass
go fmt ./pkg/mcp/...                            # Code formatting
go build ./pkg/mcp/...                          # Must compile
go vet ./pkg/mcp/...                            # Vet checks pass

# TYPE SAFETY validation:
echo "interface{} count: $(rg "interface{}" pkg/mcp/ | wc -l) (target: <50)"
echo "Unsafe type assertions: $(rg "\.(" pkg/mcp/ | grep -v ", ok" | wc -l) (target: 0)"
echo "Panic usage: $(rg "panic(" pkg/mcp/ | wc -l) (target: 0 in library code)"

# CONSTANTS validation:
echo "Magic number 30: $(rg "30.*time\.Second" pkg/mcp/ | wc -l) (should be 0)"  
echo "Magic number 100: $(rg "100," pkg/mcp/ | wc -l) (should be minimal)"
```

### Daily Validation Commands
```bash
# Morning startup:
go test -short ./pkg/mcp/... && echo "✅ All systems working"

# After interface{} elimination:
echo "interface{} progress: $(rg "interface{}" pkg/mcp/ | wc -l) remaining"

# After type assertion fixes:
echo "Unsafe assertions: $(rg "\.(" pkg/mcp/ | grep -v ", ok" | wc -l) remaining" 

# After constant creation:
echo "Magic numbers replaced: checking common patterns..."
rg "30.*time\.Second\|100,\|1024\|2048\|4096" pkg/mcp/ | wc -l

# After panic elimination:
echo "Panic usage: $(rg "panic(" pkg/mcp/ | wc -l) instances"

# End of day:
go test ./... && echo "✅ All systems functional with type safety improvements"
```

## 🚨 CRITICAL COORDINATION POINTS

### Dependencies You Need:
- **WORKSTREAM BETA** RichError + generics - MUST be complete for type replacements
- **WORKSTREAM BETA** generic registry system - Needed for tool registry type safety

### Dependencies on Your Work:
- **Production reliability** depends on your type safety improvements
- **Future development** depends on your idiomatic Go patterns
- **Code maintainability** depends on your interface{} elimination

### Files You Own (Full Authority):
- `pkg/mcp/constants/` (entire package) - You create the constants system
- `pkg/mcp/types/config/` - You create typed configuration
- `pkg/mcp/types/transport/` - You create typed transport structures
- All interface{} replacements - You decide on specific type implementations

### Files to Coordinate On:
- `pkg/mcp/internal/orchestration/` - Work with BETA's generic types
- `pkg/mcp/client_factory.go` - Coordinate breaking changes
- Any file with panic() usage - Document breaking changes

## 📊 PROGRESS TRACKING

### Daily Metrics to Track:
```bash
# Interface{} elimination progress:
echo "Total interface{} instances: $(rg "interface{}" pkg/mcp/ | wc -l)"
echo "Critical path interface{}: $(rg "interface{}" pkg/mcp/internal/orchestration/ | wc -l)"
echo "Core interface{}: $(rg "interface{}" pkg/mcp/internal/core/ | wc -l)"

# Type assertion safety:
echo "Total type assertions: $(rg "\.(" pkg/mcp/ | wc -l)"  
echo "Unsafe type assertions: $(rg "\.(" pkg/mcp/ | grep -v ", ok" | wc -l)"

# Constants replacement:
echo "Constants created: $(find pkg/mcp/constants -name "*.go" | wc -l) files"
echo "Magic numbers remaining: $(rg "30\s|100,|1024|2048|4096" pkg/mcp/ | wc -l)"

# Panic elimination:
echo "Panic usage: $(rg "panic(" pkg/mcp/ | wc -l) instances"
echo "Library panic usage: $(rg "panic(" pkg/mcp/internal/ pkg/mcp/types/ | wc -l)"
```

### Daily Summary Format:
```
WORKSTREAM EPSILON - DAY X SUMMARY
==================================
Progress: X% complete (target: 85% interface{} reduction)
Interface{} instances: X (started: 300+, target: <50)
Type assertions fixed: X (unsafe → safe)
Constants created: X files
Magic numbers replaced: X instances

Type safety improvements:
- interface{} eliminated: X instances
- Typed structs created: X
- Safe type assertions: X  
- Constants defined: X

Breaking changes introduced:
- [List any API changes that break compatibility]
- [Document migration path]

Files modified today:
- pkg/mcp/internal/orchestration/tool_orchestrator.go (interface{} → generics)
- pkg/mcp/constants/timeouts.go (created)
- [other files]

Coordination status:
- BETA generic types: [INTEGRATED/WAITING]
- BETA RichError: [INTEGRATED/WAITING]

Issues encountered:
- [any type conversion challenges]
- [performance concerns with type safety]

Tomorrow's focus:
- [next interface{} elimination targets]
- [constants creation priorities]

Quality status: All tests passing ✅
Type safety: X% compile-time checking
Breaking changes: [DOCUMENTED/NONE]
```

## 🛡️ ERROR HANDLING & ROLLBACK

### If Things Go Wrong:
1. **Breaking changes break too much**: Add compatibility wrappers
2. **Type conversion fails**: Check generic type constraints from BETA
3. **Performance regression**: Profile type-safe vs interface{} performance
4. **Compilation fails**: Check type assertion logic and error handling

### Rollback Procedure:
```bash
# Emergency rollback:
git checkout HEAD~1 -- pkg/mcp/internal/orchestration/
git checkout HEAD~1 -- pkg/mcp/client_factory.go
git checkout HEAD~1 -- pkg/mcp/constants/

# Selective rollback of breaking changes:
git checkout HEAD~1 -- <specific-file-with-breaking-changes>
```

## 🎯 KEY IMPLEMENTATION PATTERNS

### Interface{} → Typed Struct Pattern:
```go
// BEFORE: Generic interface{} usage
config := map[string]interface{}{
    "timeout": 30,
    "retries": 3,
    "enabled": true,
}

// AFTER: Typed struct with validation
type BuildConfig struct {
    Timeout time.Duration `json:"timeout" validate:"required,min=1s"`
    Retries int           `json:"retries" validate:"required,min=1,max=10"`
    Enabled bool          `json:"enabled"`
}

config := BuildConfig{
    Timeout: constants.DefaultTimeout,
    Retries: 3,
    Enabled: true,
}
```

### Unsafe → Safe Type Assertion Pattern:
```go
// BEFORE: Unsafe type assertion
result := tool.Execute(params)
buildResult := result.(BuildResult)  // Can panic!

// AFTER: Safe type assertion with RichError  
result := tool.Execute(params)
buildResult, ok := result.(BuildResult)
if !ok {
    return rich.NewError().
        Code("INVALID_RESULT_TYPE").
        Message("Tool returned unexpected result type").
        Type(rich.ErrTypeSystem).
        Context("expected_type", "BuildResult").
        Context("actual_type", reflect.TypeOf(result).String()).
        Build()
}
```

### Panic → Error Return Pattern:
```go
// BEFORE: Panic in library code (BREAKING)
func GetClient() Client {
    if factory == nil {
        panic("factory not initialized")
    }
    return factory.CreateClient()
}

// AFTER: Error return with RichError  
func GetClient() (Client, error) {
    if factory == nil {
        return nil, rich.NewError().
            Code("FACTORY_NOT_INITIALIZED").
            Message("Client factory not initialized").
            Type(rich.ErrTypeConfiguration).
            Severity(rich.SeverityHigh).
            Suggestion("Call InitializeFactory() before GetClient()").
            Build()
    }
    return factory.CreateClient(), nil
}
```

### Magic Number → Constant Pattern:
```go
// BEFORE: Magic numbers scattered throughout code
time.Sleep(30 * time.Second)
if errors > 100 { return }
buffer := make([]byte, 4096)

// AFTER: Named constants with documentation
time.Sleep(constants.DefaultTimeout)
if errors > constants.MaxErrors { return }
buffer := make([]byte, constants.MediumBufferSize)
```

## 🎯 FINAL DELIVERABLES

At completion, you must deliver:

1. **85% interface{} reduction** (300+ → <50 instances)
2. **Safe type assertions** with proper error handling throughout
3. **Constants package** replacing all magic numbers
4. **Panic-free library code** with proper error returns (BREAKING CHANGES)
5. **Typed configuration system** replacing generic maps
6. **Type-safe transport layer** with specific request/response types
7. **Documentation** of breaking changes and migration paths
8. **Updated coding standards** for idiomatic Go patterns

**Remember**: Your work makes the entire codebase more maintainable and reliable. Focus on creating compile-time type safety that catches errors before they reach production! 🚀

---

**CRITICAL**: Stop work and create summary at end of each day. Do not attempt merges - external coordination will handle branch management. Your job is to eliminate interface{} usage systematically and enforce idiomatic Go patterns while coordinating with BETA's type-safe foundations.