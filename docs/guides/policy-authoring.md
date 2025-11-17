# Policy Authoring Guide

**Version:** 4.0.0
**Last Updated:** Sprint 6 (CEL Support Added)
**Audience:** Platform engineers, DevOps teams, policy authors

This guide provides comprehensive documentation on writing custom policies for containerization-assist using **OPA Rego** or **CEL (Common Expression Language)**.

---

## Table of Contents

1. [Overview](#overview)
2. [Policy Formats: Rego vs CEL](#policy-formats-rego-vs-cel)
3. [CEL Quick Start](#cel-quick-start)
4. [Template Injection (Sprint 3)](#template-injection-sprint-3-)
5. [Policy Architecture](#policy-architecture)
6. [Phase-by-Phase Guide](#phase-by-phase-guide)
7. [Schema Reference](#schema-reference)
8. [Best Practices](#best-practices)
9. [Debugging](#debugging)
10. [Common Pitfalls](#common-pitfalls)
11. [Migration Guide: Rego → CEL](#migration-guide-rego--cel)

---

## Overview

### What Are Policies?

Policies in containerization-assist are OPA Rego modules that control and customize the containerization workflow. They enable you to:

- **Pre-configure** tool behavior before generation
- **Filter and prioritize** knowledge recommendations
- **Inject** organization-specific templates
- **Validate** generated artifacts against compliance rules
- **Customize** behavior by environment, language, cloud provider, etc.

### Policy Lifecycle

```
┌─────────────────┐
│ Input (Tool     │
│ + Context)      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 1. Pre-Gen      │ generation_config
│ Configuration   │ (Set defaults, constraints)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 2. Generation   │ knowledge_filtering, templates
│ Time            │ (Filter/inject recommendations)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 3. Post-Gen     │ validation_rules
│ Validation      │ (Check compliance)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Output          │
│ (Validated)     │
└─────────────────┘
```

### File Structure

**Rego Policies:**
```
my-policy.rego          # Rego policy implementation
my-policy_test.rego     # OPA test suite (required)
```

**CEL Policies:**
```
my-policy.yaml          # CEL policy (YAML format)
# or
my-policy.yml           # CEL policy (YML format)
```

---

## Policy Formats: Rego vs CEL

containerization-assist supports two policy formats: **Rego** and **CEL**. Both formats can be used interchangeably, and you can even mix them in the same deployment.

### Comparison Table

| Feature | Rego | CEL |
|---------|------|-----|
| **Language** | OPA Rego (domain-specific) | CEL (Google's expression language) |
| **File Format** | `.rego` files | `.yaml` or `.yml` files |
| **Complexity** | High (full programming language) | Low (simple expressions) |
| **Learning Curve** | Steep | Gentle |
| **Use Case** | Complex logic, computations, transformations | Simple validation rules |
| **Performance** | Fast (WASM compilation) | Very fast (native evaluation) |
| **Configuration Generation** | ✅ Supported (`generation_config`) | ❌ Not supported |
| **Template Injection** | ✅ Supported (`templates`) | ❌ Not supported |
| **Knowledge Filtering** | ✅ Supported (`knowledge_filtering`) | ❌ Not supported |
| **Validation Rules** | ✅ Supported (`violations`, `warnings`, `suggestions`) | ✅ Supported |
| **Testing** | `opa test` (built-in) | Manual testing |
| **Debugging** | `opa eval` with tracing | Expression evaluation |

### When to Use Each Format

**Use Rego when you need:**
- Complex logic and computations
- Configuration generation (`generation_config`)
- Template injection
- Knowledge filtering
- Advanced rule composition
- Built-in testing framework

**Use CEL when you need:**
- Simple validation rules
- Quick policy prototyping
- Easy-to-read policies for non-developers
- Fast evaluation performance
- No external dependencies (OPA binary not required)

### Hybrid Approach

You can combine both formats in a single deployment:
- **Built-in policies**: Rego only (optimized with WASM)
- **User policies** (`policies.user/`): Mix of Rego and CEL
- **Custom policies** (`CUSTOM_POLICY_PATH`): Mix of Rego and CEL

The system automatically detects the format based on file extension and merges results.

---

## CEL Quick Start

### Basic CEL Policy Structure

CEL policies are defined in YAML format with a specific schema:

```yaml
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: my-policy
  description: Human-readable policy description
spec:
  rules:
    - name: rule-name
      category: security|best-practices|optimization|health
      severity: block|warn|suggest
      condition: 'CEL expression that returns true for violations'
      message: "User-facing error message"
      description: "Detailed explanation (optional)"
      priority: 50  # Optional: 0-100, higher = more important
```

### Example: Require Non-Root User

```yaml
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: security-baseline
  description: Basic security requirements
spec:
  rules:
    - name: require-non-root-user
      category: security
      severity: block
      condition: '!input.content.contains("USER") || input.content.contains("USER root")'
      message: "Dockerfile must specify a non-root USER directive"
      description: "Running containers as root is a security risk"
      priority: 90
```

### Example: Multiple Rules with Different Severities

```yaml
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: production-ready
  description: Production readiness checks
spec:
  rules:
    # Blocking rule
    - name: no-latest-tag
      category: best-practices
      severity: block
      condition: 'input.content.contains(":latest")'
      message: "Do not use :latest tag in production"
      priority: 100

    # Warning rule
    - name: recommend-healthcheck
      category: health
      severity: warn
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "Consider adding HEALTHCHECK for production deployments"
      priority: 70

    # Suggestion rule
    - name: suggest-multi-stage
      category: optimization
      severity: suggest
      condition: '!input.content.contains("AS builder")'
      message: "Consider using multi-stage builds to reduce image size"
      priority: 50
```

### CEL Expression Reference

CEL provides built-in string operations:

| Expression | Description | Example |
|------------|-------------|---------|
| `input.content.contains("text")` | Check if content contains substring | `input.content.contains("FROM")` |
| `input.content.size()` | Get content length | `input.content.size() > 0` |
| `!expr` | Logical NOT | `!input.content.contains("USER")` |
| `expr1 && expr2` | Logical AND | `input.content.contains("FROM") && !input.content.contains("USER")` |
| `expr1 \|\| expr2` | Logical OR | `input.content.contains("alpine") \|\| input.content.contains("ubuntu")` |

**Input Structure:**
- `input.content`: The content being validated (Dockerfile, K8s manifest, etc.)

### CEL Limitations

❌ **Not supported in CEL:**
- Configuration generation (`generation_config`)
- Template injection (`templates`)
- Knowledge filtering (`knowledge_filtering`)
- Complex computations and data transformations
- Custom functions
- Testing framework (must test manually)

✅ **Use Rego instead for these features**

---

## Template Injection (Sprint 3) ✅

Template injection is now fully functional and tested. Templates allow you to automatically inject organizational standards into generated artifacts.

### Quick Start

1. **Create a template policy** (`policies/my-templates.rego`):
   ```rego
   package containerization.templates

   import rego.v1

   ca_cert_template := {
     "id": "org-ca-certs",
     "section": "security",
     "description": "Install organization CA certificates",
     "content": "COPY certs/ca.crt /usr/local/share/ca-certificates/\nRUN update-ca-certificates",
     "priority": 100
   }

   dockerfile_templates contains ca_cert_template

   templates := {
     "dockerfile": [template | template := dockerfile_templates[_]],
     "kubernetes": []
   }
   ```

2. **Use the policy**:
   ```bash
   export CUSTOM_POLICY_PATH=policies/my-templates.rego
   containerization-assist generate-dockerfile --language node --environment production
   ```

3. **See templates in output**:
   - Templates appear in recommendations with `policyDriven: true`
   - Automatically injected without user intervention

For complete examples, see:
- [Template Injection Examples](../examples/template-injection-example.md)
- [Production Templates](../../policies.user.examples/production-ready/)

---

## Policy Architecture

### Basic Structure

Every policy follows this template:

```rego
# Policy header with metadata
package containerassist.my_policy

import rego.v1

# ============================================================================
# Configuration (Phase 1: Pre-Generation)
# ============================================================================

generation_config contains config if {
    input.tool == "generate-dockerfile"
    # Your pre-generation configuration logic
    config := {
        "baseImage": "node:20-alpine",
        "requireNonRoot": true,
    }
}

# ============================================================================
# Knowledge Filtering (Phase 2: Generation-Time)
# ============================================================================

knowledge_filtering contains filter if {
    # Filter knowledge recommendations
    filter := {
        "action": "exclude",
        "pattern": "*-deprecated-*",
        "reason": "Exclude deprecated patterns",
    }
}

# ============================================================================
# Templates (Phase 2: Generation-Time)
# ============================================================================

templates contains template if {
    # Inject organization-specific templates
    template := {
        "id": "my-org-template",
        "category": "security",
        "recommendation": "Add company CA certificates",
        "code_snippet": "COPY ca-certs.pem /etc/ssl/certs/",
        "policyDriven": true,
    }
}

# ============================================================================
# Validation (Phase 3: Post-Generation)
# ============================================================================

validation_rules contains rule if {
    # Validate generated content
    input.content != null
    # Your validation logic
    rule := {
        "level": "error",  # or "warning", "info"
        "message": "Validation failed",
        "suggestion": "How to fix it",
    }
}

# ============================================================================
# Metadata
# ============================================================================

metadata := {
    "name": "My Policy",
    "version": "1.0.0",
    "description": "Policy description",
}
```

---

## Phase-by-Phase Guide

### Phase 1: Pre-Generation Configuration

**When:** Before tool execution starts
**Purpose:** Set defaults, constraints, and configuration
**Returns:** Configuration object

#### Example: Dockerfile Generation Config

```rego
generation_config contains config if {
    input.tool == "generate-dockerfile"
    input.environment == "production"

    config := {
        "baseImage": "gcr.io/distroless/nodejs20-debian12",
        "requireNonRoot": true,
        "requireHealthCheck": true,
        "enableMultiStage": true,
        "optimizationLevel": "aggressive",
    }
}
```

#### Example: Kubernetes Generation Config

```rego
generation_config contains config if {
    input.tool == "generate-k8s-manifests"

    # Calculate resource limits based on tier
    tier_cpu := tier_cpu_limits[input.tier]
    tier_memory := tier_memory_limits[input.tier]

    config := {
        "resources": {
            "requests": {
                "cpu": sprintf("%dm", [tier_cpu * 0.5]),
                "memory": sprintf("%dMi", [tier_memory * 0.75]),
            },
            "limits": {
                "cpu": sprintf("%dm", [tier_cpu]),
                "memory": sprintf("%dMi", [tier_memory]),
            },
        },
        "replicas": tier_replicas[input.tier],
        "enableHPA": input.tier != "starter",
    }
}

# Helper data structures
tier_cpu_limits := {"starter": 500, "pro": 2000, "enterprise": 8000}
tier_memory_limits := {"starter": 512, "pro": 2048, "enterprise": 8192}
tier_replicas := {"starter": 1, "pro": 3, "enterprise": 5}
```

#### Available Configuration Keys

**Dockerfile:**
- `baseImage`: Override default base image
- `requireNonRoot`: Enforce non-root user
- `requireHealthCheck`: Mandate HEALTHCHECK directive
- `enableMultiStage`: Force multi-stage builds
- `optimizationLevel`: "aggressive", "balanced", "quality"
- `includeDevTools`: Include development tools
- `includeBuildTools`: Include build-time dependencies

**Kubernetes:**
- `resources.requests`: CPU/memory requests
- `resources.limits`: CPU/memory limits
- `replicas`: Number of pod replicas
- `enableHPA`: Enable HorizontalPodAutoscaler
- `securityContext`: Pod security context
- `networkPolicy`: "required", "recommended", "optional"
- `podSecurityStandard`: "privileged", "baseline", "restricted"

---

### Phase 2a: Knowledge Filtering

**When:** During tool execution
**Purpose:** Filter/prioritize knowledge recommendations
**Returns:** Set of filter rules

#### Exclude Patterns

```rego
# Block deprecated recommendations
knowledge_filtering contains filter if {
    filter := {
        "action": "exclude",
        "pattern": "*-deprecated-*",
        "reason": "Deprecated patterns not allowed",
    }
}

# Environment-specific exclusions
knowledge_filtering contains filter if {
    input.environment == "production"
    filter := {
        "action": "exclude",
        "pattern": "debug-*",
        "reason": "Debug tools not allowed in production",
    }
}
```

#### Prioritize Patterns

```rego
# Boost security recommendations
knowledge_filtering contains filter if {
    filter := {
        "action": "prioritize",
        "tags": ["security", "hardening"],
        "weight": 2.0,  # 2x priority
        "reason": "Security is top priority",
    }
}

# Cloud-specific prioritization
knowledge_filtering contains filter if {
    input.cloudProvider == "aws"
    filter := {
        "action": "prioritize",
        "tags": ["ecr", "aws"],
        "weight": 1.5,
        "reason": "Prefer AWS-native solutions",
    }
}
```

#### Filter Actions

- `exclude`: Remove matching knowledge entries
- `prioritize`: Boost weight of matching entries
- `deprioritize`: Reduce weight of matching entries

#### Pattern Matching

- `*` wildcard: `"node-*"` matches `"node-security-scan"`
- Tag matching: `["security", "dockerfile"]`
- ID matching: `"dockerfile-user-root"`

---

### Phase 2b: Template Injection

**When:** During tool execution
**Purpose:** Add organization-specific recommendations
**Returns:** Set of templates to inject

#### Basic Template

```rego
templates contains template if {
    input.tool == "generate-dockerfile"

    template := {
        "id": "org-ca-certificates",
        "category": "security",
        "recommendation": "Install company CA certificates",
        "code_snippet": `# Company CA certificates
COPY certificates/ca-bundle.crt /etc/ssl/certs/company-ca.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/company-ca.crt`,
        "policyDriven": true,
        "priority": "high",
    }
}
```

#### Conditional Templates

```rego
# Only for production Java apps
templates contains template if {
    input.tool == "generate-dockerfile"
    input.environment == "production"
    lower(input.language) == "java"

    template := {
        "id": "org-java-observability",
        "category": "monitoring",
        "recommendation": "Add Datadog APM agent",
        "code_snippet": `# Datadog APM
RUN wget -O dd-java-agent.jar https://dtdg.co/latest-java-tracer
ENV JAVA_TOOL_OPTIONS=-javaagent:/app/dd-java-agent.jar`,
        "policyDriven": true,
    }
}
```

#### Template Structure

Required fields:
- `id`: Unique identifier
- `category`: "security", "optimization", "monitoring", etc.
- `recommendation`: Human-readable description
- `code_snippet`: Code to inject
- `policyDriven`: Always `true` for policy-injected templates

Optional fields:
- `priority`: "critical", "high", "medium", "low"
- `tags`: `["production", "java"]`
- `documentation`: Link to internal docs

---

### Phase 3: Post-Generation Validation

**When:** After content is generated
**Purpose:** Validate against compliance rules
**Returns:** Set of validation rules (violations/warnings)

#### Error Rules (Blocking)

```rego
validation_rules contains rule if {
    input.tool == "generate-dockerfile"
    input.content != null

    # Check for root user
    contains(lower(input.content), "user root")

    rule := {
        "level": "error",  # Blocks generation
        "message": "Root user detected in Dockerfile",
        "suggestion": "Add USER directive with non-root user (e.g., USER 65534)",
    }
}
```

#### Warning Rules (Non-blocking)

```rego
validation_rules contains rule if {
    input.tool == "generate-k8s-manifests"
    input.content != null

    # Check resource limits
    cpu_limit := parse_cpu(input.content.resources.limits.cpu)
    cpu_limit > 4000  # > 4 CPU

    rule := {
        "level": "warning",  # Doesn't block
        "message": sprintf("High CPU limit: %dm", [cpu_limit]),
        "suggestion": "Consider reducing CPU limit to save costs",
    }
}
```

#### Info Rules (Advisory)

```rego
validation_rules contains rule if {
    input.environment == "development"

    rule := {
        "level": "info",
        "message": "Running in development mode",
        "suggestion": "Remember to use production policy before deploying",
    }
}
```

---

## Schema Reference

### Input Schema

The `input` object contains tool context:

```rego
input := {
    # Required
    "tool": "generate-dockerfile" | "generate-k8s-manifests" | ...,

    # Common
    "environment": "development" | "staging" | "production",
    "language": "node" | "python" | "java" | "go" | ...,

    # Tool-specific
    "repositoryPath": "/path/to/repo",
    "targetPlatform": "linux/amd64",
    "name": "my-app",
    "version": "1.0.0",

    # Custom (your organization)
    "tier": "starter" | "professional" | "enterprise",
    "cloudProvider": "aws" | "gcp" | "azure",
    "region": "us-east-1",
    "teamId": "platform-team",

    # Post-generation only
    "content": "..." | {...},  # Generated artifact
}
```

### Output Schema

#### generation_config

```rego
config := {
    # Any key-value pairs
    "baseImage": "node:20",
    "resources": {...},
    ...
}
```

#### knowledge_filtering

```rego
filter := {
    "action": "exclude" | "prioritize" | "deprioritize",
    "pattern": "*-pattern-*",    # For pattern matching
    "tags": ["tag1", "tag2"],    # For tag matching
    "weight": 2.0,                # For prioritize/deprioritize
    "reason": "Why this filter",
}
```

#### templates

```rego
template := {
    "id": "unique-id",
    "category": "security" | "optimization" | ...,
    "recommendation": "Human description",
    "code_snippet": "Code to inject",
    "policyDriven": true,
    "priority": "critical" | "high" | "medium" | "low",  # Optional
    "tags": ["tag1"],              # Optional
    "documentation": "https://...", # Optional
}
```

#### validation_rules

```rego
rule := {
    "level": "error" | "warning" | "info",
    "message": "What went wrong",
    "suggestion": "How to fix it",
}
```

---

## Best Practices

### 1. Use Descriptive IDs

```rego
# ✅ Good
"id": "org-security-ca-certificates"

# ❌ Bad
"id": "template1"
```

### 2. Provide Helpful Messages

```rego
# ✅ Good
rule := {
    "level": "error",
    "message": "CPU limit (8000m) exceeds starter tier allowance (500m)",
    "suggestion": "Reduce CPU limit to 500m or upgrade to Professional tier"
}

# ❌ Bad
rule := {
    "level": "error",
    "message": "CPU too high",
    "suggestion": "Fix it"
}
```

### 3. Test Everything

Every policy should have comprehensive tests:

```rego
# my-policy_test.rego
package containerassist.my_policy_test

import rego.v1
import data.containerassist.my_policy

test_production_uses_distroless if {
    config := my_policy.generation_config with input as {
        "tool": "generate-dockerfile",
        "environment": "production",
    }
    contains(config.baseImage, "distroless")
}
```

Run tests:
```bash
opa test my-policy.rego my-policy_test.rego -v
```

### 4. Use Helper Functions

```rego
# Extract repeated logic
parse_cpu(cpu_str) := millicores if {
    endswith(cpu_str, "m")
    trimmed := trim_suffix(cpu_str, "m")
    millicores := to_number(trimmed)
}

parse_cpu(cpu_str) := millicores if {
    not endswith(cpu_str, "m")
    cores := to_number(cpu_str)
    millicores := cores * 1000
}
```

### 5. Environment-Aware Rules

```rego
# Strict in production
validation_rules contains rule if {
    input.environment == "production"
    has_issue(input.content)
    rule := {"level": "error", ...}
}

# Lenient in development
validation_rules contains rule if {
    input.environment == "development"
    has_issue(input.content)
    rule := {"level": "warning", ...}
}
```

---

## Debugging

### Policy Simulation Tool (Recommended)

The **policy simulation tool** shows how your custom policy combines with the built-in system by running tools with and without your policy:

```bash
# Simulate your policy
npm run policy:simulate -- \
  --policy policies.user.examples/my-policy.rego \
  --tool generate-dockerfile \
  --input '{"language": "node", "environment": "production", "teamTier": "starter"}'
```

**What it shows:**
- ✅ Generation configuration changes
- ✅ Before/After output comparison
- ✅ Policy-driven recommendations highlighted
- ✅ Validation rules triggered

**Example output:**
```
================================================================================
📈 SIMULATION RESULTS
================================================================================

📊 Impact Summary:
  • Generation Config: ✅ Modified
  • Output Changed: ✅ Yes

📦 Output Comparison:

  WITHOUT Policy:
  Summary: Standard Dockerfile recommendations
  Recommendations: 10 total

  WITH Policy:
  Summary: Policy-customized Dockerfile
  Recommendations: 15 total
  Policy-Driven: 5 recommendations
    • org-ca-certificates: Install company CA certificates
    • tier-resource-limits: Apply tier-based resource limits
```

**Use cases:**
- Preview policy impact before deployment
- Understand how custom policy combines with built-in policies
- Debug unexpected policy behavior
- Validate policy changes

### Test Policy in Isolation

For testing individual policy rules in isolation (doesn't show integration):

```bash
# Test generation_config
echo '{"tool": "generate-dockerfile", "environment": "production"}' | \
  opa eval --data my-policy.rego \
  'data.containerassist.my_policy.generation_config'

# Test templates
echo '{"tool": "generate-k8s-manifests", "language": "java"}' | \
  opa eval --data my-policy.rego \
  'data.containerassist.my_policy.templates'
```

### Enable Debug Logging

Set environment variable:
```bash
export LOG_LEVEL=debug
```

### Check Policy Syntax

```bash
opa check my-policy.rego
```

### Run with Coverage

```bash
opa test --coverage my-policy.rego my-policy_test.rego
```

### Trace Policy Evaluation

```rego
# Add trace statements
trace(sprintf("Config: %v", [config]))
```

---

## Common Pitfalls

### 1. Forgetting `import rego.v1`

```rego
# ❌ Will cause issues
package containerassist.my_policy

# ✅ Always import
package containerassist.my_policy
import rego.v1
```

### 2. Missing Conditionals

```rego
# ❌ Fires for all tools
generation_config contains config if {
    config := {"baseImage": "node:20"}
}

# ✅ Tool-specific
generation_config contains config if {
    input.tool == "generate-dockerfile"
    config := {"baseImage": "node:20"}
}
```

### 3. Not Handling Null/Missing Values

```rego
# ❌ Crashes if input.tier is null
tier_cpu_limits[input.tier]

# ✅ Safe with default
tier := object.get(input, "tier", "starter")
tier_cpu_limits[tier]
```

### 4. Inefficient Validation

```rego
# ❌ Checks even when content is null
validation_rules contains rule if {
    contains(input.content, "USER root")  # Crashes!
}

# ✅ Guard with null check
validation_rules contains rule if {
    input.content != null
    is_string(input.content)
    contains(input.content, "USER root")
}
```

### 5. Overly Broad Patterns

```rego
# ❌ Blocks too much
knowledge_filtering contains filter if {
    filter := {"action": "exclude", "pattern": "*"}
}

# ✅ Specific patterns
knowledge_filtering contains filter if {
    filter := {"action": "exclude", "pattern": "*-deprecated-*"}
}
```

---

## Additional Resources

- [OPA Documentation](https://www.openpolicyagent.org/docs/latest/)
- [Rego Style Guide](https://www.openpolicyagent.org/docs/latest/policy-language/)
- [CEL Specification](https://github.com/google/cel-spec)
- [Production-Ready Examples](../../policies.user.examples/production-ready/)
- [Migration Guide](./policy-migration-v3.md)
- [Sprint 5 Plan](../sprints/sprint-5.md)

---

## Migration Guide: Rego → CEL

This section helps you convert existing Rego validation rules to CEL format.

### When to Migrate

**Migrate to CEL if:**
- ✅ Your policy only does validation (no config generation or templates)
- ✅ Your rules are simple pattern matching
- ✅ You want easier maintenance for non-Rego developers
- ✅ You want faster evaluation performance

**Keep Rego if:**
- ❌ You need `generation_config`, `templates`, or `knowledge_filtering`
- ❌ Your logic involves complex computations or data transformations
- ❌ You rely on OPA's testing framework

### Migration Examples

#### Example 1: Simple String Matching

**Rego:**
```rego
package containerization.validation

import rego.v1

violations contains result if {
  not regex.match("USER", input.content)
  result := {
    "rule": "require-user",
    "category": "security",
    "severity": "block",
    "message": "Must specify USER directive"
  }
}
```

**CEL:**
```yaml
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: validation
spec:
  rules:
    - name: require-user
      category: security
      severity: block
      condition: '!input.content.contains("USER")'
      message: "Must specify USER directive"
```

#### Example 2: Multiple Conditions

**Rego:**
```rego
violations contains result if {
  regex.match("FROM", input.content)
  not regex.match("USER", input.content)
  result := {
    "rule": "user-required-when-from",
    "category": "security",
    "severity": "block",
    "message": "USER required when using FROM"
  }
}
```

**CEL:**
```yaml
spec:
  rules:
    - name: user-required-when-from
      category: security
      severity: block
      condition: 'input.content.contains("FROM") && !input.content.contains("USER")'
      message: "USER required when using FROM"
```

#### Example 3: Warnings vs Blocking

**Rego:**
```rego
warnings contains result if {
  not regex.match("HEALTHCHECK", input.content)
  result := {
    "rule": "recommend-healthcheck",
    "category": "health",
    "severity": "warn",
    "message": "HEALTHCHECK recommended"
  }
}
```

**CEL:**
```yaml
spec:
  rules:
    - name: recommend-healthcheck
      category: health
      severity: warn
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "HEALTHCHECK recommended"
```

#### Example 4: Multiple Rules in One Policy

**Rego:**
```rego
package containerization.security

violations contains user_violation if {
  not regex.match("USER", input.content)
  user_violation := {...}
}

violations contains latest_violation if {
  regex.match(":latest", input.content)
  latest_violation := {...}
}

warnings contains healthcheck_warning if {
  not regex.match("HEALTHCHECK", input.content)
  healthcheck_warning := {...}
}
```

**CEL:**
```yaml
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: security
spec:
  rules:
    - name: require-user
      category: security
      severity: block
      condition: '!input.content.contains("USER")'
      message: "Must specify USER"

    - name: no-latest-tag
      category: security
      severity: block
      condition: 'input.content.contains(":latest")'
      message: "Do not use :latest tag"

    - name: recommend-healthcheck
      category: health
      severity: warn
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "HEALTHCHECK recommended"
```

### Migration Checklist

- [ ] Identify pure validation rules (no `generation_config`, `templates`, etc.)
- [ ] Convert each `violations` rule to `severity: block`
- [ ] Convert each `warnings` rule to `severity: warn`
- [ ] Convert each `suggestions` rule to `severity: suggest`
- [ ] Translate Rego regex patterns to CEL `contains()` calls
- [ ] Add `apiVersion` and `kind` headers
- [ ] Add `metadata` section with `name` and `description`
- [ ] Test the CEL policy with sample input
- [ ] Update deployment to include `.yaml` files
- [ ] Remove old `.rego` file if fully migrated

### Hybrid Deployment Strategy

You can migrate incrementally:

1. **Start**: All Rego policies
2. **Phase 1**: Add new CEL policies for simple validation
3. **Phase 2**: Migrate simple Rego rules to CEL
4. **Phase 3**: Keep complex Rego policies, CEL for validation

**Example directory structure:**
```
policies.user/
├── security-baseline.yaml     # CEL: Simple validation
├── production-ready.yaml       # CEL: Simple validation
├── advanced-config.rego        # Rego: Complex logic + config
└── templates.rego              # Rego: Template injection
```

Both formats will be loaded and merged automatically!

### CEL Expression Patterns

| Rego Pattern | CEL Equivalent |
|--------------|----------------|
| `regex.match("USER", input.content)` | `input.content.contains("USER")` |
| `not regex.match("X", input.content)` | `!input.content.contains("X")` |
| `regex.match("X", input.content); regex.match("Y", input.content)` | `input.content.contains("X") && input.content.contains("Y")` |
| `regex.match("X", input.content)` OR `regex.match("Y", input.content)` | `input.content.contains("X") \|\| input.content.contains("Y")` |
| `count(split(input.content, "\n")) > 100` | `input.content.size() > ...` (approximate) |

**Note:** CEL `contains()` is substring matching, not regex. For complex patterns, keep using Rego.

---

## Support

- GitHub Issues: [Report bugs](https://github.com/your-org/containerization-assist/issues)
- Discussions: [Ask questions](https://github.com/your-org/containerization-assist/discussions)
- Internal Wiki: Link to your organization's internal documentation

---

**Version:** 4.0.0
**Last Updated:** Sprint 6 (CEL Support)
**License:** MIT
