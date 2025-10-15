# Policy System Guide

## Overview

The policy system enables enforcement of security, quality, and compliance rules across your containerization workflow. Policies are defined in YAML files and evaluated automatically during tool execution.

## Quick Start

```bash
# Set policy path via environment variable
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml

# Or specify when creating orchestrator
npm start -- --policy ./policies/security-baseline.yaml

# Apply multiple policies (colon-separated)
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml:./policies/base-images.yaml
```

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
      - kind: dockerfile-directive|dockerfile-content|kubernetes-manifest
        directive: FROM|RUN|USER|HEALTHCHECK|etc.
        pattern: regex-pattern  # Optional
        value: expected-value   # Optional
        missing: true|false     # Optional
    actions:
      block: true|false   # Prevents action with error
      warn: true|false    # Shows warning but allows
      suggest: true|false # Provides suggestion
      message: User-facing error/warning message
```

## Condition Types

### Dockerfile Directives
Match specific Dockerfile instructions:
```yaml
- kind: dockerfile-directive
  directive: FROM
  pattern: ':latest$'  # Matches images with :latest tag
```

### Dockerfile Content
Match patterns anywhere in Dockerfile:
```yaml
- kind: dockerfile-content
  pattern: 'apt-get\s+(upgrade|dist-upgrade)'
```

### Kubernetes Manifests
Validate Kubernetes configuration:
```yaml
- kind: kubernetes-manifest
  field: securityContext.privileged
  value: true
```

### Configuration
Check tool configuration:
```yaml
- kind: configuration
  field: scanner
  operator: not-in
  values: ['trivy', 'grype']
```

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

## Combining Policies

Apply multiple policies by separating paths with colons:

```bash
# Combine security + base images + best practices
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml:./policies/base-images.yaml:./policies/container-best-practices.yaml
```

Later policies can override earlier ones if rules have the same `id`.

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
# Run validation via orchestrator (automatically validates on load)
npm start -- --policy ./policies/my-policy.yaml --validate
```

Policies are validated against the schema on load. Invalid policies will be rejected with clear error messages.

## Best Practices

1. **Start with warnings**: Begin with `enforcement: warn` to understand impact before enforcing
2. **Combine policies**: Use multiple focused policies rather than one monolithic file
3. **Document rules**: Include clear `description` and `message` fields
4. **Test policies**: Validate policies with real Dockerfiles before deploying
5. **Version control**: Keep policies in version control with your infrastructure code
6. **Review regularly**: Update policies as security best practices evolve

## Common Use Cases

### Enforce Security Baseline
```bash
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml
```

### Optimize Image Size
```bash
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/base-images.yaml
```

### Production Readiness
```bash
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/security-baseline.yaml:./policies/container-best-practices.yaml
```

### Development (Relaxed)
```bash
export CONTAINERIZATION_ASSIST_POLICY_PATH=./policies/container-best-practices.yaml
export CONTAINERIZATION_ASSIST_POLICY_ENVIRONMENT=development
```

## Examples

See the `policies/` directory for complete, production-ready policy examples:
- `security-baseline.yaml` - Essential security rules
- `base-images.yaml` - Base image governance
- `container-best-practices.yaml` - Docker best practices
