# Error Migration Tool

Automated tool to help migrate from standard Go error handling to RichError patterns.

## Overview

This tool analyzes Go source code and automatically migrates error creation patterns:
- `fmt.Errorf()` → `types.NewRichError()`
- `errors.New()` → `types.NewRichError()`
- `errors.Wrap()` → `types.WrapRichError()`

The tool intelligently categorizes errors and suggests appropriate error types based on the error message content.

## Features

- **Automatic Error Categorization**: Analyzes error messages to determine the appropriate error type
- **Safe Migration**: Dry-run mode by default to preview changes
- **Interactive Mode**: Review and approve each migration individually
- **Selective Processing**: Target specific packages or files
- **Detailed Reporting**: Generate migration reports with statistics
- **Import Management**: Automatically adds required imports

## Usage

### Basic Usage

```bash
# Dry run - see what would be changed
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/build

# Actually perform migrations
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/build -dry-run=false

# Interactive mode - approve each change
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/build -dry-run=false -interactive

# Generate a report
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/build -report migration-report.txt
```

### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-package` | (required) | Package path to migrate |
| `-dry-run` | true | Show changes without modifying files |
| `-verbose` | false | Show detailed migration information |
| `-interactive` | false | Prompt for each migration |
| `-report` | "" | Output migration report to file |
| `-include-tests` | false | Include test files in migration |
| `-auto-only` | false | Only perform automatic migrations |

## Error Categorization

The tool automatically categorizes errors based on keywords:

| Error Type | Keywords | Example |
|------------|----------|---------|
| ValidationError | validation, invalid | "invalid configuration" |
| NotFoundError | not found, does not exist | "resource not found" |
| UnauthorizedError | unauthorized, permission | "permission denied" |
| TimeoutError | timeout, deadline | "operation timeout" |
| NetworkError | connection, network | "connection refused" |
| ParseError | parse, unmarshal | "failed to parse JSON" |
| ConfigError | config, configuration | "missing configuration" |
| InternalError | internal, unexpected | "internal server error" |
| GeneralError | (default) | Any other error |

## Migration Examples

### Simple Error Migration

**Before:**
```go
return fmt.Errorf("invalid configuration: %s", configPath)
```

**After:**
```go
return types.NewRichError("ValidationError", "invalid configuration: %s", configPath)
```

### Error Wrapping Migration

**Before:**
```go
return errors.Wrap(err, "failed to connect to database")
```

**After:**
```go
return types.WrapRichError(err, "NetworkError", "failed to connect to database")
```

### Complex Format String

**Before:**
```go
return fmt.Errorf("user %s not found in organization %s", userID, orgID)
```

**After:**
```go
return types.NewRichError("NotFoundError", "user %s not found in organization %s", userID, orgID)
```

## Migration Strategy

### Phase 1: High-Impact Files
Start with files that have the most errors:

```bash
# Find top candidates from quality dashboard
go run tools/quality-dashboard/main.go -format json | \
  jq -r '.error_handling.top_files_to_migrate[] |
  "\(.path): \(.standard_errors) errors"'
```

### Phase 2: Package by Package
Migrate entire packages systematically:

```bash
# Migrate a package
go run tools/migrate-errors/main.go \
  -package ./pkg/mcp/internal/workflow \
  -dry-run=false \
  -report workflow-migration.txt
```

### Phase 3: Review and Refine
Review migrations that need manual attention:

```bash
# Show only non-automatic migrations
go run tools/migrate-errors/main.go \
  -package ./pkg/mcp \
  -verbose | grep "manual review"
```

## Integration with CI/CD

### Pre-commit Hook
Add to your pre-commit hook to prevent new standard errors:

```bash
# Check for new standard errors in staged files
STAGED=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')
if [ -n "$STAGED" ]; then
  go run tools/migrate-errors/main.go -package . -dry-run | grep "Would migrate"
fi
```

### GitHub Actions
Include in your quality gates workflow:

```yaml
- name: Check Error Migration Opportunities
  run: |
    go run tools/migrate-errors/main.go \
      -package ./pkg \
      -report error-migration.txt

    OPPORTUNITIES=$(grep "Total Errors Found:" error-migration.txt | awk '{print $4}')
    if [ "$OPPORTUNITIES" -gt 100 ]; then
      echo "⚠️ Found $OPPORTUNITIES error migration opportunities"
      exit 1
    fi
```

## Best Practices

1. **Review Before Applying**: Always run in dry-run mode first
2. **Test After Migration**: Run tests to ensure behavior hasn't changed
3. **Commit Separately**: Keep error migrations in separate commits
4. **Update Documentation**: Document any new error types introduced
5. **Gradual Migration**: Migrate package by package, not all at once

## Limitations

The tool will skip migrations that:
- Use complex format strings with `%v` or `%+v`
- Have function calls in error arguments (potential side effects)
- Are in generated code files
- Have ambiguous error types

These require manual review and migration.

## Troubleshooting

### "Failed to parse file"
- Check if the file has syntax errors
- Ensure the file is valid Go code

### "Import already exists"
- The tool detected the types package is already imported
- Safe to ignore

### "Requires manual review"
- The error pattern is too complex for automatic migration
- Review the suggestion and apply manually if appropriate

## Extending the Tool

To add new error categorization rules:

1. Edit the `categorizeError` function
2. Add new keywords and error types
3. Update the documentation

To support new error patterns:

1. Add new case in `analyzeCallExpr`
2. Implement the analysis function
3. Add migration generation logic
