# Dynamic Defaults Examples

Policy-driven dynamic defaults allow you to automatically calculate environment-aware configuration values like replica counts, health check timings, and autoscaling parameters.

## Overview

Dynamic defaults enable:
- Language-specific health check timings (Java=120s, Go=10s)
- Environment-based replica counts (dev=1, prod=3)
- Traffic-aware scaling (high traffic = more replicas)
- Criticality-based resource allocation (tier-1 apps get more)

## Example 1: Language-Specific Health Checks

### Policy File: `policies/health-checks.rego`

```rego
package containerization.dynamic_defaults

import rego.v1

# Health check configuration based on language startup characteristics
health_checks := config if {
  config := {
    "startupPeriodSeconds": startup_period,
    "livenessProbe": {
      "periodSeconds": 10,
      "timeoutSeconds": 3,
      "failureThreshold": 3
    },
    "readinessProbe": {
      "periodSeconds": 5,
      "timeoutSeconds": 2,
      "failureThreshold": 2
    }
  }
}

# Java apps need long startup time (JVM initialization)
startup_period := 120 if { input.language == "java" }

# Go apps start very quickly
startup_period := 10 if { input.language == "go" }

# Node.js moderate startup
startup_period := 30 if { input.language in {"node", "javascript", "typescript"} }

# Python moderate startup
startup_period := 45 if { input.language == "python" }

# Rust very fast startup
startup_period := 15 if { input.language == "rust" }

# .NET longer startup
startup_period := 60 if { input.language in {"csharp", "dotnet"} }

# Default for unknown languages
default startup_period := 60

defaults := {
  "healthChecks": health_checks
}
```

### Usage

```bash
# Java app gets 120s startup period
containerization-assist generate-k8s-manifests \
  --app-name java-api \
  --language java \
  --environment production

# Go app gets 10s startup period
containerization-assist generate-k8s-manifests \
  --app-name go-api \
  --language go \
  --environment production
```

## Example 2: Replica Count Calculation

### Policy File: `policies/replica-calculation.rego`

```rego
package containerization.dynamic_defaults

import rego.v1

# Calculate replica count based on environment, traffic, and criticality
replicas := count if {
  # Base replica count by environment
  base := base_replicas

  # Traffic multiplier (high/medium/low)
  traffic_mult := traffic_multiplier

  # Criticality multiplier (tier-1/tier-2/tier-3)
  crit_mult := criticality_multiplier

  # Final count (minimum 1)
  count := max([1, base * traffic_mult * crit_mult])
}

# Base replicas by environment
base_replicas := 1 if { input.environment == "development" }
base_replicas := 2 if { input.environment == "staging" }
base_replicas := 3 if { input.environment == "production" }
default base_replicas := 1

# Traffic level multipliers
traffic_multiplier := 2 if { input.trafficLevel == "high" }
traffic_multiplier := 1 if { input.trafficLevel == "medium" }
traffic_multiplier := 1 if { input.trafficLevel == "low" }
default traffic_multiplier := 1

# Criticality tier multipliers
criticality_multiplier := 2 if { input.criticalityTier == "tier-1" }
criticality_multiplier := 1 if { input.criticalityTier == "tier-2" }
criticality_multiplier := 1 if { input.criticalityTier == "tier-3" }
default criticality_multiplier := 1

defaults := {
  "replicas": replicas
}
```

### Usage

```bash
# Development: 1 replica (base=1, traffic=1, crit=1)
containerization-assist generate-k8s-manifests \
  --app-name my-api \
  --language node \
  --environment development

# Production high-traffic tier-1: 6 replicas (base=3, traffic=2, crit=2)
containerization-assist generate-k8s-manifests \
  --app-name critical-api \
  --language node \
  --environment production \
  --traffic-level high \
  --criticality-tier tier-1

# Production medium-traffic tier-2: 3 replicas (base=3, traffic=1, crit=1)
containerization-assist generate-k8s-manifests \
  --app-name standard-api \
  --language node \
  --environment production \
  --traffic-level medium \
  --criticality-tier tier-2
```

## Example 3: Autoscaling Configuration

### Policy File: `policies/autoscaling.rego`

```rego
package containerization.dynamic_defaults

import rego.v1

# Calculate HPA (Horizontal Pod Autoscaler) configuration
autoscaling := hpa_config if {
  base_replicas := replicas  # From replica calculation

  hpa_config := {
    "enabled": enabled,
    "minReplicas": base_replicas,
    "maxReplicas": base_replicas * 3,
    "targetCPUUtilizationPercentage": cpu_target,
    "targetMemoryUtilizationPercentage": memory_target
  }
}

# Autoscaling enabled in production/staging
enabled := true if { input.environment in {"production", "staging"} }
default enabled := false

# Aggressive scaling in production
cpu_target := 70 if { input.environment == "production" }
memory_target := 80 if { input.environment == "production" }

# Conservative scaling in staging
cpu_target := 80 if { input.environment == "staging" }
memory_target := 85 if { input.environment == "staging" }

# Default targets
default cpu_target := 75
default memory_target := 80

defaults := {
  "autoscaling": autoscaling
}
```

### Usage

```bash
# Production with autoscaling: min=3, max=9, CPU=70%
containerization-assist generate-k8s-manifests \
  --app-name api \
  --language node \
  --environment production

# Development without autoscaling
containerization-assist generate-k8s-manifests \
  --app-name api \
  --language node \
  --environment development
```

## Example 4: Complete Dynamic Defaults Policy

### Policy File: `policies/dynamic-defaults.rego`

```rego
package containerization.dynamic_defaults

import rego.v1

# ===== REPLICA CALCULATION =====

replicas := count if {
  base := base_replicas
  traffic_mult := traffic_multiplier
  crit_mult := criticality_multiplier
  count := max([1, base * traffic_mult * crit_mult])
}

base_replicas := 1 if { input.environment == "development" }
base_replicas := 2 if { input.environment == "staging" }
base_replicas := 3 if { input.environment == "production" }
default base_replicas := 1

traffic_multiplier := 2 if { input.trafficLevel == "high" }
traffic_multiplier := 1 if { input.trafficLevel in {"medium", "low"} }
default traffic_multiplier := 1

criticality_multiplier := 2 if { input.criticalityTier == "tier-1" }
criticality_multiplier := 1 if { input.criticalityTier in {"tier-2", "tier-3"} }
default criticality_multiplier := 1

# ===== HEALTH CHECK TIMING =====

health_checks := {
  "startupPeriodSeconds": startup_period,
  "livenessProbe": {
    "periodSeconds": 10,
    "timeoutSeconds": 3,
    "failureThreshold": 3
  },
  "readinessProbe": {
    "periodSeconds": 5,
    "timeoutSeconds": 2,
    "failureThreshold": 2
  }
}

startup_period := 120 if { input.language == "java" }
startup_period := 60 if { input.language in {"csharp", "dotnet"} }
startup_period := 45 if { input.language == "python" }
startup_period := 30 if { input.language in {"node", "javascript", "typescript"} }
startup_period := 15 if { input.language == "rust" }
startup_period := 10 if { input.language == "go" }
default startup_period := 60

# ===== AUTOSCALING =====

autoscaling := {
  "enabled": hpa_enabled,
  "minReplicas": replicas,
  "maxReplicas": replicas * 3,
  "targetCPUUtilizationPercentage": cpu_target,
  "targetMemoryUtilizationPercentage": memory_target
}

hpa_enabled := true if { input.environment in {"production", "staging"} }
default hpa_enabled := false

cpu_target := 70 if { input.environment == "production" }
cpu_target := 80 if { input.environment == "staging" }
default cpu_target := 75

memory_target := 80 if { input.environment == "production" }
memory_target := 85 if { input.environment == "staging" }
default memory_target := 80

# ===== EXPORT =====

defaults := {
  "replicas": replicas,
  "healthChecks": health_checks,
  "autoscaling": autoscaling
}
```

## Testing Dynamic Defaults

### Unit Tests

```rego
# File: policies/dynamic-defaults_test.rego
package containerization.dynamic_defaults

import rego.v1

test_java_gets_long_startup if {
  result := defaults with input as {"language": "java", "environment": "production"}
  result.healthChecks.startupPeriodSeconds == 120
}

test_go_gets_short_startup if {
  result := defaults with input as {"language": "go", "environment": "production"}
  result.healthChecks.startupPeriodSeconds == 10
}

test_production_high_traffic_tier1_replicas if {
  result := defaults with input as {
    "environment": "production",
    "trafficLevel": "high",
    "criticalityTier": "tier-1",
    "language": "node"
  }
  result.replicas == 12  # 3 * 2 * 2 = 12
}

test_development_gets_one_replica if {
  result := defaults with input as {"environment": "development", "language": "node"}
  result.replicas == 1
}

test_autoscaling_enabled_in_production if {
  result := defaults with input as {"environment": "production", "language": "node"}
  result.autoscaling.enabled == true
  result.autoscaling.minReplicas == 3
  result.autoscaling.maxReplicas == 9
}

test_autoscaling_disabled_in_development if {
  result := defaults with input as {"environment": "development", "language": "node"}
  result.autoscaling.enabled == false
}
```

### Run Tests

```bash
opa test policies/dynamic-defaults.rego policies/dynamic-defaults_test.rego
```

## Integration with Tools

Dynamic defaults automatically integrate with `generate-k8s-manifests`:

```typescript
// Tools query dynamic defaults automatically
const dynamicDefaultsQuery = await ctx.queryConfig<DynamicDefaults>(
  'containerization.dynamic_defaults.defaults',
  {
    language: input.language,
    environment: input.environment,
    trafficLevel: input.trafficLevel,
    criticalityTier: input.criticalityTier
  }
);

if (dynamicDefaultsQuery?.replicas) {
  // Apply replica count to manifest
  manifest.spec.replicas = dynamicDefaultsQuery.replicas;
}

if (dynamicDefaultsQuery?.healthChecks) {
  // Apply health check timing
  manifest.spec.template.spec.containers[0].startupProbe = {
    httpGet: { path: '/health', port: 8080 },
    initialDelaySeconds: dynamicDefaultsQuery.healthChecks.startupPeriodSeconds,
    ...dynamicDefaultsQuery.healthChecks.livenessProbe
  };
}
```

## Best Practices

1. **Use sensible defaults**: Always provide `default` values for undefined cases
2. **Document multipliers**: Explain why certain values are chosen (Java=120s because JVM init)
3. **Test edge cases**: Test with missing inputs, unknown languages, etc.
4. **Keep calculations simple**: Complex math should be well-commented
5. **Validate results**: Ensure calculated values are within acceptable ranges
6. **Consider cost**: High replica counts = high cloud costs
7. **Monitor actual usage**: Adjust policies based on real-world performance data

## Troubleshooting

### Dynamic defaults not appearing in output

```bash
# Check if policy is loaded
export LOG_LEVEL=debug
containerization-assist generate-k8s-manifests --app-name test --language node

# Query policy directly
echo '{"language": "node", "environment": "production"}' | \
  opa eval -d policies/dynamic-defaults.rego -f json 'data.containerization.dynamic_defaults.defaults'
```

### Unexpected replica count

```bash
# Debug replica calculation step by step
opa eval -d policies/dynamic-defaults.rego \
  --explain full \
  'data.containerization.dynamic_defaults.replicas' \
  --input <(echo '{"environment": "production", "trafficLevel": "high", "criticalityTier": "tier-1"}')
```

## Related Documentation

- [Template Injection Guide](./template-injection-example.md)
- [Policy Authoring Guide](../guides/policy-authoring.md)
- [Sprint 3 Overview](../sprints/sprint-3.md)
