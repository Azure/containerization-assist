package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

type ProductionSecretScanner struct {
	logger            zerolog.Logger
	gitleaksAvailable bool
	customPatterns    []*SecretPattern
	entropyThreshold  float64
	minSecretLength   int
}

type SecretPattern struct {
	ID          string
	Description string
	Regex       *regexp.Regexp
	Entropy     float64
	Keywords    []string
	Severity    string
	Confidence  int
}

type DetectedSecret struct {
	Type        string  `json:"type"`
	Value       string  `json:"value"`
	Redacted    string  `json:"redacted"`
	Pattern     string  `json:"pattern"`
	Line        int     `json:"line"`
	Column      int     `json:"column"`
	File        string  `json:"file"`
	Severity    string  `json:"severity"`
	Confidence  int     `json:"confidence"`
	Entropy     float64 `json:"entropy"`
	Context     string  `json:"context"`
	Fingerprint string  `json:"fingerprint"`
	IsVerified  bool    `json:"is_verified"`
}

type GitLeaksResult struct {
	Description string   `json:"Description"`
	StartLine   int      `json:"StartLine"`
	EndLine     int      `json:"EndLine"`
	StartColumn int      `json:"StartColumn"`
	EndColumn   int      `json:"EndColumn"`
	Match       string   `json:"Match"`
	Secret      string   `json:"Secret"`
	File        string   `json:"File"`
	SymlinkFile string   `json:"SymlinkFile"`
	Commit      string   `json:"Commit"`
	Entropy     float64  `json:"Entropy"`
	Author      string   `json:"Author"`
	Email       string   `json:"Email"`
	Date        string   `json:"Date"`
	Message     string   `json:"Message"`
	Tags        []string `json:"Tags"`
	RuleID      string   `json:"RuleID"`
	Fingerprint string   `json:"Fingerprint"`
}

func NewProductionSecretScanner(logger zerolog.Logger) *ProductionSecretScanner {
	scanner := &ProductionSecretScanner{
		logger:           logger.With().Str("component", "production_secret_scanner").Logger(),
		entropyThreshold: 4.5,
		minSecretLength:  8,
	}

	scanner.gitleaksAvailable = scanner.checkGitleaksAvailability()

	scanner.customPatterns = scanner.initializeCustomPatterns()

	return scanner
}

func (pss *ProductionSecretScanner) ScanWithGitleaks(ctx context.Context, path string) ([]DetectedSecret, error) {
	if !pss.gitleaksAvailable {
		pss.logger.Debug().Msg("GitLeaks not available, falling back to custom patterns")
		return pss.ScanWithCustomPatterns(path)
	}

	pss.logger.Info().Str("path", path).Msg("Running GitLeaks scan")

	cmd := exec.CommandContext(ctx, "gitleaks", "detect", "--source", path, "--format", "json", "--no-git")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			output = exitErr.Stderr
		}
	}

	var gitleaksResults []GitLeaksResult
	if err := json.Unmarshal(output, &gitleaksResults); err != nil {
		pss.logger.Warn().Err(err).Msg("Failed to parse GitLeaks output, using custom patterns")
		return pss.ScanWithCustomPatterns(path)
	}

	var secrets []DetectedSecret
	for _, result := range gitleaksResults {
		secret := DetectedSecret{
			Type:        result.RuleID,
			Value:       result.Secret,
			Redacted:    pss.redactSecret(result.Secret),
			Pattern:     result.RuleID,
			Line:        result.StartLine,
			Column:      result.StartColumn,
			File:        result.File,
			Severity:    pss.classifySeverity(result.RuleID, result.Secret),
			Confidence:  pss.calculateConfidence(result.RuleID, result.Secret, result.Entropy),
			Entropy:     result.Entropy,
			Context:     result.Match,
			Fingerprint: result.Fingerprint,
			IsVerified:  false,
		}
		secrets = append(secrets, secret)
	}

	pss.logger.Info().Int("secrets_found", len(secrets)).Msg("GitLeaks scan completed")
	return secrets, nil
}

func (pss *ProductionSecretScanner) ScanWithCustomPatterns(path string) ([]DetectedSecret, error) {
	pss.logger.Info().Str("path", path).Msg("Running custom pattern scan")

	var secrets []DetectedSecret

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			pss.logger.Warn().Err(err).Str("file", filePath).Msg("Error accessing file")
			return nil
		}
		if info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			pss.logger.Warn().Err(err).Str("file", filePath).Msg("Error reading file")
			return nil
		}
		for _, pattern := range pss.customPatterns {
			pss.logger.Debug().Str("pattern", pattern.ID).Str("file", filePath).Msg("Checking pattern")
			if pattern.Regex == nil {
				pss.logger.Warn().Str("pattern", pattern.ID).Msg("Pattern regex is nil")
				continue
			}
			matches := pattern.Regex.FindAllString(string(content), -1)
			for _, match := range matches {
				secret := DetectedSecret{
					Type:       pattern.ID,
					Value:      match,
					Redacted:   pss.redactSecret(match),
					Pattern:    pattern.Regex.String(),
					File:       filePath,
					Line:       -1,
					Column:     -1,
					Confidence: pattern.Confidence,
					IsVerified: false,
				}
				secrets = append(secrets, secret)
			}
		}
		return nil
	})
	if err != nil {
		pss.logger.Error().Err(err).Msg("Error during file traversal")
		return nil, err
	}
	pss.logger.Info().Int("secrets_found", len(secrets)).Msg("Custom pattern scan completed")

	return secrets, nil
}

func (pss *ProductionSecretScanner) VerifySecret(ctx context.Context, secret DetectedSecret) bool {
	switch secret.Type {
	case "github-pat", "github-fine-grained-pat":
		return pss.verifyGitHubToken(ctx, secret.Value)
	case "aws-access-token":
		return pss.verifyAWSKey(ctx, secret.Value)
	case "google-api-key":
		return pss.verifyGoogleAPIKey(ctx, secret.Value)
	default:
		return false
	}
}

func (pss *ProductionSecretScanner) calculateEntropy(data string) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, char := range data {
		freq[char]++
	}

	entropy := 0.0
	length := float64(len(data))
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

func (pss *ProductionSecretScanner) checkGitleaksAvailability() bool {
	cmd := exec.Command("gitleaks", "version")
	err := cmd.Run()
	available := err == nil
	pss.logger.Info().Bool("available", available).Msg("GitLeaks availability check")
	return available
}

func (pss *ProductionSecretScanner) initializeCustomPatterns() []*SecretPattern {
	patterns := []*SecretPattern{
		{
			ID:          "github-pat",
			Description: "GitHub Personal Access Token",
			Regex:       regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`),
			Entropy:     4.0,
			Keywords:    []string{"github", "token", "pat"},
			Severity:    "high",
			Confidence:  95,
		},
		{
			ID:          "github-fine-grained-pat",
			Description: "GitHub Fine-grained Personal Access Token",
			Regex:       regexp.MustCompile(`github_pat_[0-9a-zA-Z_]{82}`),
			Entropy:     4.5,
			Keywords:    []string{"github", "token", "pat"},
			Severity:    "high",
			Confidence:  95,
		},
		{
			ID:          "aws-access-token",
			Description: "AWS Access Key ID",
			Regex:       regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			Entropy:     3.5,
			Keywords:    []string{"aws", "access", "key"},
			Severity:    "critical",
			Confidence:  90,
		},
		{
			ID:          "aws-secret-key",
			Description: "AWS Secret Access Key",
			Regex:       regexp.MustCompile(`(?i)[0-9a-z/+=]{40}`),
			Entropy:     4.8,
			Keywords:    []string{"aws", "secret", "key"},
			Severity:    "critical",
			Confidence:  75,
		},
		{
			ID:          "google-api-key",
			Description: "Google API Key",
			Regex:       regexp.MustCompile(`AIza[0-9A-Za-z\\-_]{35}`),
			Entropy:     4.0,
			Keywords:    []string{"google", "api", "key"},
			Severity:    "high",
			Confidence:  90,
		},
		{
			ID:          "slack-token",
			Description: "Slack Token",
			Regex:       regexp.MustCompile(`xox[baprs]-[0-9]{12}-[0-9]{12}-[0-9a-zA-Z]{24,32}`),
			Entropy:     4.2,
			Keywords:    []string{"slack", "token"},
			Severity:    "medium",
			Confidence:  90,
		},
		{
			ID:          "discord-token",
			Description: "Discord Bot Token",
			Regex:       regexp.MustCompile(`[MN][A-Za-z\\d]{23}\\.[\\w-]{6}\\.[\\w-]{27}`),
			Entropy:     4.5,
			Keywords:    []string{"discord", "bot", "token"},
			Severity:    "medium",
			Confidence:  85,
		},
		{
			ID:          "stripe-api-key",
			Description: "Stripe API Key",
			Regex:       regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24,34}`),
			Entropy:     4.0,
			Keywords:    []string{"stripe", "api", "key"},
			Severity:    "critical",
			Confidence:  95,
		},
		{
			ID:          "jwt-token",
			Description: "JSON Web Token",
			Regex:       regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\\.eyJ[A-Za-z0-9_-]*\\.[A-Za-z0-9_-]*`),
			Entropy:     4.0,
			Keywords:    []string{"jwt", "token", "bearer"},
			Severity:    "medium",
			Confidence:  80,
		},
		{
			ID:          "generic-high-entropy",
			Description: "Generic High Entropy String",
			Regex:       regexp.MustCompile(`[A-Za-z0-9+/=]{32,}`),
			Entropy:     5.0,
			Keywords:    []string{"secret", "key", "token", "password"},
			Severity:    "low",
			Confidence:  60,
		},
	}

	pss.logger.Info().Int("pattern_count", len(patterns)).Msg("Initialized custom secret patterns")
	return patterns
}

func (pss *ProductionSecretScanner) classifySeverity(ruleID, secret string) string {
	for _, pattern := range pss.customPatterns {
		if pattern.ID == ruleID {
			return pattern.Severity
		}
	}

	secretLower := strings.ToLower(secret)
	switch {
	case strings.Contains(secretLower, "aws") || strings.Contains(secretLower, "stripe"):
		return "critical"
	case strings.Contains(secretLower, "github") || strings.Contains(secretLower, "google"):
		return "high"
	case strings.Contains(secretLower, "slack") || strings.Contains(secretLower, "discord"):
		return "medium"
	default:
		return "low"
	}
}

func (pss *ProductionSecretScanner) calculateConfidence(ruleID, secret string, entropy float64) int {
	baseConfidence := 50

	for _, pattern := range pss.customPatterns {
		if pattern.ID == ruleID {
			baseConfidence = pattern.Confidence
			break
		}
	}

	if entropy >= pss.entropyThreshold {
		baseConfidence += 20
	}

	if len(secret) >= 32 {
		baseConfidence += 10
	}

	if baseConfidence > 100 {
		baseConfidence = 100
	}

	return baseConfidence
}

func (pss *ProductionSecretScanner) redactSecret(secret string) string {
	if len(secret) <= 6 {
		return "***"
	}
	return secret[:3] + "***" + secret[len(secret)-3:]
}

func (pss *ProductionSecretScanner) verifyGitHubToken(_ context.Context, _ string) bool {
	pss.logger.Debug().Msg("GitHub token verification not implemented for security")
	return false
}

func (pss *ProductionSecretScanner) verifyAWSKey(_ context.Context, _ string) bool {
	pss.logger.Debug().Msg("AWS key verification not implemented for security")
	return false
}

func (pss *ProductionSecretScanner) verifyGoogleAPIKey(_ context.Context, _ string) bool {
	pss.logger.Debug().Msg("Google API key verification not implemented for security")
	return false
}

func (pss *ProductionSecretScanner) GenerateFingerprint(secret, file string, line int) string {
	data := fmt.Sprintf("%s:%s:%d", secret, file, line)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

func (pss *ProductionSecretScanner) IsHighEntropyString(data string) bool {
	if len(data) < pss.minSecretLength {
		return false
	}

	entropy := pss.calculateEntropy(data)
	return entropy >= pss.entropyThreshold
}

func (pss *ProductionSecretScanner) FilterFalsePositives(secrets []DetectedSecret) []DetectedSecret {
	var filtered []DetectedSecret

	for _, secret := range secrets {
		if pss.isLikelyFalsePositive(secret) {
			pss.logger.Debug().Str("type", secret.Type).Str("value", secret.Redacted).Msg("Filtered false positive")
			continue
		}
		filtered = append(filtered, secret)
	}

	pss.logger.Info().Int("original", len(secrets)).Int("filtered", len(filtered)).Msg("False positive filtering complete")
	return filtered
}

func (pss *ProductionSecretScanner) isLikelyFalsePositive(secret DetectedSecret) bool {
	valueLower := strings.ToLower(secret.Value)
	contextLower := strings.ToLower(secret.Context)

	falsePositives := []string{
		"test", "example", "dummy", "fake", "sample", "placeholder",
		"xxx", "yyy", "zzz", "000", "123", "abc",
		"localhost", "127.0.0.1", "0.0.0.0",
		"null", "none", "empty", "default",
	}

	for _, fp := range falsePositives {
		if strings.Contains(valueLower, fp) || strings.Contains(contextLower, fp) {
			return true
		}
	}

	if strings.Contains(secret.File, "test") ||
		strings.Contains(secret.File, "spec") ||
		strings.Contains(secret.File, "mock") {
		return true
	}

	return false
}
