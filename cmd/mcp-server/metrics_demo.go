package main

import (
	"context"

	"github.com/Azure/container-copilot/pkg/mcp"
	"github.com/rs/zerolog/log"
)

// runMetricsDemo demonstrates the Prometheus metrics functionality
func runMetricsDemo(ctx context.Context, server *mcp.Server) error {
	log.Info().Msg("=== Metrics Demo ===")
	log.Warn().Msg("Metrics demo temporarily disabled due to API restructuring")
	log.Info().Msg("Telemetry is still available at the configured port for Prometheus scraping")
	return nil
}
