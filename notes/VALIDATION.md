# Unified Validation Implementation Plan

## Executive Summary

This plan outlines the systematic migration from scattered validation code to the unified validation framework across the Container Kit codebase. The migration will be executed in phases to minimize disruption while maximizing benefits.

## Current State Analysis

### Validation Infrastructure Inventory

**Scattered Validation Locations (39+ files):**
- `pkg/mcp/types/validation.go` - 4 different ValidationResult types
- `pkg/mcp/internal/errors/validation.go` - Legacy validation errors
- `pkg/mcp/internal/runtime/validator.go` - Runtime-specific validation
- `pkg/mcp/utils/typed_validation.go` - Type-safe validation
- `pkg/mcp/utils/validation_utils.go` - String/format utilities
- `pkg/mcp/utils/path_utils.go` - Path validation (3+ implementations)
- `pkg/mcp/internal/build/security_validator.go` - Security validation
- `pkg/mcp/internal/build/syntax_validator.go` - Dockerfile syntax
- `pkg/mcp/internal/deploy/health_validator.go` - Health validation
- `pkg/mcp/internal/scan/validators.go` - Secret scanning validation
- `pkg/mcp/internal/state/validators.go` - State validation

**Key Problems:**
- 4+ different ValidationResult type definitions
- 6+ incompatible validator interfaces
- Duplicate validation utilities across packages
- Inconsistent error handling and context
- No centralized validation registry

## Migration Strategy

### Phase 1: Foundation & Core Infrastructure (Week 1-2)
**Goal:** Establish unified validation foundation

#### 1.1 Core Package Stabilization
- [x] ✅ **COMPLETED**: Unified validation package structure
- [x] ✅ **COMPLETED**: Core types (ValidationResult, ValidationError, ValidationOptions)
- [x] ✅ **COMPLETED**: Standard validator interfaces
- [x] ✅ **COMPLETED**: Validator registry and factory
- [x] ✅ **COMPLETED**: Validator composition (chains, parallel)

#### 1.2 Utility Consolidation
- [x] ✅ **COMPLETED**: String validation utilities
- [x] ✅ **COMPLETED**: Path validation utilities
- [ ] **TODO**: Format validation utilities (email, URL, JSON, YAML)
- [ ] **TODO**: Network validation utilities (IP, port, hostname)
- [ ] **TODO**: Security validation utilities (secrets, permissions)

#### 1.3 Migration Infrastructure
- [x] ✅ **COMPLETED**: Migration utilities and compatibility layers
- [x] ✅ **COMPLETED**: Legacy result conversion functions
- [ ] **TODO**: Automated migration detection tools
- [ ] **TODO**: Validation pattern analysis scripts

### Phase 2: Domain Validator Migration (Week 3-4)
**Goal:** Replace domain-specific validators with unified implementations

#### 2.1 Build Package Migration
- [x] ✅ **COMPLETED**: Docker validation (Dockerfile, image names)
- [ ] **IN PROGRESS**: Security validation migration
- [ ] **TODO**: Syntax validation complete migration
- [ ] **TODO**: Context validation migration
- [ ] **TODO**: Image validation migration

**Implementation Tasks:**
```bash
# Replace existing build validators
pkg/mcp/internal/build/security_validator.go → Use pkg/mcp/validation/validators/docker.go
pkg/mcp/internal/build/syntax_validator.go → Update to use unified validation
pkg/mcp/internal/build/context_validator.go → Migrate to unified framework
pkg/mcp/internal/build/image_validator.go → Consolidate with docker validator
```

#### 2.2 Deploy Package Migration
- [ ] **TODO**: Health validation migration
- [ ] **TODO**: Manifest validation migration 
- [ ] **TODO**: Deployment validation migration
- [ ] **TODO**: Kubernetes resource validation

**Implementation Tasks:**
```bash
# Create unified deploy validators
pkg/mcp/validation/validators/kubernetes.go → New unified K8s validator
pkg/mcp/validation/validators/health.go → Migrate health validation
pkg/mcp/internal/deploy/health_validator.go → Update to use unified
pkg/mcp/internal/deploy/manifest_validator.go → Update to use unified
```

#### 2.3 Scan Package Migration
- [ ] **TODO**: Secret scanning validation
- [ ] **TODO**: Vulnerability validation
- [ ] **TODO**: Compliance validation

**Implementation Tasks:**
```bash
# Integrate with existing security validation
pkg/mcp/validation/validators/security.go → Create security validator
pkg/mcp/internal/scan/validators.go → Update to use unified
```

### Phase 3: Core System Integration (Week 5-6)
**Goal:** Integrate unified validation into core MCP systems

#### 3.1 Runtime System Integration
- [ ] **TODO**: Tool validation integration
- [ ] **TODO**: Session validation integration
- [ ] **TODO**: Workflow validation integration

**Implementation Tasks:**
```go
// Update runtime validation
pkg/mcp/internal/runtime/validator.go → Use unified validation
pkg/mcp/internal/runtime/tool_validator.go → Migrate to unified
pkg/mcp/internal/session/validation.go → Update validation calls
pkg/mcp/internal/workflow/validation.go → Update validation calls
```

#### 3.2 State Management Integration
- [ ] **TODO**: Session state validation
- [ ] **TODO**: Conversation state validation
- [ ] **TODO**: Configuration validation

**Implementation Tasks:**
```go
// Update state validation
pkg/mcp/internal/state/validators.go → Use unified validation
pkg/mcp/internal/config/validation.go → Migrate to unified
```

#### 3.3 Transport Layer Integration
- [ ] **TODO**: Request validation
- [ ] **TODO**: Response validation
- [ ] **TODO**: Protocol validation

### Phase 4: Legacy Code Cleanup (Week 7-8)
**Goal:** Remove duplicate validation code and complete migration

#### 4.1 Remove Duplicate Utilities
- [ ] **TODO**: Delete duplicate string validation functions
- [ ] **TODO**: Delete duplicate path validation functions
- [ ] **TODO**: Delete duplicate format validation functions
- [ ] **TODO**: Consolidate validation error types

#### 4.2 Update Import Statements
- [ ] **TODO**: Replace validation utility imports across codebase
- [ ] **TODO**: Update validation result type usage
- [ ] **TODO**: Update validation error handling

#### 4.3 Documentation Updates
- [ ] **TODO**: Update API documentation
- [ ] **TODO**: Update developer guides
- [ ] **TODO**: Create validation best practices guide

## Implementation Details

### Phase 2 Detailed Implementation

#### 2.1 Build Package Migration

**Step 1: Complete Syntax Validator Migration**
```go
// File: pkg/mcp/internal/build/syntax_validator.go
// Replace existing implementation with unified validation

package build

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/validation/core"
    "github.com/Azure/container-kit/pkg/mcp/validation/validators"
)

type SyntaxValidator struct {
    validator core.Validator
    logger    zerolog.Logger
}

func NewSyntaxValidator(logger zerolog.Logger) *SyntaxValidator {
    return &SyntaxValidator{
        validator: validators.NewDockerfileValidator().WithSyntaxChecks(true),
        logger:    logger,
    }
}

func (v *SyntaxValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
    ctx := context.Background()
    unifiedOptions := convertLegacyOptions(options)
    result := v.validator.Validate(ctx, content, unifiedOptions)
    return convertToLegacyResult(result), nil
}
```

**Step 2: Security Validator Integration**
```go
// File: pkg/mcp/validation/validators/security.go
// Create comprehensive security validator

package validators

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/validation/core"
)

type SecurityValidator struct {
    *BaseValidatorImpl
    dockerfileValidator *DockerfileValidator
    secretValidator     *SecretValidator
    complianceValidator *ComplianceValidator
}

func NewSecurityValidator() *SecurityValidator {
    return &SecurityValidator{
        BaseValidatorImpl:   NewBaseValidator("security", "1.0.0", []string{"dockerfile", "config", "secrets"}),
        dockerfileValidator: NewDockerfileValidator().WithSecurityChecks(true),
        secretValidator:     NewSecretValidator(),
        complianceValidator: NewComplianceValidator(),
    }
}

func (s *SecurityValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
    // Chain security validators based on data type
    chain := chains.NewCompositeValidator("security-chain", "1.0.0")
    
    switch data.(type) {
    case string: // Dockerfile content
        chain.Add(s.dockerfileValidator)
    case map[string]interface{}: // Configuration
        chain.Add(s.secretValidator)
    }
    
    return chain.Validate(ctx, data, options)
}
```

#### 2.2 Deploy Package Migration

**Step 1: Kubernetes Validator Creation**
```go
// File: pkg/mcp/validation/validators/kubernetes.go
// Create unified Kubernetes resource validator

package validators

type KubernetesValidator struct {
    *BaseValidatorImpl
    manifestValidator *ManifestValidator
    resourceValidator *ResourceValidator
    healthValidator   *HealthValidator
}

func NewKubernetesValidator() *KubernetesValidator {
    return &KubernetesValidator{
        BaseValidatorImpl: NewBaseValidator("kubernetes", "1.0.0", []string{"yaml", "json", "k8s-manifest"}),
        manifestValidator: NewManifestValidator(),
        resourceValidator: NewResourceValidator(),
        healthValidator:   NewHealthValidator(),
    }
}

func (k *KubernetesValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
    // Validate Kubernetes manifests, resources, and health configurations
    chain := chains.NewCompositeValidator("k8s-chain", "1.0.0")
    
    // Add validators based on validation rules enabled
    if !options.ShouldSkipRule("manifest") {
        chain.Add(k.manifestValidator)
    }
    if !options.ShouldSkipRule("resource") {
        chain.Add(k.resourceValidator)
    }
    if !options.ShouldSkipRule("health") {
        chain.Add(k.healthValidator)
    }
    
    return chain.Validate(ctx, data, options)
}
```

### Phase 3 Implementation Examples

#### 3.1 Runtime System Integration

**Tool Validation Integration**
```go
// File: pkg/mcp/internal/runtime/tool_manager.go
// Update to use unified validation

package runtime

import (
    "context"
    "github.com/Azure/container-kit/pkg/mcp/validation/core"
    "github.com/Azure/container-kit/pkg/mcp/validation/validators"
)

type ToolManager struct {
    validator core.Validator
    registry  core.ValidatorRegistry
}

func NewToolManager() *ToolManager {
    // Create composite validator for tools
    chain := chains.NewCompositeValidator("tool-validation", "1.0.0")
    chain.Add(validators.NewConfigValidator())
    chain.Add(validators.NewSecurityValidator())
    chain.Add(validators.NewSchemaValidator())
    
    return &ToolManager{
        validator: chain,
        registry:  core.GlobalRegistry,
    }
}

func (tm *ToolManager) ValidateTool(ctx context.Context, toolConfig interface{}) error {
    options := core.NewValidationOptions().WithStrictMode(true)
    result := tm.validator.Validate(ctx, toolConfig, options)
    
    if !result.Valid {
        return fmt.Errorf("tool validation failed: %s", result.Error())
    }
    
    return nil
}
```

## Migration Execution Plan

### Week-by-Week Schedule

**Week 1: Foundation Setup**
- [x] Complete unified validation package (DONE)
- [ ] Create remaining utility validators (format, network, security)
- [ ] Set up migration tooling and analysis scripts
- [ ] Create comprehensive test suite for unified validation

**Week 2: Build Package Migration**
- [ ] Migrate syntax_validator.go to unified system
- [ ] Migrate security_validator.go completely
- [ ] Migrate context_validator.go and image_validator.go
- [ ] Update all build package validation calls

**Week 3: Deploy Package Migration**
- [ ] Create Kubernetes validator
- [ ] Migrate health validation
- [ ] Migrate manifest validation
- [ ] Update deploy package validation calls

**Week 4: Scan Package Migration**
- [ ] Integrate secret scanning with unified security validator
- [ ] Migrate vulnerability validation
- [ ] Update scan package validation calls

**Week 5: Core Runtime Integration**
- [ ] Integrate tool validation
- [ ] Integrate session validation
- [ ] Integrate workflow validation
- [ ] Update runtime package validation calls

**Week 6: State & Transport Integration**
- [ ] Migrate state validation
- [ ] Migrate configuration validation
- [ ] Integrate transport validation
- [ ] Update all core system validation calls

**Week 7: Legacy Cleanup**
- [ ] Remove duplicate validation utilities
- [ ] Delete legacy validation types
- [ ] Update all import statements
- [ ] Clean up unused validation code

**Week 8: Documentation & Testing**
- [ ] Update all documentation
- [ ] Create migration guides
- [ ] Comprehensive testing of unified system
- [ ] Performance validation and optimization

## Success Metrics

### Quantitative Goals
- **Code Reduction**: Remove 30+ duplicate validation files
- **Type Unification**: Consolidate 4+ ValidationResult types into 1
- **Interface Standardization**: Replace 6+ validator interfaces with unified system
- **Test Coverage**: Maintain >95% test coverage during migration
- **Performance**: Validation performance within 5% of current implementation

### Qualitative Goals
- **Developer Experience**: Consistent validation API across all packages
- **Maintainability**: Centralized validation logic reduces maintenance burden
- **Extensibility**: New validators can be easily added and composed
- **Documentation**: Comprehensive documentation and examples available

## Risk Management

### Migration Risks & Mitigation

**Risk 1: Breaking Changes During Migration**
- *Mitigation*: Use compatibility layers during transition period
- *Rollback Plan*: Keep legacy validators until migration is fully validated

**Risk 2: Performance Regression**
- *Mitigation*: Benchmark each migration step
- *Monitoring*: Continuous performance monitoring during rollout

**Risk 3: Integration Issues**
- *Mitigation*: Comprehensive integration testing at each phase
- *Validation*: Automated tests for all validation scenarios

## Implementation Commands

### Phase 2 Command Sequence

```bash
# Week 2: Build Package Migration

# Step 1: Backup existing validators
cp pkg/mcp/internal/build/syntax_validator.go pkg/mcp/internal/build/syntax_validator.go.backup
cp pkg/mcp/internal/build/security_validator.go pkg/mcp/internal/build/security_validator.go.backup

# Step 2: Create new unified validators
# (Create security_enhanced.go, kubernetes.go, etc.)

# Step 3: Update imports across build package
find pkg/mcp/internal/build -name "*.go" -exec sed -i 's/ValidationResult/core.ValidationResult/g' {} \;
find pkg/mcp/internal/build -name "*.go" -exec sed -i 's/ValidationError/core.ValidationError/g' {} \;

# Step 4: Update import statements
find pkg/mcp/internal/build -name "*.go" -exec sed -i '/import/a\\t"github.com/Azure/container-kit/pkg/mcp/validation/core"' {} \;

# Step 5: Run tests and validate
go test ./pkg/mcp/internal/build/...
go test ./pkg/mcp/validation/...

# Step 6: Performance benchmarks
go test -bench=. ./pkg/mcp/validation/...
```

### Phase 3 Command Sequence

```bash
# Week 5: Runtime Integration

# Update runtime package
find pkg/mcp/internal/runtime -name "*.go" -exec grep -l "ValidationResult" {} \; | \
xargs sed -i 's|pkg/mcp/types|pkg/mcp/validation/core|g'

# Update session package  
find pkg/mcp/internal/session -name "*.go" -exec grep -l "validation" {} \; | \
xargs sed -i 's|pkg/mcp/types|pkg/mcp/validation/core|g'

# Test integration
go test ./pkg/mcp/internal/runtime/...
go test ./pkg/mcp/internal/session/...
```

## Conclusion

This implementation plan provides a systematic approach to migrating the entire codebase to the unified validation framework. The phased approach minimizes risk while ensuring comprehensive coverage of all validation use cases.

**Key Success Factors:**
1. **Incremental Migration**: Phase-by-phase approach allows validation at each step
2. **Backward Compatibility**: Compatibility layers ensure no breaking changes during transition
3. **Comprehensive Testing**: Extensive testing at each phase validates functionality
4. **Clear Timeline**: 8-week timeline with specific deliverables and milestones
5. **Risk Management**: Identified risks with specific mitigation strategies

**Next Steps:**
1. Begin Phase 2 implementation (Build Package Migration)
2. Set up continuous integration for validation testing
3. Create automated migration tooling for import updates
4. Begin documentation updates for unified validation system

The unified validation framework will significantly improve code maintainability, developer experience, and system reliability across the Container Kit platform.