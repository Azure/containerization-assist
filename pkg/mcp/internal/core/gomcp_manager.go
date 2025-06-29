package core

import (
	"context"
	"log/slog"
	"os"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/transport"
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
	transport     transport.LocalTransport // Injected transport
	isInitialized bool                     // Prevent mutation after creation
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
func (gm *GomcpManager) WithTransport(t transport.LocalTransport) *GomcpManager {
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
		return errors.Internal("core/gomcp-manager", "manager already initialized")
	}

	// Validate transport is set
	if gm.transport == nil {
		return errors.Config("core/gomcp-manager", "transport must be set before initialization")
	}

	// Create gomcp server with stdio transport
	// AsStdio() must be chained directly with NewServer() for proper initialization
	gm.server = server.NewServer(gm.config.Name,
		server.WithLogger(&gm.logger),
		server.WithProtocolVersion(gm.config.ProtocolVersion),
	).AsStdio()

	// Verify server was created successfully
	if gm.server == nil {
		return errors.Internal("core/gomcp-manager", "failed to create stdio server: NewServer().AsStdio() returned nil")
	}

	gm.isInitialized = true
	return nil
}

// GetServer returns the underlying gomcp server
func (gm *GomcpManager) GetServer() server.Server {
	return gm.server
}

// GetTransport returns the configured transport
func (gm *GomcpManager) GetTransport() transport.LocalTransport {
	return gm.transport
}

// StartServer starts the gomcp server after all tools are registered
func (gm *GomcpManager) StartServer() error {
	if !gm.isInitialized {
		return errors.Internal("core/gomcp-manager", "manager not initialized")
	}
	if gm.server == nil {
		return errors.Internal("core/gomcp-manager", "server is nil - initialization may have failed")
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
			if gm.server != nil {
				if err := gm.server.Shutdown(); err != nil {
					gm.logger.Error("error shutting down gomcp server", "error", err)
					shutdownErrors = append(shutdownErrors, err)
				} else {
					gm.logger.Info("gomcp server shut down successfully")
				}
			} else {
				gm.logger.Warn("gomcp server is nil during shutdown")
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
		return errors.Wrapf(shutdownErrors[0], "core/gomcp-manager", "shutdown completed with %d errors", len(shutdownErrors))
	}

	gm.logger.Info("gomcp manager shutdown completed successfully")
	return nil
}
