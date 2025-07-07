package scan

import (
	"context"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// AtomicScanSecretsTool handles secret scanning with modular components
type AtomicScanSecretsTool struct {
	pipelineAdapter interface{}
	sessionManager  interface{}           // Legacy field for backward compatibility
	sessionStore    services.SessionStore // Modern service interface
	sessionState    services.SessionState // Modern service interface
	logger          *slog.Logger
	scanner         *FileSecretScanner
	processor       *ResultProcessor
	remediationGen  *RemediationGenerator
}

// NewScanSecretsTool creates a new scan secrets tool that implements api.Tool interface (legacy constructor)
func NewScanSecretsTool(adapter interface{}, sessionManager session.UnifiedSessionManager, logger *slog.Logger) api.Tool {
	return newAtomicScanSecretsToolImpl(adapter, sessionManager, logger)
}

// NewScanSecretsToolWithServices creates a new scan secrets tool with services
func NewScanSecretsToolWithServices(
	adapter interface{},
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) api.Tool {
	toolLogger := logger.With("tool", "atomic_scan_secrets")

	return &AtomicScanSecretsTool{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
		scanner:         NewFileSecretScanner(toolLogger),
		processor:       NewResultProcessor(toolLogger),
		remediationGen:  NewRemediationGenerator(toolLogger),
	}
}

// newAtomicScanSecretsToolImpl creates a new atomic scan secrets tool (internal implementation)
func newAtomicScanSecretsToolImpl(adapter interface{}, sessionManager interface{}, logger *slog.Logger) *AtomicScanSecretsTool {
	toolLogger := logger.With("tool", "atomic_scan_secrets")

	return &AtomicScanSecretsTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
		scanner:         NewFileSecretScanner(toolLogger),
		processor:       NewResultProcessor(toolLogger),
		remediationGen:  NewRemediationGenerator(toolLogger),
	}
}

// GetName returns the tool name
func (t *AtomicScanSecretsTool) GetName() string {
	return "atomic_scan_secrets"
}

// ExecuteScanSecrets executes secret scanning without progress reporting
func (t *AtomicScanSecretsTool) ExecuteScanSecrets(ctx context.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()
	return t.executeWithoutProgress(ctx, args, startTime)
}

// ExecuteWithContext executes secret scanning with server context and progress reporting
func (t *AtomicScanSecretsTool) ExecuteWithContext(serverCtx interface{}, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()
	ctx := context.Background()
	return t.executeWithProgress(ctx, args, startTime, serverCtx)
}

// executeWithProgress executes the scan with progress reporting
func (t *AtomicScanSecretsTool) executeWithProgress(ctx context.Context, args AtomicScanSecretsArgs, startTime time.Time, reporter interface{}) (*AtomicScanSecretsResult, error) {
	if progressReporter, ok := reporter.(interface {
		StartProgress([]types.ProgressStage)
		ReportStage(float64, string)
		CompleteProgress(string)
	}); ok {
		progressReporter.StartProgress(standardSecretScanStages())
		defer progressReporter.CompleteProgress("Secret scan completed")
	}

	// Stage 1: Initialize
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(0.1, "Initializing secret scan")
	}

	scanPath := args.ScanPath
	if scanPath == "" {
		scanPath = "/tmp" // Default path - would normally get from session
	}

	// Stage 2: Analyze
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(0.25, "Analyzing scan configuration")
	}

	filePatterns := args.FilePatterns
	if len(filePatterns) == 0 {
		filePatterns = t.scanner.GetDefaultFilePatterns(args)
	}

	// Stage 3: Scan
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(0.3, "Scanning files for secrets")
	}

	secrets, fileResults, filesScanned, err := t.scanner.PerformSecretScan(scanPath, filePatterns, args.ExcludePatterns, reporter)
	if err != nil {
		return nil, errors.NewError().Message("secret scan failed").Cause(err).WithLocation().Build()
	}

	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(0.8, "Processing scan results")
	}

	result := &AtomicScanSecretsResult{
		BaseToolResponse: types.BaseToolResponse{Success: false, Timestamp: time.Now()},
		BaseAIContextResult: types.BaseAIContextResult{
			IsSuccessful:  true,
			Duration:      time.Since(startTime),
			OperationType: "scan",
		},
		SessionID:         args.SessionID,
		ScanPath:          scanPath,
		FilesScanned:      filesScanned,
		Duration:          time.Since(startTime),
		SecretsFound:      len(secrets),
		DetectedSecrets:   secrets,
		SeverityBreakdown: t.processor.CalculateSeverityBreakdown(secrets),
		FileResults:       fileResults,
		SecurityScore:     t.processor.CalculateSecurityScore(secrets),
		RiskLevel:         t.processor.DetermineRiskLevel(t.processor.CalculateSecurityScore(secrets), secrets),
		Recommendations:   t.processor.GenerateRecommendations(secrets, args),
		ScanContext:       t.processor.GenerateScanContext(secrets, fileResults, args),
	}

	// Generate remediation plan if requested
	if args.SuggestRemediation && len(secrets) > 0 {
		result.RemediationPlan = t.remediationGen.GenerateRemediationPlan(secrets)
	}

	// Generate Kubernetes secrets if requested
	if args.GenerateSecrets && len(secrets) > 0 {
		generatedSecrets, err := t.remediationGen.GenerateKubernetesSecrets(secrets, args.SessionID)
		if err != nil {
			t.logger.Warn("Failed to generate Kubernetes secrets", "error", err)
		} else {
			result.GeneratedSecrets = generatedSecrets
		}
	}

	// Stage 5: Finalize
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(1.0, "Finalizing scan results")
	}

	t.logger.Info("Secret scan completed successfully",
		"files_scanned", filesScanned,
		"secrets_found", len(secrets),
		"risk_level", result.RiskLevel,
		"security_score", result.SecurityScore)

	return result, nil
}

// executeWithoutProgress executes the scan without progress reporting
func (t *AtomicScanSecretsTool) executeWithoutProgress(ctx context.Context, args AtomicScanSecretsArgs, startTime time.Time) (*AtomicScanSecretsResult, error) {
	scanPath := args.ScanPath
	if scanPath == "" {
		scanPath = "/tmp" // Default path - would normally get from session
	}

	filePatterns := args.FilePatterns
	if len(filePatterns) == 0 {
		filePatterns = t.scanner.GetDefaultFilePatterns(args)
	}

	secrets, fileResults, filesScanned, err := t.scanner.PerformSecretScan(scanPath, filePatterns, args.ExcludePatterns, nil)
	if err != nil {
		return nil, errors.NewError().Message("secret scan failed").Cause(err).WithLocation().Build()
	}

	result := &AtomicScanSecretsResult{
		BaseToolResponse: types.BaseToolResponse{Success: false, Timestamp: time.Now()},
		BaseAIContextResult: types.BaseAIContextResult{
			IsSuccessful:  true,
			Duration:      time.Since(startTime),
			OperationType: "scan",
		},
		SessionID:         args.SessionID,
		ScanPath:          scanPath,
		FilesScanned:      filesScanned,
		Duration:          time.Since(startTime),
		SecretsFound:      len(secrets),
		DetectedSecrets:   secrets,
		SeverityBreakdown: t.processor.CalculateSeverityBreakdown(secrets),
		FileResults:       fileResults,
		SecurityScore:     t.processor.CalculateSecurityScore(secrets),
		RiskLevel:         t.processor.DetermineRiskLevel(t.processor.CalculateSecurityScore(secrets), secrets),
		Recommendations:   t.processor.GenerateRecommendations(secrets, args),
		ScanContext:       t.processor.GenerateScanContext(secrets, fileResults, args),
	}

	// Generate remediation plan if requested
	if args.SuggestRemediation && len(secrets) > 0 {
		result.RemediationPlan = t.remediationGen.GenerateRemediationPlan(secrets)
	}

	// Generate Kubernetes secrets if requested
	if args.GenerateSecrets && len(secrets) > 0 {
		generatedSecrets, err := t.remediationGen.GenerateKubernetesSecrets(secrets, args.SessionID)
		if err != nil {
			t.logger.Warn("Failed to generate Kubernetes secrets", "error", err)
		} else {
			result.GeneratedSecrets = generatedSecrets
		}
	}

	t.logger.Info("Secret scan completed successfully",
		"files_scanned", filesScanned,
		"secrets_found", len(secrets),
		"risk_level", result.RiskLevel,
		"security_score", result.SecurityScore)

	return result, nil
}

// Tool interface methods

// GetVersion returns the tool version
func (t *AtomicScanSecretsTool) GetVersion() string {
	return "1.0.0"
}

// GetCapabilities returns the tool capabilities
func (t *AtomicScanSecretsTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// GetMetadata returns comprehensive tool metadata
func (t *AtomicScanSecretsTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:        "atomic_scan_secrets",
		Description: "Scans files for hardcoded secrets, credentials, and sensitive data with automatic remediation suggestions and Kubernetes Secret generation",
		Version:     "1.0.0",
		Category:    "security",
		Status:      "active",
		Tags:        []string{"security", "secrets", "scan", "compliance"},
		Dependencies: []string{
			"session_manager",
			"file_system_access",
		},
		Capabilities: []string{
			"secret_detection",
			"pattern_matching",
			"file_scanning",
			"security_analysis",
			"remediation_planning",
			"kubernetes_secret_generation",
			"risk_assessment",
			"compliance_checking",
		},
		Requirements: []string{
			"valid_session_id",
			"file_system_access",
		},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// Validate validates the scan arguments using tag-based validation
func (t *AtomicScanSecretsTool) Validate(_ context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

// Execute implements api.Tool interface
func (t *AtomicScanSecretsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract params from ToolInput
	var args AtomicScanSecretsArgs
	if rawParams, ok := input.Data["params"]; ok {
		if typedParams, ok := rawParams.(AtomicScanSecretsArgs); ok {
			args = typedParams
		} else {
			return api.ToolOutput{
					Success: false,
					Error:   "Invalid input type for scan secrets tool",
				}, errors.NewError().
					Code(errors.CodeInvalidParameter).
					Message("Invalid input type for scan secrets tool").
					Type(errors.ErrTypeValidation).
					Severity(errors.SeverityHigh).
					Context("tool", "scan_secrets").
					Context("operation", "type_assertion").
					Build()
		}
	} else {
		return api.ToolOutput{
				Success: false,
				Error:   "No params provided",
			}, errors.NewError().
				Code(errors.CodeInvalidParameter).
				Message("No params provided").
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityHigh).
				Build()
	}

	// Use session ID from input if available
	if input.SessionID != "" {
		args.SessionID = input.SessionID
	}

	// Execute the scan
	result, err := t.ExecuteScanSecrets(ctx, args)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Data:    map[string]interface{}{"result": result},
			Error:   err.Error(),
		}, err
	}

	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": result},
	}, nil
}

// Name implements api.Tool interface
func (t *AtomicScanSecretsTool) Name() string {
	return "scan_secrets"
}

// Description implements api.Tool interface
func (t *AtomicScanSecretsTool) Description() string {
	return "Scans files and directories for secrets and sensitive information"
}

// Schema implements api.Tool interface
func (t *AtomicScanSecretsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "scan_secrets",
		Description: "Scans files and directories for secrets and sensitive information",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"params": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"session_id": map[string]interface{}{
							"type":        "string",
							"description": "Session identifier",
						},
						"target_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to scan for secrets",
						},
						"scan_depth": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum directory depth to scan",
						},
						"exclude_patterns": map[string]interface{}{
							"type":        "array",
							"description": "Patterns to exclude from scanning",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"required": []string{"session_id", "target_path"},
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the scan was successful",
						},
						"secrets_found": map[string]interface{}{
							"type":        "integer",
							"description": "Number of secrets found",
						},
						"files_scanned": map[string]interface{}{
							"type":        "integer",
							"description": "Number of files scanned",
						},
					},
				},
			},
		},
	}
}
