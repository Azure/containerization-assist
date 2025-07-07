package deploy

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/infra/templates"
)

// GenerateManifestsTool generates Kubernetes manifests using external templates
type GenerateManifestsTool struct {
	logger *slog.Logger
}

// NewGenerateManifestsTool creates a new instance of GenerateManifestsTool
func NewGenerateManifestsTool(logger *slog.Logger, workspaceDir ...string) types.Tool {
	return &GenerateManifestsTool{
		logger: logger.With("tool", "generate_manifests"),
	}
}

// Name returns the tool name
func (t *GenerateManifestsTool) Name() string {
	return "generate_manifests"
}

// Description returns the tool description
func (t *GenerateManifestsTool) Description() string {
	return "Generates Kubernetes manifests for application deployment"
}

// Schema returns the tool schema
func (t *GenerateManifestsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "generate_manifests",
		Description: "Generates Kubernetes manifests for application deployment",
		Version:     "1.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"app_name": map[string]interface{}{
					"type":        "string",
					"description": "Application name",
				},
				"image_reference": map[string]interface{}{
					"type":        "string",
					"description": "Container image reference",
				},
				"port": map[string]interface{}{
					"type":        "integer",
					"description": "Application port",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace",
				},
				"include_ingress": map[string]interface{}{
					"type":        "boolean",
					"description": "Include ingress resource",
				},
				"ingress_host": map[string]interface{}{
					"type":        "string",
					"description": "Ingress hostname",
				},
			},
			"required": []string{"app_name", "image_reference"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"manifests": map[string]interface{}{
					"type":        "array",
					"description": "Generated manifest files",
				},
				"output_dir": map[string]interface{}{
					"type":        "string",
					"description": "Output directory path",
				},
			},
		},
	}
}

// Execute generates Kubernetes manifests
func (t *GenerateManifestsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse and validate input
	manifestArgs, err := t.parseAndValidateInput(input)
	if err != nil {
		return t.createErrorOutput("Input validation failed", err), err
	}

	// Execute manifest generation
	result, err := t.executeManifestGeneration(ctx, manifestArgs)
	if err != nil {
		return t.createErrorOutput("Manifest generation failed", err), err
	}

	// Format and return response
	return t.formatManifestResponse(result), nil
}

// parseAndValidateInput parses and validates the tool input
func (t *GenerateManifestsTool) parseAndValidateInput(input api.ToolInput) (*GenerateManifestsArgs, error) {
	var manifestArgs GenerateManifestsArgs

	// Extract fields from input.Data
	if appName, ok := input.Data["app_name"].(string); ok {
		manifestArgs.AppName = appName
	}
	if imageRef, ok := input.Data["image_reference"].(string); ok {
		manifestArgs.ImageReference = imageRef
	}
	if port, ok := input.Data["port"].(int); ok {
		manifestArgs.Port = port
	}
	if namespace, ok := input.Data["namespace"].(string); ok {
		manifestArgs.Namespace = namespace
	}
	if includeIngress, ok := input.Data["include_ingress"].(bool); ok {
		manifestArgs.IncludeIngress = includeIngress
	}
	if ingressHost, ok := input.Data["ingress_host"].(string); ok {
		manifestArgs.IngressHost = ingressHost
	}

	manifestArgs.SessionID = input.SessionID

	// Set defaults
	if manifestArgs.AppName == "" {
		manifestArgs.AppName = "app"
	}
	if manifestArgs.Namespace == "" {
		manifestArgs.Namespace = "default"
	}

	t.logger.Info("Starting manifest generation",
		"app_name", manifestArgs.AppName,
		"namespace", manifestArgs.Namespace)

	return &manifestArgs, nil
}

// executeManifestGeneration executes the manifest generation process
func (t *GenerateManifestsTool) executeManifestGeneration(ctx context.Context, args *GenerateManifestsArgs) (*kubernetes.ManifestGenerationResult, error) {
	startTime := time.Now()

	// Create manifest directory
	manifestDir := "manifests"
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return nil, errors.NewTypedError(
			"generate_manifests",
			"failed to create manifest directory",
			errors.CategoryResource,
		)
	}

	result := &kubernetes.ManifestGenerationResult{
		Success:   true,
		Manifests: []kubernetes.GeneratedManifest{},
		OutputDir: manifestDir,
		Duration:  0, // Will be set at the end
		Context:   make(map[string]interface{}),
	}

	// Generate deployment manifest
	if err := t.generateDeploymentManifest(args, result); err != nil {
		return nil, err
	}

	// Generate service manifest
	if err := t.generateServiceManifest(args, result); err != nil {
		return nil, err
	}

	// Generate ingress manifest if requested
	if err := t.generateIngressManifest(args, result); err != nil {
		return nil, err
	}

	result.Duration = time.Since(startTime)

	t.logger.Info("Manifest generation completed",
		"manifest_count", len(result.Manifests),
		"output_dir", result.OutputDir,
		"duration", result.Duration)

	return result, nil
}

// generateDeploymentManifest generates the deployment manifest
func (t *GenerateManifestsTool) generateDeploymentManifest(args *GenerateManifestsArgs, result *kubernetes.ManifestGenerationResult) error {
	deploymentData := map[string]interface{}{
		"AppName":       args.AppName,
		"Namespace":     args.Namespace,
		"Replicas":      1, // Default replicas
		"Image":         args.ImageReference,
		"ContainerPort": args.Port,
	}

	// Add resource limits if provided
	if args.CPURequest != "" || args.MemoryRequest != "" || args.CPULimit != "" || args.MemoryLimit != "" {
		resources := t.buildResourcesConfig(args)
		deploymentData["Resources"] = resources
	}

	// Add environment variables if provided
	if len(args.Environment) > 0 {
		deploymentData["EnvironmentVars"] = args.Environment
	}

	deploymentManifest, err := templates.RenderManifest("deployment", deploymentData)
	if err != nil {
		return errors.NewTypedError(
			"generate_manifests",
			"failed to render deployment manifest",
			errors.CategoryInternal,
		)
	}

	deploymentInfo, err := t.writeManifestToFile(result.OutputDir, "deployment.yaml", deploymentManifest)
	if err != nil {
		return err
	}
	result.Manifests = append(result.Manifests, *deploymentInfo)
	return nil
}

// generateServiceManifest generates the service manifest
func (t *GenerateManifestsTool) generateServiceManifest(args *GenerateManifestsArgs, result *kubernetes.ManifestGenerationResult) error {
	serviceData := map[string]interface{}{
		"AppName":     args.AppName,
		"Namespace":   args.Namespace,
		"ServiceType": "ClusterIP", // Default service type
		"ServicePort": 80,
		"TargetPort":  args.Port,
	}

	serviceManifest, err := templates.RenderManifest("service", serviceData)
	if err != nil {
		return errors.NewTypedError(
			"generate_manifests",
			"failed to render service manifest",
			errors.CategoryInternal,
		)
	}

	serviceInfo, err := t.writeManifestToFile(result.OutputDir, "service.yaml", serviceManifest)
	if err != nil {
		return err
	}
	result.Manifests = append(result.Manifests, *serviceInfo)
	return nil
}

// generateIngressManifest generates the ingress manifest if enabled
func (t *GenerateManifestsTool) generateIngressManifest(args *GenerateManifestsArgs, result *kubernetes.ManifestGenerationResult) error {
	if !args.IncludeIngress || args.IngressHost == "" {
		return nil // Skip ingress generation
	}

	ingressData := map[string]interface{}{
		"AppName":     args.AppName,
		"Namespace":   args.Namespace,
		"Host":        args.IngressHost,
		"Path":        "/",
		"PathType":    "Prefix",
		"ServicePort": 80,
	}

	ingressManifest, err := templates.RenderManifest("ingress", ingressData)
	if err != nil {
		return errors.NewTypedError(
			"generate_manifests",
			"failed to render ingress manifest",
			errors.CategoryInternal,
		)
	}

	ingressInfo, err := t.writeManifestToFile(result.OutputDir, "ingress.yaml", ingressManifest)
	if err != nil {
		return err
	}
	result.Manifests = append(result.Manifests, *ingressInfo)
	return nil
}

// buildResourcesConfig builds the resources configuration map
func (t *GenerateManifestsTool) buildResourcesConfig(args *GenerateManifestsArgs) map[string]interface{} {
	resources := map[string]interface{}{
		"requests": make(map[string]string),
		"limits":   make(map[string]string),
	}

	if args.CPURequest != "" {
		resources["requests"].(map[string]string)["cpu"] = args.CPURequest
	}
	if args.MemoryRequest != "" {
		resources["requests"].(map[string]string)["memory"] = args.MemoryRequest
	}
	if args.CPULimit != "" {
		resources["limits"].(map[string]string)["cpu"] = args.CPULimit
	}
	if args.MemoryLimit != "" {
		resources["limits"].(map[string]string)["memory"] = args.MemoryLimit
	}

	return resources
}

// formatManifestResponse formats the manifest generation response
func (t *GenerateManifestsTool) formatManifestResponse(result *kubernetes.ManifestGenerationResult) api.ToolOutput {
	return api.ToolOutput{
		Success: result.Success,
		Data: map[string]interface{}{
			"manifests":      result.Manifests,
			"output_dir":     result.OutputDir,
			"manifest_count": len(result.Manifests),
			"duration":       result.Duration.String(),
		},
	}
}

// createErrorOutput creates an error output
func (t *GenerateManifestsTool) createErrorOutput(message string, err error) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Error:   message + ": " + err.Error(),
		Data: map[string]interface{}{
			"error": err.Error(),
		},
	}
}

// writeManifestToFile writes a manifest to a file
func (t *GenerateManifestsTool) writeManifestToFile(dir, filename, content string) (*kubernetes.GeneratedManifest, error) {
	filePath := filepath.Join(dir, filename)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, errors.NewTypedError(
			"generate_manifests",
			"failed to write manifest file",
			errors.CategoryResource,
		)
	}

	// Extract resource info from filename
	kind := "Unknown"
	name := filename[:len(filename)-5] // Remove .yaml extension
	switch filename {
	case "deployment.yaml":
		kind = "Deployment"
	case "service.yaml":
		kind = "Service"
	case "configmap.yaml":
		kind = "ConfigMap"
	case "ingress.yaml":
		kind = "Ingress"
	}

	return &kubernetes.GeneratedManifest{
		Name:    name,
		Kind:    kind,
		Path:    filePath,
		Content: content,
	}, nil
}

// GetSchema returns the schema for this tool (simplified)
func (t *GenerateManifestsTool) GetSchema() interface{} {
	return GenerateManifestsArgs{}
}

// Supporting types remain the same...

type RegistrySecret struct {
	Name     string `json:"name"`
	Registry string `json:"registry"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

type ValidationOptions struct {
	StrictMode       bool     `json:"strict_mode,omitempty"`
	SkipUnknownKeys  bool     `json:"skip_unknown_keys,omitempty"`
	AllowedResources []string `json:"allowed_resources,omitempty"`
}

type NetworkPolicySpec struct {
	PodSelector []map[string]string `json:"pod_selector,omitempty"`
}

type ManifestInfo struct {
	FileName     string `json:"file_name"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Namespace    string `json:"namespace,omitempty"`
}

type ManifestValidationSummary struct {
	Valid        bool `json:"valid"`
	TotalFiles   int  `json:"total_files"`
	ValidFiles   int  `json:"valid_files"`
	InvalidFiles int  `json:"invalid_files"`
}
