package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// BuildStrategizer optimizes build strategies based on context and requirements
type BuildStrategizer struct {
	strategyDatabase *StrategyDatabase
	optimizer        *StrategyOptimizer
	logger           zerolog.Logger
}

// NewBuildStrategizer creates a new build strategizer
func NewBuildStrategizer(logger zerolog.Logger) *BuildStrategizer {
	return &BuildStrategizer{
		strategyDatabase: NewStrategyDatabase(),
		optimizer:        NewStrategyOptimizer(logger),
		logger:           logger.With().Str("component", "build_strategizer").Logger(),
	}
}

// OptimizeStrategy optimizes build strategy based on requirements
func (bs *BuildStrategizer) OptimizeStrategy(ctx context.Context, request *BuildOptimizationRequest) (*OptimizedBuildStrategy, error) {
	bs.logger.Info().
		Str("session_id", request.SessionID).
		Str("project_type", request.ProjectType).
		Str("primary_goal", request.Goals.PrimarGoal).
		Msg("Starting build strategy optimization")

	// Get base strategies for the project type
	baseStrategies := bs.strategyDatabase.GetStrategiesForProjectType(request.ProjectType)

	// Filter strategies based on constraints
	viableStrategies := bs.filterByConstraints(baseStrategies, request.Constraints)

	// Optimize strategies based on goals
	optimizedStrategies := bs.optimizer.OptimizeStrategies(viableStrategies, request.Goals)

	// Select the best strategy
	bestStrategy, err := bs.selectBestStrategy(optimizedStrategies, request)
	if err != nil {
		return nil, fmt.Errorf("failed to select best strategy: %w", err)
	}

	// Enhance strategy with context-specific optimizations
	enhancedStrategy := bs.enhanceWithContext(bestStrategy, request.Context)

	bs.logger.Info().
		Str("strategy_name", enhancedStrategy.Name).
		Dur("expected_duration", enhancedStrategy.ExpectedDuration).
		Str("risk_level", enhancedStrategy.RiskAssessment.OverallRisk).
		Msg("Build strategy optimization completed")

	return enhancedStrategy, nil
}

// filterByConstraints filters strategies that meet the given constraints
func (bs *BuildStrategizer) filterByConstraints(strategies []*OptimizedBuildStrategy, constraints *BuildConstraints) []*OptimizedBuildStrategy {
	filtered := []*OptimizedBuildStrategy{}

	for _, strategy := range strategies {
		if bs.meetsConstraints(strategy, constraints) {
			filtered = append(filtered, strategy)
		}
	}

	// If no strategies meet constraints, relax them and try again
	if len(filtered) == 0 {
		bs.logger.Warn().Msg("No strategies meet constraints, relaxing constraints")
		relaxedConstraints := bs.relaxConstraints(constraints)
		for _, strategy := range strategies {
			if bs.meetsConstraints(strategy, relaxedConstraints) {
				filtered = append(filtered, strategy)
			}
		}
	}

	return filtered
}

func (bs *BuildStrategizer) meetsConstraints(strategy *OptimizedBuildStrategy, constraints *BuildConstraints) bool {
	// Check duration constraint
	if constraints.MaxDuration > 0 && strategy.ExpectedDuration > constraints.MaxDuration {
		return false
	}

	// Check memory constraint
	if constraints.MaxMemory > 0 && strategy.ResourceUsage.Memory > constraints.MaxMemory {
		return false
	}

	// Check CPU constraint
	if constraints.MaxCPU > 0 && strategy.ResourceUsage.CPU > constraints.MaxCPU {
		return false
	}

	// Check allowed tools
	if len(constraints.AllowedTools) > 0 {
		for _, step := range strategy.Steps {
			toolAllowed := false
			for _, allowedTool := range constraints.AllowedTools {
				if strings.Contains(step.Command, allowedTool) {
					toolAllowed = true
					break
				}
			}
			if !toolAllowed {
				return false
			}
		}
	}

	// Check security level
	if constraints.SecurityLevel != "" {
		if !bs.meetsSecurityLevel(strategy, constraints.SecurityLevel) {
			return false
		}
	}

	return true
}

func (bs *BuildStrategizer) meetsSecurityLevel(strategy *OptimizedBuildStrategy, requiredLevel string) bool {
	// Simple security level check - could be enhanced
	switch requiredLevel {
	case "high":
		return strategy.RiskAssessment.OverallRisk == "low"
	case "medium":
		return strategy.RiskAssessment.OverallRisk == "low" || strategy.RiskAssessment.OverallRisk == "medium"
	case "low":
		return true
	default:
		return true
	}
}

func (bs *BuildStrategizer) relaxConstraints(constraints *BuildConstraints) *BuildConstraints {
	relaxed := &BuildConstraints{
		MaxDuration:   constraints.MaxDuration + time.Minute*10, // Add 10 minutes
		MaxMemory:     constraints.MaxMemory * 2,                // Double memory limit
		MaxCPU:        constraints.MaxCPU * 1.5,                 // Increase CPU by 50%
		AllowedTools:  constraints.AllowedTools,                 // Keep tool restrictions
		SecurityLevel: constraints.SecurityLevel,                // Keep security level
	}

	// If duration was 0 (no limit), set a reasonable default
	if constraints.MaxDuration == 0 {
		relaxed.MaxDuration = time.Hour
	}

	return relaxed
}

func (bs *BuildStrategizer) selectBestStrategy(strategies []*OptimizedBuildStrategy, request *BuildOptimizationRequest) (*OptimizedBuildStrategy, error) {
	if len(strategies) == 0 {
		return nil, fmt.Errorf("no viable strategies available")
	}

	// Score each strategy based on goals
	bestStrategy := strategies[0]
	bestScore := bs.scoreStrategy(bestStrategy, request.Goals)

	for _, strategy := range strategies[1:] {
		score := bs.scoreStrategy(strategy, request.Goals)
		if score > bestScore {
			bestStrategy = strategy
			bestScore = score
		}
	}

	return bestStrategy, nil
}

func (bs *BuildStrategizer) scoreStrategy(strategy *OptimizedBuildStrategy, goals *OptimizationGoals) float64 {
	score := 0.0

	// Score based on primary goal
	score += bs.scorePrimaryGoal(strategy, goals.PrimarGoal)

	// Adjust for risk tolerance
	score += bs.scoreRiskTolerance(strategy, goals.AcceptableRisk)

	// Adjust for time constraints
	score += bs.scoreTimeConstraints(strategy, goals.TimeConstraints)

	// Adjust for quality level
	score += bs.scoreQualityLevel(strategy, goals.QualityLevel)

	return score
}

func (bs *BuildStrategizer) scorePrimaryGoal(strategy *OptimizedBuildStrategy, primaryGoal string) float64 {
	switch primaryGoal {
	case "speed":
		return bs.scoreSpeed(strategy.ExpectedDuration)
	case "size":
		return bs.scoreSize(strategy.ResourceUsage.Disk)
	case "security":
		return bs.scoreSecurity(strategy.RiskAssessment.OverallRisk)
	case "reliability":
		return bs.scoreReliability(len(strategy.RiskAssessment.FailurePoints))
	default:
		return 0
	}
}

func (bs *BuildStrategizer) scoreSpeed(duration time.Duration) float64 {
	if duration < time.Minute*5 {
		return 40
	} else if duration < time.Minute*10 {
		return 30
	} else if duration < time.Minute*20 {
		return 20
	}
	return 10
}

func (bs *BuildStrategizer) scoreSize(diskUsage int64) float64 {
	if diskUsage < 100*1024*1024 { // Less than 100MB
		return 40
	} else if diskUsage < 500*1024*1024 { // Less than 500MB
		return 30
	}
	return 10
}

func (bs *BuildStrategizer) scoreSecurity(overallRisk string) float64 {
	switch overallRisk {
	case "low":
		return 40
	case "medium":
		return 25
	case "high":
		return 10
	default:
		return 0
	}
}

func (bs *BuildStrategizer) scoreReliability(failurePointCount int) float64 {
	if failurePointCount == 0 {
		return 40
	} else if failurePointCount < 3 {
		return 30
	}
	return 15
}

func (bs *BuildStrategizer) scoreRiskTolerance(strategy *OptimizedBuildStrategy, acceptableRisk string) float64 {
	if strategy.RiskAssessment.OverallRisk == "high" {
		switch acceptableRisk {
		case "low":
			return -20
		case "medium":
			return -10
		}
	}
	return 0
}

func (bs *BuildStrategizer) scoreTimeConstraints(strategy *OptimizedBuildStrategy, timeConstraints time.Duration) float64 {
	if timeConstraints > 0 && strategy.ExpectedDuration > timeConstraints {
		return -30 // Heavy penalty for exceeding time constraints
	}
	return 0
}

func (bs *BuildStrategizer) scoreQualityLevel(strategy *OptimizedBuildStrategy, qualityLevel string) float64 {
	stepCount := len(strategy.Steps)
	switch qualityLevel {
	case "high":
		if stepCount > 8 {
			return 10
		} else if stepCount > 5 {
			return 5
		}
	case "fast":
		if stepCount < 4 {
			return 10
		} else if stepCount < 6 {
			return 5
		}
	}
	return 0
}

func (bs *BuildStrategizer) enhanceWithContext(strategy *OptimizedBuildStrategy, context map[string]interface{}) *OptimizedBuildStrategy {
	enhanced := &OptimizedBuildStrategy{
		Name:             strategy.Name,
		Description:      strategy.Description,
		Steps:            make([]*BuildStep, len(strategy.Steps)),
		ExpectedDuration: strategy.ExpectedDuration,
		ResourceUsage:    strategy.ResourceUsage,
		RiskAssessment:   strategy.RiskAssessment,
		Advantages:       strategy.Advantages,
		Disadvantages:    strategy.Disadvantages,
		Metadata:         make(map[string]interface{}),
	}

	// Copy steps
	copy(enhanced.Steps, strategy.Steps)

	// Copy metadata
	for k, v := range strategy.Metadata {
		enhanced.Metadata[k] = v
	}

	// Add context-specific enhancements
	if parallelism, exists := context["max_parallelism"]; exists {
		if parallel, ok := parallelism.(int); ok && parallel > 1 {
			enhanced = bs.addParallelizationOptimizations(enhanced, parallel)
		}
	}

	if cacheDir, exists := context["cache_directory"]; exists {
		if cache, ok := cacheDir.(string); ok && cache != "" {
			enhanced = bs.addCacheOptimizations(enhanced, cache)
		}
	}

	if registry, exists := context["container_registry"]; exists {
		if reg, ok := registry.(string); ok && reg != "" {
			enhanced = bs.addRegistryOptimizations(enhanced, reg)
		}
	}

	return enhanced
}

func (bs *BuildStrategizer) addParallelizationOptimizations(strategy *OptimizedBuildStrategy, maxParallel int) *OptimizedBuildStrategy {
	// Mark parallelizable steps
	for _, step := range strategy.Steps {
		if bs.isParallelizable(step) && maxParallel > 1 {
			step.Parallel = true
			step.Environment["PARALLEL_JOBS"] = fmt.Sprintf("%d", maxParallel)
		}
	}

	// Adjust expected duration for parallelization
	strategy.ExpectedDuration = time.Duration(float64(strategy.ExpectedDuration) * 0.7) // 30% improvement estimate

	strategy.Advantages = append(strategy.Advantages, "Parallelized execution for faster builds")
	strategy.Metadata["parallelization_enabled"] = true
	strategy.Metadata["max_parallel_jobs"] = maxParallel

	return strategy
}

func (bs *BuildStrategizer) addCacheOptimizations(strategy *OptimizedBuildStrategy, cacheDir string) *OptimizedBuildStrategy {
	// Add cache configuration to relevant steps
	for _, step := range strategy.Steps {
		if bs.benefitsFromCache(step) {
			if step.Environment == nil {
				step.Environment = make(map[string]string)
			}
			step.Environment["CACHE_DIR"] = cacheDir
			step.Environment["ENABLE_CACHE"] = "true"
		}
	}

	// Adjust expected duration for caching
	strategy.ExpectedDuration = time.Duration(float64(strategy.ExpectedDuration) * 0.8) // 20% improvement estimate

	strategy.Advantages = append(strategy.Advantages, "Build caching for faster subsequent builds")
	strategy.Metadata["cache_enabled"] = true
	strategy.Metadata["cache_directory"] = cacheDir

	return strategy
}

func (bs *BuildStrategizer) addRegistryOptimizations(strategy *OptimizedBuildStrategy, registry string) *OptimizedBuildStrategy {
	// Configure registry for container-related steps
	for _, step := range strategy.Steps {
		if bs.isContainerRelated(step) {
			if step.Environment == nil {
				step.Environment = make(map[string]string)
			}
			step.Environment["CONTAINER_REGISTRY"] = registry
			step.Environment["REGISTRY_OPTIMIZATION"] = "true"
		}
	}

	strategy.Advantages = append(strategy.Advantages, "Optimized container registry usage")
	strategy.Metadata["container_registry"] = registry

	return strategy
}

func (bs *BuildStrategizer) isParallelizable(step *BuildStep) bool {
	parallelizableCommands := []string{"make", "npm", "go build", "mvn", "gradle"}
	for _, cmd := range parallelizableCommands {
		if strings.Contains(step.Command, cmd) {
			return true
		}
	}
	return false
}

func (bs *BuildStrategizer) benefitsFromCache(step *BuildStep) bool {
	cacheableCommands := []string{"npm", "go build", "mvn", "gradle", "pip", "cargo"}
	for _, cmd := range cacheableCommands {
		if strings.Contains(step.Command, cmd) {
			return true
		}
	}
	return false
}

func (bs *BuildStrategizer) isContainerRelated(step *BuildStep) bool {
	containerCommands := []string{"docker", "podman", "buildah", "skopeo"}
	for _, cmd := range containerCommands {
		if strings.Contains(step.Command, cmd) {
			return true
		}
	}
	return false
}

// StrategyDatabase holds predefined build strategies
type StrategyDatabase struct {
	strategies map[string][]*OptimizedBuildStrategy
}

// NewStrategyDatabase creates a new strategy database with predefined strategies
func NewStrategyDatabase() *StrategyDatabase {
	db := &StrategyDatabase{
		strategies: make(map[string][]*OptimizedBuildStrategy),
	}
	db.loadPredefinedStrategies()
	return db
}

func (db *StrategyDatabase) loadPredefinedStrategies() {
	// Go strategies
	db.strategies["go"] = []*OptimizedBuildStrategy{
		{
			Name:        "Go Fast Build",
			Description: "Fast Go build with minimal checks",
			Steps: []*BuildStep{
				{Name: "Download Dependencies", Command: "go", Args: []string{"mod", "download"}, ExpectedTime: time.Minute * 2},
				{Name: "Build Binary", Command: "go", Args: []string{"build", "-o", "app", "."}, ExpectedTime: time.Minute * 1},
			},
			ExpectedDuration: time.Minute * 3,
			ResourceUsage:    &ResourceEstimate{CPU: 2.0, Memory: 512 * 1024 * 1024, Disk: 50 * 1024 * 1024},
			RiskAssessment:   &RiskAssessment{OverallRisk: "medium", RiskFactors: []string{"Minimal validation"}},
			Advantages:       []string{"Very fast", "Simple"},
			Disadvantages:    []string{"Limited error checking", "No optimization"},
		},
		{
			Name:        "Go Production Build",
			Description: "Production-ready Go build with full validation",
			Steps: []*BuildStep{
				{Name: "Download Dependencies", Command: "go", Args: []string{"mod", "download"}, ExpectedTime: time.Minute * 2},
				{Name: "Run Tests", Command: "go", Args: []string{"test", "./..."}, ExpectedTime: time.Minute * 5},
				{Name: "Run Linting", Command: "golangci-lint", Args: []string{"run"}, ExpectedTime: time.Minute * 2},
				{Name: "Build Binary", Command: "go", Args: []string{"build", "-ldflags", "-s -w", "-o", "app", "."}, ExpectedTime: time.Minute * 1},
				{Name: "Security Scan", Command: "gosec", Args: []string{"./..."}, ExpectedTime: time.Minute * 1},
			},
			ExpectedDuration: time.Minute * 11,
			ResourceUsage:    &ResourceEstimate{CPU: 4.0, Memory: 1024 * 1024 * 1024, Disk: 100 * 1024 * 1024},
			RiskAssessment:   &RiskAssessment{OverallRisk: "low", RiskFactors: []string{"Comprehensive validation"}},
			Advantages:       []string{"High quality", "Security validated", "Optimized binary"},
			Disadvantages:    []string{"Slower", "More resource intensive"},
		},
	}

	// Python strategies
	db.strategies["python"] = []*OptimizedBuildStrategy{
		{
			Name:        "Python Fast Build",
			Description: "Fast Python package build",
			Steps: []*BuildStep{
				{Name: "Install Dependencies", Command: "pip", Args: []string{"install", "-r", "requirements.txt"}, ExpectedTime: time.Minute * 3},
				{Name: "Package Application", Command: "python", Args: []string{"setup.py", "sdist"}, ExpectedTime: time.Minute * 1},
			},
			ExpectedDuration: time.Minute * 4,
			ResourceUsage:    &ResourceEstimate{CPU: 1.0, Memory: 256 * 1024 * 1024, Disk: 100 * 1024 * 1024},
			RiskAssessment:   &RiskAssessment{OverallRisk: "medium", RiskFactors: []string{"No testing", "Dependency conflicts possible"}},
			Advantages:       []string{"Quick build", "Simple process"},
			Disadvantages:    []string{"No validation", "Potential dependency issues"},
		},
	}

	// JavaScript/Node.js strategies
	db.strategies["javascript"] = []*OptimizedBuildStrategy{
		{
			Name:        "Node.js Fast Build",
			Description: "Fast Node.js application build",
			Steps: []*BuildStep{
				{Name: "Install Dependencies", Command: "npm", Args: []string{"ci"}, ExpectedTime: time.Minute * 2},
				{Name: "Build Application", Command: "npm", Args: []string{"run", "build"}, ExpectedTime: time.Minute * 3},
			},
			ExpectedDuration: time.Minute * 5,
			ResourceUsage:    &ResourceEstimate{CPU: 2.0, Memory: 512 * 1024 * 1024, Disk: 200 * 1024 * 1024},
			RiskAssessment:   &RiskAssessment{OverallRisk: "medium", RiskFactors: []string{"Node modules complexity"}},
			Advantages:       []string{"Fast execution", "Reproducible builds"},
			Disadvantages:    []string{"Large node_modules", "Potential security issues"},
		},
	}

	// Generic/unknown project type strategies
	db.strategies["generic"] = []*OptimizedBuildStrategy{
		{
			Name:        "Generic Build",
			Description: "Generic build strategy for unknown project types",
			Steps: []*BuildStep{
				{Name: "Detect Build System", Command: "detect-build-system", Args: []string{"."}, ExpectedTime: time.Minute * 1},
				{Name: "Run Build", Command: "make", Args: []string{}, ExpectedTime: time.Minute * 5},
			},
			ExpectedDuration: time.Minute * 6,
			ResourceUsage:    &ResourceEstimate{CPU: 2.0, Memory: 512 * 1024 * 1024, Disk: 100 * 1024 * 1024},
			RiskAssessment:   &RiskAssessment{OverallRisk: "high", RiskFactors: []string{"Unknown project structure", "Uncertain build process"}},
			Advantages:       []string{"Flexible", "Works with most projects"},
			Disadvantages:    []string{"Uncertain outcome", "Not optimized"},
		},
	}
}

func (db *StrategyDatabase) GetStrategiesForProjectType(projectType string) []*OptimizedBuildStrategy {
	strategies, exists := db.strategies[strings.ToLower(projectType)]
	if !exists {
		// Return generic strategies for unknown project types
		return db.strategies["generic"]
	}

	// Return a copy to avoid mutation
	result := make([]*OptimizedBuildStrategy, len(strategies))
	copy(result, strategies)
	return result
}

// StrategyOptimizer optimizes strategies based on goals
type StrategyOptimizer struct {
	logger zerolog.Logger
}

func NewStrategyOptimizer(logger zerolog.Logger) *StrategyOptimizer {
	return &StrategyOptimizer{
		logger: logger.With().Str("component", "strategy_optimizer").Logger(),
	}
}

func (so *StrategyOptimizer) OptimizeStrategies(strategies []*OptimizedBuildStrategy, goals *OptimizationGoals) []*OptimizedBuildStrategy {
	optimized := make([]*OptimizedBuildStrategy, len(strategies))

	for i, strategy := range strategies {
		optimized[i] = so.optimizeStrategy(strategy, goals)
	}

	return optimized
}

func (so *StrategyOptimizer) optimizeStrategy(strategy *OptimizedBuildStrategy, goals *OptimizationGoals) *OptimizedBuildStrategy {
	// Create a copy to avoid modifying the original
	optimized := &OptimizedBuildStrategy{
		Name:             strategy.Name + " (Optimized)",
		Description:      strategy.Description + " - Optimized for " + goals.PrimarGoal,
		Steps:            make([]*BuildStep, len(strategy.Steps)),
		ExpectedDuration: strategy.ExpectedDuration,
		ResourceUsage:    strategy.ResourceUsage,
		RiskAssessment:   strategy.RiskAssessment,
		Advantages:       append([]string{}, strategy.Advantages...),
		Disadvantages:    append([]string{}, strategy.Disadvantages...),
		Metadata:         make(map[string]interface{}),
	}

	// Copy steps
	for i, step := range strategy.Steps {
		optimized.Steps[i] = &BuildStep{
			Name:         step.Name,
			Command:      step.Command,
			Args:         append([]string{}, step.Args...),
			WorkingDir:   step.WorkingDir,
			Environment:  make(map[string]string),
			ExpectedTime: step.ExpectedTime,
			CriticalPath: step.CriticalPath,
			Parallel:     step.Parallel,
		}
		// Copy environment
		for k, v := range step.Environment {
			optimized.Steps[i].Environment[k] = v
		}
	}

	// Copy metadata
	for k, v := range strategy.Metadata {
		optimized.Metadata[k] = v
	}

	// Apply goal-specific optimizations
	switch goals.PrimarGoal {
	case "speed":
		optimized = so.optimizeForSpeed(optimized)
	case "size":
		optimized = so.optimizeForSize(optimized)
	case "security":
		optimized = so.optimizeForSecurity(optimized)
	case "reliability":
		optimized = so.optimizeForReliability(optimized)
	}

	return optimized
}

func (so *StrategyOptimizer) optimizeForSpeed(strategy *OptimizedBuildStrategy) *OptimizedBuildStrategy {
	// Remove non-essential steps for speed
	essentialSteps := []*BuildStep{}
	for _, step := range strategy.Steps {
		if so.isEssentialForBuild(step) {
			essentialSteps = append(essentialSteps, step)
		}
	}
	strategy.Steps = essentialSteps

	// Enable parallelization where possible
	for _, step := range strategy.Steps {
		if so.canParallelize(step) {
			step.Parallel = true
			if step.Environment == nil {
				step.Environment = make(map[string]string)
			}
			step.Environment["PARALLEL"] = "true"
		}
	}

	// Adjust expected duration
	strategy.ExpectedDuration = time.Duration(float64(strategy.ExpectedDuration) * 0.7)

	strategy.Advantages = append(strategy.Advantages, "Optimized for speed")
	strategy.Metadata["optimization"] = "speed"

	return strategy
}

func (so *StrategyOptimizer) optimizeForSize(strategy *OptimizedBuildStrategy) *OptimizedBuildStrategy {
	// Add size optimization flags
	for _, step := range strategy.Steps {
		if so.supportsSizeOptimization(step) {
			step.Args = append(step.Args, "-ldflags", "-s -w") // For Go builds
			if step.Environment == nil {
				step.Environment = make(map[string]string)
			}
			step.Environment["OPTIMIZE_SIZE"] = "true"
		}
	}

	strategy.Advantages = append(strategy.Advantages, "Optimized for size")
	strategy.Metadata["optimization"] = "size"

	return strategy
}

func (so *StrategyOptimizer) optimizeForSecurity(strategy *OptimizedBuildStrategy) *OptimizedBuildStrategy {
	// Add security scanning steps
	securityStep := &BuildStep{
		Name:         "Security Scan",
		Command:      "security-scanner",
		Args:         []string{"--scan", "."},
		ExpectedTime: time.Minute * 2,
		CriticalPath: true,
	}
	strategy.Steps = append(strategy.Steps, securityStep)

	// Update duration
	strategy.ExpectedDuration += securityStep.ExpectedTime

	strategy.Advantages = append(strategy.Advantages, "Enhanced security validation")
	strategy.RiskAssessment.OverallRisk = "low"
	strategy.Metadata["optimization"] = "security"

	return strategy
}

func (so *StrategyOptimizer) optimizeForReliability(strategy *OptimizedBuildStrategy) *OptimizedBuildStrategy {
	// Add validation steps
	validationStep := &BuildStep{
		Name:         "Validation",
		Command:      "validate-build",
		Args:         []string{"--comprehensive"},
		ExpectedTime: time.Minute * 3,
		CriticalPath: true,
	}
	strategy.Steps = append(strategy.Steps, validationStep)

	// Add retry logic to critical steps
	for _, step := range strategy.Steps {
		if step.CriticalPath {
			if step.Environment == nil {
				step.Environment = make(map[string]string)
			}
			step.Environment["RETRY_COUNT"] = "3"
		}
	}

	// Update duration
	strategy.ExpectedDuration += validationStep.ExpectedTime

	strategy.Advantages = append(strategy.Advantages, "Enhanced reliability and validation")
	strategy.RiskAssessment.OverallRisk = "low"
	strategy.Metadata["optimization"] = "reliability"

	return strategy
}

func (so *StrategyOptimizer) isEssentialForBuild(step *BuildStep) bool {
	essential := []string{"build", "compile", "package", "install"}
	stepName := strings.ToLower(step.Name)
	for _, keyword := range essential {
		if strings.Contains(stepName, keyword) {
			return true
		}
	}
	return false
}

func (so *StrategyOptimizer) canParallelize(step *BuildStep) bool {
	parallelizable := []string{"test", "lint", "scan", "analyze"}
	stepName := strings.ToLower(step.Name)
	for _, keyword := range parallelizable {
		if strings.Contains(stepName, keyword) {
			return true
		}
	}
	return false
}

func (so *StrategyOptimizer) supportsSizeOptimization(step *BuildStep) bool {
	return strings.Contains(step.Command, "go") && strings.Contains(step.Name, "Build")
}
