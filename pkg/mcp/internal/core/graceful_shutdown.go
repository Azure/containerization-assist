package core

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

// GracefulShutdownManager handles coordinated shutdown of services
type GracefulShutdownManager struct {
	logger   zerolog.Logger
	mu       sync.RWMutex
	services []ShutdownService
	timeout  time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	started  bool
}

// ShutdownService defines the interface for services that can be gracefully shutdown
type ShutdownService interface {
	// Shutdown gracefully shuts down the service within the given context
	Shutdown(ctx context.Context) error
	// Name returns the service name for logging
	Name() string
}

// NewGracefulShutdownManager creates a new graceful shutdown manager
func NewGracefulShutdownManager(logger zerolog.Logger, timeout time.Duration) *GracefulShutdownManager {
	if timeout <= 0 {
		timeout = 30 * time.Second // Default 30 second timeout
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &GracefulShutdownManager{
		logger:  logger.With().Str("component", "graceful_shutdown").Logger(),
		timeout: timeout,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
}

// RegisterService registers a service for graceful shutdown
func (gsm *GracefulShutdownManager) RegisterService(service ShutdownService) {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	gsm.services = append(gsm.services, service)
	gsm.logger.Info().Str("service", service.Name()).Msg("Registered service for graceful shutdown")
}

// Start begins listening for shutdown signals
func (gsm *GracefulShutdownManager) Start() {
	gsm.mu.Lock()
	if gsm.started {
		gsm.mu.Unlock()
		return
	}
	gsm.started = true
	gsm.mu.Unlock()

	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		// Wait for shutdown signal
		sig := <-sigChan
		gsm.logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

		// Trigger shutdown
		gsm.shutdown()
	}()

	gsm.logger.Info().Msg("Graceful shutdown manager started")
}

// Shutdown manually triggers graceful shutdown
func (gsm *GracefulShutdownManager) Shutdown() {
	gsm.shutdown()
}

// WaitForShutdown blocks until shutdown is complete
func (gsm *GracefulShutdownManager) WaitForShutdown() {
	<-gsm.done
}

// Context returns the shutdown context that gets cancelled on shutdown
func (gsm *GracefulShutdownManager) Context() context.Context {
	return gsm.ctx
}

// shutdown performs the actual shutdown process
func (gsm *GracefulShutdownManager) shutdown() {
	gsm.logger.Info().Msg("Beginning graceful shutdown")

	// Cancel the context to signal all listeners
	gsm.cancel()

	// Create timeout context for shutdown operations
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gsm.timeout)
	defer cancel()

	// Shutdown services in reverse order
	gsm.mu.RLock()
	services := make([]ShutdownService, len(gsm.services))
	copy(services, gsm.services)
	gsm.mu.RUnlock()

	// Shutdown services concurrently with individual timeouts
	var wg sync.WaitGroup
	for i := len(services) - 1; i >= 0; i-- {
		service := services[i]
		wg.Add(1)

		go func(svc ShutdownService) {
			defer wg.Done()

			// Create individual service timeout (half of total timeout)
			svcCtx, svcCancel := context.WithTimeout(shutdownCtx, gsm.timeout/2)
			defer svcCancel()

			gsm.logger.Info().Str("service", svc.Name()).Msg("Shutting down service")

			if err := svc.Shutdown(svcCtx); err != nil {
				gsm.logger.Error().
					Err(err).
					Str("service", svc.Name()).
					Msg("Error during service shutdown")
			} else {
				gsm.logger.Info().Str("service", svc.Name()).Msg("Service shutdown completed")
			}
		}(service)
	}

	// Wait for all services to shutdown or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		gsm.logger.Info().Msg("All services shutdown gracefully")
	case <-shutdownCtx.Done():
		gsm.logger.Warn().Msg("Graceful shutdown timeout exceeded")
	}

	// Signal completion
	close(gsm.done)
	gsm.logger.Info().Msg("Graceful shutdown completed")
}

// ServiceWrapper wraps a simple shutdown function as a ShutdownService
type ServiceWrapper struct {
	name     string
	shutdown func(context.Context) error
}

// NewServiceWrapper creates a ShutdownService from a function
func NewServiceWrapper(name string, shutdownFunc func(context.Context) error) ShutdownService {
	return &ServiceWrapper{
		name:     name,
		shutdown: shutdownFunc,
	}
}

func (sw *ServiceWrapper) Name() string {
	return sw.name
}

func (sw *ServiceWrapper) Shutdown(ctx context.Context) error {
	return sw.shutdown(ctx)
}

// Integration helpers for common services

// HTTPServerService wraps an HTTP server for graceful shutdown
type HTTPServerService struct {
	name   string
	server interface{ Shutdown(context.Context) error }
}

// NewHTTPServerService creates a ShutdownService for HTTP servers
func NewHTTPServerService(name string, server interface{ Shutdown(context.Context) error }) ShutdownService {
	return &HTTPServerService{
		name:   name,
		server: server,
	}
}

func (hss *HTTPServerService) Name() string {
	return hss.name
}

func (hss *HTTPServerService) Shutdown(ctx context.Context) error {
	return hss.server.Shutdown(ctx)
}
