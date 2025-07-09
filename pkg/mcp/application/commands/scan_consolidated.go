package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/security"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// ConsolidatedScanCommand consolidates all scan tool functionality into a single command
// This replaces the 33 files in pkg/mcp/tools/scan/ with a unified implementation
type ConsolidatedScanCommand struct {
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	dockerClient    services.DockerClient
	secretDiscovery *security.SecretDiscovery
	logger          *slog.Logger
}

// NewConsolidatedScanCommand creates a new consolidated scan command
func NewConsolidatedScanCommand(
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	dockerClient services.DockerClient,
	logger *slog.Logger,
) *ConsolidatedScanCommand {
	// Initialize security services
	// Convert slog to zerolog for compatibility
	zlog := zerolog.New(os.Stderr).With().Timestamp().Logger()
	secretDiscovery := security.NewSecretDiscovery(zlog)

	return &ConsolidatedScanCommand{
		sessionStore:    sessionStore,
		sessionState:    sessionState,
		dockerClient:    dockerClient,
		secretDiscovery: secretDiscovery,
		logger:          logger,
	}
}

// Execute performs scan operations with full functionality from original tools
func (cmd *ConsolidatedScanCommand) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Extract and validate input parameters
	scanRequest, err := cmd.parseScanInput(input)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("failed to parse scan input").
			Cause(err).
			Build()
	}

	// Validate using domain rules
	if validationErrors := cmd.validateScanRequest(scanRequest); len(validationErrors) > 0 {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("scan request validation failed").
			Context("validation_errors", validationErrors).
			Build()
	}

	// Get workspace directory for the session
	workspaceDir, err := cmd.getSessionWorkspace(scanRequest.SessionID)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("failed to get session workspace").
			Cause(err).
			Build()
	}

	// Execute scan operation based on scan type
	var scanResult *ConsolidatedScanResult
	switch scanRequest.ScanType {
	case "image_security":
		scanResult, err = cmd.executeScanImageSecurity(ctx, scanRequest, workspaceDir)
	case "secrets":
		scanResult, err = cmd.executeScanSecrets(ctx, scanRequest, workspaceDir)
	case "vulnerabilities":
		scanResult, err = cmd.executeScanVulnerabilities(ctx, scanRequest, workspaceDir)
	case "combined":
		scanResult, err = cmd.executeCombinedScan(ctx, scanRequest, workspaceDir)
	default:
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message(fmt.Sprintf("unsupported scan type: %s", scanRequest.ScanType)).
			Build()
	}

	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("scan operation failed").
			Cause(err).
			Build()
	}

	// Update session state with scan results
	if err := cmd.updateSessionState(scanRequest.SessionID, scanResult); err != nil {
		cmd.logger.Warn("failed to update session state", "error", err)
	}

	// Create consolidated response
	response := cmd.createScanResponse(scanResult, time.Since(startTime))

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"scan_result": response,
		},
	}, nil
}

// parseScanInput extracts and validates scan parameters from tool input
func (cmd *ConsolidatedScanCommand) parseScanInput(input api.ToolInput) (*ScanRequest, error) {
	// Extract scan type
	scanType := getStringParam(input.Data, "scan_type", "image_security")

	// Extract common parameters
	request := &ScanRequest{
		SessionID:         input.SessionID,
		ScanType:          scanType,
		Target:            getStringParam(input.Data, "target", ""),
		ImageRef:          getStringParam(input.Data, "image_ref", ""),
		Path:              getStringParam(input.Data, "path", ""),
		SeverityThreshold: getStringParam(input.Data, "severity_threshold", "medium"),
		ScanOptions: ScanOptions{
			IncludeSecrets:      getBoolParam(input.Data, "include_secrets", true),
			IncludeVulns:        getBoolParam(input.Data, "include_vulnerabilities", true),
			IncludeCompliance:   getBoolParam(input.Data, "include_compliance", true),
			IncludeRemediations: getBoolParam(input.Data, "include_remediations", true),
			MaxResults:          getIntParam(input.Data, "max_results", 0),
			FailOnCritical:      getBoolParam(input.Data, "fail_on_critical", false),
			GenerateReport:      getBoolParam(input.Data, "generate_report", false),
			OutputFormat:        getStringParam(input.Data, "output_format", "json"),
			Timeout:             getDurationParam(input.Data, "timeout", 10*time.Minute),
			FilePatterns:        getStringArrayParam(input.Data, "file_patterns"),
			ExcludePatterns:     getStringArrayParam(input.Data, "exclude_patterns"),
			ScanDepth:           getIntParam(input.Data, "scan_depth", 3),
			CustomRules:         getStringArrayParam(input.Data, "custom_rules"),
		},
		CreatedAt: time.Now(),
	}

	// Validate required fields based on scan type
	if err := cmd.validateScanTypeParams(request); err != nil {
		return nil, err
	}

	return request, nil
}

// validateScanTypeParams validates scan type-specific parameters
func (cmd *ConsolidatedScanCommand) validateScanTypeParams(request *ScanRequest) error {
	switch request.ScanType {
	case "image_security":
		if request.ImageRef == "" {
			return fmt.Errorf("image_ref is required for image_security scan")
		}
	case "secrets":
		if request.Path == "" {
			return fmt.Errorf("path is required for secrets scan")
		}
	case "vulnerabilities":
		if request.Target == "" {
			return fmt.Errorf("target is required for vulnerabilities scan")
		}
	case "combined":
		if request.ImageRef == "" && request.Path == "" {
			return fmt.Errorf("either image_ref or path is required for combined scan")
		}
	}
	return nil
}

// validateScanRequest validates scan request using domain rules
func (cmd *ConsolidatedScanCommand) validateScanRequest(request *ScanRequest) []ValidationError {
	var errors []ValidationError

	// Session ID validation
	if request.SessionID == "" {
		errors = append(errors, ValidationError{
			Field:   "session_id",
			Message: "session ID is required",
			Code:    "MISSING_SESSION_ID",
		})
	}

	// Scan type validation
	validScanTypes := []string{"image_security", "secrets", "vulnerabilities", "combined"}
	if !slices.Contains(validScanTypes, request.ScanType) {
		errors = append(errors, ValidationError{
			Field:   "scan_type",
			Message: fmt.Sprintf("scan type must be one of: %s", strings.Join(validScanTypes, ", ")),
			Code:    "INVALID_SCAN_TYPE",
		})
	}

	// Severity threshold validation
	validSeverities := []string{"low", "medium", "high", "critical"}
	if !slices.Contains(validSeverities, request.SeverityThreshold) {
		errors = append(errors, ValidationError{
			Field:   "severity_threshold",
			Message: fmt.Sprintf("severity threshold must be one of: %s", strings.Join(validSeverities, ", ")),
			Code:    "INVALID_SEVERITY",
		})
	}

	// Image reference validation (if provided)
	if request.ImageRef != "" {
		if !isValidImageRef(request.ImageRef) {
			errors = append(errors, ValidationError{
				Field:   "image_ref",
				Message: "invalid image reference format",
				Code:    "INVALID_IMAGE_REF",
			})
		}
	}

	// Path validation (if provided)
	if request.Path != "" {
		if !isValidPath(request.Path) {
			errors = append(errors, ValidationError{
				Field:   "path",
				Message: "invalid path format",
				Code:    "INVALID_PATH",
			})
		}
	}

	return errors
}

// getSessionWorkspace retrieves the workspace directory for a session
func (cmd *ConsolidatedScanCommand) getSessionWorkspace(sessionID string) (string, error) {
	sessionMetadata, err := cmd.sessionState.GetSessionMetadata(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session metadata: %w", err)
	}

	workspaceDir, ok := sessionMetadata["workspace_dir"].(string)
	if !ok || workspaceDir == "" {
		return "", fmt.Errorf("workspace directory not found for session %s", sessionID)
	}

	return workspaceDir, nil
}

// executeScanImageSecurity performs image security scanning operation
func (cmd *ConsolidatedScanCommand) executeScanImageSecurity(ctx context.Context, request *ScanRequest, workspaceDir string) (*ConsolidatedScanResult, error) {
	// Perform image security scan using Docker client
	scanResult, err := cmd.performImageSecurityScan(ctx, request.ImageRef, request.ScanOptions)
	if err != nil {
		return nil, fmt.Errorf("image security scan failed: %w", err)
	}

	// Create consolidated result
	result := &ConsolidatedScanResult{
		ScanID:             fmt.Sprintf("image-scan-%d", time.Now().Unix()),
		SessionID:          request.SessionID,
		ScanType:           request.ScanType,
		Target:             request.ImageRef,
		Status:             "completed",
		SecurityScanResult: scanResult,
		CreatedAt:          time.Now(),
	}

	return result, nil
}

// executeScanSecrets performs secret scanning operation
func (cmd *ConsolidatedScanCommand) executeScanSecrets(ctx context.Context, request *ScanRequest, workspaceDir string) (*ConsolidatedScanResult, error) {
	// Perform secrets scan using security discovery
	scanResult, err := cmd.performSecretsscan(ctx, request.Path, request.ScanOptions)
	if err != nil {
		return nil, fmt.Errorf("secrets scan failed: %w", err)
	}

	// Create consolidated result
	result := &ConsolidatedScanResult{
		ScanID:            fmt.Sprintf("secrets-scan-%d", time.Now().Unix()),
		SessionID:         request.SessionID,
		ScanType:          request.ScanType,
		Target:            request.Path,
		Status:            "completed",
		SecretsScanResult: scanResult,
		CreatedAt:         time.Now(),
	}

	return result, nil
}

// executeScanVulnerabilities performs vulnerability scanning operation
func (cmd *ConsolidatedScanCommand) executeScanVulnerabilities(ctx context.Context, request *ScanRequest, workspaceDir string) (*ConsolidatedScanResult, error) {
	// Perform vulnerability scan
	scanResult, err := cmd.performVulnerabilityyScan(ctx, request.Target, request.ScanOptions)
	if err != nil {
		return nil, fmt.Errorf("vulnerability scan failed: %w", err)
	}

	// Create consolidated result
	result := &ConsolidatedScanResult{
		ScanID:         fmt.Sprintf("vuln-scan-%d", time.Now().Unix()),
		SessionID:      request.SessionID,
		ScanType:       request.ScanType,
		Target:         request.Target,
		Status:         "completed",
		VulnScanResult: scanResult,
		CreatedAt:      time.Now(),
	}

	return result, nil
}

// executeCombinedScan performs combined scanning operation
func (cmd *ConsolidatedScanCommand) executeCombinedScan(ctx context.Context, request *ScanRequest, workspaceDir string) (*ConsolidatedScanResult, error) {
	// Perform combined scan
	result := &ConsolidatedScanResult{
		ScanID:    fmt.Sprintf("combined-scan-%d", time.Now().Unix()),
		SessionID: request.SessionID,
		ScanType:  request.ScanType,
		Target:    request.Target,
		Status:    "in_progress",
		CreatedAt: time.Now(),
	}

	// Run image security scan if image ref provided
	if request.ImageRef != "" {
		securityResult, err := cmd.performImageSecurityScan(ctx, request.ImageRef, request.ScanOptions)
		if err != nil {
			cmd.logger.Warn("image security scan failed in combined scan", "error", err)
		} else {
			result.SecurityScanResult = securityResult
		}
	}

	// Run secrets scan if path provided
	if request.Path != "" {
		secretsResult, err := cmd.performSecretsscan(ctx, request.Path, request.ScanOptions)
		if err != nil {
			cmd.logger.Warn("secrets scan failed in combined scan", "error", err)
		} else {
			result.SecretsScanResult = secretsResult
		}
	}

	// Run vulnerability scan if target provided
	if request.Target != "" {
		vulnResult, err := cmd.performVulnerabilityyScan(ctx, request.Target, request.ScanOptions)
		if err != nil {
			cmd.logger.Warn("vulnerability scan failed in combined scan", "error", err)
		} else {
			result.VulnScanResult = vulnResult
		}
	}

	result.Status = "completed"
	return result, nil
}

// updateSessionState updates session state with scan results
func (cmd *ConsolidatedScanCommand) updateSessionState(sessionID string, result *ConsolidatedScanResult) error {
	// Update session state with scan results
	stateUpdate := map[string]interface{}{
		"last_scan":     result,
		"scan_time":     time.Now(),
		"scan_success":  result.Status == "completed",
		"scan_type":     result.ScanType,
		"scan_target":   result.Target,
		"scan_duration": result.Duration,
	}

	return cmd.sessionState.UpdateSessionData(sessionID, stateUpdate)
}

// createScanResponse creates the final scan response
func (cmd *ConsolidatedScanCommand) createScanResponse(result *ConsolidatedScanResult, duration time.Duration) *ConsolidatedScanResponse {
	response := &ConsolidatedScanResponse{
		Success:       result.Status == "completed",
		ScanID:        result.ScanID,
		ScanType:      result.ScanType,
		Target:        result.Target,
		Status:        result.Status,
		Duration:      result.Duration,
		Error:         result.Error,
		TotalDuration: duration,
		Metadata:      convertScanMetadata(result.Metadata),
	}

	// Add scan-specific results
	if result.SecurityScanResult != nil {
		response.SecurityScan = convertSecurityScanResult(result.SecurityScanResult)
	}

	if result.SecretsScanResult != nil {
		response.SecretsScan = convertSecretsScanResult(result.SecretsScanResult)
	}

	if result.VulnScanResult != nil {
		response.VulnScan = convertVulnScanResult(result.VulnScanResult)
	}

	return response
}

// Tool registration for consolidated scan command
func (cmd *ConsolidatedScanCommand) Name() string {
	return "scan_security"
}

func (cmd *ConsolidatedScanCommand) Description() string {
	return "Comprehensive security scanning tool that consolidates all scanning capabilities"
}

func (cmd *ConsolidatedScanCommand) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        cmd.Name(),
		Description: cmd.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"scan_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of security scan to perform",
					"enum":        []string{"image_security", "secrets", "vulnerabilities", "combined"},
					"default":     "image_security",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Target to scan (generic target)",
				},
				"image_ref": map[string]interface{}{
					"type":        "string",
					"description": "Docker image reference to scan",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File system path to scan",
				},
				"severity_threshold": map[string]interface{}{
					"type":        "string",
					"description": "Minimum severity threshold",
					"enum":        []string{"low", "medium", "high", "critical"},
					"default":     "medium",
				},
				"include_secrets": map[string]interface{}{
					"type":        "boolean",
					"description": "Include secret scanning",
					"default":     true,
				},
				"include_vulnerabilities": map[string]interface{}{
					"type":        "boolean",
					"description": "Include vulnerability scanning",
					"default":     true,
				},
				"include_compliance": map[string]interface{}{
					"type":        "boolean",
					"description": "Include compliance checks",
					"default":     true,
				},
				"include_remediations": map[string]interface{}{
					"type":        "boolean",
					"description": "Include remediation suggestions",
					"default":     true,
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return",
					"default":     0,
					"minimum":     0,
				},
				"fail_on_critical": map[string]interface{}{
					"type":        "boolean",
					"description": "Fail scan on critical findings",
					"default":     false,
				},
				"generate_report": map[string]interface{}{
					"type":        "boolean",
					"description": "Generate detailed report",
					"default":     false,
				},
				"output_format": map[string]interface{}{
					"type":        "string",
					"description": "Output format",
					"enum":        []string{"json", "yaml", "text", "sarif"},
					"default":     "json",
				},
				"timeout": map[string]interface{}{
					"type":        "string",
					"description": "Scan timeout duration (e.g., '10m', '600s')",
					"default":     "10m",
				},
				"file_patterns": map[string]interface{}{
					"type":        "array",
					"description": "File patterns to include in scan",
					"items":       map[string]interface{}{"type": "string"},
				},
				"exclude_patterns": map[string]interface{}{
					"type":        "array",
					"description": "File patterns to exclude from scan",
					"items":       map[string]interface{}{"type": "string"},
				},
				"scan_depth": map[string]interface{}{
					"type":        "integer",
					"description": "Directory scan depth",
					"default":     3,
					"minimum":     1,
					"maximum":     10,
				},
				"custom_rules": map[string]interface{}{
					"type":        "array",
					"description": "Custom scanning rules",
					"items":       map[string]interface{}{"type": "string"},
				},
			},
			"required": []string{"scan_type"},
		},
		Tags:     []string{"scan", "security", "vulnerability", "secrets"},
		Category: api.CategoryScan,
	}
}

// Helper types for consolidated scan functionality

// ScanRequest represents a consolidated scan request
type ScanRequest struct {
	SessionID         string      `json:"session_id"`
	ScanType          string      `json:"scan_type"`
	Target            string      `json:"target"`
	ImageRef          string      `json:"image_ref"`
	Path              string      `json:"path"`
	SeverityThreshold string      `json:"severity_threshold"`
	ScanOptions       ScanOptions `json:"scan_options"`
	CreatedAt         time.Time   `json:"created_at"`
}

// ScanOptions contains scan configuration options
type ScanOptions struct {
	IncludeSecrets      bool          `json:"include_secrets"`
	IncludeVulns        bool          `json:"include_vulnerabilities"`
	IncludeCompliance   bool          `json:"include_compliance"`
	IncludeRemediations bool          `json:"include_remediations"`
	MaxResults          int           `json:"max_results"`
	FailOnCritical      bool          `json:"fail_on_critical"`
	GenerateReport      bool          `json:"generate_report"`
	OutputFormat        string        `json:"output_format"`
	Timeout             time.Duration `json:"timeout"`
	FilePatterns        []string      `json:"file_patterns"`
	ExcludePatterns     []string      `json:"exclude_patterns"`
	ScanDepth           int           `json:"scan_depth"`
	CustomRules         []string      `json:"custom_rules"`
}

// ConsolidatedScanResult represents the consolidated scan result
type ConsolidatedScanResult struct {
	ScanID             string                   `json:"scan_id"`
	SessionID          string                   `json:"session_id"`
	ScanType           string                   `json:"scan_type"`
	Target             string                   `json:"target"`
	Status             string                   `json:"status"`
	SecurityScanResult *SecurityScanResult      `json:"security_scan_result,omitempty"`
	SecretsScanResult  *SecretsScanResult       `json:"secrets_scan_result,omitempty"`
	VulnScanResult     *VulnerabilityScanResult `json:"vulnerability_scan_result,omitempty"`
	Duration           time.Duration            `json:"duration"`
	Error              string                   `json:"error,omitempty"`
	CreatedAt          time.Time                `json:"created_at"`
	Metadata           map[string]interface{}   `json:"metadata"`
}

// SecurityScanResult represents security scan results
type SecurityScanResult struct {
	ImageRef        string                   `json:"image_ref"`
	Vulnerabilities []VulnerabilityInfo      `json:"vulnerabilities"`
	Secrets         []SecretInfo             `json:"secrets"`
	Compliance      ComplianceInfo           `json:"compliance"`
	SecurityScore   int                      `json:"security_score"`
	RiskLevel       string                   `json:"risk_level"`
	Recommendations []SecurityRecommendation `json:"recommendations"`
	RemediationPlan *RemediationPlan         `json:"remediation_plan,omitempty"`
}

// SecretsScanResult represents secrets scan results
type SecretsScanResult struct {
	Path          string         `json:"path"`
	Secrets       []SecretInfo   `json:"secrets"`
	FilesScanned  int            `json:"files_scanned"`
	SecretsFound  int            `json:"secrets_found"`
	HighRiskCount int            `json:"high_risk_count"`
	Summary       SecretsSummary `json:"summary"`
}

// VulnerabilityScanResult represents vulnerability scan results
type VulnerabilityScanResult struct {
	Target          string               `json:"target"`
	Vulnerabilities []VulnerabilityInfo  `json:"vulnerabilities"`
	Summary         VulnerabilitySummary `json:"summary"`
	CriticalCount   int                  `json:"critical_count"`
	HighCount       int                  `json:"high_count"`
	MediumCount     int                  `json:"medium_count"`
	LowCount        int                  `json:"low_count"`
}

// ConsolidatedScanResponse represents the consolidated scan response
type ConsolidatedScanResponse struct {
	Success       bool                   `json:"success"`
	ScanID        string                 `json:"scan_id"`
	ScanType      string                 `json:"scan_type"`
	Target        string                 `json:"target"`
	Status        string                 `json:"status"`
	SecurityScan  *SecurityScanInfo      `json:"security_scan,omitempty"`
	SecretsScan   *SecretsScanInfo       `json:"secrets_scan,omitempty"`
	VulnScan      *VulnerabilityScanInfo `json:"vulnerability_scan,omitempty"`
	Duration      time.Duration          `json:"duration"`
	Error         string                 `json:"error,omitempty"`
	TotalDuration time.Duration          `json:"total_duration"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// SecurityScanInfo represents security scan information
type SecurityScanInfo struct {
	ImageRef        string                   `json:"image_ref"`
	VulnCount       int                      `json:"vulnerability_count"`
	SecretsCount    int                      `json:"secrets_count"`
	SecurityScore   int                      `json:"security_score"`
	RiskLevel       string                   `json:"risk_level"`
	Recommendations []SecurityRecommendation `json:"recommendations"`
}

// SecretsScanInfo represents secrets scan information
type SecretsScanInfo struct {
	Path          string `json:"path"`
	FilesScanned  int    `json:"files_scanned"`
	SecretsFound  int    `json:"secrets_found"`
	HighRiskCount int    `json:"high_risk_count"`
	RiskLevel     string `json:"risk_level"`
}

// VulnerabilityScanInfo represents vulnerability scan information
type VulnerabilityScanInfo struct {
	Target        string `json:"target"`
	VulnCount     int    `json:"vulnerability_count"`
	CriticalCount int    `json:"critical_count"`
	HighCount     int    `json:"high_count"`
	MediumCount   int    `json:"medium_count"`
	LowCount      int    `json:"low_count"`
	RiskLevel     string `json:"risk_level"`
}

// VulnerabilityInfo represents vulnerability information
type VulnerabilityInfo struct {
	ID          string                 `json:"id"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Package     string                 `json:"package"`
	Version     string                 `json:"version"`
	FixedIn     string                 `json:"fixed_in"`
	CVSS        float64                `json:"cvss"`
	References  []string               `json:"references"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SecretInfo represents secret information
type SecretInfo struct {
	Type       string                 `json:"type"`
	File       string                 `json:"file"`
	Line       int                    `json:"line"`
	Pattern    string                 `json:"pattern"`
	Value      string                 `json:"value"`
	Severity   string                 `json:"severity"`
	Confidence float64                `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ComplianceInfo represents compliance information
type ComplianceInfo struct {
	Framework string            `json:"framework"`
	Passed    bool              `json:"passed"`
	Score     float64           `json:"score"`
	Checks    []ComplianceCheck `json:"checks"`
}

// ComplianceCheck represents a compliance check
type ComplianceCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// SecurityRecommendation represents a security recommendation
type SecurityRecommendation struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
}

// RemediationPlan represents a remediation plan
type RemediationPlan struct {
	ID        string            `json:"id"`
	Priority  string            `json:"priority"`
	Effort    string            `json:"effort"`
	Steps     []RemediationStep `json:"steps"`
	Estimated time.Duration     `json:"estimated_time"`
}

// RemediationStep represents a remediation step
type RemediationStep struct {
	ID          string `json:"id"`
	Order       int    `json:"order"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Automated   bool   `json:"automated"`
}

// SecretsSummary represents secrets scan summary
type SecretsSummary struct {
	TotalSecrets      int                    `json:"total_secrets"`
	ByType            map[string]int         `json:"by_type"`
	BySeverity        map[string]int         `json:"by_severity"`
	ByFile            map[string]int         `json:"by_file"`
	ConfidenceAverage float64                `json:"confidence_average"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// VulnerabilitySummary represents vulnerability scan summary
type VulnerabilitySummary struct {
	TotalVulns   int                    `json:"total_vulnerabilities"`
	BySeverity   map[string]int         `json:"by_severity"`
	ByPackage    map[string]int         `json:"by_package"`
	FixableCount int                    `json:"fixable_count"`
	AgeAnalysis  AgeAnalysis            `json:"age_analysis"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// AgeAnalysis represents vulnerability age analysis
type AgeAnalysis struct {
	AverageAge   time.Duration  `json:"average_age"`
	OldestVuln   time.Duration  `json:"oldest_vulnerability"`
	NewestVuln   time.Duration  `json:"newest_vulnerability"`
	Distribution map[string]int `json:"distribution"`
}

// Helper functions for scan operations

// isValidImageRef validates Docker image reference format
func isValidImageRef(imageRef string) bool {
	// Basic validation - can be enhanced with full Docker naming rules
	if imageRef == "" || len(imageRef) > 255 {
		return false
	}

	// Check for invalid characters
	for _, char := range imageRef {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-' ||
			char == '_' || char == '/' || char == ':') {
			return false
		}
	}

	return true
}

// isValidPath validates file system path format
func isValidPath(path string) bool {
	// Basic path validation
	if path == "" {
		return false
	}

	// Check for valid absolute or relative path
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, "../") {
		// Relative path without explicit relative prefix
		if !strings.Contains(path, "..") {
			return true
		}
	}

	return filepath.IsAbs(path)
}

// Note: getStringArrayParam is defined in commands.go

// convertScanMetadata converts scan metadata to response format
func convertScanMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return make(map[string]interface{})
	}
	return metadata
}

// convertSecurityScanResult converts security scan result to response format
func convertSecurityScanResult(result *SecurityScanResult) *SecurityScanInfo {
	if result == nil {
		return nil
	}

	return &SecurityScanInfo{
		ImageRef:        result.ImageRef,
		VulnCount:       len(result.Vulnerabilities),
		SecretsCount:    len(result.Secrets),
		SecurityScore:   result.SecurityScore,
		RiskLevel:       result.RiskLevel,
		Recommendations: result.Recommendations,
	}
}

// convertSecretsScanResult converts secrets scan result to response format
func convertSecretsScanResult(result *SecretsScanResult) *SecretsScanInfo {
	if result == nil {
		return nil
	}

	return &SecretsScanInfo{
		Path:          result.Path,
		FilesScanned:  result.FilesScanned,
		SecretsFound:  result.SecretsFound,
		HighRiskCount: result.HighRiskCount,
		RiskLevel:     determineSecretsRiskLevel(result.HighRiskCount, result.SecretsFound),
	}
}

// convertVulnScanResult converts vulnerability scan result to response format
func convertVulnScanResult(result *VulnerabilityScanResult) *VulnerabilityScanInfo {
	if result == nil {
		return nil
	}

	return &VulnerabilityScanInfo{
		Target:        result.Target,
		VulnCount:     len(result.Vulnerabilities),
		CriticalCount: result.CriticalCount,
		HighCount:     result.HighCount,
		MediumCount:   result.MediumCount,
		LowCount:      result.LowCount,
		RiskLevel:     determineVulnRiskLevel(result.CriticalCount, result.HighCount, result.MediumCount),
	}
}

// determineSecretsRiskLevel determines risk level based on secrets findings
func determineSecretsRiskLevel(highRiskCount, totalSecrets int) string {
	if highRiskCount > 0 {
		return "high"
	}
	if totalSecrets > 10 {
		return "medium"
	}
	if totalSecrets > 0 {
		return "low"
	}
	return "none"
}

// determineVulnRiskLevel determines risk level based on vulnerability counts
func determineVulnRiskLevel(critical, high, medium int) string {
	if critical > 0 {
		return "critical"
	}
	if high > 0 {
		return "high"
	}
	if medium > 0 {
		return "medium"
	}
	return "low"
}
