// Package steps provides a runtime plugin registry for workflow steps.
package steps

import (
	"fmt"
	"sort"
	"sync"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
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
