package containerization.registry_override_test

import data.containerization.registry_override
import rego.v1

# ==============================================================================
# Test Suite: Allow All Registries Policy
# ==============================================================================

# Test: Docker Hub registry should be allowed
test_allow_docker_hub if {
  result := registry_override.result with input as {"content": "FROM docker.io/node:20-alpine\nUSER node"}
  result.allow == true
  count(result.violations) == 0
}

# Test: GCR registry should be allowed
test_allow_gcr if {
  result := registry_override.result with input as {"content": "FROM gcr.io/distroless/base\nUSER nonroot"}
  result.allow == true
  count(result.violations) == 0
}

# Test: ECR registry should be allowed
test_allow_ecr if {
  result := registry_override.result with input as {"content": "FROM 123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:latest\nUSER app"}
  result.allow == true
  count(result.violations) == 0
}

# Test: Private registry should be allowed
test_allow_private_registry if {
  result := registry_override.result with input as {"content": "FROM myregistry.example.com/myapp:v1.0\nUSER appuser"}
  result.allow == true
  count(result.violations) == 0
}

# Test: MCR registry should be allowed
test_allow_mcr if {
  result := registry_override.result with input as {"content": "FROM mcr.microsoft.com/dotnet/runtime:8.0\nUSER app"}
  result.allow == true
  count(result.violations) == 0
}

# Test: Should suggest using official images
test_suggest_official_images if {
  result := registry_override.result with input as {"content": "FROM node:20-alpine\nUSER node"}
  count(result.suggestions) > 0

  # Verify suggestion message
  some suggestion in result.suggestions
  suggestion.rule == "suggest-official-images"
  suggestion.severity == "suggest"
}

# Test: No violations for any registry
test_no_violations_any_registry if {
  result := registry_override.result with input as {"content": "FROM random-registry.io/random-image:tag\nUSER user"}
  result.allow == true
  count(result.violations) == 0
}

# Test: No warnings for any registry
test_no_warnings if {
  result := registry_override.result with input as {"content": "FROM docker.io/node:20\nUSER node"}
  count(result.warnings) == 0
}

# Test: Policy metadata
test_policy_metadata if {
  registry_override.policy_name == "Allow All Registries"
  registry_override.policy_version == "1.0"
  registry_override.policy_category == "compliance"
}

# Test: Summary structure
test_summary_structure if {
  result := registry_override.result with input as {"content": "FROM node:20\nUSER node"}

  result.summary.total_violations == 0
  result.summary.total_warnings == 0
  result.summary.total_suggestions >= 0
}
