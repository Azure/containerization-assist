# AI Assistant Prompt: Workstream Delta - Structural Cleanup

## 🎯 Mission Brief
You are the **Lead Architect for Workstream Delta** in the Container Kit MCP architecture completion project. Your mission is to **consolidate and optimize the pkg/mcp directory structure** over **2 days**.

## 📋 Project Context
- **Repository**: Container Kit MCP server (`pkg/mcp/` directory)
- **Goal**: Consolidate scattered functionality and eliminate structural debt
- **Team**: 4 parallel workstreams (you are Delta - structural architect)
- **Timeline**: 2 days (coordinated with Alpha, Beta, and Gamma workstreams)
- **Impact**: Clean, maintainable architecture with clear boundaries

## 🚨 Critical Success Factors

### Must-Do Items
1. **Consolidate Error Handling**: Merge 5 scattered error locations into unified structure
2. **Unify Server Implementation**: Consolidate 15 server files across 4 packages
3. **Clean Interface Structure**: Eliminate duplicate/empty interface files
4. **Optimize Utilities**: Clarify public vs internal utility boundaries
5. **Simplify Session State Management**: Eliminate complex conversion patterns and dual SessionState types

### Must-Not-Do Items
- ❌ **Do NOT break existing imports** (maintain backward compatibility during transition)
- ❌ **Do NOT modify tool logic** (focus on structure, not functionality)
- ❌ **Do NOT change public APIs** (internal restructuring only)
- ❌ **Do NOT remove test files** (move them with their corresponding code)
- ❌ **Do NOT rush breaking changes** (incremental refactoring with validation)

## 📂 Your File Ownership (You Own These)

### Primary Targets (446 total Go files)
```
pkg/mcp/
├── errors/                    # PUBLIC: 2 files → CONSOLIDATE
├── internal/errors/           # INTERNAL: 2 files → MERGE TARGET
├── internal/runtime/errors.go # MERGE INTO: internal/errors/
├── internal/types/errors*.go  # MERGE INTO: internal/errors/ (3 files)
├── server/                    # PUBLIC: 1 file → KEEP AS BRIDGE
├── internal/core/server*.go   # INTERNAL: 7 files → CONSOLIDATE
├── internal/server/           # INTERNAL: 6 files → MERGE TARGET
├── interfaces.go             # PUBLIC: Empty redirect → REMOVE
├── core/interfaces.go        # PUBLIC: Main interfaces → KEEP
├── utils/                    # PUBLIC: 9 files → REVIEW/MOVE
└── internal/utils/           # INTERNAL: 18 files → TARGET
```

## 🔥 Day 1: Error Handling & Server Consolidation (8 hours)

### Morning (4 hours): Consolidate Error Handling

#### Task 1: Error Analysis & Planning (1 hour)
```bash
# CRITICAL: Map all error types and dependencies
echo "=== ERROR CONSOLIDATION ANALYSIS ==="

# Analyze current error structure
find pkg/mcp -name "*error*" -type f | sort
rg -n "type.*Error" pkg/mcp --type go
rg -n "fmt\.Errorf\|errors\." pkg/mcp/internal/ --type go | wc -l

# Document current error patterns
echo "Current error locations:"
echo "1. pkg/mcp/errors/ - Public error types"
echo "2. pkg/mcp/internal/errors/ - Internal error handling"
echo "3. pkg/mcp/internal/runtime/errors.go - Runtime errors"
echo "4. pkg/mcp/internal/types/errors*.go - Type errors (3 files)"

# Plan consolidation strategy
echo "Target structure:"
echo "pkg/mcp/internal/errors/"
echo "├── core.go        # Core system errors"
echo "├── tool.go        # Tool-specific errors"
echo "├── validation.go  # Validation errors"
echo "├── runtime.go     # Runtime errors"
echo "└── types.go       # Error type definitions"
```

#### Task 2: Create Unified Error Structure (2 hours)
```bash
# IMPLEMENTATION: New unified error structure
echo "=== CREATING UNIFIED ERROR STRUCTURE ==="

# Create new consolidated error files
# Target: pkg/mcp/internal/errors/

# 1. Merge pkg/mcp/internal/runtime/errors.go → pkg/mcp/internal/errors/runtime.go
# 2. Merge pkg/mcp/internal/types/errors*.go → pkg/mcp/internal/errors/types.go
# 3. Consolidate pkg/mcp/internal/errors/core_error.go patterns
# 4. Create pkg/mcp/internal/errors/tool.go for tool-specific errors
# 5. Create pkg/mcp/internal/errors/validation.go for validation errors

# Update imports throughout codebase
rg -l "pkg/mcp/internal/runtime.*errors" pkg/mcp --type go | xargs sed -i 's|pkg/mcp/internal/runtime.*errors|pkg/mcp/internal/errors|g'
rg -l "pkg/mcp/internal/types.*errors" pkg/mcp --type go | xargs sed -i 's|pkg/mcp/internal/types.*errors|pkg/mcp/internal/errors|g'
```

#### Task 3: Update Error References (1 hour)
```bash
# VALIDATION: Ensure all imports work
echo "=== UPDATING ERROR REFERENCES ==="

# Update all files that import scattered error packages
find pkg/mcp -name "*.go" -exec grep -l "errors\|Error" {} \; | head -20

# Validate compilation after changes
go build ./pkg/mcp/...
if [ $? -ne 0 ]; then
    echo "❌ COMPILATION FAILED - Fix imports"
    exit 1
fi

# Run targeted error-related tests
go test ./pkg/mcp/internal/errors/... -v
go test ./pkg/mcp/internal/types/... -v
```

### Afternoon (4 hours): Server Consolidation

#### Task 4: Server Structure Analysis (1 hour)
```bash
# CRITICAL: Map server implementation spread
echo "=== SERVER CONSOLIDATION ANALYSIS ==="

# Current server locations (15 files across 4 packages):
echo "Current server structure:"
echo "1. pkg/mcp/server/server.go - Public API bridge"
echo "2. pkg/mcp/internal/core/server*.go - Core server (7 files)"
echo "3. pkg/mcp/internal/server/ - Additional tools (6 files)"
echo "4. pkg/mcp/cmd/mcp-server/ - Command entry point"

# Analyze server dependencies
rg -n "type.*Server" pkg/mcp --type go
rg -n "func.*Server" pkg/mcp --type go

# Plan unified structure
echo "Target server structure:"
echo "pkg/mcp/internal/server/"
echo "├── core.go         # Core server implementation"
echo "├── lifecycle.go    # Server lifecycle management"
echo "├── transport.go    # Transport layer"
echo "├── handlers.go     # Request handlers"
echo "├── config.go       # Server configuration"
echo "└── tools.go        # Server tool utilities"
```

#### Task 5: Consolidate Server Implementation (2.5 hours)
```bash
# IMPLEMENTATION: Unified server structure
echo "=== CONSOLIDATING SERVER IMPLEMENTATION ==="

# Merge server functionality into pkg/mcp/internal/server/
# 1. Merge pkg/mcp/internal/core/server.go → pkg/mcp/internal/server/core.go
# 2. Merge pkg/mcp/internal/core/server_lifecycle*.go → pkg/mcp/internal/server/lifecycle.go
# 3. Merge pkg/mcp/internal/core/server_config*.go → pkg/mcp/internal/server/config.go
# 4. Merge existing pkg/mcp/internal/server/ tools into appropriate files
# 5. Keep pkg/mcp/server/server.go as public API bridge

# Update server imports throughout codebase
rg -l "pkg/mcp/internal/core.*server" pkg/mcp --type go | head -10

# Maintain public API compatibility in pkg/mcp/server/server.go
echo "// Bridge imports to new internal structure" >> pkg/mcp/server/server.go
```

#### Task 6: Validate Server Consolidation (0.5 hours)
```bash
# VALIDATION: Server functionality intact
echo "=== VALIDATING SERVER CONSOLIDATION ==="

# Test server compilation
go build ./pkg/mcp/server/...
go build ./pkg/mcp/internal/server/...

# Test cmd compilation (critical dependency)
go build ./pkg/mcp/cmd/mcp-server/...

# Run server-related tests
go test ./pkg/mcp/internal/server/... -v
```

## 🔧 Day 2: Interface, Utility & Session Optimization (8 hours)

### Morning (4 hours): Interface Structure Cleanup

#### Task 7: Interface Analysis & Planning (1 hour)
```bash
# CRITICAL: Map interface duplication
echo "=== INTERFACE CLEANUP ANALYSIS ==="

# Current interface structure:
echo "Current interface locations:"
echo "1. pkg/mcp/interfaces.go - Empty redirect (202 bytes)"
echo "2. pkg/mcp/core/interfaces.go - Main interfaces (unified source)"
echo "3. pkg/mcp/types/ - Type definitions (6 files)"
echo "4. pkg/mcp/internal/types/ - Internal types (12 files)"

# Analyze interface usage
rg -n "interface\s*{" pkg/mcp --type go | wc -l
wc -l pkg/mcp/interfaces.go pkg/mcp/core/interfaces.go

# Plan cleanup strategy
echo "Target interface structure:"
echo "pkg/mcp/core/interfaces.go    # Single source of truth (keep)"
echo "pkg/mcp/types/               # Consolidated public types"
echo "pkg/mcp/internal/types/      # Internal types only"
```

#### Task 8: Remove Empty Interface Files (1 hour)
```bash
# IMPLEMENTATION: Clean empty/duplicate interfaces
echo "=== REMOVING EMPTY INTERFACE FILES ==="

# Remove empty redirect file
echo "Removing pkg/mcp/interfaces.go (empty redirect)"
rm pkg/mcp/interfaces.go

# Update any imports pointing to empty file
rg -l "pkg/mcp.*interfaces" pkg/mcp --type go | head -10
# Replace with core/interfaces imports where needed

# Validate no broken imports
go build ./pkg/mcp/...
```

#### Task 9: Consolidate Type Definitions (2 hours)
```bash
# IMPLEMENTATION: Merge overlapping type definitions
echo "=== CONSOLIDATING TYPE DEFINITIONS ==="

# Analyze type overlap between pkg/mcp/types/ and pkg/mcp/internal/types/
echo "Public types (should be public APIs):"
ls pkg/mcp/types/

echo "Internal types (implementation details):"
ls pkg/mcp/internal/types/

# Strategy: Move truly internal types, keep public APIs separate
# Review each type file for public vs internal usage
rg -l "package types" pkg/mcp/types/ --type go
rg -l "package types" pkg/mcp/internal/types/ --type go

# Move internal-only types from pkg/mcp/types/ to pkg/mcp/internal/types/
# Keep client-facing types in pkg/mcp/types/
```

### Afternoon (4 hours): Utility Package Optimization

#### Task 10: Utility Analysis & Categorization (1 hour)
```bash
# CRITICAL: Categorize public vs internal utilities
echo "=== UTILITY PACKAGE ANALYSIS ==="

# Current utility structure:
echo "Public utilities (pkg/mcp/utils/): 9 files"
ls pkg/mcp/utils/

echo "Internal utilities (pkg/mcp/internal/utils/): 18 files"
ls pkg/mcp/internal/utils/

# Analyze which utilities should be public
echo "Analyzing utility usage patterns..."
for file in pkg/mcp/utils/*.go; do
    echo "=== $file ==="
    rg -l "$(basename "$file" .go)" pkg/ --type go | head -3
done

# Categorize utilities:
echo "Likely PUBLIC utilities (client-facing):"
echo "- logging.go - Client logging utilities"
echo "- validation_utils.go - Public validation helpers"
echo "- error_sanitizer.go - Client error handling"

echo "Likely INTERNAL utilities (implementation):"
echo "- sandbox_executor.go - Internal execution"
echo "- workspace.go - Internal workspace management"
echo "- secret_scanner.go - Internal security scanning"
```

#### Task 11: Move Internal Utilities (1 hour)
```bash
# IMPLEMENTATION: Optimize utility boundaries
echo "=== OPTIMIZING UTILITY BOUNDARIES ==="

# Move clearly internal utilities from pkg/mcp/utils/ to pkg/mcp/internal/utils/
# Examples of likely moves:
# - pkg/mcp/utils/path_utils.go → pkg/mcp/internal/utils/
# - pkg/mcp/utils/sanitization_utils.go → pkg/mcp/internal/utils/

# Keep only truly public utilities in pkg/mcp/utils/:
# - logging.go (if used by clients)
# - validation_utils.go (if public API)
# - error_sanitizer.go (if client-facing)

# Update imports throughout codebase
echo "Updating utility imports..."
rg -l "pkg/mcp/utils" pkg/mcp --type go | head -10
```

#### Task 12: Session State Simplification (2 hours)
```bash
# CRITICAL: Simplify session state conversion patterns
echo "=== SESSION STATE SIMPLIFICATION ==="

# Current problem: Complex conversion patterns in 30+ files:
# sessionInterface.(*sessiontypes.SessionState) → .ToCoreSessionState()
# sessionInterface.(*core.SessionState) - direct assertion
# sessionInterface.(*session.SessionState) - legacy pattern

# Analysis of current patterns:
echo "Current session conversion patterns:"
rg -n "sessionInterface.*\(\*.*SessionState\)" pkg/mcp --type go | wc -l
rg -n "ToCoreSessionState" pkg/mcp --type go

# Strategy 1: Create session adapter methods
echo "Creating session adapter utilities..."
# Add helper methods to eliminate repetitive conversion boilerplate

# Strategy 2: Unify session interfaces
echo "Analyzing session interface unification..."
# Investigate consolidating core.SessionState and internal SessionState

# Update conversion patterns throughout codebase
echo "Simplifying session conversions in analyze tools..."
# Focus on analyze_repository_atomic.go and other heavy users

# Benefits:
# - Eliminate 30+ repetitive conversion patterns
# - Reduce cognitive load for session handling
# - Simplify testing and debugging
# - Clear single pattern for session access
```

#### Task 12: Validate & Optimize Structure (1 hour)
```bash
# VALIDATION: Complete structural validation
echo "=== FINAL STRUCTURAL VALIDATION ==="

# Comprehensive build check
echo "Building all packages..."
go build ./pkg/mcp/...
if [ $? -ne 0 ]; then
    echo "❌ STRUCTURAL CHANGES BROKE BUILD"
    exit 1
fi

# Test all packages
echo "Testing structural changes..."
go test ./pkg/mcp/... -v

# Generate final structure report
echo "=== FINAL STRUCTURE REPORT ==="
echo "Total Go files: $(find pkg/mcp -name "*.go" | wc -l)"
echo "Internal files: $(find pkg/mcp/internal -name "*.go" | wc -l)"
echo "Public API files: $(find pkg/mcp -name "*.go" -not -path "*/internal/*" -not -path "*/cmd/*" | wc -l)"

echo "Error handling locations: $(find pkg/mcp -name "*error*" -type d | wc -l)"
echo "Server implementation files: $(find pkg/mcp -name "*server*" -type f | wc -l)"
echo "Interface files: $(find pkg/mcp -name "*interface*" -type f | wc -l)"
echo "Utility locations: $(find pkg/mcp -name "*utils*" -o -name "*util*" -type d | wc -l)"
```

## 🎯 Success Metrics

### Quantitative Targets
- **Error Locations**: Reduced from 5 to 1 (pkg/mcp/internal/errors/)
- **Server Files**: Consolidated from 15 files across 4 packages to 6 files in 1 package
- **Interface Duplication**: Eliminated empty/duplicate interface files
- **Utility Clarity**: Clear separation of 9 public vs 18 internal utilities

### Qualitative Goals
- **Import Clarity**: Developers know exactly where to find functionality
- **Maintenance Simplicity**: Single location for each cross-cutting concern
- **API Boundaries**: Clear distinction between public and internal APIs
- **Build Performance**: Reduced compilation dependencies through better organization

## 📚 Reference Materials

- **Current Structure**: `/pkg/mcp/` directory analysis
- **Architecture Patterns**: Study existing clean domain separation in `internal/analyze/`, `internal/build/`
- **Import Dependencies**: Use `go mod graph` to understand current dependencies
- **CLAUDE.md**: Project build commands for validation

## 🔄 End-of-Day Process

```bash
# At the end of each day - MANDATORY VALIDATION STEPS:

# 1. CRITICAL: Full compilation check
echo "=== STEP 1: COMPILATION VALIDATION ==="
go build ./pkg/mcp/...
if [ $? -ne 0 ]; then
    echo "❌ STRUCTURAL CHANGES BROKE COMPILATION"
    echo "Must fix before ending day"
    exit 1
fi

# 2. CRITICAL: Test suite validation
echo "=== STEP 2: TEST VALIDATION ==="
go test ./pkg/mcp/... -timeout=60s
if [ $? -ne 0 ]; then
    echo "❌ STRUCTURAL CHANGES BROKE TESTS"
    echo "Must fix before ending day"
    exit 1
fi

# 3. CRITICAL: Import validation
echo "=== STEP 3: IMPORT VALIDATION ==="
# Check for broken imports
go list -f '{{.ImportPath}}: {{.Incomplete}}' ./pkg/mcp/... | grep -v "false"
if [ $? -eq 0 ]; then
    echo "❌ INCOMPLETE PACKAGES DETECTED"
    echo "Must fix broken imports before ending day"
    exit 1
fi

# 4. Documentation of changes
echo "=== STEP 4: CHANGE DOCUMENTATION ==="
echo "Documenting structural changes made today..."
echo "Files moved: [list]"
echo "Packages consolidated: [list]"
echo "Imports updated: [count]"

echo "✅ STRUCTURAL CLEANUP DAY COMPLETE"
echo "Ready for next workstream coordination"
```

## 🚨 Critical Coordination Points

### With Workstream Alpha (Auto-fixing)
- **DO NOT** modify auto-fixing logic during structural moves
- **COORDINATE** on any imports to fixing components
- **NOTIFY** before moving error handling that auto-fixing depends on

### With Workstream Beta (Technical Debt)
- **COORDINATE** on TODO items in files being moved
- **SHARE** consolidated error structure for their TODO resolution
- **SEQUENCE** structural moves before their detailed TODO fixes

### With Workstream Gamma (Quality Assurance)
- **VALIDATE** all structural changes pass their quality gates
- **REQUEST** approval before major structural reorganization
- **COORDINATE** on test file movements and updates

## 📈 Impact Assessment

**Estimated Effort**: 16 hours over 2 days
**Files Affected**: ~100 files (imports/moves)
**Risk Level**: Medium (many import changes)
**Benefit Impact**: High (long-term maintainability)

**Success Indicators**:
- Single import path for each concern (errors, server, interfaces, utils)
- Reduced cognitive load when finding functionality
- Clear public vs internal API boundaries
- Faster onboarding for new developers

This workstream focuses on the foundational structure that will make all future development more efficient and maintainable.
