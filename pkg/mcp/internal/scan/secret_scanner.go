package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
)

// FileSecretScanner handles the actual scanning of files for secrets
type FileSecretScanner struct {
	logger zerolog.Logger
}

// NewFileSecretScanner creates a new file secret scanner
func NewFileSecretScanner(logger zerolog.Logger) *FileSecretScanner {
	return &FileSecretScanner{
		logger: logger,
	}
}

// PerformSecretScan scans a directory for secrets
func (s *FileSecretScanner) PerformSecretScan(scanPath string, filePatterns, excludePatterns []string, reporter interface{}) ([]ScannedSecret, []FileSecretScanResult, int, error) {
	scanner := utils.NewSecretScanner()
	var allSecrets []ScannedSecret
	var fileResults []FileSecretScanResult
	filesScanned := 0

	totalFiles := 0
	err := filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && s.shouldScanFile(path, filePatterns, excludePatterns) {
			totalFiles++
		}
		return nil
	})
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to count files for progress")
	}

	err = filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !s.shouldScanFile(path, filePatterns, excludePatterns) {
			return nil
		}

		fileSecrets, err := s.scanFileForSecrets(path, scanner)
		if err != nil {
			s.logger.Warn().Err(err).Str("file", path).Msg("Failed to scan file")
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
			FileType:     s.getFileType(path),
			SecretsFound: len(fileSecrets),
			Secrets:      fileSecrets,
			CleanStatus:  s.determineCleanStatus(fileSecrets),
		}

		fileResults = append(fileResults, fileResult)
		allSecrets = append(allSecrets, fileSecrets...)

		return nil
	})

	return allSecrets, fileResults, filesScanned, err
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

// scanFileForSecrets scans a single file for secrets
func (s *FileSecretScanner) scanFileForSecrets(filePath string, scanner *utils.SecretScanner) ([]ScannedSecret, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	sensitiveVars := scanner.ScanContent(string(content))

	var secrets []ScannedSecret
	for _, sensitiveVar := range sensitiveVars {
		secret := ScannedSecret{
			File:       filePath,
			Type:       s.classifySecretType(sensitiveVar.Pattern),
			Pattern:    sensitiveVar.Pattern,
			Value:      sensitiveVar.Redacted,
			Severity:   s.determineSeverity(sensitiveVar.Pattern, sensitiveVar.Value),
			Confidence: s.calculateConfidence(sensitiveVar.Pattern),
		}
		secrets = append(secrets, secret)
	}

	return secrets, nil
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

	if strings.Contains(pattern, "api") && strings.Contains(pattern, "key") {
		return "api_key"
	}
	if strings.Contains(pattern, "password") {
		return "password"
	}
	if strings.Contains(pattern, "token") {
		return "token"
	}
	if strings.Contains(pattern, "secret") {
		return "secret"
	}
	if strings.Contains(pattern, "cert") || strings.Contains(pattern, "certificate") {
		return "certificate"
	}
	if strings.Contains(pattern, "private") && strings.Contains(pattern, "key") {
		return "private_key"
	}
	if strings.Contains(pattern, "database") || strings.Contains(pattern, "db") {
		return "database_credential"
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
