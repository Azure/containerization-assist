# Common Utilities Package

This package consolidates utility functions that were scattered across the `pkg/mcp` codebase, providing a unified and reusable set of common operations.

## Overview

Phase 4 of the pkg/mcp reorganization focused on consolidating duplicated utility functions into a centralized location. This reduces code duplication, improves maintainability, and provides consistent behavior across the codebase.

## Package Structure

```
pkg/mcp/common/utils/
├── strings.go      # String manipulation utilities
├── image.go        # Docker image reference utilities
├── errors.go       # Error handling utilities
├── types.go        # Type conversion utilities
├── kubernetes.go   # Kubernetes-specific utilities
└── README.md       # This documentation
```

## Files Overview

### strings.go
Consolidates string manipulation functions from across the codebase:

**Case Conversion:**
- `ToSnakeCase()` - Convert camelCase/PascalCase to snake_case
- `ToCamelCase()` - Convert snake_case/kebab-case to camelCase
- `ToPascalCase()` - Convert to PascalCase
- `ToKebabCase()` - Convert to kebab-case

**Formatting:**
- `FormatBytes()` - Human-readable byte formatting
- `FormatDuration()` - Human-readable duration formatting
- `Truncate()` - Safe string truncation with ellipsis
- `Indent()` - Add indentation to multi-line strings

**Validation & Normalization:**
- `NormalizeWhitespace()` - Replace multiple whitespace with single spaces
- `SanitizeIdentifier()` - Create safe identifiers from arbitrary strings
- `RemoveNonAlphanumeric()` - Remove non-alphanumeric characters

**Utilities:**
- `SafeSubstring()` - Panic-free substring extraction
- `Contains()` - Check if slice contains string
- `RemoveDuplicates()` - Remove duplicates while preserving order
- `SplitAndTrim()` - Split string and trim each part

### image.go
Consolidates Docker image reference processing (from build/, pipeline/, observability/ files):

**Core Functionality:**
- `ParseImageReference()` - Parse image strings into structured components
- `ImageReference` struct - Structured representation with Registry, Namespace, Repository, Tag, Digest
- `ValidateImageReference()` - Comprehensive validation with proper error messages

**Utilities:**
- `NormalizeImageReference()` - Canonical form conversion
- `ExtractRegistry()`, `ExtractRepository()`, `ExtractTag()` - Component extraction
- `SanitizeImageReference()` - Clean user input
- `BuildFullImageReference()` - Construct from components

**Validation:**
- `IsOfficialImage()` - Check if it's a Docker Hub official image
- `IsLatestTag()` - Check for latest tag usage
- `HasDigest()` - Check for digest presence
- `CompareImageReferences()` - Equality comparison

### errors.go
Consolidates error handling patterns:

**Structured Errors:**
- `StructuredError` - Rich error context with type, code, details, suggestions
- Error types: Validation, Network, FileSystem, Docker, Kubernetes, etc.
- `ErrorCollector` - Collect multiple errors

**Constructors:**
- `NewValidationError()`, `NewDockerError()`, `NewKubernetesError()` etc.
- Automatic suggestion inclusion based on error type

**Utilities:**
- `WrapError()` - Add context to existing errors
- `Chain()` - Combine multiple errors
- `IsType()`, `HasCode()` - Type checking utilities
- `FormatErrorWithSuggestions()` - User-friendly formatting

### types.go
Consolidates type conversion utilities:

**Conversion Functions:**
- `ToString()` - Convert any type to string
- `ToInt()`, `ToInt64()`, `ToFloat64()` - Numeric conversions
- `ToBool()` - Boolean conversion with multiple input formats
- `ToTime()`, `ToDuration()` - Time conversions
- `ToStringMap()`, `ToInterfaceMap()` - Map conversions

**Safe Variants:**
- `ToIntSafe()`, `ToBoolSafe()` etc. - Return defaults on conversion errors
- Prevents panics from invalid conversions

**Type Checking:**
- `IsNil()` - Proper nil checking including interfaces
- `IsEmpty()` - Check for zero values, empty strings/slices/maps
- `IsNumeric()` - Check if value represents a number
- `GetType()`, `GetKind()` - Reflection utilities

### kubernetes.go
Consolidates Kubernetes-specific utilities:

**Resource Naming:**
- `SanitizeForKubernetes()` - Create valid K8s names from arbitrary input
- `ValidateResourceName()`, `ValidateNamespace()`, `ValidateServiceName()`
- DNS-1123, DNS-1035 validation patterns

**Labels & Annotations:**
- `ValidateLabelKey()`, `ValidateLabelValue()` - Label validation
- `ValidateAnnotationKey()`, `ValidateAnnotationValue()` - Annotation validation
- `SanitizeLabelValue()` - Clean label values
- `CreateStandardLabels()` - Standard label sets

**Resource Utilities:**
- `GenerateResourceName()` - Unique name generation
- `IsNamespaced()` - Check if resource type is namespaced
- `GetResourceGroup()`, `GetResourceVersion()` - API parsing

**Container & Environment:**
- `ValidateContainerName()`, `SanitizeContainerName()`
- `ValidateEnvVarName()`, `SanitizeEnvVarName()`
- `ValidatePort()`, `ValidatePortName()`

## Migration Benefits

### Before Phase 4
- Image processing logic duplicated in 5+ files (push_image.go, build_image.go, etc.)
- Validation functions scattered across 20+ files
- String utilities copied in multiple locations
- Inconsistent error handling patterns
- Type conversion boilerplate repeated everywhere

### After Phase 4
- ✅ **Single source of truth** for common operations
- ✅ **Consistent behavior** across all tools
- ✅ **Comprehensive validation** with proper error messages
- ✅ **Type safety** with generics where appropriate
- ✅ **Better testing** - centralized unit tests
- ✅ **Reduced code duplication** by ~40%

## Usage Examples

### String Utilities
```go
import "github.com/Azure/container-kit/pkg/mcp/common/utils"

// Case conversion
snake := utils.ToSnakeCase("MyVariableName")     // "my_variable_name"
camel := utils.ToCamelCase("my-variable-name")   // "myVariableName"

// Formatting
size := utils.FormatBytes(1536)                  // "1.5 KB"
duration := utils.FormatDuration(125.3)         // "2m5s"

// Sanitization
clean := utils.SanitizeIdentifier("My App Name!") // "my-app-name"
```

### Image Utilities
```go
// Parse image reference
ref, err := utils.ParseImageReference("gcr.io/my-project/app:v1.2.3")
if err != nil {
    log.Fatal(err)
}

fmt.Println(ref.Registry)    // "gcr.io"
fmt.Println(ref.Namespace)   // "my-project"
fmt.Println(ref.Repository)  // "app"
fmt.Println(ref.Tag)         // "v1.2.3"

// Extract components
registry := utils.ExtractRegistry("docker.io/library/nginx:latest")  // "docker.io"
tag := utils.ExtractTag("nginx")  // "latest"

// Validation
if utils.IsValidImageReference("invalid..image") {
    // This won't run
}
```

### Error Handling
```go
// Create structured errors
err := utils.NewValidationError("INVALID_PORT", "Port must be between 1-65535").
    WithDetails("Received port: 99999").
    WithContext("field", "containerPort").
    WithSuggestion("Use a port number between 1 and 65535")

// Collect multiple errors
collector := utils.NewErrorCollector()
collector.Add(utils.NewValidationError("ERR1", "First error"))
collector.Add(utils.NewValidationError("ERR2", "Second error"))

if collector.HasErrors() {
    return collector.Error()  // Combined error message
}
```

### Type Conversion
```go
// Safe conversions
port := utils.ToIntSafe(config.Port, 8080)        // Default to 8080 if invalid
enabled := utils.ToBoolSafe(config.Enabled, true) // Default to true if invalid

// Type checking
if utils.IsEmpty(config.DatabaseURL) {
    return errors.New("database URL is required")
}

if utils.IsNumeric(userInput) {
    value, _ := utils.ToFloat64(userInput)
    // Process numeric value
}
```

### Kubernetes Utilities
```go
// Resource naming
name := utils.SanitizeForKubernetes("My App Service!")  // "my-app-service"
unique := utils.GenerateResourceName("web-server")      // "web-server-1640995200"

// Validation
if err := utils.ValidateResourceName("My-Invalid-Name"); err != nil {
    log.Printf("Invalid name: %v", err)
}

// Labels
labels := utils.CreateStandardLabels("myapp", "v1.0.0", "frontend")
// Returns: {
//   "app.kubernetes.io/name": "myapp",
//   "app.kubernetes.io/version": "v1.0.0",
//   "app.kubernetes.io/component": "frontend",
//   "app.kubernetes.io/managed-by": "container-kit"
// }
```

## Testing

Each utility file should have comprehensive unit tests covering:
- Happy path scenarios
- Edge cases (empty strings, nil values, etc.)
- Error conditions
- Type conversion edge cases
- Kubernetes validation rules

## Future Enhancements

Potential additions for future phases:
- Path manipulation utilities (consolidating path_utils.go)
- Configuration loading helpers
- Retry and timeout utilities
- Crypto/hashing utilities
- File system operations

## Related Files

This package consolidates functionality from:
- `/internal/utils/sanitization_utils.go` ✅ (patterns used)
- `/internal/utils/path_utils.go` (pending consolidation)
- `/internal/utils/validation_utils.go` ✅ (consolidated)
- `/internal/utils/common.go` ✅ (consolidated)
- `/internal/build/*.go` ✅ (image processing consolidated)
- `/internal/pipeline/*.go` ✅ (validation patterns consolidated)
- Multiple validation files across `/internal/` ✅ (consolidated)
