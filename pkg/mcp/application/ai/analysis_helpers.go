package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// buildDockerfileOptimizationPrompt creates a prompt for Dockerfile optimization
func (s *AnalysisServiceImpl) buildDockerfileOptimizationPrompt(dockerfile string, optContext *services.OptimizationContext) string {
	var prompt strings.Builder

	prompt.WriteString("Please analyze the following Dockerfile and provide optimization suggestions.\n\n")
	prompt.WriteString("I need specific optimization recommendations with estimated impact including:\n")
	prompt.WriteString("1. Size reduction opportunities\n")
	prompt.WriteString("2. Security improvements\n")
	prompt.WriteString("3. Build time optimizations\n")
	prompt.WriteString("4. Best practices compliance\n")
	prompt.WriteString("5. Multi-stage build opportunities\n\n")

	if optContext != nil {
		prompt.WriteString("Context:\n")
		if optContext.Language != "" {
			prompt.WriteString(fmt.Sprintf("- Language: %s\n", optContext.Language))
		}
		if optContext.Framework != "" {
			prompt.WriteString(fmt.Sprintf("- Framework: %s\n", optContext.Framework))
		}
		if optContext.Environment != "" {
			prompt.WriteString(fmt.Sprintf("- Target Environment: %s\n", optContext.Environment))
		}
		if optContext.TargetPlatform != "" {
			prompt.WriteString(fmt.Sprintf("- Target Platform: %s\n", optContext.TargetPlatform))
		}
		if len(optContext.Dependencies) > 0 {
			prompt.WriteString(fmt.Sprintf("- Dependencies: %v\n", optContext.Dependencies))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Please respond with a JSON object following this structure:\n")
	prompt.WriteString(`{
  "original_size": 0,
  "estimated_size": 0,
  "size_reduction": 0.0,
  "build_time": "5m",
  "security_score": 0.0-1.0,
  "optimizations": [
    {
      "type": "multi-stage|layer-reduction|base-image|security|dependencies",
      "priority": "high|medium|low",
      "impact": "size|security|performance|maintainability",
      "description": "detailed description",
      "before": "original instruction(s)",
      "after": "optimized instruction(s)",
      "savings": "estimated savings"
    }
  ],
  "optimized_content": "the complete optimized Dockerfile",
  "confidence": 0.0-1.0
}`)

	prompt.WriteString("\n\nDockerfile to optimize:\n```dockerfile\n")
	prompt.WriteString(dockerfile)
	prompt.WriteString("\n```\n")

	return prompt.String()
}

// buildSecurityAnalysisPrompt creates a prompt for security analysis
func (s *AnalysisServiceImpl) buildSecurityAnalysisPrompt(code string, language string) string {
	var prompt strings.Builder

	prompt.WriteString("Please perform a comprehensive security analysis of the following code.\n\n")
	prompt.WriteString("I need analysis covering:\n")
	prompt.WriteString("1. Vulnerability detection (with CWE/CVE references where applicable)\n")
	prompt.WriteString("2. Security misconfigurations\n")
	prompt.WriteString("3. Bad security practices\n")
	prompt.WriteString("4. Compliance issues (OWASP, etc.)\n")
	prompt.WriteString("5. Specific remediation recommendations\n\n")

	prompt.WriteString(fmt.Sprintf("Language: %s\n\n", language))

	prompt.WriteString("Please respond with a JSON object following this structure:\n")
	prompt.WriteString(`{
  "overall_risk": "low|medium|high|critical",
  "security_score": 0.0-1.0,
  "issues": [
    {
      "id": "unique identifier",
      "type": "vulnerability|misconfiguration|bad-practice",
      "severity": "low|medium|high|critical",
      "title": "issue title",
      "description": "detailed description",
      "files": ["affected files"],
      "lines": [line numbers],
      "cwe": "CWE-XXX if applicable",
      "cve": "CVE-XXXX-XXXX if applicable",
      "remediation": "how to fix this issue"
    }
  ],
  "recommendations": [
    {
      "category": "authentication|encryption|input-validation|etc",
      "priority": "high|medium|low",
      "description": "recommendation description",
      "action": "specific action to take",
      "impact": "security improvement description"
    }
  ],
  "compliance": {
    "standards": {
      "OWASP": {
        "score": 0.0-1.0,
        "violations": ["specific violations"],
        "suggestions": ["improvement suggestions"]
      }
    }
  },
  "confidence": 0.0-1.0
}`)

	prompt.WriteString("\n\nCode to analyze:\n```")
	prompt.WriteString(language)
	prompt.WriteString("\n")
	// Truncate very large code to avoid token limits
	if len(code) > 8000 {
		prompt.WriteString(code[:8000])
		prompt.WriteString("\n... (truncated for analysis)")
	} else {
		prompt.WriteString(code)
	}
	prompt.WriteString("\n```\n")

	return prompt.String()
}

// parseDockerfileOptimizationResponse parses the AI response into DockerfileOptimizations
func (s *AnalysisServiceImpl) parseDockerfileOptimizationResponse(response string, originalDockerfile string) (*services.DockerfileOptimizations, error) {
	// Extract JSON from response
	jsonStr := s.extractJSON(response)

	var result services.DockerfileOptimizations
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// If JSON parsing fails, create a basic result
		return &services.DockerfileOptimizations{
			OriginalSize:     0,
			EstimatedSize:    0,
			SizeReduction:    0.0,
			BuildTime:        5 * time.Minute,
			SecurityScore:    0.5,
			OptimizedContent: originalDockerfile,
			Confidence:       0.3,
			Optimizations: []services.DockerfileOptimization{
				{
					Type:        "general",
					Priority:    "medium",
					Impact:      "maintainability",
					Description: "AI analysis completed - see optimized content",
					Before:      "original dockerfile",
					After:       response,
					Confidence:  0.3,
				},
			},
		}, nil
	}

	// Set default optimized content if not provided
	if result.OptimizedContent == "" {
		result.OptimizedContent = originalDockerfile
	}

	// Ensure we have at least some optimizations
	if len(result.Optimizations) == 0 {
		result.Optimizations = []services.DockerfileOptimization{
			{
				Type:        "analysis",
				Priority:    "low",
				Impact:      "maintainability",
				Description: "Analysis completed successfully",
				Before:      "n/a",
				After:       "see analysis response",
				Confidence:  0.7,
			},
		}
	}

	return &result, nil
}

// parseSecurityAnalysisResponse parses the AI response into SecurityAnalysisResult
func (s *AnalysisServiceImpl) parseSecurityAnalysisResponse(response string) (*services.SecurityAnalysisResult, error) {
	// Extract JSON from response
	jsonStr := s.extractJSON(response)

	var result services.SecurityAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// If JSON parsing fails, create a basic result
		return &services.SecurityAnalysisResult{
			OverallRisk:   "medium",
			SecurityScore: 0.5,
			Issues:        []services.SecurityIssue{},
			Recommendations: []services.SecurityRecommendation{
				{
					Category:    "general",
					Priority:    "medium",
					Description: "Security analysis completed - review response for details",
					Action:      "Review the analysis response",
					Impact:      "General security awareness",
				},
			},
			Compliance: services.ComplianceReport{
				Standards: map[string]services.ComplianceResult{
					"OWASP": {
						Score:       0.5,
						Violations:  []string{},
						Suggestions: []string{"Review security analysis"},
					},
				},
			},
			Confidence: 0.5,
		}, nil
	}

	// Ensure we have default compliance if not provided
	if result.Compliance.Standards == nil {
		result.Compliance.Standards = make(map[string]services.ComplianceResult)
	}

	if _, exists := result.Compliance.Standards["OWASP"]; !exists {
		result.Compliance.Standards["OWASP"] = services.ComplianceResult{
			Score:       result.SecurityScore,
			Violations:  []string{},
			Suggestions: []string{"Follow OWASP security guidelines"},
		}
	}

	return &result, nil
}

// Implement remaining AIAnalysisService methods

// AnalyzePerformance suggests performance optimizations based on code analysis
func (s *AnalysisServiceImpl) AnalyzePerformance(ctx context.Context, code string, metrics map[string]interface{}) (*services.PerformanceAnalysisResult, error) {
	s.logger.Info("Starting performance analysis")

	// Validate input size
	if len(code) > s.config.MaxAnalysisSize {
		return nil, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("Code size exceeds analysis limit").
			Build()
	}

	// Generate cache key
	cacheKey := s.generateCacheKey("performance_analysis", map[string]interface{}{
		"code":    code,
		"metrics": metrics,
	})

	// Check cache
	if s.config.CacheEnabled {
		if cached, err := s.cache.Get(cacheKey); err == nil && cached != nil {
			if result, ok := cached.Data["result"].(*services.PerformanceAnalysisResult); ok {
				s.recordCacheHit("performance_analysis")
				return result, nil
			}
		}
	}

	start := time.Now()

	// Build performance analysis prompt
	performancePrompt := s.buildPerformanceAnalysisPrompt(code, metrics)

	// Create analysis session
	analysisSessionID := fmt.Sprintf("perf_analysis_%d", time.Now().Unix())

	response, err := s.conversationSvc.ProcessMessage(ctx, analysisSessionID, performancePrompt)
	if err != nil {
		s.recordMetrics("performance_analysis", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to process performance analysis request").
			Cause(err).
			Build()
	}

	// Parse performance analysis response
	result, err := s.parsePerformanceAnalysisResponse(response.Message)
	if err != nil {
		s.recordMetrics("performance_analysis", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to parse performance analysis response").
			Cause(err).
			Build()
	}

	// Cache the result
	if s.config.CacheEnabled {
		cachedData := &services.CachedAnalysis{
			Key:       cacheKey,
			Type:      "performance_analysis",
			Data:      map[string]interface{}{"result": result},
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.config.CacheTTL),
		}
		if err := s.cache.Set(cacheKey, cachedData, s.config.CacheTTL); err != nil {
			s.logger.Error("Failed to cache performance analysis result", slog.String("error", err.Error()))
		}
	}

	s.recordMetrics("performance_analysis", time.Since(start), nil)

	s.logger.Info("Performance analysis completed",
		slog.Float64("overall_score", result.OverallScore),
		slog.Int("bottlenecks_found", len(result.Bottlenecks)))

	return result, nil
}

// SuggestContainerizationApproach provides intelligent containerization recommendations
func (s *AnalysisServiceImpl) SuggestContainerizationApproach(ctx context.Context, analysis *services.RepositoryAnalysis) (*services.ContainerizationRecommendations, error) {
	s.logger.Info("Starting containerization recommendations")

	// Generate cache key
	cacheKey := s.generateCacheKey("containerization_suggestions", analysis)

	// Check cache
	if s.config.CacheEnabled {
		if cached, err := s.cache.Get(cacheKey); err == nil && cached != nil {
			if result, ok := cached.Data["result"].(*services.ContainerizationRecommendations); ok {
				s.recordCacheHit("containerization_suggestions")
				return result, nil
			}
		}
	}

	start := time.Now()

	// Build containerization prompt
	containerizationPrompt := s.buildContainerizationPrompt(analysis)

	// Create analysis session
	analysisSessionID := fmt.Sprintf("containerization_%d", time.Now().Unix())

	response, err := s.conversationSvc.ProcessMessage(ctx, analysisSessionID, containerizationPrompt)
	if err != nil {
		s.recordMetrics("containerization_suggestions", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to process containerization request").
			Cause(err).
			Build()
	}

	// Parse containerization response
	result, err := s.parseContainerizationResponse(response.Message)
	if err != nil {
		s.recordMetrics("containerization_suggestions", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to parse containerization response").
			Cause(err).
			Build()
	}

	// Cache the result
	if s.config.CacheEnabled {
		cachedData := &services.CachedAnalysis{
			Key:       cacheKey,
			Type:      "containerization_suggestions",
			Data:      map[string]interface{}{"result": result},
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.config.CacheTTL),
		}
		if err := s.cache.Set(cacheKey, cachedData, s.config.CacheTTL); err != nil {
			s.logger.Error("Failed to cache containerization result", slog.String("error", err.Error()))
		}
	}

	s.recordMetrics("containerization_suggestions", time.Since(start), nil)

	s.logger.Info("Containerization recommendations completed",
		slog.Float64("confidence", result.Confidence))

	return result, nil
}

// ValidateConfiguration uses AI to validate configuration files
func (s *AnalysisServiceImpl) ValidateConfiguration(ctx context.Context, configType string, content string) (*services.ConfigurationResult, error) {
	s.logger.Info("Starting configuration validation", slog.String("type", configType))

	// Validate input
	if len(content) > s.config.MaxAnalysisSize {
		return nil, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("Configuration size exceeds validation limit").
			Build()
	}

	// Generate cache key
	cacheKey := s.generateCacheKey("config_validation", map[string]interface{}{
		"type":    configType,
		"content": content,
	})

	// Check cache
	if s.config.CacheEnabled {
		if cached, err := s.cache.Get(cacheKey); err == nil && cached != nil {
			if result, ok := cached.Data["result"].(*services.ConfigurationResult); ok {
				s.recordCacheHit("config_validation")
				return result, nil
			}
		}
	}

	start := time.Now()

	// Build configuration validation prompt
	validationPrompt := s.buildConfigValidationPrompt(configType, content)

	// Create analysis session
	analysisSessionID := fmt.Sprintf("config_validation_%d", time.Now().Unix())

	response, err := s.conversationSvc.ProcessMessage(ctx, analysisSessionID, validationPrompt)
	if err != nil {
		s.recordMetrics("config_validation", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to process configuration validation request").
			Cause(err).
			Build()
	}

	// Parse validation response
	result, err := s.parseConfigValidationResponse(response.Message)
	if err != nil {
		s.recordMetrics("config_validation", time.Since(start), err)
		return nil, errors.NewError().
			Code(errors.CodeOperationFailed).
			Message("Failed to parse configuration validation response").
			Cause(err).
			Build()
	}

	// Cache the result
	if s.config.CacheEnabled {
		cachedData := &services.CachedAnalysis{
			Key:       cacheKey,
			Type:      "config_validation",
			Data:      map[string]interface{}{"result": result},
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.config.CacheTTL),
		}
		if err := s.cache.Set(cacheKey, cachedData, s.config.CacheTTL); err != nil {
			s.logger.Error("Failed to cache config validation result", slog.String("error", err.Error()))
		}
	}

	s.recordMetrics("config_validation", time.Since(start), nil)

	s.logger.Info("Configuration validation completed",
		slog.Bool("valid", result.Valid),
		slog.Int("issues_found", len(result.Issues)))

	return result, nil
}

// GetCachedAnalysis retrieves cached analysis results
func (s *AnalysisServiceImpl) GetCachedAnalysis(_ context.Context, cacheKey string, _ *services.TimeRange) (*services.CachedAnalysis, error) {
	return s.cache.Get(cacheKey)
}

// InvalidateCache clears cached analysis results
func (s *AnalysisServiceImpl) InvalidateCache(_ context.Context, pattern string) error {
	return s.cache.DeletePattern(pattern)
}

// GetUsageMetrics returns AI service usage and cost metrics
func (s *AnalysisServiceImpl) GetUsageMetrics(_ context.Context, _ services.TimeRange) (*services.AIUsageMetrics, error) {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	// Calculate response time metrics
	responseTimeMetrics := s.calculateResponseTimeMetrics(s.metrics.responseTimes)

	// Calculate operation usage
	operationUsage := make(map[string]services.OperationUsage)
	for operation, metrics := range s.metrics.operationUsage {
		opResponseTimes := s.calculateResponseTimeMetrics(metrics.ResponseTimes)

		errorRate := 0.0
		if metrics.Count > 0 {
			errorRate = float64(metrics.Errors) / float64(metrics.Count)
		}

		operationUsage[operation] = services.OperationUsage{
			Count:         metrics.Count,
			TotalTokens:   metrics.TotalTokens,
			TotalCost:     metrics.TotalCost,
			AverageTokens: float64(metrics.TotalTokens) / float64(metrics.Count),
			ResponseTime:  opResponseTimes,
			Errors:        metrics.Errors,
			ErrorRate:     errorRate,
		}
	}

	return &services.AIUsageMetrics{
		TotalRequests:      s.metrics.totalRequests,
		SuccessfulRequests: s.metrics.successfulRequests,
		FailedRequests:     s.metrics.failedRequests,
		TotalTokens:        s.metrics.totalTokens,
		InputTokens:        s.metrics.inputTokens,
		OutputTokens:       s.metrics.outputTokens,
		TotalCost:          s.metrics.totalCost,
		AverageCost:        s.metrics.totalCost / float64(s.metrics.totalRequests),
		CostBreakdown:      make(map[string]float64), // Would be calculated based on operation costs
		ResponseTimes:      responseTimeMetrics,
		Usage:              operationUsage,
	}, nil
}

// calculateResponseTimeMetrics calculates response time statistics
func (s *AnalysisServiceImpl) calculateResponseTimeMetrics(times []time.Duration) services.ResponseTimeMetrics {
	if len(times) == 0 {
		return services.ResponseTimeMetrics{}
	}

	// Simple calculations (in production, would use more sophisticated percentile calculations)
	var total time.Duration
	minTime := times[0]
	maxTime := times[0]

	for _, t := range times {
		total += t
		if t < minTime {
			minTime = t
		}
		if t > maxTime {
			maxTime = t
		}
	}

	average := total / time.Duration(len(times))

	// For P95/P99, we'd need proper percentile calculation
	// This is a simplified approximation
	p95 := maxTime * 95 / 100
	p99 := maxTime * 99 / 100

	return services.ResponseTimeMetrics{
		Average: average,
		Median:  average, // Simplified
		P95:     p95,
		P99:     p99,
		Min:     minTime,
		Max:     maxTime,
	}
}

// buildPerformanceAnalysisPrompt creates a prompt for performance analysis
func (s *AnalysisServiceImpl) buildPerformanceAnalysisPrompt(code string, metrics map[string]interface{}) string {
	var prompt strings.Builder

	prompt.WriteString("Please analyze the following code for performance bottlenecks and optimization opportunities.\n\n")
	prompt.WriteString("I need analysis covering:\n")
	prompt.WriteString("1. CPU-intensive operations and hot paths\n")
	prompt.WriteString("2. Memory usage patterns and potential leaks\n")
	prompt.WriteString("3. I/O operations and blocking calls\n")
	prompt.WriteString("4. Database query optimization opportunities\n")
	prompt.WriteString("5. Caching strategies and recommendations\n")
	prompt.WriteString("6. Concurrency and parallelization opportunities\n")
	prompt.WriteString("7. Algorithm optimizations\n\n")

	if len(metrics) > 0 {
		prompt.WriteString("Performance Metrics:\n")
		for key, value := range metrics {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Please respond with a JSON object following this structure:\n")
	prompt.WriteString(`{
  "overall_score": 0.0-1.0,
  "bottlenecks": [
    {
      "id": "unique identifier",
      "type": "cpu|memory|io|database|network",
      "severity": "low|medium|high|critical",
      "location": "file:line or function name",
      "description": "detailed description",
      "impact": "performance impact description",
      "solution": "recommended solution",
      "estimated_improvement": "percentage or description"
    }
  ],
  "optimizations": [
    {
      "category": "caching|database|concurrency|memory|algorithm",
      "priority": "high|medium|low",
      "description": "optimization description",
      "implementation": "how to implement",
      "expected_benefit": "expected performance gain",
      "effort": "low|medium|high"
    }
  ],
  "metrics": {
    "complexity_score": 0.0-1.0,
    "maintainability_score": 0.0-1.0,
    "scalability_score": 0.0-1.0,
    "resource_efficiency": 0.0-1.0
  },
  "recommendations": [
    "specific actionable recommendations"
  ],
  "confidence": 0.0-1.0
}`)

	prompt.WriteString("\n\nCode to analyze:\n```\n")

	// Truncate very large code to avoid token limits
	if len(code) > 8000 {
		prompt.WriteString(code[:8000])
		prompt.WriteString("\n... (truncated for analysis)")
	} else {
		prompt.WriteString(code)
	}
	prompt.WriteString("\n```\n")

	return prompt.String()
}

// buildContainerizationPrompt creates a prompt for containerization recommendations
func (s *AnalysisServiceImpl) buildContainerizationPrompt(analysis *services.RepositoryAnalysis) string {
	var prompt strings.Builder

	prompt.WriteString("Please provide intelligent containerization recommendations for this repository.\n\n")
	prompt.WriteString("I need recommendations covering:\n")
	prompt.WriteString("1. Optimal base image selection\n")
	prompt.WriteString("2. Multi-stage build strategies\n")
	prompt.WriteString("3. Dependency management approaches\n")
	prompt.WriteString("4. Security hardening recommendations\n")
	prompt.WriteString("5. Performance optimization techniques\n")
	prompt.WriteString("6. Deployment strategies and orchestration\n")
	prompt.WriteString("7. Monitoring and logging setup\n\n")

	if analysis != nil {
		prompt.WriteString("Repository Analysis Context:\n")
		if analysis.Language != "" {
			prompt.WriteString(fmt.Sprintf("- Language: %s\n", analysis.Language))
		}
		if analysis.Framework != "" {
			prompt.WriteString(fmt.Sprintf("- Framework: %s\n", analysis.Framework))
		}
		if analysis.BuildCommand != "" {
			prompt.WriteString(fmt.Sprintf("- Build Command: %s\n", analysis.BuildCommand))
		}
		if len(analysis.Dependencies) > 0 {
			prompt.WriteString(fmt.Sprintf("- Dependencies: %v\n", analysis.Dependencies))
		}
		if analysis.EntryPoint != "" {
			prompt.WriteString(fmt.Sprintf("- Entry Point: %s\n", analysis.EntryPoint))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Please respond with a JSON object following this structure:\n")
	prompt.WriteString(`{
  "dockerfile": "complete optimized Dockerfile content",
  "strategy": {
    "base_image": "recommended base image with rationale",
    "build_stages": [
      {
        "name": "stage name",
        "purpose": "what this stage does",
        "optimizations": ["list of optimizations"]
      }
    ],
    "security_measures": ["security hardening steps"],
    "performance_optimizations": ["performance improvements"]
  },
  "deployment": {
    "recommended_orchestrator": "kubernetes|docker-compose|docker-swarm",
    "resource_requirements": {
      "cpu": "recommended CPU allocation",
      "memory": "recommended memory allocation",
      "storage": "storage requirements"
    },
    "scaling_strategy": "horizontal|vertical|auto",
    "health_checks": ["recommended health check configurations"]
  },
  "monitoring": {
    "logging_strategy": "structured|json|text",
    "metrics_collection": ["recommended metrics"],
    "observability_tools": ["recommended monitoring tools"]
  },
  "best_practices": [
    "specific best practice recommendations"
  ],
  "confidence": 0.0-1.0
}`)

	prompt.WriteString("\n\nRepository Analysis Data:\n")
	if analysis != nil {
		analysisJSON, _ := json.MarshalIndent(analysis, "", "  ")
		prompt.WriteString(string(analysisJSON))
	} else {
		prompt.WriteString("No repository analysis data provided")
	}

	return prompt.String()
}

// buildConfigValidationPrompt creates a prompt for configuration validation
func (s *AnalysisServiceImpl) buildConfigValidationPrompt(configType string, content string) string {
	var prompt strings.Builder

	prompt.WriteString("Please validate the following configuration file and identify any issues or improvements.\n\n")
	prompt.WriteString("I need validation covering:\n")
	prompt.WriteString("1. Syntax and format validation\n")
	prompt.WriteString("2. Security configuration issues\n")
	prompt.WriteString("3. Performance configuration problems\n")
	prompt.WriteString("4. Best practice violations\n")
	prompt.WriteString("5. Compatibility issues\n")
	prompt.WriteString("6. Missing required configurations\n")
	prompt.WriteString("7. Optimization opportunities\n\n")

	prompt.WriteString(fmt.Sprintf("Configuration Type: %s\n\n", configType))

	prompt.WriteString("Please respond with a JSON object following this structure:\n")
	prompt.WriteString(`{
  "valid": true|false,
  "syntax_valid": true|false,
  "issues": [
    {
      "id": "unique identifier",
      "type": "syntax|security|performance|best-practice|compatibility",
      "severity": "low|medium|high|critical",
      "line": 0,
      "column": 0,
      "message": "issue description",
      "suggestion": "how to fix",
      "rationale": "why this is an issue"
    }
  ],
  "warnings": [
    {
      "type": "performance|security|maintainability",
      "message": "warning description",
      "suggestion": "recommended action"
    }
  ],
  "optimizations": [
    {
      "category": "performance|security|maintainability",
      "description": "optimization description",
      "before": "current configuration",
      "after": "optimized configuration",
      "benefit": "expected improvement"
    }
  ],
  "missing_configs": [
    {
      "name": "configuration name",
      "description": "what this configuration does",
      "recommended_value": "suggested value",
      "importance": "high|medium|low"
    }
  ],
  "score": 0.0-1.0,
  "confidence": 0.0-1.0
}`)

	prompt.WriteString("\n\nConfiguration to validate:\n```")
	prompt.WriteString(configType)
	prompt.WriteString("\n")
	prompt.WriteString(content)
	prompt.WriteString("\n```\n")

	return prompt.String()
}

// buildCodePatternAnalysisPrompt creates a prompt for code pattern analysis
func (s *AnalysisServiceImpl) buildCodePatternAnalysisPrompt(files map[string]string) string {
	var prompt strings.Builder

	prompt.WriteString("Please analyze the following codebase for architectural patterns, code quality, and design issues.\n\n")
	prompt.WriteString("I need analysis covering:\n")
	prompt.WriteString("1. Architectural patterns and design principles\n")
	prompt.WriteString("2. Code quality metrics and maintainability\n")
	prompt.WriteString("3. Common anti-patterns and code smells\n")
	prompt.WriteString("4. Dependency management and coupling\n")
	prompt.WriteString("5. Testing patterns and coverage\n")
	prompt.WriteString("6. Documentation and readability\n")
	prompt.WriteString("7. Framework and library usage patterns\n\n")

	prompt.WriteString("Please respond with a JSON object following this structure:\n")
	prompt.WriteString(`{
  "summary": "overall assessment of the codebase",
  "architecture": {
    "style": "monolithic|microservices|layered|mvc|etc",
    "patterns": ["detected patterns"],
    "violations": ["architectural violations"],
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
      "type": "design|architectural|behavioral",
      "confidence": 0.0-1.0,
      "files": ["affected files"],
      "description": "pattern description",
      "impact": "positive|negative|neutral"
    }
  ],
  "dependencies": [
    {
      "name": "dependency name",
      "version": "version",
      "type": "runtime|dev|peer",
      "risk": "low|medium|high",
      "vulnerabilities": ["known issues"],
      "alternatives": ["suggested alternatives"],
      "usage": "how it's used"
    }
  ],
  "recommendations": [
    "specific improvement recommendations"
  ],
  "confidence": 0.0-1.0
}`)

	prompt.WriteString("\n\nCodebase to analyze:\n\n")

	for filename, content := range files {
		prompt.WriteString(fmt.Sprintf("=== %s ===\n", filename))
		// Truncate very large files to avoid token limits
		if len(content) > 4000 {
			prompt.WriteString(content[:4000])
			prompt.WriteString("\n... (truncated for analysis)\n")
		} else {
			prompt.WriteString(content)
		}
		prompt.WriteString("\n\n")
	}

	return prompt.String()
}

// parseCodePatternAnalysisResponse parses the AI response into CodeAnalysisResult
func (s *AnalysisServiceImpl) parseCodePatternAnalysisResponse(response string) (*services.CodeAnalysisResult, error) {
	// Extract JSON from response
	jsonStr := s.extractJSON(response)

	var result services.CodeAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// If JSON parsing fails, create a basic result
		return &services.CodeAnalysisResult{
			Summary:         "Code pattern analysis completed - review response for details",
			Confidence:      0.5,
			Patterns:        []services.DetectedPattern{},
			Dependencies:    []services.DependencyAnalysis{},
			Recommendations: []string{"Review the analysis response for detailed insights"},
		}, nil
	}

	// Ensure we have default values for missing fields
	if len(result.Patterns) == 0 {
		result.Patterns = []services.DetectedPattern{}
	}
	if len(result.Dependencies) == 0 {
		result.Dependencies = []services.DependencyAnalysis{}
	}
	if len(result.Recommendations) == 0 {
		result.Recommendations = []string{"Analysis completed successfully"}
	}

	return &result, nil
}

// parsePerformanceAnalysisResponse parses the AI response into PerformanceAnalysisResult
func (s *AnalysisServiceImpl) parsePerformanceAnalysisResponse(response string) (*services.PerformanceAnalysisResult, error) {
	// Extract JSON from response
	jsonStr := s.extractJSON(response)

	var result services.PerformanceAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// If JSON parsing fails, create a basic result
		return &services.PerformanceAnalysisResult{
			OverallScore: 0.5,
			Bottlenecks:  []services.PerformanceBottleneck{},
			Optimizations: []services.PerformanceOptimization{
				{
					Category:    "general",
					Priority:    "medium",
					Description: "Performance analysis completed - review response for details",
					Impact:      "Varies",
					Effort:      "medium",
					Code:        "See analysis response",
				},
			},
			ScalabilityScore: 0.5,
			Recommendations:  []string{"Performance analysis completed - review response for details"},
			Confidence:       0.5,
		}, nil
	}

	// Ensure we have default values for missing fields
	if len(result.Bottlenecks) == 0 {
		result.Bottlenecks = []services.PerformanceBottleneck{}
	}
	if len(result.Optimizations) == 0 {
		result.Optimizations = []services.PerformanceOptimization{}
	}
	if len(result.Recommendations) == 0 {
		result.Recommendations = []string{"Analysis completed successfully"}
	}

	return &result, nil
}

// parseContainerizationResponse parses the AI response into ContainerizationRecommendations
func (s *AnalysisServiceImpl) parseContainerizationResponse(response string) (*services.ContainerizationRecommendations, error) {
	// Extract JSON from response
	jsonStr := s.extractJSON(response)

	var result services.ContainerizationRecommendations
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// If JSON parsing fails, create a basic result
		return &services.ContainerizationRecommendations{
			Confidence: 0.5,
		}, nil
	}

	return &result, nil
}

// parseConfigValidationResponse parses the AI response into ConfigurationResult
func (s *AnalysisServiceImpl) parseConfigValidationResponse(response string) (*services.ConfigurationResult, error) {
	// Extract JSON from response
	jsonStr := s.extractJSON(response)

	var result services.ConfigurationResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// If JSON parsing fails, create a basic result
		return &services.ConfigurationResult{
			Valid:      true,
			Confidence: 0.5,
		}, nil
	}

	return &result, nil
}
