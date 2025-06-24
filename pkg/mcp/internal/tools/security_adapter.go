package tools

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools/security"
	"github.com/Azure/container-copilot/pkg/mcp/internal/tools/security/scanners"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// SecurityAdapter integrates the new modular security scanners with the existing atomic tool
type SecurityAdapter struct {
	registry *security.ScannerRegistry
	logger   zerolog.Logger
}

// NewSecurityAdapter creates a new security adapter
func NewSecurityAdapter(logger zerolog.Logger) *SecurityAdapter {
	registry := security.NewScannerRegistry(logger)

	// Register all scanners
	registry.Register(scanners.NewRegexBasedScanner(logger))
	registry.Register(scanners.NewAPIKeyScanner(logger))
	registry.Register(scanners.NewCertificateScanner(logger))

	return &SecurityAdapter{
		registry: registry,
		logger:   logger.With().Str("component", "security_adapter").Logger(),
	}
}

// ScanWithModules performs secret scanning using the modular scanners
func (s *SecurityAdapter) ScanWithModules(
	ctx context.Context,
	args interface{},
	fileContents map[string]string,
) (*SecurityScanResponse, error) {

	// Convert args to scan options
	options := s.convertToScanOptions(args)

	response := &SecurityScanResponse{
		Success:   true,
		Timestamp: time.Now(),
		Results:   make(map[string]*FileScanResult),
		Summary:   &ScanSummary{},
	}

	var allSecrets []security.Secret
	totalFiles := len(fileContents)
	scannedFiles := 0

	// Scan each file
	for filePath, content := range fileContents {
		if s.shouldSkipFile(filePath, content, options) {
			continue
		}

		fileResult, err := s.scanFile(ctx, filePath, content, options)
		if err != nil {
			s.logger.Error().Err(err).Str("file", filePath).Msg("Failed to scan file")
			continue
		}

		response.Results[filePath] = fileResult
		allSecrets = append(allSecrets, fileResult.Secrets...)
		scannedFiles++
	}

	// Generate summary
	response.Summary = s.generateSummary(allSecrets, totalFiles, scannedFiles)
	response.Duration = time.Since(response.Timestamp)

	s.logger.Info().
		Int("files_scanned", scannedFiles).
		Int("secrets_found", len(allSecrets)).
		Dur("duration", response.Duration).
		Msg("Modular security scan completed")

	return response, nil
}

// scanFile scans a single file for secrets
func (s *SecurityAdapter) scanFile(
	ctx context.Context,
	filePath, content string,
	options security.ScanOptions,
) (*FileScanResult, error) {

	contentType := s.determineContentType(filePath, content)

	config := security.ScanConfig{
		Content:     content,
		ContentType: contentType,
		FilePath:    filePath,
		Options:     options,
		Logger:      s.logger,
	}

	// Scan with all applicable scanners
	result, err := s.registry.ScanWithAllApplicable(ctx, config)
	if err != nil {
		return nil, types.NewRichError("SECURITY_SCAN_FAILED", "scanning failed: "+err.Error(), "security_error")
	}

	fileResult := &FileScanResult{
		FilePath:       filePath,
		ContentType:    string(contentType),
		Secrets:        result.AllSecrets,
		ScannerResults: make(map[string]*ScannerResult),
		Duration:       result.Duration,
		Success:        true,
	}

	// Convert scanner results
	for scannerName, scanResult := range result.ScannerResults {
		fileResult.ScannerResults[scannerName] = &ScannerResult{
			Scanner:      scanResult.Scanner,
			Success:      scanResult.Success,
			Duration:     scanResult.Duration,
			SecretsFound: len(scanResult.Secrets),
			Confidence:   scanResult.Confidence,
			Metadata:     scanResult.Metadata,
		}
	}

	return fileResult, nil
}

// convertToScanOptions converts atomic tool args to scan options
func (s *SecurityAdapter) convertToScanOptions(args interface{}) security.ScanOptions {
	// Default options - could be enhanced to parse specific args
	return security.ScanOptions{
		IncludeHighEntropy: true,
		IncludeKeywords:    true,
		IncludePatterns:    true,
		IncludeBase64:      true,
		MaxFileSize:        10 * 1024 * 1024, // 10MB
		Sensitivity:        security.SensitivityMedium,
		SkipBinary:         true,
		SkipArchives:       true,
	}
}

// determineContentType determines the content type based on file path and content
func (s *SecurityAdapter) determineContentType(filePath, content string) security.ContentType {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := strings.ToLower(filepath.Base(filePath))

	// Check by file extension
	switch ext {
	case ".dockerfile", ".dockerignore":
		return security.ContentTypeDockerfile
	case ".yaml", ".yml":
		if s.isKubernetesFile(content) {
			return security.ContentTypeKubernetes
		}
		if s.isComposeFile(content) {
			return security.ContentTypeCompose
		}
		return security.ContentTypeConfig
	case ".json":
		return security.ContentTypeConfig
	case ".env":
		return security.ContentTypeEnvironment
	case ".pem", ".crt", ".key", ".cert":
		return security.ContentTypeCertificate
	case ".sql":
		return security.ContentTypeDatabase
	case ".js", ".ts", ".py", ".go", ".java", ".cs", ".rb", ".php":
		return security.ContentTypeSourceCode
	}

	// Check by file name
	switch fileName {
	case "dockerfile":
		return security.ContentTypeDockerfile
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return security.ContentTypeCompose
	case ".env", ".env.local", ".env.production", ".env.development":
		return security.ContentTypeEnvironment
	}

	// Check by content patterns
	if s.isKubernetesFile(content) {
		return security.ContentTypeKubernetes
	}
	if s.isComposeFile(content) {
		return security.ContentTypeCompose
	}
	if s.isDockerfile(content) {
		return security.ContentTypeDockerfile
	}
	if s.isCertificateContent(content) {
		return security.ContentTypeCertificate
	}

	return security.ContentTypeGeneric
}

// shouldSkipFile determines if a file should be skipped
func (s *SecurityAdapter) shouldSkipFile(filePath, content string, options security.ScanOptions) bool {
	// Skip binary files if configured
	if options.SkipBinary && s.isBinaryFile(content) {
		return true
	}

	// Skip large files
	if int64(len(content)) > options.MaxFileSize {
		return true
	}

	// Skip common non-secret files
	skipPatterns := []string{
		".git/",
		"node_modules/",
		"vendor/",
		".png", ".jpg", ".jpeg", ".gif", ".ico",
		".zip", ".tar", ".gz", ".bz2",
		".exe", ".dll", ".so", ".dylib",
	}

	lowerPath := strings.ToLower(filePath)
	for _, pattern := range skipPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// generateSummary generates a summary of all scan results
func (s *SecurityAdapter) generateSummary(secrets []security.Secret, totalFiles, scannedFiles int) *ScanSummary {
	summary := &ScanSummary{
		TotalFiles:   totalFiles,
		ScannedFiles: scannedFiles,
		TotalSecrets: len(secrets),
		ByType:       make(map[string]int),
		BySeverity:   make(map[string]int),
		ByFile:       make(map[string]int),
	}

	// Aggregate by type and severity
	for _, secret := range secrets {
		summary.ByType[string(secret.Type)]++
		summary.BySeverity[string(secret.Severity)]++
		if secret.Location != nil {
			summary.ByFile[secret.Location.File]++
		}
	}

	// Calculate risk score
	summary.RiskScore = s.calculateRiskScore(secrets)

	return summary
}

// calculateRiskScore calculates an overall risk score
func (s *SecurityAdapter) calculateRiskScore(secrets []security.Secret) float64 {
	if len(secrets) == 0 {
		return 0.0
	}

	var totalRisk float64
	for _, secret := range secrets {
		risk := s.getSecretRisk(secret)
		totalRisk += risk
	}

	// Normalize to 0-10 scale
	averageRisk := totalRisk / float64(len(secrets))
	return averageRisk * 10
}

// getSecretRisk calculates risk for an individual secret
func (s *SecurityAdapter) getSecretRisk(secret security.Secret) float64 {
	baseRisk := 0.0

	switch secret.Severity {
	case security.SeverityCritical:
		baseRisk = 1.0
	case security.SeverityHigh:
		baseRisk = 0.8
	case security.SeverityMedium:
		baseRisk = 0.6
	case security.SeverityLow:
		baseRisk = 0.4
	default:
		baseRisk = 0.2
	}

	// Adjust by confidence
	return baseRisk * secret.Confidence
}

// Helper methods for content type detection

func (s *SecurityAdapter) isKubernetesFile(content string) bool {
	indicators := []string{"apiVersion:", "kind:", "metadata:", "spec:"}
	count := 0
	for _, indicator := range indicators {
		if strings.Contains(content, indicator) {
			count++
		}
	}
	return count >= 3
}

func (s *SecurityAdapter) isComposeFile(content string) bool {
	indicators := []string{"version:", "services:", "volumes:", "networks:"}
	count := 0
	for _, indicator := range indicators {
		if strings.Contains(content, indicator) {
			count++
		}
	}
	return count >= 2
}

func (s *SecurityAdapter) isDockerfile(content string) bool {
	indicators := []string{"FROM ", "RUN ", "COPY ", "ADD ", "WORKDIR ", "EXPOSE "}
	for _, indicator := range indicators {
		if strings.Contains(strings.ToUpper(content), indicator) {
			return true
		}
	}
	return false
}

func (s *SecurityAdapter) isCertificateContent(content string) bool {
	return strings.Contains(content, "-----BEGIN") && strings.Contains(content, "-----END")
}

func (s *SecurityAdapter) isBinaryFile(content string) bool {
	// Simple heuristic: check for null bytes
	return strings.Contains(content, "\x00")
}

// Response types to match existing atomic tool interface

// SecurityScanResponse represents the response from security scanning
type SecurityScanResponse struct {
	Success   bool                       `json:"success"`
	Timestamp time.Time                  `json:"timestamp"`
	Duration  time.Duration              `json:"duration"`
	Results   map[string]*FileScanResult `json:"results"`
	Summary   *ScanSummary               `json:"summary"`
}

// FileScanResult represents scan results for a single file
type FileScanResult struct {
	FilePath       string                    `json:"file_path"`
	ContentType    string                    `json:"content_type"`
	Secrets        []security.Secret         `json:"secrets"`
	ScannerResults map[string]*ScannerResult `json:"scanner_results"`
	Duration       time.Duration             `json:"duration"`
	Success        bool                      `json:"success"`
}

// ScannerResult represents results from a single scanner
type ScannerResult struct {
	Scanner      string                 `json:"scanner"`
	Success      bool                   `json:"success"`
	Duration     time.Duration          `json:"duration"`
	SecretsFound int                    `json:"secrets_found"`
	Confidence   float64                `json:"confidence"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ScanSummary provides a summary of all scan results
type ScanSummary struct {
	TotalFiles   int            `json:"total_files"`
	ScannedFiles int            `json:"scanned_files"`
	TotalSecrets int            `json:"total_secrets"`
	ByType       map[string]int `json:"by_type"`
	BySeverity   map[string]int `json:"by_severity"`
	ByFile       map[string]int `json:"by_file"`
	RiskScore    float64        `json:"risk_score"`
}
