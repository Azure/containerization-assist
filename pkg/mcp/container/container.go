// Package di provides dependency injection container for MCP services.
package di

import (
	"os"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/Azure/container-kit/pkg/mcp/services/build"
	serviceerrors "github.com/Azure/container-kit/pkg/mcp/services/errors"
	appregistry "github.com/Azure/container-kit/pkg/mcp/app/registry"
	"github.com/Azure/container-kit/pkg/mcp/services/scanner"
	"github.com/Azure/container-kit/pkg/mcp/services/session"
	"github.com/Azure/container-kit/pkg/mcp/services/validation"
	"github.com/Azure/container-kit/pkg/mcp/services/workflow"
	"github.com/rs/zerolog"
)

// Container implements ServiceContainer interface and provides dependency injection
type Container struct {
	sessionStore     services.SessionStore
	sessionState     services.SessionState
	buildExecutor    services.BuildExecutor
	toolRegistry     services.ToolRegistry
	workflowExecutor services.WorkflowExecutor
	scanner          services.Scanner
	configValidator  services.ConfigValidator
	errorReporter    services.ErrorReporter
	config           *Config
	logger           zerolog.Logger
}

// Config provides configuration for the DI container
type Config struct {
	DatabasePath string
	LogLevel     string
	Environment  string
}

// NewContainer creates a new dependency injection container
func NewContainer(config *Config) (*Container, error) {
	if config == nil {
		config = &Config{
			DatabasePath: "./sessions.db",
			LogLevel:     "info",
			Environment:  "development",
		}
	}

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("component", "container-kit").
		Str("environment", config.Environment).
		Logger()

	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	logger = logger.Level(level)

	container := &Container{
		config: config,
		logger: logger,
	}

	if err := container.initializeServices(); err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message("failed to initialize services").
			Cause(err).Build()
	}

	return container, nil
}

// initializeServices initializes all services in the correct dependency order
func (c *Container) initializeServices() error {
	c.logger.Info().Msg("Initializing independent services")

	c.errorReporter = serviceerrors.NewErrorReporter(c.logger)

	c.configValidator = validation.NewUnifiedConfigValidator()

	sessionStore, err := session.NewBoltSessionStore(c.config.DatabasePath)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message("failed to create session store").
			Cause(err).Build()
	}
	c.sessionStore = sessionStore

	sessionState, err := session.NewBoltSessionState(c.config.DatabasePath)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message("failed to create session state").
			Cause(err).Build()
	}
	c.sessionState = sessionState

	c.toolRegistry = appregistry.NewMemoryToolRegistry()

	c.scanner = scanner.NewSecurityScanner()

	c.logger.Info().Msg("Initializing dependent services")

	buildExecutor, err := build.NewDockerBuildExecutor(c.configValidator)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message("failed to create build executor").
			Cause(err).Build()
	}
	c.buildExecutor = buildExecutor

	c.workflowExecutor = workflow.NewWorkflowExecutor(c.sessionState, c.toolRegistry)

	c.logger.Info().Msg("All services initialized successfully")
	return nil
}

// SessionStore implements ServiceContainer.SessionStore
func (c *Container) SessionStore() services.SessionStore {
	return c.sessionStore
}

// SessionState implements ServiceContainer.SessionState
func (c *Container) SessionState() services.SessionState {
	return c.sessionState
}

// BuildExecutor implements ServiceContainer.BuildExecutor
func (c *Container) BuildExecutor() services.BuildExecutor {
	return c.buildExecutor
}

// ToolRegistry implements ServiceContainer.ToolRegistry
func (c *Container) ToolRegistry() services.ToolRegistry {
	return c.toolRegistry
}

// WorkflowExecutor implements ServiceContainer.WorkflowExecutor
func (c *Container) WorkflowExecutor() services.WorkflowExecutor {
	return c.workflowExecutor
}

// Scanner implements ServiceContainer.Scanner
func (c *Container) Scanner() services.Scanner {
	return c.scanner
}

// ConfigValidator implements ServiceContainer.ConfigValidator
func (c *Container) ConfigValidator() services.ConfigValidator {
	return c.configValidator
}

// ErrorReporter implements ServiceContainer.ErrorReporter
func (c *Container) ErrorReporter() services.ErrorReporter {
	return c.errorReporter
}

// Close implements ServiceContainer.Close
func (c *Container) Close() error {
	c.logger.Info().Msg("Shutting down services")

	var lastError error

	if c.workflowExecutor != nil {
		if closer, ok := c.workflowExecutor.(*workflow.WorkflowExecutorImpl); ok {
			if err := closer.Close(); err != nil {
				c.logger.Error().Err(err).Msg("Failed to close workflow executor")
				lastError = err
			}
		}
	}

	if c.buildExecutor != nil {
		if closer, ok := c.buildExecutor.(*build.DockerBuildExecutor); ok {
			if err := closer.Close(); err != nil {
				c.logger.Error().Err(err).Msg("Failed to close build executor")
				lastError = err
			}
		}
	}

	if c.scanner != nil {
		if closer, ok := c.scanner.(*scanner.SecurityScannerImpl); ok {
			if err := closer.Close(); err != nil {
				c.logger.Error().Err(err).Msg("Failed to close scanner")
				lastError = err
			}
		}
	}

	if c.toolRegistry != nil {
		if closer, ok := c.toolRegistry.(*appregistry.MemoryToolRegistry); ok {
			if err := closer.Close(); err != nil {
				c.logger.Error().Err(err).Msg("Failed to close tool registry")
				lastError = err
			}
		}
	}

	if c.sessionState != nil {
		if closer, ok := c.sessionState.(*session.BoltSessionState); ok {
			if err := closer.Close(); err != nil {
				c.logger.Error().Err(err).Msg("Failed to close session state")
				lastError = err
			}
		}
	}

	if c.sessionStore != nil {
		if closer, ok := c.sessionStore.(*session.BoltSessionStore); ok {
			if err := closer.Close(); err != nil {
				c.logger.Error().Err(err).Msg("Failed to close session store")
				lastError = err
			}
		}
	}

	if lastError != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message("failed to close some services").
			Cause(lastError).Build()
	}

	c.logger.Info().Msg("All services shut down successfully")
	return nil
}

// GetLogger returns the container's logger
func (c *Container) GetLogger() zerolog.Logger {
	return c.logger
}

// GetConfig returns the container's configuration
func (c *Container) GetConfig() *Config {
	return c.config
}
