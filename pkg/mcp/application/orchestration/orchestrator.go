package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Orchestrator is the single implementation for tool orchestration.
type Orchestrator struct {
	tools        sync.Map
	logger       *slog.Logger
	config       *OrchestratorConfig
	mu           sync.RWMutex
	isRunning    bool
	startTime    time.Time
	requestCount int64
	errorCount   int64
}

// OrchestratorConfig holds orchestrator configuration.
type OrchestratorConfig struct {
	Timeout            time.Duration
	MaxConcurrentTools int
	EnableMetrics      bool
	EnableTracing      bool
	Logger             *slog.Logger
	Component          string
}

// OrchestratorOption configures the Orchestrator using functional options pattern.
type OrchestratorOption func(*OrchestratorConfig)

// NewOrchestrator creates a new orchestrator with the specified options.
func NewOrchestrator(opts ...OrchestratorOption) api.Orchestrator {
	config := &OrchestratorConfig{
		Timeout:            10 * time.Minute,
		MaxConcurrentTools: 10,
		EnableMetrics:      true,
		EnableTracing:      false,
		Component:          "orchestrator",
	}

	for _, opt := range opts {
		opt(config)
	}

	// Telemetry initialization removed

	return &Orchestrator{
		logger:    config.Logger.With("component", config.Component),
		config:    config,
		isRunning: true,
		startTime: time.Now(),
	}
}

// WithTimeout sets the default execution timeout.
func WithTimeout(timeout time.Duration) OrchestratorOption {
	return func(c *OrchestratorConfig) { c.Timeout = timeout }
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) OrchestratorOption {
	return func(c *OrchestratorConfig) { c.Logger = logger }
}

// WithMetrics enables or disables metrics collection.
func WithMetrics(enabled bool) OrchestratorOption {
	return func(c *OrchestratorConfig) { c.EnableMetrics = enabled }
}

// WithTracing enables or disables distributed tracing.
func WithTracing(enabled bool) OrchestratorOption {
	return func(c *OrchestratorConfig) { c.EnableTracing = enabled }
}

// WithComponent sets the component name for logging.
func WithComponent(component string) OrchestratorOption {
	return func(c *OrchestratorConfig) { c.Component = component }
}

// Execute runs a tool with the given input.
func (o *Orchestrator) Execute(ctx context.Context, toolName string, input api.ToolInput) (api.ToolOutput, error) {
	o.mu.RLock()
	if !o.isRunning {
		o.mu.RUnlock()
		return api.ToolOutput{}, errors.New("orchestrator", "orchestrator is not running", errors.CategoryResource)
	}
	o.requestCount++
	o.mu.RUnlock()

	// Get tool
	raw, ok := o.tools.Load(toolName)
	if !ok {
		o.incrementErrorCount()
		return api.ToolOutput{}, errors.New("orchestrator",
			fmt.Sprintf("tool %q not found", toolName),
			errors.CategoryResource)
	}

	tool := raw.(api.Tool)

	// Apply timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.config.Timeout)
		defer cancel()
	}

	// Direct execution
	start := time.Now()
	output, err := tool.Execute(ctx, input)
	duration := time.Since(start)

	if err != nil {
		o.incrementErrorCount()
		o.logger.Error("Tool execution failed",
			"error", err,
			"tool", toolName,
			"duration", duration)
		return api.ToolOutput{}, err
	}

	o.logger.Info("Tool execution completed",
		"tool", toolName,
		"duration", duration)

	return output, nil
}

// ExecuteWorkflow executes a workflow with multiple steps.
func (o *Orchestrator) ExecuteWorkflow(ctx context.Context, workflow api.Workflow) (api.WorkflowResult, error) {
	o.logger.Info("Starting workflow execution", "workflow", workflow.Name)

	startTime := time.Now()
	var stepResults []api.StepResult
	success := true
	var workflowError error

	// Execute workflow steps
	for i, step := range workflow.Steps {
		stepStart := time.Now()

		// Execute step
		// Create ToolInput from step input
		toolInput := api.ToolInput{
			SessionID: fmt.Sprintf("%s-step-%d", workflow.ID, i),
			Data:      step.Input,
		}
		stepOutput, err := o.Execute(ctx, step.Tool, toolInput)
		stepDuration := time.Since(stepStart)

		stepResult := api.StepResult{
			StepID:    fmt.Sprintf("%s-%d", workflow.Name, i),
			StepName:  step.Name,
			Success:   err == nil,
			Output:    stepOutput.Data,
			StartTime: stepStart,
			EndTime:   time.Now(),
			Duration:  stepDuration,
			Retries:   0,
		}

		if err != nil {
			success = false
			workflowError = errors.NewError().Messagef("step %s failed: %v", step.Name, err).WithLocation().Build()

			o.logger.Error("Workflow step failed",
				"error", err,
				"workflow", workflow.Name,
				"step", step.Name)

			// Add error to step result
			stepResult.Error = err.Error()

			// Stop on first failure
			stepResults = append(stepResults, stepResult)
			break
		} else {
			o.logger.Info("Workflow step completed",
				"workflow", workflow.Name,
				"step", step.Name,
				"duration", stepDuration)
		}

		stepResults = append(stepResults, stepResult)
	}

	totalDuration := time.Since(startTime)

	// Count successful and failed steps
	successSteps := 0
	failedSteps := 0
	for _, step := range stepResults {
		if step.Success {
			successSteps++
		} else {
			failedSteps++
		}
	}

	result := api.WorkflowResult{
		WorkflowID:   workflow.ID,
		Success:      success,
		StepResults:  stepResults,
		Error:        "",
		StartTime:    startTime,
		EndTime:      time.Now(),
		Duration:     totalDuration,
		TotalSteps:   len(workflow.Steps),
		SuccessSteps: successSteps,
		FailedSteps:  failedSteps,
	}

	if workflowError != nil {
		result.Error = workflowError.Error()
	}

	o.logger.Info("Workflow execution completed",
		"workflow", workflow.Name,
		"success", success,
		"duration", totalDuration)

	return result, nil
}

// Register adds a tool to the orchestrator.
func (o *Orchestrator) Register(tool api.Tool, _ ...api.RegistryOption) error {
	if _, loaded := o.tools.LoadOrStore(tool.Name(), tool); loaded {
		return errors.New("orchestrator",
			fmt.Sprintf("tool %q already registered", tool.Name()),
			errors.CategoryValidation)
	}

	o.logger.Info("Tool registered", "tool", tool.Name())
	return nil
}

// List returns all registered tool names.
func (o *Orchestrator) List() []string {
	var names []string
	o.tools.Range(func(key, _ interface{}) bool {
		names = append(names, key.(string))
		return true
	})
	return names
}

// GetTool retrieves a tool by name.
func (o *Orchestrator) GetTool(toolName string) (api.Tool, bool) {
	if raw, ok := o.tools.Load(toolName); ok {
		return raw.(api.Tool), true
	}
	return nil, false
}

// GetRegistry returns the registry interface.
func (o *Orchestrator) GetRegistry() api.Registry {
	// The orchestrator itself implements the registry interface
	return o
}

// Close releases all resources.
func (o *Orchestrator) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.isRunning = false

	// Telemetry shutdown removed

	o.logger.Info("Orchestrator closed")
	return nil
}

// Registry interface implementation

// Unregister removes a tool from the registry.
func (o *Orchestrator) Unregister(name string) error {
	if _, existed := o.tools.LoadAndDelete(name); !existed {
		return errors.New("orchestrator",
			fmt.Sprintf("tool %q not found", name),
			errors.CategoryResource)
	}
	o.logger.Info("Tool unregistered", "tool", name)
	return nil
}

// Get retrieves a tool from the registry.
func (o *Orchestrator) Get(name string) (api.Tool, error) {
	if tool, ok := o.GetTool(name); ok {
		return tool, nil
	}
	return nil, errors.New("orchestrator",
		fmt.Sprintf("tool %q not found", name),
		errors.CategoryResource)
}

// ListByCategory returns tools filtered by category.
func (o *Orchestrator) ListByCategory(_ api.ToolCategory) []string {
	var names []string
	o.tools.Range(func(key, _ interface{}) bool {
		// TODO: Filter by category when ToolSchema includes category field
		// For now, return all tools
		names = append(names, key.(string))
		return true
	})
	return names
}

// ListByTags returns tools filtered by tags.
func (o *Orchestrator) ListByTags(_ ...string) []string {
	// Simplified implementation - returns all tools
	// Could be enhanced with tag support in ToolSchema
	return o.List()
}

// ExecuteWithRetry executes a tool with retry logic.
func (o *Orchestrator) ExecuteWithRetry(ctx context.Context, name string, input api.ToolInput, _ api.RetryPolicy) (api.ToolOutput, error) {
	// Simple implementation - could be enhanced with actual retry logic
	return o.Execute(ctx, name, input)
}

// ExecuteTool implements the api.Orchestrator interface
func (o *Orchestrator) ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error) {
	// Convert args to ToolInput if needed
	input, ok := args.(api.ToolInput)
	if !ok {
		// Try to convert map[string]interface{} to ToolInput
		if m, ok := args.(map[string]interface{}); ok {
			input = api.ToolInput{
				SessionID: "default",
				Data:      m,
			}
		} else {
			return nil, errors.New("orchestrator", "invalid input type", errors.CategoryResource)
		}
	}

	output, err := o.Execute(ctx, toolName, input)
	if err != nil {
		return nil, err
	}

	// Convert ToolOutput to interface{}
	return output, nil
}

// RegisterTool implements the api.Orchestrator interface
func (o *Orchestrator) RegisterTool(name string, tool api.Tool) error {
	return o.Register(tool)
}

// ListTools implements the api.Orchestrator interface
func (o *Orchestrator) ListTools() []string {
	return o.List()
}

// GetStats implements the api.Orchestrator interface
func (o *Orchestrator) GetStats() interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	uptime := time.Since(o.startTime)

	tools := o.List()
	return map[string]interface{}{
		"uptime":      uptime.String(),
		"is_running":  o.isRunning,
		"tools_count": len(tools),
		"tools":       tools,
		"start_time":  o.startTime,
	}
}

// GetMetadata returns metadata for a tool.
func (o *Orchestrator) GetMetadata(name string) (api.ToolMetadata, error) {
	tool, err := o.Get(name)
	if err != nil {
		return api.ToolMetadata{}, err
	}

	schema := tool.Schema()
	return api.ToolMetadata{
		Name:        name,
		Description: schema.Description,
		Version:     "1.0.0",             // Default version
		Category:    api.CategoryAnalyze, // Default category
		Status:      api.StatusActive,
	}, nil
}

// GetStatus returns the status of a tool.
func (o *Orchestrator) GetStatus(name string) (api.ToolStatus, error) {
	if _, err := o.Get(name); err != nil {
		return api.StatusInactive, err
	}
	return api.StatusActive, nil
}

// ValidateToolArgs validates tool arguments
func (o *Orchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	tool, err := o.Get(toolName)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeNotFound).
			Type(errors.ErrTypeValidation).
			Message("tool not found for validation").
			Context("tool_name", toolName).
			Build()
	}

	// If the tool implements a Validate method, use it
	if validator, ok := tool.(interface {
		Validate(ctx context.Context, args interface{}) error
	}); ok {
		return validator.Validate(context.Background(), args)
	}

	// Basic validation - ensure args is not nil
	if args == nil {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("tool arguments cannot be nil").
			Context("tool_name", toolName).
			Build()
	}

	return nil
}

// GetToolMetadata retrieves metadata for a specific tool
func (o *Orchestrator) GetToolMetadata(toolName string) (*api.ToolMetadata, error) {
	metadata, err := o.GetMetadata(toolName)
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

// RegisterGenericTool registers a tool with generic interface
func (o *Orchestrator) RegisterGenericTool(name string, tool interface{}) error {
	// Try to cast to api.Tool interface
	apiTool, ok := tool.(api.Tool)
	if !ok {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("tool does not implement api.Tool interface").
			Context("tool_name", name).
			Context("tool_type", fmt.Sprintf("%T", tool)).
			Build()
	}

	return o.RegisterTool(name, apiTool)
}

// GetTypedToolMetadata retrieves typed metadata for a specific tool
func (o *Orchestrator) GetTypedToolMetadata(toolName string) (*api.ToolMetadata, error) {
	// For now, this is the same as GetToolMetadata
	// In future versions, this could provide enhanced type information
	return o.GetToolMetadata(toolName)
}

// SetStatus sets the status of a tool.
func (o *Orchestrator) SetStatus(_ string, _ api.ToolStatus) error {
	// Status management not implemented in this simple version
	// Could be enhanced with tool state tracking
	return nil
}

// Helper methods

func (o *Orchestrator) incrementErrorCount() {
	o.mu.Lock()
	o.errorCount++
	o.mu.Unlock()
}

// Health returns health status information.
func (o *Orchestrator) Health() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	status := "healthy"
	uptime := time.Duration(0)
	errorRate := float64(0)

	if o.isRunning {
		uptime = time.Since(o.startTime)
		if o.requestCount > 0 {
			errorRate = float64(o.errorCount) / float64(o.requestCount)
		}

		// Mark as degraded if error rate is high
		if errorRate > 0.1 { // 10% error rate threshold
			status = "degraded"
		}
	} else {
		status = "unhealthy"
	}

	return map[string]interface{}{
		"status":        status,
		"uptime":        uptime.String(),
		"request_count": o.requestCount,
		"error_count":   o.errorCount,
		"error_rate":    errorRate,
		"tool_count":    len(o.List()),
	}
}
