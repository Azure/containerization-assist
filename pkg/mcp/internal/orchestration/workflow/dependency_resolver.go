package workflow

import (
	"fmt"
	"sort"
)

// DefaultDependencyResolver implements DependencyResolver interface
type DefaultDependencyResolver struct{}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver() DependencyResolver {
	return &DefaultDependencyResolver{}
}

// ResolveDependencies groups stages by their dependency level for parallel execution
func (r *DefaultDependencyResolver) ResolveDependencies(stages []WorkflowStage) ([][]WorkflowStage, error) {
	// First validate dependencies
	if err := r.ValidateDependencies(stages); err != nil {
		return nil, err
	}

	// Build dependency graph
	stageMap := make(map[string]*WorkflowStage)
	dependents := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize maps
	for i := range stages {
		stage := &stages[i]
		stageMap[stage.Name] = stage
		inDegree[stage.Name] = 0
		dependents[stage.Name] = []string{}
	}

	// Build dependency relationships
	for i := range stages {
		stage := &stages[i]
		for _, dep := range stage.DependsOn {
			dependents[dep] = append(dependents[dep], stage.Name)
			inDegree[stage.Name]++
		}
	}

	// Find execution groups using topological sort with levels
	var executionGroups [][]WorkflowStage
	processed := make(map[string]bool)

	for {
		// Find all stages with no pending dependencies
		var currentGroup []WorkflowStage
		for name, stage := range stageMap {
			if processed[name] {
				continue
			}

			// Check if all dependencies are processed
			canExecute := true
			for _, dep := range stage.DependsOn {
				if !processed[dep] {
					canExecute = false
					break
				}
			}

			if canExecute {
				currentGroup = append(currentGroup, *stage)
			}
		}

		// No more stages to process
		if len(currentGroup) == 0 {
			break
		}

		// Sort stages in group by name for deterministic order
		sort.Slice(currentGroup, func(i, j int) bool {
			return currentGroup[i].Name < currentGroup[j].Name
		})

		// Mark stages as processed
		for _, stage := range currentGroup {
			processed[stage.Name] = true
		}

		executionGroups = append(executionGroups, currentGroup)
	}

	// Check if all stages were processed
	if len(processed) != len(stages) {
		var unprocessed []string
		for name := range stageMap {
			if !processed[name] {
				unprocessed = append(unprocessed, name)
			}
		}
		return nil, fmt.Errorf("circular dependency detected or unreachable stages: %v", unprocessed)
	}

	return executionGroups, nil
}

// ValidateDependencies checks for circular dependencies and missing stages
func (r *DefaultDependencyResolver) ValidateDependencies(stages []WorkflowStage) error {
	// Create stage name set for quick lookup
	stageNames := make(map[string]bool)
	for _, stage := range stages {
		if _, exists := stageNames[stage.Name]; exists {
			return fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		stageNames[stage.Name] = true
	}

	// Validate each stage's dependencies
	for _, stage := range stages {
		for _, dep := range stage.DependsOn {
			if !stageNames[dep] {
				return fmt.Errorf("stage '%s' depends on non-existent stage '%s'", stage.Name, dep)
			}
			if dep == stage.Name {
				return fmt.Errorf("stage '%s' has self-dependency", stage.Name)
			}
		}
	}

	// Check for circular dependencies using DFS
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited
	for _, stage := range stages {
		if err := r.detectCycle(stage.Name, stages, visited); err != nil {
			return err
		}
	}

	return nil
}

// GetExecutionOrder returns a flat list of stage names in execution order
func (r *DefaultDependencyResolver) GetExecutionOrder(stages []WorkflowStage) ([]string, error) {
	groups, err := r.ResolveDependencies(stages)
	if err != nil {
		return nil, err
	}

	var order []string
	for _, group := range groups {
		for _, stage := range group {
			order = append(order, stage.Name)
		}
	}

	return order, nil
}

// detectCycle uses DFS to detect circular dependencies
func (r *DefaultDependencyResolver) detectCycle(stageName string, stages []WorkflowStage, visited map[string]int) error {
	if visited[stageName] == 1 {
		return fmt.Errorf("circular dependency detected involving stage '%s'", stageName)
	}
	if visited[stageName] == 2 {
		return nil
	}

	visited[stageName] = 1

	// Find the stage
	var stage *WorkflowStage
	for i := range stages {
		if stages[i].Name == stageName {
			stage = &stages[i]
			break
		}
	}

	if stage == nil {
		return fmt.Errorf("stage '%s' not found", stageName)
	}

	// Check dependencies
	for _, dep := range stage.DependsOn {
		if err := r.detectCycle(dep, stages, visited); err != nil {
			return err
		}
	}

	visited[stageName] = 2
	return nil
}