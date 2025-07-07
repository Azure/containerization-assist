package validators

// KubernetesValidator validates Kubernetes manifest files and resources
type KubernetesValidator struct {
	*BaseValidatorImpl
	strictMode         bool
	securityValidation bool
	allowedVersions    map[string][]string // apiVersion -> kind mappings
}

// ManifestData represents structured Kubernetes manifest data
type ManifestData struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   *ObjectMetadata        `json:"metadata,omitempty"`
	Spec       *ResourceSpec          `json:"spec,omitempty"`
	Status     interface{}            `json:"status,omitempty"`
	Data       map[string]string      `json:"data,omitempty"`       // For ConfigMap
	StringData map[string]string      `json:"stringData,omitempty"` // For Secret
	Raw        map[string]interface{} `json:"raw,omitempty"`        // Full manifest data for backward compatibility
}

// TypedManifestData represents fully typed Kubernetes manifest data
type TypedManifestData struct {
	APIVersion    string               `json:"apiVersion"`
	Kind          string               `json:"kind"`
	Metadata      *TypedObjectMetadata `json:"metadata"`
	Spec          *TypedResourceSpec   `json:"spec,omitempty"`
	Status        interface{}          `json:"status,omitempty"`
	Data          map[string]string    `json:"data,omitempty"`
	BinaryData    map[string][]byte    `json:"binaryData,omitempty"`
	StringData    map[string]string    `json:"stringData,omitempty"`
	Rules         []TypedPolicyRule    `json:"rules,omitempty"`    // For RBAC resources
	Subjects      []TypedRBACSubject   `json:"subjects,omitempty"` // For RBAC bindings
	RoleRef       *TypedRoleRef        `json:"roleRef,omitempty"`  // For RBAC bindings
	Webhooks      []TypedWebhook       `json:"webhooks,omitempty"` // For admission controllers
	CustomFields  *TypedCustomFields   `json:"customFields,omitempty"`
	Annotations   map[string]string    `json:"annotations,omitempty"`
	Labels        map[string]string    `json:"labels,omitempty"`
	OwnerRefs     []string             `json:"ownerReferences,omitempty"`
	Finalizers    []string             `json:"finalizers,omitempty"`
	ClusterName   string               `json:"clusterName,omitempty"`
	GenerateName  string               `json:"generateName,omitempty"`
	ManagedFields []string             `json:"managedFields,omitempty"`
	Namespace     string               `json:"namespace,omitempty"`
	ResourceVer   string               `json:"resourceVersion,omitempty"`
	SelfLink      string               `json:"selfLink,omitempty"`
	UID           string               `json:"uid,omitempty"`
	Generation    int64                `json:"generation,omitempty"`
	CreationTime  string               `json:"creationTimestamp,omitempty"`
	DeletionTime  string               `json:"deletionTimestamp,omitempty"`
	DeletionGrace *int64               `json:"deletionGracePeriodSeconds,omitempty"`
}

// TypedObjectMetadata represents typed Kubernetes object metadata
type TypedObjectMetadata struct {
	Name                       string            `json:"name"`
	Namespace                  string            `json:"namespace,omitempty"`
	Labels                     map[string]string `json:"labels,omitempty"`
	Annotations                map[string]string `json:"annotations,omitempty"`
	UID                        string            `json:"uid,omitempty"`
	Generation                 int64             `json:"generation,omitempty"`
	ResourceVersion            string            `json:"resourceVersion,omitempty"`
	CreationTimestamp          string            `json:"creationTimestamp,omitempty"`
	DeletionTimestamp          string            `json:"deletionTimestamp,omitempty"`
	DeletionGracePeriodSeconds *int64            `json:"deletionGracePeriodSeconds,omitempty"`
	Finalizers                 []string          `json:"finalizers,omitempty"`
	GenerateName               string            `json:"generateName,omitempty"`
	SelfLink                   string            `json:"selfLink,omitempty"`
	ClusterName                string            `json:"clusterName,omitempty"`
	ManagedFields              []string          `json:"managedFields,omitempty"`
}

// TypedResourceSpec represents typed Kubernetes resource specification
type TypedResourceSpec struct {
	Replicas         *int32                     `json:"replicas,omitempty"`
	Selector         *TypedLabelSelector        `json:"selector,omitempty"`
	Template         *TypedPodTemplateSpec      `json:"template,omitempty"`
	Containers       []TypedContainer           `json:"containers,omitempty"`
	InitContainers   []TypedContainer           `json:"initContainers,omitempty"`
	Volumes          []TypedVolume              `json:"volumes,omitempty"`
	Ports            []TypedServicePort         `json:"ports,omitempty"`
	ServiceType      string                     `json:"type,omitempty"`
	ClusterIP        string                     `json:"clusterIP,omitempty"`
	LoadBalancerIP   string                     `json:"loadBalancerIP,omitempty"`
	ExternalIPs      []string                   `json:"externalIPs,omitempty"`
	Secrets          []TypedSecretRef           `json:"secrets,omitempty"`
	ImagePullSecrets []TypedLocalObjectRef      `json:"imagePullSecrets,omitempty"`
	ServiceAccount   string                     `json:"serviceAccountName,omitempty"`
	Strategy         *TypedDeploymentStrategy   `json:"strategy,omitempty"`
	Resources        *TypedResourceRequirements `json:"resources,omitempty"`
}

// TypedCustomFields represents typed custom extension fields
type TypedCustomFields struct {
	StringFields  map[string]string   `json:"stringFields,omitempty"`
	NumberFields  map[string]float64  `json:"numberFields,omitempty"`
	BooleanFields map[string]bool     `json:"booleanFields,omitempty"`
	ArrayFields   map[string][]string `json:"arrayFields,omitempty"`
}

// ObjectMetadata represents Kubernetes object metadata
type ObjectMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	UID         string            `json:"uid,omitempty"`
	Generation  int64             `json:"generation,omitempty"`
}

// ResourceSpec represents Kubernetes resource specification
type ResourceSpec struct {
	Replicas *int32                 `json:"replicas,omitempty"`
	Selector *LabelSelector         `json:"selector,omitempty"`
	Template *PodTemplateSpec       `json:"template,omitempty"`
	Ports    []ServicePort          `json:"ports,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Raw      map[string]interface{} `json:"raw,omitempty"` // For complex specs
}

// LabelSelector represents label selector
type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// PodTemplateSpec represents pod template specification
type PodTemplateSpec struct {
	Metadata *ObjectMetadata `json:"metadata,omitempty"`
	Spec     *PodSpec        `json:"spec,omitempty"`
}

// PodSpec represents pod specification
type PodSpec struct {
	Containers []Container `json:"containers"`
	Volumes    []Volume    `json:"volumes,omitempty"`
}

// Container represents container specification
type Container struct {
	Name  string          `json:"name"`
	Image string          `json:"image"`
	Ports []ContainerPort `json:"ports,omitempty"`
	Env   []EnvVar        `json:"env,omitempty"`
}

// ContainerPort represents container port
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// EnvVar represents environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// Volume represents volume specification
type Volume struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// ServicePort represents service port
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

// TypedLabelSelector represents typed label selector
type TypedLabelSelector struct {
	MatchLabels      map[string]string       `json:"matchLabels,omitempty"`
	MatchExpressions []TypedLabelSelectorReq `json:"matchExpressions,omitempty"`
}

// TypedLabelSelectorReq represents typed label selector requirement
type TypedLabelSelectorReq struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// TypedPodTemplateSpec represents typed pod template spec
type TypedPodTemplateSpec struct {
	Metadata *TypedObjectMetadata `json:"metadata,omitempty"`
	Spec     *TypedPodSpec        `json:"spec"`
}

// TypedPodSpec represents typed pod specification
type TypedPodSpec struct {
	Containers                    []TypedContainer         `json:"containers"`
	InitContainers                []TypedContainer         `json:"initContainers,omitempty"`
	Volumes                       []TypedVolume            `json:"volumes,omitempty"`
	RestartPolicy                 string                   `json:"restartPolicy,omitempty"`
	TerminationGracePeriodSeconds *int64                   `json:"terminationGracePeriodSeconds,omitempty"`
	DNSPolicy                     string                   `json:"dnsPolicy,omitempty"`
	NodeSelector                  map[string]string        `json:"nodeSelector,omitempty"`
	ServiceAccountName            string                   `json:"serviceAccountName,omitempty"`
	HostNetwork                   bool                     `json:"hostNetwork,omitempty"`
	HostPID                       bool                     `json:"hostPID,omitempty"`
	HostIPC                       bool                     `json:"hostIPC,omitempty"`
	SecurityContext               *TypedPodSecurityContext `json:"securityContext,omitempty"`
	ImagePullSecrets              []TypedLocalObjectRef    `json:"imagePullSecrets,omitempty"`
	Tolerations                   []TypedToleration        `json:"tolerations,omitempty"`
	Affinity                      *TypedAffinity           `json:"affinity,omitempty"`
	SchedulerName                 string                   `json:"schedulerName,omitempty"`
	PriorityClassName             string                   `json:"priorityClassName,omitempty"`
	Priority                      *int32                   `json:"priority,omitempty"`
}

// TypedContainer represents typed container specification
type TypedContainer struct {
	Name                     string                     `json:"name"`
	Image                    string                     `json:"image"`
	Command                  []string                   `json:"command,omitempty"`
	Args                     []string                   `json:"args,omitempty"`
	WorkingDir               string                     `json:"workingDir,omitempty"`
	Ports                    []TypedContainerPort       `json:"ports,omitempty"`
	Env                      []TypedEnvVar              `json:"env,omitempty"`
	EnvFrom                  []TypedEnvFromSource       `json:"envFrom,omitempty"`
	Resources                *TypedResourceRequirements `json:"resources,omitempty"`
	VolumeMounts             []TypedVolumeMount         `json:"volumeMounts,omitempty"`
	LivenessProbe            *TypedProbe                `json:"livenessProbe,omitempty"`
	ReadinessProbe           *TypedProbe                `json:"readinessProbe,omitempty"`
	StartupProbe             *TypedProbe                `json:"startupProbe,omitempty"`
	Lifecycle                *TypedLifecycle            `json:"lifecycle,omitempty"`
	TerminationMessagePath   string                     `json:"terminationMessagePath,omitempty"`
	TerminationMessagePolicy string                     `json:"terminationMessagePolicy,omitempty"`
	ImagePullPolicy          string                     `json:"imagePullPolicy,omitempty"`
	SecurityContext          *TypedSecurityContext      `json:"securityContext,omitempty"`
	Stdin                    bool                       `json:"stdin,omitempty"`
	StdinOnce                bool                       `json:"stdinOnce,omitempty"`
	TTY                      bool                       `json:"tty,omitempty"`
}

// TypedContainerPort represents typed container port
type TypedContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
	HostPort      int32  `json:"hostPort,omitempty"`
	HostIP        string `json:"hostIP,omitempty"`
}

// TypedEnvVar represents typed environment variable
type TypedEnvVar struct {
	Name      string             `json:"name"`
	Value     string             `json:"value,omitempty"`
	ValueFrom *TypedEnvVarSource `json:"valueFrom,omitempty"`
}

// TypedEnvVarSource represents typed environment variable source
type TypedEnvVarSource struct {
	FieldRef         *TypedObjectFieldSelector   `json:"fieldRef,omitempty"`
	ResourceFieldRef *TypedResourceFieldSelector `json:"resourceFieldRef,omitempty"`
	ConfigMapKeyRef  *TypedConfigMapKeySelector  `json:"configMapKeyRef,omitempty"`
	SecretKeyRef     *TypedSecretKeySelector     `json:"secretKeyRef,omitempty"`
}

// TypedEnvFromSource represents typed environment from source
type TypedEnvFromSource struct {
	Prefix       string                   `json:"prefix,omitempty"`
	ConfigMapRef *TypedConfigMapEnvSource `json:"configMapRef,omitempty"`
	SecretRef    *TypedSecretEnvSource    `json:"secretRef,omitempty"`
}

// TypedResourceRequirements represents typed resource requirements
type TypedResourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

// TypedVolumeMount represents typed volume mount
type TypedVolumeMount struct {
	Name             string `json:"name"`
	MountPath        string `json:"mountPath"`
	SubPath          string `json:"subPath,omitempty"`
	SubPathExpr      string `json:"subPathExpr,omitempty"`
	ReadOnly         bool   `json:"readOnly,omitempty"`
	MountPropagation string `json:"mountPropagation,omitempty"`
}

// TypedVolume represents typed volume
type TypedVolume struct {
	Name                  string                                  `json:"name"`
	HostPath              *TypedHostPathVolumeSource              `json:"hostPath,omitempty"`
	EmptyDir              *TypedEmptyDirVolumeSource              `json:"emptyDir,omitempty"`
	GCEPersistentDisk     *TypedGCEPersistentDiskVolumeSource     `json:"gcePersistentDisk,omitempty"`
	Secret                *TypedSecretVolumeSource                `json:"secret,omitempty"`
	ConfigMap             *TypedConfigMapVolumeSource             `json:"configMap,omitempty"`
	DownwardAPI           *TypedDownwardAPIVolumeSource           `json:"downwardAPI,omitempty"`
	PersistentVolumeClaim *TypedPersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// TypedServicePort represents typed service port
type TypedServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort,omitempty"` // Can be int or string
	NodePort   int32  `json:"nodePort,omitempty"`
}

// TypedSecretRef represents typed secret reference
type TypedSecretRef struct {
	Name string `json:"name"`
}

// TypedLocalObjectRef represents typed local object reference
type TypedLocalObjectRef struct {
	Name string `json:"name"`
}

// TypedDeploymentStrategy represents typed deployment strategy
type TypedDeploymentStrategy struct {
	Type          string                        `json:"type,omitempty"`
	RollingUpdate *TypedRollingUpdateDeployment `json:"rollingUpdate,omitempty"`
}

// TypedRollingUpdateDeployment represents typed rolling update deployment
type TypedRollingUpdateDeployment struct {
	MaxUnavailable string `json:"maxUnavailable,omitempty"` // Can be int or percentage
	MaxSurge       string `json:"maxSurge,omitempty"`       // Can be int or percentage
}

// TypedPolicyRule represents typed policy rule
type TypedPolicyRule struct {
	Verbs           []string `json:"verbs"`
	APIGroups       []string `json:"apiGroups,omitempty"`
	Resources       []string `json:"resources,omitempty"`
	ResourceNames   []string `json:"resourceNames,omitempty"`
	NonResourceURLs []string `json:"nonResourceURLs,omitempty"`
}

// TypedRBACSubject represents typed RBAC subject
type TypedRBACSubject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	APIGroup  string `json:"apiGroup,omitempty"`
}

// TypedRoleRef represents typed role reference
type TypedRoleRef struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

// TypedWebhook represents typed webhook
type TypedWebhook struct {
	Name                    string                    `json:"name"`
	ClientConfig            *TypedWebhookClientConfig `json:"clientConfig"`
	Rules                   []TypedRuleWithOperations `json:"rules,omitempty"`
	FailurePolicy           string                    `json:"failurePolicy,omitempty"`
	SideEffects             string                    `json:"sideEffects,omitempty"`
	AdmissionReviewVersions []string                  `json:"admissionReviewVersions,omitempty"`
	TimeoutSeconds          *int32                    `json:"timeoutSeconds,omitempty"`
	ReinvocationPolicy      string                    `json:"reinvocationPolicy,omitempty"`
}

// TypedObjectFieldSelector represents object field selector
type TypedObjectFieldSelector struct {
	APIVersion string `json:"apiVersion,omitempty"`
	FieldPath  string `json:"fieldPath"`
}

// TypedResourceFieldSelector represents resource field selector
type TypedResourceFieldSelector struct {
	ContainerName string `json:"containerName,omitempty"`
	Resource      string `json:"resource"`
	Divisor       string `json:"divisor,omitempty"`
}

// TypedConfigMapKeySelector represents configmap key selector
type TypedConfigMapKeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional *bool  `json:"optional,omitempty"`
}

// TypedSecretKeySelector represents secret key selector
type TypedSecretKeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional *bool  `json:"optional,omitempty"`
}

// TypedConfigMapEnvSource represents configmap env source
type TypedConfigMapEnvSource struct {
	Name     string `json:"name"`
	Optional *bool  `json:"optional,omitempty"`
}

// TypedSecretEnvSource represents secret env source
type TypedSecretEnvSource struct {
	Name     string `json:"name"`
	Optional *bool  `json:"optional,omitempty"`
}

// TypedHostPathVolumeSource represents host path volume source
type TypedHostPathVolumeSource struct {
	Path string `json:"path"`
	Type string `json:"type,omitempty"`
}

// TypedEmptyDirVolumeSource represents empty dir volume source
type TypedEmptyDirVolumeSource struct {
	Medium    string `json:"medium,omitempty"`
	SizeLimit string `json:"sizeLimit,omitempty"`
}

// TypedGCEPersistentDiskVolumeSource represents GCE persistent disk volume source
type TypedGCEPersistentDiskVolumeSource struct {
	PDName    string `json:"pdName"`
	FSType    string `json:"fsType,omitempty"`
	Partition int32  `json:"partition,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// TypedSecretVolumeSource represents secret volume source
type TypedSecretVolumeSource struct {
	SecretName  string           `json:"secretName,omitempty"`
	Items       []TypedKeyToPath `json:"items,omitempty"`
	DefaultMode *int32           `json:"defaultMode,omitempty"`
	Optional    *bool            `json:"optional,omitempty"`
}

// TypedConfigMapVolumeSource represents configmap volume source
type TypedConfigMapVolumeSource struct {
	Name        string           `json:"name,omitempty"`
	Items       []TypedKeyToPath `json:"items,omitempty"`
	DefaultMode *int32           `json:"defaultMode,omitempty"`
	Optional    *bool            `json:"optional,omitempty"`
}

// TypedDownwardAPIVolumeSource represents downward API volume source
type TypedDownwardAPIVolumeSource struct {
	Items       []TypedDownwardAPIVolumeFile `json:"items,omitempty"`
	DefaultMode *int32                       `json:"defaultMode,omitempty"`
}

// TypedPersistentVolumeClaimVolumeSource represents PVC volume source
type TypedPersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// TypedKeyToPath represents key to path mapping
type TypedKeyToPath struct {
	Key  string `json:"key"`
	Path string `json:"path"`
	Mode *int32 `json:"mode,omitempty"`
}

// TypedDownwardAPIVolumeFile represents downward API volume file
type TypedDownwardAPIVolumeFile struct {
	Path             string                      `json:"path"`
	FieldRef         *TypedObjectFieldSelector   `json:"fieldRef,omitempty"`
	ResourceFieldRef *TypedResourceFieldSelector `json:"resourceFieldRef,omitempty"`
	Mode             *int32                      `json:"mode,omitempty"`
}

// TypedProbe represents typed probe
type TypedProbe struct {
	HTTPGet             *TypedHTTPGetAction   `json:"httpGet,omitempty"`
	Exec                *TypedExecAction      `json:"exec,omitempty"`
	TCPSocket           *TypedTCPSocketAction `json:"tcpSocket,omitempty"`
	InitialDelaySeconds int32                 `json:"initialDelaySeconds,omitempty"`
	TimeoutSeconds      int32                 `json:"timeoutSeconds,omitempty"`
	PeriodSeconds       int32                 `json:"periodSeconds,omitempty"`
	SuccessThreshold    int32                 `json:"successThreshold,omitempty"`
	FailureThreshold    int32                 `json:"failureThreshold,omitempty"`
}

// TypedHTTPGetAction represents HTTP GET action
type TypedHTTPGetAction struct {
	Path        string            `json:"path,omitempty"`
	Port        string            `json:"port"`
	Host        string            `json:"host,omitempty"`
	Scheme      string            `json:"scheme,omitempty"`
	HTTPHeaders []TypedHTTPHeader `json:"httpHeaders,omitempty"`
}

// TypedExecAction represents exec action
type TypedExecAction struct {
	Command []string `json:"command,omitempty"`
}

// TypedTCPSocketAction represents TCP socket action
type TypedTCPSocketAction struct {
	Port string `json:"port"`
	Host string `json:"host,omitempty"`
}

// TypedHTTPHeader represents HTTP header
type TypedHTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TypedLifecycle represents typed lifecycle
type TypedLifecycle struct {
	PostStart *TypedLifecycleHandler `json:"postStart,omitempty"`
	PreStop   *TypedLifecycleHandler `json:"preStop,omitempty"`
}

// TypedLifecycleHandler represents typed lifecycle handler
type TypedLifecycleHandler struct {
	Exec      *TypedExecAction      `json:"exec,omitempty"`
	HTTPGet   *TypedHTTPGetAction   `json:"httpGet,omitempty"`
	TCPSocket *TypedTCPSocketAction `json:"tcpSocket,omitempty"`
}

// TypedSecurityContext represents typed security context
type TypedSecurityContext struct {
	Capabilities             *TypedCapabilities                  `json:"capabilities,omitempty"`
	Privileged               *bool                               `json:"privileged,omitempty"`
	SELinuxOptions           *TypedSELinuxOptions                `json:"seLinuxOptions,omitempty"`
	WindowsOptions           *TypedWindowsSecurityContextOptions `json:"windowsOptions,omitempty"`
	RunAsUser                *int64                              `json:"runAsUser,omitempty"`
	RunAsGroup               *int64                              `json:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool                               `json:"runAsNonRoot,omitempty"`
	ReadOnlyRootFilesystem   *bool                               `json:"readOnlyRootFilesystem,omitempty"`
	AllowPrivilegeEscalation *bool                               `json:"allowPrivilegeEscalation,omitempty"`
	ProcMount                string                              `json:"procMount,omitempty"`
	SeccompProfile           *TypedSeccompProfile                `json:"seccompProfile,omitempty"`
}

// TypedPodSecurityContext represents typed pod security context
type TypedPodSecurityContext struct {
	SELinuxOptions      *TypedSELinuxOptions                `json:"seLinuxOptions,omitempty"`
	WindowsOptions      *TypedWindowsSecurityContextOptions `json:"windowsOptions,omitempty"`
	RunAsUser           *int64                              `json:"runAsUser,omitempty"`
	RunAsGroup          *int64                              `json:"runAsGroup,omitempty"`
	RunAsNonRoot        *bool                               `json:"runAsNonRoot,omitempty"`
	SupplementalGroups  []int64                             `json:"supplementalGroups,omitempty"`
	FSGroup             *int64                              `json:"fsGroup,omitempty"`
	FSGroupChangePolicy string                              `json:"fsGroupChangePolicy,omitempty"`
	SeccompProfile      *TypedSeccompProfile                `json:"seccompProfile,omitempty"`
	Sysctls             []TypedSysctl                       `json:"sysctls,omitempty"`
}

// TypedCapabilities represents capabilities
type TypedCapabilities struct {
	Add  []string `json:"add,omitempty"`
	Drop []string `json:"drop,omitempty"`
}

// TypedSELinuxOptions represents SELinux options
type TypedSELinuxOptions struct {
	User  string `json:"user,omitempty"`
	Role  string `json:"role,omitempty"`
	Type  string `json:"type,omitempty"`
	Level string `json:"level,omitempty"`
}

// TypedWindowsSecurityContextOptions represents Windows security context options
type TypedWindowsSecurityContextOptions struct {
	GMSACredentialSpecName string `json:"gmsaCredentialSpecName,omitempty"`
	GMSACredentialSpec     string `json:"gmsaCredentialSpec,omitempty"`
	RunAsUserName          string `json:"runAsUserName,omitempty"`
	HostProcess            *bool  `json:"hostProcess,omitempty"`
}

// TypedSeccompProfile represents seccomp profile
type TypedSeccompProfile struct {
	Type             string `json:"type"`
	LocalhostProfile string `json:"localhostProfile,omitempty"`
}

// TypedSysctl represents sysctl
type TypedSysctl struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TypedToleration represents typed toleration
type TypedToleration struct {
	Key               string `json:"key,omitempty"`
	Operator          string `json:"operator,omitempty"`
	Value             string `json:"value,omitempty"`
	Effect            string `json:"effect,omitempty"`
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// TypedAffinity represents typed affinity
type TypedAffinity struct {
	NodeAffinity    *TypedNodeAffinity    `json:"nodeAffinity,omitempty"`
	PodAffinity     *TypedPodAffinity     `json:"podAffinity,omitempty"`
	PodAntiAffinity *TypedPodAntiAffinity `json:"podAntiAffinity,omitempty"`
}

// TypedNodeAffinity represents node affinity
type TypedNodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *TypedNodeSelector             `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []TypedPreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// TypedPodAffinity represents pod affinity
type TypedPodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []TypedPodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []TypedWeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// TypedPodAntiAffinity represents pod anti-affinity
type TypedPodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []TypedPodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []TypedWeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// TypedWebhookClientConfig represents webhook client config
type TypedWebhookClientConfig struct {
	URL      string                 `json:"url,omitempty"`
	Service  *TypedServiceReference `json:"service,omitempty"`
	CABundle []byte                 `json:"caBundle,omitempty"`
}

// TypedServiceReference represents service reference
type TypedServiceReference struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"`
	Port      *int32 `json:"port,omitempty"`
}

// TypedRuleWithOperations represents rule with operations
type TypedRuleWithOperations struct {
	Operations  []string `json:"operations,omitempty"`
	APIGroups   []string `json:"apiGroups,omitempty"`
	APIVersions []string `json:"apiVersions,omitempty"`
	Resources   []string `json:"resources,omitempty"`
	Scope       string   `json:"scope,omitempty"`
}

// TypedNodeSelector represents node selector
type TypedNodeSelector struct {
	NodeSelectorTerms []TypedNodeSelectorTerm `json:"nodeSelectorTerms"`
}

// TypedNodeSelectorTerm represents node selector term
type TypedNodeSelectorTerm struct {
	MatchExpressions []TypedNodeSelectorRequirement `json:"matchExpressions,omitempty"`
	MatchFields      []TypedNodeSelectorRequirement `json:"matchFields,omitempty"`
}

// TypedNodeSelectorRequirement represents node selector requirement
type TypedNodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// TypedPreferredSchedulingTerm represents preferred scheduling term
type TypedPreferredSchedulingTerm struct {
	Weight     int32                 `json:"weight"`
	Preference TypedNodeSelectorTerm `json:"preference"`
}

// TypedPodAffinityTerm represents pod affinity term
type TypedPodAffinityTerm struct {
	LabelSelector     *TypedLabelSelector `json:"labelSelector,omitempty"`
	Namespaces        []string            `json:"namespaces,omitempty"`
	TopologyKey       string              `json:"topologyKey"`
	NamespaceSelector *TypedLabelSelector `json:"namespaceSelector,omitempty"`
}

// TypedWeightedPodAffinityTerm represents weighted pod affinity term
type TypedWeightedPodAffinityTerm struct {
	Weight          int32                `json:"weight"`
	PodAffinityTerm TypedPodAffinityTerm `json:"podAffinityTerm"`
}

// getDefaultK8sVersions returns default Kubernetes API versions and their supported kinds
func getDefaultK8sVersions() map[string][]string {
	return map[string][]string{
		"v1": {
			"Pod", "Service", "ConfigMap", "Secret", "PersistentVolume",
			"PersistentVolumeClaim", "Namespace", "ServiceAccount", "Endpoints",
		},
		"apps/v1": {
			"Deployment", "StatefulSet", "DaemonSet", "ReplicaSet",
		},
		"networking.k8s.io/v1": {
			"Ingress", "NetworkPolicy",
		},
		"rbac.authorization.k8s.io/v1": {
			"Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding",
		},
		"batch/v1": {
			"Job",
		},
		"batch/v1beta1": {
			"CronJob",
		},
		"autoscaling/v1": {
			"HorizontalPodAutoscaler",
		},
		"autoscaling/v2": {
			"HorizontalPodAutoscaler",
		},
		"policy/v1": {
			"PodDisruptionBudget",
		},
	}
}
