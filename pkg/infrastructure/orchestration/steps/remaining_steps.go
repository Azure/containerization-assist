// Package steps contains placeholder implementations for remaining workflow steps.
package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/infrastructure/container"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core/runner"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core/utils"
	"github.com/Azure/containerization-assist/pkg/infrastructure/kubernetes"
	"github.com/rs/zerolog"
)

func init() {
	// Register all steps in this file
	Register(NewBuildStep())
	Register(NewScanStep())
	Register(NewTagStep())
	Register(NewPushStep())
	Register(NewManifestStep())
	Register(NewClusterStep())
	Register(NewDeployStep())
	Register(NewVerifyStep())
}

// Use utils.ExtractRepoName instead of local function

// captureDeploymentDiagnostics captures deployment diagnostics and stores them in K8s result metadata
func captureDeploymentDiagnostics(ctx context.Context, state *workflow.WorkflowState, infraK8sResult *K8sResult, logger *slog.Logger) {
	if diagnostics, diagErr := VerifyDeploymentWithDiagnostics(ctx, infraK8sResult, logger); diagErr == nil {
		// Store deployment diagnostics in K8s result metadata for access in response
		if state.K8sResult.Metadata == nil {
			state.K8sResult.Metadata = make(map[string]interface{})
		}
		state.K8sResult.Metadata["deployment_diagnostics"] = map[string]interface{}{
			"deployment_ok":  diagnostics.DeploymentOK,
			"pods_ready":     diagnostics.PodsReady,
			"pods_total":     diagnostics.PodsTotal,
			"pod_statuses":   diagnostics.PodStatuses,
			"services":       diagnostics.Services,
			"recent_events":  diagnostics.Events,
			"pod_logs":       diagnostics.Logs,
			"resource_usage": diagnostics.ResourceUsage,
			"errors":         diagnostics.Errors,
			"warnings":       diagnostics.Warnings,
			"timestamp":      diagnostics.Timestamp,
		}
	}
}

// createZerologFromSlog creates a zerolog logger from an slog logger
// This is a bridge function to use existing security scanners that expect zerolog
func createZerologFromSlog(slogLogger *slog.Logger) zerolog.Logger {
	// Create a zerolog logger that uses the same underlying writer as the slog logger
	// Use os.Stderr as the default output to match typical slog behavior
	writer := zerolog.ConsoleWriter{Out: os.Stderr}
	return zerolog.New(writer).
		With().
		Timestamp().
		Str("component", "security_scanner").
		Logger()
}

// BuildStep implements Docker image building
type BuildStep struct{}

func NewBuildStep() workflow.Step    { return &BuildStep{} }
func (s *BuildStep) Name() string    { return "build_image" }
func (s *BuildStep) MaxRetries() int { return 3 }
func (s *BuildStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	if state.DockerfileResult == nil || state.AnalyzeResult == nil {
		return nil, errors.New(errors.CodeInvalidState, "build_step", "dockerfile and analyze results are required for build", nil)
	}

	state.Logger.Info("Step 3: Building Docker image")

	// Convert workflow types to infrastructure types for compatibility
	infraDockerfileResult := &DockerfileResult{
		Content:     state.DockerfileResult.Content,
		Path:        state.DockerfileResult.Path,
		BaseImage:   state.DockerfileResult.BaseImage,
		ExposedPort: state.DockerfileResult.ExposedPort,
	}

	// Generate image name and tag from cached repo identifier
	imageName := utils.ExtractRepoName(state.RepoIdentifier)
	imageTag := extractAndValidateTag(state, state.Logger)

	buildContext := state.AnalyzeResult.RepoPath

	// In test mode, skip actual Docker operations
	if state.IsTestMode() {
		state.Logger.Info("Test mode: Simulating Docker build")

		// Create simulated build result
		buildResult := &BuildResult{
			ImageName: imageName,
			ImageTag:  imageTag,
			ImageID:   fmt.Sprintf("sha256:test-%s", imageName),
			BuildTime: time.Now(),
			Size:      100 * 1024 * 1024, // 100MB simulated size
		}

		// Store build result in workflow state
		state.BuildResult = &workflow.BuildResult{
			ImageID:   buildResult.ImageID,
			ImageRef:  fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag),
			ImageSize: buildResult.Size,
			BuildTime: buildResult.BuildTime.Format(time.RFC3339),
			Metadata: map[string]interface{}{
				"build_context": buildContext,
				"image_name":    buildResult.ImageName,
				"image_tag":     buildResult.ImageTag,
				"test_mode":     true,
			},
		}

		state.Logger.Info("Test mode: Docker build simulation completed")

		// Return basic success result
		return &workflow.StepResult{Success: true}, nil
	}

	// Call the infrastructure build function
	buildResult, err := BuildImage(ctx, infraDockerfileResult, imageName, imageTag, buildContext, state.Logger)
	if err != nil {
		return nil, errors.New(errors.CodeImageBuildFailed, "build_step", err.Error(), err)
	}

	if buildResult == nil {
		return nil, errors.New(errors.CodeInternalError, "build_step", "build result is nil after successful build", nil)
	}

	// Store build result in workflow state
	state.BuildResult = &workflow.BuildResult{
		ImageID:   buildResult.ImageID,
		ImageRef:  fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag),
		ImageSize: buildResult.Size,
		BuildTime: buildResult.BuildTime.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"build_context": buildContext,
			"image_name":    buildResult.ImageName,
			"image_tag":     buildResult.ImageTag,
		},
	}

	state.Logger.Info("Step completed")

	// Return StepResult with build data
	return &workflow.StepResult{
		Success: true,
		Data: map[string]interface{}{
			"image_id":   buildResult.ImageID,
			"image_ref":  fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag),
			"image_size": formatBytes(buildResult.Size),
			"build_time": buildResult.BuildTime.Format(time.RFC3339),
		},
		Metadata: map[string]interface{}{
			"image_name": buildResult.ImageName,
			"image_tag":  buildResult.ImageTag,
		},
	}, nil
}

// ScanStep implements security scanning
type ScanStep struct{}

func NewScanStep() workflow.Step    { return &ScanStep{} }
func (s *ScanStep) Name() string    { return "security_scan" }
func (s *ScanStep) MaxRetries() int { return 2 }
func (s *ScanStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	// Skip if scanning is not requested
	if !state.Args.Scan {
		state.Logger.Info("Step 4: Skipping security scan (not requested)")
		// Return basic success result

		return &workflow.StepResult{Success: true}, nil
	}

	if state.BuildResult == nil {
		return nil, errors.New(errors.CodeInvalidState, "scan_step", "build result is required for security scan", nil)
	}

	state.Logger.Info("Step 4: Running security vulnerability scan")

	// Implement actual vulnerability scanning using UnifiedSecurityScanner
	imageRef := state.BuildResult.ImageRef
	if imageRef == "" {
		return nil, errors.New(errors.CodeInvalidState, "scan_step", "no image reference available for security scan", nil)
	}

	// Create zerolog logger from slog logger for the scanner
	zerologLogger := createZerologFromSlog(state.Logger)

	// Initialize the unified security scanner
	scanner := container.NewUnifiedSecurityScanner(zerologLogger)

	// Run the security scan with medium severity threshold
	scanResult, err := scanner.ScanImage(ctx, imageRef, "medium")
	if err != nil {
		state.Logger.Error("Security scan failed", "error", err, "image", imageRef)
		// Store scan error information
		state.ScanReport = map[string]interface{}{
			"scanner":   "unified",
			"scan_time": time.Now().Format(time.RFC3339),
			"status":    "error",
			"error":     err.Error(),
			"image_ref": imageRef,
		}
		// In strict mode, fail the workflow for scan errors
		if state.Args.StrictMode {
			return nil, errors.New(errors.CodeVulnerabilityFound, "scan_step", "security scan failed in strict mode", err)
		}
		state.Logger.Warn("Continuing workflow despite scan failure")
		// Return basic success result

		return &workflow.StepResult{Success: true}, nil
	}

	// Process scan results
	vulnerabilityCount := 0
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0

	// Extract vulnerability counts from the unified scan result
	if scanResult.TrivyResult != nil && scanResult.TrivyResult.Vulnerabilities != nil {
		for _, vuln := range scanResult.TrivyResult.Vulnerabilities {
			vulnerabilityCount++
			switch vuln.Severity {
			case "CRITICAL":
				criticalCount++
			case "HIGH":
				highCount++
			case "MEDIUM":
				mediumCount++
			case "LOW":
				lowCount++
			}
		}
	}

	// Determine scan status
	scanStatus := "clean"
	if criticalCount > 0 {
		scanStatus = "critical"
	} else if highCount > 0 {
		scanStatus = "high"
	} else if mediumCount > 0 {
		scanStatus = "medium"
	} else if vulnerabilityCount > 0 {
		scanStatus = "low"
	}

	// Create comprehensive scan report
	state.ScanReport = map[string]interface{}{
		"scanner":         "unified",
		"scan_time":       scanResult.ScanTime.Format(time.RFC3339),
		"duration":        scanResult.Duration.String(),
		"image_ref":       imageRef,
		"vulnerabilities": vulnerabilityCount,
		"critical_vulns":  criticalCount,
		"high_vulns":      highCount,
		"medium_vulns":    mediumCount,
		"low_vulns":       lowCount,
		"status":          scanStatus,
		"trivy_enabled":   scanResult.TrivyResult != nil,
		"grype_enabled":   scanResult.GrypeResult != nil,
		"success":         scanResult.Success,
	}

	// Add remediation information if available
	if len(scanResult.Remediation) > 0 {
		remediationSteps := make([]map[string]interface{}, len(scanResult.Remediation))
		for i, step := range scanResult.Remediation {
			remediationSteps[i] = map[string]interface{}{
				"action":      step.Action,
				"description": step.Description,
				"priority":    step.Priority,
			}
		}
		state.ScanReport["remediation"] = remediationSteps
	}

	state.Logger.Info("Step completed")

	// Return StepResult with scan data
	return &workflow.StepResult{
		Success: true,
		Data: map[string]interface{}{
			"scanner":         "unified",
			"vulnerabilities": vulnerabilityCount,
			"critical_vulns":  criticalCount,
			"high_vulns":      highCount,
			"medium_vulns":    mediumCount,
			"low_vulns":       lowCount,
			"status":          scanStatus,
		},
		Metadata: map[string]interface{}{
			"scan_time": time.Now().Format(time.RFC3339),
			"image_ref": state.BuildResult.ImageRef,
		},
	}, nil
}

// TagStep implements image tagging
type TagStep struct{}

func NewTagStep() workflow.Step    { return &TagStep{} }
func (s *TagStep) Name() string    { return "tag_image" }
func (s *TagStep) MaxRetries() int { return 2 }
func (s *TagStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	if state.BuildResult == nil || state.BuildResult.ImageID == "" {
		state.Logger.Error("Cannot tag image: missing build result")
		return &workflow.StepResult{
				Success: false,
			}, errors.New(
				errors.CodeInvalidState,
				"tag_step",
				"cannot tag image: build result is missing or incomplete; do not proceed. Run build_image again after fixing Dockerfile.",
				nil,
			)
	}

	state.Logger.Info("Step 5: Tagging image for registry")

	// Extract tag from request parameters, default to "latest" if not provided
	imageTag := extractAndValidateTag(state, state.Logger)
	imageName := utils.ExtractRepoName(state.RepoIdentifier)

	// In test mode, skip actual Docker operations
	if state.IsTestMode() {
		state.Logger.Info("Step completed")

		// Update the build result with the final tag
		state.BuildResult.ImageRef = fmt.Sprintf("%s:%s", imageName, imageTag)

		state.Logger.Info("Step completed")

		return &workflow.StepResult{Success: true}, nil
	}

	// Actually tag the Docker image using the built image ID
	sourceImage := state.BuildResult.ImageID
	targetImage := fmt.Sprintf("%s:%s", imageName, imageTag)

	// Create Docker registry manager for tagging
	dockerClient := container.NewDockerCmdRunner(nil)
	registryManager := container.NewRegistryManager(dockerClient, state.Logger)

	state.Logger.Info("Step completed")

	tagResult, err := registryManager.TagImage(ctx, sourceImage, targetImage)
	if err != nil {
		return nil, errors.New(errors.CodeOperationFailed, "tag_step", fmt.Sprintf("failed to tag image: %v", err), err)
	}

	if !tagResult.Success {
		errorMsg := "Docker tag operation failed"
		if tagResult.Error != nil {
			errorMsg = tagResult.Error.Message
		}
		return nil, errors.New(errors.CodeOperationFailed, "tag_step", errorMsg, nil)
	}

	// Update the build result with the final tag
	state.BuildResult.ImageRef = targetImage

	state.Logger.Info("Step completed")

	return &workflow.StepResult{Success: true}, nil
}

// PushStep implements image pushing
type PushStep struct{}

func NewPushStep() workflow.Step    { return &PushStep{} }
func (s *PushStep) Name() string    { return "push_image" }
func (s *PushStep) MaxRetries() int { return 3 }
func (s *PushStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	state.Logger.Info("Step 6: Preparing image for deployment")

	if state.BuildResult == nil {
		return nil, errors.New(errors.CodeInvalidState, "push_step", "build result is required for image preparation", nil)
	}

	// Update the build result with the correct local registry reference for kind deployment
	imageName := utils.ExtractRepoName(state.RepoIdentifier)
	// Preserve the tag from the previous TagStep instead of hardcoding "latest"
	imageTag := parseImageReference(state.BuildResult.ImageRef)
	localRegistryImageRef := fmt.Sprintf("localhost:5001/%s:%s", imageName, imageTag)

	// Update both BuildResult and Result with the correct local registry reference
	state.BuildResult.ImageRef = localRegistryImageRef
	state.Result.ImageRef = localRegistryImageRef

	state.Logger.Info("Step completed")

	state.Logger.Info("Build result updated")

	// Return StepResult with corrected image reference in data
	return &workflow.StepResult{
		Success: true,
		Data: map[string]interface{}{
			"image_ref": localRegistryImageRef,
		},
	}, nil
}

// ManifestStep implements Kubernetes manifest generation
type ManifestStep struct{}

func NewManifestStep() workflow.Step    { return &ManifestStep{} }
func (s *ManifestStep) Name() string    { return "generate_manifests" }
func (s *ManifestStep) MaxRetries() int { return 2 }
func (s *ManifestStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	// Check if manifest content is provided
	var manifestList []string

	if manifestsData, exists := state.RequestParams["manifests"]; exists {
		if manifestsMap, ok := manifestsData.(map[string]interface{}); ok {
			state.Logger.Info("Using provided Kubernetes manifests from the LLM Host")

			// Create manifests directory
			manifestsDir := filepath.Join(state.AnalyzeResult.RepoPath, "manifests")
			if err := createManifestsDirectory(manifestsDir, state.Logger); err != nil {
				return nil, fmt.Errorf("failed to create manifests directory: %v", err)
			}

			// Write individual manifest files
			for filename, content := range manifestsMap {
				if contentStr, ok := content.(string); ok && contentStr != "" {
					filePath := filepath.Join(manifestsDir, filename)
					if err := writeManifestFile(filePath, contentStr, state.Logger); err != nil {
						return nil, fmt.Errorf("failed to write manifest file %s: %v", filename, err)
					}
					manifestList = append(manifestList, contentStr)
					state.Logger.Info("Wrote manifest file", "filename", filename, "path", filePath)
				}
			}

			// Set results with AI metadata
			state.K8sResult = &workflow.K8sResult{
				Manifests:   manifestList,
				Namespace:   extractNamespaceFromManifests(manifestList),
				ServiceName: extractServiceNameFromManifests(manifestList),
				Endpoint:    "", // Will be set after deployment
				Metadata: map[string]interface{}{
					"ai_generated":   true,
					"manifest_path":  manifestsDir,
					"manifest_count": len(manifestList),
				},
			}

			state.Logger.Info("Step completed")

			// Return basic success result

			return &workflow.StepResult{Success: true}, nil
		}
	}

	// If no content provided, generate manifests normally
	state.Logger.Info("Generating Kubernetes manifests through the MCP")

	// Check if deployment is actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping manifest generation (deployment not requested)")
		// Return basic success result

		return &workflow.StepResult{Success: true}, nil
	}

	if state.BuildResult == nil || state.AnalyzeResult == nil {
		return nil, errors.New(errors.CodeInvalidState, "manifest_step", "build result and analyze result are required for manifest generation", nil)
	}

	// Convert workflow BuildResult to infrastructure BuildResult
	imageName := utils.ExtractRepoName(state.RepoIdentifier)
	imageTag := parseImageReference(state.BuildResult.ImageRef)

	infraBuildResult := &BuildResult{
		ImageName: imageName,
		ImageTag:  imageTag,
		ImageID:   state.BuildResult.ImageID,
	}

	// Generate manifests
	appName := utils.ExtractRepoName(state.RepoIdentifier)
	namespace := "default"

	// In test mode, use test namespace and prefix app name
	if state.IsTestMode() {
		namespace = "test-namespace"
		appName = "test-" + appName
	}

	// Determine registry URL (default to localhost if not provided)
	registryURL := "localhost:5001"
	if val, ok := state.RequestParams["registry"].(string); ok && val != "" {
		registryURL = val
		state.Logger.Info("Using registry URL from request parameters", "registry", registryURL)
	}

	k8sResult, err := GenerateManifests(
		infraBuildResult,
		appName,
		namespace,
		state.AnalyzeResult.Port,
		state.AnalyzeResult.RepoPath,
		registryURL,
		state.Logger,
	)
	if err != nil {
		return nil, errors.New(errors.CodeManifestInvalid, "manifest_step", "k8s manifest generation failed", err)
	}

	// Extract actual manifest content from the result
	manifestContent := []string{}
	if k8sResult.Manifests != nil {
		if manifests, ok := k8sResult.Manifests["manifests"]; ok {
			// Handle different types of manifest content
			switch v := manifests.(type) {
			case []string:
				// If manifests are already a slice of strings
				manifestContent = v
			case []interface{}:
				// If manifests are a slice of interfaces, convert to strings
				for _, manifest := range v {
					if manifestStr, ok := manifest.(string); ok {
						manifestContent = append(manifestContent, manifestStr)
					}
				}
			case map[string]interface{}:
				// If manifests are a map, extract values as strings
				for _, manifest := range v {
					if manifestStr, ok := manifest.(string); ok {
						manifestContent = append(manifestContent, manifestStr)
					}
				}
			case string:
				// If it's a single manifest string
				manifestContent = []string{v}
			}
		}
	}

	// Store K8s result in workflow state with extracted manifest content
	manifestPath := k8sResult.Manifests["path"]
	state.Logger.Info("Step completed")

	metadata := map[string]interface{}{
		"app_name":       k8sResult.AppName,
		"ingress_url":    k8sResult.IngressURL,
		"manifest_path":  manifestPath, // Store manifest path for deployment
		"manifest_count": len(manifestContent),
		"template_used":  k8sResult.Manifests["template"],
	}

	state.K8sResult = &workflow.K8sResult{
		Manifests:   manifestContent, // Now contains actual manifest content
		Namespace:   k8sResult.Namespace,
		ServiceName: k8sResult.AppName, // Use AppName as service name
		Endpoint:    k8sResult.ServiceURL,
		Metadata:    metadata,
	}

	state.Logger.Info("Step completed")
	// Return basic success result

	return &workflow.StepResult{Success: true}, nil
}

// ClusterStep implements cluster setup
type ClusterStep struct{}

func NewClusterStep() workflow.Step    { return &ClusterStep{} }
func (s *ClusterStep) Name() string    { return "setup_cluster" }
func (s *ClusterStep) MaxRetries() int { return 2 }
func (s *ClusterStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	state.Logger.Info("Step 7: Setting up kind cluster")

	// Check if deployment is actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping cluster setup (deployment not requested)")
		// Return basic success result

		return &workflow.StepResult{Success: true}, nil
	}

	// In test mode, simulate cluster setup
	var registryURL string
	if state.IsTestMode() {
		state.Logger.Info("Test mode: Simulating kind cluster setup")
		registryURL = "test-registry.local:5000"
	} else {
		// Setup kind cluster with registry
		var err error
		registryURL, err = SetupKindCluster(ctx, "containerization-assist", state.Logger)
		if err != nil {
			return nil, errors.New(errors.CodeKubernetesApiError, "cluster_step", "kind cluster setup failed", err)
		}

		// Wait for nodes to become ready after cluster setup
		state.Logger.Info("Waiting for cluster nodes to become ready...")
		if err := waitForNodesReady(ctx, state.Logger); err != nil {
			state.Logger.Warn("Nodes may not be fully ready, but continuing", "error", err)
			// Don't fail the step, just warn - nodes often become ready shortly after
		}
	}

	// Store registry URL for later use
	if state.K8sResult == nil {
		state.K8sResult = &workflow.K8sResult{}
	}
	if state.K8sResult.Metadata == nil {
		state.K8sResult.Metadata = make(map[string]interface{})
	}
	state.K8sResult.Metadata["registry_url"] = registryURL

	state.Logger.Info("Kind cluster setup completed successfully", "registry_url", registryURL)
	// Return basic success result

	return &workflow.StepResult{Success: true}, nil
}

// DeployStep implements application deployment
type DeployStep struct{}

func NewDeployStep() workflow.Step    { return &DeployStep{} }
func (s *DeployStep) Name() string    { return "deploy_application" }
func (s *DeployStep) MaxRetries() int { return 3 }
func (s *DeployStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	state.Logger.Info("Step 8: Deploying application to Kubernetes")

	// Check if deployment is actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping deployment (not requested)")
		// Return basic success result

		return &workflow.StepResult{Success: true}, nil
	}

	if state.K8sResult == nil {
		return nil, errors.New(errors.CodeInvalidState, "deploy_step", "K8s manifests are required for deployment", nil)
	}

	// First, load image into kind cluster if needed
	if state.BuildResult != nil && !state.IsTestMode() {
		infraBuildResult := &BuildResult{
			ImageName: utils.ExtractRepoName(state.RepoIdentifier),
			ImageTag:  parseImageReference(state.BuildResult.ImageRef),
			ImageID:   state.BuildResult.ImageID,
		}

		err := LoadImageToKind(ctx, infraBuildResult, "containerization-assist", state.Logger)
		if err != nil {
			return nil, errors.New(errors.CodeImagePullFailed, "deploy_step", "failed to load image to kind", err)
		}
		state.Logger.Info("Image loaded into kind cluster successfully")
	} else if state.IsTestMode() {
		state.Logger.Info("Test mode: Skipping image load to kind cluster")
	}

	// Convert workflow K8sResult to infrastructure K8sResult for deployment
	manifestPath := ""
	if pathFromMetadata, exists := state.K8sResult.Metadata["manifest_path"]; exists {
		if pathStr, ok := pathFromMetadata.(string); ok {
			manifestPath = pathStr
		}
	}

	state.Logger.Info("Step completed")

	if manifestPath == "" {
		return nil, errors.New(errors.CodeInvalidState, "deploy_step", "manifest path not found in K8s result metadata", nil)
	}

	infraK8sResult := &K8sResult{
		AppName:    utils.ExtractRepoName(state.RepoIdentifier),
		Namespace:  state.K8sResult.Namespace,
		ServiceURL: state.K8sResult.Endpoint,
		Manifests: map[string]interface{}{
			"path": manifestPath,
		},
		Metadata: map[string]interface{}{
			"port":      0,                          // Default port
			"image_ref": state.BuildResult.ImageRef, // Use the tagged image reference from TagStep
		},
	}

	// Add port using priority: Dockerfile → Analysis → Default
	port := 8080 // Default fallback
	switch {
	case state.DockerfileResult != nil && state.DockerfileResult.ExposedPort > 0:
		port = state.DockerfileResult.ExposedPort
		state.Logger.Info("Using port from Dockerfile", "port", port)

	case state.AnalyzeResult != nil && state.AnalyzeResult.Port > 0:
		port = state.AnalyzeResult.Port
		state.Logger.Info("Using port from analysis", "port", port)

	default:
		state.Logger.Info("Using default application port", "port", port, "reason", "no port detected from Dockerfile or analysis")
	}
	infraK8sResult.Metadata["port"] = port

	// Add actual image ref from build result if available
	if state.BuildResult != nil && state.BuildResult.ImageRef != "" {
		infraK8sResult.Metadata["image_ref"] = state.BuildResult.ImageRef
	}

	// Log deployment details for debugging
	if path, ok := infraK8sResult.Manifests["path"].(string); ok {
		manifestPath = path
	}
	state.Logger.Info("Step completed")

	// Deploy to Kubernetes
	if state.IsTestMode() {
		state.Logger.Info("Step completed")
	} else {
		err := DeployToKubernetes(ctx, infraK8sResult, state.Logger)
		if err != nil {
			// Capture deployment diagnostics for error reporting
			captureDeploymentDiagnostics(ctx, state, infraK8sResult, state.Logger)

			return &workflow.StepResult{
				Success: false,
			}, err
		}
	}

	state.Logger.Info("Application deployed successfully", "namespace", state.K8sResult.Namespace)

	return &workflow.StepResult{
		Success: true,
		Data: map[string]interface{}{
			"namespace": state.K8sResult.Namespace,
			"app_name":  infraK8sResult.AppName,
		},
	}, nil
}

// VerifyStep implements deployment verification
type VerifyStep struct{}

func NewVerifyStep() workflow.Step    { return &VerifyStep{} }
func (s *VerifyStep) Name() string    { return "verify_deployment" }
func (s *VerifyStep) MaxRetries() int { return 2 }
func (s *VerifyStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	state.Logger.Info("Step 10: Verifying deployment health with port forwarding")

	// Check if deployment was actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping verification (deployment not requested)")
		// Return basic success result

		return &workflow.StepResult{Success: true}, nil
	}

	if state.K8sResult == nil {
		return nil, errors.New(errors.CodeInvalidState, "verify_step", "K8s result is required for deployment verification", nil)
	}

	// Convert workflow K8sResult to infrastructure K8sResult
	infraK8sResult := &K8sResult{
		AppName:    utils.ExtractRepoName(state.RepoIdentifier),
		Namespace:  state.K8sResult.Namespace,
		ServiceURL: state.K8sResult.Endpoint,
		Manifests: map[string]interface{}{
			"path": state.K8sResult.Metadata["manifest_path"], // Pass manifest path for verification
		},
	}

	// Perform enhanced verification with port forwarding and health checks
	var verifyResult *VerificationResult
	if state.IsTestMode() {
		state.Logger.Info("Test mode: Simulating enhanced deployment verification")
		state.Result.Endpoint = fmt.Sprintf("http://test-%s.%s.svc.cluster.local:8080",
			utils.ExtractRepoName(state.RepoIdentifier), state.K8sResult.Namespace)

		// Simulate successful verification output
		state.Logger.Info("✅ Deployment verified successfully")
		state.Logger.Info("✅ Port forwarding active (timeout: 30min)")
		state.Logger.Info("✅ Application responding (200 OK, 45ms)")
		state.Logger.Info("🔗 Access your app: http://localhost:8080")
	} else {
		// Use enhanced verification with port forwarding
		var err error
		verifyResult, err = VerifyDeploymentWithPortForward(ctx, infraK8sResult, state.Logger)
		if err != nil {
			state.Logger.Warn("Enhanced deployment verification failed", "error", err)
			// In strict mode, fail the workflow for verification errors
			if state.Args.StrictMode {
				return nil, errors.New(errors.CodeDeploymentFailed, "verify_step", "enhanced deployment verification failed in strict mode", err)
			}
			// Otherwise, don't fail the workflow - just warn
		}

		// Process and display verification results
		if verifyResult != nil {
			// Store the detailed verification result in K8sResult metadata for the orchestrator
			if state.K8sResult != nil {
				if state.K8sResult.Metadata == nil {
					state.K8sResult.Metadata = make(map[string]interface{})
				}
				state.K8sResult.Metadata["verification_result"] = map[string]interface{}{
					"deployment_success": verifyResult.DeploymentSuccess,
					"access_url":         verifyResult.AccessURL,
					"user_message":       verifyResult.UserMessage,
					"next_steps":         verifyResult.NextSteps,
					"messages":           verifyResult.Messages,
					"port_forward":       verifyResult.PortForwardResult,
					"health_check":       verifyResult.HealthCheckResult,
				}

				// Also capture current deployment diagnostics for the final response
				captureDeploymentDiagnostics(ctx, state, infraK8sResult, state.Logger)
			}

			for _, message := range verifyResult.Messages {
				switch message.Level {
				case "success":
					state.Logger.Info(fmt.Sprintf("%s %s", message.Icon, message.Message))
				case "warning":
					state.Logger.Warn(fmt.Sprintf("%s %s", message.Icon, message.Message))
				case "error":
					state.Logger.Error(fmt.Sprintf("%s %s", message.Icon, message.Message))
				case "info":
					state.Logger.Info(fmt.Sprintf("%s %s", message.Icon, message.Message))
				}
			}

			// Set endpoint in result
			if url := verifyResult.AccessURL; url != "" {
				state.Result.Endpoint = url
				state.Logger.Info(fmt.Sprintf("🔗 Access your app: %s", url))
				// Return basic success result

				return &workflow.StepResult{Success: true}, nil
			}

			endpoint, err := GetServiceEndpoint(ctx, infraK8sResult, state.Logger)
			if err != nil {
				if state.Args.StrictMode {
					return nil, errors.New(errors.CodeKubernetesApiError, "verify_step", "failed to get service endpoint in strict mode", err)
				}
				state.Logger.Warn("Failed to get service endpoint (non-critical)", "error", err)
			} else {
				state.Result.Endpoint = endpoint
				state.Logger.Info("Service endpoint discovered", "endpoint", endpoint)
			}

			// Log the formatted summary message and next steps from verification result
			if verifyResult.NextSteps != "" {
				state.Logger.Info(fmt.Sprintf("Next: %s", verifyResult.NextSteps))
			}
		}
	}

	state.Logger.Info("Enhanced deployment verification completed")
	// Return basic success result

	return &workflow.StepResult{Success: true}, nil
}

// extractNamespaceFromManifests extracts namespace from AI-generated manifests
func extractNamespaceFromManifests(manifests []string) string {
	namespaceRe := regexp.MustCompile(`namespace:\s*(\w+)`)

	for _, manifest := range manifests {
		if matches := namespaceRe.FindStringSubmatch(manifest); len(matches) > 1 {
			return matches[1]
		}
	}

	return "default" // fallback to default namespace
}

// extractServiceNameFromManifests extracts service name from AI-generated manifests
func extractServiceNameFromManifests(manifests []string) string {
	serviceRe := regexp.MustCompile(`kind:\s*Service[\s\S]*?name:\s*(\w+)`)

	for _, manifest := range manifests {
		if matches := serviceRe.FindStringSubmatch(manifest); len(matches) > 1 {
			return matches[1]
		}
	}

	return "app" // fallback service name
}

// createManifestsDirectory creates the manifests directory if it doesn't exist
func createManifestsDirectory(manifestsDir string, logger *slog.Logger) error {

	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return fmt.Errorf("failed to create manifests directory: %w", err)
	}

	return nil
}

// writeManifestFile writes a single manifest file to disk
func writeManifestFile(filePath, content string, logger *slog.Logger) error {

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

// validateDockerTag validates a Docker tag format according to Docker Registry specification
func validateDockerTag(tag string) error {
	// Docker tag rules from registry specification grammar:
	// tag := /[\w][\w.-]{0,127}/
	// (see: https://pkg.go.dev/github.com/distribution/reference#pkg-overview)
	// - Must start with word character (letter, digit, underscore)
	// - Followed by 0-127 word characters, dots, or dashes
	// - Maximum 128 characters total
	// - Case sensitive (uppercase allowed)
	if len(tag) == 0 {
		return fmt.Errorf("tag cannot be empty")
	}
	if len(tag) > 128 {
		return fmt.Errorf("tag too long (max 128 characters)")
	}

	// Check format according to registry specification: [\w][\w.-]{0,127}
	validTag := regexp.MustCompile(`^[\w][\w.-]{0,127}$`)
	if !validTag.MatchString(tag) {
		return fmt.Errorf("invalid tag format: must start with word character, followed by word chars/dots/dashes, max 128 chars")
	}

	return nil
}

// extractAndValidateTag extracts and validates a Docker tag from the request parameters.
func extractAndValidateTag(state *workflow.WorkflowState, logger *slog.Logger) string {
	const fallbackTag = "latest"

	tagStr, ok := state.RequestParams["tag"].(string)
	if !ok || tagStr == "" {
		return fallbackTag
	}

	if err := validateDockerTag(tagStr); err != nil {
		return fallbackTag
	}

	return tagStr
}

// parseImageReference parses an image reference and extracts the tag, default returns "latest"
func parseImageReference(imageRef string) string {
	if imageRef == "" {
		return "latest"
	}

	parts := strings.Split(imageRef, ":")
	if len(parts) == 2 {
		return parts[1]
	}

	return "latest"
}

// waitForNodesReady waits for Kubernetes nodes to become ready
func waitForNodesReady(ctx context.Context, logger *slog.Logger) error {
	kubeRunner := kubernetes.NewKubeCmdRunner(&runner.DefaultCommandRunner{})
	maxAttempts := 12 // 60 seconds total (5 seconds * 12)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check node status
		output, err := kubeRunner.GetNodes(ctx)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		// Check if nodes are ready (not tainted with not-ready)
		if strings.Contains(string(output), "Ready") && !strings.Contains(string(output), "NotReady") {
			return nil
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("nodes did not become ready within timeout period")
}
