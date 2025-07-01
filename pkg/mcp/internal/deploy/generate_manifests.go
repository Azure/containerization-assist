package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// Supporting types
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

// GenerateManifestsArgs represents the arguments for the generate_manifests tool
type GenerateManifestsArgs struct {
	types.BaseToolArgs
	AppName            string               `json:"app_name,omitempty" description:"Application name for labels and naming"`
	ImageRef           types.ImageReference `json:"image_ref" description:"Container image reference"`
	Namespace          string               `json:"namespace,omitempty" description:"Kubernetes namespace"`
	ServiceType        string               `json:"service_type,omitempty" description:"Service type (ClusterIP, NodePort, LoadBalancer)"`
	Replicas           int                  `json:"replicas,omitempty" description:"Number of replicas"`
	Resources          ResourceRequests     `json:"resources,omitempty" description:"Resource requirements"`
	Environment        map[string]string    `json:"environment,omitempty" description:"Environment variables"`
	Secrets            []SecretRef          `json:"secrets,omitempty" description:"Secret references"`
	IncludeIngress     bool                 `json:"include_ingress,omitempty" description:"Generate Ingress resource"`
	HelmTemplate       bool                 `json:"helm_template,omitempty" description:"Generate as Helm template"`
	ConfigMapData      map[string]string    `json:"configmap_data,omitempty" description:"ConfigMap data key-value pairs"`
	ConfigMapFiles     map[string]string    `json:"configmap_files,omitempty" description:"ConfigMap file paths to mount"`
	BinaryData         map[string][]byte    `json:"binary_data,omitempty" description:"ConfigMap binary data"`
	IngressHosts       []IngressHost        `json:"ingress_hosts,omitempty" description:"Ingress host configuration"`
	IngressTLS         []IngressTLS         `json:"ingress_tls,omitempty" description:"Ingress TLS configuration"`
	IngressClass       string               `json:"ingress_class,omitempty" description:"Ingress class name"`
	ServicePorts       []ServicePort        `json:"service_ports,omitempty" description:"Service port configuration"`
	LoadBalancerIP     string               `json:"load_balancer_ip,omitempty" description:"LoadBalancer IP for service"`
	SessionAffinity    string               `json:"session_affinity,omitempty" description:"Session affinity (None, ClientIP)"`
	WorkflowLabels     map[string]string    `json:"workflow_labels,omitempty" description:"Additional labels from workflow session"`
	RegistrySecrets    []RegistrySecret     `json:"registry_secrets,omitempty" description:"Registry credentials for pull secrets"`
	GeneratePullSecret bool                 `json:"generate_pull_secret,omitempty" description:"Generate image pull secret"`
	ValidateManifests  bool                 `json:"validate_manifests,omitempty" description:"Validate generated manifests against K8s schemas"`
	ValidationOptions  ValidationOptions    `json:"validation_options,omitempty" description:"Options for manifest validation"`

	// NetworkPolicy configuration
	IncludeNetworkPolicy bool              `json:"include_network_policy,omitempty" description:"Generate NetworkPolicy resource"`
	NetworkPolicySpec    NetworkPolicySpec `json:"network_policy_spec,omitempty" description:"NetworkPolicy specification"`
}

// GenerateManifestsResult represents the result of manifest generation
type GenerateManifestsResult struct {
	types.BaseToolResponse
	Manifests        []ManifestInfo             `json:"manifests"`
	ManifestPath     string                     `json:"manifest_path"`
	ImageRef         types.ImageReference       `json:"image_ref"`
	Namespace        string                     `json:"namespace"`
	ServiceType      string                     `json:"service_type"`
	Replicas         int                        `json:"replicas"`
	Resources        ResourceRequests           `json:"resources"`
	Duration         time.Duration              `json:"duration"`
	ValidationResult *ManifestValidationSummary `json:"validation_result,omitempty"`
	Error            *types.ToolError           `json:"error,omitempty"`
}

// GenerateManifestsTool implements the generate_manifests tool
type GenerateManifestsTool struct {
	logger        zerolog.Logger
	workspaceBase string
	validator     interface{} // Simplified for now
}

// NewGenerateManifestsTool creates a new GenerateManifestsTool
func NewGenerateManifestsTool(logger zerolog.Logger, workspaceBase string) *GenerateManifestsTool {
	return &GenerateManifestsTool{
		logger:        logger.With().Str("tool", "generate_manifests").Logger(),
		workspaceBase: workspaceBase,
	}
}

// NewGenerateManifestsToolWithValidator creates a new GenerateManifestsTool with a validator
func NewGenerateManifestsToolWithValidator(logger zerolog.Logger, workspaceBase string, validator interface{}) *GenerateManifestsTool {
	return &GenerateManifestsTool{
		logger:        logger.With().Str("tool", "generate_manifests").Logger(),
		workspaceBase: workspaceBase,
		validator:     validator,
	}
}

// Execute implements the Tool interface
func (t *GenerateManifestsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	startTime := time.Now()

	// Convert args to typed struct
	var typedArgs GenerateManifestsArgs

	switch v := args.(type) {
	case GenerateManifestsArgs:
		typedArgs = v
	case map[string]interface{}:
		var err error
		typedArgs, err = convertToGenerateManifestsArgs(v)
		if err != nil {
			return nil, fmt.Errorf("failed to convert arguments: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported argument type: %T", args)
	}

	result, err := t.ExecuteTyped(ctx, typedArgs)
	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// ExecuteTyped executes the tool with typed arguments
func (t *GenerateManifestsTool) ExecuteTyped(ctx context.Context, args GenerateManifestsArgs) (*GenerateManifestsResult, error) {
	startTime := time.Now()

	t.logger.Info().
		Str("image_ref", args.ImageRef.String()).
		Str("namespace", args.Namespace).
		Str("service_type", args.ServiceType).
		Int("replicas", args.Replicas).
		Bool("include_ingress", args.IncludeIngress).
		Bool("helm_template", args.HelmTemplate).
		Msg("Starting manifest generation")

	result := &GenerateManifestsResult{
		BaseToolResponse: types.NewBaseResponse("generate_manifests", args.SessionID, args.DryRun),
		Manifests:        []ManifestInfo{},
		ImageRef:         args.ImageRef,
		Namespace:        args.Namespace,
		ServiceType:      args.ServiceType,
		Replicas:         args.Replicas,
		Resources:        args.Resources,
	}

	// Use workspace base directory for manifests
	manifestDir := t.workspaceBase
	// Ensure the directory exists
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create manifest directory: %w", err)
	}
	result.ManifestPath = manifestDir

	// Generate basic deployment manifest
	deploymentManifest := t.generateDeploymentManifest(args)
	manifestInfo, err := t.writeManifestToFile(manifestDir, "deployment.yaml", deploymentManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to write deployment manifest: %w", err)
	}
	result.Manifests = append(result.Manifests, *manifestInfo)

	// Generate service manifest (always generate a default ClusterIP service)
	serviceManifest := t.generateServiceManifest(args)
	manifestInfo, err = t.writeManifestToFile(manifestDir, "service.yaml", serviceManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to write service manifest: %w", err)
	}
	result.Manifests = append(result.Manifests, *manifestInfo)

	result.Duration = time.Since(startTime)

	t.logger.Info().
		Int("manifest_count", len(result.Manifests)).
		Str("manifest_path", result.ManifestPath).
		Dur("duration", result.Duration).
		Msg("Manifest generation completed")

	return result, nil
}

// generateDeploymentManifest generates a basic deployment manifest
func (t *GenerateManifestsTool) generateDeploymentManifest(args GenerateManifestsArgs) string {
	appName := args.AppName
	if appName == "" {
		appName = "app"
	}
	namespace := args.Namespace
	if namespace == "" {
		namespace = "default"
	}
	replicas := args.Replicas
	if replicas == 0 {
		replicas = 1
	}

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: %s
        image: %s
        ports:
        - containerPort: 8080
`, appName, namespace, appName, replicas, appName, appName, appName, args.ImageRef.String())
}

// generateServiceManifest generates a basic service manifest
func (t *GenerateManifestsTool) generateServiceManifest(args GenerateManifestsArgs) string {
	appName := args.AppName
	if appName == "" {
		appName = "app"
	}
	namespace := args.Namespace
	if namespace == "" {
		namespace = "default"
	}
	serviceType := args.ServiceType
	if serviceType == "" {
		serviceType = "ClusterIP"
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s-service
  namespace: %s
  labels:
    app: %s
spec:
  type: %s
  selector:
    app: %s
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
`, appName, namespace, appName, serviceType, appName)
}

// writeManifestToFile writes a manifest to a file
func (t *GenerateManifestsTool) writeManifestToFile(manifestDir string, fileName string, content string) (*ManifestInfo, error) {
	filePath := fmt.Sprintf("%s/%s", manifestDir, fileName)

	err := os.WriteFile(filePath, []byte(content), 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write manifest file: %w", err)
	}

	return &ManifestInfo{
		FileName:     fileName,
		ResourceType: "unknown", // Would need YAML parsing to determine
		ResourceName: "unknown",
		Namespace:    "",
	}, nil
}

// GetName returns the tool name
func (t *GenerateManifestsTool) GetName() string {
	return "generate_manifests"
}

// GetDescription returns the tool description
func (t *GenerateManifestsTool) GetDescription() string {
	return "Generate Kubernetes manifests for container deployment using session-based configuration"
}

// GetVersion returns the tool version
func (t *GenerateManifestsTool) GetVersion() string {
	return "1.0.0"
}

// GetCapabilities returns the tool capabilities
func (t *GenerateManifestsTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		RequiresAuth: false,
	}
}

// Validate validates the tool arguments
func (t *GenerateManifestsTool) Validate(ctx context.Context, args interface{}) error {
	var typedArgs GenerateManifestsArgs

	switch v := args.(type) {
	case GenerateManifestsArgs:
		typedArgs = v
	case map[string]interface{}:
		var err error
		typedArgs, err = convertToGenerateManifestsArgs(v)
		if err != nil {
			return fmt.Errorf("failed to convert arguments: %w", err)
		}
	default:
		return fmt.Errorf("unsupported argument type: %T", args)
	}

	// Validate required fields
	if typedArgs.ImageRef.String() == "" {
		return fmt.Errorf("image_ref is required")
	}

	// Validate service type
	if typedArgs.ServiceType != "" {
		validTypes := map[string]bool{
			"ClusterIP":    true,
			"NodePort":     true,
			"LoadBalancer": true,
			"ExternalName": true,
		}
		if !validTypes[typedArgs.ServiceType] {
			return fmt.Errorf("invalid service_type: %s", typedArgs.ServiceType)
		}
	}

	// Validate replicas
	if typedArgs.Replicas < 0 {
		return fmt.Errorf("replicas must be non-negative")
	}

	return nil
}

// GetMetadata returns tool metadata
func (t *GenerateManifestsTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:         t.GetName(),
		Description:  t.GetDescription(),
		Version:      t.GetVersion(),
		Category:     "deployment",
		Dependencies: []string{},
		Capabilities: []string{"kubernetes", "manifests"},
		Requirements: []string{},
		Parameters:   map[string]string{},
		Examples:     []mcptypes.ToolExample{},
	}
}

// Copilot-compatible wrapper functions

// GenerateManifests is a Copilot-compatible wrapper that accepts untyped arguments
func GenerateManifests(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	workspaceBase := "/tmp/container-kit"

	tool := NewGenerateManifestsTool(logger, workspaceBase)

	// Convert untyped map to typed args
	typedArgs, err := convertToGenerateManifestsArgs(args)
	if err != nil {
		return nil, err
	}

	// Execute with typed args
	result, err := tool.ExecuteTyped(ctx, typedArgs)
	if err != nil {
		return nil, err
	}

	// Convert result to untyped map
	return convertGenerateManifestsResultToMap(result), nil
}

// convertToGenerateManifestsArgs converts untyped map to typed GenerateManifestsArgs
func convertToGenerateManifestsArgs(args map[string]interface{}) (GenerateManifestsArgs, error) {
	jsonBytes, err := json.Marshal(args)
	if err != nil {
		return GenerateManifestsArgs{}, fmt.Errorf("failed to marshal args: %w", err)
	}

	var result GenerateManifestsArgs
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return GenerateManifestsArgs{}, fmt.Errorf("failed to unmarshal args: %w", err)
	}

	return result, nil
}

// convertGenerateManifestsResultToMap converts typed result to untyped map
func convertGenerateManifestsResultToMap(result *GenerateManifestsResult) map[string]interface{} {
	jsonBytes, _ := json.Marshal(result)
	var resultMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &resultMap); err != nil {
		// Return empty map on error
		return make(map[string]interface{})
	}
	return resultMap
}
