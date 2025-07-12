// Package resources provides MCP resource providers for logs and progress
package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/utilities"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Store manages workflow progress and logs
type Store struct {
	progressData map[string]*workflow.WorkflowProgress
	logData      map[string]map[string][]string // workflowID -> stepName -> logs
	mu           sync.RWMutex
	logger       *slog.Logger
	maxLogSize   int
	cleanupStop  chan struct{}
	lastCleanup  time.Time
}

// NewStore creates a new resource store
func NewStore(logger *slog.Logger) *Store {
	return &Store{
		progressData: make(map[string]*workflow.WorkflowProgress),
		logData:      make(map[string]map[string][]string),
		logger:       logger.With("component", "resource-store"),
		maxLogSize:   4096, // 4KB max per log resource
	}
}

// RegisterProviders registers all resource providers with the MCP server
func (s *Store) RegisterProviders(mcpServer interface {
	AddResource(resource mcp.Resource, handler server.ResourceHandlerFunc)
	AddResourceTemplate(template mcp.ResourceTemplate, handler server.ResourceTemplateHandlerFunc)
}) error {
	s.logger.Info("Registering MCP resource providers")

	// Register static resources for known workflows
	s.mu.RLock()
	for workflowID, progressData := range s.progressData {
		resourceURI := fmt.Sprintf("progress://%s", workflowID)
		data, _ := json.Marshal(progressData)

		resource := mcp.NewResource(
			resourceURI,
			fmt.Sprintf("Progress for workflow %s", workflowID),
			mcp.WithResourceDescription("Live progress tracking for containerization workflow"),
			mcp.WithMIMEType("application/json"),
		)

		mcpServer.AddResource(resource, server.ResourceHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			}, nil
		}))
	}
	s.mu.RUnlock()

	// Register resource templates for dynamic access
	// Progress resources
	progressTemplate := mcp.NewResourceTemplate(
		"progress://{workflowID}",
		"Workflow Progress",
		mcp.WithTemplateDescription("Live progress tracking for containerization workflows"),
		mcp.WithTemplateMIMEType("application/json"),
	)

	mcpServer.AddResourceTemplate(progressTemplate, server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return s.handleProgressResource(ctx, request)
	}))

	// Log resources
	logsTemplate := mcp.NewResourceTemplate(
		"logs://{workflowID}/{stepName}",
		"Workflow Step Logs",
		mcp.WithTemplateDescription("Logs for specific workflow steps"),
		mcp.WithTemplateMIMEType("text/plain"),
	)

	mcpServer.AddResourceTemplate(logsTemplate, server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return s.handleLogsResource(ctx, request)
	}))

	s.logger.Info("Resource providers registered",
		"static_resources", len(s.progressData),
		"templates", 2)

	return nil
}

// handleProgressResource handles dynamic progress resource requests
func (s *Store) handleProgressResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract workflowID from URI
	parts := strings.Split(req.Params.URI, "://")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid resource URI: %s", req.Params.URI)
	}
	workflowID := parts[1]

	resourceData, err := s.GetProgressAsResource(workflowID)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(resourceData)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

// handleLogsResource handles dynamic log resource requests
func (s *Store) handleLogsResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract workflowID and stepName from URI
	// URI format: logs://workflowID/stepName
	parts := strings.Split(req.Params.URI, "://")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid resource URI: %s", req.Params.URI)
	}

	pathParts := strings.Split(parts[1], "/")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid log resource path: %s", parts[1])
	}

	workflowID := pathParts[0]
	stepName := pathParts[1]

	logData, err := s.GetLogsAsResource(workflowID, stepName)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "text/plain",
			Text:     logData.(string),
		},
	}, nil
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
	maskedResult := utilities.MaskMap(result)

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
	maskedLogs := utilities.Mask(combinedLogs)

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
func (s *Store) StoreProgress(workflowID string, progressData *workflow.WorkflowProgress) {
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
func (s *Store) GetProgress(workflowID string) (*workflow.WorkflowProgress, bool) {
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
	s.mu.Lock()
	s.cleanupStop = make(chan struct{})
	s.lastCleanup = time.Now()
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.CleanupOldData(maxAge)
				s.mu.Lock()
				s.lastCleanup = time.Now()
				s.mu.Unlock()
			case <-s.cleanupStop:
				return
			}
		}
	}()

	s.logger.Info("Started resource cleanup routine",
		"interval", cleanupInterval,
		"max_age", maxAge)
}

// StopCleanupRoutine stops the background cleanup routine
func (s *Store) StopCleanupRoutine() {
	s.mu.Lock()
	if s.cleanupStop != nil {
		close(s.cleanupStop)
		s.cleanupStop = nil
	}
	s.mu.Unlock()

	s.logger.Info("Stopped resource cleanup routine")
}

// GetResourceCount returns the total number of resources
func (s *Store) GetResourceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := len(s.progressData)
	for _, logs := range s.logData {
		count += len(logs)
	}
	return count
}

// GetLastCleanupTime returns the last cleanup time
func (s *Store) GetLastCleanupTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCleanup
}
