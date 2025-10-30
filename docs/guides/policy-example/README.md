# Custom Policy Example - Platform and Tag Enforcement

## Overview

This example policy enforces:
- **Platform**: All Dockerfiles must build for `linux/arm64`
- **Tag**: All Dockerfiles must include `LABEL tag="demo"`

## Files

- `platform-and-tag.rego` - The policy implementation
- `platform-and-tag_test.rego` - Test suite (11 passing tests)

## Using with MCP (VS Code)

The policy is configured via environment variable in `.vscode/mcp.json`:

```json
{
  "servers": {
    "containerization-assist-dev": {
      "command": "npx",
      "args": ["tsx", "./src/cli/cli.ts"],
      "env": {
        "MCP_MODE": "true",
        "MCP_QUIET": "true",
        "NODE_ENV": "development",
        "CONTAINERIZATION_ASSIST_POLICY_PATH": "${workspaceFolder}/docs/guides/policy-example/platform-and-tag.rego"
      }
    }
  }
}
```

### How It Works

1. **Environment Variable**: `CONTAINERIZATION_ASSIST_POLICY_PATH` points to your custom policy
2. **VS Code Variable**: `${workspaceFolder}` resolves to your workspace root
3. **Policy Loading**: When the MCP server starts, it loads your custom policy **in addition to** built-in policies (they are merged together)

### Activating the Configuration

After updating `.vscode/mcp.json`:

1. **Reload VS Code**: Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on Mac) → "Developer: Reload Window"
2. **Restart MCP Server**: If using Claude Desktop or other MCP client, restart the application
3. **Verify**: The policy will now be enforced on all `generate-dockerfile` and `fix-dockerfile` operations

## Using with CLI

If running the MCP server directly from the command line:

```bash
# Set environment variable
export CONTAINERIZATION_ASSIST_POLICY_PATH="$PWD/docs/guides/policy-example/platform-and-tag.rego"

# Run the MCP server
npm run dev

# Or for production build
export CONTAINERIZATION_ASSIST_POLICY_PATH="$PWD/docs/guides/policy-example/platform-and-tag.rego"
containerization-assist-mcp
```

## Testing the Policy

```bash
# Run the test suite
opa test docs/guides/policy-example/platform-and-tag.rego \
         docs/guides/policy-example/platform-and-tag_test.rego -v

# Test with coverage
opa test docs/guides/policy-example/platform-and-tag.rego \
         docs/guides/policy-example/platform-and-tag_test.rego --coverage
```

## How Policy Validation Works

When `generate-dockerfile` runs:

1. **System detects your platform** automatically (e.g., `linux/amd64` on x64, `linux/arm64` on ARM)
2. **Generates pseudo-Dockerfile** with detected platform and default tag (`v1`)
3. **Your custom policy validates** against the pseudo-Dockerfile
4. **Reports violations** if detected platform or tag don't match your requirements

| Your System | Auto-Detected | Policy Requires | Result |
|-------------|---------------|-----------------|---------|
| Intel/AMD (x64) | `linux/amd64` | `linux/arm64` | ❌ Violation detected |
| Apple Silicon | `linux/arm64` | `linux/arm64` | ✅ Policy passes |
| ARM 32-bit | `linux/arm/v7` | `linux/arm64` | ❌ Violation detected |

**Note**: The policy enforces your requirements regardless of your system architecture. If you're on x64 but require arm64 images, the policy will catch this and report it.

## What This Policy Enforces

### ✅ Valid Dockerfile

```dockerfile
FROM --platform=linux/arm64 node:20-alpine

WORKDIR /app
LABEL tag="demo"

COPY package*.json ./
RUN npm ci --only=production

COPY . .
USER node

CMD ["node", "server.js"]
```

**Result**: Policy passes, Dockerfile is accepted

### ❌ Invalid Dockerfile - Missing Platform

```dockerfile
FROM node:20-alpine  # ❌ Missing --platform=linux/arm64

WORKDIR /app
LABEL tag="demo"
CMD ["node", "server.js"]
```

**Error**:
```
Rule: require-arm64-platform
Severity: block
Message: All FROM statements must specify --platform=linux/arm64.
         Example: FROM --platform=linux/arm64 node:20-alpine
```

### ❌ Invalid Dockerfile - Missing Tag Label

```dockerfile
FROM --platform=linux/arm64 node:20-alpine

WORKDIR /app
# ❌ Missing LABEL tag="demo"
CMD ["node", "server.js"]
```

**Error**:
```
Rule: require-demo-tag-label
Severity: block
Message: Dockerfile must include LABEL with tag=demo. Add: LABEL tag="demo"
```

## Multi-Stage Builds

All stages must include the platform specification:

```dockerfile
# Build stage
FROM --platform=linux/arm64 golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o app

# Runtime stage
FROM --platform=linux/arm64 alpine:latest
LABEL tag="demo"
COPY --from=builder /build/app /app
CMD ["/app"]
```

## Customizing the Policy

To modify the requirements:

### Change the Required Platform

Edit `platform-and-tag.rego`, line 52:

```rego
# Change from linux/arm64 to linux/amd64
not contains(line, "--platform=linux/amd64")
```

### Change the Required Tag

Edit `platform-and-tag.rego`, line 76:

```rego
# Change from "demo" to "production"
not regex.match(`(?mi)^LABEL\s+.*tag\s*=\s*["']?production["']?`, input.content)
```

### Add Additional Rules

Follow the pattern in the policy file:

```rego
# New rule: Require specific maintainer label
violations contains result if {
    input_type == "dockerfile"
    not regex.match(`(?mi)^LABEL\s+maintainer\s*=`, input.content)

    result := {
        "rule": "require-maintainer",
        "category": "compliance",
        "priority": 85,
        "severity": "block",
        "message": "Dockerfile must include LABEL maintainer=\"...\""
    }
}
```

## Troubleshooting

### Policy not loading

1. **Check the path**: Verify the file exists at the specified location
   ```bash
   ls -la docs/guides/policy-example/platform-and-tag.rego
   ```

2. **Validate syntax**: Check for Rego syntax errors
   ```bash
   opa check docs/guides/policy-example/platform-and-tag.rego
   ```

3. **Check environment variable**: Ensure it's set correctly
   ```bash
   echo $CONTAINERIZATION_ASSIST_POLICY_PATH
   ```

### VS Code not applying policy

1. **Reload window**: `Ctrl+Shift+P` → "Developer: Reload Window"
2. **Check MCP logs**: Look for policy loading messages in the output panel
3. **Verify workspace folder**: Ensure `${workspaceFolder}` resolves correctly

### Policy not enforcing rules

1. **Test the policy directly**:
   ```bash
   echo '{"content": "FROM node:20-alpine\nCMD []"}' | \
     opa eval -d docs/guides/policy-example/platform-and-tag.rego \
     -I --format pretty 'data.containerization.platform.result'
   ```

2. **Expected output** should show violations:
   ```json
   {
     "allow": false,
     "violations": [...]
   }
   ```

## Complete Example Workflow

1. **Configure MCP** (already done in `.vscode/mcp.json`)
2. **Reload VS Code** to pick up the new configuration
3. **Use containerization tools**:
   - When you run `generate-dockerfile`, the policy will validate the output
   - When you run `fix-dockerfile`, the policy will check for violations
4. **Review violations**: If the policy is violated, you'll see clear error messages
5. **Fix issues**: Update the Dockerfile to comply with the policy
6. **Retry**: The policy will allow compliant Dockerfiles to proceed

## Benefits of Custom Policy

- **Consistency**: Ensures all team members use the same platform and tagging conventions
- **Documentation**: Policy serves as living documentation of requirements
- **Automation**: Catches issues early in the development process
- **Flexibility**: Easy to customize for your organization's needs

## Additional Resources

- [Writing Rego Policies Guide](../writing-rego-policies.md)
- [OPA Documentation](https://www.openpolicyagent.org/docs/latest/)
- [Rego Language Reference](https://www.openpolicyagent.org/docs/latest/policy-language/)
