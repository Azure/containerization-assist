# 🧹 Legacy Code Cleanup Plan

## 📊 Executive Summary

**Status**: Three-layer architecture foundation is **complete** (26%), but **74% of legacy code** remains unresolved.

**Critical Issue**: Build failures due to incomplete migrations blocking development.

**Goal**: Complete legacy cleanup to achieve 100% three-layer architecture compliance.

---

## 🚨 **Current State Assessment**

### **✅ Achievements (26% Complete)**
- **Domain Layer**: ✅ Complete (21 files, 4,675 lines)
- **Application Layer**: ✅ Complete (69 files, 23,423 lines)  
- **Infrastructure Layer**: ✅ Complete (25 files, 8,397 lines)
- **Total New Architecture**: 115 files, 36,495 lines

### **❌ Critical Issues (74% Remaining)**
- **Legacy Packages**: 399 files, 101,969 lines
- **Build Status**: ❌ **BROKEN** - Multiple import/type failures
- **Test Status**: ❌ **FAILING** - Undefined types and imports
- **Architecture Violations**: 6+ legacy packages still exist

---

## 🎯 **Legacy Cleanup Strategy**

This plan focuses on **systematic elimination** of legacy packages while maintaining functionality.

### **Three-Phase Approach**:
1. **Phase A**: **Fix Build Issues** (1-2 days) - Restore compilation
2. **Phase B**: **Core Legacy Migration** (1 week) - Major package cleanup  
3. **Phase C**: **Final Consolidation** (2-3 days) - Complete legacy removal

---

## 🚀 **PHASE A: Emergency Build Restoration (Days 1-2)**

**Objective**: Fix critical build failures to restore development capability

### **Critical Build Errors**:
```
pkg/mcp/core/clients.go:28:11: undefined: AnalysisService
pkg/mcp/api/validation_test.go:17:36: undefined: ValidationResult  
pkg/mcp/core/conversation/handler.go:10:2: no required module provides package github.com/Azure/container-kit/pkg/mcp/services
```

### **Day 1: Import Path Fixes**

**Tasks**:

1. **Update Service Import Paths**
   ```bash
   # Fix 23+ files importing old service paths
   find pkg/mcp -name "*.go" -exec sed -i 's|github.com/Azure/container-kit/pkg/mcp/services|github.com/Azure/container-kit/pkg/mcp/application/services|g' {} \;
   
   # Update imports to use new application layer
   find pkg/mcp -name "*.go" -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp/core|"github.com/Azure/container-kit/pkg/mcp/application/core|g' {} \;
   ```

2. **Fix Missing Type Definitions**
   ```go
   // Add to pkg/mcp/application/services/interfaces.go
   type AnalysisService interface {
       AnalyzeRepository(ctx context.Context, path string) (*RepositoryAnalysis, error)
       AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error)
   }
   
   type SessionState interface {
       GetID() string
       GetWorkspace() string
       UpdateMetadata(metadata map[string]interface{}) error
   }
   ```

3. **Fix Validation System**
   ```go
   // Add to pkg/mcp/application/api/types.go
   type ValidationResult struct {
       Valid    bool                    `json:"valid"`
       Errors   []ValidationError      `json:"errors,omitempty"`
       Warnings []ValidationWarning    `json:"warnings,omitempty"`
   }
   
   type ValidationError struct {
       Field   string `json:"field"`
       Message string `json:"message"`
       Code    string `json:"code"`
   }
   
   type ValidationWarning struct {
       Field   string `json:"field"`
       Message string `json:"message"`
   }
   ```

### **Day 2: Build Verification & Test Fixes**

**Tasks**:

1. **Verify Build Success**
   ```bash
   go build ./cmd/mcp-server && echo "✅ Build fixed" || echo "❌ Build still broken"
   ```

2. **Fix Test Dependencies**
   ```bash
   # Update test imports
   find . -name "*_test.go" -exec sed -i 's|pkg/mcp/core|pkg/mcp/application/core|g' {} \;
   find . -name "*_test.go" -exec sed -i 's|pkg/mcp/services|pkg/mcp/application/services|g' {} \;
   ```

3. **Validation Tests**
   ```bash
   make test && echo "✅ Tests pass" || echo "❌ Tests still failing"
   ```

#### **Success Criteria Phase A**:
- ✅ `go build ./cmd/mcp-server` succeeds
- ✅ `make test` passes
- ✅ No undefined type errors
- ✅ No import path errors

---

## 🏗️ **PHASE B: Core Legacy Migration (Week 1)**

**Objective**: Systematically migrate remaining legacy packages to three-layer architecture

### **Legacy Package Priority Matrix**:

| **Package** | **Files** | **Priority** | **Target Layer** | **Effort** |
|-------------|-----------|--------------|------------------|------------|
| `pkg/mcp/tools/` | 160+ | **CRITICAL** | `application/commands/` | 3 days |
| `pkg/mcp/core/` | 50+ | **HIGH** | Split across layers | 2 days |
| `pkg/mcp/session/` | 20+ | **HIGH** | `domain/session/` | 1 day |
| `pkg/mcp/internal/` | 100+ | **MEDIUM** | Distribute | 2 days |
| `pkg/mcp/security/` | 15+ | **MEDIUM** | `domain/security/` | 1 day |

### **Day 3-5: Tool Package Migration**

**Priority**: **CRITICAL** - 160+ files in `pkg/mcp/tools/`

**Strategy**: **Consolidation Migration** - Merge tool implementations into application commands

**Tasks**:

1. **Analyze Tools → Commands Migration**
   ```bash
   # Map current tool structure
   find pkg/mcp/tools -name "*.go" | head -20
   
   # Target: pkg/mcp/application/commands/
   # Strategy: Consolidate by domain (analyze, build, deploy, scan)
   ```

2. **Migrate Tool Categories**
   ```bash
   # Analyze tools
   mv pkg/mcp/tools/analyze/* pkg/mcp/application/commands/analyze/
   
   # Build tools  
   mv pkg/mcp/tools/build/* pkg/mcp/application/commands/build/
   
   # Deploy tools
   mv pkg/mcp/tools/deploy/* pkg/mcp/application/commands/deploy/
   
   # Scan tools
   mv pkg/mcp/tools/scan/* pkg/mcp/application/commands/scan/
   ```

3. **Update Tool Registrations**
   ```go
   // Update pkg/mcp/application/commands/tool_registry.go
   func RegisterAllTools() {
       RegisterTool("analyze", NewAnalyzeCommand())
       RegisterTool("build", NewBuildCommand())
       RegisterTool("deploy", NewDeployCommand())
       RegisterTool("scan", NewScanCommand())
   }
   ```

4. **Remove Legacy Tools Directory**
   ```bash
   # After migration complete
   rm -rf pkg/mcp/tools/
   ```

### **Day 6-7: Core Package Cleanup**

**Priority**: **HIGH** - 50+ files in `pkg/mcp/core/`

**Strategy**: **Layer Distribution** - Split core functionality across appropriate layers

**Analysis**:
```
pkg/mcp/core/
├── Business Logic → pkg/mcp/domain/
├── Application Logic → pkg/mcp/application/core/
├── Infrastructure → pkg/mcp/infra/
└── Delete Duplicates
```

**Tasks**:

1. **Analyze Core Package Contents**
   ```bash
   # Identify business logic
   find pkg/mcp/core -name "*.go" -exec grep -l "business\|entity\|rule" {} \;
   
   # Identify application logic  
   find pkg/mcp/core -name "*.go" -exec grep -l "orchestrat\|coordinat\|command" {} \;
   
   # Identify infrastructure
   find pkg/mcp/core -name "*.go" -exec grep -l "transport\|persist\|external" {} \;
   ```

2. **Migrate Core Files by Category**
   ```bash
   # Business logic → Domain
   mv pkg/mcp/core/session_types.go pkg/mcp/domain/session/types.go
   mv pkg/mcp/core/analysis_types.go pkg/mcp/domain/containerization/types.go
   
   # Application logic → Application (already done)
   # Infrastructure → Infrastructure  
   mv pkg/mcp/core/transport.go pkg/mcp/infra/transport/core.go
   ```

3. **Remove Duplicate Interfaces**
   ```bash
   # Find duplicated interfaces
   grep -r "interface.*{" pkg/mcp/core/ pkg/mcp/application/
   
   # Remove duplicates, keep canonical in application/api/
   ```

4. **Update Dependencies**
   ```bash
   # Update imports pointing to old core locations
   find pkg/mcp -name "*.go" -exec sed -i 's|pkg/mcp/core/session_types|pkg/mcp/domain/session|g' {} \;
   ```

### **Day 8: Session Package Consolidation**

**Priority**: **HIGH** - 20+ files in `pkg/mcp/session/`

**Strategy**: **Domain Consolidation** - Move session business logic to domain layer

**Tasks**:

1. **Migrate Session Business Logic**
   ```bash
   # Session entities → Domain
   mv pkg/mcp/session/session_core.go pkg/mcp/domain/session/core.go
   mv pkg/mcp/session/session_labels.go pkg/mcp/domain/session/labels.go
   mv pkg/mcp/session/session_queries.go pkg/mcp/domain/session/queries.go
   ```

2. **Session Infrastructure → Infrastructure**
   ```bash
   # Session persistence → Infrastructure (already done)
   # Session managers → Application services
   mv pkg/mcp/session/session_service.go pkg/mcp/application/services/session_service.go
   ```

3. **Remove Legacy Session Package**
   ```bash
   # After migration complete
   rm -rf pkg/mcp/session/
   ```

#### **Success Criteria Phase B**:
- ✅ `pkg/mcp/tools/` directory removed
- ✅ `pkg/mcp/core/` content properly distributed
- ✅ `pkg/mcp/session/` migrated to domain layer
- ✅ All tool functionality preserved
- ✅ No duplicate interfaces

---

## 🧹 **PHASE C: Final Consolidation (Days 9-10)**

**Objective**: Complete legacy cleanup and achieve 100% three-layer architecture

### **Day 9: Internal Package Reorganization**

**Priority**: **MEDIUM** - 100+ files in `pkg/mcp/internal/`

**Strategy**: **Distribute & Eliminate** - Move utilities to appropriate layers

**Tasks**:

1. **Categorize Internal Utilities**
   ```bash
   # Domain utilities
   find pkg/mcp/internal -name "*.go" -exec grep -l "entity\|business\|rule" {} \;
   
   # Application utilities  
   find pkg/mcp/internal -name "*.go" -exec grep -l "command\|orchestrat\|workflow" {} \;
   
   # Infrastructure utilities
   find pkg/mcp/internal -name "*.go" -exec grep -l "transport\|persist\|external" {} \;
   ```

2. **Migrate Internal Utilities**
   ```bash
   # Common utilities → Application
   mv pkg/mcp/internal/common/ pkg/mcp/application/internal/common/
   
   # Runtime utilities → Application  
   mv pkg/mcp/internal/runtime/ pkg/mcp/application/internal/runtime/
   
   # Infrastructure utilities → Infrastructure
   mv pkg/mcp/internal/transport/ pkg/mcp/infra/internal/transport/
   ```

3. **Remove Empty Internal Package**
   ```bash
   # After migration complete
   rm -rf pkg/mcp/internal/
   ```

### **Day 10: Security & Error System Consolidation**

**Priority**: **MEDIUM** - Security and error packages

**Tasks**:

1. **Migrate Security Validation**
   ```bash
   # Security business rules → Domain
   mv pkg/mcp/security/validation/ pkg/mcp/domain/security/validation/
   mv pkg/mcp/security/tags.go pkg/mcp/domain/security/tags.go
   
   # Security infrastructure → Infrastructure
   mv pkg/mcp/security/scanner/ pkg/mcp/infra/security/scanner/
   ```

2. **Consolidate Error System**
   ```bash
   # Error system is cross-cutting - keep at root level
   # But ensure single system usage
   find pkg/mcp -name "*.go" -exec grep -l "fmt\.Errorf" {} \; | head -5
   ```

3. **Final Architecture Validation**
   ```bash
   make validate-architecture
   ```

#### **Success Criteria Phase C**:
- ✅ `pkg/mcp/internal/` directory removed
- ✅ Security validation properly layered
- ✅ Error system consolidated
- ✅ Architecture validation passes
- ✅ No legacy packages remain

---

## 📊 **Final Success Metrics**

### **Architecture Quality Targets**:
- ✅ **Package Count**: ≤5 top-level packages (vs 14+ current)
- ✅ **Package Depth**: ≤3 levels (vs 5+ current)  
- ✅ **Legacy Files**: 0 files in legacy packages
- ✅ **Manager Pattern**: 0 manager files
- ✅ **Import Cycles**: 0 cycles
- ✅ **Build Success**: 100% clean builds
- ✅ **Test Success**: 100% test passage

### **Code Distribution Target**:
- **Domain Layer**: 30-40 files (~8,000 lines)
- **Application Layer**: 80-100 files (~30,000 lines)
- **Infrastructure Layer**: 40-50 files (~12,000 lines)
- **Cross-cutting (errors)**: 10-15 files (~2,000 lines)
- **Total Clean Architecture**: 160-205 files (~52,000 lines)

---

## ⏰ **Timeline & Resource Allocation**

### **Phase A** (Days 1-2): **Critical Priority**
- **Effort**: 1-2 developer days
- **Blocker**: Must complete before other work
- **Outcome**: Restore build & test capability

### **Phase B** (Days 3-8): **Major Migration**  
- **Effort**: 4-5 developer days
- **Focus**: Tool migration (60% of effort)
- **Outcome**: Legacy packages eliminated

### **Phase C** (Days 9-10): **Final Polish**
- **Effort**: 1-2 developer days  
- **Focus**: Cleanup and validation
- **Outcome**: 100% architecture compliance

### **Total Effort**: 7-9 developer days (~2 weeks)

---

## 🚨 **Risk Mitigation**

### **High Risk Items**:
1. **Tool Migration Complexity**: 160+ files with complex dependencies
2. **Import Cycles**: Potential cycles during migration
3. **Test Breakage**: Extensive test updates required

### **Mitigation Strategies**:
1. **Incremental Migration**: One package at a time with validation
2. **Automated Testing**: Run tests after each package migration
3. **Rollback Planning**: Git branches for each phase
4. **Import Monitoring**: Check for cycles after each migration

### **Success Validation**:
```bash
# After each phase
make validate-architecture
make test-all
go build ./cmd/mcp-server
```

---

## 🎯 **Implementation Guidelines**

### **Team Coordination**:
1. **Phase A**: Single developer (critical path)
2. **Phase B**: Can parallelize by package
3. **Phase C**: Single developer (integration)

### **Quality Gates**:
- **No commits** without passing `make validate-architecture`
- **No commits** without passing `make test-all`  
- **No commits** without successful `go build ./cmd/mcp-server`

### **Documentation Updates**:
- Update CLAUDE.md with new architecture
- Update README with legacy cleanup completion
- Create migration summary document

This plan will achieve **100% three-layer architecture compliance** and eliminate all legacy technical debt within 2 weeks.

---

## 🏆 **Completion Criteria**

**Legacy Cleanup is COMPLETE when**:
- ✅ All legacy packages removed (`tools/`, `core/`, `internal/`, `session/`)
- ✅ Architecture validation passes with 0 violations
- ✅ Build succeeds with no errors or warnings
- ✅ All tests pass with 100% success rate  
- ✅ No manager pattern files remain
- ✅ Package depth ≤3 levels
- ✅ Import cycles = 0

**Result**: Clean, maintainable three-layer architecture ready for production deployment.