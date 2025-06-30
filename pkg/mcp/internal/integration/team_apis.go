package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// TeamAPI provides integration APIs for other teams to use InfraBot's infrastructure
type TeamAPI interface {
	// Docker Operations
	ExecuteDockerOperation(ctx context.Context, req DockerOperationRequest) (*DockerOperationResponse, error)

	// Session Management
	CreateManagedSession(ctx context.Context, req SessionRequest) (*SessionResponse, error)
	GetSessionState(ctx context.Context, sessionID string) (*SessionStateResponse, error)
	TrackTeamOperation(ctx context.Context, req OperationTrackingRequest) error

	// Performance Integration
	RecordTeamMetrics(ctx context.Context, req MetricsRequest) error
	GetPerformanceReport(ctx context.Context, teamName string) (*TeamPerformanceReport, error)

	// Progress Tracking
	StartProgressTracking(ctx context.Context, req ProgressTrackingRequest) (*ProgressTracker, error)
	GetOperationProgress(ctx context.Context, operationID string) (*ProgressState, error)
}

// Request/Response types

type DockerOperationRequest struct {
	SessionID     string            `json:"session_id"`
	Operation     string            `json:"operation"` // "pull", "push", "tag", "build"
	ImageRef      string            `json:"image_ref"`
	SourceRef     string            `json:"source_ref,omitempty"`
	TargetRef     string            `json:"target_ref,omitempty"`
	Registry      string            `json:"registry,omitempty"`
	Username      string            `json:"username,omitempty"`
	Password      string            `json:"password,omitempty"`
	Token         string            `json:"token,omitempty"`
	Options       map[string]string `json:"options,omitempty"`
	TeamName      string            `json:"team_name"`
	ComponentName string            `json:"component_name"`
}

type DockerOperationResponse struct {
	Success     bool              `json:"success"`
	Output      string            `json:"output"`
	Duration    time.Duration     `json:"duration"`
	Error       string            `json:"error,omitempty"`
	Metrics     *OperationMetrics `json:"metrics,omitempty"`
	OperationID string            `json:"operation_id"`
}

type SessionRequest struct {
	TeamName      string            `json:"team_name"`
	ComponentName string            `json:"component_name"`
	RepoURL       string            `json:"repo_url,omitempty"`
	Labels        []string          `json:"labels,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	TTL           time.Duration     `json:"ttl,omitempty"`
}

type SessionResponse struct {
	SessionID    string    `json:"session_id"`
	WorkspaceDir string    `json:"workspace_dir"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type SessionStateResponse struct {
	SessionID      string                 `json:"session_id"`
	Status         string                 `json:"status"`
	ActiveJobs     []string               `json:"active_jobs"`
	CompletedTools []string               `json:"completed_tools"`
	LastError      string                 `json:"last_error,omitempty"`
	DiskUsage      int64                  `json:"disk_usage_bytes"`
	WorkspaceDir   string                 `json:"workspace_dir"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      time.Time              `json:"created_at"`
	LastAccessed   time.Time              `json:"last_accessed"`
	ExpiresAt      time.Time              `json:"expires_at"`
}

type OperationTrackingRequest struct {
	SessionID     string                 `json:"session_id"`
	ToolName      string                 `json:"tool_name"`
	OperationType string                 `json:"operation_type"`
	TeamName      string                 `json:"team_name"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Success       bool                   `json:"success"`
	Error         error                  `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type MetricsRequest struct {
	TeamName      string                 `json:"team_name"`
	ComponentName string                 `json:"component_name"`
	MetricName    string                 `json:"metric_name"`
	Value         float64                `json:"value"`
	Unit          string                 `json:"unit"`
	Labels        map[string]string      `json:"labels,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
}

type ProgressTrackingRequest struct {
	OperationID   string `json:"operation_id"`
	SessionID     string `json:"session_id"`
	ToolName      string `json:"tool_name"`
	TeamName      string `json:"team_name"`
	ComponentName string `json:"component_name"`
}

// Integration implementation

type InfraTeamAPI struct {
	dockerClient       docker.DockerClient
	sessionManager     *session.SessionManager
	performanceMonitor *observability.PerformanceMonitor
	progressTracker    observability.ProgressTracker
	logger             zerolog.Logger
}

// NewInfraTeamAPI creates a new team integration API
func NewInfraTeamAPI(
	dockerClient docker.DockerClient,
	sessionManager *session.SessionManager,
	performanceMonitor *observability.PerformanceMonitor,
	progressTracker observability.ProgressTracker,
	logger zerolog.Logger,
) *InfraTeamAPI {
	return &InfraTeamAPI{
		dockerClient:       dockerClient,
		sessionManager:     sessionManager,
		performanceMonitor: performanceMonitor,
		progressTracker:    progressTracker,
		logger:             logger.With().Str("component", "team_api").Logger(),
	}
}

// ExecuteDockerOperation implements TeamAPI.ExecuteDockerOperation
func (api *InfraTeamAPI) ExecuteDockerOperation(ctx context.Context, req DockerOperationRequest) (*DockerOperationResponse, error) {
	// Performance tracking will be done after operation completion

	operationID := fmt.Sprintf("%s-%s-%d", req.Operation, req.SessionID, time.Now().UnixNano())
	startTime := time.Now()

	api.logger.Info().
		Str("operation", req.Operation).
		Str("session_id", req.SessionID).
		Str("team", req.TeamName).
		Str("component", req.ComponentName).
		Str("operation_id", operationID).
		Msg("Starting Docker operation for team")

	var output string
	var err error

	// Authenticate if credentials provided
	if req.Username != "" && req.Password != "" {
		_, authErr := api.dockerClient.Login(ctx, req.Registry, req.Username, req.Password)
		if authErr != nil {
			return &DockerOperationResponse{
				Success:     false,
				Error:       fmt.Sprintf("authentication failed: %v", authErr),
				OperationID: operationID,
				Duration:    time.Since(startTime),
			}, authErr
		}
	} else if req.Token != "" {
		_, authErr := api.dockerClient.LoginWithToken(ctx, req.Registry, req.Token)
		if authErr != nil {
			return &DockerOperationResponse{
				Success:     false,
				Error:       fmt.Sprintf("token authentication failed: %v", authErr),
				OperationID: operationID,
				Duration:    time.Since(startTime),
			}, authErr
		}
	}

	// Validate operation type first
	switch req.Operation {
	case "pull", "push":
		// Basic operations that need only image reference
	case "tag":
		if req.SourceRef == "" || req.TargetRef == "" {
			return &DockerOperationResponse{
				Success:     false,
				Error:       "source_ref and target_ref required for tag operation",
				OperationID: operationID,
				Duration:    time.Since(startTime),
			}, fmt.Errorf("missing required parameters for tag operation")
		}
	default:
		return &DockerOperationResponse{
			Success:     false,
			Error:       fmt.Sprintf("unsupported operation: %s", req.Operation),
			OperationID: operationID,
			Duration:    time.Since(startTime),
		}, fmt.Errorf("unsupported operation: %s", req.Operation)
	}

	// Check if Docker client is available
	if api.dockerClient == nil {
		return &DockerOperationResponse{
			Success:     false,
			Error:       "Docker client not available",
			OperationID: operationID,
			Duration:    time.Since(startTime),
		}, fmt.Errorf("Docker client not configured")
	}

	// Execute the requested operation
	switch req.Operation {
	case "pull":
		output, err = api.dockerClient.Pull(ctx, req.ImageRef)
	case "push":
		output, err = api.dockerClient.Push(ctx, req.ImageRef)
	case "tag":
		output, err = api.dockerClient.Tag(ctx, req.SourceRef, req.TargetRef)
	}

	duration := time.Since(startTime)
	success := err == nil

	// Record performance metrics
	api.performanceMonitor.RecordMeasurement(req.TeamName, req.ComponentName, observability.Measurement{
		Timestamp: time.Now(),
		Latency:   duration,
		Success:   success,
		ErrorMessage: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
	})

	// Track in session if session manager available
	if api.sessionManager != nil {
		trackErr := api.sessionManager.TrackToolExecution(req.SessionID, fmt.Sprintf("docker_%s", req.Operation), req)
		if trackErr != nil {
			api.logger.Warn().Err(trackErr).Msg("Failed to track tool execution")
		}

		// Complete the tracking
		completeErr := api.sessionManager.CompleteToolExecution(req.SessionID, fmt.Sprintf("docker_%s", req.Operation), success, err, 0)
		if completeErr != nil {
			api.logger.Warn().Err(completeErr).Msg("Failed to complete tool execution tracking")
		}
	}

	response := &DockerOperationResponse{
		Success:     success,
		Output:      output,
		Duration:    duration,
		OperationID: operationID,
	}

	if err != nil {
		response.Error = err.Error()
	}

	api.logger.Info().
		Str("operation", req.Operation).
		Str("operation_id", operationID).
		Dur("duration", duration).
		Bool("success", success).
		Msg("Completed Docker operation for team")

	return response, nil
}

// CreateManagedSession implements TeamAPI.CreateManagedSession
func (api *InfraTeamAPI) CreateManagedSession(ctx context.Context, req SessionRequest) (*SessionResponse, error) {
	sessionInterface, err := api.sessionManager.CreateSession("")
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	sessionState, ok := sessionInterface.(*session.SessionState)
	if !ok {
		return nil, fmt.Errorf("invalid session state type")
	}

	// Configure session with team-specific settings
	err = api.sessionManager.UpdateSession(sessionState.SessionID, func(s interface{}) {
		if state, ok := s.(*session.SessionState); ok {
			state.RepoURL = req.RepoURL
			state.Labels = req.Labels

			// Add team metadata
			if state.Metadata == nil {
				state.Metadata = make(map[string]interface{})
			}
			state.Metadata["team_name"] = req.TeamName
			state.Metadata["component_name"] = req.ComponentName

			for k, v := range req.Metadata {
				state.Metadata[k] = v
			}

			// Set custom TTL if provided
			if req.TTL > 0 {
				state.ExpiresAt = state.CreatedAt.Add(req.TTL)
			}
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to configure session: %w", err)
	}

	api.logger.Info().
		Str("session_id", sessionState.SessionID).
		Str("team", req.TeamName).
		Str("component", req.ComponentName).
		Msg("Created managed session for team")

	return &SessionResponse{
		SessionID:    sessionState.SessionID,
		WorkspaceDir: sessionState.WorkspaceDir,
		CreatedAt:    sessionState.CreatedAt,
		ExpiresAt:    sessionState.ExpiresAt,
	}, nil
}

// GetSessionState implements TeamAPI.GetSessionState
func (api *InfraTeamAPI) GetSessionState(ctx context.Context, sessionID string) (*SessionStateResponse, error) {
	sessionData, err := api.sessionManager.GetSessionData(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session state: %w", err)
	}

	// Convert to response format
	response := &SessionStateResponse{
		SessionID:      sessionData.ID,
		Status:         "active", // Default status
		ActiveJobs:     sessionData.ActiveJobs,
		CompletedTools: sessionData.CompletedTools,
		LastError:      sessionData.LastError,
		DiskUsage:      sessionData.DiskUsage,
		WorkspaceDir:   sessionData.WorkspacePath,
		CreatedAt:      sessionData.CreatedAt,
		LastAccessed:   sessionData.UpdatedAt,
		ExpiresAt:      sessionData.ExpiresAt,
		Metadata:       make(map[string]interface{}),
	}

	// Convert metadata
	for k, v := range sessionData.Metadata {
		response.Metadata[k] = v
	}

	// Determine status
	if time.Now().After(sessionData.ExpiresAt) {
		response.Status = "expired"
	} else if len(sessionData.ActiveJobs) > 0 {
		response.Status = "busy"
	}

	return response, nil
}

// TrackTeamOperation implements TeamAPI.TrackTeamOperation
func (api *InfraTeamAPI) TrackTeamOperation(ctx context.Context, req OperationTrackingRequest) error {
	// Track in session manager
	if req.EndTime == nil {
		// Starting operation
		err := api.sessionManager.TrackToolExecution(req.SessionID, req.ToolName, req.Metadata)
		if err != nil {
			return fmt.Errorf("failed to track tool execution start: %w", err)
		}
	} else {
		// Completing operation
		err := api.sessionManager.CompleteToolExecution(req.SessionID, req.ToolName, req.Success, req.Error, 0)
		if err != nil {
			return fmt.Errorf("failed to complete tool execution tracking: %w", err)
		}
	}

	// Record performance metrics if we have timing information
	if req.EndTime != nil {
		duration := req.EndTime.Sub(req.StartTime)
		api.performanceMonitor.RecordMeasurement(req.TeamName, req.ToolName, observability.Measurement{
			Timestamp: *req.EndTime,
			Latency:   duration,
			Success:   req.Success,
			ErrorMessage: func() string {
				if req.Error != nil {
					return req.Error.Error()
				}
				return ""
			}(),
		})
	}

	api.logger.Debug().
		Str("session_id", req.SessionID).
		Str("tool", req.ToolName).
		Str("team", req.TeamName).
		Bool("success", req.Success).
		Msg("Tracked team operation")

	return nil
}

// RecordTeamMetrics implements TeamAPI.RecordTeamMetrics
func (api *InfraTeamAPI) RecordTeamMetrics(ctx context.Context, req MetricsRequest) error {
	// Record measurement using the performance monitor
	api.performanceMonitor.RecordMeasurement(req.TeamName, req.ComponentName, observability.Measurement{
		Timestamp: req.Timestamp,
		Success:   true,
	})

	api.logger.Debug().
		Str("team", req.TeamName).
		Str("component", req.ComponentName).
		Str("metric", req.MetricName).
		Float64("value", req.Value).
		Msg("Recorded team metric")

	return nil
}

// GetPerformanceReport implements TeamAPI.GetPerformanceReport
func (api *InfraTeamAPI) GetPerformanceReport(ctx context.Context, teamName string) (*TeamPerformanceReport, error) {
	// Get overall performance report
	fullReport := api.performanceMonitor.GetPerformanceReport()

	// Filter for specific team
	teamPerf, exists := fullReport.TeamMetrics[teamName]
	if !exists {
		return &TeamPerformanceReport{
			Timestamp:     time.Now(),
			OverallHealth: "UNKNOWN",
			TeamMetrics:   make(map[string]observability.TeamPerformance),
		}, nil
	}

	// Create team-specific report
	teamReport := &TeamPerformanceReport{
		Timestamp:     fullReport.Timestamp,
		OverallHealth: api.getTeamHealthStatus(teamPerf),
		TeamMetrics:   map[string]observability.TeamPerformance{teamName: teamPerf},
		SystemSummary: fullReport.SystemSummary,
	}

	return teamReport, nil
}

// StartProgressTracking implements TeamAPI.StartProgressTracking
func (api *InfraTeamAPI) StartProgressTracking(ctx context.Context, req ProgressTrackingRequest) (*ProgressTracker, error) {
	if api.progressTracker == nil {
		return nil, fmt.Errorf("progress tracking not available")
	}

	callback := api.progressTracker.Start(req.OperationID)

	// Set session info if available
	if tracker, ok := api.progressTracker.(*observability.ComprehensiveProgressTracker); ok {
		tracker.SetSessionInfo(req.OperationID, req.SessionID, req.ToolName)
	}

	api.logger.Info().
		Str("operation_id", req.OperationID).
		Str("session_id", req.SessionID).
		Str("team", req.TeamName).
		Msg("Started progress tracking for team operation")

	return &ProgressTracker{
		OperationID: req.OperationID,
		Callback:    callback,
		tracker:     api.progressTracker,
	}, nil
}

// GetOperationProgress implements TeamAPI.GetOperationProgress
func (api *InfraTeamAPI) GetOperationProgress(ctx context.Context, operationID string) (*ProgressState, error) {
	if api.progressTracker == nil {
		return nil, fmt.Errorf("progress tracking not available")
	}

	return api.progressTracker.GetProgress(operationID)
}

// Helper types for team API

type ProgressTracker struct {
	OperationID string
	Callback    observability.ProgressCallback
	tracker     observability.ProgressTracker
}

type ProgressState = observability.ProgressState

// OperationMetrics represents metrics for an operation
type OperationMetrics struct {
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	MemoryUsage int64         `json:"memory_usage"`
}
type TeamPerformanceReport = observability.TeamPerformanceReport

func (pt *ProgressTracker) Update(progress float64, message string) {
	if pt.Callback != nil {
		pt.Callback(progress, message)
	}
}

func (pt *ProgressTracker) Complete(result interface{}, err error) {
	if pt.tracker != nil {
		pt.tracker.Complete(pt.OperationID, result, err)
	}
}

// Helper methods

func (api *InfraTeamAPI) getTeamHealthStatus(teamPerf observability.TeamPerformance) string {
	redCount := 0
	yellowCount := 0
	greenCount := 0

	for _, metrics := range teamPerf.Components {
		switch metrics.AlertStatus {
		case "RED":
			redCount++
		case "YELLOW":
			yellowCount++
		case "GREEN":
			greenCount++
		}
	}

	if redCount > 0 {
		return "RED"
	} else if yellowCount > 0 {
		return "YELLOW"
	} else if greenCount > 0 {
		return "GREEN"
	}
	return "UNKNOWN"
}
