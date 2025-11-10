package containerization.templates

import rego.v1

# ===== DOCKERFILE TEMPLATE TESTS =====

test_ca_cert_template_always_included if {
	result := templates with input as {
		"language": "python",
		"environment": "development",
	}
	count(result.dockerfile) > 0
	some template in result.dockerfile
	template.id == "org-ca-certificates"
}

test_java_observability_production if {
	result := templates with input as {
		"language": "java",
		"environment": "production",
	}
	some template in result.dockerfile
	template.id == "org-java-observability"
}

test_java_observability_not_in_dev if {
	result := templates with input as {
		"language": "java",
		"environment": "development",
	}
	not_observability := [template | template := result.dockerfile[_]; template.id == "org-java-observability"]
	count(not_observability) == 0
}

test_node_observability_production if {
	result := templates with input as {
		"language": "node",
		"environment": "production",
	}
	some template in result.dockerfile
	template.id == "org-node-observability"
}

test_security_hardening_production_only if {
	result := templates with input as {
		"language": "python",
		"environment": "production",
	}
	some template in result.dockerfile
	template.id == "org-security-hardening"
}

test_security_hardening_not_in_dev if {
	result := templates with input as {
		"language": "python",
		"environment": "development",
	}
	not_hardening := [template | template := result.dockerfile[_]; template.id == "org-security-hardening"]
	count(not_hardening) == 0
}

# ===== KUBERNETES TEMPLATE TESTS =====

test_log_forwarder_sidecar_production if {
	result := templates with input as {
		"language": "java",
		"environment": "production",
	}
	some template in result.kubernetes
	template.id == "org-log-forwarder"
	template.type == "sidecar"
}

test_log_forwarder_not_in_dev if {
	result := templates with input as {
		"language": "java",
		"environment": "development",
	}
	not_forwarder := [template | template := result.kubernetes[_]; template.id == "org-log-forwarder"]
	count(not_forwarder) == 0
}

test_secrets_volume_always_included if {
	result := templates with input as {
		"language": "python",
		"environment": "development",
	}
	some template in result.kubernetes
	template.id == "org-secrets-volume"
}

test_secrets_volume_mount_always_included if {
	result := templates with input as {
		"language": "go",
		"environment": "staging",
	}
	some template in result.kubernetes
	template.id == "org-secrets-volume-mount"
}

test_db_migration_java_only if {
	result := templates with input as {
		"language": "java",
		"environment": "production",
	}
	some template in result.kubernetes
	template.id == "org-db-migration"
}

test_db_migration_not_for_python if {
	result := templates with input as {
		"language": "python",
		"environment": "production",
	}
	not_migration := [template | template := result.kubernetes[_]; template.id == "org-db-migration"]
	count(not_migration) == 0
}

# ===== COMBINED SCENARIO TESTS =====

test_production_java_gets_all_templates if {
	result := templates with input as {
		"language": "java",
		"environment": "production",
	}

	# Should have: CA certs, Java observability, security hardening
	count(result.dockerfile) >= 3

	# Should have: log forwarder, secrets volume, secrets mount, db migration
	count(result.kubernetes) >= 4
}

test_dev_python_minimal_templates if {
	result := templates with input as {
		"language": "python",
		"environment": "development",
	}

	# Should only have CA certs
	count(result.dockerfile) == 1

	# Should only have secrets volume and mount
	count(result.kubernetes) == 2
}
