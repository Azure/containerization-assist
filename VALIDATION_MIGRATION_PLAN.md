# 📋 VALIDATION MIGRATION PLAN: Remove Old Validation Packages

## Overview
This plan outlines the migration from `pkg/common/validation*` packages to the new unified validation system in `pkg/mcp/domain/validation` and `pkg/mcp/domain/security`.

## Analysis Summary

### Files to Migrate (7 total):
1. **pkg/mcp/domain/security/tag_based_validator.go** - Uses `validation-core/core`
2. **pkg/mcp/application/internal/runtime/validator_service.go** - Uses `validation-core/core`
3. **pkg/mcp/application/internal/runtime/validator_test.go** - Uses `validation-core/core`
4. **pkg/mcp/application/internal/runtime/validator.go** - Heavy usage of `validation-core`
5. **pkg/mcp/application/internal/conversation/chat_tool.go** - Uses `validation`
6. **pkg/mcp/application/internal/conversation/validators.go** - Heavy usage of both packages
7. **pkg/mcp/application/state/validators.go** - Uses `validation-core/core`

## Migration Phases

### Phase 1: Easy Migrations (1-2 hours) 🟢
Simple import replacements with minimal code changes:

#### 1.1 pkg/mcp/domain/security/tag_based_validator.go
- **Current**: `pkg/common/validation-core/core`
- **Target**: Remove import, use internal security types
- **Complexity**: Easy - already has replacement types
- **Changes**: Remove `core.*` imports, use existing `security.*` types

#### 1.2 pkg/mcp/application/internal/runtime/validator_service.go
- **Current**: `core.ValidationOptions`, `core.NonGenericResult`
- **Target**: `security.Options`, `security.Result`
- **Complexity**: Easy - direct type replacement

#### 1.3 pkg/mcp/application/internal/conversation/chat_tool.go
- **Current**: `validation.ValidationError`
- **Target**: `security.ValidationError`
- **Complexity**: Easy - single type replacement

### Phase 2: Medium Migrations (2-3 hours) 🟡
Interface updates while preserving logic:

#### 2.1 pkg/mcp/application/state/validators.go
- **Current**: `core.*` types for state validation
- **Target**: `validation.DomainValidator` interface
- **Complexity**: Medium - interface conversion required
- **Strategy**: Use `validation.ValidationResult` and `validation.DomainValidator[T]`

#### 2.2 pkg/mcp/application/internal/runtime/validator_test.go
- **Current**: `core.*` types for testing
- **Target**: New validation test patterns
- **Complexity**: Medium - test updates required

### Phase 3: Hard Migrations (4-6 hours) 🔴
Significant refactoring required:

#### 3.1 pkg/mcp/application/internal/runtime/validator.go
- **Current**: Heavy usage of `validation-core` registry and interfaces
- **Target**: `validation.ValidatorRegistry` and `validation.DomainValidator`
- **Complexity**: Hard - core architecture changes
- **Strategy**:
  - Convert `core.ValidatorRegistry` → `validation.ValidatorRegistry`
  - Convert `core.Validator` → `validation.DomainValidator[T]`
  - Preserve runtime validation logic
  - Update error handling patterns

#### 3.2 pkg/mcp/application/internal/conversation/validators.go
- **Current**: Heavy usage of both validation packages
- **Target**: `validation.DomainValidator` system
- **Complexity**: Hard - comprehensive refactoring
- **Strategy**:
  - Convert `validation.BaseValidator` → `validation.DomainValidator[T]`
  - Migrate validation rules to new error system
  - Preserve conversation-specific validation logic
  - Update result handling

### Phase 4: Package Removal & Testing (1 hour) 🧹
- Remove `pkg/common/validation/` directory
- Remove `pkg/common/validation-core/` directory
- Run full test suite: `make test`
- Fix any remaining import issues
- Verify all builds pass: `go build ./pkg/mcp/...`

## Migration Patterns

### Easy Migration Pattern:
```go
// BEFORE:
import "github.com/Azure/container-kit/pkg/common/validation-core/core"
result := &core.NonGenericResult{}

// AFTER:
import "github.com/Azure/container-kit/pkg/mcp/domain/security"
result := &security.Result{}
```

### Medium Migration Pattern:
```go
// BEFORE:
type MyValidator struct {
    // fields
}
func (v *MyValidator) Validate(ctx context.Context, data interface{}) *core.Result

// AFTER:
type MyValidator struct {
    // fields  
}
func (v *MyValidator) Validate(ctx context.Context, data interface{}) validation.ValidationResult
func (v *MyValidator) Domain() string { return "my-domain" }
func (v *MyValidator) Category() string { return "my-category" }
func (v *MyValidator) Priority() int { return 100 }
func (v *MyValidator) Dependencies() []string { return nil }
```

### Hard Migration Pattern:
```go
// BEFORE: 
type ValidatorRegistry interface {
    Register(validator core.Validator) error
    // ... core methods
}

// AFTER:
type ValidatorRegistry interface {
    Register(validator validation.DomainValidator[interface{}]) error  
    // ... validation methods
}
```

## Available New Types

### From pkg/mcp/domain/security:
- `security.ValidationResult[T]` - Type-safe validation results
- `security.ValidationError` - Rich validation errors
- `security.ValidationWarning` - Validation warnings  
- `security.Validator` - Main validator interface
- `security.TypedValidator[T]` - Type-safe validator interface
- `security.Result` - Untyped validation result for compatibility
- `security.Error` - Rich error with severity and context
- `security.Warning` - Rich warning with suggestions
- `security.Options` - Validation options
- `security.Metadata` - Validation metadata

### From pkg/mcp/domain/validation:
- `validation.Validator[T]` - Generic validator interface
- `validation.DomainValidator[T]` - Domain-specific validator
- `validation.ValidatorRegistry` - Validator registry interface
- `validation.ValidationResult` - Validation result type
- `validation.ValidationContext` - Validation context
- `validation.ValidatorChain[T]` - Validator composition

## Risk Mitigation

1. **Incremental Approach**: Migrate one file at a time
2. **Test After Each Phase**: Ensure builds and tests pass
3. **Preserve Behavior**: Keep existing validation logic intact
4. **Git Commits**: Commit after each successful file migration
5. **Rollback Plan**: Can revert individual file changes if needed

## Success Criteria

- ✅ All 7 files successfully migrated
- ✅ `pkg/common/validation*` directories removed
- ✅ All tests pass: `make test`
- ✅ All packages build: `go build ./pkg/mcp/...`
- ✅ No remaining imports of old validation packages
- ✅ Existing validation behavior preserved

## Time Estimate: 8-12 hours total
- Phase 1: 1-2 hours (3 easy files)
- Phase 2: 2-3 hours (2 medium files)  
- Phase 3: 4-6 hours (2 hard files)
- Phase 4: 1 hour (cleanup & testing)

## Next Steps
1. Start with Phase 1 easy migrations
2. Test after each file migration  
3. Proceed through phases sequentially
4. Complete with package removal and final testing