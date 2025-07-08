package deploy

import (
	"context"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
)

// K8sDeployerAdapter provides an interface for Kubernetes deployment operations
type K8sDeployerAdapter interface {
	// Deploy performs the actual deployment
	Deploy(config kubernetes.DeploymentConfig) (*kubernetes.DeploymentResult, error)

	// CheckApplicationHealth checks the health of a deployment
	CheckApplicationHealth(ctx context.Context, options kubernetes.HealthCheckOptions) (*kubernetes.HealthCheckResult, error)

	// WaitForRollout waits for a rollout to complete
	WaitForRollout(ctx context.Context, config kubernetes.RolloutConfig) error

	// GetRolloutHistory gets the rollout history for a deployment
	GetRolloutHistory(ctx context.Context, config kubernetes.RolloutHistoryConfig) (*kubernetes.RolloutHistory, error)

	// RollbackDeployment performs a rollback operation
	RollbackDeployment(ctx context.Context, config kubernetes.RollbackConfig) error
}

// DeploymentStrategy defines the interface for different deployment strategies
type DeploymentStrategy interface {
	// GetName returns the strategy name
	GetName() string

	// GetDescription returns a human-readable description
	GetDescription() string

	// Deploy executes the deployment using this strategy
	Deploy(ctx context.Context, config DeploymentConfig) (*DeploymentResult, error)

	// Rollback performs a rollback if supported by the strategy
	Rollback(ctx context.Context, config DeploymentConfig) error

	// ValidatePrerequisites checks if the strategy can be used
	ValidatePrerequisites(ctx context.Context, config DeploymentConfig) error
}

// DeploymentConfig contains all configuration for a deployment
type DeploymentConfig struct {
	// Basic configuration
	SessionID    string
	Namespace    string
	AppName      string
	ImageRef     string
	ManifestPath string

	// Deployment parameters
	Replicas    int
	WaitTimeout time.Duration
	DryRun      bool

	// Resources
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string

	// Service configuration
	Port        int
	ServiceType string

	// Advanced options
	Environment    map[string]string
	Labels         map[string]string
	Annotations    map[string]string
	IncludeIngress bool

	// Dependencies
	K8sDeployer      K8sDeployerAdapter
	ProgressReporter interface{} // Progress reporting interface
	Logger           *slog.Logger
}

// DeploymentResult contains the results of a deployment
type DeploymentResult struct {
	Success   bool
	Strategy  string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	// Kubernetes resources created/updated
	Resources []DeployedResource

	// Health check results
	HealthStatus  string
	ReadyReplicas int
	TotalReplicas int

	// Rollback information
	RollbackAvailable bool
	PreviousVersion   string

	// Error details if failed
	Error                       error
	ConsolidatedFailureAnalysis *ConsolidatedFailureAnalysis
}

// DeployedResource represents a deployed Kubernetes resource
type DeployedResource struct {
	Kind       string
	Name       string
	Namespace  string
	APIVersion string
	Status     string
}

// ConsolidatedFailureAnalysis provides detailed failure information
type ConsolidatedFailureAnalysis struct {
	Stage       string
	Reason      string
	Message     string
	Suggestions []string
	CanRetry    bool
	CanRollback bool
}

// BaseStrategy provides common functionality for all strategies
type BaseStrategy struct {
	logger *slog.Logger
}

// NewBaseStrategy creates a new base strategy
func NewBaseStrategy(logger *slog.Logger) *BaseStrategy {
	return &BaseStrategy{
		logger: logger,
	}
}

// WaitForDeployment waits for a deployment to become ready
func (bs *BaseStrategy) WaitForDeployment(ctx context.Context, config DeploymentConfig, deploymentName string) error {
	bs.logger.Info("Waiting for deployment to become ready",
		"deployment", deploymentName,
		"namespace", config.Namespace)

	// Use K8sDeployer to check deployment status
	healthOptions := kubernetes.HealthCheckOptions{
		Namespace:     config.Namespace,
		LabelSelector: "app=" + deploymentName,
		Timeout:       config.WaitTimeout,
	}

	result, err := config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
	if err != nil {
		return err
	}

	if !result.Success {
		bs.logger.Warn("Deployment is not healthy",
			"deployment", deploymentName,
			"ready_pods", result.Summary.ReadyPods,
			"total_pods", result.Summary.TotalPods)
	}

	return nil
}

// GetServiceEndpoint retrieves the service endpoint for a deployment
func (bs *BaseStrategy) GetServiceEndpoint(ctx context.Context, config DeploymentConfig) (string, error) {
	// This would interact with Kubernetes to get the actual endpoint
	// For now, return a placeholder
	endpoint := ""

	switch config.ServiceType {
	case "LoadBalancer":
		endpoint = "pending-external-ip"
	case "NodePort":
		endpoint = "node-ip:node-port"
	default:
		endpoint = config.AppName + "." + config.Namespace + ".svc.cluster.local"
	}

	return endpoint, nil
}

// CreateFailureAnalysis creates a failure analysis from an error
func (bs *BaseStrategy) CreateFailureAnalysis(err error, stage string) *ConsolidatedFailureAnalysis {
	return &ConsolidatedFailureAnalysis{
		Stage:   stage,
		Reason:  "deployment_failed",
		Message: err.Error(),
		Suggestions: []string{
			"Check if the cluster is accessible",
			"Verify RBAC permissions",
			"Ensure the namespace exists",
			"Check resource quotas",
		},
		CanRetry:    true,
		CanRollback: stage != "pre_deployment",
	}
}
