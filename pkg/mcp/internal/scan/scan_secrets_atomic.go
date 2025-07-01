package scan

import (
	"context"

	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// This file now serves as a compatibility layer and main entry point.
// The actual implementation has been split into focused modules:
// - secrets_types.go: Type definitions and data structures
// - secret_scanner.go: File scanning and secret detection logic
// - result_processor.go: Result analysis and scoring
// - remediation_generator.go: Remediation plans and Kubernetes manifest generation
// - scan_secrets_tool.go: Main tool orchestration

// NewAtomicScanSecretsTool creates a new atomic scan secrets tool
// This is the main entry point that external packages should use
func NewAtomicScanSecretsTool(adapter interface{}, sessionManager interface{}, logger zerolog.Logger) *AtomicScanSecretsTool {
	// Forward to the actual implementation in scan_secrets_tool.go
	return newAtomicScanSecretsToolImpl(adapter, sessionManager, logger)
}

// Legacy compatibility functions - these delegate to the main tool implementation

// ExecuteScanSecrets provides backward compatibility for direct function calls
func ExecuteScanSecrets(ctx context.Context, args AtomicScanSecretsArgs, adapter interface{}, sessionManager interface{}, logger zerolog.Logger) (*AtomicScanSecretsResult, error) {
	tool := NewAtomicScanSecretsTool(adapter, sessionManager, logger)
	return tool.ExecuteScanSecrets(ctx, args)
}

// ExecuteWithContext provides backward compatibility for server context calls
func ExecuteWithContext(serverCtx *server.Context, args AtomicScanSecretsArgs, adapter interface{}, sessionManager interface{}, logger zerolog.Logger) (*AtomicScanSecretsResult, error) {
	tool := NewAtomicScanSecretsTool(adapter, sessionManager, logger)
	return tool.ExecuteWithContext(serverCtx, args)
}
