# ADR: Migration from Zerolog to Slog

## Status
Accepted

## Date
2025-07-07

## Context
Container Kit currently uses zerolog for logging throughout the codebase (326 files with ~1,152 API calls). However, the gomcp library that we depend on for Model Context Protocol implementation requires slog as its logging interface. This creates an incompatibility that manifests as:

1. **Pre-commit errors**: Logger API mismatches between zerolog and slog
2. **Interface conflicts**: gomcp expects `*slog.Logger` but we provide `zerolog.Logger`
3. **Maintenance overhead**: Managing two different logging libraries increases complexity
4. **Inconsistent logging**: Different parts of the system use different logging patterns

## Decision
We will migrate the entire codebase from zerolog to Go's standard library slog package.

## Rationale

### Technical Alignment
- **Library requirement**: gomcp mandates slog usage, making this migration necessary for compatibility
- **Standard library**: slog is part of Go's standard library (since Go 1.21), reducing external dependencies
- **Performance**: slog offers structured logging with performance characteristics suitable for our needs
- **Maintenance**: Using Go's standard library reduces long-term maintenance burden

### Migration Benefits
- **Unified logging**: Single logging interface across the entire application
- **Compatibility**: Full compatibility with gomcp and other slog-expecting libraries
- **Simplification**: Reduces complexity of managing multiple logging frameworks
- **Future-proofing**: Alignment with Go standard library ecosystem

### Risk Assessment
- **Low risk**: API patterns are similar between zerolog and slog
- **Automated migration**: Most conversions can be automated with scripts
- **Incremental approach**: Phase-by-phase migration minimizes disruption
- **Rollback capability**: Git-based approach allows easy rollback if needed

## Implementation Plan

### Migration Approach
1. **Automated conversion scripts** for common API patterns
2. **Phase-by-phase migration** (Core → Application → Domain → Infrastructure → Tests)
3. **Manual review** for complex logging scenarios
4. **Comprehensive testing** after each phase

### API Mapping Strategy
```go
// Zerolog pattern → Slog equivalent
logger.Error().Err(err).Msg("message")     → logger.Error("message", "error", err)
logger.Info().Str("key", "val").Msg("msg") → logger.Info("msg", "key", "val")
logger.With().Str("comp", "name").Logger() → logger.With("comp", "name")
```

### Timeline
4-week migration across 5 phases, with each phase validated before proceeding.

## Consequences

### Positive
- **Eliminates gomcp compatibility issues**
- **Reduces external dependencies** (removes zerolog dependency)
- **Simplifies logging architecture** (single logging interface)
- **Improves maintainability** (standard library alignment)
- **Enables clean pre-commit builds**

### Negative
- **Short-term development overhead** during migration period
- **Team coordination required** for breaking changes
- **Potential log format changes** may affect existing tooling
- **Learning curve** for team members unfamiliar with slog

### Neutral
- **Functionally equivalent**: Both libraries provide structured logging
- **Performance impact**: Minimal, slog performance is comparable to zerolog
- **Log output**: Similar structured logging capabilities

## Alternatives Considered

### 1. Adapter Pattern
Create an adapter layer to bridge zerolog and slog interfaces.
- **Rejected**: Adds complexity and maintains dual logging systems

### 2. Fork gomcp
Modify gomcp to accept zerolog instead of slog.
- **Rejected**: Creates maintenance burden for external dependency

### 3. Gradual mixed approach
Keep both logging systems and migrate incrementally.
- **Rejected**: Increases long-term complexity and inconsistency

## Success Metrics
- All 326 files successfully migrated to slog
- Zero compilation errors related to logging
- All tests pass with new logging implementation
- Pre-commit hooks pass without logging-related errors
- Performance within 5% of baseline
- Log output maintains readability and usefulness

## Related ADRs
- [2025-07-07-three-context-architecture.md](2025-07-07-three-context-architecture.md) - Architectural foundation supporting this migration
- [2025-01-07-unified-error-system.md](2025-01-07-unified-error-system.md) - Error handling that integrates with logging

## References
- [Go slog documentation](https://pkg.go.dev/log/slog)
- [Gomcp library requirements](https://github.com/localrivet/gomcp)
- [Migration implementation plan](../../SLOG_MIGRATION_PLAN.md)
