# Error Migration Pattern Guide

## Migration Pattern

### From fmt.Errorf to types.NewRichError

**Before:**
```go
return fmt.Errorf("error message: %s", value)
```

**After:**
```go
return types.NewRichError("ERROR_CODE", fmt.Sprintf("error message: %s", value), "error_type")
```

### Key Points
1. types.NewRichError takes 3 parameters: (code, message, errorType)
2. Format strings must be resolved with fmt.Sprintf
3. Choose appropriate error codes and types

## Error Code Guidelines

| Error Pattern | Code | Type |
|--------------|------|------|
| Invalid arguments | INVALID_ARGUMENTS | validation_error |
| Not found | NOT_FOUND | not_found_error |
| Configuration missing | CONFIG_ERROR | config_error |
| Permission denied | UNAUTHORIZED | auth_error |
| Operation failed | OPERATION_FAILED | execution_error |
| Timeout | TIMEOUT | timeout_error |
| Network/connection | CONNECTION_ERROR | network_error |
| Parse/unmarshal | PARSE_ERROR | parse_error |

## Migration Priority

Given the large number of instances (255), we should focus on:

1. **High-impact files** - Files with the most errors
2. **Public API tools** - Tools that users interact with directly
3. **Critical path tools** - Build, deploy, health check tools

## Automated vs Manual Migration

Due to the complexity of:
- Choosing appropriate error codes
- Determining error types
- Handling format strings

This migration requires manual review for each instance rather than full automation.