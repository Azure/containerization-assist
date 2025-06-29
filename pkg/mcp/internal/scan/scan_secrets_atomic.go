package scan

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// mcp import removed - using mcptypes
	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"

	mcptypes "github.com/Azure/container-kit/pkg/mcp"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

type AtomicScanSecretsArgs struct {
	types.BaseToolArgs

	ScanPath        string   `json:"scan_path,omitempty" description:"Path to scan (default: session workspace)"`
	FilePatterns    []string `json:"file_patterns,omitempty" description:"File patterns to include in scan (e.g., '*.py', '*.js')"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty" description:"File patterns to exclude from scan"`

	ScanDockerfiles bool `json:"scan_dockerfiles,omitempty" description:"Include Dockerfiles in scan"`
	ScanManifests   bool `json:"scan_manifests,omitempty" description:"Include Kubernetes manifests in scan"`
	ScanSourceCode  bool `json:"scan_source_code,omitempty" description:"Include source code files in scan"`
	ScanEnvFiles    bool `json:"scan_env_files,omitempty" description:"Include .env files in scan"`

	SuggestRemediation bool `json:"suggest_remediation,omitempty" description:"Provide remediation suggestions"`
	GenerateSecrets    bool `json:"generate_secrets,omitempty" description:"Generate Kubernetes Secret manifests"`
}

type AtomicScanSecretsResult struct {
	types.BaseToolResponse
	internal.BaseAIContextResult

	SessionID    string        `json:"session_id"`
	ScanPath     string        `json:"scan_path"`
	FilesScanned int           `json:"files_scanned"`
	Duration     time.Duration `json:"duration"`

	SecretsFound      int             `json:"secrets_found"`
	DetectedSecrets   []ScannedSecret `json:"detected_secrets"`
	SeverityBreakdown map[string]int  `json:"severity_breakdown"`

	FileResults []FileSecretScanResult `json:"file_results"`

	RemediationPlan  *SecretRemediationPlan    `json:"remediation_plan,omitempty"`
	GeneratedSecrets []GeneratedSecretManifest `json:"generated_secrets,omitempty"`

	SecurityScore   int      `json:"security_score"`
	RiskLevel       string   `json:"risk_level"`
	Recommendations []string `json:"recommendations"`

	ScanContext map[string]interface{} `json:"scan_context"`
}

type ScannedSecret struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Type       string `json:"type"`
	Pattern    string `json:"pattern"`
	Value      string `json:"value"`
	Severity   string `json:"severity"`
	Context    string `json:"context"`
	Confidence int    `json:"confidence"`
}

type FileSecretScanResult struct {
	FilePath     string          `json:"file_path"`
	FileType     string          `json:"file_type"`
	SecretsFound int             `json:"secrets_found"`
	Secrets      []ScannedSecret `json:"secrets"`
	CleanStatus  string          `json:"clean_status"`
}

type SecretRemediationPlan struct {
	ImmediateActions []string          `json:"immediate_actions"`
	SecretReferences []SecretReference `json:"secret_references"`
	ConfigMapEntries map[string]string `json:"config_map_entries"`
	PreferredManager string            `json:"preferred_manager"`
	MigrationSteps   []string          `json:"migration_steps"`
}

type SecretReference struct {
	SecretName     string `json:"secret_name"`
	SecretKey      string `json:"secret_key"`
	OriginalEnvVar string `json:"original_env_var"`
	KubernetesRef  string `json:"kubernetes_ref"`
}

type GeneratedSecretManifest struct {
	Name     string   `json:"name"`
	Content  string   `json:"content"`
	FilePath string   `json:"file_path"`
	Keys     []string `json:"keys"`
}

func standardSecretScanStages() []mcp.ProgressStage {
	return []mcp.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating scan path"},
		{Name: "Analyze", Weight: 0.15, Description: "Analyzing file patterns and scan configuration"},
		{Name: "Scan", Weight: 0.50, Description: "Scanning files for secrets"},
		{Name: "Process", Weight: 0.20, Description: "Processing results and generating recommendations"},
		{Name: "Finalize", Weight: 0.05, Description: "Generating reports and remediation plans"},
	}
}

type AtomicScanSecretsTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcp.ToolSessionManager
	logger          zerolog.Logger
}

func NewAtomicScanSecretsTool(adapter mcptypes.PipelineOperations, sessionManager mcp.ToolSessionManager, logger zerolog.Logger) *AtomicScanSecretsTool {
	return &AtomicScanSecretsTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With().Str("tool", "atomic_scan_secrets").Logger(),
	}
}

func (t *AtomicScanSecretsTool) ExecuteScanSecrets(ctx context.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()

	return t.executeWithoutProgress(ctx, args, startTime)
}

func (t *AtomicScanSecretsTool) ExecuteWithContext(serverCtx *server.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	startTime := time.Now()

	_ = internal.NewGoMCPProgressAdapter(serverCtx, []internal.LocalProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session"},
		{Name: "Scan", Weight: 0.80, Description: "Scanning"},
		{Name: "Finalize", Weight: 0.10, Description: "Updating state"},
	})

	ctx := context.Background()
	result, err := t.executeWithProgress(ctx, args, startTime, nil)

	if err != nil {
		t.logger.Info().Msg("Secrets scan failed")
		if result == nil {
			result = &AtomicScanSecretsResult{
				BaseToolResponse: types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
				SessionID:        args.SessionID,
				Duration:         time.Since(startTime),
				RiskLevel:        "unknown",
			}
		}
		return result, nil
	} else {
		t.logger.Info().Msg("Secrets scan completed successfully")
	}

	return result, nil
}

func (t *AtomicScanSecretsTool) executeWithProgress(ctx context.Context, args AtomicScanSecretsArgs, startTime time.Time, reporter interface{}) (*AtomicScanSecretsResult, error) {
	t.logger.Info().Msg("Loading session")

	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicScanSecretsResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
			BaseAIContextResult: internal.NewBaseAIContextResult("scan", false, time.Since(startTime)),
			SessionID:           args.SessionID,
			Duration:            time.Since(startTime),
			RiskLevel:           "unknown",
		}
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, mcp.NewRichError("SESSION_ACCESS_FAILED", fmt.Sprintf("failed to get session: %v", err), types.ErrTypeSession)
	}
	session := sessionInterface.(*mcp.SessionState)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("scan_path", args.ScanPath).
		Msg("Starting atomic secret scanning")

	result := &AtomicScanSecretsResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", session.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("scan", false, 0),
		SessionID:           session.SessionID,
		ScanContext:         make(map[string]interface{}),
		SeverityBreakdown:   make(map[string]int),
	}

	t.logger.Info().Msg("Session loaded")

	scanPath := args.ScanPath
	if scanPath == "" {
		scanPath = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	}
	result.ScanPath = scanPath

	if _, err := os.Stat(scanPath); os.IsNotExist(err) {
		t.logger.Error().Str("scan_path", scanPath).Msg("Scan path does not exist")
		result.Duration = time.Since(startTime)
		return result, mcp.NewRichError("SCAN_PATH_NOT_FOUND", fmt.Sprintf("scan path does not exist: %s", scanPath), types.ErrTypeSystem)
	}

	t.logger.Info().Msg("Initialization complete")

	t.logger.Info().Msg("Analyzing scan configuration")

	filePatterns := args.FilePatterns
	if len(filePatterns) == 0 {
		filePatterns = t.getDefaultFilePatterns(args)
	}

	excludePatterns := args.ExcludePatterns
	if len(excludePatterns) == 0 {
		excludePatterns = []string{"*.git/*", "node_modules/*", "vendor/*", "*.log"}
	}

	t.logger.Info().Msg("Scan configuration analyzed")

	t.logger.Info().Msg("Scanning files for secrets")

	allSecrets, fileResults, filesScanned, err := t.performSecretScan(scanPath, filePatterns, excludePatterns, reporter)
	if err != nil {
		t.logger.Error().Err(err).Str("scan_path", scanPath).Msg("Failed to scan directory")
		result.Duration = time.Since(startTime)
		return result, mcp.NewRichError("SCAN_DIRECTORY_FAILED", fmt.Sprintf("failed to scan directory: %v", err), types.ErrTypeSystem)
	}

	result.FilesScanned = filesScanned
	result.SecretsFound = len(allSecrets)
	result.DetectedSecrets = allSecrets
	result.FileResults = fileResults

	t.logger.Info().Msg(fmt.Sprintf("Scanned %d files, found %d secrets", filesScanned, len(allSecrets)))

	t.logger.Info().Msg("Processing scan results")

	result.SeverityBreakdown = t.calculateSeverityBreakdown(allSecrets)
	result.SecurityScore = t.calculateSecurityScore(allSecrets)
	result.RiskLevel = t.determineRiskLevel(result.SecurityScore, allSecrets)
	result.Recommendations = t.generateRecommendations(allSecrets, args)

	t.logger.Info().Msg("Generated security analysis")

	if args.SuggestRemediation && len(allSecrets) > 0 {
		result.RemediationPlan = t.generateRemediationPlan(allSecrets)
		t.logger.Info().Msg("Generated remediation plan")
	}

	t.logger.Info().Msg("Result processing complete")

	t.logger.Info().Msg("Finalizing results")

	if args.GenerateSecrets && len(allSecrets) > 0 {
		generatedSecrets, err := t.generateKubernetesSecrets(allSecrets, session.SessionID)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to generate Kubernetes secrets")
		} else {
			result.GeneratedSecrets = generatedSecrets
			t.logger.Info().Msg("Generated Kubernetes secrets")
		}
	}

	result.Duration = time.Since(startTime)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Int("files_scanned", result.FilesScanned).
		Int("secrets_found", result.SecretsFound).
		Str("risk_level", result.RiskLevel).
		Int("security_score", result.SecurityScore).
		Dur("duration", result.Duration).
		Msg("Secret scanning completed")

	t.logger.Info().Msg("Secret scanning completed")

	return result, nil
}

func (t *AtomicScanSecretsTool) executeWithoutProgress(ctx context.Context, args AtomicScanSecretsArgs, startTime time.Time) (*AtomicScanSecretsResult, error) {
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicScanSecretsResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", args.SessionID, args.DryRun),
			BaseAIContextResult: internal.NewBaseAIContextResult("scan", false, time.Since(startTime)),
			SessionID:           args.SessionID,
			Duration:            time.Since(startTime),
			RiskLevel:           "unknown",
		}
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, mcp.NewRichError("SESSION_ACCESS_FAILED", fmt.Sprintf("failed to get session: %v", err), types.ErrTypeSession)
	}
	session := sessionInterface.(*mcp.SessionState)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("scan_path", args.ScanPath).
		Msg("Starting atomic secret scanning")

	result := &AtomicScanSecretsResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_scan_secrets", session.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("scan", false, 0),
		SessionID:           session.SessionID,
		ScanContext:         make(map[string]interface{}),
		SeverityBreakdown:   make(map[string]int),
	}

	scanPath := args.ScanPath
	if scanPath == "" {
		scanPath = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)
	}
	result.ScanPath = scanPath

	if _, err := os.Stat(scanPath); os.IsNotExist(err) {
		t.logger.Error().Str("scan_path", scanPath).Msg("Scan path does not exist")
		result.Duration = time.Since(startTime)
		return result, mcp.NewRichError("SCAN_PATH_NOT_FOUND", fmt.Sprintf("scan path does not exist: %s", scanPath), types.ErrTypeSystem)
	}

	filePatterns := args.FilePatterns
	if len(filePatterns) == 0 {
		filePatterns = t.getDefaultFilePatterns(args)
	}

	excludePatterns := args.ExcludePatterns
	if len(excludePatterns) == 0 {
		excludePatterns = []string{"*.git/*", "node_modules/*", "vendor/*", "*.log"}
	}

	allSecrets, fileResults, filesScanned, err := t.performSecretScan(scanPath, filePatterns, excludePatterns, nil)
	if err != nil {
		t.logger.Error().Err(err).Str("scan_path", scanPath).Msg("Failed to scan directory")
		result.Duration = time.Since(startTime)
		return result, mcp.NewRichError("SCAN_DIRECTORY_FAILED", fmt.Sprintf("failed to scan directory: %v", err), types.ErrTypeSystem)
	}

	result.FilesScanned = filesScanned
	result.SecretsFound = len(allSecrets)
	result.DetectedSecrets = allSecrets
	result.FileResults = fileResults
	result.SeverityBreakdown = t.calculateSeverityBreakdown(allSecrets)

	result.SecurityScore = t.calculateSecurityScore(allSecrets)
	result.RiskLevel = t.determineRiskLevel(result.SecurityScore, allSecrets)

	result.Recommendations = t.generateRecommendations(allSecrets, args)

	if args.SuggestRemediation && len(allSecrets) > 0 {
		result.RemediationPlan = t.generateRemediationPlan(allSecrets)
	}

	if args.GenerateSecrets && len(allSecrets) > 0 {
		generatedSecrets, err := t.generateKubernetesSecrets(allSecrets, session.SessionID)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to generate Kubernetes secrets")
		} else {
			result.GeneratedSecrets = generatedSecrets
		}
	}

	result.Duration = time.Since(startTime)

	result.BaseAIContextResult.Duration = result.Duration
	result.BaseAIContextResult.IsSuccessful = true // Scan completed successfully
	result.BaseAIContextResult.ErrorCount = result.SecretsFound
	result.BaseAIContextResult.WarningCount = len(result.Recommendations)

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

func (t *AtomicScanSecretsTool) performSecretScan(scanPath string, filePatterns, excludePatterns []string, reporter interface{}) ([]ScannedSecret, []FileSecretScanResult, int, error) {
	scanner := utils.NewSecretScanner()
	var allSecrets []ScannedSecret
	var fileResults []FileSecretScanResult
	filesScanned := 0

	totalFiles := 0
	err := filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && t.shouldScanFile(path, filePatterns, excludePatterns) {
			totalFiles++
		}
		return nil
	})
	if err != nil {
		t.logger.Warn().Err(err).Msg("Failed to count files for progress")
	}

	err = filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !t.shouldScanFile(path, filePatterns, excludePatterns) {
			return nil
		}

		fileSecrets, err := t.scanFileForSecrets(path, scanner)
		if err != nil {
			t.logger.Warn().Err(err).Str("file", path).Msg("Failed to scan file")
			return nil // Continue with other files
		}

		filesScanned++

		if reporter != nil && totalFiles > 0 {
			progress := float64(filesScanned) / float64(totalFiles)
			if progressReporter, ok := reporter.(interface {
				ReportStage(float64, string)
			}); ok {
				progressReporter.ReportStage(progress, fmt.Sprintf("Scanned %d/%d files", filesScanned, totalFiles))
			}
		}

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

	if len(patterns) == 0 {
		patterns = []string{"*.yaml", "*.yml", "*.json", ".env*", "*.env", "Dockerfile*"}
	}

	return patterns
}

func (t *AtomicScanSecretsTool) shouldScanFile(path string, includePatterns, excludePatterns []string) bool {
	filename := filepath.Base(path)

	for _, pattern := range excludePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err != nil {
			continue
		}
		if matched {
			return false
		}
	}

	for _, pattern := range includePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err != nil {
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

	if len(value) > 20 && (strings.Contains(pattern, "key") || strings.Contains(pattern, "token")) {
		return "critical"
	}

	if strings.Contains(pattern, "password") || strings.Contains(pattern, "secret") {
		return "high"
	}

	return "medium"
}

func (t *AtomicScanSecretsTool) calculateConfidence(pattern string) int {
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
	t.logger.Info().
		Int("secret_count", len(secrets)).
		Str("session_id", sessionID).
		Msg("Generating Kubernetes Secret manifests")

	if len(secrets) == 0 {
		t.logger.Info().Msg("No secrets found, skipping manifest generation")
		return []GeneratedSecretManifest{}, nil
	}

	var manifests []GeneratedSecretManifest

	secretsByType := make(map[string][]ScannedSecret)
	for _, secret := range secrets {
		secretType := t.normalizeSecretType(secret.Type)
		secretsByType[secretType] = append(secretsByType[secretType], secret)
	}

	for secretType, typeSecrets := range secretsByType {
		secretName := t.generateSecretName(secretType)

		secretData := make(map[string]string)
		var keys []string

		for i, secret := range typeSecrets {
			key := t.generateSecretKey(secret, i)
			keys = append(keys, key)
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
	yamlContent := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  labels:
    app: %s
    generated-by: container-kit
    secret-type: %s
  annotations:
    description: "Generated from detected secrets in source code"
    secrets-detected: "%d"
    generation-time: "%s"
type: Opaque
data:
`, name, t.extractAppName(name), t.extractSecretType(name), len(detectedSecrets), time.Now().UTC().Format(time.RFC3339))

	for key, value := range secretData {
		yamlContent += fmt.Sprintf("  %s: %s\n", key, value)
	}

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

	for i, secret := range detectedSecrets {
		yamlContent += fmt.Sprintf("# %d. Found in %s:%d - Type: %s (Severity: %s)\n",
			i+1, secret.File, secret.Line, secret.Type, secret.Severity)
	}

	return yamlContent
}

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
		normalized := strings.ToLower(secretType)
		normalized = strings.ReplaceAll(normalized, "_", "-")
		normalized = strings.ReplaceAll(normalized, " ", "-")
		return normalized
	}
}

func (t *AtomicScanSecretsTool) generateSecretName(secretType string) string {
	name := fmt.Sprintf("app-%s", secretType)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.TrimSuffix(name, "-")
	return name
}

func (t *AtomicScanSecretsTool) generateSecretKey(secret ScannedSecret, index int) string {
	var keyName string

	fileName := filepath.Base(secret.File)
	fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

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

	keyName = strings.ToLower(keyName)
	keyName = strings.ReplaceAll(keyName, "_", "-")
	keyName = strings.ReplaceAll(keyName, " ", "-")
	keyName = strings.ReplaceAll(keyName, ".", "-")

	if index > 0 {
		keyName = fmt.Sprintf("%s-%d", keyName, index+1)
	}

	return keyName
}

func (t *AtomicScanSecretsTool) generatePlaceholderValue(secret ScannedSecret) string {
	var placeholderText string

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
		contextInfo := ""
		if secret.Pattern != "" {
			patternName := strings.ToUpper(strings.ReplaceAll(secret.Pattern, "-", "_"))
			contextInfo = fmt.Sprintf("_FOR_%s", patternName)
		} else if secret.Context != "" {
			contextWords := strings.Fields(secret.Context)
			if len(contextWords) > 0 {
				contextInfo = fmt.Sprintf("_FROM_%s", strings.ToUpper(contextWords[0]))
			}
		}

		placeholderText = fmt.Sprintf("YOUR_%s_VALUE_HERE%s_REPLACE_WITH_ACTUAL_SECRET",
			strings.ToUpper(strings.ReplaceAll(secretTypeLower, "-", "_")),
			contextInfo)
	}

	placeholderWithMetadata := fmt.Sprintf("%s\n# Secret Type: %s\n# Original Location: %s:%d\n# SECURITY: Replace this placeholder with actual secret value",
		placeholderText, secret.Type, secret.File, secret.Line)

	encoded := base64.StdEncoding.EncodeToString([]byte(placeholderWithMetadata))

	t.logger.Debug().
		Str("secret_type", secret.Type).
		Str("secret_file", secret.File).
		Int("secret_line", secret.Line).
		Bool("placeholder_generated", true).
		Msg("Generated secure placeholder for secret")

	return encoded
}

func (t *AtomicScanSecretsTool) extractAppName(secretName string) string {
	appName := strings.TrimPrefix(secretName, "app-")
	parts := strings.Split(appName, "-")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return "my-app"
}

func (t *AtomicScanSecretsTool) extractSecretType(secretName string) string {
	parts := strings.Split(secretName, "-")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return "general"
}

func (t *AtomicScanSecretsTool) GetName() string {
	return "atomic_scan_secrets"
}

func (t *AtomicScanSecretsTool) GetDescription() string {
	return "Scans files for hardcoded secrets, credentials, and sensitive data with automatic remediation suggestions"
}

func (t *AtomicScanSecretsTool) GetVersion() string {
	return "1.0.0"
}

func (t *AtomicScanSecretsTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

func (t *AtomicScanSecretsTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
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
			"scan_dockerfiles":    "bool - Include Dockerfiles in scan",
			"scan_manifests":      "bool - Include Kubernetes manifests in scan",
			"scan_source_code":    "bool - Include source code files in scan",
			"scan_env_files":      "bool - Include .env files in scan",
			"suggest_remediation": "bool - Provide remediation suggestions",
			"generate_secrets":    "bool - Generate Kubernetes Secret manifests",
			"dry_run":             "bool - Scan without making changes",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Basic Secret Scan",
				Description: "Scan session workspace for hardcoded secrets",
				Input: map[string]interface{}{
					"session_id":       "session-123",
					"scan_source_code": true,
					"scan_env_files":   true,
					"scan_dockerfiles": true,
				},
				Output: map[string]interface{}{
					"success":        true,
					"files_scanned":  25,
					"secrets_found":  3,
					"risk_level":     "medium",
					"security_score": 75,
				},
			},
			{
				Name:        "Comprehensive Security Scan",
				Description: "Full security scan with remediation and secret generation",
				Input: map[string]interface{}{
					"session_id":          "session-456",
					"scan_path":           "/workspace/myapp",
					"suggest_remediation": true,
					"generate_secrets":    true,
					"scan_dockerfiles":    true,
					"scan_manifests":      true,
					"scan_source_code":    true,
					"scan_env_files":      true,
				},
				Output: map[string]interface{}{
					"success":           true,
					"files_scanned":     42,
					"secrets_found":     7,
					"security_score":    45,
					"risk_level":        "high",
					"generated_secrets": 2,
					"remediation_steps": 5,
				},
			},
			{
				Name:        "Targeted Configuration Scan",
				Description: "Scan specific file patterns for configuration secrets",
				Input: map[string]interface{}{
					"session_id": "session-789",
					"file_patterns": []string{
						"*.yaml",
						"*.yml",
						"*.json",
						".env*",
					},
					"exclude_patterns": []string{
						"node_modules/*",
						"*.log",
					},
				},
				Output: map[string]interface{}{
					"success":        true,
					"files_scanned":  12,
					"secrets_found":  2,
					"security_score": 85,
					"risk_level":     "low",
				},
			},
		},
	}
}

func (t *AtomicScanSecretsTool) Validate(ctx context.Context, args interface{}) error {
	scanArgs, ok := args.(AtomicScanSecretsArgs)
	if !ok {
		return mcp.NewErrorBuilder("INVALID_ARGUMENTS_TYPE", "Invalid argument type for atomic_scan_secrets", "validation_error").
			WithField("expected", "AtomicScanSecretsArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if scanArgs.SessionID == "" {
		return mcp.NewErrorBuilder("SessionID is required", "session_id", scanArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	for _, pattern := range scanArgs.FilePatterns {
		if _, err := filepath.Match(pattern, "test"); err != nil {
			return mcp.NewErrorBuilder("Invalid file pattern", "file_pattern", pattern).
				WithField("error", err.Error()).
				Build()
		}
	}

	return nil
}

func (t *AtomicScanSecretsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	scanArgs, ok := args.(AtomicScanSecretsArgs)
	if !ok {
		return nil, mcp.NewErrorBuilder("INVALID_ARGUMENTS_TYPE", "Invalid argument type for atomic_scan_secrets", "validation_error").
			WithField("expected", "AtomicScanSecretsArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	return t.ExecuteTyped(ctx, scanArgs)
}

func (t *AtomicScanSecretsTool) ExecuteTyped(ctx context.Context, args AtomicScanSecretsArgs) (*AtomicScanSecretsResult, error) {
	return t.ExecuteScanSecrets(ctx, args)
}
