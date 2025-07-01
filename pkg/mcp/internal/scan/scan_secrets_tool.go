package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicScanSecretsTool handles secret scanning with modular components
type AtomicScanSecretsTool struct {
	pipelineAdapter interface{}
	sessionManager  interface{}
	logger          zerolog.Logger
	scanner         *FileSecretScanner
	processor       *ResultProcessor
	remediationGen  *RemediationGenerator
}

// newAtomicScanSecretsToolImpl creates a new atomic scan secrets tool (internal implementation)
func newAtomicScanSecretsToolImpl(adapter interface{}, sessionManager interface{}, logger zerolog.Logger) *AtomicScanSecretsTool {
	toolLogger := logger.With().Str("tool", "atomic_scan_secrets").Logger()

	return &AtomicScanSecretsTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
		scanner:         NewFileSecretScanner(toolLogger),
		processor:       NewResultProcessor(toolLogger),
		remediationGen:  NewRemediationGenerator(toolLogger),
	}
}

// ExecuteScanSecrets executes secret scanning without progress reporting
func (t *AtomicScanSecretsTool) ExecuteScanSecrets(ctx context.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()
	return t.executeWithoutProgress(ctx, args, startTime)
}

// ExecuteWithContext executes secret scanning with progress reporting
func (t *AtomicScanSecretsTool) ExecuteWithContext(serverCtx *server.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()

	progress := observability.NewUnifiedProgressReporter(serverCtx)

	ctx := context.Background()
	result, err := t.executeWithProgress(ctx, args, startTime, progress)

	if err != nil {
		t.logger.Info().Msg("Secrets scan failed")
		if result == nil {
			result = &AtomicScanSecretsResult{
				BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
				BaseAIContextResult: internal.NewBaseAIContextResult("scan", false, time.Since(startTime)),
				SessionID:           args.SessionID,
				Duration:            time.Since(startTime),
				SecretsFound:        0,
				DetectedSecrets:     []ScannedSecret{},
				FileResults:         []FileSecretScanResult{},
				SecurityScore:       100,
				RiskLevel:           "low",
				Recommendations:     []string{"Scan failed - please check logs and retry"},
				ScanContext:         make(map[string]interface{}),
			}
		}
	}

	return result, err
}

// executeWithProgress executes the scan with progress reporting
func (t *AtomicScanSecretsTool) executeWithProgress(ctx context.Context, args AtomicScanSecretsArgs, startTime time.Time, reporter interface{}) (*AtomicScanSecretsResult, error) {
	if progressReporter, ok := reporter.(interface {
		StartProgress([]core.ProgressStage)
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
		return nil, fmt.Errorf("secret scan failed: %w", err)
	}

	// Stage 4: Process
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(0.8, "Processing scan results")
	}

	result := &AtomicScanSecretsResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("scan", true, time.Since(startTime)),
		SessionID:           args.SessionID,
		ScanPath:            scanPath,
		FilesScanned:        filesScanned,
		Duration:            time.Since(startTime),
		SecretsFound:        len(secrets),
		DetectedSecrets:     secrets,
		SeverityBreakdown:   t.processor.CalculateSeverityBreakdown(secrets),
		FileResults:         fileResults,
		SecurityScore:       t.processor.CalculateSecurityScore(secrets),
		RiskLevel:           t.processor.DetermineRiskLevel(t.processor.CalculateSecurityScore(secrets), secrets),
		Recommendations:     t.processor.GenerateRecommendations(secrets, args),
		ScanContext:         t.processor.GenerateScanContext(secrets, fileResults, args),
	}

	// Generate remediation plan if requested
	if args.SuggestRemediation && len(secrets) > 0 {
		result.RemediationPlan = t.remediationGen.GenerateRemediationPlan(secrets)
	}

	// Generate Kubernetes secrets if requested
	if args.GenerateSecrets && len(secrets) > 0 {
		generatedSecrets, err := t.remediationGen.GenerateKubernetesSecrets(secrets, args.SessionID)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to generate Kubernetes secrets")
		} else {
			result.GeneratedSecrets = generatedSecrets
		}
	}

	// Stage 5: Finalize
	if progressReporter, ok := reporter.(interface{ ReportStage(float64, string) }); ok {
		progressReporter.ReportStage(1.0, "Finalizing scan results")
	}

	t.logger.Info().
		Int("files_scanned", filesScanned).
		Int("secrets_found", len(secrets)).
		Str("risk_level", result.RiskLevel).
		Int("security_score", result.SecurityScore).
		Msg("Secret scan completed successfully")

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
		return nil, fmt.Errorf("secret scan failed: %w", err)
	}

	result := &AtomicScanSecretsResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("scan", true, time.Since(startTime)),
		SessionID:           args.SessionID,
		ScanPath:            scanPath,
		FilesScanned:        filesScanned,
		Duration:            time.Since(startTime),
		SecretsFound:        len(secrets),
		DetectedSecrets:     secrets,
		SeverityBreakdown:   t.processor.CalculateSeverityBreakdown(secrets),
		FileResults:         fileResults,
		SecurityScore:       t.processor.CalculateSecurityScore(secrets),
		RiskLevel:           t.processor.DetermineRiskLevel(t.processor.CalculateSecurityScore(secrets), secrets),
		Recommendations:     t.processor.GenerateRecommendations(secrets, args),
		ScanContext:         t.processor.GenerateScanContext(secrets, fileResults, args),
	}

	// Generate remediation plan if requested
	if args.SuggestRemediation && len(secrets) > 0 {
		result.RemediationPlan = t.remediationGen.GenerateRemediationPlan(secrets)
	}

	// Generate Kubernetes secrets if requested
	if args.GenerateSecrets && len(secrets) > 0 {
		generatedSecrets, err := t.remediationGen.GenerateKubernetesSecrets(secrets, args.SessionID)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to generate Kubernetes secrets")
		} else {
			result.GeneratedSecrets = generatedSecrets
		}
	}

	t.logger.Info().
		Int("files_scanned", filesScanned).
		Int("secrets_found", len(secrets)).
		Str("risk_level", result.RiskLevel).
		Int("security_score", result.SecurityScore).
		Msg("Secret scan completed successfully")

	return result, nil
}

// Tool interface methods

// GetName returns the tool name
func (t *AtomicScanSecretsTool) GetName() string {
	return "atomic_scan_secrets"
}

// GetDescription returns the tool description
func (t *AtomicScanSecretsTool) GetDescription() string {
	return "Scans files for hardcoded secrets, credentials, and sensitive data with automatic remediation suggestions"
}

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
func (t *AtomicScanSecretsTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:        "atomic_scan_secrets",
		Description: "Scans files for hardcoded secrets, credentials, and sensitive data with automatic remediation suggestions and Kubernetes Secret generation",
		Version:     "1.0.0",
		Category:    "security",
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
		Parameters: map[string]string{
			"session_id":          "string - Session ID for session context",
			"scan_path":           "string - Path to scan (default: session workspace)",
			"file_patterns":       "[]string - File patterns to include (e.g., '*.py', '*.js')",
			"exclude_patterns":    "[]string - File patterns to exclude from scan",
			"scan_dockerfiles":    "boolean - Include Dockerfiles in scan",
			"scan_manifests":      "boolean - Include Kubernetes manifests in scan",
			"scan_source_code":    "boolean - Include source code files in scan",
			"scan_env_files":      "boolean - Include .env files in scan",
			"suggest_remediation": "boolean - Provide remediation suggestions",
			"generate_secrets":    "boolean - Generate Kubernetes Secret manifests",
		},
		Examples: []core.ToolExample{
			{
				Name:        "basic_scan",
				Description: "Basic secret scan of current workspace",
				Input: map[string]interface{}{
					"session_id": "session-123",
				},
				Output: map[string]interface{}{
					"secrets_found": 3,
					"risk_level":    "medium",
				},
			},
			{
				Name:        "comprehensive_scan",
				Description: "Comprehensive scan with remediation",
				Input: map[string]interface{}{
					"session_id":          "session-123",
					"scan_dockerfiles":    true,
					"scan_manifests":      true,
					"scan_source_code":    true,
					"suggest_remediation": true,
					"generate_secrets":    true,
				},
				Output: map[string]interface{}{
					"secrets_found":     5,
					"remediation_plan":  "generated",
					"generated_secrets": 2,
				},
			},
		},
	}
}

// Validate validates the tool arguments
func (t *AtomicScanSecretsTool) Validate(ctx context.Context, args interface{}) error {
	typedArgs, ok := args.(AtomicScanSecretsArgs)
	if !ok {
		return fmt.Errorf("invalid argument type: expected AtomicScanSecretsArgs")
	}

	if typedArgs.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}

	return nil
}

// Execute executes the tool with generic arguments
func (t *AtomicScanSecretsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	typedArgs, ok := args.(AtomicScanSecretsArgs)
	if !ok {
		return nil, fmt.Errorf("invalid argument type: expected AtomicScanSecretsArgs")
	}

	return t.ExecuteTyped(ctx, typedArgs)
}

// ExecuteTyped executes the tool with typed arguments
func (t *AtomicScanSecretsTool) ExecuteTyped(ctx context.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	return t.ExecuteScanSecrets(ctx, args)
}
