package services

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
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

// NewDefaultServiceContainer creates a new service container
func NewDefaultServiceContainer(logger *slog.Logger) *DefaultServiceContainer {
	return &DefaultServiceContainer{
		logger: logger,
	}
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
