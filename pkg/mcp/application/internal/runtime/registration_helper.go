package runtime

import (
	"log/slog"

	"github.com/localrivet/gomcp/server"
)

type StandardToolRegistrar struct {
	server server.Server
	logger *slog.Logger
}

func NewStandardToolRegistrar(s server.Server, logger *slog.Logger) *StandardToolRegistrar {
	return &StandardToolRegistrar{
		server: s,
		logger: logger.With("component", "tool_registrar"),
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
	r.logger.Debug("Registering atomic tool",

		"tool", name)

	r.server.Tool(name, description, func(ctx *server.Context, args *TArgs) (*TResult, error) {
		return tool.ExecuteWithContext(ctx, args)
	})

	r.logger.Info("Atomic tool registered successfully",

		"tool", name)
}

type SimpleToolFunc[TArgs, TResult any] func(ctx *server.Context, args *TArgs) (*TResult, error)

func RegisterSimpleTool[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	toolFunc SimpleToolFunc[TArgs, TResult],
) {
	r.logger.Debug("Registering simple tool",

		"tool", name)

	r.server.Tool(name, description, toolFunc)

	r.logger.Info("Simple tool registered successfully",

		"tool", name)
}

type UtilityToolFunc[TArgs, TResult any] func(deps interface{}) (func(ctx *server.Context, args *TArgs) (*TResult, error), error)

func RegisterUtilityTool[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	deps interface{},
	toolCreator UtilityToolFunc[TArgs, TResult],
) error {
	r.logger.Debug("Registering utility tool",

		"tool", name)

	toolFunc, err := toolCreator(deps)
	if err != nil {
		r.logger.Error("Failed to create utility tool",

			"error", err,

			"tool", name)
		return err
	}

	r.server.Tool(name, description, toolFunc)

	r.logger.Info("Utility tool registered successfully",

		"tool", name)
	return nil
}

type ResourceFunc[TArgs any] func(ctx *server.Context, args TArgs) (interface{}, error)

func RegisterResource[TArgs any](
	r *StandardToolRegistrar,
	path, description string,
	resourceFunc ResourceFunc[TArgs],
) {
	r.logger.Debug("Registering resource",

		"resource", path)

	r.server.Resource(path, description, resourceFunc)

	r.logger.Info("Resource registered successfully",

		"resource", path)
}
func RegisterSimpleToolWithFixedSchema[TArgs, TResult any](
	r *StandardToolRegistrar,
	name, description string,
	toolFunc SimpleToolFunc[TArgs, TResult],
) {

	RegisterSimpleTool(r, name, description, toolFunc)
}
