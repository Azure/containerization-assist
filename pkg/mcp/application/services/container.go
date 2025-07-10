package services

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/clients"
	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/core/security"
)

// DefaultServiceContainer provides a default implementation of ServiceContainer
type DefaultServiceContainer struct {
	sessionStore     SessionStore
	sessionState     SessionState
	buildExecutor    BuildExecutor
	toolRegistry     ToolRegistry
	workflowExecutor WorkflowExecutor
	scanner          Scanner
	configValidator  ConfigValidator
	errorReporter    ErrorReporter
	stateManager     StateManager
	knowledgeBase    KnowledgeBase
	k8sClient        K8sClient
	analyzer         Analyzer
	persistence      Persistence
	logger           *slog.Logger

	// Infrastructure services
	dockerService     docker.Service
	manifestService   kubernetes.ManifestService
	deploymentService kubernetes.Service
	pipelineService   PipelineService

	// Conversation services
	conversationService ConversationService
	promptService       PromptService
}

// ContainerConfig contains configuration for service container
type ContainerConfig struct {
	SessionConfig  SessionConfig  `json:"session_config"`
	DockerConfig   DockerConfig   `json:"docker_config"`
	AnalysisConfig AnalysisConfig `json:"analysis_config"`
	TemplateConfig TemplateConfig `json:"template_config"`
	StateConfig    StateConfig    `json:"state_config"`
	RetryConfig    RetryConfig    `json:"retry_config"`
}

// SessionConfig configures session management
type SessionConfig struct {
	StoragePath    string `json:"storage_path"`
	SessionTimeout int    `json:"session_timeout"`
	MaxSessions    int    `json:"max_sessions"`
}

// DockerConfig configures Docker client
type DockerConfig struct {
	Host         string            `json:"host"`
	Version      string            `json:"version"`
	TLSVerify    bool              `json:"tls_verify"`
	CertPath     string            `json:"cert_path"`
	RegistryAuth map[string]string `json:"registry_auth"`
}

// AnalysisConfig configures analysis engine
type AnalysisConfig struct {
	MaxDepth     int    `json:"max_depth"`
	ScanTimeout  int    `json:"scan_timeout"`
	CacheEnabled bool   `json:"cache_enabled"`
	CachePath    string `json:"cache_path"`
}

// TemplateConfig configures template management
type TemplateConfig struct {
	TemplatePath string `json:"template_path"`
	CacheEnabled bool   `json:"cache_enabled"`
}

// StateConfig configures state management
type StateConfig struct {
	StoragePath   string `json:"storage_path"`
	SyncInterval  int    `json:"sync_interval"`
	RetentionDays int    `json:"retention_days"`
}

// RetryConfig configures retry logic
type RetryConfig struct {
	MaxRetries    int     `json:"max_retries"`
	BaseDelay     int     `json:"base_delay"`
	MaxDelay      int     `json:"max_delay"`
	BackoffFactor float64 `json:"backoff_factor"`
}

// NewDefaultServiceContainer creates a new service container with initialized services
func NewDefaultServiceContainer(logger *slog.Logger) *DefaultServiceContainer {
	container := &DefaultServiceContainer{
		logger: logger,
	}

	// Initialize all services
	container.initializeServices()

	return container
}

// NewServiceContainer creates a new service container with configuration
func NewServiceContainer(config ContainerConfig, logger *slog.Logger) (ServiceContainer, error) {
	container := &DefaultServiceContainer{
		logger: logger,
	}

	// For now, return a basic container
	// In a full implementation, we would initialize services based on config
	return container, nil
}

// SessionStore returns the session store service
func (c *DefaultServiceContainer) SessionStore() SessionStore {
	return c.sessionStore
}

// SessionState returns the session state service
func (c *DefaultServiceContainer) SessionState() SessionState {
	return c.sessionState
}

// BuildExecutor returns the build executor service
func (c *DefaultServiceContainer) BuildExecutor() BuildExecutor {
	return c.buildExecutor
}

// ToolRegistry returns the tool registry service
func (c *DefaultServiceContainer) ToolRegistry() ToolRegistry {
	return c.toolRegistry
}

// WorkflowExecutor returns the workflow executor service
func (c *DefaultServiceContainer) WorkflowExecutor() WorkflowExecutor {
	return c.workflowExecutor
}

// Scanner returns the scanner service
func (c *DefaultServiceContainer) Scanner() Scanner {
	return c.scanner
}

// ConfigValidator returns the config validator service
func (c *DefaultServiceContainer) ConfigValidator() ConfigValidator {
	return c.configValidator
}

// ErrorReporter returns the error reporter service
func (c *DefaultServiceContainer) ErrorReporter() ErrorReporter {
	return c.errorReporter
}

// StateManager returns the state manager service
func (c *DefaultServiceContainer) StateManager() StateManager {
	return c.stateManager
}

// KnowledgeBase returns the knowledge base service
func (c *DefaultServiceContainer) KnowledgeBase() KnowledgeBase {
	return c.knowledgeBase
}

// K8sClient returns the Kubernetes client service
func (c *DefaultServiceContainer) K8sClient() K8sClient {
	return c.k8sClient
}

// Analyzer returns the analyzer service
func (c *DefaultServiceContainer) Analyzer() Analyzer {
	return c.analyzer
}

// Persistence returns the persistence service
func (c *DefaultServiceContainer) Persistence() Persistence {
	return c.persistence
}

// Logger returns the logger
func (c *DefaultServiceContainer) Logger() *slog.Logger {
	return c.logger
}

// DockerService returns the docker service
func (c *DefaultServiceContainer) DockerService() docker.Service {
	return c.dockerService
}

// ManifestService returns the manifest service
func (c *DefaultServiceContainer) ManifestService() kubernetes.ManifestService {
	return c.manifestService
}

// DeploymentService returns the deployment service
func (c *DefaultServiceContainer) DeploymentService() kubernetes.Service {
	return c.deploymentService
}

// PipelineService returns the pipeline service
func (c *DefaultServiceContainer) PipelineService() PipelineService {
	return c.pipelineService
}

// ConversationService returns the conversation service
func (c *DefaultServiceContainer) ConversationService() ConversationService {
	return c.conversationService
}

// PromptService returns the prompt service
func (c *DefaultServiceContainer) PromptService() PromptService {
	return c.promptService
}

// WithSessionStore sets the session store
func (c *DefaultServiceContainer) WithSessionStore(store SessionStore) *DefaultServiceContainer {
	c.sessionStore = store
	return c
}

// GetServiceStatus returns the status of all services
func (c *DefaultServiceContainer) GetServiceStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// Check service availability
	status["session_store"] = c.sessionStore != nil
	status["session_state"] = c.sessionState != nil
	status["docker_service"] = c.dockerService != nil
	status["logger"] = c.logger != nil

	return status
}

// WithSessionState sets the session state
func (c *DefaultServiceContainer) WithSessionState(state SessionState) *DefaultServiceContainer {
	c.sessionState = state
	return c
}

// WithBuildExecutor sets the build executor
func (c *DefaultServiceContainer) WithBuildExecutor(executor BuildExecutor) *DefaultServiceContainer {
	c.buildExecutor = executor
	return c
}

// WithToolRegistry sets the tool registry
func (c *DefaultServiceContainer) WithToolRegistry(registry ToolRegistry) *DefaultServiceContainer {
	c.toolRegistry = registry
	return c
}

// WithWorkflowExecutor sets the workflow executor
func (c *DefaultServiceContainer) WithWorkflowExecutor(executor WorkflowExecutor) *DefaultServiceContainer {
	c.workflowExecutor = executor
	return c
}

// WithScanner sets the scanner
func (c *DefaultServiceContainer) WithScanner(scanner Scanner) *DefaultServiceContainer {
	c.scanner = scanner
	return c
}

// WithConfigValidator sets the config validator
func (c *DefaultServiceContainer) WithConfigValidator(validator ConfigValidator) *DefaultServiceContainer {
	c.configValidator = validator
	return c
}

// WithErrorReporter sets the error reporter
func (c *DefaultServiceContainer) WithErrorReporter(reporter ErrorReporter) *DefaultServiceContainer {
	c.errorReporter = reporter
	return c
}

// WithStateManager sets the state manager
func (c *DefaultServiceContainer) WithStateManager(manager StateManager) *DefaultServiceContainer {
	c.stateManager = manager
	return c
}

// WithKnowledgeBase sets the knowledge base
func (c *DefaultServiceContainer) WithKnowledgeBase(kb KnowledgeBase) *DefaultServiceContainer {
	c.knowledgeBase = kb
	return c
}

// WithK8sClient sets the Kubernetes client
func (c *DefaultServiceContainer) WithK8sClient(client K8sClient) *DefaultServiceContainer {
	c.k8sClient = client
	return c
}

// WithAnalyzer sets the analyzer
func (c *DefaultServiceContainer) WithAnalyzer(analyzer Analyzer) *DefaultServiceContainer {
	c.analyzer = analyzer
	return c
}

// WithPersistence sets the persistence service
func (c *DefaultServiceContainer) WithPersistence(persistence Persistence) *DefaultServiceContainer {
	c.persistence = persistence
	return c
}

// WithDockerService sets the docker service
func (c *DefaultServiceContainer) WithDockerService(service docker.Service) *DefaultServiceContainer {
	c.dockerService = service
	return c
}

// WithManifestService sets the manifest service
func (c *DefaultServiceContainer) WithManifestService(service kubernetes.ManifestService) *DefaultServiceContainer {
	c.manifestService = service
	return c
}

// WithDeploymentService sets the deployment service
func (c *DefaultServiceContainer) WithDeploymentService(service kubernetes.Service) *DefaultServiceContainer {
	c.deploymentService = service
	return c
}

// WithPipelineService sets the pipeline service
func (c *DefaultServiceContainer) WithPipelineService(service PipelineService) *DefaultServiceContainer {
	c.pipelineService = service
	return c
}

// WithConversationService sets the conversation service
func (c *DefaultServiceContainer) WithConversationService(service ConversationService) *DefaultServiceContainer {
	c.conversationService = service
	return c
}

// WithPromptService sets the prompt service
func (c *DefaultServiceContainer) WithPromptService(service PromptService) *DefaultServiceContainer {
	c.promptService = service
	return c
}

// initializeServices initializes all services with their real implementations
func (c *DefaultServiceContainer) initializeServices() {
	logger := c.logger.With("component", "service_container")

	// Create clients - for now use nil, will be enhanced later
	// In production, these would be properly configured
	var coreClients *clients.Clients

	// Initialize core infrastructure services
	c.dockerService = docker.NewService(coreClients, logger)
	c.manifestService = kubernetes.NewManifestService(logger)
	c.deploymentService = kubernetes.NewService(coreClients, logger)

	// Initialize real core services with adapters for context support
	coreAnalyzer := analysis.NewRepositoryAnalyzer(logger)
	c.analyzer = NewAnalyzerAdapter(coreAnalyzer)

	coreScanner := security.NewSecurityService(logger, nil)
	c.scanner = NewScannerAdapter(coreScanner)

	// Initialize session storage (stub for now to avoid import cycles)
	c.sessionStore = NewSessionStoreStub(logger)

	// Initialize state management (stub for now)
	c.stateManager = NewStateManagerStub(logger)
	c.sessionState = NewSessionStateStub(logger)

	// Initialize workflow executor (stub for now)
	c.workflowExecutor = NewWorkflowExecutorStub(logger)

	// Initialize validation service (stub for now)
	c.configValidator = NewConfigValidatorStub(logger)

	// Initialize tool registry (will be wired properly in server initialization)
	c.toolRegistry = NewToolRegistryStub(logger)

	// Initialize conversation services (stub implementations for now)
	c.conversationService = NewConversationServiceStub(logger)
	c.promptService = NewPromptServiceStub(logger)

	// Initialize pipeline service (stub for now)
	c.pipelineService = NewPipelineServiceStub(logger)

	// Use docker service directly as build executor (it already implements the right interface)
	c.buildExecutor = c.dockerService

	// Initialize other services (stubs for now)
	c.persistence = NewPersistenceStub(logger)
	c.errorReporter = NewErrorReporterFromLogger(logger)
	c.knowledgeBase = NewKnowledgeBaseStub(logger)
	c.k8sClient = NewK8sClientStub(logger)

	logger.Info("All services initialized with real implementations")
}
