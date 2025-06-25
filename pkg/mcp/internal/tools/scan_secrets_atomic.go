package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/interfaces"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicScanSecretsArgs defines arguments for atomic secret scanning
type AtomicScanSecretsArgs struct {
	types.BaseToolArgs

	// Scan targets
	ScanPath        string   `json:"scan_path,omitempty" description:"Path to scan (default: session workspace)"`
	FilePatterns    []string `json:"file_patterns,omitempty" description:"File patterns to include in scan (e.g., '*.py', '*.js')"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty" description:"File patterns to exclude from scan"`

	// Scan options
	ScanDockerfiles bool `json:"scan_dockerfiles,omitempty" description:"Include Dockerfiles in scan"`
	ScanManifests   bool `json:"scan_manifests,omitempty" description:"Include Kubernetes manifests in scan"`
	ScanSourceCode  bool `json:"scan_source_code,omitempty" description:"Include source code files in scan"`
	ScanEnvFiles    bool `json:"scan_env_files,omitempty" description:"Include .env files in scan"`

	// Analysis options
	SuggestRemediation bool `json:"suggest_remediation,omitempty" description:"Provide remediation suggestions"`
	GenerateSecrets    bool `json:"generate_secrets,omitempty" description:"Generate Kubernetes Secret manifests"`
}

// AtomicScanSecretsResult represents the result of atomic secret scanning
type AtomicScanSecretsResult struct {
	types.BaseToolResponse
	BaseAIContextResult // Embed AI context methods

	// Scan metadata
	SessionID    string        `json:"session_id"`
	ScanPath     string        `json:"scan_path"`
	FilesScanned int           `json:"files_scanned"`
	Duration     time.Duration `json:"duration"`

	// Detection results
	SecretsFound      int             `json:"secrets_found"`
	DetectedSecrets   []ScannedSecret `json:"detected_secrets"`
	SeverityBreakdown map[string]int  `json:"severity_breakdown"`

	// File-specific results
	FileResults []FileSecretScanResult `json:"file_results"`

	// Remediation
	RemediationPlan  *SecretRemediationPlan    `json:"remediation_plan,omitempty"`
	GeneratedSecrets []GeneratedSecretManifest `json:"generated_secrets,omitempty"`

	// Security insights
	SecurityScore   int      `json:"security_score"` // 0-100
	RiskLevel       string   `json:"risk_level"`     // low, medium, high, critical
	Recommendations []string `json:"recommendations"`

	// Context and debugging
	ScanContext map[string]interface{} `json:"scan_context"`
}

// ScannedSecret represents a found secret with context
type ScannedSecret struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Type       string `json:"type"`       // password, api_key, token, etc.
	Pattern    string `json:"pattern"`    // what pattern matched
	Value      string `json:"value"`      // redacted value
	Severity   string `json:"severity"`   // low, medium, high, critical
	Context    string `json:"context"`    // surrounding context
	Confidence int    `json:"confidence"` // 0-100
}

// FileSecretScanResult represents scan results for a single file
type FileSecretScanResult struct {
	FilePath     string          `json:"file_path"`
	FileType     string          `json:"file_type"`
	SecretsFound int             `json:"secrets_found"`
	Secrets      []ScannedSecret `json:"secrets"`
	CleanStatus  string          `json:"clean_status"` // clean, issues, critical
}

// SecretRemediationPlan provides recommendations for fixing detected secrets
type SecretRemediationPlan struct {
	ImmediateActions []string          `json:"immediate_actions"`
	SecretReferences []SecretReference `json:"secret_references"`
	ConfigMapEntries map[string]string `json:"config_map_entries"`
	PreferredManager string            `json:"preferred_manager"`
	MigrationSteps   []string          `json:"migration_steps"`
}

// SecretReference represents how a secret should be referenced
type SecretReference struct {
	SecretName     string `json:"secret_name"`
	SecretKey      string `json:"secret_key"`
	OriginalEnvVar string `json:"original_env_var"`
	KubernetesRef  string `json:"kubernetes_ref"`
}

// GeneratedSecretManifest represents a generated Kubernetes Secret
type GeneratedSecretManifest struct {
	Name     string   `json:"name"`
	Content  string   `json:"content"`
	FilePath string   `json:"file_path"`
	Keys     []string `json:"keys"`
}

// standardSecretScanStages provides common stages for secret scanning operations
func standardSecretScanStages() []interfaces.ProgressStage {
	return []interfaces.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating scan path"},
		{Name: "Analyze", Weight: 0.15, Description: "Analyzing file patterns and scan configuration"},
		{Name: "Scan", Weight: 0.50, Description: "Scanning files for secrets"},
		{Name: "Process", Weight: 0.20, Description: "Processing results and generating recommendations"},
		{Name: "Finalize", Weight: 0.05, Description: "Generating reports and remediation plans"},
	}
}

// AtomicScanSecretsTool implements atomic secret scanning
type AtomicScanSecretsTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	logger          zerolog.Logger
}

// NewAtomicScanSecretsTool creates a new atomic secret scanning tool
func NewAtomicScanSecretsTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicScanSecretsTool {
	return &AtomicScanSecretsTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "atomic_scan_secrets").Logger(),
	}
}

// ExecuteScanSecrets runs the atomic secret scanning
func (t *AtomicScanSecretsTool) ExecuteScanSecrets(ctx context.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()

	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args, startTime)
}

// ExecuteWithContext runs the atomic secrets scan with GoMCP progress tracking
func (t *AtomicScanSecretsTool) ExecuteWithContext(serverCtx *server.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()

	// Create progress adapter for GoMCP using standard scan stages
	adapter := NewGoMCPProgressAdapter(serverCtx, interfaces.StandardScanStages())

	// Execute with progress tracking
	ctx := context.Background()
	result, err := t.executeWithProgress(ctx, args, startTime, adapter)

	// Complete progress tracking
	if err != nil {
		adapter.Complete("Secrets scan failed")
		if result == nil {
			// Create a minimal result if something went wrong
			result = &AtomicScanSecretsResult{
				BaseToolResponse: types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
				SessionID:        args.SessionID,
				Duration:         time.Since(startTime),
				RiskLevel:        "unknown",
			}
		}
		return result, nil // Return result with error info, not the error itself
	} else {
		adapter.Complete("Secrets scan completed successfully")
	}

	return result, nil
}

// executeWithProgress handles the main execution with progress reporting
func (t *AtomicScanSecretsTool) executeWithProgress(ctx context.Context, args AtomicScanSecretsArgs, startTime time.Time, reporter interfaces.ProgressReporter) (*AtomicScanSecretsResult, error) {
	// Stage 1: Initialize - Loading session and validating scan path
	reporter.ReportStage(0.1, "Loading session")

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicScanSecretsResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
			BaseAIContextResult: NewBaseAIContextResult("scan", false, time.Since(startTime)),
			SessionID:           args.SessionID,
			Duration:            time.Since(startTime),
			RiskLevel:           "unknown",
		}
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, types.NewRichError("SESSION_ACCESS_FAILED", fmt.Sprintf("failed to get session: %v", err), types.ErrTypeSession)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("scan_path", args.ScanPath).
		Msg("Starting atomic secret scanning")

	// Create base result
	result := &AtomicScanSecretsResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", session.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("scan", false, 0), // Duration and success will be updated later
		SessionID:           session.SessionID,
		ScanContext:         make(map[string]interface{}),
		SeverityBreakdown:   make(map[string]int),
	}

	reporter.ReportStage(0.5, "Session loaded")

	// Determine scan path
	scanPath := args.ScanPath
	if scanPath == "" {
		scanPath = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	}
	result.ScanPath = scanPath

	// Validate scan path exists
	if _, err := os.Stat(scanPath); os.IsNotExist(err) {
		t.logger.Error().Str("scan_path", scanPath).Msg("Scan path does not exist")
		result.Duration = time.Since(startTime)
		return result, types.NewRichError("SCAN_PATH_NOT_FOUND", fmt.Sprintf("scan path does not exist: %s", scanPath), types.ErrTypeSystem)
	}

	reporter.ReportStage(1.0, "Initialization complete")

	// Stage 2: Analyze - Analyzing file patterns and scan configuration
	reporter.NextStage("Analyzing scan configuration")

	// Use provided file patterns or defaults
	filePatterns := args.FilePatterns
	if len(filePatterns) == 0 {
		filePatterns = t.getDefaultFilePatterns(args)
	}

	excludePatterns := args.ExcludePatterns
	if len(excludePatterns) == 0 {
		// Use default exclusions
		excludePatterns = []string{"*.git/*", "node_modules/*", "vendor/*", "*.log"}
	}

	reporter.ReportStage(1.0, "Scan configuration analyzed")

	// Stage 3: Scan - Scanning files for secrets
	reporter.NextStage("Scanning files for secrets")

	// Perform the actual secret scan
	allSecrets, fileResults, filesScanned, err := t.performSecretScan(scanPath, filePatterns, excludePatterns, reporter)
	if err != nil {
		t.logger.Error().Err(err).Str("scan_path", scanPath).Msg("Failed to scan directory")
		result.Duration = time.Since(startTime)
		return result, types.NewRichError("SCAN_DIRECTORY_FAILED", fmt.Sprintf("failed to scan directory: %v", err), types.ErrTypeSystem)
	}

	// Update result with scan data
	result.FilesScanned = filesScanned
	result.SecretsFound = len(allSecrets)
	result.DetectedSecrets = allSecrets
	result.FileResults = fileResults

	reporter.ReportStage(1.0, fmt.Sprintf("Scanned %d files, found %d secrets", filesScanned, len(allSecrets)))

	// Stage 4: Process - Processing results and generating recommendations
	reporter.NextStage("Processing scan results")

	result.SeverityBreakdown = t.calculateSeverityBreakdown(allSecrets)
	result.SecurityScore = t.calculateSecurityScore(allSecrets)
	result.RiskLevel = t.determineRiskLevel(result.SecurityScore, allSecrets)
	result.Recommendations = t.generateRecommendations(allSecrets, args)

	reporter.ReportStage(0.6, "Generated security analysis")

	// Generate remediation plan if requested
	if args.SuggestRemediation && len(allSecrets) > 0 {
		result.RemediationPlan = t.generateRemediationPlan(allSecrets)
		reporter.ReportStage(0.8, "Generated remediation plan")
	}

	reporter.ReportStage(1.0, "Result processing complete")

	// Stage 5: Finalize - Generating reports and remediation plans
	reporter.NextStage("Finalizing results")

	// Generate Kubernetes secrets if requested
	if args.GenerateSecrets && len(allSecrets) > 0 {
		generatedSecrets, err := t.generateKubernetesSecrets(allSecrets, session.SessionID)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to generate Kubernetes secrets")
		} else {
			result.GeneratedSecrets = generatedSecrets
			reporter.ReportStage(0.8, "Generated Kubernetes secrets")
		}
	}

	result.Duration = time.Since(startTime)

	// Log results
	t.logger.Info().
		Str("session_id", session.SessionID).
		Int("files_scanned", result.FilesScanned).
		Int("secrets_found", result.SecretsFound).
		Str("risk_level", result.RiskLevel).
		Int("security_score", result.SecurityScore).
		Dur("duration", result.Duration).
		Msg("Secret scanning completed")

	reporter.ReportStage(1.0, "Secret scanning completed")

	return result, nil
}

// executeWithoutProgress handles the main execution without progress reporting
func (t *AtomicScanSecretsTool) executeWithoutProgress(ctx context.Context, args AtomicScanSecretsArgs, startTime time.Time) (*AtomicScanSecretsResult, error) {
	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicScanSecretsResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
			BaseAIContextResult: NewBaseAIContextResult("scan", false, time.Since(startTime)),
			SessionID:           args.SessionID,
			Duration:            time.Since(startTime),
			RiskLevel:           "unknown",
		}
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, types.NewRichError("SESSION_ACCESS_FAILED", fmt.Sprintf("failed to get session: %v", err), types.ErrTypeSession)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("scan_path", args.ScanPath).
		Msg("Starting atomic secret scanning")

	// Create base result
	result := &AtomicScanSecretsResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", session.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("scan", false, 0), // Duration and success will be updated later
		SessionID:           session.SessionID,
		ScanContext:         make(map[string]interface{}),
		SeverityBreakdown:   make(map[string]int),
	}

	// Determine scan path
	scanPath := args.ScanPath
	if scanPath == "" {
		scanPath = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	}
	result.ScanPath = scanPath

	// Validate scan path exists
	if _, err := os.Stat(scanPath); os.IsNotExist(err) {
		t.logger.Error().Str("scan_path", scanPath).Msg("Scan path does not exist")
		result.Duration = time.Since(startTime)
		return result, types.NewRichError("SCAN_PATH_NOT_FOUND", fmt.Sprintf("scan path does not exist: %s", scanPath), types.ErrTypeSystem)
	}

	// Use provided file patterns or defaults
	filePatterns := args.FilePatterns
	if len(filePatterns) == 0 {
		filePatterns = t.getDefaultFilePatterns(args)
	}

	excludePatterns := args.ExcludePatterns
	if len(excludePatterns) == 0 {
		// Use default exclusions
		excludePatterns = []string{"*.git/*", "node_modules/*", "vendor/*", "*.log"}
	}

	// Perform the actual secret scan
	allSecrets, fileResults, filesScanned, err := t.performSecretScan(scanPath, filePatterns, excludePatterns, nil)
	if err != nil {
		t.logger.Error().Err(err).Str("scan_path", scanPath).Msg("Failed to scan directory")
		result.Duration = time.Since(startTime)
		return result, types.NewRichError("SCAN_DIRECTORY_FAILED", fmt.Sprintf("failed to scan directory: %v", err), types.ErrTypeSystem)
	}

	// Process results
	result.FilesScanned = filesScanned
	result.SecretsFound = len(allSecrets)
	result.DetectedSecrets = allSecrets
	result.FileResults = fileResults
	result.SeverityBreakdown = t.calculateSeverityBreakdown(allSecrets)

	// Calculate security score and risk level
	result.SecurityScore = t.calculateSecurityScore(allSecrets)
	result.RiskLevel = t.determineRiskLevel(result.SecurityScore, allSecrets)

	// Generate recommendations
	result.Recommendations = t.generateRecommendations(allSecrets, args)

	// Generate remediation plan if requested
	if args.SuggestRemediation && len(allSecrets) > 0 {
		result.RemediationPlan = t.generateRemediationPlan(allSecrets)
	}

	// Generate Kubernetes secrets if requested
	if args.GenerateSecrets && len(allSecrets) > 0 {
		generatedSecrets, err := t.generateKubernetesSecrets(allSecrets, session.SessionID)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to generate Kubernetes secrets")
		} else {
			result.GeneratedSecrets = generatedSecrets
		}
	}

	result.Duration = time.Since(startTime)

	// Update BaseAIContextResult fields
	result.BaseAIContextResult.Duration = result.Duration
	result.BaseAIContextResult.IsSuccessful = true // Scan completed successfully
	result.BaseAIContextResult.ErrorCount = result.SecretsFound
	result.BaseAIContextResult.WarningCount = len(result.Recommendations)

	// Log results
	t.logger.Info().
		Str("session_id", session.SessionID).
		Int("files_scanned", result.FilesScanned).
		Int("secrets_found", result.SecretsFound).
		Str("risk_level", result.RiskLevel).
		Int("security_score", result.SecurityScore).
		Dur("duration", result.Duration).
		Msg("Secret scanning completed")

	return result, nil
}

// performSecretScan performs the actual file scanning for secrets
func (t *AtomicScanSecretsTool) performSecretScan(scanPath string, filePatterns, excludePatterns []string, reporter interfaces.ProgressReporter) ([]ScannedSecret, []FileSecretScanResult, int, error) {
	scanner := utils.NewSecretScanner()
	var allSecrets []ScannedSecret
	var fileResults []FileSecretScanResult
	filesScanned := 0

	// Count total files first for progress reporting
	totalFiles := 0
	if reporter != nil {
		err := filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if t.shouldScanFile(path, filePatterns, excludePatterns) {
				totalFiles++
			}
			return nil
		})
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to count files for progress")
		}
	}

	err := filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file matches patterns
		if !t.shouldScanFile(path, filePatterns, excludePatterns) {
			return nil
		}

		// Scan file for secrets
		fileSecrets, err := t.scanFileForSecrets(path, scanner)
		if err != nil {
			t.logger.Warn().Err(err).Str("file", path).Msg("Failed to scan file")
			return nil // Continue with other files
		}

		filesScanned++

		// Report progress if available
		if reporter != nil && totalFiles > 0 {
			progress := float64(filesScanned) / float64(totalFiles)
			reporter.ReportStage(progress, fmt.Sprintf("Scanned %d/%d files", filesScanned, totalFiles))
		}

		// Create file result
		fileResult := FileSecretScanResult{
			FilePath:     path,
			FileType:     t.getFileType(path),
			SecretsFound: len(fileSecrets),
			Secrets:      fileSecrets,
			CleanStatus:  t.determineCleanStatus(fileSecrets),
		}

		fileResults = append(fileResults, fileResult)
		allSecrets = append(allSecrets, fileSecrets...)

		return nil
	})

	return allSecrets, fileResults, filesScanned, err
}

// Helper methods

func (t *AtomicScanSecretsTool) getDefaultFilePatterns(args AtomicScanSecretsArgs) []string {
	var patterns []string

	if args.ScanDockerfiles {
		patterns = append(patterns, "Dockerfile*", "*.dockerfile")
	}

	if args.ScanManifests {
		patterns = append(patterns, "*.yaml", "*.yml", "*.json")
	}

	if args.ScanEnvFiles {
		patterns = append(patterns, ".env*", "*.env")
	}

	if args.ScanSourceCode {
		patterns = append(patterns, "*.py", "*.js", "*.ts", "*.go", "*.java", "*.cs", "*.php", "*.rb")
	}

	// If no specific options, scan common config files
	if len(patterns) == 0 {
		patterns = []string{"*.yaml", "*.yml", "*.json", ".env*", "*.env", "Dockerfile*"}
	}

	return patterns
}

func (t *AtomicScanSecretsTool) shouldScanFile(path string, includePatterns, excludePatterns []string) bool {
	filename := filepath.Base(path)

	// Check exclude patterns first
	for _, pattern := range excludePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err != nil {
			// Skip invalid patterns
			continue
		}
		if matched {
			return false
		}
	}

	// Check include patterns
	for _, pattern := range includePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err != nil {
			// Skip invalid patterns
			continue
		}
		if matched {
			return true
		}
	}

	return false
}

func (t *AtomicScanSecretsTool) scanFileForSecrets(filePath string, scanner *utils.SecretScanner) ([]ScannedSecret, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Use the existing secret scanner
	sensitiveVars := scanner.ScanContent(string(content))

	var secrets []ScannedSecret
	for _, sensitiveVar := range sensitiveVars {
		secret := ScannedSecret{
			File:       filePath,
			Type:       t.classifySecretType(sensitiveVar.Pattern),
			Pattern:    sensitiveVar.Pattern,
			Value:      sensitiveVar.Redacted,
			Severity:   t.determineSeverity(sensitiveVar.Pattern, sensitiveVar.Value),
			Confidence: t.calculateConfidence(sensitiveVar.Pattern),
		}
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

func (t *AtomicScanSecretsTool) getFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	if strings.HasPrefix(base, "dockerfile") {
		return "dockerfile"
	}

	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return types.LanguageJSON
	case ".env":
		return "env"
	case ".py":
		return types.LanguagePython
	case ".js", ".ts":
		return types.LanguageJavaScript
	case ".go":
		return "go"
	case ".java":
		return types.LanguageJava
	default:
		return "other"
	}
}

func (t *AtomicScanSecretsTool) determineCleanStatus(secrets []ScannedSecret) string {
	if len(secrets) == 0 {
		return "clean"
	}

	for _, secret := range secrets {
		if secret.Severity == "critical" || secret.Severity == "high" {
			return "critical"
		}
	}

	return "issues"
}

func (t *AtomicScanSecretsTool) classifySecretType(pattern string) string {
	pattern = strings.ToLower(pattern)

	if strings.Contains(pattern, "password") {
		return "password"
	}
	if strings.Contains(pattern, "key") {
		return "api_key"
	}
	if strings.Contains(pattern, "token") {
		return "token"
	}
	if strings.Contains(pattern, "secret") {
		return "secret"
	}

	return "sensitive"
}

func (t *AtomicScanSecretsTool) determineSeverity(pattern, value string) string {
	pattern = strings.ToLower(pattern)

	// Critical: actual secrets that look like real values
	if len(value) > 20 && (strings.Contains(pattern, "key") || strings.Contains(pattern, "token")) {
		return "critical"
	}

	// High: passwords and secrets
	if strings.Contains(pattern, "password") || strings.Contains(pattern, "secret") {
		return "high"
	}

	// Medium: other sensitive data
	return "medium"
}

func (t *AtomicScanSecretsTool) calculateConfidence(pattern string) int {
	// Simple confidence calculation based on pattern specificity
	if strings.Contains(strings.ToLower(pattern), "password") {
		return 90
	}
	if strings.Contains(strings.ToLower(pattern), "key") {
		return 85
	}
	if strings.Contains(strings.ToLower(pattern), "token") {
		return 85
	}

	return 70
}

func (t *AtomicScanSecretsTool) calculateSeverityBreakdown(secrets []ScannedSecret) map[string]int {
	breakdown := make(map[string]int)

	for _, secret := range secrets {
		breakdown[secret.Severity]++
	}

	return breakdown
}

func (t *AtomicScanSecretsTool) calculateSecurityScore(secrets []ScannedSecret) int {
	if len(secrets) == 0 {
		return 100
	}

	score := 100
	for _, secret := range secrets {
		switch secret.Severity {
		case "critical":
			score -= 25
		case "high":
			score -= 15
		case "medium":
			score -= 8
		case "low":
			score -= 3
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

func (t *AtomicScanSecretsTool) determineRiskLevel(score int, secrets []ScannedSecret) string {
	if score >= 80 {
		return "low"
	}
	if score >= 60 {
		return "medium"
	}
	if score >= 30 {
		return "high"
	}

	return "critical"
}

func (t *AtomicScanSecretsTool) generateRecommendations(secrets []ScannedSecret, args AtomicScanSecretsArgs) []string {
	var recommendations []string

	if len(secrets) == 0 {
		recommendations = append(recommendations, "No secrets detected - good security posture!")
		return recommendations
	}

	recommendations = append(recommendations,
		"Remove hardcoded secrets from source code and configuration files",
		"Use Kubernetes Secrets for sensitive data in container environments",
		"Consider using external secret management solutions like Azure Key Vault or HashiCorp Vault",
		"Implement .gitignore rules to prevent committing sensitive files",
		"Use environment variables with external configuration for non-secret configuration",
	)

	// Add specific recommendations based on found secrets
	hasCritical := false
	hasPasswords := false

	for _, secret := range secrets {
		if secret.Severity == "critical" {
			hasCritical = true
		}
		if secret.Type == "password" {
			hasPasswords = true
		}
	}

	if hasCritical {
		recommendations = append(recommendations,
			"URGENT: Critical secrets detected - rotate these credentials immediately",
			"Review access logs for potential unauthorized access using these credentials",
		)
	}

	if hasPasswords {
		recommendations = append(recommendations,
			"Replace hardcoded passwords with secure authentication mechanisms",
			"Consider using service accounts or managed identities where possible",
		)
	}

	return recommendations
}

func (t *AtomicScanSecretsTool) generateRemediationPlan(secrets []ScannedSecret) *SecretRemediationPlan {
	plan := &SecretRemediationPlan{
		ConfigMapEntries: make(map[string]string),
		PreferredManager: "kubernetes-secrets",
	}

	plan.ImmediateActions = []string{
		"Stop committing files with detected secrets",
		"Remove secrets from version control history if already committed",
		"Rotate any exposed credentials",
		"Review and update .gitignore to prevent future commits",
	}

	plan.MigrationSteps = []string{
		"Create Kubernetes Secret manifests for sensitive data",
		"Update Deployment manifests to reference secrets via secretKeyRef",
		"Test the application with externalized secrets",
		"Remove hardcoded secrets from source files",
		"Implement proper secret rotation procedures",
	}

	// Generate secret references
	secretMap := make(map[string][]ScannedSecret)
	for _, scannedSecret := range secrets {
		key := scannedSecret.Type
		secretMap[key] = append(secretMap[key], scannedSecret)
	}

	for secretType, typeSecrets := range secretMap {
		secretName := fmt.Sprintf("app-%s-secrets", secretType)

		for i := range typeSecrets {
			keyName := fmt.Sprintf("%s-%d", secretType, i+1)

			ref := SecretReference{
				SecretName:     secretName,
				SecretKey:      keyName,
				OriginalEnvVar: fmt.Sprintf("%s_VAR", strings.ToUpper(keyName)),
				KubernetesRef:  fmt.Sprintf("secretKeyRef: {name: %s, key: %s}", secretName, keyName),
			}

			plan.SecretReferences = append(plan.SecretReferences, ref)
		}
	}

	return plan
}

func (t *AtomicScanSecretsTool) generateKubernetesSecrets(secrets []ScannedSecret, sessionID string) ([]GeneratedSecretManifest, error) {
	// Generate actual Kubernetes Secret YAML manifests with proper structure
	t.logger.Info().
		Int("secret_count", len(secrets)).
		Str("session_id", sessionID).
		Msg("Generating Kubernetes Secret manifests")

	if len(secrets) == 0 {
		t.logger.Info().Msg("No secrets found, skipping manifest generation")
		return []GeneratedSecretManifest{}, nil
	}

	var manifests []GeneratedSecretManifest

	// Group secrets by type and create meaningful secret names
	secretsByType := make(map[string][]ScannedSecret)
	for _, secret := range secrets {
		secretType := t.normalizeSecretType(secret.Type)
		secretsByType[secretType] = append(secretsByType[secretType], secret)
	}

	// Generate a manifest for each secret type
	for secretType, typeSecrets := range secretsByType {
		secretName := t.generateSecretName(secretType)

		// Create keys and data for each secret
		secretData := make(map[string]string)
		var keys []string

		for i, secret := range typeSecrets {
			key := t.generateSecretKey(secret, i)
			keys = append(keys, key)
			// Create placeholder value for the secret (base64 encoded placeholder)
			placeholderValue := t.generatePlaceholderValue(secret)
			secretData[key] = placeholderValue
		}

		manifest := GeneratedSecretManifest{
			Name:     secretName,
			Content:  t.generateSecretYAML(secretName, secretData, typeSecrets),
			FilePath: filepath.Join("k8s", fmt.Sprintf("%s.yaml", secretName)),
			Keys:     keys,
		}

		manifests = append(manifests, manifest)

		t.logger.Info().
			Str("secret_name", secretName).
			Str("secret_type", secretType).
			Int("key_count", len(keys)).
			Msg("Generated Kubernetes Secret manifest")
	}

	return manifests, nil
}

func (t *AtomicScanSecretsTool) generateSecretYAML(name string, secretData map[string]string, detectedSecrets []ScannedSecret) string {
	// Generate a complete Kubernetes Secret YAML with actual data structure
	yamlContent := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  labels:
    app: %s
    generated-by: container-copilot
    secret-type: %s
  annotations:
    description: "Generated from detected secrets in source code"
    secrets-detected: "%d"
    generation-time: "%s"
type: Opaque
data:
`, name, t.extractAppName(name), t.extractSecretType(name), len(detectedSecrets), time.Now().UTC().Format(time.RFC3339))

	// Add each secret as a data entry
	for key, value := range secretData {
		yamlContent += fmt.Sprintf("  %s: %s\n", key, value)
	}

	// Add comments section with guidance
	yamlContent += `
# Instructions:
# 1. Replace the placeholder values above with your actual base64-encoded secrets
# 2. Use 'echo -n "your-secret-value" | base64' to encode values
# 3. Apply this secret to your cluster: kubectl apply -f this-file.yaml
# 4. Reference in your deployment using:
#    env:
#    - name: SECRET_NAME
#      valueFrom:
#        secretKeyRef:
#          name: ` + name + `
#          key: <key-name>
#
# Detected secrets that should be stored here:
`

	// Add details about each detected secret as comments
	for i, secret := range detectedSecrets {
		yamlContent += fmt.Sprintf("# %d. Found in %s:%d - Type: %s (Severity: %s)\n",
			i+1, secret.File, secret.Line, secret.Type, secret.Severity)
	}

	return yamlContent
}

// Helper methods for Kubernetes secret generation

// normalizeSecretType converts detected secret types to Kubernetes-friendly names
func (t *AtomicScanSecretsTool) normalizeSecretType(secretType string) string {
	switch strings.ToLower(secretType) {
	case "api_key", "apikey", "api-key":
		return "api-keys"
	case "password", "pwd":
		return "passwords"
	case "token", "access_token", "auth_token":
		return "tokens"
	case "private_key", "privatekey", "private-key", "ssh_key":
		return "private-keys"
	case "database_url", "db_url", "connection_string":
		return "database"
	case "webhook_url", "webhook":
		return "webhooks"
	default:
		// Clean up the type name for Kubernetes compatibility
		normalized := strings.ToLower(secretType)
		normalized = strings.ReplaceAll(normalized, "_", "-")
		normalized = strings.ReplaceAll(normalized, " ", "-")
		return normalized
	}
}

// generateSecretName creates a Kubernetes-compatible secret name
func (t *AtomicScanSecretsTool) generateSecretName(secretType string) string {
	// Ensure the name follows Kubernetes naming conventions
	name := fmt.Sprintf("app-%s", secretType)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	// Ensure it ends with a valid character
	name = strings.TrimSuffix(name, "-")
	return name
}

// generateSecretKey creates a meaningful key name for a detected secret
func (t *AtomicScanSecretsTool) generateSecretKey(secret ScannedSecret, index int) string {
	var keyName string

	// Try to extract a meaningful name from the file or context
	fileName := filepath.Base(secret.File)
	fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Create a descriptive key based on secret type and location
	switch strings.ToLower(secret.Type) {
	case "api_key", "apikey", "api-key":
		keyName = fmt.Sprintf("%s-api-key", fileName)
	case "password", "pwd":
		keyName = fmt.Sprintf("%s-password", fileName)
	case "token", "access_token", "auth_token":
		keyName = fmt.Sprintf("%s-token", fileName)
	case "private_key", "privatekey", "private-key":
		keyName = fmt.Sprintf("%s-private-key", fileName)
	case "database_url", "db_url":
		keyName = fmt.Sprintf("%s-db-url", fileName)
	default:
		keyName = fmt.Sprintf("%s-%s", fileName, strings.ToLower(secret.Type))
	}

	// Ensure Kubernetes compatibility
	keyName = strings.ToLower(keyName)
	keyName = strings.ReplaceAll(keyName, "_", "-")
	keyName = strings.ReplaceAll(keyName, " ", "-")
	keyName = strings.ReplaceAll(keyName, ".", "-")

	// Add index if needed to ensure uniqueness
	if index > 0 {
		keyName = fmt.Sprintf("%s-%d", keyName, index+1)
	}

	return keyName
}

// generatePlaceholderValue creates a secure base64-encoded placeholder value for a secret
func (t *AtomicScanSecretsTool) generatePlaceholderValue(secret ScannedSecret) string {
	var placeholderText string

	// Generate more descriptive placeholders with security guidance
	secretTypeLower := strings.ToLower(secret.Type)
	switch secretTypeLower {
	case "api_key", "apikey", "api-key":
		placeholderText = "YOUR_API_KEY_HERE_REPLACE_WITH_ACTUAL_VALUE"
	case "password", "pwd":
		placeholderText = "YOUR_SECURE_PASSWORD_HERE_MIN_12_CHARS"
	case "token", "access_token", "auth_token", "bearer_token":
		placeholderText = "YOUR_ACCESS_TOKEN_HERE_REPLACE_WITH_ACTUAL_VALUE"
	case "private_key", "privatekey", "private-key", "ssh_key":
		placeholderText = "-----BEGIN PRIVATE KEY-----\nYOUR_PRIVATE_KEY_CONTENT_HERE_REPLACE_WITH_ACTUAL_KEY\n-----END PRIVATE KEY-----"
	case "certificate", "cert", "tls_cert":
		placeholderText = "-----BEGIN CERTIFICATE-----\nYOUR_CERTIFICATE_CONTENT_HERE_REPLACE_WITH_ACTUAL_CERT\n-----END CERTIFICATE-----"
	case "database_url", "db_url", "database_connection":
		placeholderText = "postgresql://username:password@hostname:5432/database_name"
	case "webhook_url", "webhook":
		placeholderText = "https://your-domain.com/webhook/endpoint"
	case "smtp_password", "email_password":
		placeholderText = "YOUR_EMAIL_APP_PASSWORD_HERE_NOT_LOGIN_PASSWORD"
	case "encryption_key", "secret_key", "signing_key":
		placeholderText = "YOUR_ENCRYPTION_KEY_HERE_USE_SECURE_RANDOM_GENERATOR"
	case "oauth_secret", "client_secret":
		placeholderText = "YOUR_OAUTH_CLIENT_SECRET_FROM_PROVIDER_CONSOLE"
	default:
		// Provide more descriptive default with context from the pattern or context
		contextInfo := ""
		if secret.Pattern != "" {
			// Use pattern name as additional context
			patternName := strings.ToUpper(strings.ReplaceAll(secret.Pattern, "-", "_"))
			contextInfo = fmt.Sprintf("_FOR_%s", patternName)
		} else if secret.Context != "" {
			// Extract meaningful context if available
			contextWords := strings.Fields(secret.Context)
			if len(contextWords) > 0 {
				contextInfo = fmt.Sprintf("_FROM_%s", strings.ToUpper(contextWords[0]))
			}
		}

		placeholderText = fmt.Sprintf("YOUR_%s_VALUE_HERE%s_REPLACE_WITH_ACTUAL_SECRET",
			strings.ToUpper(strings.ReplaceAll(secretTypeLower, "-", "_")),
			contextInfo)
	}

	// Add security metadata as comment in the placeholder for validation
	placeholderWithMetadata := fmt.Sprintf("%s\n# Secret Type: %s\n# Original Location: %s:%d\n# SECURITY: Replace this placeholder with actual secret value",
		placeholderText, secret.Type, secret.File, secret.Line)

	// Base64 encode the enhanced placeholder
	encoded := base64.StdEncoding.EncodeToString([]byte(placeholderWithMetadata))

	// Log the placeholder generation for audit purposes
	t.logger.Debug().
		Str("secret_type", secret.Type).
		Str("secret_file", secret.File).
		Int("secret_line", secret.Line).
		Bool("placeholder_generated", true).
		Msg("Generated secure placeholder for secret")

	return encoded
}

// extractAppName extracts app name from secret name for labels
func (t *AtomicScanSecretsTool) extractAppName(secretName string) string {
	// Remove common prefixes to get app name
	appName := strings.TrimPrefix(secretName, "app-")
	parts := strings.Split(appName, "-")
	if len(parts) > 1 {
		// Remove the secret type suffix to get app name
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return "my-app"
}

// extractSecretType extracts secret type from secret name for labels
func (t *AtomicScanSecretsTool) extractSecretType(secretName string) string {
	parts := strings.Split(secretName, "-")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return "general"
}

// AI Context Interface Implementations for AtomicScanSecretsResult

// SimpleTool interface implementation

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
func (t *AtomicScanSecretsTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// Validate validates the tool arguments
func (t *AtomicScanSecretsTool) Validate(ctx context.Context, args interface{}) error {
	scanArgs, ok := args.(AtomicScanSecretsArgs)
	if !ok {
		return types.NewValidationErrorBuilder("Invalid argument type for atomic_scan_secrets", "args", args).
			WithField("expected", "AtomicScanSecretsArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if scanArgs.SessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", scanArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	// Validate file patterns if provided
	for _, pattern := range scanArgs.FilePatterns {
		if _, err := filepath.Match(pattern, "test"); err != nil {
			return types.NewValidationErrorBuilder("Invalid file pattern", "file_pattern", pattern).
				WithField("error", err.Error()).
				Build()
		}
	}

	return nil
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicScanSecretsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	scanArgs, ok := args.(AtomicScanSecretsArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_scan_secrets", "args", args).
			WithField("expected", "AtomicScanSecretsArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, scanArgs)
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicScanSecretsTool) ExecuteTyped(ctx context.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	return t.ExecuteScanSecrets(ctx, args)
}

// AI Context methods are now provided by embedded BaseAIContextResult
