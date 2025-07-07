# Security Validator Refactoring Plan

## Current State
- **File**: security/validator.go
- **Lines**: 891 (Target: <500)
- **Methods**: 28
- **Responsibilities**: Mixed - security validation, secret scanning, threat assessment, policy engine, error sanitization

## Proposed Decomposition

### 1. **security_validator.go** (~200 lines)
Core security validation orchestration
```go
type SecurityValidator struct {
    threatAssessor  ThreatAssessor
    policyEngine    PolicyEngine
    secretScanner   SecretScanner
    errorSanitizer  ErrorSanitizer
}

// Core methods:
- NewSecurityValidator()
- ValidateSecurityContext()
- GetValidationReport()
```

### 2. **threat_assessor.go** (~150 lines)
Threat assessment and modeling
```go
type ThreatAssessor struct {
    threatModel *ThreatModel
}

// Methods:
- assessThreats()
- operationMatchesThreat()
- calculateRiskScore()
- initializeDefaultThreatModel()
```

### 3. **policy_engine.go** (~150 lines)
Security policy validation
```go
type PolicyEngine struct {
    policies map[string]SecurityPolicy
}

// Methods:
- NewSecurityPolicyEngine()
- ValidateOperation()
- checkRule()
- initializeDefaultPolicies()
```

### 4. **secret_scanner.go** (~200 lines)
Secret scanning functionality
```go
type SecretScanner struct {
    patterns map[string]*regexp.Regexp
}

// Methods:
- NewSecretScanner()
- ScanForSecrets()
- ScanEnvironment()
- ScanContent()
- scanFileContent()
- calculateConfidence()
- calculateSeverity()
```

### 5. **secret_externalizer.go** (~100 lines)
Secret externalization planning
```go
type SecretExternalizer struct {
    managers []SecretManager
}

// Methods:
- CreateExternalizationPlan()
- GetSecretManagers()
- ValidateSecretManager()
```

### 6. **error_sanitizer.go** (~50 lines)
Error message sanitization
```go
type ErrorSanitizer struct {
    patterns []SanitizationPattern
}

// Methods:
- SanitizeErrorMessage()
- RemoveSensitiveData()
```

### 7. **types.go** (~40 lines)
Shared types and structures
- VulnerabilityInfo
- ThreatInfo
- SecurityPolicy
- SecurityRule
- etc.

## Migration Strategy

### Phase 1: Create New Files (Backward Compatible)
1. Create all new files with focused responsibilities
2. Move types to types.go
3. Keep original validator.go as facade/adapter

### Phase 2: Update Dependencies
1. Update imports in dependent files
2. Add deprecation notices to old methods
3. Create migration guide

### Phase 3: Remove Legacy Code
1. Remove validator.go after all dependencies updated
2. Update tests to use new structure
3. Verify no functionality lost

## Benefits
- **Single Responsibility**: Each component has one clear purpose
- **Testability**: Easier to mock and test individual components
- **Maintainability**: Smaller, focused files are easier to understand
- **Flexibility**: Components can be used independently
- **Performance**: Better code organization may lead to optimization opportunities

## Validation
```bash
# Before refactoring
wc -l pkg/mcp/security/validator.go  # 891 lines

# After refactoring
wc -l pkg/mcp/security/*_validator.go pkg/mcp/security/*_scanner.go pkg/mcp/security/*_engine.go
# Each file should be <200 lines

# Verify functionality maintained
go test ./pkg/mcp/security/...
```
