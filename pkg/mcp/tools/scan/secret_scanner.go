package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/security"
	"github.com/rs/zerolog"
)

// FileSecretScanner handles the actual scanning of files for secrets
type FileSecretScanner struct {
	logger          *slog.Logger
	secretDiscovery *security.SecretDiscovery
}

// NewFileSecretScanner creates a new file secret scanner
func NewFileSecretScanner(logger *slog.Logger) *FileSecretScanner {
	// Convert slog.Logger to zerolog.Logger for SecretDiscovery
	zerologLogger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	return &FileSecretScanner{
		logger:          logger,
		secretDiscovery: security.NewSecretDiscovery(zerologLogger),
	}
}

// PerformSecretScan scans a directory for secrets
func (s *FileSecretScanner) PerformSecretScan(scanPath string, filePatterns, excludePatterns []string, reporter interface{}) ([]ScannedSecret, []FileSecretScanResult, int, error) {
	var allSecrets []ScannedSecret
	var fileResults []FileSecretScanResult
	filesScanned := 0

	// Use SecretDiscovery to scan the directory
	ctx := context.Background()
	scanOptions := security.DefaultScanOptions()
	scanOptions.FileTypes = s.convertPatternsToExtensions(filePatterns)

	discoveryResult, err := s.secretDiscovery.ScanDirectory(ctx, scanPath, scanOptions)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("secret discovery scan failed: %w", err)
	}

	filesScanned = discoveryResult.FilesScanned

	// Convert ExtendedSecretFinding to ScannedSecret
	for _, finding := range discoveryResult.Findings {
		if !s.shouldIncludeFinding(finding, filePatterns, excludePatterns) {
			continue
		}

		secret := ScannedSecret{
			File:       finding.File,
			Type:       s.classifySecretType(finding.Type),
			Pattern:    finding.Pattern,
			Value:      finding.Redacted,
			Severity:   s.mapSeverity(finding.Severity),
			Confidence: s.parseConfidence(finding.Confidence),
		}
		allSecrets = append(allSecrets, secret)

		// Group by file for file results
		if len(fileResults) == 0 || fileResults[len(fileResults)-1].FilePath != finding.File {
			fileResult := FileSecretScanResult{
				FilePath:     finding.File,
				FileType:     s.getFileType(finding.File),
				SecretsFound: 0,
				Secrets:      make([]ScannedSecret, 0),
				CleanStatus:  "clean",
			}
			fileResults = append(fileResults, fileResult)
		}

		// Add secret to current file result
		currentFile := &fileResults[len(fileResults)-1]
		currentFile.Secrets = append(currentFile.Secrets, secret)
		currentFile.SecretsFound = len(currentFile.Secrets)
		currentFile.CleanStatus = s.determineCleanStatus(currentFile.Secrets)

		// Report progress
		if reporter != nil {
			if progressReporter, ok := reporter.(interface {
				ReportStage(float64, string)
			}); ok {
				progress := float64(len(allSecrets)) / float64(len(discoveryResult.Findings))
				progressReporter.ReportStage(progress, fmt.Sprintf("Processed %d/%d findings", len(allSecrets), len(discoveryResult.Findings)))
			}
		}
	}

	return allSecrets, fileResults, filesScanned, nil
}

// GetDefaultFilePatterns returns default file patterns based on scan options
func (s *FileSecretScanner) GetDefaultFilePatterns(args AtomicScanSecretsArgs) []string {
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

// shouldScanFile determines if a file should be scanned based on patterns
func (s *FileSecretScanner) shouldScanFile(path string, includePatterns, excludePatterns []string) bool {
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

// convertPatternsToExtensions converts file patterns to extensions for SecretDiscovery
func (s *FileSecretScanner) convertPatternsToExtensions(patterns []string) []string {
	extensions := make([]string, 0)
	for _, pattern := range patterns {
		// Convert glob patterns to extensions
		if strings.HasPrefix(pattern, "*.") {
			extensions = append(extensions, pattern[1:]) // Remove *
		} else if strings.Contains(pattern, ".") {
			// Extract extension from pattern
			parts := strings.Split(pattern, ".")
			if len(parts) > 1 {
				extensions = append(extensions, "."+parts[len(parts)-1])
			}
		}
	}
	return extensions
}

// shouldIncludeFinding determines if a finding should be included based on patterns
func (s *FileSecretScanner) shouldIncludeFinding(finding security.ExtendedSecretFinding, filePatterns, excludePatterns []string) bool {
	filename := filepath.Base(finding.File)

	// Check exclude patterns first
	for _, pattern := range excludePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err != nil {
			continue
		}
		if matched {
			return false
		}
	}

	// If no include patterns specified, include all
	if len(filePatterns) == 0 {
		return true
	}

	// Check include patterns
	for _, pattern := range filePatterns {
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

// mapSeverity maps SecretDiscovery severity to scanner severity
func (s *FileSecretScanner) mapSeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		return "medium"
	}
}

// parseConfidence parses confidence string to int
func (s *FileSecretScanner) parseConfidence(confidence string) int {
	// Convert confidence string (e.g., "0.85") to percentage
	var conf float64
	if _, err := fmt.Sscanf(confidence, "%f", &conf); err == nil {
		return int(conf * 100)
	}
	return 50 // Default confidence
}

// getFileType determines the type of file being scanned
func (s *FileSecretScanner) getFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	if strings.HasPrefix(base, "dockerfile") {
		return "dockerfile"
	}

	if strings.HasPrefix(base, ".env") || strings.HasSuffix(base, ".env") {
		return "environment"
	}

	switch ext {
	case ".yaml", ".yml":
		return "kubernetes"
	case ".json":
		return "json"
	case ".py":
		return "python"
	case ".js", ".ts":
		return "javascript"
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	default:
		return "unknown"
	}
}

// determineCleanStatus determines if a file is clean of secrets
func (s *FileSecretScanner) determineCleanStatus(secrets []ScannedSecret) string {
	if len(secrets) == 0 {
		return "clean"
	}

	hasHigh := false
	hasMedium := false
	for _, secret := range secrets {
		if secret.Severity == "high" || secret.Severity == "critical" {
			hasHigh = true
		} else if secret.Severity == "medium" {
			hasMedium = true
		}
	}

	if hasHigh {
		return "critical"
	} else if hasMedium {
		return "warning"
	}
	return "minor"
}

// classifySecretType classifies the type of secret based on pattern
func (s *FileSecretScanner) classifySecretType(pattern string) string {
	pattern = strings.ToLower(pattern)

	// Check more specific patterns first
	if strings.Contains(pattern, "api") && strings.Contains(pattern, "key") {
		return "api_key"
	}
	if strings.Contains(pattern, "private") && strings.Contains(pattern, "key") {
		return "private_key"
	}
	if strings.Contains(pattern, "database") || strings.Contains(pattern, "db") {
		return "database_credential"
	}
	if strings.Contains(pattern, "cert") || strings.Contains(pattern, "certificate") {
		return "certificate"
	}
	// Check more generic patterns last
	if strings.Contains(pattern, "password") {
		return "password"
	}
	if strings.Contains(pattern, "token") {
		return "token"
	}
	if strings.Contains(pattern, "secret") {
		return "secret"
	}

	return "unknown"
}

// determineSeverity determines the severity of a found secret
func (s *FileSecretScanner) determineSeverity(pattern, value string) string {
	pattern = strings.ToLower(pattern)
	value = strings.ToLower(value)

	// Critical - Production credentials, private keys
	if strings.Contains(pattern, "prod") || strings.Contains(pattern, "production") {
		return "critical"
	}
	if strings.Contains(pattern, "private") && strings.Contains(pattern, "key") {
		return "critical"
	}
	if strings.Contains(pattern, "root") || strings.Contains(pattern, "admin") {
		return "critical"
	}

	// High - API keys, tokens, database credentials
	if strings.Contains(pattern, "api") && strings.Contains(pattern, "key") {
		return "high"
	}
	if strings.Contains(pattern, "token") {
		return "high"
	}
	if strings.Contains(pattern, "database") || strings.Contains(pattern, "db") {
		return "high"
	}

	// Medium - General passwords, secrets
	if strings.Contains(pattern, "password") {
		return "medium"
	}
	if strings.Contains(pattern, "secret") {
		return "medium"
	}

	// Low - Other sensitive values
	return "low"
}

// calculateConfidence calculates confidence score for secret detection
func (s *FileSecretScanner) calculateConfidence(pattern string) int {
	pattern = strings.ToLower(pattern)

	// High confidence patterns
	if strings.Contains(pattern, "api_key") || strings.Contains(pattern, "apikey") {
		return 95
	}
	if strings.Contains(pattern, "private_key") || strings.Contains(pattern, "privatekey") {
		return 95
	}
	if strings.Contains(pattern, "password") {
		return 90
	}
	if strings.Contains(pattern, "token") {
		return 90
	}

	// Medium confidence patterns
	if strings.Contains(pattern, "secret") {
		return 80
	}
	if strings.Contains(pattern, "key") {
		return 75
	}

	// Lower confidence for generic patterns
	return 60
}
