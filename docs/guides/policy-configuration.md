# Policy Configuration Guide

Comprehensive guide to configuring and using the containerization-assist policy system.

## Table of Contents

- [Overview](#overview)
- [Policy File Format](#policy-file-format)
- [Rule Components](#rule-components)
- [Enforcement Modes](#enforcement-modes)
- [Example Policies](#example-policies)
- [Using Policies](#using-policies)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

The policy system enables organizations to enforce security, quality, performance, and compliance rules across their containerization workflow. Policies are defined in YAML files and automatically discovered from the `policies/` directory.

### Key Features

- **Rule-Based System**: Define conditions and actions for policy enforcement
- **Multiple Matchers**: Regex patterns and built-in function matchers
- **Flexible Actions**: Block, warn, or suggest based on severity
- **Priority System**: Control which rules take precedence
- **Policy Merging**: Combine multiple policy files automatically
- **Category Organization**: Group rules by security, quality, performance, compliance

## Policy File Format

Every policy file follows this structure:

```yaml
version: '2.0'  # Required: Policy schema version

metadata:
  name: Policy Name
  description: What this policy enforces
  category: security  # security, quality, performance, compliance
  author: your-organization

defaults:
  enforcement: strict  # strict, advisory, or lenient
  cache_ttl: 300  # Cache timeout in seconds

  # Optional: Security defaults
  security:
    nonRootUser: true
    scanners:
      required: true
      tools: ['trivy']

  # Optional: Registry restrictions
  registries:
    allowed:
      - docker.io
      - gcr.io
      - mcr.microsoft.com
    blocked:
      - '*localhost*'
      - '*:5000*'

rules:
  - id: unique-rule-id
    category: security
    priority: 95
    description: Human-readable description
    conditions:
      - kind: regex
        pattern: 'FROM\s+[^:]+:latest'
        flags: im
    actions:
      block: true
      message: Error message shown to user
```

## Rule Components

### Conditions

Conditions determine when a rule matches. All conditions must match (AND logic).

#### Regex Matcher

Match patterns in Dockerfile content:

```yaml
conditions:
  - kind: regex
    pattern: 'FROM\s+node:latest'
    flags: im  # i=case-insensitive, m=multiline
    count_threshold: 3  # Optional: must match at least N times
```

**Common Patterns:**
- Block :latest tags: `FROM\s+[^:]+:latest`
- Find USER directives: `^USER\s+(\w+)$`
- Detect secrets: `(API_KEY|SECRET|PASSWORD|TOKEN)\s*=`
- Check EXPOSE: `^EXPOSE\s+\d+$`

#### Function Matchers

Built-in functions for common checks:

```yaml
# Check if pattern exists
conditions:
  - kind: function
    name: hasPattern
    args: ['^HEALTHCHECK', 'im']

# Check if file exists
conditions:
  - kind: function
    name: fileExists
    args: ['docker-compose.yml']

# Check size threshold
conditions:
  - kind: function
    name: largerThan
    args: [1048576]  # 1MB in bytes

# Check for vulnerabilities
conditions:
  - kind: function
    name: hasVulnerabilities
    args: [['HIGH', 'CRITICAL']]
```

### Actions

Actions define what happens when a rule matches:

```yaml
actions:
  block: true              # Prevents build/deployment
  message: 'Error message' # User-facing explanation
```

```yaml
actions:
  warn: true
  message: 'Warning message'  # Logged but doesn't fail
```

```yaml
actions:
  suggest: true
  message: 'Consider using...'  # Recommendation only
```

**Other Action Properties:**
- `block_deployment: true` - Block deployment specifically
- `block_build: true` - Block build specifically
- `require_approval: true` - Flag for manual approval
- `severity: 'critical'` - Severity level for reporting

### Priority

Priority controls rule importance and evaluation order:

- **90-100**: Security rules (highest priority)
- **70-89**: Quality rules
- **50-69**: Performance rules
- **30-49**: Compliance rules

Higher priority rules are evaluated first.

### Category

Group rules by type:
- `security` - Security vulnerabilities and risks
- `quality` - Code quality and best practices
- `performance` - Image size and efficiency
- `compliance` - Organizational standards

## Enforcement Modes

Set in `defaults.enforcement`:

### Strict Mode

All rules enforced, violations block operations:

```yaml
defaults:
  enforcement: strict
```

**Use cases:** Production environments, CI/CD pipelines

### Advisory Mode

Rules evaluated, violations logged but don't block:

```yaml
defaults:
  enforcement: advisory
```

**Use cases:** Development, gradual policy rollout, informational feedback

### Lenient Mode

Minimal enforcement, warnings only:

```yaml
defaults:
  enforcement: lenient
```

**Use cases:** Local development, experimentation

## Example Policies

### Security-First Policy

```yaml
version: '2.0'
metadata:
  name: Production Security Policy
  category: security

defaults:
  enforcement: strict
  security:
    nonRootUser: true
    scanners:
      required: true
      tools: ['trivy']

rules:
  - id: block-root-user
    category: security
    priority: 95
    description: Containers must not run as root
    conditions:
      - kind: regex
        pattern: '^USER\s+(root|0)\s*$'
        flags: m
    actions:
      block: true
      message: 'Running as root is not allowed. Add USER directive with non-root user.'

  - id: block-secrets
    category: security
    priority: 100
    description: Prevent hardcoded secrets
    conditions:
      - kind: regex
        pattern: '(API_KEY|SECRET|PASSWORD|TOKEN)\s*='
        flags: i
    actions:
      block: true
      message: 'Hardcoded secrets detected. Use environment variables or secrets management.'

  - id: require-healthcheck
    category: quality
    priority: 75
    conditions:
      - kind: function
        name: hasPattern
        args: ['^HEALTHCHECK', 'im']
    actions:
      warn: true
      message: 'HEALTHCHECK directive recommended for production containers.'
```

### Base Image Governance

```yaml
version: '2.0'
metadata:
  name: Base Image Policy
  category: quality

defaults:
  enforcement: advisory

rules:
  - id: block-latest-tag
    category: quality
    priority: 80
    description: Prevent :latest for reproducibility
    conditions:
      - kind: regex
        pattern: 'FROM\s+[^:]+:latest'
        flags: im
    actions:
      block: true
      message: 'Using :latest tag is not allowed. Specify explicit version tags.'

  - id: recommend-alpine
    category: performance
    priority: 60
    description: Recommend Alpine variants
    conditions:
      - kind: regex
        pattern: 'FROM\s+(node|python):(?!.*alpine)'
        flags: im
    actions:
      suggest: true
      message: 'Consider using Alpine variant for smaller image size (e.g., node:20-alpine).'

  - id: block-deprecated-node
    category: quality
    priority: 90
    description: Block deprecated Node.js versions
    conditions:
      - kind: regex
        pattern: 'FROM\s+node:(8|10|12|14|16)\b'
        flags: im
    actions:
      block: true
      message: 'Deprecated Node.js version detected. Use Node.js 18 or higher.'
```

### Performance Optimization

```yaml
version: '2.0'
metadata:
  name: Performance Policy
  category: performance

defaults:
  enforcement: advisory

rules:
  - id: excessive-run-commands
    category: performance
    priority: 55
    description: Detect excessive RUN commands
    conditions:
      - kind: regex
        pattern: '^RUN\s+'
        flags: im
        count_threshold: 6
    actions:
      suggest: true
      message: 'Consider combining RUN commands with && to reduce layers and image size.'

  - id: apt-cleanup-missing
    category: performance
    priority: 60
    description: Recommend apt cache cleanup
    conditions:
      - kind: regex
        pattern: 'apt-get\s+install'
        flags: im
    actions:
      suggest: true
      message: 'Clean apt cache after installation: RUN apt-get update && apt-get install -y <packages> && rm -rf /var/lib/apt/lists/*'

  - id: apk-no-cache
    category: performance
    priority: 60
    description: Recommend --no-cache with apk
    conditions:
      - kind: regex
        pattern: 'apk\s+add(?!\s+--no-cache)'
        flags: im
    actions:
      suggest: true
      message: 'Use apk add --no-cache to avoid caching package indexes.'
```

## Using Policies

### Automatic Discovery

By default, all `.yaml` files in `policies/` are automatically loaded:

```bash
# All policies in policies/ directory are used
npx containerization-assist validate-dockerfile --path ./Dockerfile
```

### Specific Policy File

Use a single policy file:

```bash
# Via command-line flag
npx containerization-assist validate-dockerfile \
  --path ./Dockerfile \
  --policy-path ./policies/production.yaml

# Via environment variable
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/production.yaml
npx containerization-assist validate-dockerfile --path ./Dockerfile
```

### MCP Server Configuration

Configure in Claude Desktop:

```json
{
  "mcpServers": {
    "containerization-assist": {
      "command": "npx",
      "args": [
        "-y",
        "containerization-assist-mcp",
        "start",
        "--config",
        "./policies/security-baseline.yaml"
      ]
    }
  }
}
```

### Policy Merging

When multiple policies are discovered:

1. **Rules merged by ID**: Later policy (alphabetically) overrides earlier
2. **Defaults merged**: Later policy overrides earlier
3. **Rules sorted by priority**: Highest priority first

Example:
```
policies/
├── 01-security.yaml       # Loaded first
├── 02-quality.yaml        # Loaded second
└── 99-overrides.yaml      # Loaded last (highest precedence)
```

## Best Practices

### 1. Organize Policies by Concern

Create separate policy files for different concerns:
- `security.yaml` - Security rules only
- `quality.yaml` - Code quality rules
- `performance.yaml` - Optimization rules
- `compliance.yaml` - Organizational standards

### 2. Use Descriptive IDs

```yaml
# Good
id: block-latest-tag
id: require-healthcheck
id: prevent-root-user

# Bad
id: rule1
id: check-base-image
```

### 3. Provide Clear Messages

```yaml
# Good
message: 'Using :latest tag is not allowed. Specify explicit version tags (e.g., node:20.11-alpine).'

# Bad
message: 'Invalid tag'
```

### 4. Set Appropriate Priorities

- Reserve 95-100 for critical security issues
- Use 70-89 for important quality checks
- Use 50-69 for recommendations

### 5. Test Policies

Validate your policies before deployment:

```bash
# Test against sample Dockerfiles
npx containerization-assist validate-dockerfile \
  --path ./test-fixtures/Dockerfile.valid \
  --policy-path ./policies/new-policy.yaml

# Check policy merging
npx containerization-assist validate-dockerfile \
  --path ./Dockerfile
# (uses all policies in policies/)
```

### 6. Version Control Policies

Commit policies to version control:
```bash
git add policies/
git commit -m "Add security baseline policy"
```

### 7. Environment-Specific Policies

Use different policies for different environments:

```bash
# Development: lenient
policies/dev/base-images.yaml

# Staging: advisory
policies/staging/security.yaml

# Production: strict
policies/prod/security.yaml
policies/prod/compliance.yaml
```

## Troubleshooting

### Policy Not Loading

**Symptom:** Policy file not being applied

**Solutions:**
1. Check file extension is `.yaml` or `.yml`
2. Verify file is in `policies/` directory
3. Check YAML syntax:
   ```bash
   npx js-yaml policies/your-policy.yaml
   ```
4. Check logs for validation errors:
   ```bash
   LOG_LEVEL=debug npx containerization-assist validate-dockerfile --path ./Dockerfile
   ```

### Rules Not Matching

**Symptom:** Rule conditions don't match expected content

**Solutions:**
1. Test regex patterns separately:
   ```javascript
   const pattern = /FROM\s+node:latest/im;
   const content = 'FROM node:latest';
   console.log(pattern.test(content)); // Should be true
   ```
2. Check regex flags (i, m, g)
3. Verify condition kind is correct (`regex` vs `function`)
4. Check for escaping issues in YAML strings

### Policy Conflicts

**Symptom:** Multiple policies with conflicting rules

**Solutions:**
1. Use unique rule IDs across all policies
2. Higher priority rules take precedence
3. Later policies (alphabetically) override earlier ones for same ID
4. Use single policy file to avoid conflicts:
   ```bash
   --policy-path ./policies/production.yaml
   ```

### Performance Issues

**Symptom:** Slow policy evaluation

**Solutions:**
1. Reduce number of regex patterns
2. Avoid complex regex with backtracking
3. Use `count_threshold` only when necessary
4. Consider caching (adjust `cache_ttl` in defaults)

## Advanced Topics

### Custom Function Matchers

The policy system supports these built-in function matchers:
- `hasPattern` - Check if regex pattern exists
- `fileExists` - Check if file exists in context
- `largerThan` - Check if content/file exceeds size
- `hasVulnerabilities` - Check vulnerability scan results

Future versions may support custom function matchers.

### Policy Inheritance

Policies can build on each other:

```yaml
# Base policy (01-base.yaml)
defaults:
  enforcement: advisory

rules:
  - id: base-rule-1
    # ...

# Override policy (99-production-overrides.yaml)
defaults:
  enforcement: strict  # Override enforcement mode

rules:
  - id: base-rule-1  # Override base rule
    # Different configuration
```

### Integration with CI/CD

```yaml
# .github/workflows/docker.yml
- name: Validate Dockerfile
  run: |
    npx containerization-assist validate-dockerfile \
      --path ./Dockerfile \
      --policy-path ./policies/ci.yaml

    if [ $? -ne 0 ]; then
      echo "Policy validation failed"
      exit 1
    fi
```

## Reference

### Available Policy Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | string | Yes | Policy schema version (must be '2.0') |
| `metadata.name` | string | No | Policy display name |
| `metadata.description` | string | No | Policy description |
| `metadata.category` | string | No | Policy category |
| `metadata.author` | string | No | Policy author |
| `defaults.enforcement` | enum | No | Enforcement mode (strict/advisory/lenient) |
| `defaults.cache_ttl` | number | No | Cache timeout in seconds |
| `rules` | array | Yes | List of policy rules |
| `rules[].id` | string | Yes | Unique rule identifier |
| `rules[].category` | enum | No | Rule category |
| `rules[].priority` | number | Yes | Rule priority (1-100) |
| `rules[].description` | string | No | Rule description |
| `rules[].conditions` | array | Yes | List of conditions (AND logic) |
| `rules[].actions` | object | Yes | Actions to take when matched |

### Condition Types

| Type | Fields | Description |
|------|--------|-------------|
| `regex` | `pattern`, `flags`, `count_threshold` | Pattern matching |
| `function` | `name`, `args` | Built-in function matcher |

### Action Types

| Action | Type | Effect |
|--------|------|--------|
| `block` | boolean | Prevents build/deployment |
| `warn` | boolean | Logs warning |
| `suggest` | boolean | Provides recommendation |
| `message` | string | User-facing message |
| `severity` | string | Severity level |

---

For more examples, see the existing policies in the [`policies/`](../../policies/) directory:
- [`base-images.yaml`](../../policies/base-images.yaml)
- [`security-baseline.yaml`](../../policies/security-baseline.yaml)
- [`container-best-practices.yaml`](../../policies/container-best-practices.yaml)
