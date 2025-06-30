package pipeline

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// AutoScaler provides intelligent auto-scaling for session management and pipeline operations
type AutoScaler struct {
	sessionManager       *session.SessionManager
	logger               zerolog.Logger
	monitoringIntegrator *MonitoringIntegrator

	// Scaling configuration
	config AutoScalingConfig

	// Current scaling state
	currentCapacity   int
	targetCapacity    int
	lastScaleAction   time.Time
	scalingInProgress bool
	scalingMutex      sync.RWMutex

	// Metrics tracking
	loadHistory     []LoadMetric
	capacityHistory []CapacityMetric
	metricsMutex    sync.RWMutex

	// Worker pools
	sessionWorkerPool   *WorkerPool
	operationWorkerPool *WorkerPool

	// Predictive scaling
	loadPredictor   *LoadPredictor
	anomalyDetector *AnomalyDetector
}

// AutoScalingConfig configures auto-scaling behavior
type AutoScalingConfig struct {
	MinCapacity         int           `json:"min_capacity"`
	MaxCapacity         int           `json:"max_capacity"`
	TargetUtilization   float64       `json:"target_utilization"`
	ScaleUpThreshold    float64       `json:"scale_up_threshold"`
	ScaleDownThreshold  float64       `json:"scale_down_threshold"`
	ScaleUpCooldown     time.Duration `json:"scale_up_cooldown"`
	ScaleDownCooldown   time.Duration `json:"scale_down_cooldown"`
	MetricsWindow       time.Duration `json:"metrics_window"`
	EvaluationInterval  time.Duration `json:"evaluation_interval"`
	PredictiveScaling   bool          `json:"predictive_scaling"`
	MaxScaleUpPercent   float64       `json:"max_scale_up_percent"`
	MaxScaleDownPercent float64       `json:"max_scale_down_percent"`
}

// LoadMetric represents load metrics at a point in time
type LoadMetric struct {
	Timestamp         time.Time     `json:"timestamp"`
	SessionCount      int           `json:"session_count"`
	ActiveOperations  int           `json:"active_operations"`
	QueuedOperations  int           `json:"queued_operations"`
	CPUUtilization    float64       `json:"cpu_utilization"`
	MemoryUtilization float64       `json:"memory_utilization"`
	ResponseTime      time.Duration `json:"response_time"`
	ErrorRate         float64       `json:"error_rate"`
}

// CapacityMetric represents capacity changes over time
type CapacityMetric struct {
	Timestamp   time.Time `json:"timestamp"`
	Capacity    int       `json:"capacity"`
	ScaleAction string    `json:"scale_action"`
	Reason      string    `json:"reason"`
	LoadAtScale float64   `json:"load_at_scale"`
}

// WorkerPool manages a pool of workers for operations
type WorkerPool struct {
	workers    []Worker
	workChan   chan WorkItem
	capacity   int
	active     int
	mutex      sync.RWMutex
	shutdownCh chan struct{}
}

// Worker represents a worker in the pool
type Worker struct {
	ID       int
	WorkChan chan WorkItem
	QuitChan chan struct{}
	Active   bool
}

// WorkItem represents work to be processed
type WorkItem struct {
	ID       string
	Type     string
	Data     interface{}
	Callback func(interface{}) error
	Context  context.Context
}

// LoadPredictor provides predictive load analysis
type LoadPredictor struct {
	historicalData []LoadMetric
	patterns       map[string]LoadPattern
	mutex          sync.RWMutex
}

// LoadPattern represents recurring load patterns
type LoadPattern struct {
	Name       string    `json:"name"`
	TimeOfDay  time.Time `json:"time_of_day"`
	DayOfWeek  int       `json:"day_of_week"`
	LoadFactor float64   `json:"load_factor"`
	Confidence float64   `json:"confidence"`
}

// AnomalyDetector detects unusual load patterns
type AnomalyDetector struct {
	baseline        LoadMetric
	thresholds      AnomalyThresholds
	recentAnomalies []Anomaly
	mutex           sync.RWMutex
}

// AnomalyThresholds defines thresholds for anomaly detection
type AnomalyThresholds struct {
	CPUVariance    float64 `json:"cpu_variance"`
	MemoryVariance float64 `json:"memory_variance"`
	SessionSpike   float64 `json:"session_spike"`
	ErrorRateSpike float64 `json:"error_rate_spike"`
}

// Anomaly represents a detected anomaly
type Anomaly struct {
	Timestamp   time.Time  `json:"timestamp"`
	Type        string     `json:"type"`
	Severity    string     `json:"severity"`
	Description string     `json:"description"`
	Metric      LoadMetric `json:"metric"`
}

// ScalingDecision represents a scaling decision
type ScalingDecision struct {
	Action          string    `json:"action"`
	CurrentCapacity int       `json:"current_capacity"`
	TargetCapacity  int       `json:"target_capacity"`
	Reason          string    `json:"reason"`
	Confidence      float64   `json:"confidence"`
	Timestamp       time.Time `json:"timestamp"`
}

// NewAutoScaler creates a new auto-scaler
func NewAutoScaler(
	sessionManager *session.SessionManager,
	monitoringIntegrator *MonitoringIntegrator,
	config AutoScalingConfig,
	logger zerolog.Logger,
) *AutoScaler {

	// Set defaults
	if config.MinCapacity == 0 {
		config.MinCapacity = 2
	}
	if config.MaxCapacity == 0 {
		config.MaxCapacity = 100
	}
	if config.TargetUtilization == 0 {
		config.TargetUtilization = 70.0
	}
	if config.ScaleUpThreshold == 0 {
		config.ScaleUpThreshold = 80.0
	}
	if config.ScaleDownThreshold == 0 {
		config.ScaleDownThreshold = 30.0
	}
	if config.ScaleUpCooldown == 0 {
		config.ScaleUpCooldown = 5 * time.Minute
	}
	if config.ScaleDownCooldown == 0 {
		config.ScaleDownCooldown = 15 * time.Minute
	}
	if config.MetricsWindow == 0 {
		config.MetricsWindow = 10 * time.Minute
	}
	if config.EvaluationInterval == 0 {
		config.EvaluationInterval = 1 * time.Minute
	}
	if config.MaxScaleUpPercent == 0 {
		config.MaxScaleUpPercent = 100.0
	}
	if config.MaxScaleDownPercent == 0 {
		config.MaxScaleDownPercent = 50.0
	}

	as := &AutoScaler{
		sessionManager:       sessionManager,
		logger:               logger.With().Str("component", "auto_scaler").Logger(),
		monitoringIntegrator: monitoringIntegrator,
		config:               config,
		currentCapacity:      config.MinCapacity,
		targetCapacity:       config.MinCapacity,
		loadHistory:          make([]LoadMetric, 0),
		capacityHistory:      make([]CapacityMetric, 0),
	}

	// Initialize worker pools
	as.sessionWorkerPool = NewWorkerPool(config.MinCapacity, "session")
	as.operationWorkerPool = NewWorkerPool(config.MinCapacity, "operation")

	// Initialize predictive scaling components
	if config.PredictiveScaling {
		as.loadPredictor = NewLoadPredictor()
		as.anomalyDetector = NewAnomalyDetector(AnomalyThresholds{
			CPUVariance:    20.0,
			MemoryVariance: 15.0,
			SessionSpike:   50.0,
			ErrorRateSpike: 10.0,
		})
	}

	// Start auto-scaling loop
	go as.startAutoScalingLoop()

	// Start metrics collection
	go as.startMetricsCollection()

	as.logger.Info().
		Int("min_capacity", config.MinCapacity).
		Int("max_capacity", config.MaxCapacity).
		Float64("target_utilization", config.TargetUtilization).
		Bool("predictive_scaling", config.PredictiveScaling).
		Msg("Auto-scaler initialized")

	return as
}

// EvaluateScaling evaluates current load and makes scaling decisions
func (as *AutoScaler) EvaluateScaling(ctx context.Context) (*ScalingDecision, error) {
	as.scalingMutex.RLock()
	defer as.scalingMutex.RUnlock()

	// Get current load metrics
	loadMetric, err := as.getCurrentLoadMetric(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get load metrics: %w", err)
	}

	// Record load metric
	as.recordLoadMetric(loadMetric)

	// Calculate utilization
	utilization := as.calculateUtilization(loadMetric)

	// Determine scaling action
	decision := &ScalingDecision{
		CurrentCapacity: as.currentCapacity,
		TargetCapacity:  as.currentCapacity,
		Timestamp:       time.Now(),
		Confidence:      1.0,
	}

	// Check cooldown periods
	timeSinceLastScale := time.Since(as.lastScaleAction)

	if utilization > as.config.ScaleUpThreshold {
		if timeSinceLastScale >= as.config.ScaleUpCooldown {
			newCapacity := as.calculateScaleUpCapacity(utilization, loadMetric)
			decision.Action = "scale_up"
			decision.TargetCapacity = newCapacity
			decision.Reason = fmt.Sprintf("High utilization: %.2f%% > %.2f%%", utilization, as.config.ScaleUpThreshold)
		} else {
			decision.Action = "none"
			decision.Reason = fmt.Sprintf("Scale up in cooldown (%.1fs remaining)",
				as.config.ScaleUpCooldown.Seconds()-timeSinceLastScale.Seconds())
		}
	} else if utilization < as.config.ScaleDownThreshold {
		if timeSinceLastScale >= as.config.ScaleDownCooldown {
			newCapacity := as.calculateScaleDownCapacity(utilization, loadMetric)
			decision.Action = "scale_down"
			decision.TargetCapacity = newCapacity
			decision.Reason = fmt.Sprintf("Low utilization: %.2f%% < %.2f%%", utilization, as.config.ScaleDownThreshold)
		} else {
			decision.Action = "none"
			decision.Reason = fmt.Sprintf("Scale down in cooldown (%.1fs remaining)",
				as.config.ScaleDownCooldown.Seconds()-timeSinceLastScale.Seconds())
		}
	} else {
		decision.Action = "none"
		decision.Reason = fmt.Sprintf("Utilization within target range: %.2f%%", utilization)
	}

	// Apply predictive scaling if enabled
	if as.config.PredictiveScaling && decision.Action == "none" {
		predictiveDecision := as.evaluatePredictiveScaling(loadMetric)
		if predictiveDecision.Action != "none" {
			decision = predictiveDecision
		}
	}

	// Validate scaling decision
	decision.TargetCapacity = as.validateCapacity(decision.TargetCapacity)

	return decision, nil
}

// ExecuteScaling executes a scaling decision
func (as *AutoScaler) ExecuteScaling(ctx context.Context, decision *ScalingDecision) error {
	if decision.Action == "none" || decision.TargetCapacity == as.currentCapacity {
		return nil
	}

	as.scalingMutex.Lock()
	defer as.scalingMutex.Unlock()

	if as.scalingInProgress {
		return fmt.Errorf("scaling operation already in progress")
	}

	as.scalingInProgress = true
	defer func() { as.scalingInProgress = false }()

	oldCapacity := as.currentCapacity

	// Execute scaling
	switch decision.Action {
	case "scale_up":
		err := as.scaleUp(ctx, decision.TargetCapacity)
		if err != nil {
			return fmt.Errorf("failed to scale up: %w", err)
		}
	case "scale_down":
		err := as.scaleDown(ctx, decision.TargetCapacity)
		if err != nil {
			return fmt.Errorf("failed to scale down: %w", err)
		}
	}

	// Update state
	as.currentCapacity = decision.TargetCapacity
	as.targetCapacity = decision.TargetCapacity
	as.lastScaleAction = time.Now()

	// Record capacity change
	as.recordCapacityChange(CapacityMetric{
		Timestamp:   time.Now(),
		Capacity:    decision.TargetCapacity,
		ScaleAction: decision.Action,
		Reason:      decision.Reason,
		LoadAtScale: as.calculateCurrentLoad(),
	})

	as.logger.Info().
		Str("action", decision.Action).
		Int("old_capacity", oldCapacity).
		Int("new_capacity", decision.TargetCapacity).
		Str("reason", decision.Reason).
		Msg("Scaling operation completed")

	return nil
}

// GetScalingMetrics returns current scaling metrics
func (as *AutoScaler) GetScalingMetrics() ScalingMetrics {
	as.metricsMutex.RLock()
	defer as.metricsMutex.RUnlock()

	currentLoad := LoadMetric{}
	if len(as.loadHistory) > 0 {
		currentLoad = as.loadHistory[len(as.loadHistory)-1]
	}

	return ScalingMetrics{
		CurrentCapacity:   as.currentCapacity,
		TargetCapacity:    as.targetCapacity,
		Utilization:       as.calculateUtilization(currentLoad),
		LoadHistory:       as.getRecentLoadHistory(as.config.MetricsWindow),
		CapacityHistory:   as.getRecentCapacityHistory(as.config.MetricsWindow),
		ScalingInProgress: as.scalingInProgress,
		LastScaleAction:   as.lastScaleAction,
	}
}

// Private helper methods

func (as *AutoScaler) startAutoScalingLoop() {
	ticker := time.NewTicker(as.config.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			decision, err := as.EvaluateScaling(ctx)
			if err != nil {
				as.logger.Error().Err(err).Msg("Failed to evaluate scaling")
				cancel()
				continue
			}

			if decision.Action != "none" {
				if err := as.ExecuteScaling(ctx, decision); err != nil {
					as.logger.Error().Err(err).Msg("Failed to execute scaling")
				}
			}

			cancel()
		}
	}
}

func (as *AutoScaler) startMetricsCollection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			if loadMetric, err := as.getCurrentLoadMetric(ctx); err == nil {
				as.recordLoadMetric(loadMetric)

				// Detect anomalies if enabled
				if as.anomalyDetector != nil {
					if anomaly := as.anomalyDetector.DetectAnomaly(loadMetric); anomaly != nil {
						as.handleAnomaly(*anomaly)
					}
				}
			}

			cancel()
		}
	}
}

func (as *AutoScaler) getCurrentLoadMetric(ctx context.Context) (LoadMetric, error) {
	// Get metrics from monitoring integrator
	monitoringMetrics, err := as.monitoringIntegrator.GetMonitoringMetrics(ctx)
	if err != nil {
		return LoadMetric{}, fmt.Errorf("failed to get monitoring metrics: %w", err)
	}

	return LoadMetric{
		Timestamp:         time.Now(),
		SessionCount:      int(monitoringMetrics.ActiveSessions),
		ActiveOperations:  as.operationWorkerPool.GetActiveCount(),
		QueuedOperations:  as.operationWorkerPool.GetQueuedCount(),
		CPUUtilization:    monitoringMetrics.SystemHealth.CPUUsage,
		MemoryUtilization: monitoringMetrics.SystemHealth.MemoryUsage,
		ResponseTime:      monitoringMetrics.AverageLatency,
		ErrorRate:         monitoringMetrics.ErrorRate,
	}, nil
}

func (as *AutoScaler) calculateUtilization(metric LoadMetric) float64 {
	// Calculate composite utilization score
	sessionUtilization := float64(metric.SessionCount) / float64(as.currentCapacity) * 100
	operationUtilization := float64(metric.ActiveOperations) / float64(as.currentCapacity) * 100

	// Weight different factors
	utilization := sessionUtilization*0.4 +
		operationUtilization*0.3 +
		metric.CPUUtilization*0.2 +
		metric.MemoryUtilization*0.1

	return math.Min(utilization, 100.0)
}

func (as *AutoScaler) calculateScaleUpCapacity(utilization float64, metric LoadMetric) int {
	// Calculate target capacity based on utilization
	scaleFactor := utilization / as.config.TargetUtilization
	newCapacity := int(math.Ceil(float64(as.currentCapacity) * scaleFactor))

	// Apply maximum scale up percentage
	maxIncrease := int(math.Ceil(float64(as.currentCapacity) * as.config.MaxScaleUpPercent / 100))
	if newCapacity > as.currentCapacity+maxIncrease {
		newCapacity = as.currentCapacity + maxIncrease
	}

	return as.validateCapacity(newCapacity)
}

func (as *AutoScaler) calculateScaleDownCapacity(utilization float64, metric LoadMetric) int {
	// Calculate target capacity based on utilization
	scaleFactor := utilization / as.config.TargetUtilization
	newCapacity := int(math.Floor(float64(as.currentCapacity) * scaleFactor))

	// Apply maximum scale down percentage
	maxDecrease := int(math.Ceil(float64(as.currentCapacity) * as.config.MaxScaleDownPercent / 100))
	if newCapacity < as.currentCapacity-maxDecrease {
		newCapacity = as.currentCapacity - maxDecrease
	}

	return as.validateCapacity(newCapacity)
}

func (as *AutoScaler) validateCapacity(capacity int) int {
	if capacity < as.config.MinCapacity {
		return as.config.MinCapacity
	}
	if capacity > as.config.MaxCapacity {
		return as.config.MaxCapacity
	}
	return capacity
}

func (as *AutoScaler) scaleUp(ctx context.Context, targetCapacity int) error {
	increase := targetCapacity - as.currentCapacity

	// Scale session worker pool
	if err := as.sessionWorkerPool.ScaleUp(increase); err != nil {
		return fmt.Errorf("failed to scale up session worker pool: %w", err)
	}

	// Scale operation worker pool
	if err := as.operationWorkerPool.ScaleUp(increase); err != nil {
		return fmt.Errorf("failed to scale up operation worker pool: %w", err)
	}

	return nil
}

func (as *AutoScaler) scaleDown(ctx context.Context, targetCapacity int) error {
	decrease := as.currentCapacity - targetCapacity

	// Scale down session worker pool
	if err := as.sessionWorkerPool.ScaleDown(decrease); err != nil {
		return fmt.Errorf("failed to scale down session worker pool: %w", err)
	}

	// Scale down operation worker pool
	if err := as.operationWorkerPool.ScaleDown(decrease); err != nil {
		return fmt.Errorf("failed to scale down operation worker pool: %w", err)
	}

	return nil
}

func (as *AutoScaler) recordLoadMetric(metric LoadMetric) {
	as.metricsMutex.Lock()
	defer as.metricsMutex.Unlock()

	as.loadHistory = append(as.loadHistory, metric)

	// Keep only recent history
	cutoff := time.Now().Add(-as.config.MetricsWindow * 2)
	var filteredHistory []LoadMetric
	for _, m := range as.loadHistory {
		if m.Timestamp.After(cutoff) {
			filteredHistory = append(filteredHistory, m)
		}
	}
	as.loadHistory = filteredHistory
}

func (as *AutoScaler) recordCapacityChange(metric CapacityMetric) {
	as.metricsMutex.Lock()
	defer as.metricsMutex.Unlock()

	as.capacityHistory = append(as.capacityHistory, metric)
}

func (as *AutoScaler) calculateCurrentLoad() float64 {
	if len(as.loadHistory) == 0 {
		return 0.0
	}
	return as.calculateUtilization(as.loadHistory[len(as.loadHistory)-1])
}

func (as *AutoScaler) getRecentLoadHistory(window time.Duration) []LoadMetric {
	cutoff := time.Now().Add(-window)
	var recent []LoadMetric
	for _, metric := range as.loadHistory {
		if metric.Timestamp.After(cutoff) {
			recent = append(recent, metric)
		}
	}
	return recent
}

func (as *AutoScaler) getRecentCapacityHistory(window time.Duration) []CapacityMetric {
	cutoff := time.Now().Add(-window)
	var recent []CapacityMetric
	for _, metric := range as.capacityHistory {
		if metric.Timestamp.After(cutoff) {
			recent = append(recent, metric)
		}
	}
	return recent
}

func (as *AutoScaler) evaluatePredictiveScaling(currentMetric LoadMetric) *ScalingDecision {
	// Placeholder for predictive scaling logic
	return &ScalingDecision{
		Action:          "none",
		CurrentCapacity: as.currentCapacity,
		TargetCapacity:  as.currentCapacity,
		Reason:          "No predictive scaling action needed",
		Confidence:      0.8,
		Timestamp:       time.Now(),
	}
}

func (as *AutoScaler) handleAnomaly(anomaly Anomaly) {
	as.logger.Warn().
		Str("anomaly_type", anomaly.Type).
		Str("severity", anomaly.Severity).
		Str("description", anomaly.Description).
		Msg("Anomaly detected")
}

// Worker Pool Implementation

func NewWorkerPool(capacity int, poolType string) *WorkerPool {
	wp := &WorkerPool{
		workers:    make([]Worker, 0, capacity),
		workChan:   make(chan WorkItem, capacity*2),
		capacity:   capacity,
		shutdownCh: make(chan struct{}),
	}

	// Start initial workers
	for i := 0; i < capacity; i++ {
		wp.addWorker(i)
	}

	return wp
}

func (wp *WorkerPool) addWorker(id int) {
	worker := Worker{
		ID:       id,
		WorkChan: wp.workChan,
		QuitChan: make(chan struct{}),
		Active:   true,
	}

	wp.workers = append(wp.workers, worker)

	go func() {
		for {
			select {
			case work := <-worker.WorkChan:
				wp.processWork(work)
			case <-worker.QuitChan:
				return
			case <-wp.shutdownCh:
				return
			}
		}
	}()
}

func (wp *WorkerPool) processWork(work WorkItem) {
	wp.mutex.Lock()
	wp.active++
	wp.mutex.Unlock()

	defer func() {
		wp.mutex.Lock()
		wp.active--
		wp.mutex.Unlock()
	}()

	if work.Callback != nil {
		work.Callback(work.Data)
	}
}

func (wp *WorkerPool) ScaleUp(increase int) error {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	for i := 0; i < increase; i++ {
		wp.addWorker(len(wp.workers))
	}

	wp.capacity += increase
	return nil
}

func (wp *WorkerPool) ScaleDown(decrease int) error {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	if decrease >= len(wp.workers) {
		decrease = len(wp.workers) - 1 // Keep at least one worker
	}

	// Stop workers
	for i := 0; i < decrease; i++ {
		if len(wp.workers) > 0 {
			worker := wp.workers[len(wp.workers)-1]
			close(worker.QuitChan)
			wp.workers = wp.workers[:len(wp.workers)-1]
		}
	}

	wp.capacity -= decrease
	return nil
}

func (wp *WorkerPool) GetActiveCount() int {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()
	return wp.active
}

func (wp *WorkerPool) GetQueuedCount() int {
	return len(wp.workChan)
}

// Load Predictor Implementation

func NewLoadPredictor() *LoadPredictor {
	return &LoadPredictor{
		historicalData: make([]LoadMetric, 0),
		patterns:       make(map[string]LoadPattern),
	}
}

// Anomaly Detector Implementation

func NewAnomalyDetector(thresholds AnomalyThresholds) *AnomalyDetector {
	return &AnomalyDetector{
		thresholds:      thresholds,
		recentAnomalies: make([]Anomaly, 0),
	}
}

func (ad *AnomalyDetector) DetectAnomaly(metric LoadMetric) *Anomaly {
	// Simple anomaly detection - in production this would be more sophisticated
	if metric.CPUUtilization > 90.0 {
		return &Anomaly{
			Timestamp:   time.Now(),
			Type:        "high_cpu",
			Severity:    "high",
			Description: fmt.Sprintf("CPU utilization spike: %.2f%%", metric.CPUUtilization),
			Metric:      metric,
		}
	}

	return nil
}

// ScalingMetrics represents current scaling state and metrics
type ScalingMetrics struct {
	CurrentCapacity   int              `json:"current_capacity"`
	TargetCapacity    int              `json:"target_capacity"`
	Utilization       float64          `json:"utilization"`
	LoadHistory       []LoadMetric     `json:"load_history"`
	CapacityHistory   []CapacityMetric `json:"capacity_history"`
	ScalingInProgress bool             `json:"scaling_in_progress"`
	LastScaleAction   time.Time        `json:"last_scale_action"`
}
