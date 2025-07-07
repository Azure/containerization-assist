package diagnostics

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// FailureAnalyzer provides comprehensive failure analysis and categorization
type FailureAnalyzer struct {
	logger           zerolog.Logger
	config           FailureAnalysisConfig
	patternAnalyzer  *PatternAnalyzer
	contextExtractor *ContextExtractor
	categoryMapping  map[string]FailureCategory
}

// FailureAnalysisConfig defines failure analysis configuration
type FailureAnalysisConfig struct {
	EnablePatternAnalysis   bool              `json:"enable_pattern_analysis"`
	EnableContextExtraction bool              `json:"enable_context_extraction"`
	EnableCategorization    bool              `json:"enable_categorization"`
	MaxAnalysisDepth        int               `json:"max_analysis_depth"`
	PatternTimeout          time.Duration     `json:"pattern_timeout"`
	CustomPatterns          map[string]string `json:"custom_patterns"`
	IgnorePatterns          []string          `json:"ignore_patterns"`
}

// PatternAnalyzer analyzes error patterns to identify common failure modes
type PatternAnalyzer struct {
	logger   zerolog.Logger
	patterns map[string]*regexp.Regexp
	stats    PatternStats
}

// PatternStats tracks pattern matching statistics
type PatternStats struct {
	TotalAnalyses   int64            `json:"total_analyses"`
	PatternsMatched map[string]int64 `json:"patterns_matched"`
	AnalysisTime    time.Duration    `json:"analysis_time"`
	LastAnalysis    time.Time        `json:"last_analysis"`
	TopPatterns     []PatternMatch   `json:"top_patterns"`
}

// PatternMatch represents a matched error pattern
type PatternMatch struct {
	Pattern     string    `json:"pattern"`
	Confidence  float64   `json:"confidence"`
	Count       int64     `json:"count"`
	LastSeen    time.Time `json:"last_seen"`
	Description string    `json:"description"`
}

// ContextExtractor extracts relevant context from error messages and stack traces
type ContextExtractor struct {
	logger     zerolog.Logger
	extractors map[string]ContextExtractorFunc
}

// ContextExtractorFunc defines a function that extracts context from error data
type ContextExtractorFunc func(error) map[string]interface{}

// FailureCategory represents a category of failure
type FailureCategory struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Severity     string   `json:"severity"`
	CommonCauses []string `json:"common_causes"`
	Remediation  []string `json:"remediation"`
	Pattern      string   `json:"pattern"`
}

// AnalysisResult contains the result of failure analysis
type AnalysisResult struct {
	Success           bool                   `json:"success"`
	FailureCategory   *FailureCategory       `json:"failure_category,omitempty"`
	PatternMatches    []PatternMatch         `json:"pattern_matches"`
	ExtractedContext  map[string]interface{} `json:"extracted_context"`
	Confidence        float64                `json:"confidence"`
	Recommendations   []string               `json:"recommendations"`
	SimilarFailures   []SimilarFailure       `json:"similar_failures"`
	RootCauseAnalysis *RootCauseAnalysis     `json:"root_cause_analysis,omitempty"`
	ProcessingTime    time.Duration          `json:"processing_time"`
}

// SimilarFailure represents a similar failure pattern
type SimilarFailure struct {
	Pattern    string    `json:"pattern"`
	Similarity float64   `json:"similarity"`
	LastSeen   time.Time `json:"last_seen"`
	Count      int64     `json:"count"`
}

// RootCauseAnalysis provides detailed root cause analysis
type RootCauseAnalysis struct {
	PrimaryRoot          string   `json:"primary_root"`
	ContributingFactors  []string `json:"contributing_factors"`
	SystemComponents     []string `json:"system_components"`
	UserActions          []string `json:"user_actions"`
	EnvironmentalFactors []string `json:"environmental_factors"`
	Confidence           float64  `json:"confidence"`
}

// NewFailureAnalyzer creates a new failure analyzer
func NewFailureAnalyzer(config FailureAnalysisConfig, logger zerolog.Logger) *FailureAnalyzer {
	fa := &FailureAnalyzer{
		logger:           logger.With().Str("component", "failure_analyzer").Logger(),
		config:           config,
		patternAnalyzer:  NewPatternAnalyzer(logger),
		contextExtractor: NewContextExtractor(logger),
		categoryMapping:  initializeFailureCategories(),
	}

	// Initialize custom patterns if provided
	if config.CustomPatterns != nil {
		for name, pattern := range config.CustomPatterns {
			fa.patternAnalyzer.AddPattern(name, pattern)
		}
	}

	return fa
}

// AnalyzeFailure performs comprehensive failure analysis
func (fa *FailureAnalyzer) AnalyzeFailure(ctx context.Context, err error) (*AnalysisResult, error) {
	startTime := time.Now()

	result := &AnalysisResult{
		Success:          true,
		PatternMatches:   make([]PatternMatch, 0),
		ExtractedContext: make(map[string]interface{}),
		Recommendations:  make([]string, 0),
		SimilarFailures:  make([]SimilarFailure, 0),
	}

	// Extract error message and details
	errorMsg := err.Error()
	result.ExtractedContext["error_message"] = errorMsg
	result.ExtractedContext["error_type"] = fmt.Sprintf("%T", err)

	// Pattern analysis
	if fa.config.EnablePatternAnalysis {
		patterns, err := fa.patternAnalyzer.AnalyzePatterns(ctx, errorMsg)
		if err != nil {
			fa.logger.Warn().Err(err).Msg("Pattern analysis failed")
		} else {
			result.PatternMatches = patterns
		}
	}

	// Context extraction
	if fa.config.EnableContextExtraction {
		context := fa.contextExtractor.ExtractContext(err)
		for key, value := range context {
			result.ExtractedContext[key] = value
		}
	}

	// Categorization
	if fa.config.EnableCategorization {
		category := fa.categorizeFailure(errorMsg, result.PatternMatches)
		if category != nil {
			result.FailureCategory = category
			result.Confidence = fa.calculateConfidence(result.PatternMatches, category)
		}
	}

	// Generate recommendations
	result.Recommendations = fa.generateRecommendations(result)

	// Root cause analysis
	result.RootCauseAnalysis = fa.performRootCauseAnalysis(err, result)

	result.ProcessingTime = time.Since(startTime)

	fa.logger.Debug().
		Str("error_type", fmt.Sprintf("%T", err)).
		Int("pattern_matches", len(result.PatternMatches)).
		Float64("confidence", result.Confidence).
		Dur("processing_time", result.ProcessingTime).
		Msg("Failure analysis completed")

	return result, nil
}

// NewPatternAnalyzer creates a new pattern analyzer
func NewPatternAnalyzer(logger zerolog.Logger) *PatternAnalyzer {
	pa := &PatternAnalyzer{
		logger:   logger.With().Str("component", "pattern_analyzer").Logger(),
		patterns: make(map[string]*regexp.Regexp),
		stats: PatternStats{
			PatternsMatched: make(map[string]int64),
			TopPatterns:     make([]PatternMatch, 0),
		},
	}

	// Initialize common error patterns
	pa.initializeCommonPatterns()

	return pa
}

// AddPattern adds a new error pattern
func (pa *PatternAnalyzer) AddPattern(name, pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Messagef("Invalid regex pattern for '%s'", name).
			Context("pattern", pattern).
			Cause(err).
			Suggestion("Check regex syntax and escape special characters").
			WithLocation().
			Build()
	}

	pa.patterns[name] = compiled
	pa.logger.Debug().Str("pattern_name", name).Msg("Pattern added")

	return nil
}

// AnalyzePatterns analyzes error message against known patterns
func (pa *PatternAnalyzer) AnalyzePatterns(ctx context.Context, errorMsg string) ([]PatternMatch, error) {
	matches := make([]PatternMatch, 0)

	for name, pattern := range pa.patterns {
		if pattern.MatchString(errorMsg) {
			match := PatternMatch{
				Pattern:     name,
				Confidence:  pa.calculatePatternConfidence(name, errorMsg),
				Count:       pa.stats.PatternsMatched[name] + 1,
				LastSeen:    time.Now(),
				Description: pa.getPatternDescription(name),
			}

			matches = append(matches, match)
			pa.stats.PatternsMatched[name]++
		}
	}

	pa.stats.TotalAnalyses++
	pa.stats.LastAnalysis = time.Now()

	return matches, nil
}

// NewContextExtractor creates a new context extractor
func NewContextExtractor(logger zerolog.Logger) *ContextExtractor {
	ce := &ContextExtractor{
		logger:     logger.With().Str("component", "context_extractor").Logger(),
		extractors: make(map[string]ContextExtractorFunc),
	}

	// Initialize standard extractors
	ce.initializeExtractors()

	return ce
}

// ExtractContext extracts context from an error
func (ce *ContextExtractor) ExtractContext(err error) map[string]interface{} {
	context := make(map[string]interface{})

	// Extract basic information
	context["timestamp"] = time.Now()
	context["error_message"] = err.Error()
	context["error_type"] = fmt.Sprintf("%T", err)

	// Apply all extractors
	for name, extractor := range ce.extractors {
		if extracted := extractor(err); extracted != nil {
			for key, value := range extracted {
				context[fmt.Sprintf("%s_%s", name, key)] = value
			}
		}
	}

	return context
}

// AddExtractor adds a custom context extractor
func (ce *ContextExtractor) AddExtractor(name string, extractor ContextExtractorFunc) {
	ce.extractors[name] = extractor
	ce.logger.Debug().Str("extractor_name", name).Msg("Context extractor added")
}

func (pa *PatternAnalyzer) initializeCommonPatterns() {
	patterns := map[string]string{
		"network_timeout":      `(?i)(timeout|timed out|deadline exceeded)`,
		"connection_refused":   `(?i)(connection refused|connection reset)`,
		"file_not_found":       `(?i)(file not found|no such file|does not exist)`,
		"permission_denied":    `(?i)(permission denied|access denied|forbidden)`,
		"out_of_memory":        `(?i)(out of memory|cannot allocate memory|memory exhausted)`,
		"disk_full":            `(?i)(no space left|disk full|storage exhausted)`,
		"invalid_argument":     `(?i)(invalid argument|bad parameter|illegal value)`,
		"docker_error":         `(?i)(docker|container|image.*not found|build failed)`,
		"kubernetes_error":     `(?i)(kubernetes|k8s|pod|deployment|service.*not found)`,
		"database_error":       `(?i)(database|sql|connection.*failed|query.*error)`,
		"authentication_error": `(?i)(authentication|unauthorized|invalid.*credentials)`,
		"json_parse_error":     `(?i)(json|parse|unmarshal|invalid.*syntax)`,
	}

	for name, pattern := range patterns {
		if err := pa.AddPattern(name, pattern); err != nil {
			pa.logger.Warn().Err(err).Str("pattern", name).Msg("Failed to add common pattern")
		}
	}
}

func (ce *ContextExtractor) initializeExtractors() {
	ce.AddExtractor("file", func(err error) map[string]interface{} {
		re := regexp.MustCompile(`([/\\]?[^/\\]+[/\\])*[^/\\]+\.(go|js|py|java|cpp|c|h)`)
		if match := re.FindString(err.Error()); match != "" {
			return map[string]interface{}{"path": match}
		}
		return nil
	})

	ce.AddExtractor("line", func(err error) map[string]interface{} {
		re := regexp.MustCompile(`line\s+(\d+)`)
		if matches := re.FindStringSubmatch(err.Error()); len(matches) > 1 {
			return map[string]interface{}{"number": matches[1]}
		}
		return nil
	})

	ce.AddExtractor("url", func(err error) map[string]interface{} {
		re := regexp.MustCompile(`https?://[^\s]+`)
		if match := re.FindString(err.Error()); match != "" {
			return map[string]interface{}{"url": match}
		}
		return nil
	})
}

func (fa *FailureAnalyzer) categorizeFailure(errorMsg string, patterns []PatternMatch) *FailureCategory {
	var bestCategory *FailureCategory
	var bestScore float64

	for _, category := range fa.categoryMapping {
		score := fa.calculateCategoryScore(errorMsg, patterns, category)
		if score > bestScore {
			bestScore = score
			bestCategory = &category
		}
	}

	if bestScore > 0.5 {
		return bestCategory
	}

	return nil
}

func (fa *FailureAnalyzer) calculateCategoryScore(errorMsg string, patterns []PatternMatch, category FailureCategory) float64 {
	score := 0.0
	for _, pattern := range patterns {
		if pattern.Pattern == category.Pattern {
			score += pattern.Confidence
		}
	}

	for _, cause := range category.CommonCauses {
		if strings.Contains(strings.ToLower(errorMsg), strings.ToLower(cause)) {
			score += 0.3
		}
	}

	return score
}

func (fa *FailureAnalyzer) calculateConfidence(patterns []PatternMatch, category *FailureCategory) float64 {
	if len(patterns) == 0 {
		return 0.0
	}

	totalConfidence := 0.0
	for _, pattern := range patterns {
		totalConfidence += pattern.Confidence
	}

	baseConfidence := totalConfidence / float64(len(patterns))

	if category != nil {
		baseConfidence *= 1.2
	}

	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	return baseConfidence
}

func (pa *PatternAnalyzer) calculatePatternConfidence(pattern, errorMsg string) float64 {
	confidence := 0.7
	if strings.Contains(pattern, "docker") || strings.Contains(pattern, "kubernetes") {
		confidence += 0.2
	}

	if pattern == "file_not_found" && strings.Contains(errorMsg, "not found") {
		confidence += 0.1
	}

	return confidence
}

func (pa *PatternAnalyzer) getPatternDescription(pattern string) string {
	descriptions := map[string]string{
		"network_timeout":      "Network operation timed out",
		"connection_refused":   "Connection was refused by remote host",
		"file_not_found":       "Requested file or resource does not exist",
		"permission_denied":    "Access was denied due to insufficient permissions",
		"out_of_memory":        "System ran out of available memory",
		"disk_full":            "Storage device is full or exhausted",
		"invalid_argument":     "Invalid argument or parameter provided",
		"docker_error":         "Docker-related operation failed",
		"kubernetes_error":     "Kubernetes operation failed",
		"database_error":       "Database operation failed",
		"authentication_error": "Authentication or authorization failed",
		"json_parse_error":     "JSON parsing or validation failed",
	}

	if desc, exists := descriptions[pattern]; exists {
		return desc
	}

	return "Unknown error pattern"
}

func (fa *FailureAnalyzer) generateRecommendations(result *AnalysisResult) []string {
	recommendations := make([]string, 0)
	for _, pattern := range result.PatternMatches {
		switch pattern.Pattern {
		case "network_timeout":
			recommendations = append(recommendations, "Check network connectivity and increase timeout values")
		case "file_not_found":
			recommendations = append(recommendations, "Verify file path exists and check permissions")
		case "permission_denied":
			recommendations = append(recommendations, "Check user permissions and file access rights")
		case "docker_error":
			recommendations = append(recommendations, "Verify Docker daemon is running and image exists")
		case "kubernetes_error":
			recommendations = append(recommendations, "Check Kubernetes cluster status and resource availability")
		}
	}

	if result.FailureCategory != nil {
		recommendations = append(recommendations, result.FailureCategory.Remediation...)
	}

	return recommendations
}

func (fa *FailureAnalyzer) performRootCauseAnalysis(err error, result *AnalysisResult) *RootCauseAnalysis {
	analysis := &RootCauseAnalysis{
		ContributingFactors:  make([]string, 0),
		SystemComponents:     make([]string, 0),
		UserActions:          make([]string, 0),
		EnvironmentalFactors: make([]string, 0),
	}

	errorMsg := strings.ToLower(err.Error())

	if len(result.PatternMatches) > 0 {
		analysis.PrimaryRoot = result.PatternMatches[0].Description
		analysis.Confidence = result.PatternMatches[0].Confidence
	}

	if strings.Contains(errorMsg, "docker") {
		analysis.SystemComponents = append(analysis.SystemComponents, "Docker")
	}
	if strings.Contains(errorMsg, "kubernetes") || strings.Contains(errorMsg, "k8s") {
		analysis.SystemComponents = append(analysis.SystemComponents, "Kubernetes")
	}
	if strings.Contains(errorMsg, "network") {
		analysis.SystemComponents = append(analysis.SystemComponents, "Network")
	}

	return analysis
}

// initializeFailureCategories initializes common failure categories
func initializeFailureCategories() map[string]FailureCategory {
	return map[string]FailureCategory{
		"network": {
			ID:           "NETWORK_FAILURE",
			Name:         "Network Failure",
			Description:  "Network-related connectivity or communication failure",
			Severity:     "HIGH",
			CommonCauses: []string{"timeout", "connection refused", "dns", "firewall"},
			Remediation:  []string{"Check network connectivity", "Verify firewall rules", "Test DNS resolution"},
			Pattern:      "network_timeout",
		},
		"filesystem": {
			ID:           "FILESYSTEM_FAILURE",
			Name:         "File System Failure",
			Description:  "File system access or storage-related failure",
			Severity:     "MEDIUM",
			CommonCauses: []string{"file not found", "permission denied", "disk full"},
			Remediation:  []string{"Check file permissions", "Verify disk space", "Validate file paths"},
			Pattern:      "file_not_found",
		},
		"container": {
			ID:           "CONTAINER_FAILURE",
			Name:         "Container Failure",
			Description:  "Container runtime or orchestration failure",
			Severity:     "HIGH",
			CommonCauses: []string{"image not found", "build failed", "runtime error"},
			Remediation:  []string{"Check container runtime", "Verify image availability", "Review container logs"},
			Pattern:      "docker_error",
		},
	}
}

// NewSessionNotFound creates a session not found error
func NewSessionNotFound(sessionID string, details ...map[string]interface{}) *errors.RichError {
	builder := errors.NewError().
		Code(errors.CodeResourceNotFound).
		Type(errors.ErrTypeSession).
		Severity(errors.SeverityMedium).
		Messagef("Session not found: %s", sessionID).
		Context("session_id", sessionID)

	if len(details) > 0 {
		for k, v := range details[0] {
			builder = builder.Context(k, v)
		}
	}

	return builder.
		Suggestion("Check the session ID and ensure the session exists").
		WithLocation().
		Build()
}

// NewWithData creates a rich error with additional data context
func NewWithData(code, message string, data map[string]interface{}) *errors.RichError {
	builder := errors.NewError().
		Code(errors.ErrorCode(code)).
		Type(errors.ErrTypeInternal).
		Severity(errors.SeverityMedium).
		Message(message)

	for k, v := range data {
		builder = builder.Context(k, v)
	}

	return builder.
		WithLocation().
		Build()
}

// SystemState captures system state at error time
type SystemState struct {
	DockerAvailable bool    `json:"docker_available"`
	K8sConnected    bool    `json:"k8s_connected"`
	DiskSpaceMB     int64   `json:"disk_space_mb"`
	MemoryMB        int64   `json:"memory_mb"`
	LoadAverage     float64 `json:"load_average"`
}

// ResourceUsage captures resource usage at error time
type ResourceUsage struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryMB       int64   `json:"memory_mb"`
	DiskUsageMB    int64   `json:"disk_usage_mb"`
	NetworkBytesTx int64   `json:"network_bytes_tx"`
	NetworkBytesRx int64   `json:"network_bytes_rx"`
}

// LogEntry represents a relevant log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

// DiagnosticCheck represents a diagnostic check result
type DiagnosticCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details"`
}
