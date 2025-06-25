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

// APIKeyScanner specializes in detecting various API keys and tokens
type APIKeyScanner struct {
	name     string
	patterns map[string]*APIKeyPattern
	logger   zerolog.Logger
}

// APIKeyPattern represents a pattern for detecting specific API keys
type APIKeyPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Confidence  float64
	Severity    scan.Severity
	Description string
}

// NewAPIKeyScanner creates a new API key scanner
func NewAPIKeyScanner(logger zerolog.Logger) *APIKeyScanner {
	scanner := &APIKeyScanner{
		name:     "api_key_scanner",
		patterns: make(map[string]*APIKeyPattern),
		logger:   logger.With().Str("scanner", "api_key").Logger(),
	}

	scanner.initializePatterns()
	return scanner
}

// GetName returns the scanner name
func (a *APIKeyScanner) GetName() string {
	return a.name
}

// GetScanTypes returns the types of secrets this scanner can detect
func (a *APIKeyScanner) GetScanTypes() []string {
	return []string{
		string(scan.SecretTypeAPIKey),
		string(scan.SecretTypeToken),
	}
}

// IsApplicable determines if this scanner should run
func (a *APIKeyScanner) IsApplicable(content string, contentType scan.ContentType) bool {
	// API key scanner is applicable to most content types
	return true
}

// Scan performs API key scanning
func (a *APIKeyScanner) Scan(ctx context.Context, config scan.ScanConfig) (*scan.ScanResult, error) {
	startTime := time.Now()
	result := &scan.ScanResult{
		Scanner:  a.GetName(),
		Secrets:  make([]scan.Secret, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	lines := strings.Split(config.Content, "\n")

	for lineNum, line := range lines {
		secrets, err := a.scanLineForAPIKeys(line, lineNum+1, config)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Secrets = append(result.Secrets, secrets...)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Confidence = a.calculateConfidence(result)
	result.Metadata["lines_scanned"] = len(lines)
	result.Metadata["patterns_used"] = len(a.patterns)

	return result, nil
}

// scanLineForAPIKeys scans a line for API keys
func (a *APIKeyScanner) scanLineForAPIKeys(line string, lineNum int, config scan.ScanConfig) ([]scan.Secret, error) {
	var secrets []scan.Secret

	for patternName, pattern := range a.patterns {
		matches := pattern.Pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 1 {
				value := match[1] // Primary capture group
				if a.isValidAPIKey(value, patternName) {
					secret := a.createAPIKeySecret(pattern, value, line, lineNum, config)
					secrets = append(secrets, secret)
				}
			}
		}
	}

	return secrets, nil
}

// createAPIKeySecret creates a secret from API key detection
func (a *APIKeyScanner) createAPIKeySecret(
	pattern *APIKeyPattern,
	value, line string,
	lineNum int,
	config scan.ScanConfig,
) scan.Secret {

	// Calculate confidence based on pattern and value characteristics
	confidence := a.calculateAPIKeyConfidence(pattern, value, line)

	secret := scan.Secret{
		Type:        scan.SecretTypeAPIKey,
		Value:       value,
		MaskedValue: scan.MaskSecret(value),
		Location: &scan.Location{
			File:   config.FilePath,
			Line:   lineNum,
			Column: strings.Index(line, value) + 1,
		},
		Confidence: confidence,
		Severity:   a.getAPIKeySeverity(pattern, confidence),
		Context:    strings.TrimSpace(line),
		Pattern:    pattern.Name,
		Entropy:    scan.CalculateEntropy(value),
		Metadata: map[string]interface{}{
			"detection_method":   "api_key_pattern",
			"api_service":        pattern.Name,
			"pattern_confidence": pattern.Confidence,
			"value_length":       len(value),
		},
		Evidence: []scan.Evidence{
			{
				Type:        "api_key_pattern",
				Description: fmt.Sprintf("Matched %s API key pattern", pattern.Name),
				Value:       value,
				Pattern:     pattern.Pattern.String(),
				Context:     line,
			},
		},
	}

	return secret
}

// initializePatterns initializes patterns for various API key services
func (a *APIKeyScanner) initializePatterns() {
	patterns := map[string]string{
		// GitHub
		"GitHub":              `(?i)(?:github|gh)[_-]?(?:token|key)[\"'\s]*[:=][\"'\s]*([a-zA-Z0-9_]{36,40})`,
		"GitHub_Classic":      `ghp_[a-zA-Z0-9]{36}`,
		"GitHub_Fine_Grained": `github_pat_[a-zA-Z0-9_]{82}`,

		// AWS
		"AWS_Access_Key": `AKIA[0-9A-Z]{16}`,
		"AWS_Secret_Key": `(?i)aws[_-]?secret[_-]?(?:access[_-]?)?key[\"'\s]*[:=][\"'\s]*([a-zA-Z0-9/+]{40})`,

		// Google
		"Google_API":   `AIza[0-9A-Za-z\\-_]{35}`,
		"Google_OAuth": `ya29\\.[0-9A-Za-z\\-_]+`,

		// Slack
		"Slack_Token":   `xox[baprs]-[0-9]{12}-[0-9]{12}-[0-9a-zA-Z]{24}`,
		"Slack_Webhook": `https://hooks\\.slack\\.com/services/[A-Z0-9]{9}/[A-Z0-9]{9}/[a-zA-Z0-9]{24}`,

		// Discord
		"Discord_Bot":     `[MN][a-zA-Z\\d]{23}\\.[\\w-]{6}\\.[\\w-]{27}`,
		"Discord_Webhook": `https://discord(?:app)?\\.com/api/webhooks/\\d+/[A-Za-z0-9\\-_]+`,

		// Stripe
		"Stripe_Publishable": `pk_live_[0-9a-zA-Z]{24}`,
		"Stripe_Secret":      `sk_live_[0-9a-zA-Z]{24}`,

		// Twilio
		"Twilio_SID":  `AC[a-zA-Z0-9_\\-]{32}`,
		"Twilio_Auth": `(?i)twilio[_-]?auth[_-]?token[\"'\s]*[:=][\"'\s]*([a-f0-9]{32})`,

		// SendGrid
		"SendGrid": `SG\\.[a-zA-Z0-9_\\-]{22}\\.[a-zA-Z0-9_\\-]{43}`,

		// Mailgun
		"Mailgun": `key-[a-f0-9]{32}`,

		// JWT Tokens
		"JWT": `eyJ[a-zA-Z0-9_\\-]*\\.[a-zA-Z0-9_\\-]*\\.[a-zA-Z0-9_\\-]*`,

		// Generic OAuth
		"OAuth_Token": `(?i)oauth[_-]?(?:token|key)[\"'\s]*[:=][\"'\s]*([a-zA-Z0-9_\\-\\.]{20,128})`,

		// Generic Bearer Token
		"Bearer_Token": `(?i)bearer[\"'\s]+([a-zA-Z0-9_\\-\\.]{20,128})`,

		// Generic API Key
		"Generic_API_Key": `(?i)(?:api[_-]?key|apikey)[\"'\s]*[:=][\"'\s]*([a-zA-Z0-9_\\-\\.]{20,128})`,
	}

	for name, patternStr := range patterns {
		compiled, err := regexp.Compile(patternStr)
		if err != nil {
			a.logger.Error().Err(err).Str("pattern", name).Msg("Failed to compile API key pattern")
			continue
		}

		confidence := a.getPatternConfidence(name)
		severity := a.getPatternSeverity(name)

		a.patterns[name] = &APIKeyPattern{
			Name:        name,
			Pattern:     compiled,
			Confidence:  confidence,
			Severity:    severity,
			Description: fmt.Sprintf("%s API key or token", name),
		}
	}

	a.logger.Debug().Int("patterns", len(a.patterns)).Msg("Initialized API key patterns")
}

// getPatternConfidence returns base confidence for different pattern types
func (a *APIKeyScanner) getPatternConfidence(patternName string) float64 {
	confidenceMap := map[string]float64{
		"GitHub_Classic":      0.95,
		"GitHub_Fine_Grained": 0.95,
		"AWS_Access_Key":      0.90,
		"Google_API":          0.90,
		"Slack_Token":         0.90,
		"Discord_Bot":         0.85,
		"Stripe_Publishable":  0.85,
		"Stripe_Secret":       0.90,
		"JWT":                 0.80,
		"Bearer_Token":        0.70,
		"Generic_API_Key":     0.60,
	}

	if confidence, exists := confidenceMap[patternName]; exists {
		return confidence
	}
	return 0.70 // Default confidence
}

// getPatternSeverity returns severity for different pattern types
func (a *APIKeyScanner) getPatternSeverity(patternName string) scan.Severity {
	severityMap := map[string]scan.Severity{
		"AWS_Secret_Key":      scan.SeverityCritical,
		"Stripe_Secret":       scan.SeverityCritical,
		"GitHub_Classic":      scan.SeverityHigh,
		"GitHub_Fine_Grained": scan.SeverityHigh,
		"Google_API":          scan.SeverityHigh,
		"Slack_Token":         scan.SeverityHigh,
		"Discord_Bot":         scan.SeverityMedium,
		"JWT":                 scan.SeverityMedium,
		"Bearer_Token":        scan.SeverityMedium,
	}

	if severity, exists := severityMap[patternName]; exists {
		return severity
	}
	return scan.SeverityMedium // Default severity
}

// isValidAPIKey performs additional validation on detected API keys
func (a *APIKeyScanner) isValidAPIKey(value, patternName string) bool {
	// Basic length checks
	if len(value) < 8 {
		return false
	}

	// Check for obvious test/example values
	valueLower := strings.ToLower(value)
	invalidValues := []string{
		"your_api_key_here",
		"api_key_placeholder",
		"example_key",
		"test_key",
		"dummy_key",
		"sample_key",
		"replace_with_your_key",
		"xxxxxxxxxx",
	}

	for _, invalid := range invalidValues {
		if strings.Contains(valueLower, invalid) {
			return false
		}
	}

	// Pattern-specific validation
	switch patternName {
	case "AWS_Access_Key":
		return len(value) == 20 && strings.HasPrefix(value, "AKIA")
	case "Google_API":
		return len(value) == 39 && strings.HasPrefix(value, "AIza")
	case "GitHub_Classic":
		return len(value) == 40 && strings.HasPrefix(value, "ghp_")
	case "JWT":
		return strings.Count(value, ".") == 2
	}

	return true
}

// calculateAPIKeyConfidence calculates confidence for an API key detection
func (a *APIKeyScanner) calculateAPIKeyConfidence(pattern *APIKeyPattern, value, context string) float64 {
	confidence := pattern.Confidence

	// Adjust based on context
	contextLower := strings.ToLower(context)
	if strings.Contains(contextLower, "example") ||
		strings.Contains(contextLower, "test") ||
		strings.Contains(contextLower, "dummy") {
		confidence *= 0.3
	}

	// Boost confidence for specific patterns
	if strings.Contains(pattern.Name, "GitHub") ||
		strings.Contains(pattern.Name, "AWS") ||
		strings.Contains(pattern.Name, "Google") {
		confidence += 0.1
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

// getAPIKeySeverity determines severity for an API key
func (a *APIKeyScanner) getAPIKeySeverity(pattern *APIKeyPattern, confidence float64) scan.Severity {
	baseSeverity := pattern.Severity

	// Reduce severity for low confidence
	if confidence < 0.5 {
		switch baseSeverity {
		case scan.SeverityCritical:
			return scan.SeverityHigh
		case scan.SeverityHigh:
			return scan.SeverityMedium
		case scan.SeverityMedium:
			return scan.SeverityLow
		default:
			return scan.SeverityInfo
		}
	}

	return baseSeverity
}

// calculateConfidence calculates overall confidence for the scan result
func (a *APIKeyScanner) calculateConfidence(result *scan.ScanResult) float64 {
	if len(result.Secrets) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, secret := range result.Secrets {
		totalConfidence += secret.Confidence
	}

	return totalConfidence / float64(len(result.Secrets))
}
