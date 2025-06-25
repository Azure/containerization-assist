// Package mcp provides adapters for integrating MCP session state with the existing pipeline.
// This file contains the PipelineAdapter which converts between MCP session contexts
// and pipeline states, enabling reuse of existing pipeline logic.
package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/core/analysis"
	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/core/git"
	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/adapter"
	pipelinehelpers "github.com/Azure/container-copilot/pkg/mcp/internal/pipeline"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/Azure/container-copilot/pkg/pipeline"
	"github.com/rs/zerolog"
)

// PipelineAdapter bridges MCP session state and core operations
// It enables atomic MCP tools to use core mechanical operations
// while maintaining session-aware context.
type PipelineAdapter struct {
	sessionManager  *session.SessionManager
	clients         *adapter.MCPClients
	dockerBuilder   *coredocker.Builder
	dockerTemplates *coredocker.TemplateEngine
	dockerRegistry  *coredocker.RegistryManager
	k8sManifests    *kubernetes.ManifestManager
	k8sDeployment   *kubernetes.DeploymentManager
	k8sHealth       *kubernetes.HealthChecker
	repoAnalyzer    *analysis.RepositoryAnalyzer
	gitCloner       *git.Manager
	workspaceRoot   string
	logger          zerolog.Logger

	// Type-safe helpers for metadata and analysis handling
	analysisConverter *pipelinehelpers.AnalysisConverter
	insightGenerator  *pipelinehelpers.InsightGenerator
}

// NewPipelineAdapter creates a new pipeline adapter with core operations
func NewPipelineAdapter(sessionManager *session.SessionManager, mcpClients *adapter.MCPClients, logger zerolog.Logger) *PipelineAdapter {
	adapterLogger := logger.With().Str("component", "pipeline_adapter").Logger()

	// Convert MCP clients to CLI clients for compatibility with core packages
	cliClients := &clients.Clients{
		AzOpenAIClient: nil, // No AI in MCP mode
		Docker:         mcpClients.Docker,
		Kind:           mcpClients.Kind,
		Kube:           mcpClients.Kube,
	}

	// Create secure git manager with filesystem jail
	workspaceRoot := "/tmp/container-kit-workspace" // Default workspace root
	if envWorkspace := os.Getenv("CONTAINER_KIT_WORKSPACE"); envWorkspace != "" {
		workspaceRoot = envWorkspace
	}

	securityOpts := git.DefaultSecurityOptions()
	securityOpts.WorkspaceRoot = workspaceRoot

	gitManager, err := git.NewSecureManager(adapterLogger, securityOpts)
	if err != nil {
		// Fall back to regular manager if secure manager fails
		adapterLogger.Warn().Err(err).Msg("Failed to create secure git manager, using regular manager")
		gitManager = git.NewManager(adapterLogger)
	} else {
		adapterLogger.Info().Str("workspace_root", workspaceRoot).Msg("Created secure git manager with filesystem jail")
	}

	return &PipelineAdapter{
		sessionManager:    sessionManager,
		clients:           mcpClients,
		dockerBuilder:     coredocker.NewBuilder(cliClients, adapterLogger),
		dockerTemplates:   coredocker.NewTemplateEngine(adapterLogger),
		dockerRegistry:    coredocker.NewRegistryManager(cliClients, adapterLogger),
		k8sManifests:      kubernetes.NewManifestManager(adapterLogger),
		k8sDeployment:     kubernetes.NewDeploymentManager(cliClients, adapterLogger),
		k8sHealth:         kubernetes.NewHealthChecker(cliClients, adapterLogger),
		repoAnalyzer:      analysis.NewRepositoryAnalyzer(adapterLogger),
		gitCloner:         gitManager,
		workspaceRoot:     workspaceRoot,
		logger:            adapterLogger,
		analysisConverter: pipelinehelpers.NewAnalysisConverter(),
		insightGenerator:  pipelinehelpers.NewInsightGenerator(),
	}
}

// ConvertToRepositoryAnalysisState converts MCP session to repository analysis pipeline state
func (a *PipelineAdapter) ConvertToRepositoryAnalysisState(sessionID, targetRepo, extraContext string) (*pipeline.PipelineState, error) {
	sessionInterface, err := a.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Type assert to concrete session type
	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return nil, fmt.Errorf("session type assertion failed")
	}

	// Create pipeline state with session context
	state := &pipeline.PipelineState{
		// Core pipeline fields
		RepoFileTree: session.RepoFileTree,
		ExtraContext: extraContext,

		// Initialize metadata if not exists
		Metadata: make(map[pipeline.MetadataKey]any),
	}

	// If we already have repository analysis from session, include it
	if session.RepoAnalysis != nil {
		state.Metadata[pipeline.RepoAnalysisResultKey] = session.RepoAnalysis
		a.logger.Info().Msg("Reusing existing repository analysis from session")
	}

	// If we have Dockerfile context, include it
	if session.Dockerfile.Content != "" {
		state.Dockerfile = docker.Dockerfile{
			Content: session.Dockerfile.Content,
		}
	}

	// Add session metadata
	state.Metadata[pipeline.MetadataKey("mcp_session_id")] = sessionID
	state.Metadata[pipeline.MetadataKey("session_created_at")] = session.CreatedAt
	state.Metadata[pipeline.MetadataKey("session_updated_at")] = session.LastAccessed

	a.logger.Info().
		Str("session_id", sessionID).
		Str("target_dir", targetRepo).
		Msg("Converted MCP session to repository analysis pipeline state")

	return state, nil
}

// ConvertToDockerPipelineState converts MCP session to Docker pipeline state
func (a *PipelineAdapter) ConvertToDockerPipelineState(sessionID, imageName, registryURL string) (*pipeline.PipelineState, error) {
	sessionInterface, err := a.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Type assert to concrete session type
	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return nil, fmt.Errorf("session type assertion failed")
	}

	state := &pipeline.PipelineState{
		// Core pipeline fields
		ImageName:    imageName,
		RegistryURL:  registryURL,
		RepoFileTree: session.RepoFileTree,

		// Initialize metadata
		Metadata: make(map[pipeline.MetadataKey]any),
	}

	// Repository analysis is required for Docker stage
	if session.RepoAnalysis == nil {
		return nil, fmt.Errorf("repository analysis is required for dockerfile generation")
	}
	state.Metadata[pipeline.RepoAnalysisResultKey] = session.RepoAnalysis

	// Include Dockerfile if already generated
	if session.Dockerfile.Content != "" {
		state.Dockerfile = docker.Dockerfile{
			Content: session.Dockerfile.Content,
			Path:    session.Dockerfile.Path,
		}
	}

	// Add build history from session
	if len(session.BuildLogs) > 0 {
		state.Metadata[pipeline.MetadataKey("previous_build_logs")] = len(session.BuildLogs)
		state.Metadata[pipeline.MetadataKey("build_logs")] = session.BuildLogs
	}

	a.logger.Info().
		Str("session_id", sessionID).
		Str("image_name", imageName).
		Str("registry_url", registryURL).
		Msg("Converted MCP session to Docker pipeline state")

	return state, nil
}

// ConvertToManifestState converts MCP session to manifest pipeline state
func (a *PipelineAdapter) ConvertToManifestState(sessionID, namespace string) (*pipeline.PipelineState, error) {
	sessionInterface, err := a.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Type assert to concrete session type
	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return nil, fmt.Errorf("session type assertion failed")
	}

	state := &pipeline.PipelineState{
		// Core pipeline fields
		RepoFileTree: session.RepoFileTree,

		// Initialize metadata
		Metadata: make(map[pipeline.MetadataKey]any),
	}

	// Repository analysis is required
	if session.RepoAnalysis == nil {
		return nil, fmt.Errorf("repository analysis is required for manifests generation")
	}
	state.Metadata[pipeline.RepoAnalysisResultKey] = session.RepoAnalysis

	// Dockerfile context is required
	if session.Dockerfile.Content == "" {
		return nil, fmt.Errorf("dockerfile is required for manifests generation")
	}
	state.Dockerfile = docker.Dockerfile{
		Content: session.Dockerfile.Content,
		Path:    session.Dockerfile.Path,
	}

	// Set image name from session image reference
	if session.ImageRef.Registry != "" && session.ImageRef.Repository != "" {
		state.ImageName = fmt.Sprintf("%s/%s:%s",
			session.ImageRef.Registry,
			session.ImageRef.Repository,
			session.ImageRef.Tag)
	}

	if state.ImageName == "" {
		return nil, fmt.Errorf("build is required for manifests generation")
	}

	// Add namespace if provided
	if namespace != "" {
		state.Metadata["namespace"] = namespace
	}

	a.logger.Info().
		Str("session_id", sessionID).
		Str("image_name", state.ImageName).
		Str("namespace", namespace).
		Msg("Converted MCP session to manifest pipeline state")

	return state, nil
}

// UpdateSessionFromRepositoryAnalysis updates session state with repository analysis results
func (a *PipelineAdapter) UpdateSessionFromRepositoryAnalysis(sessionID string, pipelineState *pipeline.PipelineState) error {
	repoAnalysis, exists := pipelineState.Metadata[pipeline.RepoAnalysisResultKey]
	if !exists {
		return fmt.Errorf("no repository analysis in pipeline state")
	}

	// Use type-safe analysis converter
	analysisMap, err := a.analysisConverter.ToMap(repoAnalysis)
	if err != nil {
		return fmt.Errorf("failed to convert repository analysis: %w", err)
	}

	// Update session with repository analysis
	err = a.sessionManager.UpdateSession(sessionID, func(session *sessiontypes.SessionState) {
		session.RepoAnalysis = analysisMap
	})
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	a.logger.Info().
		Str("session_id", sessionID).
		Str("language", a.analysisConverter.GetLanguage(analysisMap)).
		Str("framework", a.analysisConverter.GetFramework(analysisMap)).
		Msg("Updated session with repository analysis results")

	return nil
}

// UpdateSessionFromDockerResults updates session state with Docker stage results
func (a *PipelineAdapter) UpdateSessionFromDockerResults(sessionID string, result interface{}) error {
	// Type assert the result to the expected type
	pipelineState, ok := result.(*pipeline.PipelineState)
	if !ok {
		return fmt.Errorf("invalid result type for UpdateSessionFromDockerResults")
	}
	// Update Dockerfile state if generated
	if pipelineState.Dockerfile.Content != "" {
		err := a.sessionManager.UpdateSession(sessionID, func(session *sessiontypes.SessionState) {
			session.Dockerfile.Content = pipelineState.Dockerfile.Content
			session.Dockerfile.Path = pipelineState.Dockerfile.Path
			session.Dockerfile.Built = true
			if session.Dockerfile.BuildTime == nil {
				now := time.Now()
				session.Dockerfile.BuildTime = &now
			}
		})
		if err != nil {
			return fmt.Errorf("failed to update session dockerfile: %w", err)
		}
	}

	// Update build results if available
	if pipelineState.ImageName != "" {
		err := a.sessionManager.UpdateSession(sessionID, func(session *sessiontypes.SessionState) {
			// Update image reference
			parts := strings.Split(pipelineState.ImageName, "/")
			if len(parts) >= 2 {
				if tagParts := strings.Split(parts[len(parts)-1], ":"); len(tagParts) >= 2 {
					session.ImageRef.Repository = strings.Join(parts[:len(parts)-1], "/") + "/" + tagParts[0]
					session.ImageRef.Tag = tagParts[1]
				} else {
					session.ImageRef.Repository = pipelineState.ImageName
					session.ImageRef.Tag = "latest"
				}
			}

			// Add build success log
			session.BuildLogs = append(session.BuildLogs, fmt.Sprintf("Build successful: %s", pipelineState.ImageName))
		})
		if err != nil {
			return fmt.Errorf("failed to update session build results: %w", err)
		}
	}

	a.logger.Info().
		Str("session_id", sessionID).
		Str("image_name", pipelineState.ImageName).
		Msg("Updated session with Docker stage results")

	return nil
}

// UpdateSessionFromManifestResults updates session state with manifest stage results
func (a *PipelineAdapter) UpdateSessionFromManifestResults(sessionID string, pipelineState *pipeline.PipelineState) error {
	// Use type-safe metadata manager
	metadata := pipelinehelpers.NewMetadataManager(pipelineState.Metadata)

	// Update manifest generation state
	err := a.sessionManager.UpdateSession(sessionID, func(session *sessiontypes.SessionState) {
		// Check if manifest path exists in metadata
		if manifestPath, exists := metadata.GetString("manifest_path"); exists {
			// Create a basic manifest entry
			manifestName := "deployment"
			if session.K8sManifests == nil {
				session.K8sManifests = make(map[string]types.K8sManifest)
			}
			session.K8sManifests[manifestName] = types.K8sManifest{
				Name:    manifestName,
				Kind:    "Deployment",
				Content: "", // Content would be read from manifestPath if needed
				Applied: false,
				Status:  "generated",
			}

			a.logger.Debug().
				Str("manifest_path", manifestPath).
				Msg("Manifest path found in metadata")
		}
	})
	if err != nil {
		return fmt.Errorf("failed to update session with manifest results: %w", err)
	}

	a.logger.Info().
		Str("session_id", sessionID).
		Msg("Updated session with manifest stage results")

	return nil
}

// ExtractInsights extracts insights from pipeline execution results using type-safe helpers
func (a *PipelineAdapter) ExtractInsights(pipelineState *pipeline.PipelineState, stageName string) (map[string]interface{}, error) {
	// Use type-safe metadata manager
	metadata := pipelinehelpers.NewMetadataManager(pipelineState.Metadata)

	insights := map[string]interface{}{
		"summary":      fmt.Sprintf("%s stage completed successfully", stageName),
		"key_findings": make([]string, 0),
	}

	var findings []string

	// Generate stage-specific insights using type-safe helpers
	switch stageName {
	case "repository_analysis":
		findings = a.insightGenerator.GenerateRepositoryInsights(metadata)
	case "docker_build":
		findings = a.insightGenerator.GenerateDockerInsights(metadata)
	case "manifest_generation":
		findings = a.insightGenerator.GenerateManifestInsights(metadata)
	default:
		findings = []string{fmt.Sprintf("%s stage completed successfully", stageName)}
	}

	// Add common insights
	commonInsights := a.insightGenerator.GenerateCommonInsights(metadata)
	findings = append(findings, commonInsights...)

	insights["key_findings"] = findings
	return insights, nil
}

// InjectClients ensures the pipeline state has properly configured clients
// Simplified: clients are directly available through the adapter, no injection needed
func (a *PipelineAdapter) InjectClients(pipelineState *pipeline.PipelineState) error {
	if a.clients == nil {
		return fmt.Errorf("no clients available")
	}

	a.logger.Debug().Msg("Clients available in pipeline adapter")
	return nil
}

// GetSessionWorkspace returns the workspace directory for a session
func (a *PipelineAdapter) GetSessionWorkspace(sessionID string) string {
	sessionInterface, err := a.sessionManager.GetSession(sessionID)
	if err != nil || sessionInterface == nil {
		// Return a default path if session not found
		return filepath.Join("/tmp/container-kit/workspaces", sessionID)
	}

	// Type assert to concrete session type
	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		// Return a default path if type assertion fails
		return filepath.Join("/tmp/container-kit/workspaces", sessionID)
	}

	return session.WorkspaceDir
}

// Core operation methods using new atomic operations

// AnalyzeRepository performs mechanical repository analysis
func (a *PipelineAdapter) AnalyzeRepository(sessionID, repoPath string) (*analysis.AnalysisResult, error) {
	a.logger.Info().
		Str("session_id", sessionID).
		Str("repo_path", repoPath).
		Msg("Starting mechanical repository analysis")

	result, err := a.repoAnalyzer.AnalyzeRepository(repoPath)
	if err != nil {
		return nil, fmt.Errorf("repository analysis failed: %w", err)
	}

	// Update session with analysis results
	if err := a.updateSessionWithAnalysis(sessionID, result); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to update session with analysis results")
	}

	return result, nil
}

// CloneRepository clones a repository to session workspace
// Updated to accept context parameter for proper context propagation
func (a *PipelineAdapter) CloneRepositoryWithBranch(ctx context.Context, sessionID, repoURL, branch string) (*git.CloneResult, error) {
	workspaceDir := a.GetSessionWorkspace(sessionID)
	repoDir := filepath.Join(workspaceDir, "repo")

	a.logger.Info().
		Str("session_id", sessionID).
		Str("repo_url", repoURL).
		Str("branch", branch).
		Str("target_dir", repoDir).
		Msg("Cloning repository to session workspace")

	options := git.CloneOptions{
		URL:    repoURL,
		Branch: branch,
		Depth:  1, // Shallow clone for faster operation
	}

	return a.gitCloner.CloneRepository(ctx, repoDir, options)
}

// Backward compatibility wrappers for tools.PipelineOperations interface
// These methods delegate to the context-aware versions using context.Background()

func (a *PipelineAdapter) CloneRepository(sessionID, repoURL, branch string) (*git.CloneResult, error) {
	return a.CloneRepositoryWithContext(context.Background(), sessionID, repoURL, branch)
}

func (a *PipelineAdapter) BuildDockerImage(sessionID, imageName, dockerfilePath string) (*mcptypes.BuildResult, error) {
	coreResult, err := a.BuildDockerImageWithContext(context.Background(), sessionID, imageName, dockerfilePath)
	if err != nil {
		return nil, err
	}

	// Convert core.BuildResult to unified types.BuildResult
	result := &mcptypes.BuildResult{
		ImageID:  coreResult.ImageID,
		ImageRef: coreResult.ImageRef,
		Success:  coreResult.Success,
		Logs:     strings.Join(coreResult.Logs, "\n"),
	}

	if coreResult.Error != nil {
		result.Error = &mcptypes.BuildError{
			Type:    coreResult.Error.Type,
			Message: coreResult.Error.Message,
		}
		result.Success = false
	}

	return result, nil
}

func (a *PipelineAdapter) PushDockerImage(sessionID, imageName string) error {
	// Extract registry URL from image name if needed
	_, err := a.PushDockerImageWithContext(context.Background(), sessionID, imageName, "")
	return err
}

func (a *PipelineAdapter) TagDockerImage(sessionID, sourceImage, targetImage string) error {
	_, err := a.TagDockerImageWithContext(context.Background(), sessionID, sourceImage, targetImage)
	return err
}

func (a *PipelineAdapter) PullDockerImage(sessionID, imageRef string) error {
	_, err := a.PullDockerImageWithContext(context.Background(), sessionID, imageRef)
	return err
}

func (a *PipelineAdapter) CheckApplicationHealth(sessionID, namespace, labelSelector string, timeout time.Duration) (*mcptypes.HealthCheckResult, error) {
	coreResult, err := a.CheckApplicationHealthWithContext(context.Background(), sessionID, namespace, labelSelector, timeout)
	if err != nil {
		return nil, err
	}

	// Convert core.kubernetes.HealthCheckResult to unified types.HealthCheckResult
	result := &mcptypes.HealthCheckResult{
		Healthy:     coreResult.Success,
		Status:      fmt.Sprintf("Health check completed. Pods: %d/%d ready", coreResult.Summary.ReadyPods, coreResult.Summary.TotalPods),
		PodStatuses: make([]mcptypes.PodStatus, 0),
	}

	// Convert pod statuses if they exist
	for _, pod := range coreResult.Pods {
		podStatus := mcptypes.PodStatus{
			Name:   pod.Name,
			Ready:  pod.Ready,
			Status: pod.Status,
			Reason: pod.Phase,
		}
		result.PodStatuses = append(result.PodStatuses, podStatus)
	}

	if coreResult.Error != nil {
		result.Error = &mcptypes.HealthCheckError{
			Type:    "health_check_failed",
			Message: coreResult.Error.Message,
		}
		result.Healthy = false
	}

	return result, nil
}

func (a *PipelineAdapter) PreviewDeployment(sessionID, manifestPath, namespace string) (string, error) {
	return a.PreviewDeploymentWithContext(context.Background(), sessionID, manifestPath, namespace)
}

// Legacy context management methods (no-op implementations for interface compatibility)
// These methods are no longer used since context is now passed directly to methods

func (a *PipelineAdapter) SetContext(sessionID string, ctx context.Context) {
	// No-op: Context is now passed directly to method calls
	a.logger.Debug().Str("session_id", sessionID).Msg("SetContext called (legacy no-op)")
}

func (a *PipelineAdapter) GetContext(sessionID string) context.Context {
	// Return background context as fallback for legacy code
	a.logger.Debug().Str("session_id", sessionID).Msg("GetContext called (legacy fallback)")
	return context.Background()
}

func (a *PipelineAdapter) ClearContext(sessionID string) {
	// No-op: Context is now passed directly to method calls
	a.logger.Debug().Str("session_id", sessionID).Msg("ClearContext called (legacy no-op)")
}

// Context-aware implementations (previously public methods, now internal)

// CloneRepositoryWithContext implements the tools.PipelineAdapter interface
func (a *PipelineAdapter) CloneRepositoryWithContext(ctx context.Context, sessionID, repoURL, branch string) (*git.CloneResult, error) {
	// If no branch specified, let Git auto-detect the default branch
	if branch == "" {
		// Try to detect the default branch first
		detectedBranch, err := a.detectDefaultBranch(repoURL)
		if err != nil {
			a.logger.Warn().Err(err).Msg("Could not detect default branch, proceeding without branch specification")
			// Proceed with empty branch - Git will use the remote's default
		} else {
			branch = detectedBranch
			a.logger.Info().Str("detected_branch", branch).Msg("Auto-detected default branch")
		}
	}
	return a.CloneRepositoryWithBranch(ctx, sessionID, repoURL, branch)
}

// GenerateDockerfile generates Dockerfile using templates
func (a *PipelineAdapter) GenerateDockerfile(sessionID, language, framework string) (string, error) {
	workspaceDir := a.GetSessionWorkspace(sessionID)

	a.logger.Info().
		Str("session_id", sessionID).
		Str("language", language).
		Str("framework", framework).
		Msg("Generating Dockerfile from template")

	// Get repository path from workspace
	repoPath := filepath.Join(workspaceDir, "repo")

	// Get config files for context
	configFiles := []string{}
	if err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if !d.IsDir() {
			name := d.Name()
			if name == "package.json" || name == "requirements.txt" || name == "go.mod" || name == "pom.xml" {
				configFiles = append(configFiles, name)
			}
		}
		return nil
	}); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to scan for config files")
	}

	// First, determine which template to use
	templateName := fmt.Sprintf("%s-%s", language, framework)
	if framework == "" {
		templateName = language
	}

	// Generate Dockerfile from template
	result, err := a.dockerTemplates.GenerateFromTemplate(templateName, repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	if !result.Success {
		return "", fmt.Errorf("Dockerfile generation failed: %s", result.Error.Message)
	}

	return result.Dockerfile, nil
}

// BuildDockerImageWithContext builds Docker image using core operations
func (a *PipelineAdapter) BuildDockerImageWithContext(ctx context.Context, sessionID, imageName, dockerfilePath string) (*coredocker.BuildResult, error) {
	workspaceDir := a.GetSessionWorkspace(sessionID)
	buildContext := filepath.Join(workspaceDir, "repo")

	a.logger.Info().
		Str("session_id", sessionID).
		Str("image_name", imageName).
		Str("dockerfile_path", dockerfilePath).
		Str("build_context", buildContext).
		Msg("Building Docker image")

	// Read Dockerfile content
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	options := coredocker.BuildOptions{
		ImageName: imageName,
		Platform:  "linux/amd64", // Default platform
	}

	return a.dockerBuilder.BuildImage(ctx, string(dockerfileContent), buildContext, options)
}

// PushDockerImageWithContext pushes image to registry
func (a *PipelineAdapter) PushDockerImageWithContext(ctx context.Context, sessionID, imageName, registryURL string) (*coredocker.RegistryPushResult, error) {
	a.logger.Info().
		Str("session_id", sessionID).
		Str("image_name", imageName).
		Str("registry_url", registryURL).
		Msg("Pushing Docker image to registry")

	options := coredocker.PushOptions{
		Registry:   registryURL,
		RetryCount: 3,
		Timeout:    5 * time.Minute,
	}

	// Create full image reference
	imageRef := fmt.Sprintf("%s/%s", registryURL, imageName)

	return a.dockerRegistry.PushImage(ctx, imageRef, options)
}

// TagDockerImageWithContext tags a Docker image with a new name
func (a *PipelineAdapter) TagDockerImageWithContext(ctx context.Context, sessionID, sourceImage, targetImage string) (*coredocker.TagResult, error) {
	a.logger.Info().
		Str("session_id", sessionID).
		Str("source_image", sourceImage).
		Str("target_image", targetImage).
		Msg("Tagging Docker image")

	return a.dockerRegistry.TagImage(ctx, sourceImage, targetImage)
}

// PullDockerImageWithContext pulls a Docker image from a registry
func (a *PipelineAdapter) PullDockerImageWithContext(ctx context.Context, sessionID, imageRef string) (*coredocker.PullResult, error) {
	a.logger.Info().
		Str("session_id", sessionID).
		Str("image_ref", imageRef).
		Msg("Pulling Docker image")

	return a.dockerRegistry.PullImage(ctx, imageRef)
}

// GenerateKubernetesManifestsWithContext generates K8s manifests
func (a *PipelineAdapter) GenerateKubernetesManifestsWithContext(ctx context.Context, sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*kubernetes.ManifestGenerationResult, error) {
	workspaceDir := a.GetSessionWorkspace(sessionID)
	manifestDir := filepath.Join(workspaceDir, "manifests")

	a.logger.Info().
		Str("session_id", sessionID).
		Str("image_name", imageName).
		Str("app_name", appName).
		Int("port", port).
		Str("output_dir", manifestDir).
		Msg("Generating Kubernetes manifests")

	// Build resource requirements if specified
	var resources *kubernetes.ResourceRequirements
	if cpuRequest != "" || memoryRequest != "" || cpuLimit != "" || memoryLimit != "" {
		resources = &kubernetes.ResourceRequirements{}

		if cpuRequest != "" || memoryRequest != "" {
			resources.Requests = &kubernetes.ResourceQuantity{
				CPU:    cpuRequest,
				Memory: memoryRequest,
			}
		}

		if cpuLimit != "" || memoryLimit != "" {
			resources.Limits = &kubernetes.ResourceQuantity{
				CPU:    cpuLimit,
				Memory: memoryLimit,
			}
		}
	}

	options := kubernetes.ManifestOptions{
		ImageRef:       imageName,
		AppName:        appName,
		Namespace:      "default",
		Port:           port,
		Replicas:       1,
		Template:       "basic", // Use basic template
		OutputDir:      manifestDir,
		IncludeService: true,
		IncludeIngress: false,
		Resources:      resources,
		Labels: map[string]string{
			"session_id": sessionID,
			"app":        appName,
		},
	}

	return a.k8sManifests.GenerateManifests(ctx, options)
}

// DeployToKubernetesWithContext deploys manifests to Kubernetes
func (a *PipelineAdapter) DeployToKubernetesWithContext(ctx context.Context, sessionID, manifestPath, namespace string) (*kubernetes.DeploymentResult, error) {
	a.logger.Info().
		Str("session_id", sessionID).
		Str("manifest_path", manifestPath).
		Str("namespace", namespace).
		Msg("Deploying to Kubernetes")

	options := kubernetes.DeploymentOptions{
		Namespace:   namespace,
		Wait:        true,
		WaitTimeout: 5 * time.Minute,
		Validate:    true,
		DryRun:      false,
	}

	return a.k8sDeployment.DeployManifest(ctx, manifestPath, options)
}

// CheckApplicationHealthWithContext checks deployed application health
func (a *PipelineAdapter) CheckApplicationHealthWithContext(ctx context.Context, sessionID, namespace, labelSelector string, timeout time.Duration) (*kubernetes.HealthCheckResult, error) {
	a.logger.Info().
		Str("session_id", sessionID).
		Str("namespace", namespace).
		Str("label_selector", labelSelector).
		Dur("timeout", timeout).
		Msg("Checking application health")

	options := kubernetes.HealthCheckOptions{
		Namespace:       namespace,
		LabelSelector:   labelSelector,
		IncludeEvents:   true,
		IncludeServices: true,
		Timeout:         timeout,
	}

	return a.k8sHealth.CheckApplicationHealth(ctx, options)
}

// PreviewDeploymentWithContext runs kubectl diff to preview deployment changes
func (a *PipelineAdapter) PreviewDeploymentWithContext(ctx context.Context, sessionID, manifestPath, namespace string) (string, error) {
	a.logger.Info().
		Str("session_id", sessionID).
		Str("manifest_path", manifestPath).
		Str("namespace", namespace).
		Msg("Previewing deployment changes")

	// Use kubectl diff to preview changes
	return a.k8sDeployment.PreviewChanges(ctx, manifestPath, namespace)
}

// Helper method to update session with analysis results
func (a *PipelineAdapter) updateSessionWithAnalysis(sessionID string, result *analysis.AnalysisResult) error {
	return a.sessionManager.UpdateSession(sessionID, func(session *sessiontypes.SessionState) {
		// Convert analysis result to map format for RepoAnalysis
		analysisMap := map[string]interface{}{
			"language":     result.Language,
			"framework":    result.Framework,
			"port":         result.Port,
			"dependencies": result.Dependencies,
		}
		session.RepoAnalysis = analysisMap
	})
}

// SaveAnalysisCache saves analysis results to session for caching
func (a *PipelineAdapter) SaveAnalysisCache(sessionID string, result *analysis.AnalysisResult) error {
	a.logger.Debug().
		Str("session_id", sessionID).
		Str("language", result.Language).
		Str("framework", result.Framework).
		Msg("Saving analysis cache to session")

	return a.updateSessionWithAnalysis(sessionID, result)
}

// detectDefaultBranch attempts to detect the default branch of a remote repository
func (a *PipelineAdapter) detectDefaultBranch(repoURL string) (string, error) {
	// Use git ls-remote to get the default branch
	cmd := exec.Command("git", "ls-remote", "--symref", repoURL, "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect default branch: %w", err)
	}

	// Parse the output to find the default branch
	// Output format: "ref: refs/heads/master	HEAD"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ref: refs/heads/") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 1 {
				ref := parts[0]
				// Extract branch name from "ref: refs/heads/branchname"
				if strings.HasPrefix(ref, "ref: refs/heads/") {
					branch := strings.TrimPrefix(ref, "ref: refs/heads/")
					return branch, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not parse default branch from git ls-remote output")
}

// AcquireResource manages resource allocation for a session
func (pa *PipelineAdapter) AcquireResource(sessionID, resourceType string) error {
	pa.logger.Debug().
		Str("session_id", sessionID).
		Str("resource_type", resourceType).
		Msg("Acquiring resource")

	// For now, this is a no-op as resource management is handled by session manager
	// In the future, this could track resource quotas, locks, etc.
	return nil
}

// ReleaseResource manages resource cleanup for a session
func (pa *PipelineAdapter) ReleaseResource(sessionID, resourceType string) error {
	pa.logger.Debug().
		Str("session_id", sessionID).
		Str("resource_type", resourceType).
		Msg("Releasing resource")

	// For now, this is a no-op as resource management is handled by session manager
	// In the future, this could track resource quotas, locks, etc.
	return nil
}

// Interface compatibility methods - simplified versions for the unified interface

// ConvertToDockerState creates a simple Docker state for interface compatibility
func (a *PipelineAdapter) ConvertToDockerState(sessionID string) (*mcptypes.DockerState, error) {
	// This is the interface-compatible version that returns unified DockerState
	return &mcptypes.DockerState{
		Images:     []string{},
		Containers: []string{},
		Networks:   []string{},
		Volumes:    []string{},
	}, nil
}

// DeployToKubernetes simplified interface method
func (a *PipelineAdapter) DeployToKubernetes(sessionID string, manifests []string) (*mcptypes.KubernetesDeploymentResult, error) {
	// For interface compatibility, deploy the first manifest to default namespace
	if len(manifests) == 0 {
		return &mcptypes.KubernetesDeploymentResult{
			Success: false,
			Error: &mcptypes.RichError{
				Code:    "NO_MANIFESTS",
				Message: "No manifests provided for deployment",
			},
		}, nil
	}

	result, err := a.DeployToKubernetesWithContext(context.Background(), sessionID, manifests[0], "default")
	if err != nil {
		return &mcptypes.KubernetesDeploymentResult{
			Success: false,
			Error: &mcptypes.RichError{
				Code:    "DEPLOYMENT_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	// Extract deployment and service names from resources
	deployments := []string{}
	services := []string{}
	for _, resource := range result.Resources {
		if resource.Kind == "Deployment" {
			deployments = append(deployments, resource.Name)
		} else if resource.Kind == "Service" {
			services = append(services, resource.Name)
		}
	}

	return &mcptypes.KubernetesDeploymentResult{
		Success:     result.Success,
		Namespace:   result.Namespace,
		Deployments: deployments,
		Services:    services,
	}, nil
}

// GenerateKubernetesManifests simplified interface method
func (a *PipelineAdapter) GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*mcptypes.KubernetesManifestResult, error) {
	result, err := a.GenerateKubernetesManifestsWithContext(context.Background(), sessionID, imageRef, appName, port, cpuRequest, memoryRequest, cpuLimit, memoryLimit)
	if err != nil {
		return &mcptypes.KubernetesManifestResult{
			Success: false,
			Error: &mcptypes.RichError{
				Code:    "MANIFEST_GENERATION_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	// Convert kubernetes.ManifestGenerationResult to mcptypes.KubernetesManifestResult
	manifestResult := &mcptypes.KubernetesManifestResult{
		Success:   result.Success,
		Manifests: make([]mcptypes.GeneratedManifest, 0),
	}

	// Convert manifests if available
	for _, manifest := range result.Manifests {
		manifestResult.Manifests = append(manifestResult.Manifests, mcptypes.GeneratedManifest{
			Kind:    manifest.Kind,
			Name:    manifest.Name,
			Path:    manifest.Path,
			Content: manifest.Content,
		})
	}

	if result.Error != nil {
		manifestResult.Error = &mcptypes.RichError{
			Code:    "GENERATION_ERROR",
			Message: result.Error.Message,
		}
	}

	return manifestResult, nil
}

// Removed convertMetadataToStringMap - now using types.MetadataManager for type-safe metadata access
