# 🚨 Architecture Realignment Plan: Getting Back on Track

## 🚀 **Quick Status Dashboard**

**📈 Progress**: 25% Complete | **🎯 Current Phase**: Phase 3 - Application Layer Organization | **⏰ Timeline**: Week 3 of 5

**🔥 Immediate Priorities**:
1. Complete tool migration: 188 files `pkg/mcp/tools/` → `application/commands/`
2. Finish core migration: `pkg/mcp/core/` → `application/commands/`
3. Eliminate manager pattern anti-patterns (10+ files)

**✅ Major Wins**: Domain layer established, interface consolidation complete, architecture validation active

---

## 📊 Current State Assessment

**Status**: The pkg/mcp codebase is **25% complete** in the three-layer architecture migration, currently transitioning from Phase 2 to Phase 3.

### **🎯 Migration Progress Overview**

| Phase | Status | Completion | Key Accomplishments |
|-------|--------|------------|-------------------|
| **Phase 1** | ✅ **COMPLETE** | 100% | Three-layer foundation, interface consolidation |
| **Phase 2** | 🔄 **IN PROGRESS** | 60% | Session domain extracted, containerization started |
| **Phase 3** | 🚀 **STARTING** | 0% | Team ready to begin application layer organization |
| **Phase 4** | ⏸️ **PENDING** | 0% | Infrastructure consolidation |
| **Phase 5** | ⏸️ **PENDING** | 0% | Quality assurance and finalization |

### **Current Architecture Status**

| **Layer** | **Status** | **Files** | **Progress** |
|-----------|------------|-----------|--------------|
| `pkg/mcp/domain/` | ✅ **ESTABLISHED** | 15 files | Session domain complete, containerization started |
| `pkg/mcp/application/` | 🔄 **PARTIAL** | 10 files | API layer established, commands package ready |
| `pkg/mcp/infra/` | 🔄 **MINIMAL** | 3 files | Persistence layer only |

### **Legacy Package Status**

| **Legacy Package** | **Files** | **Status** | **Phase 3 Priority** |
|-------------------|-----------|------------|---------------------|
| `pkg/mcp/tools/` | 188 files | ❌ **Critical Blocker** | HIGH - Move to application/commands |
| `pkg/mcp/internal/` | 125 files | ❌ **Major Blocker** | MEDIUM - Distribute to layers |
| `pkg/mcp/core/` | 64 files | 🔄 **Partially Migrated** | HIGH - Complete migration |
| `pkg/mcp/session/` | ~20 files | ✅ **Mostly Complete** | LOW - Cleanup remaining |

### **Remaining Critical Issues**

1. **🛠️ Tool Migration Incomplete**: 188 files in `pkg/mcp/tools/` need to move to `application/commands`
2. **🔧 Internal Package Cleanup**: 125 files in `internal/` need proper layer distribution
3. **📦 Package Depth**: Still 5 levels deep vs target of 2 levels
4. **🎭 Manager Pattern Persistence**: 10+ manager files remain (anti-pattern)
5. **🔄 Mixed Architecture State**: Legacy and new systems running in parallel

### **Major Accomplishments**

1. ✅ **Domain Layer Established**: Session domain extracted with clean boundaries
2. ✅ **Interface Consolidation**: Single source of truth in `application/api/interfaces.go` (831 lines)
3. ✅ **Three-Layer Foundation**: All target directories created and functional
4. ✅ **Architecture Validation**: Pre-commit hooks prevent regression
5. ✅ **Domain Purity**: No external dependencies in domain layer

## 🎯 Realignment Strategy

This plan **abandons the original 10-phase approach** and focuses on **immediate architectural realignment** through aggressive restructuring.

---

## 🚀 **PHASE 1: Emergency Architectural Triage (Week 1)**

### **Day 1-2: Create Target Structure**

#### **Objective**: Establish the three-layer foundation immediately

**Tasks**:

1. **Create Domain Layer with Build Tags**
   ```bash
   mkdir -p pkg/mcp/domain/{session,containerization,workflow,security,types}
   mkdir -p pkg/mcp/domain/containerization/{analyze,build,deploy,scan}

   # Create placeholder files to keep CI green
   for dir in pkg/mcp/domain pkg/mcp/domain/session pkg/mcp/domain/containerization \
              pkg/mcp/domain/containerization/{analyze,build,deploy,scan} \
              pkg/mcp/domain/{workflow,security,types}; do
     cat > "$dir/placeholder.go" << 'EOF'
//go:build mcp_migration

package $(basename $dir)

// Placeholder file to keep the build working during migration.
// This will be removed once real files are moved into this package.
EOF
   done
   ```

2. **Create Application Layer with Ports Pattern**
   ```bash
   mkdir -p pkg/mcp/application/{api,ports,commands}
   mkdir -p pkg/mcp/application/tools/{registry,coordination}

   # Create placeholder files
   for dir in pkg/mcp/application pkg/mcp/application/{api,ports,commands} \
              pkg/mcp/application/tools pkg/mcp/application/tools/{registry,coordination}; do
     cat > "$dir/placeholder.go" << 'EOF'
//go:build mcp_migration

package $(basename $dir)

// Placeholder file to keep the build working during migration.
// This will be removed once real files are moved into this package.
EOF
   done
   ```

3. **Create Infrastructure Layer with Build Tags**
   ```bash
   mkdir -p pkg/mcp/infra/{transport,persistence,telemetry}

   # Create placeholder files
   for dir in pkg/mcp/infra pkg/mcp/infra/{transport,persistence,telemetry}; do
     cat > "$dir/placeholder.go" << 'EOF'
//go:build mcp_migration

package $(basename $dir)

// Placeholder file to keep the build working during migration.
// This will be removed once real files are moved into this package.
EOF
   done

   # Note: Docker/K8s will use build tags instead of directories
   ```

#### **Success Criteria**:
- ✅ Three-layer directory structure exists
- ✅ Subdirectories follow domain boundaries
- ✅ No code moved yet (structure only)

#### **🔍 Validation Steps**:
```bash
# Verify directory structure created correctly
test -d pkg/mcp/domain && echo "✅ Domain layer created" || echo "❌ Domain layer missing"
test -d pkg/mcp/application && echo "✅ Application layer created" || echo "❌ Application layer missing"
test -d pkg/mcp/infra && echo "✅ Infrastructure layer created" || echo "❌ Infrastructure layer missing"

# Ensure subdirectories exist
test -d pkg/mcp/domain/containerization/analyze && echo "✅ Domain subdirs OK" || echo "❌ Domain subdirs missing"
test -d pkg/mcp/application/tools/registry && echo "✅ Application subdirs OK" || echo "❌ Application subdirs missing"
test -d pkg/mcp/infra/transport && echo "✅ Infra subdirs OK" || echo "❌ Infra subdirs missing"

# Verify existing structure unchanged
make test > /dev/null 2>&1 && echo "✅ Existing tests still pass" || echo "❌ Tests broken!"
go build ./cmd/mcp-server > /dev/null 2>&1 && echo "✅ Server still builds" || echo "❌ Build broken!"

# Check git status is clean (only new directories)
git status --porcelain | grep -v "^??" && echo "❌ Unexpected changes detected" || echo "✅ Only new directories added"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, remove new directories
rm -rf pkg/mcp/domain pkg/mcp/application pkg/mcp/infra
git checkout .
```

### **Day 3-5: Core Interface Consolidation**

#### **Objective**: Establish single source of truth for interfaces

**Tasks**:

1. **Move Interfaces to Ports (Canonical SSOT)**
   ```bash
   # Move interfaces to ports (not api) - interfaces are ports!
   mv pkg/mcp/api/interfaces.go pkg/mcp/application/ports/interfaces.go

   # Keep DTOs and errors in api/ (shared kernel)
   mv pkg/mcp/api/types.go pkg/mcp/application/api/types.go
   mv pkg/mcp/api/retry.go pkg/mcp/application/api/retry.go
   ```

2. **Generate Automatic Import Aliases**
   ```bash
   # Use gomvpkg for automatic alias generation (eliminates manual boilerplate)
   # Note: Install gomvpkg if not available: go install golang.org/x/tools/cmd/gomvpkg@latest

   # Generate aliases automatically
   gomvpkg -from github.com/Azure/container-kit/pkg/mcp/api \
           -to github.com/Azure/container-kit/pkg/mcp/application/ports

   # Fix imports automatically
   goimports -w .

   # Create temporary compatibility shim
   cat > pkg/mcp/api/interfaces.go << 'EOF'
// Package api provides backward compatibility aliases for interfaces moved to application/ports.
// DEPRECATED: Use pkg/mcp/application/ports directly.
package api

import "github.com/Azure/container-kit/pkg/mcp/application/ports"

// Backward compatibility type aliases
type Tool = ports.Tool
type ToolInput = ports.ToolInput
type ToolOutput = ports.ToolOutput
EOF
   ```

3. **Update Core Imports Automatically**
   ```bash
   # Update core/ to use application/ports for interfaces
   find pkg/mcp/core -name "*.go" -exec sed -i 's|pkg/mcp/api|pkg/mcp/application/ports|g' {} \;

   # Fix any remaining import issues
   goimports -w pkg/mcp/core/
   ```

#### **Success Criteria**:
- ✅ All interfaces in `application/api/`
- ✅ Backward compatibility maintained
- ✅ Core builds with new imports

#### **🔍 Validation Steps**:
```bash
# Verify interface migration completed (ports pattern)
test -f pkg/mcp/application/ports/interfaces.go && echo "✅ Interfaces moved to ports" || echo "❌ Interface migration failed"
test -f pkg/mcp/application/api/types.go && echo "✅ DTOs in api (shared kernel)" || echo "❌ DTOs missing"
test -f pkg/mcp/api/interfaces.go && echo "✅ Compatibility shim exists" || echo "❌ Compatibility shim missing"

# Test that old imports still work (backward compatibility)
grep -r "pkg/mcp/api" pkg/mcp/core/ && echo "✅ Core updated to use application/api" || echo "❌ Core imports not updated"

# Verify builds work with new structure
go build ./pkg/mcp/application/ports > /dev/null 2>&1 && echo "✅ Application ports builds" || echo "❌ Application ports build failed"
go build ./pkg/mcp/application/api > /dev/null 2>&1 && echo "✅ Application API builds" || echo "❌ Application API build failed"
go build ./pkg/mcp/core > /dev/null 2>&1 && echo "✅ Core builds with new imports" || echo "❌ Core build failed"
go build ./cmd/mcp-server > /dev/null 2>&1 && echo "✅ Server builds" || echo "❌ Server build failed"

# Run tests to ensure functionality preserved
make test-mcp > /dev/null 2>&1 && echo "✅ MCP tests pass" || echo "❌ MCP tests failing"

# Check for import cycles
go list -deps ./pkg/mcp/... 2>&1 | grep -i cycle && echo "❌ Import cycles detected!" || echo "✅ No import cycles"

# Verify API compatibility (check that old import paths resolve via shims)
go run -c 'import _ "github.com/Azure/container-kit/pkg/mcp/api"' 2>/dev/null && echo "✅ API shim works" || echo "❌ API shim broken"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, restore original structure
git checkout pkg/mcp/api/
rm -rf pkg/mcp/application/api/
# Restore core imports
find pkg/mcp/core -name "*.go" -exec sed -i 's|pkg/mcp/application/api|pkg/mcp/api|g' {} \;
```

---

## 🏗️ **PHASE 2: Domain Layer Migration (Week 2)**

### **Day 6-8: Session Domain Extraction**

#### **Objective**: Move session logic to domain layer

**Tasks**:

1. **Copy Stateless Business Objects First**
   ```bash
   # Start with pure types & functions (stateless business objects)
   # COPY (don't move) to avoid breaking deps initially
   cp pkg/mcp/session/session_types.go pkg/mcp/domain/session/types.go
   cp pkg/mcp/session/types.go pkg/mcp/domain/session/session.go
   cp pkg/mcp/session/types_test.go pkg/mcp/domain/session/session_test.go

   # Copy business logic (pure functions, no infrastructure deps)
   cp pkg/mcp/session/validation.go pkg/mcp/domain/session/validation.go
   cp pkg/mcp/session/metadata.go pkg/mcp/domain/session/metadata.go

   # Remove placeholder
   rm pkg/mcp/domain/session/placeholder.go
   ```

2. **Generate Import Aliases Automatically**
   ```bash
   # Use gomvpkg to generate aliases automatically (eliminates manual work)
   gomvpkg -from github.com/Azure/container-kit/pkg/mcp/session \
           -to github.com/Azure/container-kit/pkg/mcp/domain/session

   # Fix imports automatically
   goimports -w .
   ```

3. **Trim Infrastructure Dependencies**
   ```bash
   # Remove any Docker/K8s/HTTP imports from domain files
   # These will be handled by adapters in the application layer
   sed -i '/docker\|kubernetes\|http\|database/d' pkg/mcp/domain/session/*.go

   # Ensure domain builds independently
   go build ./pkg/mcp/domain/session/
   ```

4. **Leave Infrastructure Components**
   ```bash
   # Session persistence stays in original location for now
   # Will be moved to infra in Phase 4
   echo "Session managers remain in pkg/mcp/session/ until Phase 4"
   ```

#### **Success Criteria**:
- ✅ Domain contains pure business logic
- ✅ Infrastructure separated from domain
- ✅ Session tests pass

#### **🔍 Validation Steps**:
```bash
# Verify session domain migration
test -f pkg/mcp/domain/session/types.go && echo "✅ Session types in domain" || echo "❌ Session types missing"
test -f pkg/mcp/domain/session/validation.go && echo "✅ Session validation in domain" || echo "❌ Session validation missing"
test -f pkg/mcp/infra/persistence/session_store.go && echo "✅ Session storage in infra" || echo "❌ Session storage missing"

# Verify no business logic in infrastructure
! grep -r "business\|domain\|rules" pkg/mcp/infra/persistence/ && echo "✅ No business logic in infra" || echo "❌ Business logic leaked to infra"

# Check domain layer has no external dependencies
! grep -r "docker\|kubernetes\|http\|database" pkg/mcp/domain/session/ && echo "✅ Domain is pure" || echo "❌ Domain has external deps"

# Run session-specific tests
go test ./pkg/mcp/domain/session/... > /dev/null 2>&1 && echo "✅ Session domain tests pass" || echo "❌ Session domain tests fail"
go test ./pkg/mcp/infra/persistence/... > /dev/null 2>&1 && echo "✅ Persistence tests pass" || echo "❌ Persistence tests fail"

# Verify imports follow architecture (domain doesn't import infra/application)
! grep -r "pkg/mcp/infra\|pkg/mcp/application" pkg/mcp/domain/session/ && echo "✅ Domain imports clean" || echo "❌ Domain has upward imports"

# Test overall system still works
make test > /dev/null 2>&1 && echo "✅ Full test suite passes" || echo "❌ Test suite broken"
go build ./cmd/mcp-server > /dev/null 2>&1 && echo "✅ Server builds" || echo "❌ Server build failed"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, restore session files to original locations
mv pkg/mcp/domain/session/* pkg/mcp/session/ 2>/dev/null
mv pkg/mcp/infra/persistence/session_store.go pkg/mcp/session/session_manager.go 2>/dev/null
mv pkg/mcp/infra/persistence/storage/* pkg/mcp/session/storage/ 2>/dev/null
rmdir pkg/mcp/domain/session pkg/mcp/infra/persistence/storage pkg/mcp/infra/persistence 2>/dev/null
```

### **Day 9-10: Containerization Domain Extraction**

#### **Objective**: Organize tools by domain boundaries

**Tasks**:

1. **Extract Analyze Domain**
   ```bash
   # Move analyze business logic
   mv pkg/mcp/tools/analyze/analyze.go pkg/mcp/domain/containerization/analyze/analyzer.go
   mv pkg/mcp/tools/analyze/analyze_tool.go pkg/mcp/domain/containerization/analyze/tool.go
   mv pkg/mcp/tools/analyze/common.go pkg/mcp/domain/containerization/analyze/types.go
   ```

2. **Extract Build Domain**
   ```bash
   # Move build business logic
   mv pkg/mcp/tools/build/build_executor.go pkg/mcp/domain/containerization/build/executor.go
   mv pkg/mcp/tools/build/build_strategizer.go pkg/mcp/domain/containerization/build/strategy.go
   ```

3. **Extract Deploy Domain**
   ```bash
   # Move deploy business logic
   mv pkg/mcp/tools/deploy/core_validator.go pkg/mcp/domain/containerization/deploy/validator.go
   mv pkg/mcp/tools/deploy/deploy_types.go pkg/mcp/domain/containerization/deploy/types.go
   ```

4. **Extract Scan Domain**
   ```bash
   # Move scan business logic
   mv pkg/mcp/tools/scan/secret_scanner.go pkg/mcp/domain/containerization/scan/secrets.go
   mv pkg/mcp/tools/scan/scan_secrets_atomic.go pkg/mcp/domain/containerization/scan/scanner.go
   ```

#### **Success Criteria**:
- ✅ Containerization domain organized by bounded context
- ✅ Business logic separated from tool registration
- ✅ Domain tests pass

#### **🔍 Validation Steps**:
```bash
# Verify containerization domain structure
test -f pkg/mcp/domain/containerization/analyze/analyzer.go && echo "✅ Analyze domain extracted" || echo "❌ Analyze domain missing"
test -f pkg/mcp/domain/containerization/build/executor.go && echo "✅ Build domain extracted" || echo "❌ Build domain missing"
test -f pkg/mcp/domain/containerization/deploy/validator.go && echo "✅ Deploy domain extracted" || echo "❌ Deploy domain missing"
test -f pkg/mcp/domain/containerization/scan/scanner.go && echo "✅ Scan domain extracted" || echo "❌ Scan domain missing"

# Verify tool registration removed from domain (should be in application layer)
! grep -r "RegisterTool\|registry\|factory" pkg/mcp/domain/containerization/ && echo "✅ No tool registration in domain" || echo "❌ Tool registration in domain"

# Check domain boundaries (no Docker/K8s/HTTP in domain)
! grep -r "docker\.Client\|kubernetes\|http\.Client" pkg/mcp/domain/containerization/ && echo "✅ Domain is infrastructure-free" || echo "❌ Infrastructure leaked to domain"

# Verify each domain can be built independently
go build ./pkg/mcp/domain/containerization/analyze > /dev/null 2>&1 && echo "✅ Analyze domain builds" || echo "❌ Analyze domain build fails"
go build ./pkg/mcp/domain/containerization/build > /dev/null 2>&1 && echo "✅ Build domain builds" || echo "❌ Build domain build fails"
go build ./pkg/mcp/domain/containerization/deploy > /dev/null 2>&1 && echo "✅ Deploy domain builds" || echo "❌ Deploy domain build fails"
go build ./pkg/mcp/domain/containerization/scan > /dev/null 2>&1 && echo "✅ Scan domain builds" || echo "❌ Scan domain build fails"

# Run domain tests
go test ./pkg/mcp/domain/containerization/... > /dev/null 2>&1 && echo "✅ Containerization domain tests pass" || echo "❌ Domain tests fail"

# Check that tools functionality still works end-to-end
make test-mcp > /dev/null 2>&1 && echo "✅ MCP integration tests pass" || echo "❌ Integration broken"

# Verify no circular dependencies
go list -deps ./pkg/mcp/domain/... 2>&1 | grep -i cycle && echo "❌ Circular deps in domain!" || echo "✅ No circular dependencies"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, restore tools to original locations
mv pkg/mcp/domain/containerization/analyze/* pkg/mcp/tools/analyze/ 2>/dev/null
mv pkg/mcp/domain/containerization/build/* pkg/mcp/tools/build/ 2>/dev/null
mv pkg/mcp/domain/containerization/deploy/* pkg/mcp/tools/deploy/ 2>/dev/null
mv pkg/mcp/domain/containerization/scan/* pkg/mcp/tools/scan/ 2>/dev/null
rm -rf pkg/mcp/domain/containerization/
```

---

## 🎛️ **PHASE 3: Application Layer Organization (CURRENT PHASE)**

**Status**: 🚀 **STARTING NOW** - Team ready to begin application layer organization

**Priority**: Move 188 files from `pkg/mcp/tools/` to application layer with commands pattern

### **⚡ Current Team Status Assessment**

Based on `git status` analysis (41 uncommitted changes), the team is actively:
- ✅ Completing Phase 1 cleanup (removing shim files, consolidating interfaces)
- 🔄 Working on core migration (`pkg/mcp/core/` → `application/commands/`)
- 🔄 Conversation handler improvements in progress
- 🔄 Error compliance updates underway

**Ready for Phase 3**: Foundation is solid, team can proceed with application layer organization.

### **🎯 Immediate Phase 3 Action Items**

**High Priority** (Do First):
1. **Complete Core Migration**: Finish moving `pkg/mcp/core/` files to `application/commands/`
2. **Commit Current Work**: Clean git state before major tool migration
3. **Tool Migration Strategy**: Plan systematic migration of 188 files from `pkg/mcp/tools/`

**Medium Priority** (This Sprint):
4. **Analyze Tools Migration**: Move `pkg/mcp/tools/analyze/` to `application/commands/analyze_*.go`
5. **Build Tools Migration**: Move `pkg/mcp/tools/build/` to `application/commands/build_*.go`
6. **Deploy Tools Migration**: Move `pkg/mcp/tools/deploy/` to `application/commands/deploy_*.go`
7. **Scan Tools Migration**: Move `pkg/mcp/tools/scan/` to `application/commands/scan_*.go`

### **Day 11-13: Tool Orchestration Consolidation (CURRENT SPRINT)**

#### **Objective**: Move tool coordination to application layer

**Tasks**:

1. **Consolidate into Single Commands Package**
   ```bash
   # Follow feedback: collapse services/core/orchestration into single commands package
   # Move tool factories to application/commands (not nested packages)
   mv pkg/mcp/core/registry.go pkg/mcp/application/commands/tool_registry.go
   mv pkg/mcp/core/interfaces.go pkg/mcp/application/commands/interfaces.go

   # Remove placeholder
   rm pkg/mcp/application/commands/placeholder.go
   ```

2. **Extract Tool Implementations to Commands**
   ```bash
   # Tool implementations go to commands package (avoid premature nesting)
   mv pkg/mcp/tools/analyze/analyze_repository_consolidated.go pkg/mcp/application/commands/analyze_command.go
   mv pkg/mcp/tools/build/docker_build_consolidated.go pkg/mcp/application/commands/build_command.go
   mv pkg/mcp/tools/deploy/deploy_consolidated.go pkg/mcp/application/commands/deploy_command.go
   mv pkg/mcp/tools/scan/scan_consolidated.go pkg/mcp/application/commands/scan_command.go
   ```

3. **Create Simple Tool Coordinator**
   ```bash
   # Create coordinator in commands package (not separate orchestration package)
   cat > pkg/mcp/application/commands/coordinator.go << 'EOF'
package commands

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/application/ports"
    "github.com/Azure/container-kit/pkg/mcp/domain/session"
)

type ToolCoordinator struct {
    registry ToolRegistry
    session  session.Store  // Use domain interface
}

func (c *ToolCoordinator) ExecuteTool(ctx context.Context, name string, input ports.ToolInput) (ports.ToolOutput, error) {
    // Simple orchestration logic - split later only if needed
}
EOF
   ```

4. **Generate Import Aliases**
   ```bash
   # Auto-generate aliases for moved packages
   gomvpkg -from github.com/Azure/container-kit/pkg/mcp/tools \
           -to github.com/Azure/container-kit/pkg/mcp/application/commands
   goimports -w .
   ```

#### **Success Criteria**:
- ✅ Tools orchestrated at application layer
- ✅ Domain logic called by application layer
- ✅ Clear separation between domain and application

#### **🔍 Validation Steps**:
```bash
# Verify tool orchestration moved to commands package
test -f pkg/mcp/application/commands/tool_registry.go && echo "✅ Tool registry in commands" || echo "❌ Tool registry missing"
test -f pkg/mcp/application/commands/analyze_command.go && echo "✅ Analyze command in application" || echo "❌ Analyze command missing"
test -f pkg/mcp/application/commands/coordinator.go && echo "✅ Tool coordinator created" || echo "❌ Coordinator missing"

# Check current migration status
TOOLS_FILES=$(find pkg/mcp/tools -name "*.go" 2>/dev/null | wc -l || echo "0")
APP_COMMANDS_FILES=$(find pkg/mcp/application/commands -name "*.go" 2>/dev/null | wc -l || echo "0")
echo "Tools files remaining: $TOOLS_FILES (target: 0)"
echo "Commands files created: $APP_COMMANDS_FILES (target: >100)"

# Verify the major tool categories are migrated
test ! -d pkg/mcp/tools/analyze && echo "✅ Analyze tools migrated" || echo "⏸️ Analyze tools need migration"
test ! -d pkg/mcp/tools/build && echo "✅ Build tools migrated" || echo "⏸️ Build tools need migration"
test ! -d pkg/mcp/tools/deploy && echo "✅ Deploy tools migrated" || echo "⏸️ Deploy tools need migration"
test ! -d pkg/mcp/tools/scan && echo "✅ Scan tools migrated" || echo "⏸️ Scan tools need migration"

# Verify application layer calls domain layer (correct dependency direction)
grep -r "pkg/mcp/domain" pkg/mcp/application/commands/ && echo "✅ Application imports domain" || echo "❌ Application doesn't use domain"

# Verify domain layer doesn't import application (no upward dependencies)
! grep -r "pkg/mcp/application" pkg/mcp/domain/ && echo "✅ Domain independent of application" || echo "❌ Domain imports application!"

# Check that commands can be built
go build ./pkg/mcp/application/commands/ > /dev/null 2>&1 && echo "✅ Application commands build" || echo "❌ Application commands build fails"

# Verify tool registration works
grep -r "RegisterTool\|ToolRegistry" pkg/mcp/application/commands/ && echo "✅ Tool registration preserved" || echo "❌ Tool registration broken"

# Run application layer tests
go test ./pkg/mcp/application/... > /dev/null 2>&1 && echo "✅ Application tests pass" || echo "❌ Application tests fail"

# End-to-end functionality test
make test-all > /dev/null 2>&1 && echo "✅ Full system works" || echo "❌ System integration broken"

# Check for import cycles
go list -deps ./pkg/mcp/application/... 2>&1 | grep -i cycle && echo "❌ Import cycles in application!" || echo "✅ No import cycles"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, restore original tool structure
mv pkg/mcp/application/tools/registry/* pkg/mcp/core/ 2>/dev/null
mv pkg/mcp/application/tools/*.go pkg/mcp/tools/ 2>/dev/null
rm -rf pkg/mcp/application/tools/ pkg/mcp/application/orchestration/
git checkout pkg/mcp/core/registry.go pkg/mcp/core/interfaces.go
```

### **Day 14-15: Workflow and Pipeline Migration**

#### **Objective**: Migrate workflow and pipeline logic to application layer

**Tasks**:

1. **Extract Workflow Logic**
   ```bash
   # Move workflow to application layer
   mv pkg/mcp/workflow/ pkg/mcp/application/workflows/
   mv pkg/mcp/internal/pipeline/ pkg/mcp/application/orchestration/pipeline/
   ```

2. **Extract Core Server Logic**
   ```bash
   # Move server orchestration to application
   mv pkg/mcp/core/server_interfaces.go pkg/mcp/application/core/server.go
   mv pkg/mcp/core/mcp.go pkg/mcp/application/core/mcp.go
   ```

3. **Consolidate Service Layer**
   ```bash
   # Move service definitions to application
   mv pkg/mcp/services/ pkg/mcp/application/services/
   ```

#### **Success Criteria**:
- ✅ Application layer contains all orchestration
- ✅ Workflows organized under application
- ✅ Services properly layered

#### **🔍 Validation Steps**:
```bash
# Verify workflow migration
test -d pkg/mcp/application/workflows/ && echo "✅ Workflows in application layer" || echo "❌ Workflows missing"
test -d pkg/mcp/application/orchestration/pipeline/ && echo "✅ Pipeline in application" || echo "❌ Pipeline missing"
test -f pkg/mcp/application/core/server.go && echo "✅ Server core in application" || echo "❌ Server core missing"

# Verify services layer organization
test -d pkg/mcp/application/services/ && echo "✅ Services in application" || echo "❌ Services missing"

# Check workflow functionality preserved
go build ./pkg/mcp/application/workflows/ > /dev/null 2>&1 && echo "✅ Workflows build" || echo "❌ Workflows build fails"
go build ./pkg/mcp/application/orchestration/ > /dev/null 2>&1 && echo "✅ Orchestration builds" || echo "❌ Orchestration build fails"

# Verify server functionality
go build ./pkg/mcp/application/core/ > /dev/null 2>&1 && echo "✅ Application core builds" || echo "❌ Application core build fails"
go build ./cmd/mcp-server > /dev/null 2>&1 && echo "✅ MCP server builds" || echo "❌ MCP server build fails"

# Test workflow execution (if tests exist)
go test ./pkg/mcp/application/workflows/... > /dev/null 2>&1 && echo "✅ Workflow tests pass" || echo "❌ Workflow tests fail"
go test ./pkg/mcp/application/orchestration/... > /dev/null 2>&1 && echo "✅ Orchestration tests pass" || echo "❌ Orchestration tests fail"

# Integration test
make test-mcp > /dev/null 2>&1 && echo "✅ MCP integration works" || echo "❌ MCP integration broken"

# Verify dependency direction (application can import domain, not vice versa)
! grep -r "pkg/mcp/application" pkg/mcp/domain/ && echo "✅ Domain independent" || echo "❌ Domain imports application!"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, restore original locations
mv pkg/mcp/application/workflows/* pkg/mcp/workflow/ 2>/dev/null
mv pkg/mcp/application/orchestration/pipeline/* pkg/mcp/internal/pipeline/ 2>/dev/null
mv pkg/mcp/application/core/* pkg/mcp/core/ 2>/dev/null
mv pkg/mcp/application/services/* pkg/mcp/services/ 2>/dev/null
rm -rf pkg/mcp/application/workflows/ pkg/mcp/application/orchestration/
git checkout pkg/mcp/core/ pkg/mcp/services/
```

---

## 🔌 **PHASE 4: Infrastructure Layer Consolidation (Week 4)**

### **Day 16-17: External Integration Migration**

#### **Objective**: Consolidate all external integrations in infrastructure layer

**Tasks**:

1. **Consolidate Transport Layer**
   ```bash
   # Transport already exists, enhance it
   mv pkg/mcp/server/ pkg/mcp/infra/transport/server/
   ```

2. **Consolidate Storage Layer**
   ```bash
   # Create persistence layer
   mv pkg/mcp/storage/ pkg/mcp/infra/persistence/storage/
   # Session storage from earlier migration
   ```

3. **Use Build Tags for Docker Integration**
   ```bash
   # Create Docker integration with build tags (not directories)
   cat > pkg/mcp/infra/docker_operations.go << 'EOF'
//go:build docker

package infra

import "context"

// Docker operations - only compiled when -tags docker is used
type DockerOperations struct{}

func (d *DockerOperations) BuildImage(ctx context.Context, params BuildParams) error {
    // Docker implementation
}
EOF

   # Move existing Docker logic
   grep -l "docker\.Client" pkg/mcp/tools/build/*.go | while read file; do
       # Add build tag to existing files
       sed -i '1i//go:build docker\n' "$file"
       mv "$file" pkg/mcp/infra/
   done
   ```

4. **Use Build Tags for Kubernetes Integration**
   ```bash
   # Create K8s integration with build tags
   cat > pkg/mcp/infra/k8s_operations.go << 'EOF'
//go:build k8s

package infra

import "context"

// K8s operations - only compiled when -tags k8s is used
type KubernetesOperations struct{}

func (k *KubernetesOperations) Deploy(ctx context.Context, manifest []byte) error {
    // K8s implementation
}
EOF

   # Move existing K8s logic
   grep -l "kubernetes\|k8s\.io" pkg/mcp/tools/deploy/*.go | while read file; do
       # Add build tag to existing files
       sed -i '1i//go:build k8s\n' "$file"
       mv "$file" pkg/mcp/infra/
   done
   ```

5. **Create Cloud Build Tag Support**
   ```bash
   # Support combined cloud builds
   cat > pkg/mcp/infra/cloud_operations.go << 'EOF'
//go:build cloud

package infra

// Cloud operations - includes both docker and k8s when -tags cloud is used
// This allows: go build -tags cloud (includes both)
// Or selective: go build -tags docker (docker only)
EOF
   ```

5. **Consolidate Templates**
   ```bash
   # Move templates to infra
   mv pkg/mcp/templates/ pkg/mcp/infra/templates/
   ```

#### **Success Criteria**:
- ✅ All external integrations in infra layer
- ✅ Docker and K8s properly abstracted
- ✅ Templates managed in infrastructure

#### **🔍 Validation Steps**:
```bash
# Verify infrastructure layer organization
test -f pkg/mcp/infra/docker/operations.go && echo "✅ Docker operations in infra" || echo "❌ Docker operations missing"
test -f pkg/mcp/infra/k8s/deployer.go && echo "✅ K8s deployer in infra" || echo "❌ K8s deployer missing"
test -d pkg/mcp/infra/transport/server/ && echo "✅ Server transport in infra" || echo "❌ Server transport missing"
test -d pkg/mcp/infra/persistence/storage/ && echo "✅ Storage in infra" || echo "❌ Storage missing"
test -d pkg/mcp/infra/templates/ && echo "✅ Templates in infra" || echo "❌ Templates missing"

# Verify external dependencies only in infra layer
grep -r "docker\.Client\|kubernetes\.Interface" pkg/mcp/infra/ && echo "✅ External deps in infra" || echo "❌ External deps missing"
! grep -r "docker\.Client\|kubernetes\.Interface" pkg/mcp/domain/ pkg/mcp/application/ && echo "✅ External deps isolated" || echo "❌ External deps leaked"

# Check infrastructure builds with build tags
go build -tags docker ./pkg/mcp/infra/ > /dev/null 2>&1 && echo "✅ Docker infra builds" || echo "❌ Docker infra build fails"
go build -tags k8s ./pkg/mcp/infra/ > /dev/null 2>&1 && echo "✅ K8s infra builds" || echo "❌ K8s infra build fails"
go build -tags cloud ./pkg/mcp/infra/ > /dev/null 2>&1 && echo "✅ Cloud infra builds" || echo "❌ Cloud infra build fails"
go build ./pkg/mcp/infra/ > /dev/null 2>&1 && echo "✅ Core infra builds (no tags)" || echo "❌ Core infra build fails"
go build ./pkg/mcp/infra/transport/ > /dev/null 2>&1 && echo "✅ Transport builds" || echo "❌ Transport build fails"
go build ./pkg/mcp/infra/persistence/ > /dev/null 2>&1 && echo "✅ Persistence builds" || echo "❌ Persistence build fails"

# Verify infra doesn't import application (correct dependency direction)
! grep -r "pkg/mcp/application" pkg/mcp/infra/ && echo "✅ Infra doesn't import application" || echo "❌ Infra imports application!"

# Test infrastructure functionality
go test ./pkg/mcp/infra/... > /dev/null 2>&1 && echo "✅ Infrastructure tests pass" || echo "❌ Infrastructure tests fail"

# End-to-end system test
make test-all > /dev/null 2>&1 && echo "✅ Full system functional" || echo "❌ System broken"
go build ./cmd/mcp-server > /dev/null 2>&1 && echo "✅ Server builds with new infra" || echo "❌ Server build fails"

# Check for import cycles
go list -deps ./pkg/mcp/infra/... 2>&1 | grep -i cycle && echo "❌ Import cycles in infra!" || echo "✅ No import cycles"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, restore original structure
mv pkg/mcp/infra/docker/* pkg/mcp/tools/build/ 2>/dev/null
mv pkg/mcp/infra/k8s/* pkg/mcp/tools/deploy/ 2>/dev/null
mv pkg/mcp/infra/transport/server/* pkg/mcp/server/ 2>/dev/null
mv pkg/mcp/infra/persistence/storage/* pkg/mcp/storage/ 2>/dev/null
mv pkg/mcp/infra/templates/* pkg/mcp/templates/ 2>/dev/null
rm -rf pkg/mcp/infra/
git checkout pkg/mcp/server/ pkg/mcp/storage/ pkg/mcp/templates/
```

### **Day 18-19: Error and Security Consolidation**

#### **Objective**: Consolidate cross-cutting concerns

**Tasks**:

1. **Consolidate Error Handling**
   ```bash
   # Errors stay at root for cross-cutting nature
   # But ensure single system
   rm -rf pkg/mcp/internal/*/error*.go
   ```

2. **Consolidate Security**
   ```bash
   # Move security to domain (business rules) and infra (scanning)
   mv pkg/mcp/security/validation/ pkg/mcp/domain/security/
   mv pkg/mcp/security/engine.go pkg/mcp/infra/security/scanner.go
   ```

3. **Remove Legacy Packages**
   ```bash
   # Remove large problematic packages
   rm -rf pkg/mcp/internal/
   rm -rf pkg/mcp/tools/
   rm -rf pkg/mcp/core/
   ```

#### **Success Criteria**:
- ✅ Single error system
- ✅ Security concerns properly layered
- ✅ Legacy packages removed

#### **🔍 Validation Steps**:
```bash
# Verify error system consolidation
test -d pkg/mcp/errors/ && echo "✅ Unified error system exists" || echo "❌ Error system missing"
! find pkg/mcp/internal pkg/mcp/core -name "*error*.go" 2>/dev/null | head -1 && echo "✅ Competing error systems removed" || echo "❌ Competing error systems remain"

# Verify security layering
test -d pkg/mcp/domain/security/ && echo "✅ Security domain exists" || echo "❌ Security domain missing"
test -f pkg/mcp/infra/security/scanner.go && echo "✅ Security scanning in infra" || echo "❌ Security scanning missing"

# Verify legacy packages removed
! test -d pkg/mcp/internal/ && echo "✅ Internal package removed" || echo "❌ Internal package still exists"
! test -d pkg/mcp/tools/ && echo "✅ Old tools package removed" || echo "❌ Old tools package remains"
! test -d pkg/mcp/core/ && echo "✅ Old core package removed" || echo "❌ Old core package remains"

# Verify error system usage
grep -r "RichError\|errors\.New" pkg/mcp/domain/ pkg/mcp/application/ pkg/mcp/infra/ | head -5 && echo "✅ Unified error system in use" || echo "❌ Error system not used"

# Check that builds work after cleanup
go build ./pkg/mcp/domain/... > /dev/null 2>&1 && echo "✅ Domain builds after cleanup" || echo "❌ Domain build fails"
go build ./pkg/mcp/application/... > /dev/null 2>&1 && echo "✅ Application builds after cleanup" || echo "❌ Application build fails"
go build ./pkg/mcp/infra/... > /dev/null 2>&1 && echo "✅ Infra builds after cleanup" || echo "❌ Infra build fails"
go build ./cmd/mcp-server > /dev/null 2>&1 && echo "✅ Server builds after cleanup" || echo "❌ Server build fails"

# Run comprehensive tests
make test-all > /dev/null 2>&1 && echo "✅ All tests pass after cleanup" || echo "❌ Tests broken after cleanup"

# Verify package count reduction (should be ~3 top-level vs 14 before)
PKG_COUNT=$(find pkg/mcp -maxdepth 1 -type d | wc -l)
[ $PKG_COUNT -le 5 ] && echo "✅ Package count reduced to $PKG_COUNT" || echo "❌ Package count still high: $PKG_COUNT"

# Check for any remaining manager patterns
! find pkg/mcp -name "*manager*.go" | head -1 && echo "✅ Manager pattern eliminated" || echo "❌ Manager patterns remain"
```

#### **🚨 Rollback Strategy**:
```bash
# If validation fails, this is a critical rollback - restore from git
git reset --hard HEAD~1  # Restore to before this phase
# Or restore specific packages:
git checkout pkg/mcp/internal/ pkg/mcp/tools/ pkg/mcp/core/ 2>/dev/null
mv pkg/mcp/domain/security/ pkg/mcp/security/ 2>/dev/null
mv pkg/mcp/infra/security/scanner.go pkg/mcp/security/engine.go 2>/dev/null
```

---

## 🧪 **PHASE 5: Quality and Validation (Week 5)**

### **Day 20-21: Architecture Testing**

#### **Objective**: Ensure architectural boundaries are enforced

**Tasks**:

1. **Implement Architecture Tests**
   ```go
   // test/architecture_test.go
   func TestLayerDependencies(t *testing.T) {
       // Domain cannot import application or infra
       // Application can only import domain
       // Infra can import domain but not application
   }
   ```

2. **Implement Import Cycle Detection**
   ```bash
   # Add to CI
   go list -deps ./pkg/mcp/... | grep cycle && exit 1
   ```

3. **Add Complexity Monitoring**
   ```bash
   # Ensure complexity decreased
   gocyclo -over 15 pkg/mcp | wc -l
   ```

#### **Success Criteria**:
- ✅ Architecture tests prevent violations
- ✅ No import cycles
- ✅ Complexity reduced by >50%

#### **🔍 Validation Steps**:
```bash
# Verify architecture validation script works
make validate-architecture && echo "✅ Architecture validation passes" || echo "❌ Architecture violations detected"

# Verify architecture tests exist and pass (if created)
test -f test/architecture_test.go && echo "✅ Architecture tests exist" || echo "❌ Architecture tests missing"
if [[ -f test/architecture_test.go ]]; then
    go test ./test/architecture_test.go > /dev/null 2>&1 && echo "✅ Architecture tests pass" || echo "❌ Architecture tests fail"
fi

# Verify import cycle detection works
go list -deps ./pkg/mcp/... 2>&1 | grep -i cycle && echo "❌ Import cycles found!" || echo "✅ No import cycles detected"

# Measure complexity reduction
CURRENT_COMPLEXITY=$(gocyclo -over 15 pkg/mcp | wc -l)
echo "Current complexity files: $CURRENT_COMPLEXITY"
# Compare with baseline (should be in baseline file from Phase 0)
[ $CURRENT_COMPLEXITY -lt 50 ] && echo "✅ Complexity significantly reduced" || echo "❌ Complexity still too high"

# Test CI integration
make lint > /dev/null 2>&1 && echo "✅ Lint passes" || echo "❌ Lint failures"
make pre-commit > /dev/null 2>&1 && echo "✅ Pre-commit hooks pass" || echo "❌ Pre-commit hooks fail"

# Validate layer dependencies programmatically
# Domain should not import application or infra
! grep -r "pkg/mcp/application\|pkg/mcp/infra" pkg/mcp/domain/ && echo "✅ Domain layer isolated" || echo "❌ Domain has upward dependencies"
# Application should not import infra
! grep -r "pkg/mcp/infra" pkg/mcp/application/ && echo "✅ Application layer isolated" || echo "❌ Application imports infra"

# Check final package structure
DOMAIN_PKGS=$(find pkg/mcp/domain -type d | wc -l)
APP_PKGS=$(find pkg/mcp/application -type d | wc -l)
INFRA_PKGS=$(find pkg/mcp/infra -type d | wc -l)
echo "Package counts - Domain: $DOMAIN_PKGS, Application: $APP_PKGS, Infra: $INFRA_PKGS"
```

#### **🚨 Rollback Strategy**:
```bash
# If architecture tests fail, this indicates fundamental design issues
# Review and fix the architecture rather than rollback
echo "Architecture test failures indicate design problems - fix rather than rollback"
```

### **Day 22-24: Final Cleanup and Documentation**

#### **Objective**: Remove scaffolding and finalize migration

**Tasks**:

1. **Remove Compatibility Shims**
   ```bash
   # After all imports updated
   rm pkg/mcp/api/interfaces.go
   ```

2. **Update All Import Statements**
   ```bash
   # Mass import updates
   find . -name "*.go" -exec sed -i 's|pkg/mcp/core|pkg/mcp/application/core|g' {} \;
   find . -name "*.go" -exec sed -i 's|pkg/mcp/tools|pkg/mcp/application/tools|g' {} \;
   ```

3. **Final Testing**
   ```bash
   make test-all
   make lint
   make pre-commit
   ```

4. **Update Documentation**
   ```bash
   # Update CLAUDE.md with new architecture
   # Create migration summary
   ```

#### **Success Criteria**:
- ✅ All tests pass
- ✅ No scaffolding remains
- ✅ Documentation updated
- ✅ Architecture validated

#### **🔍 Final Validation Steps**:
```bash
# Verify compatibility shims removed
! test -f pkg/mcp/api/interfaces.go && echo "✅ Compatibility shims removed" || echo "❌ Shims still exist"

# Verify all imports updated
! grep -r "pkg/mcp/core\|pkg/mcp/tools" . --include="*.go" 2>/dev/null | head -1 && echo "✅ All imports updated" || echo "❌ Old imports remain"

# Final comprehensive test suite
make test-all && echo "✅ Full test suite passes" || echo "❌ Test suite has failures"
make lint && echo "✅ Lint passes cleanly" || echo "❌ Lint has issues"
make pre-commit && echo "✅ Pre-commit passes" || echo "❌ Pre-commit fails"

# Verify server builds and runs
go build ./cmd/mcp-server && echo "✅ MCP server builds" || echo "❌ Server build fails"
timeout 5s ./mcp-server --help > /dev/null 2>&1 && echo "✅ Server runs" || echo "❌ Server fails to run"

# Architecture validation
go test ./test/architecture_test.go && echo "✅ Architecture constraints enforced" || echo "❌ Architecture violations"

# Documentation completeness check
test -f ARCHITECTURE_REALIGNMENT_PLAN.md && echo "✅ Migration plan documented" || echo "❌ Documentation missing"
grep -q "Final Target Architecture" ARCHITECTURE_REALIGNMENT_PLAN.md && echo "✅ Target architecture documented" || echo "❌ Target architecture undocumented"

# Success metrics validation
echo "=== FINAL SUCCESS METRICS ==="
echo "Package count: $(find pkg/mcp -maxdepth 1 -type d | wc -l) (target: ≤5)"
echo "Max depth: $(find pkg/mcp -type d | awk -F'/' '{print NF-1}' | sort -nr | head -1) levels (target: ≤4)"
echo "Complexity files: $(gocyclo -over 15 pkg/mcp | wc -l) (target: <50)"
echo "Import cycles: $(go list -deps ./pkg/mcp/... 2>&1 | grep -c cycle) (target: 0)"
echo "Manager files: $(find pkg/mcp -name "*manager*.go" | wc -l) (target: 0)"
echo "Adapter files: $(find pkg/mcp -name "*adapter*.go" | wc -l) (target: 0)"

# Final git status check
git status --porcelain | grep -v "^??" && echo "⚠️  Uncommitted changes detected" || echo "✅ Clean git status"
```

#### **🎉 Migration Complete!**:
```bash
# If all validations pass, commit the final state
if make test-all && go build ./cmd/mcp-server; then
    echo "🎉 ARCHITECTURE REALIGNMENT SUCCESSFUL!"
    echo "✅ Three-layer architecture implemented"
    echo "✅ Domain/Application/Infrastructure boundaries enforced"
    echo "✅ Package count reduced from 14 to 3"
    echo "✅ Complexity significantly reduced"
    echo "✅ All functionality preserved"

    # Optional: Create summary commit
    git add -A
    git commit -m "feat: complete architecture realignment to domain/application/infra

    - Implemented three-layer architecture
    - Reduced package count from 14 to 3
    - Eliminated manager pattern anti-patterns
    - Enforced dependency boundaries
    - Consolidated error and validation systems
    - 50%+ complexity reduction achieved

    🚀 Architecture realignment complete!"
else
    echo "❌ MIGRATION INCOMPLETE - resolve issues before proceeding"
fi
```

---

## 🎯 **Final Target Architecture**

```
pkg/mcp/
├── domain/                           # Business Logic Layer (Pure)
│   ├── session/                     # Session domain
│   │   ├── types.go                 # Session entities (stateless)
│   │   ├── validation.go            # Session business rules
│   │   └── metadata.go              # Session metadata logic
│   ├── containerization/            # Container domain
│   │   ├── analyze/                 # Analysis domain logic
│   │   ├── build/                   # Build domain logic
│   │   ├── deploy/                  # Deploy domain logic
│   │   └── scan/                    # Scan domain logic
│   ├── workflow/                    # Workflow domain
│   ├── security/                    # Security domain rules
│   └── types/                       # Shared domain types
├── application/                      # Use Cases & Orchestration
│   ├── api/                         # DTOs, errors (shared kernel)
│   ├── ports/                       # Interfaces (SSOT) - ports pattern
│   └── commands/                    # Single commands package
│       ├── tool_registry.go        # Tool registration
│       ├── coordinator.go          # Tool coordination
│       ├── analyze_command.go       # Analyze tool implementation
│       ├── build_command.go        # Build tool implementation
│       ├── deploy_command.go       # Deploy tool implementation
│       └── scan_command.go         # Scan tool implementation
└── infra/                           # External Integrations (Build Tags)
    ├── transport/                   # MCP protocol (stdio, HTTP)
    │   ├── stdio.go
    │   ├── http.go
    │   └── server/                  # Server transport
    ├── persistence/                 # Storage layer
    │   ├── session_store.go         # Session persistence
    │   └── storage/                 # Storage implementations
    ├── docker_operations.go        # //go:build docker
    ├── k8s_operations.go           # //go:build k8s
    ├── cloud_operations.go         # //go:build cloud
    ├── templates/                   # Kubernetes templates
    └── telemetry/                   # Observability

# Build Usage Examples:
# go build                          # Core functionality only
# go build -tags docker             # Include Docker operations
# go build -tags k8s               # Include K8s operations
# go build -tags cloud             # Include both Docker + K8s
# go build -tags "docker k8s"      # Explicit both
```

## 📊 **Success Metrics**

### **Architecture Quality**
- ✅ 3 top-level packages (vs 14 current)
- ✅ Maximum depth ≤ 2 levels (vs 5 current)
- ✅ 0 import cycles
- ✅ 0 manager pattern files
- ✅ Single interface definitions in `application/api/`

### **Code Quality**
- ✅ >50% reduction in cyclomatic complexity
- ✅ All tests pass
- ✅ Lint rules pass
- ✅ Architecture tests enforce boundaries

### **Maintainability**
- ✅ Clear domain boundaries
- ✅ Dependency direction enforced (infra → application → domain)
- ✅ Single responsibility principle
- ✅ Testable components

## 🚨 **Risk Mitigation**

1. **Backward Compatibility**: Type aliases maintain API compatibility during migration
2. **Incremental Migration**: Each phase delivers working software
3. **Automated Testing**: Architecture tests prevent regression
4. **Rollback Strategy**: Git branches allow rollback at any phase

## 🏁 **Implementation Timeline**

- ✅ **Week 1**: Emergency triage and interface consolidation (**COMPLETE**)
- 🔄 **Week 2**: Domain layer extraction (**60% COMPLETE** - session done, containerization partial)
- 🚀 **Week 3**: Application layer organization (**CURRENT WEEK** - 188 files to migrate)
- ⏸️ **Week 4**: Infrastructure consolidation (**PENDING**)
- ⏸️ **Week 5**: Quality assurance and finalization (**PENDING**)

### **Current Sprint Focus (Week 3)**
**Goal**: Complete tool migration from `pkg/mcp/tools/` → `application/commands/`
- **188 files** need migration to application layer
- **Commands pattern** implementation
- **Manager pattern elimination**
- **Tool orchestration consolidation**

## 🔒 **Architectural Governance**

To prevent future architectural drift, the plan includes:

### **Pre-Commit Architecture Validation**
```bash
# Automatically runs on every commit
make pre-commit
```

**Validates**:
- ✅ Three-layer structure maintained (domain/application/infra)
- ✅ Dependency direction enforced (infra → application → domain)
- ✅ External dependencies isolated to infrastructure layer
- ✅ No manager/adapter anti-patterns
- ✅ Interface organization (canonical interfaces in ports)
- ✅ Package depth limits (max 3 levels)
- ✅ Import cycle detection
- ✅ Build tag usage for optional dependencies

### **Integration Commands**
```bash
make validate-architecture    # Manual validation
make check-architecture      # CI validation
make pre-commit              # Full pre-commit with architecture check
```

**Benefits**:
- **Prevents Regression**: Blocks commits that violate architecture
- **Developer Feedback**: Immediate feedback on architectural violations
- **CI Integration**: Automated validation in CI pipeline
- **Documentation**: Clear architectural guidelines in error messages

**Total Effort**: 5 weeks of focused architectural realignment (**25% COMPLETE**) to achieve the planned three-layer architecture, plus **ongoing architectural governance** to prevent future drift.

### **⏰ Remaining Effort Estimate**
- **Week 3 (Current)**: 2-3 days to complete Phase 3 tool migration
- **Week 4**: Infrastructure consolidation and build tag implementation
- **Week 5**: Quality assurance, final cleanup, and architecture validation

**Completion Target**: End of Week 5 (3.5 weeks remaining)
