// Package core provides unified dependency injection for core infrastructure services
package core

import (
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/filesystem"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/validation"
	"github.com/google/wire"
)

// CoreProviders provides all core infrastructure dependencies
var CoreProviders = wire.NewSet(
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
