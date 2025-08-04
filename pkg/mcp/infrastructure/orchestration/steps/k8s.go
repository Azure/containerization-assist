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

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/utils"
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
func GenerateManifests(buildResult *BuildResult, appName, namespace string, port int, repoPath string, logger *slog.Logger) (*K8sResult, error) {
	logger.Info("Generating Kubernetes manifests",
		"app_name", appName,
		"namespace", namespace,
		"port", port,
		"repo_path", repoPath)

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

	// Create manifests directory in the repository path (persistent)
	manifestDir := fmt.Sprintf("%s/manifests", repoPath)
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
	if k8sResult == nil {
		return fmt.Errorf("k8s result is required")
	}

	logger.Info("Deploying to Kubernetes",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace)

	// Initialize Kubernetes client and service
	kubeClient := kubernetes.NewKubeCmdRunner(&runner.DefaultCommandRunner{})
	deploymentService := kubernetes.NewService(kubeClient, logger.With("component", "k8s_deployment_service"))

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
		Validate:    false, // Validation will be done separately with retry logic
	}

	logger.Info("Deploying manifests with K8s service",
		"manifest_path", manifestPath,
		"options", fmt.Sprintf("%+v", deploymentOptions))

	// Get the directory containing the manifest files
	// manifestPath might be a specific file, so we need the directory
	manifestDir := filepath.Dir(manifestPath)

	// Get all YAML files in the manifest directory to ensure we deploy everything
	yamlFiles, err := getYAMLFilesInDirectory(manifestDir)
	if err != nil {
		logger.Error("Failed to get YAML files from manifest directory", "error", err, "manifest_dir", manifestDir)
		return fmt.Errorf("failed to get YAML files from manifest directory: %v", err)
	}

	logger.Info("Found manifest files to deploy", "files", yamlFiles, "count", len(yamlFiles))

	// Deploy each manifest file individually to ensure all are deployed
	// Collect all results first, then report failures at the end
	var allResources []kubernetes.DeployedResource
	var deploymentErrors []string
	var successfulDeployments []string

	for _, yamlFile := range yamlFiles {
		logger.Info("Deploying manifest file", "file", yamlFile)

		deploymentResult, err := deploymentService.DeployManifest(ctx, yamlFile, deploymentOptions)
		if err != nil {
			deploymentErrors = append(deploymentErrors, fmt.Sprintf("%s: %v", yamlFile, err))
			logger.Error("Failed to deploy manifest file", "file", yamlFile, "error", err)
			continue // Continue with other files
		}

		if !deploymentResult.Success {
			// Check if we have error details
			var errorMsg string
			if deploymentResult.Error != nil {
				errorMsg = fmt.Sprintf("%s: %s", deploymentResult.Error.Type, deploymentResult.Error.Message)
			} else if validationData, ok := deploymentResult.Context["validation"]; ok {
				// Check validation result for details
				if validation, ok := validationData.(*kubernetes.ValidationResult); ok && validation.Error != nil {
					errorMsg = fmt.Sprintf("validation failed: %s", validation.Error.Message)
				} else {
					errorMsg = "deployment validation failed but no error details available"
				}
			} else {
				errorMsg = fmt.Sprintf("deployment failed (resources deployed: %d)", len(deploymentResult.Resources))
			}

			deploymentErrors = append(deploymentErrors, fmt.Sprintf("%s: %s", yamlFile, errorMsg))
			logger.Error("Kubernetes deployment unsuccessful for file",
				"file", yamlFile,
				"error", errorMsg,
				"resources_deployed", len(deploymentResult.Resources),
				"output", deploymentResult.Output)
			continue // Continue with other files
		}

		// Collect resources from successful deployments
		allResources = append(allResources, deploymentResult.Resources...)
		successfulDeployments = append(successfulDeployments, yamlFile)
		logger.Info("Successfully deployed manifest file",
			"file", yamlFile,
			"resources_deployed", len(deploymentResult.Resources))
	}

	// Report results
	if len(successfulDeployments) > 0 {
		logger.Info("Successfully deployed manifest files",
			"files", successfulDeployments,
			"count", len(successfulDeployments))
	}

	// If we have deployment errors, we need to return an error to indicate partial failure
	if len(deploymentErrors) > 0 {
		logger.Error("Some manifest files failed to deploy",
			"errors", deploymentErrors,
			"failed_count", len(deploymentErrors),
			"successful_count", len(successfulDeployments))

		// Return error with details about what failed and what succeeded
		if len(successfulDeployments) == 0 {
			return fmt.Errorf("all manifest deployments failed: %v", deploymentErrors)
		} else {
			return fmt.Errorf("partial deployment failure: %d/%d files failed to deploy. Errors: %v. Successfully deployed: %v",
				len(deploymentErrors), len(yamlFiles), deploymentErrors, successfulDeployments)
		}
	}

	logger.Info("Kubernetes deployment completed successfully",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace,
		"total_resources_deployed", len(allResources),
		"manifest_files_deployed", len(yamlFiles))

	// Validate deployment with AI-powered retry logic
	logger.Info("Starting deployment validation with AI-powered retry logic")
	var validationResult *kubernetes.ValidationResult

	// Use AI-powered retry for deployment validation with enhanced context
	err = utils.WithAIRetry(ctx, "validate_kubernetes_deployment", 3, func() error {
		// Wait a bit for pods to initialize on first attempt
		time.Sleep(2 * time.Second)

		var validationErr error
		validationResult, validationErr = deploymentService.ValidateDeployment(ctx, manifestPath, k8sResult.Namespace)
		if validationErr != nil {
			return fmt.Errorf("deployment validation error: %w", validationErr)
		}

		if !validationResult.Success {
			errorMsg := fmt.Sprintf("deployment validation failed: %d/%d pods ready",
				validationResult.PodsReady, validationResult.PodsTotal)

			// Get pod logs and events if available
			var podLogs string
			var podEvents string
			if validationResult.PodsTotal > 0 {
				// Try to get logs from the first pod
				cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-n", k8sResult.Namespace,
					"--selector=app="+k8sResult.AppName, "-o", "jsonpath={.items[0].metadata.name}")
				podName, _ := cmd.Output()
				if len(podName) > 0 {
					// Get pod logs
					logCmd := exec.CommandContext(ctx, "kubectl", "logs", "-n", k8sResult.Namespace,
						string(podName), "--tail=50")
					logs, _ := logCmd.Output()
					if len(logs) > 0 {
						podLogs = fmt.Sprintf("\n\n--- POD LOGS ---\n%s", string(logs))
					}

					// Get pod events
					eventCmd := exec.CommandContext(ctx, "kubectl", "describe", "pod", "-n", k8sResult.Namespace, string(podName))
					events, _ := eventCmd.Output()
					if len(events) > 0 {
						// Extract just the Events section
						eventStr := string(events)
						if idx := strings.Index(eventStr, "Events:"); idx >= 0 {
							podEvents = fmt.Sprintf("\n\n--- POD EVENTS ---\n%s", eventStr[idx:])
						}
					}
				}
			}

			// Include pod logs and context in error
			// Extract port from k8sResult metadata if available
			port := 0
			if portVal, ok := k8sResult.Metadata["port"].(int); ok {
				port = portVal
			}

			// Extract image ref from k8sResult metadata if available
			imageRef := "unknown"
			if imageVal, ok := k8sResult.Metadata["image_ref"].(string); ok {
				imageRef = imageVal
			}

			contextInfo := fmt.Sprintf("App: %s, Namespace: %s, Image: %s, Port: %d%s%s",
				k8sResult.AppName, k8sResult.Namespace, imageRef, port, podLogs, podEvents)

			if validationResult.Error != nil {
				errorMsg = fmt.Sprintf("%s (error: %s)", errorMsg, validationResult.Error.Message)
			}
			return fmt.Errorf("%s\nContext: %s", errorMsg, contextInfo)
		}

		logger.Info("Deployment validation successful",
			"pods_ready", validationResult.PodsReady,
			"pods_total", validationResult.PodsTotal)
		return nil
	}, logger)

	if err != nil {
		logger.Error("Deployment validation failed after AI-assisted retries", "error", err)
		return fmt.Errorf("deployment validation failed: %w", err)
	}

	return nil
}

// GetServiceEndpoint retrieves the external endpoint for a deployed service using kubectl
func GetServiceEndpoint(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) (string, error) {
	if k8sResult == nil {
		return "", fmt.Errorf("k8s result is required")
	}

	logger.Info("Getting service endpoint",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace)

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
	nodePortStr := strings.TrimSpace(string(output))
	if nodePortStr != "" && nodePortStr != "<no value>" {
		endpoint := fmt.Sprintf("http://localhost:%s", nodePortStr)
		logger.Info("Service endpoint retrieved (NodePort)", "endpoint", endpoint)
		return endpoint, nil
	}

	return "", fmt.Errorf("could not determine service endpoint")
}

// CheckDeploymentHealth verifies that the deployment is healthy with comprehensive diagnostics
func CheckDeploymentHealth(ctx context.Context, k8sResult *K8sResult, logger *slog.Logger) error {
	if k8sResult == nil {
		return fmt.Errorf("k8s result is required")
	}

	logger.Info("Checking deployment health with comprehensive diagnostics",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace)

	// Use the new comprehensive verification
	diagnostics, err := VerifyDeploymentWithDiagnostics(ctx, k8sResult, logger)
	if err != nil {
		logger.Error("Failed to get deployment diagnostics", "error", err)
		return fmt.Errorf("failed to get deployment diagnostics: %v", err)
	}

	// Generate diagnostic report
	report := GenerateDiagnosticReport(diagnostics)

	if diagnostics.DeploymentOK {
		logger.Info("Deployment health check passed",
			"app_name", k8sResult.AppName,
			"namespace", k8sResult.Namespace,
			"pods_ready", diagnostics.PodsReady,
			"pods_total", diagnostics.PodsTotal)
		logger.Debug("Deployment diagnostics", "report", report)
		return nil
	}

	// Deployment is not healthy - provide detailed error with diagnostics
	logger.Error("Deployment health check failed",
		"app_name", k8sResult.AppName,
		"namespace", k8sResult.Namespace,
		"pods_ready", diagnostics.PodsReady,
		"pods_total", diagnostics.PodsTotal,
		"errors", diagnostics.Errors)

	// Include diagnostics in error for AI analysis
	return fmt.Errorf("deployment not healthy: %d/%d pods ready\n\nDiagnostics:\n%s",
		diagnostics.PodsReady, diagnostics.PodsTotal, report)
}

// getYAMLFilesInDirectory returns all YAML files in the given directory
func getYAMLFilesInDirectory(dirPath string) ([]string, error) {
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
