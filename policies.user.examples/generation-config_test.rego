# Tests for Generation Configuration Policy

package containerization.generation_config

import rego.v1

# ===== DOCKERFILE CONFIG TESTS =====

test_dockerfile_prod_multi_stage if {
    dockerfile.buildStrategy == "multi-stage" with input as {"environment": "prod"}
}

test_dockerfile_dev_single_stage if {
    dockerfile.buildStrategy == "single-stage" with input as {"environment": "dev"}
}

test_dockerfile_prod_distroless if {
    dockerfile.baseImageCategory == "distroless" with input as {"environment": "prod"}
}

test_dockerfile_dev_official if {
    dockerfile.baseImageCategory == "official" with input as {"environment": "dev"}
}

test_dockerfile_prod_security_priority if {
    dockerfile.optimizationPriority == "security" with input as {"environment": "prod"}
}

test_dockerfile_dev_speed_priority if {
    dockerfile.optimizationPriority == "speed" with input as {"environment": "dev"}
}

test_dockerfile_prod_security_features if {
    config := dockerfile with input as {"environment": "prod"}
    config.securityFeatures.nonRootUser == true
    config.securityFeatures.readOnlyRootFS == true
    config.securityFeatures.noNewPrivileges == true
    config.securityFeatures.dropCapabilities == true
}

test_dockerfile_dev_relaxed_security if {
    config := dockerfile with input as {"environment": "dev"}
    config.securityFeatures.nonRootUser == false
    config.securityFeatures.readOnlyRootFS == false
}

# ===== KUBERNETES CONFIG TESTS =====

# Node.js resource tests
test_k8s_node_dev_resources if {
    config := kubernetes with input as {"language": "node", "environment": "dev"}
    config.resourceDefaults.cpuRequest == "100m"
    config.resourceDefaults.cpuLimit == "500m"
    config.resourceDefaults.memoryRequest == "128Mi"
    config.resourceDefaults.memoryLimit == "256Mi"
}

test_k8s_node_prod_resources if {
    config := kubernetes with input as {"language": "node", "environment": "prod"}
    config.resourceDefaults.cpuRequest == "500m"
    config.resourceDefaults.cpuLimit == "2"
    config.resourceDefaults.memoryRequest == "512Mi"
    config.resourceDefaults.memoryLimit == "1Gi"
}

# Python resource tests
test_k8s_python_prod_resources if {
    config := kubernetes with input as {"language": "python", "environment": "prod"}
    config.resourceDefaults.cpuRequest == "500m"
    config.resourceDefaults.memoryLimit == "2Gi"
}

# Java resource tests (higher memory)
test_k8s_java_prod_resources if {
    config := kubernetes with input as {"language": "java", "environment": "prod"}
    config.resourceDefaults.cpuRequest == "1"
    config.resourceDefaults.memoryRequest == "2Gi"
    config.resourceDefaults.memoryLimit == "4Gi"
}

# Go resource tests (lower memory)
test_k8s_go_prod_resources if {
    config := kubernetes with input as {"language": "go", "environment": "prod"}
    config.resourceDefaults.cpuRequest == "250m"
    config.resourceDefaults.memoryRequest == "256Mi"
    config.resourceDefaults.memoryLimit == "512Mi"
}

# Namespace tests
test_k8s_namespace_prod if {
    config := kubernetes with input as {"environment": "prod", "appName": "myapp"}
    config.orgStandards.namespace == "myapp-prod"
}

test_k8s_namespace_dev if {
    config := kubernetes with input as {"environment": "dev", "appName": "myapp"}
    config.orgStandards.namespace == "myapp-dev"
}

# Required labels tests
test_k8s_required_labels if {
    config := kubernetes with input as {"environment": "prod", "appName": "myapp"}
    config.orgStandards.requiredLabels["app.kubernetes.io/managed-by"] == "containerization-assist"
    config.orgStandards.requiredLabels["app.kubernetes.io/environment"] == "prod"
    config.orgStandards.requiredLabels["app.kubernetes.io/name"] == "myapp"
}

# Image pull policy tests
test_k8s_image_pull_policy_prod if {
    config := kubernetes with input as {"environment": "prod"}
    config.orgStandards.imagePullPolicy == "Always"
}

test_k8s_image_pull_policy_dev if {
    config := kubernetes with input as {"environment": "dev"}
    config.orgStandards.imagePullPolicy == "IfNotPresent"
}

# Feature toggle tests
test_k8s_features_prod if {
    config := kubernetes with input as {"environment": "prod"}
    config.features.healthChecks == true
    config.features.autoscaling == true
    config.features.networkPolicies == true
}

test_k8s_features_dev if {
    config := kubernetes with input as {"environment": "dev"}
    config.features.healthChecks == false
    config.features.autoscaling == false
    config.features.ingress == false
}

# Replica tests
test_k8s_replicas_prod if {
    config := kubernetes with input as {"environment": "prod"}
    config.replicas == 3
}

test_k8s_replicas_dev if {
    config := kubernetes with input as {"environment": "dev"}
    config.replicas == 1
}

# ===== COMPLETE CONFIG TESTS =====

test_complete_config_structure if {
    result := config with input as {"language": "node", "environment": "prod", "appName": "myapp"}
    result.dockerfile != null
    result.kubernetes != null
}

test_complete_config_prod_node if {
    result := config with input as {"language": "node", "environment": "prod", "appName": "myapp"}
    result.dockerfile.buildStrategy == "multi-stage"
    result.kubernetes.replicas == 3
    result.kubernetes.resourceDefaults.cpuRequest == "500m"
}
