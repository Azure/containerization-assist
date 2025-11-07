# Policy Authoring Guide

**Version:** 3.0.0
**Last Updated:** Sprint 5
**Audience:** Platform engineers, DevOps teams, policy authors

This guide provides comprehensive documentation on writing custom policies for containerization-assist using OPA Rego.

---

## Table of Contents

1. [Overview](#overview)
2. [Template Injection (Sprint 3)](#template-injection-sprint-3-)
3. [Policy Architecture](#policy-architecture)
4. [Phase-by-Phase Guide](#phase-by-phase-guide)
5. [Schema Reference](#schema-reference)
6. [Best Practices](#best-practices)
7. [Debugging](#debugging)
8. [Common Pitfalls](#common-pitfalls)

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

```
my-policy.rego          # Policy implementation
my-policy_test.rego     # OPA test suite (required)
```

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
- [Production-Ready Examples](../../policies.user.examples/production-ready/)
- [Migration Guide](./policy-migration-v3.md)
- [Sprint 5 Plan](../sprints/sprint-5.md)

---

## Support

- GitHub Issues: [Report bugs](https://github.com/your-org/containerization-assist/issues)
- Discussions: [Ask questions](https://github.com/your-org/containerization-assist/discussions)
- Internal Wiki: Link to your organization's internal documentation

---

**Version:** 3.0.0
**Last Updated:** Sprint 5
**License:** MIT
