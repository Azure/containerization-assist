package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// FixChain represents a sequence of fix strategies to apply for complex issues
type FixChain struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Strategies  []ChainedFixStrategy `json:"strategies"`
	Conditions  []ChainCondition     `json:"conditions"`
	MaxRetries  int                  `json:"max_retries"`
	Timeout     time.Duration        `json:"timeout"`
}

// ChainedFixStrategy represents a single strategy in a fix chain
type ChainedFixStrategy struct {
	Name            string        `json:"name"`
	Strategy        FixStrategy   `json:"-"`
	Timeout         time.Duration `json:"timeout"`
	MaxRetries      int           `json:"max_retries"`
	ContinueOnError bool          `json:"continue_on_error"`
	Prerequisites   []string      `json:"prerequisites"`
	PostConditions  []string      `json:"post_conditions"`
	TransformArgs   ArgsTransform `json:"-"`
}

// ChainCondition defines when a fix chain should be applied
type ChainCondition struct {
	Type      ConditionType `json:"type"`
	Pattern   string        `json:"pattern"`
	ToolName  string        `json:"tool_name,omitempty"`
	ErrorCode string        `json:"error_code,omitempty"`
}

// ArgsTransform transforms arguments between chained strategies
type ArgsTransform func(previousResult interface{}, currentArgs interface{}) interface{}

// ConditionType defines the type of condition for chain activation
type ConditionType string

const (
	ConditionTypeErrorPattern  ConditionType = "error_pattern"
	ConditionTypeToolName      ConditionType = "tool_name"
	ConditionTypeErrorCode     ConditionType = "error_code"
	ConditionTypeMultipleFails ConditionType = "multiple_fails"
	ConditionTypeComplex       ConditionType = "complex"
)

// ChainResult represents the result of executing a fix chain
type ChainResult struct {
	ChainName           string                 `json:"chain_name"`
	Success             bool                   `json:"success"`
	ExecutedSteps       []ChainStepResult      `json:"executed_steps"`
	FinalResult         interface{}            `json:"final_result"`
	TotalDuration       time.Duration          `json:"total_duration"`
	FailureReason       string                 `json:"failure_reason,omitempty"`
	Suggestions         []string               `json:"suggestions,omitempty"`
	IntermediateResults map[string]interface{} `json:"intermediate_results"`
}

// ChainStepResult represents the result of a single step in a fix chain
type ChainStepResult struct {
	StepName      string        `json:"step_name"`
	Success       bool          `json:"success"`
	Duration      time.Duration `json:"duration"`
	Error         string        `json:"error,omitempty"`
	Result        interface{}   `json:"result,omitempty"`
	RetryCount    int           `json:"retry_count"`
	Skipped       bool          `json:"skipped"`
	SkippedReason string        `json:"skipped_reason,omitempty"`
}

// FixChainExecutor manages and executes fix strategy chains
type FixChainExecutor struct {
	logger *slog.Logger
	chains map[string]*FixChain
	helper *AutoFixHelper
}

// NewFixChainExecutor creates a new fix chain executor
func NewFixChainExecutor(logger *slog.Logger, helper *AutoFixHelper) *FixChainExecutor {
	executor := &FixChainExecutor{
		logger: logger,
		chains: make(map[string]*FixChain),
		helper: helper,
	}

	// Register common fix chains
	executor.registerCommonChains()

	return executor
}

// registerCommonChains registers commonly used fix chains
func (e *FixChainExecutor) registerCommonChains() {
	// Complex Docker build chain
	e.RegisterChain(&FixChain{
		Name:        "docker_build_complex",
		Description: "Complex Docker build error recovery chain",
		MaxRetries:  3,
		Timeout:     5 * time.Minute,
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "docker.*build.*failed"},
			{Type: ConditionTypeMultipleFails, Pattern: "dockerfile"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "dockerfile_syntax_fix",
				Timeout:         30 * time.Second,
				MaxRetries:      2,
				ContinueOnError: true,
				PostConditions:  []string{"dockerfile_valid"},
			},
			{
				Name:            "image_base_fix",
				Timeout:         45 * time.Second,
				MaxRetries:      3,
				ContinueOnError: true,
				Prerequisites:   []string{"dockerfile_valid"},
				PostConditions:  []string{"base_image_accessible"},
			},
			{
				Name:            "dependency_resolution",
				Timeout:         2 * time.Minute,
				MaxRetries:      2,
				ContinueOnError: false,
				Prerequisites:   []string{"base_image_accessible"},
			},
		},
	})

	// Network and connectivity chain
	e.RegisterChain(&FixChain{
		Name:        "network_connectivity_fix",
		Description: "Network and connectivity issue recovery chain",
		MaxRetries:  2,
		Timeout:     3 * time.Minute,
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "network|connection|timeout|registry"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "network_retry",
				Timeout:         30 * time.Second,
				MaxRetries:      3,
				ContinueOnError: true,
			},
			{
				Name:            "registry_auth_fix",
				Timeout:         45 * time.Second,
				MaxRetries:      2,
				ContinueOnError: true,
			},
			{
				Name:            "alternative_registry",
				Timeout:         60 * time.Second,
				MaxRetries:      2,
				ContinueOnError: false,
			},
		},
	})

	// Port and resource conflict chain
	e.RegisterChain(&FixChain{
		Name:        "resource_conflict_resolution",
		Description: "Port and resource conflict resolution chain",
		MaxRetries:  2,
		Timeout:     2 * time.Minute,
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "port.*use|address.*use|resource.*limit"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "port_alternative",
				Timeout:         15 * time.Second,
				MaxRetries:      5,
				ContinueOnError: true,
			},
			{
				Name:            "resource_optimization",
				Timeout:         30 * time.Second,
				MaxRetries:      2,
				ContinueOnError: true,
			},
			{
				Name:            "service_scaling_down",
				Timeout:         45 * time.Second,
				MaxRetries:      1,
				ContinueOnError: false,
			},
		},
	})

	// Manifest generation and deployment chain
	e.RegisterChain(&FixChain{
		Name:        "manifest_deployment_recovery",
		Description: "Kubernetes manifest generation and deployment recovery chain",
		MaxRetries:  3,
		Timeout:     4 * time.Minute,
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "manifest|deployment|pod.*failed|imagepullbackoff"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "manifest_simplification",
				Timeout:         30 * time.Second,
				MaxRetries:      2,
				ContinueOnError: true,
				PostConditions:  []string{"manifest_valid"},
			},
			{
				Name:            "image_verification",
				Timeout:         60 * time.Second,
				MaxRetries:      2,
				ContinueOnError: true,
				Prerequisites:   []string{"manifest_valid"},
				PostConditions:  []string{"image_available"},
			},
			{
				Name:            "deployment_strategy_fix",
				Timeout:         90 * time.Second,
				MaxRetries:      2,
				ContinueOnError: false,
				Prerequisites:   []string{"image_available"},
			},
		},
	})
}

// RegisterChain registers a new fix chain
func (e *FixChainExecutor) RegisterChain(chain *FixChain) {
	e.chains[chain.Name] = chain
	e.logger.Debug("Registered fix chain", slog.String("chain", chain.Name))
}

// ExecuteChain executes a fix chain for the given error
func (e *FixChainExecutor) ExecuteChain(ctx context.Context, tool api.Tool, args interface{}, err error) (*ChainResult, error) {
	// Find applicable chains
	applicableChains := e.findApplicableChains(tool, err)
	if len(applicableChains) == 0 {
		return nil, errors.NewError().
			Code(errors.RESOURCE_NOT_FOUND).
			Message("no applicable fix chains found").
			Build()
	}

	// Execute the most specific chain first
	chain := applicableChains[0]
	e.logger.Info("Executing fix chain",
		slog.String("chain", chain.Name),
		slog.String("tool", tool.Name()),
		slog.String("error", err.Error()))

	return e.executeChainSteps(ctx, chain, tool, args, err)
}

// findApplicableChains finds chains that match the current error conditions
func (e *FixChainExecutor) findApplicableChains(tool api.Tool, err error) []*FixChain {
	var applicable []*FixChain
	errorMsg := strings.ToLower(err.Error())
	toolName := tool.Name()

	for _, chain := range e.chains {
		if e.chainMatches(chain, toolName, errorMsg) {
			applicable = append(applicable, chain)
		}
	}

	// Sort by specificity (more conditions = more specific)
	// For now, just return in registration order
	return applicable
}

// chainMatches checks if a chain matches the current conditions
func (e *FixChainExecutor) chainMatches(chain *FixChain, toolName, errorMsg string) bool {
	for _, condition := range chain.Conditions {
		switch condition.Type {
		case ConditionTypeErrorPattern:
			if !strings.Contains(errorMsg, strings.ToLower(condition.Pattern)) {
				return false
			}
		case ConditionTypeToolName:
			if condition.ToolName != "" && condition.ToolName != toolName {
				return false
			}
		case ConditionTypeMultipleFails:
			// This would check session history for multiple failures
			// For now, assume it matches if error pattern is present
			if !strings.Contains(errorMsg, strings.ToLower(condition.Pattern)) {
				return false
			}
		}
	}
	return true
}

// executeChainSteps executes all steps in a fix chain
func (e *FixChainExecutor) executeChainSteps(ctx context.Context, chain *FixChain, tool api.Tool, args interface{}, err error) (*ChainResult, error) {
	startTime := time.Now()
	result := &ChainResult{
		ChainName:           chain.Name,
		ExecutedSteps:       make([]ChainStepResult, 0, len(chain.Strategies)),
		IntermediateResults: make(map[string]interface{}),
	}

	// Create chain context with timeout
	chainCtx := ctx
	if chain.Timeout > 0 {
		var cancel context.CancelFunc
		chainCtx, cancel = context.WithTimeout(ctx, chain.Timeout)
		defer cancel()
	}

	var currentArgs interface{} = args
	var lastResult interface{}
	var lastError error = err

	for i, strategy := range chain.Strategies {
		stepResult := e.executeChainStep(chainCtx, &strategy, tool, currentArgs, lastError, i)
		result.ExecutedSteps = append(result.ExecutedSteps, stepResult)

		if stepResult.Success {
			lastResult = stepResult.Result
			result.IntermediateResults[strategy.Name] = stepResult.Result

			// Transform args for next step if transformer is provided
			if strategy.TransformArgs != nil {
				currentArgs = strategy.TransformArgs(lastResult, currentArgs)
			}

			// Clear the error since this step succeeded
			lastError = nil
		} else {
			lastError = fmt.Errorf(stepResult.Error)

			if !strategy.ContinueOnError {
				result.Success = false
				result.FailureReason = fmt.Sprintf("Step '%s' failed: %s", strategy.Name, stepResult.Error)
				break
			}
		}
	}

	// Determine overall success
	if result.FailureReason == "" {
		// Check if we have a successful final result
		if lastResult != nil && lastError == nil {
			result.Success = true
			result.FinalResult = lastResult
		} else {
			result.Success = false
			result.FailureReason = "Chain completed but no successful result obtained"
		}
	}

	result.TotalDuration = time.Since(startTime)

	// Add suggestions based on the results
	result.Suggestions = e.generateChainSuggestions(result, chain)

	e.logger.Info("Fix chain execution completed",
		slog.String("chain", chain.Name),
		slog.Bool("success", result.Success),
		slog.Duration("duration", result.TotalDuration))

	return result, nil
}

// executeChainStep executes a single step in the fix chain
func (e *FixChainExecutor) executeChainStep(ctx context.Context, strategy *ChainedFixStrategy, tool api.Tool, args interface{}, err error, stepIndex int) ChainStepResult {
	startTime := time.Now()
	stepResult := ChainStepResult{
		StepName: strategy.Name,
	}

	// Check prerequisites
	if len(strategy.Prerequisites) > 0 {
		// For now, assume prerequisites are met
		// In a full implementation, this would check conditions
	}

	// Create step context with timeout
	stepCtx := ctx
	if strategy.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, strategy.Timeout)
		defer cancel()
	}

	// Execute the strategy with retries
	var result interface{}
	var execErr error

	for retry := 0; retry <= strategy.MaxRetries; retry++ {
		stepResult.RetryCount = retry

		// Get the actual fix strategy from the helper
		if fixStrategy, exists := e.helper.fixes[strategy.Name]; exists {
			result, execErr = fixStrategy(stepCtx, tool, args, err)
			if execErr == nil && result != nil {
				stepResult.Success = true
				stepResult.Result = result
				break
			}
		} else {
			execErr = fmt.Errorf("fix strategy '%s' not found", strategy.Name)
			break
		}

		// Log retry attempt
		if retry < strategy.MaxRetries {
			e.logger.Debug("Fix strategy retry",
				slog.String("strategy", strategy.Name),
				slog.Int("retry", retry+1),
				slog.Int("max_retries", strategy.MaxRetries))
		}
	}

	stepResult.Duration = time.Since(startTime)

	if !stepResult.Success {
		stepResult.Error = execErr.Error()
	}

	return stepResult
}

// generateChainSuggestions generates suggestions based on chain execution results
func (e *FixChainExecutor) generateChainSuggestions(result *ChainResult, chain *FixChain) []string {
	var suggestions []string

	if !result.Success {
		suggestions = append(suggestions, "Consider running the chain again with different parameters")

		// Analyze failed steps to provide specific suggestions
		for _, step := range result.ExecutedSteps {
			if !step.Success {
				switch step.StepName {
				case "dockerfile_syntax_fix":
					suggestions = append(suggestions, "Review Dockerfile syntax and ensure all instructions are valid")
				case "image_base_fix":
					suggestions = append(suggestions, "Verify base image exists and is accessible from your registry")
				case "network_retry":
					suggestions = append(suggestions, "Check network connectivity and firewall settings")
				case "port_alternative":
					suggestions = append(suggestions, "Manually specify an available port or stop conflicting services")
				case "manifest_simplification":
					suggestions = append(suggestions, "Review Kubernetes manifest for invalid configurations")
				}
			}
		}
	} else {
		suggestions = append(suggestions, "Fix chain completed successfully")
		if len(result.ExecutedSteps) > 1 {
			suggestions = append(suggestions, "Multiple strategies were needed - consider optimizing your setup")
		}
	}

	return suggestions
}

// GetAvailableChains returns information about all registered chains
func (e *FixChainExecutor) GetAvailableChains() map[string]string {
	chains := make(map[string]string)
	for name, chain := range e.chains {
		chains[name] = chain.Description
	}
	return chains
}

// HasApplicableChain checks if there's an applicable chain for the given error
func (e *FixChainExecutor) HasApplicableChain(tool api.Tool, err error) bool {
	applicable := e.findApplicableChains(tool, err)
	return len(applicable) > 0
}
