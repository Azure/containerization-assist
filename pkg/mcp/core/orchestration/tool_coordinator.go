package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	commonUtils "github.com/Azure/container-kit/pkg/commonutils"
	"github.com/rs/zerolog"
)

// ToolCoordinator manages tool-to-tool communication and coordination patterns
type ToolCoordinator struct {
	logger              zerolog.Logger
	toolRegistry        map[string]ToolCapability
	coordinationRules   []CoordinationRule
	dependencyGraph     *ToolDependencyGraph
	communicationBridge CommunicationBridge
	mutex               sync.RWMutex
	activeCoordinations map[string]*ActiveCoordination
	metrics             *CoordinationMetrics
}

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

// ToolDependencyGraph manages tool dependencies
type ToolDependencyGraph struct {
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

// CommunicationBridge handles actual tool-to-tool communication
type CommunicationBridge interface {
	SendMessage(ctx context.Context, from, to string, message ToolMessage) error
	RegisterHandler(toolName string, handler MessageHandler) error
	GetPendingMessages(toolName string) ([]ToolMessage, error)
}

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

// NewToolCoordinator creates a new tool coordinator
func NewToolCoordinator(logger zerolog.Logger, bridge CommunicationBridge) *ToolCoordinator {
	tc := &ToolCoordinator{
		logger:              logger.With().Str("component", "tool_coordinator").Logger(),
		toolRegistry:        make(map[string]ToolCapability),
		coordinationRules:   []CoordinationRule{},
		dependencyGraph:     NewToolDependencyGraph(),
		communicationBridge: bridge,
		activeCoordinations: make(map[string]*ActiveCoordination),
		metrics:             NewCoordinationMetrics(),
	}

	// Register default coordination rules
	tc.registerDefaultRules()

	return tc
}

// RegisterTool registers a tool's capabilities
func (tc *ToolCoordinator) RegisterTool(capability ToolCapability) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	tc.toolRegistry[capability.ToolName] = capability

	// Update dependency graph
	tc.dependencyGraph.AddNode(capability.ToolName, capability.Dependencies)

	tc.logger.Info().
		Str("tool_name", capability.ToolName).
		Strs("input_types", capability.InputTypes).
		Strs("output_types", capability.OutputTypes).
		Msg("Tool registered with coordinator")

	return nil
}

// AddCoordinationRule adds a new coordination rule
func (tc *ToolCoordinator) AddCoordinationRule(rule CoordinationRule) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	// Validate rule
	if _, exists := tc.toolRegistry[rule.SourceTool]; !exists {
		return fmt.Errorf("source tool %s not registered", rule.SourceTool)
	}
	if _, exists := tc.toolRegistry[rule.TargetTool]; !exists {
		return fmt.Errorf("target tool %s not registered", rule.TargetTool)
	}

	tc.coordinationRules = append(tc.coordinationRules, rule)

	tc.logger.Info().
		Str("rule_id", rule.ID).
		Str("source_tool", rule.SourceTool).
		Str("target_tool", rule.TargetTool).
		Msg("Coordination rule added")

	return nil
}

// CoordinateExecution coordinates tool execution based on events
func (tc *ToolCoordinator) CoordinateExecution(ctx context.Context, event ToolEvent) (*CoordinationResult, error) {
	startTime := time.Now()

	// Find applicable rules
	rules := tc.findApplicableRules(event)
	if len(rules) == 0 {
		return &CoordinationResult{
			Success:  true,
			Duration: time.Since(startTime),
			Metadata: map[string]interface{}{"message": "no applicable rules"},
		}, nil
	}

	// Sort by priority
	tc.sortRulesByPriority(rules)

	// Execute coordination for each rule
	var lastResult *CoordinationResult
	for _, rule := range rules {
		result, err := tc.executeCoordination(ctx, event, rule)
		if err != nil {
			tc.logger.Error().
				Err(err).
				Str("rule_id", rule.ID).
				Msg("Coordination execution failed")

			tc.metrics.recordFailure(rule.SourceTool, rule.TargetTool, time.Since(startTime))

			return &CoordinationResult{
				Success:  false,
				Error:    err,
				Duration: time.Since(startTime),
			}, err
		}
		lastResult = result
	}

	tc.metrics.recordSuccess(event.SourceTool, "", time.Since(startTime))

	return lastResult, nil
}

// executeCoordination executes a single coordination
func (tc *ToolCoordinator) executeCoordination(ctx context.Context, event ToolEvent, rule CoordinationRule) (*CoordinationResult, error) {
	coordinationID := tc.generateCoordinationID()

	// Create active coordination
	activeCoord := &ActiveCoordination{
		ID:             coordinationID,
		SourceTool:     rule.SourceTool,
		TargetTool:     rule.TargetTool,
		StartTime:      time.Now(),
		Status:         "running",
		Messages:       []ToolMessage{},
		Context:        event.Context,
		CompletionChan: make(chan CoordinationResult, 1),
	}

	tc.mutex.Lock()
	tc.activeCoordinations[coordinationID] = activeCoord
	tc.mutex.Unlock()

	defer func() {
		tc.mutex.Lock()
		delete(tc.activeCoordinations, coordinationID)
		tc.mutex.Unlock()
	}()

	// Transform data if needed
	payload := event.Data
	if rule.DataTransform != nil {
		transformedData, err := rule.DataTransform(event.Data)
		if err != nil {
			return nil, fmt.Errorf("data transformation failed: %w", err)
		}
		payload = transformedData
	}

	// Create tool message
	message := ToolMessage{
		ID:          tc.generateMessageID(),
		From:        rule.SourceTool,
		To:          rule.TargetTool,
		Type:        "coordination",
		Payload:     payload,
		Context:     event.Context,
		Timestamp:   time.Now(),
		Correlation: coordinationID,
	}

	// Send message through bridge
	if err := tc.communicationBridge.SendMessage(ctx, rule.SourceTool, rule.TargetTool, message); err != nil {
		activeCoord.Status = "failed"
		return nil, fmt.Errorf("failed to send coordination message: %w", err)
	}

	activeCoord.Messages = append(activeCoord.Messages, message)

	// Wait for completion or timeout
	select {
	case result := <-activeCoord.CompletionChan:
		activeCoord.Status = "completed"
		return &result, nil
	case <-ctx.Done():
		activeCoord.Status = "cancelled"
		return nil, ctx.Err()
	case <-time.After(30 * time.Second): // Default timeout
		activeCoord.Status = "timeout"
		return nil, fmt.Errorf("coordination timeout")
	}
}

// findApplicableRules finds rules that apply to an event
func (tc *ToolCoordinator) findApplicableRules(event ToolEvent) []CoordinationRule {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	var applicableRules []CoordinationRule

	for _, rule := range tc.coordinationRules {
		if rule.SourceTool != event.SourceTool {
			continue
		}

		if rule.TriggerEvent != "" && rule.TriggerEvent != event.EventType {
			continue
		}

		// Check conditions
		if tc.evaluateConditions(rule.Conditions, event) {
			applicableRules = append(applicableRules, rule)
		}
	}

	return applicableRules
}

// evaluateConditions evaluates rule conditions
func (tc *ToolCoordinator) evaluateConditions(conditions []RuleCondition, event ToolEvent) bool {
	for _, condition := range conditions {
		if !tc.evaluateCondition(condition, event) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (tc *ToolCoordinator) evaluateCondition(condition RuleCondition, event ToolEvent) bool {
	// Simple implementation - extend as needed
	switch condition.Type {
	case "output_contains":
		// Check if output contains expected value
		if output, ok := event.Data.(map[string]interface{}); ok {
			if value, exists := output[condition.Field]; exists {
				return tc.compareValues(value, condition.Operator, condition.Value)
			}
		}
	case "context_exists":
		// Check if context key exists
		_, exists := event.Context[condition.Field]
		return exists
	case "metric_threshold":
		// Check metric thresholds
		if metrics, ok := event.Context["metrics"].(map[string]interface{}); ok {
			if value, exists := metrics[condition.Field]; exists {
				return tc.compareValues(value, condition.Operator, condition.Value)
			}
		}
	}
	return false
}

// compareValues compares values based on operator
func (tc *ToolCoordinator) compareValues(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "equals":
		return actual == expected
	case "contains":
		if str, ok := actual.(string); ok {
			if substr, ok := expected.(string); ok {
				return commonUtils.Contains(str, substr)
			}
		}
	case "greater_than":
		return tc.compareNumeric(actual, expected, ">")
	case "less_than":
		return tc.compareNumeric(actual, expected, "<")
	}
	return false
}

// compareNumeric compares numeric values
func (tc *ToolCoordinator) compareNumeric(actual, expected interface{}, op string) bool {
	// Convert to float64 for comparison
	var actualNum, expectedNum float64

	switch v := actual.(type) {
	case int:
		actualNum = float64(v)
	case int64:
		actualNum = float64(v)
	case float64:
		actualNum = v
	default:
		return false
	}

	switch v := expected.(type) {
	case int:
		expectedNum = float64(v)
	case int64:
		expectedNum = float64(v)
	case float64:
		expectedNum = v
	default:
		return false
	}

	switch op {
	case ">":
		return actualNum > expectedNum
	case "<":
		return actualNum < expectedNum
	default:
		return false
	}
}

// sortRulesByPriority sorts rules by priority (highest first)
func (tc *ToolCoordinator) sortRulesByPriority(rules []CoordinationRule) {
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority < rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}
}

// registerDefaultRules registers default coordination rules
func (tc *ToolCoordinator) registerDefaultRules() {
	// Build failure -> Analyze
	tc.coordinationRules = append(tc.coordinationRules, CoordinationRule{
		ID:           "build_failure_analyze",
		Name:         "Route build failures to analysis",
		SourceTool:   "build_image",
		TargetTool:   "analyze_repository",
		TriggerEvent: "build_failed",
		Priority:     10,
		Conditions: []RuleCondition{
			{
				Type:     "output_contains",
				Field:    "error_type",
				Operator: "equals",
				Value:    "dockerfile_error",
			},
		},
	})

	// Security scan -> Build update
	tc.coordinationRules = append(tc.coordinationRules, CoordinationRule{
		ID:           "security_vuln_rebuild",
		Name:         "Trigger rebuild on security vulnerabilities",
		SourceTool:   "scan_security",
		TargetTool:   "build_image",
		TriggerEvent: "vulnerabilities_found",
		Priority:     9,
		Conditions: []RuleCondition{
			{
				Type:     "metric_threshold",
				Field:    "severity_score",
				Operator: "greater_than",
				Value:    7.0,
			},
		},
	})

	// Deploy failure -> Manifest regeneration
	tc.coordinationRules = append(tc.coordinationRules, CoordinationRule{
		ID:           "deploy_failure_regenerate",
		Name:         "Regenerate manifests on deployment failure",
		SourceTool:   "deploy_kubernetes",
		TargetTool:   "generate_manifests",
		TriggerEvent: "deployment_failed",
		Priority:     8,
		Conditions: []RuleCondition{
			{
				Type:     "output_contains",
				Field:    "error_type",
				Operator: "equals",
				Value:    "manifest_invalid",
			},
		},
	})
}

// GetDependencyGraph returns the current dependency graph
func (tc *ToolCoordinator) GetDependencyGraph() *ToolDependencyGraph {
	return tc.dependencyGraph
}

// GetActiveCoordinations returns currently active coordinations
func (tc *ToolCoordinator) GetActiveCoordinations() map[string]*ActiveCoordination {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	result := make(map[string]*ActiveCoordination)
	for k, v := range tc.activeCoordinations {
		result[k] = v
	}
	return result
}

// CompleteCoordination marks a coordination as complete
func (tc *ToolCoordinator) CompleteCoordination(coordinationID string, result CoordinationResult) error {
	tc.mutex.RLock()
	coord, exists := tc.activeCoordinations[coordinationID]
	tc.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("coordination %s not found", coordinationID)
	}

	select {
	case coord.CompletionChan <- result:
		return nil
	default:
		return fmt.Errorf("coordination already completed")
	}
}

// generateCoordinationID generates a unique coordination ID
func (tc *ToolCoordinator) generateCoordinationID() string {
	return fmt.Sprintf("coord_%d", time.Now().UnixNano())
}

// generateMessageID generates a unique message ID
func (tc *ToolCoordinator) generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// ToolEvent represents an event from a tool
type ToolEvent struct {
	SourceTool string                 `json:"source_tool"`
	EventType  string                 `json:"event_type"`
	Data       interface{}            `json:"data"`
	Context    map[string]interface{} `json:"context"`
	Timestamp  time.Time              `json:"timestamp"`
}

// NewToolDependencyGraph creates a new dependency graph
func NewToolDependencyGraph() *ToolDependencyGraph {
	return &ToolDependencyGraph{
		nodes: make(map[string]*DependencyNode),
		edges: make(map[string][]string),
	}
}

// AddNode adds a node to the dependency graph
func (tdg *ToolDependencyGraph) AddNode(toolName string, dependencies []string) {
	tdg.mutex.Lock()
	defer tdg.mutex.Unlock()

	node := &DependencyNode{
		ToolName:     toolName,
		Dependencies: dependencies,
		Dependents:   []string{},
		Status:       "ready",
	}

	tdg.nodes[toolName] = node

	// Update edges and dependents
	for _, dep := range dependencies {
		tdg.edges[dep] = append(tdg.edges[dep], toolName)
		if depNode, exists := tdg.nodes[dep]; exists {
			depNode.Dependents = append(depNode.Dependents, toolName)
		}
	}
}

// GetExecutionOrder returns the order in which tools should be executed
func (tdg *ToolDependencyGraph) GetExecutionOrder() ([]string, error) {
	tdg.mutex.RLock()
	defer tdg.mutex.RUnlock()

	// Topological sort
	visited := make(map[string]bool)
	stack := []string{}

	var visit func(string) error
	visit = func(node string) error {
		if visited[node] {
			return nil
		}

		visited[node] = true

		// Visit dependencies first
		if nodeData, exists := tdg.nodes[node]; exists {
			for _, dep := range nodeData.Dependencies {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		stack = append(stack, node)
		return nil
	}

	// Visit all nodes
	for toolName := range tdg.nodes {
		if err := visit(toolName); err != nil {
			return nil, err
		}
	}

	return stack, nil
}

// NewCoordinationMetrics creates new coordination metrics
func NewCoordinationMetrics() *CoordinationMetrics {
	return &CoordinationMetrics{
		ToolPairMetrics: make(map[string]*ToolPairMetric),
	}
}

// recordSuccess records a successful coordination
func (cm *CoordinationMetrics) recordSuccess(sourceTool, targetTool string, duration time.Duration) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.TotalCoordinations++
	cm.SuccessfulCoordinations++

	// Update average latency
	cm.AverageLatency = (cm.AverageLatency*time.Duration(cm.TotalCoordinations-1) + duration) / time.Duration(cm.TotalCoordinations)

	// Update tool pair metrics
	pairKey := fmt.Sprintf("%s->%s", sourceTool, targetTool)
	if metric, exists := cm.ToolPairMetrics[pairKey]; exists {
		metric.Count++
		metric.Successes++
		metric.TotalTime += duration
	} else {
		cm.ToolPairMetrics[pairKey] = &ToolPairMetric{
			SourceTool: sourceTool,
			TargetTool: targetTool,
			Count:      1,
			Successes:  1,
			TotalTime:  duration,
		}
	}
}

// recordFailure records a failed coordination
func (cm *CoordinationMetrics) recordFailure(sourceTool, targetTool string, duration time.Duration) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.TotalCoordinations++
	cm.FailedCoordinations++

	// Update tool pair metrics
	pairKey := fmt.Sprintf("%s->%s", sourceTool, targetTool)
	if metric, exists := cm.ToolPairMetrics[pairKey]; exists {
		metric.Count++
		metric.Failures++
		metric.TotalTime += duration
	} else {
		cm.ToolPairMetrics[pairKey] = &ToolPairMetric{
			SourceTool: sourceTool,
			TargetTool: targetTool,
			Count:      1,
			Failures:   1,
			TotalTime:  duration,
		}
	}
}
