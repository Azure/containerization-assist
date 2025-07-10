package runtime

import (
	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
	"github.com/localrivet/gomcp/server"
)

type StandardToolRegistrar struct {
	server server.Server
	logger logging.Standards
}

func NewStandardToolRegistrar(s server.Server, logger logging.Standards) *StandardToolRegistrar {
	return &StandardToolRegistrar{
		server: s,
		logger: logger.WithComponent("tool_registrar"),
	}
}

type AtomicTool[TArgs, TResult any] interface {
	ExecuteWithContext(ctx *server.Context, args *TArgs) (*TResult, error)
}

func RegisterAtomicTool[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	tool AtomicTool[TArgs, TResult],
) {
	r.logger.Debug().Str("tool", name).Msg("Registering atomic tool")

	r.server.Tool(name, description, func(ctx *server.Context, args *TArgs) (*TResult, error) {
		return tool.ExecuteWithContext(ctx, args)
	})

	r.logger.Info().Str("tool", name).Msg("Atomic tool registered successfully")
}

type SimpleToolFunc[TArgs, TResult any] func(ctx *server.Context, args *TArgs) (*TResult, error)

func RegisterSimpleTool[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	toolFunc SimpleToolFunc[TArgs, TResult],
) {
	r.logger.Debug().Str("tool", name).Msg("Registering simple tool")

	r.server.Tool(name, description, toolFunc)

	r.logger.Info().Str("tool", name).Msg("Simple tool registered successfully")
}

type UtilityToolFunc[TArgs, TResult any] func(deps interface{}) (func(ctx *server.Context, args *TArgs) (*TResult, error), error)

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

type ResourceFunc[TArgs any] func(ctx *server.Context, args TArgs) (interface{}, error)

func RegisterResource[TArgs any](
	r *StandardToolRegistrar,
	path, description string,
	resourceFunc ResourceFunc[TArgs],
) {
	r.logger.Debug().Str("resource", path).Msg("Registering resource")

	r.server.Resource(path, description, resourceFunc)

	r.logger.Info().Str("resource", path).Msg("Resource registered successfully")
}
func RegisterSimpleToolWithFixedSchema[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	toolFunc SimpleToolFunc[TArgs, TResult],
) {

	RegisterSimpleTool(r, name, description, toolFunc)
}
