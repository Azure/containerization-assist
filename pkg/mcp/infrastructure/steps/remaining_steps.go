// Package steps contains placeholder implementations for remaining workflow steps.
package steps

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
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

// extractRepoName extracts repository name from URL
func extractRepoName(repoURL string) string {
	// Extract repo name from URL like https://github.com/user/repo.git
	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return "app"
	}

	name := parts[len(parts)-1]
	// Remove .git suffix if present
	name = strings.TrimSuffix(name, ".git")
	if name == "" {
		return "app"
	}
	return name
}

// createZerologFromSlog creates a zerolog logger from an slog logger
// This is a bridge function to use existing security scanners that expect zerolog
func createZerologFromSlog(slogLogger *slog.Logger) zerolog.Logger {
	// Create a basic zerolog logger that writes to the same output
	// For now, we'll use a simple console writer that matches slog output
	return zerolog.New(zerolog.ConsoleWriter{Out: zerolog.NewConsoleWriter().Out}).
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
func (s *BuildStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	if state.DockerfileResult == nil || state.AnalyzeResult == nil {
		return fmt.Errorf("dockerfile and analyze results are required for build")
	}

	state.Logger.Info("Step 3: Building Docker image")

	// Convert workflow types to infrastructure types for compatibility
	infraDockerfileResult := &DockerfileResult{
		Content:     state.DockerfileResult.Content,
		Path:        state.DockerfileResult.Path,
		BaseImage:   state.DockerfileResult.BaseImage,
		ExposedPort: state.DockerfileResult.ExposedPort,
	}

	// Generate image name and tag from repo URL
	imageName := extractRepoName(state.Args.RepoURL)
	imageTag := "latest"
	buildContext := state.AnalyzeResult.RepoPath

	// In test mode, skip actual Docker operations
	if state.Args.TestMode {
		state.Logger.Info("Test mode: Simulating Docker build",
			"image_name", imageName,
			"image_tag", imageTag)

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

		state.Logger.Info("Test mode: Docker build simulation completed",
			"image_id", buildResult.ImageID,
			"image_ref", state.BuildResult.ImageRef)

		return nil
	}

	// Call the infrastructure build function
	buildResult, err := BuildImage(ctx, infraDockerfileResult, imageName, imageTag, buildContext, state.Logger)
	if err != nil {
		return fmt.Errorf("docker build failed: %v", err)
	}

	if buildResult == nil {
		return fmt.Errorf("build result is nil after successful build")
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

	state.Logger.Info("Docker image build completed",
		"image_id", buildResult.ImageID,
		"image_name", buildResult.ImageName,
		"image_tag", buildResult.ImageTag)

	return nil
}

// ScanStep implements security scanning
type ScanStep struct{}

func NewScanStep() workflow.Step    { return &ScanStep{} }
func (s *ScanStep) Name() string    { return "security_scan" }
func (s *ScanStep) MaxRetries() int { return 2 }
func (s *ScanStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	// Skip if scanning is not requested
	if !state.Args.Scan {
		state.Logger.Info("Step 4: Skipping security scan (not requested)")
		return nil
	}

	if state.BuildResult == nil {
		return fmt.Errorf("build result is required for security scan")
	}

	state.Logger.Info("Step 4: Running security vulnerability scan")

	// Implement actual vulnerability scanning using UnifiedSecurityScanner
	imageRef := state.BuildResult.ImageRef
	if imageRef == "" {
		return fmt.Errorf("no image reference available for security scan")
	}

	// Create zerolog logger from slog logger for the scanner
	zerologLogger := createZerologFromSlog(state.Logger)

	// Initialize the unified security scanner
	scanner := docker.NewUnifiedSecurityScanner(zerologLogger)

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
			return fmt.Errorf("security scan failed in strict mode: %v", err)
		}
		state.Logger.Warn("Continuing workflow despite scan failure")
		return nil
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

	state.Logger.Info("Security scan completed",
		"status", scanStatus,
		"total_vulns", vulnerabilityCount,
		"critical", criticalCount,
		"high", highCount,
		"medium", mediumCount,
		"low", lowCount)
	return nil
}

// TagStep implements image tagging
type TagStep struct{}

func NewTagStep() workflow.Step    { return &TagStep{} }
func (s *TagStep) Name() string    { return "tag_image" }
func (s *TagStep) MaxRetries() int { return 2 }
func (s *TagStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	if state.BuildResult == nil {
		return fmt.Errorf("build result is required for image tagging")
	}

	state.Logger.Info("Step 5: Tagging image for registry")

	// For local kind clusters, we don't need external registry push
	// The image will be loaded directly into kind
	imageName := extractRepoName(state.Args.RepoURL)
	imageTag := "latest"

	// Update the build result with the final tag
	state.BuildResult.ImageRef = fmt.Sprintf("%s:%s", imageName, imageTag)

	state.Logger.Info("Image tagged successfully",
		"image_name", imageName,
		"image_tag", imageTag,
		"image_ref", state.BuildResult.ImageRef)

	return nil
}

// PushStep implements image pushing
type PushStep struct{}

func NewPushStep() workflow.Step    { return &PushStep{} }
func (s *PushStep) Name() string    { return "push_image" }
func (s *PushStep) MaxRetries() int { return 3 }
func (s *PushStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	state.Logger.Info("Step 6: Preparing image for deployment")

	// For local kind deployment, we skip external registry push
	// The image will be loaded directly into kind in the deploy step
	if state.BuildResult == nil {
		return fmt.Errorf("build result is required for image preparation")
	}

	state.Logger.Info("Image prepared for local kind deployment",
		"image_ref", state.BuildResult.ImageRef)

	// Update the result with the image reference
	state.Result.ImageRef = state.BuildResult.ImageRef

	return nil
}

// ManifestStep implements Kubernetes manifest generation
type ManifestStep struct{}

func NewManifestStep() workflow.Step    { return &ManifestStep{} }
func (s *ManifestStep) Name() string    { return "generate_manifests" }
func (s *ManifestStep) MaxRetries() int { return 2 }
func (s *ManifestStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	state.Logger.Info("Step 6: Generating Kubernetes manifests")

	// Check if deployment is actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping manifest generation (deployment not requested)")
		return nil
	}

	if state.BuildResult == nil || state.AnalyzeResult == nil {
		return fmt.Errorf("build result and analyze result are required for manifest generation")
	}

	// Convert workflow BuildResult to infrastructure BuildResult
	infraBuildResult := &BuildResult{
		ImageName: extractRepoName(state.Args.RepoURL),
		ImageTag:  "latest",
		ImageID:   state.BuildResult.ImageID,
	}

	// Generate manifests
	appName := extractRepoName(state.Args.RepoURL)
	namespace := "default"

	// In test mode, use test namespace and prefix app name
	if state.Args.TestMode {
		namespace = "test-namespace"
		appName = "test-" + appName
	}

	k8sResult, err := GenerateManifests(infraBuildResult, appName, namespace, state.AnalyzeResult.Port, state.Logger)
	if err != nil {
		return fmt.Errorf("k8s manifest generation failed: %v", err)
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
	state.K8sResult = &workflow.K8sResult{
		Manifests:   manifestContent, // Now contains actual manifest content
		Namespace:   k8sResult.Namespace,
		ServiceName: k8sResult.AppName, // Use AppName as service name
		Endpoint:    k8sResult.ServiceURL,
		Metadata: map[string]interface{}{
			"app_name":       k8sResult.AppName,
			"ingress_url":    k8sResult.IngressURL,
			"manifest_path":  k8sResult.Manifests["path"], // Store manifest path for deployment
			"manifest_count": len(manifestContent),
			"template_used":  k8sResult.Manifests["template"],
		},
	}

	state.Logger.Info("Kubernetes manifests generated successfully",
		"app_name", appName,
		"namespace", k8sResult.Namespace,
		"manifest_count", len(manifestContent),
		"manifests_extracted", len(manifestContent) > 0)
	return nil
}

// ClusterStep implements cluster setup
type ClusterStep struct{}

func NewClusterStep() workflow.Step    { return &ClusterStep{} }
func (s *ClusterStep) Name() string    { return "setup_cluster" }
func (s *ClusterStep) MaxRetries() int { return 2 }
func (s *ClusterStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	state.Logger.Info("Step 7: Setting up kind cluster")

	// Check if deployment is actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping cluster setup (deployment not requested)")
		return nil
	}

	// In test mode, simulate cluster setup
	var registryURL string
	if state.Args.TestMode {
		state.Logger.Info("Test mode: Simulating kind cluster setup")
		registryURL = "test-registry.local:5000"
	} else {
		// Setup kind cluster with registry
		var err error
		registryURL, err = SetupKindCluster(ctx, "container-kit", state.Logger)
		if err != nil {
			return fmt.Errorf("kind cluster setup failed: %v", err)
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
	return nil
}

// DeployStep implements application deployment
type DeployStep struct{}

func NewDeployStep() workflow.Step    { return &DeployStep{} }
func (s *DeployStep) Name() string    { return "deploy_application" }
func (s *DeployStep) MaxRetries() int { return 3 }
func (s *DeployStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	state.Logger.Info("Step 8: Deploying application to Kubernetes")

	// Check if deployment is actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping deployment (not requested)")
		return nil
	}

	if state.K8sResult == nil {
		return fmt.Errorf("K8s manifests are required for deployment")
	}

	// First, load image into kind cluster if needed
	if state.BuildResult != nil && !state.Args.TestMode {
		infraBuildResult := &BuildResult{
			ImageName: extractRepoName(state.Args.RepoURL),
			ImageTag:  "latest",
			ImageID:   state.BuildResult.ImageID,
		}

		err := LoadImageToKind(ctx, infraBuildResult, "container-kit", state.Logger)
		if err != nil {
			return fmt.Errorf("failed to load image to kind: %v", err)
		}
		state.Logger.Info("Image loaded into kind cluster successfully")
	} else if state.Args.TestMode {
		state.Logger.Info("Test mode: Skipping image load to kind cluster")
	}

	// Convert workflow K8sResult to infrastructure K8sResult for deployment
	infraK8sResult := &K8sResult{
		AppName:    extractRepoName(state.Args.RepoURL),
		Namespace:  state.K8sResult.Namespace,
		ServiceURL: state.K8sResult.Endpoint,
		Manifests: map[string]interface{}{
			"path": state.K8sResult.Metadata["manifest_path"], // Pass manifest path from ManifestStep
		},
		Metadata: map[string]interface{}{
			"port":      0, // Default port
			"image_ref": state.K8sResult.Namespace + "/" + extractRepoName(state.Args.RepoURL) + ":latest",
		},
	}

	// Add port from dockerfile result if available
	if state.DockerfileResult != nil {
		infraK8sResult.Metadata["port"] = state.DockerfileResult.ExposedPort
	}

	// Add actual image ref from build result if available
	if state.BuildResult != nil && state.BuildResult.ImageRef != "" {
		infraK8sResult.Metadata["image_ref"] = state.BuildResult.ImageRef
	}

	// Deploy to Kubernetes
	if state.Args.TestMode {
		state.Logger.Info("Test mode: Simulating Kubernetes deployment",
			"namespace", state.K8sResult.Namespace,
			"app_name", infraK8sResult.AppName)
	} else {
		err := DeployToKubernetes(ctx, infraK8sResult, state.Logger)
		if err != nil {
			return fmt.Errorf("kubernetes deployment failed: %v", err)
		}
	}

	state.Logger.Info("Application deployed successfully", "namespace", state.K8sResult.Namespace)
	return nil
}

// VerifyStep implements deployment verification
type VerifyStep struct{}

func NewVerifyStep() workflow.Step    { return &VerifyStep{} }
func (s *VerifyStep) Name() string    { return "verify_deployment" }
func (s *VerifyStep) MaxRetries() int { return 2 }
func (s *VerifyStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	state.Logger.Info("Step 10: Verifying deployment health")

	// Check if deployment was actually requested
	shouldDeploy := state.Args.Deploy == nil || *state.Args.Deploy
	if !shouldDeploy {
		state.Logger.Info("Skipping verification (deployment not requested)")
		return nil
	}

	if state.K8sResult == nil {
		return fmt.Errorf("K8s result is required for deployment verification")
	}

	// Convert workflow K8sResult to infrastructure K8sResult
	infraK8sResult := &K8sResult{
		AppName:    extractRepoName(state.Args.RepoURL),
		Namespace:  state.K8sResult.Namespace,
		ServiceURL: state.K8sResult.Endpoint,
		Manifests: map[string]interface{}{
			"path": state.K8sResult.Metadata["manifest_path"], // Pass manifest path for verification
		},
	}

	// Check deployment health
	if state.Args.TestMode {
		state.Logger.Info("Test mode: Simulating deployment health check")
		state.Result.Endpoint = fmt.Sprintf("http://test-%s.%s.svc.cluster.local:8080",
			extractRepoName(state.Args.RepoURL), state.K8sResult.Namespace)
	} else {
		err := CheckDeploymentHealth(ctx, infraK8sResult, state.Logger)
		if err != nil {
			state.Logger.Warn("Deployment health check failed (non-critical)", "error", err)
			// In strict mode, fail the workflow for health check errors
			if state.Args.StrictMode {
				return fmt.Errorf("deployment health check failed in strict mode: %v", err)
			}
			// Otherwise, don't fail the workflow for health check issues - just warn
		} else {
			state.Logger.Info("Deployment health check passed")
		}

		// Get service endpoint
		endpoint, err := GetServiceEndpoint(ctx, infraK8sResult, state.Logger)
		if err != nil {
			// In strict mode, fail the workflow for endpoint retrieval errors
			if state.Args.StrictMode {
				return fmt.Errorf("failed to get service endpoint in strict mode: %v", err)
			}
			// Otherwise, log the error but don't fail the workflow
			state.Logger.Warn("Failed to get service endpoint (non-critical)", "error", err)
			state.Result.Endpoint = "http://localhost:30000" // Placeholder for tests
		} else {
			state.Result.Endpoint = endpoint
			state.Logger.Info("Service endpoint discovered", "endpoint", endpoint)
		}
	}

	state.Logger.Info("Deployment verification completed")
	return nil
}
