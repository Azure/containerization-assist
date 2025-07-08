# Workstream 5: Fix Test Implementation Issues

## Objective
Fix all test-related compilation and implementation issues identified in the pre-commit checks. These are issues specific to test files that prevent the test suite from running.

## Scope
Focus exclusively on fixing test compilation errors, missing test functions, and test setup issues without changing production code.

## Affected Areas and Issues

### 1. Missing Test Helper Functions

**Files**: `pkg/mcp/tools/scan/`

**Missing functions that tests are trying to call**:
- `newAtomicScanSecretsToolImpl` - Used in scan_secrets_tool_test.go
- `NewSecurityScanToolWithMocks` - Used in security_scan_tool_test.go  
- `calculateRiskScore` - Used in security_scan_tool_test.go
- `calculateRiskLevel` - Used in security_scan_tool_test.go
- `securityScanToolImpl` - Used in security_scan_tool_test.go

### 2. Test Function Implementation Issues

**File**: `pkg/mcp/internal/pipeline/interface_implementations_test.go`

**Issues**:
- Line 299: `invalid argument: result.SecretsFound (variable of type int) for built-in len` 
  - Using `len()` on an integer instead of a slice
- Incorrect usage patterns in test assertions

### 3. Test Configuration and Setup Issues

**File**: `pkg/mcp/internal/pipeline/integration_test.go`

**Issues**:
- Line 113: `config.WorkerConfig is not a type`
  - Test trying to use undefined configuration type

### 4. Mock and Test Data Issues

**Various test files** have issues with:
- Missing mock implementations
- Incorrect test data structures
- Test helper functions that don't exist

### 5. Test Import Issues

**Common patterns**:
- Tests importing packages that have been moved or renamed
- Missing imports for testing utilities
- Import cycle issues specific to test files

## Instructions

1. **Create Missing Test Helper Functions**:
   - Implement stub/mock versions of missing functions
   - Ensure they return appropriate test data
   - Focus on making tests compile and run, not full implementation

2. **Fix Test Logic Issues**:
   - Replace incorrect usage patterns (like `len()` on integers)
   - Fix test assertions to use correct types
   - Ensure test expectations match actual struct fields

3. **Create Missing Test Configurations**:
   - Define missing configuration types used by tests
   - Create test-specific configuration structs if needed
   - Ensure test setup functions exist and work

4. **Implement Missing Mocks**:
   - Create mock implementations for interfaces used in tests
   - Ensure mocks satisfy the required interfaces
   - Provide reasonable default behavior for test scenarios

5. **Fix Test Imports**:
   - Add missing testing utility imports
   - Fix import paths for moved packages
   - Resolve any test-specific import cycles

## Success Criteria
- All test files compile without errors
- Test functions can be executed (even if they fail, they should run)
- All missing test helper functions are implemented
- Mock implementations satisfy their interfaces
- Test imports are resolved

## Example Patterns

### Missing Test Helper Implementation
```go
// In scan_secrets_tool_test.go or test helpers file
func newAtomicScanSecretsToolImpl(args ...interface{}) *AtomicScanSecretsTool {
    // Return a test implementation
    return &AtomicScanSecretsTool{
        // Initialize with test-appropriate defaults
        logger: slog.Default(),
    }
}

func NewSecurityScanToolWithMocks(mocks ...interface{}) *SecurityScanTool {
    // Return a mock implementation for testing
    return &SecurityScanTool{
        // Mock dependencies
    }
}

func calculateRiskScore(vulnerabilities []Vulnerability) float64 {
    // Simple test implementation
    return float64(len(vulnerabilities)) * 0.5
}
```

### Fixing Test Logic Issues
```go
// Before (incorrect)
assert.Equal(t, 3, len(result.SecretsFound)) // SecretsFound is int, not slice

// After (correct)  
assert.Equal(t, 3, result.SecretsFound) // Compare integers directly
// OR if you need to check a slice:
assert.Equal(t, 3, len(result.Secrets)) // Use the actual slice field
```

### Creating Test Configuration Types
```go
// In test helpers or config package
type WorkerConfig struct {
    MaxWorkers  int           `json:"max_workers"`
    Timeout     time.Duration `json:"timeout"`
    BufferSize  int           `json:"buffer_size"`
}

// Default configuration for tests
func DefaultTestWorkerConfig() *WorkerConfig {
    return &WorkerConfig{
        MaxWorkers: 2,
        Timeout:    30 * time.Second,
        BufferSize: 100,
    }
}
```

### Mock Interface Implementation
```go
// Create mock that satisfies the interface
type MockSecurityScanner struct {
    ScanResults []Vulnerability
    ScanError   error
}

func (m *MockSecurityScanner) Scan(image string) ([]Vulnerability, error) {
    return m.ScanResults, m.ScanError
}

func (m *MockSecurityScanner) GetRiskScore(vulns []Vulnerability) float64 {
    return calculateRiskScore(vulns)
}
```

## Test-Specific Dependencies

### Key Testing Packages to Import
```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
)
```

### Common Test Setup Pattern
```go
func TestSomething(t *testing.T) {
    // Setup
    config := DefaultTestConfig()
    mockDeps := setupMockDependencies()
    
    // Create system under test
    sut := NewSystemUnderTest(config, mockDeps)
    
    // Execute
    result, err := sut.DoSomething()
    
    // Assert
    require.NoError(t, err)
    assert.NotNil(t, result)
}
```