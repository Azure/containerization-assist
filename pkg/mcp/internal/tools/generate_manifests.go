package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	customizerk8s "github.com/Azure/container-copilot/pkg/mcp/internal/customizer/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/mcperror"
	"github.com/Azure/container-copilot/pkg/mcp/internal/ops"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/templates"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// GenerateManifestsArgs represents the arguments for the generate_manifests tool
type GenerateManifestsArgs struct {
	types.BaseToolArgs
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
	IncludeNetworkPolicy bool               `json:"include_network_policy,omitempty" description:"Generate NetworkPolicy resource"`
	NetworkPolicySpec    *NetworkPolicySpec `json:"network_policy_spec,omitempty" description:"NetworkPolicy specification"`
}

// SecretRef represents a reference to a Kubernetes secret
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	Env  string `json:"env"`
}

// ResourceRequests represents Kubernetes resource requirements
type ResourceRequests struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

// IngressHost represents an ingress host configuration
type IngressHost struct {
	Host  string        `json:"host"`
	Paths []IngressPath `json:"paths"`
}

// IngressPath represents a path in an ingress rule
type IngressPath struct {
	Path        string `json:"path"`
	PathType    string `json:"path_type,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	ServicePort int    `json:"service_port,omitempty"`
}

// IngressTLS represents TLS configuration for ingress
type IngressTLS struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secret_name"`
}

// ServicePort represents a port in a service
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port,omitempty"`
	NodePort   int    `json:"node_port,omitempty"`
}

// ValidationOptions holds options for manifest validation
type ValidationOptions struct {
	K8sVersion           string   `json:"k8s_version,omitempty" description:"Target Kubernetes version"`
	SkipDryRun           bool     `json:"skip_dry_run,omitempty" description:"Skip dry-run validation"`
	SkipSchemaValidation bool     `json:"skip_schema_validation,omitempty" description:"Skip schema validation"`
	AllowedKinds         []string `json:"allowed_kinds,omitempty" description:"List of allowed resource kinds"`
	RequiredLabels       []string `json:"required_labels,omitempty" description:"List of required labels"`
	ForbiddenFields      []string `json:"forbidden_fields,omitempty" description:"List of forbidden fields"`
	StrictValidation     bool     `json:"strict_validation,omitempty" description:"Enable strict validation mode"`
}

// RegistrySecret represents registry authentication credentials
type RegistrySecret struct {
	Registry string `json:"registry"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

// ValidationSummary represents the summary of validation results
type ValidationSummary struct {
	Enabled      bool                      `json:"enabled"`
	OverallValid bool                      `json:"overall_valid"`
	TotalFiles   int                       `json:"total_files"`
	ValidFiles   int                       `json:"valid_files"`
	ErrorCount   int                       `json:"error_count"`
	WarningCount int                       `json:"warning_count"`
	Duration     time.Duration             `json:"duration"`
	K8sVersion   string                    `json:"k8s_version,omitempty"`
	Results      map[string]FileValidation `json:"results"`
}

// FileValidation represents validation results for a single file
type FileValidation struct {
	Valid        bool              `json:"valid"`
	Kind         string            `json:"kind"`
	APIVersion   string            `json:"api_version,omitempty"`
	Name         string            `json:"name,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	ErrorCount   int               `json:"error_count"`
	WarningCount int               `json:"warning_count"`
	Duration     time.Duration     `json:"duration"`
	Errors       []ValidationIssue `json:"errors,omitempty"`
	Warnings     []ValidationIssue `json:"warnings,omitempty"`
	Suggestions  []string          `json:"suggestions,omitempty"`
}

// ValidationIssue represents a validation error or warning
type ValidationIssue struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
}

// NetworkPolicySpec represents the specification of a NetworkPolicy
type NetworkPolicySpec struct {
	PolicyTypes []string               `json:"policy_types,omitempty" description:"Types of policies (Ingress, Egress)"`
	PodSelector map[string]string      `json:"pod_selector,omitempty" description:"Pods to which this policy applies"`
	Ingress     []NetworkPolicyIngress `json:"ingress,omitempty" description:"Ingress rules"`
	Egress      []NetworkPolicyEgress  `json:"egress,omitempty" description:"Egress rules"`
}

// NetworkPolicyIngress represents an ingress rule in a NetworkPolicy
type NetworkPolicyIngress struct {
	Ports []NetworkPolicyPort `json:"ports,omitempty" description:"Ports affected by this rule"`
	From  []NetworkPolicyPeer `json:"from,omitempty" description:"Sources allowed by this rule"`
}

// NetworkPolicyEgress represents an egress rule in a NetworkPolicy
type NetworkPolicyEgress struct {
	Ports []NetworkPolicyPort `json:"ports,omitempty" description:"Ports affected by this rule"`
	To    []NetworkPolicyPeer `json:"to,omitempty" description:"Destinations allowed by this rule"`
}

// NetworkPolicyPort represents a port in a NetworkPolicy rule
type NetworkPolicyPort struct {
	Protocol string `json:"protocol,omitempty" description:"Protocol (TCP, UDP, SCTP)"`
	Port     string `json:"port,omitempty" description:"Port number or name"`
	EndPort  *int   `json:"endPort,omitempty" description:"End port for range"`
}

// NetworkPolicyPeer represents a peer in a NetworkPolicy rule
type NetworkPolicyPeer struct {
	PodSelector       map[string]string `json:"podSelector,omitempty" description:"Pod selector"`
	NamespaceSelector map[string]string `json:"namespaceSelector,omitempty" description:"Namespace selector"`
	IPBlock           *IPBlock          `json:"ipBlock,omitempty" description:"IP block"`
}

// IPBlock represents an IP block in a NetworkPolicy
type IPBlock struct {
	CIDR   string   `json:"cidr" description:"CIDR block"`
	Except []string `json:"except,omitempty" description:"Exceptions to the CIDR block"`
}

// GenerateManifestsResult represents the result of manifest generation
type GenerateManifestsResult struct {
	types.BaseToolResponse
	Success          bool                 `json:"success"`
	Manifests        []ManifestInfo       `json:"manifests"`
	ManifestPath     string               `json:"manifest_path"`
	ImageRef         types.ImageReference `json:"image_ref"`
	Namespace        string               `json:"namespace"`
	ServiceType      string               `json:"service_type"`
	Replicas         int                  `json:"replicas"`
	Resources        ResourceRequests     `json:"resources"`
	Duration         time.Duration        `json:"duration"`
	ValidationResult *ValidationSummary   `json:"validation_result,omitempty"`
	Error            *types.ToolError     `json:"error,omitempty"`
}

// ManifestInfo represents information about a generated manifest
type ManifestInfo struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

// GenerateManifests is a Copilot-compatible wrapper that accepts untyped arguments
func GenerateManifests(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	workspaceBase := "/tmp/container-copilot"

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
	result := GenerateManifestsArgs{}

	// Base fields
	if sessionID, ok := args["session_id"].(string); ok {
		result.SessionID = sessionID
	}
	if dryRun, ok := args["dry_run"].(bool); ok {
		result.DryRun = dryRun
	}

	// Image reference
	if imageRef, ok := args["image_ref"].(string); ok {
		result.ImageRef = types.ImageReference{
			Registry:   "",
			Repository: imageRef,
			Tag:        "",
		}
	}

	// Basic fields
	if namespace, ok := args["namespace"].(string); ok {
		result.Namespace = namespace
	}
	if serviceType, ok := args["service_type"].(string); ok {
		result.ServiceType = serviceType
	}
	if replicas, ok := args["replicas"].(float64); ok {
		result.Replicas = int(replicas)
	}
	if includeIngress, ok := args["include_ingress"].(bool); ok {
		result.IncludeIngress = includeIngress
	}
	if helmTemplate, ok := args["helm_template"].(bool); ok {
		result.HelmTemplate = helmTemplate
	}
	if ingressClass, ok := args["ingress_class"].(string); ok {
		result.IngressClass = ingressClass
	}
	if loadBalancerIP, ok := args["load_balancer_ip"].(string); ok {
		result.LoadBalancerIP = loadBalancerIP
	}
	if sessionAffinity, ok := args["session_affinity"].(string); ok {
		result.SessionAffinity = sessionAffinity
	}
	if generatePullSecret, ok := args["generate_pull_secret"].(bool); ok {
		result.GeneratePullSecret = generatePullSecret
	}
	if validateManifests, ok := args["validate_manifests"].(bool); ok {
		result.ValidateManifests = validateManifests
	}

	// Resources
	if resources, ok := args["resources"].(map[string]interface{}); ok {
		result.Resources = ResourceRequests{
			CPURequest:    getStringValue(resources, "cpu_request"),
			MemoryRequest: getStringValue(resources, "memory_request"),
			CPULimit:      getStringValue(resources, "cpu_limit"),
			MemoryLimit:   getStringValue(resources, "memory_limit"),
		}
	}

	// Environment variables
	if env, ok := args["environment"].(map[string]interface{}); ok {
		result.Environment = make(map[string]string)
		for k, v := range env {
			if str, ok := v.(string); ok {
				result.Environment[k] = str
			}
		}
	}

	// ConfigMap data
	if cmData, ok := args["configmap_data"].(map[string]interface{}); ok {
		result.ConfigMapData = make(map[string]string)
		for k, v := range cmData {
			if str, ok := v.(string); ok {
				result.ConfigMapData[k] = str
			}
		}
	}

	// ConfigMap files
	if cmFiles, ok := args["configmap_files"].(map[string]interface{}); ok {
		result.ConfigMapFiles = make(map[string]string)
		for k, v := range cmFiles {
			if str, ok := v.(string); ok {
				result.ConfigMapFiles[k] = str
			}
		}
	}

	// Workflow labels
	if labels, ok := args["workflow_labels"].(map[string]interface{}); ok {
		result.WorkflowLabels = make(map[string]string)
		for k, v := range labels {
			if str, ok := v.(string); ok {
				result.WorkflowLabels[k] = str
			}
		}
	}

	// Secrets
	if secrets, ok := args["secrets"].([]interface{}); ok {
		for _, s := range secrets {
			if secretMap, ok := s.(map[string]interface{}); ok {
				secret := SecretRef{
					Name: getStringValue(secretMap, "name"),
					Key:  getStringValue(secretMap, "key"),
					Env:  getStringValue(secretMap, "env"),
				}
				result.Secrets = append(result.Secrets, secret)
			}
		}
	}

	// Ingress hosts
	if hosts, ok := args["ingress_hosts"].([]interface{}); ok {
		for _, h := range hosts {
			if hostMap, ok := h.(map[string]interface{}); ok {
				host := IngressHost{
					Host: getStringValue(hostMap, "host"),
				}
				if paths, ok := hostMap["paths"].([]interface{}); ok {
					for _, p := range paths {
						if pathMap, ok := p.(map[string]interface{}); ok {
							path := IngressPath{
								Path:        getStringValue(pathMap, "path"),
								PathType:    getStringValue(pathMap, "path_type"),
								ServiceName: getStringValue(pathMap, "service_name"),
								ServicePort: getIntValue(pathMap, "service_port"),
							}
							host.Paths = append(host.Paths, path)
						}
					}
				}
				result.IngressHosts = append(result.IngressHosts, host)
			}
		}
	}

	// Ingress TLS
	if tlsList, ok := args["ingress_tls"].([]interface{}); ok {
		for _, t := range tlsList {
			if tlsMap, ok := t.(map[string]interface{}); ok {
				tls := IngressTLS{
					SecretName: getStringValue(tlsMap, "secret_name"),
				}
				if hosts, ok := tlsMap["hosts"].([]interface{}); ok {
					for _, h := range hosts {
						if host, ok := h.(string); ok {
							tls.Hosts = append(tls.Hosts, host)
						}
					}
				}
				result.IngressTLS = append(result.IngressTLS, tls)
			}
		}
	}

	// Service ports
	if ports, ok := args["service_ports"].([]interface{}); ok {
		for _, p := range ports {
			if portMap, ok := p.(map[string]interface{}); ok {
				port := ServicePort{
					Name:       getStringValue(portMap, "name"),
					Protocol:   getStringValue(portMap, "protocol"),
					Port:       getIntValue(portMap, "port"),
					TargetPort: getIntValue(portMap, "target_port"),
					NodePort:   getIntValue(portMap, "node_port"),
				}
				result.ServicePorts = append(result.ServicePorts, port)
			}
		}
	}

	// Registry secrets
	if regSecrets, ok := args["registry_secrets"].([]interface{}); ok {
		for _, r := range regSecrets {
			if regMap, ok := r.(map[string]interface{}); ok {
				regSecret := RegistrySecret{
					Registry: getStringValue(regMap, "registry"),
					Username: getStringValue(regMap, "username"),
					Password: getStringValue(regMap, "password"),
					Email:    getStringValue(regMap, "email"),
				}
				result.RegistrySecrets = append(result.RegistrySecrets, regSecret)
			}
		}
	}

	// Validation options
	if valOpts, ok := args["validation_options"].(map[string]interface{}); ok {
		result.ValidationOptions = ValidationOptions{
			K8sVersion:           getStringValue(valOpts, "k8s_version"),
			SkipDryRun:           getBoolValue(valOpts, "skip_dry_run"),
			SkipSchemaValidation: getBoolValue(valOpts, "skip_schema_validation"),
			StrictValidation:     getBoolValue(valOpts, "strict_validation"),
		}

		if allowed, ok := valOpts["allowed_kinds"].([]interface{}); ok {
			for _, k := range allowed {
				if kind, ok := k.(string); ok {
					result.ValidationOptions.AllowedKinds = append(result.ValidationOptions.AllowedKinds, kind)
				}
			}
		}

		if required, ok := valOpts["required_labels"].([]interface{}); ok {
			for _, l := range required {
				if label, ok := l.(string); ok {
					result.ValidationOptions.RequiredLabels = append(result.ValidationOptions.RequiredLabels, label)
				}
			}
		}

		if forbidden, ok := valOpts["forbidden_fields"].([]interface{}); ok {
			for _, f := range forbidden {
				if field, ok := f.(string); ok {
					result.ValidationOptions.ForbiddenFields = append(result.ValidationOptions.ForbiddenFields, field)
				}
			}
		}
	}

	return result, nil
}

// Helper functions for safe type conversion
func getStringValue(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getIntValue(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

func getBoolValue(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// convertGenerateManifestsResultToMap converts typed result to untyped map
func convertGenerateManifestsResultToMap(result *GenerateManifestsResult) map[string]interface{} {
	output := map[string]interface{}{
		"session_id":    result.SessionID,
		"success":       result.Success,
		"manifest_path": result.ManifestPath,
		"image_ref":     result.ImageRef.String(),
		"namespace":     result.Namespace,
		"service_type":  result.ServiceType,
		"replicas":      result.Replicas,
		"duration":      result.Duration.String(),
	}

	// Resources
	if result.Resources != (ResourceRequests{}) {
		output["resources"] = map[string]interface{}{
			"cpu_request":    result.Resources.CPURequest,
			"memory_request": result.Resources.MemoryRequest,
			"cpu_limit":      result.Resources.CPULimit,
			"memory_limit":   result.Resources.MemoryLimit,
		}
	}

	// Manifests
	if len(result.Manifests) > 0 {
		manifests := make([]map[string]interface{}, len(result.Manifests))
		for i, m := range result.Manifests {
			manifests[i] = map[string]interface{}{
				"name":    m.Name,
				"kind":    m.Kind,
				"path":    m.Path,
				"content": m.Content,
			}
		}
		output["manifests"] = manifests
	}

	// Validation result
	if result.ValidationResult != nil {
		validationMap := map[string]interface{}{
			"enabled":       result.ValidationResult.Enabled,
			"overall_valid": result.ValidationResult.OverallValid,
			"total_files":   result.ValidationResult.TotalFiles,
			"valid_files":   result.ValidationResult.ValidFiles,
			"error_count":   result.ValidationResult.ErrorCount,
			"warning_count": result.ValidationResult.WarningCount,
			"duration":      result.ValidationResult.Duration.String(),
			"k8s_version":   result.ValidationResult.K8sVersion,
		}

		if len(result.ValidationResult.Results) > 0 {
			results := make(map[string]interface{})
			for file, val := range result.ValidationResult.Results {
				fileVal := map[string]interface{}{
					"valid":         val.Valid,
					"kind":          val.Kind,
					"api_version":   val.APIVersion,
					"name":          val.Name,
					"namespace":     val.Namespace,
					"error_count":   val.ErrorCount,
					"warning_count": val.WarningCount,
					"duration":      val.Duration.String(),
				}

				if len(val.Errors) > 0 {
					errors := make([]map[string]interface{}, len(val.Errors))
					for i, e := range val.Errors {
						errors[i] = map[string]interface{}{
							"field":    e.Field,
							"message":  e.Message,
							"code":     e.Code,
							"severity": e.Severity,
							"path":     e.Path,
						}
					}
					fileVal["errors"] = errors
				}

				if len(val.Warnings) > 0 {
					warnings := make([]map[string]interface{}, len(val.Warnings))
					for i, w := range val.Warnings {
						warnings[i] = map[string]interface{}{
							"field":    w.Field,
							"message":  w.Message,
							"code":     w.Code,
							"severity": w.Severity,
							"path":     w.Path,
						}
					}
					fileVal["warnings"] = warnings
				}

				if len(val.Suggestions) > 0 {
					fileVal["suggestions"] = val.Suggestions
				}

				results[file] = fileVal
			}
			validationMap["results"] = results
		}

		output["validation_result"] = validationMap
	}

	// Error
	if result.Error != nil {
		output["error"] = map[string]interface{}{
			"message": result.Error.Message,
		}
	}

	return output
}

// GenerateManifestsTool handles Kubernetes manifest generation
type GenerateManifestsTool struct {
	logger        zerolog.Logger
	workspaceBase string
	validator     *ops.ManifestValidator
}

// NewGenerateManifestsTool creates a new generate manifests tool
func NewGenerateManifestsTool(logger zerolog.Logger, workspaceBase string) *GenerateManifestsTool {
	return &GenerateManifestsTool{
		logger:        logger,
		workspaceBase: workspaceBase,
		validator:     nil, // Will be initialized on first use
	}
}

// NewGenerateManifestsToolWithValidator creates a new generate manifests tool with a custom validator
func NewGenerateManifestsToolWithValidator(logger zerolog.Logger, workspaceBase string, validator *ops.ManifestValidator) *GenerateManifestsTool {
	return &GenerateManifestsTool{
		logger:        logger,
		workspaceBase: workspaceBase,
		validator:     validator,
	}
}

// Execute implements SimpleTool interface with generic signature
func (t *GenerateManifestsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Handle both typed and untyped arguments
	var manifestArgs GenerateManifestsArgs
	var err error
	var jsonData []byte

	switch a := args.(type) {
	case GenerateManifestsArgs:
		manifestArgs = a
	case map[string]interface{}:
		// Convert from map to struct using JSON marshaling
		jsonData, err = json.Marshal(a)
		if err != nil {
			return nil, mcperror.NewWithData("invalid_arguments", "Failed to marshal map to JSON", map[string]interface{}{
				"error": err.Error(),
			})
		}
		if err = json.Unmarshal(jsonData, &manifestArgs); err != nil {
			return nil, mcperror.NewWithData("invalid_arguments", "Invalid argument structure for generate_manifests", map[string]interface{}{
				"expected": "GenerateManifestsArgs or compatible map",
				"error":    err.Error(),
			})
		}
	default:
		return nil, mcperror.NewWithData("invalid_arguments", "Invalid argument type for generate_manifests", map[string]interface{}{
			"expected": "GenerateManifestsArgs or map[string]interface{}",
			"received": fmt.Sprintf("%T", args),
		})
	}

	// Call the typed execute method
	return t.ExecuteTyped(ctx, manifestArgs)
}

// ExecuteTyped generates Kubernetes manifests based on the provided arguments
func (t *GenerateManifestsTool) ExecuteTyped(ctx context.Context, args GenerateManifestsArgs) (*GenerateManifestsResult, error) {
	startTime := time.Now()

	// Create base response with versioning
	response := &GenerateManifestsResult{
		BaseToolResponse: types.NewBaseResponse("generate_manifests", args.SessionID, args.DryRun),
		ImageRef:         args.ImageRef,
		Namespace:        args.Namespace,
		ServiceType:      args.ServiceType,
		Replicas:         args.Replicas,
		Resources:        args.Resources,
		Manifests:        []ManifestInfo{},
	}

	// Apply defaults
	if args.Namespace == "" {
		args.Namespace = "default"
		response.Namespace = "default"
	}
	if args.ServiceType == "" {
		args.ServiceType = types.ServiceTypeLoadBalancer
		response.ServiceType = types.ServiceTypeLoadBalancer
	}
	if args.Replicas == 0 {
		args.Replicas = 1
		response.Replicas = 1
	}

	// Validate image reference
	if args.ImageRef.String() == "" {
		return nil, types.NewRichError("IMAGE_REF_REQUIRED", "image_ref is required", types.ErrTypeValidation)
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("image_ref", args.ImageRef.String()).
		Str("namespace", args.Namespace).
		Bool("dry_run", args.DryRun).
		Msg("Generating Kubernetes manifests")

	// Determine workspace directory
	workspaceDir := filepath.Join(t.workspaceBase, args.SessionID)
	if args.SessionID == "" {
		workspaceDir = filepath.Join(t.workspaceBase, "default")
	}

	// Set manifest path
	manifestPath := filepath.Join(workspaceDir, "manifests")
	response.ManifestPath = manifestPath

	// For dry-run, just return what would be generated
	if args.DryRun {
		response.Manifests = []ManifestInfo{
			{Name: "app", Kind: "Deployment", Path: filepath.Join(manifestPath, "deployment.yaml")},
			{Name: "app", Kind: "Service", Path: filepath.Join(manifestPath, "service.yaml")},
			{Name: "app-config", Kind: "ConfigMap", Path: filepath.Join(manifestPath, "configmap.yaml")},
			{Name: "secret-ref", Kind: "Secret", Path: filepath.Join(manifestPath, "secret.yaml")},
		}
		if args.IncludeIngress {
			response.Manifests = append(response.Manifests, ManifestInfo{
				Name: "app", Kind: "Ingress", Path: filepath.Join(manifestPath, "ingress.yaml"),
			})
		}
		if args.IncludeNetworkPolicy {
			response.Manifests = append(response.Manifests, ManifestInfo{
				Name: "app", Kind: "NetworkPolicy", Path: filepath.Join(manifestPath, "networkpolicy.yaml"),
			})
		}
		response.Duration = time.Since(startTime)
		return response, nil
	}

	// Generate manifests from templates
	if err := k8s.WriteManifestsFromTemplate(k8s.ManifestsBasic, workspaceDir); err != nil {
		return nil, types.NewRichError("MANIFEST_TEMPLATE_WRITE_FAILED", fmt.Sprintf("failed to write manifests from template: %v", err), types.ErrTypeBuild)
	}

	// Copy ingress template if requested
	if args.IncludeIngress {
		if err := t.writeIngressTemplate(workspaceDir); err != nil {
			return nil, types.NewRichError("INGRESS_TEMPLATE_WRITE_FAILED", fmt.Sprintf("failed to write ingress template: %v", err), types.ErrTypeBuild)
		}
	}

	// Copy networkpolicy template if requested
	if args.IncludeNetworkPolicy {
		if err := t.writeNetworkPolicyTemplate(workspaceDir); err != nil {
			return nil, types.NewRichError("NETWORKPOLICY_TEMPLATE_WRITE_FAILED", fmt.Sprintf("failed to write networkpolicy template: %v", err), types.ErrTypeBuild)
		}
	}

	// Use customizer module for deployment
	deploymentCustomizer := customizerk8s.NewDeploymentCustomizer(t.logger)

	// Update deployment manifest with the correct image and settings
	deploymentPath := filepath.Join(manifestPath, "deployment.yaml")
	deploymentOptions := kubernetes.CustomizeOptions{
		ImageRef:  args.ImageRef.String(),
		Namespace: args.Namespace,
		Replicas:  args.Replicas,
		EnvVars:   args.Environment,
		Labels:    args.WorkflowLabels,
	}

	if err := deploymentCustomizer.CustomizeDeployment(deploymentPath, deploymentOptions); err != nil {
		return nil, types.NewRichError("DEPLOYMENT_CUSTOMIZATION_FAILED", fmt.Sprintf("failed to customize deployment manifest: %v", err), types.ErrTypeBuild)
	}

	// Update service manifest using customizer
	serviceCustomizer := customizerk8s.NewServiceCustomizer(t.logger)
	servicePath := filepath.Join(manifestPath, "service.yaml")
	serviceOpts := customizerk8s.ServiceCustomizationOptions{
		ServiceType:     args.ServiceType,
		ServicePorts:    t.convertServicePorts(args.ServicePorts),
		LoadBalancerIP:  args.LoadBalancerIP,
		SessionAffinity: args.SessionAffinity,
		Namespace:       args.Namespace,
		Labels:          args.WorkflowLabels,
	}
	if err := serviceCustomizer.CustomizeService(servicePath, serviceOpts); err != nil {
		return nil, fmt.Errorf("failed to customize service manifest: %w", err)
	}

	// Generate and customize ConfigMap if environment variables or data exists
	if len(args.Environment) > 0 || len(args.ConfigMapData) > 0 || len(args.ConfigMapFiles) > 0 {
		configMapPath := filepath.Join(manifestPath, "configmap.yaml")

		// Combine environment variables and configmap data
		allData := make(map[string]string)
		for k, v := range args.Environment {
			allData[k] = v
		}
		for k, v := range args.ConfigMapData {
			allData[k] = v
		}

		// Handle file data
		for fileName, filePath := range args.ConfigMapFiles {
			if fileData, err := os.ReadFile(filePath); err == nil {
				allData[fileName] = string(fileData)
			} else {
				t.logger.Warn().Str("file", filePath).Err(err).Msg("Failed to read ConfigMap file")
			}
		}

		// Use customizer module for ConfigMap
		configMapCustomizer := customizerk8s.NewConfigMapCustomizer(t.logger)

		configMapOptions := kubernetes.CustomizeOptions{
			Namespace: args.Namespace,
			EnvVars:   allData,
			Labels:    args.WorkflowLabels,
		}

		if err := configMapCustomizer.CustomizeConfigMap(configMapPath, configMapOptions); err != nil {
			return nil, types.NewRichError("CONFIGMAP_CUSTOMIZATION_FAILED", fmt.Sprintf("failed to customize configmap manifest: %v", err), types.ErrTypeBuild)
		}

		// Handle binary data if present
		if len(args.BinaryData) > 0 {
			if err := t.addBinaryDataToConfigMap(configMapPath, args.BinaryData); err != nil {
				return nil, fmt.Errorf("failed to add binary data to configmap: %w", err)
			}
		}
	} else {
		// Even if no ConfigMap data, customize the template ConfigMap with workflow labels if it exists
		configMapPath := filepath.Join(manifestPath, "configmap.yaml")
		if _, err := os.Stat(configMapPath); err == nil && len(args.WorkflowLabels) > 0 {
			configMapCustomizer := customizerk8s.NewConfigMapCustomizer(t.logger)
			configMapOptions := kubernetes.CustomizeOptions{
				Namespace: args.Namespace,
				Labels:    args.WorkflowLabels,
			}
			if err := configMapCustomizer.CustomizeConfigMap(configMapPath, configMapOptions); err != nil {
				return nil, fmt.Errorf("failed to customize configmap manifest with workflow labels: %w", err)
			}
		}
	}

	// Generate and customize Ingress if requested
	if args.IncludeIngress {
		ingressPath := filepath.Join(manifestPath, "ingress.yaml")

		// Use customizer module for Ingress
		ingressCustomizer := customizerk8s.NewIngressCustomizer(t.logger)
		ingressOpts := customizerk8s.IngressCustomizationOptions{
			IngressHosts: t.convertIngressHosts(args.IngressHosts),
			IngressTLS:   t.convertIngressTLS(args.IngressTLS),
			IngressClass: args.IngressClass,
			Namespace:    args.Namespace,
			Labels:       args.WorkflowLabels,
		}
		if err := ingressCustomizer.CustomizeIngress(ingressPath, ingressOpts); err != nil {
			return nil, fmt.Errorf("failed to customize ingress manifest: %w", err)
		}
	}

	// Generate and customize NetworkPolicy if requested
	if args.IncludeNetworkPolicy {
		networkPolicyPath := filepath.Join(manifestPath, "networkpolicy.yaml")

		// Use customizer module for NetworkPolicy
		networkPolicyCustomizer := customizerk8s.NewNetworkPolicyCustomizer(t.logger)
		networkPolicyOpts := customizerk8s.NetworkPolicyCustomizationOptions{
			Namespace: args.Namespace,
			Labels:    args.WorkflowLabels,
		}

		// Apply custom NetworkPolicy specification if provided
		if args.NetworkPolicySpec != nil {
			networkPolicyOpts.PolicyTypes = args.NetworkPolicySpec.PolicyTypes
			networkPolicyOpts.PodSelector = args.NetworkPolicySpec.PodSelector
			networkPolicyOpts.Ingress = t.convertNetworkPolicyIngress(args.NetworkPolicySpec.Ingress)
			networkPolicyOpts.Egress = t.convertNetworkPolicyEgress(args.NetworkPolicySpec.Egress)
		}

		if err := networkPolicyCustomizer.CustomizeNetworkPolicy(networkPolicyPath, networkPolicyOpts); err != nil {
			return nil, fmt.Errorf("failed to customize networkpolicy manifest: %w", err)
		}
	}

	if !args.IncludeIngress {
		// Even if no ConfigMap data, customize the template ConfigMap with workflow labels if it exists
		configMapPath := filepath.Join(manifestPath, "configmap.yaml")
		if _, err := os.Stat(configMapPath); err == nil && len(args.WorkflowLabels) > 0 {
			configMapCustomizer := customizerk8s.NewConfigMapCustomizer(t.logger)
			configMapOptions := kubernetes.CustomizeOptions{
				Namespace: args.Namespace,
				Labels:    args.WorkflowLabels,
			}
			if err := configMapCustomizer.CustomizeConfigMap(configMapPath, configMapOptions); err != nil {
				return nil, fmt.Errorf("failed to customize configmap manifest with workflow labels: %w", err)
			}
		}
	}

	// Customize secret manifest with workflow labels if it exists
	secretPath := filepath.Join(manifestPath, "secret.yaml")
	if _, err := os.Stat(secretPath); err == nil {
		secretCustomizer := customizerk8s.NewSecretCustomizer(t.logger)
		secretOpts := customizerk8s.SecretCustomizationOptions{
			Namespace: args.Namespace,
			Labels:    args.WorkflowLabels,
		}
		if err := secretCustomizer.CustomizeSecret(secretPath, secretOpts); err != nil {
			return nil, fmt.Errorf("failed to customize secret manifest: %w", err)
		}
	}

	// Generate pull secret if registry credentials are provided
	if args.GeneratePullSecret && len(args.RegistrySecrets) > 0 {
		registrySecretPath := filepath.Join(manifestPath, "registry-secret.yaml")
		if err := t.generateRegistrySecret(registrySecretPath, args); err != nil {
			return nil, fmt.Errorf("failed to generate registry secret: %w", err)
		}

		// Update deployment to use the pull secret
		deploymentPath := filepath.Join(manifestPath, "deployment.yaml")
		if err := t.addPullSecretToDeployment(deploymentPath, "registry-secret"); err != nil {
			return nil, fmt.Errorf("failed to add pull secret to deployment: %w", err)
		}
	}

	// Find and read all generated manifests
	k8sObjects, err := k8s.FindK8sObjects(manifestPath)
	if err != nil {
		return nil, types.NewRichError("MANIFEST_DISCOVERY_FAILED", fmt.Sprintf("failed to find generated manifests: %v", err), types.ErrTypeSystem)
	}

	// Convert K8sObjects to ManifestInfo
	for _, obj := range k8sObjects {
		manifestInfo := ManifestInfo{
			Name: obj.Metadata.Name,
			Kind: obj.Kind,
			Path: obj.ManifestPath,
			// Optionally include content for small manifests
			Content: string(obj.Content),
		}
		response.Manifests = append(response.Manifests, manifestInfo)
	}

	// Perform manifest validation if requested
	if args.ValidateManifests {
		validationSummary, err := t.validateGeneratedManifests(ctx, manifestPath, args.ValidationOptions)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Manifest validation failed")
			// Continue execution but include validation error
			validationSummary = &ValidationSummary{
				Enabled:      true,
				OverallValid: false,
				ErrorCount:   1,
				Results: map[string]FileValidation{
					"validation_error": {
						Valid:      false,
						ErrorCount: 1,
						Errors: []ValidationIssue{
							{
								Field:    "validation",
								Message:  fmt.Sprintf("Validation failed: %v", err),
								Code:     "VALIDATION_SYSTEM_ERROR",
								Severity: "error",
							},
						},
					},
				},
			}
		}
		response.ValidationResult = validationSummary

		t.logger.Info().
			Bool("validation_enabled", validationSummary.Enabled).
			Bool("overall_valid", validationSummary.OverallValid).
			Int("valid_files", validationSummary.ValidFiles).
			Int("total_files", validationSummary.TotalFiles).
			Int("error_count", validationSummary.ErrorCount).
			Int("warning_count", validationSummary.WarningCount).
			Dur("validation_duration", validationSummary.Duration).
			Msg("Manifest validation completed")
	}

	response.Duration = time.Since(startTime)

	t.logger.Info().
		Str("session_id", args.SessionID).
		Int("manifest_count", len(response.Manifests)).
		Dur("duration", response.Duration).
		Msg("Manifest generation completed")

	return response, nil
}

// writeIngressTemplate writes the ingress template to the workspace
func (t *GenerateManifestsTool) writeIngressTemplate(workspaceDir string) error {
	// Import the templates package to access embedded files
	data, err := templates.Templates.ReadFile("manifests/manifest-basic/ingress.yaml")
	if err != nil {
		return types.NewRichError("INGRESS_TEMPLATE_READ_FAILED", fmt.Sprintf("reading embedded ingress template: %v", err), types.ErrTypeSystem)
	}

	manifestPath := filepath.Join(workspaceDir, "manifests")
	destPath := filepath.Join(manifestPath, "ingress.yaml")

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return types.NewRichError("INGRESS_TEMPLATE_WRITE_FAILED", fmt.Sprintf("writing ingress template: %v", err), types.ErrTypeSystem)
	}

	return nil
}

// writeNetworkPolicyTemplate writes the networkpolicy template to the workspace
func (t *GenerateManifestsTool) writeNetworkPolicyTemplate(workspaceDir string) error {
	// Import the templates package to access embedded files
	data, err := templates.Templates.ReadFile("manifests/manifest-basic/networkpolicy.yaml")
	if err != nil {
		return types.NewRichError("NETWORKPOLICY_TEMPLATE_READ_FAILED", fmt.Sprintf("reading embedded networkpolicy template: %v", err), types.ErrTypeSystem)
	}

	manifestPath := filepath.Join(workspaceDir, "manifests")
	destPath := filepath.Join(manifestPath, "networkpolicy.yaml")

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return types.NewRichError("NETWORKPOLICY_TEMPLATE_WRITE_FAILED", fmt.Sprintf("writing networkpolicy template: %v", err), types.ErrTypeSystem)
	}

	return nil
}

// addBinaryDataToConfigMap adds binary data to an existing ConfigMap manifest
func (t *GenerateManifestsTool) addBinaryDataToConfigMap(configMapPath string, binaryData map[string][]byte) error {
	content, err := os.ReadFile(configMapPath)
	if err != nil {
		return fmt.Errorf("reading configmap manifest: %w", err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		return fmt.Errorf("parsing configmap YAML: %w", err)
	}

	// Add binaryData section
	if len(binaryData) > 0 {
		binaryDataMap := make(map[string]interface{})
		for key, data := range binaryData {
			// Kubernetes expects base64 encoded binary data
			binaryDataMap[key] = base64.StdEncoding.EncodeToString(data)
		}
		configMap["binaryData"] = binaryDataMap
	}

	// Write back the updated manifest
	updatedContent, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("marshaling updated configmap YAML: %w", err)
	}

	if err := os.WriteFile(configMapPath, updatedContent, 0644); err != nil {
		return fmt.Errorf("writing updated configmap manifest: %w", err)
	}

	return nil
}

// generateRegistrySecret generates a Kubernetes secret for registry authentication
func (t *GenerateManifestsTool) generateRegistrySecret(secretPath string, args GenerateManifestsArgs) error {
	// Create the pull secret structure
	pullSecret := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "registry-secret",
			"namespace": args.Namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "container-copilot",
			},
		},
		"type": "kubernetes.io/dockerconfigjson",
		"data": map[string]interface{}{},
	}

	// Add workflow labels if present
	if len(args.WorkflowLabels) > 0 {
		labels := pullSecret["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
		for k, v := range args.WorkflowLabels {
			if _, exists := labels[k]; !exists {
				labels[k] = v
			}
		}
	}

	// Build the docker config JSON
	dockerConfig := map[string]interface{}{
		"auths": map[string]interface{}{},
	}

	auths := dockerConfig["auths"].(map[string]interface{})
	for _, regSecret := range args.RegistrySecrets {
		// Create base64 encoded auth string
		authString := base64.StdEncoding.EncodeToString([]byte(regSecret.Username + ":" + regSecret.Password))

		registryAuth := map[string]interface{}{
			"username": regSecret.Username,
			"password": regSecret.Password,
			"auth":     authString,
		}

		if regSecret.Email != "" {
			registryAuth["email"] = regSecret.Email
		}

		auths[regSecret.Registry] = registryAuth
	}

	// Encode the entire docker config as base64
	dockerConfigJSON, err := json.Marshal(dockerConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal docker config: %w", err)
	}

	pullSecret["data"].(map[string]interface{})[".dockerconfigjson"] = base64.StdEncoding.EncodeToString(dockerConfigJSON)

	// Write the secret to file
	secretContent, err := yaml.Marshal(pullSecret)
	if err != nil {
		return fmt.Errorf("failed to marshal pull secret YAML: %w", err)
	}

	if err := os.WriteFile(secretPath, secretContent, 0644); err != nil {
		return fmt.Errorf("failed to write pull secret file: %w", err)
	}

	t.logger.Debug().
		Str("secret_path", secretPath).
		Int("registry_count", len(args.RegistrySecrets)).
		Msg("Successfully generated registry pull secret")

	return nil
}

// addPullSecretToDeployment adds imagePullSecrets to a deployment
func (t *GenerateManifestsTool) addPullSecretToDeployment(deploymentPath, secretName string) error {
	content, err := os.ReadFile(deploymentPath)
	if err != nil {
		return fmt.Errorf("reading deployment manifest: %w", err)
	}

	var deployment map[string]interface{}
	if err := yaml.Unmarshal(content, &deployment); err != nil {
		return fmt.Errorf("parsing deployment YAML: %w", err)
	}

	// Navigate to spec.template.spec.imagePullSecrets
	pullSecrets := []interface{}{
		map[string]interface{}{
			"name": secretName,
		},
	}

	if err := t.updateNestedValue(deployment, pullSecrets, "spec", "template", "spec", "imagePullSecrets"); err != nil {
		return fmt.Errorf("updating imagePullSecrets: %w", err)
	}

	// Write back the updated deployment
	updatedContent, err := yaml.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("marshaling updated deployment YAML: %w", err)
	}

	if err := os.WriteFile(deploymentPath, updatedContent, 0644); err != nil {
		return fmt.Errorf("writing updated deployment manifest: %w", err)
	}

	t.logger.Debug().
		Str("deployment_path", deploymentPath).
		Str("secret_name", secretName).
		Msg("Successfully added pull secret to deployment")

	return nil
}

// SimpleTool interface implementation

// GetName returns the tool name
func (t *GenerateManifestsTool) GetName() string {
	return "generate_manifests"
}

// GetDescription returns the tool description
func (t *GenerateManifestsTool) GetDescription() string {
	return "Generates Kubernetes manifests for deploying containerized applications"
}

// GetVersion returns the tool version
func (t *GenerateManifestsTool) GetVersion() string {
	return "1.0.0"
}

// GetCapabilities returns the tool capabilities
func (t *GenerateManifestsTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: false,
		IsLongRunning:     false,
		RequiresAuth:      false,
	}
}

// Validate validates the tool arguments
func (t *GenerateManifestsTool) Validate(ctx context.Context, args interface{}) error {
	manifestArgs, ok := args.(GenerateManifestsArgs)
	if !ok {
		// Try to convert from map if it's not already typed
		if mapArgs, ok := args.(map[string]interface{}); ok {
			var err error
			manifestArgs, err = convertToGenerateManifestsArgs(mapArgs)
			if err != nil {
				return mcperror.NewWithData("conversion_error", fmt.Sprintf("Failed to convert arguments: %v", err), map[string]interface{}{
					"error": err.Error(),
				})
			}
		} else {
			return mcperror.NewWithData("invalid_arguments", "Invalid argument type for generate_manifests", map[string]interface{}{
				"expected": "GenerateManifestsArgs or map[string]interface{}",
				"received": fmt.Sprintf("%T", args),
			})
		}
	}

	if manifestArgs.ImageRef.Repository == "" {
		return mcperror.NewWithData("missing_required_field", "ImageRef is required", map[string]interface{}{
			"field": "image_ref",
		})
	}

	if manifestArgs.SessionID == "" {
		return mcperror.NewWithData("missing_required_field", "SessionID is required", map[string]interface{}{
			"field": "session_id",
		})
	}

	return nil
}

// validateGeneratedManifests validates all generated manifests
func (t *GenerateManifestsTool) validateGeneratedManifests(ctx context.Context, manifestPath string, options ValidationOptions) (*ValidationSummary, error) {
	start := time.Now()

	// Convert ValidationOptions to ManifestValidationOptions
	validationOptions := ops.ManifestValidationOptions{
		K8sVersion:           options.K8sVersion,
		SkipDryRun:           options.SkipDryRun,
		SkipSchemaValidation: options.SkipSchemaValidation,
		AllowedKinds:         options.AllowedKinds,
		RequiredLabels:       options.RequiredLabels,
		ForbiddenFields:      options.ForbiddenFields,
		StrictValidation:     options.StrictValidation,
	}

	// Create kubectl validator (without requiring actual kubectl for now)
	var validator *ops.ManifestValidator
	if !options.SkipDryRun {
		kubectlValidator := ops.NewKubectlValidator(t.logger, ops.KubectlValidationOptions{
			Timeout: 30 * time.Second,
		})
		validator = ops.NewManifestValidator(t.logger, kubectlValidator)
	} else {
		validator = ops.NewManifestValidator(t.logger, nil)
	}

	// Validate the manifest directory
	batchResult, err := validator.ValidateManifestDirectory(ctx, manifestPath, validationOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to validate manifest directory: %w", err)
	}

	// Convert BatchValidationResult to ValidationSummary
	summary := &ValidationSummary{
		Enabled:      true,
		OverallValid: batchResult.OverallValid,
		TotalFiles:   batchResult.TotalManifests,
		ValidFiles:   batchResult.ValidManifests,
		ErrorCount:   batchResult.ErrorCount,
		WarningCount: batchResult.WarningCount,
		Duration:     time.Since(start),
		K8sVersion:   "unknown", // We don't have kubectl available
		Results:      make(map[string]FileValidation),
	}

	// Convert individual validation results
	for fileName, result := range batchResult.Results {
		fileValidation := FileValidation{
			Valid:        result.Valid,
			Kind:         result.Kind,
			APIVersion:   result.APIVersion,
			Name:         result.Name,
			Namespace:    result.Namespace,
			ErrorCount:   len(result.Errors),
			WarningCount: len(result.Warnings),
			Duration:     result.Duration,
			Suggestions:  result.Suggestions,
		}

		// Convert errors
		for _, err := range result.Errors {
			fileValidation.Errors = append(fileValidation.Errors, ValidationIssue{
				Field:    err.Field,
				Message:  err.Message,
				Code:     err.Code,
				Severity: string(err.Severity),
				Path:     err.Path,
			})
		}

		// Convert warnings
		for _, warning := range result.Warnings {
			fileValidation.Warnings = append(fileValidation.Warnings, ValidationIssue{
				Field:    warning.Field,
				Message:  warning.Message,
				Code:     warning.Code,
				Severity: "warning",
				Path:     warning.Path,
			})
		}

		summary.Results[fileName] = fileValidation
	}

	return summary, nil
}

// updateNestedValue updates a nested value in a YAML structure
func (t *GenerateManifestsTool) updateNestedValue(obj interface{}, value interface{}, path ...interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("path cannot be empty")
	}

	current := obj
	// Navigate to the parent of the final key
	for i := 0; i < len(path)-1; i++ {
		switch curr := current.(type) {
		case map[string]interface{}:
			keyStr, ok := path[i].(string)
			if !ok {
				return fmt.Errorf("non-string key at position %d", i)
			}
			next, exists := curr[keyStr]
			if !exists {
				// Create intermediate maps as needed
				curr[keyStr] = make(map[string]interface{})
				next = curr[keyStr]
			}
			current = next
		case []interface{}:
			keyInt, ok := path[i].(int)
			if !ok {
				return fmt.Errorf("non-integer key at position %d for array", i)
			}
			if keyInt >= len(curr) {
				return fmt.Errorf("array index %d out of bounds at position %d", keyInt, i)
			}
			current = curr[keyInt]
		default:
			return fmt.Errorf("cannot navigate through non-map/non-array at position %d", i)
		}
	}

	// Set the final value
	finalKey := path[len(path)-1]
	switch curr := current.(type) {
	case map[string]interface{}:
		keyStr, ok := finalKey.(string)
		if !ok {
			return fmt.Errorf("non-string final key")
		}
		curr[keyStr] = value
	case []interface{}:
		keyInt, ok := finalKey.(int)
		if !ok {
			return fmt.Errorf("non-integer final key for array")
		}
		if keyInt < len(curr) {
			curr[keyInt] = value
		} else {
			return fmt.Errorf("array index %d out of bounds for final key", keyInt)
		}
	default:
		return fmt.Errorf("cannot set value on non-map/non-array")
	}

	return nil
}

// Converter methods for customizer types

// convertServicePorts converts ServicePort slice to customizer format
func (t *GenerateManifestsTool) convertServicePorts(ports []ServicePort) []customizerk8s.ServicePort {
	result := make([]customizerk8s.ServicePort, len(ports))
	for i, p := range ports {
		result[i] = customizerk8s.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: p.TargetPort,
			NodePort:   p.NodePort,
			Protocol:   p.Protocol,
		}
	}
	return result
}

// convertIngressHosts converts IngressHost slice to customizer format
func (t *GenerateManifestsTool) convertIngressHosts(hosts []IngressHost) []customizerk8s.IngressHost {
	result := make([]customizerk8s.IngressHost, len(hosts))
	for i, h := range hosts {
		paths := make([]customizerk8s.IngressPath, len(h.Paths))
		for j, p := range h.Paths {
			paths[j] = customizerk8s.IngressPath{
				Path:        p.Path,
				PathType:    p.PathType,
				ServiceName: p.ServiceName,
				ServicePort: p.ServicePort,
			}
		}
		result[i] = customizerk8s.IngressHost{
			Host:  h.Host,
			Paths: paths,
		}
	}
	return result
}

// convertIngressTLS converts IngressTLS slice to customizer format
func (t *GenerateManifestsTool) convertIngressTLS(tls []IngressTLS) []customizerk8s.IngressTLS {
	result := make([]customizerk8s.IngressTLS, len(tls))
	for i, t := range tls {
		result[i] = customizerk8s.IngressTLS{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		}
	}
	return result
}

// convertNetworkPolicyIngress converts NetworkPolicyIngress slice to customizer format
func (t *GenerateManifestsTool) convertNetworkPolicyIngress(ingress []NetworkPolicyIngress) []customizerk8s.NetworkPolicyIngressRule {
	result := make([]customizerk8s.NetworkPolicyIngressRule, len(ingress))
	for i, rule := range ingress {
		result[i] = customizerk8s.NetworkPolicyIngressRule{
			Ports: t.convertNetworkPolicyPorts(rule.Ports),
			From:  t.convertNetworkPolicyPeers(rule.From),
		}
	}
	return result
}

// convertNetworkPolicyEgress converts NetworkPolicyEgress slice to customizer format
func (t *GenerateManifestsTool) convertNetworkPolicyEgress(egress []NetworkPolicyEgress) []customizerk8s.NetworkPolicyEgressRule {
	result := make([]customizerk8s.NetworkPolicyEgressRule, len(egress))
	for i, rule := range egress {
		result[i] = customizerk8s.NetworkPolicyEgressRule{
			Ports: t.convertNetworkPolicyPorts(rule.Ports),
			To:    t.convertNetworkPolicyPeers(rule.To),
		}
	}
	return result
}

// convertNetworkPolicyPorts converts NetworkPolicyPort slice to customizer format
func (t *GenerateManifestsTool) convertNetworkPolicyPorts(ports []NetworkPolicyPort) []customizerk8s.NetworkPolicyPortRule {
	result := make([]customizerk8s.NetworkPolicyPortRule, len(ports))
	for i, port := range ports {
		result[i] = customizerk8s.NetworkPolicyPortRule{
			Protocol: port.Protocol,
			Port:     port.Port,
			EndPort:  port.EndPort,
		}
	}
	return result
}

// convertNetworkPolicyPeers converts NetworkPolicyPeer slice to customizer format
func (t *GenerateManifestsTool) convertNetworkPolicyPeers(peers []NetworkPolicyPeer) []customizerk8s.NetworkPolicyPeerRule {
	result := make([]customizerk8s.NetworkPolicyPeerRule, len(peers))
	for i, peer := range peers {
		var ipBlock *customizerk8s.NetworkPolicyIPBlock
		if peer.IPBlock != nil {
			ipBlock = &customizerk8s.NetworkPolicyIPBlock{
				CIDR:   peer.IPBlock.CIDR,
				Except: peer.IPBlock.Except,
			}
		}
		result[i] = customizerk8s.NetworkPolicyPeerRule{
			PodSelector:       peer.PodSelector,
			NamespaceSelector: peer.NamespaceSelector,
			IPBlock:           ipBlock,
		}
	}
	return result
}
