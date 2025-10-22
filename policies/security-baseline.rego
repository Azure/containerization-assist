package containerization.security

# ==============================================================================
# Security Baseline Policy
# ==============================================================================
#
# This policy enforces essential security rules for containerization.
# It replaces the YAML-based security-baseline.yaml policy with
# industry-standard Rego (Open Policy Agent).
#
# Policy enforcement happens at multiple points:
# - generate-dockerfile: Validates generated Dockerfile plans
# - fix-dockerfile: Validates actual Dockerfile content
# - generate-k8s-manifests: Validates K8s manifest plans
#
# ==============================================================================

# Metadata
policy_name := "Security Baseline"

policy_version := "2.0"

policy_category := "security"

# Default enforcement level
default enforcement := "strict"

# ==============================================================================
# INPUT TYPE DETECTION
# ==============================================================================

# Detect if input is a Dockerfile
is_dockerfile if {
	contains(input.content, "FROM ")
}

# Detect if input is a Kubernetes manifest
is_kubernetes if {
	contains(input.content, "apiVersion:")
}

# Detect if input is a Kubernetes manifest (alternative check)
is_kubernetes if {
	contains(input.content, "kind:")
}

# Determine input type
input_type := "dockerfile" if {
	is_dockerfile
}
else := "kubernetes" if {
	is_kubernetes
}
else := "unknown"

# ==============================================================================
# DOCKERFILE SECURITY RULES
# ==============================================================================

# Rule: block-root-user (priority: 95)
# Block containers running as root user (USER root or USER 0)
violations contains result if {
	input_type == "dockerfile"
	regex.match(`(?m)^USER\s+(root|0)\s*$`, input.content)

	result := {
		"rule": "block-root-user",
		"category": "security",
		"priority": 95,
		"severity": "block",
		"message": "Running as root user is not allowed. Add USER directive with non-root user.",
		"description": "Detect and block root user in Dockerfiles",
	}
}

# Rule: require-user-directive (priority: 90)
# Warn if Dockerfile doesn't specify a USER directive
warnings contains result if {
	input_type == "dockerfile"
	not regex.match(`(?m)^USER\s+\w+`, input.content)

	result := {
		"rule": "require-user-directive",
		"category": "security",
		"priority": 90,
		"severity": "warn",
		"message": "USER directive recommended. Containers should run as non-root user.",
		"description": "Require USER directive in Dockerfile",
	}
}

# Rule: block-secrets-in-env (priority: 100)
# Block hardcoded secrets in environment variables
violations contains result if {
	input_type == "dockerfile"
	regex.match(`(?i)(password|secret|api[_-]?key|token).*=.*\S+`, input.content)

	result := {
		"rule": "block-secrets-in-env",
		"category": "security",
		"priority": 100,
		"severity": "block",
		"message": "Secrets in plain environment variables are not allowed. Use Kubernetes Secrets.",
		"description": "Detect secrets in environment variables",
	}
}

# ==============================================================================
# KUBERNETES SECURITY RULES
# ==============================================================================

# Rule: block-privileged (priority: 95)
# Block privileged containers
violations contains result if {
	input_type == "kubernetes"
	regex.match(`privileged:\s*true`, input.content)

	result := {
		"rule": "block-privileged",
		"category": "security",
		"priority": 95,
		"severity": "block",
		"message": "Privileged containers are not allowed for security reasons.",
		"description": "Block privileged mode in container configs",
	}
}

# Rule: block-host-network (priority: 90)
# Block host network access
violations contains result if {
	input_type == "kubernetes"
	regex.match(`hostNetwork:\s*true`, input.content)

	result := {
		"rule": "block-host-network",
		"category": "security",
		"priority": 90,
		"severity": "block",
		"message": "Host network access is not allowed. Use pod networking instead.",
		"description": "Block host network access",
	}
}

# ==============================================================================
# QUALITY RULES
# ==============================================================================

# Rule: require-healthcheck (priority: 75)
# Recommend HEALTHCHECK directive for production containers
warnings contains result if {
	input_type == "dockerfile"
	not regex.match(`(?mi)^HEALTHCHECK`, input.content)

	result := {
		"rule": "require-healthcheck",
		"category": "quality",
		"priority": 75,
		"severity": "warn",
		"message": "HEALTHCHECK directive is recommended for production containers.",
		"description": "Recommend HEALTHCHECK directive",
	}
}

# Rule: avoid-apt-upgrade (priority: 65)
# Warn about apt-get upgrade in Dockerfile
warnings contains result if {
	input_type == "dockerfile"
	regex.match(`(?mi)apt-get\s+(upgrade|dist-upgrade)`, input.content)

	result := {
		"rule": "avoid-apt-upgrade",
		"category": "quality",
		"priority": 65,
		"severity": "warn",
		"message": "Avoid apt-get upgrade in Dockerfile. Use specific package versions instead.",
		"description": "Avoid apt-get upgrade",
	}
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

# Suggestions set (currently empty, can be extended)
suggestions := []
