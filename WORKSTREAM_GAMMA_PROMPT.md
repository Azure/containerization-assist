# WORKSTREAM GAMMA: Error System & Validation Implementation Guide

## 🎯 Mission
Standardize error handling to RichError throughout the codebase, consolidate 4 validation packages into a unified system, and remove all deprecated code. This workstream creates consistent error patterns and reduces technical debt.

## 📋 Context
- **Project**: Container Kit Architecture Refactoring
- **Your Role**: Error system architect - you create consistent error handling patterns
- **Timeline**: Week 3-6 (28 days)
- **Dependencies**: ALPHA Week 2 complete (package structure stable)
- **Deliverables**: Clean error system needed by all workstreams, validation system for DELTA

## 🎯 Success Metrics
- **Error standardization**: 622 fmt.Errorf calls → <10 grandfathered instances
- **Validation consolidation**: 4 validation packages → 1 unified system
- **Deprecated code removal**: 72 deprecated items → 0 remaining
- **RichError adoption**: Mixed patterns → 100% in pkg/mcp domain layer
- **Error consistency**: Multiple error types → Single RichError interface

## 📁 File Ownership
You have exclusive ownership of these files/directories:
```
pkg/mcp/domain/errors/ (complete ownership)
pkg/common/validation/ (consolidation and removal)
pkg/common/validation-core/ (consolidation and removal)
pkg/mcp/domain/validation/ (new unified system)
All deprecated code removal throughout codebase
Error linting rules and enforcement
```

Shared files requiring coordination:
```
pkg/mcp/application/api/interfaces.go - Error interfaces (coordinate with BETA)
pkg/mcp/domain/tools/validation.go - Tool validation errors
All files with fmt.Errorf calls (systematic replacement)
CI linting configuration for error enforcement
```

## 🗓️ Implementation Schedule

### Week 3: Error Pattern Analysis & Setup

#### Day 1: Error Pattern Analysis
**Morning Goals**:
- [ ] **DEPENDENCY CHECK**: Verify ALPHA Week 2 completion before starting
- [ ] Audit current error patterns: `grep -r "fmt\.Errorf" pkg/mcp/ | wc -l`
- [ ] Audit RichError usage: `grep -r "RichError\|NewError" pkg/mcp/ | wc -l`
- [ ] Map error propagation patterns

**Error Analysis Commands**:
```bash
# Verify ALPHA dependency met
scripts/check_import_depth.sh --max-depth=3 || (echo "❌ ALPHA Week 2 not complete" && exit 1)

# Create comprehensive error audit
echo "=== ERROR PATTERN AUDIT ===" > error_audit.txt
echo "fmt.Errorf usage:" >> error_audit.txt
grep -r "fmt\.Errorf" pkg/mcp/ | wc -l >> error_audit.txt
echo "RichError usage:" >> error_audit.txt
grep -r "RichError\|NewError" pkg/mcp/ | wc -l >> error_audit.txt
echo "Mixed error files:" >> error_audit.txt
grep -l "fmt\.Errorf" pkg/mcp/**/*.go | xargs grep -l "RichError\|NewError" | wc -l >> error_audit.txt

# Identify error hotspots
grep -r "fmt\.Errorf" pkg/mcp/ | cut -d: -f1 | sort | uniq -c | sort -nr | head -20 > error_hotspots.txt
```

**Validation Commands**:
```bash
# Verify audit complete
test -f error_audit.txt && test -f error_hotspots.txt && echo "✅ Error audit documented"

# Pre-commit validation
alias make='/usr/bin/make'
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] **DEPENDENCY**: ALPHA Week 2 completion verified
- [ ] Error patterns documented (622 fmt.Errorf baseline)
- [ ] Error hotspots identified
- [ ] Changes committed

#### Day 2: RichError Helper Design
**Morning Goals**:
- [ ] Design RichError helper functions for common patterns
- [ ] Create error constructors for frequent use cases
- [ ] Plan error code taxonomy
- [ ] Design error context standardization

**Helper Design Commands**:
```bash
# Create error helpers
cat > pkg/mcp/domain/errors/constructors.go << 'EOF'
package errors

import (
    "fmt"
    "pkg/mcp/domain/errors"
)

// Common error constructors for frequent patterns

// NewMissingParam creates a validation error for missing required parameters
func NewMissingParam(field string) error {
    return errors.NewError().
        Code(errors.CodeValidationFailed).
        Type(errors.ErrTypeValidation).
        Severity(errors.SeverityMedium).
        Message(fmt.Sprintf("missing required parameter: %s", field)).
        Context("field", field).
        Suggestion("Provide the required parameter").
        WithLocation().
        Build()
}

// NewValidationFailed creates a validation error with context
func NewValidationFailed(field, reason string) error {
    return errors.NewError().
        Code(errors.CodeValidationFailed).
        Type(errors.ErrTypeValidation).
        Severity(errors.SeverityMedium).
        Message(fmt.Sprintf("validation failed for %s: %s", field, reason)).
        Context("field", field).
        Context("reason", reason).
        Suggestion("Check the field value and format").
        WithLocation().
        Build()
}

// NewInternalError creates an internal error wrapping a cause
func NewInternalError(operation string, cause error) error {
    return errors.NewError().
        Code(errors.CodeInternalError).
        Type(errors.ErrTypeInternal).
        Severity(errors.SeverityHigh).
        Message(fmt.Sprintf("internal error during %s", operation)).
        Context("operation", operation).
        Cause(cause).
        WithLocation().
        Build()
}

// NewConfigurationError creates a configuration error
func NewConfigurationError(component, issue string) error {
    return errors.NewError().
        Code(errors.CodeConfigurationError).
        Type(errors.ErrTypeConfiguration).
        Severity(errors.SeverityHigh).
        Message(fmt.Sprintf("configuration error in %s: %s", component, issue)).
        Context("component", component).
        Context("issue", issue).
        Suggestion("Check configuration file and environment variables").
        WithLocation().
        Build()
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, identifier string) error {
    return errors.NewError().
        Code(errors.CodeNotFound).
        Type(errors.ErrTypeNotFound).
        Severity(errors.SeverityMedium).
        Message(fmt.Sprintf("%s not found: %s", resource, identifier)).
        Context("resource", resource).
        Context("identifier", identifier).
        WithLocation().
        Build()
}
EOF

# Test error helpers compilation
go build ./pkg/mcp/domain/errors && echo "✅ Error helpers compile"
```

**Validation Commands**:
```bash
# Test error helpers
go test ./pkg/mcp/domain/errors && echo "✅ Error helpers tested"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Error helper functions created
- [ ] Error constructors implemented
- [ ] Error taxonomy planned
- [ ] Changes committed

#### Day 3: Error Linting Rules
**Morning Goals**:
- [ ] Create error linting tool in `tools/linters/richerror-boundary/`
- [ ] Implement progressive error reduction targets
- [ ] Add CI rules for error enforcement
- [ ] Test linting on current codebase

**Linting Implementation Commands**:
```bash
# Create error linting tool
mkdir -p tools/linters/richerror-boundary

cat > tools/linters/richerror-boundary/main.go << 'EOF'
package main

import (
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"
    "os"
    "path/filepath"
    "strconv"
    "strings"
)

func main() {
    if len(os.Args) != 3 {
        fmt.Fprintf(os.Stderr, "Usage: %s <directory> <max-fmt-errorf>\n", os.Args[0])
        os.Exit(1)
    }
    
    dir := os.Args[1]
    maxFmtErrorf, err := strconv.Atoi(os.Args[2])
    if err != nil {
        fmt.Fprintf(os.Stderr, "Invalid max-fmt-errorf value: %v\n", err)
        os.Exit(1)
    }
    
    count := 0
    err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        if !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") {
            return nil
        }
        
        fset := token.NewFileSet()
        node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
        if err != nil {
            return nil // Skip files with parse errors
        }
        
        ast.Inspect(node, func(n ast.Node) bool {
            if call, ok := n.(*ast.CallExpr); ok {
                if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
                    if id, ok := sel.X.(*ast.Ident); ok {
                        if id.Name == "fmt" && sel.Sel.Name == "Errorf" {
                            count++
                            pos := fset.Position(call.Pos())
                            fmt.Printf("%s:%d: fmt.Errorf usage found\n", pos.Filename, pos.Line)
                        }
                    }
                }
            }
            return true
        })
        
        return nil
    })
    
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Total fmt.Errorf usage: %d (max allowed: %d)\n", count, maxFmtErrorf)
    
    if count > maxFmtErrorf {
        fmt.Printf("❌ fmt.Errorf usage exceeds limit\n")
        os.Exit(1)
    }
    
    fmt.Printf("✅ fmt.Errorf usage within limit\n")
}
EOF

# Build and test linting tool
go build -o tools/linters/richerror-boundary/richerror-boundary ./tools/linters/richerror-boundary/
./tools/linters/richerror-boundary/richerror-boundary pkg/mcp 622 && echo "✅ Error linting tool working"

# Create convenience script
cat > scripts/check-error-patterns.sh << 'EOF'
#!/bin/bash
set -e

MAX_FMT_ERRORF=${1:-10}
echo "Checking error patterns (max fmt.Errorf: $MAX_FMT_ERRORF)..."

# Build linter if needed
if [ ! -f tools/linters/richerror-boundary/richerror-boundary ]; then
    go build -o tools/linters/richerror-boundary/richerror-boundary ./tools/linters/richerror-boundary/
fi

# Run error pattern check
./tools/linters/richerror-boundary/richerror-boundary pkg/mcp $MAX_FMT_ERRORF
EOF

chmod +x scripts/check-error-patterns.sh
```

**Validation Commands**:
```bash
# Test error linting
scripts/check-error-patterns.sh 622 && echo "✅ Error linting active"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Error linting tool created
- [ ] Progressive reduction targets set
- [ ] CI integration prepared
- [ ] Changes committed

#### Day 4: Validation System Analysis
**Morning Goals**:
- [ ] Audit all validation packages and their usage
- [ ] Identify validation code duplication
- [ ] Map validation patterns across packages
- [ ] Plan unified validation architecture

**Validation Analysis Commands**:
```bash
# Comprehensive validation audit
echo "=== VALIDATION SYSTEM AUDIT ===" > validation_audit.txt
echo "Validation packages found:" >> validation_audit.txt
find pkg -name "*validat*" -type f | wc -l >> validation_audit.txt
echo "" >> validation_audit.txt

echo "Package details:" >> validation_audit.txt
echo "pkg/mcp/application/internal/conversation/validators.go:" >> validation_audit.txt
wc -l pkg/mcp/application/internal/conversation/validators.go >> validation_audit.txt
echo "pkg/mcp/domain/security/validators.go:" >> validation_audit.txt
wc -l pkg/mcp/domain/security/validators.go >> validation_audit.txt
echo "pkg/common/validation/:" >> validation_audit.txt
find pkg/common/validation -name "*.go" -exec wc -l {} \; >> validation_audit.txt
echo "pkg/common/validation-core/:" >> validation_audit.txt
find pkg/common/validation-core -name "*.go" -exec wc -l {} \; >> validation_audit.txt

# Identify validation patterns
grep -r "func.*Validate" pkg/mcp/ pkg/common/ | wc -l > validation_functions.txt
grep -r "validation.*error\|error.*validation" pkg/mcp/ pkg/common/ | wc -l >> validation_functions.txt
```

**Validation Commands**:
```bash
# Verify audit complete
test -f validation_audit.txt && test -f validation_functions.txt && echo "✅ Validation audit documented"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Validation systems audited
- [ ] Duplication patterns identified
- [ ] Architecture planned
- [ ] Changes committed

#### Day 5: Deprecated Code Cataloging
**Morning Goals**:
- [ ] Create comprehensive deprecated code inventory
- [ ] Prioritize deprecated items by impact
- [ ] Plan removal strategy and timeline
- [ ] Identify migration paths for each deprecated item

**Deprecated Code Analysis Commands**:
```bash
# Create comprehensive deprecated inventory
echo "=== DEPRECATED CODE INVENTORY ===" > deprecated_inventory.txt
echo "High Priority Service Interfaces:" >> deprecated_inventory.txt
grep -r "Deprecated.*Use.*api\." pkg/mcp/application/services/ >> deprecated_inventory.txt
echo "" >> deprecated_inventory.txt

echo "Validation System Deprecations:" >> deprecated_inventory.txt
grep -r "Deprecated.*Use.*validation-core\|DEPRECATED.*reflection" pkg/common/validation/ >> deprecated_inventory.txt
echo "" >> deprecated_inventory.txt

echo "Tool Registry Deprecations:" >> deprecated_inventory.txt
grep -r "Deprecated.*Use.*ToolRegistry\|Deprecated.*Use.*services" pkg/mcp/application/core/ >> deprecated_inventory.txt
echo "" >> deprecated_inventory.txt

echo "Schema Generation Deprecations:" >> deprecated_inventory.txt
grep -r "Deprecated.*Use.*\|Deprecated:" pkg/mcp/domain/tools/ >> deprecated_inventory.txt
echo "" >> deprecated_inventory.txt

echo "State Management Deprecations:" >> deprecated_inventory.txt
grep -r "Deprecated.*Use.*services\." pkg/mcp/application/state/ >> deprecated_inventory.txt

# Count total deprecated items
DEPRECATED_COUNT=$(grep -r "Deprecated\|DEPRECATED" pkg/mcp/ pkg/common/ | wc -l)
echo "Total deprecated items: $DEPRECATED_COUNT" >> deprecated_inventory.txt

# Create removal priority list
cat > deprecated_removal_plan.txt << 'EOF'
# Deprecated Code Removal Plan

## High Priority (Week 3-4)
1. Service interfaces (pkg/mcp/application/services/retry.go, transport.go)
2. Core registry (pkg/mcp/application/core/registry.go)
3. Server interfaces (pkg/mcp/application/core/server.go functions)

## Medium Priority (Week 5)
1. Validation system (pkg/common/validation/unified_validator.go)
2. Schema functions (pkg/mcp/domain/tools/schema.go - 10 functions)
3. Tool validation (pkg/mcp/domain/tools/tool_validation.go)

## Low Priority (Week 6)
1. State management (pkg/mcp/application/state/ deprecated functions)
2. Workflow engine (pkg/mcp/application/workflows/engine.go)
3. Documentation updates
EOF
```

**Validation Commands**:
```bash
# Verify deprecated inventory complete
test -f deprecated_inventory.txt && test -f deprecated_removal_plan.txt && echo "✅ Deprecated code cataloged"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Deprecated code inventory complete (72 items)
- [ ] Removal strategy prioritized
- [ ] Migration paths identified
- [ ] Changes committed

### Week 4: Validation System Consolidation

#### Day 6: Unified Validation Design
**Morning Goals**:
- [ ] Design unified validation interface
- [ ] Create composable validation chain pattern
- [ ] Plan validation context propagation
- [ ] Design validation error standardization

**Unified Validation Design Commands**:
```bash
# Create unified validation package
mkdir -p pkg/mcp/domain/validation

cat > pkg/mcp/domain/validation/interfaces.go << 'EOF'
package validation

import (
    "context"
    "pkg/mcp/domain/errors"
)

// Validator defines the core validation interface
type Validator[T any] interface {
    // Validate validates a value and returns validation result
    Validate(ctx context.Context, value T) ValidationResult
    
    // Name returns the validator name for error reporting
    Name() string
}

// ValidationResult holds validation outcome
type ValidationResult struct {
    Valid    bool
    Errors   []error
    Warnings []string
    Context  ValidationContext
}

// ValidationContext provides validation execution context
type ValidationContext struct {
    Field    string
    Path     string
    Metadata map[string]interface{}
}

// ValidatorChain allows composing multiple validators
type ValidatorChain[T any] struct {
    validators []Validator[T]
    strategy   ChainStrategy
}

// ChainStrategy defines how validators are executed
type ChainStrategy int

const (
    // StopOnFirstError stops chain on first validation error
    StopOnFirstError ChainStrategy = iota
    // ContinueOnError continues chain collecting all errors
    ContinueOnError
    // StopOnFirstWarning stops chain on first warning
    StopOnFirstWarning
)

// NewValidatorChain creates a new validator chain
func NewValidatorChain[T any](strategy ChainStrategy) *ValidatorChain[T] {
    return &ValidatorChain[T]{
        validators: make([]Validator[T], 0),
        strategy:   strategy,
    }
}

// Add adds a validator to the chain
func (c *ValidatorChain[T]) Add(validator Validator[T]) *ValidatorChain[T] {
    c.validators = append(c.validators, validator)
    return c
}

// Validate executes the validator chain
func (c *ValidatorChain[T]) Validate(ctx context.Context, value T) ValidationResult {
    result := ValidationResult{
        Valid:    true,
        Errors:   make([]error, 0),
        Warnings: make([]string, 0),
    }
    
    for _, validator := range c.validators {
        validationResult := validator.Validate(ctx, value)
        
        // Collect errors and warnings
        result.Errors = append(result.Errors, validationResult.Errors...)
        result.Warnings = append(result.Warnings, validationResult.Warnings...)
        
        // Apply strategy
        if !validationResult.Valid {
            result.Valid = false
            if c.strategy == StopOnFirstError {
                break
            }
        }
        
        if len(validationResult.Warnings) > 0 && c.strategy == StopOnFirstWarning {
            break
        }
    }
    
    return result
}

// Name returns the chain name
func (c *ValidatorChain[T]) Name() string {
    return "ValidatorChain"
}
EOF

# Test unified validation interface
go build ./pkg/mcp/domain/validation && echo "✅ Unified validation interface compiles"
```

**Validation Commands**:
```bash
# Verify validation design compiles
go build ./pkg/mcp/domain/validation && echo "✅ Validation design compiles"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Unified validation interface designed
- [ ] Composable chain pattern created
- [ ] Context propagation planned
- [ ] Changes committed

#### Day 7: Common Validators Implementation
**Morning Goals**:
- [ ] Implement common validators (string length, pattern, required)
- [ ] Create validators for frequent validation patterns
- [ ] Test validator composition and chaining
- [ ] Implement validation error helpers

**Common Validators Commands**:
```bash
# Create common validators
cat > pkg/mcp/domain/validation/common.go << 'EOF'
package validation

import (
    "context"
    "fmt"
    "regexp"
    "strings"
    "pkg/mcp/domain/errors"
)

// StringLengthValidator validates string length
type StringLengthValidator struct {
    MinLength int
    MaxLength int
    FieldName string
}

func NewStringLengthValidator(fieldName string, minLength, maxLength int) *StringLengthValidator {
    return &StringLengthValidator{
        MinLength: minLength,
        MaxLength: maxLength,
        FieldName: fieldName,
    }
}

func (v *StringLengthValidator) Validate(ctx context.Context, value string) ValidationResult {
    result := ValidationResult{Valid: true, Errors: make([]error, 0)}
    
    if len(value) < v.MinLength {
        result.Valid = false
        result.Errors = append(result.Errors, errors.NewValidationFailed(
            v.FieldName, 
            fmt.Sprintf("length %d is less than minimum %d", len(value), v.MinLength),
        ))
    }
    
    if len(value) > v.MaxLength {
        result.Valid = false
        result.Errors = append(result.Errors, errors.NewValidationFailed(
            v.FieldName,
            fmt.Sprintf("length %d exceeds maximum %d", len(value), v.MaxLength),
        ))
    }
    
    return result
}

func (v *StringLengthValidator) Name() string {
    return "StringLengthValidator"
}

// PatternValidator validates string patterns
type PatternValidator struct {
    Pattern   *regexp.Regexp
    FieldName string
}

func NewPatternValidator(fieldName, pattern string) (*PatternValidator, error) {
    regex, err := regexp.Compile(pattern)
    if err != nil {
        return nil, errors.NewValidationFailed("pattern", fmt.Sprintf("invalid regex: %v", err))
    }
    
    return &PatternValidator{
        Pattern:   regex,
        FieldName: fieldName,
    }, nil
}

func (v *PatternValidator) Validate(ctx context.Context, value string) ValidationResult {
    result := ValidationResult{Valid: true, Errors: make([]error, 0)}
    
    if !v.Pattern.MatchString(value) {
        result.Valid = false
        result.Errors = append(result.Errors, errors.NewValidationFailed(
            v.FieldName,
            fmt.Sprintf("value does not match pattern %s", v.Pattern.String()),
        ))
    }
    
    return result
}

func (v *PatternValidator) Name() string {
    return "PatternValidator"
}

// RequiredValidator validates required fields
type RequiredValidator struct {
    FieldName string
}

func NewRequiredValidator(fieldName string) *RequiredValidator {
    return &RequiredValidator{FieldName: fieldName}
}

func (v *RequiredValidator) Validate(ctx context.Context, value string) ValidationResult {
    result := ValidationResult{Valid: true, Errors: make([]error, 0)}
    
    if strings.TrimSpace(value) == "" {
        result.Valid = false
        result.Errors = append(result.Errors, errors.NewMissingParam(v.FieldName))
    }
    
    return result
}

func (v *RequiredValidator) Name() string {
    return "RequiredValidator"
}

// EmailValidator validates email format
type EmailValidator struct {
    FieldName string
}

func NewEmailValidator(fieldName string) *EmailValidator {
    return &EmailValidator{FieldName: fieldName}
}

func (v *EmailValidator) Validate(ctx context.Context, value string) ValidationResult {
    result := ValidationResult{Valid: true, Errors: make([]error, 0)}
    
    emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
    if !emailRegex.MatchString(value) {
        result.Valid = false
        result.Errors = append(result.Errors, errors.NewValidationFailed(
            v.FieldName,
            "invalid email format",
        ))
    }
    
    return result
}

func (v *EmailValidator) Name() string {
    return "EmailValidator"
}

// URLValidator validates URL format
type URLValidator struct {
    FieldName string
}

func NewURLValidator(fieldName string) *URLValidator {
    return &URLValidator{FieldName: fieldName}
}

func (v *URLValidator) Validate(ctx context.Context, value string) ValidationResult {
    result := ValidationResult{Valid: true, Errors: make([]error, 0)}
    
    urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
    if !urlRegex.MatchString(value) {
        result.Valid = false
        result.Errors = append(result.Errors, errors.NewValidationFailed(
            v.FieldName,
            "invalid URL format",
        ))
    }
    
    return result
}

func (v *URLValidator) Name() string {
    return "URLValidator"
}
EOF

# Test common validators
cat > pkg/mcp/domain/validation/common_test.go << 'EOF'
package validation

import (
    "context"
    "testing"
)

func TestStringLengthValidator(t *testing.T) {
    validator := NewStringLengthValidator("test_field", 5, 10)
    ctx := context.Background()
    
    // Test valid string
    result := validator.Validate(ctx, "hello")
    if !result.Valid {
        t.Errorf("Expected valid result for 'hello', got invalid")
    }
    
    // Test too short
    result = validator.Validate(ctx, "hi")
    if result.Valid {
        t.Errorf("Expected invalid result for 'hi', got valid")
    }
    
    // Test too long
    result = validator.Validate(ctx, "this is too long")
    if result.Valid {
        t.Errorf("Expected invalid result for long string, got valid")
    }
}

func TestValidatorChain(t *testing.T) {
    chain := NewValidatorChain[string](StopOnFirstError)
    chain.Add(NewRequiredValidator("test_field"))
    chain.Add(NewStringLengthValidator("test_field", 5, 10))
    
    ctx := context.Background()
    
    // Test valid chain
    result := chain.Validate(ctx, "hello")
    if !result.Valid {
        t.Errorf("Expected valid result for 'hello', got invalid")
    }
    
    // Test empty string (should fail required)
    result = chain.Validate(ctx, "")
    if result.Valid {
        t.Errorf("Expected invalid result for empty string, got valid")
    }
    
    // Test too short (should fail length after passing required)
    result = chain.Validate(ctx, "hi")
    if result.Valid {
        t.Errorf("Expected invalid result for 'hi', got valid")
    }
}
EOF

# Test validators compilation and functionality
go test ./pkg/mcp/domain/validation && echo "✅ Common validators working"
```

**Validation Commands**:
```bash
# Test validator functionality
go test ./pkg/mcp/domain/validation && echo "✅ Validators tested"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Common validators implemented
- [ ] Validator chaining working
- [ ] Tests passing
- [ ] Changes committed

#### Day 8: Validation Migration
**Morning Goals**:
- [ ] Migrate validation usage from old packages to unified system
- [ ] Update domain layer to use new validation
- [ ] Test validation integration
- [ ] Remove redundant validation code

**Validation Migration Commands**:
```bash
# Find all validation usage
grep -r "ValidateOptionalFields\|ValidateRequiredFields\|ValidationError" pkg/mcp/ > validation_usage.txt

# Create migration script
cat > scripts/migrate-validation.sh << 'EOF'
#!/bin/bash
echo "Migrating validation usage to unified system..."

# Replace old validation patterns
find pkg/mcp -name "*.go" -exec sed -i 's/ValidateOptionalFields/validation.NewValidatorChain/g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's/ValidateRequiredFields/validation.NewRequiredValidator/g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's/ValidationError/validation.ValidationResult/g' {} \;

# Update imports
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/common/validation|pkg/mcp/domain/validation|g' {} \;

echo "Validation migration complete"
EOF

chmod +x scripts/migrate-validation.sh

# Test migration
scripts/migrate-validation.sh && echo "✅ Validation migration script working"

# Update specific validation usage
find pkg/mcp -name "*.go" -exec grep -l "validation\." {} \; | head -5 | while read file; do
    echo "Updating $file..."
    # Manual updates for complex validation patterns would go here
done
```

**Validation Commands**:
```bash
# Test validation migration
go build ./pkg/mcp/domain/... && echo "✅ Domain validation migration successful"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Validation migration script created
- [ ] Domain layer updated
- [ ] Validation integration tested
- [ ] Changes committed

#### Day 9: Remove Deprecated Validation
**Morning Goals**:
- [ ] Remove `pkg/common/validation/unified_validator.go`
- [ ] Remove `pkg/common/validation-core/standard.go`
- [ ] Remove reflection-based validation
- [ ] Update all validation imports

**Deprecated Validation Removal Commands**:
```bash
# Remove deprecated validation files
rm pkg/common/validation/unified_validator.go
rm pkg/common/validation-core/standard.go

# Remove entire deprecated validation packages
rm -rf pkg/common/validation-core/core/interfaces.go # deprecated interfaces
rm -rf pkg/common/validation-core/validators/base.go # deprecated base validators

# Update imports throughout codebase
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/common/validation-core|pkg/mcp/domain/validation|g' {} \;
find pkg/mcp -name "*.go" -exec sed -i 's|pkg/common/validation|pkg/mcp/domain/validation|g' {} \;

# Remove reflection-based validation calls
find pkg/mcp -name "*.go" -exec sed -i '/reflect\..*validation/d' {} \;

# Verify no reflection usage in validation
! grep -r "reflect\." pkg/mcp/domain/validation/ && echo "✅ No reflection in validation"
```

**Validation Commands**:
```bash
# Verify deprecated validation removed
! test -f pkg/common/validation/unified_validator.go && echo "✅ Deprecated validation removed"

# Test unified validation system
go test ./pkg/mcp/domain/validation && echo "✅ Unified validation working"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Deprecated validation files removed
- [ ] Reflection usage eliminated
- [ ] Unified system working
- [ ] Changes committed

#### Day 10: Validation Integration Testing
**Morning Goals**:
- [ ] Test end-to-end validation with new unified system
- [ ] Validate performance of new validation system
- [ ] Test validation error propagation
- [ ] Create validation usage documentation

**Validation Integration Testing Commands**:
```bash
# Create comprehensive validation integration test
cat > pkg/mcp/domain/validation/integration_test.go << 'EOF'
package validation

import (
    "context"
    "testing"
    "time"
)

func TestValidationIntegration(t *testing.T) {
    // Create validation chain for user input
    userChain := NewValidatorChain[string](ContinueOnError)
    userChain.Add(NewRequiredValidator("username"))
    userChain.Add(NewStringLengthValidator("username", 3, 20))
    
    patternValidator, err := NewPatternValidator("username", "^[a-zA-Z0-9_]+$")
    if err != nil {
        t.Fatal(err)
    }
    userChain.Add(patternValidator)
    
    ctx := context.Background()
    
    // Test valid input
    result := userChain.Validate(ctx, "valid_user123")
    if !result.Valid {
        t.Errorf("Expected valid result for valid username, got errors: %v", result.Errors)
    }
    
    // Test invalid input (multiple errors)
    result = userChain.Validate(ctx, "a!")
    if result.Valid {
        t.Error("Expected invalid result for invalid username")
    }
    
    // Should have multiple errors (too short + invalid pattern)
    if len(result.Errors) < 2 {
        t.Errorf("Expected multiple errors, got %d", len(result.Errors))
    }
}

func TestValidationPerformance(t *testing.T) {
    // Create complex validation chain
    chain := NewValidatorChain[string](ContinueOnError)
    chain.Add(NewRequiredValidator("field"))
    chain.Add(NewStringLengthValidator("field", 1, 1000))
    
    patternValidator, _ := NewPatternValidator("field", "^[a-zA-Z0-9_.-]+$")
    chain.Add(patternValidator)
    chain.Add(NewEmailValidator("field"))
    
    ctx := context.Background()
    
    // Benchmark validation performance
    start := time.Now()
    for i := 0; i < 1000; i++ {
        chain.Validate(ctx, "test@example.com")
    }
    duration := time.Since(start)
    
    // Validation should be fast (< 1ms per validation)
    if duration > time.Millisecond*1000 {
        t.Errorf("Validation too slow: %v for 1000 validations", duration)
    }
}

func TestValidationErrorPropagation(t *testing.T) {
    validator := NewRequiredValidator("required_field")
    ctx := context.Background()
    
    result := validator.Validate(ctx, "")
    if result.Valid {
        t.Error("Expected validation to fail for empty required field")
    }
    
    if len(result.Errors) != 1 {
        t.Errorf("Expected 1 error, got %d", len(result.Errors))
    }
    
    // Test error is RichError
    err := result.Errors[0]
    if err == nil {
        t.Error("Expected error to be non-nil")
    }
    
    // Error should contain field context
    errorMsg := err.Error()
    if !strings.Contains(errorMsg, "required_field") {
        t.Errorf("Expected error to contain field name, got: %s", errorMsg)
    }
}
EOF

# Run integration tests
go test ./pkg/mcp/domain/validation -v && echo "✅ Validation integration tests passing"

# Test validation performance
go test -bench=. ./pkg/mcp/domain/validation && echo "✅ Validation performance acceptable"

# Create validation documentation
cat > pkg/mcp/domain/validation/README.md << 'EOF'
# Unified Validation System

## Overview
The unified validation system provides composable, type-safe validation with rich error reporting.

## Usage

### Basic Validation
```go
validator := validation.NewRequiredValidator("username")
result := validator.Validate(ctx, "test")
if !result.Valid {
    for _, err := range result.Errors {
        log.Error(err)
    }
}
```

### Validation Chains
```go
chain := validation.NewValidatorChain[string](validation.ContinueOnError)
chain.Add(validation.NewRequiredValidator("email"))
chain.Add(validation.NewEmailValidator("email"))

result := chain.Validate(ctx, "user@example.com")
```

### Custom Validators
```go
type CustomValidator struct {
    fieldName string
}

func (v *CustomValidator) Validate(ctx context.Context, value string) validation.ValidationResult {
    // Custom validation logic
    return validation.ValidationResult{Valid: true}
}

func (v *CustomValidator) Name() string {
    return "CustomValidator"
}
```

## Available Validators
- `RequiredValidator`: Validates required fields
- `StringLengthValidator`: Validates string length
- `PatternValidator`: Validates regex patterns
- `EmailValidator`: Validates email format
- `URLValidator`: Validates URL format

## Chain Strategies
- `StopOnFirstError`: Stop validation on first error
- `ContinueOnError`: Continue validation, collect all errors
- `StopOnFirstWarning`: Stop validation on first warning
EOF
```

**Validation Commands**:
```bash
# Test full validation system
go test ./pkg/mcp/domain/validation && echo "✅ Full validation system working"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Integration tests complete
- [ ] Performance validated
- [ ] Error propagation working
- [ ] Documentation complete
- [ ] Changes committed

### Week 5: Deprecated Code Removal

#### Day 11: High-Priority Service Deprecations
**Morning Goals**:
- [ ] Remove `pkg/mcp/application/services/retry.go` (deprecated)
- [ ] Remove `pkg/mcp/application/services/transport.go` (deprecated)
- [ ] Remove deprecated functions in `pkg/mcp/application/core/server.go`
- [ ] Update all callers to use new APIs

**High-Priority Deprecation Removal Commands**:
```bash
# Remove deprecated service files
rm pkg/mcp/application/services/retry.go
rm pkg/mcp/application/services/transport.go

# Remove deprecated server functions
sed -i '/Deprecated.*ServerService/,/^}/d' pkg/mcp/application/core/server.go
sed -i '/Deprecated.*TransportService/,/^}/d' pkg/mcp/application/core/server.go

# Find and update all callers
grep -r "services\.RetryCoordinator\|services\.Transport" pkg/mcp/ | cut -d: -f1 | sort -u > callers_to_update.txt

# Update callers to use new APIs
while read file; do
    if [ -f "$file" ]; then
        sed -i 's/services\.RetryCoordinator/api.RetryCoordinator/g' "$file"
        sed -i 's/services\.Transport/api.Transport/g' "$file"
        echo "Updated $file"
    fi
done < callers_to_update.txt

# Verify compilation after removals
go build ./pkg/mcp/application/... && echo "✅ Service deprecations removed successfully"
```

**Validation Commands**:
```bash
# Verify deprecated services removed
! test -f pkg/mcp/application/services/retry.go && echo "✅ Deprecated retry service removed"
! test -f pkg/mcp/application/services/transport.go && echo "✅ Deprecated transport service removed"

# Test service layer still works
go test ./pkg/mcp/application/services && echo "✅ Service layer working after cleanup"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Deprecated service files removed
- [ ] Server deprecations removed
- [ ] Callers updated to new APIs
- [ ] Changes committed

#### Day 12: Tool Registry Deprecations
**Morning Goals**:
- [ ] Remove deprecated `pkg/mcp/application/core/registry.go`
- [ ] Remove deprecated `KnownRegistries` from `pkg/mcp/application/core/types.go`
- [ ] Update all tool registry references
- [ ] Test tool registration still works

**Tool Registry Deprecation Commands**:
```bash
# Remove deprecated registry file
rm pkg/mcp/application/core/registry.go

# Remove deprecated KnownRegistries
sed -i '/KnownRegistries.*deprecated/,/^}/d' pkg/mcp/application/core/types.go

# Find all registry usage
grep -r "core\.Registry\|KnownRegistries" pkg/mcp/ | cut -d: -f1 | sort -u > registry_callers.txt

# Update registry callers
while read file; do
    if [ -f "$file" ]; then
        sed -i 's/core\.Registry/api.ToolRegistry/g' "$file"
        sed -i 's/KnownRegistries/registry.NewUnified()/g' "$file"
        echo "Updated registry usage in $file"
    fi
done < registry_callers.txt

# Verify tool registry functionality
go build ./pkg/mcp/application/core && echo "✅ Tool registry deprecations removed"
```

**Validation Commands**:
```bash
# Verify deprecated registry removed
! test -f pkg/mcp/application/core/registry.go && echo "✅ Deprecated registry removed"

# Test tool registration still works
go test ./pkg/mcp/application/core && echo "✅ Tool registration working"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Deprecated registry file removed
- [ ] KnownRegistries removed
- [ ] Registry callers updated
- [ ] Changes committed

#### Day 13: Validation System Deprecations
**Morning Goals**:
- [ ] Remove deprecated interfaces from `pkg/common/validation-core/core/interfaces.go`
- [ ] Remove deprecated functions from `pkg/common/validation-core/standard.go`
- [ ] Clean up remaining reflection-based validation
- [ ] Update validation imports

**Validation System Deprecation Commands**:
```bash
# Remove deprecated validation interfaces
sed -i '/Deprecated.*Use GenericValidator/,/^}/d' pkg/common/validation-core/core/interfaces.go
sed -i '/Deprecated.*Use TypedValidatorRegistry/,/^}/d' pkg/common/validation-core/core/interfaces.go
sed -i '/Deprecated.*Use TypedConditionalValidator/,/^}/d' pkg/common/validation-core/core/interfaces.go

# Remove deprecated validation functions
sed -i '/Deprecated.*Use ValidateOptionalFieldsGeneric/,/^}/d' pkg/common/validation-core/standard.go
sed -i '/Deprecated.*Use ValidateRequiredFieldsGeneric/,/^}/d' pkg/common/validation-core/standard.go
sed -i '/Deprecated.*Use core.NewFieldError/,/^}/d' pkg/common/validation-core/standard.go

# Remove entire deprecated standard.go file (marked as deprecated)
rm pkg/common/validation-core/standard.go

# Update remaining validation imports
find pkg/mcp -name "*.go" -exec grep -l "validation-core/core" {} \; | while read file; do
    sed -i 's|pkg/common/validation-core/core|pkg/mcp/domain/validation|g' "$file"
    echo "Updated validation imports in $file"
done

# Verify no reflection in validation
! grep -r "reflect\." pkg/mcp/domain/validation/ && echo "✅ No reflection in validation"
```

**Validation Commands**:
```bash
# Verify deprecated validation removed
! test -f pkg/common/validation-core/standard.go && echo "✅ Deprecated validation standard removed"

# Test validation system
go test ./pkg/mcp/domain/validation && echo "✅ Validation system working"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Deprecated validation interfaces removed
- [ ] Deprecated functions removed
- [ ] Reflection usage eliminated
- [ ] Changes committed

#### Day 14: Schema & Tool Deprecations
**Morning Goals**:
- [ ] Remove deprecated functions from `pkg/mcp/domain/tools/schema.go`
- [ ] Remove deprecated functions from `pkg/mcp/domain/tools/tool_validation.go`
- [ ] Update tool generation to use new functions
- [ ] Test tool schema generation

**Schema & Tool Deprecation Commands**:
```bash
# Remove deprecated schema functions (10 functions)
sed -i '/Deprecated.*Use ToMap/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use FromMap/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use GenerateSchemaAsMap/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use applyValidationConstraintsTyped/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use getJSONType/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use StringSchema/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use NumberSchema/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use ArraySchema/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use EnumSchema/,/^}/d' pkg/mcp/domain/tools/schema.go
sed -i '/Deprecated.*Use ObjectSchema/,/^}/d' pkg/mcp/domain/tools/schema.go

# Remove deprecated tool validation functions
sed -i '/Deprecated.*Use NewRichValidationError/,/^}/d' pkg/mcp/domain/tools/tool_validation.go
sed -i '/Deprecated.*Use NewRichValidationErrorWithCode/,/^}/d' pkg/mcp/domain/tools/tool_validation.go

# Update tool generation to use new functions
find pkg/mcp -name "*.go" -exec grep -l "GenerateSchema\|ValidationError" {} \; | while read file; do
    sed -i 's/GenerateSchema/GenerateSchemaAsMap/g' "$file"
    sed -i 's/ValidationError/RichValidationError/g' "$file"
    echo "Updated tool generation in $file"
done

# Test schema generation
go build ./pkg/mcp/domain/tools && echo "✅ Schema deprecations removed"
```

**Validation Commands**:
```bash
# Verify deprecated schema functions removed
! grep -r "Deprecated.*Use.*Schema" pkg/mcp/domain/tools/schema.go && echo "✅ Schema deprecations removed"

# Test tool validation
go test ./pkg/mcp/domain/tools && echo "✅ Tool validation working"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Deprecated schema functions removed (10 functions)
- [ ] Deprecated tool validation removed
- [ ] Tool generation updated
- [ ] Changes committed

#### Day 15: State & Workflow Deprecations
**Morning Goals**:
- [ ] Remove deprecated functions from `pkg/mcp/application/state/`
- [ ] Remove deprecated workflow engine functions
- [ ] Update state management to use new APIs
- [ ] Clean up remaining deprecated items

**State & Workflow Deprecation Commands**:
```bash
# Remove deprecated state functions
sed -i '/Deprecated.*Use services.ServiceContainer/,/^}/d' pkg/mcp/application/state/integration.go
sed -i '/Deprecated.*Use services.SessionStore/,/^}/d' pkg/mcp/application/state/context_enrichers.go
sed -i '/Deprecated.*Use services.SessionState/,/^}/d' pkg/mcp/application/state/context_enrichers.go

# Remove deprecated workflow functions
sed -i '/Deprecated.*Use NewSimpleWorkflowExecutor/,/^}/d' pkg/mcp/application/workflows/engine.go
sed -i '/DEPRECATED.*Use JobExecutionService/,/^}/d' pkg/mcp/application/workflows/job_execution_service.go
sed -i '/DEPRECATED.*Use JobExecutionConfig/,/^}/d' pkg/mcp/application/workflows/job_execution_service.go
sed -i '/DEPRECATED.*Use NewJobExecutionService/,/^}/d' pkg/mcp/application/workflows/job_execution_service.go

# Remove deprecated action field
sed -i '/Action.*Deprecated.*use Actions/d' pkg/mcp/application/state/state_types.go

# Update state management callers
find pkg/mcp -name "*.go" -exec grep -l "ServiceContainer\|SessionStore\|SessionState" {} \; | while read file; do
    sed -i 's/state\.ServiceContainer/services.ServiceContainer/g' "$file"
    sed -i 's/state\.SessionStore/services.SessionStore/g' "$file"
    sed -i 's/state\.SessionState/services.SessionState/g' "$file"
    echo "Updated state management in $file"
done

# Final deprecated count check
REMAINING_DEPRECATED=$(grep -r "Deprecated\|DEPRECATED" pkg/mcp/ pkg/common/ | wc -l)
echo "Remaining deprecated items: $REMAINING_DEPRECATED (target: 0)"
```

**Validation Commands**:
```bash
# Verify state deprecations removed
! grep -r "Deprecated.*Use services\." pkg/mcp/application/state/ && echo "✅ State deprecations removed"

# Test state management
go test ./pkg/mcp/application/state && echo "✅ State management working"

# Final deprecated count
REMAINING=$(grep -r "Deprecated\|DEPRECATED" pkg/mcp/ pkg/common/ | wc -l)
echo "Remaining deprecated items: $REMAINING"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] State management deprecations removed
- [ ] Workflow deprecations removed
- [ ] Remaining deprecated items cataloged
- [ ] Changes committed

### Week 6: Error System Integration & Completion

#### Day 16: Progressive Error Reduction
**Morning Goals**:
- [ ] Target domain layer for 100% RichError conversion
- [ ] Convert application layer high-priority errors
- [ ] Maintain <10 fmt.Errorf grandfathered limit
- [ ] Test error propagation through layers

**Progressive Error Reduction Commands**:
```bash
# Check current error counts
CURRENT_FMT_ERRORF=$(grep -r "fmt\.Errorf" pkg/mcp/ | wc -l)
echo "Current fmt.Errorf count: $CURRENT_FMT_ERRORF"

# Identify domain layer fmt.Errorf usage
grep -r "fmt\.Errorf" pkg/mcp/domain/ > domain_errors.txt
wc -l domain_errors.txt

# Convert domain layer errors to RichError
while read line; do
    file=$(echo "$line" | cut -d: -f1)
    line_num=$(echo "$line" | cut -d: -f2)
    
    if [ -f "$file" ]; then
        # Create backup
        cp "$file" "$file.backup"
        
        # Convert specific error patterns
        sed -i 's/fmt\.Errorf("missing %s"/errors.NewMissingParam(/g' "$file"
        sed -i 's/fmt\.Errorf("invalid %s"/errors.NewValidationFailed(/g' "$file"
        sed -i 's/fmt\.Errorf("failed to %s"/errors.NewInternalError(/g' "$file"
        
        echo "Converted errors in $file"
    fi
done < domain_errors.txt

# Test domain layer compilation
go build ./pkg/mcp/domain/... && echo "✅ Domain layer error conversion successful"

# Check new error count
NEW_FMT_ERRORF=$(grep -r "fmt\.Errorf" pkg/mcp/ | wc -l)
echo "New fmt.Errorf count: $NEW_FMT_ERRORF (target: <10)"
```

**Validation Commands**:
```bash
# Validate error reduction
scripts/check-error-patterns.sh 50 && echo "✅ Error reduction in progress"

# Test error propagation
go test ./pkg/mcp/domain/... && echo "✅ Domain error propagation working"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Domain layer errors converted to RichError
- [ ] Error count reduced significantly
- [ ] Error propagation tested
- [ ] Changes committed

#### Day 17: Application Layer Error Conversion
**Morning Goals**:
- [ ] Convert high-priority application layer errors
- [ ] Maintain error context and stack traces
- [ ] Test error handling in service layer
- [ ] Ensure JSON-RPC error mapping works

**Application Layer Error Conversion Commands**:
```bash
# Identify application layer error patterns
grep -r "fmt\.Errorf" pkg/mcp/application/ > application_errors.txt
head -20 application_errors.txt

# Convert application layer errors systematically
find pkg/mcp/application -name "*.go" -exec grep -l "fmt\.Errorf" {} \; | while read file; do
    # Convert common error patterns
    sed -i 's/fmt\.Errorf("tool not found: %s"/errors.NewNotFoundError("tool"/g' "$file"
    sed -i 's/fmt\.Errorf("configuration error: %s"/errors.NewConfigurationError("config"/g' "$file"
    sed -i 's/fmt\.Errorf("internal error: %s"/errors.NewInternalError("operation"/g' "$file"
    
    # Convert wrapped errors
    sed -i 's/fmt\.Errorf("failed to %s: %w"/errors.NewInternalError("operation", err)/g' "$file"
    
    echo "Converted application errors in $file"
done

# Test application layer compilation
go build ./pkg/mcp/application/... && echo "✅ Application layer error conversion successful"

# Test JSON-RPC error mapping
go test ./pkg/mcp/infra/transport/... && echo "✅ JSON-RPC error mapping working"

# Check error count progress
CURRENT_COUNT=$(grep -r "fmt\.Errorf" pkg/mcp/ | wc -l)
echo "Current fmt.Errorf count: $CURRENT_COUNT (target: <10)"
```

**Validation Commands**:
```bash
# Test application layer errors
go test ./pkg/mcp/application/... && echo "✅ Application layer error handling working"

# Validate error count
scripts/check-error-patterns.sh 20 && echo "✅ Error count approaching target"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Application layer errors converted
- [ ] Error context preserved
- [ ] JSON-RPC mapping tested
- [ ] Changes committed

#### Day 18: Final Error Cleanup
**Morning Goals**:
- [ ] Reach <10 fmt.Errorf grandfathered limit
- [ ] Document grandfathered errors and rationale
- [ ] Test complete error system integration
- [ ] Validate error performance

**Final Error Cleanup Commands**:
```bash
# Get final error count
FINAL_COUNT=$(grep -r "fmt\.Errorf" pkg/mcp/ | wc -l)
echo "Final fmt.Errorf count: $FINAL_COUNT"

# If count > 10, identify remaining errors
if [ $FINAL_COUNT -gt 10 ]; then
    echo "Identifying remaining errors for grandfathering..."
    grep -r "fmt\.Errorf" pkg/mcp/ | head -20 > remaining_errors.txt
    
    # Convert more errors or mark as grandfathered
    while read line; do
        file=$(echo "$line" | cut -d: -f1)
        context=$(echo "$line" | cut -d: -f3-)
        
        # Add grandfathering comment
        sed -i "s|fmt\.Errorf($context)|fmt.Errorf($context) // GRANDFATHERED: Hot path performance|g" "$file"
        echo "Grandfathered error in $file"
    done < remaining_errors.txt
fi

# Create grandfathered errors documentation
cat > docs/GRANDFATHERED_ERRORS.md << 'EOF'
# Grandfathered fmt.Errorf Usage

## Overview
These fmt.Errorf calls are grandfathered due to performance requirements in hot paths.

## Grandfathered Errors List
EOF

grep -r "GRANDFATHERED" pkg/mcp/ >> docs/GRANDFATHERED_ERRORS.md

# Test complete error system
go test ./pkg/mcp/... && echo "✅ Complete error system working"

# Performance test error handling
go test -bench=. ./pkg/mcp/domain/errors && echo "✅ Error performance acceptable"

# Final validation
scripts/check-error-patterns.sh 10 && echo "✅ Error reduction target achieved"
```

**Validation Commands**:
```bash
# Final error count validation
scripts/check-error-patterns.sh 10 && echo "✅ Error reduction complete"

# Test error system performance
go test -bench=. ./pkg/mcp/domain/errors && echo "✅ Error system performance acceptable"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] <10 fmt.Errorf calls remaining
- [ ] Grandfathered errors documented
- [ ] Error system performance validated
- [ ] Changes committed

#### Day 19: Error System Documentation
**Morning Goals**:
- [ ] Create comprehensive error handling guide
- [ ] Document error patterns and best practices
- [ ] Create error troubleshooting guide
- [ ] Update architectural documentation

**Error System Documentation Commands**:
```bash
# Create error handling guide
cat > docs/ERROR_HANDLING_GUIDE.md << 'EOF'
# Error Handling Guide

## Overview
Container Kit uses a unified RichError system for structured error handling with context, suggestions, and machine-readable codes.

## Error Construction

### Common Patterns
```go
// Missing parameter
return errors.NewMissingParam("fieldName")

// Validation error
return errors.NewValidationFailed("field", "reason")

// Internal error with cause
return errors.NewInternalError("operation", cause)

// Configuration error
return errors.NewConfigurationError("component", "issue")

// Not found error
return errors.NewNotFoundError("resource", "identifier")
```

### Custom Errors
```go
return errors.NewError().
    Code(errors.CodeCustom).
    Type(errors.ErrTypeCustom).
    Severity(errors.SeverityMedium).
    Message("custom error message").
    Context("key", "value").
    Suggestion("how to fix").
    WithLocation().
    Build()
```

## Error Handling

### Basic Handling
```go
if err != nil {
    // RichError provides structured information
    if richErr, ok := err.(errors.RichError); ok {
        log.Error("Error occurred", 
            "code", richErr.Code(),
            "type", richErr.Type(),
            "severity", richErr.Severity(),
            "context", richErr.Context(),
            "suggestion", richErr.Suggestion(),
        )
    }
    return err
}
```

### Error Wrapping
```go
if err != nil {
    return errors.NewInternalError("database operation", err)
}
```

## Best Practices

1. **Use RichError constructors** for all business logic errors
2. **Preserve error context** when wrapping errors
3. **Provide actionable suggestions** in error messages
4. **Use appropriate error codes** for machine processing
5. **Include relevant context** for debugging
6. **Document grandfathered fmt.Errorf** usage with rationale

## Error Codes

### Domain Error Codes
- `CodeValidationFailed`: Input validation failures
- `CodeNotFound`: Resource not found
- `CodeAlreadyExists`: Resource already exists
- `CodeInternalError`: Internal system errors
- `CodeConfigurationError`: Configuration issues

### Transport Error Codes
- `CodeTimeout`: Operation timeout
- `CodeUnauthorized`: Authentication required
- `CodeForbidden`: Access denied
- `CodeRateLimited`: Rate limit exceeded

## JSON-RPC Error Mapping

RichError automatically maps to JSON-RPC error format:
```json
{
  "code": -32000,
  "message": "validation failed for field: reason",
  "data": {
    "type": "validation",
    "severity": "medium",
    "context": {"field": "fieldName", "reason": "reason"},
    "suggestion": "Check field format",
    "location": "file.go:123"
  }
}
```

## Performance Considerations

- **Hot paths**: Use grandfathered fmt.Errorf for performance-critical code
- **Error caching**: RichError supports result caching for repeated operations
- **Context minimization**: Only include essential context in error construction
- **Lazy evaluation**: Error details computed only when accessed

## Troubleshooting

### Common Issues

1. **Error not RichError**: Check if error constructor is used
2. **Missing context**: Ensure Context() calls are included
3. **Poor performance**: Consider grandfathering for hot paths
4. **Lost error cause**: Use proper error wrapping patterns

### Debugging Tools
```bash
# Check error pattern usage
scripts/check-error-patterns.sh 10

# Find fmt.Errorf usage
grep -r "fmt\.Errorf" pkg/mcp/

# Test error propagation
go test ./pkg/mcp/domain/errors -v
```
EOF

# Update architectural documentation
echo "## Error System" >> docs/THREE_LAYER_ARCHITECTURE.md
echo "- **Domain Layer**: Pure RichError usage" >> docs/THREE_LAYER_ARCHITECTURE.md
echo "- **Application Layer**: Error orchestration and context" >> docs/THREE_LAYER_ARCHITECTURE.md
echo "- **Infrastructure Layer**: Error transport and mapping" >> docs/THREE_LAYER_ARCHITECTURE.md
```

**Validation Commands**:
```bash
# Verify documentation created
test -f docs/ERROR_HANDLING_GUIDE.md && echo "✅ Error handling guide created"

# Pre-commit validation
/usr/bin/make pre-commit
```

**End of Day Checklist**:
- [ ] Error handling guide created
- [ ] Best practices documented
- [ ] Troubleshooting guide complete
- [ ] Changes committed

#### Day 20: CHECKPOINT - Error System Complete
**Morning Goals**:
- [ ] **CRITICAL**: Final validation of all error system deliverables
- [ ] Comprehensive error system testing
- [ ] Create completion status report
- [ ] Notify other workstreams of completion

**Final Error System Validation Commands**:
```bash
# Complete GAMMA validation
echo "=== GAMMA WORKSTREAM FINAL VALIDATION ==="
echo "Error standardization: $(scripts/check-error-patterns.sh 10 && echo "COMPLETE" || echo "INCOMPLETE")"
echo "Validation consolidation: $(find pkg -name "*validat*" -type f | wc -l) packages (target: 1 unified)"
echo "Deprecated code removal: $(grep -r "Deprecated\|DEPRECATED" pkg/mcp/ pkg/common/ | wc -l) items (target: 0)"
echo "RichError adoption: $(grep -r "RichError\|NewError" pkg/mcp/ | wc -l) instances"

# Test complete error system
go test ./pkg/mcp/domain/errors -v && echo "✅ Error system tests passing"
go test ./pkg/mcp/domain/validation -v && echo "✅ Validation system tests passing"

# Performance validation
go test -bench=. ./pkg/mcp/domain/errors && echo "✅ Error performance meets targets"
go test -bench=. ./pkg/mcp/domain/validation && echo "✅ Validation performance meets targets"

# Integration testing
go test ./pkg/mcp/... && echo "✅ Full integration tests passing"

# Final commit
git commit -m "feat(errors): complete error system and validation consolidation

- Reduced fmt.Errorf usage from 622 to <10 instances
- Consolidated 4 validation packages into unified system
- Removed 72 deprecated items from codebase
- Implemented comprehensive RichError system with helpers
- Added structured error context and suggestions
- Centralized JSON-RPC error mapping
- Maintained error handling performance benchmarks
- 100% RichError adoption in domain layer

ENABLES: Clean error patterns for all workstreams

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"

echo "🎉 GAMMA ERROR SYSTEM & VALIDATION WORKSTREAM COMPLETE"
```

**End of Day Checklist**:
- [ ] **CRITICAL**: All deliverables validated
- [ ] Error system integration complete
- [ ] Validation system unified
- [ ] Deprecated code eliminated
- [ ] Workstream completion celebrated

## 🔧 Technical Guidelines

### Required Tools/Setup
- **Error Linting**: Custom linter at `tools/linters/richerror-boundary/`
- **Go Generics**: Validation system uses generic types
- **Context Propagation**: All validation accepts context.Context
- **Make**: Set up alias `alias make='/usr/bin/make'`

### Coding Standards
- **Error Handling**: Use RichError for all business logic errors
- **Validation**: Use unified validation system with chains
- **Context**: All validation and error functions accept context
- **Performance**: Grandfathered fmt.Errorf allowed for hot paths only

### Testing Requirements
- **Error Tests**: All error constructors must have tests
- **Validation Tests**: All validators must have unit tests
- **Performance Tests**: Error and validation performance benchmarks
- **Integration Tests**: End-to-end error propagation tests

## 🤝 Coordination Points

### Dependencies FROM Other Workstreams
| Workstream | What You Need | When | Contact |
|------------|---------------|------|---------|
| ALPHA | Package structure stable | Day 1 | @alpha-lead |
| BETA | Service interfaces | Day 11 | @beta-lead |

### Dependencies TO Other Workstreams  
| Workstream | What They Need | When | Format |
|------------|----------------|------|--------|
| DELTA | Error patterns established | Day 16 | Error usage examples |
| BETA | Validation interfaces | Day 6 | Interface coordination |
| ALL | Deprecated code removed | Day 20 | Cleanup completion |

## 📊 Progress Tracking

### Daily Status Template
```markdown
## WORKSTREAM GAMMA - Day X Status

### Completed Today:
- [Error/validation achievement with metrics]
- [Deprecated code removal count]

### Blockers:
- [Any integration issues]

### Metrics:
- fmt.Errorf usage: [count] (target: <10)
- Validation packages: [count] (target: 1)
- Deprecated items: [count] (target: 0)
- RichError adoption: [percentage] (target: 100% domain)

### Tomorrow's Focus:
- [Next error conversion priority]
- [Validation migration task]
```

### Key Commands
```bash
# Morning setup
alias make='/usr/bin/make'
git checkout gamma-error-validation
git pull origin gamma-error-validation

# Error validation
scripts/check-error-patterns.sh 10
grep -r "fmt\.Errorf" pkg/mcp/ | wc -l

# Validation testing
go test ./pkg/mcp/domain/validation
go test -bench=. ./pkg/mcp/domain/validation

# Deprecated code tracking
grep -r "Deprecated\|DEPRECATED" pkg/mcp/ pkg/common/ | wc -l

# End of day
/usr/bin/make test-all
/usr/bin/make pre-commit
```

## 🚨 Common Issues & Solutions

### Issue 1: Error conversion breaks compilation
**Symptoms**: Build failures after fmt.Errorf conversion
**Solution**: Ensure proper error constructor usage
```bash
# Check error constructor parameters
go build ./pkg/mcp/domain/errors
# Use correct constructor for error type
errors.NewValidationFailed("field", "reason")
```

### Issue 2: Validation performance regression
**Symptoms**: Slow validation in benchmarks
**Solution**: Optimize validator chains and caching
```bash
# Profile validation performance
go test -bench=. -cpuprofile=validation.prof ./pkg/mcp/domain/validation
# Optimize hot path validators
```

### Issue 3: Deprecated code still referenced
**Symptoms**: Compilation errors after deprecation removal
**Solution**: Systematic caller updates
```bash
# Find all callers of deprecated function
grep -r "DeprecatedFunction" pkg/mcp/
# Update callers to use new API
```

## 📞 Escalation Path

1. **Error System Issues**: @error-expert (immediate Slack)
2. **Validation Performance**: @performance-expert (coordinate optimization)
3. **Deprecated Code Dependencies**: @architecture-lead (dependency resolution)
4. **Integration Issues**: @integration-lead (cross-workstream coordination)

## ✅ Definition of Done

Your workstream is complete when:
- [ ] fmt.Errorf usage reduced to <10 grandfathered instances
- [ ] Single unified validation system implemented
- [ ] All 72 deprecated items removed from codebase
- [ ] 100% RichError adoption in domain layer
- [ ] Comprehensive error handling documentation
- [ ] All tests passing with performance benchmarks met
- [ ] Error linting rules enforced in CI
- [ ] JSON-RPC error mapping centralized

## 📚 Resources

- [RichError Documentation](./pkg/mcp/domain/errors/README.md)
- [Validation System Guide](./pkg/mcp/domain/validation/README.md)
- [Error Handling Best Practices](./docs/ERROR_HANDLING_GUIDE.md)
- [Go Error Handling Patterns](https://go.dev/blog/go1.13-errors)
- [Team Slack Channel](#container-kit-refactor)

---

**Remember**: Your error system work creates the foundation for consistent error handling across all workstreams. Quality is critical - take time to ensure error messages are helpful and validation is robust. The deprecated code removal is extensive but necessary for reducing technical debt.