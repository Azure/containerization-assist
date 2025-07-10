package di

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
)

// NewToolRegistry creates a new unified tool registry instance
func NewToolRegistry() api.ToolRegistry {
	return registry.NewUnified()
}

// NewSessionStore creates a new session store backed by BoltDB
func NewSessionStore() services.SessionStore {
	// TODO: Add proper configuration
	return &sessionStoreStub{}
}

// NewSessionState creates a new session state manager
func NewSessionState() services.SessionState {
	// TODO: Add proper implementation
	return &sessionStateStub{}
}

// NewBuildExecutor creates a new build executor for Docker operations
func NewBuildExecutor() services.BuildExecutor {
	// TODO: Add proper Docker client initialization
	return &buildExecutorStub{}
}

// NewToolRegistryService creates a new tool registry service wrapper
func NewToolRegistryService(registry api.ToolRegistry) services.ToolRegistry {
	return &toolRegistryServiceAdapter{registry: registry}
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(registry api.ToolRegistry) services.WorkflowExecutor {
	// TODO: Add proper implementation
	return &workflowExecutorStub{}
}

// NewScanner creates a new security scanner
func NewScanner() services.Scanner {
	// TODO: Add proper Trivy/Grype integration
	return &scannerStub{}
}

// NewConfigValidator creates a new config validator
func NewConfigValidator() services.ConfigValidator {
	// TODO: Add proper validation implementation
	return &configValidatorStub{}
}

// NewErrorReporter creates a new error reporter
func NewErrorReporter() services.ErrorReporter {
	// TODO: Add proper error reporting implementation
	return &errorReporterStub{}
}
