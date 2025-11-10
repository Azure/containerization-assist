# Workflow Validation Policy
#
# Cross-tool consistency checks for containerization workflows.
# Validates consistency between Dockerfiles and Kubernetes manifests
# to catch deployment mismatches early.
#
# Sprint 4: Semantic Validation & Cross-Tool Consistency

package workflow_validation

import rego.v1

#-----------------------------------------------------------------------------
# Helper Functions: Extraction
#-----------------------------------------------------------------------------

# Extract base image name from Dockerfile content
extract_dockerfile_image(content) := image if {
	lines := split(content, "\n")
	from_lines := [line | line := lines[_]; startswith(trim_space(line), "FROM")]
	count(from_lines) > 0
	last_from := from_lines[count(from_lines) - 1]

	# Parse FROM instruction (handle "FROM image" and "FROM image AS stage")
	parts := regex.find_n(`FROM\s+([^\s]+)(?:\s+AS\s+[^\s]+)?`, last_from, -1)
	count(parts) > 0
	image := trim_space(split(parts[0], " ")[1])
}

# Extract EXPOSE ports from Dockerfile content
extract_dockerfile_ports(content) := ports if {
	lines := split(content, "\n")
	expose_lines := [line | line := lines[_]; startswith(trim_space(line), "EXPOSE")]

	port_list := [port |
		line := expose_lines[_]
		port_spec := trim_space(substring(line, 7, -1)) # Remove "EXPOSE "
		port_strings := split(port_spec, " ")
		port_str := port_strings[_]
		# Remove protocol suffix (e.g., "3000/tcp" -> "3000")
		port_num := split(port_str, "/")[0]
		port := to_number(port_num)
	]

	ports := port_list
}

# Extract container images from Kubernetes manifest
extract_manifest_images(manifest) := images if {
	manifest.kind == "Deployment"
	containers := manifest.spec.template.spec.containers
	images := [c.image | c := containers[_]]
}

extract_manifest_images(manifest) := images if {
	manifest.kind == "Pod"
	containers := manifest.spec.containers
	images := [c.image | c := containers[_]]
}

# Extract target ports from Kubernetes Service
extract_service_ports(service) := ports if {
	service.kind == "Service"
	port_specs := service.spec.ports
	ports := [p.targetPort |
		p := port_specs[_]
		p.targetPort
		is_number(p.targetPort)
	]
}

#-----------------------------------------------------------------------------
# Image Name Consistency Validation
#-----------------------------------------------------------------------------

# Check if built image name matches deployed image name
image_name_mismatch contains violation if {
	# Workflow must provide both dockerfile and manifest
	input.dockerfile
	input.manifest

	# Extract built image name from Dockerfile
	dockerfile_image := extract_dockerfile_image(input.dockerfile.content)

	# Extract deployed image names from manifest
	manifest_images := extract_manifest_images(input.manifest)

	# Check if any manifest image matches the built image
	count(manifest_images) > 0
	not dockerfile_image in manifest_images

	violation := {
		"severity": "error",
		"message": "Image name mismatch between Dockerfile and Kubernetes manifest",
		"hint": sprintf(
			"Dockerfile builds '%s' but manifest deploys %v. This will cause deployment failures.",
			[dockerfile_image, manifest_images],
		),
		"field": "workflow",
	}
}

#-----------------------------------------------------------------------------
# Port Consistency Validation
#-----------------------------------------------------------------------------

# Check if EXPOSE ports match Service targetPort
port_mismatch contains violation if {
	# Workflow must provide dockerfile, manifest, and service
	input.dockerfile
	input.service

	# Extract EXPOSE ports from Dockerfile
	dockerfile_ports := extract_dockerfile_ports(input.dockerfile.content)
	count(dockerfile_ports) > 0

	# Extract target ports from Service
	service_ports := extract_service_ports(input.service)
	count(service_ports) > 0

	# Check for mismatches
	some service_port in service_ports
	not service_port in dockerfile_ports

	violation := {
		"severity": "warning",
		"message": sprintf("Service targetPort %d not exposed in Dockerfile", [service_port]),
		"hint": sprintf(
			"Dockerfile exposes %v but Service targets port %d. Consider adding EXPOSE %d to Dockerfile.",
			[dockerfile_ports, service_port, service_port],
		),
		"field": "workflow",
	}
}

#-----------------------------------------------------------------------------
# Resource Consistency Validation
#-----------------------------------------------------------------------------

# Check if Dockerfile and manifest have consistent resource assumptions
# For example, if Dockerfile runs heavy builds, manifest should have adequate resources
resource_assumptions_mismatch contains violation if {
	input.dockerfile
	input.manifest

	# Check if Dockerfile has build stages (indicates resource-intensive build)
	content := input.dockerfile.content
	has_multi_stage_build := count([line |
		line := split(content, "\n")[_]
		startswith(trim_space(line), "FROM")
	]) > 1

	# Check if manifest has resource limits
	containers := input.manifest.spec.template.spec.containers
	some container in containers
	not container.resources
	has_multi_stage_build

	violation := {
		"severity": "warning",
		"message": sprintf("Container '%s' missing resource limits despite complex Dockerfile", [container.name]),
		"hint": "Multi-stage Dockerfile indicates resource-intensive build. Consider adding resource requests/limits to manifest.",
		"field": sprintf("spec.template.spec.containers[%s].resources", [container.name]),
	}
}

#-----------------------------------------------------------------------------
# Security Context Consistency
#-----------------------------------------------------------------------------

# Check if Dockerfile USER matches manifest securityContext
user_context_mismatch contains violation if {
	input.dockerfile
	input.manifest

	# Check if Dockerfile specifies USER
	content := input.dockerfile.content
	user_lines := [line |
		line := split(content, "\n")[_]
		startswith(trim_space(line), "USER")
	]
	count(user_lines) > 0

	# Check if manifest enforces runAsNonRoot
	containers := input.manifest.spec.template.spec.containers
	some container in containers
	not container.securityContext.runAsNonRoot

	violation := {
		"severity": "warning",
		"message": sprintf("Container '%s' Dockerfile has USER but manifest doesn't enforce runAsNonRoot", [container.name]),
		"hint": "Add securityContext.runAsNonRoot: true to manifest to match Dockerfile security posture.",
		"field": sprintf("spec.template.spec.containers[%s].securityContext", [container.name]),
	}
}

#-----------------------------------------------------------------------------
# Health Check Consistency
#-----------------------------------------------------------------------------

# Check if Dockerfile HEALTHCHECK matches manifest probes
healthcheck_consistency contains suggestion if {
	input.dockerfile
	input.manifest

	# Check if Dockerfile has HEALTHCHECK
	content := input.dockerfile.content
	healthcheck_lines := [line |
		line := split(content, "\n")[_]
		startswith(trim_space(line), "HEALTHCHECK")
	]
	count(healthcheck_lines) > 0

	# Check if manifest has probes
	containers := input.manifest.spec.template.spec.containers
	some container in containers
	not container.livenessProbe

	suggestion := {
		"severity": "suggestion",
		"message": sprintf("Container '%s' has Dockerfile HEALTHCHECK but no Kubernetes probes", [container.name]),
		"hint": "Consider adding livenessProbe and readinessProbe to manifest based on Dockerfile HEALTHCHECK.",
		"field": sprintf("spec.template.spec.containers[%s]", [container.name]),
	}
}

#-----------------------------------------------------------------------------
# Main Validation Rules
#-----------------------------------------------------------------------------

# Collect all blocking violations
deny contains violation if {
	some v in image_name_mismatch
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in port_mismatch
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in resource_assumptions_mismatch
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in user_context_mismatch
	v.severity == "error"
	violation := v
}

deny contains violation if {
	some v in healthcheck_consistency
	v.severity == "error"
	violation := v
}

# Collect all warning violations
warn contains violation if {
	some v in image_name_mismatch
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in port_mismatch
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in resource_assumptions_mismatch
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in user_context_mismatch
	v.severity == "warning"
	violation := v
}

warn contains violation if {
	some v in healthcheck_consistency
	v.severity == "warning"
	violation := v
}

# Collect all suggestions
suggest contains suggestion if {
	some s in image_name_mismatch
	s.severity == "suggestion"
	suggestion := s
}

suggest contains suggestion if {
	some s in port_mismatch
	s.severity == "suggestion"
	suggestion := s
}

suggest contains suggestion if {
	some s in resource_assumptions_mismatch
	s.severity == "suggestion"
	suggestion := s
}

suggest contains suggestion if {
	some s in user_context_mismatch
	s.severity == "suggestion"
	suggestion := s
}

suggest contains suggestion if {
	some s in healthcheck_consistency
	s.severity == "suggestion"
	suggestion := s
}
