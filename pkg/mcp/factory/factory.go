// Package factory provides the factory implementation for creating MCP servers
// This package bridges the public interfaces and internal implementations
package factory

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/core"
)

// NewServer creates a new MCP server using the internal core implementation
func NewServer(ctx context.Context, config mcp.ServerConfig) (mcp.Server, error) {
	// Convert mcp.ServerConfig to core.ServerConfig
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

// serverAdapter adapts the internal core.Server to implement mcp.Server
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

func (s *serverAdapter) EnableConversationMode(config mcp.ConversationConfig) error {
	// Convert mcp.ConversationConfig to core.ConversationConfig
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

func (s *serverAdapter) GetStats() *mcp.ServerStats {
	// Convert core stats to mcp stats
	coreStats := s.coreServer.GetStats()
	if coreStats == nil {
		return &mcp.ServerStats{}
	}

	// Get the start time from the server
	startTime := s.coreServer.GetStartTime()

	return &mcp.ServerStats{
		Transport: coreStats.Transport,
		Sessions:  s.GetSessionManagerStats(),
		Workspace: s.GetWorkspaceStats(),
		Uptime:    coreStats.Uptime,
		StartTime: startTime,
	}
}

func (s *serverAdapter) GetSessionManagerStats() *mcp.SessionManagerStats {
	// Get stats from core server and convert
	// For now, return empty stats
	return &mcp.SessionManagerStats{}
}

func (s *serverAdapter) GetWorkspaceStats() *mcp.WorkspaceStats {
	// Get stats from core server and convert
	// For now, return empty stats
	return &mcp.WorkspaceStats{}
}

func (s *serverAdapter) GetLogger() interface{} {
	return s.coreServer.GetLogger()
}
