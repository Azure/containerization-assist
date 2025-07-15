// Package core provides unified dependency injection for core infrastructure services
package core

import (
	"log/slog"
	"os"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/filesystem"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/validation"
	"github.com/google/wire"
)

// Providers provides all core infrastructure dependencies
var Providers = wire.NewSet(
	// Command runner
	ProvideCommandRunner,

	// Filesystem operations - using existing constructor
	filesystem.NewFileSystemManager,

	// Validation - using existing constructor
	validation.NewPreflightValidator,

	// Placeholders for when more constructors are created
	// Middleware
	// retry.NewRetryMiddleware,
	// trace.NewTraceMiddleware,

	// Resource management
	// resources.NewResourceProvider,

	// Utilities
	// utilities.NewAIRetryHandler,

	// Interface bindings would go here if needed
)

// ProvideLogger creates a structured logger instance
func ProvideLogger() *slog.Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return slog.New(handler)
}

// ProvideCommandRunner creates a command runner instance
func ProvideCommandRunner() runner.CommandRunner {
	return &runner.DefaultCommandRunner{}
}
