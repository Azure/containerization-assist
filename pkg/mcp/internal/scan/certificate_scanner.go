package scanners

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/scan"
	"github.com/rs/zerolog"
)

// CertificateScanner specializes in detecting certificates and private keys
type CertificateScanner struct {
	name     string
	patterns map[string]*CertificatePattern
	logger   zerolog.Logger
}

// CertificatePattern represents a pattern for detecting certificates/keys
type CertificatePattern struct {
	Name        string
	Pattern     *regexp.Regexp
	SecretType  scan.SecretType
	Confidence  float64
	Severity    scan.Severity
	Description string
}

// NewCertificateScanner creates a new certificate scanner
func NewCertificateScanner(logger zerolog.Logger) *CertificateScanner {
	scanner := &CertificateScanner{
		name:     "certificate_scanner",
		patterns: make(map[string]*CertificatePattern),
		logger:   logger.With().Str("scanner", "certificate").Logger(),
	}

	scanner.initializePatterns()
	return scanner
}

// GetName returns the scanner name
func (c *CertificateScanner) GetName() string {
	return c.name
}

// GetScanTypes returns the types of secrets this scanner can detect
func (c *CertificateScanner) GetScanTypes() []string {
	return []string{
		string(scan.SecretTypePrivateKey),
		string(scan.SecretTypeCertificate),
	}
}

// IsApplicable determines if this scanner should run
func (c *CertificateScanner) IsApplicable(content string, contentType scan.ContentType) bool {
	// Look for certificate/key indicators
	indicators := []string{
		"-----BEGIN",
		"-----END",
		"PRIVATE KEY",
		"CERTIFICATE",
		"RSA PRIVATE KEY",
		"EC PRIVATE KEY",
		"OPENSSH PRIVATE KEY",
	}

	contentUpper := strings.ToUpper(content)
	for _, indicator := range indicators {
		if strings.Contains(contentUpper, indicator) {
			return true
		}
	}

	return false
}

// Scan performs certificate and private key scanning
func (c *CertificateScanner) Scan(ctx context.Context, config scan.ScanConfig) (*scan.ScanResult, error) {
	startTime := time.Now()
	result := &scan.ScanResult{
		Scanner:  c.GetName(),
		Secrets:  make([]scan.Secret, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Scan for multi-line certificate blocks
	secrets, err := c.scanForCertificateBlocks(config)
	if err != nil {
		result.Errors = append(result.Errors, err)
	} else {
		result.Secrets = append(result.Secrets, secrets...)
	}

	// Scan line by line for embedded certificates
	lines := strings.Split(config.Content, "\n")
	for lineNum, line := range lines {
		lineSecrets, err := c.scanLineForCertificates(line, lineNum+1, config)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Secrets = append(result.Secrets, lineSecrets...)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Confidence = c.calculateConfidence(result)
	result.Metadata["lines_scanned"] = len(lines)
	result.Metadata["patterns_used"] = len(c.patterns)

	return result, nil
}

// scanForCertificateBlocks scans for multi-line certificate blocks
func (c *CertificateScanner) scanForCertificateBlocks(config scan.ScanConfig) ([]scan.Secret, error) {
	var secrets []scan.Secret

	// Patterns for multi-line certificate blocks
	blockPatterns := map[string]struct {
		pattern    *regexp.Regexp
		secretType scan.SecretType
		severity   scan.Severity
	}{
		"RSA_Private_Key": {
			pattern:    regexp.MustCompile(`(?s)-----BEGIN RSA PRIVATE KEY-----(.*?)-----END RSA PRIVATE KEY-----`),
			secretType: scan.SecretTypePrivateKey,
			severity:   scan.SeverityCritical,
		},
		"EC_Private_Key": {
			pattern:    regexp.MustCompile(`(?s)-----BEGIN EC PRIVATE KEY-----(.*?)-----END EC PRIVATE KEY-----`),
			secretType: scan.SecretTypePrivateKey,
			severity:   scan.SeverityCritical,
		},
		"Private_Key": {
			pattern:    regexp.MustCompile(`(?s)-----BEGIN PRIVATE KEY-----(.*?)-----END PRIVATE KEY-----`),
			secretType: scan.SecretTypePrivateKey,
			severity:   scan.SeverityCritical,
		},
		"OpenSSH_Private_Key": {
			pattern:    regexp.MustCompile(`(?s)-----BEGIN OPENSSH PRIVATE KEY-----(.*?)-----END OPENSSH PRIVATE KEY-----`),
			secretType: scan.SecretTypePrivateKey,
			severity:   scan.SeverityCritical,
		},
		"Certificate": {
			pattern:    regexp.MustCompile(`(?s)-----BEGIN CERTIFICATE-----(.*?)-----END CERTIFICATE-----`),
			secretType: scan.SecretTypeCertificate,
			severity:   scan.SeverityHigh,
		},
		"Public_Key": {
			pattern:    regexp.MustCompile(`(?s)-----BEGIN PUBLIC KEY-----(.*?)-----END PUBLIC KEY-----`),
			secretType: scan.SecretTypeCertificate,
			severity:   scan.SeverityMedium,
		},
	}

	for _, patternInfo := range blockPatterns {
		matches := patternInfo.pattern.FindAllStringSubmatch(config.Content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				fullBlock := match[0]
				content := strings.TrimSpace(match[1])

				if c.isValidCertificateContent(content) {
					secret := c.createCertificateSecret(
						"certificate_block",
						fullBlock,
						content,
						patternInfo.secretType,
						patternInfo.severity,
						config,
					)
					secrets = append(secrets, secret)
				}
			}
		}
	}

	return secrets, nil
}

// scanLineForCertificates scans a single line for certificate content
func (c *CertificateScanner) scanLineForCertificates(line string, lineNum int, config scan.ScanConfig) ([]scan.Secret, error) {
	var secrets []scan.Secret

	for _, pattern := range c.patterns {
		matches := pattern.Pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 1 {
				value := match[1]
				if c.isValidCertificateContent(value) {
					secret := c.createLineSecret(pattern, value, line, lineNum, config)
					secrets = append(secrets, secret)
				}
			}
		}
	}

	return secrets, nil
}

// createCertificateSecret creates a secret from certificate block detection
func (c *CertificateScanner) createCertificateSecret(
	patternName, fullBlock, content string,
	secretType scan.SecretType,
	severity scan.Severity,
	config scan.ScanConfig,
) scan.Secret {

	// Calculate line number for the beginning of the block
	lines := strings.Split(config.Content, "\n")
	lineNum := 1
	for i, line := range lines {
		if strings.Contains(line, "-----BEGIN") {
			lineNum = i + 1
			break
		}
	}

	confidence := c.calculateCertificateConfidence(secretType, content, fullBlock)

	secret := scan.Secret{
		Type:        secretType,
		Value:       fullBlock,
		MaskedValue: c.maskCertificate(fullBlock),
		Location: &scan.Location{
			File:   config.FilePath,
			Line:   lineNum,
			Column: 1,
		},
		Confidence: confidence,
		Severity:   severity,
		Context:    c.extractCertificateContext(fullBlock),
		Pattern:    patternName,
		Entropy:    scan.CalculateEntropy(content),
		Metadata: map[string]interface{}{
			"detection_method": "certificate_block",
			"certificate_type": patternName,
			"block_size":       len(fullBlock),
			"content_size":     len(content),
			"is_multiline":     true,
		},
		Evidence: []scan.Evidence{
			{
				Type:        "certificate_block",
				Description: fmt.Sprintf("PEM-encoded %s detected", patternName),
				Value:       fullBlock,
				Pattern:     patternName,
				Context:     c.extractCertificateContext(fullBlock),
			},
		},
	}

	return secret
}

// createLineSecret creates a secret from single-line certificate detection
func (c *CertificateScanner) createLineSecret(
	pattern *CertificatePattern,
	value, line string,
	lineNum int,
	config scan.ScanConfig,
) scan.Secret {

	confidence := c.calculateCertificateConfidence(pattern.SecretType, value, line)

	secret := scan.Secret{
		Type:        pattern.SecretType,
		Value:       value,
		MaskedValue: c.maskCertificate(value),
		Location: &scan.Location{
			File:   config.FilePath,
			Line:   lineNum,
			Column: strings.Index(line, value) + 1,
		},
		Confidence: confidence,
		Severity:   pattern.Severity,
		Context:    strings.TrimSpace(line),
		Pattern:    pattern.Name,
		Entropy:    scan.CalculateEntropy(value),
		Metadata: map[string]interface{}{
			"detection_method": "certificate_line",
			"certificate_type": pattern.Name,
			"value_length":     len(value),
			"is_multiline":     false,
		},
		Evidence: []scan.Evidence{
			{
				Type:        "certificate_line",
				Description: fmt.Sprintf("%s detected in line", pattern.Description),
				Value:       value,
				Pattern:     pattern.Pattern.String(),
				Context:     line,
			},
		},
	}

	return secret
}

// initializePatterns initializes patterns for certificate detection
func (c *CertificateScanner) initializePatterns() {
	patterns := map[string]struct {
		pattern     string
		secretType  scan.SecretType
		confidence  float64
		severity    scan.Severity
		description string
	}{
		"Inline_Private_Key": {
			pattern:     `(?i)(?:private[_-]?key|privatekey)[\"'\s]*[:=][\"'\s]*([A-Za-z0-9+/=]{100,})`,
			secretType:  scan.SecretTypePrivateKey,
			confidence:  0.80,
			severity:    scan.SeverityCritical,
			description: "Inline private key",
		},
		"Base64_Certificate": {
			pattern:     `(?i)(?:certificate|cert)[\"'\s]*[:=][\"'\s]*([A-Za-z0-9+/=]{100,})`,
			secretType:  scan.SecretTypeCertificate,
			confidence:  0.70,
			severity:    scan.SeverityHigh,
			description: "Base64-encoded certificate",
		},
		"PEM_Marker": {
			pattern:     `(-----BEGIN [A-Z ]+-----[A-Za-z0-9+/=\s]+-----END [A-Z ]+-----)`,
			secretType:  scan.SecretTypeCertificate,
			confidence:  0.95,
			severity:    scan.SeverityHigh,
			description: "PEM-formatted certificate or key",
		},
	}

	for name, patternInfo := range patterns {
		compiled, err := regexp.Compile(patternInfo.pattern)
		if err != nil {
			c.logger.Error().Err(err).Str("pattern", name).Msg("Failed to compile certificate pattern")
			continue
		}

		c.patterns[name] = &CertificatePattern{
			Name:        name,
			Pattern:     compiled,
			SecretType:  patternInfo.secretType,
			Confidence:  patternInfo.confidence,
			Severity:    patternInfo.severity,
			Description: patternInfo.description,
		}
	}

	c.logger.Debug().Int("patterns", len(c.patterns)).Msg("Initialized certificate patterns")
}

// isValidCertificateContent validates certificate content
func (c *CertificateScanner) isValidCertificateContent(content string) bool {
	// Remove whitespace
	cleaned := strings.ReplaceAll(content, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	cleaned = strings.ReplaceAll(cleaned, "\t", "")

	// Must be at least 50 characters for a valid certificate/key
	if len(cleaned) < 50 {
		return false
	}

	// Must be valid base64 characters
	base64Pattern := regexp.MustCompile(`^[A-Za-z0-9+/=]+$`)
	if !base64Pattern.MatchString(cleaned) {
		return false
	}

	// Check for obvious test/example values
	contentLower := strings.ToLower(cleaned)
	invalidValues := []string{
		"example",
		"test",
		"dummy",
		"placeholder",
		"sample",
		"xxxxxxxxxx",
	}

	for _, invalid := range invalidValues {
		if strings.Contains(contentLower, invalid) {
			return false
		}
	}

	return true
}

// maskCertificate masks a certificate for safe display
func (c *CertificateScanner) maskCertificate(value string) string {
	lines := strings.Split(value, "\n")
	var maskedLines []string

	for _, line := range lines {
		if strings.Contains(line, "-----BEGIN") || strings.Contains(line, "-----END") {
			maskedLines = append(maskedLines, line)
		} else if strings.TrimSpace(line) != "" {
			// Mask the content but keep structure
			if len(line) > 20 {
				maskedLines = append(maskedLines, line[:10]+"..."+line[len(line)-10:])
			} else {
				maskedLines = append(maskedLines, "***")
			}
		} else {
			maskedLines = append(maskedLines, line)
		}
	}

	return strings.Join(maskedLines, "\n")
}

// extractCertificateContext extracts context from certificate
func (c *CertificateScanner) extractCertificateContext(fullBlock string) string {
	lines := strings.Split(fullBlock, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return "Certificate or private key"
}

// calculateCertificateConfidence calculates confidence for certificate detection
func (c *CertificateScanner) calculateCertificateConfidence(secretType scan.SecretType, content, context string) float64 {
	confidence := 0.8 // Base confidence

	// Higher confidence for well-formed PEM blocks
	if strings.Contains(context, "-----BEGIN") && strings.Contains(context, "-----END") {
		confidence = 0.95
	}

	// Adjust based on content length
	if len(content) > 1000 {
		confidence += 0.05
	}

	// Private keys are more critical
	if secretType == scan.SecretTypePrivateKey {
		confidence += 0.05
	}

	// Check for test/example indicators
	contextLower := strings.ToLower(context)
	if strings.Contains(contextLower, "example") ||
		strings.Contains(contextLower, "test") ||
		strings.Contains(contextLower, "dummy") {
		confidence *= 0.2
	}

	// Ensure within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// calculateConfidence calculates overall confidence for the scan result
func (c *CertificateScanner) calculateConfidence(result *scan.ScanResult) float64 {
	if len(result.Secrets) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, secret := range result.Secrets {
		totalConfidence += secret.Confidence
	}

	return totalConfidence / float64(len(result.Secrets))
}
