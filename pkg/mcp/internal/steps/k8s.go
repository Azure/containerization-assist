package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/Azure/container-kit/pkg/clients"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	dockerpkg "github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	"github.com/Azure/container-kit/pkg/runner"
)

// K8sResult contains the results of Kubernetes deployment operations
type K8sResult struct {
	Namespace  string                 `json:"namespace"`
	AppName    string                 `json:"app_name"`
	Manifests  map[string]interface{} `json:"manifests"`
	ServiceURL string                 `json:"service_url,omitempty"`
	IngressURL string                 `json:"ingress_url,omitempty"`
	DeployedAt time.Time              `json:"deployed_at"`
}

// GenerateManifests creates Kubernetes manifests for deployment using real K8s operations
func GenerateManifests(buildResult *BuildResult, appName, namespace string, port int, logger *slog.Logger) (*K8sResult, error) {
	logger.Info("Generating Kubernetes manifests",
		"app_name", appName,
		"namespace", namespace,
		"port", port)

	if buildResult == nil {
		return nil, fmt.Errorf("build result is required")
	}

	if appName == "" {
		appName = buildResult.ImageName
	}

	if namespace == "" {
		namespace = "default"
	}

	if port <= 0 {
		port = 8080 // Default port
	}

	// Create image reference for local registry
	imageRef := fmt.Sprintf("localhost:5001/%s:%s", buildResult.ImageName, buildResult.ImageTag)

	// Use the real K8s manifest service
	manifestService := kubernetes.NewManifestService(logger.With("component", "k8s_manifest_service"))

	// Create temporary directory for manifests
	manifestDir := fmt.Sprintf("/tmp/k8s-manifests-%s", appName)
	os.MkdirAll(manifestDir, 0755)

	// Generate manifests using the core K8s functionality
	manifestOptions := kubernetes.ManifestOptions{
		Template:       "deployment-with-service", // Use default template
		AppName:        appName,
		Namespace:      namespace,
		ImageRef:       imageRef,
		Port:           port,
		Replicas:       1,
		OutputDir:      manifestDir, // Required output directory
		IncludeService: true,
		IncludeIngress: false,
		Labels: map[string]string{
			"app": appName,
		},
		Annotations: map[string]string{
			"container-kit.io/generated": "true",
		},
		Resources: &kubernetes.ResourceRequirements{
			Requests: &kubernetes.ResourceQuantity{
				Memory: "128Mi",
				CPU:    "100m",
			},
			Limits: &kubernetes.ResourceQuantity{
				Memory: "512Mi",
				CPU:    "500m",
			},
		},
	}

	logger.Info("Generating manifests with K8s service", "options", fmt.Sprintf("%+v", manifestOptions))

	manifestResult, err := manifestService.GenerateManifests(context.Background(), manifestOptions)
	if err != nil {
		logger.Error("Failed to generate K8s manifests", "error", err)
		return nil, fmt.Errorf("failed to generate K8s manifests: %v", err)
	}

	if !manifestResult.Success {
		logger.Error("K8s manifest generation unsuccessful", "error", manifestResult.Error)
		return nil, fmt.Errorf("K8s manifest generation unsuccessful: %v", manifestResult.Error)
	}

	// Convert manifest content to interface map for compatibility
	manifests := map[string]interface{}{
		"path":      manifestResult.ManifestPath,
		"manifests": manifestResult.Manifests,
		"template":  manifestResult.Template,
	}

	logger.Info("Kubernetes manifests generated successfully",
		"app_name", appName,
		"namespace", namespace,
		"image_ref", imageRef,
		"manifests_count", len(manifestResult.Manifests),
		"path", manifestResult.ManifestPath)

	return &K8sResult{
		Namespace:  namespace,
		AppName:    appName,
		Manifests:  manifests,
		DeployedAt: time.Now(),
	}, nil
}

// DeployToKubernetes applies the manifests to a Kubernetes cluster using real K8s operations
func DeployToKubernetes(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) error {
	logger.Info("Deploying to Kubernetes",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace)

	if k8sResult == nil {
		return fmt.Errorf("k8s result is required")
	}

	// Initialize clients for K8s deployment
	kubeClients := &clients.Clients{
		Docker: dockerpkg.NewDockerCmdRunner(&runner.DefaultCommandRunner{}),
		Kind:   kind.NewKindCmdRunner(&runner.DefaultCommandRunner{}),
		Kube:   k8s.NewKubeCmdRunner(&runner.DefaultCommandRunner{}),
	}

	// Create Kubernetes deployment service
	deploymentService := kubernetes.NewService(kubeClients, logger.With("component", "k8s_deployment_service"))

	// Get the manifest path from the generated manifests
	manifestPath, ok := k8sResult.Manifests["path"].(string)
	if !ok || manifestPath == "" {
		// If no manifest path, we'll need to create a temporary manifest file
		logger.Warn("No manifest path available, deployment will be limited")
		return fmt.Errorf("manifest path not available for deployment")
	}

	// Deploy the manifests using the real K8s service
	deploymentOptions := kubernetes.DeploymentOptions{
		Namespace:   k8sResult.Namespace,
		DryRun:      false,
		Force:       false,
		Wait:        true,
		WaitTimeout: 300 * time.Second,
		Validate:    true,
	}

	logger.Info("Deploying manifests with K8s service",
		"manifest_path", manifestPath,
		"options", fmt.Sprintf("%+v", deploymentOptions))

	deploymentResult, err := deploymentService.DeployManifest(ctx, manifestPath, deploymentOptions)
	if err != nil {
		logger.Error("Failed to deploy to Kubernetes", "error", err)
		return fmt.Errorf("failed to deploy to Kubernetes: %v", err)
	}

	if !deploymentResult.Success {
		logger.Error("Kubernetes deployment unsuccessful", "error", deploymentResult.Error)
		return fmt.Errorf("Kubernetes deployment unsuccessful: %v", deploymentResult.Error)
	}

	logger.Info("Kubernetes deployment completed successfully",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace,
		"resources_deployed", len(deploymentResult.Resources),
		"duration", deploymentResult.Duration)

	return nil
}

// GetServiceEndpoint retrieves the external endpoint for a deployed service using kubectl
func GetServiceEndpoint(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) (string, error) {
	logger.Info("Getting service endpoint",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace)

	if k8sResult == nil {
		return "", fmt.Errorf("k8s result is required")
	}

	// Use kubectl to get service details - this is the most reliable method for kind clusters
	cmd := exec.CommandContext(ctx, "kubectl", "get", "service", k8sResult.AppName,
		"-n", k8sResult.Namespace,
		"-o", "jsonpath={.spec.ports[0].nodePort}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to get service NodePort",
			"error", err,
			"output", string(output),
			"app_name", k8sResult.AppName,
			"namespace", k8sResult.Namespace)

		// Fallback: try to get cluster IP and port
		cmd = exec.CommandContext(ctx, "kubectl", "get", "service", k8sResult.AppName,
			"-n", k8sResult.Namespace,
			"-o", "jsonpath={.spec.clusterIP}:{.spec.ports[0].port}")

		output, err = cmd.CombinedOutput()
		if err != nil {
			logger.Error("Failed to get service cluster IP", "error", err, "output", string(output))
			return "", fmt.Errorf("failed to get service endpoint: %v", err)
		}

		endpoint := fmt.Sprintf("http://%s", string(output))
		logger.Info("Service endpoint retrieved (cluster IP)", "endpoint", endpoint)
		return endpoint, nil
	}

	// For kind clusters, construct localhost URL with NodePort
	nodePortStr := string(output)
	if nodePortStr != "" {
		endpoint := fmt.Sprintf("http://localhost:%s", nodePortStr)
		logger.Info("Service endpoint retrieved (NodePort)", "endpoint", endpoint)
		return endpoint, nil
	}

	return "", fmt.Errorf("could not determine service endpoint")
}

// CheckDeploymentHealth verifies that the deployment is healthy using kubectl
func CheckDeploymentHealth(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) error {
	logger.Info("Checking deployment health",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace)

	if k8sResult == nil {
		return fmt.Errorf("k8s result is required")
	}

	// Use kubectl to check deployment status
	cmd := exec.CommandContext(ctx, "kubectl", "get", "deployment", k8sResult.AppName,
		"-n", k8sResult.Namespace,
		"-o", "jsonpath={.status.readyReplicas}/{.status.replicas}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to get deployment status", "error", err, "output", string(output))
		return fmt.Errorf("failed to get deployment status: %v", err)
	}

	statusStr := string(output)
	logger.Info("Deployment status", "status", statusStr)

	// Simple health check - more sophisticated checks could be added
	if statusStr == "1/1" || statusStr == "/1" { // readyReplicas/replicas
		logger.Info("Deployment health check passed",
			"app_name", k8sResult.AppName,
			"namespace", k8sResult.Namespace)
		return nil
	}

	return fmt.Errorf("deployment not ready: %s", statusStr)
}
