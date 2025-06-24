package orchestration

import (
	"fmt"
	"sort"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration/workflow"
	"github.com/rs/zerolog"
)

// DefaultDependencyResolver implements DependencyResolver using topological sorting
type DefaultDependencyResolver struct {
	logger zerolog.Logger
}

// NewDefaultDependencyResolver creates a new dependency resolver
func NewDefaultDependencyResolver(logger zerolog.Logger) *DefaultDependencyResolver {
	return &DefaultDependencyResolver{
		logger: logger.With().Str("component", "dependency_resolver").Logger(),
	}
}

// ResolveDependencies resolves stage dependencies and returns execution groups
func (dr *DefaultDependencyResolver) ResolveDependencies(stages []workflow.WorkflowStage) ([][]workflow.WorkflowStage, error) {
	// Validate dependencies first
	if err := dr.ValidateDependencies(stages); err != nil {
		return nil, err
	}

	// Build stage map for easy lookup
	stageMap := make(map[string]workflow.WorkflowStage)
	for _, stage := range stages {
		stageMap[stage.Name] = stage
	}

	// Track stages that can be executed in parallel
	var executionGroups [][]workflow.WorkflowStage
	completed := make(map[string]bool)
	processing := make(map[string]bool)

	for len(completed) < len(stages) {
		var currentGroup []workflow.WorkflowStage

		// Find stages that can be executed now
		for _, stage := range stages {
			if completed[stage.Name] || processing[stage.Name] {
				continue
			}

			// Check if all dependencies are completed
			canExecute := true
			for _, dep := range stage.DependsOn {
				if !completed[dep] {
					canExecute = false
					break
				}
			}

			if canExecute {
				currentGroup = append(currentGroup, stage)
				processing[stage.Name] = true
			}
		}

		if len(currentGroup) == 0 {
			// No stages can be executed - this shouldn't happen if validation passed
			var remaining []string
			for _, stage := range stages {
				if !completed[stage.Name] {
					remaining = append(remaining, stage.Name)
				}
			}
			return nil, fmt.Errorf("circular dependency detected or missing dependencies for stages: %v", remaining)
		}

		// Sort stages in group by name for consistent execution order
		sort.Slice(currentGroup, func(i, j int) bool {
			return currentGroup[i].Name < currentGroup[j].Name
		})

		executionGroups = append(executionGroups, currentGroup)

		// Mark all stages in this group as completed
		for _, stage := range currentGroup {
			completed[stage.Name] = true
			delete(processing, stage.Name)
		}

		dr.logger.Debug().
			Int("group_index", len(executionGroups)-1).
			Int("stages_in_group", len(currentGroup)).
			Strs("stage_names", dr.getStageNames(currentGroup)).
			Msg("Resolved execution group")
	}

	dr.logger.Info().
		Int("total_stages", len(stages)).
		Int("execution_groups", len(executionGroups)).
		Msg("Successfully resolved stage dependencies")

	return executionGroups, nil
}

// ValidateDependencies validates that stage dependencies are valid
func (dr *DefaultDependencyResolver) ValidateDependencies(stages []workflow.WorkflowStage) error {
	// Build stage map for validation
	stageMap := make(map[string]bool)
	for _, stage := range stages {
		if stageMap[stage.Name] {
			return fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		stageMap[stage.Name] = true
	}

	// Validate that all dependencies exist
	for _, stage := range stages {
		for _, dep := range stage.DependsOn {
			if !stageMap[dep] {
				return fmt.Errorf("stage %s depends on non-existent stage: %s", stage.Name, dep)
			}
		}
	}

	// Check for circular dependencies using DFS
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	for _, stage := range stages {
		if !visited[stage.Name] {
			if dr.hasCycle(stage.Name, stages, visited, recursionStack) {
				return fmt.Errorf("circular dependency detected involving stage: %s", stage.Name)
			}
		}
	}

	return nil
}

// GetExecutionOrder returns a simple execution order (not grouped)
func (dr *DefaultDependencyResolver) GetExecutionOrder(stages []workflow.WorkflowStage) ([]string, error) {
	executionGroups, err := dr.ResolveDependencies(stages)
	if err != nil {
		return nil, err
	}

	var order []string
	for _, group := range executionGroups {
		for _, stage := range group {
			order = append(order, stage.Name)
		}
	}

	return order, nil
}

// GetDependencyGraph returns a visual representation of the dependency graph
func (dr *DefaultDependencyResolver) GetDependencyGraph(stages []workflow.WorkflowStage) (*DependencyGraph, error) {
	if err := dr.ValidateDependencies(stages); err != nil {
		return nil, err
	}

	graph := &DependencyGraph{
		Nodes: make(map[string]*GraphNode),
		Edges: []GraphEdge{},
	}

	// Create nodes
	for _, stage := range stages {
		node := &GraphNode{
			ID:         stage.Name,
			Name:       stage.Name,
			Type:       "stage",
			Tools:      stage.Tools,
			Parallel:   stage.Parallel,
			Conditions: len(stage.Conditions) > 0,
			Properties: make(map[string]interface{}),
		}

		// Add stage properties
		if stage.Timeout != nil {
			node.Properties["timeout"] = stage.Timeout.String()
		}
		if stage.RetryPolicy != nil {
			node.Properties["retry_policy"] = stage.RetryPolicy
		}
		if len(stage.Variables) > 0 {
			node.Properties["variables"] = stage.Variables
		}

		graph.Nodes[stage.Name] = node
	}

	// Create edges
	for _, stage := range stages {
		for _, dep := range stage.DependsOn {
			edge := GraphEdge{
				From:       dep,
				To:         stage.Name,
				Type:       "depends_on",
				Properties: make(map[string]interface{}),
			}
			graph.Edges = append(graph.Edges, edge)
		}
	}

	return graph, nil
}

// GetCriticalPath calculates the critical path through the workflow
func (dr *DefaultDependencyResolver) GetCriticalPath(stages []workflow.WorkflowStage, stageDurations map[string]time.Duration) ([]string, time.Duration, error) {
	// Build stage map
	stageMap := make(map[string]*workflow.WorkflowStage)
	for i := range stages {
		stageMap[stages[i].Name] = &stages[i]
	}

	// Initialize data structures for critical path calculation
	earliestStart := make(map[string]time.Duration)
	earliestFinish := make(map[string]time.Duration)
	latestStart := make(map[string]time.Duration)
	latestFinish := make(map[string]time.Duration)
	slack := make(map[string]time.Duration)

	// Build adjacency lists
	successors := make(map[string][]string)
	predecessors := make(map[string][]string)

	for _, stage := range stages {
		for _, dep := range stage.DependsOn {
			successors[dep] = append(successors[dep], stage.Name)
			predecessors[stage.Name] = append(predecessors[stage.Name], dep)
		}
	}

	// Forward pass: Calculate earliest start and finish times
	var processStage func(stageName string) time.Duration
	processed := make(map[string]bool)

	processStage = func(stageName string) time.Duration {
		if processed[stageName] {
			return earliestFinish[stageName]
		}

		// Calculate earliest start time
		var maxPredFinish time.Duration
		for _, pred := range predecessors[stageName] {
			predFinish := processStage(pred)
			if predFinish > maxPredFinish {
				maxPredFinish = predFinish
			}
		}

		earliestStart[stageName] = maxPredFinish
		duration := stageDurations[stageName]
		if duration == 0 {
			duration = time.Minute // Default duration if not specified
		}
		earliestFinish[stageName] = earliestStart[stageName] + duration
		processed[stageName] = true

		return earliestFinish[stageName]
	}

	// Process all stages
	var maxFinish time.Duration
	for _, stage := range stages {
		finish := processStage(stage.Name)
		if finish > maxFinish {
			maxFinish = finish
		}
	}

	// Backward pass: Calculate latest start and finish times
	for _, stage := range stages {
		latestFinish[stage.Name] = maxFinish
		latestStart[stage.Name] = maxFinish
	}

	// Process stages in reverse topological order
	var reverseProcess func(stageName string)
	reverseProcessed := make(map[string]bool)

	reverseProcess = func(stageName string) {
		if reverseProcessed[stageName] {
			return
		}

		// If stage has successors, calculate based on them
		if len(successors[stageName]) > 0 {
			minSuccStart := maxFinish
			for _, succ := range successors[stageName] {
				reverseProcess(succ)
				if latestStart[succ] < minSuccStart {
					minSuccStart = latestStart[succ]
				}
			}
			latestFinish[stageName] = minSuccStart
		}

		duration := stageDurations[stageName]
		if duration == 0 {
			duration = time.Minute
		}
		latestStart[stageName] = latestFinish[stageName] - duration

		// Calculate slack
		slack[stageName] = latestStart[stageName] - earliestStart[stageName]

		reverseProcessed[stageName] = true
	}

	for _, stage := range stages {
		reverseProcess(stage.Name)
	}

	// Find critical path (stages with zero slack)
	var criticalStages []string
	for _, stage := range stages {
		if slack[stage.Name] == 0 {
			criticalStages = append(criticalStages, stage.Name)
		}
	}

	// Build the critical path by following dependencies
	criticalPath := dr.buildCriticalPath(criticalStages, predecessors, successors, slack)

	dr.logger.Debug().
		Strs("critical_path", criticalPath).
		Dur("total_duration", maxFinish).
		Msg("Critical path calculated")

	return criticalPath, maxFinish, nil
}

// buildCriticalPath constructs the ordered critical path from critical stages
func (dr *DefaultDependencyResolver) buildCriticalPath(
	criticalStages []string,
	predecessors map[string][]string,
	successors map[string][]string,
	slack map[string]time.Duration,
) []string {
	// Create a set for quick lookup
	criticalSet := make(map[string]bool)
	for _, stage := range criticalStages {
		criticalSet[stage] = true
	}

	// Find starting nodes (no critical predecessors)
	var startNodes []string
	for _, stage := range criticalStages {
		hasCriticalPred := false
		for _, pred := range predecessors[stage] {
			if criticalSet[pred] {
				hasCriticalPred = true
				break
			}
		}
		if !hasCriticalPred {
			startNodes = append(startNodes, stage)
		}
	}

	// Build path from start nodes
	var path []string
	visited := make(map[string]bool)

	var buildPath func(node string)
	buildPath = func(node string) {
		if visited[node] {
			return
		}
		visited[node] = true
		path = append(path, node)

		// Find critical successors
		for _, succ := range successors[node] {
			if criticalSet[succ] && !visited[succ] {
				buildPath(succ)
				break // Follow only one path
			}
		}
	}

	// Build from each start node
	for _, start := range startNodes {
		buildPath(start)
	}

	return path
}

// Helper methods

func (dr *DefaultDependencyResolver) hasCycle(
	stageName string,
	stages []workflow.WorkflowStage,
	visited map[string]bool,
	recursionStack map[string]bool,
) bool {
	visited[stageName] = true
	recursionStack[stageName] = true

	// Find the stage by name
	var currentStage *workflow.WorkflowStage
	for _, stage := range stages {
		if stage.Name == stageName {
			currentStage = &stage
			break
		}
	}

	if currentStage == nil {
		return false
	}

	// Visit all dependencies
	for _, dep := range currentStage.DependsOn {
		if !visited[dep] {
			if dr.hasCycle(dep, stages, visited, recursionStack) {
				return true
			}
		} else if recursionStack[dep] {
			return true
		}
	}

	recursionStack[stageName] = false
	return false
}

func (dr *DefaultDependencyResolver) getStageNames(stages []workflow.WorkflowStage) []string {
	names := make([]string, len(stages))
	for i, stage := range stages {
		names[i] = stage.Name
	}
	return names
}

// DependencyGraph represents the dependency relationships between stages
type DependencyGraph struct {
	Nodes map[string]*GraphNode `json:"nodes"`
	Edges []GraphEdge           `json:"edges"`
}

// GraphNode represents a stage in the dependency graph
type GraphNode struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Tools      []string               `json:"tools"`
	Parallel   bool                   `json:"parallel"`
	Conditions bool                   `json:"conditions"`
	Properties map[string]interface{} `json:"properties"`
}

// GraphEdge represents a dependency relationship between stages
type GraphEdge struct {
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// AnalyzeDependencyComplexity analyzes the complexity of the dependency graph
func (dr *DefaultDependencyResolver) AnalyzeDependencyComplexity(stages []workflow.WorkflowStage) (*DependencyAnalysis, error) {
	if err := dr.ValidateDependencies(stages); err != nil {
		return nil, err
	}

	analysis := &DependencyAnalysis{
		TotalStages:     len(stages),
		ParallelStages:  0,
		SequentialDepth: 0,
		MaxFanOut:       0,
		MaxFanIn:        0,
		Bottlenecks:     []string{},
		IsolatedStages:  []string{},
	}

	// Build dependency maps
	dependents := make(map[string][]string)   // stages that depend on this stage
	dependencies := make(map[string][]string) // stages this stage depends on

	for _, stage := range stages {
		dependencies[stage.Name] = stage.DependsOn
		for _, dep := range stage.DependsOn {
			dependents[dep] = append(dependents[dep], stage.Name)
		}
	}

	// Analyze each stage
	for _, stage := range stages {
		fanOut := len(dependents[stage.Name])
		fanIn := len(dependencies[stage.Name])

		// Track max fan-out and fan-in
		if fanOut > analysis.MaxFanOut {
			analysis.MaxFanOut = fanOut
		}
		if fanIn > analysis.MaxFanIn {
			analysis.MaxFanIn = fanIn
		}

		// Identify bottlenecks (high fan-out)
		if fanOut > 3 {
			analysis.Bottlenecks = append(analysis.Bottlenecks, stage.Name)
		}

		// Identify isolated stages (no dependencies or dependents)
		if fanOut == 0 && fanIn == 0 {
			analysis.IsolatedStages = append(analysis.IsolatedStages, stage.Name)
		}

		// Count parallel stages
		if stage.Parallel {
			analysis.ParallelStages++
		}
	}

	// Calculate sequential depth
	executionGroups, err := dr.ResolveDependencies(stages)
	if err != nil {
		return nil, err
	}
	analysis.SequentialDepth = len(executionGroups)

	// Calculate parallelization potential
	if analysis.TotalStages > 0 {
		analysis.ParallelizationPotential = float64(analysis.ParallelStages) / float64(analysis.TotalStages)
	}

	return analysis, nil
}

// DependencyAnalysis contains analysis of the dependency graph complexity
type DependencyAnalysis struct {
	TotalStages              int      `json:"total_stages"`
	ParallelStages           int      `json:"parallel_stages"`
	SequentialDepth          int      `json:"sequential_depth"`
	MaxFanOut                int      `json:"max_fan_out"`
	MaxFanIn                 int      `json:"max_fan_in"`
	ParallelizationPotential float64  `json:"parallelization_potential"`
	Bottlenecks              []string `json:"bottlenecks"`
	IsolatedStages           []string `json:"isolated_stages"`
}

// GetOptimizationSuggestions returns suggestions for optimizing the workflow
func (dr *DefaultDependencyResolver) GetOptimizationSuggestions(stages []workflow.WorkflowStage) ([]OptimizationSuggestion, error) {
	analysis, err := dr.AnalyzeDependencyComplexity(stages)
	if err != nil {
		return nil, err
	}

	var suggestions []OptimizationSuggestion

	// Suggest parallelization opportunities
	if analysis.ParallelizationPotential < 0.3 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "parallelization",
			Priority:    "medium",
			Title:       "Consider adding parallelization",
			Description: "Your workflow has low parallelization potential. Consider if some stages can run in parallel.",
			Impact:      "Reduced execution time",
			Effort:      "medium",
		})
	}

	// Identify bottlenecks
	if len(analysis.Bottlenecks) > 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "bottleneck",
			Priority:    "high",
			Title:       "Address bottleneck stages",
			Description: fmt.Sprintf("Stages %v have high fan-out and may be bottlenecks", analysis.Bottlenecks),
			Impact:      "Improved parallelization and reduced critical path",
			Effort:      "high",
		})
	}

	// Suggest grouping isolated stages
	if len(analysis.IsolatedStages) > 0 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "grouping",
			Priority:    "low",
			Title:       "Consider grouping isolated stages",
			Description: fmt.Sprintf("Stages %v are isolated and could potentially be grouped", analysis.IsolatedStages),
			Impact:      "Simplified workflow structure",
			Effort:      "low",
		})
	}

	// Suggest reducing sequential depth
	if analysis.SequentialDepth > 5 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "depth",
			Priority:    "medium",
			Title:       "Consider reducing sequential depth",
			Description: fmt.Sprintf("Workflow has %d sequential levels, which may impact execution time", analysis.SequentialDepth),
			Impact:      "Faster execution through better parallelization",
			Effort:      "high",
		})
	}

	return suggestions, nil
}

// OptimizationSuggestion represents a suggestion for optimizing the workflow
type OptimizationSuggestion struct {
	Type        string `json:"type"`     // parallelization, bottleneck, grouping, depth
	Priority    string `json:"priority"` // high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"` // low, medium, high
}
