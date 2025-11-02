package containerization.platform

# ==============================================================================
# Platform and Tag Enforcement Policy
# ==============================================================================
#
# This policy enforces:
# 1. All Dockerfiles must build for linux/arm64 platform
# 2. All Dockerfiles must be tagged with "demo"
#
# Policy enforcement happens at multiple points:
# - generate-dockerfile: Validates generated Dockerfile plans
# - fix-dockerfile: Validates actual Dockerfile content
#
# ==============================================================================

# Metadata
policy_name := "Platform and Tag Enforcement"

policy_version := "1.0"

policy_category := "compliance"

# Default enforcement level
default enforcement := "strict"

# ==============================================================================
# INPUT TYPE DETECTION
# ==============================================================================

# Detect if input is a Dockerfile
is_dockerfile if {
	contains(input.content, "FROM ")
}

# Determine input type
input_type := "dockerfile" if {
	is_dockerfile
}
else := "unknown"

# ==============================================================================
# PLATFORM RULES
# ==============================================================================

# Rule: require-arm64-platform (priority: 100)
# Require all FROM statements to specify --platform=linux/arm64
violations contains result if {
	input_type == "dockerfile"

	# Extract all FROM lines
	from_lines := [line |
		line := split(input.content, "\n")[_]
		startswith(trim_space(line), "FROM ")
	]

	# Check if any FROM line is missing --platform=linux/arm64
	some line in from_lines
	not contains(line, "--platform=linux/arm64")

	result := {
		"rule": "require-arm64-platform",
		"category": "compliance",
		"priority": 100,
		"severity": "block",
		"message": "All FROM statements must specify --platform=linux/arm64. Example: FROM --platform=linux/arm64 node:20-alpine",
		"description": "Enforce linux/arm64 platform for all base images",
	}
}

# ==============================================================================
# TAG RULES
# ==============================================================================

# Rule: require-demo-tag-label (priority: 95)
# Require LABEL to indicate demo tag should be used
violations contains result if {
	input_type == "dockerfile"

	# Check for LABEL with tag=demo or image.tag=demo
	not regex.match(`(?mi)^LABEL\s+.*tag\s*=\s*["']?demo["']?`, input.content)

	result := {
		"rule": "require-demo-tag-label",
		"category": "compliance",
		"priority": 95,
		"severity": "block",
		"message": "Dockerfile must include LABEL with tag=demo. Add: LABEL tag=\"demo\"",
		"description": "Require LABEL indicating demo tag for build-time tagging",
	}
}

# Rule: verify-demo-tag-format (priority: 90)
# Ensure tag label uses correct format
suggestions contains result if {
	input_type == "dockerfile"

	# Has tag label but not in recommended format
	regex.match(`(?mi)^LABEL\s+.*tag\s*=`, input.content)
	regex.match(`(?mi)^LABEL\s+tag\s*=\s*["']?demo["']?`, input.content)

	result := {
		"rule": "verify-demo-tag-format",
		"category": "quality",
		"priority": 90,
		"severity": "suggest",
		"message": "Good! Using 'demo' tag as required. This will be applied during docker build.",
		"description": "Verify demo tag label format",
	}
}

# ==============================================================================
# HELPER FUNCTIONS
# ==============================================================================

# Helper: trim leading/trailing whitespace
trim_space(str) := trimmed if {
	trimmed := trim(str, " \t\r\n")
}

# ==============================================================================
# POLICY DECISION
# ==============================================================================

# Allow if no blocking violations
default allow := false

allow if {
	count(violations) == 0
}

# Final result structure
result := {
	"allow": allow,
	"violations": violations,
	"warnings": warnings,
	"suggestions": suggestions,
	"summary": {
		"total_violations": count(violations),
		"total_warnings": count(warnings),
		"total_suggestions": count(suggestions),
	},
}

# Warnings set (currently empty, can be extended)
warnings := []
