package core

import (
	"encoding/json"
)

// Legacy tool constraint interfaces removed - use api.Tool for all new implementations

// ValidationError represents a validation issue
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Value   any    `json:"value"`
}

// Schema represents the JSON schema for tool parameters and results
type Schema[TParams any, TResult any] struct {
	Name           string
	Description    string
	Version        string
	ParamsSchema   *JSONSchema // Typed JSON Schema for parameters
	ResultSchema   *JSONSchema // Typed JSON Schema for results
	Examples       []Example[TParams, TResult]
	Deprecated     bool
	DeprecationMsg string
}

// Example represents an example usage of a tool
type Example[TParams any, TResult any] struct {
	Name        string
	Description string
	Params      TParams
	Result      TResult
}

// TypedValidationDocument represents a structured validation document instead of map[string]interface{}
type TypedValidationDocument struct {
	APIVersion string                     `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                     `json:"kind" yaml:"kind"`
	Metadata   *TypedValidationMetadata   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       json.RawMessage            `json:"spec,omitempty" yaml:"spec,omitempty"` // Keep as RawMessage for flexibility
	Data       map[string]string          `json:"data,omitempty" yaml:"data,omitempty"`
	StringData map[string]string          `json:"stringData,omitempty" yaml:"stringData,omitempty"`
	BinaryData map[string][]byte          `json:"binaryData,omitempty" yaml:"binaryData,omitempty"`
	RawFields  map[string]json.RawMessage `json:"-" yaml:"-"` // For unknown fields
}

// TypedValidationMetadata represents validation metadata instead of map[string]interface{}
type TypedValidationMetadata struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	UID         string            `json:"uid,omitempty" yaml:"uid,omitempty"`
	Generation  int64             `json:"generation,omitempty" yaml:"generation,omitempty"`
}

// TypedValidationSpec represents a typed validation spec for different resource types
type TypedValidationSpec struct {
	// Deployment fields
	Replicas *int32                      `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Selector *TypedValidationSelector    `json:"selector,omitempty" yaml:"selector,omitempty"`
	Template *TypedValidationPodTemplate `json:"template,omitempty" yaml:"template,omitempty"`

	// Service fields
	Type  string                `json:"type,omitempty" yaml:"type,omitempty"`
	Ports []TypedValidationPort `json:"ports,omitempty" yaml:"ports,omitempty"`

	// Generic spec fields
	GenericFields map[string]json.RawMessage `json:"-" yaml:"-"` // For unknown fields
}

// TypedValidationSelector represents a label selector for validation
type TypedValidationSelector struct {
	MatchLabels      map[string]string                 `json:"matchLabels,omitempty" yaml:"matchLabels,omitempty"`
	MatchExpressions []TypedValidationLabelSelectorReq `json:"matchExpressions,omitempty" yaml:"matchExpressions,omitempty"`
}

// TypedValidationLabelSelectorReq represents a label selector requirement
type TypedValidationLabelSelectorReq struct {
	Key      string   `json:"key" yaml:"key"`
	Operator string   `json:"operator" yaml:"operator"`
	Values   []string `json:"values,omitempty" yaml:"values,omitempty"`
}

// TypedValidationPodTemplate represents a pod template for validation
type TypedValidationPodTemplate struct {
	Metadata *TypedValidationMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec     *TypedValidationPodSpec  `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// TypedValidationPodSpec represents a pod spec for validation
type TypedValidationPodSpec struct {
	Containers     []TypedValidationContainer `json:"containers" yaml:"containers"`
	InitContainers []TypedValidationContainer `json:"initContainers,omitempty" yaml:"initContainers,omitempty"`
	Volumes        []TypedValidationVolume    `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	RestartPolicy  string                     `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	NodeSelector   map[string]string          `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
}

// TypedValidationContainer represents a container for validation
type TypedValidationContainer struct {
	Name            string                         `json:"name" yaml:"name"`
	Image           string                         `json:"image" yaml:"image"`
	Ports           []TypedValidationContainerPort `json:"ports,omitempty" yaml:"ports,omitempty"`
	Env             []TypedValidationEnvVar        `json:"env,omitempty" yaml:"env,omitempty"`
	Resources       *TypedValidationResources      `json:"resources,omitempty" yaml:"resources,omitempty"`
	ImagePullPolicy string                         `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Command         []string                       `json:"command,omitempty" yaml:"command,omitempty"`
	Args            []string                       `json:"args,omitempty" yaml:"args,omitempty"`
}

// TypedValidationContainerPort represents a container port for validation
type TypedValidationContainerPort struct {
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	ContainerPort int32  `json:"containerPort" yaml:"containerPort"`
	Protocol      string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	HostIP        string `json:"hostIP,omitempty" yaml:"hostIP,omitempty"`
	HostPort      int32  `json:"hostPort,omitempty" yaml:"hostPort,omitempty"`
}

// TypedValidationEnvVar represents an environment variable for validation
type TypedValidationEnvVar struct {
	Name      string                       `json:"name" yaml:"name"`
	Value     string                       `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *TypedValidationEnvVarSource `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}

// TypedValidationEnvVarSource represents an environment variable source
type TypedValidationEnvVarSource struct {
	FieldRef         *TypedValidationObjectFieldSelector   `json:"fieldRef,omitempty" yaml:"fieldRef,omitempty"`
	ResourceFieldRef *TypedValidationResourceFieldSelector `json:"resourceFieldRef,omitempty" yaml:"resourceFieldRef,omitempty"`
	ConfigMapKeyRef  *TypedValidationKeySelector           `json:"configMapKeyRef,omitempty" yaml:"configMapKeyRef,omitempty"`
	SecretKeyRef     *TypedValidationKeySelector           `json:"secretKeyRef,omitempty" yaml:"secretKeyRef,omitempty"`
}

// TypedValidationObjectFieldSelector represents an object field selector
type TypedValidationObjectFieldSelector struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	FieldPath  string `json:"fieldPath" yaml:"fieldPath"`
}

// TypedValidationResourceFieldSelector represents a resource field selector
type TypedValidationResourceFieldSelector struct {
	ContainerName string `json:"containerName,omitempty" yaml:"containerName,omitempty"`
	Resource      string `json:"resource" yaml:"resource"`
	Divisor       string `json:"divisor,omitempty" yaml:"divisor,omitempty"`
}

// TypedValidationKeySelector represents a key selector for ConfigMap/Secret
type TypedValidationKeySelector struct {
	Name     string `json:"name" yaml:"name"`
	Key      string `json:"key" yaml:"key"`
	Optional *bool  `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// TypedValidationResources represents resource requirements for validation
type TypedValidationResources struct {
	Limits   map[string]string `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty" yaml:"requests,omitempty"`
}

// TypedValidationVolume represents a volume for validation
type TypedValidationVolume struct {
	Name                  string                          `json:"name" yaml:"name"`
	HostPath              *TypedValidationHostPathVolume  `json:"hostPath,omitempty" yaml:"hostPath,omitempty"`
	EmptyDir              *TypedValidationEmptyDirVolume  `json:"emptyDir,omitempty" yaml:"emptyDir,omitempty"`
	ConfigMap             *TypedValidationConfigMapVolume `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	Secret                *TypedValidationSecretVolume    `json:"secret,omitempty" yaml:"secret,omitempty"`
	PersistentVolumeClaim *TypedValidationPVCVolume       `json:"persistentVolumeClaim,omitempty" yaml:"persistentVolumeClaim,omitempty"`
}

// TypedValidationHostPathVolume represents a host path volume
type TypedValidationHostPathVolume struct {
	Path string `json:"path" yaml:"path"`
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
}

// TypedValidationEmptyDirVolume represents an empty dir volume
type TypedValidationEmptyDirVolume struct {
	Medium    string `json:"medium,omitempty" yaml:"medium,omitempty"`
	SizeLimit string `json:"sizeLimit,omitempty" yaml:"sizeLimit,omitempty"`
}

// TypedValidationConfigMapVolume represents a config map volume
type TypedValidationConfigMapVolume struct {
	Name        string                     `json:"name" yaml:"name"`
	Items       []TypedValidationKeyToPath `json:"items,omitempty" yaml:"items,omitempty"`
	DefaultMode *int32                     `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	Optional    *bool                      `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// TypedValidationSecretVolume represents a secret volume
type TypedValidationSecretVolume struct {
	SecretName  string                     `json:"secretName" yaml:"secretName"`
	Items       []TypedValidationKeyToPath `json:"items,omitempty" yaml:"items,omitempty"`
	DefaultMode *int32                     `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	Optional    *bool                      `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// TypedValidationPVCVolume represents a persistent volume claim volume
type TypedValidationPVCVolume struct {
	ClaimName string `json:"claimName" yaml:"claimName"`
	ReadOnly  bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}

// TypedValidationKeyToPath represents a key to path mapping
type TypedValidationKeyToPath struct {
	Key  string `json:"key" yaml:"key"`
	Path string `json:"path" yaml:"path"`
	Mode *int32 `json:"mode,omitempty" yaml:"mode,omitempty"`
}

// TypedValidationPort represents a service port for validation
type TypedValidationPort struct {
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	Port       int32  `json:"port" yaml:"port"`
	TargetPort string `json:"targetPort,omitempty" yaml:"targetPort,omitempty"` // Can be int or string
	Protocol   string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	NodePort   int32  `json:"nodePort,omitempty" yaml:"nodePort,omitempty"`
}

// TypedManifestValidationLocation represents the location of a validation issue (avoiding conflicts)
type TypedManifestValidationLocation struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Path   string `json:"path,omitempty"` // JSON path like "spec.containers[0].image"
}

// TypedValidationSummary represents a summary of validation results
type TypedValidationSummary struct {
	TotalManifests     int `json:"total_manifests"`
	ValidManifests     int `json:"valid_manifests"`
	InvalidManifests   int `json:"invalid_manifests"`
	TotalErrors        int `json:"total_errors"`
	TotalWarnings      int `json:"total_warnings"`
	CriticalErrors     int `json:"critical_errors"`
	HighSeverityIssues int `json:"high_severity_issues"`
}

// ToMap converts TypedValidationDocument to map[string]interface{} for backward compatibility
func (d *TypedValidationDocument) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	result["apiVersion"] = d.APIVersion
	result["kind"] = d.Kind

	if d.Metadata != nil {
		result["metadata"] = d.Metadata.ToMap()
	}

	if len(d.Spec) > 0 {
		var spec interface{}
		if err := json.Unmarshal(d.Spec, &spec); err == nil {
			result["spec"] = spec
		}
	}

	if len(d.Data) > 0 {
		result["data"] = d.Data
	}

	if len(d.StringData) > 0 {
		result["stringData"] = d.StringData
	}

	if len(d.BinaryData) > 0 {
		result["binaryData"] = d.BinaryData
	}

	// Include any raw fields
	for k, v := range d.RawFields {
		var value interface{}
		if err := json.Unmarshal(v, &value); err == nil {
			result[k] = value
		}
	}

	return result
}

// ToMap converts TypedValidationMetadata to map[string]interface{} for backward compatibility
func (m *TypedValidationMetadata) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	result["name"] = m.Name

	if m.Namespace != "" {
		result["namespace"] = m.Namespace
	}

	if len(m.Labels) > 0 {
		result["labels"] = m.Labels
	}

	if len(m.Annotations) > 0 {
		result["annotations"] = m.Annotations
	}

	if m.UID != "" {
		result["uid"] = m.UID
	}

	if m.Generation > 0 {
		result["generation"] = m.Generation
	}

	return result
}
