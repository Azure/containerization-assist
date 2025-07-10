# File Size Limits

This document explains the file size limits enforced by the CI system and the rationale behind them.

## Size Limits by File Type

| File Type | Limit | Rationale |
|-----------|--------|-----------|
| **Default files** | 800 lines | Standard business logic should be focused and maintainable |
| **Interface files** | 1200 lines | Interface definitions, domain logic, and helpers can be larger due to comprehensive APIs |
| **Implementation files** | 1600 lines | Complex implementations and consolidated files need more space for full feature implementation |
| **Large analysis files** | 2200 lines | Command implementations with extensive analysis logic require maximum space |

## File Type Classification

The CI system automatically classifies files based on their names and purposes:

### Default Files (800 lines)
- Standard Go files not matching specific patterns
- General business logic files
- Simple utility files

### Interface Files (1200 lines)  
- `*interfaces.go` - Interface definitions
- `*api.go` - API definitions
- `*domain_validators.go` - Domain validation logic
- `*tags.go` - Security and metadata tags
- `*helpers.go`, `*helper.go` - Helper utilities
- `*state_types.go` - State management types
- `*context_enrichers.go` - Context enrichment logic
- `*auto_fix_helper.go` - Auto-fix functionality
- `*k8s_operations.go` - Kubernetes operations
- `*operations.go` - General operations

### Implementation Files (1600 lines)
- `*consolidated*.go` - Consolidated implementations
- `*implementation*.go` - Feature implementations  
- `*_impl.go` - Service implementations
- `*server_impl.go` - Server implementations
- `*tool_registration.go` - Tool registration logic

### Large Analysis Files (2200 lines)
- `*analyze_implementation.go` - Complex analysis implementations

## Guidelines

1. **Keep files focused**: Even with higher limits, strive to keep files focused on a single responsibility
2. **Consider splitting**: If a file approaches its limit, consider if it can be split into logical modules
3. **Document complexity**: Large files should have clear documentation explaining their structure
4. **Review regularly**: Periodically review large files to see if they can be refactored

## Exceptions

If you need to exceed these limits:

1. First consider if the file can be refactored into smaller, more focused modules
2. If the file truly needs to be larger, update the classification in `scripts/check_file_size.sh`
3. Document the rationale in this file
4. Ensure the file has comprehensive documentation and clear structure

## Tools

- **File size checker**: `scripts/check_file_size.sh`
- **Manual check**: `find pkg/mcp -name "*.go" -exec wc -l {} \; | sort -n`