package containerization.dynamic_defaults

import rego.v1

# ===== REPLICA COUNT TESTS =====

test_dev_environment_one_replica if {
	result := defaults with input as {"environment": "development"}
	result.replicas == 1
}

test_staging_environment_two_replicas if {
	result := defaults with input as {"environment": "staging"}
	result.replicas == 2
}

test_prod_environment_three_replicas if {
	result := defaults with input as {"environment": "production"}
	result.replicas == 3
}

test_high_traffic_doubles_replicas if {
	result := defaults with input as {
		"environment": "production",
		"trafficLevel": "high",
	}
	result.replicas == 6 # 3 base * 2 traffic multiplier
}

test_tier1_criticality_doubles_replicas if {
	result := defaults with input as {
		"environment": "production",
		"criticalityTier": "tier-1",
	}
	result.replicas == 6 # 3 base * 2 criticality multiplier
}

test_combined_multipliers if {
	result := defaults with input as {
		"environment": "production",
		"trafficLevel": "high",
		"criticalityTier": "tier-1",
	}
	result.replicas == 12 # 3 base * 2 traffic * 2 criticality
}

# ===== HEALTH CHECK TESTS =====

test_java_long_startup_time if {
	result := defaults with input as {
		"language": "java",
		"environment": "production",
	}
	result.healthChecks.initialDelaySeconds == 96 # 80% of 120 seconds
}

test_go_short_startup_time if {
	result := defaults with input as {
		"language": "go",
		"environment": "production",
	}
	result.healthChecks.initialDelaySeconds == 8 # 80% of 10 seconds
}

test_node_moderate_startup if {
	result := defaults with input as {
		"language": "node",
		"environment": "production",
	}
	result.healthChecks.initialDelaySeconds == 24 # 80% of 30 seconds
}

test_production_frequent_health_checks if {
	result := defaults with input as {
		"language": "python",
		"environment": "production",
	}
	result.healthChecks.periodSeconds == 10
}

test_dev_infrequent_health_checks if {
	result := defaults with input as {
		"language": "python",
		"environment": "development",
	}
	result.healthChecks.periodSeconds == 30
}

test_health_check_has_all_fields if {
	result := defaults with input as {
		"language": "java",
		"environment": "production",
	}
	result.healthChecks.initialDelaySeconds > 0
	result.healthChecks.periodSeconds > 0
	result.healthChecks.timeoutSeconds == 5
	result.healthChecks.failureThreshold == 3
	result.healthChecks.successThreshold == 1
}

# ===== AUTOSCALING (HPA) TESTS =====

test_hpa_min_equals_base_replicas if {
	result := defaults with input as {"environment": "production"}
	result.autoscaling.minReplicas == 3
}

test_hpa_max_is_3x_base if {
	result := defaults with input as {"environment": "production"}
	result.autoscaling.maxReplicas == 9 # 3 * 3
}

test_production_aggressive_cpu_target if {
	result := defaults with input as {"environment": "production"}
	result.autoscaling.targetCPUUtilization == 70
}

test_dev_conservative_cpu_target if {
	result := defaults with input as {"environment": "development"}
	result.autoscaling.targetCPUUtilization == 90
}

test_hpa_with_high_traffic if {
	result := defaults with input as {
		"environment": "production",
		"trafficLevel": "high",
	}
	result.autoscaling.minReplicas == 6 # 3 base * 2 traffic
	result.autoscaling.maxReplicas == 18 # 6 * 3
}

# ===== COMBINED SCENARIO TESTS =====

test_production_java_tier1_high_traffic if {
	result := defaults with input as {
		"language": "java",
		"environment": "production",
		"trafficLevel": "high",
		"criticalityTier": "tier-1",
	}

	# Replicas: 3 base * 2 traffic * 2 criticality = 12
	result.replicas == 12

	# Health checks: Java has long startup (120s -> 96s initial delay)
	result.healthChecks.initialDelaySeconds == 96

	# HPA: min=12, max=36 (12*3)
	result.autoscaling.minReplicas == 12
	result.autoscaling.maxReplicas == 36
}

test_dev_go_minimal_config if {
	result := defaults with input as {
		"language": "go",
		"environment": "development",
	}

	# Replicas: 1 (dev baseline)
	result.replicas == 1

	# Health checks: Go has fast startup (10s -> 8s initial delay)
	result.healthChecks.initialDelaySeconds == 8

	# HPA: min=1, max=3
	result.autoscaling.minReplicas == 1
	result.autoscaling.maxReplicas == 3
}

test_staging_python_medium_traffic if {
	result := defaults with input as {
		"language": "python",
		"environment": "staging",
		"trafficLevel": "medium",
	}

	# Replicas: 2 (staging baseline)
	result.replicas == 2

	# Health checks: Python moderate startup (45s -> 36s initial delay)
	result.healthChecks.initialDelaySeconds == 36
	result.healthChecks.periodSeconds == 15 # staging period

	# HPA: min=2, max=6
	result.autoscaling.minReplicas == 2
	result.autoscaling.maxReplicas == 6
}
