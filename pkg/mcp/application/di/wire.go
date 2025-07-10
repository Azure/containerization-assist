//go:build wireinject
// +build wireinject

//go:generate wire

package di

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/google/wire"
)

// Container holds all application services with dependency injection
type Container struct {
	ToolRegistry     api.ToolRegistry
	SessionStore     services.SessionStore
	SessionState     services.SessionState
	BuildExecutor    services.BuildExecutor
	WorkflowExecutor services.WorkflowExecutor
	Scanner          services.Scanner
	ConfigValidator  services.ConfigValidator
	ErrorReporter    services.ErrorReporter
}

// InitializeContainer creates a fully wired container with all dependencies
func InitializeContainer() (*Container, error) {
	wire.Build(
		// Registry provider
		NewToolRegistry,

		// Service providers
		NewSessionStore,
		NewSessionState,
		NewBuildExecutor,
		NewWorkflowExecutor,
		NewScanner,
		NewConfigValidator,
		NewErrorReporter,

		// Container construction
		wire.Struct(new(Container), "*"),
	)
	return &Container{}, nil
}
