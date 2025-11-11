# Semantic Validation Policy
#
# Demonstrates semantic validation that goes beyond pattern matching:
# - Resource efficiency analysis (over-provisioning detection)
# - Security posture scoring
# - Environment-specific validation rules
# - Composite checks across multiple configuration aspects
#
# Sprint 4: Semantic Validation & Cross-Tool Consistency

package semantic_validation

import rego.v1

#-----------------------------------------------------------------------------
# Helper Functions: Resource Parsing
#-----------------------------------------------------------------------------

# Parse CPU string to millicores
parse_cpu(cpu) := millicores if {
	# Handle millicores format (e.g., "1000m")
	endswith(cpu, "m")
	value := to_number(trim_suffix(cpu, "m"))
	millicores := value
}

parse_cpu(cpu) := millicores if {
	# Handle core format (e.g., "1.5" or "2")
	not endswith(cpu, "m")
	value := to_number(cpu)
	millicores := value * 1000
}

# Parse memory string to bytes
parse_memory(memory) := bytes if {
	# Binary units (powers of 1024)
	units := {
		"Ki": 1024,
		"Mi": 1048576, # 1024^2
		"Gi": 1073741824, # 1024^3
		"Ti": 1099511627776, # 1024^4
	}
	some unit, multiplier in units
	endswith(memory, unit)
	value := to_number(trim_suffix(memory, unit))
	bytes := value * multiplier
}

parse_memory(memory) := bytes if {
	# Decimal units (powers of 1000)
	units := {
		"k": 1000,
		"M": 1000000, # 1000^2
		"G": 1000000000, # 1000^3
		"T": 1000000000000, # 1000^4
	}
	some unit, multiplier in units
	endswith(memory, unit)
	value := to_number(trim_suffix(memory, unit))
	bytes := value * multiplier
}

parse_memory(memory) := bytes if {
	# No unit - assume bytes
	not contains(memory, "i")
	not regex.match("[kMGT]$", memory)
	bytes := to_number(memory)
}

#-----------------------------------------------------------------------------
# Resource Efficiency Validation
#-----------------------------------------------------------------------------

# Configuration: Resource efficiency thresholds
default resource_ratio_warn_threshold := 4.0

default resource_ratio_block_threshold := 10.0

# Check for over-provisioned CPU resources
over_provisioned_cpu contains violation if {
	input.kind == "Deployment"
	some container in input.spec.template.spec.containers
	limit := container.resources.limits.cpu
	request := container.resources.requests.cpu

	limit_mc := parse_cpu(limit)
	request_mc := parse_cpu(request)

	ratio := limit_mc / request_mc
	ratio > resource_ratio_warn_threshold

	violation := {
		"severity": "warning",
		"message": sprintf("Container '%s' has over-provisioned CPU (ratio: %vx)", [container.name, ratio]),
		"hint": sprintf("CPU limit (%s) is %vx the request (%s). Consider reducing to 2-4x for better efficiency.", [limit, ratio, request]),
		"field": sprintf("spec.template.spec.containers[%s].resources", [container.name]),
	}
}

# Check for over-provisioned memory resources
over_provisioned_memory contains violation if {
	input.kind == "Deployment"
	some container in input.spec.template.spec.containers
	limit := container.resources.limits.memory
	request := container.resources.requests.memory

	limit_bytes := parse_memory(limit)
	request_bytes := parse_memory(request)

	ratio := limit_bytes / request_bytes
	ratio > resource_ratio_warn_threshold

	violation := {
		"severity": "warning",
		"message": sprintf("Container '%s' has over-provisioned memory (ratio: %vx)", [container.name, ratio]),
		"hint": sprintf("Memory limit (%s) is %vx the request (%s). Consider reducing to 2-4x for better efficiency.", [limit, ratio, request]),
		"field": sprintf("spec.template.spec.containers[%s].resources", [container.name]),
	}
}

#-----------------------------------------------------------------------------
# Security Posture Analysis
#-----------------------------------------------------------------------------

# Detect Alpine base images in production
alpine_in_production contains violation if {
	input.kind == "Dockerfile"
	input.environment == "production"

	some line in split(input.content, "\n")
	startswith(trim_space(line), "FROM")
	contains(lower(line), "alpine")

	violation := {
		"severity": "warning",
		"message": "Alpine Linux base image detected in production Dockerfile",
		"hint": "Alpine is fast but less hardened. Consider using distroless or hardened images for production.",
		"field": "FROM instruction",
	}
}

# Detect missing USER instruction in production Dockerfiles
missing_nonroot_user_production contains violation if {
	input.kind == "Dockerfile"
	input.environment == "production"

	content := input.content
	not contains(content, "USER")

	violation := {
		"severity": "error",
		"message": "Missing USER instruction in production Dockerfile",
		"hint": "Production containers must run as non-root. Add 'USER <non-root-user>' before CMD/ENTRYPOINT.",
		"field": "USER instruction",
	}
}

# Detect missing health checks in production manifests
missing_health_checks_production contains violation if {
	input.kind == "Deployment"
	input.environment == "production"

	some container in input.spec.template.spec.containers
	not container.livenessProbe
	not container.readinessProbe

	violation := {
		"severity": "error",
		"message": sprintf("Container '%s' missing health checks in production", [container.name]),
		"hint": "Production deployments must have liveness and readiness probes for reliability.",
		"field": sprintf("spec.template.spec.containers[%s]", [container.name]),
	}
}

#-----------------------------------------------------------------------------
# Security Posture Score (Composite Check)
#-----------------------------------------------------------------------------

# Calculate security score (0-100)
security_score := score if {
	input.kind == "Deployment"

	# Score components (all weighted equally)
	checks := [
		has_security_context,
		runs_as_nonroot,
		has_health_checks,
		not_privileged,
		readonly_root_filesystem,
	]

	passed := count([c | c := checks[_]; c == true])
	total := count(checks)
	score := (passed * 100) / total
}

# Default false for all security checks
default has_security_context := false

default runs_as_nonroot := false

default has_health_checks := false

default not_privileged := false

default readonly_root_filesystem := false

# Check if containers have security context
has_security_context if {
	input.kind == "Deployment"
	containers := input.spec.template.spec.containers
	count(containers) > 0
	every container in containers {
		"securityContext" in object.keys(container)
	}
}

# Check if containers run as non-root
runs_as_nonroot if {
	input.kind == "Deployment"
	containers := input.spec.template.spec.containers
	count(containers) > 0
	every container in containers {
		"securityContext" in object.keys(container)
		container.securityContext.runAsNonRoot == true
	}
}

# Check if containers have health checks
has_health_checks if {
	input.kind == "Deployment"
	containers := input.spec.template.spec.containers
	count(containers) > 0
	every container in containers {
		"livenessProbe" in object.keys(container)
		"readinessProbe" in object.keys(container)
	}
}

# Check if containers are not privileged
not_privileged if {
	input.kind == "Deployment"
	containers := input.spec.template.spec.containers
	count(containers) > 0
	every container in containers {
		"securityContext" in object.keys(container)
		not object.get(container.securityContext, "privileged", false)
	}
}

# Check if containers use read-only root filesystem
readonly_root_filesystem if {
	input.kind == "Deployment"
	containers := input.spec.template.spec.containers
	count(containers) > 0
	every container in containers {
		"securityContext" in object.keys(container)
		container.securityContext.readOnlyRootFilesystem == true
	}
}

# Low security score violation
low_security_score contains violation if {
	input.kind == "Deployment"
	input.environment == "production"

	score := security_score
	score < 60

	violation := {
		"severity": "error",
		"message": sprintf("Low security posture score: %d/100", [score]),
		"hint": "Production deployments should score at least 60/100. Add security context, health checks, and non-root user.",
		"field": "spec.template.spec",
	}
}

#-----------------------------------------------------------------------------
# Environment-Specific Validation
#-----------------------------------------------------------------------------

# Development environment: Allow Alpine images (informational)
alpine_allowed_in_dev if {
	input.kind == "Dockerfile"
	input.environment == "development"
	# Alpine is explicitly allowed in dev
}

#-----------------------------------------------------------------------------
# Main Validation Rules
#-----------------------------------------------------------------------------

# Collect all blocking violations
deny contains violation if {
	some v in over_provisioned_cpu
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in over_provisioned_memory
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in alpine_in_production
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in missing_nonroot_user_production
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in missing_health_checks_production
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in low_security_score
	v.severity == "error"
	violation := v
}

# Collect all warning violations
warn contains violation if {
	some v in over_provisioned_cpu
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in over_provisioned_memory
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in alpine_in_production
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in missing_nonroot_user_production
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in missing_health_checks_production
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in low_security_score
	v.severity == "warning"
	violation := v
}
