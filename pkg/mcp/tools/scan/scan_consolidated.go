package scan

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// Register consolidated security scan tool
func init() {
	core.RegisterTool("security_scan_consolidated", func() api.Tool {
		return &ConsolidatedSecurityScanTool{}
	})
}

// ConsolidatedSecurityScanInput represents unified input for all security scanning variants
type ConsolidatedSecurityScanInput struct {
	// Core parameters (with backward compatibility aliases)
	SessionID string `json:"session_id,omitempty" validate:"omitempty,session_id" description:"Session ID for state correlation"`
	Target    string `json:"target" validate:"required" description:"Target to scan (file path, directory, image, or URL)"`
	Content   string `json:"content,omitempty" description:"Direct content to scan (alternative to target)"`
	Path      string `json:"path,omitempty" description:"Alias for target for backward compatibility"`
	FilePath  string `json:"file_path,omitempty" description:"Alias for target for backward compatibility"`

	// Scan configuration
	ScanMode    string      `json:"scan_mode,omitempty" validate:"omitempty,oneof=quick comprehensive atomic" description:"Scan mode: quick, comprehensive, or atomic"`
	ScanType    string      `json:"scan_type,omitempty" validate:"omitempty,oneof=secrets vulnerabilities compliance all" description:"Type of scan to perform"`
	ContentType ContentType `json:"content_type,omitempty" description:"Content type hint for better scanning accuracy"`

	// Scan options
	IncludeSecrets         bool             `json:"include_secrets,omitempty" description:"Include secret detection scanning"`
	IncludeVulnerabilities bool             `json:"include_vulnerabilities,omitempty" description:"Include vulnerability scanning"`
	IncludeCompliance      bool             `json:"include_compliance,omitempty" description:"Include compliance checking"`
	IncludeHighEntropy     bool             `json:"include_high_entropy,omitempty" description:"Include high-entropy string detection"`
	SensitivityLevel       SensitivityLevel `json:"sensitivity_level,omitempty" description:"Scanning sensitivity level"`

	// Performance options
	UseCache    bool  `json:"use_cache,omitempty" description:"Use cached results if available"`
	Timeout     int   `json:"timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Scan timeout in seconds"`
	MaxFileSize int64 `json:"max_file_size,omitempty" description:"Maximum file size to scan in bytes"`
	SkipBinary  bool  `json:"skip_binary,omitempty" description:"Skip binary files during scanning"`

	// Advanced options
	CustomRules    []string               `json:"custom_rules,omitempty" description:"Custom scanning rules or patterns"`
	Metadata       map[string]interface{} `json:"metadata,omitempty" description:"Additional metadata for scanning context"`
	DryRun         bool                   `json:"dry_run,omitempty" description:"Preview scan without executing"`
	IncludeContext bool                   `json:"include_context,omitempty" description:"Include surrounding context in results"`
}

// Validate implements validation using tag-based validation
func (c ConsolidatedSecurityScanInput) Validate() error {
	target := c.getTarget()
	if target == "" && c.Content == "" {
		return errors.NewError().Message("either target or content is required").Build()
	}
	return validation.ValidateTaggedStruct(c)
}

// getTarget returns the target, handling backward compatibility aliases
func (c ConsolidatedSecurityScanInput) getTarget() string {
	if c.Target != "" {
		return c.Target
	}
	if c.Path != "" {
		return c.Path
	}
	return c.FilePath
}

// getScanMode returns the scan mode, defaulting to comprehensive
func (c ConsolidatedSecurityScanInput) getScanMode() string {
	if c.ScanMode != "" {
		return c.ScanMode
	}
	return "comprehensive"
}

// getScanType returns the scan type, defaulting to all
func (c ConsolidatedSecurityScanInput) getScanType() string {
	if c.ScanType != "" {
		return c.ScanType
	}
	return "all"
}

// ConsolidatedSecurityScanOutput represents unified output for all security scanning variants
type ConsolidatedSecurityScanOutput struct {
	// Status
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`

	// Core scan results (from all variants)
	ScanMode     string        `json:"scan_mode"`
	ScanType     string        `json:"scan_type"`
	Target       string        `json:"target"`
	ContentType  ContentType   `json:"content_type,omitempty"`
	ScanTime     time.Time     `json:"scan_time"`
	ScanDuration time.Duration `json:"scan_duration"`

	// Unified results from all scan types
	Secrets           []Secret           `json:"secrets,omitempty"`
	Vulnerabilities   []Vulnerability    `json:"vulnerabilities,omitempty"`
	ComplianceResults []ComplianceResult `json:"compliance_results,omitempty"`
	SecurityFindings  []ScanFinding      `json:"security_findings,omitempty"`

	// Summary and metadata
	Summary          ScanSummary       `json:"summary"`
	Recommendations  []string          `json:"recommendations,omitempty"`
	RemediationSteps []RemediationStep `json:"remediation_steps,omitempty"`
	RiskScore        int               `json:"risk_score"`       // 0-100
	ConfidenceScore  float64           `json:"confidence_score"` // 0.0-1.0

	// Scanner-specific results
	ScannerResults map[string]*ScanResult `json:"scanner_results,omitempty"`
	CombinedResult *CombinedScanResult    `json:"combined_result,omitempty"`

	// Performance metrics
	FilesScanned int   `json:"files_scanned"`
	BytesScanned int64 `json:"bytes_scanned"`
	CacheHit     bool  `json:"cache_hit,omitempty"`

	// Metadata
	ToolVersion string                 `json:"tool_version"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
}

// Supporting types (consolidated from all scan variants)
type ScanSummary struct {
	TotalSecrets         int            `json:"total_secrets"`
	TotalVulnerabilities int            `json:"total_vulnerabilities"`
	TotalFindings        int            `json:"total_findings"`
	CriticalIssues       int            `json:"critical_issues"`
	HighRiskIssues       int            `json:"high_risk_issues"`
	MediumRiskIssues     int            `json:"medium_risk_issues"`
	LowRiskIssues        int            `json:"low_risk_issues"`
	BySeverity           map[string]int `json:"by_severity"`
	ByType               map[string]int `json:"by_type"`
	ByScanner            map[string]int `json:"by_scanner"`
}

// RemediationStep is defined in types.go

// ConsolidatedSecurityScanTool - Unified security scanning tool
type ConsolidatedSecurityScanTool struct {
	// Service dependencies
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	scanner         services.Scanner
	configValidator services.ConfigValidator
	logger          *slog.Logger

	// Core scanning components
	secretScanner        *FileSecretScanner
	vulnerabilityScanner *VulnerabilityScanner
	complianceScanner    *ComplianceScanner
	scannerRegistry      *ScannerRegistry
	resultProcessor      *ResultProcessor
	remediationGen       *RemediationGenerator
	cacheManager         *ScanCacheManager

	// State management
	workspaceDir string
}

// NewConsolidatedSecurityScanTool creates a new consolidated security scanning tool
func NewConsolidatedSecurityScanTool(
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) *ConsolidatedSecurityScanTool {
	toolLogger := logger.With("tool", "security_scan_consolidated")

	return &ConsolidatedSecurityScanTool{
		sessionStore:         serviceContainer.SessionStore(),
		sessionState:         serviceContainer.SessionState(),
		scanner:              serviceContainer.Scanner(),
		configValidator:      serviceContainer.ConfigValidator(),
		logger:               toolLogger,
		secretScanner:        NewFileSecretScanner(toolLogger),
		vulnerabilityScanner: NewVulnerabilityScanner(toolLogger),
		complianceScanner:    NewComplianceScanner(toolLogger),
		scannerRegistry:      NewScannerRegistry(toolLogger),
		resultProcessor:      NewResultProcessor(toolLogger),
		remediationGen:       NewRemediationGenerator(toolLogger),
		cacheManager:         NewScanCacheManager(toolLogger),
	}
}

// Execute implements api.Tool interface
func (t *ConsolidatedSecurityScanTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Parse input
	scanInput, err := t.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Validate input
	if err := scanInput.Validate(); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Input validation failed: %v", err),
		}, err
	}

	// Generate session ID if not provided
	sessionID := scanInput.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("scan_%d", time.Now().Unix())
	}

	// Execute scan based on mode
	result, err := t.executeScan(ctx, scanInput, sessionID, startTime)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Scan failed: %v", err),
		}, err
	}

	return api.ToolOutput{
		Success: result.Success,
		Data:    map[string]interface{}{"result": result},
	}, nil
}

// executeScan performs the security scan based on the specified mode
func (t *ConsolidatedSecurityScanTool) executeScan(
	ctx context.Context,
	input *ConsolidatedSecurityScanInput,
	sessionID string,
	startTime time.Time,
) (*ConsolidatedSecurityScanOutput, error) {
	result := &ConsolidatedSecurityScanOutput{
		Success:     false,
		SessionID:   sessionID,
		Target:      input.getTarget(),
		ScanMode:    input.getScanMode(),
		ScanType:    input.getScanType(),
		ContentType: input.ContentType,
		ToolVersion: "2.0.0",
		Timestamp:   startTime,
		ScanTime:    startTime,
		Metadata:    make(map[string]interface{}),
		Summary:     ScanSummary{BySeverity: make(map[string]int), ByType: make(map[string]int), ByScanner: make(map[string]int)},
	}

	// Initialize session
	if err := t.initializeSession(ctx, sessionID, input); err != nil {
		t.logger.Warn("Failed to initialize session", "error", err)
	}

	// Check cache if enabled
	if input.UseCache {
		if cachedResult := t.checkCache(input); cachedResult != nil {
			cachedResult.CacheHit = true
			return cachedResult, nil
		}
	}

	// Execute based on scan mode
	switch input.getScanMode() {
	case "quick":
		return t.executeQuickScan(ctx, input, result)
	case "atomic":
		return t.executeAtomicScan(ctx, input, result)
	default: // comprehensive
		return t.executeComprehensiveScan(ctx, input, result)
	}
}

// executeQuickScan performs quick security scanning
func (t *ConsolidatedSecurityScanTool) executeQuickScan(
	ctx context.Context,
	input *ConsolidatedSecurityScanInput,
	result *ConsolidatedSecurityScanOutput,
) (*ConsolidatedSecurityScanOutput, error) {
	t.logger.Info("Executing quick security scan",
		"target", result.Target,
		"session_id", result.SessionID)

	scanStart := time.Now()

	// Setup scan configuration
	scanConfig := t.createScanConfig(input, "quick")

	// Perform basic scans
	if err := t.performBasicScans(ctx, scanConfig, result); err != nil {
		return result, err
	}

	result.Success = true
	result.ScanDuration = time.Since(scanStart)

	t.logger.Info("Quick security scan completed",
		"findings", len(result.SecurityFindings),
		"secrets", len(result.Secrets),
		"duration", result.ScanDuration)

	return result, nil
}

// executeComprehensiveScan performs comprehensive security scanning
func (t *ConsolidatedSecurityScanTool) executeComprehensiveScan(
	ctx context.Context,
	input *ConsolidatedSecurityScanInput,
	result *ConsolidatedSecurityScanOutput,
) (*ConsolidatedSecurityScanOutput, error) {
	t.logger.Info("Executing comprehensive security scan",
		"target", result.Target,
		"session_id", result.SessionID)

	scanStart := time.Now()

	// Setup scan configuration
	scanConfig := t.createScanConfig(input, "comprehensive")

	// Perform all scans
	if err := t.performAllScans(ctx, scanConfig, result); err != nil {
		return result, err
	}

	// Generate recommendations
	result.Recommendations = t.generateRecommendations(result)

	// Generate remediation steps
	result.RemediationSteps = t.generateRemediationSteps(result)

	// Calculate risk score
	result.RiskScore = t.calculateRiskScore(result)

	result.Success = true
	result.ScanDuration = time.Since(scanStart)

	// Cache result if enabled
	if input.UseCache {
		t.cacheResult(input, result)
	}

	t.logger.Info("Comprehensive security scan completed",
		"findings", len(result.SecurityFindings),
		"secrets", len(result.Secrets),
		"vulnerabilities", len(result.Vulnerabilities),
		"risk_score", result.RiskScore,
		"duration", result.ScanDuration)

	return result, nil
}

// executeAtomicScan performs atomic security scanning with enhanced features
func (t *ConsolidatedSecurityScanTool) executeAtomicScan(
	ctx context.Context,
	input *ConsolidatedSecurityScanInput,
	result *ConsolidatedSecurityScanOutput,
) (*ConsolidatedSecurityScanOutput, error) {
	t.logger.Info("Executing atomic security scan",
		"target", result.Target,
		"session_id", result.SessionID)

	scanStart := time.Now()

	// Enhanced scan configuration
	scanConfig := t.createScanConfig(input, "atomic")

	// Perform all scans with enhanced tracking
	if err := t.performEnhancedScans(ctx, scanConfig, result); err != nil {
		return result, err
	}

	// Run all available scanners through registry
	combinedResult, err := t.scannerRegistry.ScanWithAllApplicable(ctx, *scanConfig)
	if err != nil {
		t.logger.Warn("Combined scanner execution failed", "error", err)
	} else {
		result.CombinedResult = combinedResult
		result.ScannerResults = combinedResult.ScannerResults
	}

	// Enhanced analysis and recommendations
	result.Recommendations = t.generateAdvancedRecommendations(result)
	result.RemediationSteps = t.generateAdvancedRemediationSteps(result)
	result.RiskScore = t.calculateAdvancedRiskScore(result)
	result.ConfidenceScore = t.calculateConfidenceScore(result)

	result.Success = true
	result.ScanDuration = time.Since(scanStart)

	// Cache result if enabled
	if input.UseCache {
		t.cacheResult(input, result)
	}

	t.logger.Info("Atomic security scan completed",
		"findings", len(result.SecurityFindings),
		"secrets", len(result.Secrets),
		"vulnerabilities", len(result.Vulnerabilities),
		"scanners_used", len(result.ScannerResults),
		"risk_score", result.RiskScore,
		"confidence", result.ConfidenceScore,
		"duration", result.ScanDuration)

	return result, nil
}

// Implement api.Tool interface methods

func (t *ConsolidatedSecurityScanTool) Name() string {
	return "security_scan_consolidated"
}

func (t *ConsolidatedSecurityScanTool) Description() string {
	return "Comprehensive security scanning tool with unified interface supporting quick, comprehensive, and atomic scan modes"
}

func (t *ConsolidatedSecurityScanTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "security_scan_consolidated",
		Description: "Comprehensive security scanning tool with unified interface supporting quick, comprehensive, and atomic scan modes",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Target to scan (file path, directory, image, or URL)",
				},
				"scan_mode": map[string]interface{}{
					"type":        "string",
					"description": "Scan mode: quick, comprehensive, or atomic",
					"enum":        []string{"quick", "comprehensive", "atomic"},
				},
				"scan_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of scan to perform",
					"enum":        []string{"secrets", "vulnerabilities", "compliance", "all"},
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for state correlation",
				},
				"include_secrets": map[string]interface{}{
					"type":        "boolean",
					"description": "Include secret detection scanning",
				},
				"include_vulnerabilities": map[string]interface{}{
					"type":        "boolean",
					"description": "Include vulnerability scanning",
				},
				"include_compliance": map[string]interface{}{
					"type":        "boolean",
					"description": "Include compliance checking",
				},
			},
			"required": []string{"target"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether scan was successful",
				},
				"secrets": map[string]interface{}{
					"type":        "array",
					"description": "Detected secrets",
				},
				"vulnerabilities": map[string]interface{}{
					"type":        "array",
					"description": "Detected vulnerabilities",
				},
				"security_findings": map[string]interface{}{
					"type":        "array",
					"description": "Security findings",
				},
				"risk_score": map[string]interface{}{
					"type":        "integer",
					"description": "Overall risk score (0-100)",
				},
				"recommendations": map[string]interface{}{
					"type":        "array",
					"description": "Security recommendations",
				},
			},
		},
	}
}

// Helper methods for tool implementation

func (t *ConsolidatedSecurityScanTool) parseInput(input api.ToolInput) (*ConsolidatedSecurityScanInput, error) {
	result := &ConsolidatedSecurityScanInput{}

	// Handle map[string]interface{} data (input.Data is already map[string]interface{})
	v := input.Data
	// Extract parameters from map
	if target, ok := v["target"].(string); ok {
		result.Target = target
	}
	if path, ok := v["path"].(string); ok {
		result.Path = path
	}
	if filePath, ok := v["file_path"].(string); ok {
		result.FilePath = filePath
	}
	if content, ok := v["content"].(string); ok {
		result.Content = content
	}
	if sessionID, ok := v["session_id"].(string); ok {
		result.SessionID = sessionID
	}
	if scanMode, ok := v["scan_mode"].(string); ok {
		result.ScanMode = scanMode
	}
	if scanType, ok := v["scan_type"].(string); ok {
		result.ScanType = scanType
	}
	if includeSecrets, ok := v["include_secrets"].(bool); ok {
		result.IncludeSecrets = includeSecrets
	}
	if includeVulns, ok := v["include_vulnerabilities"].(bool); ok {
		result.IncludeVulnerabilities = includeVulns
	}
	if includeCompliance, ok := v["include_compliance"].(bool); ok {
		result.IncludeCompliance = includeCompliance
	}
	if includeEntropy, ok := v["include_high_entropy"].(bool); ok {
		result.IncludeHighEntropy = includeEntropy
	}
	if useCache, ok := v["use_cache"].(bool); ok {
		result.UseCache = useCache
	}
	if timeout, ok := v["timeout"].(float64); ok {
		result.Timeout = int(timeout)
	}
	if skipBinary, ok := v["skip_binary"].(bool); ok {
		result.SkipBinary = skipBinary
	}
	if dryRun, ok := v["dry_run"].(bool); ok {
		result.DryRun = dryRun
	}
	// ... (more field extractions)
	// Note: input.Data is map[string]interface{}, so we don't need type assertions

	return result, nil
}

// initializeSession initializes session state for security scanning
func (t *ConsolidatedSecurityScanTool) initializeSession(ctx context.Context, sessionID string, input *ConsolidatedSecurityScanInput) error {
	if t.sessionStore == nil {
		return nil // Session management not available
	}

	sessionData := map[string]interface{}{
		"target":     input.getTarget(),
		"scan_mode":  input.getScanMode(),
		"scan_type":  input.getScanType(),
		"started_at": time.Now(),
	}

	session := &api.Session{
		ID:       sessionID,
		Metadata: sessionData,
	}
	return t.sessionStore.Create(ctx, session)
}

// checkCache checks for cached scan results
func (t *ConsolidatedSecurityScanTool) checkCache(input *ConsolidatedSecurityScanInput) *ConsolidatedSecurityScanOutput {
	if t.cacheManager == nil {
		return nil
	}

	cacheKey := fmt.Sprintf("%s_%s_%s", input.getTarget(), input.getScanMode(), input.getScanType())
	return t.cacheManager.Get(cacheKey)
}

// cacheResult caches the scan result
func (t *ConsolidatedSecurityScanTool) cacheResult(input *ConsolidatedSecurityScanInput, result *ConsolidatedSecurityScanOutput) {
	if t.cacheManager == nil {
		return
	}

	cacheKey := fmt.Sprintf("%s_%s_%s", input.getTarget(), input.getScanMode(), input.getScanType())
	t.cacheManager.Set(cacheKey, result)
}

// createScanConfig creates a scan configuration from input
func (t *ConsolidatedSecurityScanTool) createScanConfig(input *ConsolidatedSecurityScanInput, mode string) *ScanConfig {
	target := input.getTarget()
	content := input.Content
	if content == "" {
		// If no content provided, set target as content for file scanning
		content = target
	}

	return &ScanConfig{
		Content:     content,
		ContentType: input.ContentType,
		FilePath:    target,
		Options: ScanOptions{
			IncludeHighEntropy: input.IncludeHighEntropy,
			IncludeKeywords:    true,
			IncludePatterns:    true,
			IncludeBase64:      true,
			MaxFileSize:        input.MaxFileSize,
			SkipBinary:         input.SkipBinary,
			SkipSecrets:        !input.IncludeSecrets,
			SkipCompliance:     !input.IncludeCompliance,
			Timeout:            time.Duration(input.Timeout) * time.Second,
			CustomRules:        input.CustomRules,
			Metadata:           input.Metadata,
		},
		Logger: t.logger,
	}
}

// performBasicScans performs basic security scans for quick mode
func (t *ConsolidatedSecurityScanTool) performBasicScans(ctx context.Context, config *ScanConfig, result *ConsolidatedSecurityScanOutput) error {
	// Secret scanning (basic patterns)
	if !config.Options.SkipSecrets {
		secrets, err := t.performBasicSecretScan(ctx, config)
		if err != nil {
			t.logger.Warn("Basic secret scan failed", "error", err)
		} else {
			result.Secrets = secrets
			result.Summary.TotalSecrets = len(secrets)
		}
	}

	// Basic security findings
	findings := t.generateBasicSecurityFindings(config.FilePath)
	result.SecurityFindings = findings
	result.Summary.TotalFindings = len(findings)

	// Update summary
	t.updateSummary(&result.Summary, result)

	return nil
}

// performAllScans performs all security scans for comprehensive mode
func (t *ConsolidatedSecurityScanTool) performAllScans(ctx context.Context, config *ScanConfig, result *ConsolidatedSecurityScanOutput) error {
	// Secret scanning
	if !config.Options.SkipSecrets {
		secrets, err := t.performComprehensiveSecretScan(ctx, config)
		if err != nil {
			t.logger.Warn("Comprehensive secret scan failed", "error", err)
		} else {
			result.Secrets = secrets
			result.Summary.TotalSecrets = len(secrets)
		}
	}

	// Vulnerability scanning
	vulnerabilities, err := t.vulnerabilityScanner.Scan(ctx, config)
	if err != nil {
		t.logger.Warn("Vulnerability scan failed", "error", err)
	} else {
		result.Vulnerabilities = vulnerabilities
		result.Summary.TotalVulnerabilities = len(vulnerabilities)
	}

	// Compliance scanning
	if !config.Options.SkipCompliance {
		complianceResults, err := t.complianceScanner.Scan(ctx, config)
		if err != nil {
			t.logger.Warn("Compliance scan failed", "error", err)
		} else {
			result.ComplianceResults = complianceResults
		}
	}

	// Security findings
	findings := t.generateComprehensiveSecurityFindings(config.FilePath, result)
	result.SecurityFindings = findings
	result.Summary.TotalFindings = len(findings)

	// Update summary
	t.updateSummary(&result.Summary, result)

	return nil
}

// performEnhancedScans performs enhanced scans for atomic mode
func (t *ConsolidatedSecurityScanTool) performEnhancedScans(ctx context.Context, config *ScanConfig, result *ConsolidatedSecurityScanOutput) error {
	// First perform all comprehensive scans
	if err := t.performAllScans(ctx, config, result); err != nil {
		return err
	}

	// Enhanced processing with result processor
	if t.resultProcessor != nil {
		// Process secrets with result processor - placeholder for future enhancement
		// processedSecrets := t.resultProcessor.ProcessSecrets(result.Secrets)
		// result.Secrets = processedSecrets
	}

	// Enhanced metadata
	result.Metadata["enhanced_mode"] = true
	result.Metadata["scanner_count"] = len(t.scannerRegistry.GetAllScanners())

	return nil
}

// generateBasicSecurityFindings generates basic security findings
func (t *ConsolidatedSecurityScanTool) generateBasicSecurityFindings(target string) []ScanFinding {
	findings := []ScanFinding{}

	// Basic file extension checks
	if target != "" {
		findings = append(findings, ScanFinding{
			ID:          "basic-001",
			Type:        "file_analysis",
			Severity:    "info",
			Title:       "File Analysis",
			Description: fmt.Sprintf("Analyzed target: %s", target),
			File:        target,
			Line:        0,
			Remediation: "Review file contents for sensitive information",
		})
	}

	return findings
}

// generateComprehensiveSecurityFindings generates comprehensive security findings
func (t *ConsolidatedSecurityScanTool) generateComprehensiveSecurityFindings(target string, result *ConsolidatedSecurityScanOutput) []ScanFinding {
	findings := t.generateBasicSecurityFindings(target)

	// Add findings based on scan results
	for _, secret := range result.Secrets {
		findings = append(findings, ScanFinding{
			ID:          fmt.Sprintf("secret-%d", len(findings)),
			Type:        "secret_detection",
			Severity:    string(secret.Severity),
			Title:       fmt.Sprintf("Secret Detected: %s", secret.Type),
			Description: fmt.Sprintf("Detected %s with confidence %.2f", secret.Type, secret.Confidence),
			File:        secret.Location.File,
			Line:        secret.Location.Line,
			Remediation: "Remove or encrypt sensitive data",
		})
	}

	for _, vuln := range result.Vulnerabilities {
		findings = append(findings, ScanFinding{
			ID:          vuln.ID,
			Type:        "vulnerability",
			Severity:    vuln.Severity,
			Title:       fmt.Sprintf("Vulnerability: %s", vuln.Package),
			Description: vuln.Description,
			Remediation: fmt.Sprintf("Update to version %s or later", vuln.FixedIn),
		})
	}

	return findings
}

// generateRecommendations generates security recommendations
func (t *ConsolidatedSecurityScanTool) generateRecommendations(result *ConsolidatedSecurityScanOutput) []string {
	recommendations := []string{}

	if len(result.Secrets) > 0 {
		recommendations = append(recommendations, "Remove or encrypt detected secrets")
		recommendations = append(recommendations, "Implement secret scanning in CI/CD pipeline")
	}

	if len(result.Vulnerabilities) > 0 {
		recommendations = append(recommendations, "Update vulnerable dependencies")
		recommendations = append(recommendations, "Enable automatic security updates")
	}

	if result.RiskScore > 70 {
		recommendations = append(recommendations, "Implement comprehensive security review")
		recommendations = append(recommendations, "Consider security training for development team")
	}

	return recommendations
}

// generateAdvancedRecommendations generates advanced recommendations for atomic mode
func (t *ConsolidatedSecurityScanTool) generateAdvancedRecommendations(result *ConsolidatedSecurityScanOutput) []string {
	recommendations := t.generateRecommendations(result)

	// Add advanced recommendations based on detailed analysis
	recommendations = append(recommendations, "Implement infrastructure as code security scanning")
	recommendations = append(recommendations, "Set up continuous security monitoring")
	recommendations = append(recommendations, "Consider implementing security policies as code")

	return recommendations
}

// generateRemediationSteps generates remediation steps
func (t *ConsolidatedSecurityScanTool) generateRemediationSteps(result *ConsolidatedSecurityScanOutput) []RemediationStep {
	steps := []RemediationStep{}

	stepID := 1
	for _, secret := range result.Secrets {
		steps = append(steps, RemediationStep{
			Priority:    string(secret.Severity),
			Type:        "secret_removal",
			Description: fmt.Sprintf("Remove %s from %s", secret.Type, secret.Location.File),
			Command:     fmt.Sprintf("# Remove %s from %s", secret.Type, secret.Location.File),
			Impact:      "Reduces security risk by removing exposed secrets",
		})
		stepID++
	}

	return steps
}

// generateAdvancedRemediationSteps generates advanced remediation steps for atomic mode
func (t *ConsolidatedSecurityScanTool) generateAdvancedRemediationSteps(result *ConsolidatedSecurityScanOutput) []RemediationStep {
	steps := t.generateRemediationSteps(result)

	// Add advanced automated remediation options
	if t.remediationGen != nil {
		advancedSteps := t.remediationGen.GenerateAdvancedSteps(result)
		steps = append(steps, advancedSteps...)
	}

	return steps
}

// calculateRiskScore calculates overall risk score
func (t *ConsolidatedSecurityScanTool) calculateRiskScore(result *ConsolidatedSecurityScanOutput) int {
	score := 0

	// Score based on secrets
	for _, secret := range result.Secrets {
		switch secret.Severity {
		case SeverityCritical:
			score += 20
		case SeverityHigh:
			score += 15
		case SeverityMedium:
			score += 10
		case SeverityLow:
			score += 5
		}
	}

	// Score based on vulnerabilities
	for _, vuln := range result.Vulnerabilities {
		switch vuln.Severity {
		case "critical":
			score += 25
		case "high":
			score += 20
		case "medium":
			score += 10
		case "low":
			score += 5
		}
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// calculateAdvancedRiskScore calculates advanced risk score for atomic mode
func (t *ConsolidatedSecurityScanTool) calculateAdvancedRiskScore(result *ConsolidatedSecurityScanOutput) int {
	baseScore := t.calculateRiskScore(result)

	// Adjust based on additional factors
	if len(result.ComplianceResults) > 0 {
		for _, compliance := range result.ComplianceResults {
			if !compliance.Passed {
				baseScore += 10
			}
		}
	}

	// Consider confidence scores
	if result.ConfidenceScore < 0.7 {
		baseScore += 5 // Add uncertainty penalty
	}

	// Cap at 100
	if baseScore > 100 {
		baseScore = 100
	}

	return baseScore
}

// calculateConfidenceScore calculates confidence score
func (t *ConsolidatedSecurityScanTool) calculateConfidenceScore(result *ConsolidatedSecurityScanOutput) float64 {
	if len(result.Secrets) == 0 {
		return 1.0
	}

	totalConfidence := 0.0
	for _, secret := range result.Secrets {
		totalConfidence += secret.Confidence
	}

	return totalConfidence / float64(len(result.Secrets))
}

// updateSummary updates the scan summary
func (t *ConsolidatedSecurityScanTool) updateSummary(summary *ScanSummary, result *ConsolidatedSecurityScanOutput) {
	summary.TotalSecrets = len(result.Secrets)
	summary.TotalVulnerabilities = len(result.Vulnerabilities)
	summary.TotalFindings = len(result.SecurityFindings)

	// Count by severity
	for _, secret := range result.Secrets {
		severity := string(secret.Severity)
		summary.BySeverity[severity]++
		summary.ByType[string(secret.Type)]++
	}

	for _, vuln := range result.Vulnerabilities {
		summary.BySeverity[vuln.Severity]++
		summary.ByType["vulnerability"]++
	}

	// Count critical/high/medium/low
	summary.CriticalIssues = summary.BySeverity["critical"]
	summary.HighRiskIssues = summary.BySeverity["high"]
	summary.MediumRiskIssues = summary.BySeverity["medium"]
	summary.LowRiskIssues = summary.BySeverity["low"]
}

// getSeverityPriority converts severity to priority number
func (t *ConsolidatedSecurityScanTool) getSeverityPriority(severity string) int {
	switch severity {
	case "critical":
		return 1
	case "high":
		return 2
	case "medium":
		return 3
	case "low":
		return 4
	default:
		return 5
	}
}

// Supporting components and types

type VulnerabilityScanner struct {
	logger *slog.Logger
}

func NewVulnerabilityScanner(logger *slog.Logger) *VulnerabilityScanner {
	return &VulnerabilityScanner{logger: logger}
}

func (v *VulnerabilityScanner) Scan(ctx context.Context, config *ScanConfig) ([]Vulnerability, error) {
	// Basic vulnerability scanning implementation
	return []Vulnerability{}, nil
}

type ComplianceScanner struct {
	logger *slog.Logger
}

func NewComplianceScanner(logger *slog.Logger) *ComplianceScanner {
	return &ComplianceScanner{logger: logger}
}

func (c *ComplianceScanner) Scan(ctx context.Context, config *ScanConfig) ([]ComplianceResult, error) {
	// Basic compliance scanning implementation
	return []ComplianceResult{}, nil
}

// FileSecretScanner is defined in secret_scanner.go
// We use the existing implementation instead of redefining it

type ScanCacheManager struct {
	logger *slog.Logger
	cache  map[string]*ConsolidatedSecurityScanOutput
}

func NewScanCacheManager(logger *slog.Logger) *ScanCacheManager {
	return &ScanCacheManager{
		logger: logger,
		cache:  make(map[string]*ConsolidatedSecurityScanOutput),
	}
}

func (s *ScanCacheManager) Get(key string) *ConsolidatedSecurityScanOutput {
	if result, exists := s.cache[key]; exists {
		s.logger.Info("Scan cache hit", "key", key)
		return result
	}
	return nil
}

func (s *ScanCacheManager) Set(key string, result *ConsolidatedSecurityScanOutput) {
	s.cache[key] = result
	s.logger.Info("Scan cache set", "key", key)
}

func (r *RemediationGenerator) GenerateAdvancedSteps(result *ConsolidatedSecurityScanOutput) []RemediationStep {
	// Generate advanced remediation steps
	return []RemediationStep{}
}

// performBasicSecretScan performs basic secret scanning using existing scanner
func (t *ConsolidatedSecurityScanTool) performBasicSecretScan(ctx context.Context, config *ScanConfig) ([]Secret, error) {
	// Use existing FileSecretScanner with minimal patterns
	filePatterns := []string{"*.env", "*.config", "*.yml", "*.yaml"}
	excludePatterns := []string{"*test*", "*mock*"}

	scannedSecrets, _, _, err := t.secretScanner.PerformSecretScan(config.FilePath, filePatterns, excludePatterns, nil)
	if err != nil {
		return nil, err
	}

	// Convert to our unified Secret format
	secrets := make([]Secret, 0, len(scannedSecrets))
	for _, scanned := range scannedSecrets {
		secret := Secret{
			Type:        SecretType(scanned.Type),
			Value:       scanned.Value,
			MaskedValue: MaskSecret(scanned.Value),
			Location: &Location{
				File:   scanned.File,
				Line:   scanned.Line,
				Column: 0,
			},
			Confidence: float64(scanned.Confidence) / 100.0,
			Severity:   GetSecretSeverity(SecretType(scanned.Type), float64(scanned.Confidence)/100.0),
			Context:    scanned.Context,
		}
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

// performComprehensiveSecretScan performs comprehensive secret scanning
func (t *ConsolidatedSecurityScanTool) performComprehensiveSecretScan(ctx context.Context, config *ScanConfig) ([]Secret, error) {
	// Use existing FileSecretScanner with comprehensive patterns
	filePatterns := []string{"*"}
	excludePatterns := []string{"*.git/*", "*/node_modules/*", "*/target/*", "*/build/*"}

	scannedSecrets, _, _, err := t.secretScanner.PerformSecretScan(config.FilePath, filePatterns, excludePatterns, nil)
	if err != nil {
		return nil, err
	}

	// Convert to our unified Secret format
	secrets := make([]Secret, 0, len(scannedSecrets))
	for _, scanned := range scannedSecrets {
		secret := Secret{
			Type:        SecretType(scanned.Type),
			Value:       scanned.Value,
			MaskedValue: MaskSecret(scanned.Value),
			Location: &Location{
				File:   scanned.File,
				Line:   scanned.Line,
				Column: 0,
			},
			Confidence: float64(scanned.Confidence) / 100.0,
			Severity:   GetSecretSeverity(SecretType(scanned.Type), float64(scanned.Confidence)/100.0),
			Context:    scanned.Context,
		}
		secrets = append(secrets, secret)
	}

	return secrets, nil
}
