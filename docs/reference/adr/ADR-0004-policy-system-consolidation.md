# ADR-0004: Policy System Consolidation

## Status
Accepted

## Context

The containerization assistant previously used multiple policy-related modules that led to duplication, maintenance overhead, and type safety issues:

- `policy-lite.ts`: Lightweight policy handling
- `policy-normalizer.ts`: Policy normalization and transformation
- `resolver.ts`: Complex policy resolution with cache mechanisms

This fragmentation created:
- Repeated environment merging logic across modules
- Inconsistent type definitions between modules
- Complex resolution pipeline with multiple cache layers
- Difficulty maintaining policy format consistency

The Sprint 2 goal was to merge these into a unified policy module with stricter types and compile-time safety.

## Decision

We consolidate all policy functionality into a single `src/config/policy.ts` module with:

### Unified Policy Module Features

1. **Discriminated Unions for Type Safety**
   ```typescript
   export type RegexMatcher = {
     kind: 'regex';
     pattern: string;
     flags?: string;
     count_threshold?: number;
     comparison?: 'greater_than' | 'greater_than_or_equal' | 'equal' | 'less_than';
   };

   export type FunctionMatcher = {
     kind: 'function';
     function: string;
   };

   export type Matcher = RegexMatcher | FunctionMatcher;
   ```

2. **Environment Resolution at Load Time**
   ```typescript
   export function loadPolicy(path: string, environment = 'development'): UnifiedPolicy {
     // Load, parse, and resolve environment overrides in one step
     // Return fully resolved policy object
     // Cache resolved policies by environment
   }
   ```

3. **Simplified API Surface**
   - `loadPolicy()` - Load and resolve policy with environment
   - `selectStrategy()` - Strategy selection with resolved policy
   - `getRuleWeights()` - Get rule weights for content type
   - `validatePolicyFile()` - File validation
   - `validatePolicyData()` - In-memory validation

4. **Legacy Compatibility**
   - Maintains support for existing policy formats
   - Graceful migration path from old policy files
   - Backward-compatible exports for existing consumers

### Resolver Module Simplification

The `src/config/resolver.ts` module is simplified from 656 lines to ~300 lines by:
- Removing deprecated cache mechanisms
- Delegating policy operations to unified policy module
- Focusing on core resolution logic only
- Adding proper error boundaries

## Implementation

### Migration Strategy

1. **Phase 1**: Create unified policy module with all functionality
2. **Phase 2**: Update resolver to use unified module
3. **Phase 3**: Update all policy consumers to new API
4. **Phase 4**: Remove deprecated policy files

### Type Safety Improvements

- All matchers use discriminated unions preventing runtime type errors
- Compile-time validation of policy structure
- Strict TypeScript types throughout the policy pipeline
- Zod schemas for runtime validation

### Performance Benefits

- Single policy loading operation instead of multiple
- Reduced memory footprint from eliminating duplicate caches
- Faster policy resolution through unified cache strategy
- Elimination of redundant environment merging

## Rationale

### Why Consolidate?

1. **Reduced Complexity**: 50% reduction in policy-related code
2. **Type Safety**: Discriminated unions prevent runtime errors
3. **Maintainability**: Single source of truth for policy logic
4. **Performance**: Unified caching and resolution strategy
5. **Developer Experience**: Clearer API with fewer modules to understand

### Why Environment Resolution at Load Time?

1. **Eliminate Redundancy**: No repeated env merging in downstream code
2. **Cache Efficiency**: Cache fully resolved policies instead of raw + transforms
3. **Predictable Behavior**: Policy consumers receive consistent, resolved data
4. **Simpler Testing**: Test resolved policies instead of resolution logic

### Why Discriminated Unions?

1. **Compile-time Safety**: TypeScript can validate matcher types
2. **Runtime Performance**: No need for runtime type checking
3. **Better IDE Support**: Autocomplete and type checking in editors
4. **Maintenance**: Prevents addition of invalid matcher combinations

## Consequences

### Positive

1. **Code Reduction**: 50% less policy-related code to maintain
2. **Type Safety**: 100% compile-time validation of policy matchers
3. **Performance**: Faster policy resolution and reduced memory usage
4. **Consistency**: Single module ensures consistent policy handling
5. **Testability**: Easier to test unified policy operations

### Negative

1. **Migration Effort**: Existing policy consumers need updates
2. **Temporary Duplication**: During migration, both old and new modules exist
3. **Learning Curve**: Developers need to learn new unified API

### Mitigation Strategies

1. **Incremental Migration**: Update consumers one at a time
2. **Comprehensive Tests**: Ensure unified module handles all existing cases
3. **Documentation**: Clear migration guide and API documentation
4. **Backward Compatibility**: Maintain exports for smooth transition

## Examples

### Old vs New API

```typescript
// Old approach (multiple modules)
import { loadPolicy } from './policy-lite';
import { normalizePolicy } from './policy-normalizer';
import { resolveWithEnvironment } from './resolver';

const raw = loadPolicy(path);
const normalized = normalizePolicy(raw);
const resolved = resolveWithEnvironment(normalized, env);

// New approach (unified module)
import { loadPolicy } from './policy';

const policy = loadPolicy(path, env); // Fully resolved
```

### Type Safety Improvements

```typescript
// Old: Runtime type checking required
if (matcher.type === 'regex') {
  // Could fail at runtime
  const pattern = matcher.pattern;
}

// New: Compile-time type safety
if (matcher.kind === 'regex') {
  // TypeScript guarantees pattern exists
  const pattern = matcher.pattern;
}
```

### Simplified Policy Loading

```typescript
// Old: Multiple steps
const cacheKey = `${path}:${environment}`;
const cached = cache.get(cacheKey);
if (!cached) {
  const raw = loadPolicy(path);
  const resolved = applyEnvironment(raw, environment);
  cache.set(cacheKey, resolved);
}

// New: Single operation
const policy = loadPolicy(path, environment); // Handles caching internally
```

## Related Decisions

- ADR-0001: Effective Config & Policy Precedence
- ADR-0002: Prompt DSL Removal (variables-only)
- ADR-0003: Router Architecture Split

## References

- [Unified Policy Module](../../src/config/policy.ts)
- [Simplified Resolver](../../src/config/resolver.ts)
- [Sprint 2 Planning](../../../plans/SPRINT_PLAN.md#sprint-2-policy-system-consolidation)