package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
	"github.com/Azure/containerization-assist/pkg/infrastructure/kubernetes"
)

// K8sResult contains the results of Kubernetes deployment operations
type K8sResult struct {
	Namespace  string                 `json:"namespace"`
	AppName    string                 `json:"app_name"`
	Manifests  map[string]interface{} `json:"manifests"`
	ServiceURL string                 `json:"service_url,omitempty"`
	IngressURL string                 `json:"ingress_url,omitempty"`
	DeployedAt time.Time              `json:"deployed_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// GenerateManifests creates Kubernetes manifests for deployment using real K8s operations
func GenerateManifests(buildResult *BuildResult, appName, namespace string, port int, repoPath, registryURL string, logger *slog.Logger) (*K8sResult, error) {

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

	if registryURL == "" {
		registryURL = "localhost:5001" // Default fallback
	}

	// Create image reference using the provided registry URL
	imageRef := fmt.Sprintf("%s/%s:%s", registryURL, buildResult.ImageName, buildResult.ImageTag)

	// Use the real K8s manifest service

	// Create manifests directory in the repository path (persistent)
	manifestDir := fmt.Sprintf("%s/manifests", repoPath)
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create manifests directory: %w", err)
	}

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
			"containerization-assist.io/generated": "true",
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

	manifestService := kubernetes.NewManifestService(logger)
	manifestResult, err := manifestService.GenerateManifests(context.Background(), manifestOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate K8s manifests: %v", err)
	}

	if !manifestResult.Success {
		return nil, fmt.Errorf("K8s manifest generation unsuccessful: %v", manifestResult.Error)
	}

	// Convert manifest content to interface map for compatibility
	manifests := map[string]interface{}{
		"path":      manifestResult.ManifestPath,
		"manifests": manifestResult.Manifests,
		"template":  manifestResult.Template,
	}

	return &K8sResult{
		Namespace:  namespace,
		AppName:    appName,
		Manifests:  manifests,
		DeployedAt: time.Now(),
	}, nil
}

// DeployToKubernetes applies the manifests to a Kubernetes cluster using real K8s operations
func DeployToKubernetes(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) error {
	if k8sResult == nil {
		return fmt.Errorf("k8s result is required")
	}

	// Initialize Kubernetes client and service
	kubeClient := kubernetes.NewKubeCmdRunner(&core.DefaultCommandRunner{})
	deploymentService := kubernetes.NewService(kubeClient, logger)

	// Get the manifest path from the generated manifests
	manifestPath, ok := k8sResult.Manifests["path"].(string)
	if !ok || manifestPath == "" {
		// If no manifest path, we'll need to create a temporary manifest file
		return fmt.Errorf("manifest path not available for deployment")
	}

	// Deploy the manifests using the real K8s service
	deploymentOptions := kubernetes.DeploymentOptions{
		Namespace:   k8sResult.Namespace,
		DryRun:      false,
		Force:       false,
		Wait:        true,
		WaitTimeout: 300 * time.Second,
		Validate:    false, // Validation will be done separately with retry logic
	}

	// Get the directory containing the manifest files
	// manifestPath might be a specific file or already a directory
	var manifestDir string
	if fileInfo, err := os.Stat(manifestPath); err == nil && fileInfo.IsDir() {
		// manifestPath is already a directory
		manifestDir = manifestPath
	} else {
		// manifestPath is a file, get its directory
		manifestDir = filepath.Dir(manifestPath)
	}

	// Get all YAML files in the manifest directory to ensure we deploy everything
	yamlFiles, err := getYAMLFilesInDirectory(manifestDir, logger)
	if err != nil {
		return fmt.Errorf("failed to get YAML files from manifest directory: %v", err)
	}

	// Deploy each manifest file individually to ensure all are deployed
	// Collect all results first, then report failures at the end
	var allResources []kubernetes.DeployedResource
	var deploymentErrors []string
	var successfulDeployments []string

	for _, yamlFile := range yamlFiles {

		deploymentResult, err := deploymentService.DeployManifest(ctx, yamlFile, deploymentOptions)
		if err != nil {
			deploymentErrors = append(deploymentErrors, fmt.Sprintf("File: %s, Error: %v", yamlFile, err))
			continue // Continue with other files
		}

		if !deploymentResult.Success {
			errorMsg := extractDeploymentErrorMessage(deploymentResult)
			fullError := fmt.Sprintf("File: %s, Error: %s", yamlFile, errorMsg)
			if deploymentResult.Output != "" {
				fullError = fmt.Sprintf("File: %s, Error: %s, kubectl output: %s", yamlFile, errorMsg, deploymentResult.Output)
			}
			deploymentErrors = append(deploymentErrors, fullError)
			continue // Continue with other files
		}

		// Collect resources from successful deployments
		allResources = append(allResources, deploymentResult.Resources...)
		successfulDeployments = append(successfulDeployments, yamlFile)
	}

	// Report results
	if len(successfulDeployments) > 0 {
	}

	// Report all deployment results - both failures and successes
	if len(deploymentErrors) > 0 {

		// Return error with details about what failed and what succeeded
		if len(successfulDeployments) == 0 {
			return fmt.Errorf("all manifest deployments failed: %v", deploymentErrors)
		}
		return fmt.Errorf("partial deployment failure: %d/%d files failed to deploy. Errors: %v. Successfully deployed: %v",
			len(deploymentErrors), len(yamlFiles), deploymentErrors, successfulDeployments)

	}

	// Log the deployed resources for debugging

	// Simple deployment validation - check for pod/deployment errors

	// Wait a bit for pods to initialize
	time.Sleep(5 * time.Second)

	validationResult, err := deploymentService.ValidateDeployment(ctx, manifestPath, k8sResult.Namespace)
	if err != nil {
		return fmt.Errorf("deployment validation error: %w", err)
	}

	// If validation fails, return error with pod details
	if !validationResult.Success {

		// Build detailed error message with pod diagnostics
		errorMsg := fmt.Sprintf("deployment validation failed: %d/%d pods ready",
			validationResult.PodsReady, validationResult.PodsTotal)

		// Add pod status details
		if len(validationResult.Pods) > 0 {
			errorMsg += "\n\nPod Status Details:"
			for _, pod := range validationResult.Pods {
				errorMsg += fmt.Sprintf("\n- Pod: %s, Status: %s, Ready: %s, Restarts: %d",
					pod.Name, pod.Status, pod.Ready, pod.Restarts)
			}
		}

		// Add deployment error details if available
		if validationResult.Error != nil {
			errorMsg += fmt.Sprintf("\n\nDeployment Error: %s", validationResult.Error.Message)

			// Include kubectl output for debugging
			if validationResult.Error.Output != "" {
				errorMsg += fmt.Sprintf("\n\nKubectl Output:\n%s", validationResult.Error.Output)
			}
		}

		// Add service status if available
		if len(validationResult.Services) > 0 {
			errorMsg += "\n\nService Status:"
			for _, svc := range validationResult.Services {
				errorMsg += fmt.Sprintf("\n- Service: %s, Type: %s, ClusterIP: %s",
					svc.Name, svc.Type, svc.ClusterIP)
			}
		}

		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}

// GetServiceEndpoint retrieves the external endpoint for a deployed service using kubectl
func GetServiceEndpoint(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) (string, error) {
	if k8sResult == nil {
		return "", fmt.Errorf("k8s result is required")
	}

	// Use kubectl to get service details - this is the most reliable method for kind clusters
	cmd := exec.CommandContext(ctx, "kubectl", "get", "service", k8sResult.AppName)

	output, err := cmd.CombinedOutput()
	if err != nil {

		// Fallback: try to get cluster IP and port
		cmd = exec.CommandContext(ctx, "kubectl", "get", "service", k8sResult.AppName)

		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to get service endpoint: %v", err)
		}

		endpoint := fmt.Sprintf("http://%s", string(output))
		return endpoint, nil
	}

	// For kind clusters, construct localhost URL with NodePort
	nodePortStr := strings.TrimSpace(string(output))
	if nodePortStr != "" && nodePortStr != "<no value>" {
		endpoint := fmt.Sprintf("http://localhost:%s", nodePortStr)
		return endpoint, nil
	}

	return "", fmt.Errorf("could not determine service endpoint")
}

// CheckDeploymentHealth function removed as dead code

// getYAMLFilesInDirectory returns all YAML files in the given directory
func getYAMLFilesInDirectory(dirPath string, logger *slog.Logger) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var yamlFiles []string
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			switch ext := strings.ToLower(filepath.Ext(entry.Name())); ext {
			case ".yaml", ".yml":
				yamlFiles = append(yamlFiles, filepath.Join(dirPath, entry.Name()))
			}
		}
	}

	return yamlFiles, nil
}

// extractDeploymentErrorMessage extracts error message from a failed deployment result
func extractDeploymentErrorMessage(deploymentResult *kubernetes.DeploymentResult) string {
	// Check if we have error details
	if deploymentResult.Error != nil {
		return fmt.Sprintf("%s: %s", deploymentResult.Error.Type, deploymentResult.Error.Message)
	}

	// Check validation result for details
	if validationData, ok := deploymentResult.Context["validation"]; ok {
		if validation, ok := validationData.(*kubernetes.ValidationResult); ok && validation.Error != nil {
			return fmt.Sprintf("validation failed: %s", validation.Error.Message)
		}
		return "deployment validation failed but no error details available"
	}

	// Fallback to resources deployed count
	return fmt.Sprintf("deployment failed (resources deployed: %d)", len(deploymentResult.Resources))
}
