# Rego Policy Guide

## Overview

This project uses **Rego policies exclusively** (from Open Policy Agent). YAML policy support has been removed in favor of the more expressive and industry-standard Rego format.

## Why Rego?

### YAML Policy Limitations
- Limited expressiveness (only regex and simple functions)
- No composition or reusability
- No built-in testing framework
- Custom evaluation engine to maintain

### Rego Benefits
- ✅ Industry standard (CNCF graduated project)
- ✅ Rich built-in functions (strings, arrays, objects, crypto, time, etc.)
- ✅ Composable policies with imports
- ✅ Built-in testing framework (`opa test`)
- ✅ Integration with Kubernetes admission controllers
- ✅ Active community and tooling ecosystem

## Current Status

### ✅ Implemented
- OPA Rego policy evaluator (`src/config/policy-rego.ts`)
- Async policy loader (`loadPolicy`, `loadAndMergePolicies` from `@/config`)
- Three reference Rego policies:
  - `policies/security-baseline.rego` - Core security rules
  - `policies/base-images.rego` - Base image governance
  - `policies/container-best-practices.rego` - Docker best practices
- Comprehensive Rego policy tests (`policies/security-baseline_test.rego`)
- Policy evaluation wrapper (`applyPolicy`)

### ⚠️ Breaking Changes
- YAML policy support has been completely removed
- `loadPolicy()` and `loadAndMergePolicies()` are now async
- Policy files must use `.rego` extension
- Old policy test files have been removed

## Usage

### Loading Rego Policies

```typescript
import { loadPolicyAsync, applyRegoPolicyAsync } from '@/config';

// Load a Rego policy
const result = await loadPolicyAsync('policies/security-baseline.rego');

if (result.ok) {
  const evaluator = result.value;

  // Evaluate content against the policy
  const evalResult = await evaluator.evaluate({
    content: dockerfileContent
  });

  if (!evalResult.allow) {
    console.log('Violations:', evalResult.violations);
  }
}
```

### Using in Tools

Tools receive policies via ToolContext. For Rego policies:

```typescript
async function myTool(input: MyInput, ctx: ToolContext): Promise<Result<MyOutput>> {
  if (ctx.policy) {
    // Policy could be YAML or Rego
    // Use async evaluation to support both
    const validation = await applyPolicyAsync(ctx.policy, content);

    if ('allow' in validation && !validation.allow) {
      return Failure('Policy violations detected', {
        violations: validation.violations
      });
    }
  }

  // ... tool logic
}
```

## Policy File Format

### Rego Policy Structure

```rego
package containerization.security

# Metadata
policy_name := "Security Baseline"
policy_version := "2.0"

# Input type detection
is_dockerfile {
  contains(input.content, "FROM ")
}

# Violations (blocking)
violations[result] {
  is_dockerfile
  regex.match(`(?m)^USER\s+(root|0)\s*$`, input.content)

  result := {
    "rule": "block-root-user",
    "category": "security",
    "priority": 95,
    "severity": "block",
    "message": "Running as root user is not allowed."
  }
}

# Warnings (non-blocking)
warnings[result] {
  is_dockerfile
  not regex.match(`(?m)^USER\s+\w+`, input.content)

  result := {
    "rule": "require-user-directive",
    "severity": "warn",
    "message": "USER directive recommended."
  }
}

# Final decision
allow {
  count(violations) == 0
}

# Result structure
result := {
  "allow": allow,
  "violations": violations,
  "warnings": warnings,
  "suggestions": suggestions,
  "summary": {
    "total_violations": count(violations),
    "total_warnings": count(warnings),
    "total_suggestions": count(suggestions)
  }
}
```

### Required Output Structure

Rego policies MUST export a `result` object with:

```typescript
{
  allow: boolean,              // true if no blocking violations
  violations: Array<{          // Blocking issues
    rule: string,
    message: string,
    severity: "block",
    category: string,
    priority?: number,
    description?: string
  }>,
  warnings: Array<{            // Non-blocking warnings
    rule: string,
    message: string,
    severity: "warn",
    category: string
  }>,
  suggestions: Array<{         // Optional improvements
    rule: string,
    message: string,
    severity: "suggest",
    category: string
  }>,
  summary?: {
    total_violations: number,
    total_warnings: number,
    total_suggestions: number
  }
}
```

## Testing Rego Policies

### Writing Tests

Create a `*_test.rego` file alongside your policy:

```rego
package containerization.security

test_block_root_user {
  violations["block-root-user"] with input as {
    "content": "FROM node:20\nUSER root"
  }
}

test_allow_nonroot_user {
  allow with input as {
    "content": "FROM node:20\nUSER node"
  }
}
```

### Running Tests

```bash
# Run all policy tests
npm run test:policies

# Or use OPA directly
opa test policies/
```

## Migration Path

### Phase 1: Gradual Migration (Current)
- ✅ YAML policies continue to work
- ✅ New Rego policies can be added alongside YAML
- ✅ Tools support both formats

### Phase 2: Rego Adoption (Recommended)
1. Convert critical policies to Rego format
2. Test thoroughly using `opa test`
3. Update policy references to use `.rego` files
4. Keep YAML as fallback during transition

### Phase 3: YAML Deprecation (Future)
1. Mark YAML policy support as deprecated
2. Provide migration tooling (YAML → Rego converter)
3. Remove YAML support in major version update

## Converting YAML to Rego

### Example Conversion

**YAML Policy:**
```yaml
rules:
  - id: block-root-user
    category: security
    priority: 95
    conditions:
      - kind: regex
        pattern: '^USER\s+(root|0)\s*$'
        flags: m
    actions:
      block: true
      message: 'Running as root is not allowed'
```

**Equivalent Rego:**
```rego
violations[result] {
  regex.match(`(?m)^USER\s+(root|0)\s*$`, input.content)

  result := {
    "rule": "block-root-user",
    "category": "security",
    "priority": 95,
    "severity": "block",
    "message": "Running as root is not allowed"
  }
}
```

## API Reference

### New Functions

#### `loadPolicyAsync(file: string): Promise<Result<RegoEvaluator | Policy>>`
Load policy asynchronously. Supports both `.rego` and `.yaml` files.

#### `loadAndMergePoliciesAsync(paths: string[]): Promise<Result<RegoEvaluator | Policy>>`
Load and merge multiple policy files asynchronously.

#### `applyRegoPolicyAsync(evaluator: RegoEvaluator, input: string | object): Promise<RegoPolicyResult>`
Evaluate Rego policy against input.

#### `applyPolicyAsync(policy: Policy | RegoEvaluator, input: string | object): Promise<...>`
Polymorphic policy application (auto-detects YAML vs Rego).

### New Types

#### `RegoEvaluator`
```typescript
interface RegoEvaluator {
  evaluate(input: string | Record<string, unknown>): Promise<RegoPolicyResult>;
  close(): void;
}
```

#### `RegoPolicyResult`
```typescript
interface RegoPolicyResult {
  allow: boolean;
  violations: RegoPolicyViolation[];
  warnings: RegoPolicyViolation[];
  suggestions: RegoPolicyViolation[];
  summary?: {
    total_violations: number;
    total_warnings: number;
    total_suggestions: number;
  };
}
```

#### `RegoPolicyViolation`
```typescript
interface RegoPolicyViolation {
  rule: string;
  message: string;
  severity: 'block' | 'warn' | 'suggest';
  category: string;
  priority?: number;
  description?: string;
}
```

## Best Practices

### 1. Input Structure
Always provide input as an object with a `content` field:

```rego
# Good - supports both string and structured input
is_dockerfile {
  contains(input.content, "FROM ")
}

# Less flexible - only works with string input
is_dockerfile {
  contains(input, "FROM ")
}
```

### 2. Rule Organization
Group related rules by category:

```rego
# ============================================================================
# SECURITY RULES
# ============================================================================

violations[result] { ... }
violations[result] { ... }

# ============================================================================
# QUALITY RULES
# ============================================================================

warnings[result] { ... }
```

### 3. Testing Coverage
Test both positive and negative cases:

```rego
# Test that violations are caught
test_block_root_user {
  violations["block-root-user"] with input as {"content": "USER root"}
}

# Test that compliant configs pass
test_allow_nonroot_user {
  allow with input as {"content": "USER node"}
}
```

### 4. Error Handling
Policies should never crash. Use safe navigation:

```rego
# Safe - returns false if path doesn't exist
has_healthcheck {
  regex.match(`HEALTHCHECK`, input.content)
}

# Unsafe - could crash if input.content is missing
has_healthcheck {
  input.content[_] == "HEALTHCHECK"
}
```

## Resources

- [OPA Documentation](https://www.openpolicyagent.org/docs/)
- [Rego Playground](https://play.openpolicyagent.org/)
- [Rego Language Reference](https://www.openpolicyagent.org/docs/latest/policy-language/)
- [OPA Testing](https://www.openpolicyagent.org/docs/latest/policy-testing/)

## Support

For questions or issues:
1. Check existing YAML policies for reference patterns
2. Review `policies/security-baseline.rego` for examples
3. Run `opa test policies/` to validate syntax
4. Open an issue on GitHub

## Future Enhancements

- [ ] YAML to Rego conversion tool
- [ ] Policy bundle support (multiple packages)
- [ ] OPA server integration for real-time evaluation
- [ ] Kubernetes admission controller integration
- [ ] Policy-as-Code CI/CD integration
