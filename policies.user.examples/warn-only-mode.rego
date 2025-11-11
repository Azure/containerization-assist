package containerization.warn_only

# ==============================================================================
# Warn-Only Mode Policy
# ==============================================================================
#
# This policy provides recommendations without blocking builds or deployments.
# All violations are converted to warnings for advisory-only enforcement.
#
# USE CASE:
# - Testing policy changes without breaking CI/CD
# - Gradual policy adoption
# - Development environments with relaxed requirements
#
# QUICK START:
# 1. Copy this file to policies.user/ directory
#    OR set CUSTOM_POLICY_PATH=/path/to/this/directory
# 2. Restart your MCP client
# 3. Done! All policies are now advisory-only
#
# NOTE: This policy OVERRIDES built-in blocking policies. Built-in policies
# will still run, but their violations will be logged as warnings instead
# of blocking operations.
#
# ==============================================================================

policy_name := "Warn-Only Mode"
policy_version := "1.0"
policy_category := "advisory"

# Override enforcement level
default enforcement := "advisory"

# No blocking violations in warn-only mode
violations := []

# Common security warnings (converted from built-in blocking rules)
warnings contains result if {
  input_type == "dockerfile"
  regex.match(`(?m)^USER\s+(root|0)\s*$`, input.content)

  result := {
    "rule": "root-user-warning",
    "category": "security",
    "priority": 95,
    "severity": "warn",
    "message": "⚠️ Running as root user detected (advisory). Consider adding USER directive with non-root user.",
    "description": "Advisory: Containers should run as non-root user",
  }
}

warnings contains result if {
  input_type == "dockerfile"
  regex.match(`FROM\s+[^:]+:latest`, input.content)

  result := {
    "rule": "latest-tag-warning",
    "category": "quality",
    "priority": 80,
    "severity": "warn",
    "message": "⚠️ Using :latest tag detected (advisory). Consider specifying explicit version tags for reproducibility.",
    "description": "Advisory: Specify explicit version tags",
  }
}

# Input type detection
is_dockerfile if contains(input.content, "FROM ")
input_type := "dockerfile" if is_dockerfile else := "unknown"

# Always allow
default allow := true
allow := true

# Result structure
result := {
  "allow": allow,
  "violations": violations,
  "warnings": warnings,
  "suggestions": [],
  "summary": {
    "total_violations": 0,
    "total_warnings": count(warnings),
    "total_suggestions": 0,
  },
}
