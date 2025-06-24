package types

// Registry-related constants
const (
	// DefaultRegistry is the default Docker registry
	DefaultRegistry = "docker.io"

	// NetworkError represents a network-related error category
	NetworkError = "network_error"
)

// Language constants for analysis and scanning
const (
	LanguageTypeScript = "typescript"
	LanguagePython     = "python"
	LanguageJavaScript = "javascript"
	LanguageJava       = "java"
	LanguageJSON       = "json"
)

// Build system constants
const (
	BuildSystemMaven  = "maven"
	BuildSystemGradle = "gradle"
)

// Dockerfile strategy constants
const (
	DockerfileStrategyMultiStage = "multi-stage"
)

// Application server constants
const (
	AppServerTomcat = "tomcat"
)

// Size constants for image analysis
const (
	SizeSmall = "small"
	SizeLarge = "large"
)

// Health status constants
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusPending   = "pending"
	HealthStatusFailed    = "failed"
)

// Kubernetes service type constants
const (
	ServiceTypeLoadBalancer = "LoadBalancer"
)

// Resource allocation constants
const (
	ResourceCPUDefault    = "500m"
	ResourceMemoryDefault = "256Mi"
	ResourceModeAuto      = "auto"
)

// Session status constants
const (
	SessionStatusActive  = "active"
	SessionStatusExpired = "expired"
	SessionSortOrderAsc  = "asc"
)

// Tool and operation names
const (
	ToolNameGenerateManifests      = "generate_manifests"
	OperationManifestGeneration    = "manifest_generation"
	OperationSessionRetrieval      = "session_retrieval"
	OperationInitialization        = "initialization"
	OperationValidatePrerequisites = "validate_prerequisites"
	OperationDockerPush            = "docker_push"
	OperationAuthentication        = "authentication"
	ToolNameAnalyzeRepository      = "analyze_repository"
)

// Error categories
const (
	ErrorCategoryAuthError = "auth_error"
	ErrorCategoryRateLimit = "rate_limit"
	ErrorCategoryUnknown   = "unknown"
)

// Security severity levels
const (
	SeverityExcellent = "excellent"
	SeverityGood      = "good"
	SeverityPoor      = "poor"
	SeverityCritical  = "critical"
	SeverityHigh      = "high"
	SeverityMedium    = "medium"
	SeverityLow       = "low"
)

// Validation constants
const (
	ValidationModeInline = "inline"
	ValidationTypeError  = "error"
)

// Common strings
const (
	DefaultString        = "default"
	AppLabel             = "app"
	ExternalSecretsLabel = "external-secrets"
	UnknownString        = "unknown"
	PromptString         = "prompt"
)

// Quality/Performance levels
const (
	QualityPoor = "poor"
)

// Test-specific constants (only used in test files)
const (
	TestStringHello = "hello"
)
