package containerization.dynamic_defaults

# Dynamic Defaults Policy
# Sprint 3: Calculates context-aware defaults for replicas, health checks, and HPA
#
# This policy computes intelligent defaults based on:
# - Environment (dev=1 replica, prod=3+ replicas)
# - Language/framework startup characteristics (Java=longer timeouts than Go)
# - Traffic level (high/medium/low)
# - Criticality tier (tier-1/tier-2/tier-3)

import rego.v1

# ===== REPLICA COUNT CALCULATION =====

# Base replica count by environment
default replica_count_base := 1

replica_count_base := 1 if {
	input.environment == "development"
}

replica_count_base := 2 if {
	input.environment == "staging"
}

replica_count_base := 3 if {
	input.environment == "production"
}

# Adjust replicas based on traffic level
default traffic_multiplier := 1

traffic_multiplier := 2 if {
	input.trafficLevel == "high"
}

traffic_multiplier := 1 if {
	input.trafficLevel in {"medium", "low"}
}

# Adjust replicas based on criticality tier
default criticality_multiplier := 1

criticality_multiplier := 2 if {
	input.criticalityTier == "tier-1"
}

criticality_multiplier := 1 if {
	input.criticalityTier in {"tier-2", "tier-3"}
}

# Final replica count
replicas := replica_count_base * traffic_multiplier * criticality_multiplier

# ===== HEALTH CHECK CONFIGURATION =====

# Startup time estimates by language (seconds)
default language_startup_time := 30

language_startup_time := 120 if {
	input.language == "java"
}

language_startup_time := 30 if {
	input.language in {"node", "javascript", "typescript"}
}

language_startup_time := 45 if {
	input.language == "python"
}

language_startup_time := 10 if {
	input.language == "go"
}

language_startup_time := 15 if {
	input.language == "rust"
}

language_startup_time := 60 if {
	input.language == "dotnet"
}

# Health check period by environment
default health_check_period := 10

health_check_period := 10 if {
	input.environment == "production"
}

health_check_period := 15 if {
	input.environment == "staging"
}

health_check_period := 30 if {
	input.environment == "development"
}

# Health check configuration
health_checks := {
	"initialDelaySeconds": round(language_startup_time * 0.8),
	"periodSeconds": health_check_period,
	"timeoutSeconds": 5,
	"failureThreshold": 3,
	"successThreshold": 1,
}

# ===== AUTO-SCALING (HPA) CONFIGURATION =====

# CPU/Memory targets by environment
default hpa_cpu_target := 75

default hpa_memory_target := 80

hpa_cpu_target := 70 if {
	input.environment == "production"
}

hpa_memory_target := 80 if {
	input.environment == "production"
}

hpa_cpu_target := 80 if {
	input.environment == "staging"
}

hpa_memory_target := 85 if {
	input.environment == "staging"
}

hpa_cpu_target := 90 if {
	input.environment == "development"
}

hpa_memory_target := 90 if {
	input.environment == "development"
}

# Combined HPA configuration
autoscaling := {
	"minReplicas": replicas,
	"maxReplicas": replicas * 3,
	"targetCPUUtilization": hpa_cpu_target,
	"targetMemoryUtilization": hpa_memory_target,
}

# ===== DYNAMIC DEFAULTS QUERY ENTRYPOINT =====

# Return all dynamic defaults
# This is called by tools via ctx.queryConfig('containerization.dynamic_defaults.defaults', input)
defaults := {
	"replicas": replicas,
	"healthChecks": health_checks,
	"autoscaling": autoscaling,
}
