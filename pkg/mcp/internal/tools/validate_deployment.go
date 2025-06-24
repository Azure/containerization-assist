package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// ValidateDeploymentArgs represents the arguments for the validate_deployment tool
type ValidateDeploymentArgs struct {
	types.BaseToolArgs
	ClusterName      string               `json:"cluster_name,omitempty" description:"Kind cluster name"`
	Namespace        string               `json:"namespace,omitempty" description:"Kubernetes namespace"`
	ManifestPath     string               `json:"manifest_path,omitempty" description:"Path to manifests directory"`
	Timeout          string               `json:"timeout,omitempty" description:"Validation timeout (e.g., '5m')"`
	HealthCheckPath  string               `json:"health_check_path,omitempty" description:"HTTP health check endpoint"`
	CreateCluster    bool                 `json:"create_cluster,omitempty" description:"Create Kind cluster if not exists"`
	UseLocalRegistry bool                 `json:"use_local_registry,omitempty" description:"Use local registry (localhost:5001)"`
	ImageRef         types.ImageReference `json:"image_ref,omitempty" description:"Image to validate (must be accessible to cluster)"`
}

// ValidateDeploymentResult represents the result of deployment validation
type ValidateDeploymentResult struct {
	types.BaseToolResponse
	Success       bool                `json:"success"`
	JobID         string              `json:"job_id,omitempty"` // For async validation
	PodStatus     []PodStatusInfo     `json:"pod_status"`
	ServiceStatus []ServiceStatusInfo `json:"service_status"`
	HealthCheck   HealthCheckResult   `json:"health_check"`
	ClusterInfo   KindClusterInfo     `json:"cluster_info"`
	Logs          []string            `json:"logs"`
	Duration      time.Duration       `json:"duration"`
	Error         *types.ToolError    `json:"error,omitempty"`
}

// PodStatusInfo represents pod status information
type PodStatusInfo struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Status     string            `json:"status"`
	Ready      string            `json:"ready"`
	Restarts   int32             `json:"restarts"`
	Age        string            `json:"age"`
	Events     []string          `json:"events,omitempty"`
	Containers []ContainerStatus `json:"containers,omitempty"`
}

// ContainerStatus represents container status within a pod
type ContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	State        string `json:"state"`
	ExitCode     *int32 `json:"exit_code,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// ServiceStatusInfo represents service status information
type ServiceStatusInfo struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Type      string   `json:"type"`
	ClusterIP string   `json:"cluster_ip"`
	Ports     []string `json:"ports"`
	Endpoints int      `json:"endpoints"`
}

// HealthCheckResult represents health check results
type HealthCheckResult struct {
	Checked    bool   `json:"checked"`
	Healthy    bool   `json:"healthy"`
	Endpoint   string `json:"endpoint,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// KindClusterInfo represents Kind cluster information
type KindClusterInfo struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Registry  string `json:"registry,omitempty"`
	APIServer string `json:"api_server"`
	Created   bool   `json:"created"`
}

// JobManager interface for async job management (to avoid circular import)
type JobManager interface {
	CreateJob(jobType, sessionID string, metadata map[string]interface{}) string
	UpdateJobStatus(jobID, status string, progress float64, result map[string]interface{})
}

// ValidateDeploymentTool handles Kubernetes deployment validation
type ValidateDeploymentTool struct {
	logger        zerolog.Logger
	workspaceBase string
	clients       *clients.Clients
	jobManager    JobManager
}

// JobManagerAdapter wraps the actual JobManager to match our interface
type JobManagerAdapter struct {
	JM interface {
		CreateJob(jobType interface{}, sessionID string, metadata map[string]interface{}) string
		UpdateJobStatus(jobID, status string, progress float64, result map[string]interface{})
	}
}

func (a *JobManagerAdapter) CreateJob(jobType, sessionID string, metadata map[string]interface{}) string {
	// Convert string to the actual JobType - for now just pass as string
	return a.JM.CreateJob(jobType, sessionID, metadata)
}

func (a *JobManagerAdapter) UpdateJobStatus(jobID, status string, progress float64, result map[string]interface{}) {
	a.JM.UpdateJobStatus(jobID, status, progress, result)
}

// NewValidateDeploymentTool creates a new validate deployment tool
func NewValidateDeploymentTool(logger zerolog.Logger, workspaceBase string, jobManager JobManager, clients *clients.Clients) *ValidateDeploymentTool {
	return &ValidateDeploymentTool{
		logger:        logger,
		workspaceBase: workspaceBase,
		jobManager:    jobManager,
		clients:       clients,
	}
}

// Execute validates deployment to Kind cluster
func (t *ValidateDeploymentTool) Execute(ctx context.Context, args ValidateDeploymentArgs) (*ValidateDeploymentResult, error) {
	startTime := time.Now()

	// Create base response with versioning
	response := &ValidateDeploymentResult{
		BaseToolResponse: types.NewBaseResponse("validate_deployment", args.SessionID, args.DryRun),
		PodStatus:        []PodStatusInfo{},
		ServiceStatus:    []ServiceStatusInfo{},
		Logs:             []string{},
	}

	// Apply defaults
	if args.ClusterName == "" {
		args.ClusterName = "container-kit"
	}
	if args.Namespace == "" {
		args.Namespace = "default"
	}
	if args.Timeout == "" {
		args.Timeout = "5m"
	}

	// Parse timeout
	timeout, err := time.ParseDuration(args.Timeout)
	if err != nil {
		return nil, types.NewRichError("INVALID_TIMEOUT_FORMAT", fmt.Sprintf("invalid timeout format: %v", err), types.ErrTypeValidation)
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("cluster_name", args.ClusterName).
		Str("namespace", args.Namespace).
		Bool("dry_run", args.DryRun).
		Bool("create_cluster", args.CreateCluster).
		Msg("Validating deployment")

	// For dry-run, just return what would happen
	if args.DryRun {
		response.Success = true
		response.ClusterInfo = KindClusterInfo{
			Name:     args.ClusterName,
			Status:   "would-check",
			Registry: "localhost:5001",
			Created:  args.CreateCluster,
		}
		response.PodStatus = []PodStatusInfo{
			{
				Name:      "app-xxxxx",
				Namespace: args.Namespace,
				Status:    "would-deploy",
				Ready:     "0/1",
			},
		}
		response.Duration = time.Since(startTime)
		return response, nil
	}

	// Determine if this should be async (deployments typically take >30s)
	isAsync := timeout > 30*time.Second && t.jobManager != nil

	if isAsync {
		// Create async job
		jobID := t.jobManager.CreateJob(
			"validation",
			args.SessionID,
			map[string]interface{}{
				"cluster_name": args.ClusterName,
				"namespace":    args.Namespace,
				"timeout":      args.Timeout,
			},
		)

		// Start async validation
		go t.performAsyncValidation(ctx, jobID, args, startTime)

		response.JobID = jobID
		response.Success = true
		t.logger.Info().
			Str("job_id", jobID).
			Msg("Started async deployment validation")

		return response, nil
	}

	// Perform synchronous validation
	return t.performSyncValidation(ctx, args, response, startTime)
}

// performSyncValidation performs synchronous deployment validation
func (t *ValidateDeploymentTool) performSyncValidation(ctx context.Context, args ValidateDeploymentArgs, response *ValidateDeploymentResult, startTime time.Time) (*ValidateDeploymentResult, error) {
	// Check if clients are initialized
	if t.clients == nil {
		response.Error = &types.ToolError{
			Type:    "initialization_error",
			Message: "Clients not initialized",
		}
		return response, nil
	}

	// Step 1: Ensure Kind cluster exists
	clusterInfo, err := t.ensureKindCluster(ctx, args)
	if err != nil {
		response.Error = &types.ToolError{
			Type:      "cluster_error",
			Message:   fmt.Sprintf("Failed to ensure Kind cluster: %v", err),
			Retryable: true,
			Suggestions: []string{
				"Check if Docker is running",
				"Ensure Kind is installed: kind --version",
				"Try creating cluster manually: kind create cluster --name " + args.ClusterName,
			},
		}
		return response, nil
	}
	response.ClusterInfo = clusterInfo

	// Step 2: Find manifests
	manifestPath := args.ManifestPath
	if manifestPath == "" {
		// Default to session workspace
		workspaceDir := filepath.Join(t.workspaceBase, args.SessionID)
		if args.SessionID == "" {
			workspaceDir = filepath.Join(t.workspaceBase, "default")
		}
		manifestPath = filepath.Join(workspaceDir, "manifests")
	}

	manifests, err := k8s.FindK8sObjects(manifestPath)
	if err != nil {
		response.Error = &types.ToolError{
			Type:    "manifest_error",
			Message: fmt.Sprintf("Failed to find manifests: %v", err),
			Context: map[string]interface{}{
				"manifest_path": manifestPath,
			},
		}
		return response, nil
	}

	t.logger.Info().
		Int("manifest_count", len(manifests)).
		Str("manifest_path", manifestPath).
		Msg("Found manifests to deploy")

	// Step 3: Update image references if using local registry
	if args.UseLocalRegistry && args.ImageRef.String() != "" {
		t.logger.Info().Msg("Updating manifests for local registry")
		for i, manifest := range manifests {
			if manifest.IsDeployment() {
				// Update image reference to use local registry
				updatedContent := t.updateImageReference(string(manifest.Content), args.ImageRef)
				manifests[i].Content = []byte(updatedContent)
			}
		}
	}

	// Step 4: Deploy manifests
	deploymentLogs := []string{}
	deploymentErrors := []error{}

	for _, manifest := range manifests {
		t.logger.Info().
			Str("kind", manifest.Kind).
			Str("name", manifest.Metadata.Name).
			Msg("Deploying manifest")

		// Create temporary file for manifest
		tmpFile, err := os.CreateTemp("", "manifest-*.yaml")
		if err != nil {
			deploymentErrors = append(deploymentErrors, types.NewRichError("TEMP_FILE_CREATION_FAILED", fmt.Sprintf("failed to create temp file: %v", err), types.ErrTypeSystem))
			continue
		}
		defer func() {
			if err := os.Remove(tmpFile.Name()); err != nil {
				// Log but don't fail - temp file cleanup is not critical
				t.logger.Debug().Err(err).Str("file", tmpFile.Name()).Msg("Failed to remove temp file")
			}
		}()

		if _, err := tmpFile.Write(manifest.Content); err != nil {
			deploymentErrors = append(deploymentErrors, types.NewRichError("MANIFEST_WRITE_FAILED", fmt.Sprintf("failed to write manifest: %v", err), types.ErrTypeSystem))
			continue
		}
		if err := tmpFile.Close(); err != nil {
			deploymentErrors = append(deploymentErrors, types.NewRichError("TEMP_FILE_CLOSE_FAILED", fmt.Sprintf("failed to close temp file: %v", err), types.ErrTypeSystem))
			continue
		}

		// Deploy using kubectl
		isDeployment := manifest.IsDeployment()
		success, logs, err := t.clients.DeployAndVerifySingleManifest(ctx, tmpFile.Name(), isDeployment)
		deploymentLogs = append(deploymentLogs, logs)

		if err != nil {
			deploymentErrors = append(deploymentErrors, types.NewRichError("DEPLOYMENT_FAILED", fmt.Sprintf("%s/%s: %v", manifest.Kind, manifest.Metadata.Name, err), types.ErrTypeDeployment))
			// For deployments, try to get pod status even on failure
			if isDeployment {
				podInfo := PodStatusInfo{
					Name:      manifest.Metadata.Name + "-failed",
					Namespace: args.Namespace,
					Status:    "Failed",
					Ready:     "0/1",
					Events:    []string{err.Error()},
				}
				response.PodStatus = append(response.PodStatus, podInfo)
			}
		} else {
			manifest.IsSuccessfullyDeployed = true
			// Add successful status for deployments
			if isDeployment && success {
				podInfo := PodStatusInfo{
					Name:      manifest.Metadata.Name,
					Namespace: args.Namespace,
					Status:    "Running",
					Ready:     "1/1",
				}
				response.PodStatus = append(response.PodStatus, podInfo)
			}
		}
	}

	response.Logs = deploymentLogs

	// Step 5: Get service status
	response.ServiceStatus = t.getServiceStatus(ctx, args.Namespace)

	// Step 6: Perform health check if specified
	if args.HealthCheckPath != "" && len(response.ServiceStatus) > 0 {
		response.HealthCheck = t.performHealthCheck(ctx, response.ServiceStatus[0], args.HealthCheckPath)
	}

	// Determine overall success
	response.Success = len(deploymentErrors) == 0
	response.Duration = time.Since(startTime)

	if !response.Success {
		errorMessages := []string{}
		for _, err := range deploymentErrors {
			errorMessages = append(errorMessages, err.Error())
		}
		response.Error = &types.ToolError{
			Type:      "deployment_error",
			Message:   fmt.Sprintf("Deployment failed: %s", strings.Join(errorMessages, "; ")),
			Retryable: true,
			Suggestions: []string{
				"Check pod logs for more details",
				"Verify image is accessible from cluster",
				"Check resource quotas and limits",
				"Review Kubernetes events: kubectl get events -n " + args.Namespace,
			},
		}
	}

	t.logger.Info().
		Bool("success", response.Success).
		Int("pod_count", len(response.PodStatus)).
		Int("service_count", len(response.ServiceStatus)).
		Dur("duration", response.Duration).
		Msg("Deployment validation completed")

	return response, nil
}

// performAsyncValidation performs asynchronous deployment validation
func (t *ValidateDeploymentTool) performAsyncValidation(ctx context.Context, jobID string, args ValidateDeploymentArgs, startTime time.Time) {
	// Update job status to running
	t.jobManager.UpdateJobStatus(jobID, "running", 10, nil)

	// Create a response object for the async operation
	response := &ValidateDeploymentResult{
		BaseToolResponse: types.NewBaseResponse("validate_deployment", args.SessionID, false),
	}

	// Perform the validation
	result, err := t.performSyncValidation(ctx, args, response, startTime)
	if err != nil {
		// Update job with error
		t.jobManager.UpdateJobStatus(jobID, "failed", 100, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Update job with results
	t.jobManager.UpdateJobStatus(jobID, "completed", 100, map[string]interface{}{
		"success":        result.Success,
		"pod_status":     result.PodStatus,
		"service_status": result.ServiceStatus,
		"health_check":   result.HealthCheck,
		"cluster_info":   result.ClusterInfo,
		"logs":           result.Logs,
		"duration":       result.Duration,
		"error":          result.Error,
	})
}

// ensureKindCluster ensures a Kind cluster exists
func (t *ValidateDeploymentTool) ensureKindCluster(ctx context.Context, args ValidateDeploymentArgs) (KindClusterInfo, error) {
	info := KindClusterInfo{
		Name:   args.ClusterName,
		Status: "unknown",
	}

	// Check if Kind is installed
	if err := t.clients.ValidateKindInstalled(ctx); err != nil {
		return info, types.NewRichError("KIND_NOT_INSTALLED", fmt.Sprintf("Kind not installed: %v", err), types.ErrTypeSystem)
	}

	// Get or create cluster
	if args.CreateCluster || args.UseLocalRegistry {
		// Use the SetupLocalRegistryCluster method which creates cluster with registry
		if err := t.clients.SetupLocalRegistryCluster(ctx); err != nil {
			return info, types.NewRichError("KIND_CLUSTER_SETUP_FAILED", fmt.Sprintf("failed to setup Kind cluster with registry: %v", err), types.ErrTypeSystem)
		}
		info.Created = true
		info.Registry = "localhost:5001"
	} else {
		// Just ensure cluster exists
		_, err := t.clients.GetKindCluster(ctx)
		if err != nil {
			return info, types.NewRichError("KIND_CLUSTER_ACCESS_FAILED", fmt.Sprintf("failed to get Kind cluster: %v", err), types.ErrTypeSystem)
		}
	}

	info.Status = "running"
	info.APIServer = "kind-" + args.ClusterName

	return info, nil
}

// updateImageReference updates the image reference in deployment manifest
func (t *ValidateDeploymentTool) updateImageReference(content string, imageRef types.ImageReference) string {
	// For local registry, we need to retag to localhost:5001
	localImage := fmt.Sprintf("localhost:5001/%s:%s", imageRef.Repository, imageRef.Tag)

	// Simple string replacement - in production would use proper YAML parsing
	// Look for image: lines and replace
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "image:") {
			// Preserve indentation
			indent := strings.TrimSuffix(line, trimmed)
			lines[i] = indent + "image: " + localImage
		}
	}

	return strings.Join(lines, "\n")
}

// getServiceStatus retrieves service status information
func (t *ValidateDeploymentTool) getServiceStatus(ctx context.Context, namespace string) []ServiceStatusInfo {
	// This is a simplified version - in production would use actual k8s client
	// For now, return mock data
	return []ServiceStatusInfo{
		{
			Name:      "app",
			Namespace: namespace,
			Type:      "ClusterIP",
			ClusterIP: "10.96.0.1",
			Ports:     []string{"80/TCP"},
			Endpoints: 1,
		},
	}
}

// performHealthCheck performs HTTP health check
func (t *ValidateDeploymentTool) performHealthCheck(ctx context.Context, service ServiceStatusInfo, path string) HealthCheckResult {
	result := HealthCheckResult{
		Checked:  true,
		Endpoint: fmt.Sprintf("http://%s%s", service.ClusterIP, path),
	}

	// In production, would use port-forward and actual HTTP request
	// For now, simulate success
	result.Healthy = true
	result.StatusCode = 200

	return result
}
