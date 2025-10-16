# Policy System Guide

## Overview

The policy system enables enforcement of security, quality, and compliance rules across your containerization workflow. Policies are defined in YAML files and evaluated automatically during tool execution.

**Key Features:**
- **Auto-loading:** Policies are automatically discovered from the `policies/` directory
- **All tools:** Applied to all 17 MCP tools via the orchestrator
- **Shipped with package:** Policies are included in the npm package and ready to use
- **Override support:** Optional environment variable to select specific policies
- **Type-safe:** Validated using Zod schemas at load time

## Quick Start

**Policies are auto-loaded** from the `policies/` directory. All `.yaml` files are discovered and automatically merged into a unified policy.

```bash
# Default: Auto-loads and merges ALL policies from policies/ directory
npm start

# Override with specific policy path (optional, disables auto-discovery)
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml

# Or specify when creating orchestrator
npm start -- --policy ./policies/security-baseline.yaml
```

**Auto-discovered policies** (merged alphabetically):
1. `policies/base-images.yaml`
2. `policies/container-best-practices.yaml`
3. `policies/security-baseline.yaml`

All rules from all policies are combined. If multiple policies have rules with the same ID, the later policy's rule overrides the earlier one.

## Available Policies

### Security Baseline (`policies/security-baseline.yaml`)
Essential security rules for production containerization:
- **Blocks**: Untrusted registries, root users, privileged containers, host network, secrets in env vars
- **Warns**: Missing security scans, unapproved scanners
- **Suggests**: Read-only root filesystem
- **Use for**: Production deployments, security-conscious environments

### Base Images (`policies/base-images.yaml`)
Base image governance and optimization:
- **Blocks**: `:latest` tag, untagged images, deprecated versions
- **Warns**: Oversized base images, EOL versions
- **Suggests**: Microsoft Azure Linux (enterprise), Alpine/slim variants, distroless images, digest pinning
- **Use for**: Enterprise deployments, image optimization, reproducible builds, supply chain security

### Container Best Practices (`policies/container-best-practices.yaml`)
Docker best practices for production readiness:
- **Warns**: Missing HEALTHCHECK, missing WORKDIR, apt-get upgrade usage, sudo in containers
- **Suggests**: Multi-stage builds, layer optimization, dependency caching, .dockerignore
- **Use for**: Improving Dockerfile quality, reducing image size, production readiness

## Policy Format

```yaml
version: '2.0'
metadata:
  name: Policy Name
  description: Policy description
  category: security|quality|compliance
  author: your-org

defaults:
  enforcement: strict|warn  # Default enforcement level

rules:
  - id: unique-rule-id
    category: security|quality|optimization|best-practice
    priority: 100  # Higher = more important
    description: Human-readable description
    conditions:
      - kind: regex|function
        pattern: regex-pattern  # For kind: regex
        flags: i|m|s           # Optional regex flags (i=case-insensitive, m=multiline, s=dotall)
        # OR
        name: function-name     # For kind: function
        args: [arg1, arg2]     # Function arguments
    actions:
      block: true|false   # Prevents action with error
      warn: true|false    # Shows warning but allows
      suggest: true|false # Provides suggestion
      message: User-facing error/warning message
```

## Condition Types

### Regex Pattern Matching
Match text patterns using regular expressions:
```yaml
- kind: regex
  pattern: 'FROM.*:latest'
  flags: m  # Optional: i (case-insensitive), m (multiline), s (dotall)
```

**Common patterns:**
```yaml
# Block latest tag
pattern: ':latest\s*$'
flags: m

# Detect secrets in env vars (case-insensitive)
pattern: '(password|secret|api[_-]?key|token).*=.*\S+'
flags: i

# Block root user
pattern: '^USER\s+(root|0)\s*$'
flags: m

# Detect privileged containers
pattern: 'privileged:\s*true'

# Detect apt-get upgrade usage
pattern: 'apt-get\s+(upgrade|dist-upgrade)'
```

### Function Matching
Use predefined validation functions:
```yaml
- kind: function
  name: hasPattern
  args: ['^USER\s+\w+', 'm']  # Pattern and flags
```

**Available functions:**
- `hasPattern(pattern, flags)` - Check if content matches regex pattern
- Additional functions can be added via the policy evaluation engine

## Enforcement Levels

### Block (Strict Enforcement)
- **Effect**: Prevents action, returns error
- **Use for**: Security violations, compliance requirements, critical issues
- **Example**: Blocking root users, untrusted registries, privileged containers

### Warn
- **Effect**: Shows warning but allows action to proceed
- **Use for**: Best practices, quality improvements, non-critical issues
- **Example**: Missing HEALTHCHECK, outdated base images

### Suggest
- **Effect**: Provides informational suggestion
- **Use for**: Optimization opportunities, alternative approaches
- **Example**: Recommending Alpine variants, multi-stage builds

## Rule Priority

Rules are evaluated by priority (higher first):
- **90-100**: Critical security issues
- **70-89**: Important quality/security rules
- **50-69**: Best practices and optimization
- **<50**: Minor suggestions and documentation

## Multiple Policies

**All policies are automatically merged!** When multiple `.yaml` files exist in `policies/`, they are loaded in alphabetical order and merged into a unified policy.

**Merging behavior:**
- All rules from all policies are combined
- Rules with duplicate IDs: later policies override earlier ones
- Defaults are merged: later policies override earlier defaults
- Final rule list is sorted by priority (descending)

To use only a specific policy (disabling auto-merge), set the environment variable:

```bash
# Use only security baseline (disables auto-discovery and merging)
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml
```

## Environment-Specific Policies

Create environment-specific policy variants:

```yaml
# policies/production.yaml
version: '2.0'
metadata:
  name: Production Policy
defaults:
  enforcement: strict  # Strict in production

environments:
  development:
    defaults:
      enforcement: warn  # Relaxed in dev
```

Specify environment:
```bash
export CONTAINERIZATION_ASSIST_POLICY_ENVIRONMENT=development
```

## Policy Validation

Validate your policies before use:

```bash
# Policies are automatically validated on load
# Use the --validate flag to check configuration
containerization-assist-mcp --validate

# Or test with a specific policy
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/my-policy.yaml
containerization-assist-mcp --validate
```

Policies are validated against the schema on load. Invalid policies will be rejected with clear error messages.

## Package Distribution

**Policies are included in the npm package** via the `files` array in `package.json`:

```json
"files": [
  "dist/**/*",
  "dist-cjs/**/*",
  "policies/**/*.yaml",
  "knowledge/**/*.json"
]
```

When you install `containerization-assist-mcp`, all production-ready policies are available immediately in the `policies/` directory. No additional setup required.

## How Policies Are Applied

Policies are enforced by the **tool orchestrator** (`src/app/orchestrator.ts`) which:

1. **Auto-discovers** all `.yaml` files in `policies/` directory at startup
2. **Loads and merges** all discovered policies into a unified policy (can be overridden via env var)
3. **Validates** tool parameters using Zod schemas
4. **Evaluates** merged policy rules against tool inputs before execution
5. **Blocks or warns** based on rule actions (block/warn/suggest)
6. **Returns** actionable error messages with guidance when policies block

All 17 MCP tools are automatically protected by the merged policy system. No tool-specific configuration needed.

**Implementation details:**
- Policy merging: `src/app/orchestrator.ts:54-100` (mergePolicies function)
- Policy evaluation: `src/app/orchestrator.ts:176-189` (executeWithOrchestration)

## Best Practices

1. **Start with warnings**: Begin with `enforcement: warn` to understand impact before enforcing
2. **Choose the right policy**: Select the policy that best matches your use case (security, base images, or best practices)
3. **Custom policies**: Create custom policies by copying and modifying the provided examples
4. **Document rules**: Include clear `description` and `message` fields in your rules
5. **Test policies**: Validate policies with real Dockerfiles before deploying
6. **Version control**: Keep policies in version control with your infrastructure code
7. **Review regularly**: Update policies as security best practices evolve

## Common Use Cases

### Default (All Policies Merged)
```bash
# Automatically merges all 3 policies:
# - base-images.yaml
# - container-best-practices.yaml
# - security-baseline.yaml
npm start
```

This gives you comprehensive coverage: security rules + base image governance + Docker best practices.

### Security-Only Mode
```bash
# Use only security baseline (no base image or best practice rules)
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml
npm start
```

### Base Images Only
```bash
# Use only base image governance
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/base-images.yaml
npm start
```

### Development (Relaxed Enforcement)
```bash
# Use best practices only with relaxed enforcement
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/container-best-practices.yaml
export CONTAINERIZATION_ASSIST_POLICY_ENVIRONMENT=development
npm start
```

## Examples

See the `policies/` directory for complete, production-ready policy examples:
- `security-baseline.yaml` - Essential security rules
- `base-images.yaml` - Base image governance
- `container-best-practices.yaml` - Docker best practices
