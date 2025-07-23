// Package workflow provides the unified orchestrator implementation
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ExecutionMode defines the orchestrator execution strategy
type ExecutionMode string

const (
	// SequentialMode executes steps one after another
	SequentialMode ExecutionMode = "sequential"
	// DAGMode executes steps based on dependency graph with parallelization
	DAGMode ExecutionMode = "dag"
	// AdaptiveMode uses AI to optimize execution based on patterns
	AdaptiveMode ExecutionMode = "adaptive"
)

// OrchestratorConfig represents the configuration for the unified orchestrator
type OrchestratorConfig struct {
	// ExecutionMode determines how steps are executed (sequential, dag, adaptive)
	ExecutionMode ExecutionMode `yaml:"execution_mode"`

	// ParallelConfig controls parallel execution settings
	ParallelConfig ParallelConfig `yaml:"parallel_execution"`

	// AdaptiveConfig controls adaptive features
	AdaptiveConfig AdaptiveConfig `yaml:"adaptive_features"`

	// EventsEnabled enables workflow event publishing
	EventsEnabled bool `yaml:"events_enabled"`

	// MaxConcurrency limits the number of concurrent step executions
	MaxConcurrency int `yaml:"max_concurrency"`

	// DefaultTimeout is the default timeout for step execution
	DefaultTimeout time.Duration `yaml:"default_timeout"`

	// MiddlewareConfig controls middleware behavior
	MiddlewareConfig MiddlewareConfig `yaml:"middleware"`
}

// ParallelConfig controls parallel execution behavior
type ParallelConfig struct {
	// Enabled determines if parallel execution is allowed
	Enabled bool `yaml:"enabled"`

	// MaxParallelSteps limits concurrent step execution
	MaxParallelSteps int `yaml:"max_parallel_steps"`

	// DependencyAware enables dependency-based parallel execution
	DependencyAware bool `yaml:"dependency_aware"`
}

// AdaptiveConfig controls adaptive orchestration features
type AdaptiveConfig struct {
	// Enabled determines if adaptive features are active
	Enabled bool `yaml:"enabled"`

	// PatternRecognition enables error pattern learning
	PatternRecognition bool `yaml:"pattern_recognition"`

	// StrategyLearning enables adaptation strategy learning
	StrategyLearning bool `yaml:"strategy_learning"`

	// MinConfidence is the minimum confidence for applying adaptations
	MinConfidence float64 `yaml:"min_confidence"`
}

// MiddlewareConfig controls middleware behavior and ordering
type MiddlewareConfig struct {
	// LoggingLevel controls logging verbosity (minimal, standard, detailed, debug)
	LoggingLevel string `yaml:"logging_level"`

	// ProgressMode controls progress reporting (simple, comprehensive, retry_aware)
	ProgressMode string `yaml:"progress_mode"`

	// RetryPolicy controls retry behavior
	RetryPolicy RetryPolicy `yaml:"retry_policy"`

	// TimeoutConfig controls timeout behavior
	TimeoutConfig TimeoutConfig `yaml:"timeout_config"`

	// TracingEnabled enables distributed tracing
	TracingEnabled bool `yaml:"tracing_enabled"`

	// EnhancementEnabled enables AI-powered step enhancement
	EnhancementEnabled bool `yaml:"enhancement_enabled"`
}

// Orchestrator is the configurable orchestrator that replaces
// all previous orchestrator implementations (BaseOrchestrator, DAGOrchestrator, AdaptiveOrchestrator)
type Orchestrator struct {
	config          OrchestratorConfig
	stepProvider    StepProvider
	emitterFactory  ProgressEmitterFactory
	executionEngine ExecutionEngine
	logger          *slog.Logger
	middlewares     []StepMiddleware

	// Optional dependencies - nil if not enabled
	eventPublisher EventPublisher
	errorContext   ErrorPatternProvider
	stepEnhancer   StepEnhancer
	tracer         Tracer

	// Context cancellation for cleanup
	cancel context.CancelFunc
}

// ExecutionEngine interface for different execution strategies
type ExecutionEngine interface {
	Execute(ctx context.Context, steps []Step, state *WorkflowState, middlewares []StepMiddleware) (*ContainerizeAndDeployResult, error)
}

// EventPublisher interface for workflow event publishing
type EventPublisher interface {
	PublishWorkflowEvent(ctx context.Context, workflowID string, eventType string, payload interface{}) error
}

// NewOrchestrator creates a new orchestrator with the specified configuration
func NewOrchestrator(
	config OrchestratorConfig,
	stepProvider StepProvider,
	emitterFactory ProgressEmitterFactory,
	logger *slog.Logger,
	dependencies *OrchestratorDependencies,
) (*Orchestrator, error) {

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid orchestrator configuration: %w", err)
	}

	// Create execution engine based on mode
	executionEngine, err := createExecutionEngine(config, dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution engine: %w", err)
	}

	// Create middleware stack
	middlewares := createMiddlewareStack(config, dependencies, logger)

	orchestrator := &Orchestrator{
		config:          config,
		stepProvider:    stepProvider,
		emitterFactory:  emitterFactory,
		executionEngine: executionEngine,
		logger:          logger,
		middlewares:     middlewares,
	}

	// Set optional dependencies based on configuration
	if config.EventsEnabled && dependencies.EventPublisher != nil {
		orchestrator.eventPublisher = dependencies.EventPublisher
	}

	if config.AdaptiveConfig.Enabled && dependencies.ErrorContext != nil {
		orchestrator.errorContext = dependencies.ErrorContext
	}

	if config.MiddlewareConfig.EnhancementEnabled && dependencies.StepEnhancer != nil {
		orchestrator.stepEnhancer = dependencies.StepEnhancer
	}

	if config.MiddlewareConfig.TracingEnabled && dependencies.Tracer != nil {
		orchestrator.tracer = dependencies.Tracer
	}

	return orchestrator, nil
}

// OrchestratorDependencies holds optional dependencies for the orchestrator
type OrchestratorDependencies struct {
	EventPublisher EventPublisher
	ErrorContext   ErrorPatternProvider
	StepEnhancer   StepEnhancer
	Tracer         Tracer
}

// Execute implements WorkflowOrchestrator interface
func (o *Orchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// Store workflow start time for duration calculation
	startTime := time.Now()

	// Initialize workflow context
	workflowCtx, err := o.initContext(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize workflow context: %w", err)
	}

	// Create workflow state
	state, err := o.newState(workflowCtx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow state: %w", err)
	}

	// Publish workflow start event if enabled
	if o.eventPublisher != nil {
		_ = o.eventPublisher.PublishWorkflowEvent(workflowCtx, state.WorkflowID, "workflow_started", map[string]interface{}{
			"execution_mode": string(o.config.ExecutionMode),
			"args":           args,
		})
	}

	// Log workflow start
	o.logger.Info("Starting workflow execution",
		slog.String("workflow_id", state.WorkflowID),
		slog.String("execution_mode", string(o.config.ExecutionMode)),
		slog.String("repository", getRepoIdentifier(args)),
	)

	// Get workflow steps
	steps := o.getWorkflowSteps()

	// Execute workflow using the configured execution engine
	result, err := o.executionEngine.Execute(workflowCtx, steps, state, o.middlewares)

	// Handle workflow completion
	if err != nil {
		o.logger.Error("Workflow execution failed",
			slog.String("workflow_id", state.WorkflowID),
			slog.String("error", err.Error()),
		)

		if o.eventPublisher != nil {
			_ = o.eventPublisher.PublishWorkflowEvent(workflowCtx, state.WorkflowID, "workflow_failed", map[string]interface{}{
				"error": err.Error(),
			})
		}

		return nil, err
	}

	o.logger.Info("Workflow execution completed successfully",
		slog.String("workflow_id", state.WorkflowID),
		slog.Duration("duration", time.Since(startTime)),
	)

	if o.eventPublisher != nil {
		_ = o.eventPublisher.PublishWorkflowEvent(workflowCtx, state.WorkflowID, "workflow_completed", map[string]interface{}{
			"result": result,
		})
	}

	return result, nil
}

// Cleanup releases resources associated with the orchestrator
func (o *Orchestrator) Cleanup() {
	if o.cancel != nil {
		o.cancel()
		o.cancel = nil
	}
}

// getWorkflowSteps returns the standard workflow steps
func (o *Orchestrator) getWorkflowSteps() []Step {
	return []Step{
		o.stepProvider.GetAnalyzeStep(),
		o.stepProvider.GetDockerfileStep(),
		o.stepProvider.GetBuildStep(),
		o.stepProvider.GetScanStep(),
		o.stepProvider.GetTagStep(),
		o.stepProvider.GetPushStep(),
		o.stepProvider.GetManifestStep(),
		o.stepProvider.GetClusterStep(),
		o.stepProvider.GetDeployStep(),
		o.stepProvider.GetVerifyStep(),
	}
}

// initContext initializes the workflow execution context
func (o *Orchestrator) initContext(ctx context.Context, args *ContainerizeAndDeployArgs) (context.Context, error) {
	// Apply default timeout if configured
	if o.config.DefaultTimeout > 0 {
		ctx, o.cancel = context.WithTimeout(ctx, o.config.DefaultTimeout)
	}

	// Add orchestrator configuration to context for middleware access
	ctx = WithOrchestratorConfig(ctx, o.config)

	return ctx, nil
}

// newState creates a new workflow state
func (o *Orchestrator) newState(ctx context.Context, args *ContainerizeAndDeployArgs) (*WorkflowState, error) {
	// Create progress emitter (using dummy request for now)
	emitter := o.emitterFactory.CreateEmitter(ctx, &mcp.CallToolRequest{}, 10)

	// Use the existing NewWorkflowState function
	state := NewWorkflowState(ctx, &mcp.CallToolRequest{}, args, emitter, o.logger)

	// Override the workflow ID with our generated one
	state.WorkflowID = generateWorkflowID()

	return state, nil
}

// Helper functions

// validateConfig validates the orchestrator configuration
func validateConfig(config OrchestratorConfig) error {
	// Validate execution mode
	switch config.ExecutionMode {
	case SequentialMode, DAGMode, AdaptiveMode:
		// Valid modes
	case "":
		return fmt.Errorf("execution mode must be specified")
	default:
		return fmt.Errorf("invalid execution mode: %s", config.ExecutionMode)
	}

	// Validate parallel configuration
	if config.ParallelConfig.Enabled && config.ParallelConfig.MaxParallelSteps <= 0 {
		return fmt.Errorf("max_parallel_steps must be positive when parallel execution is enabled")
	}

	// Validate adaptive configuration
	if config.AdaptiveConfig.Enabled {
		if config.AdaptiveConfig.MinConfidence < 0 || config.AdaptiveConfig.MinConfidence > 1 {
			return fmt.Errorf("adaptive min_confidence must be between 0 and 1")
		}
	}

	return nil
}

// createExecutionEngine creates the appropriate execution engine based on configuration
func createExecutionEngine(config OrchestratorConfig, dependencies *OrchestratorDependencies) (ExecutionEngine, error) {
	switch config.ExecutionMode {
	case SequentialMode:
		return NewSequentialEngine(), nil
	case DAGMode:
		return NewDAGEngine(config.ParallelConfig), nil
	case AdaptiveMode:
		if dependencies.ErrorContext == nil {
			return nil, fmt.Errorf("adaptive mode requires error context dependency")
		}
		return NewAdaptiveEngine(config.AdaptiveConfig, dependencies.ErrorContext), nil
	default:
		return nil, fmt.Errorf("unsupported execution mode: %s", config.ExecutionMode)
	}
}

// createMiddlewareStack creates the middleware stack based on configuration
func createMiddlewareStack(config OrchestratorConfig, dependencies *OrchestratorDependencies, logger *slog.Logger) []StepMiddleware {
	var middlewares []StepMiddleware

	// Add enhancement middleware first (if enabled) - should run before other middleware
	if config.MiddlewareConfig.EnhancementEnabled && dependencies.StepEnhancer != nil {
		// TODO: Create enhancement middleware - this will be implemented when enhancement middleware is consolidated
	}

	// Add retry middleware
	retryMiddleware := RetryMiddleware(config.MiddlewareConfig.RetryPolicy, dependencies.ErrorContext)
	middlewares = append(middlewares, retryMiddleware)

	// Add timeout middleware
	timeoutMiddleware := TimeoutMiddleware(config.MiddlewareConfig.TimeoutConfig)
	middlewares = append(middlewares, timeoutMiddleware)

	// Add progress middleware
	progressMode := parseProgressMode(config.MiddlewareConfig.ProgressMode)
	progressMiddleware := ProgressMiddleware(progressMode)
	middlewares = append(middlewares, progressMiddleware)

	// Add logging middleware
	loggingLevel := parseLoggingLevel(config.MiddlewareConfig.LoggingLevel)
	loggingMiddleware := LoggingMiddleware(LoggingConfig{
		Level:  loggingLevel,
		Logger: logger,
	})
	middlewares = append(middlewares, loggingMiddleware)

	// Add tracing middleware last (if enabled) - should capture all other middleware activity
	if config.MiddlewareConfig.TracingEnabled && dependencies.Tracer != nil {
		// Use existing tracing middleware
		tracingMiddleware := TracingMiddleware(dependencies.Tracer)
		middlewares = append(middlewares, tracingMiddleware)
	}

	return middlewares
}

// parseProgressMode converts string to ProgressMode enum
func parseProgressMode(mode string) ProgressMode {
	switch mode {
	case "simple":
		return SimpleProgress
	case "comprehensive":
		return ComprehensiveProgress
	case "retry_aware":
		return RetryAwareProgress
	default:
		return SimpleProgress // Default to simple
	}
}

// parseLoggingLevel converts string to LogLevel enum
func parseLoggingLevel(level string) LogLevel {
	switch level {
	case "minimal":
		return LogLevelMinimal
	case "standard":
		return LogLevelStandard
	case "detailed":
		return LogLevelDetailed
	case "debug":
		return LogLevelDebug
	default:
		return LogLevelStandard // Default to standard
	}
}

// generateWorkflowID generates a unique workflow ID
func generateWorkflowID() string {
	// Simple ID generation - in production this could use UUID or similar
	return fmt.Sprintf("workflow_%d", time.Now().UnixNano())
}

// getRepoIdentifier returns a string identifier for the repository
func getRepoIdentifier(args *ContainerizeAndDeployArgs) string {
	if args.RepoURL != "" {
		return args.RepoURL
	}
	if args.RepoPath != "" {
		return args.RepoPath
	}
	return "unknown"
}
