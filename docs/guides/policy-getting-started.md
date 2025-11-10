# Policy Customization - Getting Started

This guide helps you customize containerization policies using the priority-ordered policy system.

## Quick Start (60 seconds)

### For Source Installation Users (10 seconds)

```bash
# Copy example policy to policies.user/
mkdir -p policies.user
cp policies.user.examples/allow-all-registries.rego policies.user/

# Restart your MCP client
```

### For NPM Installation Users (30 seconds)

```bash
# 1. Create custom policy directory
mkdir -p ~/.config/containerization-assist/policies

# 2. Copy example policy
cp node_modules/containerization-assist-mcp/policies.user.examples/allow-all-registries.rego \
   ~/.config/containerization-assist/policies/

# 3. Set environment variable in .vscode/mcp.json
{
  "servers": {
    "containerization-assist": {
      "env": {
        "CUSTOM_POLICY_PATH": "${env:HOME}/.config/containerization-assist/policies"
      }
    }
  }
}

# 4. Restart VS Code
```

## Policy Priority

Policies are discovered and merged from three locations in priority order:

1. **Built-in policies/** (lowest priority) - Base security and quality rules
2. **policies.user/** (middle priority) - Repository-specific customizations
3. **CUSTOM_POLICY_PATH** (highest priority) - Organization-wide policies

Later policies override earlier policies by package namespace.

## Common Use Cases

### Allow All Container Registries

Override built-in MCR preference to allow Docker Hub, GCR, ECR, etc.

```bash
cp policies.user.examples/allow-all-registries.rego policies.user/
# Restart MCP client
```

### Advisory-Only Mode

Convert all blocking violations to warnings for testing or development.

```bash
cp policies.user.examples/warn-only-mode.rego policies.user/
# Restart MCP client
```

### Organization-Specific Rules

Create custom policies for your organization's requirements.

```bash
cp policies.user.examples/custom-organization-template.rego policies.user/my-org-policy.rego
# Edit my-org-policy.rego to customize
# Restart MCP client
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
Discovered user policies from policies.user/: 1 files
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
ls -la policies.user/
# ✅ my-policy.rego
# ❌ my-policy.rego.txt or my-policy.yaml
```

Check discovery logs:
```bash
npx containerization-assist-mcp list-policies
```

**Q: Built-in policies still blocking**

Custom policies override by package namespace. See `policies.user.examples/allow-all-registries.rego` for examples of how to override built-in rules.

**Q: Changes not taking effect**

Restart your MCP client (VS Code, Claude Desktop, etc.) after modifying policies.

**Q: Syntax error in my policy**

Validate policy syntax:
```bash
opa check policies.user/my-policy.rego
opa test policies.user/
```

## Reverting to Built-In Policies

```bash
# Remove user policies
rm -rf policies.user/

# Remove environment variable from .vscode/mcp.json
# Restart MCP client
```

## Support

- [Policy Customization Examples](../../policies.user.examples/README.md)
- [OPA Rego Documentation](https://www.openpolicyagent.org/docs/latest/)
- [GitHub Issues](https://github.com/Azure/containerization-assist/issues)
