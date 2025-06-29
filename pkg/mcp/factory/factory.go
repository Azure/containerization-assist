// Package factory provides the factory implementation for creating MCP servers
// This package bridges the public interfaces and internal implementations
package factory

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/core"
)

// NewServer creates a new MCP server using the internal core implementation
func NewServer(ctx context.Context, config core.ServerConfig) (core.Server, error) {
	// Convert core.ServerConfig to core.ServerConfig
	coreConfig := core.ServerConfig{
		WorkspaceDir:      config.WorkspaceDir,
		MaxSessions:       config.MaxSessions,
		SessionTTL:        config.SessionTTL,
		MaxDiskPerSession: config.MaxDiskPerSession,
		TotalDiskLimit:    config.TotalDiskLimit,
		StorePath:         config.StorePath,
		TransportType:     config.TransportType,
		HTTPAddr:          config.HTTPAddr,
		HTTPPort:          config.HTTPPort,
		SandboxEnabled:    config.SandboxEnabled,
		LogLevel:          config.LogLevel,
		LogHTTPBodies:     config.LogHTTPBodies,
		MaxBodyLogSize:    config.MaxBodyLogSize,
		CleanupInterval:   config.CleanupInterval,
		MaxWorkers:        config.MaxWorkers,
		JobTTL:            config.JobTTL,
		EnableOTEL:        config.EnableOTEL,
		OTELEndpoint:      config.OTELEndpoint,
		OTELHeaders:       config.OTELHeaders,
		ServiceName:       config.ServiceName,
		ServiceVersion:    config.ServiceVersion,
		Environment:       config.Environment,
		TraceSampleRate:   config.TraceSampleRate,
	}

	// Create the core server
	coreServer, err := core.NewServer(ctx, coreConfig)
	if err != nil {
		return nil, err
	}

	// Wrap it in an adapter to implement the public interface
	return &serverAdapter{coreServer: coreServer}, nil
}

// serverAdapter adapts the internal core.Server to implement core.Server
type serverAdapter struct {
	coreServer *core.Server
}

func (s *serverAdapter) Start(ctx context.Context) error {
	return s.coreServer.Start(ctx)
}

func (s *serverAdapter) Stop() error {
	return s.coreServer.Stop()
}

func (s *serverAdapter) Shutdown(ctx context.Context) error {
	return s.coreServer.Shutdown(ctx)
}

func (s *serverAdapter) EnableConversationMode(config core.ConversationConfig) error {
	// Convert core.ConversationConfig to core.ConversationConfig
	coreConfig := core.ConversationConfig{
		EnableTelemetry:   config.EnableTelemetry,
		TelemetryPort:     config.TelemetryPort,
		PreferencesDBPath: config.PreferencesDBPath,
		EnableOTEL:        config.EnableOTEL,
		OTELEndpoint:      config.OTELEndpoint,
		OTELHeaders:       config.OTELHeaders,
		ServiceName:       config.ServiceName,
		ServiceVersion:    config.ServiceVersion,
		Environment:       config.Environment,
		TraceSampleRate:   config.TraceSampleRate,
	}
	return s.coreServer.EnableConversationMode(coreConfig)
}

func (s *serverAdapter) GetStats() *core.ServerStats {
	// Convert core stats to mcp stats
	coreStats := s.coreServer.GetStats()
	if coreStats == nil {
		return &core.ServerStats{}
	}

	// Get the start time from the server
	startTime := s.coreServer.GetStartTime()

	return &core.ServerStats{
		Transport: coreStats.Transport,
		Sessions:  s.GetSessionManagerStats(),
		Workspace: s.GetWorkspaceStats(),
		Uptime:    coreStats.Uptime,
		StartTime: startTime,
	}
}

func (s *serverAdapter) GetSessionManagerStats() *core.SessionManagerStats {
	// Get stats from core server and convert
	// For now, return empty stats
	return &core.SessionManagerStats{}
}

func (s *serverAdapter) GetWorkspaceStats() *core.WorkspaceStats {
	// Get stats from core server and convert
	// For now, return empty stats
	return &core.WorkspaceStats{}
}

func (s *serverAdapter) GetLogger() interface{} {
	return s.coreServer.GetLogger()
}
