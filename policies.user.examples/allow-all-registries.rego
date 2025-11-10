package containerization.registry_override

# ==============================================================================
# Allow All Registries Policy
# ==============================================================================
#
# This policy overrides the built-in base-images.rego policy that recommends
# Microsoft Container Registry (mcr.microsoft.com).
#
# USE CASE:
# - Organizations using Docker Hub, GCR, ECR, or private registries
# - Teams that want flexibility in base image selection
#
# QUICK START:
# 1. Copy this file to policies.user/ directory (source installation)
#    OR set CUSTOM_POLICY_PATH=/path/to/this/directory
# 2. Restart your MCP client
# 3. Done! All registries are now allowed
#
# ==============================================================================

policy_name := "Allow All Registries"
policy_version := "1.0"
policy_category := "compliance"

# No violations - allow all registries
violations := []

# No warnings
warnings := []

# Suggest using official images
suggestions contains result if {
  contains(input.content, "FROM ")

  result := {
    "rule": "suggest-official-images",
    "category": "quality",
    "priority": 50,
    "severity": "suggest",
    "message": "Consider using official images from Docker Hub, GCR, or ECR for reliability.",
    "description": "Recommendation to use official container images",
  }
}

# Always allow
default allow := true
allow := true

# Result structure
result := {
  "allow": allow,
  "violations": violations,
  "warnings": warnings,
  "suggestions": suggestions,
  "summary": {
    "total_violations": 0,
    "total_warnings": 0,
    "total_suggestions": count(suggestions),
  },
}
