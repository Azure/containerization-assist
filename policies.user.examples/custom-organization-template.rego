package containerization.custom_org

# ==============================================================================
# Custom Organization Policy Template
# ==============================================================================
#
# This template provides a starting point for organization-specific policies.
# Customize the rules below to match your organization's requirements.
#
# QUICK START:
# 1. Copy this file to policies.user/ directory
# 2. Customize the rules below (see examples)
# 3. Test with: opa test policies.user/
# 4. Restart your MCP client
#
# CUSTOMIZATION EXAMPLES:
# - Enforce specific base image registry (e.g., your private registry)
# - Require specific labels (team, cost-center, compliance)
# - Block certain packages or configurations
# - Enforce naming conventions
#
# ==============================================================================

policy_name := "Custom Organization Policy"
policy_version := "1.0"
policy_category := "compliance"

# Metadata (customize these)
organization := "YOUR_ORG_NAME"
contact := "devops@your-org.com"

# ==============================================================================
# CUSTOMIZE: Required Labels
# ==============================================================================

# Rule: require-team-label (priority: 90)
# Require all Dockerfiles to specify team label
violations contains result if {
  input_type == "dockerfile"
  not regex.match(`(?mi)^LABEL\s+.*team\s*=`, input.content)

  result := {
    "rule": "require-team-label",
    "category": "compliance",
    "priority": 90,
    "severity": "block",
    "message": "Dockerfile must include LABEL team=\"YOUR_TEAM\". Example: LABEL team=\"platform-engineering\"",
    "description": "Require team label for ownership tracking",
  }
}

# ==============================================================================
# CUSTOMIZE: Allowed Base Registries
# ==============================================================================

# Rule: enforce-private-registry (priority: 85)
# Require base images from organization's private registry
violations contains result if {
  input_type == "dockerfile"

  # Extract all FROM lines
  from_lines := [line |
    line := split(input.content, "\n")[_]
    startswith(trim_space(line), "FROM ")
  ]

  # Check if any FROM line uses non-approved registry
  some line in from_lines
  not contains(line, "your-registry.example.com")  # CUSTOMIZE THIS
  not contains(line, "mcr.microsoft.com")          # Allow MCR as fallback

  result := {
    "rule": "enforce-private-registry",
    "category": "compliance",
    "priority": 85,
    "severity": "block",
    "message": "Base images must come from your-registry.example.com or mcr.microsoft.com",
    "description": "Enforce approved container registries",
  }
}

# ==============================================================================
# CUSTOMIZE: Security Requirements
# ==============================================================================

# Rule: require-security-scanning-label (priority: 95)
# Require label indicating security scan completion
warnings contains result if {
  input_type == "dockerfile"
  not regex.match(`(?mi)^LABEL\s+.*security[_-]?scan\s*=\s*["']?true["']?`, input.content)

  result := {
    "rule": "require-security-scanning-label",
    "category": "security",
    "priority": 95,
    "severity": "warn",
    "message": "Add LABEL security-scan=\"true\" after running security scan",
    "description": "Security scanning label recommended",
  }
}

# ==============================================================================
# Helper Functions
# ==============================================================================

is_dockerfile if contains(input.content, "FROM ")
input_type := "dockerfile" if is_dockerfile else := "unknown"

trim_space(str) := trimmed if {
  trimmed := trim(str, " \t\r\n")
}

# ==============================================================================
# Policy Decision
# ==============================================================================

default allow := false
allow if count(violations) == 0

result := {
  "allow": allow,
  "violations": violations,
  "warnings": warnings,
  "suggestions": [],
  "summary": {
    "total_violations": count(violations),
    "total_warnings": count(warnings),
    "total_suggestions": 0,
  },
}
