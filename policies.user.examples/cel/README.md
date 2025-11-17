# CEL Policy Examples

This directory contains example policies written in CEL (Common Expression Language) format for containerization-assist.

## What is CEL?

CEL (Common Expression Language) is a simple, fast expression language designed for validation rules. It's ideal for policy rules that check patterns, strings, or simple conditions without requiring complex logic.

## When to Use CEL vs Rego

**Use CEL when:**
- ✅ You need simple validation rules (pattern matching, string checks)
- ✅ You want easy-to-read policies for non-developers
- ✅ You need fast evaluation performance
- ✅ You want no external dependencies (OPA binary not required)

**Use Rego when:**
- ✅ You need complex logic and computations
- ✅ You need configuration generation (`generation_config`)
- ✅ You need template injection
- ✅ You need knowledge filtering
- ✅ You want built-in testing framework (`opa test`)

See [Policy Authoring Guide](../../docs/guides/policy-authoring.md) for detailed comparison.

## Example Policies

### 1. simple-validation.yaml
**Use Case:** Basic Dockerfile validation rules

Covers:
- Security: Non-root user requirement
- Best practices: Avoid `:latest` tags
- Health checks: HEALTHCHECK directive
- Optimization: Multi-stage builds
- Security: Detect hardcoded secrets

**Severity Levels:**
- `block`: Prevents operation (e.g., no root user)
- `warn`: Logs warning, allows operation (e.g., no HEALTHCHECK)
- `suggest`: Informational suggestion (e.g., use LABEL)

**Try it:**
```bash
export CUSTOM_POLICY_PATH=/path/to/simple-validation.yaml
ca-mcp start
```

### 2. kubernetes-validation.yaml
**Use Case:** Kubernetes manifest production readiness

Covers:
- Resource limits and requests
- Liveness and readiness probes
- Security context (runAsNonRoot, readOnlyRootFilesystem)
- High availability (replica count)
- Observability (Prometheus annotations)

**Try it:**
```bash
export CUSTOM_POLICY_PATH=/path/to/kubernetes-validation.yaml
ca-mcp start
```

### 3. production-ready.yaml
**Use Case:** Comprehensive production readiness checks

Combines multiple validation rules organized by severity:
- **BLOCKING:** No root user, no `:latest` tags, no hardcoded secrets
- **WARNING:** Health checks, multi-stage builds, minimal base images
- **SUGGESTION:** Labels, layer optimization, metrics endpoints

**Try it:**
```bash
export CUSTOM_POLICY_PATH=/path/to/production-ready.yaml
ca-mcp start
```

## CEL Policy Structure

All CEL policies follow this YAML structure:

```yaml
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: my-policy-name
  version: "1.0.0"
  description: Human-readable description
spec:
  rules:
    - name: rule-identifier
      category: security|best-practices|optimization|health
      severity: block|warn|suggest
      priority: 0-100  # Higher = more important (optional, default: 50)
      condition: 'CEL expression returning true for violations'
      message: "User-facing error message"
      description: "Detailed explanation (optional)"
```

## CEL Expression Syntax

CEL expressions have access to the `input` object:

```yaml
# String operations
condition: 'input.content.contains("FROM alpine")'
condition: '!input.content.contains("USER root")'

# Boolean operators
condition: |
  input.content.contains("FROM") &&
  !input.content.contains("HEALTHCHECK")

# Complex conditions
condition: |
  input.content.contains("PASSWORD=") ||
  input.content.contains("SECRET=") ||
  input.content.contains("API_KEY=")
```

## Available Input Context

When evaluating policies, the following context is available:

- `input.content` - String content (Dockerfile, Kubernetes manifest, etc.)
- Additional fields may be available depending on the tool context

## Using Multiple Policies

You can combine Rego and CEL policies together:

```bash
# Mix CEL and Rego policies in policies.user/ directory
policies.user/
├── my-rego-policy.rego
├── my-cel-policy.yaml
└── another-cel-policy.yml

# All policies are automatically discovered and merged
ca-mcp start
```

The system:
1. Auto-detects format by file extension (`.rego` vs `.yaml`/`.yml`)
2. Loads and compiles all policies
3. Evaluates all policies in parallel
4. Merges results and removes duplicates
5. Sorts violations by priority

## Testing CEL Policies

### Manual Testing

Use the policy simulation command:

```bash
# Test against a Dockerfile
npx tsx src/cli/policy-simulate.ts \
  --policy policies.user.examples/cel/simple-validation.yaml \
  --tool generate-dockerfile \
  --input '{"language": "node", "environment": "production"}'
```

### Integration Testing

CEL policies are validated during tool execution:

```bash
# generate-dockerfile validates against all loaded policies
# If violations are found, they're reported in the output
ca-mcp start
```

## Migration from Rego to CEL

If you have simple Rego validation rules, they can be migrated to CEL:

**Rego:**
```rego
violations contains result if {
  not regex.match("USER", input.content)
  result := {
    "rule": "require-user",
    "severity": "block",
    "message": "USER directive required"
  }
}
```

**CEL Equivalent:**
```yaml
- name: require-user
  category: security
  severity: block
  condition: '!input.content.contains("USER")'
  message: "USER directive required"
```

**Limitations when migrating:**
- CEL cannot generate configuration objects (no `generation_config`)
- CEL cannot inject templates
- CEL cannot filter knowledge base
- CEL has limited string operations (no full regex support)

For complex policies, continue using Rego.

## Best Practices

1. **Use Descriptive Names:** `require-non-root-user` not `rule1`
2. **Set Appropriate Severity:**
   - `block`: Security issues, compliance violations
   - `warn`: Best practices, recommended patterns
   - `suggest`: Nice-to-have optimizations
3. **Add Descriptions:** Help users understand why the rule exists
4. **Use Priority:** Higher priority violations appear first (0-100)
5. **Keep Conditions Simple:** CEL is for simple checks, not complex logic
6. **Test Thoroughly:** Validate against real Dockerfiles/manifests

## Further Reading

- [Policy Authoring Guide](../../docs/guides/policy-authoring.md) - Complete guide to policies
- [CEL Specification](https://github.com/google/cel-spec) - Official CEL language spec
- [Rego Examples](../README.md) - Rego policy examples for complex use cases

## Support

For questions or issues:
- GitHub Issues: https://github.com/azure/containerization-assist/issues
- Documentation: [Policy Authoring Guide](../../docs/guides/policy-authoring.md)
