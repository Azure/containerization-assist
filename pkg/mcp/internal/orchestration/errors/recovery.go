package errors

import (
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	"github.com/rs/zerolog"
)

// RecoveryManager handles error recovery strategies
type RecoveryManager struct {
	logger             zerolog.Logger
	recoveryStrategies map[string]RecoveryStrategy
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(logger zerolog.Logger) *RecoveryManager {
	return &RecoveryManager{
		logger:             logger.With().Str("component", "recovery_manager").Logger(),
		recoveryStrategies: make(map[string]RecoveryStrategy),
	}
}

// AddRecoveryStrategy adds a custom recovery strategy
func (rm *RecoveryManager) AddRecoveryStrategy(strategy RecoveryStrategy) {
	rm.recoveryStrategies[strategy.ID] = strategy

	rm.logger.Info().
		Str("strategy_id", strategy.ID).
		Str("strategy_name", strategy.Name).
		Msg("Added custom recovery strategy")
}

// GetRecoveryOptions returns available recovery options for an error
func (rm *RecoveryManager) GetRecoveryOptions(workflowError *workflow.WorkflowError, classifier *ErrorClassifier) []RecoveryOption {
	var options []RecoveryOption

	// Find applicable recovery strategies
	for _, strategy := range rm.recoveryStrategies {
		for _, errorType := range strategy.ApplicableErrors {
			if classifier.matchesErrorType(workflowError.ErrorType, errorType) {
				option := RecoveryOption{
					Name:        strategy.Name,
					Description: strategy.Description,
					Action:      "recover",
					Parameters: map[string]interface{}{
						"strategy_id":   strategy.ID,
						"auto_recovery": strategy.AutoRecovery,
					},
					Probability: strategy.SuccessProbability,
					Cost:        rm.calculateRecoveryCost(strategy),
				}
				options = append(options, option)
			}
		}
	}

	// Add standard recovery options
	if workflowError.Retryable {
		options = append(options, RecoveryOption{
			Name:        "Retry Stage",
			Description: "Retry the failed stage with the same parameters",
			Action:      "retry",
			Parameters:  map[string]interface{}{"max_attempts": 3},
			Probability: 0.6,
			Cost:        "low",
		})
	}

	// Add skip option for non-critical errors
	if workflowError.Severity != "critical" {
		options = append(options, RecoveryOption{
			Name:        "Skip Stage",
			Description: "Skip this stage and continue with the workflow",
			Action:      "skip",
			Parameters:  map[string]interface{}{"mark_as_skipped": true},
			Probability: 1.0,
			Cost:        "low",
		})
	}

	// Add manual intervention option
	options = append(options, RecoveryOption{
		Name:        "Manual Intervention",
		Description: "Pause workflow for manual review and intervention",
		Action:      "pause",
		Parameters:  map[string]interface{}{"require_approval": true},
		Probability: 0.9,
		Cost:        "high",
	})

	return options
}

// GetRecoveryStrategy returns a specific recovery strategy by ID
func (rm *RecoveryManager) GetRecoveryStrategy(strategyID string) (RecoveryStrategy, bool) {
	strategy, exists := rm.recoveryStrategies[strategyID]
	return strategy, exists
}

// InitializeDefaultStrategies sets up default recovery strategies
func (rm *RecoveryManager) InitializeDefaultStrategies() {
	// Network recovery strategy
	rm.recoveryStrategies["network_recovery"] = RecoveryStrategy{
		ID:          "network_recovery",
		Name:        "Network Issue Recovery",
		Description: "Recover from network-related issues",
		ApplicableErrors: []string{
			"network_error",
			"connection_timeout",
			"dns_resolution_error",
		},
		AutoRecovery:       true,
		SuccessProbability: 0.8,
		EstimatedDuration:  30 * time.Second,
		Requirements:       []string{"network_connectivity"},
		RecoverySteps: []RecoveryStep{
			{
				ID:     "wait_network",
				Name:   "Wait for Network",
				Action: "wait",
				Parameters: &RecoveryStepParameters{
					CustomParams: map[string]string{"duration": "10s"},
				},
				Timeout: 15 * time.Second,
			},
			{
				ID:     "test_connectivity",
				Name:   "Test Connectivity",
				Action: "test_connection",
				Parameters: &RecoveryStepParameters{
					CustomParams: map[string]string{"target": "default_endpoint"},
				},
				Timeout:     30 * time.Second,
				RetryOnFail: true,
			},
		},
	}

	// Resource recovery strategy
	rm.recoveryStrategies["resource_recovery"] = RecoveryStrategy{
		ID:          "resource_recovery",
		Name:        "Resource Availability Recovery",
		Description: "Recover from resource unavailability issues",
		ApplicableErrors: []string{
			"resource_unavailable",
			"insufficient_resources",
			"resource_locked",
		},
		AutoRecovery:       true,
		SuccessProbability: 0.7,
		EstimatedDuration:  60 * time.Second,
		Requirements:       []string{"resource_manager"},
		RecoverySteps: []RecoveryStep{
			{
				ID:     "cleanup_resources",
				Name:   "Cleanup Unused Resources",
				Action: "cleanup",
				Parameters: &RecoveryStepParameters{
					CustomParams: map[string]string{"scope": "session"},
				},
				Timeout: 30 * time.Second,
			},
			{
				ID:     "wait_resources",
				Name:   "Wait for Resources",
				Action: "wait",
				Parameters: &RecoveryStepParameters{
					CustomParams: map[string]string{"duration": "30s"},
				},
				Timeout: 45 * time.Second,
			},
		},
	}
}

func (rm *RecoveryManager) calculateRecoveryCost(strategy RecoveryStrategy) string {
	if strategy.EstimatedDuration < 30*time.Second {
		return "low"
	} else if strategy.EstimatedDuration < 5*time.Minute {
		return "medium"
	} else {
		return "high"
	}
}
