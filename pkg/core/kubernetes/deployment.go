package kubernetes

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/clients"
	"github.com/rs/zerolog"
	"sigs.k8s.io/yaml"
)

// DeploymentConfig contains configuration for deployment operations
type DeploymentConfig struct {
	ManifestPath string
	Namespace    string
	Options      DeploymentOptions
}

// HealthCheckConfig contains configuration for health check operations
type HealthCheckConfig struct {
	Namespace   string
	AppName     string
	WaitTimeout time.Duration
}

// RolloutConfig contains configuration for rollout operations
type RolloutConfig struct {
	ResourceType string
	ResourceName string
	Namespace    string
	Timeout      time.Duration
}

// RolloutHistoryConfig contains configuration for rollout history operations
type RolloutHistoryConfig struct {
	ResourceType string
	ResourceName string
	Namespace    string
}

// RolloutHistory contains rollout history information
type RolloutHistory struct {
	Revisions []RolloutRevision
}

// RolloutRevision represents a single rollout revision
type RolloutRevision struct {
	Number      int
	ChangeDate  time.Time
	ChangeCause string
}

// RollbackConfig contains configuration for rollback operations
type RollbackConfig struct {
	ResourceType string
	ResourceName string
	Namespace    string
	ToRevision   int // Optional - if 0, rollback to previous revision
}

// DeploymentManager provides mechanical Kubernetes deployment operations
type DeploymentManager struct {
	clients *clients.Clients
	logger  zerolog.Logger
}

// NewDeploymentManager creates a new deployment manager
func NewDeploymentManager(clients *clients.Clients, logger zerolog.Logger) *DeploymentManager {
	return &DeploymentManager{
		clients: clients,
		logger:  logger.With().Str("component", "k8s_deployment_manager").Logger(),
	}
}

// DeploymentResult contains the result of a deployment operation
type DeploymentResult struct {
	Success      bool                   `json:"success"`
	ManifestPath string                 `json:"manifest_path"`
	Namespace    string                 `json:"namespace"`
	Resources    []DeployedResource     `json:"resources"`
	Output       string                 `json:"output"`
	Duration     time.Duration          `json:"duration"`
	Context      map[string]interface{} `json:"context"`
	Error        *DeploymentError       `json:"error,omitempty"`
}

// DeployedResource represents a deployed Kubernetes resource
type DeployedResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Ready     bool   `json:"ready"`
	Age       string `json:"age"`
}

// DeploymentError provides detailed deployment error information
type DeploymentError struct {
	Type         string                 `json:"type"` // "kubectl_error", "cluster_error", "validation_error", "timeout_error"
	Message      string                 `json:"message"`
	ManifestPath string                 `json:"manifest_path,omitempty"`
	Output       string                 `json:"output"`
	Context      map[string]interface{} `json:"context"`
}

// ValidationResult contains pod validation results
type ValidationResult struct {
	Success   bool                   `json:"success"`
	PodsReady int                    `json:"pods_ready"`
	PodsTotal int                    `json:"pods_total"`
	Pods      []PodStatus            `json:"pods"`
	Services  []ServiceStatus        `json:"services"`
	Duration  time.Duration          `json:"duration"`
	Context   map[string]interface{} `json:"context"`
	Error     *DeploymentError       `json:"error,omitempty"`
}

// PodStatus represents the status of a pod
type PodStatus struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
	Node      string `json:"node,omitempty"`
	IP        string `json:"ip,omitempty"`
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Type      string   `json:"type"`
	ClusterIP string   `json:"cluster_ip"`
	Ports     []string `json:"ports"`
	Age       string   `json:"age"`
}

// DeploymentOptions contains options for deployment
type DeploymentOptions struct {
	Namespace   string
	Wait        bool
	WaitTimeout time.Duration
	DryRun      bool
	Force       bool
	Validate    bool
}

// DeployManifest deploys a Kubernetes manifest file
func (dm *DeploymentManager) DeployManifest(ctx context.Context, manifestPath string, options DeploymentOptions) (*DeploymentResult, error) {
	startTime := time.Now()

	result := &DeploymentResult{
		ManifestPath: manifestPath,
		Namespace:    options.Namespace,
		Resources:    make([]DeployedResource, 0),
		Context:      make(map[string]interface{}),
	}

	dm.logger.Info().
		Str("manifest_path", manifestPath).
		Str("namespace", options.Namespace).
		Bool("dry_run", options.DryRun).
		Msg("Starting manifest deployment")

	// Validate inputs
	if err := dm.validateDeploymentInputs(manifestPath, options); err != nil {
		result.Error = &DeploymentError{
			Type:         "validation_error",
			Message:      err.Error(),
			ManifestPath: manifestPath,
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Check kubectl installation
	if err := dm.checkKubectlInstalled(); err != nil {
		result.Error = &DeploymentError{
			Type:         "kubectl_error",
			Message:      err.Error(),
			ManifestPath: manifestPath,
			Context: map[string]interface{}{
				"suggestion": "Install kubectl or ensure it's in PATH",
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Set timeout context if specified
	deployCtx := ctx
	if options.WaitTimeout > 0 {
		var cancel context.CancelFunc
		deployCtx, cancel = context.WithTimeout(ctx, options.WaitTimeout)
		defer cancel()
	}

	// Execute deployment using the existing clients
	output, err := dm.clients.Kube.Apply(deployCtx, manifestPath)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {
		dm.logger.Error().Err(err).Str("output", output).Msg("Manifest deployment failed")

		result.Error = &DeploymentError{
			Type:         dm.categorizeError(err, output),
			Message:      fmt.Sprintf("Deployment failed: %v", err),
			ManifestPath: manifestPath,
			Output:       output,
			Context: map[string]interface{}{
				"options":  options,
				"duration": result.Duration.Seconds(),
			},
		}
		return result, nil
	}

	// Extract deployed resources from output
	result.Resources = dm.parseDeploymentOutput(output)

	// If validation is requested, validate the deployment
	if options.Validate {
		validationResult, err := dm.ValidateDeployment(ctx, manifestPath, options.Namespace)
		if err != nil {
			dm.logger.Warn().Err(err).Msg("Failed to validate deployment")
		} else {
			result.Context["validation"] = validationResult
			result.Success = validationResult.Success
		}
	} else {
		result.Success = true
	}

	result.Context = map[string]interface{}{
		"deployment_time": result.Duration.Seconds(),
		"resource_count":  len(result.Resources),
		"namespace":       options.Namespace,
		"dry_run":         options.DryRun,
	}

	dm.logger.Info().
		Str("manifest_path", manifestPath).
		Int("resource_count", len(result.Resources)).
		Bool("success", result.Success).
		Dur("duration", result.Duration).
		Msg("Manifest deployment completed")

	return result, nil
}

// ValidateDeployment validates that deployed resources are healthy
func (dm *DeploymentManager) ValidateDeployment(ctx context.Context, manifestPath string, namespace string) (*ValidationResult, error) {
	startTime := time.Now()

	result := &ValidationResult{
		Pods:     make([]PodStatus, 0),
		Services: make([]ServiceStatus, 0),
		Context:  make(map[string]interface{}),
	}

	if namespace == "" {
		namespace = "default"
	}

	dm.logger.Info().
		Str("manifest_path", manifestPath).
		Str("namespace", namespace).
		Msg("Starting deployment validation")

	// Get pod status
	podsOutput, err := dm.clients.Kube.GetPods(ctx, namespace, "")
	if err != nil {
		result.Error = &DeploymentError{
			Type:    "kubectl_error",
			Message: fmt.Sprintf("Failed to get pod status: %v", err),
			Output:  podsOutput,
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Parse pod information
	result.Pods = dm.parsePodStatus(podsOutput)

	// Count ready pods
	for _, pod := range result.Pods {
		result.PodsTotal++
		if pod.Ready == "1/1" || strings.Contains(pod.Status, "Running") {
			result.PodsReady++
		}
	}

	// Check if all pods are ready
	result.Success = result.PodsTotal > 0 && result.PodsReady == result.PodsTotal

	result.Duration = time.Since(startTime)
	result.Context = map[string]interface{}{
		"validation_time": result.Duration.Seconds(),
		"namespace":       namespace,
		"pods_ready":      result.PodsReady,
		"pods_total":      result.PodsTotal,
	}

	dm.logger.Info().
		Int("pods_ready", result.PodsReady).
		Int("pods_total", result.PodsTotal).
		Bool("success", result.Success).
		Dur("duration", result.Duration).
		Msg("Deployment validation completed")

	return result, nil
}

// DeleteDeployment deletes a deployed manifest
func (dm *DeploymentManager) DeleteDeployment(ctx context.Context, manifestPath string) (*DeploymentResult, error) {
	startTime := time.Now()

	result := &DeploymentResult{
		ManifestPath: manifestPath,
		Resources:    make([]DeployedResource, 0),
		Context:      make(map[string]interface{}),
	}

	dm.logger.Info().Str("manifest_path", manifestPath).Msg("Starting deployment deletion")

	// Execute deletion using the existing clients
	output, err := dm.clients.Kube.DeleteDeployment(ctx, manifestPath)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {
		dm.logger.Error().Err(err).Str("output", output).Msg("Deployment deletion failed")

		result.Error = &DeploymentError{
			Type:         dm.categorizeError(err, output),
			Message:      fmt.Sprintf("Deletion failed: %v", err),
			ManifestPath: manifestPath,
			Output:       output,
		}
		return result, nil
	}

	result.Success = true
	result.Context = map[string]interface{}{
		"deletion_time": result.Duration.Seconds(),
	}

	dm.logger.Info().
		Str("manifest_path", manifestPath).
		Dur("duration", result.Duration).
		Msg("Deployment deletion completed successfully")

	return result, nil
}

// CheckClusterConnection verifies connection to the Kubernetes cluster
func (dm *DeploymentManager) CheckClusterConnection(ctx context.Context) error {
	// Try a simple kubectl command to verify cluster connection
	output, err := dm.clients.Kube.GetPods(ctx, "kube-system", "")
	if err != nil {
		return fmt.Errorf("cannot connect to Kubernetes cluster: %v (output: %s)", err, output)
	}
	return nil
}

// PreviewChanges runs kubectl diff to preview what would change
func (dm *DeploymentManager) PreviewChanges(ctx context.Context, manifestPath string, namespace string) (string, error) {
	dm.logger.Info().
		Str("manifest_path", manifestPath).
		Str("namespace", namespace).
		Msg("Running kubectl diff to preview changes")

	// kubectl diff shows what would change if the manifest were applied
	output, err := dm.executeKubectlDiff(ctx, manifestPath, namespace)

	// kubectl diff returns exit code 1 when there are differences (normal behavior)
	// Only log as error if output is empty (real failure)
	if err != nil && output == "" {
		dm.logger.Error().Err(err).Msg("kubectl diff failed")
		return "", fmt.Errorf("failed to preview changes: %w", err)
	}

	if output == "" {
		dm.logger.Info().Msg("No changes detected by kubectl diff")
	} else {
		dm.logger.Info().Msg("kubectl diff found changes")
	}

	return output, nil
}

// Helper methods

func (dm *DeploymentManager) validateDeploymentInputs(manifestPath string, options DeploymentOptions) error {
	if manifestPath == "" {
		return fmt.Errorf("manifest path is required")
	}

	// Check if manifest file exists
	if err := dm.validateManifestFile(manifestPath); err != nil {
		return err
	}

	return nil
}

func (dm *DeploymentManager) validateManifestFile(manifestPath string) error {
	info, err := os.Stat(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("manifest file not found: %s", manifestPath)
		}
		return fmt.Errorf("error accessing manifest file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("manifest path is a directory, not a file: %s", manifestPath)
	}

	// Read and validate YAML
	file, err := os.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("cannot read manifest file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading manifest content: %w", err)
	}

	// Parse YAML to ensure it's valid
	var data interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("invalid YAML in manifest file: %w", err)
	}

	// Basic Kubernetes resource validation
	if m, ok := data.(map[string]interface{}); ok {
		// Check for required fields
		if _, hasAPIVersion := m["apiVersion"]; !hasAPIVersion {
			return fmt.Errorf("manifest missing required field: apiVersion")
		}
		if _, hasKind := m["kind"]; !hasKind {
			return fmt.Errorf("manifest missing required field: kind")
		}
		if _, hasMetadata := m["metadata"]; !hasMetadata {
			return fmt.Errorf("manifest missing required field: metadata")
		}
	}

	return nil
}

func (dm *DeploymentManager) checkKubectlInstalled() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl executable not found in PATH. Please install kubectl")
	}

	// Check kubectl version
	cmd := exec.Command("kubectl", "version", "--client", "--short")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl is not functioning properly: %v", err)
	}

	return nil
}

func (dm *DeploymentManager) categorizeError(err error, output string) string {
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// Cluster connection errors
	if strings.Contains(errStr, "connection refused") || strings.Contains(outputStr, "connection refused") ||
		strings.Contains(errStr, "unable to connect") || strings.Contains(outputStr, "unable to connect") {
		return "cluster_error"
	}

	// Authentication/authorization errors
	if strings.Contains(errStr, "unauthorized") || strings.Contains(outputStr, "unauthorized") ||
		strings.Contains(errStr, "forbidden") || strings.Contains(outputStr, "forbidden") {
		return "auth_error"
	}

	// Validation errors
	if strings.Contains(errStr, "validation") || strings.Contains(outputStr, "validation") ||
		strings.Contains(errStr, "invalid") || strings.Contains(outputStr, "invalid") {
		return "validation_error"
	}

	// Timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(outputStr, "timeout") {
		return "timeout_error"
	}

	// Default to generic kubectl error
	return "kubectl_error"
}

func (dm *DeploymentManager) parseDeploymentOutput(output string) []DeployedResource {
	resources := make([]DeployedResource, 0)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "/") {
			continue
		}

		// Parse kubectl apply output (e.g., "deployment.apps/my-app created")
		if strings.Contains(line, "created") || strings.Contains(line, "configured") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				resourceParts := strings.Split(parts[0], "/")
				if len(resourceParts) == 2 {
					kindParts := strings.Split(resourceParts[0], ".")
					kind := kindParts[0]
					name := resourceParts[1]

					status := "created"
					if strings.Contains(line, "configured") {
						status = "configured"
					}

					resource := DeployedResource{
						Kind:   kind,
						Name:   name,
						Status: status,
					}

					resources = append(resources, resource)
				}
			}
		}
	}

	return resources
}

func (dm *DeploymentManager) parsePodStatus(output string) []PodStatus {
	pods := make([]PodStatus, 0)

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		pod := PodStatus{
			Name:     fields[0],
			Ready:    fields[1],
			Status:   fields[2],
			Restarts: 0, // Would need to parse this from fields[3]
			Age:      fields[4],
		}

		if len(fields) > 5 {
			pod.Node = fields[5]
		}
		if len(fields) > 6 {
			pod.IP = fields[6]
		}

		pods = append(pods, pod)
	}

	return pods
}

// executeKubectlDiff runs kubectl diff to preview deployment changes
func (dm *DeploymentManager) executeKubectlDiff(ctx context.Context, manifestPath, namespace string) (string, error) {
	args := []string{"diff", "-f", manifestPath}
	if namespace != "" && namespace != "default" {
		args = append(args, "-n", namespace)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
