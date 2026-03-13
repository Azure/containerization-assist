# Policy Customization Examples

This directory contains pre-built example policies you can use to customize containerization behavior.

## Quick Start (60 seconds)

### Quick Start

Copy an example policy to your policy directory:

```bash
# Global policies (all projects)
mkdir -p ~/.config/containerization-assist/policy
cp node_modules/containerization-assist-mcp/policies.user.examples/allow-all-registries.rego \
   ~/.config/containerization-assist/policy/
# Policies are auto-reloaded on the next tool execution — no restart needed
```

Or use a custom location via `.vscode/mcp.json`:
### For NPM Installation Users

**Option 1: Use environment variable (30 seconds)**

```bash
# 1. Create a policies directory in your workspace
mkdir -p ~/.config/containerization-assist/policy

# 2. Copy example policy
cp node_modules/containerization-assist-mcp/policies.user.examples/allow-all-registries.rego \
   ~/.config/containerization-assist/policy/

# 3. Set environment variable in .vscode/mcp.json
{
  "servers": {
    "containerization-assist": {
      "command": "npx",
      "args": ["-y", "containerization-assist-mcp", "start"],
      "env": {
        "CUSTOM_POLICY_PATH": "${env:HOME}/.config/containerization-assist/policies"
      }
    }
  }
}

# 4. Policies are auto-reloaded on the next tool execution — no restart needed
```

## Available Examples

### 1. allow-all-registries.rego
**Purpose:** Override built-in Microsoft Container Registry preference
**Use Case:** Teams using Docker Hub, GCR, ECR, or private registries

```bash
cp policies.user.examples/allow-all-registries.rego .containerization-assist/policy/
```

### 2. warn-only-mode.rego
**Purpose:** Convert all blocking violations to warnings
**Use Case:** Testing policies, gradual adoption, development environments

```bash
cp policies.user.examples/warn-only-mode.rego .containerization-assist/policy/
```

### 3. custom-organization-template.rego
**Purpose:** Template for organization-specific policies
**Use Case:** Enforce custom labels, registries, naming conventions

```bash
cp policies.user.examples/custom-organization-template.rego .containerization-assist/policy/my-org-policy.rego
# Edit my-org-policy.rego to customize rules
```

## Testing Policies

```bash
# Test policy syntax and rules
opa test .containerization-assist/policy/

# Test specific policy file
opa test .containerization-assist/policy/allow-all-registries.rego

# Create test file (example: allow-all-registries_test.rego)
# See policies/security-baseline_test.rego for examples
```

## Policy Priority

Policies are merged in this order (later policies override earlier):

1. **Built-in policies/** (lowest priority)
   - security-baseline.rego
   - base-images.rego
   - container-best-practices.rego

2. **Global** `~/.config/containerization-assist/policy/` (low-medium priority)
   - User-wide customizations

3. **Project** `<git-root>/.containerization-assist/policy/` (medium-high priority)
   - Project-specific policies (tracked in git)

4. **Custom directory via `CUSTOM_POLICY_PATH`** (highest priority)
   - Organization-wide policies

## Writing Custom Policies

See `custom-organization-template.rego` for a complete template with examples.

**Key Components:**

```rego
package containerization.your_namespace

# Blocking violations
violations contains result if {
  # Your condition
  result := {
    "rule": "rule-id",
    "category": "security",
    "priority": 95,
    "severity": "block",
    "message": "User-facing error message",
    "description": "Rule description",
  }
}

# Non-blocking warnings
warnings contains result if {
  # Your condition
  result := { /* same structure */ }
}

# Policy decision
default allow := false
allow if count(violations) == 0

result := {
  "allow": allow,
  "violations": violations,
  "warnings": warnings,
  "suggestions": [],
  "summary": {
    "total_violations": count(violations),
    "total_warnings": count(warnings),
    "total_suggestions": 0,
  },
}
```

## Troubleshooting

**Q: My policy isn't being loaded**

```bash
# Check logs for policy discovery
npx containerization-assist-mcp start --log-level debug | grep -i policy

# Verify file extension is .rego (not .txt or .rego.txt)
ls -la ~/.config/containerization-assist/policy/
# or
ls -la .containerization-assist/policy/

# Policies are auto-reloaded — verify with the ops tool's status action
```

**Q: Syntax error in my policy**

```bash
# Validate syntax
opa check .containerization-assist/policy/my-policy.rego

# Run tests
opa test .containerization-assist/policy/
```

**Q: Built-in policies still blocking**

Custom policies must override specific rules by package namespace. See `allow-all-registries.rego` for examples.

## Documentation

- [OPA Rego Language Reference](https://www.openpolicyagent.org/docs/latest/policy-language/)
- [Built-in Policies Source](../policies/)
- [MCP Configuration Guide](../docs/guides/mcp-configuration.md)
