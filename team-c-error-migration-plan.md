# Team C - Error Migration Plan

## Objective
Replace fmt.Errorf with types.NewRichError throughout MCP tools using the automated migration tool.

## Current Status
According to REORG.md: 247 proper types vs 860 fmt.Errorf (28% adoption)

## Migration Strategy

### Phase 1: Dry Run Analysis
First, analyze each package to see what would be migrated:

```bash
# Analyze build package
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/build -verbose

# Analyze deploy package  
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/deploy -verbose

# Analyze scan package
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/scan -verbose

# Analyze analyze package
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/analyze -verbose

# Analyze session package
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/session -verbose

# Analyze server package
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/server -verbose
```

### Phase 2: Package-by-Package Migration
Migrate each package systematically:

```bash
# Start with smaller packages first
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/scan -dry-run=false -report scan-migration.txt
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/server -dry-run=false -report server-migration.txt

# Then larger packages
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/build -dry-run=false -report build-migration.txt
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/deploy -dry-run=false -report deploy-migration.txt
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/analyze -dry-run=false -report analyze-migration.txt
go run tools/migrate-errors/main.go -package ./pkg/mcp/internal/session -dry-run=false -report session-migration.txt
```

### Phase 3: Verification
After each package migration:
1. Run `go build ./pkg/mcp/...` to ensure compilation
2. Run `go test -short ./pkg/mcp/...` to ensure tests pass
3. Review the migration report for any manual interventions needed

### Phase 4: Final Cleanup
- Handle any errors that couldn't be automatically migrated
- Update imports if needed
- Run final validation

## Expected Outcomes
- Significant reduction in fmt.Errorf usage
- Consistent error handling with proper error types
- Better error categorization for debugging