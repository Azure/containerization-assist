package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
)

// TypeSafeDeployTool implements the deploy tool with type safety and comprehensive features
type TypeSafeDeployTool struct {
	k8sClient      K8sClient
	stateManager   StateManager
	sessionManager SessionManager
	analyzer       DeployAnalyzer
	logger         *slog.Logger
	mu             sync.RWMutex
	state          map[string]interface{}
}

// DeployArgs represents the arguments for deploying to Kubernetes
type DeployArgs struct {
	SessionID    string        `json:"session_id" validate:"required"`
	ManifestPath string        `json:"manifest_path" validate:"required"`
	Namespace    string        `json:"namespace" validate:"required"`
	DryRun       bool          `json:"dry_run,omitempty"`
	Wait         bool          `json:"wait,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty"`
	WaitForReady bool          `json:"wait_for_ready,omitempty"`
}

// Remove local type alias and use core.DeployResult directly

// K8sResource represents a Kubernetes resource
type K8sResource struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	APIVersion string `json:"api_version"`
	Status     string `json:"status"`
}

// K8sClient interface for Kubernetes operations
type K8sClient interface {
	Apply(ctx context.Context, manifestPath, namespace string) ([]K8sResource, error)
	Delete(ctx context.Context, manifestPath, namespace string) error
	WaitForReady(ctx context.Context, resources []K8sResource, timeout time.Duration) (bool, error)
}

// StateManager interface for state management
type StateManager interface {
	GetState(ctx context.Context, key string) (interface{}, error)
	SetState(ctx context.Context, key string, value interface{}) error
}

// SessionManager interface for session management
type SessionManager interface {
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error
}

// Session represents a deployment session
type Session struct {
	ID        string
	State     map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DeployAnalyzer interface for deployment analysis
type DeployAnalyzer interface {
	AnalyzeDeployment(ctx context.Context, args *DeployArgs) (*DeployAnalysis, error)
}

// DeployAnalysis represents deployment analysis results
type DeployAnalysis struct {
	Warnings []string `json:"warnings"`
	Issues   []string `json:"issues"`
}

// NewTypeS afeDeployTool creates a new TypeSafeDeployTool
func NewTypeSafeDeployTool(
	k8sClient K8sClient,
	stateManager StateManager,
	sessionManager SessionManager,
	analyzer DeployAnalyzer,
	logger *slog.Logger,
) *TypeSafeDeployTool {
	return &TypeSafeDeployTool{
		k8sClient:      k8sClient,
		stateManager:   stateManager,
		sessionManager: sessionManager,
		analyzer:       analyzer,
		logger:         logger,
		state:          make(map[string]interface{}),
	}
}

// Name implements api.Tool
func (t *TypeSafeDeployTool) Name() string {
	return "k8s_deploy"
}

// Description implements api.Tool
func (t *TypeSafeDeployTool) Description() string {
	return "Deploy applications to Kubernetes clusters using manifests with comprehensive validation and monitoring"
}

// Execute implements api.Tool
func (t *TypeSafeDeployTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse input
	args, err := t.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Validate input
	if err := t.Validate(args); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Validation failed: %v", err),
		}, err
	}

	// Execute deployment
	result, err := t.ExecuteTyped(ctx, args)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
			Data:    t.resultToMap(result),
		}, err
	}

	return api.ToolOutput{
		Success: true,
		Data:    t.resultToMap(result),
	}, nil
}

// Schema implements api.Tool
func (t *TypeSafeDeployTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for the deployment",
				},
				"manifest_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the Kubernetes manifest file or directory",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace for deployment",
				},
				"dry_run": map[string]interface{}{
					"type":        "boolean",
					"description": "Perform a dry run without applying changes",
				},
				"wait": map[string]interface{}{
					"type":        "boolean",
					"description": "Wait for resources to be ready",
				},
				"timeout": map[string]interface{}{
					"type":        "string",
					"description": "Timeout for deployment operations (e.g., '5m')",
				},
				"wait_for_ready": map[string]interface{}{
					"type":        "boolean",
					"description": "Wait for all resources to be ready before completing",
				},
			},
			"required": []string{"session_id", "manifest_path", "namespace"},
		},
	}
}

// Validate validates the deployment arguments
func (t *TypeSafeDeployTool) Validate(args interface{}) error {
	deployArgs, ok := args.(*DeployArgs)
	if !ok {
		return fmt.Errorf("invalid argument type: expected *DeployArgs, got %T", args)
	}

	// Validate required fields
	if deployArgs.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	if deployArgs.ManifestPath == "" {
		return fmt.Errorf("manifest path is required")
	}

	if deployArgs.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	// Check if manifest path exists
	info, err := os.Stat(deployArgs.ManifestPath)
	if err != nil {
		return fmt.Errorf("manifest path error: %w", err)
	}

	// Can be either a file or directory
	if info.IsDir() {
		// Check if directory contains manifest files
		files, err := filepath.Glob(filepath.Join(deployArgs.ManifestPath, "*.yaml"))
		if err != nil {
			return fmt.Errorf("error checking manifest directory: %w", err)
		}
		yamlFiles, err := filepath.Glob(filepath.Join(deployArgs.ManifestPath, "*.yml"))
		if err != nil {
			return fmt.Errorf("error checking manifest directory: %w", err)
		}

		if len(files) == 0 && len(yamlFiles) == 0 {
			return fmt.Errorf("no manifest files found in directory: %s", deployArgs.ManifestPath)
		}
	}

	// Set default timeout if not specified
	if deployArgs.Timeout == 0 {
		deployArgs.Timeout = 10 * time.Minute
	}

	return nil
}

// ExecuteTyped executes the deployment with typed arguments and results
func (t *TypeSafeDeployTool) ExecuteTyped(ctx context.Context, args *DeployArgs) (*core.DeployResult, error) {
	// Initialize result
	result := &core.DeployResult{
		BaseToolResponse: core.NewToolResponse("deploy", args.SessionID, args.DryRun),
		Namespace:        args.Namespace,
		Errors:           []string{},
		Warnings:         []string{},
		Data:             make(map[string]interface{}),
		DeploymentTime:   time.Now(),
	}

	// Pre-deployment checks
	if t.analyzer != nil {
		analysis, err := t.analyzer.AnalyzeDeployment(ctx, args)
		if err != nil {
			result.AddWarning(fmt.Sprintf("Pre-deployment analysis warning: %v", err))
		} else {
			for _, warning := range analysis.Warnings {
				result.AddWarning(warning)
			}
			for _, issue := range analysis.Issues {
				result.AddWarning(issue)
			}
		}
	}

	// Deploy to Kubernetes
	deployStart := time.Now()

	if args.DryRun {
		t.logger.Info("Performing dry run deployment",
			slog.String("session_id", args.SessionID),
			slog.String("namespace", args.Namespace),
			slog.String("manifest_path", args.ManifestPath))

		// For dry run, just validate and return success
		result.Success = true
		result.Duration = time.Since(deployStart).String()
		result.Message = "Dry run completed successfully - no changes applied"
		return result, nil
	}

	resources, err := t.k8sClient.Apply(ctx, args.ManifestPath, args.Namespace)
	if err != nil {
		result.AddError(fmt.Sprintf("Deployment failed: %v", err))
		return result, err
	}

	// Deployment succeeded
	result.Success = true
	result.Resources = resources
	result.Duration = time.Since(deployStart).String()
	result.Message = "Deployment completed successfully"

	// Wait for resources to be ready if requested
	if args.WaitForReady {
		ready, err := t.waitForReady(ctx, resources, args.Timeout)
		if err != nil {
			result.AddWarning(fmt.Sprintf("Resources may not be ready: %v", err))
		}
		result.Data["resources_ready"] = ready
	}

	// Update state
	t.updateState(args.SessionID, result)

	// Log success
	t.logger.Info("Deployment completed successfully",
		slog.String("session_id", args.SessionID),
		slog.String("namespace", args.Namespace),
		slog.String("duration", result.Duration),
		slog.Int("resources", len(resources)),
		slog.Int("warnings", len(result.Warnings)))

	return result, nil
}

// waitForReady waits for resources to be ready
func (t *TypeSafeDeployTool) waitForReady(ctx context.Context, resources []K8sResource, timeout time.Duration) (bool, error) {
	if t.k8sClient == nil {
		return false, fmt.Errorf("k8s client not available")
	}

	return t.k8sClient.WaitForReady(ctx, resources, timeout)
}

// updateState updates the tool state after a deployment
func (t *TypeSafeDeployTool) updateState(sessionID string, result *core.DeployResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Store deployment result in state
	deployKey := fmt.Sprintf("deploy_%s", sessionID)
	t.state[deployKey] = map[string]interface{}{
		"namespace":       result.Namespace,
		"success":         result.Success,
		"deployment_time": result.DeploymentTime,
		"resources":       len(result.Resources),
		"warnings":        len(result.Warnings),
		"errors":          len(result.Errors),
	}

	// Store last successful deployment
	if result.Success {
		t.state["last_successful_deployment"] = map[string]interface{}{
			"namespace":       result.Namespace,
			"deployment_time": result.DeploymentTime,
			"resources":       result.Resources,
		}
	}

	// Update session state if state manager is available
	if t.stateManager != nil {
		ctx := context.Background()
		stateData := map[string]interface{}{
			"last_deployment": map[string]interface{}{
				"namespace":       result.Namespace,
				"deployment_time": result.DeploymentTime,
				"success":         result.Success,
				"resources":       len(result.Resources),
			},
		}
		if err := t.stateManager.SetState(ctx, sessionID, stateData); err != nil {
			t.logger.Error("Failed to update session state",
				slog.String("session_id", sessionID),
				slog.String("error", err.Error()))
		}
	}
}

// SetState sets the tool state
func (t *TypeSafeDeployTool) SetState(state map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == nil {
		t.state = make(map[string]interface{})
	}

	for key, value := range state {
		t.state[key] = value
	}
}

// GetState gets the tool state
func (t *TypeSafeDeployTool) GetState() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]interface{})
	for key, value := range t.state {
		result[key] = value
	}

	return result
}

// parseInput parses the tool input into DeployArgs
func (t *TypeSafeDeployTool) parseInput(input api.ToolInput) (*DeployArgs, error) {
	args := &DeployArgs{
		SessionID: input.SessionID,
		Timeout:   10 * time.Minute, // Default timeout
	}

	// Parse manifest_path
	if v, ok := input.Data["manifest_path"].(string); ok {
		args.ManifestPath = v
	}

	// Parse namespace
	if v, ok := input.Data["namespace"].(string); ok {
		args.Namespace = v
	}

	// Parse boolean fields
	if v, ok := input.Data["dry_run"].(bool); ok {
		args.DryRun = v
	}
	if v, ok := input.Data["wait"].(bool); ok {
		args.Wait = v
	}
	if v, ok := input.Data["wait_for_ready"].(bool); ok {
		args.WaitForReady = v
	}

	// Parse timeout
	if v, ok := input.Data["timeout"].(string); ok {
		if timeout, err := time.ParseDuration(v); err == nil {
			args.Timeout = timeout
		}
	}

	return args, nil
}

// resultToMap converts the result to a map for API response
func (t *TypeSafeDeployTool) resultToMap(result *core.DeployResult) map[string]interface{} {
	return map[string]interface{}{
		"success":         result.Success,
		"deployment_name": result.DeploymentName,
		"service_name":    result.ServiceName,
		"namespace":       result.Namespace,
		"endpoints":       result.Endpoints,
		"message":         result.Message,
		"errors":          result.Errors,
		"warnings":        result.Warnings,
		"deployment_time": result.DeploymentTime,
		"data":            result.Data,
	}
}

// Methods for DeployResult

// AddError adds an error to the result
func (r *core.DeployResult) AddError(err string) {
	if r.Errors == nil {
		r.Errors = []string{}
	}
	r.Errors = append(r.Errors, err)
}

// AddWarning adds a warning to the result
func (r *core.DeployResult) AddWarning(warning string) {
	if r.Warnings == nil {
		r.Warnings = []string{}
	}
	r.Warnings = append(r.Warnings, warning)
}

// SetData sets data in the result
func (r *core.DeployResult) SetData(key string, value interface{}) {
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	r.Data[key] = value
}
