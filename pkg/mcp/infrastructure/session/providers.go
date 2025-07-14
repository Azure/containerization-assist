// Package session provides Wire providers for session infrastructure
package session

import (
	"log/slog"

	domainsession "github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/google/wire"
)

// ProviderSet provides all session infrastructure dependencies
var ProviderSet = wire.NewSet(
	ProvideBoltStore,
	wire.Bind(new(domainsession.Store), new(*BoltStore)),
)

// ProvideBoltStore creates a BoltDB session store
func ProvideBoltStore(dbPath string, logger *slog.Logger) (*BoltStore, error) {
	return NewBoltStore(dbPath, logger)
}

// ProvideDefaultBoltStore creates a BoltDB session store with default path
func ProvideDefaultBoltStore(logger *slog.Logger) (*BoltStore, error) {
	return NewBoltStore("/tmp/container-kit/sessions.db", logger)
}
