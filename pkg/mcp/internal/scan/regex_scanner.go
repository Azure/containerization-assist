package scan

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// RegexBasedScanner implements secret detection using regular expressions
type RegexBasedScanner struct {
	name     string
	patterns map[SecretType]*regexp.Regexp
	logger   zerolog.Logger
}

// NewRegexBasedScanner creates a new regex-based scanner
func NewRegexBasedScanner(logger zerolog.Logger) *RegexBasedScanner {
	scanner := &RegexBasedScanner{
		name:     "regex_scanner",
		patterns: make(map[SecretType]*regexp.Regexp),
		logger:   logger.With().Str("scanner", "regex").Logger(),
	}

	scanner.initializePatterns()
	return scanner
}

// GetName returns the scanner name
func (r *RegexBasedScanner) GetName() string {
	return r.name
}

// GetScanTypes returns the types of secrets this scanner can detect
func (r *RegexBasedScanner) GetScanTypes() []string {
	return []string{
		string(SecretTypeAPIKey),
		string(SecretTypePassword),
		string(SecretTypeToken),
		string(SecretTypeCredential),
		string(SecretTypeSecret),
		string(SecretTypeEnvironmentVar),
	}
}

// IsApplicable determines if this scanner should run
func (r *RegexBasedScanner) IsApplicable(content string, contentType ContentType) bool {
	// Regex scanner is applicable to most content types
	switch contentType {
	case ContentTypeSourceCode, ContentTypeConfig,
		ContentTypeEnvironment, ContentTypeGeneric:
		return true
	default:
		return false
	}
}

// Scan performs regex-based secret scanning
func (r *RegexBasedScanner) Scan(ctx context.Context, config ScanConfig) (*ScanResult, error) {
	startTime := time.Now()
	result := &ScanResult{
		Scanner:  r.GetName(),
		Secrets:  make([]Secret, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Split content into lines for line-by-line analysis
	lines := strings.Split(config.Content, "\n")

	for lineNum, line := range lines {
		secrets, err := r.scanLine(line, lineNum+1, config)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Secrets = append(result.Secrets, secrets...)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Confidence = r.calculateConfidence(result)
	result.Metadata["lines_scanned"] = len(lines)
	result.Metadata["patterns_used"] = len(r.patterns)

	return result, nil
}

// scanLine scans a single line for secrets
func (r *RegexBasedScanner) scanLine(line string, lineNum int, config ScanConfig) ([]Secret, error) {
	var secrets []Secret

	for secretType, pattern := range r.patterns {
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 1 {
				value := match[1] // Capture group
				if len(value) > 0 {
					secret := r.createSecret(secretType, value, line, lineNum, config)
					secrets = append(secrets, secret)
				}
			}
		}
	}

	// Additional high-entropy detection
	if config.Options.IncludeHighEntropy {
		entropySecrets := r.detectHighEntropy(line, lineNum, config)
		secrets = append(secrets, entropySecrets...)
	}

	return secrets, nil
}

// createSecret creates a secret from detection results
func (r *RegexBasedScanner) createSecret(
	secretType SecretType,
	value, line string,
	lineNum int,
	config ScanConfig,
) Secret {

	// Calculate confidence based on various factors
	confidence := r.calculateSecretConfidence(secretType, value, line)

	// Determine severity
	severity := GetSecretSeverity(secretType, confidence)

	// Calculate entropy
	entropy := CalculateEntropy(value)

	secret := Secret{
		Type:        secretType,
		Value:       value,
		MaskedValue: MaskSecret(value),
		Location: &Location{
			File:   config.FilePath,
			Line:   lineNum,
			Column: strings.Index(line, value) + 1,
		},
		Confidence: confidence,
		Severity:   severity,
		Context:    strings.TrimSpace(line),
		Pattern:    r.getPatternString(secretType),
		Entropy:    entropy,
		Metadata: map[string]interface{}{
			"detection_method": "regex",
			"line_length":      len(line),
			"value_length":     len(value),
		},
		Evidence: []Evidence{
			{
				Type:        "regex_match",
				Description: fmt.Sprintf("Matched %s pattern", secretType),
				Value:       value,
				Pattern:     r.getPatternString(secretType),
				Context:     line,
			},
		},
	}

	return secret
}

// detectHighEntropy detects high-entropy strings that might be secrets
func (r *RegexBasedScanner) detectHighEntropy(line string, lineNum int, config ScanConfig) []Secret {
	var secrets []Secret

	// Split line into potential secret tokens
	tokens := r.extractTokens(line)

	for _, token := range tokens {
		if len(token) >= 16 && len(token) <= 100 { // Reasonable secret length
			entropy := CalculateEntropy(token)
			if entropy > 4.5 { // High entropy threshold
				confidence := r.calculateEntropyConfidence(entropy, token)
				if confidence > 0.6 {
					secret := Secret{
						Type:        SecretTypeHighEntropy,
						Value:       token,
						MaskedValue: MaskSecret(token),
						Location: &Location{
							File:   config.FilePath,
							Line:   lineNum,
							Column: strings.Index(line, token) + 1,
						},
						Confidence: confidence,
						Severity:   GetSecretSeverity(SecretTypeHighEntropy, confidence),
						Context:    strings.TrimSpace(line),
						Pattern:    "high_entropy",
						Entropy:    entropy,
						Metadata: map[string]interface{}{
							"detection_method": "entropy",
							"entropy_score":    entropy,
							"token_length":     len(token),
						},
						Evidence: []Evidence{
							{
								Type:        "entropy_analysis",
								Description: fmt.Sprintf("High entropy string (%.2f)", entropy),
								Value:       token,
								Pattern:     "entropy > 4.5",
								Context:     line,
							},
						},
					}
					secrets = append(secrets, secret)
				}
			}
		}
	}

	return secrets
}

// extractTokens extracts potential secret tokens from a line
func (r *RegexBasedScanner) extractTokens(line string) []string {
	// Extract quoted strings, assignment values, etc.
	tokenPatterns := []*regexp.Regexp{
		regexp.MustCompile(`["']([^"']{16,100})["']`),                                      // Quoted strings
		regexp.MustCompile(`(?i)(?:key|token|secret|password)\s*[:=]\s*([^\s"']{16,100})`), // Key-value pairs
		regexp.MustCompile(`[a-zA-Z0-9+/]{20,100}={0,2}`),                                  // Base64-like
		regexp.MustCompile(`[a-fA-F0-9]{32,128}`),                                          // Hex strings
	}

	var tokens []string
	for _, pattern := range tokenPatterns {
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tokens = append(tokens, match[1])
			} else if len(match) > 0 {
				tokens = append(tokens, match[0])
			}
		}
	}

	return tokens
}

// initializePatterns initializes regex patterns for different secret types
func (r *RegexBasedScanner) initializePatterns() {
	patterns := map[SecretType]string{
		// API Keys
		SecretTypeAPIKey: `(?i)(?:api[_-]?key|apikey)[\"'\s]*[:=][\"'\s]*([a-zA-Z0-9_\-]{16,64})`,

		// Generic tokens
		SecretTypeToken: `(?i)(?:token|access[_-]?token)[\"'\s]*[:=][\"'\s]*([a-zA-Z0-9_\-\.]{20,128})`,

		// Passwords
		SecretTypePassword: `(?i)(?:password|passwd|pwd)[\"'\s]*[:=][\"'\s]*([^\s\"']{8,64})`,

		// Generic secrets
		SecretTypeSecret: `(?i)(?:secret|client[_-]?secret)[\"'\s]*[:=][\"'\s]*([a-zA-Z0-9_\-]{16,128})`,

		// Environment variables with secret-like names
		SecretTypeEnvironmentVar: `(?i)(?:SECRET|KEY|TOKEN|PASSWORD)_[A-Z0-9_]*[\"'\s]*[:=][\"'\s]*([^\s\"']{8,128})`,

		// Generic credentials
		SecretTypeCredential: `(?i)(?:credential|cred)[\"'\s]*[:=][\"'\s]*([^\s\"']{8,64})`,
	}

	for secretType, patternStr := range patterns {
		compiled, err := regexp.Compile(patternStr)
		if err != nil {
			r.logger.Error().Err(err).Str("pattern", patternStr).Msg("Failed to compile regex pattern")
			continue
		}
		r.patterns[secretType] = compiled
	}

	r.logger.Debug().Int("patterns", len(r.patterns)).Msg("Initialized regex patterns")
}

// calculateSecretConfidence calculates confidence for a detected secret
func (r *RegexBasedScanner) calculateSecretConfidence(secretType SecretType, value, context string) float64 {
	confidence := 0.5 // Base confidence

	// Adjust based on value characteristics
	if len(value) >= 20 {
		confidence += 0.1
	}
	if len(value) >= 32 {
		confidence += 0.1
	}

	// Check for mixed case
	if strings.ToLower(value) != value && strings.ToUpper(value) != value {
		confidence += 0.1
	}

	// Check for numbers
	if regexp.MustCompile(`\d`).MatchString(value) {
		confidence += 0.1
	}

	// Check for special characters
	if regexp.MustCompile(`[_\-\.]`).MatchString(value) {
		confidence += 0.05
	}

	// Context-based adjustments
	contextLower := strings.ToLower(context)
	if strings.Contains(contextLower, "example") ||
		strings.Contains(contextLower, "test") ||
		strings.Contains(contextLower, "dummy") ||
		strings.Contains(contextLower, "placeholder") {
		confidence -= 0.3
	}

	// Check for obvious non-secrets
	valueLower := strings.ToLower(value)
	if valueLower == "password" ||
		valueLower == "secret" ||
		valueLower == "token" ||
		valueLower == "your_api_key_here" ||
		strings.HasPrefix(valueLower, "xxx") {
		confidence = 0.1
	}

	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// calculateEntropyConfidence calculates confidence based on entropy
func (r *RegexBasedScanner) calculateEntropyConfidence(entropy float64, value string) float64 {
	// Base confidence from entropy
	confidence := (entropy - 4.0) / 4.0 // Scale from 4.0-8.0 to 0.0-1.0

	// Adjust based on length
	if len(value) < 16 {
		confidence -= 0.2
	}
	if len(value) > 64 {
		confidence -= 0.1
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
func (r *RegexBasedScanner) calculateConfidence(result *ScanResult) float64 {
	if len(result.Secrets) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, secret := range result.Secrets {
		totalConfidence += secret.Confidence
	}

	return totalConfidence / float64(len(result.Secrets))
}

// getPatternString returns the pattern string for a secret type
func (r *RegexBasedScanner) getPatternString(secretType SecretType) string {
	if pattern, exists := r.patterns[secretType]; exists {
		return pattern.String()
	}
	return "unknown"
}
