package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/tools/manifests"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ManifestsAdapter adapts the refactored manifests modules to work with AtomicGenerateManifestsTool
type ManifestsAdapter struct {
	k8sGenerator      *manifests.K8sManifestGenerator
	secretsHandler    *manifests.SecretsHandler
	templateProcessor *manifests.TemplateProcessor
	validator         *manifests.ManifestValidator
	logger            zerolog.Logger
}

// NewManifestsAdapter creates a new adapter for the refactored manifests modules
func NewManifestsAdapter(pipelineAdapter mcptypes.PipelineOperations, logger zerolog.Logger) *ManifestsAdapter {
	// Create wrapper for pipeline operations
	wrapper := &manifestsPipelineWrapper{adapter: pipelineAdapter}

	return &ManifestsAdapter{
		k8sGenerator:      manifests.NewK8sManifestGenerator(wrapper, logger),
		secretsHandler:    manifests.NewSecretsHandler(logger),
		templateProcessor: manifests.NewTemplateProcessor(logger),
		validator:         manifests.NewManifestValidator(logger),
		logger:            logger.With().Str("component", "manifests_adapter").Logger(),
	}
}

// manifestsPipelineWrapper wraps mcptypes.PipelineOperations to implement manifests.PipelineAdapter
type manifestsPipelineWrapper struct {
	adapter mcptypes.PipelineOperations
}

func (w *manifestsPipelineWrapper) GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*kubernetes.ManifestGenerationResult, error) {
	manifestResult, err := w.adapter.GenerateKubernetesManifests(sessionID, imageRef, appName, port, cpuRequest, memoryRequest, cpuLimit, memoryLimit)
	if err != nil {
		return nil, err
	}

	// Convert from mcptypes.KubernetesManifestResult to kubernetes.ManifestGenerationResult
	if manifestResult == nil {
		return nil, nil
	}

	result := &kubernetes.ManifestGenerationResult{
		Success: manifestResult.Success,
	}

	if manifestResult.Error != nil {
		result.Error = &kubernetes.ManifestError{
			Type:    manifestResult.Error.Type,
			Message: manifestResult.Error.Message,
		}
	}

	// Convert manifests
	for _, manifest := range manifestResult.Manifests {
		result.Manifests = append(result.Manifests, kubernetes.GeneratedManifest{
			Kind:    manifest.Kind,
			Name:    manifest.Name,
			Path:    manifest.Path,
			Content: manifest.Content,
		})
	}

	return result, nil
}

// GenerateManifestsWithModules generates manifests using the refactored modules
func (a *ManifestsAdapter) GenerateManifestsWithModules(ctx context.Context, args AtomicGenerateManifestsArgs, session *sessiontypes.SessionState, workspaceDir string) (*AtomicGenerateManifestsResult, error) {
	startTime := time.Now()

	// Initialize result
	result := &AtomicGenerateManifestsResult{
		Success:      true,
		SessionID:    args.SessionID,
		WorkspaceDir: workspaceDir,
		ImageRef:     args.ImageRef,
		AppName:      args.AppName,
		Namespace:    args.Namespace,
		ManifestContext: &ManifestContext{
			ResourceTypes:    []string{},
			DeploymentConfig: make(map[string]interface{}),
			BestPractices:    []string{},
			SecurityIssues:   []string{},
			NextSteps:        []string{},
			DeploymentTips:   []string{},
		},
	}

	// Convert to manifests types
	manifestsRequest := manifests.GenerateManifestsRequest{
		SessionID:      args.SessionID,
		ImageReference: args.ImageRef,
		AppName:        a.getAppName(args.AppName, args.ImageRef),
		Port:           a.getPort(args.Port),
		Namespace:      a.getNamespace(args.Namespace),
		CPURequest:     args.CPURequest,
		MemoryRequest:  args.MemoryRequest,
		CPULimit:       args.CPULimit,
		MemoryLimit:    args.MemoryLimit,
		Environment:    a.convertEnvironmentToSecretRefs(args.Environment),
		IncludeIngress: args.IncludeIngress,
	}

	// Step 1: Scan for secrets
	secrets, err := a.secretsHandler.ScanForSecrets(manifestsRequest.Environment)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to scan for secrets")
		return nil, types.NewRichError("SECRETS_SCAN_FAILED", "failed to scan for secrets: "+err.Error(), "security_error")
	}

	result.ManifestContext.SecretsDetected = len(secrets)
	result.SecretsDetected = a.convertToDetectedSecrets(secrets)

	// Step 2: Externalize secrets if found
	if len(secrets) > 0 {
		externalizedEnv, err := a.secretsHandler.ExternalizeSecrets(manifestsRequest.Environment, secrets)
		if err != nil {
			return nil, types.NewRichError("SECRETS_EXTERNALIZATION_FAILED", "failed to externalize secrets: "+err.Error(), "security_error")
		}
		manifestsRequest.Environment = externalizedEnv
		result.ManifestContext.SecretsExternalized = len(secrets)

		// Create secrets plan
		result.SecretsPlan = a.createSecretsPlan(secrets, args)
	}

	// Step 3: Select template
	_, _, err = a.templateProcessor.SelectTemplate(session, manifestsRequest)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to select template, using default")
	}

	// Store template info in deployment strategy context
	if result.DeploymentStrategyContext == nil {
		result.DeploymentStrategyContext = &DeploymentStrategyContext{}
	}
	// Note: ManifestTemplateContext is defined in generate_manifests_types.go

	// Step 4: Generate base manifests
	manifestResult, err := a.k8sGenerator.GenerateManifests(ctx, manifestsRequest)
	if err != nil {
		return nil, types.NewRichError("MANIFEST_GENERATION_FAILED", "failed to generate manifests: "+err.Error(), "manifest_error")
	}

	result.ManifestResult = manifestResult

	// Step 5: Generate additional manifests (ConfigMap, Ingress, Secrets)
	additionalManifests := []GeneratedManifest{}

	// Generate ConfigMap for non-sensitive environment variables
	nonSecretEnv := a.getNonSecretEnvironment(manifestsRequest.Environment, secrets)
	if len(nonSecretEnv) > 0 {
		configMap, err := a.k8sGenerator.GenerateConfigMap(args.AppName, args.Namespace, nonSecretEnv)
		if err != nil {
			a.logger.Error().Err(err).Msg("Failed to generate ConfigMap")
		} else if configMap != nil {
			additionalManifests = append(additionalManifests, GeneratedManifest{
				Name:    configMap.Name,
				Kind:    configMap.Kind,
				Path:    filepath.Join("manifests", fmt.Sprintf("%s-configmap.yaml", args.AppName)),
				Purpose: "Environment configuration",
			})
			result.ManifestContext.ConfigMapsCreated++
		}
	}

	// Generate Ingress if requested
	if args.IncludeIngress {
		ingress, err := a.k8sGenerator.GenerateIngress(args.AppName, args.Namespace, args.AppName+".local", manifestsRequest.Port)
		if err != nil {
			a.logger.Error().Err(err).Msg("Failed to generate Ingress")
		} else if ingress != nil {
			additionalManifests = append(additionalManifests, GeneratedManifest{
				Name:    ingress.Name,
				Kind:    ingress.Kind,
				Path:    filepath.Join("manifests", fmt.Sprintf("%s-ingress.yaml", args.AppName)),
				Purpose: "External access configuration",
			})
		}
	}

	// Generate Secret manifests
	if len(secrets) > 0 {
		secretManifests, err := a.secretsHandler.GenerateSecretManifests(secrets, args.Namespace)
		if err != nil {
			a.logger.Error().Err(err).Msg("Failed to generate secret manifests")
		} else {
			for _, sm := range secretManifests {
				additionalManifests = append(additionalManifests, GeneratedManifest{
					Name:    sm.Name,
					Kind:    sm.Kind,
					Path:    sm.FilePath,
					Purpose: "Secret storage",
				})
			}
			result.SecretManifests = additionalManifests
		}
	}

	// Step 6: Validate all manifests
	allManifests := a.convertToValidatableManifests(manifestResult.Manifests)
	validationResults := a.validator.ValidateManifests(allManifests)

	// Process validation results
	for _, vr := range validationResults {
		if !vr.Valid {
			result.Success = false
			result.ManifestContext.SecurityIssues = append(result.ManifestContext.SecurityIssues, vr.Errors...)
		}
		for _, warning := range vr.Warnings {
			a.logger.Warn().Str("manifest", vr.ManifestName).Msg(warning)
		}
	}

	// Step 7: Save manifests to disk
	manifestDir := filepath.Join(workspaceDir, "manifests")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return nil, types.NewRichError("DIRECTORY_CREATION_FAILED", "failed to create manifest directory: "+err.Error(), "file_error")
	}

	for _, manifest := range manifestResult.Manifests {
		manifestPath := filepath.Join(workspaceDir, manifest.Path)
		if err := os.WriteFile(manifestPath, []byte(manifest.Content), 0644); err != nil {
			a.logger.Error().Err(err).Str("path", manifestPath).Msg("Failed to write manifest file")
		}
	}

	// Update manifest context
	a.updateManifestContext(result, args)

	// Set timing
	result.GenerationDuration = time.Since(startTime)
	result.TotalDuration = result.GenerationDuration

	return result, nil
}

// Helper methods

func (a *ManifestsAdapter) getAppName(appName, imageRef string) string {
	return a.k8sGenerator.GetDefaultAppName(appName, imageRef)
}

func (a *ManifestsAdapter) getNamespace(namespace string) string {
	return a.k8sGenerator.GetDefaultNamespace(namespace)
}

func (a *ManifestsAdapter) getPort(port int) int {
	return a.k8sGenerator.GetDefaultPort(port)
}

func (a *ManifestsAdapter) convertEnvironmentToSecretRefs(environment map[string]string) []manifests.SecretRef {
	var refs []manifests.SecretRef
	for name, value := range environment {
		refs = append(refs, manifests.SecretRef{
			Name:  name,
			Value: value,
		})
	}
	return refs
}

func (a *ManifestsAdapter) convertToDetectedSecrets(secrets []manifests.SecretInfo) []DetectedSecret {
	var detected []DetectedSecret
	for _, secret := range secrets {
		detected = append(detected, DetectedSecret{
			Name:          secret.Name,
			RedactedValue: "***REDACTED***",
			SuggestedRef:  fmt.Sprintf("${%s}", secret.SecretKey),
			Pattern:       secret.Pattern,
		})
	}
	return detected
}

func (a *ManifestsAdapter) createSecretsPlan(secrets []manifests.SecretInfo, args AtomicGenerateManifestsArgs) *SecretsPlan {
	plan := &SecretsPlan{
		Strategy:         args.SecretHandling,
		SecretManager:    args.SecretManager,
		SecretReferences: make(map[string]SecretRef),
		ConfigMapEntries: make(map[string]string),
		Instructions:     []string{},
	}

	if plan.Strategy == "" {
		plan.Strategy = "auto"
	}
	if plan.SecretManager == "" {
		plan.SecretManager = "kubernetes-secrets"
	}

	// Create secret references
	for _, secret := range secrets {
		plan.SecretReferences[secret.Name] = SecretRef{
			Name: secret.SecretName,
			Key:  secret.SecretKey,
		}
	}

	// Add instructions
	plan.Instructions = append(plan.Instructions,
		fmt.Sprintf("Create %d Kubernetes secrets", len(secrets)),
		"Update deployment to reference secrets",
		"Ensure RBAC permissions for secret access",
	)

	return plan
}

func (a *ManifestsAdapter) getNonSecretEnvironment(environment []manifests.SecretRef, secrets []manifests.SecretInfo) map[string]string {
	secretMap := make(map[string]bool)
	for _, secret := range secrets {
		secretMap[secret.Name] = true
	}

	nonSecrets := make(map[string]string)
	for _, env := range environment {
		if !secretMap[env.Name] {
			nonSecrets[env.Name] = env.Value
		}
	}
	return nonSecrets
}

func (a *ManifestsAdapter) convertToValidatableManifests(k8sManifests []kubernetes.GeneratedManifest) []manifests.GeneratedManifest {
	var result []manifests.GeneratedManifest
	for _, m := range k8sManifests {
		result = append(result, manifests.GeneratedManifest{
			Kind:     m.Kind,
			Name:     m.Name,
			Content:  m.Content,
			FilePath: m.Path,
		})
	}
	return result
}

func (a *ManifestsAdapter) updateManifestContext(result *AtomicGenerateManifestsResult, args AtomicGenerateManifestsArgs) {
	ctx := result.ManifestContext

	// Count resources
	if result.ManifestResult != nil {
		ctx.ManifestsGenerated = len(result.ManifestResult.Manifests)
		for _, m := range result.ManifestResult.Manifests {
			ctx.ResourceTypes = append(ctx.ResourceTypes, m.Kind)
		}
		ctx.TotalResources = len(result.ManifestResult.Manifests) + len(result.SecretManifests)
	}

	// Security assessment
	if ctx.SecretsDetected > 0 && ctx.SecretsExternalized == ctx.SecretsDetected {
		ctx.SecurityLevel = "high"
		ctx.BestPractices = append(ctx.BestPractices, "All secrets externalized")
	} else if ctx.SecretsDetected > 0 && ctx.SecretsExternalized > 0 {
		ctx.SecurityLevel = "medium"
		ctx.BestPractices = append(ctx.BestPractices, "Some secrets externalized")
	} else if ctx.SecretsDetected > 0 {
		ctx.SecurityLevel = "low"
		ctx.SecurityIssues = append(ctx.SecurityIssues, "Secrets detected but not externalized")
	} else {
		ctx.SecurityLevel = "high"
		ctx.BestPractices = append(ctx.BestPractices, "No secrets detected in environment")
	}

	// Resource limits
	if args.CPURequest != "" || args.MemoryRequest != "" {
		ctx.ResourceLimitsSet = true
		ctx.BestPractices = append(ctx.BestPractices, "Resource limits configured")
	}

	// Deployment config
	ctx.DeploymentConfig["replicas"] = args.Replicas
	ctx.DeploymentConfig["namespace"] = args.Namespace
	ctx.DeploymentConfig["service_type"] = args.ServiceType

	// Next steps
	ctx.NextSteps = []string{
		"Review generated manifests",
		"Apply manifests to cluster: kubectl apply -f manifests/",
		"Verify deployment: kubectl get pods -n " + args.Namespace,
	}

	// Deployment tips
	ctx.DeploymentTips = []string{
		"Use kubectl diff to preview changes before applying",
		"Consider using a GitOps tool for production deployments",
		"Monitor pod logs after deployment: kubectl logs -f deployment/" + args.AppName,
	}
}
