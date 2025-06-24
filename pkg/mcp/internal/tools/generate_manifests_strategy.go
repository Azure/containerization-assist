package tools

import (
	"strings"

	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
)

// generateDeploymentStrategyContext creates AI decision-making context for deployment strategies
func (t *AtomicGenerateManifestsTool) generateDeploymentStrategyContext(result *AtomicGenerateManifestsResult, args AtomicGenerateManifestsArgs, session *sessiontypes.SessionState) {
	context := result.DeploymentStrategyContext

	// Determine recommended strategy based on analysis
	context.RecommendedStrategy = t.determineRecommendedStrategy(args, session, result)

	// Generate strategy options with trade-offs
	context.StrategyOptions = t.generateStrategyOptions(args, session, result)

	// Provide resource sizing recommendations
	context.ResourceSizing = t.generateResourceRecommendations(args, session, result)

	// Assess security posture
	context.SecurityPosture = t.assessSecurityPosture(args, result)

	// Analyze scaling considerations
	context.ScalingConsiderations = t.analyzeScalingConsiderations(args, result)

	// Generate environment-specific profiles
	context.EnvironmentProfiles = t.generateEnvironmentProfiles(args, result)
}

// determineRecommendedStrategy suggests the best deployment strategy
func (t *AtomicGenerateManifestsTool) determineRecommendedStrategy(args AtomicGenerateManifestsArgs, session *sessiontypes.SessionState, result *AtomicGenerateManifestsResult) string {
	// Consider application characteristics
	if args.Replicas > 3 {
		return "rolling"
	}

	// Consider security requirements
	if result.ManifestContext.SecretsDetected > 0 || args.GitOpsReady {
		return "blue-green"
	}

	// Consider service type
	if args.ServiceType == "LoadBalancer" {
		return "canary"
	}

	// Default for simple applications
	return "rolling"
}

// generateStrategyOptions provides deployment strategy options with trade-offs
func (t *AtomicGenerateManifestsTool) generateStrategyOptions(args AtomicGenerateManifestsArgs, session *sessiontypes.SessionState, result *AtomicGenerateManifestsResult) []DeploymentOption {
	options := []DeploymentOption{
		{
			Strategy:     "rolling",
			Description:  "Gradually replace old pods with new ones",
			Pros:         []string{"Zero downtime", "Resource efficient", "Simple to implement"},
			Cons:         []string{"Mixed versions during rollout", "Slower rollback"},
			Complexity:   "simple",
			UseCase:      "Most applications, especially stateless services",
			Requirements: []string{"Health checks configured", "Graceful shutdown"},
			RiskLevel:    "low",
		},
		{
			Strategy:     "blue-green",
			Description:  "Deploy to separate environment, then switch traffic",
			Pros:         []string{"Instant rollback", "Full testing before switch", "No mixed versions"},
			Cons:         []string{"Double resource requirements", "Complex traffic switching"},
			Complexity:   "moderate",
			UseCase:      "Critical applications, when instant rollback is required",
			Requirements: []string{"Load balancer", "Double resources", "Service mesh (optional)"},
			RiskLevel:    "medium",
		},
		{
			Strategy:     "canary",
			Description:  "Gradually shift traffic to new version",
			Pros:         []string{"Risk mitigation", "Real user validation", "Gradual rollout"},
			Cons:         []string{"Complex setup", "Requires advanced monitoring"},
			Complexity:   "complex",
			UseCase:      "High-risk deployments, when gradual validation is needed",
			Requirements: []string{"Service mesh or ingress controller", "Advanced monitoring", "Automated rollback"},
			RiskLevel:    "low",
		},
	}

	// Add recreate strategy for certain scenarios
	if args.Replicas == 1 || strings.Contains(strings.ToLower(args.ImageRef), "database") {
		options = append(options, DeploymentOption{
			Strategy:     "recreate",
			Description:  "Terminate all old pods before creating new ones",
			Pros:         []string{"Simple", "No resource conflicts", "Clean state"},
			Cons:         []string{"Downtime during deployment", "No rollback without redeploy"},
			Complexity:   "simple",
			UseCase:      "Single-instance applications, databases, development environments",
			Requirements: []string{"Tolerance for downtime"},
			RiskLevel:    "high",
		})
	}

	return options
}

// assessSecurityPosture analyzes security aspects of the deployment
func (t *AtomicGenerateManifestsTool) assessSecurityPosture(args AtomicGenerateManifestsArgs, result *AtomicGenerateManifestsResult) SecurityAssessment {
	assessment := SecurityAssessment{
		SecurityControls: []SecurityControl{},
		Vulnerabilities:  []DeploymentSecurityIssue{},
		Compliance:       []ComplianceCheck{},
		Recommendations:  []string{},
	}

	// Assess secrets handling
	if result.ManifestContext.SecretsDetected > 0 {
		if result.ManifestContext.SecretsExternalized > 0 {
			assessment.SecurityControls = append(assessment.SecurityControls, SecurityControl{
				Name:        "Secret Externalization",
				Implemented: true,
				Description: "Secrets are properly externalized from manifests",
				Impact:      "high",
			})
		} else {
			assessment.Vulnerabilities = append(assessment.Vulnerabilities, DeploymentSecurityIssue{
				Category:    "secrets",
				Severity:    "high",
				Description: "Secrets detected but not externalized",
				Remediation: []string{"Use Kubernetes secrets", "Implement sealed-secrets", "Use external secret management"},
			})
		}
	}

	// Assess GitOps readiness
	if args.GitOpsReady {
		assessment.SecurityControls = append(assessment.SecurityControls, SecurityControl{
			Name:        "GitOps Ready",
			Implemented: true,
			Description: "Configuration externalized for GitOps workflow",
			Impact:      "medium",
		})
	}

	// Assess namespace isolation
	if args.Namespace != "default" {
		assessment.SecurityControls = append(assessment.SecurityControls, SecurityControl{
			Name:        "Namespace Isolation",
			Implemented: true,
			Description: "Application deployed to dedicated namespace",
			Impact:      "medium",
		})
	} else {
		assessment.Vulnerabilities = append(assessment.Vulnerabilities, DeploymentSecurityIssue{
			Category:    "rbac",
			Severity:    "medium",
			Description: "Using default namespace reduces isolation",
			Remediation: []string{"Create dedicated namespace", "Implement NetworkPolicies"},
		})
	}

	// Resource limits assessment
	hasResourceLimits := args.CPULimit != "" || args.MemoryLimit != ""
	if hasResourceLimits {
		assessment.SecurityControls = append(assessment.SecurityControls, SecurityControl{
			Name:        "Resource Limits",
			Implemented: true,
			Description: "Resource limits configured to prevent resource exhaustion",
			Impact:      "medium",
		})
	} else {
		assessment.Vulnerabilities = append(assessment.Vulnerabilities, DeploymentSecurityIssue{
			Category:    "resources",
			Severity:    "medium",
			Description: "No resource limits configured",
			Remediation: []string{"Set CPU and memory limits", "Implement ResourceQuotas"},
		})
	}

	// Calculate overall rating
	controls := len(assessment.SecurityControls)
	vulnerabilities := len(assessment.Vulnerabilities)

	if vulnerabilities == 0 && controls >= 3 {
		assessment.OverallRating = "excellent"
	} else if vulnerabilities <= 1 && controls >= 2 {
		assessment.OverallRating = "good"
	} else if vulnerabilities <= 2 {
		assessment.OverallRating = "needs-improvement"
	} else {
		assessment.OverallRating = "poor"
	}

	// Add general recommendations
	assessment.Recommendations = append(assessment.Recommendations,
		"Implement pod security standards",
		"Use network policies for traffic segmentation",
		"Regular security scanning of container images",
		"Implement least privilege RBAC")

	return assessment
}

// analyzeScalingConsiderations provides scaling strategy recommendations
func (t *AtomicGenerateManifestsTool) analyzeScalingConsiderations(args AtomicGenerateManifestsArgs, result *AtomicGenerateManifestsResult) ScalingAnalysis {
	analysis := ScalingAnalysis{
		AutoscalingOptions: []AutoscalingOption{
			{
				Type:        "HPA",
				Description: "Horizontal Pod Autoscaler based on CPU/memory metrics",
				Triggers:    []string{"CPU utilization", "Memory utilization"},
				Pros:        []string{"Built into Kubernetes", "Simple setup", "Handles traffic spikes"},
				Cons:        []string{"Limited to basic metrics", "Slower scaling"},
				Complexity:  "simple",
			},
			{
				Type:        "VPA",
				Description: "Vertical Pod Autoscaler for right-sizing resources",
				Triggers:    []string{"Historical resource usage"},
				Pros:        []string{"Right-sizes resources", "Cost optimization"},
				Cons:        []string{"Requires pod restart", "Less mature"},
				Complexity:  "moderate",
			},
			{
				Type:        "KEDA",
				Description: "Event-driven autoscaling with custom metrics",
				Triggers:    []string{"Queue length", "HTTP requests", "Custom metrics"},
				Pros:        []string{"Event-driven", "Multiple triggers", "Scale to zero"},
				Cons:        []string{"Additional component", "Complex configuration"},
				Complexity:  "complex",
			},
		},
		LoadTesting: LoadTestingGuidance{
			RecommendedApproach: "Gradual load increase with plateau testing",
			TestScenarios:       []string{"Normal load", "Peak load", "Sustained load", "Spike testing"},
			Tools:               []string{"k6", "Artillery", "JMeter", "Gatling"},
			Metrics:             []string{"Response time", "Throughput", "Error rate", "Resource utilization"},
		},
		MonitoringStrategy: MonitoringStrategy{
			KeyMetrics:     []string{"CPU usage", "Memory usage", "Request latency", "Error rate", "Throughput"},
			AlertingRules:  []string{"High CPU/Memory", "Error rate spike", "Response time degradation"},
			DashboardTypes: []string{"Application metrics", "Infrastructure metrics", "Business metrics"},
			LoggingLevel:   "info",
		},
	}

	// Determine recommended pattern based on app characteristics
	if args.Replicas <= 3 {
		analysis.RecommendedPattern = "horizontal"
	} else if result.ManifestContext.TotalResources > 5 {
		analysis.RecommendedPattern = "both"
	} else {
		analysis.RecommendedPattern = "vertical"
	}

	return analysis
}

// generateEnvironmentProfiles creates environment-specific configurations
func (t *AtomicGenerateManifestsTool) generateEnvironmentProfiles(args AtomicGenerateManifestsArgs, result *AtomicGenerateManifestsResult) []EnvironmentProfile {
	profiles := []EnvironmentProfile{
		{
			Environment:     "development",
			Configuration:   map[string]string{"LOG_LEVEL": "debug", "CACHE_TTL": "60s"},
			ResourceProfile: "small",
			SecurityLevel:   "basic",
			Monitoring:      "basic",
			Backup:          "none",
			Compliance:      []string{},
		},
		{
			Environment:     "staging",
			Configuration:   map[string]string{"LOG_LEVEL": "info", "CACHE_TTL": "300s"},
			ResourceProfile: "medium",
			SecurityLevel:   "enhanced",
			Monitoring:      "comprehensive",
			Backup:          "daily",
			Compliance:      []string{"internal-audit"},
		},
		{
			Environment:     "production",
			Configuration:   map[string]string{"LOG_LEVEL": "warn", "CACHE_TTL": "3600s"},
			ResourceProfile: "large",
			SecurityLevel:   "strict",
			Monitoring:      "full-observability",
			Backup:          "continuous",
			Compliance:      []string{"SOC2", "PCI-DSS", "GDPR"},
		},
	}

	return profiles
}
