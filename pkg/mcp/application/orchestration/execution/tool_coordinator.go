package execution

import (
	"context"
	"sync"
	"time"
)

// ToolCapability describes what a tool can do and its requirements
type ToolCapability struct {
	ToolName        string                 `json:"tool_name"`
	InputTypes      []string               `json:"input_types"`
	OutputTypes     []string               `json:"output_types"`
	RequiredContext []string               `json:"required_context"`
	ProvidedContext []string               `json:"provided_context"`
	Dependencies    []string               `json:"dependencies"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	Constraints     ToolConstraints        `json:"constraints"`
}

// ToolConstraints defines operational constraints for a tool
type ToolConstraints struct {
	MaxConcurrency  int           `json:"max_concurrency"`
	Timeout         time.Duration `json:"timeout"`
	RequiredMemory  int64         `json:"required_memory"`
	RequiredCPU     float64       `json:"required_cpu"`
	AllowedFailures int           `json:"allowed_failures"`
	CooldownPeriod  time.Duration `json:"cooldown_period"`
	ExclusiveWith   []string      `json:"exclusive_with"`
}

// CoordinationRule defines how tools should coordinate
type CoordinationRule struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	SourceTool    string                 `json:"source_tool"`
	TargetTool    string                 `json:"target_tool"`
	TriggerEvent  string                 `json:"trigger_event"`
	Conditions    []RuleCondition        `json:"conditions"`
	DataTransform DataTransformFunction  `json:"-"`
	Priority      int                    `json:"priority"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// RuleCondition defines when a coordination rule should apply
type RuleCondition struct {
	Type     string      `json:"type"` // "output_contains", "context_exists", "metric_threshold"
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // "equals", "contains", "greater_than", "less_than"
	Value    interface{} `json:"value"`
}

// DataTransformFunction transforms data between tools
type DataTransformFunction func(sourceData interface{}) (interface{}, error)

// Graph manages tool dependencies
type Graph struct {
	nodes map[string]*DependencyNode
	edges map[string][]string
	mutex sync.RWMutex
}

// DependencyNode represents a tool in the dependency graph
type DependencyNode struct {
	ToolName     string
	Dependencies []string
	Dependents   []string
	Status       string // "ready", "waiting", "running", "completed", "failed"
	LastRun      time.Time
	RunCount     int
}

// CommunicationBridge interface removed - use types.InternalStreamTool instead

// ToolMessage represents a message between tools
type ToolMessage struct {
	ID          string                 `json:"id"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Type        string                 `json:"type"`
	Payload     interface{}            `json:"payload"`
	Context     map[string]interface{} `json:"context"`
	Timestamp   time.Time              `json:"timestamp"`
	ReplyTo     string                 `json:"reply_to,omitempty"`
	Correlation string                 `json:"correlation,omitempty"`
}

// MessageHandler processes incoming messages
type MessageHandler func(ctx context.Context, message ToolMessage) error

// ActiveCoordination tracks an ongoing coordination
type ActiveCoordination struct {
	ID             string
	SourceTool     string
	TargetTool     string
	StartTime      time.Time
	Status         string
	Messages       []ToolMessage
	Context        map[string]interface{}
	CompletionChan chan CoordinationResult
}

// CoordinationResult represents the outcome of a coordination
type CoordinationResult struct {
	Success  bool
	Error    error
	Duration time.Duration
	Output   interface{}
	Metadata map[string]interface{}
}

// CoordinationMetrics tracks coordination performance
type CoordinationMetrics struct {
	TotalCoordinations      int64
	SuccessfulCoordinations int64
	FailedCoordinations     int64
	AverageLatency          time.Duration
	ToolPairMetrics         map[string]*ToolPairMetric
	mutex                   sync.RWMutex
}

// ToolPairMetric tracks metrics for a specific tool pair
type ToolPairMetric struct {
	SourceTool string
	TargetTool string
	Count      int64
	Successes  int64
	Failures   int64
	TotalTime  time.Duration
}

// Tool coordinator methods removed - functionality deprecated
