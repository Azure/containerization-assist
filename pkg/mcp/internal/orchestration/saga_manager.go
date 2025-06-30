package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SagaManager manages distributed transaction workflows using the Saga pattern
type SagaManager struct {
	logger              zerolog.Logger
	sagas               map[string]*Saga
	sagaDefinitions     map[string]*SagaDefinition
	compensationManager *CompensationManager
	sagaOrchestrator    *SagaOrchestrator
	persistenceStore    SagaPersistenceStore
	eventBus            SagaEventBus
	timeoutManager      *SagaTimeoutManager
	mutex               sync.RWMutex
}

// Saga represents a distributed transaction using the Saga pattern
type Saga struct {
	ID               string                 `json:"id"`
	DefinitionID     string                 `json:"definition_id"`
	Status           SagaStatus             `json:"status"`
	CurrentStep      int                    `json:"current_step"`
	Steps            []*SagaStep            `json:"steps"`
	CompensationMode bool                   `json:"compensation_mode"`
	StartTime        time.Time              `json:"start_time"`
	EndTime          *time.Time             `json:"end_time,omitempty"`
	Context          map[string]interface{} `json:"context"`
	Variables        map[string]interface{} `json:"variables"`
	Metadata         map[string]interface{} `json:"metadata"`
	LastUpdated      time.Time              `json:"last_updated"`
	CompletedSteps   []int                  `json:"completed_steps"`
	FailedSteps      []int                  `json:"failed_steps"`
	CompensatedSteps []int                  `json:"compensated_steps"`
	ErrorDetails     map[int]string         `json:"error_details"`
	RetryCount       int                    `json:"retry_count"`
	MaxRetries       int                    `json:"max_retries"`
	TimeoutDuration  time.Duration          `json:"timeout_duration"`
}

// SagaStatus represents the status of a saga
type SagaStatus string

const (
	SagaStatusPending      SagaStatus = "pending"
	SagaStatusRunning      SagaStatus = "running"
	SagaStatusCompleted    SagaStatus = "completed"
	SagaStatusFailed       SagaStatus = "failed"
	SagaStatusCompensating SagaStatus = "compensating"
	SagaStatusCompensated  SagaStatus = "compensated"
	SagaStatusAborted      SagaStatus = "aborted"
	SagaStatusTimeout      SagaStatus = "timeout"
)

// SagaDefinition defines a saga workflow
type SagaDefinition struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Version          string                 `json:"version"`
	Steps            []*SagaStepDefinition  `json:"steps"`
	CompensationMode string                 `json:"compensation_mode"` // "forward", "backward", "parallel"
	TimeoutDuration  time.Duration          `json:"timeout_duration"`
	RetryPolicy      *SagaRetryPolicy       `json:"retry_policy"`
	Variables        map[string]interface{} `json:"variables"`
	Metadata         map[string]interface{} `json:"metadata"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// SagaStepDefinition defines a step in a saga
type SagaStepDefinition struct {
	ID                     string                      `json:"id"`
	Name                   string                      `json:"name"`
	Type                   string                      `json:"type"` // "action", "decision", "parallel", "sub_saga"
	ActionDefinition       *SagaActionDefinition       `json:"action_definition,omitempty"`
	CompensationDefinition *SagaCompensationDefinition `json:"compensation_definition,omitempty"`
	Conditions             []SagaCondition             `json:"conditions"`
	Timeout                time.Duration               `json:"timeout"`
	RetryPolicy            *SagaStepRetryPolicy        `json:"retry_policy"`
	CriticalityLevel       string                      `json:"criticality_level"` // "low", "medium", "high", "critical"
	Dependencies           []string                    `json:"dependencies"`
	Metadata               map[string]interface{}      `json:"metadata"`
}

// SagaActionDefinition defines an action to be executed
type SagaActionDefinition struct {
	ToolName      string                 `json:"tool_name"`
	Operation     string                 `json:"operation"`
	Parameters    map[string]interface{} `json:"parameters"`
	InputMapping  map[string]string      `json:"input_mapping"`
	OutputMapping map[string]string      `json:"output_mapping"`
	Headers       map[string]string      `json:"headers"`
	Timeout       time.Duration          `json:"timeout"`
}

// SagaCompensationDefinition defines compensation action
type SagaCompensationDefinition struct {
	ToolName     string                 `json:"tool_name"`
	Operation    string                 `json:"operation"`
	Parameters   map[string]interface{} `json:"parameters"`
	InputMapping map[string]string      `json:"input_mapping"`
	Condition    string                 `json:"condition"` // When to apply compensation
	Timeout      time.Duration          `json:"timeout"`
	Optional     bool                   `json:"optional"` // Can compensation fail without failing the saga
}

// SagaCondition defines conditions for step execution
type SagaCondition struct {
	Expression string   `json:"expression"`
	Variables  []string `json:"variables"`
	Operator   string   `json:"operator"` // "and", "or", "not"
}

// SagaRetryPolicy defines retry behavior for the entire saga
type SagaRetryPolicy struct {
	MaxAttempts       int           `json:"max_attempts"`
	InitialDelay      time.Duration `json:"initial_delay"`
	MaxDelay          time.Duration `json:"max_delay"`
	BackoffType       string        `json:"backoff_type"` // "exponential", "linear", "constant"
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	RetryableErrors   []string      `json:"retryable_errors"`
}

// SagaStepRetryPolicy defines retry behavior for individual steps
type SagaStepRetryPolicy struct {
	MaxAttempts     int           `json:"max_attempts"`
	Delay           time.Duration `json:"delay"`
	BackoffType     string        `json:"backoff_type"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// SagaStep represents an executed step in a saga
type SagaStep struct {
	ID                 string                 `json:"id"`
	DefinitionID       string                 `json:"definition_id"`
	Status             SagaStepStatus         `json:"status"`
	StartTime          time.Time              `json:"start_time"`
	EndTime            *time.Time             `json:"end_time,omitempty"`
	Duration           time.Duration          `json:"duration"`
	Result             interface{}            `json:"result"`
	Error              string                 `json:"error,omitempty"`
	RetryCount         int                    `json:"retry_count"`
	CompensationStatus SagaCompensationStatus `json:"compensation_status"`
	CompensationResult interface{}            `json:"compensation_result"`
	CompensationError  string                 `json:"compensation_error,omitempty"`
	Context            map[string]interface{} `json:"context"`
}

// SagaStepStatus represents the status of a saga step
type SagaStepStatus string

const (
	SagaStepStatusPending   SagaStepStatus = "pending"
	SagaStepStatusRunning   SagaStepStatus = "running"
	SagaStepStatusCompleted SagaStepStatus = "completed"
	SagaStepStatusFailed    SagaStepStatus = "failed"
	SagaStepStatusSkipped   SagaStepStatus = "skipped"
	SagaStepStatusTimeout   SagaStepStatus = "timeout"
)

// SagaCompensationStatus represents the status of compensation
type SagaCompensationStatus string

const (
	SagaCompensationStatusNone      SagaCompensationStatus = "none"
	SagaCompensationStatusPending   SagaCompensationStatus = "pending"
	SagaCompensationStatusRunning   SagaCompensationStatus = "running"
	SagaCompensationStatusCompleted SagaCompensationStatus = "completed"
	SagaCompensationStatusFailed    SagaCompensationStatus = "failed"
	SagaCompensationStatusSkipped   SagaCompensationStatus = "skipped"
)

// CompensationManager manages compensation actions
type CompensationManager struct {
	compensationStrategies map[string]CompensationStrategy
	compensationQueue      []CompensationAction
	mutex                  sync.RWMutex
	logger                 zerolog.Logger
}

// CompensationStrategy defines how to compensate for a failed saga
type CompensationStrategy interface {
	Compensate(ctx context.Context, saga *Saga, failedStepIndex int) error
	CanCompensate(saga *Saga, failedStepIndex int) bool
	GetCompensationOrder(saga *Saga, failedStepIndex int) []int
}

// CompensationAction represents a compensation action to be executed
type CompensationAction struct {
	ID          string                      `json:"id"`
	SagaID      string                      `json:"saga_id"`
	StepID      string                      `json:"step_id"`
	Definition  *SagaCompensationDefinition `json:"definition"`
	Context     map[string]interface{}      `json:"context"`
	Priority    int                         `json:"priority"`
	ScheduledAt time.Time                   `json:"scheduled_at"`
	Status      string                      `json:"status"`
}

// SagaOrchestrator orchestrates saga execution
type SagaOrchestrator struct {
	stepExecutor       SagaStepExecutor
	conditionEvaluator SagaConditionEvaluator
	variableResolver   SagaVariableResolver
	logger             zerolog.Logger
}

// SagaStepExecutor executes saga steps
type SagaStepExecutor interface {
	ExecuteStep(ctx context.Context, saga *Saga, step *SagaStepDefinition) (*SagaStepResult, error)
	ExecuteCompensation(ctx context.Context, saga *Saga, step *SagaStep) (*SagaCompensationResult, error)
}

// SagaStepResult represents the result of executing a saga step
type SagaStepResult struct {
	Success   bool                   `json:"success"`
	Result    interface{}            `json:"result"`
	Error     error                  `json:"error"`
	Duration  time.Duration          `json:"duration"`
	Variables map[string]interface{} `json:"variables"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// SagaCompensationResult represents the result of compensation execution
type SagaCompensationResult struct {
	Success  bool                   `json:"success"`
	Result   interface{}            `json:"result"`
	Error    error                  `json:"error"`
	Duration time.Duration          `json:"duration"`
	Metadata map[string]interface{} `json:"metadata"`
}

// SagaConditionEvaluator evaluates saga conditions
type SagaConditionEvaluator interface {
	EvaluateConditions(conditions []SagaCondition, variables map[string]interface{}) (bool, error)
}

// SagaVariableResolver resolves saga variables
type SagaVariableResolver interface {
	ResolveVariables(template string, variables map[string]interface{}) (interface{}, error)
	UpdateVariables(saga *Saga, stepResult *SagaStepResult) error
}

// SagaPersistenceStore handles saga persistence
type SagaPersistenceStore interface {
	SaveSaga(saga *Saga) error
	LoadSaga(sagaID string) (*Saga, error)
	UpdateSagaStatus(sagaID string, status SagaStatus) error
	UpdateSagaStep(sagaID string, stepIndex int, step *SagaStep) error
	DeleteSaga(sagaID string) error
	ListSagas(filter SagaFilter) ([]*Saga, error)
}

// SagaFilter defines filters for listing sagas
type SagaFilter struct {
	Status        []SagaStatus `json:"status"`
	StartedAfter  time.Time    `json:"started_after"`
	StartedBefore time.Time    `json:"started_before"`
	DefinitionID  string       `json:"definition_id"`
	Limit         int          `json:"limit"`
	Offset        int          `json:"offset"`
}

// SagaEventBus handles saga events
type SagaEventBus interface {
	PublishEvent(event SagaEvent) error
	SubscribeToEvents(eventType SagaEventType, handler SagaEventHandler) error
}

// SagaEvent represents a saga event
type SagaEvent struct {
	Type      SagaEventType          `json:"type"`
	SagaID    string                 `json:"saga_id"`
	StepID    string                 `json:"step_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// SagaEventType represents types of saga events
type SagaEventType string

const (
	SagaEventStarted               SagaEventType = "saga_started"
	SagaEventCompleted             SagaEventType = "saga_completed"
	SagaEventFailed                SagaEventType = "saga_failed"
	SagaEventAborted               SagaEventType = "saga_aborted"
	SagaEventStepStarted           SagaEventType = "step_started"
	SagaEventStepCompleted         SagaEventType = "step_completed"
	SagaEventStepFailed            SagaEventType = "step_failed"
	SagaEventCompensationStarted   SagaEventType = "compensation_started"
	SagaEventCompensationCompleted SagaEventType = "compensation_completed"
	SagaEventCompensationFailed    SagaEventType = "compensation_failed"
)

// SagaEventHandler handles saga events
type SagaEventHandler func(event SagaEvent) error

// SagaTimeoutManager manages saga timeouts
type SagaTimeoutManager struct {
	timeouts map[string]*SagaTimeout
	mutex    sync.RWMutex
	logger   zerolog.Logger
}

// SagaTimeout represents a saga timeout
type SagaTimeout struct {
	SagaID    string        `json:"saga_id"`
	StepID    string        `json:"step_id,omitempty"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"start_time"`
	Timer     *time.Timer   `json:"-"`
	Handler   func()        `json:"-"`
}

// NewSagaManager creates a new saga manager
func NewSagaManager(logger zerolog.Logger, persistenceStore SagaPersistenceStore, eventBus SagaEventBus) *SagaManager {
	sm := &SagaManager{
		logger:              logger.With().Str("component", "saga_manager").Logger(),
		sagas:               make(map[string]*Saga),
		sagaDefinitions:     make(map[string]*SagaDefinition),
		compensationManager: NewCompensationManager(logger),
		sagaOrchestrator:    NewSagaOrchestrator(logger),
		persistenceStore:    persistenceStore,
		eventBus:            eventBus,
		timeoutManager:      NewSagaTimeoutManager(logger),
	}

	// Register default saga definitions
	sm.registerDefaultSagaDefinitions()

	return sm
}

// RegisterSagaDefinition registers a saga definition
func (sm *SagaManager) RegisterSagaDefinition(definition *SagaDefinition) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Validate definition
	if err := sm.validateSagaDefinition(definition); err != nil {
		return fmt.Errorf("invalid saga definition: %w", err)
	}

	sm.sagaDefinitions[definition.ID] = definition

	sm.logger.Info().
		Str("definition_id", definition.ID).
		Str("name", definition.Name).
		Str("version", definition.Version).
		Msg("Saga definition registered")

	return nil
}

// StartSaga starts a new saga execution
func (sm *SagaManager) StartSaga(ctx context.Context, definitionID string, variables map[string]interface{}) (*Saga, error) {
	sm.mutex.RLock()
	definition, exists := sm.sagaDefinitions[definitionID]
	sm.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("saga definition %s not found", definitionID)
	}

	// Create saga instance
	saga := &Saga{
		ID:               sm.generateSagaID(),
		DefinitionID:     definitionID,
		Status:           SagaStatusPending,
		CurrentStep:      0,
		Steps:            make([]*SagaStep, len(definition.Steps)),
		CompensationMode: false,
		StartTime:        time.Now(),
		Context:          make(map[string]interface{}),
		Variables:        variables,
		Metadata:         make(map[string]interface{}),
		LastUpdated:      time.Now(),
		CompletedSteps:   []int{},
		FailedSteps:      []int{},
		CompensatedSteps: []int{},
		ErrorDetails:     make(map[int]string),
		RetryCount:       0,
		MaxRetries:       definition.RetryPolicy.MaxAttempts,
		TimeoutDuration:  definition.TimeoutDuration,
	}

	// Initialize steps
	for i, stepDef := range definition.Steps {
		saga.Steps[i] = &SagaStep{
			ID:                 stepDef.ID,
			DefinitionID:       stepDef.ID,
			Status:             SagaStepStatusPending,
			CompensationStatus: SagaCompensationStatusNone,
			Context:            make(map[string]interface{}),
		}
	}

	// Save saga
	if err := sm.persistenceStore.SaveSaga(saga); err != nil {
		return nil, fmt.Errorf("failed to save saga: %w", err)
	}

	// Store in memory
	sm.mutex.Lock()
	sm.sagas[saga.ID] = saga
	sm.mutex.Unlock()

	// Publish start event
	sm.eventBus.PublishEvent(SagaEvent{
		Type:      SagaEventStarted,
		SagaID:    saga.ID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"definition_id": definitionID,
			"variables":     variables,
		},
	})

	// Set up timeout
	if saga.TimeoutDuration > 0 {
		sm.timeoutManager.SetTimeout(saga.ID, "", saga.TimeoutDuration, func() {
			sm.timeoutSaga(saga.ID)
		})
	}

	sm.logger.Info().
		Str("saga_id", saga.ID).
		Str("definition_id", definitionID).
		Msg("Saga started")

	// Start execution asynchronously
	go sm.executeSaga(ctx, saga)

	return saga, nil
}

// executeSaga executes a saga
func (sm *SagaManager) executeSaga(ctx context.Context, saga *Saga) {
	sm.updateSagaStatus(saga, SagaStatusRunning)

	definition := sm.sagaDefinitions[saga.DefinitionID]

	// Execute steps sequentially
	for i, stepDef := range definition.Steps {
		if saga.CompensationMode {
			break // Stop forward execution if in compensation mode
		}

		// Check if step should be skipped based on conditions
		shouldExecute, err := sm.sagaOrchestrator.conditionEvaluator.EvaluateConditions(stepDef.Conditions, saga.Variables)
		if err != nil {
			sm.logger.Error().
				Err(err).
				Str("saga_id", saga.ID).
				Str("step_id", stepDef.ID).
				Msg("Failed to evaluate step conditions")

			sm.failSaga(saga, i, fmt.Errorf("condition evaluation failed: %w", err))
			return
		}

		if !shouldExecute {
			saga.Steps[i].Status = SagaStepStatusSkipped
			sm.logger.Debug().
				Str("saga_id", saga.ID).
				Str("step_id", stepDef.ID).
				Msg("Step skipped due to conditions")
			continue
		}

		// Execute step
		if err := sm.executeStep(ctx, saga, i, stepDef); err != nil {
			sm.failSaga(saga, i, err)
			return
		}

		saga.CompletedSteps = append(saga.CompletedSteps, i)
		saga.CurrentStep = i + 1
		saga.LastUpdated = time.Now()

		// Update persistence
		sm.persistenceStore.UpdateSagaStep(saga.ID, i, saga.Steps[i])
	}

	// If we reach here, all steps completed successfully
	sm.completeSaga(saga)
}

// executeStep executes a single saga step
func (sm *SagaManager) executeStep(ctx context.Context, saga *Saga, stepIndex int, stepDef *SagaStepDefinition) error {
	step := saga.Steps[stepIndex]
	step.Status = SagaStepStatusRunning
	step.StartTime = time.Now()

	sm.eventBus.PublishEvent(SagaEvent{
		Type:      SagaEventStepStarted,
		SagaID:    saga.ID,
		StepID:    stepDef.ID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"step_index": stepIndex,
			"step_name":  stepDef.Name,
		},
	})

	// Set step timeout
	if stepDef.Timeout > 0 {
		stepCtx, cancel := context.WithTimeout(ctx, stepDef.Timeout)
		defer cancel()
		ctx = stepCtx
	}

	// Execute step with retry logic
	var result *SagaStepResult
	var err error

	retryPolicy := stepDef.RetryPolicy
	if retryPolicy == nil {
		retryPolicy = &SagaStepRetryPolicy{MaxAttempts: 1}
	}

	for attempt := 0; attempt < retryPolicy.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			time.Sleep(retryPolicy.Delay)
		}

		result, err = sm.sagaOrchestrator.stepExecutor.ExecuteStep(ctx, saga, stepDef)
		if err == nil {
			break
		}

		// Check if error is retryable
		if !sm.isRetryableError(err, retryPolicy.RetryableErrors) {
			break
		}

		step.RetryCount++
	}

	endTime := time.Now()
	step.EndTime = &endTime
	step.Duration = endTime.Sub(step.StartTime)

	if err != nil {
		step.Status = SagaStepStatusFailed
		step.Error = err.Error()

		sm.eventBus.PublishEvent(SagaEvent{
			Type:      SagaEventStepFailed,
			SagaID:    saga.ID,
			StepID:    stepDef.ID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"step_index":  stepIndex,
				"error":       err.Error(),
				"retry_count": step.RetryCount,
			},
		})

		return err
	}

	// Step completed successfully
	step.Status = SagaStepStatusCompleted
	step.Result = result.Result

	// Update saga variables
	if err := sm.sagaOrchestrator.variableResolver.UpdateVariables(saga, result); err != nil {
		sm.logger.Warn().
			Err(err).
			Str("saga_id", saga.ID).
			Str("step_id", stepDef.ID).
			Msg("Failed to update saga variables")
	}

	sm.eventBus.PublishEvent(SagaEvent{
		Type:      SagaEventStepCompleted,
		SagaID:    saga.ID,
		StepID:    stepDef.ID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"step_index": stepIndex,
			"result":     result.Result,
			"duration":   step.Duration,
		},
	})

	sm.logger.Info().
		Str("saga_id", saga.ID).
		Str("step_id", stepDef.ID).
		Dur("duration", step.Duration).
		Msg("Step completed successfully")

	return nil
}

// failSaga handles saga failure and initiates compensation
func (sm *SagaManager) failSaga(saga *Saga, failedStepIndex int, err error) {
	saga.Status = SagaStatusFailed
	saga.FailedSteps = append(saga.FailedSteps, failedStepIndex)
	saga.ErrorDetails[failedStepIndex] = err.Error()

	now := time.Now()
	saga.EndTime = &now
	saga.LastUpdated = now

	sm.logger.Error().
		Err(err).
		Str("saga_id", saga.ID).
		Int("failed_step", failedStepIndex).
		Msg("Saga step failed")

	sm.eventBus.PublishEvent(SagaEvent{
		Type:      SagaEventFailed,
		SagaID:    saga.ID,
		StepID:    saga.Steps[failedStepIndex].ID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"failed_step": failedStepIndex,
			"error":       err.Error(),
		},
	})

	// Start compensation
	go sm.startCompensation(context.Background(), saga, failedStepIndex)
}

// startCompensation starts the compensation process
func (sm *SagaManager) startCompensation(ctx context.Context, saga *Saga, failedStepIndex int) {
	saga.CompensationMode = true
	saga.Status = SagaStatusCompensating
	saga.LastUpdated = time.Now()

	sm.eventBus.PublishEvent(SagaEvent{
		Type:      SagaEventCompensationStarted,
		SagaID:    saga.ID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"failed_step": failedStepIndex,
		},
	})

	// Get compensation strategy
	definition := sm.sagaDefinitions[saga.DefinitionID]
	strategy := sm.compensationManager.getCompensationStrategy(definition.CompensationMode)

	if err := strategy.Compensate(ctx, saga, failedStepIndex); err != nil {
		sm.logger.Error().
			Err(err).
			Str("saga_id", saga.ID).
			Msg("Compensation failed")

		saga.Status = SagaStatusAborted

		sm.eventBus.PublishEvent(SagaEvent{
			Type:      SagaEventAborted,
			SagaID:    saga.ID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"compensation_error": err.Error(),
			},
		})
	} else {
		sm.logger.Info().
			Str("saga_id", saga.ID).
			Msg("Compensation completed successfully")

		saga.Status = SagaStatusCompensated

		sm.eventBus.PublishEvent(SagaEvent{
			Type:      SagaEventCompensationCompleted,
			SagaID:    saga.ID,
			Timestamp: time.Now(),
		})
	}

	now := time.Now()
	saga.EndTime = &now
	saga.LastUpdated = now

	// Update persistence
	sm.persistenceStore.UpdateSagaStatus(saga.ID, saga.Status)
}

// completeSaga marks a saga as completed
func (sm *SagaManager) completeSaga(saga *Saga) {
	saga.Status = SagaStatusCompleted
	now := time.Now()
	saga.EndTime = &now
	saga.LastUpdated = now

	// Clear timeout
	sm.timeoutManager.ClearTimeout(saga.ID)

	sm.eventBus.PublishEvent(SagaEvent{
		Type:      SagaEventCompleted,
		SagaID:    saga.ID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"completed_steps": len(saga.CompletedSteps),
			"duration":        saga.EndTime.Sub(saga.StartTime),
		},
	})

	sm.logger.Info().
		Str("saga_id", saga.ID).
		Dur("duration", saga.EndTime.Sub(saga.StartTime)).
		Int("completed_steps", len(saga.CompletedSteps)).
		Msg("Saga completed successfully")

	// Update persistence
	sm.persistenceStore.UpdateSagaStatus(saga.ID, saga.Status)
}

// timeoutSaga handles saga timeout
func (sm *SagaManager) timeoutSaga(sagaID string) {
	sm.mutex.RLock()
	saga, exists := sm.sagas[sagaID]
	sm.mutex.RUnlock()

	if !exists {
		return
	}

	saga.Status = SagaStatusTimeout
	now := time.Now()
	saga.EndTime = &now
	saga.LastUpdated = now

	sm.logger.Warn().
		Str("saga_id", sagaID).
		Dur("timeout_duration", saga.TimeoutDuration).
		Msg("Saga timed out")

	// Start compensation for timeout
	go sm.startCompensation(context.Background(), saga, saga.CurrentStep)
}

// updateSagaStatus updates the status of a saga
func (sm *SagaManager) updateSagaStatus(saga *Saga, status SagaStatus) {
	saga.Status = status
	saga.LastUpdated = time.Now()
	sm.persistenceStore.UpdateSagaStatus(saga.ID, status)
}

// isRetryableError checks if an error is retryable
func (sm *SagaManager) isRetryableError(err error, retryableErrors []string) bool {
	if len(retryableErrors) == 0 {
		return false
	}

	errorStr := err.Error()
	for _, retryableError := range retryableErrors {
		if contains(errorStr, retryableError) {
			return true
		}
	}

	return false
}

// validateSagaDefinition validates a saga definition
func (sm *SagaManager) validateSagaDefinition(definition *SagaDefinition) error {
	if definition.ID == "" {
		return fmt.Errorf("saga definition ID is required")
	}

	if definition.Name == "" {
		return fmt.Errorf("saga definition name is required")
	}

	if len(definition.Steps) == 0 {
		return fmt.Errorf("saga definition must have at least one step")
	}

	// Validate steps
	stepIDs := make(map[string]bool)
	for _, step := range definition.Steps {
		if step.ID == "" {
			return fmt.Errorf("step ID is required")
		}

		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		stepIDs[step.ID] = true

		// Validate dependencies
		for _, dep := range step.Dependencies {
			if !stepIDs[dep] {
				return fmt.Errorf("step %s depends on non-existent step %s", step.ID, dep)
			}
		}
	}

	return nil
}

// generateSagaID generates a unique saga ID
func (sm *SagaManager) generateSagaID() string {
	return fmt.Sprintf("saga_%d", time.Now().UnixNano())
}

// registerDefaultSagaDefinitions registers default saga definitions
func (sm *SagaManager) registerDefaultSagaDefinitions() {
	// Container Deployment Saga
	deploymentSaga := &SagaDefinition{
		ID:          "container_deployment",
		Name:        "Container Deployment Saga",
		Description: "Deploys a containerized application with rollback capabilities",
		Version:     "1.0.0",
		Steps: []*SagaStepDefinition{
			{
				ID:   "analyze",
				Name: "Analyze Repository",
				Type: "action",
				ActionDefinition: &SagaActionDefinition{
					ToolName:  "analyze_repository",
					Operation: "analyze",
					Timeout:   2 * time.Minute,
				},
				CompensationDefinition: &SagaCompensationDefinition{
					ToolName:  "cleanup",
					Operation: "cleanup_analysis",
					Timeout:   30 * time.Second,
					Optional:  true,
				},
				Timeout: 2 * time.Minute,
			},
			{
				ID:           "build",
				Name:         "Build Image",
				Type:         "action",
				Dependencies: []string{"analyze"},
				ActionDefinition: &SagaActionDefinition{
					ToolName:  "build_image",
					Operation: "build",
					Timeout:   10 * time.Minute,
				},
				CompensationDefinition: &SagaCompensationDefinition{
					ToolName:  "cleanup",
					Operation: "cleanup_image",
					Timeout:   1 * time.Minute,
				},
				Timeout: 15 * time.Minute,
			},
			{
				ID:           "deploy",
				Name:         "Deploy to Kubernetes",
				Type:         "action",
				Dependencies: []string{"build"},
				ActionDefinition: &SagaActionDefinition{
					ToolName:  "deploy_kubernetes",
					Operation: "deploy",
					Timeout:   5 * time.Minute,
				},
				CompensationDefinition: &SagaCompensationDefinition{
					ToolName:  "deploy_kubernetes",
					Operation: "rollback",
					Timeout:   3 * time.Minute,
				},
				Timeout:          5 * time.Minute,
				CriticalityLevel: "high",
			},
		},
		CompensationMode: "backward",
		TimeoutDuration:  30 * time.Minute,
		RetryPolicy: &SagaRetryPolicy{
			MaxAttempts:       3,
			InitialDelay:      1 * time.Second,
			MaxDelay:          30 * time.Second,
			BackoffType:       "exponential",
			BackoffMultiplier: 2.0,
			RetryableErrors:   []string{"timeout", "connection", "temporary"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sm.RegisterSagaDefinition(deploymentSaga)
}

// GetSaga retrieves a saga by ID
func (sm *SagaManager) GetSaga(sagaID string) (*Saga, error) {
	sm.mutex.RLock()
	saga, exists := sm.sagas[sagaID]
	sm.mutex.RUnlock()

	if exists {
		return saga, nil
	}

	// Try loading from persistence store
	return sm.persistenceStore.LoadSaga(sagaID)
}

// ListSagas lists sagas based on filter criteria
func (sm *SagaManager) ListSagas(filter SagaFilter) ([]*Saga, error) {
	return sm.persistenceStore.ListSagas(filter)
}

// AbortSaga aborts a running saga
func (sm *SagaManager) AbortSaga(sagaID string) error {
	saga, err := sm.GetSaga(sagaID)
	if err != nil {
		return err
	}

	if saga.Status != SagaStatusRunning {
		return fmt.Errorf("saga %s is not running", sagaID)
	}

	saga.Status = SagaStatusAborted
	now := time.Now()
	saga.EndTime = &now
	saga.LastUpdated = now

	// Clear timeout
	sm.timeoutManager.ClearTimeout(sagaID)

	sm.eventBus.PublishEvent(SagaEvent{
		Type:      SagaEventAborted,
		SagaID:    sagaID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"reason": "manual_abort",
		},
	})

	return sm.persistenceStore.UpdateSagaStatus(sagaID, SagaStatusAborted)
}

// Helper functions for creating sub-components
func NewCompensationManager(logger zerolog.Logger) *CompensationManager {
	return &CompensationManager{
		compensationStrategies: make(map[string]CompensationStrategy),
		compensationQueue:      []CompensationAction{},
		logger:                 logger.With().Str("component", "compensation_manager").Logger(),
	}
}

func (cm *CompensationManager) getCompensationStrategy(mode string) CompensationStrategy {
	// Return default backward compensation strategy
	return &BackwardCompensationStrategy{logger: cm.logger}
}

func NewSagaOrchestrator(logger zerolog.Logger) *SagaOrchestrator {
	return &SagaOrchestrator{
		stepExecutor:       &DefaultSagaStepExecutor{logger: logger},
		conditionEvaluator: &DefaultSagaConditionEvaluator{logger: logger},
		variableResolver:   &DefaultSagaVariableResolver{logger: logger},
		logger:             logger.With().Str("component", "saga_orchestrator").Logger(),
	}
}

func NewSagaTimeoutManager(logger zerolog.Logger) *SagaTimeoutManager {
	return &SagaTimeoutManager{
		timeouts: make(map[string]*SagaTimeout),
		logger:   logger.With().Str("component", "saga_timeout_manager").Logger(),
	}
}

// SetTimeout sets a timeout for a saga or step
func (stm *SagaTimeoutManager) SetTimeout(sagaID, stepID string, duration time.Duration, handler func()) {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	key := sagaID
	if stepID != "" {
		key = fmt.Sprintf("%s_%s", sagaID, stepID)
	}

	timeout := &SagaTimeout{
		SagaID:    sagaID,
		StepID:    stepID,
		Duration:  duration,
		StartTime: time.Now(),
		Handler:   handler,
	}

	timeout.Timer = time.AfterFunc(duration, handler)
	stm.timeouts[key] = timeout
}

// ClearTimeout clears a timeout
func (stm *SagaTimeoutManager) ClearTimeout(sagaID string) {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	if timeout, exists := stm.timeouts[sagaID]; exists {
		timeout.Timer.Stop()
		delete(stm.timeouts, sagaID)
	}
}

// Placeholder implementations for interfaces
type BackwardCompensationStrategy struct {
	logger zerolog.Logger
}

func (bcs *BackwardCompensationStrategy) Compensate(ctx context.Context, saga *Saga, failedStepIndex int) error {
	// Compensate in reverse order
	for i := failedStepIndex - 1; i >= 0; i-- {
		if err := bcs.compensateStep(ctx, saga, i); err != nil {
			return err
		}
	}
	return nil
}

func (bcs *BackwardCompensationStrategy) CanCompensate(saga *Saga, failedStepIndex int) bool {
	return true
}

func (bcs *BackwardCompensationStrategy) GetCompensationOrder(saga *Saga, failedStepIndex int) []int {
	order := []int{}
	for i := failedStepIndex - 1; i >= 0; i-- {
		order = append(order, i)
	}
	return order
}

func (bcs *BackwardCompensationStrategy) compensateStep(ctx context.Context, saga *Saga, stepIndex int) error {
	// Placeholder compensation logic
	step := saga.Steps[stepIndex]
	step.CompensationStatus = SagaCompensationStatusCompleted
	saga.CompensatedSteps = append(saga.CompensatedSteps, stepIndex)
	return nil
}

type DefaultSagaStepExecutor struct {
	logger zerolog.Logger
}

func (dsse *DefaultSagaStepExecutor) ExecuteStep(ctx context.Context, saga *Saga, step *SagaStepDefinition) (*SagaStepResult, error) {
	// Placeholder step execution
	return &SagaStepResult{
		Success:  true,
		Result:   map[string]interface{}{"status": "completed"},
		Duration: 100 * time.Millisecond,
	}, nil
}

func (dsse *DefaultSagaStepExecutor) ExecuteCompensation(ctx context.Context, saga *Saga, step *SagaStep) (*SagaCompensationResult, error) {
	// Placeholder compensation execution
	return &SagaCompensationResult{
		Success:  true,
		Result:   map[string]interface{}{"status": "compensated"},
		Duration: 50 * time.Millisecond,
	}, nil
}

type DefaultSagaConditionEvaluator struct {
	logger zerolog.Logger
}

func (dsce *DefaultSagaConditionEvaluator) EvaluateConditions(conditions []SagaCondition, variables map[string]interface{}) (bool, error) {
	if len(conditions) == 0 {
		return true, nil
	}
	// Placeholder condition evaluation
	return true, nil
}

type DefaultSagaVariableResolver struct {
	logger zerolog.Logger
}

func (dsvr *DefaultSagaVariableResolver) ResolveVariables(template string, variables map[string]interface{}) (interface{}, error) {
	// Placeholder variable resolution
	return template, nil
}

func (dsvr *DefaultSagaVariableResolver) UpdateVariables(saga *Saga, stepResult *SagaStepResult) error {
	// Placeholder variable update
	if stepResult.Variables != nil {
		for k, v := range stepResult.Variables {
			saga.Variables[k] = v
		}
	}
	return nil
}
