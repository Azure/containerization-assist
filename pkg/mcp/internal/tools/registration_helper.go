package tools

import (
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// StandardToolRegistrar provides a consistent interface for registering tools with GoMCP
type StandardToolRegistrar struct {
	server server.Server
	logger zerolog.Logger
}

// NewStandardToolRegistrar creates a new tool registrar
func NewStandardToolRegistrar(s server.Server, logger zerolog.Logger) *StandardToolRegistrar {
	return &StandardToolRegistrar{
		server: s,
		logger: logger.With().Str("component", "tool_registrar").Logger(),
	}
}

// AtomicTool represents a standardized atomic tool interface
type AtomicTool[TArgs, TResult any] interface {
	ExecuteWithContext(ctx *server.Context, args TArgs) (*TResult, error)
}

// RegisterAtomicTool registers an atomic tool with consistent patterns
func RegisterAtomicTool[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	tool AtomicTool[TArgs, TResult],
) {
	r.logger.Debug().Str("tool", name).Msg("Registering atomic tool")

	r.server.Tool(name, description, func(ctx *server.Context, args *TArgs) (*TResult, error) {
		return tool.ExecuteWithContext(ctx, *args)
	})

	r.logger.Info().Str("tool", name).Msg("Atomic tool registered successfully")
}

// SimpleToolFunc represents a simple tool function
type SimpleToolFunc[TArgs, TResult any] func(ctx *server.Context, args *TArgs) (*TResult, error)

// RegisterSimpleTool registers a simple tool function with consistent patterns
func RegisterSimpleTool[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	toolFunc SimpleToolFunc[TArgs, TResult],
) {
	r.logger.Debug().Str("tool", name).Msg("Registering simple tool")

	r.server.Tool(name, description, toolFunc)

	r.logger.Info().Str("tool", name).Msg("Simple tool registered successfully")
}

// UtilityToolFunc represents a utility tool that creates tools inline (legacy pattern)
type UtilityToolFunc[TArgs, TResult any] func(deps interface{}) (func(ctx *server.Context, args *TArgs) (*TResult, error), error)

// RegisterUtilityTool registers a utility tool with dependency injection
func RegisterUtilityTool[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	deps interface{},
	toolCreator UtilityToolFunc[TArgs, TResult],
) error {
	r.logger.Debug().Str("tool", name).Msg("Registering utility tool")

	toolFunc, err := toolCreator(deps)
	if err != nil {
		r.logger.Error().Err(err).Str("tool", name).Msg("Failed to create utility tool")
		return err
	}

	r.server.Tool(name, description, toolFunc)

	r.logger.Info().Str("tool", name).Msg("Utility tool registered successfully")
	return nil
}

// ResourceFunc represents a resource handler function
type ResourceFunc[TArgs any] func(ctx *server.Context, args TArgs) (interface{}, error)

// RegisterResource registers a GoMCP resource with consistent patterns
func RegisterResource[TArgs any](
	r *StandardToolRegistrar,
	path, description string,
	resourceFunc ResourceFunc[TArgs],
) {
	r.logger.Debug().Str("resource", path).Msg("Registering resource")

	r.server.Resource(path, description, resourceFunc)

	r.logger.Info().Str("resource", path).Msg("Resource registered successfully")
}

// ToolDependencies is defined in gomcp_tools.go to avoid circular imports
