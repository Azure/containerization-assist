package runtime

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// ProgressStage represents a stage in a multi-stage operation
type ProgressStage struct {
	Name        string
	Weight      float64
	Description string
	StartTime   time.Time
	EndTime     time.Time
	Status      StageStatus
}

// StageStatus represents the status of a stage
type StageStatus string

const (
	StageStatusPending    StageStatus = "pending"
	StageStatusInProgress StageStatus = "in_progress"
	StageStatusCompleted  StageStatus = "completed"
	StageStatusFailed     StageStatus = "failed"
	StageStatusSkipped    StageStatus = "skipped"
)

// ProgressTracker tracks progress across multiple stages
type ProgressTracker struct {
	stages       []ProgressStage
	currentStage int
	callbacks    []StageProgressCallback
	mu           sync.RWMutex
	startTime    time.Time
}

// StageProgressCallback is called when progress is updated
type StageProgressCallback func(progress float64, stage string, message string)

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(stages []ProgressStage) *ProgressTracker {
	return &ProgressTracker{
		stages:       stages,
		currentStage: -1,
		callbacks:    make([]StageProgressCallback, 0),
		startTime:    time.Now(),
	}
}

// AddCallback adds a progress callback
func (t *ProgressTracker) AddCallback(callback StageProgressCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callbacks = append(t.callbacks, callback)
}

// StartStage starts a new stage
func (t *ProgressTracker) StartStage(stageName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Find the stage
	stageIndex := -1
	for i, stage := range t.stages {
		if stage.Name == stageName {
			stageIndex = i
			break
		}
	}

	if stageIndex == -1 {
		return errors.Resourcef("runtime/progress", "stage %s not found", stageName)
	}

	// Update current stage
	t.currentStage = stageIndex
	t.stages[stageIndex].Status = StageStatusInProgress
	t.stages[stageIndex].StartTime = time.Now()

	// Notify callbacks
	t.notifyCallbacks(0.0, fmt.Sprintf("Starting %s", stageName))

	return nil
}

// UpdateProgress updates progress within the current stage
func (t *ProgressTracker) UpdateProgress(stageProgress float64, message string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.currentStage < 0 || t.currentStage >= len(t.stages) {
		return
	}

	// Ensure progress is within bounds
	if stageProgress < 0 {
		stageProgress = 0
	}
	if stageProgress > 1 {
		stageProgress = 1
	}

	t.notifyCallbacks(stageProgress, message)
}

// CompleteStage completes the current stage
func (t *ProgressTracker) CompleteStage() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentStage < 0 || t.currentStage >= len(t.stages) {
		return
	}

	t.stages[t.currentStage].Status = StageStatusCompleted
	t.stages[t.currentStage].EndTime = time.Now()

	t.notifyCallbacks(1.0, fmt.Sprintf("Completed %s", t.stages[t.currentStage].Name))
}

// FailStage marks the current stage as failed
func (t *ProgressTracker) FailStage(reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentStage < 0 || t.currentStage >= len(t.stages) {
		return
	}

	t.stages[t.currentStage].Status = StageStatusFailed
	t.stages[t.currentStage].EndTime = time.Now()

	t.notifyCallbacks(0.0, fmt.Sprintf("Failed %s: %s", t.stages[t.currentStage].Name, reason))
}

// SkipStage marks a stage as skipped
func (t *ProgressTracker) SkipStage(stageName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, stage := range t.stages {
		if stage.Name == stageName {
			t.stages[i].Status = StageStatusSkipped
			return nil
		}
	}

	return errors.Resourcef("runtime/progress", "stage %s not found", stageName)
}

// GetOverallProgress returns the overall progress (0.0 to 1.0)
func (t *ProgressTracker) GetOverallProgress() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var completedWeight float64
	var currentStageProgress float64

	for i, stage := range t.stages {
		switch stage.Status {
		case StageStatusCompleted:
			completedWeight += stage.Weight
		case StageStatusInProgress:
			if i == t.currentStage {
				// Add partial progress of current stage
				currentStageProgress = stage.Weight * 0.5 // Assume 50% if in progress
			}
		case StageStatusSkipped:
			completedWeight += stage.Weight
		}
	}

	return completedWeight + currentStageProgress
}

// GetCurrentStage returns the current stage
func (t *ProgressTracker) GetCurrentStage() (ProgressStage, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.currentStage < 0 || t.currentStage >= len(t.stages) {
		return ProgressStage{}, false
	}

	return t.stages[t.currentStage], true
}

// GetElapsedTime returns the elapsed time since start
func (t *ProgressTracker) GetElapsedTime() time.Duration {
	return time.Since(t.startTime)
}

// GetStageSummary returns a summary of all stages
func (t *ProgressTracker) GetStageSummary() []StageSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summaries := make([]StageSummary, len(t.stages))

	for i, stage := range t.stages {
		summary := StageSummary{
			Name:   stage.Name,
			Status: stage.Status,
			Weight: stage.Weight,
		}

		if !stage.StartTime.IsZero() && !stage.EndTime.IsZero() {
			summary.Duration = stage.EndTime.Sub(stage.StartTime)
		}

		summaries[i] = summary
	}

	return summaries
}

// StageSummary provides a summary of a stage
type StageSummary struct {
	Name     string
	Status   StageStatus
	Weight   float64
	Duration time.Duration
}

// notifyCallbacks notifies all registered callbacks
func (t *ProgressTracker) notifyCallbacks(stageProgress float64, message string) {
	if t.currentStage < 0 || t.currentStage >= len(t.stages) {
		return
	}

	currentStage := t.stages[t.currentStage]

	// Calculate overall progress
	var baseProgress float64
	for i := 0; i < t.currentStage; i++ {
		if t.stages[i].Status == StageStatusCompleted || t.stages[i].Status == StageStatusSkipped {
			baseProgress += t.stages[i].Weight
		}
	}

	overallProgress := baseProgress + (stageProgress * currentStage.Weight)

	// Notify all callbacks
	for _, callback := range t.callbacks {
		callback(overallProgress, currentStage.Name, message)
	}
}

// SimpleProgressReporter provides a simple progress reporting interface
type SimpleProgressReporter struct {
	tracker *ProgressTracker
	logger  interface{} // zerolog.Logger
}

// NewSimpleProgressReporter creates a new simple progress reporter
func NewSimpleProgressReporter(stages []ProgressStage, logger interface{}) *SimpleProgressReporter {
	tracker := NewProgressTracker(stages)
	return &SimpleProgressReporter{
		tracker: tracker,
		logger:  logger,
	}
}

// StartStage starts a new stage
func (r *SimpleProgressReporter) StartStage(stageName string) {
	if err := r.tracker.StartStage(stageName); err != nil {
		// Log error
	}
}

// Update updates progress with a message
func (r *SimpleProgressReporter) Update(progress float64, message string) {
	r.tracker.UpdateProgress(progress, message)
}

// Complete completes the current stage
func (r *SimpleProgressReporter) Complete() {
	r.tracker.CompleteStage()
}

// Fail marks the current stage as failed
func (r *SimpleProgressReporter) Fail(reason string) {
	r.tracker.FailStage(reason)
}

// GetProgress returns the overall progress
func (r *SimpleProgressReporter) GetProgress() float64 {
	return r.tracker.GetOverallProgress()
}

// GetSummary returns a summary of all stages
func (r *SimpleProgressReporter) GetSummary() []StageSummary {
	return r.tracker.GetStageSummary()
}

// BatchProgressReporter reports progress for batch operations
type BatchProgressReporter struct {
	totalItems     int
	processedItems int
	currentItem    string
	callbacks      []StageProgressCallback
	mu             sync.RWMutex
}

// NewBatchProgressReporter creates a new batch progress reporter
func NewBatchProgressReporter(totalItems int) *BatchProgressReporter {
	return &BatchProgressReporter{
		totalItems: totalItems,
		callbacks:  make([]StageProgressCallback, 0),
	}
}

// AddCallback adds a progress callback
func (r *BatchProgressReporter) AddCallback(callback StageProgressCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, callback)
}

// StartItem starts processing a new item
func (r *BatchProgressReporter) StartItem(itemName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.currentItem = itemName

	progress := float64(r.processedItems) / float64(r.totalItems)
	message := fmt.Sprintf("Processing %s (%d/%d)", itemName, r.processedItems+1, r.totalItems)

	for _, callback := range r.callbacks {
		callback(progress, "batch", message)
	}
}

// CompleteItem marks the current item as complete
func (r *BatchProgressReporter) CompleteItem() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.processedItems++

	progress := float64(r.processedItems) / float64(r.totalItems)
	message := fmt.Sprintf("Completed %s (%d/%d)", r.currentItem, r.processedItems, r.totalItems)

	for _, callback := range r.callbacks {
		callback(progress, "batch", message)
	}
}

// GetProgress returns the current progress
func (r *BatchProgressReporter) GetProgress() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.totalItems == 0 {
		return 1.0
	}

	return float64(r.processedItems) / float64(r.totalItems)
}

// IsComplete returns true if all items have been processed
func (r *BatchProgressReporter) IsComplete() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.processedItems >= r.totalItems
}
