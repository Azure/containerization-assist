# Writing Rego Policies for Containerization-Assist

A comprehensive guide to writing custom OPA Rego policies for enforcing Docker and Kubernetes security, quality, and compliance standards.

---

## Table of Contents

1. [Introduction](#introduction)
2. [Getting Started](#getting-started)
3. [Policy Structure](#policy-structure)
4. [Writing Rules](#writing-rules)
5. [Testing Policies](#testing-policies)
6. [Common Use Cases](#common-use-cases)
7. [Best Practices](#best-practices)
8. [Troubleshooting](#troubleshooting)

---

## Introduction

### What is Rego?

Rego is the policy language used by [Open Policy Agent (OPA)](https://www.openpolicyagent.org/), a CNCF graduated project for policy-based control. It's a declarative language designed for querying and manipulating structured data.

### Why Rego for Containerization Policies?

- **Industry Standard**: Used by Kubernetes, Terraform, and major cloud providers
- **Expressive**: Rich built-in functions, conditionals, and data manipulation
- **Composable**: Import and reuse policies across projects
- **Testable**: Built-in testing framework with `opa test`
- **Fast**: Compiled to WebAssembly for efficient evaluation

### How Policies Are Used

Policies are enforced at multiple points in the containerization workflow:

1. **generate-dockerfile**: Validates generated Dockerfile plans against policies
2. **fix-dockerfile**: Validates actual Dockerfile content and returns violations
3. **generate-k8s-manifests**: Validates generated Kubernetes manifest plans

---

## Getting Started

### Prerequisites

Install the OPA CLI for testing:

```bash
# macOS
brew install opa

# Linux
curl -L -o opa https://openpolicyagent.org/downloads/latest/opa_linux_amd64
chmod +x opa
sudo mv opa /usr/local/bin/

# Verify installation
opa version
```

### Policy Location

Create your custom `.rego` policy file in your workspace or a dedicated policies directory:

```
your-workspace/
├── my-policies/
│   └── my-company-policy.rego
└── your-app/
    └── ...
```

Or use a shared location:

```
~/.containerization-assist/
└── policy.rego
```

### Loading Policies

**Built-in Policies**: By default, the built-in policies in the `policies/` directory are automatically discovered and merged. These provide baseline security, best practices, and base image governance.

**Custom Policies**: To use your own custom policy instead of (or in addition to) the built-in policies, set the environment variable to point to your policy file:

```bash
export CONTAINERIZATION_ASSIST_POLICY_PATH=/path/to/your/policy.rego
```

Or use the CLI flag:

```bash
containerization-assist-mcp --config /path/to/your/policy.rego
```

**Example Workflow for Custom Policies:**

```bash
# Create your policy directory
mkdir -p ~/.containerization-assist

# Create your policy file (see examples below)
cat > ~/.containerization-assist/policy.rego << 'EOF'
package containerization.security

# Your custom rules here
EOF

# Set environment variable to use custom policy
export CONTAINERIZATION_ASSIST_POLICY_PATH=~/.containerization-assist/policy.rego

# Run the tool - your custom policy will now be enforced
containerization-assist-mcp
```

**Note**: When you specify a custom policy path, it will be used instead of the built-in policies. If you want to extend the built-in policies, you can import and reference them in your custom policy file.

---

## Policy Structure

### Anatomy of a Rego Policy

```rego
package containerization.security

# ==============================================================================
# METADATA
# ==============================================================================

policy_name := "Security Baseline"
policy_version := "2.0"
policy_category := "security"

# ==============================================================================
# INPUT TYPE DETECTION
# ==============================================================================

# Detect if input is a Dockerfile
is_dockerfile {
    contains(input.content, "FROM ")
}

# Detect if input is a Kubernetes manifest
is_kubernetes {
    contains(input.content, "apiVersion:")
}

input_type := "dockerfile" {
    is_dockerfile
} else := "kubernetes" {
    is_kubernetes
} else := "unknown"

# ==============================================================================
# RULES
# ==============================================================================

# Violations - blocking issues
violations[result] {
    input_type == "dockerfile"
    regex.match(`(?m)^USER\s+(root|0)\s*$`, input.content)

    result := {
        "rule": "block-root-user",
        "category": "security",
        "priority": 95,
        "severity": "block",
        "message": "Running as root user is not allowed. Add USER directive with non-root user.",
        "description": "Containers running as root pose security risks"
    }
}

# Warnings - non-blocking issues
warnings[result] {
    input_type == "dockerfile"
    not regex.match(`(?m)^USER\s+\w+`, input.content)

    result := {
        "rule": "require-user-directive",
        "category": "security",
        "priority": 90,
        "severity": "warn",
        "message": "USER directive recommended. Containers should run as non-root user."
    }
}

# Suggestions - best practices
suggestions[result] {
    input_type == "dockerfile"
    not regex.match(`(?mi)^HEALTHCHECK`, input.content)

    result := {
        "rule": "recommend-healthcheck",
        "category": "quality",
        "priority": 75,
        "severity": "suggest",
        "message": "Consider adding a HEALTHCHECK instruction for production containers."
    }
}

# ==============================================================================
# POLICY DECISION
# ==============================================================================

# Allow if no blocking violations
allow {
    count(violations) == 0
}

# Result structure (required)
result := {
    "allow": allow,
    "violations": violations,
    "warnings": warnings,
    "suggestions": suggestions,
    "summary": {
        "total_violations": count(violations),
        "total_warnings": count(warnings),
        "total_suggestions": count(suggestions)
    }
}
```

### Required Components

1. **Package Declaration**: `package containerization.<category>`
2. **Input Type Detection**: Determine if input is Dockerfile, K8s, etc.
3. **Rule Sets**:
   - `violations` - Blocking issues (severity: "block")
   - `warnings` - Non-blocking issues (severity: "warn")
   - `suggestions` - Best practice recommendations (severity: "suggest")
4. **Policy Decision**: `allow` rule (true if no violations)
5. **Result Object**: Structured output matching expected format

---

## Writing Rules

### Rule Anatomy

Every rule should return a structured result:

```rego
violations[result] {
    # Condition 1: Check input type
    input_type == "dockerfile"

    # Condition 2: Detect the issue
    regex.match(`pattern`, input.content)

    # Condition 3: Additional checks (optional)
    not has_exception

    # Result: Structured violation
    result := {
        "rule": "rule-id",           # Unique identifier
        "category": "security",       # Category: security, quality, performance
        "priority": 95,               # Priority: 0-100 (higher = more important)
        "severity": "block",          # Severity: block, warn, suggest
        "message": "User message",    # Clear, actionable message
        "description": "Details"      # Optional: Additional context
    }
}
```

### Using Regex

Rego uses RE2 regex syntax with modifiers:

```rego
# Case-insensitive match
regex.match(`(?i)pattern`, input.content)

# Multiline mode (^ and $ match line boundaries)
regex.match(`(?m)^USER\s+root$`, input.content)

# Dotall mode (. matches newlines)
regex.match(`(?s)FROM.*USER`, input.content)

# Combined modifiers
regex.match(`(?mi)^HEALTHCHECK`, input.content)
```

### Common Patterns

#### Check for Missing Instructions

```rego
warnings[result] {
    input_type == "dockerfile"
    not regex.match(`(?mi)^HEALTHCHECK`, input.content)

    result := {
        "rule": "missing-healthcheck",
        "category": "quality",
        "priority": 75,
        "severity": "warn",
        "message": "HEALTHCHECK instruction is missing"
    }
}
```

#### Count Occurrences

```rego
# Count RUN instructions
run_count := count([line |
    line := split(input.content, "\n")[_]
    startswith(trim_space(line), "RUN ")
])

suggestions[result] {
    input_type == "dockerfile"
    run_count > 5

    result := {
        "rule": "excessive-run-commands",
        "severity": "suggest",
        "message": sprintf("Consider combining %d RUN commands to reduce layers", [run_count])
    }
}
```

#### Check Multiple Conditions

```rego
violations[result] {
    input_type == "dockerfile"

    # Must match all conditions
    has_secrets_in_env
    not has_secret_scanning
    environment == "production"

    result := {
        "rule": "secrets-without-scanning",
        "severity": "block",
        "message": "Secrets detected without secret scanning enabled"
    }
}

# Helper functions
has_secrets_in_env {
    regex.match(`(?i)(password|token|key)=`, input.content)
}

has_secret_scanning {
    # Check for secret scanning configuration
    contains(input.content, "secret-scan")
}

environment := "production" {
    # Extract from input metadata
    input.environment == "production"
}
```

#### Check for Specific Values

```rego
# Check for latest tags
violations[result] {
    input_type == "dockerfile"

    # Extract FROM lines
    from_lines := [line |
        line := split(input.content, "\n")[_]
        startswith(trim_space(line), "FROM ")
    ]

    # Check if any use :latest tag
    some line in from_lines
    contains(line, ":latest")

    result := {
        "rule": "no-latest-tag",
        "category": "quality",
        "priority": 80,
        "severity": "warn",
        "message": "Avoid using :latest tag. Pin to specific versions."
    }
}
```

---

## Testing Policies

### Writing Tests

Create a test file alongside your policy file. For example, if your policy is at `~/.containerization-assist/policy.rego`, create `~/.containerization-assist/policy_test.rego`:

```rego
package containerization.security

# Test: Blocking root user
test_block_root_user_explicit {
    violations["block-root-user"] with input as {
        "content": "FROM node:20-alpine\nUSER root\nCMD [\"node\", \"app.js\"]"
    }
}

test_block_root_user_uid {
    violations["block-root-user"] with input as {
        "content": "FROM node:20-alpine\nUSER 0\nCMD [\"node\", \"app.js\"]"
    }
}

# Test: Should allow non-root user
test_allow_nonroot_user {
    allow with input as {
        "content": "FROM node:20-alpine\nUSER node\nCMD [\"node\", \"app.js\"]"
    }
}

# Test: Warning for missing USER
test_warn_missing_user {
    warnings["require-user-directive"] with input as {
        "content": "FROM node:20-alpine\nCMD [\"node\", \"app.js\"]"
    }
}

# Test: Kubernetes privileged container
test_block_privileged_k8s {
    violations["block-privileged"] with input as {
        "content": "apiVersion: v1\nkind: Pod\nspec:\n  containers:\n  - securityContext:\n      privileged: true"
    }
}
```

### Running Tests

```bash
# Run all policy tests in your policy directory
opa test ~/.containerization-assist/

# Run tests with verbose output
opa test -v ~/.containerization-assist/

# Run tests with coverage
opa test --coverage ~/.containerization-assist/

# Run specific test file
opa test ~/.containerization-assist/policy_test.rego
```

### Test Output

```
PASS: 5/5
COVERAGE: 92.3%
```

---

## Common Use Cases

### 1. Base Image Restrictions

Enforce allowed base images from approved registries:

```rego
package containerization.compliance

# Allowed registries
allowed_registries := [
    "docker.io",
    "gcr.io",
    "ghcr.io",
    "mcr.microsoft.com"
]

violations[result] {
    input_type == "dockerfile"

    # Extract FROM instructions
    from_lines := [line |
        line := split(input.content, "\n")[_]
        startswith(trim_space(line), "FROM ")
    ]

    # Check each FROM line
    some line in from_lines
    image := extract_image(line)
    not is_allowed_registry(image)

    result := {
        "rule": "unapproved-registry",
        "category": "compliance",
        "priority": 90,
        "severity": "block",
        "message": sprintf("Image '%s' is from an unapproved registry", [image])
    }
}

extract_image(from_line) := image {
    parts := split(trim_space(from_line), " ")
    image := parts[1]
}

is_allowed_registry(image) {
    some registry in allowed_registries
    startswith(image, registry)
}
```

### 2. Security Scanning Requirements

Require vulnerability scanning in production:

```rego
package containerization.security

violations[result] {
    input_type == "dockerfile"
    input.environment == "production"

    # Check for security scanning step
    not has_security_scan

    result := {
        "rule": "require-security-scan",
        "category": "security",
        "priority": 95,
        "severity": "block",
        "message": "Production images must include security scanning"
    }
}

has_security_scan {
    # Check for Trivy, Grype, or other scanners
    regex.match(`(?i)(trivy|grype|snyk)`, input.content)
}
```

### 3. Resource Limits Enforcement

Ensure resource limits are set for Kubernetes:

```rego
package containerization.kubernetes

violations[result] {
    input_type == "kubernetes"

    # Check for resource limits
    not has_resource_limits

    result := {
        "rule": "require-resource-limits",
        "category": "performance",
        "priority": 85,
        "severity": "block",
        "message": "All containers must specify resource limits (CPU and memory)"
    }
}

has_resource_limits {
    regex.match(`(?s)resources:\s+limits:`, input.content)
    regex.match(`(?s)cpu:`, input.content)
    regex.match(`(?s)memory:`, input.content)
}
```

### 4. Label Requirements

Enforce required labels for compliance:

```rego
package containerization.compliance

required_labels := {
    "maintainer",
    "version",
    "description",
    "org.opencontainers.image.source"
}

violations[result] {
    input_type == "dockerfile"

    # Check each required label
    some label in required_labels
    not has_label(label)

    result := {
        "rule": "missing-required-label",
        "category": "compliance",
        "priority": 70,
        "severity": "warn",
        "message": sprintf("Missing required label: %s", [label])
    }
}

has_label(label) {
    regex.match(sprintf(`(?mi)^LABEL\s+%s\s*=`, [label]), input.content)
}
```

### 5. Multi-Stage Build Enforcement

Require multi-stage builds for compiled languages:

```rego
package containerization.optimization

compiled_languages := ["java", "go", "rust", "dotnet", "c#"]

violations[result] {
    input_type == "dockerfile"

    # Check if language is compiled
    some lang in compiled_languages
    input.language == lang

    # Check if multi-stage build is used
    not is_multistage

    result := {
        "rule": "require-multistage-build",
        "category": "optimization",
        "priority": 80,
        "severity": "block",
        "message": sprintf("%s projects must use multi-stage builds to reduce image size", [lang])
    }
}

is_multistage {
    # Count FROM instructions
    from_count := count([line |
        line := split(input.content, "\n")[_]
        startswith(trim_space(line), "FROM ")
    ])
    from_count > 1
}
```

---

## Best Practices

### 1. Use Clear Rule IDs

```rego
# ✅ Good - descriptive and unique
"rule": "block-root-user"
"rule": "require-healthcheck"
"rule": "enforce-resource-limits"

# ❌ Bad - vague
"rule": "security-1"
"rule": "check"
```

### 2. Write Actionable Messages

```rego
# ✅ Good - tells user what to do
"message": "Add USER directive with non-root user (e.g., USER node)"

# ❌ Bad - just states the problem
"message": "Root user detected"
```

### 3. Use Appropriate Severity Levels

- **block**: Security vulnerabilities, compliance violations, critical issues
- **warn**: Best practice violations, potential issues, deprecated patterns
- **suggest**: Optimizations, nice-to-haves, informational

### 4. Set Meaningful Priorities

- **90-100**: Critical security/compliance issues
- **70-89**: Important quality/performance issues
- **50-69**: Best practices and optimizations
- **0-49**: Nice-to-have improvements

### 5. Test Everything

```rego
# Write tests for:
# - Rules triggering correctly
# - Rules not triggering on valid input
# - Edge cases
# - Multiple conditions
```

### 6. Use Helper Functions

```rego
# Extract complex logic into reusable functions
is_production {
    input.environment == "production"
}

has_user_directive {
    regex.match(`(?mi)^USER\s+\w+`, input.content)
}

# Use in rules
violations[result] {
    is_production
    not has_user_directive
    # ...
}
```

### 7. Document Your Policies

```rego
# ==============================================================================
# SECURITY BASELINE POLICY
# ==============================================================================
#
# Purpose: Enforce minimum security requirements for all containers
# Applies to: Dockerfile generation and validation
# Enforcement: Strict (blocks on violations)
#
# Rules:
# - block-root-user: Containers must not run as root
# - require-scanning: Production images must be scanned
# - block-secrets: No hardcoded secrets in ENV
#
# ==============================================================================
```

### 8. Organize by Package

```rego
# Group related policies
package containerization.security      # Security rules
package containerization.compliance    # Compliance rules
package containerization.optimization  # Performance/size rules
package containerization.base          # Shared utilities
```

---

## Troubleshooting

### Common Issues

#### 1. Policy Not Loading

**Symptom**: Custom policy file exists but isn't being enforced

**Causes**:
- Custom policy path not set (built-in policies will be used instead)
- File doesn't have `.rego` extension
- Syntax error in policy file
- Policy package name doesn't match expected namespace

**Solution**:
```bash
# Verify environment variable is set (if using custom policy)
echo $CONTAINERIZATION_ASSIST_POLICY_PATH

# Validate policy syntax
opa check ~/.containerization-assist/policy.rego

# Test policy evaluation
opa eval -d ~/.containerization-assist/ -i test-input.txt "data.containerization.security.result"

# Check that policy uses correct namespace (should be under data.containerization.*)
opa eval -d ~/.containerization-assist/ "data.containerization"
```

#### 2. Rules Not Triggering

**Symptom**: Expected violations not appearing

**Causes**:
- Input type detection failing
- Regex pattern not matching
- Missing conditions

**Solution**:
```rego
# Add debug output to check conditions
violations[result] {
    trace(sprintf("Input type: %s", [input_type]))
    trace(sprintf("Content: %s", [input.content]))

    input_type == "dockerfile"
    # ... rest of rule
}
```

Run with trace output:
```bash
opa eval --explain=full -d ~/.containerization-assist/ -i input.txt "data.containerization.security.violations"
```

#### 3. Regex Not Matching

**Symptom**: Regex patterns not finding expected content

**Causes**:
- Missing multiline flag `(?m)`
- Incorrect escaping
- Whitespace sensitivity

**Solution**:
```rego
# Test regex patterns interactively
regex.match(`(?m)^USER\s+root$`, "FROM node\nUSER root\n")  # true

# Use regex debugger: https://regex101.com/
# Select flavor: RE2 (Golang)
```

#### 4. Policy Returns No Results

**Symptom**: `result` object is empty or undefined

**Causes**:
- Missing `result` rule
- Package mismatch
- Input format incorrect

**Solution**:
```rego
# Ensure result rule exists and matches expected format
result := {
    "allow": allow,
    "violations": violations,  # Must be defined
    "warnings": warnings,      # Must be defined
    "suggestions": suggestions # Must be defined
}
```

#### 5. Performance Issues

**Symptom**: Policy evaluation is slow

**Causes**:
- Complex regex patterns
- Nested loops
- Large input documents

**Solution**:
```rego
# Cache expensive computations
lines := split(input.content, "\n")

violations[result] {
    # Use cached lines instead of re-splitting
    some line in lines
    # ...
}
```

---

## Additional Resources

### Official Documentation

- [OPA Documentation](https://www.openpolicyagent.org/docs/latest/)
- [Rego Language Reference](https://www.openpolicyagent.org/docs/latest/policy-language/)
- [Rego Built-in Functions](https://www.openpolicyagent.org/docs/latest/policy-reference/)

### Examples

- [OPA Policy Library](https://github.com/open-policy-agent/library)
- [Kubernetes Admission Control](https://www.openpolicyagent.org/docs/latest/kubernetes-introduction/)
- [Conftest Examples](https://github.com/open-policy-agent/conftest/tree/master/examples)

### Tools

- [Rego Playground](https://play.openpolicyagent.org/)
- [OPA VSCode Extension](https://marketplace.visualstudio.com/items?itemName=tsandall.opa)
- [Regex101](https://regex101.com/) - Test regex patterns (use RE2/Golang flavor)

---

## Example: Complete Custom Policy

Here's a complete example policy for a fictional organization:

```rego
package containerization.acme

# ==============================================================================
# ACME Corporation Container Policy
# Version: 1.0
# ==============================================================================

policy_name := "ACME Container Standards"
policy_version := "1.0"

# ==============================================================================
# INPUT TYPE DETECTION
# ==============================================================================

is_dockerfile {
    contains(input.content, "FROM ")
}

is_kubernetes {
    contains(input.content, "apiVersion:")
}

input_type := "dockerfile" {
    is_dockerfile
} else := "kubernetes" {
    is_kubernetes
} else := "unknown"

# ==============================================================================
# CONFIGURATION
# ==============================================================================

# Approved base images
approved_bases := [
    "node:20-alpine",
    "python:3.11-slim",
    "golang:1.21-alpine",
    "mcr.microsoft.com/dotnet/aspnet:8.0"
]

# Required labels
required_labels := [
    "maintainer",
    "version",
    "team"
]

# ==============================================================================
# DOCKERFILE RULES
# ==============================================================================

# SECURITY: Block root user
violations[result] {
    input_type == "dockerfile"
    regex.match(`(?m)^USER\s+(root|0)\s*$`, input.content)

    result := {
        "rule": "acme-no-root",
        "category": "security",
        "priority": 100,
        "severity": "block",
        "message": "ACME Policy: Containers must not run as root. Use USER node, appuser, etc."
    }
}

# COMPLIANCE: Enforce approved base images
violations[result] {
    input_type == "dockerfile"
    not uses_approved_base

    result := {
        "rule": "acme-approved-bases",
        "category": "compliance",
        "priority": 95,
        "severity": "block",
        "message": sprintf("ACME Policy: Use approved base images only: %s", [concat(", ", approved_bases)])
    }
}

uses_approved_base {
    from_line := [line |
        line := split(input.content, "\n")[_]
        startswith(trim_space(line), "FROM ")
    ][0]

    some base in approved_bases
    contains(from_line, base)
}

# COMPLIANCE: Required labels
warnings[result] {
    input_type == "dockerfile"
    some label in required_labels
    not has_label(label)

    result := {
        "rule": "acme-required-labels",
        "category": "compliance",
        "priority": 80,
        "severity": "warn",
        "message": sprintf("ACME Policy: Add required label: LABEL %s=\"...\"", [label])
    }
}

has_label(label) {
    regex.match(sprintf(`(?mi)^LABEL\s+%s\s*=`, [label]), input.content)
}

# QUALITY: Healthchecks required
warnings[result] {
    input_type == "dockerfile"
    not regex.match(`(?mi)^HEALTHCHECK`, input.content)

    result := {
        "rule": "acme-healthcheck",
        "category": "quality",
        "priority": 75,
        "severity": "warn",
        "message": "ACME Policy: Add HEALTHCHECK for production readiness"
    }
}

# ==============================================================================
# KUBERNETES RULES
# ==============================================================================

# SECURITY: Block privileged containers
violations[result] {
    input_type == "kubernetes"
    regex.match(`privileged:\s*true`, input.content)

    result := {
        "rule": "acme-no-privileged",
        "category": "security",
        "priority": 100,
        "severity": "block",
        "message": "ACME Policy: Privileged containers are not allowed"
    }
}

# PERFORMANCE: Require resource limits
violations[result] {
    input_type == "kubernetes"
    not has_resource_limits

    result := {
        "rule": "acme-resource-limits",
        "category": "performance",
        "priority": 85,
        "severity": "block",
        "message": "ACME Policy: All containers must specify CPU and memory limits"
    }
}

has_resource_limits {
    regex.match(`(?s)resources:\s+limits:`, input.content)
    regex.match(`cpu:`, input.content)
    regex.match(`memory:`, input.content)
}

# ==============================================================================
# POLICY DECISION
# ==============================================================================

allow {
    count(violations) == 0
}

result := {
    "allow": allow,
    "violations": violations,
    "warnings": warnings,
    "suggestions": suggestions,
    "summary": {
        "total_violations": count(violations),
        "total_warnings": count(warnings),
        "total_suggestions": count(suggestions)
    }
}
```

Save this as `~/.containerization-assist/acme-standards.rego` and test:

```bash
# Test the policy
opa test ~/.containerization-assist/

# Validate a Dockerfile against it
opa eval -d ~/.containerization-assist/ -i Dockerfile "data.containerization.acme.result"

# Set environment variable to use this policy
export CONTAINERIZATION_ASSIST_POLICY_PATH=~/.containerization-assist/acme-standards.rego
```

---

## Summary

You now have a comprehensive understanding of:

- ✅ How Rego policies work in containerization-assist
- ✅ The structure and anatomy of policy files
- ✅ How to write rules for Dockerfiles and Kubernetes manifests
- ✅ Testing policies with `opa test`
- ✅ Common use cases and patterns
- ✅ Best practices for maintainable policies
- ✅ Troubleshooting common issues

**Next Steps**:

1. Create your custom policy file (e.g., `~/.containerization-assist/policy.rego`)
2. Write custom rules for your organization's requirements
3. Test policies thoroughly with `opa test`
4. Set `CONTAINERIZATION_ASSIST_POLICY_PATH` environment variable
5. Share policies across teams
6. Iterate based on feedback

For questions or issues, refer to the [OPA documentation](https://www.openpolicyagent.org/docs/latest/) or open an issue on GitHub.
