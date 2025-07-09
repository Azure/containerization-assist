// Package deploy contains business rules for container deployment operations
package deploy

import (
	"fmt"
	"regexp"
	"time"
)

// ValidationError represents a deployment validation error
type DomainValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e DomainValidationError) Error() string {
	return fmt.Sprintf("deployment validation error: %s - %s", e.Field, e.Message)
}

// Validate performs domain-level validation on a deployment request
func (dr *DeploymentRequest) Validate() []DomainValidationError {
	var errors []DomainValidationError

	// Session ID is required
	if dr.SessionID == "" {
		errors = append(errors, DomainValidationError{
			Field:   "session_id",
			Message: "session ID is required",
			Code:    "MISSING_SESSION_ID",
		})
	}

	// Name is required and must be valid
	if dr.Name == "" {
		errors = append(errors, DomainValidationError{
			Field:   "name",
			Message: "deployment name is required",
			Code:    "MISSING_NAME",
		})
	} else if !isValidKubernetesName(dr.Name) {
		errors = append(errors, DomainValidationError{
			Field:   "name",
			Message: "deployment name must be a valid Kubernetes name",
			Code:    "INVALID_NAME",
		})
	}

	// Namespace validation
	if dr.Namespace != "" && !isValidKubernetesName(dr.Namespace) {
		errors = append(errors, DomainValidationError{
			Field:   "namespace",
			Message: "namespace must be a valid Kubernetes name",
			Code:    "INVALID_NAMESPACE",
		})
	}

	// Image is required
	if dr.Image == "" {
		errors = append(errors, DomainValidationError{
			Field:   "image",
			Message: "container image is required",
			Code:    "MISSING_IMAGE",
		})
	}

	// Replicas must be positive
	if dr.Replicas < 0 {
		errors = append(errors, DomainValidationError{
			Field:   "replicas",
			Message: "replicas must be non-negative",
			Code:    "INVALID_REPLICAS",
		})
	}

	// Validate environment
	if !isValidEnvironment(dr.Environment) {
		errors = append(errors, DomainValidationError{
			Field:   "environment",
			Message: "invalid environment",
			Code:    "INVALID_ENVIRONMENT",
		})
	}

	// Validate strategy
	if !isValidDeploymentStrategy(dr.Strategy) {
		errors = append(errors, DomainValidationError{
			Field:   "strategy",
			Message: "invalid deployment strategy",
			Code:    "INVALID_STRATEGY",
		})
	}

	// Validate resource requirements
	resourceErrors := validateResourceRequirements(dr.Resources)
	errors = append(errors, resourceErrors...)

	// Validate health checks
	healthErrors := validateHealthChecks(dr.Configuration.HealthChecks)
	errors = append(errors, healthErrors...)

	// Validate ports
	portErrors := validatePorts(dr.Configuration.Ports)
	errors = append(errors, portErrors...)

	return errors
}

// Business Rules for Deployment Operations

// IsCompleted returns true if the deployment has completed
func (dr *DeploymentResult) IsCompleted() bool {
	return dr.Status == StatusCompleted ||
		dr.Status == StatusFailed ||
		dr.Status == StatusRolledBack
}

// IsHealthy returns true if the deployment is running and healthy
func (dr *DeploymentResult) IsHealthy() bool {
	return dr.Status == StatusRunning &&
		dr.Metadata.ScalingInfo.ReadyReplicas > 0 &&
		dr.Metadata.ScalingInfo.ReadyReplicas == dr.Metadata.ScalingInfo.DesiredReplicas
}

// NeedsScaling returns true if the deployment needs scaling
func (dr *DeploymentResult) NeedsScaling() bool {
	scaling := dr.Metadata.ScalingInfo
	return scaling.ReadyReplicas != scaling.DesiredReplicas
}

// HasSecurityIssues returns true if deployment has security issues above threshold
func (dr *DeploymentResult) HasSecurityIssues(maxSeverity SeverityLevel) bool {
	if dr.Metadata.SecurityScan == nil {
		return false
	}

	severityOrder := map[SeverityLevel]int{
		SeverityCritical: 5,
		SeverityHigh:     4,
		SeverityMedium:   3,
		SeverityLow:      2,
		SeverityInfo:     1,
	}

	threshold := severityOrder[maxSeverity]
	for _, issue := range dr.Metadata.SecurityScan.Issues {
		if severityOrder[issue.Severity] >= threshold {
			return true
		}
	}
	return false
}

// GetCriticalSecurityIssues returns security issues with critical severity
func (dr *DeploymentResult) GetCriticalSecurityIssues() []SecurityIssue {
	if dr.Metadata.SecurityScan == nil {
		return nil
	}

	var critical []SecurityIssue
	for _, issue := range dr.Metadata.SecurityScan.Issues {
		if issue.Severity == SeverityCritical {
			critical = append(critical, issue)
		}
	}
	return critical
}

// ShouldRollback determines if deployment should be rolled back based on health
func (dr *DeploymentResult) ShouldRollback() bool {
	// Failed deployments should be rolled back
	if dr.Status == StatusFailed {
		return true
	}

	// Deployments with critical security issues should be rolled back
	if len(dr.GetCriticalSecurityIssues()) > 0 {
		return true
	}

	// Deployments that fail compliance should be rolled back
	if dr.Metadata.SecurityScan != nil && !dr.Metadata.SecurityScan.Compliance.Passed {
		for _, check := range dr.Metadata.SecurityScan.Compliance.Checks {
			if check.Required && !check.Passed {
				return true
			}
		}
	}

	// Deployments with no ready replicas after reasonable time should be rolled back
	if dr.Status == StatusRunning && dr.Metadata.ScalingInfo.ReadyReplicas == 0 &&
		time.Since(dr.CreatedAt) > 10*time.Minute {
		return true
	}

	return false
}

// CanScale returns true if the deployment can be scaled
func (dr *DeploymentResult) CanScale() bool {
	return dr.Status == StatusRunning || dr.Status == StatusCompleted
}

// GetRecommendedReplicas calculates recommended replica count based on environment
func (dr *DeploymentRequest) GetRecommendedReplicas() int {
	if dr.Replicas > 0 {
		return dr.Replicas
	}

	// Default recommendations based on environment
	switch dr.Environment {
	case EnvironmentProduction:
		return 3 // High availability
	case EnvironmentStaging:
		return 2 // Some redundancy
	case EnvironmentDevelopment, EnvironmentTest:
		return 1 // Minimal resources
	default:
		return 1
	}
}

// Business Rules for Strategy Selection

// SelectOptimalStrategy determines the best deployment strategy
func SelectOptimalStrategy(req *DeploymentRequest) DeploymentStrategy {
	// If explicitly specified, use that strategy
	if req.Strategy != "" {
		return req.Strategy
	}

	// For production, prefer rolling updates
	if req.Environment == EnvironmentProduction {
		return StrategyRolling
	}

	// For development/test, recreate is often simpler
	if req.Environment == EnvironmentDevelopment || req.Environment == EnvironmentTest {
		return StrategyRecreate
	}

	// Default to rolling for staging
	return StrategyRolling
}

// EstimateDeploymentTime estimates deployment duration
func EstimateDeploymentTime(req *DeploymentRequest) time.Duration {
	baseTime := 2 * time.Minute // Base deployment time

	// Adjust for number of replicas
	baseTime += time.Duration(req.GetRecommendedReplicas()) * 30 * time.Second

	// Adjust for strategy
	switch req.Strategy {
	case StrategyBlueGreen:
		baseTime *= 2 // Blue-green takes longer
	case StrategyCanary:
		baseTime = time.Duration(float64(baseTime) * 1.5) // Canary has gradual rollout
	case StrategyRecreate:
		baseTime = time.Duration(float64(baseTime) * 0.8) // Recreate is faster
	}

	// Adjust for environment (production has more checks)
	if req.Environment == EnvironmentProduction {
		baseTime = time.Duration(float64(baseTime) * 1.3)
	}

	return baseTime
}

// Business Rules for Resource Allocation

// CalculateResourceRequirements calculates resource requirements based on environment
func CalculateResourceRequirements(req *DeploymentRequest) ResourceRequirements {
	// If explicitly specified, use those
	if req.Resources.CPU.Request != "" || req.Resources.Memory.Request != "" {
		return req.Resources
	}

	// Environment-based defaults
	switch req.Environment {
	case EnvironmentProduction:
		return ResourceRequirements{
			CPU:    ResourceSpec{Request: "500m", Limit: "1000m"},
			Memory: ResourceSpec{Request: "512Mi", Limit: "1Gi"},
		}
	case EnvironmentStaging:
		return ResourceRequirements{
			CPU:    ResourceSpec{Request: "250m", Limit: "500m"},
			Memory: ResourceSpec{Request: "256Mi", Limit: "512Mi"},
		}
	case EnvironmentDevelopment, EnvironmentTest:
		return ResourceRequirements{
			CPU:    ResourceSpec{Request: "100m", Limit: "250m"},
			Memory: ResourceSpec{Request: "128Mi", Limit: "256Mi"},
		}
	default:
		return ResourceRequirements{
			CPU:    ResourceSpec{Request: "100m", Limit: "500m"},
			Memory: ResourceSpec{Request: "128Mi", Limit: "512Mi"},
		}
	}
}

// ShouldUseHorizontalPodAutoscaler determines if HPA should be enabled
func ShouldUseHorizontalPodAutoscaler(req *DeploymentRequest) bool {
	// Only recommend for production with multiple replicas
	if req.Environment != EnvironmentProduction {
		return false
	}

	if req.GetRecommendedReplicas() < 2 {
		return false
	}

	// If resource requests are defined, HPA can work
	return req.Resources.CPU.Request != "" || req.Resources.Memory.Request != ""
}

// Validation helper functions

// isValidKubernetesName validates Kubernetes resource names
func isValidKubernetesName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	// Kubernetes name pattern: lowercase alphanumeric, dashes, starts/ends with alphanumeric
	pattern := `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	matched, err := regexp.MatchString(pattern, name)
	return err == nil && matched
}

// isValidEnvironment validates environment values
func isValidEnvironment(env Environment) bool {
	validEnvs := []Environment{
		EnvironmentDevelopment,
		EnvironmentStaging,
		EnvironmentProduction,
		EnvironmentTest,
	}

	for _, validEnv := range validEnvs {
		if env == validEnv {
			return true
		}
	}
	return false
}

// isValidDeploymentStrategy validates deployment strategy
func isValidDeploymentStrategy(strategy DeploymentStrategy) bool {
	validStrategies := []DeploymentStrategy{
		StrategyRolling,
		StrategyRecreate,
		StrategyBlueGreen,
		StrategyCanary,
		StrategyABTesting,
	}

	for _, validStrategy := range validStrategies {
		if strategy == validStrategy {
			return true
		}
	}
	return false
}

// validateResourceRequirements validates resource specifications
func validateResourceRequirements(resources ResourceRequirements) []DomainValidationError {
	var errors []DomainValidationError

	// Validate CPU
	if resources.CPU.Request != "" && !isValidResourceQuantity(resources.CPU.Request) {
		errors = append(errors, DomainValidationError{
			Field:   "resources.cpu.request",
			Message: "invalid CPU request format",
			Code:    "INVALID_CPU_REQUEST",
		})
	}

	if resources.CPU.Limit != "" && !isValidResourceQuantity(resources.CPU.Limit) {
		errors = append(errors, DomainValidationError{
			Field:   "resources.cpu.limit",
			Message: "invalid CPU limit format",
			Code:    "INVALID_CPU_LIMIT",
		})
	}

	// Validate Memory
	if resources.Memory.Request != "" && !isValidResourceQuantity(resources.Memory.Request) {
		errors = append(errors, DomainValidationError{
			Field:   "resources.memory.request",
			Message: "invalid memory request format",
			Code:    "INVALID_MEMORY_REQUEST",
		})
	}

	if resources.Memory.Limit != "" && !isValidResourceQuantity(resources.Memory.Limit) {
		errors = append(errors, DomainValidationError{
			Field:   "resources.memory.limit",
			Message: "invalid memory limit format",
			Code:    "INVALID_MEMORY_LIMIT",
		})
	}

	return errors
}

// validateHealthChecks validates health check configuration
func validateHealthChecks(healthChecks HealthCheckConfig) []DomainValidationError {
	var errors []DomainValidationError

	if healthChecks.Liveness != nil {
		errors = append(errors, validateHealthCheck("liveness", *healthChecks.Liveness)...)
	}

	if healthChecks.Readiness != nil {
		errors = append(errors, validateHealthCheck("readiness", *healthChecks.Readiness)...)
	}

	if healthChecks.Startup != nil {
		errors = append(errors, validateHealthCheck("startup", *healthChecks.Startup)...)
	}

	return errors
}

// validateHealthCheck validates a single health check
func validateHealthCheck(checkType string, check HealthCheck) []DomainValidationError {
	var errors []DomainValidationError

	// Validate check type
	validTypes := []HealthCheckType{HealthCheckTypeHTTP, HealthCheckTypeTCP, HealthCheckTypeExec}
	isValidType := false
	for _, validType := range validTypes {
		if check.Type == validType {
			isValidType = true
			break
		}
	}

	if !isValidType {
		errors = append(errors, DomainValidationError{
			Field:   fmt.Sprintf("health_checks.%s.type", checkType),
			Message: "invalid health check type",
			Code:    "INVALID_HEALTH_CHECK_TYPE",
		})
	}

	// Type-specific validation
	switch check.Type {
	case HealthCheckTypeHTTP:
		if check.Path == "" {
			errors = append(errors, DomainValidationError{
				Field:   fmt.Sprintf("health_checks.%s.path", checkType),
				Message: "HTTP health check requires path",
				Code:    "MISSING_HTTP_PATH",
			})
		}
		if check.Port <= 0 {
			errors = append(errors, DomainValidationError{
				Field:   fmt.Sprintf("health_checks.%s.port", checkType),
				Message: "HTTP health check requires valid port",
				Code:    "INVALID_HTTP_PORT",
			})
		}
	case HealthCheckTypeTCP:
		if check.Port <= 0 {
			errors = append(errors, DomainValidationError{
				Field:   fmt.Sprintf("health_checks.%s.port", checkType),
				Message: "TCP health check requires valid port",
				Code:    "INVALID_TCP_PORT",
			})
		}
	case HealthCheckTypeExec:
		if len(check.Command) == 0 {
			errors = append(errors, DomainValidationError{
				Field:   fmt.Sprintf("health_checks.%s.command", checkType),
				Message: "Exec health check requires command",
				Code:    "MISSING_EXEC_COMMAND",
			})
		}
	}

	return errors
}

// validatePorts validates service port configuration
func validatePorts(ports []ServicePort) []DomainValidationError {
	var errors []DomainValidationError

	for i, port := range ports {
		if port.Port <= 0 || port.Port > 65535 {
			errors = append(errors, DomainValidationError{
				Field:   fmt.Sprintf("ports[%d].port", i),
				Message: "port must be between 1 and 65535",
				Code:    "INVALID_PORT",
			})
		}

		if port.TargetPort <= 0 || port.TargetPort > 65535 {
			errors = append(errors, DomainValidationError{
				Field:   fmt.Sprintf("ports[%d].target_port", i),
				Message: "target port must be between 1 and 65535",
				Code:    "INVALID_TARGET_PORT",
			})
		}

		if port.Protocol != ProtocolTCP && port.Protocol != ProtocolUDP {
			errors = append(errors, DomainValidationError{
				Field:   fmt.Sprintf("ports[%d].protocol", i),
				Message: "protocol must be TCP or UDP",
				Code:    "INVALID_PROTOCOL",
			})
		}
	}

	return errors
}

// isValidResourceQuantity validates Kubernetes resource quantity format
func isValidResourceQuantity(quantity string) bool {
	// Simplified validation for resource quantities (CPU: m, cores; Memory: Ki, Mi, Gi, etc.)
	cpuPattern := `^[0-9]+[m]?$|^[0-9]*\.?[0-9]+$`
	memoryPattern := `^[0-9]+[KMGTPE]i?$`

	cpuMatch, _ := regexp.MatchString(cpuPattern, quantity)
	memoryMatch, _ := regexp.MatchString(memoryPattern, quantity)

	return cpuMatch || memoryMatch
}

// Business Rules for Security

// GetSecurityRecommendations returns security recommendations for deployment
func GetSecurityRecommendations(req *DeploymentRequest) []SecurityRecommendation {
	var recommendations []SecurityRecommendation

	// Check if running as root
	if req.Configuration.SecurityContext.RunAsNonRoot == nil || !*req.Configuration.SecurityContext.RunAsNonRoot {
		recommendations = append(recommendations, SecurityRecommendation{
			Type:        "security_context",
			Priority:    "high",
			Description: "Configure deployment to run as non-root user",
			Remediation: "Set securityContext.runAsNonRoot: true",
		})
	}

	// Check read-only root filesystem
	if req.Configuration.SecurityContext.ReadOnlyRootFS == nil || !*req.Configuration.SecurityContext.ReadOnlyRootFS {
		recommendations = append(recommendations, SecurityRecommendation{
			Type:        "security_context",
			Priority:    "medium",
			Description: "Use read-only root filesystem for better security",
			Remediation: "Set securityContext.readOnlyRootFilesystem: true",
		})
	}

	// Check for privileged escalation
	if req.Configuration.SecurityContext.AllowPrivilegeEsc == nil || *req.Configuration.SecurityContext.AllowPrivilegeEsc {
		recommendations = append(recommendations, SecurityRecommendation{
			Type:        "security_context",
			Priority:    "high",
			Description: "Disable privilege escalation",
			Remediation: "Set securityContext.allowPrivilegeEscalation: false",
		})
	}

	// Check for resource limits
	if req.Resources.CPU.Limit == "" || req.Resources.Memory.Limit == "" {
		recommendations = append(recommendations, SecurityRecommendation{
			Type:        "resources",
			Priority:    "medium",
			Description: "Set resource limits to prevent resource exhaustion attacks",
			Remediation: "Define CPU and memory limits in resources section",
		})
	}

	return recommendations
}

// SecurityRecommendation represents a security recommendation
type SecurityRecommendation struct {
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}
