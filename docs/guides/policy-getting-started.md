# Policy Customization - Getting Started

Customize containerization policies using the priority-ordered policy system.

## Quickstart - Guided

The fastest way to create a policy is the **`create-containerization-policy`** tool. Run it from your MCP client (VS Code Copilot, Claude Desktop, etc.):

```
Use the create-containerization-policy tool to help me set up a policy
```

The tool walks you through each decision:

1. **Policy type** — Dockerfile (image) or Kubernetes manifest
2. **What to enforce** — base image allowlist, security rules, resource limits, etc.
3. **Severity** — `block` (reject non-compliant content) or `warn` (advisory only)
4. **Scope** — project-local (`.containerization-assist/policy/`) or global (`~/.config/containerization-assist/policy/`)

It generates a complete `.rego` file, saves it to the right location, and validates the syntax — no Rego knowledge required.

> **VS Code tip**: If `manage_todo_list` and `vscode_askQuestions` are available, the tool uses them for interactive progress tracking and selection prompts.

## Quickstart - Manual

1. Create a directory for your policies and add a `.rego` file:

```bash
mkdir -p ~/.config/containerization-assist/policies
```

2. Save a policy file to that directory, e.g. `~/.config/containerization-assist/policies/require-mcr-images.rego`:

```rego
# Rego policy: block Dockerfiles that use base images outside mcr.microsoft.com
package containerization.require_mcr

# Extract all FROM lines from the Dockerfile
from_lines := [line |
  line := split(input.content, "\n")[_]
  startswith(trim_space(line), "FROM ")
]

# Flag each FROM line that does not reference mcr.microsoft.com
violations contains result if {
  some line in from_lines
  not contains(line, "mcr.microsoft.com/")

  result := {
    "rule": "require-mcr-images",
    "category": "security",
    "priority": 95,
    "severity": "block",
    "message": sprintf("Base image must come from mcr.microsoft.com: %s", [trim_space(line)]),
  }
}

default allow := false
allow if count(violations) == 0
result := { "allow": allow, "violations": violations }
```

3. Point the server to your policies directory in `.vscode/mcp.json`:

```json
{
  "servers": {
    "containerization-assist": {
      "env": {
        "CUSTOM_POLICY_PATH": "${env:HOME}/.config/containerization-assist/policies"
      }
    }
  }
}
```

4. Policies are auto-reloaded on the next tool execution — no restart needed.

## Policy Priority

Policies are discovered and merged from four locations in priority order:

1. **Built-in policies/** (lowest priority) - Base security and quality rules
2. **~/.config/containerization-assist/policy/** (low-medium priority) - Global user customizations
3. **`<git-root>/.containerization-assist/policy/`** (medium-high priority) - Project-specific policies
4. **`CUSTOM_POLICY_PATH` environment variable** (highest priority) - Custom location

Later policies override earlier policies by package namespace.

## Common Use Cases

### Allow All Container Registries

Override built-in MCR preference to allow Docker Hub, GCR, ECR, etc.

```bash
# From npm package
cp node_modules/containerization-assist-mcp/policies.user.examples/allow-all-registries.rego \
   .containerization-assist/policy/
# Policies are auto-reloaded on the next tool execution — no restart needed
```

### Advisory-Only Mode

Convert all blocking violations to warnings for testing or development.

```bash
# From npm package
cp node_modules/containerization-assist-mcp/policies.user.examples/warn-only-mode.rego \
   .containerization-assist/policy/
# Policies are auto-reloaded on the next tool execution — no restart needed
```

### Organization-Specific Rules

Create custom policies for your organization's requirements.

```bash
# From npm package
cp node_modules/containerization-assist-mcp/policies.user.examples/custom-organization-template.rego \
   .containerization-assist/policy/my-org-policy.rego
# Edit my-org-policy.rego to customize
# Policies are auto-reloaded on the next tool execution — no restart needed
```

## Testing Your Policies

### 1. List Discovered Policies

```bash
# List all discovered policies
npx containerization-assist-mcp list-policies

# Show merged policy result
npx containerization-assist-mcp list-policies --show-merged
```

### 2. Check Discovery Logs

```bash
npx containerization-assist-mcp start --log-level debug 2>&1 | grep -i policy
```

Look for:
```
Discovered built-in policies: 3 files
Discovered project policies from .containerization-assist/policy/: 1 files
```

### 3. Test with Dockerfile Validation

```bash
echo 'FROM node:latest\nUSER root' > test.Dockerfile
# Use fix-dockerfile tool via your MCP client
```

## Troubleshooting

**Q: My custom policy isn't loading**

Check file extension (must be `.rego`):
```bash
ls -la .containerization-assist/policy/
# ✅ my-policy.rego
# ❌ my-policy.rego.txt or my-policy.yaml
```

Check discovery logs:
```bash
npx containerization-assist-mcp list-policies
```

**Q: Built-in policies still blocking**

Custom policies override by package namespace. See the `allow-all-registries.rego` example in `policies.user.examples/` for how to override built-in rules.

**Q: Changes not taking effect**

Policy file changes (additions, edits, deletions) are picked up automatically on the next tool execution — no restart needed. Restart is only required for environment variable or transport configuration changes.

**Q: Syntax error in my policy**

Validate policy syntax:
```bash
opa check .containerization-assist/policy/my-policy.rego
opa test .containerization-assist/policy/
```

## Reverting to Built-In Policies

```bash
# Remove project policies
rm -rf .containerization-assist/policy/

# Or remove global policies
rm -rf ~/.config/containerization-assist/policy/

# Remove environment variable from .vscode/mcp.json if set
# Restart only needed for env var or transport config changes (not policy file changes)
```

## Support

- [Policy Customization Examples](https://github.com/Azure/containerization-assist/tree/main/policies.user.examples) (also available at `node_modules/containerization-assist-mcp/policies.user.examples/`)
- [OPA Rego Documentation](https://www.openpolicyagent.org/docs/latest/)
- [GitHub Issues](https://github.com/Azure/containerization-assist/issues)
