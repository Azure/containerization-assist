# Test Suite: Workflow Validation Policy
#
# Tests for cross-tool consistency checks including image name matching,
# port consistency, and resource alignment.

package workflow_validation

import rego.v1

#-----------------------------------------------------------------------------
# Test: Image Extraction
#-----------------------------------------------------------------------------

test_extract_dockerfile_image_simple if {
	content := "FROM node:18\nRUN npm install\n"
	image := extract_dockerfile_image(content)
	image == "node:18"
}

test_extract_dockerfile_image_multistage if {
	content := "FROM node:18 AS builder\nRUN npm build\nFROM nginx:alpine\nCOPY --from=builder /app /usr/share/nginx/html\n"
	image := extract_dockerfile_image(content)
	image == "nginx:alpine"
}

test_extract_manifest_images_deployment if {
	manifest := {
		"kind": "Deployment",
		"spec": {"template": {"spec": {"containers": [
			{"name": "app", "image": "myapp:v1"},
			{"name": "sidecar", "image": "sidecar:latest"},
		]}}},
	}
	images := extract_manifest_images(manifest)
	images == ["myapp:v1", "sidecar:latest"]
}

#-----------------------------------------------------------------------------
# Test: Port Extraction
#-----------------------------------------------------------------------------

test_extract_dockerfile_ports_single if {
	content := "FROM node:18\nEXPOSE 3000\n"
	ports := extract_dockerfile_ports(content)
	ports == [3000]
}

test_extract_dockerfile_ports_multiple if {
	content := "FROM node:18\nEXPOSE 3000\nEXPOSE 8080 9090\n"
	ports := extract_dockerfile_ports(content)
	count(ports) == 3
	3000 in ports
	8080 in ports
	9090 in ports
}

test_extract_service_ports if {
	service := {
		"kind": "Service",
		"spec": {"ports": [
			{"port": 80, "targetPort": 3000},
			{"port": 443, "targetPort": 8443},
		]},
	}
	ports := extract_service_ports(service)
	ports == [3000, 8443]
}

#-----------------------------------------------------------------------------
# Test: Image Name Mismatch
#-----------------------------------------------------------------------------

test_image_name_mismatch_detected if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nCOPY . /app\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{"name": "app", "image": "myapp:v1"}]}}},
		},
	}

	violations := image_name_mismatch with input as workflow
	count(violations) == 1
	some v in violations
	v.severity == "error"
	contains(v.message, "mismatch")
}

test_image_name_match_passes if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nCOPY . /app\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{"name": "app", "image": "node:18"}]}}},
		},
	}

	violations := image_name_mismatch with input as workflow
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Port Mismatch
#-----------------------------------------------------------------------------

test_port_mismatch_detected if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nEXPOSE 3000\n"},
		"service": {
			"kind": "Service",
			"spec": {"ports": [{"port": 80, "targetPort": 8080}]},
		},
	}

	violations := port_mismatch with input as workflow
	count(violations) == 1
	some v in violations
	v.severity == "warning"
	contains(v.message, "8080")
}

test_port_match_passes if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nEXPOSE 3000\n"},
		"service": {
			"kind": "Service",
			"spec": {"ports": [{"port": 80, "targetPort": 3000}]},
		},
	}

	violations := port_mismatch with input as workflow
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Resource Assumptions Mismatch
#-----------------------------------------------------------------------------

test_resource_assumptions_mismatch_detected if {
	workflow := {
		"dockerfile": {"content": "FROM node:18 AS builder\nRUN npm build\nFROM nginx:alpine\nCOPY --from=builder /app /usr/share/nginx/html\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{"name": "app"}]}}},
		},
	}

	violations := resource_assumptions_mismatch with input as workflow
	count(violations) == 1
	some v in violations
	v.severity == "warning"
	contains(v.message, "resource limits")
}

test_resource_assumptions_with_limits_passes if {
	workflow := {
		"dockerfile": {"content": "FROM node:18 AS builder\nRUN npm build\nFROM nginx:alpine\nCOPY --from=builder /app /usr/share/nginx/html\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{
				"name": "app",
				"resources": {
					"limits": {"cpu": "1000m", "memory": "512Mi"},
					"requests": {"cpu": "500m", "memory": "256Mi"},
				},
			}]}}},
		},
	}

	violations := resource_assumptions_mismatch with input as workflow
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: User Context Mismatch
#-----------------------------------------------------------------------------

test_user_context_mismatch_detected if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nUSER node\nCMD [\"node\", \"app.js\"]\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{"name": "app", "securityContext": {}}]}}},
		},
	}

	violations := user_context_mismatch with input as workflow
	count(violations) == 1
	some v in violations
	v.severity == "warning"
	contains(v.message, "runAsNonRoot")
}

test_user_context_match_passes if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nUSER node\nCMD [\"node\", \"app.js\"]\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{
				"name": "app",
				"securityContext": {"runAsNonRoot": true},
			}]}}},
		},
	}

	violations := user_context_mismatch with input as workflow
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Health Check Consistency
#-----------------------------------------------------------------------------

test_healthcheck_consistency_suggestion if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nHEALTHCHECK CMD curl -f http://localhost:3000/health\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{"name": "app"}]}}},
		},
	}

	suggestions := healthcheck_consistency with input as workflow
	count(suggestions) == 1
	some s in suggestions
	s.severity == "suggestion"
	contains(s.message, "HEALTHCHECK")
}

test_healthcheck_consistency_with_probes_passes if {
	workflow := {
		"dockerfile": {"content": "FROM node:18\nHEALTHCHECK CMD curl -f http://localhost:3000/health\n"},
		"manifest": {
			"kind": "Deployment",
			"spec": {"template": {"spec": {"containers": [{
				"name": "app",
				"livenessProbe": {"httpGet": {"path": "/health"}},
				"readinessProbe": {"httpGet": {"path": "/ready"}},
			}]}}},
		},
	}

	suggestions := healthcheck_consistency with input as workflow
	count(suggestions) == 0
}
