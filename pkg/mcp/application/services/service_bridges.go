package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/security"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// WorkflowExecutorStub provides a stub implementation
type WorkflowExecutorStub struct {
	logger *slog.Logger
}

func NewWorkflowExecutorStub(logger *slog.Logger) WorkflowExecutor {
	return &WorkflowExecutorStub{logger: logger}
}

func (w *WorkflowExecutorStub) ExecuteWorkflow(_ context.Context, workflow *api.Workflow) (*api.WorkflowResult, error) {
	w.logger.Info("Executing workflow", "workflow_id", workflow.ID)
	return &api.WorkflowResult{
		WorkflowID:   workflow.ID,
		Success:      true,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Duration:     time.Minute * 2,
		TotalSteps:   5,
		SuccessSteps: 5,
		FailedSteps:  0,
	}, nil
}

func (w *WorkflowExecutorStub) ExecuteStep(_ context.Context, step *api.WorkflowStep) (*api.StepResult, error) {
	w.logger.Info("Executing workflow step", "step_id", step.ID)
	return &api.StepResult{
		StepID:  step.ID,
		Success: true,
		Output:  map[string]interface{}{"result": "success"},
	}, nil
}

func (w *WorkflowExecutorStub) ValidateWorkflow(ctx context.Context, workflow *api.Workflow) error {
	w.logger.Info("Validating workflow", "workflow_id", workflow.ID)
	return nil
}

// ConversationServiceStub provides a stub implementation
type ConversationServiceStub struct {
	logger *slog.Logger
}

func NewConversationServiceStub(logger *slog.Logger) ConversationService {
	return &ConversationServiceStub{logger: logger}
}

func (c *ConversationServiceStub) ProcessMessage(_ context.Context, sessionID, message string) (*ConversationResponse, error) {
	c.logger.Info("Processing message", "session_id", sessionID, "message", message)
	return &ConversationResponse{
		SessionID:     sessionID,
		Message:       "I understand your request. How can I help you further?",
		Stage:         domaintypes.StageAnalysis,
		Status:        "active",
		RequiresInput: false,
	}, nil
}

func (c *ConversationServiceStub) GetConversationState(ctx context.Context, sessionID string) (*ConversationState, error) {
	c.logger.Info("Getting conversation state", "session_id", sessionID)
	return &ConversationState{
		SessionID:    sessionID,
		CurrentStage: domaintypes.StageAnalysis,
		LastActivity: time.Now(),
	}, nil
}

func (c *ConversationServiceStub) UpdateConversationStage(ctx context.Context, sessionID string, stage domaintypes.ConversationStage) error {
	c.logger.Info("Updating conversation stage", "session_id", sessionID, "stage", stage)
	return nil
}

func (c *ConversationServiceStub) GetConversationHistory(ctx context.Context, sessionID string, limit int) ([]ConversationTurn, error) {
	c.logger.Info("Getting conversation history", "session_id", sessionID, "limit", limit)
	return []ConversationTurn{
		{
			ID:        "turn-1",
			Timestamp: time.Now(),
			Role:      "user",
			Content:   "Hello",
			Stage:     domaintypes.StageAnalysis,
		},
	}, nil
}

func (c *ConversationServiceStub) ClearConversationContext(ctx context.Context, sessionID string) error {
	c.logger.Info("Clearing conversation context", "session_id", sessionID)
	return nil
}

// PromptServiceStub provides a stub implementation
type PromptServiceStub struct {
	logger *slog.Logger
}

func NewPromptServiceStub(logger *slog.Logger) PromptService {
	return &PromptServiceStub{logger: logger}
}

func (p *PromptServiceStub) BuildPrompt(ctx context.Context, stage domaintypes.ConversationStage, _ map[string]interface{}) (string, error) {
	p.logger.Info("Building prompt", "stage", stage)
	return fmt.Sprintf("You are in stage %s. Please provide assistance.", stage), nil
}

func (p *PromptServiceStub) ProcessPromptResponse(ctx context.Context, response string, _ *ConversationState) error {
	p.logger.Info("Processing prompt response", "response_length", len(response))
	return nil
}

func (p *PromptServiceStub) DetectWorkflowIntent(ctx context.Context, message string) (*WorkflowIntent, error) {
	p.logger.Info("Detecting workflow intent", "message", message)
	return &WorkflowIntent{
		Detected:   true,
		Workflow:   "containerization",
		Parameters: map[string]interface{}{"action": "analyze"},
	}, nil
}

func (p *PromptServiceStub) ShouldAutoAdvance(ctx context.Context, state *ConversationState) (bool, *AutoAdvanceConfig) {
	p.logger.Info("Checking auto-advance", "session_id", state.SessionID)
	return false, &AutoAdvanceConfig{
		Enabled: false,
		Delay:   0,
	}
}

// BuildExecutorFromDockerService adapter removed - docker.Service directly implements BuildExecutor interface

// ErrorReporterFromLogger adapts logger to ErrorReporter interface
type ErrorReporterFromLogger struct {
	logger *slog.Logger
}

func NewErrorReporterFromLogger(logger *slog.Logger) ErrorReporter {
	return &ErrorReporterFromLogger{logger: logger}
}

func (e *ErrorReporterFromLogger) ReportError(_ context.Context, err error, context map[string]interface{}) {
	e.logger.Error("Error reported", "error", err, "context", context)
}

func (e *ErrorReporterFromLogger) GetErrorStats(ctx context.Context) ErrorStats {
	return ErrorStats{
		TotalErrors:  0,
		ErrorsByType: make(map[string]int64),
		RecentErrors: []ErrorEntry{},
		RecoveryRate: 1.0,
	}
}

func (e *ErrorReporterFromLogger) SuggestFix(ctx context.Context, _ error) []string {
	return []string{"Check logs for more details"}
}

// ToolRegistryStub provides a simple stub implementation
type ToolRegistryStub struct {
	logger *slog.Logger
}

func NewToolRegistryStub(logger *slog.Logger) ToolRegistry {
	return &ToolRegistryStub{logger: logger}
}

func (t *ToolRegistryStub) Register(name string, _ interface{}) error {
	t.logger.Info("Registering tool", "name", name)
	return nil
}

func (t *ToolRegistryStub) Discover(name string) (interface{}, error) {
	return nil, errors.NewError().
		Code(errors.CodeToolNotFound).
		Type(errors.ErrTypeTool).
		Severity(errors.SeverityMedium).
		Messagef("tool not found: %s", name).
		WithLocation().
		Build()
}

func (t *ToolRegistryStub) List() []string {
	return []string{}
}

func (t *ToolRegistryStub) Metadata(name string) (api.ToolMetadata, error) {
	return api.ToolMetadata{}, errors.NewError().
		Code(errors.CodeToolNotFound).
		Type(errors.ErrTypeTool).
		Severity(errors.SeverityMedium).
		Messagef("tool not found: %s", name).
		WithLocation().
		Build()
}

func (t *ToolRegistryStub) SetMetadata(name string, _ api.ToolMetadata) error {
	return errors.NewError().
		Code(errors.CodeToolNotFound).
		Type(errors.ErrTypeTool).
		Severity(errors.SeverityMedium).
		Messagef("tool not found: %s", name).
		WithLocation().
		Build()
}

func (t *ToolRegistryStub) Unregister(name string) error {
	return errors.NewError().
		Code(errors.CodeToolNotFound).
		Type(errors.ErrTypeTool).
		Severity(errors.SeverityMedium).
		Messagef("tool not found: %s", name).
		WithLocation().
		Build()
}

func (t *ToolRegistryStub) Execute(_ context.Context, name string, _ api.ToolInput) (api.ToolOutput, error) {
	return api.ToolOutput{}, errors.NewError().
		Code(errors.CodeToolNotFound).
		Type(errors.ErrTypeTool).
		Severity(errors.SeverityMedium).
		Messagef("tool not found: %s", name).
		WithLocation().
		Build()
}

func (t *ToolRegistryStub) Close() error {
	t.logger.Info("Closing ToolRegistry")
	return nil
}

// PipelineServiceStub provides a stub implementation
type PipelineServiceStub struct {
	logger *slog.Logger
}

func NewPipelineServiceStub(logger *slog.Logger) PipelineService {
	return &PipelineServiceStub{logger: logger}
}

func (p *PipelineServiceStub) Start(ctx context.Context) error {
	p.logger.Info("Starting pipeline service")
	return nil
}

func (p *PipelineServiceStub) Stop(ctx context.Context) error {
	p.logger.Info("Stopping pipeline service")
	return nil
}

func (p *PipelineServiceStub) IsRunning() bool {
	return true
}

func (p *PipelineServiceStub) CancelJob(ctx context.Context, jobID string) error {
	p.logger.Info("Canceling job", "job_id", jobID)
	return nil
}

// SessionStoreStub provides a stub implementation
type SessionStoreStub struct {
	logger *slog.Logger
}

func NewSessionStoreStub(logger *slog.Logger) SessionStore {
	return &SessionStoreStub{logger: logger}
}

func (s *SessionStoreStub) Create(_ context.Context, session *api.Session) error {
	s.logger.Info("Creating session", "session_id", session.ID)
	return nil
}

func (s *SessionStoreStub) Get(_ context.Context, sessionID string) (*api.Session, error) {
	s.logger.Info("Getting session", "session_id", sessionID)
	return &api.Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  map[string]interface{}{"status": "active"},
		State:     map[string]interface{}{"active": true},
	}, nil
}

func (s *SessionStoreStub) Update(_ context.Context, session *api.Session) error {
	s.logger.Info("Updating session", "session_id", session.ID)
	return nil
}

func (s *SessionStoreStub) Delete(_ context.Context, sessionID string) error {
	s.logger.Info("Deleting session", "session_id", sessionID)
	return nil
}

func (s *SessionStoreStub) List(_ context.Context) ([]*api.Session, error) {
	s.logger.Info("Listing sessions")
	return []*api.Session{
		{
			ID:        "session-1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  map[string]interface{}{"status": "active"},
			State:     map[string]interface{}{"active": true},
		},
	}, nil
}

// StateManagerStub provides a stub implementation
type StateManagerStub struct {
	logger *slog.Logger
}

func NewStateManagerStub(logger *slog.Logger) StateManager {
	return &StateManagerStub{logger: logger}
}

func (s *StateManagerStub) SaveState(_ context.Context, key string, _ interface{}) error {
	s.logger.Info("Saving state", "key", key)
	return nil
}

func (s *StateManagerStub) GetState(_ context.Context, key string, _ interface{}) error {
	s.logger.Info("Getting state", "key", key)
	return nil
}

func (s *StateManagerStub) UpdateState(_ context.Context, key string, tool string, _ interface{}) error {
	s.logger.Info("Updating state", "key", key, "tool", tool)
	return nil
}

func (s *StateManagerStub) DeleteState(_ context.Context, key string) error {
	s.logger.Info("Deleting state", "key", key)
	return nil
}

// SessionStateStub provides a stub implementation
type SessionStateStub struct {
	logger *slog.Logger
}

func NewSessionStateStub(logger *slog.Logger) SessionState {
	return &SessionStateStub{logger: logger}
}

func (s *SessionStateStub) SaveState(_ context.Context, sessionID string, _ map[string]interface{}) error {
	s.logger.Info("Saving session state", "session_id", sessionID)
	return nil
}

func (s *SessionStateStub) GetState(_ context.Context, sessionID string) (map[string]interface{}, error) {
	s.logger.Info("Getting session state", "session_id", sessionID)
	return map[string]interface{}{"status": "active"}, nil
}

func (s *SessionStateStub) CreateCheckpoint(_ context.Context, sessionID string, name string) error {
	s.logger.Info("Creating checkpoint", "session_id", sessionID, "name", name)
	return nil
}

func (s *SessionStateStub) RestoreCheckpoint(_ context.Context, sessionID string, name string) error {
	s.logger.Info("Restoring checkpoint", "session_id", sessionID, "name", name)
	return nil
}

func (s *SessionStateStub) ListCheckpoints(_ context.Context, sessionID string) ([]string, error) {
	s.logger.Info("Listing checkpoints", "session_id", sessionID)
	return []string{"checkpoint-1"}, nil
}

func (s *SessionStateStub) GetWorkspaceDir(_ context.Context, sessionID string) (string, error) {
	s.logger.Info("Getting workspace dir", "session_id", sessionID)
	return "/tmp/workspace", nil
}

func (s *SessionStateStub) SetWorkspaceDir(_ context.Context, sessionID string, dir string) error {
	s.logger.Info("Setting workspace dir", "session_id", sessionID, "dir", dir)
	return nil
}

func (s *SessionStateStub) GetSessionMetadata(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	s.logger.Info("Getting session metadata", "session_id", sessionID)
	return map[string]interface{}{"created": time.Now()}, nil
}

func (s *SessionStateStub) UpdateSessionData(ctx context.Context, sessionID string, _ map[string]interface{}) error {
	s.logger.Info("Updating session data", "session_id", sessionID)
	return nil
}

// ConfigValidatorStub provides a stub implementation
type ConfigValidatorStub struct {
	logger *slog.Logger
}

func NewConfigValidatorStub(logger *slog.Logger) ConfigValidator {
	return &ConfigValidatorStub{logger: logger}
}

func (v *ConfigValidatorStub) ValidateDockerfile(ctx context.Context, _ string) (*ValidationResult, error) {
	v.logger.Info("Validating Dockerfile")
	return &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Score:    100,
	}, nil
}

func (v *ConfigValidatorStub) ValidateManifest(ctx context.Context, _ string) (*ValidationResult, error) {
	v.logger.Info("Validating manifest")
	return &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Score:    100,
	}, nil
}

func (v *ConfigValidatorStub) ValidateConfig(ctx context.Context, _ map[string]interface{}) (*ValidationResult, error) {
	v.logger.Info("Validating configuration")
	return &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Score:    100,
	}, nil
}

// AnalyzerAdapter adapts the core analyzer to accept context
type AnalyzerAdapter struct {
	analyzer *analysis.RepositoryAnalyzer
}

func NewAnalyzerAdapter(analyzer *analysis.RepositoryAnalyzer) Analyzer {
	return &AnalyzerAdapter{analyzer: analyzer}
}

func (a *AnalyzerAdapter) AnalyzeRepository(ctx context.Context, repoPath string) (*analysis.AnalysisResult, error) {
	// Check context first
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.analyzer.AnalyzeRepository(repoPath)
}

// ScannerAdapter adapts the core scanner to accept context
type ScannerAdapter struct {
	scanner security.Service
}

func NewScannerAdapter(scanner security.Service) Scanner {
	return &ScannerAdapter{scanner: scanner}
}

func (s *ScannerAdapter) ScanImage(ctx context.Context, image string, options security.ScanOptionsService) (*security.ScanResult, error) {
	return s.scanner.ScanImage(ctx, image, options)
}

func (s *ScannerAdapter) ScanDirectory(ctx context.Context, path string, options security.ScanOptionsService) (*security.ScanResult, error) {
	return s.scanner.ScanDirectory(ctx, path, options)
}

func (s *ScannerAdapter) GetAvailableScanners(ctx context.Context) []string {
	// Check context first
	if err := ctx.Err(); err != nil {
		return nil
	}
	return s.scanner.GetAvailableScanners()
}

// PersistenceStub provides a stub implementation
type PersistenceStub struct {
	logger *slog.Logger
}

func NewPersistenceStub(logger *slog.Logger) Persistence {
	return &PersistenceStub{logger: logger}
}

func (p *PersistenceStub) Put(_ context.Context, bucket string, key string, _ interface{}) error {
	p.logger.Info("Storing data", "bucket", bucket, "key", key)
	return nil
}

func (p *PersistenceStub) Get(_ context.Context, bucket string, key string, _ interface{}) error {
	p.logger.Info("Getting data", "bucket", bucket, "key", key)
	return nil
}

func (p *PersistenceStub) Delete(_ context.Context, bucket string, key string) error {
	p.logger.Info("Deleting data", "bucket", bucket, "key", key)
	return nil
}

func (p *PersistenceStub) List(_ context.Context, bucket string) (map[string]interface{}, error) {
	p.logger.Info("Listing data", "bucket", bucket)
	return map[string]interface{}{}, nil
}

func (p *PersistenceStub) Close() error {
	p.logger.Info("Closing persistence")
	return nil
}

// KnowledgeBaseStub provides a stub implementation
type KnowledgeBaseStub struct {
	logger *slog.Logger
}

func NewKnowledgeBaseStub(logger *slog.Logger) KnowledgeBase {
	return &KnowledgeBaseStub{logger: logger}
}

func (k *KnowledgeBaseStub) Store(_ context.Context, key string, _ interface{}) error {
	k.logger.Info("Storing knowledge", "key", key)
	return nil
}

func (k *KnowledgeBaseStub) Retrieve(_ context.Context, key string) (interface{}, error) {
	k.logger.Info("Retrieving knowledge", "key", key)
	return map[string]interface{}{"result": "example"}, nil
}

func (k *KnowledgeBaseStub) Search(_ context.Context, query string) ([]interface{}, error) {
	k.logger.Info("Searching knowledge base", "query", query)
	return []interface{}{map[string]interface{}{"result": "example"}}, nil
}

// K8sClientStub provides a stub implementation
type K8sClientStub struct {
	logger *slog.Logger
}

func NewK8sClientStub(logger *slog.Logger) K8sClient {
	return &K8sClientStub{logger: logger}
}

func (k *K8sClientStub) Apply(_ context.Context, _ string, namespace string) error {
	k.logger.Info("Applying manifests to Kubernetes", "namespace", namespace)
	return nil
}

func (k *K8sClientStub) Delete(_ context.Context, _ string, namespace string) error {
	k.logger.Info("Deleting from Kubernetes", "namespace", namespace)
	return nil
}

func (k *K8sClientStub) GetStatus(_ context.Context, resource, name, namespace string) (interface{}, error) {
	k.logger.Info("Getting resource status", "resource", resource, "name", name, "namespace", namespace)
	return map[string]interface{}{"status": "running"}, nil
}

// FileAccessServiceStub provides a stub implementation for testing
type FileAccessServiceStub struct {
	logger *slog.Logger
}

func NewFileAccessServiceStub(logger *slog.Logger) FileAccessService {
	return &FileAccessServiceStub{logger: logger}
}

func (f *FileAccessServiceStub) ReadFile(ctx context.Context, sessionID, path string) (string, error) {
	f.logger.Info("Reading file", "session_id", sessionID, "path", path)
	return "# Sample file content\npackage main\n\nfunc main() {\n    println(\"Hello, World!\")\n}", nil
}

func (f *FileAccessServiceStub) ListDirectory(ctx context.Context, sessionID, path string) ([]FileInfo, error) {
	f.logger.Info("Listing directory", "session_id", sessionID, "path", path)
	return []FileInfo{
		{
			Name:    "main.go",
			Path:    "main.go",
			Size:    1024,
			ModTime: time.Now(),
			IsDir:   false,
			Mode:    "-rw-r--r--",
		},
		{
			Name:    "go.mod",
			Path:    "go.mod",
			Size:    256,
			ModTime: time.Now(),
			IsDir:   false,
			Mode:    "-rw-r--r--",
		},
	}, nil
}

func (f *FileAccessServiceStub) FileExists(ctx context.Context, sessionID, path string) (bool, error) {
	f.logger.Info("Checking file existence", "session_id", sessionID, "path", path)
	return true, nil
}

func (f *FileAccessServiceStub) GetFileTree(ctx context.Context, sessionID, rootPath string) (string, error) {
	f.logger.Info("Getting file tree", "session_id", sessionID, "root_path", rootPath)
	return ".\n├── main.go (1024 bytes)\n├── go.mod (256 bytes)\n└── README.md (512 bytes)", nil
}

func (f *FileAccessServiceStub) ReadFileWithMetadata(ctx context.Context, sessionID, path string) (*FileContent, error) {
	f.logger.Info("Reading file with metadata", "session_id", sessionID, "path", path)
	content := "# Sample file content\npackage main\n\nfunc main() {\n    println(\"Hello, World!\")\n}"
	return &FileContent{
		Path:     path,
		Content:  content,
		Size:     int64(len(content)),
		ModTime:  time.Now(),
		Encoding: "UTF-8",
		Lines:    5,
	}, nil
}

func (f *FileAccessServiceStub) SearchFiles(ctx context.Context, sessionID, pattern string) ([]FileInfo, error) {
	f.logger.Info("Searching files", "session_id", sessionID, "pattern", pattern)
	return []FileInfo{
		{
			Name:    "main.go",
			Path:    "main.go",
			Size:    1024,
			ModTime: time.Now(),
			IsDir:   false,
			Mode:    "-rw-r--r--",
		},
	}, nil
}
