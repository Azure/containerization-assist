// Package server provides a bridge to create MCP servers without import cycles.
// This package can be imported by cmd packages to access internal functionality.
package server

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"
	internalserver "github.com/Azure/container-kit/pkg/mcp/internal/server"
)

// NewServer creates a new MCP server with the given configuration.
// This function bridges the public API to the internal implementation.
func NewServer(ctx context.Context, config core.ServerConfig) (core.Server, error) {
	// Convert public config to internal config
	internalConfig := internalserver.ServerConfig{
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
		EnableOTEL:        config.EnableOTEL,
		OTELEndpoint:      config.OTELEndpoint,
		OTELHeaders:       config.OTELHeaders,
		ServiceName:       config.ServiceName,
		ServiceVersion:    config.ServiceVersion,
		Environment:       config.Environment,
	}

	// Create the internal server and return it as the public interface
	return internalserver.NewServer(ctx, internalConfig)
}
