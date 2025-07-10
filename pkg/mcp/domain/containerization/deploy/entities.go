// Package deploy contains pure business entities and rules for container deployment operations.
// This package has no external dependencies and represents the core deployment domain.
package deploy

import (
	"time"
)

// DeploymentRequest represents a request to deploy containerized applications
type DeploymentRequest struct {
	ID            string                  `json:"id"`
	SessionID     string                  `json:"session_id"`
	Name          string                  `json:"name"`
	Namespace     string                  `json:"namespace"`
	Environment   Environment             `json:"environment"`
	Strategy      DeploymentStrategy      `json:"strategy"`
	Image         string                  `json:"image"`
	Tag           string                  `json:"tag"`
	Replicas      int                     `json:"replicas"`
	Resources     ResourceRequirements    `json:"resources"`
	Configuration DeploymentConfiguration `json:"configuration"`
	Options       DeploymentOptions       `json:"options"`
	CreatedAt     time.Time               `json:"created_at"`
}

// Environment represents a deployment environment
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentStaging     Environment = "staging"
	EnvironmentProduction  Environment = "production"
	EnvironmentTest        Environment = "test"
)

// DeploymentStrategy represents different deployment strategies
type DeploymentStrategy string

const (
	StrategyRolling   DeploymentStrategy = "rolling"
	StrategyRecreate  DeploymentStrategy = "recreate"
	StrategyBlueGreen DeploymentStrategy = "blue_green"
	StrategyCanary    DeploymentStrategy = "canary"
	StrategyABTesting DeploymentStrategy = "ab_testing"
)

// ResourceRequirements defines resource limits and requests
type ResourceRequirements struct {
	CPU     ResourceSpec `json:"cpu"`
	Memory  ResourceSpec `json:"memory"`
	Storage ResourceSpec `json:"storage,omitempty"`
}

// ResourceSpec defines resource requests and limits
type ResourceSpec struct {
	Request string `json:"request"`
	Limit   string `json:"limit"`
}

// DeploymentConfiguration contains deployment-specific configuration
type DeploymentConfiguration struct {
	Environment     map[string]string    `json:"environment,omitempty"`
	Secrets         []SecretReference    `json:"secrets,omitempty"`
	ConfigMaps      []ConfigMapReference `json:"config_maps,omitempty"`
	Volumes         []VolumeMount        `json:"volumes,omitempty"`
	Ports           []ServicePort        `json:"ports,omitempty"`
	HealthChecks    HealthCheckConfig    `json:"health_checks,omitempty"`
	SecurityContext SecurityContext      `json:"security_context,omitempty"`
}

// SecretReference represents a reference to a secret
type SecretReference struct {
	Name      string `json:"name"`
	Key       string `json:"key,omitempty"`
	MountPath string `json:"mount_path,omitempty"`
	EnvVar    string `json:"env_var,omitempty"`
}

// ConfigMapReference represents a reference to a config map
type ConfigMapReference struct {
	Name      string `json:"name"`
	Key       string `json:"key,omitempty"`
	MountPath string `json:"mount_path,omitempty"`
	EnvVar    string `json:"env_var,omitempty"`
}

// VolumeMount represents a volume mount
type VolumeMount struct {
	Name         string     `json:"name"`
	MountPath    string     `json:"mount_path"`
	VolumeType   VolumeType `json:"volume_type"`
	Size         string     `json:"size,omitempty"`
	AccessMode   string     `json:"access_mode,omitempty"`
	StorageClass string     `json:"storage_class,omitempty"`
}

// VolumeType represents the type of volume
type VolumeType string

const (
	VolumeTypePersistent VolumeType = "persistent"
	VolumeTypeEmptyDir   VolumeType = "empty_dir"
	VolumeTypeConfigMap  VolumeType = "config_map"
	VolumeTypeSecret     VolumeType = "secret"
	VolumeTypeHostPath   VolumeType = "host_path"
)

// ServicePort represents a service port
type ServicePort struct {
	Name        string      `json:"name"`
	Port        int         `json:"port"`
	TargetPort  int         `json:"target_port"`
	Protocol    Protocol    `json:"protocol"`
	ServiceType ServiceType `json:"service_type,omitempty"`
}

// Protocol represents the network protocol
type Protocol string

const (
	ProtocolTCP Protocol = "TCP"
	ProtocolUDP Protocol = "UDP"
)

// ServiceType represents the type of Kubernetes service
type ServiceType string

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"
	ServiceTypeNodePort     ServiceType = "NodePort"
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer"
	ServiceTypeExternalName ServiceType = "ExternalName"
)

// HealthCheckConfig defines health check configuration
type HealthCheckConfig struct {
	Liveness  *HealthCheck `json:"liveness,omitempty"`
	Readiness *HealthCheck `json:"readiness,omitempty"`
	Startup   *HealthCheck `json:"startup,omitempty"`
}

// HealthCheck represents a health check probe
type HealthCheck struct {
	Type                HealthCheckType `json:"type"`
	Path                string          `json:"path,omitempty"`
	Port                int             `json:"port,omitempty"`
	Command             []string        `json:"command,omitempty"`
	InitialDelaySeconds int             `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int             `json:"period_seconds,omitempty"`
	TimeoutSeconds      int             `json:"timeout_seconds,omitempty"`
	FailureThreshold    int             `json:"failure_threshold,omitempty"`
	SuccessThreshold    int             `json:"success_threshold,omitempty"`
}

// HealthCheckType represents the type of health check
type HealthCheckType string

const (
	HealthCheckTypeHTTP HealthCheckType = "http"
	HealthCheckTypeTCP  HealthCheckType = "tcp"
	HealthCheckTypeExec HealthCheckType = "exec"
)

// SecurityContext defines security settings
type SecurityContext struct {
	RunAsUser         *int64          `json:"run_as_user,omitempty"`
	RunAsGroup        *int64          `json:"run_as_group,omitempty"`
	RunAsNonRoot      *bool           `json:"run_as_non_root,omitempty"`
	ReadOnlyRootFS    *bool           `json:"read_only_root_fs,omitempty"`
	AllowPrivilegeEsc *bool           `json:"allow_privilege_escalation,omitempty"`
	Capabilities      *Capabilities   `json:"capabilities,omitempty"`
	SELinuxOptions    *SELinuxOptions `json:"selinux_options,omitempty"`
}

// Capabilities defines Linux capabilities
type Capabilities struct {
	Add  []string `json:"add,omitempty"`
	Drop []string `json:"drop,omitempty"`
}

// SELinuxOptions defines SELinux options
type SELinuxOptions struct {
	User  string `json:"user,omitempty"`
	Role  string `json:"role,omitempty"`
	Type  string `json:"type,omitempty"`
	Level string `json:"level,omitempty"`
}

// DeploymentOptions contains additional deployment options
type DeploymentOptions struct {
	DryRun               bool              `json:"dry_run,omitempty"`
	Timeout              time.Duration     `json:"timeout,omitempty"`
	RollbackOnFailure    bool              `json:"rollback_on_failure,omitempty"`
	WaitForReady         bool              `json:"wait_for_ready,omitempty"`
	ProgressDeadline     time.Duration     `json:"progress_deadline,omitempty"`
	RevisionHistoryLimit *int              `json:"revision_history_limit,omitempty"`
	Annotations          map[string]string `json:"annotations,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}

// DeploymentResult represents the result of a deployment operation
type DeploymentResult struct {
	DeploymentID string             `json:"deployment_id"`
	RequestID    string             `json:"request_id"`
	SessionID    string             `json:"session_id"`
	Name         string             `json:"name"`
	Namespace    string             `json:"namespace"`
	Status       DeploymentStatus   `json:"status"`
	Resources    DeployedResources  `json:"resources"`
	Endpoints    []Endpoint         `json:"endpoints"`
	Events       []DeploymentEvent  `json:"events"`
	Error        string             `json:"error,omitempty"`
	Duration     time.Duration      `json:"duration"`
	CreatedAt    time.Time          `json:"created_at"`
	CompletedAt  *time.Time         `json:"completed_at,omitempty"`
	Metadata     DeploymentMetadata `json:"metadata"`
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	StatusPending     DeploymentStatus = "pending"
	StatusDeploying   DeploymentStatus = "deploying"
	StatusRunning     DeploymentStatus = "running"
	StatusCompleted   DeploymentStatus = "completed"
	StatusFailed      DeploymentStatus = "failed"
	StatusRolledBack  DeploymentStatus = "rolled_back"
	StatusUpdating    DeploymentStatus = "updating"
	StatusTerminating DeploymentStatus = "terminating"
)

// DeployedResources represents the resources that were deployed
type DeployedResources struct {
	Deployment string   `json:"deployment,omitempty"`
	Service    string   `json:"service,omitempty"`
	Ingress    string   `json:"ingress,omitempty"`
	ConfigMaps []string `json:"config_maps,omitempty"`
	Secrets    []string `json:"secrets,omitempty"`
	Volumes    []string `json:"volumes,omitempty"`
}

// Endpoint represents a deployment endpoint
type Endpoint struct {
	Name     string       `json:"name"`
	URL      string       `json:"url"`
	Type     EndpointType `json:"type"`
	Port     int          `json:"port"`
	Protocol Protocol     `json:"protocol"`
	Ready    bool         `json:"ready"`
}

// EndpointType represents the type of endpoint
type EndpointType string

const (
	EndpointTypeExternal EndpointType = "external"
	EndpointTypeInternal EndpointType = "internal"
	EndpointTypeIngress  EndpointType = "ingress"
)

// DeploymentEvent represents an event during deployment
type DeploymentEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      EventType `json:"type"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Component string    `json:"component,omitempty"`
	Count     int       `json:"count,omitempty"`
}

// EventType represents the type of deployment event
type EventType string

const (
	EventTypeNormal  EventType = "Normal"
	EventTypeWarning EventType = "Warning"
	EventTypeError   EventType = "Error"
)

// DeploymentMetadata contains additional deployment information
type DeploymentMetadata struct {
	Strategy        DeploymentStrategy  `json:"strategy"`
	Environment     Environment         `json:"environment"`
	ImageDigest     string              `json:"image_digest,omitempty"`
	PreviousVersion string              `json:"previous_version,omitempty"`
	ResourceUsage   ResourceUsage       `json:"resource_usage"`
	ScalingInfo     ScalingInfo         `json:"scaling_info"`
	NetworkInfo     NetworkInfo         `json:"network_info"`
	SecurityScan    *SecurityScanResult `json:"security_scan,omitempty"`
}

// ResourceUsage represents actual resource consumption
type ResourceUsage struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage,omitempty"`
}

// ScalingInfo contains information about deployment scaling
type ScalingInfo struct {
	DesiredReplicas   int `json:"desired_replicas"`
	AvailableReplicas int `json:"available_replicas"`
	ReadyReplicas     int `json:"ready_replicas"`
	UpdatedReplicas   int `json:"updated_replicas"`
}

// NetworkInfo contains network-related deployment information
type NetworkInfo struct {
	ClusterIP    string            `json:"cluster_ip,omitempty"`
	ExternalIPs  []string          `json:"external_ips,omitempty"`
	LoadBalancer *LoadBalancerInfo `json:"load_balancer,omitempty"`
	Ingress      []IngressInfo     `json:"ingress,omitempty"`
}

// LoadBalancerInfo contains load balancer information
type LoadBalancerInfo struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Ports    []int  `json:"ports,omitempty"`
}

// IngressInfo contains ingress information
type IngressInfo struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Path string `json:"path,omitempty"`
	TLS  bool   `json:"tls"`
}

// SecurityScanResult represents deployment security scan results
type SecurityScanResult struct {
	Scanner    string           `json:"scanner"`
	ScanTime   time.Time        `json:"scan_time"`
	Passed     bool             `json:"passed"`
	Issues     []SecurityIssue  `json:"issues"`
	Compliance ComplianceResult `json:"compliance"`
}

// SecurityIssue represents a security issue in deployment
type SecurityIssue struct {
	ID          string        `json:"id"`
	Type        SecurityType  `json:"type"`
	Severity    SeverityLevel `json:"severity"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Resource    string        `json:"resource,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
}

// SecurityType represents the type of security issue
type SecurityType string

const (
	SecurityTypeConfiguration SecurityType = "configuration"
	SecurityTypeRBAC          SecurityType = "rbac"
	SecurityTypeNetwork       SecurityType = "network"
	SecurityTypePodSecurity   SecurityType = "pod_security"
	SecurityTypeImage         SecurityType = "image"
)

// SeverityLevel represents the severity of a security issue
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical"
	SeverityHigh     SeverityLevel = "high"
	SeverityMedium   SeverityLevel = "medium"
	SeverityLow      SeverityLevel = "low"
	SeverityInfo     SeverityLevel = "info"
)

// ComplianceResult represents compliance check results
type ComplianceResult struct {
	Framework string            `json:"framework"`
	Passed    bool              `json:"passed"`
	Score     float64           `json:"score"`
	Checks    []ComplianceCheck `json:"checks"`
}

// ComplianceCheck represents a single compliance check
type ComplianceCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// RollbackRequest represents a request to rollback a deployment
type RollbackRequest struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	DeploymentID string    `json:"deployment_id"`
	ToRevision   *int      `json:"to_revision,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// RollbackResult represents the result of a rollback operation
type RollbackResult struct {
	RollbackID   string         `json:"rollback_id"`
	RequestID    string         `json:"request_id"`
	DeploymentID string         `json:"deployment_id"`
	FromRevision int            `json:"from_revision"`
	ToRevision   int            `json:"to_revision"`
	Status       RollbackStatus `json:"status"`
	Error        string         `json:"error,omitempty"`
	Duration     time.Duration  `json:"duration"`
	CreatedAt    time.Time      `json:"created_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
}

// RollbackStatus represents the status of a rollback operation
type RollbackStatus string

const (
	RollbackStatusPending   RollbackStatus = "pending"
	RollbackStatusRunning   RollbackStatus = "running"
	RollbackStatusCompleted RollbackStatus = "completed"
	RollbackStatusFailed    RollbackStatus = "failed"
)

// ManifestGenerationRequest represents a request to generate Kubernetes manifests
type ManifestGenerationRequest struct {
	ID              string                  `json:"id"`
	SessionID       string                  `json:"session_id"`
	TemplateType    TemplateType            `json:"template_type"`
	Configuration   DeploymentConfiguration `json:"configuration"`
	ResourceReqs    ResourceRequirements    `json:"resource_requirements"`
	CustomTemplates map[string]string       `json:"custom_templates,omitempty"`
	Options         ManifestOptions         `json:"options"`
	CreatedAt       time.Time               `json:"created_at"`
}

// TemplateType represents the type of manifest template
type TemplateType string

const (
	TemplateTypeDeployment TemplateType = "deployment"
	TemplateTypeService    TemplateType = "service"
	TemplateTypeIngress    TemplateType = "ingress"
	TemplateTypeConfigMap  TemplateType = "configmap"
	TemplateTypeSecret     TemplateType = "secret"
	TemplateTypePV         TemplateType = "persistent_volume"
	TemplateTypePVC        TemplateType = "persistent_volume_claim"
)

// ManifestOptions contains options for manifest generation
type ManifestOptions struct {
	APIVersion     string            `json:"api_version,omitempty"`
	Namespace      string            `json:"namespace,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	IncludeExample bool              `json:"include_example,omitempty"`
	Validate       bool              `json:"validate,omitempty"`
}

// ManifestGenerationResult represents the result of manifest generation
type ManifestGenerationResult struct {
	GenerationID string             `json:"generation_id"`
	RequestID    string             `json:"request_id"`
	Manifests    map[string]string  `json:"manifests"`
	Status       ManifestStatus     `json:"status"`
	Validation   ManifestValidation `json:"validation,omitempty"`
	Error        string             `json:"error,omitempty"`
	Duration     time.Duration      `json:"duration"`
	CreatedAt    time.Time          `json:"created_at"`
}

// ManifestStatus represents the status of manifest generation
type ManifestStatus string

const (
	ManifestStatusCompleted ManifestStatus = "completed"
	ManifestStatusFailed    ManifestStatus = "failed"
)

// ManifestValidation represents manifest validation results
type ManifestValidation struct {
	Valid    bool                `json:"valid"`
	Errors   []ValidationError   `json:"errors,omitempty"`
	Warnings []ValidationWarning `json:"warnings,omitempty"`
}

// ValidationError represents a manifest validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationWarning represents a manifest validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}
