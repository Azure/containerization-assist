// Package server provides a bridge to create MCP servers without import cycles.
// This package can be imported by cmd packages to access internal functionality.
package server

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	servercore "github.com/Azure/container-kit/pkg/mcp/application/services/core"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// NewServer creates a new MCP server with the given configuration.
// This function bridges the public API to the internal implementation.
func NewServer(ctx context.Context, config core.ServerConfig) (core.Server, error) {
	// Convert core.ServerConfig to servercore.ServerConfig
	serverConfig := servercore.ServerConfig{
		WorkspaceDir:      config.WorkspaceDir,
		MaxSessions:       config.MaxSessions,
		SessionTTL:        config.SessionTTL,
		MaxDiskPerSession: config.MaxDiskPerSession,
		TotalDiskLimit:    config.TotalDiskLimit,
		StorePath:         config.StorePath,
		TransportType:     config.TransportType,
		HTTPAddr:          config.HTTPAddr,
		HTTPPort:          config.HTTPPort,
		CORSOrigins:       config.CORSOrigins,
		APIKey:            config.APIKey,
		RateLimit:         config.RateLimit,
		SandboxEnabled:    config.SandboxEnabled,
		LogLevel:          config.LogLevel,
		LogHTTPBodies:     config.LogHTTPBodies,
		MaxBodyLogSize:    config.MaxBodyLogSize,
		CleanupInterval:   config.CleanupInterval,
		MaxWorkers:        config.MaxWorkers,
		JobTTL:            config.JobTTL,
		ServiceName:       config.ServiceName,
		ServiceVersion:    config.ServiceVersion,
		Environment:       config.Environment,
	}
	return servercore.NewServer(ctx, serverConfig)
}

// NewServerWithServices creates a new MCP server using service-based architecture.
// This method accepts a DI container and integrates it with the server.
func NewServerWithServices(ctx context.Context, config core.ServerConfig, container services.ServiceContainer) (core.Server, error) {
	// Convert core.ServerConfig to servercore.ServerConfig
	serverConfig := servercore.ServerConfig{
		WorkspaceDir:      config.WorkspaceDir,
		MaxSessions:       config.MaxSessions,
		SessionTTL:        config.SessionTTL,
		MaxDiskPerSession: config.MaxDiskPerSession,
		TotalDiskLimit:    config.TotalDiskLimit,
		StorePath:         config.StorePath,
		TransportType:     config.TransportType,
		HTTPAddr:          config.HTTPAddr,
		HTTPPort:          config.HTTPPort,
		CORSOrigins:       config.CORSOrigins,
		APIKey:            config.APIKey,
		RateLimit:         config.RateLimit,
		SandboxEnabled:    config.SandboxEnabled,
		LogLevel:          config.LogLevel,
		LogHTTPBodies:     config.LogHTTPBodies,
		MaxBodyLogSize:    config.MaxBodyLogSize,
		CleanupInterval:   config.CleanupInterval,
		MaxWorkers:        config.MaxWorkers,
		JobTTL:            config.JobTTL,
		ServiceName:       config.ServiceName,
		ServiceVersion:    config.ServiceVersion,
		Environment:       config.Environment,
	}
	return servercore.NewServerWithServices(ctx, serverConfig, container)
}
