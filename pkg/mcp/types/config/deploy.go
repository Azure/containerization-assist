package config

import (
	"time"
)

// DeployConfig represents typed configuration for Kubernetes deployment operations
type DeployConfig struct {
	// Basic deployment information
	Name        string            `json:"name" validate:"required"`
	Namespace   string            `json:"namespace" validate:"required"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// Image configuration
	Image       string   `json:"image" validate:"required"`
	ImageTag    string   `json:"image_tag,omitempty"`
	PullPolicy  string   `json:"pull_policy,omitempty"`
	PullSecrets []string `json:"pull_secrets,omitempty"`

	// Replica configuration
	Replicas    int32 `json:"replicas" validate:"min=1"`
	MinReplicas int32 `json:"min_replicas,omitempty"`
	MaxReplicas int32 `json:"max_replicas,omitempty"`

	// Resource requirements
	Resources ResourceConfig `json:"resources,omitempty"`

	// Container configuration
	Ports       []PortConfig      `json:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Secrets     []SecretConfig    `json:"secrets,omitempty"`
	ConfigMaps  []ConfigMapConfig `json:"config_maps,omitempty"`

	// Health checks
	HealthCheck HealthCheckConfig `json:"health_check,omitempty"`

	// Service configuration
	Service ServiceConfig `json:"service,omitempty"`

	// Ingress configuration
	Ingress IngressConfig `json:"ingress,omitempty"`

	// Deployment strategy
	Strategy DeploymentStrategy `json:"strategy,omitempty"`

	// Timeout and retry configuration
	Timeout time.Duration `json:"timeout" validate:"required,min=1s"`
	Retries int           `json:"retries" validate:"min=0,max=10"`

	// Security context
	Security SecurityConfig `json:"security,omitempty"`

	// Persistence configuration
	Persistence []VolumeConfig `json:"persistence,omitempty"`

	// Monitoring and observability
	Monitoring MonitoringConfig `json:"monitoring,omitempty"`

	// Metadata
	CreatedBy    string `json:"created_by,omitempty"`
	DeploymentID string `json:"deployment_id,omitempty"`
}

// ResourceConfig represents resource requirements and limits
type ResourceConfig struct {
	Requests ResourceLimits `json:"requests,omitempty"`
	Limits   ResourceLimits `json:"limits,omitempty"`
}

// ResourceLimits represents CPU and memory limits
type ResourceLimits struct {
	CPU    string `json:"cpu,omitempty"`    // e.g., "100m", "1"
	Memory string `json:"memory,omitempty"` // e.g., "128Mi", "1Gi"
}

// PortConfig represents a container port configuration
type PortConfig struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"container_port" validate:"required,min=1,max=65535"`
	Protocol      string `json:"protocol,omitempty"` // TCP, UDP, SCTP
	HostPort      int32  `json:"host_port,omitempty"`
}

// SecretConfig represents secret mount configuration
type SecretConfig struct {
	Name      string      `json:"name" validate:"required"`
	MountPath string      `json:"mount_path" validate:"required"`
	ReadOnly  bool        `json:"read_only,omitempty"`
	Items     []KeyToPath `json:"items,omitempty"`
}

// ConfigMapConfig represents configmap mount configuration
type ConfigMapConfig struct {
	Name      string      `json:"name" validate:"required"`
	MountPath string      `json:"mount_path" validate:"required"`
	ReadOnly  bool        `json:"read_only,omitempty"`
	Items     []KeyToPath `json:"items,omitempty"`
}

// KeyToPath represents a key to path mapping
type KeyToPath struct {
	Key  string `json:"key" validate:"required"`
	Path string `json:"path" validate:"required"`
	Mode int32  `json:"mode,omitempty"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled        bool        `json:"enabled,omitempty"`
	LivenessProbe  ProbeConfig `json:"liveness_probe,omitempty"`
	ReadinessProbe ProbeConfig `json:"readiness_probe,omitempty"`
	StartupProbe   ProbeConfig `json:"startup_probe,omitempty"`
}

// ProbeConfig represents a health check probe configuration
type ProbeConfig struct {
	HTTPGet             *HTTPGetAction   `json:"http_get,omitempty"`
	TCPSocket           *TCPSocketAction `json:"tcp_socket,omitempty"`
	Exec                *ExecAction      `json:"exec,omitempty"`
	InitialDelaySeconds int32            `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int32            `json:"period_seconds,omitempty"`
	TimeoutSeconds      int32            `json:"timeout_seconds,omitempty"`
	SuccessThreshold    int32            `json:"success_threshold,omitempty"`
	FailureThreshold    int32            `json:"failure_threshold,omitempty"`
}

// HTTPGetAction represents an HTTP GET probe
type HTTPGetAction struct {
	Path    string            `json:"path,omitempty"`
	Port    int32             `json:"port" validate:"required"`
	Host    string            `json:"host,omitempty"`
	Scheme  string            `json:"scheme,omitempty"` // HTTP, HTTPS
	Headers map[string]string `json:"headers,omitempty"`
}

// TCPSocketAction represents a TCP socket probe
type TCPSocketAction struct {
	Port int32  `json:"port" validate:"required"`
	Host string `json:"host,omitempty"`
}

// ExecAction represents an exec probe
type ExecAction struct {
	Command []string `json:"command" validate:"required"`
}

// ServiceConfig represents Kubernetes service configuration
type ServiceConfig struct {
	Enabled        bool              `json:"enabled,omitempty"`
	Type           string            `json:"type,omitempty"` // ClusterIP, NodePort, LoadBalancer, ExternalName
	Ports          []ServicePort     `json:"ports,omitempty"`
	Selector       map[string]string `json:"selector,omitempty"`
	ExternalIPs    []string          `json:"external_ips,omitempty"`
	LoadBalancerIP string            `json:"load_balancer_ip,omitempty"`
}

// ServicePort represents a service port configuration
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port" validate:"required"`
	TargetPort int32  `json:"target_port,omitempty"`
	NodePort   int32  `json:"node_port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

// IngressConfig represents Kubernetes ingress configuration
type IngressConfig struct {
	Enabled     bool              `json:"enabled,omitempty"`
	Host        string            `json:"host,omitempty"`
	Paths       []IngressPath     `json:"paths,omitempty"`
	TLS         []IngressTLS      `json:"tls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	ClassName   string            `json:"class_name,omitempty"`
}

// IngressPath represents an ingress path configuration
type IngressPath struct {
	Path     string `json:"path" validate:"required"`
	PathType string `json:"path_type,omitempty"` // Exact, Prefix, ImplementationSpecific
	Service  string `json:"service" validate:"required"`
	Port     int32  `json:"port" validate:"required"`
}

// IngressTLS represents TLS configuration for ingress
type IngressTLS struct {
	Hosts      []string `json:"hosts,omitempty"`
	SecretName string   `json:"secret_name,omitempty"`
}

// DeploymentStrategy represents deployment strategy configuration
type DeploymentStrategy struct {
	Type          string                 `json:"type,omitempty"` // Recreate, RollingUpdate
	RollingUpdate *RollingUpdateStrategy `json:"rolling_update,omitempty"`
}

// RollingUpdateStrategy represents rolling update configuration
type RollingUpdateStrategy struct {
	MaxUnavailable string `json:"max_unavailable,omitempty"` // e.g., "25%", "1"
	MaxSurge       string `json:"max_surge,omitempty"`       // e.g., "25%", "1"
}

// SecurityConfig represents security context configuration
type SecurityConfig struct {
	RunAsUser                int64              `json:"run_as_user,omitempty"`
	RunAsGroup               int64              `json:"run_as_group,omitempty"`
	RunAsNonRoot             bool               `json:"run_as_non_root,omitempty"`
	ReadOnlyRootFS           bool               `json:"read_only_root_fs,omitempty"`
	AllowPrivilegeEscalation bool               `json:"allow_privilege_escalation,omitempty"`
	Capabilities             CapabilitiesConfig `json:"capabilities,omitempty"`
}

// CapabilitiesConfig represents Linux capabilities configuration
type CapabilitiesConfig struct {
	Add  []string `json:"add,omitempty"`
	Drop []string `json:"drop,omitempty"`
}

// VolumeConfig represents persistent volume configuration
type VolumeConfig struct {
	Name         string   `json:"name" validate:"required"`
	MountPath    string   `json:"mount_path" validate:"required"`
	Size         string   `json:"size,omitempty"` // e.g., "10Gi"
	StorageClass string   `json:"storage_class,omitempty"`
	AccessModes  []string `json:"access_modes,omitempty"` // ReadWriteOnce, ReadOnlyMany, ReadWriteMany
}

// MonitoringConfig represents monitoring and observability configuration
type MonitoringConfig struct {
	Enabled bool          `json:"enabled,omitempty"`
	Metrics MetricsConfig `json:"metrics,omitempty"`
	Logging LoggingConfig `json:"logging,omitempty"`
	Tracing TracingConfig `json:"tracing,omitempty"`
}

// MetricsConfig represents metrics collection configuration
type MetricsConfig struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Path     string `json:"path,omitempty"`
	Port     int32  `json:"port,omitempty"`
	Interval string `json:"interval,omitempty"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `json:"level,omitempty"`  // debug, info, warn, error
	Format string `json:"format,omitempty"` // json, text
}

// TracingConfig represents distributed tracing configuration
type TracingConfig struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Service  string `json:"service,omitempty"`
}

// Validate validates the deployment configuration
func (dc *DeployConfig) Validate() error {
	if dc.Name == "" {
		return NewValidationError("name", "required field cannot be empty")
	}

	if dc.Namespace == "" {
		return NewValidationError("namespace", "required field cannot be empty")
	}

	if dc.Image == "" {
		return NewValidationError("image", "required field cannot be empty")
	}

	if dc.Replicas < 1 {
		return NewValidationError("replicas", "must be at least 1")
	}

	if dc.Timeout < time.Second {
		return NewValidationError("timeout", "must be at least 1 second")
	}

	if dc.Retries < 0 || dc.Retries > 10 {
		return NewValidationError("retries", "must be between 0 and 10")
	}

	// Validate ports
	for i, port := range dc.Ports {
		if port.ContainerPort < 1 || port.ContainerPort > 65535 {
			return NewValidationError("ports["+string(rune(i))+"].container_port", "must be between 1 and 65535")
		}
	}

	return nil
}

// SetDefaults sets default values for deployment configuration
func (dc *DeployConfig) SetDefaults() {
	if dc.Namespace == "" {
		dc.Namespace = "default"
	}

	if dc.Replicas == 0 {
		dc.Replicas = 1
	}

	if dc.Timeout == 0 {
		dc.Timeout = 5 * time.Minute
	}

	if dc.Retries == 0 {
		dc.Retries = 3
	}

	if dc.ImageTag == "" {
		dc.ImageTag = "latest"
	}

	if dc.PullPolicy == "" {
		dc.PullPolicy = "Always"
	}

	// Set default resource requests
	if dc.Resources.Requests.CPU == "" {
		dc.Resources.Requests.CPU = "100m"
	}
	if dc.Resources.Requests.Memory == "" {
		dc.Resources.Requests.Memory = "128Mi"
	}

	// Set default resource limits
	if dc.Resources.Limits.CPU == "" {
		dc.Resources.Limits.CPU = "500m"
	}
	if dc.Resources.Limits.Memory == "" {
		dc.Resources.Limits.Memory = "512Mi"
	}

	// Set default deployment strategy
	if dc.Strategy.Type == "" {
		dc.Strategy.Type = "RollingUpdate"
	}
}

// IsValid checks if the configuration is valid
func (dc *DeployConfig) IsValid() bool {
	return dc.Validate() == nil
}

// GetFullImageName returns the fully qualified image name with tag
func (dc *DeployConfig) GetFullImageName() string {
	if dc.ImageTag == "" {
		return dc.Image + ":latest"
	}
	return dc.Image + ":" + dc.ImageTag
}

// HasIngress checks if ingress is enabled
func (dc *DeployConfig) HasIngress() bool {
	return dc.Ingress.Enabled
}

// HasService checks if service is enabled
func (dc *DeployConfig) HasService() bool {
	return dc.Service.Enabled
}

// HasMonitoring checks if monitoring is enabled
func (dc *DeployConfig) HasMonitoring() bool {
	return dc.Monitoring.Enabled
}
