package core

import (
	"context"
	"time"
)

// AnalysisService consolidates all analysis operations
// Replaces: Analyzer, RepositoryAnalyzer, AIAnalyzer, ProgressReporter
type AnalysisService interface {
	AnalyzeRepository(ctx context.Context, path string, callback ProgressCallback) (*RepositoryAnalysis, error)
	AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error)
	GetAnalysisProgress(ctx context.Context, analysisID string) (*AnalysisProgress, error)
}

// ProgressCallback replaces ProgressReporter interface
type ProgressCallback func(stage string, progress int, total int)

// ServerService consolidates server operations
// Replaces: Server, Transport, ToolOrchestrator, RequestHandler, TypedPipelineOperations
type ServerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	HandleRequest(ctx context.Context, request *Request) (*Response, error)
	RegisterTool(tool Tool) error
	ExecuteOperation(ctx context.Context, operation Operation) (*OperationResult, error)
}

// StateService consolidates state management
// Replaces: StateProvider, StateObserver, StateValidator, StateMapping
// Replaces: ContextProvider, ContextEnricher
type StateService interface {
	GetState(ctx context.Context, key string) (interface{}, error)
	SetState(ctx context.Context, key string, value interface{}) error
	ValidateState(ctx context.Context, state interface{}) error
	ObserveChanges(ctx context.Context, callback StateChangeCallback)
	ProvideContext(ctx context.Context, request *ContextRequest) (*ContextResponse, error)
	EnrichContext(ctx context.Context, context *ContextData) (*ContextData, error)
}

// StateChangeCallback replaces StateObserver interface
type StateChangeCallback func(key string, oldValue, newValue interface{})

// WorkflowService consolidates workflow operations
// Replaces: WorkflowSessionInterface, ConversationStateInterface, ConversationEntryInterface
// Replaces: DecisionInterface, ArtifactInterface
type WorkflowService interface {
	CreateSession(ctx context.Context, config SessionConfig) (*Session, error)
	ExecuteWorkflow(ctx context.Context, sessionID string, workflow *Workflow) (*WorkflowResult, error)
	GetConversationHistory(ctx context.Context, sessionID string) ([]*ConversationEntry, error)
	MakeDecision(ctx context.Context, decisionRequest *DecisionRequest) (*DecisionResult, error)
	CreateArtifact(ctx context.Context, artifactData *ArtifactData) (*Artifact, error)
}

// OperationService consolidates operation handling
// Replaces: ConsolidatedFixableOperation
type OperationService interface {
	ExecuteFixableOperation(ctx context.Context, operation *FixableOperation) (*OperationResult, error)
	ValidateOperation(ctx context.Context, operation *FixableOperation) error
	GetOperationStatus(ctx context.Context, operationID string) (*OperationStatus, error)
}

// TransportService handles communication
// Replaces: LLMTransport
type TransportService interface {
	SendMessage(ctx context.Context, message *Message) (*Response, error)
	ReceiveMessage(ctx context.Context) (*Message, error)
	GetTransportMetrics() (*TransportMetrics, error)
}

// ConstraintService handles validation constraints
// Replaces: Constraint
type ConstraintService interface {
	ValidateConstraint(ctx context.Context, value interface{}, constraint *ConstraintDefinition) error
	GetConstraintDefinition(name string) (*ConstraintDefinition, error)
}

// Supporting types for the unified interfaces
type RepositoryAnalysis struct {
	Language     string
	Framework    string
	Dependencies []string
	Structure    map[string]interface{}
	Metrics      map[string]float64
	Issues       []AnalysisIssue
	Suggestions  []string
}

type AIAnalysis struct {
	Summary         string
	Recommendations []string
	Confidence      float64
	Analysis        map[string]interface{}
	Metadata        map[string]interface{}
}

type AnalysisProgress struct {
	ID        string
	Stage     string
	Progress  int
	Total     int
	Complete  bool
	StartTime time.Time
	Duration  time.Duration
	Messages  []string
}

type AnalysisIssue struct {
	Type       string
	Severity   string
	Message    string
	File       string
	Line       int
	Column     int
	Suggestion string
}

type Request struct {
	ID        string
	Type      string
	Data      map[string]interface{}
	Timestamp time.Time
	Metadata  map[string]interface{}
}

type Response struct {
	ID        string
	RequestID string
	Data      interface{}
	Error     string
	Timestamp time.Time
	Metadata  map[string]interface{}
}

type Tool struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, input map[string]interface{}) (interface{}, error)
	Schema      map[string]interface{}
}

type Operation struct {
	ID         string
	Type       string
	Parameters map[string]interface{}
	Timeout    time.Duration
	Metadata   map[string]interface{}
}

type OperationResult struct {
	ID       string
	Success  bool
	Data     interface{}
	Error    string
	Duration time.Duration
	Metadata map[string]interface{}
}

type ContextRequest struct {
	SessionID string
	Type      string
	Data      map[string]interface{}
	Metadata  map[string]interface{}
}

type ContextResponse struct {
	Context  *ContextData
	Metadata map[string]interface{}
}

type ContextData struct {
	Values    map[string]interface{}
	Metadata  map[string]interface{}
	Timestamp time.Time
}

type SessionConfig struct {
	ID       string
	Type     string
	Settings map[string]interface{}
	Timeout  time.Duration
	Metadata map[string]interface{}
}

type Session struct {
	ID        string
	Config    SessionConfig
	State     map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
	Metadata  map[string]interface{}
}

type Workflow struct {
	ID       string
	Steps    []WorkflowStep
	Settings map[string]interface{}
	Metadata map[string]interface{}
}

type WorkflowStep struct {
	ID         string
	Type       string
	Parameters map[string]interface{}
	Conditions []string
	Metadata   map[string]interface{}
}

type WorkflowResult struct {
	ID       string
	Success  bool
	Results  []StepResult
	Duration time.Duration
	Error    string
	Metadata map[string]interface{}
}

type StepResult struct {
	StepID   string
	Success  bool
	Data     interface{}
	Error    string
	Duration time.Duration
	Metadata map[string]interface{}
}

type ConversationEntry struct {
	ID        string
	Type      string
	Content   string
	Timestamp time.Time
	Author    string
	Metadata  map[string]interface{}
}

type DecisionRequest struct {
	ID       string
	Context  map[string]interface{}
	Options  []DecisionOption
	Criteria []string
	Metadata map[string]interface{}
}

type DecisionOption struct {
	ID          string
	Name        string
	Description string
	Parameters  map[string]interface{}
	Score       float64
	Metadata    map[string]interface{}
}

type DecisionResult struct {
	ID           string
	SelectedID   string
	Confidence   float64
	Reasoning    string
	Alternatives []DecisionOption
	Metadata     map[string]interface{}
}

type ArtifactData struct {
	Type      string
	Content   []byte
	Metadata  map[string]interface{}
	Timestamp time.Time
}

type Artifact struct {
	ID        string
	Type      string
	Content   []byte
	Hash      string
	Size      int64
	CreatedAt time.Time
	Metadata  map[string]interface{}
}

type FixableOperation struct {
	ID         string
	Type       string
	Target     string
	Fix        string
	Parameters map[string]interface{}
	Metadata   map[string]interface{}
}

type OperationStatus struct {
	ID        string
	Status    string
	Progress  int
	Total     int
	StartTime time.Time
	Duration  time.Duration
	Error     string
	Metadata  map[string]interface{}
}

type Message struct {
	ID        string
	Type      string
	Content   string
	Timestamp time.Time
	Metadata  map[string]interface{}
}

type TransportMetrics struct {
	MessagesSent     int64
	MessagesReceived int64
	Errors           int64
	AverageLatency   time.Duration
	LastActivity     time.Time
	Metadata         map[string]interface{}
}

type ConstraintDefinition struct {
	Name        string
	Type        string
	Parameters  map[string]interface{}
	Validator   func(interface{}) error
	Description string
	Metadata    map[string]interface{}
}
