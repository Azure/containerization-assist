//go:build !wireinject
// +build !wireinject

package wire

import (
	"github.com/Azure/container-kit/pkg/mcp/application"
)

func init() {
	// Register the wire-generated factories with the application package
	// This avoids import cycles by having wire depend on application,
	// but application not directly depending on wire
	application.SetServerFactories(InitializeServer, InitializeServerWithConfig)
}
