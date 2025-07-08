package scan

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

// NOTE: The following interfaces have been consolidated into ScanService
// in unified_interface.go for better maintainability:
// - ScanEngine
// - SecurityAnalyzer
// - ComplianceReporter
// - RemediationPlanner
// - MetricsCollector
// - ScanEngineExtended
//
// Use ScanService instead of these interfaces for new implementations.

// ============================================================================
// Composite Interface for Main Tool
// ============================================================================

// ScanImageSecurityTool is the main interface that orchestrates all security scanning operations
// This interface remains as it provides the main tool functionality
type ScanImageSecurityTool interface {
	// Main scanning operations
	ScanImageSecurity(ctx context.Context, imageRef string) (*core.ScanResult, error)

	// Configuration and lifecycle
	Configure(config map[string]interface{}) error
	Close() error
}

// ============================================================================
// Legacy type aliases for backward compatibility
// ============================================================================

// Legacy types that point to the unified interfaces
type LegacyScanEngine = ScanService
type LegacySecurityAnalyzer = ScanService
type LegacyComplianceReporter = ScanService
type LegacyRemediationPlanner = ScanService
type LegacyMetricsCollector = ScanService
