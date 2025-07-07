package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/tools"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// ScanTool implements the canonical api.Tool interface using services
type ScanTool struct {
	sessionStore services.SessionStore
	sessionState services.SessionState
	scanner      services.Scanner
	logger       *slog.Logger
	scanEngine   ScanEngineExtended
	metrics      *SecurityMetrics

	// Optional client dependencies
	dockerClient   DockerClient   // Optional docker client for image validation
	securityClient SecurityClient // Optional security client for database updates

	// TypeSafe compatibility fields
	timeout    time.Duration
	atomicTool *AtomicScanImageSecurityTool
}

// DockerClient interface for docker operations
type DockerClient interface {
	ImageExists(ctx context.Context, imageName string) (bool, error)
	PullImage(ctx context.Context, imageName string) error
}

// SecurityClient interface for security database operations
type SecurityClient interface {
	UpdateDatabase(ctx context.Context) error
}

// NewScanTool creates a new scan tool using service container
func NewScanTool(container services.ServiceContainer, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "scan_security")

	// Create atomic tool for TypeSafe compatibility
	atomicTool := NewAtomicScanImageSecurityTool(nil, container, toolLogger)

	return &ScanTool{
		sessionStore: container.SessionStore(),
		sessionState: container.SessionState(),
		scanner:      container.Scanner(),
		logger:       toolLogger,
		scanEngine:   NewScanEngineExtended(toolLogger),
		metrics:      NewSecurityMetrics(),

		// TypeSafe compatibility
		timeout:    15 * time.Minute,
		atomicTool: atomicTool,
	}
}

// NewScanToolLegacy creates a new scan tool using legacy session manager (backward compatibility)
func NewScanToolLegacy(sessionManager session.UnifiedSessionManager, logger *slog.Logger) api.Tool {
	// For backward compatibility, we'll create a minimal service container wrapper
	// This allows existing code to continue working while new code uses services
	toolLogger := logger.With("tool", "scan_security_legacy")

	return &ScanTool{
		sessionStore: nil, // Legacy mode - no services
		sessionState: nil,
		scanner:      nil,
		logger:       toolLogger,
		scanEngine:   NewScanEngineExtended(toolLogger),
		metrics:      NewSecurityMetrics(),
		timeout:      15 * time.Minute,
		atomicTool:   nil,
	}
}

// Name implements api.Tool
func (t *ScanTool) Name() string {
	return "scan_security"
}

// Description implements api.Tool
func (t *ScanTool) Description() string {
	return "Performs atomic security scanning of Docker images with vulnerability detection, secret scanning, and compliance checks"
}

// Category implements api.Tool
func (t *ScanTool) Category() string {
	return "scan"
}

// Tags implements api.Tool
func (t *ScanTool) Tags() []string {
	return []string{"security", "vulnerability", "secrets", "scanning", "compliance"}
}

// Version implements api.Tool
func (t *ScanTool) Version() string {
	return "1.0.0"
}

// Schema implements api.Tool
func (t *ScanTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "scan_security",
		Description: "Scan Docker images and containers for security vulnerabilities, secrets, and compliance issues",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking the scan workflow",
				},
				"data": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"image_name": map[string]interface{}{
							"type":        "string",
							"description": "Docker image name or reference to scan",
							"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._/-]*([:].*)?$",
						},
						"scan_types": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
								"enum": []string{"vulnerability", "secrets", "malware", "compliance"},
							},
							"description": "Types of scans to perform (default: [vulnerability, secrets])",
						},
						"severity_filter": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
								"enum": []string{"critical", "high", "medium", "low"},
							},
							"description": "Severity levels to include in results (default: all)",
						},
						"include_secrets": map[string]interface{}{
							"type":        "boolean",
							"description": "Include secret scanning (default: true)",
						},
						"include_malware": map[string]interface{}{
							"type":        "boolean",
							"description": "Include malware scanning (default: false)",
						},
						"include_compliance": map[string]interface{}{
							"type":        "boolean",
							"description": "Include compliance checking (default: false)",
						},
						"fail_on_critical": map[string]interface{}{
							"type":        "boolean",
							"description": "Fail the scan if critical vulnerabilities are found",
						},
						"fail_on_high": map[string]interface{}{
							"type":        "boolean",
							"description": "Fail the scan if high severity vulnerabilities are found",
						},
						"timeout_seconds": map[string]interface{}{
							"type":        "integer",
							"description": "Scan timeout in seconds (default: 600)",
							"minimum":     30,
							"maximum":     3600,
						},
						"force_rescan": map[string]interface{}{
							"type":        "boolean",
							"description": "Force rescan even if cached results exist",
						},
						"output_format": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"json", "sarif", "cyclonedx"},
							"description": "Output format for scan results (default: json)",
						},
						"database_update": map[string]interface{}{
							"type":        "boolean",
							"description": "Update vulnerability database before scanning (default: true)",
						},
					},
					"required": []string{"image_name"},
				},
			},
			"required": []string{"session_id", "data"},
		},
		Tags:     []string{"scan", "security", "vulnerability", "secrets", "compliance"},
		Category: api.ToolCategory("scan"),
		Version:  "1.0.0",
	}
}

// Execute implements api.Tool
// ScanRequest represents a parsed and validated scan request
type ScanRequest struct {
	SessionID         string
	ImageName         string
	ScanTypes         []string
	SeverityFilter    []string
	IncludeSecrets    bool
	IncludeMalware    bool
	IncludeCompliance bool
	FailOnCritical    bool
	FailOnHigh        bool
	TimeoutSeconds    int
	ForceRescan       bool
	OutputFormat      string
	DatabaseUpdate    bool
}

func (t *ScanTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse and validate input
	request, err := t.parseAndValidateInput(input)
	if err != nil {
		return t.createErrorOutput("Input validation failed", err), err
	}

	// Execute the scan
	result, err := t.executeScan(ctx, request)
	if err != nil {
		return t.createErrorOutput("Scan execution failed", err), err
	}

	// Format and return response
	return t.formatScanResponse(request, result), nil
}

// parseAndValidateInput parses and validates the scan input
func (t *ScanTool) parseAndValidateInput(input api.ToolInput) (*ScanRequest, error) {
	// Parse input JSON
	var params struct {
		SessionID         string   `json:"session_id"`
		ImageName         string   `json:"image_name"`
		ScanTypes         []string `json:"scan_types,omitempty"`
		SeverityFilter    []string `json:"severity_filter,omitempty"`
		IncludeSecrets    bool     `json:"include_secrets,omitempty"`
		IncludeMalware    bool     `json:"include_malware,omitempty"`
		IncludeCompliance bool     `json:"include_compliance,omitempty"`
		FailOnCritical    bool     `json:"fail_on_critical,omitempty"`
		FailOnHigh        bool     `json:"fail_on_high,omitempty"`
		TimeoutSeconds    int      `json:"timeout_seconds,omitempty"`
		ForceRescan       bool     `json:"force_rescan,omitempty"`
		OutputFormat      string   `json:"output_format,omitempty"`
		DatabaseUpdate    bool     `json:"database_update,omitempty"`
	}

	// Extract parameters from input data
	if err := extractParamsFromInput(input.Data, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %v", err)
	}

	// Validate required parameters
	if params.SessionID == "" {
		return nil, errors.NewError().Messagef("session_id is required").WithLocation().Build()
	}

	if params.ImageName == "" {
		return nil, errors.NewError().Messagef("image_name is required").WithLocation().Build()
	}

	// Set defaults and create request
	request := &ScanRequest{
		SessionID:         params.SessionID,
		ImageName:         params.ImageName,
		ScanTypes:         params.ScanTypes,
		SeverityFilter:    params.SeverityFilter,
		IncludeSecrets:    params.IncludeSecrets,
		IncludeMalware:    params.IncludeMalware,
		IncludeCompliance: params.IncludeCompliance,
		FailOnCritical:    params.FailOnCritical,
		FailOnHigh:        params.FailOnHigh,
		TimeoutSeconds:    params.TimeoutSeconds,
		ForceRescan:       params.ForceRescan,
		OutputFormat:      params.OutputFormat,
		DatabaseUpdate:    params.DatabaseUpdate,
	}

	// Set defaults
	if len(request.ScanTypes) == 0 {
		request.ScanTypes = []string{"vulnerability", "secrets"}
	}
	if len(request.SeverityFilter) == 0 {
		request.SeverityFilter = []string{"critical", "high", "medium", "low"}
	}
	if request.TimeoutSeconds == 0 {
		request.TimeoutSeconds = 600
	}
	if request.OutputFormat == "" {
		request.OutputFormat = "json"
	}
	if !request.ForceRescan {
		request.DatabaseUpdate = true // Default to updating unless force rescan
	}

	// Set scan type flags based on array
	for _, scanType := range request.ScanTypes {
		switch scanType {
		case "secrets":
			request.IncludeSecrets = true
		case "malware":
			request.IncludeMalware = true
		case "compliance":
			request.IncludeCompliance = true
		}
	}

	return request, nil
}

// ToolScanResult represents the results of a security scan (renamed to avoid conflict)
type ToolScanResult struct {
	ScanData        interface{}
	FilteredVulns   []ScanVulnerability
	FilteredSecrets []Secret
	Summary         interface{}
	Success         bool
	FailureReason   string
	Session         *api.Session
	StartTime       time.Time
}

// executeScan performs the actual security scan
func (t *ScanTool) executeScan(ctx context.Context, request *ScanRequest) (*ToolScanResult, error) {
	startTime := time.Now()

	// Log the execution
	t.logScanExecution(request)

	// Create or get session using services
	sess, err := t.getOrCreateSession(ctx, request.SessionID)
	if err != nil {
		return nil, fmt.Errorf("session handling failed: %v", err)
	}

	// Validate image exists and is accessible
	if err := t.validateImageAccess(ctx, request.ImageName); err != nil {
		return nil, fmt.Errorf("image validation failed: %v", err)
	}

	// Update vulnerability database if requested
	if request.DatabaseUpdate {
		if err := t.updateVulnerabilityDatabase(ctx); err != nil {
			t.logger.Warn("Failed to update vulnerability database, continuing with existing data", "error", err)
		}
	}

	// Perform the scan
	scanParams := struct {
		SessionID         string   `json:"session_id"`
		ImageName         string   `json:"image_name"`
		ScanTypes         []string `json:"scan_types,omitempty"`
		SeverityFilter    []string `json:"severity_filter,omitempty"`
		IncludeSecrets    bool     `json:"include_secrets,omitempty"`
		IncludeMalware    bool     `json:"include_malware,omitempty"`
		IncludeCompliance bool     `json:"include_compliance,omitempty"`
		FailOnCritical    bool     `json:"fail_on_critical,omitempty"`
		FailOnHigh        bool     `json:"fail_on_high,omitempty"`
		TimeoutSeconds    int      `json:"timeout_seconds,omitempty"`
		ForceRescan       bool     `json:"force_rescan,omitempty"`
		OutputFormat      string   `json:"output_format,omitempty"`
		DatabaseUpdate    bool     `json:"database_update,omitempty"`
	}{
		SessionID:         request.SessionID,
		ImageName:         request.ImageName,
		ScanTypes:         request.ScanTypes,
		SeverityFilter:    request.SeverityFilter,
		IncludeSecrets:    request.IncludeSecrets,
		IncludeMalware:    request.IncludeMalware,
		IncludeCompliance: request.IncludeCompliance,
		FailOnCritical:    request.FailOnCritical,
		FailOnHigh:        request.FailOnHigh,
		TimeoutSeconds:    request.TimeoutSeconds,
		ForceRescan:       request.ForceRescan,
		OutputFormat:      request.OutputFormat,
		DatabaseUpdate:    request.DatabaseUpdate,
	}

	scanData, err := t.performScan(ctx, scanParams)
	if err != nil {
		return nil, fmt.Errorf("scan execution failed: %v", err)
	}

	// Filter results by severity
	filteredVulns := t.filterVulnerabilitiesBySeverity(scanData.GetVulnerabilities(), request.SeverityFilter)
	filteredSecrets := t.filterSecretsBySeverity(scanData.Secrets, request.SeverityFilter)

	// Check failure conditions
	scanSuccess, failureReason := t.checkFailureConditions(request, filteredVulns)

	// Generate summary statistics
	summary := t.generateScanSummary(filteredVulns, filteredSecrets, scanData)

	// Update session state
	t.updateSessionState(ctx, sess, scanSuccess)

	return &ToolScanResult{
		ScanData:        scanData,
		FilteredVulns:   filteredVulns,
		FilteredSecrets: filteredSecrets,
		Summary:         summary,
		Success:         scanSuccess,
		FailureReason:   failureReason,
		Session:         sess,
		StartTime:       startTime,
	}, nil
}

// logScanExecution logs the scan execution details
func (t *ScanTool) logScanExecution(request *ScanRequest) {
	t.logger.Info("Starting security scan",
		"session_id", request.SessionID,
		"image_name", request.ImageName,
		"scan_types", request.ScanTypes,
		"severity_filter", request.SeverityFilter,
		"include_secrets", request.IncludeSecrets,
		"include_malware", request.IncludeMalware,
		"include_compliance", request.IncludeCompliance,
		"fail_on_critical", request.FailOnCritical,
		"fail_on_high", request.FailOnHigh,
		"timeout_seconds", request.TimeoutSeconds,
		"force_rescan", request.ForceRescan)
}

// getOrCreateSession creates or retrieves a session
func (t *ScanTool) getOrCreateSession(ctx context.Context, sessionID string) (*api.Session, error) {
	if t.sessionStore == nil {
		return nil, nil // No session store available
	}

	session, err := t.sessionStore.Get(ctx, sessionID)
	if err != nil {
		// Create new session if it doesn't exist
		newSessionID, createErr := t.sessionStore.Create(ctx, map[string]interface{}{
			"tool_name": "scan_security",
			"scan_type": "comprehensive",
		})
		if createErr != nil {
			return nil, fmt.Errorf("failed to create session: %v", createErr)
		}
		session, err = t.sessionStore.Get(ctx, newSessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get created session: %v", err)
		}
	}
	return session, nil
}

// checkFailureConditions checks if the scan should be marked as failed
func (t *ScanTool) checkFailureConditions(request *ScanRequest, filteredVulns []ScanVulnerability) (bool, string) {
	if request.FailOnCritical && t.hasVulnerabilitiesOfSeverity(filteredVulns, "critical") {
		return false, "Critical vulnerabilities found"
	}
	if request.FailOnHigh && t.hasVulnerabilitiesOfSeverity(filteredVulns, "high") {
		return false, "High severity vulnerabilities found"
	}
	return true, ""
}

// updateSessionState updates the session with scan results
func (t *ScanTool) updateSessionState(ctx context.Context, sess *api.Session, scanSuccess bool) {
	if t.sessionState != nil && sess != nil {
		sessionData := map[string]interface{}{
			"status":    "scan_completed",
			"success":   scanSuccess,
			"scan_time": time.Now(),
		}
		t.sessionState.SaveState(ctx, sess.ID, sessionData)
	}
}

// formatScanResponse formats the scan response
func (t *ScanTool) formatScanResponse(request *ScanRequest, result *ToolScanResult) api.ToolOutput {
	// Create result text
	var resultText string
	if result.Success {
		resultText = fmt.Sprintf("Security scan completed for image: %s", request.ImageName)
	} else {
		resultText = fmt.Sprintf("Security scan failed for image: %s - %s", request.ImageName, result.FailureReason)
	}

	return api.ToolOutput{
		Success: result.Success,
		Data: map[string]interface{}{
			"message":         resultText,
			"session_id":      request.SessionID,
			"image_name":      request.ImageName,
			"scan_types":      request.ScanTypes,
			"success":         result.Success,
			"failure_reason":  result.FailureReason,
			"duration_ms":     int64(time.Since(result.StartTime).Milliseconds()),
			"scan_summary":    result.Summary,
			"vulnerabilities": t.convertVulnerabilities(result.FilteredVulns),
			"secrets":         t.convertSecrets(result.FilteredSecrets),
		},
	}
}

// createErrorOutput creates an error output
func (t *ScanTool) createErrorOutput(message string, err error) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Error:   fmt.Sprintf("%s: %v", message, err),
		Data: map[string]interface{}{
			"error": err.Error(),
		},
	}
}

func (t *ScanTool) validateImageAccess(ctx context.Context, imageName string) error {
	if t.dockerClient == nil {
		return nil // Skip validation if client not available
	}

	// Check if image exists locally or can be pulled
	exists, err := t.dockerClient.ImageExists(ctx, imageName)
	if err != nil {
		return errors.NewError().
			Message("Failed to check image accessibility").
			Cause(err).
			Build()
	}

	if !exists {
		// Try to pull the image
		if err := t.dockerClient.PullImage(ctx, imageName); err != nil {
			return errors.NewError().
				Message("Image not found locally and could not be pulled").
				Cause(err).
				Build()
		}
	}

	return nil
}

// updateVulnerabilityDatabase updates the vulnerability database
func (t *ScanTool) updateVulnerabilityDatabase(ctx context.Context) error {
	if t.securityClient == nil {
		return nil // Skip update if client not available
	}

	return t.securityClient.UpdateDatabase(ctx)
}

// performScan executes the security scan
func (t *ScanTool) performScan(ctx context.Context, params struct {
	SessionID         string   `json:"session_id"`
	ImageName         string   `json:"image_name"`
	ScanTypes         []string `json:"scan_types,omitempty"`
	SeverityFilter    []string `json:"severity_filter,omitempty"`
	IncludeSecrets    bool     `json:"include_secrets,omitempty"`
	IncludeMalware    bool     `json:"include_malware,omitempty"`
	IncludeCompliance bool     `json:"include_compliance,omitempty"`
	FailOnCritical    bool     `json:"fail_on_critical,omitempty"`
	FailOnHigh        bool     `json:"fail_on_high,omitempty"`
	TimeoutSeconds    int      `json:"timeout_seconds,omitempty"`
	ForceRescan       bool     `json:"force_rescan,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	DatabaseUpdate    bool     `json:"database_update,omitempty"`
}) (*EnhancedScanResult, error) {
	if t.scanEngine == nil {
		return nil, errors.NewError().
			Message("Scan engine not available").
			Build()
	}

	scanOptions := ScanOptionsExtended{
		ImageName:         params.ImageName,
		ScanTypes:         params.ScanTypes,
		IncludeSecrets:    params.IncludeSecrets,
		IncludeMalware:    params.IncludeMalware,
		IncludeCompliance: params.IncludeCompliance,
		Timeout:           time.Duration(params.TimeoutSeconds) * time.Second,
		ForceRescan:       params.ForceRescan,
		OutputFormat:      params.OutputFormat,
	}

	scanCtx, cancel := context.WithTimeout(ctx, scanOptions.Timeout)
	defer cancel()

	result, err := t.scanEngine.ScanImage(scanCtx, scanOptions)
	if err != nil {
		return nil, errors.NewError().
			Message("Security scan execution failed").
			Cause(err).
			Build()
	}

	return result, nil
}

// Helper methods for filtering and processing results

func (t *ScanTool) filterVulnerabilitiesBySeverity(vulns []ScanVulnerability, severityFilter []string) []ScanVulnerability {
	if len(severityFilter) == 0 {
		return vulns
	}

	severityMap := make(map[string]bool)
	for _, severity := range severityFilter {
		severityMap[strings.ToLower(severity)] = true
	}

	filtered := make([]ScanVulnerability, 0)
	for _, vuln := range vulns {
		if severityMap[strings.ToLower(vuln.Severity)] {
			filtered = append(filtered, vuln)
		}
	}

	return filtered
}

func (t *ScanTool) filterSecretsBySeverity(secrets []Secret, severityFilter []string) []Secret {
	// Secrets don't have traditional severity, but can have confidence levels
	return secrets
}

func (t *ScanTool) hasVulnerabilitiesOfSeverity(vulns []ScanVulnerability, severity string) bool {
	for _, vuln := range vulns {
		if strings.EqualFold(vuln.Severity, severity) {
			return true
		}
	}
	return false
}

func (t *ScanTool) generateScanSummary(vulns []ScanVulnerability, secrets []Secret, scanResult *EnhancedScanResult) ScanSummary {
	summary := ScanSummary{
		TotalVulnerabilities: len(vulns),
		SecretsFound:         len(secrets),
		ScanDuration:         scanResult.Duration,
		Scanner:              scanResult.Scanner,
		DatabaseVersion:      scanResult.DatabaseVersion,
	}

	// Count by severity
	for _, vuln := range vulns {
		switch strings.ToLower(vuln.Severity) {
		case "critical":
			summary.Critical++
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		case "low":
			summary.Low++
		}
	}

	return summary
}

func (t *ScanTool) convertVulnerabilities(vulns []ScanVulnerability) []map[string]interface{} {
	result := make([]map[string]interface{}, len(vulns))
	for i, vuln := range vulns {
		result[i] = map[string]interface{}{
			"id":            vuln.ID,
			"severity":      vuln.Severity,
			"package":       vuln.Package,
			"version":       vuln.Version,
			"fixed_version": vuln.FixedVersion,
			"description":   vuln.Description,
			"cvss_score":    vuln.CVSSScore,
			"references":    vuln.References,
		}
	}
	return result
}

func (t *ScanTool) convertSecrets(secrets []Secret) []map[string]interface{} {
	result := make([]map[string]interface{}, len(secrets))
	for i, secret := range secrets {
		result[i] = map[string]interface{}{
			"type":       string(secret.Type),
			"file":       getSecretFile(secret),
			"line":       getSecretLine(secret),
			"confidence": secret.Confidence,
			"match":      getSecretMatch(secret),
		}
	}
	return result
}

// Helper functions to safely extract fields from Secret
func getSecretFile(secret Secret) string {
	if secret.Location != nil {
		return secret.Location.File
	}
	return ""
}

func getSecretLine(secret Secret) int {
	if secret.Location != nil {
		return secret.Location.Line
	}
	return 0
}

func getSecretMatch(secret Secret) string {
	if secret.MaskedValue != "" {
		return secret.MaskedValue
	}
	return secret.Pattern
}

func (t *ScanTool) createErrorResult(sessionID, message string, err error, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Data: map[string]interface{}{
			"message":    message + ": " + err.Error(),
			"session_id": sessionID,
			"error":      true,
		},
		Error: message + ": " + err.Error(),
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        sessionID,
			"tool_version":      t.Version(),
			"error":             true,
		},
	}
}

// ScanToolDomainWrapper wraps ScanTool to implement tools.Tool interface
type ScanToolDomainWrapper struct {
	apiTool *ScanTool
}

// ScanParameters represents the input parameters for the scan tool
type ScanParameters struct {
	ImageName         string `json:"image_name"`
	SessionID         string `json:"session_id,omitempty"`
	SeverityThreshold string `json:"severity_threshold,omitempty"`
	IncludeFixable    bool   `json:"include_fixable,omitempty"`
	MaxResults        int    `json:"max_results,omitempty"`
}

// Name implements tools.Tool interface
func (w *ScanToolDomainWrapper) Name() string {
	return w.apiTool.Name()
}

// Description implements tools.Tool interface
func (w *ScanToolDomainWrapper) Description() string {
	return w.apiTool.Description()
}

// Category implements tools.Tool interface
func (w *ScanToolDomainWrapper) Category() string {
	return w.apiTool.Category()
}

// Tags implements tools.Tool interface
func (w *ScanToolDomainWrapper) Tags() []string {
	return []string{"security", "scan", "vulnerability"}
}

// Version implements tools.Tool interface
func (w *ScanToolDomainWrapper) Version() string {
	return "1.0.0"
}

// InputSchema implements tools.Tool interface
func (w *ScanToolDomainWrapper) InputSchema() *json.RawMessage {
	// Return a basic schema for scan parameters
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"image_name": {"type": "string", "description": "Docker image to scan"},
			"session_id": {"type": "string", "description": "Session ID for tracking"},
			"severity_threshold": {"type": "string", "enum": ["LOW", "MEDIUM", "HIGH", "CRITICAL"], "description": "Minimum severity to report"},
			"include_fixable": {"type": "boolean", "description": "Include only fixable vulnerabilities"},
			"max_results": {"type": "integer", "description": "Maximum number of results to return"}
		},
		"required": ["image_name"]
	}`)
	return &schema
}

// Execute implements tools.Tool interface
func (w *ScanToolDomainWrapper) Execute(ctx context.Context, input json.RawMessage) (*tools.ExecutionResult, error) {
	// Parse input to our scan parameters
	var params ScanParameters
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.ExecutionResult{
			Content: []tools.ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Invalid input format: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Convert to internal format and execute
	args := AtomicScanImageSecurityArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: params.SessionID,
		},
		ImageName:         params.ImageName,
		SeverityThreshold: params.SeverityThreshold,
		IncludeFixable:    params.IncludeFixable,
		MaxResults:        params.MaxResults,
	}

	result, err := w.apiTool.atomicTool.ExecuteScan(ctx, args)
	if err != nil {
		return &tools.ExecutionResult{
			Content: []tools.ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Scan failed: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Convert result to tools.ExecutionResult format
	return &tools.ExecutionResult{
		Content: []tools.ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Security scan completed successfully for image: %s", result.ImageName),
			Data: result,
		}},
		IsError: false,
		Metadata: map[string]any{
			"scan_time":  result.ScanTime,
			"scanner":    result.Scanner,
			"session_id": result.SessionID,
			"duration":   result.Duration,
		},
	}, nil
}

// NewTypeSafeScanTool creates a TypeSafe-compatible scan tool using services
func NewTypeSafeScanTool(container services.ServiceContainer, logger *slog.Logger) tools.Tool {
	return &ScanToolDomainWrapper{
		apiTool: NewScanTool(container, logger).(*ScanTool),
	}
}

// NewTypeSafeScanToolLegacy creates a TypeSafe-compatible scan tool (backward compatibility)
func NewTypeSafeScanToolLegacy(sessionManager session.UnifiedSessionManager, logger *slog.Logger) tools.Tool {
	return &ScanToolDomainWrapper{
		apiTool: NewScanToolLegacy(sessionManager, logger).(*ScanTool),
	}
}

// NewTypeVirtualSecurityScanTool creates TypeSafe security scan tool using services
func NewTypeVirtualSecurityScanTool(container services.ServiceContainer, logger *slog.Logger) tools.Tool {
	return &ScanToolDomainWrapper{
		apiTool: NewScanTool(container, logger).(*ScanTool),
	}
}

// NewTypeVirtualSecurityScanToolLegacy creates TypeSafe security scan tool (backward compatibility)
func NewTypeVirtualSecurityScanToolLegacy(sessionManager session.UnifiedSessionManager, logger *slog.Logger) tools.Tool {
	return &ScanToolDomainWrapper{
		apiTool: NewScanToolLegacy(sessionManager, logger).(*ScanTool),
	}
}

// SecretScanTool implements the canonical tools.Tool interface for secret scanning using services
type SecretScanTool struct {
	sessionStore services.SessionStore
	sessionState services.SessionState
	scanner      services.Scanner
	logger       *slog.Logger
	atomicTool   *AtomicScanSecretsTool
}

// NewSecretScanTool creates a new secret scan tool using service container
func NewSecretScanTool(container services.ServiceContainer, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "scan_secrets")

	// Create atomic tool for compatibility
	atomicTool := newAtomicScanSecretsToolImpl(nil, container, toolLogger)

	return &SecretScanTool{
		sessionStore: container.SessionStore(),
		sessionState: container.SessionState(),
		scanner:      container.Scanner(),
		logger:       toolLogger,
		atomicTool:   atomicTool,
	}
}

// NewSecretScanToolLegacy creates a new secret scan tool using legacy session manager (backward compatibility)
func NewSecretScanToolLegacy(sessionManager session.UnifiedSessionManager, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "scan_secrets_legacy")

	return &SecretScanTool{
		sessionStore: nil, // Legacy mode
		sessionState: nil,
		scanner:      nil,
		logger:       toolLogger,
		atomicTool:   nil,
	}
}

// Name implements api.Tool
func (t *SecretScanTool) Name() string {
	return "scan_secrets"
}

// Description implements api.Tool
func (t *SecretScanTool) Description() string {
	return "Scans code repositories and files for exposed secrets like API keys, passwords, and credentials"
}

// Category implements api.Tool
func (t *SecretScanTool) Category() string {
	return "scan"
}

// Tags implements api.Tool
func (t *SecretScanTool) Tags() []string {
	return []string{"security", "secrets", "credentials", "scanning", "detection"}
}

// Version implements api.Tool
func (t *SecretScanTool) Version() string {
	return "1.0.0"
}

// Schema implements api.Tool
func (t *SecretScanTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking the secret scan workflow",
				},
				"scan_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to scan for secrets (repository or directory)",
				},
				"file_patterns": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File patterns to include in scan (default: common source files)",
				},
				"exclude_patterns": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File patterns to exclude from scan",
				},
				"secret_types": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"api_keys", "passwords", "tokens", "certificates", "database_urls", "all"},
					},
					"description": "Types of secrets to detect (default: all)",
				},
				"confidence_threshold": map[string]interface{}{
					"type":        "number",
					"description": "Minimum confidence threshold for detection (0-1, default: 0.7)",
					"minimum":     0,
					"maximum":     1,
				},
				"include_git_history": map[string]interface{}{
					"type":        "boolean",
					"description": "Include git history in scan (default: false)",
				},
				"max_file_size": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum file size to scan in bytes (default: 10MB)",
					"minimum":     1024,
					"maximum":     104857600,
				},
				"dry_run": map[string]interface{}{
					"type":        "boolean",
					"description": "Preview changes without executing",
				},
			},
			"required": []string{"session_id", "scan_path"},
		},
		Tags:     t.Tags(),
		Category: "scan",
	}
}

// Execute implements api.Tool
func (t *SecretScanTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse input JSON
	var params struct {
		SessionID           string   `json:"session_id"`
		ScanPath            string   `json:"scan_path"`
		FilePatterns        []string `json:"file_patterns,omitempty"`
		ExcludePatterns     []string `json:"exclude_patterns,omitempty"`
		SecretTypes         []string `json:"secret_types,omitempty"`
		ConfidenceThreshold float64  `json:"confidence_threshold,omitempty"`
		IncludeGitHistory   bool     `json:"include_git_history,omitempty"`
		MaxFileSize         int64    `json:"max_file_size,omitempty"`
		DryRun              bool     `json:"dry_run,omitempty"`
	}

	// Extract parameters from input data
	if err := extractParamsFromInput(input.Data, &params); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse input: %v", err),
		}, err
	}

	// Validate required parameters
	if params.SessionID == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "session_id is required",
			Data: map[string]interface{}{
				"error": "session_id is required",
			},
		}, errors.NewError().Messagef("session_id is required").WithLocation().Build()
	}

	if params.ScanPath == "" {
		return api.ToolOutput{
			Success: false,
			Data: map[string]interface{}{
				"message": "scan_path is required",
				"error":   "scan_path is required",
			},
			Error: "scan_path is required",
		}, errors.NewError().Messagef("scan_path is required").WithLocation().Build()
	}

	// Set defaults
	if len(params.SecretTypes) == 0 {
		params.SecretTypes = []string{"all"}
	}
	if params.ConfidenceThreshold == 0 {
		params.ConfidenceThreshold = 0.7
	}
	if params.MaxFileSize == 0 {
		params.MaxFileSize = 10 * 1024 * 1024 // 10MB default
	}
	if len(params.FilePatterns) == 0 {
		params.FilePatterns = []string{"*.go", "*.js", "*.py", "*.java", "*.yaml", "*.yml", "*.json", "*.env"}
	}

	// Log the execution
	t.logger.Info("Starting secret scan",
		"session_id", params.SessionID,
		"scan_path", params.ScanPath,
		"secret_types", params.SecretTypes,
		"confidence_threshold", params.ConfidenceThreshold,
		"include_git_history", params.IncludeGitHistory,
		"dry_run", params.DryRun)

	startTime := time.Now()

	// Create or get session using services
	var sess *api.Session
	if t.sessionStore != nil {
		session, err := t.sessionStore.Get(ctx, params.SessionID)
		if err != nil {
			// Create new session if it doesn't exist
			sessionID, createErr := t.sessionStore.Create(ctx, map[string]interface{}{
				"tool_name": "scan_secrets",
				"scan_type": "secrets",
			})
			if createErr != nil {
				return api.ToolOutput{
					Success: false,
					Error:   "Failed to create session: " + createErr.Error(),
					Data: map[string]interface{}{
						"error": createErr.Error(),
					},
				}, createErr
			}
			session, err = t.sessionStore.Get(ctx, sessionID)
			if err != nil {
				return api.ToolOutput{
					Success: false,
					Error:   "Failed to get created session: " + err.Error(),
					Data: map[string]interface{}{
						"error": err.Error(),
					},
				}, err
			}
		}
		sess = session

		// Save scan state
		if t.sessionState != nil {
			scanState := map[string]interface{}{
				"status":     "scanning_secrets",
				"scan_path":  params.ScanPath,
				"start_time": time.Now(),
			}
			t.sessionState.SaveState(ctx, sess.ID, scanState)
		}
	}

	// Handle dry run
	if params.DryRun {
		return t.handleSecretScanDryRun(params, sess, startTime), nil
	}

	// Perform secret scan
	scanResult, err := t.performSecretScan(ctx, params)
	if err != nil {
		return t.createSecretScanErrorResult(params.SessionID, "Secret scan failed", err, startTime), err
	}

	// Update session state
	if t.sessionState != nil && sess != nil {
		sessionData := map[string]interface{}{
			"status":        "secret_scan_completed",
			"success":       true,
			"scan_time":     time.Now(),
			"secrets_found": len(scanResult.SecretsFound),
		}
		t.sessionState.SaveState(ctx, sess.ID, sessionData)
	}

	// Create successful result
	result := api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"message":           fmt.Sprintf("Secret scan completed: found %d potential secrets in %s", len(scanResult.SecretsFound), params.ScanPath),
			"session_id":        params.SessionID,
			"scan_path":         params.ScanPath,
			"secrets_found":     scanResult.SecretsFound,
			"files_scanned":     scanResult.FilesScanned,
			"secret_types":      scanResult.SecretTypes,
			"remediation_steps": scanResult.RemediationSteps,
			"success":           true,
			"duration_ms":       int64(time.Since(startTime).Milliseconds()),
			"scan_metadata":     scanResult.Metadata,
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        params.SessionID,
			"tool_version":      t.Version(),
			"dry_run":           params.DryRun,
		},
	}

	t.logger.Info("Secret scan completed",
		"session_id", params.SessionID,
		"scan_path", params.ScanPath,
		"secrets_found", len(scanResult.SecretsFound),
		"duration", time.Since(startTime))

	return result, nil
}

// handleSecretScanDryRun returns early result for dry run mode
func (t *SecretScanTool) handleSecretScanDryRun(params struct {
	SessionID           string   `json:"session_id"`
	ScanPath            string   `json:"scan_path"`
	FilePatterns        []string `json:"file_patterns,omitempty"`
	ExcludePatterns     []string `json:"exclude_patterns,omitempty"`
	SecretTypes         []string `json:"secret_types,omitempty"`
	ConfidenceThreshold float64  `json:"confidence_threshold,omitempty"`
	IncludeGitHistory   bool     `json:"include_git_history,omitempty"`
	MaxFileSize         int64    `json:"max_file_size,omitempty"`
	DryRun              bool     `json:"dry_run,omitempty"`
}, sess *api.Session, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"message":    "Dry run: Secret scan would be performed",
			"session_id": params.SessionID,
			"scan_path":  params.ScanPath,
			"dry_run":    true,
			"preview": map[string]interface{}{
				"would_scan_path":      params.ScanPath,
				"would_detect_types":   params.SecretTypes,
				"file_patterns":        params.FilePatterns,
				"exclude_patterns":     params.ExcludePatterns,
				"confidence_threshold": params.ConfidenceThreshold,
				"would_include_git":    params.IncludeGitHistory,
				"max_file_size_mb":     params.MaxFileSize / 1024 / 1024,
				"estimated_duration_s": 30,
			},
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        params.SessionID,
			"tool_version":      t.Version(),
			"dry_run":           true,
		},
	}
}

// performSecretScan executes the secret scanning logic
func (t *SecretScanTool) performSecretScan(ctx context.Context, params struct {
	SessionID           string   `json:"session_id"`
	ScanPath            string   `json:"scan_path"`
	FilePatterns        []string `json:"file_patterns,omitempty"`
	ExcludePatterns     []string `json:"exclude_patterns,omitempty"`
	SecretTypes         []string `json:"secret_types,omitempty"`
	ConfidenceThreshold float64  `json:"confidence_threshold,omitempty"`
	IncludeGitHistory   bool     `json:"include_git_history,omitempty"`
	MaxFileSize         int64    `json:"max_file_size,omitempty"`
	DryRun              bool     `json:"dry_run,omitempty"`
}) (*SecretScanResult, error) {
	// Simulate secret scanning logic
	// In a real implementation, this would use the atomic tool

	scanResult := &SecretScanResult{
		SecretsFound:     []DetectedSecret{},
		FilesScanned:     []string{},
		SecretTypes:      params.SecretTypes,
		RemediationSteps: []string{},
		Metadata: SecretScanMetadata{
			ScanStarted:    time.Now(),
			ScanPath:       params.ScanPath,
			ScanDuration:   0,
			FilesProcessed: 0,
			TotalFileSize:  0,
		},
	}

	// Mock scan results
	if strings.Contains(params.ScanPath, "api") ||
		contains(params.SecretTypes, "api_keys") || contains(params.SecretTypes, "all") {
		scanResult.SecretsFound = append(scanResult.SecretsFound, DetectedSecret{
			Type:       "api_key",
			File:       "config/api.json",
			Line:       15,
			Confidence: 0.9,
			Pattern:    "API_KEY=sk-1234567890abcdef",
		})
	}

	scanResult.FilesScanned = []string{"src/main.go", "config/api.json", ".env"}
	scanResult.RemediationSteps = []string{
		"Move API keys to environment variables",
		"Use secret management systems",
		"Add .env files to .gitignore",
		"Rotate exposed credentials",
	}

	scanResult.Metadata.FilesProcessed = len(scanResult.FilesScanned)
	scanResult.Metadata.TotalFileSize = 2048 // Mock size

	return scanResult, nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// LegacySecurityScanTool implements the canonical tools.Tool interface for legacy security scan functionality using services
type LegacySecurityScanTool struct {
	sessionStore services.SessionStore
	sessionState services.SessionState
	scanner      services.Scanner
	logger       *slog.Logger
	legacyTool   *securityScanToolImpl
}

// NewLegacySecurityScanTool creates a new legacy security scan tool using service container
func NewLegacySecurityScanTool(container services.ServiceContainer, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "legacy_security_scan")

	// Create legacy tool for compatibility
	legacyTool := &securityScanToolImpl{
		sessionStore: container.SessionStore(),
		scanner:      container.Scanner(),
		logger:       toolLogger,
	}

	return &LegacySecurityScanTool{
		sessionStore: container.SessionStore(),
		sessionState: container.SessionState(),
		scanner:      container.Scanner(),
		logger:       toolLogger,
		legacyTool:   legacyTool,
	}
}

// NewLegacySecurityScanToolLegacy creates a new legacy security scan tool using session manager (backward compatibility)
func NewLegacySecurityScanToolLegacy(sessionManager session.UnifiedSessionManager, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "legacy_security_scan_legacy")

	return &LegacySecurityScanTool{
		sessionStore: nil, // Legacy mode
		sessionState: nil,
		scanner:      nil,
		logger:       toolLogger,
		legacyTool:   nil,
	}
}

// Name implements api.Tool
func (t *LegacySecurityScanTool) Name() string {
	return "legacy_security_scan"
}

// Description implements api.Tool
func (t *LegacySecurityScanTool) Description() string {
	return "Legacy security scan tool with strongly-typed parameters and comprehensive error handling"
}

// Category implements api.Tool
func (t *LegacySecurityScanTool) Category() string {
	return "scan"
}

// Tags implements api.Tool
func (t *LegacySecurityScanTool) Tags() []string {
	return []string{"security", "legacy", "scanning", "vulnerability"}
}

// Version implements api.Tool
func (t *LegacySecurityScanTool) Version() string {
	return "1.0.0"
}

// Schema implements api.Tool
func (t *LegacySecurityScanTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking the security scan workflow",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Target to scan (image, directory, etc.)",
				},
				"scan_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of scan to perform",
					"enum":        []string{"vulnerability", "secret", "compliance", "license"},
				},
				"scanner": map[string]interface{}{
					"type":        "string",
					"description": "Scanner to use (trivy, grype, etc.)",
					"default":     "trivy",
				},
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Output format",
					"enum":        []string{"json", "table", "sarif"},
					"default":     "json",
				},
				"severity_filter": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"},
					},
					"description": "Filter by severity levels",
				},
			},
			"required": []string{"session_id", "target", "scan_type"},
		},
		Tags:     t.Tags(),
		Category: "scan",
	}
}

// Execute implements api.Tool
func (t *LegacySecurityScanTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse input JSON
	var params struct {
		SessionID      string   `json:"session_id"`
		Target         string   `json:"target"`
		ScanType       string   `json:"scan_type"`
		Scanner        string   `json:"scanner,omitempty"`
		Format         string   `json:"format,omitempty"`
		SeverityFilter []string `json:"severity_filter,omitempty"`
	}

	// Extract parameters from input data
	if err := extractParamsFromInput(input.Data, &params); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse input: %v", err),
		}, err
	}

	// Validate required parameters
	if params.SessionID == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "session_id is required",
			Data: map[string]interface{}{
				"error": "session_id is required",
			},
		}, errors.NewError().Messagef("session_id is required").WithLocation().Build()
	}

	if params.Target == "" {
		return api.ToolOutput{
			Success: false,
			Data: map[string]interface{}{
				"message": "target is required",
				"error":   "target is required",
			},
			Error: "target is required",
		}, errors.NewError().Messagef("target is required").WithLocation().Build()
	}

	if params.ScanType == "" {
		return api.ToolOutput{
			Success: false,
			Data: map[string]interface{}{
				"message": "scan_type is required",
				"error":   "scan_type is required",
			},
			Error: "scan_type is required",
		}, errors.NewError().Messagef("scan_type is required").WithLocation().Build()
	}

	// Set defaults
	if params.Scanner == "" {
		params.Scanner = "trivy"
	}
	if params.Format == "" {
		params.Format = "json"
	}

	// Log the execution
	t.logger.Info("Starting legacy security scan",
		"session_id", params.SessionID,
		"target", params.Target,
		"scan_type", params.ScanType,
		"scanner", params.Scanner)

	startTime := time.Now()

	// Create or get session using services
	var sess *api.Session
	if t.sessionStore != nil {
		session, err := t.sessionStore.Get(ctx, params.SessionID)
		if err != nil {
			// Create new session if it doesn't exist
			sessionID, createErr := t.sessionStore.Create(ctx, map[string]interface{}{
				"tool_name": "legacy_security_scan",
				"scan_type": params.ScanType,
			})
			if createErr != nil {
				return api.ToolOutput{
					Success: false,
					Error:   "Failed to create session: " + createErr.Error(),
					Data: map[string]interface{}{
						"error": createErr.Error(),
					},
				}, createErr
			}
			session, err = t.sessionStore.Get(ctx, sessionID)
			if err != nil {
				return api.ToolOutput{
					Success: false,
					Error:   "Failed to get created session: " + err.Error(),
					Data: map[string]interface{}{
						"error": err.Error(),
					},
				}, err
			}
		}
		sess = session

		// Save scan state
		if t.sessionState != nil {
			scanState := map[string]interface{}{
				"status":     "legacy_security_scanning",
				"target":     params.Target,
				"scanner":    params.Scanner,
				"start_time": time.Now(),
			}
			t.sessionState.SaveState(ctx, sess.ID, scanState)
		}
	}

	// Simulate legacy scan execution
	scanResult, err := t.performLegacyScan(ctx, params)
	if err != nil {
		return t.createLegacyScanErrorResult(params.SessionID, "Legacy security scan failed", err, startTime), err
	}

	// Update session state
	if t.sessionState != nil && sess != nil {
		sessionData := map[string]interface{}{
			"status":          "legacy_scan_completed",
			"success":         true,
			"scan_time":       time.Now(),
			"vulnerabilities": scanResult.TotalVulnerabilities,
		}
		t.sessionState.SaveState(ctx, sess.ID, sessionData)
	}

	// Create successful result
	result := api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"message":                     fmt.Sprintf("Legacy security scan completed: found %d vulnerabilities for target %s", scanResult.TotalVulnerabilities, params.Target),
			"session_id":                  params.SessionID,
			"target":                      params.Target,
			"scan_type":                   params.ScanType,
			"scanner":                     params.Scanner,
			"success":                     true,
			"total_vulnerabilities":       scanResult.TotalVulnerabilities,
			"vulnerabilities_by_severity": scanResult.VulnerabilitiesBySeverity,
			"risk_score":                  scanResult.RiskScore,
			"risk_level":                  scanResult.RiskLevel,
			"vulnerabilities":             scanResult.Vulnerabilities,
			"secrets":                     scanResult.Secrets,
			"compliance_results":          scanResult.ComplianceResults,
			"licenses":                    scanResult.Licenses,
			"recommendations":             scanResult.Recommendations,
			"duration_ms":                 int64(time.Since(startTime).Milliseconds()),
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        params.SessionID,
			"tool_version":      t.Version(),
			"legacy_mode":       true,
		},
	}

	t.logger.Info("Legacy security scan completed",
		"session_id", params.SessionID,
		"target", params.Target,
		"vulnerabilities", scanResult.TotalVulnerabilities,
		"risk_score", scanResult.RiskScore,
		"duration", time.Since(startTime))

	return result, nil
}

// performLegacyScan simulates legacy security scan execution
func (t *LegacySecurityScanTool) performLegacyScan(ctx context.Context, params struct {
	SessionID      string   `json:"session_id"`
	Target         string   `json:"target"`
	ScanType       string   `json:"scan_type"`
	Scanner        string   `json:"scanner,omitempty"`
	Format         string   `json:"format,omitempty"`
	SeverityFilter []string `json:"severity_filter,omitempty"`
}) (*LegacySecurityScanResult, error) {
	// Mock legacy scan results based on target and scan type
	result := &LegacySecurityScanResult{
		Success:   true,
		Target:    params.Target,
		ScanType:  params.ScanType,
		Scanner:   params.Scanner,
		Duration:  time.Millisecond * 200, // Mock scan duration
		SessionID: params.SessionID,
	}

	// Generate mock vulnerabilities based on target
	if strings.Contains(params.Target, "nginx") || strings.Contains(params.Target, "apache") {
		result.TotalVulnerabilities = 3
		result.VulnerabilitiesBySeverity = map[string]int{
			"CRITICAL": 0,
			"HIGH":     1,
			"MEDIUM":   2,
			"LOW":      0,
		}
		result.Vulnerabilities = []LegacySecurityVulnerability{
			{
				ID:          "CVE-2023-webserver",
				Title:       "Web server vulnerability",
				Description: "HTTP request smuggling vulnerability",
				Severity:    "HIGH",
				CVSS:        7.5,
				Package: LegacyPackageInfo{
					Name:           "nginx",
					Version:        "1.18.0",
					FixedVersion:   "1.20.1",
					PackageManager: "apt",
				},
				References: []string{"https://nginx.org/security"},
				Fixed:      true,
				Fix:        "Update to nginx 1.20.1 or later",
			},
		}
	} else {
		result.TotalVulnerabilities = 1
		result.VulnerabilitiesBySeverity = map[string]int{
			"CRITICAL": 0,
			"HIGH":     0,
			"MEDIUM":   1,
			"LOW":      0,
		}
		result.Vulnerabilities = []LegacySecurityVulnerability{
			{
				ID:          "CVE-2023-general",
				Title:       "General application vulnerability",
				Description: "Minor security issue in application dependency",
				Severity:    "MEDIUM",
				CVSS:        4.3,
				Package: LegacyPackageInfo{
					Name:           "example-lib",
					Version:        "1.0.0",
					FixedVersion:   "1.0.1",
					PackageManager: "npm",
				},
				References: []string{"https://example.com/security"},
				Fixed:      true,
				Fix:        "Update to example-lib 1.0.1",
			},
		}
	}

	// Mock compliance results
	result.ComplianceResults = []LegacyComplianceResult{
		{
			Standard:    "CIS Docker Benchmark",
			Control:     "4.1",
			Status:      "PASS",
			Description: "Ensure a user for the container has been created",
		},
	}

	// Mock secrets (only if scan type includes secrets)
	if params.ScanType == "secret" || strings.Contains(params.ScanType, "all") {
		result.Secrets = []LegacyDetectedSecret{
			{
				Type:        "api_key",
				File:        "config.yaml",
				Line:        15,
				Description: "Potential API key detected",
				Severity:    "MEDIUM",
			},
		}
	}

	// Mock license information
	result.Licenses = []LegacyLicenseInfo{
		{
			Package: "example-lib",
			License: "MIT",
			Type:    "permissive",
			Risk:    "low",
		},
	}

	// Calculate risk score and recommendations
	result.RiskScore = calculateLegacyRiskScore(result.VulnerabilitiesBySeverity)
	result.RiskLevel = calculateLegacyRiskLevel(result.RiskScore)
	result.Recommendations = []string{
		"Update identified packages to fixed versions",
		"Review detected secrets and move to secure storage",
		"Implement regular security scanning in CI/CD pipeline",
	}

	return result, nil
}

// Helper types for legacy security scanning
type LegacySecurityScanResult struct {
	Success                   bool                          `json:"success"`
	Target                    string                        `json:"target"`
	ScanType                  string                        `json:"scan_type"`
	Scanner                   string                        `json:"scanner"`
	Duration                  time.Duration                 `json:"duration"`
	SessionID                 string                        `json:"session_id"`
	TotalVulnerabilities      int                           `json:"total_vulnerabilities"`
	VulnerabilitiesBySeverity map[string]int                `json:"vulnerabilities_by_severity"`
	Vulnerabilities           []LegacySecurityVulnerability `json:"vulnerabilities"`
	ComplianceResults         []LegacyComplianceResult      `json:"compliance_results"`
	Secrets                   []LegacyDetectedSecret        `json:"secrets"`
	Licenses                  []LegacyLicenseInfo           `json:"licenses"`
	RiskScore                 float64                       `json:"risk_score"`
	RiskLevel                 string                        `json:"risk_level"`
	Recommendations           []string                      `json:"recommendations"`
}

type LegacySecurityVulnerability struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Severity    string            `json:"severity"`
	CVSS        float64           `json:"cvss"`
	Package     LegacyPackageInfo `json:"package"`
	References  []string          `json:"references"`
	Fixed       bool              `json:"fixed"`
	Fix         string            `json:"fix"`
}

type LegacyPackageInfo struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	FixedVersion   string `json:"fixed_version,omitempty"`
	PackageManager string `json:"package_manager,omitempty"`
}

type LegacyComplianceResult struct {
	Standard    string `json:"standard"`
	Control     string `json:"control"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type LegacyDetectedSecret struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type LegacyLicenseInfo struct {
	Package string `json:"package"`
	License string `json:"license"`
	Type    string `json:"type"`
	Risk    string `json:"risk"`
}

func (t *LegacySecurityScanTool) createLegacyScanErrorResult(sessionID, message string, err error, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Data: map[string]interface{}{
			"message":    message + ": " + err.Error(),
			"session_id": sessionID,
			"error":      true,
		},
		Error: message + ": " + err.Error(),
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        sessionID,
			"tool_version":      t.Version(),
			"error":             true,
			"legacy_mode":       true,
		},
	}
}

// calculateLegacyRiskScore calculates a risk score based on vulnerability counts
func calculateLegacyRiskScore(vulnerabilities map[string]int) float64 {
	score := 0.0
	weights := map[string]float64{
		"CRITICAL": 10.0,
		"HIGH":     7.0,
		"MEDIUM":   4.0,
		"LOW":      1.0,
		"UNKNOWN":  0.5,
	}

	for severity, count := range vulnerabilities {
		if weight, exists := weights[severity]; exists {
			score += weight * float64(count)
		}
	}

	if score > 10 {
		return 10.0
	}
	return score
}

// calculateLegacyRiskLevel determines risk level based on risk score
func calculateLegacyRiskLevel(score float64) string {
	if score >= 8.0 {
		return "CRITICAL"
	} else if score >= 6.0 {
		return "HIGH"
	} else if score >= 4.0 {
		return "MEDIUM"
	} else if score >= 2.0 {
		return "LOW"
	}
	return "MINIMAL"
}

// Helper types for secret scanning
type SecretScanResult struct {
	SecretsFound     []DetectedSecret   `json:"secrets_found"`
	FilesScanned     []string           `json:"files_scanned"`
	SecretTypes      []string           `json:"secret_types"`
	RemediationSteps []string           `json:"remediation_steps"`
	Metadata         SecretScanMetadata `json:"metadata"`
}

type DetectedSecret struct {
	Type       string  `json:"type"`
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Confidence float64 `json:"confidence"`
	Pattern    string  `json:"pattern"`
}

type SecretScanMetadata struct {
	ScanStarted    time.Time     `json:"scan_started"`
	ScanPath       string        `json:"scan_path"`
	ScanDuration   time.Duration `json:"scan_duration"`
	FilesProcessed int           `json:"files_processed"`
	TotalFileSize  int64         `json:"total_file_size"`
}

// extractParamsFromInput is a helper function to extract parameters from input data
func extractParamsFromInput(data map[string]interface{}, params interface{}) error {
	inputData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal input data: %v", err)
	}

	if err := json.Unmarshal(inputData, params); err != nil {
		return fmt.Errorf("failed to unmarshal input params: %v", err)
	}

	return nil
}

func (t *SecretScanTool) createSecretScanErrorResult(sessionID, message string, err error, startTime time.Time) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Data: map[string]interface{}{
			"message":    message + ": " + err.Error(),
			"session_id": sessionID,
			"error":      true,
		},
		Error: message + ": " + err.Error(),
		Metadata: map[string]interface{}{
			"execution_time_ms": int64(time.Since(startTime).Milliseconds()),
			"session_id":        sessionID,
			"tool_version":      t.Version(),
			"error":             true,
		},
	}
}
