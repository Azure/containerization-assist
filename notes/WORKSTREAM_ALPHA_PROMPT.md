# WORKSTREAM ALPHA: Validation Framework Migration
**AI Assistant Prompt - Container Kit MCP Cleanup**

## 🎯 MISSION OVERVIEW

You are the **Validation Migration Specialist** responsible for migrating 39+ scattered validation files to a unified validation framework. This is the **foundation workstream** that other workstreams depend on.

**Duration**: Week 1-2 (10 days)  
**Dependencies**: None (you are the foundation)  
**Critical Success**: Unified validation system enabling other workstreams

## 📋 YOUR SPECIFIC RESPONSIBILITIES

### Week 1 (Days 1-5): Foundation & Build Migration

#### Day 1-2: Complete Foundation Infrastructure
```bash
# PRIORITY 1: Complete remaining utility validators
cd pkg/mcp/validation/

# Create missing utility validators:
touch validators/format.go      # email, URL, JSON, YAML validation
touch validators/network.go     # IP, port, hostname validation  
touch validators/security.go    # secrets, permissions validation

# Create migration tooling:
touch utils/migration_tools.go  # Automated migration detection
touch utils/pattern_analysis.go # Validation pattern analysis

# VALIDATION REQUIRED:
go test ./pkg/mcp/validation/... && echo "✅ Foundation complete"
go fmt ./pkg/mcp/validation/...
```

#### Day 3-4: Build Package Migration (CRITICAL)
```bash
# MIGRATE these 4 files to unified framework:
# File 1: pkg/mcp/internal/build/security_validator.go
# - Replace with unified security validator
# - Update all security validation calls
# - Maintain backward compatibility

# File 2: pkg/mcp/internal/build/syntax_validator.go  
# - Replace with unified dockerfile validator
# - Update syntax validation throughout build package
# - Ensure all Dockerfile validation works

# File 3: pkg/mcp/internal/build/context_validator.go
# - Replace with unified context validator
# - Update build context validation
# - Maintain existing validation logic

# File 4: pkg/mcp/internal/build/image_validator.go
# - Replace with unified image validator
# - Consolidate with docker validator
# - Update image validation calls

# VALIDATION REQUIRED AFTER EACH FILE:
go test -short ./pkg/mcp/internal/build/... && echo "✅ Build file X migrated"
```

#### Day 5: Deploy Package Migration Start
```bash
# Begin deploy package migration:
# File 1: pkg/mcp/internal/deploy/health_validator.go
# - Create unified health validator in validators/health.go
# - Update health validation calls
# - Test deployment health checks

# File 2: pkg/mcp/internal/deploy/manifest_validator.go  
# - Create unified manifest validator in validators/kubernetes.go
# - Update manifest validation throughout deploy package
# - Ensure K8s validation works

# END OF WEEK 1 CHECKPOINT:
go test -short ./pkg/mcp/internal/build/... ./pkg/mcp/internal/deploy/...

# COMMIT AND PAUSE:
git add .
git commit -m "feat(validation): complete build package migration and begin deploy

- Completed foundation utility validators (format, network, security)
- Migrated 4 build validators to unified framework  
- Started deploy package migration (health, manifest)
- Maintained all existing validation functionality

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# PAUSE POINT: Wait for external merge before Week 2
```

### Week 2 (Days 6-10): Complete Migration & Cleanup

#### Day 6-7: Complete Deploy & Scan Migration
```bash
# WAIT: Until Week 1 changes are merged and branch updated

# Complete deploy migration:
# File 3: pkg/mcp/internal/deploy/deploy_kubernetes_validate.go
# - Update to use unified kubernetes validator
# - Test deployment validation workflows

# Scan package migration:
# File 1: pkg/mcp/internal/scan/validators.go
# - Replace with unified security validator
# - Update secret scanning validation
# - Update vulnerability validation

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/internal/deploy/... ./pkg/mcp/internal/scan/...
```

#### Day 8-9: Core System Integration (CRITICAL FOR OTHER WORKSTREAMS)
```bash
# Runtime system integration:
# File 1: pkg/mcp/internal/runtime/validator.go
# - Update to use unified validation framework
# - Replace old validation calls
# - Test tool validation integration

# File 2: pkg/mcp/internal/session/validation.go
# - Update session validation to unified system
# - Replace legacy validation calls
# - Test session state validation

# File 3: pkg/mcp/internal/state/validators.go
# - Update state validation to unified system
# - Replace old validation patterns
# - Test configuration validation

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/internal/runtime/... ./pkg/mcp/internal/session/... ./pkg/mcp/internal/state/...
```

#### Day 10: Legacy Cleanup & Final Validation
```bash
# Remove duplicate validation utilities:
find pkg/mcp -name "*validation*" -type f | grep -v "pkg/mcp/validation/" | head -20

# Remove these duplicate files (after confirming no usage):
# - pkg/mcp/types/validation.go (4 different ValidationResult types)
# - pkg/mcp/internal/errors/validation.go (legacy validation errors)
# - pkg/mcp/utils/typed_validation.go (type-safe validation)
# - pkg/mcp/utils/validation_utils.go (string/format utilities)
# - pkg/mcp/utils/path_utils.go (path validation implementations)

# Update import statements across codebase:
find pkg/mcp -name "*.go" -exec grep -l "ValidationResult" {} \; | head -10
# Replace with unified validation imports

# FINAL VALIDATION:
go test ./... && echo "✅ ALPHA WORKSTREAM COMPLETE"

# FINAL COMMIT:
git add .
git commit -m "feat(validation): complete unified validation migration

- Migrated all 39+ validation files to unified framework
- Consolidated 4+ ValidationResult types into 1
- Removed 30+ duplicate validation utilities  
- Updated all import statements across codebase
- Maintained 100% functionality with unified system

ALPHA WORKSTREAM COMPLETE ✅

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

## 🎯 SUCCESS CRITERIA

### Must Achieve (100% Required):
- ✅ **39+ validation files migrated** to unified framework
- ✅ **4+ ValidationResult types consolidated** into 1 type
- ✅ **6+ validator interfaces replaced** with unified system  
- ✅ **30+ duplicate validation files removed** 
- ✅ **All tests pass** throughout migration
- ✅ **Zero breaking changes** to existing functionality

### Quality Gates (Enforce Strictly):
```bash
# REQUIRED before each commit:
go test -short ./pkg/mcp/validation/...  # Foundation tests
go test -short ./pkg/mcp/internal/*/...  # Integration tests
go fmt ./pkg/mcp/...                     # Code formatting
go build ./pkg/mcp/...                   # Must compile

# PERFORMANCE check:
go test -bench=. ./pkg/mcp/validation/... | grep "ns/op"
# Validation performance must be within 5% of baseline
```

### Daily Validation Commands
```bash
# Morning startup:
go test -short ./pkg/mcp/... && echo "✅ Ready to work"

# Throughout day after each major change:
go test -short ./pkg/mcp/internal/build/...     # After build migration
go test -short ./pkg/mcp/internal/deploy/...    # After deploy migration  
go test -short ./pkg/mcp/internal/scan/...      # After scan migration
go test -short ./pkg/mcp/internal/runtime/...   # After runtime migration

# End of day:
go test ./... && echo "✅ All systems functional"
```

## 🚨 CRITICAL COORDINATION POINTS

### Dependencies on Your Work:
- **WORKSTREAM BETA** needs unified validation for RichError integration
- **WORKSTREAM GAMMA** needs stable validation for testing framework
- **WORKSTREAM DELTA** needs consolidated validation interfaces
- **WORKSTREAM EPSILON** needs typed validation for interface{} elimination

### Files You Own (Full Authority):
- `pkg/mcp/validation/` (entire package) - You are the authority
- All files with "validation" in the name - Migrate to your unified system
- All ValidationResult types - Consolidate to your single type

### Files to Coordinate On:
- `pkg/mcp/core/interfaces.go` - Work with WORKSTREAM DELTA on validation interfaces
- Import statements across codebase - Update systematically after migration

## 📊 PROGRESS TRACKING

### Daily Metrics to Track:
```bash
# Validation files migrated:
find pkg/mcp -path "*/validation/*" -name "*.go" | wc -l  # Should increase

# Legacy validation files remaining:
find pkg/mcp -name "*validation*" -not -path "*/validation/*" | wc -l  # Should decrease  

# ValidationResult types remaining:
rg "type.*ValidationResult" pkg/mcp/ | wc -l  # Should become 1

# Import statements updated:
rg 'import.*validation' pkg/mcp/ | grep -v "pkg/mcp/validation" | wc -l  # Should become 0
```

### Daily Summary Format:
```
WORKSTREAM ALPHA - DAY X SUMMARY
================================
Progress: X% complete (X/39 files migrated)
Validation files migrated: X
Legacy files removed: X  
ValidationResult types: X (target: 1)

Files modified today:
- pkg/mcp/internal/build/security_validator.go (migrated)
- pkg/mcp/internal/deploy/health_validator.go (migrated)
- [other files]

Issues encountered:
- [any blockers or concerns]

Coordination needed:
- [shared file concerns]

Tomorrow's focus:
- [next priorities]

Quality status: All tests passing ✅
```

## 🛡️ ERROR HANDLING & ROLLBACK

### If Things Go Wrong:
1. **Compilation fails**: Revert last change, fix imports
2. **Tests fail**: Check validation logic compatibility  
3. **Performance regression**: Review validation efficiency
4. **Breaking changes**: Add compatibility layer

### Rollback Procedure:
```bash
# Emergency rollback:
git checkout HEAD~1 -- pkg/mcp/validation/
git checkout HEAD~1 -- pkg/mcp/internal/*/

# Selective rollback:
git checkout HEAD~1 -- <specific-problematic-file>
```

## 🎯 KEY IMPLEMENTATION PATTERNS

### Unified Validator Creation:
```go
// Example: Creating a unified security validator
package validators

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/validation/core"
)

type SecurityValidator struct {
    *BaseValidatorImpl
    secretValidator     *SecretValidator
    complianceValidator *ComplianceValidator
}

func NewSecurityValidator() *SecurityValidator {
    return &SecurityValidator{
        BaseValidatorImpl: NewBaseValidator("security", "1.0.0", []string{"secrets", "compliance"}),
        secretValidator:   NewSecretValidator(),
        complianceValidator: NewComplianceValidator(),
    }
}

func (s *SecurityValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
    // Chain validators based on validation rules
    chain := chains.NewCompositeValidator("security-chain", "1.0.0")
    
    if !options.ShouldSkipRule("secrets") {
        chain.Add(s.secretValidator)
    }
    if !options.ShouldSkipRule("compliance") {
        chain.Add(s.complianceValidator)
    }
    
    return chain.Validate(ctx, data, options)
}
```

### Migration Pattern:
```go
// 1. Create new unified validator
// 2. Update existing file to use unified validator
// 3. Add compatibility layer if needed
// 4. Test thoroughly
// 5. Remove old implementation
```

## 🎯 FINAL DELIVERABLES

At completion, you must deliver:

1. **Unified validation package** (`pkg/mcp/validation/`) with all validators
2. **Single ValidationResult type** used throughout codebase
3. **Zero duplicate validation utilities** remaining
4. **Complete migration documentation** showing before/after
5. **All tests passing** with no functionality lost
6. **Performance within 5%** of original implementation

**Remember**: You are the foundation for other workstreams. Your success enables their success. Focus on creating a robust, well-tested unified validation system that others can depend on! 🚀

---

**CRITICAL**: Stop work and create summary at end of each day. Do not attempt merges - external coordination will handle branch management. Your job is to migrate validation systematically and maintain quality throughout.