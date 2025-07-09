// Package security provides comprehensive secret detection and security scanning capabilities
package security

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// SecretDiscovery provides comprehensive secret detection capabilities
type SecretDiscovery struct {
	logger          zerolog.Logger
	patternDetector *PatternDetector
	entropyDetector *EntropyDetector
	fileTypeHandler *FileTypeHandler
	exclusions      *ExclusionManager
	results         *sync.Map
}

// NewSecretDiscovery creates a new secret discovery engine
func NewSecretDiscovery(logger zerolog.Logger) *SecretDiscovery {
	return &SecretDiscovery{
		logger:          logger.With().Str("component", "secret_discovery").Logger(),
		patternDetector: NewPatternDetector(),
		entropyDetector: NewEntropyDetector(),
		fileTypeHandler: NewFileTypeHandler(),
		exclusions:      NewExclusionManager(),
		results:         &sync.Map{},
	}
}

// DiscoveryResult represents the result of a secret discovery scan
type DiscoveryResult struct {
	StartTime    time.Time               `json:"start_time"`
	EndTime      time.Time               `json:"end_time"`
	Duration     time.Duration           `json:"duration"`
	FilesScanned int                     `json:"files_scanned"`
	Findings     []ExtendedSecretFinding `json:"findings"`
	Summary      DiscoverySummary        `json:"summary"`
	RiskScore    int                     `json:"risk_score"`
}

// SecretFinding represents a discovered secret
type SecretFinding struct {
	Type        string  `json:"type"`
	File        string  `json:"file"`
	Line        int     `json:"line"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
	RuleID      string  `json:"rule_id"`
}

// ExtendedSecretFinding provides additional fields for rich secret detection
type ExtendedSecretFinding struct {
	SecretFinding
	ID            string                 `json:"id"`
	Column        int                    `json:"column"`
	Severity      string                 `json:"severity"`
	Match         string                 `json:"match"`
	Redacted      string                 `json:"redacted"`
	Context       string                 `json:"context"`
	Entropy       float64                `json:"entropy,omitempty"`
	Pattern       string                 `json:"pattern,omitempty"`
	Verified      bool                   `json:"verified"`
	FalsePositive bool                   `json:"false_positive"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ToSecretFinding converts ExtendedSecretFinding to SecretFinding
func (esf ExtendedSecretFinding) ToSecretFinding() SecretFinding {
	return esf.SecretFinding
}

// DiscoverySummary provides a summary of findings
type DiscoverySummary struct {
	TotalFindings    int            `json:"total_findings"`
	BySeverity       map[string]int `json:"by_severity"`
	ByType           map[string]int `json:"by_type"`
	ByFile           map[string]int `json:"by_file"`
	VerifiedFindings int            `json:"verified_findings"`
	FalsePositives   int            `json:"false_positives"`
	UniqueSecrets    int            `json:"unique_secrets"`
}

// ScanDirectory scans a directory for secrets
func (sd *SecretDiscovery) ScanDirectory(ctx context.Context, path string, options ScanOptions) (*DiscoveryResult, error) {
	startTime := time.Now()
	result := &DiscoveryResult{
		StartTime: startTime,
		Findings:  make([]ExtendedSecretFinding, 0),
		Summary: DiscoverySummary{
			BySeverity: make(map[string]int),
			ByType:     make(map[string]int),
			ByFile:     make(map[string]int),
		},
	}

	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, mcperrors.NewError().Messagef("directory does not exist: %s", path).WithLocation().Build()
	}

	sd.logger.Info().
		Str("path", path).
		Bool("recursive", options.Recursive).
		Strs("file_types", options.FileTypes).
		Msg("Starting secret discovery scan")

	// Walk through directory
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, options.MaxConcurrency)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if we should process this file
		if !sd.shouldProcessFile(filePath, info, options) {
			return nil
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			findings := sd.scanFile(ctx, filePath, options)

			// Thread-safe updates to result
			mu.Lock()
			if len(findings) > 0 {
				for _, finding := range findings {
					sd.results.Store(finding.ID, finding)
					result.Findings = append(result.Findings, finding)
				}
			}
			result.FilesScanned++
			mu.Unlock()
		}()

		return nil
	})

	if err != nil {
		return nil, mcperrors.NewError().Messagef("failed to walk directory: %w", err).WithLocation().Build()
	}

	wg.Wait()

	sd.processResults(result)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	result.RiskScore = sd.calculateRiskScore(result)

	sd.logger.Info().
		Int("files_scanned", result.FilesScanned).
		Int("findings", len(result.Findings)).
		Int("risk_score", result.RiskScore).
		Dur("duration", result.Duration).
		Msg("Secret discovery scan completed")

	return result, nil
}

// ScanFile scans a single file for secrets
func (sd *SecretDiscovery) scanFile(_ context.Context, filePath string, options ScanOptions) []ExtendedSecretFinding {
	// nolint:gosec // filePath is controlled within the scanning logic
	file, err := os.Open(filePath)
	if err != nil {
		sd.logger.Debug().Err(err).Str("file", filePath).Msg("Failed to open file")
		return nil
	}
	defer func() { _ = file.Close() }()

	var findings []ExtendedSecretFinding
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Pattern-based detection
		if patternFindings := sd.patternDetector.Scan(line, filePath, lineNumber); len(patternFindings) > 0 {
			findings = append(findings, patternFindings...)
		}

		// Entropy-based detection
		if options.EnableEntropyDetection {
			if entropyFindings := sd.entropyDetector.Scan(line, filePath, lineNumber); len(entropyFindings) > 0 {
				findings = append(findings, entropyFindings...)
			}
		}
	}

	// Filter and verify findings
	filteredFindings := sd.filterFindings(findings, options)

	if options.VerifyFindings {
		sd.verifyFindings(filteredFindings)
	}

	return filteredFindings
}

// shouldProcessFile determines if a file should be scanned
func (sd *SecretDiscovery) shouldProcessFile(filePath string, info os.FileInfo, options ScanOptions) bool {
	// Skip directories
	if info.IsDir() {
		return false
	}

	// Skip excluded paths
	if sd.exclusions.IsExcluded(filePath) {
		return false
	}

	// Check file size limit
	if options.MaxFileSize > 0 && info.Size() > options.MaxFileSize {
		sd.logger.Debug().
			Str("file", filePath).
			Int64("size", info.Size()).
			Msg("Skipping file due to size limit")
		return false
	}

	// Check file types if specified
	if len(options.FileTypes) > 0 {
		ext := strings.ToLower(filepath.Ext(filePath))
		found := false
		for _, ft := range options.FileTypes {
			if ext == ft || ext == "."+ft {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// filterFindings removes duplicates and false positives
func (sd *SecretDiscovery) filterFindings(findings []ExtendedSecretFinding, options ScanOptions) []ExtendedSecretFinding {
	filtered := make([]ExtendedSecretFinding, 0)
	seen := make(map[string]bool)

	for _, finding := range findings {
		// Create unique key
		key := fmt.Sprintf("%s:%d:%s", finding.File, finding.Line, finding.Match)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Check confidence threshold
		if finding.Confidence < options.MinConfidence {
			continue
		}

		// Check for false positives
		if sd.isFalsePositive(finding) {
			finding.FalsePositive = true
			if options.ExcludeFalsePositives {
				continue
			}
		}

		filtered = append(filtered, finding)
	}

	return filtered
}

// verifyFindings attempts to verify if findings are real secrets
func (sd *SecretDiscovery) verifyFindings(findings []ExtendedSecretFinding) {
	for i := range findings {
		// Skip if already marked as false positive
		if findings[i].FalsePositive {
			continue
		}

		// Verification logic based on secret type
		switch findings[i].Type {
		case "aws_access_key":
			findings[i].Verified = sd.verifyAWSKey(findings[i].Match)
		case "github_token":
			findings[i].Verified = sd.verifyGitHubToken(findings[i].Match)
		case "jwt":
			findings[i].Verified = sd.verifyJWT(findings[i].Match)
		default:
			// For other types, use entropy and pattern matching confidence
			findings[i].Verified = findings[i].Confidence > 0.8 && findings[i].Entropy > 4.5
		}
	}
}

// processResults aggregates and summarizes findings
func (sd *SecretDiscovery) processResults(result *DiscoveryResult) {
	uniqueSecrets := make(map[string]bool)

	for _, finding := range result.Findings {
		// Count by severity
		result.Summary.BySeverity[finding.Severity]++

		// Count by type
		result.Summary.ByType[finding.Type]++

		// Count by file
		result.Summary.ByFile[finding.File]++

		// Track unique secrets
		secretHash := sd.hashSecret(finding.Match)
		uniqueSecrets[secretHash] = true

		// Count verified and false positives
		if finding.Verified {
			result.Summary.VerifiedFindings++
		}
		if finding.FalsePositive {
			result.Summary.FalsePositives++
		}
	}

	result.Summary.TotalFindings = len(result.Findings)
	result.Summary.UniqueSecrets = len(uniqueSecrets)
}

// calculateRiskScore calculates an overall risk score
func (sd *SecretDiscovery) calculateRiskScore(result *DiscoveryResult) int {
	score := 0

	// Base score on severity
	score += result.Summary.BySeverity["critical"] * 25
	score += result.Summary.BySeverity["high"] * 15
	score += result.Summary.BySeverity["medium"] * 5
	score += result.Summary.BySeverity["low"] * 1

	// Increase score for verified findings
	score += result.Summary.VerifiedFindings * 10

	// Decrease score for false positives
	score -= result.Summary.FalsePositives * 2

	// Cap at 100
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

// hashSecret creates a hash of a secret for deduplication
func (sd *SecretDiscovery) hashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// isFalsePositive checks if a finding is likely a false positive
func (sd *SecretDiscovery) isFalsePositive(finding ExtendedSecretFinding) bool {
	// Check for common false positive patterns
	falsePositivePatterns := []string{
		"example", "sample", "test", "demo", "dummy",
		"xxx", "placeholder", "your-", "my-",
		"<", ">", "{", "}", "null", "none", "n/a",
	}

	lowerMatch := strings.ToLower(finding.Match)
	for _, pattern := range falsePositivePatterns {
		if strings.Contains(lowerMatch, pattern) {
			return true
		}
	}

	// Check for low entropy in supposedly high-entropy secrets
	if finding.Type == "generic_secret" && finding.Entropy < 2.5 {
		return true
	}

	// Check for repeated characters
	if sd.hasRepeatedCharacters(finding.Match, 0.5) {
		return true
	}

	return false
}

// hasRepeatedCharacters checks if a string has too many repeated characters
func (sd *SecretDiscovery) hasRepeatedCharacters(s string, threshold float64) bool {
	if len(s) == 0 {
		return false
	}

	charCount := make(map[rune]int)
	for _, c := range s {
		charCount[c]++
	}

	maxCount := 0
	for _, count := range charCount {
		if count > maxCount {
			maxCount = count
		}
	}

	return float64(maxCount)/float64(len(s)) > threshold
}

// Verification methods (simplified for example)
func (sd *SecretDiscovery) verifyAWSKey(key string) bool {
	// AWS access keys are 20 characters long and start with AKIA
	return len(key) == 20 && strings.HasPrefix(key, "AKIA")
}

func (sd *SecretDiscovery) verifyGitHubToken(token string) bool {
	// GitHub tokens have specific prefixes
	prefixes := []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

func (sd *SecretDiscovery) verifyJWT(token string) bool {
	// Basic JWT validation - check for three base64 parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if _, err := base64.RawURLEncoding.DecodeString(part); err != nil {
			return false
		}
	}

	return true
}

// ScanOptions configures the secret scanning behavior
type ScanOptions struct {
	Recursive              bool     `json:"recursive"`
	FileTypes              []string `json:"file_types"`
	MaxFileSize            int64    `json:"max_file_size"`
	MaxConcurrency         int      `json:"max_concurrency"`
	EnableEntropyDetection bool     `json:"enable_entropy_detection"`
	MinConfidence          float64  `json:"min_confidence"`
	VerifyFindings         bool     `json:"verify_findings"`
	ExcludeFalsePositives  bool     `json:"exclude_false_positives"`
	CustomPatterns         []string `json:"custom_patterns"`
}

// DefaultScanOptions returns default scanning options
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		Recursive:              true,
		FileTypes:              []string{},       // Scan all file types by default
		MaxFileSize:            10 * 1024 * 1024, // 10MB
		MaxConcurrency:         4,
		EnableEntropyDetection: true,
		MinConfidence:          0.7,
		VerifyFindings:         true,
		ExcludeFalsePositives:  false,
		CustomPatterns:         []string{},
	}
}

// PatternDetector handles pattern-based secret detection
type PatternDetector struct {
	patterns map[string]*SecretPattern
}

// SecretPattern defines a pattern for detecting secrets
type SecretPattern struct {
	Name       string
	Pattern    *regexp.Regexp
	Severity   string
	Confidence float64
	SecretType string
}

// NewPatternDetector creates a new pattern detector with built-in patterns
func NewPatternDetector() *PatternDetector {
	pd := &PatternDetector{
		patterns: make(map[string]*SecretPattern),
	}

	// Initialize with common patterns
	pd.addBuiltInPatterns()

	return pd
}

// addBuiltInPatterns adds common secret patterns
func (pd *PatternDetector) addBuiltInPatterns() {
	patterns := []SecretPattern{
		// AWS
		{
			Name:       "aws_access_key",
			Pattern:    regexp.MustCompile(`\b(AKIA[0-9A-Z]{16})\b`),
			Severity:   "critical",
			Confidence: 0.95,
			SecretType: "aws_access_key",
		},
		{
			Name:       "aws_secret_key",
			Pattern:    regexp.MustCompile(`\b([A-Za-z0-9/+=]{40})\b`),
			Severity:   "critical",
			Confidence: 0.7,
			SecretType: "aws_secret_key",
		},

		// GitHub
		{
			Name:       "github_token",
			Pattern:    regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9_]{30,40})\b`),
			Severity:   "high",
			Confidence: 0.95,
			SecretType: "github_token",
		},

		// Generic API Keys
		{
			Name:       "api_key",
			Pattern:    regexp.MustCompile(`(?i)(api[_\-\s]?key|apikey|api[_\-\s]?token)[\s]*[:=][\s]*['"]?([A-Za-z0-9_\-]{20,})['"]?`),
			Severity:   "high",
			Confidence: 0.8,
			SecretType: "api_key",
		},

		// Private Keys
		{
			Name:       "private_key",
			Pattern:    regexp.MustCompile(`-----BEGIN\s+(RSA|DSA|EC|OPENSSH|PGP)\s+PRIVATE KEY-----`),
			Severity:   "critical",
			Confidence: 1.0,
			SecretType: "private_key",
		},

		// JWT
		{
			Name:       "jwt",
			Pattern:    regexp.MustCompile(`\b(ey[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,})\b`),
			Severity:   "medium",
			Confidence: 0.9,
			SecretType: "jwt",
		},

		// Database URLs
		{
			Name:       "database_url",
			Pattern:    regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis)://[^:]+:[^@]+@[^/]+/\w+`),
			Severity:   "high",
			Confidence: 0.9,
			SecretType: "database_url",
		},

		// Slack
		{
			Name:       "slack_token",
			Pattern:    regexp.MustCompile(`\b(xox[baprs]-[0-9]{10,}-[0-9]{10,}-[a-zA-Z0-9]{24,})\b`),
			Severity:   "medium",
			Confidence: 0.95,
			SecretType: "slack_token",
		},
	}

	for _, p := range patterns {
		pattern := p
		pd.patterns[pattern.Name] = &pattern
	}
}

// Scan scans a line for pattern matches
func (pd *PatternDetector) Scan(line, filePath string, lineNumber int) []ExtendedSecretFinding {
	var findings []ExtendedSecretFinding

	for _, pattern := range pd.patterns {
		if matches := pattern.Pattern.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, match := range matches {
				secretValue := match[len(match)-1] // Last capture group
				if len(match) > 1 {
					secretValue = match[1] // First capture group if available
				}

				finding := ExtendedSecretFinding{
					SecretFinding: SecretFinding{
						Type:        pattern.SecretType,
						File:        filePath,
						Line:        lineNumber,
						Description: pattern.Name,
						Confidence:  pattern.Confidence,
						RuleID:      pattern.Name,
					},
					ID:       fmt.Sprintf("%s:%d:%s", filePath, lineNumber, pattern.Name),
					Column:   strings.Index(line, secretValue),
					Severity: pattern.Severity,
					Match:    secretValue,
					Redacted: pd.redactSecret(secretValue),
					Context:  line,
					Pattern:  pattern.Name,
				}

				findings = append(findings, finding)
			}
		}
	}

	return findings
}

// redactSecret redacts a secret value for safe display
func (pd *PatternDetector) redactSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}

	// Show first 3 and last 3 characters
	return secret[:3] + "***" + secret[len(secret)-3:]
}

// EntropyDetector handles entropy-based secret detection
type EntropyDetector struct {
	minEntropy float64
	minLength  int
}

// NewEntropyDetector creates a new entropy detector
func NewEntropyDetector() *EntropyDetector {
	return &EntropyDetector{
		minEntropy: 3.5, // Lower threshold for testing
		minLength:  8,   // Shorter minimum length
	}
}

// Scan scans a line for high-entropy strings
func (ed *EntropyDetector) Scan(line, filePath string, lineNumber int) []ExtendedSecretFinding {
	var findings []ExtendedSecretFinding

	// Split line into tokens
	tokens := ed.tokenize(line)

	for _, token := range tokens {
		if len(token) < ed.minLength {
			continue
		}

		entropy := ed.calculateEntropy(token)
		if entropy >= ed.minEntropy {
			finding := ExtendedSecretFinding{
				SecretFinding: SecretFinding{
					Type:        "generic_secret",
					File:        filePath,
					Line:        lineNumber,
					Description: "High entropy string detected",
					Confidence:  ed.getConfidenceByEntropy(entropy),
					RuleID:      "entropy_detection",
				},
				ID:       fmt.Sprintf("%s:%d:entropy:%s", filePath, lineNumber, token[:10]),
				Column:   strings.Index(line, token),
				Severity: ed.getSeverityByEntropy(entropy),
				Match:    token,
				Redacted: ed.redactSecret(token),
				Context:  line,
				Entropy:  entropy,
			}

			findings = append(findings, finding)
		}
	}

	return findings
}

// tokenize splits a line into potential secret tokens
func (ed *EntropyDetector) tokenize(line string) []string {
	// Remove common delimiters and split
	delimiters := regexp.MustCompile(`[\s,;:"'=\[\]{}()<>]+`)
	tokens := delimiters.Split(line, -1)

	// Filter out empty tokens and common words
	filtered := make([]string, 0)
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" && !ed.isCommonWord(token) {
			filtered = append(filtered, token)
		}
	}

	return filtered
}

// calculateEntropy calculates Shannon entropy of a string
func (ed *EntropyDetector) calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Count character frequencies
	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}

	// Calculate entropy
	var entropy float64
	length := float64(len(s))

	for _, count := range freq {
		if count > 0 {
			probability := float64(count) / length
			entropy -= probability * math.Log2(probability)
		}
	}

	return entropy
}

// getSeverityByEntropy determines severity based on entropy value
func (ed *EntropyDetector) getSeverityByEntropy(entropy float64) string {
	switch {
	case entropy >= 5.5:
		return "high"
	case entropy >= 5.0:
		return "medium"
	default:
		return "low"
	}
}

// getConfidenceByEntropy determines confidence based on entropy value
func (ed *EntropyDetector) getConfidenceByEntropy(entropy float64) float64 {
	// Map entropy to confidence (4.5 -> 0.6, 6.0 -> 0.9)
	confidence := (entropy-4.5)/1.5*0.3 + 0.6
	if confidence > 0.9 {
		confidence = 0.9
	}
	return confidence
}

// isCommonWord checks if a token is a common word
func (ed *EntropyDetector) isCommonWord(token string) bool {
	commonWords := []string{
		"true", "false", "null", "undefined", "none",
		"default", "localhost", "example", "test", "demo",
		"admin", "root", "user", "password", "secret",
	}

	lower := strings.ToLower(token)
	for _, word := range commonWords {
		if lower == word {
			return true
		}
	}

	return false
}

// redactSecret redacts a secret value
func (ed *EntropyDetector) redactSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}

// FileTypeHandler manages file type specific handling
type FileTypeHandler struct {
	handlers map[string]FileHandler
}

// FileHandler defines how to handle specific file types
type FileHandler interface {
	CanHandle(filePath string) bool
	PreProcess(content string) string
}

// NewFileTypeHandler creates a new file type handler
func NewFileTypeHandler() *FileTypeHandler {
	return &FileTypeHandler{
		handlers: make(map[string]FileHandler),
	}
}

// ExclusionManager manages path and pattern exclusions
type ExclusionManager struct {
	excludedPaths    []string
	excludedPatterns []*regexp.Regexp
}

// NewExclusionManager creates a new exclusion manager
func NewExclusionManager() *ExclusionManager {
	em := &ExclusionManager{
		excludedPaths: []string{
			".git", ".svn", ".hg",
			"node_modules", "vendor", ".venv",
			"__pycache__", ".pytest_cache",
			".idea", ".vscode",
		},
		excludedPatterns: make([]*regexp.Regexp, 0),
	}

	// Add common binary and generated file patterns
	patterns := []string{
		`\.min\.js$`, `\.min\.css$`,
		`\.map$`, `\.sum$`,
		`\.lock$`, `\.cache$`,
		`\.(jpg|jpeg|png|gif|ico|svg)$`,
		`\.(zip|tar|gz|bz2|7z|rar)$`,
		`\.(exe|dll|so|dylib)$`,
	}

	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			em.excludedPatterns = append(em.excludedPatterns, re)
		}
	}

	return em
}

// IsExcluded checks if a path should be excluded
func (em *ExclusionManager) IsExcluded(path string) bool {
	// Check excluded paths
	for _, excluded := range em.excludedPaths {
		if strings.Contains(path, excluded) {
			return true
		}
	}

	// Check excluded patterns
	for _, pattern := range em.excludedPatterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}
