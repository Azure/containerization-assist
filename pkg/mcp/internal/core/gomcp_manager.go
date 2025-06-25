package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
)

// GomcpConfig holds configuration for the gomcp server
type GomcpConfig struct {
	Name            string
	ProtocolVersion string
	LogLevel        slog.Level
}

// GomcpManager manages the gomcp server and tool registration
type GomcpManager struct {
	server        server.Server
	config        GomcpConfig
	logger        slog.Logger
	transport     mcptypes.Transport // Injected transport
	isInitialized bool                       // Prevent mutation after creation
}

// NewGomcpManager creates a new gomcp manager with builder pattern
func NewGomcpManager(config GomcpConfig) *GomcpManager {
	// Create slog logger
	slogHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: config.LogLevel,
	})
	logger := *slog.New(slogHandler)

	return &GomcpManager{
		config:        config,
		logger:        logger,
		isInitialized: false,
	}
}

// WithTransport sets the transport for the gomcp manager
func (gm *GomcpManager) WithTransport(t mcptypes.Transport) *GomcpManager {
	if gm.isInitialized {
		gm.logger.Error("cannot set transport: manager already initialized")
		return gm
	}
	gm.transport = t
	return gm
}

// WithLogger updates the logger for the gomcp manager
func (gm *GomcpManager) WithLogger(logger slog.Logger) *GomcpManager {
	if gm.isInitialized {
		gm.logger.Error("cannot set logger: manager already initialized")
		return gm
	}
	gm.logger = logger
	return gm
}

// Initialize creates and configures the gomcp server
func (gm *GomcpManager) Initialize() error {
	if gm.isInitialized {
		return fmt.Errorf("manager already initialized")
	}

	// Validate transport is set
	if gm.transport == nil {
		return fmt.Errorf("transport must be set before initialization")
	}

	// Create gomcp server
	gm.server = server.NewServer(gm.config.Name,
		server.WithLogger(&gm.logger),
		server.WithProtocolVersion(gm.config.ProtocolVersion),
	)

	// Configure transport - default to stdio
	// Since Transport interface doesn't have Name() method,
	// we'll use stdio as the default transport type
	gm.server = gm.server.AsStdio()

	gm.isInitialized = true
	return nil
}

// GetServer returns the underlying gomcp server
func (gm *GomcpManager) GetServer() server.Server {
	return gm.server
}

// GetTransport returns the configured transport
func (gm *GomcpManager) GetTransport() mcptypes.Transport {
	return gm.transport
}

// StartServer starts the gomcp server after all tools are registered
func (gm *GomcpManager) StartServer() error {
	if !gm.isInitialized {
		return fmt.Errorf("manager not initialized")
	}
	gm.logger.Info("Starting gomcp server with all tools registered")
	return gm.server.Run()
}

// IsInitialized returns whether the manager has been initialized
func (gm *GomcpManager) IsInitialized() bool {
	return gm.isInitialized
}

// Shutdown gracefully shuts down the gomcp server
func (gm *GomcpManager) Shutdown(ctx context.Context) error {
	if !gm.isInitialized {
		return nil
	}

	gm.logger.Info("shutting down gomcp server")

	// Create error collector for potential errors during shutdown
	var shutdownErrors []error

	// Shutdown the underlying gomcp server if available
	if gm.server != nil {
		select {
		case <-ctx.Done():
			gm.logger.Warn("shutdown context cancelled before server shutdown")
			shutdownErrors = append(shutdownErrors, ctx.Err())
		default:
			// Attempt graceful shutdown of the server
			if err := gm.server.Shutdown(); err != nil {
				gm.logger.Error("error shutting down gomcp server", "error", err)
				shutdownErrors = append(shutdownErrors, err)
			} else {
				gm.logger.Info("gomcp server shut down successfully")
			}
		}
	}

	// Shutdown the transport if available
	if gm.transport != nil {
		select {
		case <-ctx.Done():
			gm.logger.Warn("shutdown context cancelled before transport shutdown")
			shutdownErrors = append(shutdownErrors, ctx.Err())
		default:
			// Stop the transport
			if err := gm.transport.Stop(ctx); err != nil {
				gm.logger.Error("error stopping transport", "error", err)
				shutdownErrors = append(shutdownErrors, err)
			} else {
				gm.logger.Info("transport stopped successfully")
			}
		}
	}

	// Mark as not initialized
	gm.isInitialized = false

	// Return first error if any occurred
	if len(shutdownErrors) > 0 {
		return fmt.Errorf("shutdown completed with %d errors, first error: %w", len(shutdownErrors), shutdownErrors[0])
	}

	gm.logger.Info("gomcp manager shutdown completed successfully")
	return nil
}
