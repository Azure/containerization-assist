package kubernetes

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"log/slog"

	mcperrors "github.com/Azure/containerization-assist/pkg/domain/errors"
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

// Service provides a unified interface to Kubernetes deployment operations
type Service interface {
	// Deployment operations
	DeployManifest(ctx context.Context, manifestPath string, options DeploymentOptions) (*DeploymentResult, error)
	ValidateDeployment(ctx context.Context, manifestPath string, namespace string) (*ValidationResult, error)
	DeleteDeployment(ctx context.Context, manifestPath string) (*DeploymentResult, error)

	// Cluster operations
	CheckClusterConnection(ctx context.Context) error
	PreviewChanges(ctx context.Context, manifestPath string, namespace string) (string, error)
}

// deploymentService implements the deployment Service interface
type deploymentService struct {
	kube   KubeRunner
	logger *slog.Logger
}

// NewService creates a new deployment service
func NewService(kube KubeRunner, logger *slog.Logger) Service {
	return &deploymentService{
		kube:   kube,
		logger: logger,
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

// Service methods implementing the Service interface

// DeployManifest deploys a Kubernetes manifest file
func (s *deploymentService) DeployManifest(ctx context.Context, manifestPath string, options DeploymentOptions) (*DeploymentResult, error) {
	startTime := time.Now()

	result := &DeploymentResult{
		ManifestPath: manifestPath,
		Namespace:    options.Namespace,
		Resources:    make([]DeployedResource, 0),
		Context:      make(map[string]interface{}),
	}

	// Validate inputs
	if err := s.validateDeploymentInputs(manifestPath, options); err != nil {
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
	if err := s.checkKubectlInstalled(); err != nil {
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
	output, err := s.kube.Apply(deployCtx, manifestPath)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {

		result.Error = &DeploymentError{
			Type:         s.categorizeError(err, output),
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
	result.Resources = s.parseDeploymentOutput(output)

	// If validation is requested, validate the deployment
	if options.Validate {
		validationResult, err := s.ValidateDeployment(ctx, manifestPath, options.Namespace)
		if err != nil {
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

	return result, nil
}

// ValidateDeployment validates that deployed resources are healthy
func (s *deploymentService) ValidateDeployment(ctx context.Context, manifestPath string, namespace string) (*ValidationResult, error) {
	startTime := time.Now()

	result := &ValidationResult{
		Pods:     make([]PodStatus, 0),
		Services: make([]ServiceStatus, 0),
		Context:  make(map[string]interface{}),
	}

	if namespace == "" {
		namespace = "default"
	}

	// Get pod status
	podsOutput, err := s.kube.GetPods(ctx, namespace, "")
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
	result.Pods = s.parsePodStatus(podsOutput)

	// Count ready pods
	for _, pod := range result.Pods {
		result.PodsTotal++
		if pod.Ready == "1/1" || strings.Contains(pod.Status, "Running") {
			result.PodsReady++
		}
	}

	// Check if all pods are ready
	result.Success = result.PodsTotal > 0 && result.PodsReady == result.PodsTotal

	// If validation fails, gather additional diagnostic information
	if !result.Success {

		// Collect detailed diagnostics for failed/pending pods
		diagnosticInfo := make([]string, 0)

		// Get events for the namespace (most critical information)
		if eventsOutput, err := s.kube.GetEvents(ctx, namespace); err == nil {
			filteredEvents := s.filterRelevantEvents(eventsOutput)
			if filteredEvents != "" {
				diagnosticInfo = append(diagnosticInfo, "Critical Events:")
				diagnosticInfo = append(diagnosticInfo, filteredEvents)
			}
		} else {
		}

		// Get essential pod information for non-ready pods
		for _, pod := range result.Pods {
			if pod.Ready != "1/1" && !strings.Contains(pod.Status, "Running") {
				if describeOutput, err := s.kube.DescribePod(ctx, pod.Name, namespace); err == nil {
					essentialInfo := s.extractEssentialPodInfo(describeOutput, pod.Name)
					if essentialInfo != "" {
						diagnosticInfo = append(diagnosticInfo, fmt.Sprintf("\nPod Issues for %s:", pod.Name))
						diagnosticInfo = append(diagnosticInfo, essentialInfo)
					}
				} else {
				}
			}
		}

		// Store diagnostic information in error context
		if len(diagnosticInfo) > 0 {
			result.Error = &DeploymentError{
				Type:    "validation_error",
				Message: fmt.Sprintf("Deployment validation failed: %d/%d pods ready", result.PodsReady, result.PodsTotal),
				Output:  strings.Join(diagnosticInfo, "\n\n"),
				Context: map[string]interface{}{
					"diagnostic_info_collected": true,
					"events_checked":            true,
					"pods_described":            len(diagnosticInfo) > 1, // More than just events
				},
			}
		}
	}

	result.Duration = time.Since(startTime)
	result.Context = map[string]interface{}{
		"validation_time": result.Duration.Seconds(),
		"namespace":       namespace,
		"pods_ready":      result.PodsReady,
		"pods_total":      result.PodsTotal,
	}

	return result, nil
}

// DeleteDeployment deletes a deployed manifest
func (s *deploymentService) DeleteDeployment(ctx context.Context, manifestPath string) (*DeploymentResult, error) {
	startTime := time.Now()

	result := &DeploymentResult{
		ManifestPath: manifestPath,
		Resources:    make([]DeployedResource, 0),
		Context:      make(map[string]interface{}),
	}

	// Execute deletion using the existing clients
	output, err := s.kube.DeleteDeployment(ctx, manifestPath)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {

		result.Error = &DeploymentError{
			Type:         s.categorizeError(err, output),
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

	return result, nil
}

// CheckClusterConnection verifies connection to the Kubernetes cluster
func (s *deploymentService) CheckClusterConnection(ctx context.Context) error {
	// Try a simple kubectl command to verify cluster connection
	output, err := s.kube.GetPods(ctx, "kube-system", "")
	if err != nil {
		return mcperrors.New(mcperrors.CodeKubernetesApiError, "core", fmt.Sprintf("cannot connect to Kubernetes cluster: %v (output: %s)", err, output), err)
	}
	return nil
}

// PreviewChanges runs kubectl diff to preview what would change
func (s *deploymentService) PreviewChanges(ctx context.Context, manifestPath string, namespace string) (string, error) {

	// kubectl diff shows what would change if the manifest were applied
	output, err := s.executeKubectlDiff(ctx, manifestPath, namespace)

	// kubectl diff returns exit code 1 when there are differences (normal behavior)
	// Only log as error if output is empty (real failure)
	if err != nil && output == "" {
		return "", mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to preview changes", err)
	}

	if output == "" {
	} else {
	}

	return output, nil
}

// Service helper methods

func (s *deploymentService) validateDeploymentInputs(manifestPath string, _ DeploymentOptions) error {
	if manifestPath == "" {
		return fmt.Errorf("manifest path is required")
	}

	// Check if manifest file exists
	if err := s.validateManifestFile(manifestPath); err != nil {
		return err
	}

	return nil
}

func (s *deploymentService) validateManifestFile(manifestPath string) error {
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
	defer func() {
		if closeErr := file.Close(); closeErr != nil && s.logger != nil {
			s.logger.Warn("failed to close manifest file", "error", closeErr)
		}
	}()

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

func (s *deploymentService) checkKubectlInstalled() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl executable not found in PATH. Please install kubectl")
	}

	// Check kubectl version (--short flag was deprecated and removed in newer versions)
	cmd := exec.Command("kubectl", "version", "--client", "-o", "json")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl is not functioning properly: %v", err)
	}

	return nil
}

func (s *deploymentService) categorizeError(err error, output string) string {
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

func (s *deploymentService) parseDeploymentOutput(output string) []DeployedResource {
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

func (s *deploymentService) parsePodStatus(output string) []PodStatus {
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
func (s *deploymentService) executeKubectlDiff(ctx context.Context, manifestPath, namespace string) (string, error) {
	args := []string{"diff", "-f", manifestPath}
	if namespace != "" && namespace != "default" {
		args = append(args, "-n", namespace)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// filterRelevantEvents filters events to show only critical debugging information
func (s *deploymentService) filterRelevantEvents(eventsOutput string) string {
	var relevantEvents []string
	lines := strings.Split(eventsOutput, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and headers
		if line == "" || strings.Contains(line, "LAST SEEN") {
			continue
		}

		// Include lines that indicate problems
		if strings.Contains(line, "Failed") ||
			strings.Contains(line, "Error") ||
			strings.Contains(line, "Warning") ||
			strings.Contains(line, "BackOff") ||
			strings.Contains(line, "Unhealthy") ||
			strings.Contains(line, "ImagePull") ||
			strings.Contains(line, "Created container") ||
			strings.Contains(line, "Started container") ||
			strings.Contains(line, "Pulled") {
			relevantEvents = append(relevantEvents, "  "+line)
		}
	}

	return strings.Join(relevantEvents, "\n")
}

// extractEssentialPodInfo extracts only the most critical information from kubectl describe pod output
func (s *deploymentService) extractEssentialPodInfo(describeOutput, podName string) string {
	var essential []string
	lines := strings.Split(describeOutput, "\n")

	var inContainerSection, inConditionsSection, inEventsSection bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Track sections
		if strings.HasPrefix(line, "Containers:") {
			inContainerSection = true
			inConditionsSection = false
			inEventsSection = false
			continue
		} else if strings.HasPrefix(line, "Conditions:") {
			inContainerSection = false
			inConditionsSection = true
			inEventsSection = false
			continue
		} else if strings.HasPrefix(line, "Events:") {
			inContainerSection = false
			inConditionsSection = false
			inEventsSection = true
			continue
		} else if strings.HasPrefix(line, "Volumes:") ||
			strings.HasPrefix(line, "QoS Class:") ||
			strings.HasPrefix(line, "Node-Selectors:") ||
			strings.HasPrefix(line, "Tolerations:") {
			inContainerSection = false
			inConditionsSection = false
			inEventsSection = false
			continue
		}

		// Extract key information
		if strings.Contains(line, "Status:") ||
			strings.Contains(line, "Reason:") ||
			strings.Contains(line, "Message:") {
			essential = append(essential, "  "+line)
		}

		// Container state information
		if inContainerSection && (strings.Contains(line, "State:") ||
			strings.Contains(line, "Last State:") ||
			strings.Contains(line, "Ready:") ||
			strings.Contains(line, "Restart Count:") ||
			strings.Contains(line, "Image:") ||
			strings.Contains(line, "Reason:") ||
			strings.Contains(line, "Exit Code:") ||
			strings.Contains(line, "Started:") ||
			strings.Contains(line, "Finished:")) {
			essential = append(essential, "  "+line)
		}

		// Condition information (Ready, Scheduled, etc.)
		if inConditionsSection && !strings.HasPrefix(line, " ") && line != "" {
			essential = append(essential, "  "+line)
		}

		// Recent events (last 5-10 lines of events)
		if inEventsSection && (strings.Contains(line, "Failed") ||
			strings.Contains(line, "Error") ||
			strings.Contains(line, "Warning") ||
			strings.Contains(line, "BackOff") ||
			strings.Contains(line, "ImagePull") ||
			strings.Contains(line, "Started") ||
			strings.Contains(line, "Created")) {
			essential = append(essential, "  "+line)
		}
	}

	if len(essential) == 0 {
		return ""
	}

	return strings.Join(essential, "\n")
}
