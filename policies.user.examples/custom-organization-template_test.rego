package containerization.custom_org_test

import data.containerization.custom_org
import rego.v1

# ==============================================================================
# Test Suite: Custom Organization Policy Template
# ==============================================================================

# Test: Missing team label triggers violation
test_require_team_label_violation if {
  result := custom_org.result with input as {"content": "FROM node:20\nUSER node"}

  result.allow == false
  count(result.violations) > 0

  some violation in result.violations
  violation.rule == "require-team-label"
  violation.severity == "block"
}

# Test: Team label present allows build
test_team_label_present if {
  result := custom_org.result with input as {"content": "FROM your-registry.example.com/node:20\nLABEL team=\"platform\"\nUSER node"}

  result.allow == true
  count(result.violations) == 0
}

# Test: Non-approved registry triggers violation
test_unapproved_registry_violation if {
  result := custom_org.result with input as {"content": "FROM docker.io/node:20\nLABEL team=\"platform\"\nUSER node"}

  result.allow == false
  count(result.violations) > 0

  some violation in result.violations
  violation.rule == "enforce-private-registry"
  violation.severity == "block"
}

# Test: Approved registry (custom) allows build
test_approved_custom_registry if {
  result := custom_org.result with input as {"content": "FROM your-registry.example.com/node:20\nLABEL team=\"platform\"\nUSER node"}

  result.allow == true
  count(result.violations) == 0
}

# Test: Approved registry (MCR) allows build
test_approved_mcr_registry if {
  result := custom_org.result with input as {"content": "FROM mcr.microsoft.com/dotnet/runtime:8.0\nLABEL team=\"platform\"\nUSER app"}

  result.allow == true
  count(result.violations) == 0
}

# Test: Missing security scan label triggers warning
test_security_scan_warning if {
  result := custom_org.result with input as {"content": "FROM your-registry.example.com/node:20\nLABEL team=\"platform\"\nUSER node"}

  count(result.warnings) > 0

  some warning in result.warnings
  warning.rule == "require-security-scanning-label"
  warning.severity == "warn"
}

# Test: Security scan label present removes warning
test_security_scan_label_present if {
  result := custom_org.result with input as {"content": "FROM your-registry.example.com/node:20\nLABEL team=\"platform\"\nLABEL security-scan=\"true\"\nUSER node"}

  count(result.warnings) == 0
}

# Test: Multiple violations block build
test_multiple_violations if {
  result := custom_org.result with input as {"content": "FROM docker.io/node:20\nUSER node"}

  result.allow == false
  count(result.violations) == 2  # Missing team label + unapproved registry
}

# Test: trim_space helper function
test_trim_space_helper if {
  custom_org.trim_space("  test  ") == "test"
  custom_org.trim_space("\t\ntest\r\n") == "test"
}

# Test: Input type detection
test_dockerfile_detection if {
  custom_org.input_type == "dockerfile" with input as {"content": "FROM node:20\nUSER node"}
}

# Test: Policy metadata
test_policy_metadata if {
  custom_org.policy_name == "Custom Organization Policy"
  custom_org.policy_version == "1.0"
  custom_org.policy_category == "compliance"
  custom_org.organization == "YOUR_ORG_NAME"
  custom_org.contact == "devops@your-org.com"
}
