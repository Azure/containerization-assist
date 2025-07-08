package deploy

import (
	"testing"
)

func TestDeploymentRequest_Validate(t *testing.T) {
	validRequest := &DeploymentRequest{
		SessionID:   "test-session",
		Name:        "test-app",
		Namespace:   "default",
		Environment: EnvironmentDevelopment,
		Strategy:    StrategyRolling,
		Image:       "nginx:latest",
		Replicas:    1,
		Resources: ResourceRequirements{
			CPU:    ResourceSpec{Request: "100m", Limit: "500m"},
			Memory: ResourceSpec{Request: "128Mi", Limit: "256Mi"},
		},
	}

	errors := validRequest.Validate()
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errors), errors)
	}

	// Test invalid request
	invalidRequest := &DeploymentRequest{
		SessionID:   "",
		Name:        "Invalid_Name!",
		Namespace:   "Invalid_Namespace!",
		Environment: "invalid",
		Strategy:    "invalid",
		Image:       "",
		Replicas:    -1,
		Resources: ResourceRequirements{
			CPU:    ResourceSpec{Request: "invalid", Limit: "invalid"},
			Memory: ResourceSpec{Request: "invalid", Limit: "invalid"},
		},
	}

	errors = invalidRequest.Validate()
	if len(errors) == 0 {
		t.Error("expected validation errors for invalid request")
	}
}

func TestDeploymentResult_IsHealthy(t *testing.T) {
	healthyResult := &DeploymentResult{
		Status: StatusRunning,
		Metadata: DeploymentMetadata{
			ScalingInfo: ScalingInfo{
				DesiredReplicas: 3,
				ReadyReplicas:   3,
			},
		},
	}

	if !healthyResult.IsHealthy() {
		t.Error("expected deployment to be healthy")
	}

	unhealthyResult := &DeploymentResult{
		Status: StatusRunning,
		Metadata: DeploymentMetadata{
			ScalingInfo: ScalingInfo{
				DesiredReplicas: 3,
				ReadyReplicas:   1,
			},
		},
	}

	if unhealthyResult.IsHealthy() {
		t.Error("expected deployment to be unhealthy")
	}
}

func TestSelectOptimalStrategy(t *testing.T) {
	// Production should use rolling
	prodReq := &DeploymentRequest{
		Environment: EnvironmentProduction,
	}
	if SelectOptimalStrategy(prodReq) != StrategyRolling {
		t.Error("expected rolling strategy for production")
	}

	// Development should use recreate
	devReq := &DeploymentRequest{
		Environment: EnvironmentDevelopment,
	}
	if SelectOptimalStrategy(devReq) != StrategyRecreate {
		t.Error("expected recreate strategy for development")
	}

	// Explicit strategy should be used
	explicitReq := &DeploymentRequest{
		Environment: EnvironmentProduction,
		Strategy:    StrategyBlueGreen,
	}
	if SelectOptimalStrategy(explicitReq) != StrategyBlueGreen {
		t.Error("expected explicit strategy to be used")
	}
}

func TestGetRecommendedReplicas(t *testing.T) {
	// Explicit replicas should be used
	explicitReq := &DeploymentRequest{
		Replicas: 5,
	}
	if explicitReq.GetRecommendedReplicas() != 5 {
		t.Error("expected explicit replicas to be used")
	}

	// Production should get 3 replicas
	prodReq := &DeploymentRequest{
		Environment: EnvironmentProduction,
	}
	if prodReq.GetRecommendedReplicas() != 3 {
		t.Error("expected 3 replicas for production")
	}

	// Development should get 1 replica
	devReq := &DeploymentRequest{
		Environment: EnvironmentDevelopment,
	}
	if devReq.GetRecommendedReplicas() != 1 {
		t.Error("expected 1 replica for development")
	}
}

func TestEstimateDeploymentTime(t *testing.T) {
	simpleReq := &DeploymentRequest{
		Environment: EnvironmentDevelopment,
		Strategy:    StrategyRecreate,
		Replicas:    1,
	}

	duration := EstimateDeploymentTime(simpleReq)
	if duration <= 0 {
		t.Error("expected positive deployment time")
	}

	// Production should take longer
	prodReq := &DeploymentRequest{
		Environment: EnvironmentProduction,
		Strategy:    StrategyRolling,
		Replicas:    3,
	}

	prodDuration := EstimateDeploymentTime(prodReq)
	if prodDuration <= duration {
		t.Error("expected production deployment to take longer")
	}
}

func TestShouldUseHorizontalPodAutoscaler(t *testing.T) {
	// Production with multiple replicas and resources should use HPA
	prodReq := &DeploymentRequest{
		Environment: EnvironmentProduction,
		Replicas:    3,
		Resources: ResourceRequirements{
			CPU: ResourceSpec{Request: "100m"},
		},
	}
	if !ShouldUseHorizontalPodAutoscaler(prodReq) {
		t.Error("expected to recommend HPA for production deployment")
	}

	// Development should not use HPA
	devReq := &DeploymentRequest{
		Environment: EnvironmentDevelopment,
		Replicas:    1,
	}
	if ShouldUseHorizontalPodAutoscaler(devReq) {
		t.Error("expected not to recommend HPA for development")
	}
}

func TestGetSecurityRecommendations(t *testing.T) {
	insecureReq := &DeploymentRequest{
		Configuration: DeploymentConfiguration{
			SecurityContext: SecurityContext{
				RunAsNonRoot:      nil,
				ReadOnlyRootFS:    nil,
				AllowPrivilegeEsc: nil,
			},
		},
		Resources: ResourceRequirements{},
	}

	recommendations := GetSecurityRecommendations(insecureReq)
	if len(recommendations) == 0 {
		t.Error("expected security recommendations for insecure deployment")
	}

	// Check for specific recommendation types
	types := make(map[string]bool)
	for _, rec := range recommendations {
		types[rec.Type] = true
	}

	if !types["security_context"] {
		t.Error("expected security_context recommendations")
	}
}