# WORKSTREAM ALPHA: Foundation & Three-Layer Architecture Implementation

## 🚨 CRITICAL ISSUE: Architecture Violation Must Be Fixed

### **URGENT: Current Package Structure Violates ADR-001**

**Current State**: 26 top-level directories in `pkg/mcp/` - **COMPLETELY VIOLATES** [ADR-001 Three-Context Architecture](docs/architecture/adr/2025-07-07-three-context-architecture.md)

```bash
# WRONG: Current structure (26 directories)
pkg/mcp/analyze      pkg/mcp/errorcodes   pkg/mcp/scan
pkg/mcp/api          pkg/mcp/errors       pkg/mcp/security  
pkg/mcp/application  pkg/mcp/infra        pkg/mcp/services
pkg/mcp/appstate     pkg/mcp/knowledge    pkg/mcp/session
pkg/mcp/build        pkg/mcp/logging      pkg/mcp/shared
pkg/mcp/commands     pkg/mcp/retry        pkg/mcp/tools
pkg/mcp/config       pkg/mcp/scan         pkg/mcp/workflows
pkg/mcp/core         pkg/mcp/security     pkg/mcp/domain
# ... 26 total directories violating three-layer architecture
```

**Required State**: **EXACTLY 3 directories** per ADR-001 and [Three-Layer Architecture](docs/THREE_LAYER_ARCHITECTURE.md):

```bash
# CORRECT: Required structure (3 directories only)
pkg/mcp/domain/      # Business logic (no dependencies)
pkg/mcp/application/ # Orchestration (depends on domain only)  
pkg/mcp/infra/       # External integrations (depends on domain + application)
```

## 🎯 Mission

**IMMEDIATELY implement ADR-001 Three-Context Architecture** by consolidating 26 scattered directories into the required 3-layer structure, while standardizing logging, implementing context propagation, and enforcing clean architecture boundaries.

## 📋 Context
- **Project**: Container Kit Architecture Refactoring
- **Your Role**: Foundation architect - you MUST implement the existing ADR-001 three-layer architecture
- **Timeline**: Week 1-3 (21 days) 
- **Dependencies**: None (you are the foundation)
- **Deliverables**: ADR-001 compliant three-layer architecture needed by ALL other workstreams

## 🎯 Success Metrics
- **Package structure**: **EXACTLY 3 top-level directories** per ADR-001 (domain, application, infra)
- **Package depth**: ≤3 levels maximum within each layer
- **Circular dependencies**: 0 import cycles between layers
- **Context propagation**: 100% functions accept context.Context
- **Logging consistency**: Single slog framework throughout
- **Architecture boundaries**: 0 violations of three-layer rules

## 📁 File Ownership
You have exclusive ownership of ALL package restructuring:
```
pkg/mcp/ (complete restructuring to three-layer architecture)
scripts/check_import_depth.sh
scripts/check-context-params.sh  
scripts/check-cycles.sh
scripts/validate-architecture.sh
tools/check-boundaries/
All package restructuring and import path changes
```

## 🚨 IMMEDIATE ACTION REQUIRED: Fix Architecture Violation

### **Phase 0: Emergency Architecture Fix (Days 1-5)**

#### Day 1: Architecture Violation Assessment
**Morning Goals**:
- [ ] **CRITICAL**: Acknowledge that current 26-directory structure violates ADR-001
- [ ] Read and understand [ADR-001](docs/architecture/adr/2025-07-07-three-context-architecture.md)
- [ ] Read and understand [Three-Layer Architecture](docs/THREE_LAYER_ARCHITECTURE.md)
- [ ] Create migration plan to move 26 directories into 3 layers

**Assessment Commands**:
```bash
# Document the violation
echo "=== ARCHITECTURE VIOLATION ASSESSMENT ===" > architecture_violation.txt
echo "Current directories (VIOLATES ADR-001):" >> architecture_violation.txt
find pkg/mcp -maxdepth 1 -type d | sort >> architecture_violation.txt
echo "Count: $(find pkg/mcp -maxdepth 1 -type d | wc -l) directories" >> architecture_violation.txt
echo "Required: EXACTLY 3 directories per ADR-001" >> architecture_violation.txt

# Read the requirements
echo "=== READING ADR-001 REQUIREMENTS ===" >> architecture_violation.txt
cat docs/architecture/adr/2025-07-07-three-context-architecture.md >> architecture_violation.txt

# Create migration mapping
echo "=== MIGRATION MAPPING ===" > migration_plan.txt
echo "Domain Layer (pkg/mcp/domain/):" >> migration_plan.txt
echo "- config/ (from pkg/mcp/config/)" >> migration_plan.txt
echo "- containerization/ (from pkg/mcp/analyze/, pkg/mcp/build/, pkg/mcp/deploy/, pkg/mcp/scan/)" >> migration_plan.txt
echo "- errors/ (from pkg/mcp/errors/, pkg/mcp/errorcodes/)" >> migration_plan.txt
echo "- security/ (from pkg/mcp/security/)" >> migration_plan.txt
echo "- session/ (from pkg/mcp/session/)" >> migration_plan.txt
echo "- types/ (from pkg/mcp/domaintypes/, pkg/mcp/shared/)" >> migration_plan.txt

echo "Application Layer (pkg/mcp/application/):" >> migration_plan.txt
echo "- api/ (from pkg/mcp/api/)" >> migration_plan.txt
echo "- commands/ (from pkg/mcp/commands/)" >> migration_plan.txt
echo "- core/ (from pkg/mcp/core/)" >> migration_plan.txt
echo "- services/ (from pkg/mcp/services/)" >> migration_plan.txt
echo "- tools/ (from pkg/mcp/tools/)" >> migration_plan.txt
echo "- workflows/ (from pkg/mcp/workflows/)" >> migration_plan.txt
echo "- state/ (from pkg/mcp/appstate/)" >> migration_plan.txt
echo "- knowledge/ (from pkg/mcp/knowledge/)" >> migration_plan.txt

echo "Infrastructure Layer (pkg/mcp/infra/):" >> migration_plan.txt
echo "- logging/ (from pkg/mcp/logging/)" >> migration_plan.txt
echo "- retry/ (from pkg/mcp/retry/)" >> migration_plan.txt
echo "- persistence/ (already exists)" >> migration_plan.txt
echo "- transport/ (already exists)" >> migration_plan.txt
```

**Validation Commands**:
```bash
# Verify documentation read
test -f architecture_violation.txt && test -f migration_plan.txt && echo "✅ Architecture violation documented"

# Pre-commit validation  
alias make='/usr/bin/make'
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] **CRITICAL**: ADR-001 requirements fully understood
- [ ] Current violation documented (26 directories → must be 3)
- [ ] Migration plan created for all directories
- [ ] Team acknowledges architecture fix is Day 1 priority

#### Day 2: Domain Layer Migration
**Morning Goals**:
- [ ] **CRITICAL**: Create proper `pkg/mcp/domain/` structure per ADR-001
- [ ] Migrate business logic from scattered directories into domain layer
- [ ] Ensure domain layer has NO external dependencies

**Domain Migration Commands**:
```bash
# Create domain layer structure
mkdir -p pkg/mcp/domain/{config,containerization,errors,security,session,types,workflow,internal}

# Migrate config domain logic
echo "Migrating config domain logic..."
mv pkg/mcp/config/* pkg/mcp/domain/config/ 2>/dev/null || echo "Config already in correct location"

# Migrate containerization domain logic  
echo "Migrating containerization domain logic..."
mkdir -p pkg/mcp/domain/containerization/{analyze,build,deploy,scan}
mv pkg/mcp/analyze/* pkg/mcp/domain/containerization/analyze/ 2>/dev/null || echo "Analyze logic moved"
mv pkg/mcp/build/* pkg/mcp/domain/containerization/build/ 2>/dev/null || echo "Build logic moved"
mv pkg/mcp/deploy/* pkg/mcp/domain/containerization/deploy/ 2>/dev/null || echo "Deploy logic moved"  
mv pkg/mcp/scan/* pkg/mcp/domain/containerization/scan/ 2>/dev/null || echo "Scan logic moved"

# Migrate error domain logic
echo "Migrating error domain logic..."
mv pkg/mcp/errors/* pkg/mcp/domain/errors/ 2>/dev/null || echo "Errors already in domain"
mv pkg/mcp/errorcodes/* pkg/mcp/domain/errors/ 2>/dev/null || echo "Error codes moved"

# Migrate security domain logic
echo "Migrating security domain logic..."
mv pkg/mcp/security/* pkg/mcp/domain/security/ 2>/dev/null || echo "Security logic moved"

# Migrate session domain logic
echo "Migrating session domain logic..."
mv pkg/mcp/session/* pkg/mcp/domain/session/ 2>/dev/null || echo "Session logic moved"

# Migrate types and shared logic
echo "Migrating types domain logic..."
mv pkg/mcp/domaintypes/* pkg/mcp/domain/types/ 2>/dev/null || echo "Domain types moved"
mv pkg/mcp/shared/* pkg/mcp/domain/internal/ 2>/dev/null || echo "Shared logic moved"

# Remove empty directories
rmdir pkg/mcp/config pkg/mcp/analyze pkg/mcp/build pkg/mcp/deploy pkg/mcp/scan pkg/mcp/errors pkg/mcp/errorcodes pkg/mcp/security pkg/mcp/session pkg/mcp/domaintypes pkg/mcp/shared 2>/dev/null || echo "Some directories still have content"
```

**Validation Commands**:
```bash
# Verify domain layer structure
find pkg/mcp/domain -type d | sort && echo "✅ Domain layer structure created"

# Verify compilation
go build ./pkg/mcp/domain/... && echo "✅ Domain layer compiles"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Domain layer structure created per ADR-001
- [ ] Business logic migrated from scattered directories
- [ ] Domain layer compiles successfully
- [ ] Empty directories removed

#### Day 3: Application Layer Migration  
**Morning Goals**:
- [ ] **CRITICAL**: Migrate orchestration logic to `pkg/mcp/application/` per ADR-001
- [ ] Ensure application layer depends ONLY on domain layer
- [ ] Consolidate command, service, and workflow logic

**Application Migration Commands**:
```bash
# Application layer already exists - consolidate scattered logic into it
echo "Consolidating application layer per ADR-001..."

# Migrate API logic (if not already in application)
mv pkg/mcp/api/* pkg/mcp/application/api/ 2>/dev/null || echo "API already in application"

# Migrate commands logic
mv pkg/mcp/commands/* pkg/mcp/application/commands/ 2>/dev/null || echo "Commands already in application"

# Migrate core logic
mv pkg/mcp/core/* pkg/mcp/application/core/ 2>/dev/null || echo "Core already in application"

# Migrate services logic
mv pkg/mcp/services/* pkg/mcp/application/services/ 2>/dev/null || echo "Services already in application"

# Migrate tools logic
mv pkg/mcp/tools/* pkg/mcp/application/tools/ 2>/dev/null || echo "Tools already in application"

# Migrate workflows logic
mv pkg/mcp/workflows/* pkg/mcp/application/workflows/ 2>/dev/null || echo "Workflows already in application"

# Migrate state management
mkdir -p pkg/mcp/application/state
mv pkg/mcp/appstate/* pkg/mcp/application/state/ 2>/dev/null || echo "State logic moved"

# Migrate knowledge management
mv pkg/mcp/knowledge/* pkg/mcp/application/knowledge/ 2>/dev/null || echo "Knowledge already in application"

# Remove empty directories
rmdir pkg/mcp/api pkg/mcp/commands pkg/mcp/core pkg/mcp/services pkg/mcp/tools pkg/mcp/workflows pkg/mcp/appstate pkg/mcp/knowledge 2>/dev/null || echo "Some directories still have content"
```

**Validation Commands**:
```bash
# Verify application layer structure
find pkg/mcp/application -type d | sort && echo "✅ Application layer structure complete"

# Verify compilation
go build ./pkg/mcp/application/... && echo "✅ Application layer compiles"

# Check dependencies (should only import domain)
grep -r "pkg/mcp/infra" pkg/mcp/application/ && echo "❌ Application imports infra (VIOLATION)" || echo "✅ Application dependencies clean"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Application layer consolidated per ADR-001
- [ ] All orchestration logic in application layer
- [ ] No infra dependencies in application layer
- [ ] Application layer compiles successfully

#### Day 4: Infrastructure Layer Migration
**Morning Goals**:
- [ ] **CRITICAL**: Migrate external integrations to `pkg/mcp/infra/` per ADR-001
- [ ] Ensure infrastructure layer handles only external concerns
- [ ] Complete three-layer architecture implementation

**Infrastructure Migration Commands**:
```bash
# Infra layer already exists - migrate remaining external concerns
echo "Completing infrastructure layer per ADR-001..."

# Migrate logging infrastructure  
mkdir -p pkg/mcp/infra/logging
mv pkg/mcp/logging/* pkg/mcp/infra/logging/ 2>/dev/null || echo "Logging infrastructure moved"

# Migrate retry infrastructure
mkdir -p pkg/mcp/infra/retry
mv pkg/mcp/retry/* pkg/mcp/infra/retry/ 2>/dev/null || echo "Retry infrastructure moved"

# Remove empty directories
rmdir pkg/mcp/logging pkg/mcp/retry 2>/dev/null || echo "Some directories still have content"

# Verify ONLY three directories remain
echo "=== FINAL ARCHITECTURE VALIDATION ===" > final_architecture.txt
echo "Top-level directories (MUST BE EXACTLY 3):" >> final_architecture.txt
find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | sort >> final_architecture.txt
echo "Count: $(find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | wc -l)" >> final_architecture.txt

# If more than 3 directories exist, ERROR
DIRECTORY_COUNT=$(find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | wc -l)
if [ $DIRECTORY_COUNT -ne 3 ]; then
    echo "❌ ARCHITECTURE VIOLATION: $DIRECTORY_COUNT directories found, ADR-001 requires EXACTLY 3"
    echo "Remaining directories that need migration:"
    find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | grep -v -E "(domain|application|infra)$"
    exit 1
else
    echo "✅ ADR-001 COMPLIANT: Exactly 3 directories (domain, application, infra)"
fi
```

**Validation Commands**:
```bash
# Verify three-layer architecture
test -d pkg/mcp/domain && test -d pkg/mcp/application && test -d pkg/mcp/infra && echo "✅ Three layers exist"

# Verify ONLY three directories  
DIRS=$(find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | wc -l)
test $DIRS -eq 3 && echo "✅ EXACTLY 3 directories (ADR-001 compliant)" || echo "❌ $DIRS directories found (ADR-001 violation)"

# Verify compilation
go build ./pkg/mcp/... && echo "✅ All layers compile"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] **CRITICAL**: EXACTLY 3 directories in pkg/mcp (domain, application, infra)
- [ ] ADR-001 three-context architecture implemented
- [ ] All code migrated to appropriate layers
- [ ] Full compilation successful

#### Day 5: Import Path Updates & Validation
**Morning Goals**:
- [ ] **CRITICAL**: Update ALL import paths to reflect new three-layer structure
- [ ] Fix all broken imports from architecture migration
- [ ] Validate three-layer architecture compliance

**Import Path Fix Commands**:
```bash
# Update imports throughout codebase
echo "Updating import paths for three-layer architecture..."

# Fix domain imports
find . -name "*.go" -exec sed -i 's|pkg/mcp/config|pkg/mcp/domain/config|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/analyze|pkg/mcp/domain/containerization/analyze|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/build|pkg/mcp/domain/containerization/build|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/deploy|pkg/mcp/domain/containerization/deploy|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/scan|pkg/mcp/domain/containerization/scan|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/security|pkg/mcp/domain/security|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/session|pkg/mcp/domain/session|g' {} \;

# Fix application imports
find . -name "*.go" -exec sed -i 's|pkg/mcp/api|pkg/mcp/application/api|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/commands|pkg/mcp/application/commands|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/core|pkg/mcp/application/core|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/services|pkg/mcp/application/services|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/tools|pkg/mcp/application/tools|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/workflows|pkg/mcp/application/workflows|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/knowledge|pkg/mcp/application/knowledge|g' {} \;

# Fix infrastructure imports
find . -name "*.go" -exec sed -i 's|pkg/mcp/logging|pkg/mcp/infra/logging|g' {} \;
find . -name "*.go" -exec sed -i 's|pkg/mcp/retry|pkg/mcp/infra/retry|g' {} \;

# Clean up go.mod
go mod tidy

# Verify compilation after import fixes
go build ./... && echo "✅ All import paths fixed" || echo "❌ Import path errors remain"
```

**Final Architecture Validation**:
```bash
# Validate ADR-001 compliance
echo "=== ADR-001 COMPLIANCE VALIDATION ==="

# 1. Verify exactly 3 directories
DIRS=$(find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | wc -l)
test $DIRS -eq 3 && echo "✅ Exactly 3 directories" || echo "❌ $DIRS directories (must be 3)"

# 2. Verify required directories exist
test -d pkg/mcp/domain && echo "✅ Domain layer exists" || echo "❌ Domain layer missing"
test -d pkg/mcp/application && echo "✅ Application layer exists" || echo "❌ Application layer missing"  
test -d pkg/mcp/infra && echo "✅ Infrastructure layer exists" || echo "❌ Infrastructure layer missing"

# 3. Verify no extra directories
EXTRA_DIRS=$(find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | grep -v -E "(domain|application|infra)$")
if [ -n "$EXTRA_DIRS" ]; then
    echo "❌ EXTRA DIRECTORIES FOUND (ADR-001 VIOLATION):"
    echo "$EXTRA_DIRS"
    exit 1
else
    echo "✅ No extra directories (ADR-001 compliant)"
fi

# 4. Verify package depth ≤3
find pkg/mcp -type d | awk -F/ 'NF>4{print NF-1, $0}' | wc -l | grep "^0$" && echo "✅ Package depth ≤3" || echo "❌ Package depth violations"

# 5. Test compilation
go build ./... && echo "✅ Full compilation successful" || echo "❌ Compilation errors"

# 6. Run architecture validation
scripts/validate-architecture.sh && echo "✅ Architecture boundaries clean" || echo "❌ Architecture violations"

echo "🚨 ADR-001 THREE-LAYER ARCHITECTURE IMPLEMENTATION COMPLETE"
```

**End of Day Checklist**:
- [ ] **CRITICAL**: All import paths updated for three-layer structure
- [ ] EXACTLY 3 directories in pkg/mcp (domain, application, infra)
- [ ] Full codebase compilation successful
- [ ] ADR-001 compliance verified
- [ ] Architecture validation passing

---

## 🗓️ Implementation Schedule (After Architecture Fixed)

### Week 1: Foundation Work (Days 6-10)

#### Day 6: Logging Standardization
**Morning Goals** (AFTER architecture is fixed):
- [ ] Audit remaining logging usage in three-layer structure
- [ ] Create slog adapter in `pkg/mcp/infra/logging/`
- [ ] Replace all zerolog/logrus with slog

**Logging Migration Commands**:
```bash
# Now that architecture is correct, fix logging
find pkg/mcp -name "*.go" | xargs grep -l "zerolog\\|logrus" | tee logging_audit.txt

# Create slog adapter in correct location
cat > pkg/mcp/infra/logging/logger.go << 'EOF'
package logging

import (
    "log/slog"
    "os"
)

// Logger provides unified logging interface
type Logger struct {
    *slog.Logger
}

// New creates a new logger
func New() *Logger {
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
    return &Logger{
        Logger: slog.New(handler),
    }
}
EOF

# Replace old logging throughout three-layer architecture
find pkg/mcp -name "*.go" -exec sed -i 's|zerolog|slog|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|logrus|slog|g' {} \;

# Verify no old logging remains
! grep -r "zerolog\\|logrus" pkg/mcp/ && echo "✅ Single logging backend achieved"
```

#### Day 7-10: Context Propagation & Quality Gates
Continue with context propagation, circular dependency removal, and quality validation - but now within the correct three-layer architecture.

### Week 2-3: Foundation Completion
Continue with remaining foundation work, but emphasize that the three-layer architecture is now correctly implemented per ADR-001.

## 🚨 Critical Requirements

### **ADR-001 Compliance Checkpoints**
Every day MUST include this validation:
```bash
# Daily ADR-001 compliance check
DIRS=$(find pkg/mcp -maxdepth 1 -type d | grep -v "^pkg/mcp$" | wc -l)
if [ $DIRS -ne 3 ]; then
    echo "❌ ADR-001 VIOLATION: $DIRS directories found, MUST BE EXACTLY 3"
    exit 1
fi
echo "✅ ADR-001 compliant: 3 directories (domain, application, infra)"
```

### **Architecture Boundaries**
- **Domain**: NO external dependencies
- **Application**: Depends ONLY on domain  
- **Infrastructure**: Depends on domain + application

### **Success Criteria**
- [ ] **EXACTLY 3 directories**: pkg/mcp/domain, pkg/mcp/application, pkg/mcp/infra
- [ ] **Package depth**: ≤3 levels within each layer
- [ ] **No violations**: Architecture validation passes
- [ ] **Full compilation**: All code compiles in new structure

## ❌ **What Went Wrong & How to Fix It**

### **Problem**: 26 directories created instead of 3-layer architecture
### **Root Cause**: ADR-001 was ignored during initial work
### **Solution**: IMMEDIATE migration to correct three-layer structure
### **Timeline**: 5 days to fix architecture violation + foundation work

**This is CRITICAL PATH work** - all other workstreams depend on proper three-layer architecture implementation per ADR-001.

---

**Remember**: You MUST implement ADR-001 three-context architecture. The current 26-directory structure completely violates the existing architectural decision and blocks all other workstreams. Fix this FIRST, then continue with foundation work within the correct structure.