package ai

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// AIAnalysisServiceImpl implements the AIAnalysisService interface using MCP conversation patterns
// This service leverages the calling AI assistant (Claude Code) for analysis rather than direct OpenAI integration
type AnalysisServiceImpl struct {
	logger          *slog.Logger
	conversationSvc services.ConversationService
	promptSvc       services.PromptService
	cache           CacheService
	config          Config
	metrics         *Metrics
	mu              sync.RWMutex
}

// AIConfig contains configuration for the AI analysis service
type Config struct {
	CacheEnabled     bool          `json:"cache_enabled"`
	CacheTTL         time.Duration `json:"cache_ttl"`
	MaxAnalysisSize  int           `json:"max_analysis_size"` // max size of code to analyze
	AnalysisTimeout  time.Duration `json:"analysis_timeout"`  // timeout for analysis operations
	RetryAttempts    int           `json:"retry_attempts"`
	RetryDelay       time.Duration `json:"retry_delay"`
	EnableMetrics    bool          `json:"enable_metrics"`
	MaxConcurrentOps int           `json:"max_concurrent_ops"` // limit concurrent analysis operations
}

// CacheService interface for caching analysis results (simplified for MCP context)
type CacheService interface {
	Get(key string) (*services.CachedAnalysis, error)
	Set(key string, data *services.CachedAnalysis, ttl time.Duration) error
	Delete(key string) error
	DeletePattern(pattern string) error
	Stats() CacheStats
}

// CacheStats represents cache statistics
type CacheStats struct {
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	Size    int64   `json:"size"`
	Entries int     `json:"entries"`
	HitRate float64 `json:"hit_rate"`
}

// AIMetrics tracks AI service metrics (simplified for MCP)
type Metrics struct {
	mu                 sync.RWMutex
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	totalTokens        int64
	inputTokens        int64
	outputTokens       int64
	totalCost          float64
	responseTimes      []time.Duration
	operationUsage     map[string]*OperationMetrics
	startTime          time.Time
}

// OperationMetrics tracks metrics for specific operations
type OperationMetrics struct {
	Count         int64
	ResponseTimes []time.Duration
	Errors        int64
	CacheHits     int64
	TotalTokens   int64
	TotalCost     float64
}

// NewAIAnalysisService creates a new MCP-based AI analysis service
func NewAIAnalysisService(
	logger *slog.Logger,
	conversationSvc services.ConversationService,
	promptSvc services.PromptService,
	cache CacheService,
	config Config,
) services.AIAnalysisService {
	return &AnalysisServiceImpl{
		logger:          logger,
		conversationSvc: conversationSvc,
		promptSvc:       promptSvc,
		cache:           cache,
		config:          config,
		metrics: &Metrics{
			operationUsage: make(map[string]*OperationMetrics),
			startTime:      time.Now(),
		},
	}
}

// AnalyzeCodePatterns performs AI analysis of code patterns and architecture
// This works by creating a conversation context that asks the calling AI assistant to analyze the code
func (s *AnalysisServiceImpl) AnalyzeCodePatterns(ctx context.Context, files map[string]string) (*services.CodeAnalysisResult, error) {
	s.logger.Info("Starting code pattern analysis", slog.Int("file_count", len(files)))

	// Validate input size
	totalSize := 0
	for _, content := range files {
		totalSize += len(content)
	}
	if totalSize > s.config.MaxAnalysisSize {
		return nil, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("Code size exceeds maximum analysis limit").
			Context("max_size", s.config.MaxAnalysisSize).
			Context("actual_size", totalSize).
			Build()
	}

	// Generate cache key
	cacheKey := s.generateCacheKey("code_patterns", files)

	// Check cache first
	if s.config.CacheEnabled {
		if cached, err := s.cache.Get(cacheKey); err == nil && cached != nil {
			s.logger.Debug("Using cached code pattern analysis")
			if result, ok := cached.Data["result"].(*services.CodeAnalysisResult); ok {
				s.recordCacheHit("analyze_code_patterns")
				return result, nil
			}
		}
	}

	start := time.Now()

	// Create analysis request through conversation service
	analysisPrompt := s.buildCodeAnalysisPrompt(files)

	// Use a dedicated session for AI analysis to avoid interfering with user sessions
	analysisSessionID := fmt.Sprintf("ai_analysis_%d", time.Now().Unix())

	response, err := s.conversationSvc.ProcessMessage(ctx, analysisSessionID, analysisPrompt)
	if err != nil {
		s.recordMetrics("analyze_code_patterns", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to process code analysis request").
			Cause(err).
			Build()
	}

	// Parse the AI response into structured analysis result
	result, err := s.parseCodeAnalysisResponse(response.Message)
	if err != nil {
		s.recordMetrics("analyze_code_patterns", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to parse code analysis response").
			Cause(err).
			Build()
	}

	// Cache the result
	if s.config.CacheEnabled {
		cachedData := &services.CachedAnalysis{
			Key:       cacheKey,
			Type:      "code_patterns",
			Data:      map[string]interface{}{"result": result},
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.config.CacheTTL),
		}
		if err := s.cache.Set(cacheKey, cachedData, s.config.CacheTTL); err != nil {
			s.logger.Error("Failed to cache code analysis result", slog.String("error", err.Error()))
		}
	}

	s.recordMetrics("analyze_code_patterns", time.Since(start), nil)

	s.logger.Info("Code pattern analysis completed",
		slog.Float64("confidence", result.Confidence),
		slog.Int("patterns_found", len(result.Patterns)))

	return result, nil
}

// SuggestDockerfileOptimizations provides AI-powered Dockerfile optimization suggestions
func (s *AnalysisServiceImpl) SuggestDockerfileOptimizations(ctx context.Context, dockerfile string, optContext *services.OptimizationContext) (*services.DockerfileOptimizations, error) {
	s.logger.Info("Starting Dockerfile optimization analysis")

	// Validate input
	if len(dockerfile) > s.config.MaxAnalysisSize {
		return nil, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("Dockerfile size exceeds analysis limit").
			Build()
	}

	// Generate cache key
	cacheKey := s.generateCacheKey("dockerfile_optimization", map[string]interface{}{
		"dockerfile": dockerfile,
		"context":    optContext,
	})

	// Check cache
	if s.config.CacheEnabled {
		if cached, err := s.cache.Get(cacheKey); err == nil && cached != nil {
			if result, ok := cached.Data["result"].(*services.DockerfileOptimizations); ok {
				s.recordCacheHit("dockerfile_optimization")
				return result, nil
			}
		}
	}

	start := time.Now()

	// Build optimization prompt
	optimizationPrompt := s.buildDockerfileOptimizationPrompt(dockerfile, optContext)

	// Create analysis session
	analysisSessionID := fmt.Sprintf("dockerfile_opt_%d", time.Now().Unix())

	response, err := s.conversationSvc.ProcessMessage(ctx, analysisSessionID, optimizationPrompt)
	if err != nil {
		s.recordMetrics("dockerfile_optimization", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to process Dockerfile optimization request").
			Cause(err).
			Build()
	}

	// Parse optimization response
	result, err := s.parseDockerfileOptimizationResponse(response.Message, dockerfile)
	if err != nil {
		s.recordMetrics("dockerfile_optimization", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to parse Dockerfile optimization response").
			Cause(err).
			Build()
	}

	// Cache the result
	if s.config.CacheEnabled {
		cachedData := &services.CachedAnalysis{
			Key:       cacheKey,
			Type:      "dockerfile_optimization",
			Data:      map[string]interface{}{"result": result},
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.config.CacheTTL),
		}
		if err := s.cache.Set(cacheKey, cachedData, s.config.CacheTTL); err != nil {
			s.logger.Error("Failed to cache dockerfile optimization result", slog.String("error", err.Error()))
		}
	}

	s.recordMetrics("dockerfile_optimization", time.Since(start), nil)

	s.logger.Info("Dockerfile optimization completed",
		slog.Int("optimizations", len(result.Optimizations)),
		slog.Float64("estimated_reduction", result.SizeReduction))

	return result, nil
}

// DetectSecurityIssues uses AI to detect potential security vulnerabilities
func (s *AnalysisServiceImpl) DetectSecurityIssues(ctx context.Context, code string, language string) (*services.SecurityAnalysisResult, error) {
	s.logger.Info("Starting security analysis", slog.String("language", language))

	// Validate input
	if len(code) > s.config.MaxAnalysisSize {
		return nil, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("Code size exceeds security analysis limit").
			Build()
	}

	// Generate cache key
	cacheKey := s.generateCacheKey("security_analysis", map[string]interface{}{
		"code":     code,
		"language": language,
	})

	// Check cache
	if s.config.CacheEnabled {
		if cached, err := s.cache.Get(cacheKey); err == nil && cached != nil {
			if result, ok := cached.Data["result"].(*services.SecurityAnalysisResult); ok {
				s.recordCacheHit("security_analysis")
				return result, nil
			}
		}
	}

	start := time.Now()

	// Build security analysis prompt
	securityPrompt := s.buildSecurityAnalysisPrompt(code, language)

	// Create analysis session
	analysisSessionID := fmt.Sprintf("security_analysis_%d", time.Now().Unix())

	response, err := s.conversationSvc.ProcessMessage(ctx, analysisSessionID, securityPrompt)
	if err != nil {
		s.recordMetrics("security_analysis", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to process security analysis request").
			Cause(err).
			Build()
	}

	// Parse security analysis response
	result, err := s.parseSecurityAnalysisResponse(response.Message)
	if err != nil {
		s.recordMetrics("security_analysis", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to parse security analysis response").
			Cause(err).
			Build()
	}

	// Cache the result
	if s.config.CacheEnabled {
		cachedData := &services.CachedAnalysis{
			Key:       cacheKey,
			Type:      "security_analysis",
			Data:      map[string]interface{}{"result": result},
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.config.CacheTTL),
		}
		if err := s.cache.Set(cacheKey, cachedData, s.config.CacheTTL); err != nil {
			s.logger.Error("Failed to cache security analysis result", slog.String("error", err.Error()))
		}
	}

	s.recordMetrics("security_analysis", time.Since(start), nil)

	s.logger.Info("Security analysis completed",
		slog.String("overall_risk", result.OverallRisk),
		slog.Int("issues_found", len(result.Issues)))

	return result, nil
}

// buildCodeAnalysisPrompt creates a prompt for code pattern analysis
func (s *AnalysisServiceImpl) buildCodeAnalysisPrompt(files map[string]string) string {
	var prompt strings.Builder

	prompt.WriteString("Please analyze the following codebase for architectural patterns, code quality, and provide recommendations.\n\n")
	prompt.WriteString("I need a comprehensive analysis including:\n")
	prompt.WriteString("1. Architecture style and patterns\n")
	prompt.WriteString("2. Code quality metrics\n")
	prompt.WriteString("3. Detected design patterns\n")
	prompt.WriteString("4. Dependencies analysis\n")
	prompt.WriteString("5. Recommendations for improvement\n\n")

	prompt.WriteString("Please respond with a JSON object following this structure:\n")
	prompt.WriteString(`{
  "summary": "Brief overall summary",
  "architecture": {
    "style": "microservices|monolith|layered|etc",
    "layers": ["presentation", "business", "data"],
    "patterns": ["MVC", "Repository", "Factory"],
    "violations": ["any anti-patterns"],
    "complexity": 0.0-1.0,
    "maintainability": 0.0-1.0
  },
  "code_quality": {
    "readability": 0.0-1.0,
    "testability": 0.0-1.0,
    "modularity": 0.0-1.0,
    "documentation": 0.0-1.0,
    "error_handling": 0.0-1.0,
    "performance": 0.0-1.0,
    "security": 0.0-1.0,
    "overall_score": 0.0-1.0
  },
  "patterns": [
    {
      "name": "pattern name",
      "type": "design pattern|anti-pattern|best practice",
      "confidence": 0.0-1.0,
      "files": ["file paths"],
      "description": "description",
      "impact": "positive|negative|neutral"
    }
  ],
  "dependencies": [
    {
      "name": "dependency name",
      "version": "version",
      "type": "direct|transitive|dev",
      "risk": "low|medium|high",
      "vulnerabilities": ["list of known issues"],
      "alternatives": ["suggested alternatives"],
      "usage": "how it's used"
    }
  ],
  "recommendations": ["list of recommendations"],
  "confidence": 0.0-1.0
}`)

	prompt.WriteString("\n\nFiles to analyze:\n\n")

	for filename, content := range files {
		prompt.WriteString(fmt.Sprintf("=== %s ===\n", filename))
		// Truncate very large files to avoid token limits
		if len(content) > 5000 {
			prompt.WriteString(content[:5000])
			prompt.WriteString("\n... (truncated)\n")
		} else {
			prompt.WriteString(content)
		}
		prompt.WriteString("\n\n")
	}

	return prompt.String()
}

// generateCacheKey generates a cache key from input data
func (s *AnalysisServiceImpl) generateCacheKey(operation string, data interface{}) string {
	// Create a stable hash of the input data
	content := fmt.Sprintf("%s:%+v", operation, data)
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("ai_%s_%x", operation, hash)
}

// recordMetrics records execution metrics
func (s *AnalysisServiceImpl) recordMetrics(operation string, duration time.Duration, err error) {
	if !s.config.EnableMetrics {
		return
	}

	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.totalRequests++

	if err == nil {
		s.metrics.successfulRequests++
	} else {
		s.metrics.failedRequests++
	}

	s.metrics.responseTimes = append(s.metrics.responseTimes, duration)

	// Track operation-specific metrics
	if _, exists := s.metrics.operationUsage[operation]; !exists {
		s.metrics.operationUsage[operation] = &OperationMetrics{}
	}

	opMetrics := s.metrics.operationUsage[operation]
	opMetrics.Count++
	opMetrics.ResponseTimes = append(opMetrics.ResponseTimes, duration)

	if err != nil {
		opMetrics.Errors++
	}
}

// recordCacheHit records a cache hit for metrics
func (s *AnalysisServiceImpl) recordCacheHit(operation string) {
	if !s.config.EnableMetrics {
		return
	}

	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	if _, exists := s.metrics.operationUsage[operation]; !exists {
		s.metrics.operationUsage[operation] = &OperationMetrics{}
	}

	s.metrics.operationUsage[operation].CacheHits++
}

// parseCodeAnalysisResponse parses the AI response into a CodeAnalysisResult
func (s *AnalysisServiceImpl) parseCodeAnalysisResponse(response string) (*services.CodeAnalysisResult, error) {
	// Extract JSON from the response (may be wrapped in markdown)
	jsonStr := s.extractJSON(response)

	var result services.CodeAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// If JSON parsing fails, create a basic result from the text response
		return &services.CodeAnalysisResult{
			Summary: "Analysis completed - see recommendations for details",
			Architecture: services.ArchitectureAnalysis{
				Style:           "unknown",
				Complexity:      0.5,
				Maintainability: 0.5,
			},
			CodeQuality: services.CodeQualityMetrics{
				OverallScore: 0.5,
			},
			Recommendations: []string{response},
			Confidence:      0.5,
			Metadata: map[string]interface{}{
				"raw_response": response,
			},
		}, nil
	}

	return &result, nil
}

// extractJSON attempts to extract JSON from a potentially markdown-wrapped response
func (s *AnalysisServiceImpl) extractJSON(response string) string {
	// Look for JSON code blocks
	if start := strings.Index(response, "```json"); start != -1 {
		start += 7 // Skip "```json"
		if end := strings.Index(response[start:], "```"); end != -1 {
			return response[start : start+end]
		}
	}

	// Look for JSON objects
	if start := strings.Index(response, "{"); start != -1 {
		if end := strings.LastIndex(response, "}"); end != -1 && end > start {
			return response[start : end+1]
		}
	}

	// Return the whole response if no JSON structure found
	return response
}
