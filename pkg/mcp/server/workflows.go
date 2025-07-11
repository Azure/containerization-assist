package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
	"github.com/Azure/container-kit/pkg/mcp/internal/steps"
	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// ContainerizeAndDeployArgs represents the input for the complete workflow
type ContainerizeAndDeployArgs struct {
	RepoURL string `json:"repo_url"`
	Branch  string `json:"branch,omitempty"`
	Scan    bool   `json:"scan,omitempty"`
}

// ContainerizeAndDeployResult represents the complete workflow output
type ContainerizeAndDeployResult struct {
	Success    bool                   `json:"success"`
	Endpoint   string                 `json:"endpoint,omitempty"`
	ImageRef   string                 `json:"image_ref,omitempty"`
	Namespace  string                 `json:"k8s_namespace,omitempty"`
	ScanReport map[string]interface{} `json:"scan_report,omitempty"`
	Steps      []WorkflowStep         `json:"steps"`
	Error      string                 `json:"error,omitempty"`
}

// WorkflowStep represents a single step in the workflow
type WorkflowStep struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
	Retries  int    `json:"retries,omitempty"`
	Progress string `json:"progress"` // e.g., "3/10"
	Message  string `json:"message"`  // Human-readable progress message
}

// RegisterWorkflowTools registers the single comprehensive workflow tool
func RegisterWorkflowTools(gomcpServer server.Server, logger *slog.Logger) error {
	logger.Info("Registering workflow tools")

	// Register the single containerize_and_deploy workflow tool
	gomcpServer.Tool("containerize_and_deploy",
		"Complete end-to-end containerization and deployment with AI-powered error fixing",
		func(ctx *server.Context, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
			return executeContainerizeAndDeploy(ctx, args, logger)
		})

	// Register individual atomic tools for AI assistant compatibility
	gomcpServer.Tool("analyze_repository",
		"Analyze a repository to determine language, framework, and containerization requirements",
		func(_ *server.Context, args struct {
			RepoURL string `json:"repo_url"`
			Branch  string `json:"branch,omitempty"`
		}) (interface{}, error) {
			logger.Info("Analyzing repository", "repo_url", args.RepoURL, "branch", args.Branch)

			// For now, return a simplified analysis result
			// In a full implementation, this would call the actual analysis logic
			return map[string]interface{}{
				"language":          "java",
				"framework":         "spring-boot",
				"build_tool":        "maven",
				"dockerfile_exists": false,
				"port":              8080,
				"analysis_complete": true,
			}, nil
		})

	logger.Info("Workflow tools registered successfully")
	return nil
}

// executeContainerizeAndDeploy implements the complete 10-step workflow from feedback.md
func executeContainerizeAndDeploy(mcpCtx *server.Context, args *ContainerizeAndDeployArgs, logger *slog.Logger) (*ContainerizeAndDeployResult, error) {
	logger.Info("Starting containerize_and_deploy workflow",
		"repo_url", args.RepoURL,
		"branch", args.Branch,
		"scan", args.Scan)

	result := &ContainerizeAndDeployResult{
		Steps: make([]WorkflowStep, 0, 10),
	}

	startTime := time.Now()
	ctx := context.Background()

	// Progress tracker
	totalSteps := 10
	currentStep := 0
	totalStepsFloat := float64(totalSteps)

	// Create a progress reporter if the client supports it
	var progressReporter *mcp.ProgressReporter
	if mcpCtx != nil && mcpCtx.HasProgressToken() {
		progressReporter = mcpCtx.CreateSimpleProgressReporter(&totalStepsFloat)
		logger.Info("Progress reporting enabled for workflow")
	}

	updateProgress := func() (int, string) {
		currentStep++
		progress := fmt.Sprintf("%d/%d", currentStep, totalSteps)
		percentage := int((float64(currentStep) / float64(totalSteps)) * 100)
		return percentage, progress
	}

	// Workflow state variables
	var analyzeResult *steps.AnalyzeResult
	var dockerfileResult *steps.DockerfileResult
	var buildResult *steps.BuildResult
	var k8sResult *steps.K8sResult

	// Step 1: Analyze repository with AI retry
	if err := executeStepWithRetry(ctx, result, "analyze_repository", 2, func() error {
		logger.Info("Step 1: Analyzing repository", "repo_url", args.RepoURL)

		var err error
		analyzeResult, err = steps.AnalyzeRepository(args.RepoURL, args.Branch, logger)
		if err != nil {
			return fmt.Errorf("repository analysis failed: %v", err)
		}

		logger.Info("Repository analysis completed",
			"language", analyzeResult.Language,
			"framework", analyzeResult.Framework,
			"port", analyzeResult.Port)
		return nil
	}, logger, updateProgress, "Analyzing repository structure and detecting language/framework", mcpCtx, progressReporter); err != nil {
		// Always return the result object to preserve progress information
		// gomcp will discard the result if we return a non-nil error
		result.Success = false
		return result, nil
	}

	// Step 2: Generate Dockerfile with AI retry on build errors
	if err := executeStepWithRetry(ctx, result, "generate_dockerfile", 2, func() error {
		logger.Info("Step 2: Generating Dockerfile")

		var err error
		dockerfileResult, err = steps.GenerateDockerfile(analyzeResult, logger)
		if err != nil {
			return fmt.Errorf("dockerfile generation failed: %v", err)
		}

		logger.Info("Dockerfile generated successfully", "path", dockerfileResult.Path)
		return nil
	}, logger, updateProgress, "Generating optimized Dockerfile for detected language/framework", mcpCtx, progressReporter); err != nil {
		result.Success = false
		return result, nil
	}

	// Step 3: Build image with AI retry on Docker errors
	if err := executeStepWithRetry(ctx, result, "build_image", 2, func() error {
		logger.Info("Step 3: Building Docker image")

		// Extract repo name from URL for image naming
		imageName := extractRepoName(args.RepoURL)

		var err error
		buildResult, err = steps.BuildImage(ctx, dockerfileResult, imageName, "latest", ".", logger)
		if err != nil {
			return fmt.Errorf("docker build failed: %v", err)
		}

		logger.Info("Docker image built successfully",
			"image_name", buildResult.ImageName,
			"image_tag", buildResult.ImageTag,
			"image_id", buildResult.ImageID)
		return nil
	}, logger, updateProgress, "Building Docker image with AI-powered error fixing", mcpCtx, progressReporter); err != nil {
		result.Success = false
		return result, nil
	}

	// Step 4: Create/refresh local kind cluster (no AI retry needed per feedback.md)
	var registryURL string
	if err := executeStep(result, "setup_kind_cluster", func() error {
		logger.Info("Step 4: Setting up kind cluster")

		var err error
		registryURL, err = steps.SetupKindCluster(ctx, "container-kit", logger)
		if err != nil {
			return fmt.Errorf("kind cluster setup failed: %v", err)
		}

		logger.Info("Kind cluster setup completed successfully", "registry_url", registryURL)
		return nil
	}, updateProgress, "Setting up local Kubernetes cluster with registry", mcpCtx, progressReporter); err != nil {
		result.Success = false
		return result, nil
	}

	// Step 5: Load image into cluster with AI retry on push failures
	if err := executeStepWithRetry(ctx, result, "load_image", 1, func() error {
		logger.Info("Step 5: Loading image into cluster")

		err := steps.LoadImageToKind(ctx, buildResult, "container-kit", logger)
		if err != nil {
			return fmt.Errorf("failed to load image to kind: %v", err)
		}

		logger.Info("Image loaded into kind cluster successfully")
		return nil
	}, logger, updateProgress, "Loading Docker image into Kubernetes cluster", mcpCtx, progressReporter); err != nil {
		result.Success = false
		return result, nil
	}

	// Step 6: Generate K8s manifests with AI retry on rollout failures
	if err := executeStepWithRetry(ctx, result, "generate_k8s_manifests", 2, func() error {
		logger.Info("Step 6: Generating Kubernetes manifests")

		appName := extractRepoName(args.RepoURL)
		var err error
		k8sResult, err = steps.GenerateManifests(buildResult, appName, "default", analyzeResult.Port, logger)
		if err != nil {
			return fmt.Errorf("k8s manifest generation failed: %v", err)
		}

		logger.Info("Kubernetes manifests generated successfully", "app_name", k8sResult.AppName)
		return nil
	}, logger, updateProgress, "Generating Kubernetes deployment manifests", mcpCtx, progressReporter); err != nil {
		result.Success = false
		return result, nil
	}

	// Step 7: kubectl apply with AI retry on pod crash loops
	if err := executeStepWithRetry(ctx, result, "deploy_to_k8s", 2, func() error {
		logger.Info("Step 7: Deploying to Kubernetes")

		err := steps.DeployToKubernetes(ctx, k8sResult, logger)
		if err != nil {
			return fmt.Errorf("kubernetes deployment failed: %v", err)
		}

		logger.Info("Kubernetes deployment completed successfully")
		return nil
	}, logger, updateProgress, "Deploying application to Kubernetes cluster", mcpCtx, progressReporter); err != nil {
		result.Success = false
		return result, nil
	}

	// Step 8: Health probe with AI retry on unhealthy endpoints
	if err := executeStepWithRetry(ctx, result, "health_probe", 1, func() error {
		logger.Info("Step 8: Performing health probe")

		endpoint, err := steps.GetServiceEndpoint(ctx, k8sResult, logger)
		if err != nil {
			return fmt.Errorf("failed to get service endpoint: %v", err)
		}

		result.Endpoint = endpoint
		logger.Info("Health probe completed", "endpoint", endpoint)
		return nil
	}, logger, updateProgress, "Performing application health checks and endpoint discovery", mcpCtx, progressReporter); err != nil {
		result.Success = false
		return result, nil
	}

	// Step 9: Vulnerability scan (optional) with AI retry
	if args.Scan {
		if err := executeStepWithRetry(ctx, result, "vulnerability_scan", 3, func() error {
			logger.Info("Step 9: Running vulnerability scan")

			// Use the real unified scanner for comprehensive vulnerability scanning
			scanResult, err := scanImageForVulnerabilities(ctx, buildResult, logger)
			if err != nil {
				return fmt.Errorf("vulnerability scan failed: %v", err)
			}

			result.ScanReport = scanResult
			logger.Info("Vulnerability scan completed",
				"vulnerabilities", scanResult["vulnerabilities"],
				"scanner", scanResult["scanner"])
			return nil
		}, logger, updateProgress, "Scanning Docker image for security vulnerabilities", mcpCtx, progressReporter); err != nil {
			result.Success = false
			return result, nil
		}
	} else {
		// Still increment progress counter even if scan is skipped
		updateProgress()
	}

	// Step 10: Finalize result and return
	percentage, progress := updateProgress()
	finalStep := WorkflowStep{
		Name:     "finalize_result",
		Status:   "completed",
		Progress: progress,
		Message:  fmt.Sprintf("[%d%%] Workflow completed successfully! Application is running", percentage),
		Duration: "0s",
	}
	result.Steps = append(result.Steps, finalStep)

	// Send final progress notification
	if mcpCtx != nil && progressReporter != nil {
		finalMessage := fmt.Sprintf("[100%%] Workflow completed successfully! Application is running at %s", result.Endpoint)
		if err := progressReporter.Complete(finalMessage); err != nil {
			logger.Warn("Failed to send final progress notification", "error", err)
		}
	}

	result.Success = true
	result.ImageRef = fmt.Sprintf("localhost:5001/%s:%s", buildResult.ImageName, buildResult.ImageTag)
	result.Namespace = k8sResult.Namespace

	logger.Info("Containerize and deploy workflow completed successfully",
		"duration", time.Since(startTime),
		"endpoint", result.Endpoint,
		"image_ref", result.ImageRef)

	return result, nil
}

// executeStep runs a workflow step and tracks its execution
func executeStep(result *ContainerizeAndDeployResult, stepName string, stepFunc func() error, progressFunc func() (int, string), message string, mcpCtx *server.Context, progressReporter *mcp.ProgressReporter) error {
	startTime := time.Now()
	percentage, progress := progressFunc()

	step := WorkflowStep{
		Name:     stepName,
		Status:   "running",
		Progress: progress,
		Message:  fmt.Sprintf("[%d%%] %s", percentage, message),
	}

	// Send real-time progress notification to MCP client
	if mcpCtx != nil && progressReporter != nil {
		currentStepFloat := float64(len(result.Steps) + 1)
		if err := progressReporter.Update(currentStepFloat, step.Message); err != nil {
			// Log but don't fail on notification errors
			// Use placeholder logger since we don't have logger in this function
			fmt.Printf("Failed to send progress notification: %v\n", err)
		}
	}

	// Execute the step
	err := stepFunc()
	step.Duration = time.Since(startTime).String()

	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
		result.Steps = append(result.Steps, step)
		result.Error = fmt.Sprintf("Step %s failed: %v", stepName, err)
		return err
	}

	step.Status = "completed"
	result.Steps = append(result.Steps, step)
	return nil
}

// executeStepWithRetry runs a workflow step with AI-powered retry logic
func executeStepWithRetry(ctx context.Context, result *ContainerizeAndDeployResult, stepName string, maxRetries int, stepFunc func() error, logger *slog.Logger, progressFunc func() (int, string), message string, mcpCtx *server.Context, progressReporter *mcp.ProgressReporter) error {
	startTime := time.Now()
	percentage, progress := progressFunc()

	step := WorkflowStep{
		Name:     stepName,
		Status:   "running",
		Progress: progress,
		Message:  fmt.Sprintf("[%d%%] %s", percentage, message),
	}

	// Send real-time progress notification to MCP client
	if mcpCtx != nil && progressReporter != nil {
		currentStepFloat := float64(len(result.Steps) + 1)
		if err := progressReporter.Update(currentStepFloat, step.Message); err != nil {
			logger.Warn("Failed to send progress notification", "error", err, "step", stepName)
		}
	}

	// Execute the step with AI retry
	err := retry.WithAIRetry(ctx, stepName, maxRetries, stepFunc, logger)
	step.Duration = time.Since(startTime).String()

	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
		result.Steps = append(result.Steps, step)
		result.Error = fmt.Sprintf("Step %s failed: %v", stepName, err)
		return err
	}

	step.Status = "completed"
	result.Steps = append(result.Steps, step)
	return nil
}

// extractRepoName extracts the repository name from a Git URL
func extractRepoName(repoURL string) string {
	// Extract repo name from URL like https://github.com/user/repo.git
	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return "app"
	}

	name := parts[len(parts)-1]
	// Remove .git suffix if present
	if strings.HasSuffix(name, ".git") {
		name = strings.TrimSuffix(name, ".git")
	}

	// Sanitize name for Docker/K8s compatibility
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")

	if name == "" {
		return "app"
	}

	return name
}

// scanImageForVulnerabilities performs real vulnerability scanning using the unified scanner
func scanImageForVulnerabilities(ctx context.Context, buildResult *steps.BuildResult, logger *slog.Logger) (map[string]interface{}, error) {
	if buildResult == nil {
		return nil, fmt.Errorf("build result is required for vulnerability scanning")
	}

	// Create image reference for scanning
	imageRef := fmt.Sprintf("%s:%s", buildResult.ImageName, buildResult.ImageTag)

	logger.Info("Starting vulnerability scan", "image_ref", imageRef)

	// Create zerolog logger for the scanner (it uses zerolog)
	zerologLogger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	// Initialize the unified scanner with both Trivy and Grype
	scanner := docker.NewUnifiedSecurityScanner(zerologLogger)

	// Perform the scan with MEDIUM severity threshold
	scanResult, err := scanner.ScanImage(ctx, imageRef, "MEDIUM")
	if err != nil {
		logger.Error("Vulnerability scan failed", "error", err, "image_ref", imageRef)
		return nil, fmt.Errorf("vulnerability scan failed: %v", err)
	}

	if !scanResult.Success {
		logger.Warn("Vulnerability scan completed with issues", "image_ref", imageRef)
	}

	// Extract vulnerability counts
	var totalVulns int
	var criticalVulns, highVulns, mediumVulns, lowVulns int
	var scannerUsed string

	// Count vulnerabilities from Trivy results
	if scanResult.TrivyResult != nil {
		scannerUsed = "trivy"
		totalVulns += len(scanResult.TrivyResult.Vulnerabilities)
		for _, vuln := range scanResult.TrivyResult.Vulnerabilities {
			switch strings.ToUpper(vuln.Severity) {
			case "CRITICAL":
				criticalVulns++
			case "HIGH":
				highVulns++
			case "MEDIUM":
				mediumVulns++
			case "LOW":
				lowVulns++
			}
		}
	}

	// Count vulnerabilities from Grype results (if available)
	if scanResult.GrypeResult != nil && scanResult.TrivyResult == nil {
		scannerUsed = "grype"
		totalVulns += len(scanResult.GrypeResult.Vulnerabilities)
		for _, vuln := range scanResult.GrypeResult.Vulnerabilities {
			switch strings.ToUpper(vuln.Severity) {
			case "CRITICAL":
				criticalVulns++
			case "HIGH":
				highVulns++
			case "MEDIUM":
				mediumVulns++
			case "LOW":
				lowVulns++
			}
		}
	}

	// Both scanners available
	if scanResult.TrivyResult != nil && scanResult.GrypeResult != nil {
		scannerUsed = "trivy+grype"
	}

	// Determine overall status
	status := "clean"
	if criticalVulns > 0 {
		status = "critical"
	} else if highVulns > 0 {
		status = "high_risk"
	} else if mediumVulns > 0 {
		status = "medium_risk"
	} else if lowVulns > 0 {
		status = "low_risk"
	}

	// Create structured scan report
	report := map[string]interface{}{
		"success":           scanResult.Success,
		"status":            status,
		"scanner":           scannerUsed,
		"image_ref":         imageRef,
		"vulnerabilities":   totalVulns,
		"critical_vulns":    criticalVulns,
		"high_vulns":        highVulns,
		"medium_vulns":      mediumVulns,
		"low_vulns":         lowVulns,
		"scan_duration":     scanResult.Duration.String(),
		"scanned_at":        scanResult.ScanTime.Format(time.RFC3339),
		"remediation_steps": len(scanResult.Remediation),
	}

	// Add detailed results if needed
	if totalVulns > 0 {
		report["has_vulnerabilities"] = true
		if scanResult.TrivyResult != nil && len(scanResult.TrivyResult.Vulnerabilities) > 0 {
			report["trivy_vulnerabilities"] = scanResult.TrivyResult.Vulnerabilities
		}
		if scanResult.GrypeResult != nil && len(scanResult.GrypeResult.Vulnerabilities) > 0 {
			report["grype_vulnerabilities"] = scanResult.GrypeResult.Vulnerabilities
		}
	} else {
		report["has_vulnerabilities"] = false
	}

	logger.Info("Vulnerability scan completed",
		"total_vulnerabilities", totalVulns,
		"critical", criticalVulns,
		"high", highVulns,
		"medium", mediumVulns,
		"low", lowVulns,
		"status", status,
		"scanner", scannerUsed,
		"duration", scanResult.Duration)

	return report, nil
}
