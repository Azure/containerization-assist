# Test Suite: Semantic Validation Policy
#
# Tests for semantic validation rules including resource efficiency,
# security posture, and environment-specific validation.

package semantic_validation

import rego.v1

#-----------------------------------------------------------------------------
# Test: Resource Parsing
#-----------------------------------------------------------------------------

test_parse_cpu_millicores if {
	parse_cpu("1000m") == 1000
}

test_parse_cpu_cores if {
	parse_cpu("1.5") == 1500
}

test_parse_cpu_integer_cores if {
	parse_cpu("2") == 2000
}

test_parse_memory_mebibytes if {
	parse_memory("512Mi") == 536870912
}

test_parse_memory_gibibytes if {
	parse_memory("2Gi") == 2147483648
}

test_parse_memory_megabytes if {
	parse_memory("512M") == 512000000
}

test_parse_memory_gigabytes if {
	parse_memory("1G") == 1000000000
}

#-----------------------------------------------------------------------------
# Test: Over-Provisioned CPU
#-----------------------------------------------------------------------------

test_over_provisioned_cpu_detected if {
	manifest := {
		"kind": "Deployment",
		"spec": {"template": {"spec": {"containers": [{
			"name": "app",
			"resources": {
				"limits": {"cpu": "4000m"},
				"requests": {"cpu": "500m"},
			},
		}]}}},
	}

	violations := over_provisioned_cpu with input as manifest
	count(violations) == 1
	some v in violations
	v.severity == "warning"
	contains(v.message, "8x")
}

test_over_provisioned_cpu_not_triggered_on_reasonable_ratio if {
	manifest := {
		"kind": "Deployment",
		"spec": {"template": {"spec": {"containers": [{
			"name": "app",
			"resources": {
				"limits": {"cpu": "2000m"},
				"requests": {"cpu": "1000m"},
			},
		}]}}},
	}

	violations := over_provisioned_cpu with input as manifest
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Over-Provisioned Memory
#-----------------------------------------------------------------------------

test_over_provisioned_memory_detected if {
	manifest := {
		"kind": "Deployment",
		"spec": {"template": {"spec": {"containers": [{
			"name": "app",
			"resources": {
				"limits": {"memory": "4Gi"},
				"requests": {"memory": "512Mi"},
			},
		}]}}},
	}

	violations := over_provisioned_memory with input as manifest
	count(violations) == 1
	some v in violations
	v.severity == "warning"
	contains(v.message, "8x")
}

test_over_provisioned_memory_not_triggered_on_reasonable_ratio if {
	manifest := {
		"kind": "Deployment",
		"spec": {"template": {"spec": {"containers": [{
			"name": "app",
			"resources": {
				"limits": {"memory": "2Gi"},
				"requests": {"memory": "1Gi"},
			},
		}]}}},
	}

	violations := over_provisioned_memory with input as manifest
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Alpine in Production
#-----------------------------------------------------------------------------

test_alpine_in_production_detected if {
	dockerfile := {
		"kind": "Dockerfile",
		"environment": "production",
		"content": "FROM node:18-alpine\nRUN npm install\n",
	}

	violations := alpine_in_production with input as dockerfile
	count(violations) == 1
	some v in violations
	v.severity == "warning"
	contains(v.message, "Alpine")
}

test_alpine_in_development_allowed if {
	dockerfile := {
		"kind": "Dockerfile",
		"environment": "development",
		"content": "FROM node:18-alpine\nRUN npm install\n",
	}

	violations := alpine_in_production with input as dockerfile
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Missing Non-Root User in Production
#-----------------------------------------------------------------------------

test_missing_nonroot_user_production_detected if {
	dockerfile := {
		"kind": "Dockerfile",
		"environment": "production",
		"content": "FROM node:18\nRUN npm install\nCMD [\"node\", \"app.js\"]\n",
	}

	violations := missing_nonroot_user_production with input as dockerfile
	count(violations) == 1
	some v in violations
	v.severity == "error"
	contains(v.message, "USER")
}

test_nonroot_user_present_passes if {
	dockerfile := {
		"kind": "Dockerfile",
		"environment": "production",
		"content": "FROM node:18\nRUN npm install\nUSER node\nCMD [\"node\", \"app.js\"]\n",
	}

	violations := missing_nonroot_user_production with input as dockerfile
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Missing Health Checks in Production
#-----------------------------------------------------------------------------

test_missing_health_checks_production_detected if {
	manifest := {
		"kind": "Deployment",
		"environment": "production",
		"spec": {"template": {"spec": {"containers": [{"name": "app"}]}}},
	}

	violations := missing_health_checks_production with input as manifest
	count(violations) == 1
	some v in violations
	v.severity == "error"
	contains(v.message, "health checks")
}

test_health_checks_present_passes if {
	manifest := {
		"kind": "Deployment",
		"environment": "production",
		"spec": {"template": {"spec": {"containers": [{
			"name": "app",
			"livenessProbe": {"httpGet": {"path": "/health"}},
			"readinessProbe": {"httpGet": {"path": "/ready"}},
		}]}}},
	}

	violations := missing_health_checks_production with input as manifest
	count(violations) == 0
}

#-----------------------------------------------------------------------------
# Test: Security Score
#-----------------------------------------------------------------------------

test_security_score_perfect if {
	manifest := {
		"kind": "Deployment",
		"spec": {"template": {"spec": {"containers": [{
			"name": "app",
			"securityContext": {
				"runAsNonRoot": true,
				"privileged": false,
				"readOnlyRootFilesystem": true,
			},
			"livenessProbe": {"httpGet": {"path": "/health"}},
			"readinessProbe": {"httpGet": {"path": "/ready"}},
		}]}}},
	}

	score := security_score with input as manifest
	score == 100
}

test_security_score_low if {
	manifest := {
		"kind": "Deployment",
		"spec": {"template": {"spec": {"containers": [{"name": "app"}]}}},
	}

	score := security_score with input as manifest
	score == 0
}

#-----------------------------------------------------------------------------
# Test: Low Security Score Violation
#-----------------------------------------------------------------------------

test_low_security_score_in_production if {
	manifest := {
		"kind": "Deployment",
		"environment": "production",
		"spec": {"template": {"spec": {"containers": [{"name": "app"}]}}},
	}

	violations := low_security_score with input as manifest
	count(violations) == 1
	some v in violations
	v.severity == "error"
	contains(v.message, "Low security posture")
}

test_good_security_score_passes if {
	manifest := {
		"kind": "Deployment",
		"environment": "production",
		"spec": {"template": {"spec": {"containers": [{
			"name": "app",
			"securityContext": {
				"runAsNonRoot": true,
				"privileged": false,
				"readOnlyRootFilesystem": true,
			},
			"livenessProbe": {"httpGet": {"path": "/health"}},
			"readinessProbe": {"httpGet": {"path": "/ready"}},
		}]}}},
	}

	violations := low_security_score with input as manifest
	count(violations) == 0
}
