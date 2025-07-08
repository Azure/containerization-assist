package session

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// SessionState represents the complete state of an MCP session
type SessionState struct {
	Version string `json:"version"`

	ID           string    `json:"id"` // Alias for SessionID for compatibility
	SessionID    string    `json:"session_id"`
	WorkspaceDir string    `json:"workspace_dir"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastAccessed time.Time `json:"last_accessed"`
	ExpiresAt    time.Time `json:"expires_at"`
	Status       string    `json:"status"` // Status of the session (active, expired, etc.)

	RepoPath     string `json:"repo_path"`
	RepoURL      string `json:"repo_url,omitempty"`
	RepoFileTree string `json:"repo_file_tree"`

	RepoAnalysis map[string]interface{}       `json:"repo_analysis"`
	ScanSummary  *types.RepositoryScanSummary `json:"scan_summary,omitempty"`

	ImageRef types.ImageReference `json:"image_ref"`

	Dockerfile DockerfileState `json:"dockerfile"`

	SecurityScan *SecurityScanSummary `json:"security_scan,omitempty"`

	K8sManifests map[string]types.K8sManifest `json:"k8s_manifests"`

	Metadata map[string]interface{} `json:"metadata"`

	BuildLogs  []string `json:"build_logs"`
	DeployLogs []string `json:"deploy_logs"`

	ActiveJobs map[string]JobInfo `json:"active_jobs"`

	LastError *types.ToolError `json:"last_error,omitempty"`

	DiskUsage    int64 `json:"disk_usage_bytes"`
	MaxDiskUsage int64 `json:"max_disk_usage_bytes"`

	Labels []string `json:"labels"`

	K8sLabels map[string]string `json:"k8s_labels"`

	TokenUsage    int                    `json:"token_usage"`
	LastKnownGood *types.SessionSnapshot `json:"last_known_good,omitempty"`
	StageHistory  []ToolExecution        `json:"stage_history"`
}

// DockerfileState represents the state of the Dockerfile
type DockerfileState struct {
	Content          string                   `json:"content"`
	Path             string                   `json:"path"`
	Built            bool                     `json:"built"`
	Pushed           bool                     `json:"pushed"`
	BuildTime        *time.Time               `json:"build_time,omitempty"`
	ImageID          string                   `json:"image_id"`
	Size             int64                    `json:"size_bytes"`
	BuildArgs        map[string]string        `json:"build_args,omitempty"`
	Platform         string                   `json:"platform,omitempty"`
	LayerCount       int                      `json:"layer_count"`
	ValidationResult *SessionValidationResult `json:"validation_result,omitempty"`
}

// SessionValidationResult now uses the unified validation framework
type SessionValidationResult = api.ValidationResult

// SessionValidationData contains session-specific validation data
type SessionValidationData struct {
	ErrorCount   int    `json:"error_count"`
	WarningCount int    `json:"warning_count"`
	ValidatedBy  string `json:"validated_by"`
}

// SecurityScanSummary represents simplified security scan results stored in session
type SecurityScanSummary struct {
	Success   bool                 `json:"success"`
	ScannedAt time.Time            `json:"scanned_at"`
	ImageRef  string               `json:"image_ref"`
	Summary   VulnerabilitySummary `json:"summary"`
	Fixable   int                  `json:"fixable"`
	Scanner   string               `json:"scanner"`
}

// VulnerabilitySummary provides a summary of scan findings
type VulnerabilitySummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
}

// ToolExecution represents enhanced execution tracking
type ToolExecution struct {
	Tool       string           `json:"tool"`
	StartTime  time.Time        `json:"start_time"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	Duration   *time.Duration   `json:"duration,omitempty"`
	Success    bool             `json:"success"`
	DryRun     bool             `json:"dry_run"`
	Error      *types.ToolError `json:"error,omitempty"`
	TokensUsed int              `json:"tokens_used"`
}

// JobInfo represents async job information
type JobInfo struct {
	JobID     string           `json:"job_id"`
	Tool      string           `json:"tool"`
	Status    JobStatus        `json:"status"`
	StartTime time.Time        `json:"start_time"`
	EndTime   *time.Time       `json:"end_time,omitempty"`
	Duration  *time.Duration   `json:"duration,omitempty"`
	Progress  *JobProgress     `json:"progress,omitempty"`
	Result    interface{}      `json:"result,omitempty"`
	Error     *types.ToolError `json:"error,omitempty"`
}

// JobStatus represents the status of an async job
type JobStatus string

const (
	CurrentSchemaVersion = "v1.0.0"

	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// JobProgress represents progress information for long-running jobs
type JobProgress struct {
	Percentage int    `json:"percentage"`
	Message    string `json:"message"`
	Step       int    `json:"step"`
	TotalSteps int    `json:"total_steps"`
}

// NewSessionState creates a new session state with defaults
func NewSessionState(sessionID, workspaceDir string) *SessionState {
	now := time.Now()
	return &SessionState{
		Version:      CurrentSchemaVersion,
		ID:           sessionID, // Set ID same as SessionID for compatibility
		SessionID:    sessionID,
		WorkspaceDir: workspaceDir,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastAccessed: now,
		ExpiresAt:    now.Add(24 * time.Hour),
		Status:       SessionStatusActive, // Set default status
		RepoAnalysis: make(map[string]interface{}),
		K8sManifests: make(map[string]types.K8sManifest),
		ActiveJobs:   make(map[string]JobInfo),
		BuildLogs:    make([]string, 0),
		DeployLogs:   make([]string, 0),
		StageHistory: make([]ToolExecution, 0),
		MaxDiskUsage: 1024 * 1024 * 1024,
		Metadata:     make(map[string]interface{}),
		Labels:       make([]string, 0),
		K8sLabels:    make(map[string]string),
	}
}

// NewSessionStateWithTTL creates a new session state with a specific TTL
func NewSessionStateWithTTL(sessionID, workspaceDir string, ttl time.Duration) *SessionState {
	state := NewSessionState(sessionID, workspaceDir)
	state.ExpiresAt = state.CreatedAt.Add(ttl)
	return state
}

// UpdateLastAccessed updates the last accessed time
func (s *SessionState) UpdateLastAccessed() {
	s.LastAccessed = time.Now()
}

// AddToolExecution adds a tool execution to the history
func (s *SessionState) AddToolExecution(execution ToolExecution) {
	s.StageHistory = append(s.StageHistory, execution)
	s.UpdateLastAccessed()
}

// SetError sets the last error for the session
func (s *SessionState) SetError(err *types.ToolError) {
	s.LastError = err
	s.UpdateLastAccessed()
}

// AddJob adds an active job to the session
func (s *SessionState) AddJob(jobInfo JobInfo) {
	s.ActiveJobs[jobInfo.JobID] = jobInfo
	s.UpdateLastAccessed()
}

// UpdateJob updates an existing job
func (s *SessionState) UpdateJob(jobID string, updater func(*JobInfo)) {
	if job, exists := s.ActiveJobs[jobID]; exists {
		updater(&job)
		s.ActiveJobs[jobID] = job
		s.UpdateLastAccessed()
	}
}

// RemoveJob removes a completed job
func (s *SessionState) RemoveJob(jobID string) {
	delete(s.ActiveJobs, jobID)
	s.UpdateLastAccessed()
}

// GetActiveJobCount returns the number of active jobs
func (s *SessionState) GetActiveJobCount() int {
	count := 0
	for _, job := range s.ActiveJobs {
		if job.Status == JobStatusRunning || job.Status == JobStatusPending {
			count++
		}
	}
	return count
}

// IsExpired checks if the session has expired based on ExpiresAt
func (s *SessionState) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// UpdateDiskUsage updates the disk usage for the session
func (s *SessionState) UpdateDiskUsage(bytes int64) {
	s.DiskUsage = bytes
	s.UpdateLastAccessed()
}

// HasExceededDiskQuota checks if the session has exceeded its disk quota
func (s *SessionState) HasExceededDiskQuota() bool {
	return s.DiskUsage > s.MaxDiskUsage
}

// AddLabel adds a label to the session if it doesn't already exist
func (s *SessionState) AddLabel(label string) {
	if !s.HasLabel(label) {
		s.Labels = append(s.Labels, label)
		s.UpdateLastAccessed()
	}
}

// RemoveLabel removes a label from the session
func (s *SessionState) RemoveLabel(label string) {
	for i, l := range s.Labels {
		if l == label {
			s.Labels = append(s.Labels[:i], s.Labels[i+1:]...)
			s.UpdateLastAccessed()
			break
		}
	}
}

// HasLabel checks if the session has a specific label
func (s *SessionState) HasLabel(label string) bool {
	for _, l := range s.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// GetLabels returns a copy of the session labels
func (s *SessionState) GetLabels() []string {
	labels := make([]string, len(s.Labels))
	copy(labels, s.Labels)
	return labels
}

// SetLabels replaces all labels with the provided set
func (s *SessionState) SetLabels(labels []string) {
	s.Labels = make([]string, len(labels))
	copy(s.Labels, labels)
	s.UpdateLastAccessed()
}

// AddK8sLabel adds a Kubernetes label to be applied to generated manifests
func (s *SessionState) AddK8sLabel(key, value string) {
	if s.K8sLabels == nil {
		s.K8sLabels = make(map[string]string)
	}
	s.K8sLabels[key] = value
	s.UpdateLastAccessed()
}

// RemoveK8sLabel removes a Kubernetes label
func (s *SessionState) RemoveK8sLabel(key string) {
	if s.K8sLabels != nil {
		delete(s.K8sLabels, key)
		s.UpdateLastAccessed()
	}
}

// GetK8sLabels returns a copy of the Kubernetes labels
func (s *SessionState) GetK8sLabels() map[string]string {
	if s.K8sLabels == nil {
		return make(map[string]string)
	}
	labels := make(map[string]string)
	for k, v := range s.K8sLabels {
		labels[k] = v
	}
	return labels
}

// SetK8sLabels replaces all Kubernetes labels with the provided set
func (s *SessionState) SetK8sLabels(labels map[string]string) {
	s.K8sLabels = make(map[string]string)
	for k, v := range labels {
		s.K8sLabels[k] = v
	}
	s.UpdateLastAccessed()
}

// GetSummary returns a summary of the session for listing
func (s *SessionState) GetSummary() SessionSummary {
	status := "active"
	if s.IsExpired() {
		status = "expired"
	}
	if s.HasExceededDiskQuota() {
		status = "quota_exceeded"
	}

	return SessionSummary{
		ID:        s.SessionID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.LastAccessed,
		Status:    status,
		Labels:    s.Labels,
	}
}

// DeriveNextStage maps completed tools to their next logical stage
func DeriveNextStage(completedTool string) string {
	stageMap := map[string]string{
		"analyze_repository":  "analysis_complete",
		"generate_dockerfile": "dockerfile_ready",
		"build_image":         "image_built",
		"push_image":          "image_pushed",
		"generate_manifests":  "manifests_ready",
		"deploy_kubernetes":   "deployed",
	}
	if nextStage, exists := stageMap[completedTool]; exists {
		return nextStage
	}
	return "unknown"
}

// ToCoreSessionState converts internal SessionState to core.SessionState
func (s *SessionState) ToCoreSessionState() *core.SessionState {
	return &core.SessionState{
		SessionID:    s.SessionID,
		CreatedAt:    s.CreatedAt,
		ExpiresAt:    s.ExpiresAt,
		UpdatedAt:    s.LastAccessed,
		WorkspaceDir: s.WorkspaceDir,
		// Map other commonly used fields
		RepoURL:             s.RepoURL,
		RepositoryAnalyzed:  len(s.RepoAnalysis) > 0,
		DockerfileGenerated: len(s.Dockerfile.Content) > 0,
		DockerfilePath:      s.Dockerfile.Path,
		ImageBuilt:          s.Dockerfile.Built,
		ImageRef:            s.ImageRef.Repository,
	}
}
