package deploy

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/Azure/container-kit/pkg/mcp/session"
)

// TypeSafeKubernetesDeployTool implements the new type-safe api.TypedDeployTool interface
type TypeSafeKubernetesDeployTool struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore    services.SessionStore         // Modern service interface
	sessionState    services.SessionState         // Modern service interface
	logger          *slog.Logger
	timeout         time.Duration
	atomicTool      interface{} // TODO: Define proper atomic deploy tool
}

// NewTypeSafeKubernetesDeployTool creates a new type-safe Kubernetes deploy tool (legacy constructor)
func NewTypeSafeKubernetesDeployTool(
	adapter core.TypedPipelineOperations,
	sessionManager session.UnifiedSessionManager,
	logger *slog.Logger,
) api.TypedDeployTool {
	return &TypeSafeKubernetesDeployTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With("tool", "typesafe_kubernetes_deploy"),
		timeout:         10 * time.Minute, // Default deploy timeout
	}
}

// NewTypeSafeKubernetesDeployToolWithServices creates a new type-safe Kubernetes deploy tool using service interfaces
func NewTypeSafeKubernetesDeployToolWithServices(
	adapter core.TypedPipelineOperations,
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) api.TypedDeployTool {
	toolLogger := logger.With("tool", "typesafe_kubernetes_deploy")

	return &TypeSafeKubernetesDeployTool{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
		timeout:         10 * time.Minute, // Default deploy timeout
	}
}

// Name implements api.TypedTool
func (t *TypeSafeKubernetesDeployTool) Name() string {
	return "kubernetes_deploy"
}

// Description implements api.TypedTool
func (t *TypeSafeKubernetesDeployTool) Description() string {
	return "Deploys applications to Kubernetes clusters using manifests"
}

// Execute implements api.TypedTool with type-safe input and output
func (t *TypeSafeKubernetesDeployTool) Execute(
	ctx context.Context,
	input api.TypedToolInput[api.TypedDeployInput, api.DeployContext],
) (api.TypedToolOutput[api.TypedDeployOutput, api.DeployDetails], error) {
	// Telemetry execution removed

	return t.executeInternal(ctx, input)
}

// executeInternal contains the core execution logic
func (t *TypeSafeKubernetesDeployTool) executeInternal(
	ctx context.Context,
	input api.TypedToolInput[api.TypedDeployInput, api.DeployContext],
) (api.TypedToolOutput[api.TypedDeployOutput, api.DeployDetails], error) {
	startTime := time.Now()

	// Validate input
	if err := t.validateInput(input); err != nil {
		return api.TypedToolOutput[api.TypedDeployOutput, api.DeployDetails]{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	t.logger.Info("Starting Kubernetes deployment",
		"session_id", input.SessionID,
		"manifests", input.Data.Manifests,
		"namespace", input.Data.Namespace,
		"dry_run", input.Data.DryRun,
		"wait", input.Data.Wait)

	// Create or get session
	sess, err := t.sessionManager.GetOrCreateSession(ctx, input.SessionID)
	if err != nil {
		return t.errorOutput(input.SessionID, "Failed to get or create session", err), err
	}

	// Update session state
	sess.AddLabel("deploying")
	sess.UpdateLastAccessed()

	// Execute deployment
	deployResult, err := t.executeDeploy(ctx, input)
	if err != nil {
		// Check if rollback is needed
		if input.Context.RollbackOnFailure {
			t.logger.Info("Attempting rollback due to deployment failure")
			if rollbackErr := t.executeRollback(ctx, input); rollbackErr != nil {
				t.logger.Error("Rollback failed", "error", rollbackErr)
			}
		}
		return t.errorOutput(input.SessionID, "Deployment failed", err), err
	}

	// Store deployment results in session
	sess.RemoveLabel("deploying")
	sess.AddLabel("deployment_completed")

	// Add execution record
	endTime := time.Now()
	sess.AddToolExecution(session.ToolExecution{
		Tool:      "kubernetes_deploy",
		StartTime: startTime,
		EndTime:   &endTime,
		Success:   err == nil,
	})

	// Build output
	output := api.TypedDeployOutput{
		Success:         true,
		SessionID:       input.SessionID,
		DeployedObjects: t.convertDeployedObjects(deployResult.Objects),
		DeployMetrics: api.DeployMetrics{
			DeployTime:       time.Since(startTime),
			ObjectsCreated:   deployResult.Created,
			ObjectsUpdated:   deployResult.Updated,
			ObjectsDeleted:   deployResult.Deleted,
			RollbackRequired: false,
		},
	}

	// Build details
	details := api.DeployDetails{
		ExecutionDetails: api.ExecutionDetails{
			Duration:  time.Since(startTime),
			StartTime: startTime,
			EndTime:   time.Now(),
			ResourcesUsed: api.ResourceUsage{
				CPUTime:    int64(time.Since(startTime).Milliseconds()),
				MemoryPeak: 0, // TODO: Implement resource tracking
				NetworkIO:  0,
				DiskIO:     0,
			},
		},
		ResourcesCreated:  deployResult.CreatedResources,
		ResourcesUpdated:  deployResult.UpdatedResources,
		ResourcesDeleted:  deployResult.DeletedResources,
		RollbackPerformed: false,
	}

	t.logger.Info("Kubernetes deployment completed",
		"session_id", input.SessionID,
		"duration", time.Since(startTime),
		"objects_created", deployResult.Created,
		"objects_updated", deployResult.Updated,
		"objects_deleted", deployResult.Deleted,
		"namespace", input.Data.Namespace)

	return api.TypedToolOutput[api.TypedDeployOutput, api.DeployDetails]{
		Success: true,
		Data:    output,
		Details: details,
	}, nil
}

// Schema implements api.TypedTool
func (t *TypeSafeKubernetesDeployTool) Schema() api.TypedToolSchema[api.TypedDeployInput, api.DeployContext, api.TypedDeployOutput, api.DeployDetails] {
	return api.TypedToolSchema[api.TypedDeployInput, api.DeployContext, api.TypedDeployOutput, api.DeployDetails]{
		Name:        t.Name(),
		Description: t.Description(),
		Version:     "2.0.0",
		InputExample: api.TypedToolInput[api.TypedDeployInput, api.DeployContext]{
			SessionID: "example-session-123",
			Data: api.TypedDeployInput{
				SessionID: "example-session-123",
				Manifests: []string{"deployment.yaml", "service.yaml"},
				Namespace: "production",
				DryRun:    false,
				Wait:      true,
				Timeout:   5 * time.Minute,
			},
			Context: api.DeployContext{
				ExecutionContext: api.ExecutionContext{
					RequestID: "req-123",
					TraceID:   "trace-456",
					Timeout:   10 * time.Minute,
				},
				Environment:       "production",
				RollbackOnFailure: true,
				MaxRetries:        3,
			},
		},
		OutputExample: api.TypedToolOutput[api.TypedDeployOutput, api.DeployDetails]{
			Success: true,
			Data: api.TypedDeployOutput{
				Success:   true,
				SessionID: "example-session-123",
				DeployedObjects: []api.DeployedObject{
					{
						Kind:      "Deployment",
						Name:      "myapp",
						Namespace: "production",
						Version:   "apps/v1",
						Status:    "Ready",
					},
				},
			},
		},
		Tags:     []string{"deploy", "kubernetes", "k8s"},
		Category: api.CategoryDeploy,
	}
}

// validateInput validates the typed input
func (t *TypeSafeKubernetesDeployTool) validateInput(input api.TypedToolInput[api.TypedDeployInput, api.DeployContext]) error {
	if input.SessionID == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Session ID is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	if len(input.Data.Manifests) == 0 {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("At least one manifest is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	return nil
}

// executeDeploy performs the actual deployment operation
func (t *TypeSafeKubernetesDeployTool) executeDeploy(ctx context.Context, input api.TypedToolInput[api.TypedDeployInput, api.DeployContext]) (*DeployResult, error) {
	// Simplified implementation for now - TODO: Implement proper deployment logic
	// return placeholder success result

	return &DeployResult{
		Objects:          []interface{}{},
		Created:          1,
		Updated:          0,
		Deleted:          0,
		CreatedResources: []string{input.Data.Namespace}, // Using namespace as placeholder
		UpdatedResources: []string{},
		DeletedResources: []string{},
	}, nil
}

// executeRollback performs rollback on failure
func (t *TypeSafeKubernetesDeployTool) executeRollback(ctx context.Context, input api.TypedToolInput[api.TypedDeployInput, api.DeployContext]) error {
	t.logger.Info("Executing deployment rollback",
		"session_id", input.SessionID,
		"namespace", input.Data.Namespace)

	// Implementation would depend on how rollback is handled
	// This is a placeholder
	return nil
}

// convertDeployedObjects converts internal objects to API format
func (t *TypeSafeKubernetesDeployTool) convertDeployedObjects(objects []interface{}) []api.DeployedObject {
	result := make([]api.DeployedObject, 0, len(objects))

	for _, obj := range objects {
		// Convert based on actual object structure
		// This is a simplified version
		if m, ok := obj.(map[string]interface{}); ok {
			deployed := api.DeployedObject{
				Kind:      t.getStringFromMap(m, "kind"),
				Name:      t.getStringFromMap(m, "name"),
				Namespace: t.getStringFromMap(m, "namespace"),
				Version:   t.getStringFromMap(m, "apiVersion"),
				Status:    "Unknown",
			}
			result = append(result, deployed)
		}
	}

	return result
}

// getStringFromMap safely gets a string from a map
func (t *TypeSafeKubernetesDeployTool) getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// errorOutput creates an error output
func (t *TypeSafeKubernetesDeployTool) errorOutput(sessionID, message string, err error) api.TypedToolOutput[api.TypedDeployOutput, api.DeployDetails] {
	return api.TypedToolOutput[api.TypedDeployOutput, api.DeployDetails]{
		Success: false,
		Data: api.TypedDeployOutput{
			Success:   false,
			SessionID: sessionID,
			ErrorMsg:  fmt.Sprintf("%s: %v", message, err),
		},
		Error: err.Error(),
	}
}

// DeployResult represents internal deployment result
type DeployResult struct {
	Objects          []interface{}
	Created          int
	Updated          int
	Deleted          int
	CreatedResources []string
	UpdatedResources []string
	DeletedResources []string
}
