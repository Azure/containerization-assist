// Package main shows an example of using Wire for dependency injection
package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/wire"
)

func main() {
	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create configuration
	config := workflow.DefaultServerConfig()

	// Create server using Wire
	ctx := context.Background()
	server, err := wire.InitializeServer(logger, config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start the server
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// To generate Wire code:
// 1. Install Wire: go install github.com/google/wire/cmd/wire@latest
// 2. Run: go generate ./pkg/mcp/wire
// 3. The wire_gen.go file will be updated with the generated code
