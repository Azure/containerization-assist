package containerization.warn_only_test

import data.containerization.warn_only
import rego.v1

# ==============================================================================
# Test Suite: Warn-Only Mode Policy
# ==============================================================================

# Test: Root user triggers warning (not violation)
test_root_user_warning if {
  result := warn_only.result with input as {"content": "FROM node:20\nUSER root"}

  result.allow == true  # Should still allow
  count(result.violations) == 0  # No blocking violations
  count(result.warnings) > 0  # But has warnings

  # Verify warning details
  some warning in result.warnings
  warning.rule == "root-user-warning"
  warning.severity == "warn"
  contains(warning.message, "root user")
}

# Test: USER 0 triggers warning
test_user_zero_warning if {
  result := warn_only.result with input as {"content": "FROM node:20\nUSER 0"}

  result.allow == true
  count(result.violations) == 0
  count(result.warnings) > 0
}

# Test: :latest tag triggers warning
test_latest_tag_warning if {
  result := warn_only.result with input as {"content": "FROM node:latest\nUSER node"}

  result.allow == true
  count(result.violations) == 0
  count(result.warnings) > 0

  some warning in result.warnings
  warning.rule == "latest-tag-warning"
  warning.severity == "warn"
  contains(warning.message, "latest tag")
}

# Test: Multiple warnings can be triggered
test_multiple_warnings if {
  result := warn_only.result with input as {"content": "FROM node:latest\nUSER root"}

  result.allow == true
  count(result.violations) == 0
  count(result.warnings) == 2  # Both root and :latest warnings
}

# Test: Good Dockerfile has no warnings
test_good_dockerfile_no_warnings if {
  result := warn_only.result with input as {"content": "FROM node:20-alpine\nUSER node"}

  result.allow == true
  count(result.violations) == 0
  count(result.warnings) == 0
}

# Test: Always allows regardless of content
test_always_allows if {
  result := warn_only.result with input as {"content": "FROM scratch\nUSER root\nRUN dangerous-command"}

  result.allow == true
  count(result.violations) == 0
}

# Test: Input type detection for Dockerfile
test_dockerfile_detection if {
  warn_only.input_type == "dockerfile" with input as {"content": "FROM node:20\nUSER node"}
}

# Test: Input type detection for non-Dockerfile
test_non_dockerfile_detection if {
  warn_only.input_type == "unknown" with input as {"content": "apiVersion: v1\nkind: Pod"}
}

# Test: Policy metadata
test_policy_metadata if {
  warn_only.policy_name == "Warn-Only Mode"
  warn_only.policy_version == "1.0"
  warn_only.policy_category == "advisory"
  warn_only.enforcement == "advisory"
}

# Test: Summary structure
test_summary_structure if {
  result := warn_only.result with input as {"content": "FROM node:latest\nUSER root"}

  result.summary.total_violations == 0
  result.summary.total_warnings == 2
  result.summary.total_suggestions == 0
}
