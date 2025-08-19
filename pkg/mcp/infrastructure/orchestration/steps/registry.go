// Package steps provides a runtime plugin registry for workflow steps.
package steps

import (
	"fmt"
	"sort"
	"sync"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
)

var (
	mu       sync.RWMutex
	registry = make(map[string]workflow.Step)
	order    []string
)

// Register makes a step discoverable by name.
//
// Usage (in the step's own file):
//
//	func init() { steps.Register(NewAnalyzeStep()) }
func Register(step workflow.Step) {
	mu.Lock()
	defer mu.Unlock()

	name := step.Name()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("duplicate step registration: %s", name))
	}
	registry[name] = step
	order = append(order, name)
}

// All returns steps in deterministic order.
func All() []workflow.Step {
	mu.RLock()
	defer mu.RUnlock()

	// copy to avoid caller mutation
	steps := make([]workflow.Step, 0, len(order))
	for _, n := range order {
		steps = append(steps, registry[n])
	}
	return steps
}

// Names returns the registered step names (useful for debugging/config).
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()

	out := append([]string(nil), order...)
	sort.Strings(out)
	return out
}

// Get returns a specific step by name.
func Get(name string) (workflow.Step, bool) {
	mu.RLock()
	defer mu.RUnlock()

	step, ok := registry[name]
	return step, ok
}

// Clear removes all registered steps (useful for testing).
func Clear() {
	mu.Lock()
	defer mu.Unlock()

	registry = make(map[string]workflow.Step)
	order = nil
}
