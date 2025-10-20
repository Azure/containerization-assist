# ADR-001: Result<T> Error Handling Pattern

**Date:** 2025-10-17
**Status:** Accepted
**Deciders:** Engineering Team
**Context:** We needed a consistent, type-safe approach to error handling across the entire codebase that avoids the pitfalls of thrown exceptions and promotes explicit error handling.

## Decision

We decided to use the `Result<T>` pattern for all error handling instead of throwing exceptions. All functions that can fail return `Result<T, Error>` where `T` is the success type.

**Implementation:**

```typescript
type Result<T, E = Error> =
  | { ok: true; value: T }
  | { ok: false; error: E };

// Usage example
async function buildImage(config: BuildConfig): Promise<Result<Image>> {
  const docker = await createDockerClient();
  if (!docker.ok) {
    return Failure('Docker client unavailable', docker.error);
  }

  // ... build logic
  return Success(image);
}
```

## Rationale

1. **Explicit Error Handling:** Forces developers to acknowledge and handle errors at every level
2. **Type Safety:** TypeScript can track which code paths have handled errors via type narrowing
3. **No Uncaught Exceptions:** Eliminates runtime crashes from unhandled exceptions
4. **Composability:** Results can be chained and transformed using functional patterns
5. **Better Error Messages:** Errors carry context through the call stack with actionable guidance

## Consequences

### Positive

- **97% adoption across 71 files:** Nearly complete migration demonstrates team commitment
- **Zero uncaught exceptions:** No runtime crashes from unhandled errors in production
- **Improved debugging:** Error messages include context and suggested actions
- **Testability:** Easy to test error paths without throwing exceptions
- **IDE support:** TypeScript's type narrowing provides excellent autocomplete
- **Predictable control flow:** No invisible exception paths to track

### Negative

- **Verbosity:** Requires explicit error checking with `if (!result.ok)` guards
- **Learning curve:** New pattern for developers familiar with try/catch
- **Async complexity:** Requires careful composition when chaining multiple async operations
- **Migration effort:** Required updating existing code that used try/catch

## Alternatives Considered

### Alternative 1: Traditional try/catch

- **Pros:**
  - Familiar to most developers
  - Built into JavaScript/TypeScript
  - Less verbose for happy path
- **Cons:**
  - Silent failures when developers forget try/catch
  - Type system can't track error handling
  - Error types not encoded in function signatures
  - Harder to compose operations
- **Rejected because:** Doesn't provide compile-time safety and allows errors to propagate invisibly

### Alternative 2: Error Callbacks (Node.js style)

- **Pros:**
  - Explicit error handling
  - Familiar pattern in Node.js ecosystem
- **Cons:**
  - Callback hell with nested operations
  - No type safety for error paths
  - Difficult to compose operations
  - Not idiomatic in modern TypeScript
- **Rejected because:** Poor ergonomics and lacks type safety benefits

### Alternative 3: fp-ts Either Type

- **Pros:**
  - Battle-tested functional programming library
  - Rich composition utilities
  - Full type safety
- **Cons:**
  - Additional dependency
  - Steep learning curve for FP concepts
  - More complex API than needed
- **Rejected because:** Overkill for our use case; custom Result<T> provides exactly what we need

## Related Decisions

- **ADR-002: Unified Tool Interface** - All tools use Result<T> for return values
- **ADR-006: Infrastructure Layer Organization** - Result<T> pattern used across all layers (@infra, @lib, @tools)

## References

- Implementation: `src/types/result.ts`
- Helper functions: `src/lib/result-utils.ts`
- Usage examples across `src/tools/*/tool.ts` (17 tools)
- Migration guide: See PR history for conversion patterns
- Related: ADR-002 (Tool Interface uses Result<T> consistently)
