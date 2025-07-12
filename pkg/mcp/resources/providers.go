// Package resources provides MCP resource providers for logs and progress
package resources

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/progress"
	"github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/localrivet/gomcp/server"
)

// Store manages workflow progress and logs
type Store struct {
	progressData map[string]*progress.WorkflowProgress
	logData      map[string]map[string][]string // workflowID -> stepName -> logs
	mu           sync.RWMutex
	logger       *slog.Logger
	maxLogSize   int
}

// NewStore creates a new resource store
func NewStore(logger *slog.Logger) *Store {
	return &Store{
		progressData: make(map[string]*progress.WorkflowProgress),
		logData:      make(map[string]map[string][]string),
		logger:       logger.With("component", "resource-store"),
		maxLogSize:   4096, // 4KB max per log resource
	}
}

// RegisterProviders registers all resource providers with the MCP server
func (s *Store) RegisterProviders(mcpServer server.Server) error {
	s.logger.Info("MCP resources feature not yet supported by gomcp library - using internal store")

	// The gomcp library currently doesn't expose a RegisterResource method
	// Resources are managed internally in the store and accessible via API methods
	// This will be implemented when the MCP specification for resources is fully supported

	s.logger.Info("Resource providers ready for internal access",
		"progress_resources", len(s.progressData),
		"log_resources", len(s.logData))

	return nil
}

// GetProgressAsResource returns progress data formatted as a resource
func (s *Store) GetProgressAsResource(workflowID string) (interface{}, error) {
	s.mu.RLock()
	progress, exists := s.progressData[workflowID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Convert to JSON with masked sensitive data
	data, err := json.Marshal(progress)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// Mask sensitive data before returning
	maskedResult := security.MaskMap(result)

	return maskedResult, nil
}

// GetLogsAsResource returns logs formatted as a resource
func (s *Store) GetLogsAsResource(workflowID, stepName string) (interface{}, error) {
	s.mu.RLock()
	workflowLogs, exists := s.logData[workflowID]
	if !exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	stepLogs, exists := workflowLogs[stepName]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("step logs not found: %s/%s", workflowID, stepName)
	}

	// Combine logs and mask sensitive data
	combinedLogs := strings.Join(stepLogs, "\n")
	maskedLogs := security.Mask(combinedLogs)

	// Tail last maxLogSize bytes for quick-peek in chat
	if len(maskedLogs) > s.maxLogSize {
		maskedLogs = maskedLogs[len(maskedLogs)-s.maxLogSize:]
		maskedLogs = "... (truncated)\n" + maskedLogs
	}

	return maskedLogs, nil
}

// GetWorkflowListAsResource returns workflow list formatted as a resource
func (s *Store) GetWorkflowListAsResource() (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workflows := make([]map[string]interface{}, 0, len(s.progressData))
	for id, wf := range s.progressData {
		workflows = append(workflows, map[string]interface{}{
			"id":           id,
			"name":         wf.WorkflowName,
			"status":       wf.Status,
			"percentage":   wf.Percentage,
			"start_time":   wf.StartTime,
			"duration":     wf.Duration.String(),
			"total_steps":  wf.TotalSteps,
			"current_step": wf.CurrentStep,
		})
	}

	return map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

// StoreProgress stores workflow progress data
func (s *Store) StoreProgress(workflowID string, progressData *progress.WorkflowProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progressData[workflowID] = progressData
	s.logger.Debug("Stored workflow progress", "workflow_id", workflowID)
}

// StoreLogs stores logs for a workflow step
func (s *Store) StoreLogs(workflowID, stepName string, logs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.logData[workflowID] == nil {
		s.logData[workflowID] = make(map[string][]string)
	}

	// Append logs
	s.logData[workflowID][stepName] = append(s.logData[workflowID][stepName], logs...)

	// Keep only last N log lines to prevent memory growth
	const maxLogLines = 1000
	if len(s.logData[workflowID][stepName]) > maxLogLines {
		start := len(s.logData[workflowID][stepName]) - maxLogLines
		s.logData[workflowID][stepName] = s.logData[workflowID][stepName][start:]
	}

	s.logger.Debug("Stored step logs",
		"workflow_id", workflowID,
		"step", stepName,
		"lines", len(logs))
}

// GetProgress retrieves workflow progress
func (s *Store) GetProgress(workflowID string) (*progress.WorkflowProgress, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	progress, exists := s.progressData[workflowID]
	return progress, exists
}

// GetLogs retrieves logs for a workflow step
func (s *Store) GetLogs(workflowID, stepName string) ([]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if workflowLogs, exists := s.logData[workflowID]; exists {
		if stepLogs, exists := workflowLogs[stepName]; exists {
			return stepLogs, true
		}
	}

	return nil, false
}

// CleanupOldData removes data older than the specified duration
func (s *Store) CleanupOldData(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	// Clean up old workflows
	for id, wf := range s.progressData {
		if wf.StartTime.Before(cutoff) {
			delete(s.progressData, id)
			delete(s.logData, id)
			s.logger.Debug("Cleaned up old workflow data", "workflow_id", id)
		}
	}
}

// StartCleanupRoutine starts a background cleanup routine
func (s *Store) StartCleanupRoutine(cleanupInterval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			s.CleanupOldData(maxAge)
		}
	}()

	s.logger.Info("Started resource cleanup routine",
		"interval", cleanupInterval,
		"max_age", maxAge)
}
