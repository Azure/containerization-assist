# ADR-0001: Effective Config & Policy Precedence

## Status
Accepted

## Context

The containerization assistant requires a robust configuration system that can handle multiple sources of configuration and policy enforcement. The system needs to resolve conflicts between different configuration sources while maintaining predictable behavior and clear precedence rules.

Key challenges addressed:
- Multiple configuration sources (environment variables, config files, CLI arguments, policies)
- Policy enforcement that can override user preferences for security/compliance
- Strategy merging when multiple strategies apply to the same tool
- Configuration validation and effective config resolution

## Decision

We implement a hierarchical configuration precedence system with the following order (highest to lowest priority):

### Precedence Order

1. **Policy Enforcement** (Highest Priority)
   - Security policies that cannot be overridden
   - Compliance policies enforced at organization level
   - Resource limit policies

2. **CLI Arguments**
   - Command-line flags and options
   - Tool-specific parameters passed directly

3. **Environment Variables**
   - Process environment variables
   - Container/deployment-specific overrides

4. **Configuration Files**
   - `.env` files
   - YAML/JSON configuration files
   - Project-specific config files

5. **Strategy Merging**
   - Multiple strategies combined using merge logic
   - Last-defined strategy takes precedence for conflicts

6. **Default Values** (Lowest Priority)
   - Built-in defaults
   - Framework defaults

### Policy Override Rules

Policies implement a clamping mechanism where:
- **Hard limits**: Cannot be exceeded regardless of user configuration
- **Soft limits**: Generate warnings but allow execution
- **Forbidden values**: Block execution with clear error messages

Example policy enforcement:
```typescript
const effectiveConfig = applyPolicyEnforcement(userConfig, policies);
// If user requests maxTokens: 10000 but policy maxTokens: 2000
// Result: maxTokens: 2000 (policy wins)
```

### Strategy Merge Order

When multiple strategies apply to the same tool:
1. Load all applicable strategies
2. Sort by specificity (more specific patterns win)
3. Merge configurations with later strategies overriding earlier ones
4. Apply policy enforcement as final step

## Rationale

### Why Hierarchical Precedence?

1. **Security First**: Policies must be able to override user preferences to enforce organizational security standards
2. **Predictable Behavior**: Clear precedence rules eliminate ambiguity about which configuration applies
3. **Flexible Override**: Users can still customize behavior within policy constraints
4. **Deployment Friendly**: Environment variables and CLI args can override file-based config for different environments

### Why Policy Clamping?

1. **Non-negotiable Security**: Some limits (like cost caps) must be enforced regardless of user intent
2. **Clear Error Messages**: Users understand why their configuration was modified
3. **Audit Trail**: Policy enforcement is logged for compliance tracking

### Why Strategy Merging?

1. **Composability**: Multiple strategies can contribute different aspects of configuration
2. **Specificity Wins**: More specific patterns should override general ones
3. **Incremental Overrides**: Later strategies can fine-tune earlier strategy decisions

## Implementation Details

### Configuration Resolution Pipeline

```typescript
function getEffectiveConfig(
  userConfig: UserConfig,
  policies: PolicySet,
  strategies: Strategy[]
): EffectiveConfig {
  // 1. Merge strategies by specificity
  const strategyConfig = mergeStrategies(strategies);

  // 2. Apply configuration precedence
  const mergedConfig = mergeConfigurations([
    defaultConfig,
    strategyConfig,
    fileConfig,
    envConfig,
    cliConfig
  ]);

  // 3. Apply policy enforcement (highest priority)
  return applyPolicyEnforcement(mergedConfig, policies);
}
```

### Policy Enforcement Types

```typescript
interface PolicyEnforcement {
  maxTokens?: number;        // Hard limit
  maxCostUsd?: number;       // Hard limit
  forbiddenModels?: string[]; // Block list
  timeoutMs?: number;        // Hard limit
  warnings?: {
    costThreshold?: number;   // Soft limit
    tokenThreshold?: number;  // Soft limit
  };
}
```

## Consequences

### Positive

1. **Predictable Configuration**: Clear precedence rules eliminate configuration surprises
2. **Security Compliance**: Policy enforcement ensures organizational standards are met
3. **Flexible Deployment**: Different environments can override config as needed
4. **Audit Trail**: All configuration decisions are traceable and logged

### Negative

1. **Complexity**: Multiple configuration sources increase system complexity
2. **Debug Difficulty**: Configuration resolution issues can be harder to debug
3. **Performance**: Configuration resolution happens on every request
4. **Policy Management**: Requires tooling for policy creation and management

### Mitigation Strategies

1. **Configuration Debugging**: Provide tools to show effective configuration resolution
2. **Performance Optimization**: Cache resolved configurations when possible
3. **Clear Documentation**: Document precedence rules and provide examples
4. **Policy Tooling**: Create utilities for policy validation and testing

## Examples

### Example 1: Cost Limit Enforcement

```yaml
# User config
maxCostUsd: 50.0

# Policy
maxCostUsd: 10.0

# Result: maxCostUsd: 10.0 (policy enforced)
```

### Example 2: Strategy Merging

```yaml
# Strategy 1 (general)
temperature: 0.7
maxTokens: 2000

# Strategy 2 (specific to tool)
temperature: 0.9  # Overrides strategy 1
model: "gpt-4"    # Adds new setting

# Result: temperature: 0.9, maxTokens: 2000, model: "gpt-4"
```

### Example 3: Environment Override

```bash
# File config
MAX_TOKENS=1000

# Environment override
export MAX_TOKENS=2000

# CLI override
--max-tokens 3000

# Policy limit
maxTokens: 1500

# Result: maxTokens: 1500 (policy clamps CLI value)
```

## Related Decisions

- ADR-0002: Prompt DSL Removal (variables-only)
- ADR-0003: Router Architecture Split

## References

- [Configuration Resolution Implementation](../config/resolver.ts)
- [Policy Enforcement System](../policies/enforcement.ts)
- [Strategy Merging Logic](../strategies/merger.ts)