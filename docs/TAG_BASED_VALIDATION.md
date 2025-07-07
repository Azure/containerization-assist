# Tag-Based Validation System

## Overview

The tag-based validation system provides a declarative, consistent way to validate struct fields across the Container Kit codebase. Instead of writing manual validation logic, developers can simply add validation tags to struct fields, and the system automatically handles validation.

## Benefits

1. **Reduced Code Duplication**: No need to write repetitive validation logic
2. **Consistent Validation**: All tools use the same validation rules
3. **Declarative Approach**: Validation rules are clearly visible in struct definitions
4. **Extensible**: Easy to add new validation rules
5. **Better Error Messages**: Standardized error reporting across all tools
6. **Type Safety**: Validation rules are checked at runtime but declared at compile time

## How to Use

### Basic Usage

```go
type MyToolArgs struct {
    SessionID string `validate:"required,session_id"`
    ImageRef  string `validate:"required,docker_image"`
    Timeout   int    `validate:"omitempty,min=30,max=3600"`
}

// In your tool's Validate method:
func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    return validation.ValidateTaggedStruct(args)
}
```

### Validation Tags

Tags are specified in the `validate` struct tag, with multiple rules separated by commas:

```go
Field string `validate:"required,min=3,max=50"`
```

## Available Validation Tags

### Basic Validation

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Field must not be empty/zero | `validate:"required"` |
| `omitempty` | Skip validation if field is empty | `validate:"omitempty,min=5"` |
| `min=N` | Minimum value/length | `validate:"min=10"` |
| `max=N` | Maximum value/length | `validate:"max=100"` |
| `len=N` | Exact length | `validate:"len=16"` |
| `oneof=X Y Z` | Value must be one of the listed options | `validate:"oneof=dev staging prod"` |

### Container Kit Specific Tags

#### Git/Repository Tags
| Tag | Description | Example |
|-----|-------------|---------|
| `git_url` | Valid Git repository URL | `validate:"git_url"` |
| `git_branch` | Valid Git branch name | `validate:"git_branch"` |

#### Docker/Container Tags
| Tag | Description | Example |
|-----|-------------|---------|
| `docker_image` | Valid Docker image reference | `validate:"docker_image"` |
| `docker_tag` | Valid Docker tag | `validate:"docker_tag"` |
| `platform` | Valid platform (linux/amd64, etc.) | `validate:"platform"` |
| `registry_url` | Valid container registry URL | `validate:"registry_url"` |

#### Kubernetes Tags
| Tag | Description | Example |
|-----|-------------|---------|
| `k8s_name` | Valid Kubernetes resource name | `validate:"k8s_name"` |
| `namespace` | Valid Kubernetes namespace | `validate:"namespace"` |
| `k8s_label` | Valid Kubernetes label | `validate:"k8s_label"` |
| `k8s_selector` | Valid Kubernetes selector | `validate:"k8s_selector"` |
| `resource_spec` | Valid resource specification | `validate:"resource_spec"` |

#### Security Tags
| Tag | Description | Example |
|-----|-------------|---------|
| `session_id` | Valid UUID session ID | `validate:"session_id"` |
| `no_sensitive` | No sensitive data patterns | `validate:"no_sensitive"` |
| `secure_path` | Secure file path (no traversal) | `validate:"secure_path"` |
| `no_injection` | No injection patterns | `validate:"no_injection"` |

#### Domain-Specific Tags
| Tag | Description | Example |
|-----|-------------|---------|
| `language` | Valid programming language | `validate:"language"` |
| `framework` | Valid framework name | `validate:"framework"` |
| `severity` | Valid severity level | `validate:"severity"` |
| `vuln_type` | Valid vulnerability type | `validate:"vuln_type"` |
| `file_pattern` | Valid file glob pattern | `validate:"file_pattern"` |
| `domain` | Valid domain name | `validate:"domain"` |
| `port` | Valid port number (1-65535) | `validate:"port"` |
| `url` | Valid URL | `validate:"url"` |
| `endpoint` | Valid HTTP endpoint | `validate:"endpoint"` |

### Collection Validation

For slices and arrays, use the `dive` tag to validate each element:

```go
type Args struct {
    // Validates that VulnTypes is not empty, then validates each element
    VulnTypes []string `validate:"required,dive,vuln_type"`

    // Optional slice, but if present, each element must be a valid file pattern
    FilePatterns []string `validate:"omitempty,dive,file_pattern"`
}
```

## Examples

### Analyze Tool Arguments

```go
type AtomicAnalyzeRepositoryArgs struct {
    BaseToolArgs
    SessionID   string   `validate:"omitempty,session_id"`
    RepoURL     string   `validate:"required,git_url"`
    Branch      string   `validate:"omitempty,git_branch"`
    Context     string   `validate:"omitempty,max=1000"`
    Languages   []string `validate:"omitempty,dive,language"`
}
```

### Build Tool Arguments

```go
type AtomicBuildImageArgs struct {
    BaseToolArgs
    SessionID    string `validate:"required,session_id"`
    DockerFile   string `validate:"required,secure_path"`
    ImageRef     string `validate:"required,docker_image"`
    Platform     string `validate:"omitempty,platform"`
    BuildTimeout int    `validate:"omitempty,min=30,max=3600"`
}
```

### Deploy Tool Arguments

```go
type GenerateManifestsRequest struct {
    SessionID     string `validate:"required,session_id"`
    ImageRef      string `validate:"required,docker_image"`
    AppName       string `validate:"required,k8s_name"`
    Namespace     string `validate:"omitempty,namespace"`
    CPURequest    string `validate:"omitempty,resource_spec"`
    MemoryRequest string `validate:"omitempty,resource_spec"`
    IngressHost   string `validate:"omitempty,domain"`
}
```

### Scan Tool Arguments

```go
type AtomicScanImageSecurityArgs struct {
    BaseToolArgs
    ImageName    string   `validate:"required,docker_image"`
    Severity     string   `validate:"omitempty,severity"`
    VulnTypes    []string `validate:"omitempty,dive,vuln_type"`
    MaxResults   int      `validate:"omitempty,min=1,max=10000"`
}
```

## Adding New Validation Tags

To add a new validation tag:

1. Define the tag constant in `/pkg/mcp/domain/validation/tags.go`:
```go
const TagMyValidation = "my_validation"
```

2. Add the validation definition to `CommonValidationTags()`:
```go
TagMyValidation: {
    Tag:         TagMyValidation,
    Description: "Validates my custom format",
    Pattern:     regexp.MustCompile(`^my-pattern$`), // Optional
    Validator:   validateMyFormat,
},
```

3. Implement the validation function:
```go
func validateMyFormat(value interface{}, fieldName string, params map[string]interface{}) error {
    str, ok := value.(string)
    if !ok {
        return NewValidationError(fieldName, "must be a string")
    }

    if str == "" {
        return NewValidationError(fieldName, "cannot be empty")
    }

    // Your validation logic here
    if !isValidMyFormat(str) {
        return NewValidationError(fieldName, "must be a valid my format")
    }

    return nil
}
```

4. Add tests in `/pkg/mcp/domain/validation/tags_test.go`:
```go
func TestValidationTags_MyValidation(t *testing.T) {
    tags := CommonValidationTags()
    validator := tags[TagMyValidation].Validator

    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid format", "my-valid-value", false},
        {"invalid format", "invalid", true},
        {"empty string", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validator(tt.input, "test_field", nil)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Migration Guide

To migrate a tool from manual validation to tag-based validation:

1. **Add validation tags to struct fields**:
```go
// Before
type Args struct {
    SessionID string `json:"session_id"`
    ImageRef  string `json:"image_ref"`
}

// After
type Args struct {
    SessionID string `json:"session_id" validate:"required,session_id"`
    ImageRef  string `json:"image_ref" validate:"required,docker_image"`
}
```

2. **Replace manual validation with ValidateTaggedStruct**:
```go
// Before
func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    myArgs := args.(MyArgs)

    if myArgs.SessionID == "" {
        return errors.New("session_id is required")
    }

    if !isValidUUID(myArgs.SessionID) {
        return errors.New("invalid session_id format")
    }

    if myArgs.ImageRef == "" {
        return errors.New("image_ref is required")
    }

    // ... more validation logic

    return nil
}

// After
func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    return validation.ValidateTaggedStruct(args)
}
```

3. **Remove old validation code**:
   - Delete custom validation functions
   - Remove validation-specific error types
   - Clean up validation helper methods

## Best Practices

1. **Use `omitempty` for optional fields**: This skips validation if the field is empty
2. **Order tags logically**: Put `required` first, then type validation, then constraints
3. **Be specific with validation**: Use domain-specific tags (e.g., `docker_image`) instead of generic ones (e.g., `string`)
4. **Document special cases**: Add comments for non-obvious validation rules
5. **Test edge cases**: Ensure your validation handles empty values, invalid types, etc.

## Troubleshooting

### Common Issues

1. **"field X is required" when field has a value**
   - Check that you're not passing a pointer to the struct
   - Ensure the field is exported (starts with capital letter)

2. **Validation passes but shouldn't**
   - Check tag spelling (tags are case-sensitive)
   - Ensure the validation tag is registered in `CommonValidationTags()`

3. **"must be a string" errors for slice fields**
   - Make sure to use `dive` before element validation tags
   - Example: `validate:"omitempty,dive,vuln_type"`

## Performance Considerations

- Validation uses reflection, which has some overhead
- Tags are parsed once per struct type (cached)
- For high-performance paths, consider code generation approaches
- Batch validation is more efficient than individual field validation

## Future Enhancements

1. **Code Generation**: Generate optimized validation code from tags
2. **Custom Messages**: Support custom error messages in tags
3. **Conditional Validation**: Support `required_if`, `required_unless`, etc.
4. **Cross-Field Validation**: Support validation that depends on other fields
5. **Async Validation**: Support validation that requires external calls
