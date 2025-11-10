# Generation Configuration Policy
#
# This policy demonstrates how to define generation configuration
# that tools can query BEFORE building plans. This enables
# policy-driven defaults for Dockerfile and Kubernetes manifest generation.
#
# Package Structure:
# - containerization.generation_config.dockerfile - Dockerfile generation config
# - containerization.generation_config.kubernetes - Kubernetes manifest generation config
# - containerization.generation_config.config - Complete generation config
#
# Tools query these packages with input containing:
# - language: detected language (node, python, go, java, etc.)
# - framework: detected framework (express, fastapi, gin, spring, etc.)
# - environment: target environment (dev, staging, prod)
# - appName: application name
# - context: additional context

package containerization.generation_config

import rego.v1

# ===== DOCKERFILE GENERATION CONFIG =====

# Dockerfile configuration based on environment
dockerfile := {
    "buildStrategy": build_strategy,
    "baseImageCategory": base_image_category,
    "optimizationPriority": optimization_priority,
    "securityFeatures": security_features,
    "buildFeatures": build_features,
}

# Build strategy selection
build_strategy := "multi-stage" if {
    input.environment == "prod"
}

build_strategy := "multi-stage" if {
    input.environment == "staging"
}

build_strategy := "single-stage" if {
    input.environment == "dev"
}

# Base image category selection
base_image_category := "distroless" if {
    input.environment == "prod"
}

base_image_category := "alpine" if {
    input.environment == "staging"
}

base_image_category := "official" if {
    input.environment == "dev"
}

# Optimization priority based on environment
optimization_priority := "security" if {
    input.environment == "prod"
}

optimization_priority := "balanced" if {
    input.environment == "staging"
}

optimization_priority := "speed" if {
    input.environment == "dev"
}

# Security features - stricter in production
security_features := {
    "nonRootUser": true,
    "readOnlyRootFS": true,
    "noNewPrivileges": true,
    "dropCapabilities": true,
} if {
    input.environment == "prod"
}

security_features := {
    "nonRootUser": true,
    "readOnlyRootFS": false,
    "noNewPrivileges": true,
    "dropCapabilities": false,
} if {
    input.environment == "staging"
}

security_features := {
    "nonRootUser": false,
    "readOnlyRootFS": false,
    "noNewPrivileges": false,
    "dropCapabilities": false,
} if {
    input.environment == "dev"
}

# Build features
build_features := {
    "buildCache": true,
    "layerOptimization": true,
    "healthcheck": true,
}

# ===== KUBERNETES MANIFEST GENERATION CONFIG =====

# Kubernetes configuration
kubernetes := {
    "resourceDefaults": resource_defaults,
    "orgStandards": org_standards,
    "features": features,
    "replicas": replicas,
    "deploymentStrategy": "RollingUpdate",
}

# Resource defaults based on language and environment
resource_defaults := {
    "cpuRequest": "100m",
    "cpuLimit": "500m",
    "memoryRequest": "128Mi",
    "memoryLimit": "256Mi",
} if {
    input.language == "node"
    input.environment == "dev"
}

resource_defaults := {
    "cpuRequest": "200m",
    "cpuLimit": "1",
    "memoryRequest": "256Mi",
    "memoryLimit": "512Mi",
} if {
    input.language == "node"
    input.environment == "staging"
}

resource_defaults := {
    "cpuRequest": "500m",
    "cpuLimit": "2",
    "memoryRequest": "512Mi",
    "memoryLimit": "1Gi",
} if {
    input.language == "node"
    input.environment == "prod"
}

# Python resource defaults
resource_defaults := {
    "cpuRequest": "100m",
    "cpuLimit": "500m",
    "memoryRequest": "256Mi",
    "memoryLimit": "512Mi",
} if {
    input.language == "python"
    input.environment == "dev"
}

resource_defaults := {
    "cpuRequest": "200m",
    "cpuLimit": "1",
    "memoryRequest": "512Mi",
    "memoryLimit": "1Gi",
} if {
    input.language == "python"
    input.environment == "staging"
}

resource_defaults := {
    "cpuRequest": "500m",
    "cpuLimit": "2",
    "memoryRequest": "1Gi",
    "memoryLimit": "2Gi",
} if {
    input.language == "python"
    input.environment == "prod"
}

# Java resource defaults (higher memory requirements)
resource_defaults := {
    "cpuRequest": "200m",
    "cpuLimit": "1",
    "memoryRequest": "512Mi",
    "memoryLimit": "1Gi",
} if {
    input.language == "java"
    input.environment == "dev"
}

resource_defaults := {
    "cpuRequest": "500m",
    "cpuLimit": "2",
    "memoryRequest": "1Gi",
    "memoryLimit": "2Gi",
} if {
    input.language == "java"
    input.environment == "staging"
}

resource_defaults := {
    "cpuRequest": "1",
    "cpuLimit": "4",
    "memoryRequest": "2Gi",
    "memoryLimit": "4Gi",
} if {
    input.language == "java"
    input.environment == "prod"
}

# Go resource defaults (lower memory requirements)
resource_defaults := {
    "cpuRequest": "50m",
    "cpuLimit": "250m",
    "memoryRequest": "64Mi",
    "memoryLimit": "128Mi",
} if {
    input.language == "go"
    input.environment == "dev"
}

resource_defaults := {
    "cpuRequest": "100m",
    "cpuLimit": "500m",
    "memoryRequest": "128Mi",
    "memoryLimit": "256Mi",
} if {
    input.language == "go"
    input.environment == "staging"
}

resource_defaults := {
    "cpuRequest": "250m",
    "cpuLimit": "1",
    "memoryRequest": "256Mi",
    "memoryLimit": "512Mi",
} if {
    input.language == "go"
    input.environment == "prod"
}

# Default fallback for unknown languages
resource_defaults := {
    "cpuRequest": "100m",
    "cpuLimit": "500m",
    "memoryRequest": "128Mi",
    "memoryLimit": "256Mi",
} if {
    # Only use default if no specific language/environment combo matches
    not input.language
}

# Final fallback if nothing else matches
resource_defaults := {
    "cpuRequest": "100m",
    "cpuLimit": "500m",
    "memoryRequest": "128Mi",
    "memoryLimit": "256Mi",
} if {
    # Specific fallback when language is provided but doesn't match known languages
    input.language
    not input.language in ["node", "python", "java", "go"]
}

# Organizational standards
org_standards := {
    "requiredLabels": required_labels,
    "namespace": namespace,
    "allowedRegistries": allowed_registries,
    "serviceAccount": "default",
    "imagePullPolicy": image_pull_policy,
}

# Required labels for all resources
required_labels := {
    "app.kubernetes.io/managed-by": "containerization-assist",
    "app.kubernetes.io/environment": input.environment,
    "app.kubernetes.io/name": input.appName,
} if {
    input.appName
}

required_labels := {
    "app.kubernetes.io/managed-by": "containerization-assist",
    "app.kubernetes.io/environment": input.environment,
} if {
    not input.appName
}

# Namespace based on environment
namespace := sprintf("%s-prod", [input.appName]) if {
    input.environment == "prod"
    input.appName
}

namespace := sprintf("%s-staging", [input.appName]) if {
    input.environment == "staging"
    input.appName
}

namespace := sprintf("%s-dev", [input.appName]) if {
    input.environment == "dev"
    input.appName
}

namespace := "default" if {
    not input.appName
}

# Allowed registries by environment
allowed_registries := [
    "docker.io",
    "gcr.io",
    "ghcr.io",
] if {
    input.environment == "prod"
}

allowed_registries := [
    "docker.io",
    "gcr.io",
    "ghcr.io",
] if {
    input.environment == "staging"
}

allowed_registries := [
    "docker.io",
] if {
    input.environment == "dev"
}

# Image pull policy
image_pull_policy := "Always" if {
    input.environment == "prod"
}

image_pull_policy := "IfNotPresent" if {
    input.environment != "prod"
}

# Feature toggles based on environment
features := {
    "healthChecks": true,
    "autoscaling": true,
    "resourceQuotas": true,
    "networkPolicies": true,
    "podSecurityPolicies": true,
    "ingress": true,
} if {
    input.environment == "prod"
}

features := {
    "healthChecks": true,
    "autoscaling": false,
    "resourceQuotas": false,
    "networkPolicies": false,
    "podSecurityPolicies": false,
    "ingress": true,
} if {
    input.environment == "staging"
}

features := {
    "healthChecks": false,
    "autoscaling": false,
    "resourceQuotas": false,
    "networkPolicies": false,
    "podSecurityPolicies": false,
    "ingress": false,
} if {
    input.environment == "dev"
}

# Replicas based on environment
replicas := 3 if {
    input.environment == "prod"
}

replicas := 2 if {
    input.environment == "staging"
}

replicas := 1 if {
    input.environment == "dev"
}

# ===== COMPLETE CONFIGURATION =====

# Complete configuration (for tools that want both)
config := {
    "dockerfile": dockerfile,
    "kubernetes": kubernetes,
}
