// Package persistence provides unified dependency injection for data persistence services
package persistence

import (
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/persistence/session"
	"github.com/google/wire"
)

// PersistenceProviders provides all persistence domain dependencies
var PersistenceProviders = wire.NewSet(
	// Session management
	session.NewBoltStore,

	// State storage - using existing constructor
	NewFileStateStore,

	// Interface bindings would go here if needed
)
