package scan

import (
	"context"

	"log/slog"

	"github.com/localrivet/gomcp/server"
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
func NewAtomicScanSecretsTool(adapter interface{}, sessionManager interface{}, logger *slog.Logger) *AtomicScanSecretsTool {
	// Forward to the actual implementation in scan_secrets_tool.go
	return newAtomicScanSecretsToolImpl(adapter, sessionManager, logger)
}

// ScanExecutionContext contains common dependencies for scan tool execution
type ScanExecutionContext struct {
	Adapter        interface{}
	SessionManager interface{}
	Logger         *slog.Logger
}

// Legacy compatibility functions - these delegate to the main tool implementation

// ExecuteScanSecrets provides backward compatibility for direct function calls
func ExecuteScanSecrets(ctx context.Context, args AtomicScanSecretsArgs, execCtx ScanExecutionContext) (*AtomicScanSecretsResult, error) {
	tool := NewAtomicScanSecretsTool(execCtx.Adapter, execCtx.SessionManager, execCtx.Logger)
	return tool.ExecuteScanSecrets(ctx, args)
}

// ExecuteWithContext provides backward compatibility for server context calls
func ExecuteWithContext(serverCtx *server.Context, args AtomicScanSecretsArgs, execCtx ScanExecutionContext) (*AtomicScanSecretsResult, error) {
	tool := NewAtomicScanSecretsTool(execCtx.Adapter, execCtx.SessionManager, execCtx.Logger)
	return tool.ExecuteWithContext(serverCtx, args)
}
