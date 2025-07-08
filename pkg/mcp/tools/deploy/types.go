package deploy

// Type definitions for Kubernetes manifest validation
// This file contains all type definitions that represent strongly typed
// Kubernetes manifests and their components for validation purposes.

// ===============================
// Core Kubernetes Resources
// ===============================

// TypedManifest represents a strongly typed Kubernetes manifest
type TypedManifest struct {
	APIVersion string            `yaml:"apiVersion" json:"apiVersion"`
	Kind       string            `yaml:"kind" json:"kind"`
	Metadata   TypedMetadata     `yaml:"metadata" json:"metadata"`
	Spec       interface{}       `yaml:"spec,omitempty" json:"spec,omitempty"`
	Data       map[string]string `yaml:"data,omitempty" json:"data,omitempty"`
	StringData map[string]string `yaml:"stringData,omitempty" json:"stringData,omitempty"`
	BinaryData map[string][]byte `yaml:"binaryData,omitempty" json:"binaryData,omitempty"`
}

// TypedMetadata represents Kubernetes object metadata
type TypedMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	Namespace   string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// ===============================
// Deployment Types
// ===============================

// TypedDeploymentSpec represents Deployment specification
type TypedDeploymentSpec struct {
	Replicas int32                   `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	Selector TypedLabelSelector      `yaml:"selector" json:"selector"`
	Template TypedPodTemplateSpec    `yaml:"template" json:"template"`
	Strategy TypedDeploymentStrategy `yaml:"strategy,omitempty" json:"strategy,omitempty"`
}

// TypedLabelSelector represents label selector
type TypedLabelSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels,omitempty" json:"matchLabels,omitempty"`
}

// TypedPodTemplateSpec represents pod template
type TypedPodTemplateSpec struct {
	Metadata TypedMetadata `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Spec     TypedPodSpec  `yaml:"spec" json:"spec"`
}

// TypedPodSpec represents pod specification
type TypedPodSpec struct {
	Containers []TypedContainer `yaml:"containers" json:"containers"`
	Volumes    []TypedVolume    `yaml:"volumes,omitempty" json:"volumes,omitempty"`
}

// TypedDeploymentStrategy represents deployment strategy
type TypedDeploymentStrategy struct {
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
}

// ===============================
// Container Types
// ===============================

// TypedContainer represents container specification
type TypedContainer struct {
	Name            string                    `yaml:"name" json:"name"`
	Image           string                    `yaml:"image" json:"image"`
	Ports           []TypedContainerPort      `yaml:"ports,omitempty" json:"ports,omitempty"`
	Env             []TypedEnvVar             `yaml:"env,omitempty" json:"env,omitempty"`
	Resources       TypedResourceRequirements `yaml:"resources,omitempty" json:"resources,omitempty"`
	ImagePullPolicy string                    `yaml:"imagePullPolicy,omitempty" json:"imagePullPolicy,omitempty"`
}

// TypedContainerPort represents container port
type TypedContainerPort struct {
	Name          string `yaml:"name,omitempty" json:"name,omitempty"`
	ContainerPort int32  `yaml:"containerPort" json:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

// TypedEnvVar represents environment variable
type TypedEnvVar struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
}

// TypedResourceRequirements represents resource requirements
type TypedResourceRequirements struct {
	Limits   map[string]string `yaml:"limits,omitempty" json:"limits,omitempty"`
	Requests map[string]string `yaml:"requests,omitempty" json:"requests,omitempty"`
}

// ===============================
// Service Types
// ===============================

// TypedServiceSpec represents Service specification
type TypedServiceSpec struct {
	Selector map[string]string  `yaml:"selector,omitempty" json:"selector,omitempty"`
	Ports    []TypedServicePort `yaml:"ports" json:"ports"`
	Type     string             `yaml:"type,omitempty" json:"type,omitempty"`
}

// TypedServicePort represents service port
type TypedServicePort struct {
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
	Port       int32  `yaml:"port" json:"port"`
	TargetPort int32  `yaml:"targetPort,omitempty" json:"targetPort,omitempty"`
	Protocol   string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

// ===============================
// Volume Types
// ===============================

// TypedVolume represents volume specification
type TypedVolume struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
}
