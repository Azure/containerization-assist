package deploy

import (
	"fmt"

	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
)

// generateResourceRecommendations provides resource sizing guidance
func (t *AtomicGenerateManifestsTool) generateResourceRecommendations(args AtomicGenerateManifestsArgs, session *sessiontypes.SessionState, result *AtomicGenerateManifestsResult) ResourceRecommendation {
	// Determine profile based on replicas and manifest complexity
	profile := "small"
	if args.Replicas > 3 || result.ManifestContext.TotalResources > 5 {
		profile = "medium"
	}
	if args.Replicas > 10 || len(result.SecretsDetected) > 5 {
		profile = "large"
	}

	recommendation := ResourceRecommendation{
		RecommendedProfile: profile,
		CPURecommendation: ResourceSpec{
			Request:      t.getCPURequest(args, profile),
			Limit:        t.getCPULimit(args, profile),
			Rationale:    t.getCPURationale(profile),
			Alternatives: t.getCPUAlternatives(profile),
		},
		MemoryRecommendation: ResourceSpec{
			Request:      t.getMemoryRequest(args, profile),
			Limit:        t.getMemoryLimit(args, profile),
			Rationale:    t.getMemoryRationale(profile),
			Alternatives: t.getMemoryAlternatives(profile),
		},
		ScalingMetrics:   t.generateScalingMetrics(args, profile),
		CostImplications: t.generateCostImplications(profile, args.Replicas),
		Rationale:        t.getResourceRationale(args, profile),
	}

	return recommendation
}

// getCPURequest returns CPU request recommendation
func (t *AtomicGenerateManifestsTool) getCPURequest(args AtomicGenerateManifestsArgs, profile string) string {
	if args.CPURequest != "" {
		return args.CPURequest
	}

	switch profile {
	case "small":
		return "100m"
	case "medium":
		return "250m"
	case "large":
		return "500m"
	default:
		return "100m"
	}
}

// getCPULimit returns CPU limit recommendation
func (t *AtomicGenerateManifestsTool) getCPULimit(args AtomicGenerateManifestsArgs, profile string) string {
	if args.CPULimit != "" {
		return args.CPULimit
	}

	switch profile {
	case "small":
		return "500m"
	case "medium":
		return "1000m"
	case "large":
		return "2000m"
	default:
		return "500m"
	}
}

// getCPURationale returns CPU allocation rationale
func (t *AtomicGenerateManifestsTool) getCPURationale(profile string) string {
	switch profile {
	case "small":
		return "Small applications typically need minimal CPU, with burst capability for handling occasional spikes"
	case "medium":
		return "Medium applications require moderate CPU for steady workloads with headroom for peak traffic"
	case "large":
		return "Large applications need significant CPU resources for high-throughput or compute-intensive operations"
	default:
		return "Conservative CPU allocation suitable for most applications"
	}
}

// getCPUAlternatives returns alternative CPU configurations
func (t *AtomicGenerateManifestsTool) getCPUAlternatives(profile string) []string {
	switch profile {
	case "small":
		return []string{"50m (minimal)", "200m (comfortable)", "300m (generous)"}
	case "medium":
		return []string{"200m (conservative)", "500m (balanced)", "750m (performance)"}
	case "large":
		return []string{"400m (efficient)", "1000m (standard)", "1500m (high-performance)"}
	default:
		return []string{"100m", "250m", "500m"}
	}
}

// getMemoryRequest returns memory request recommendation
func (t *AtomicGenerateManifestsTool) getMemoryRequest(args AtomicGenerateManifestsArgs, profile string) string {
	if args.MemoryRequest != "" {
		return args.MemoryRequest
	}

	switch profile {
	case "small":
		return "128Mi"
	case "medium":
		return "512Mi"
	case "large":
		return "1Gi"
	default:
		return "128Mi"
	}
}

// getMemoryLimit returns memory limit recommendation
func (t *AtomicGenerateManifestsTool) getMemoryLimit(args AtomicGenerateManifestsArgs, profile string) string {
	if args.MemoryLimit != "" {
		return args.MemoryLimit
	}

	switch profile {
	case "small":
		return "512Mi"
	case "medium":
		return "1Gi"
	case "large":
		return "2Gi"
	default:
		return "512Mi"
	}
}

// getMemoryRationale returns memory allocation rationale
func (t *AtomicGenerateManifestsTool) getMemoryRationale(profile string) string {
	switch profile {
	case "small":
		return "Small applications typically have modest memory requirements with buffer for temporary spikes"
	case "medium":
		return "Medium applications need sufficient memory for caching and processing moderate workloads"
	case "large":
		return "Large applications require substantial memory for in-memory operations, caching, and high concurrency"
	default:
		return "Conservative memory allocation suitable for most applications"
	}
}

// getMemoryAlternatives returns alternative memory configurations
func (t *AtomicGenerateManifestsTool) getMemoryAlternatives(profile string) []string {
	switch profile {
	case "small":
		return []string{"64Mi (minimal)", "256Mi (comfortable)", "384Mi (generous)"}
	case "medium":
		return []string{"384Mi (conservative)", "768Mi (balanced)", "1Gi (performance)"}
	case "large":
		return []string{"768Mi (efficient)", "1.5Gi (standard)", "2Gi (high-performance)"}
	default:
		return []string{"128Mi", "256Mi", "512Mi"}
	}
}

// generateScalingMetrics creates scaling metric recommendations
func (t *AtomicGenerateManifestsTool) generateScalingMetrics(args AtomicGenerateManifestsArgs, profile string) []ScalingMetric {
	metrics := []ScalingMetric{
		{
			Type:        "cpu",
			Threshold:   "70%",
			MinReplicas: 1,
			MaxReplicas: 10,
			Behavior:    "balanced",
		},
		{
			Type:        "memory",
			Threshold:   "80%",
			MinReplicas: 1,
			MaxReplicas: 10,
			Behavior:    "conservative",
		},
	}

	// Adjust based on profile
	switch profile {
	case "small":
		metrics[0].MaxReplicas = 5
		metrics[1].MaxReplicas = 5
	case "large":
		metrics[0].MinReplicas = 2
		metrics[1].MinReplicas = 2
		metrics[0].MaxReplicas = 20
		metrics[1].MaxReplicas = 20
		metrics[0].Behavior = "aggressive"
	}

	// Add custom metrics for specific scenarios
	if args.ServiceType == "LoadBalancer" {
		metrics = append(metrics, ScalingMetric{
			Type:        "custom",
			Threshold:   "100 requests/second",
			MinReplicas: 2,
			MaxReplicas: 15,
			Behavior:    "aggressive",
		})
	}

	return metrics
}

// generateCostImplications creates cost analysis
func (t *AtomicGenerateManifestsTool) generateCostImplications(profile string, replicas int) []string {
	baseImplications := []string{
		fmt.Sprintf("Base resource allocation for %d replica(s)", replicas),
	}

	switch profile {
	case "small":
		baseImplications = append(baseImplications,
			"Low resource footprint suitable for cost-conscious deployments",
			"Minimal compute costs, ideal for development or low-traffic services",
			"Consider spot instances or preemptible VMs for additional savings")
	case "medium":
		baseImplications = append(baseImplications,
			"Moderate resource allocation balancing cost and performance",
			"Standard compute tier recommended for predictable performance",
			"Consider reserved instances for production workloads")
	case "large":
		baseImplications = append(baseImplications,
			"Higher resource allocation for performance-critical applications",
			"Premium compute tier may be beneficial for consistent performance",
			"Consider dedicated node pools for resource isolation")
	}

	if replicas > 5 {
		baseImplications = append(baseImplications,
			"Multiple replicas increase total resource consumption",
			"Consider implementing pod disruption budgets for cost-effective updates")
	}

	return baseImplications
}

// getResourceRationale provides overall resource allocation reasoning
func (t *AtomicGenerateManifestsTool) getResourceRationale(args AtomicGenerateManifestsArgs, profile string) string {
	rationale := fmt.Sprintf("Based on profile '%s' ", profile)

	if args.Replicas > 10 {
		rationale += "and high replica count, resources are sized for distributed load. "
	} else if args.Replicas > 3 {
		rationale += "and moderate replica count, resources balance efficiency and performance. "
	} else {
		rationale += "and low replica count, resources are optimized for cost efficiency. "
	}

	if args.GitOpsReady {
		rationale += "GitOps configuration suggests production use case requiring stable resource allocation. "
	}

	if len(args.Environment) > 10 {
		rationale += "Large number of environment variables may indicate complex application requiring additional memory. "
	}

	return rationale
}
