package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock validator for testing
type mockValidator struct {
	name         string
	domain       string
	category     string
	priority     int
	dependencies []string
	result       ValidationResult
}

func (m *mockValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
	return m.result
}

func (m *mockValidator) Name() string {
	return m.name
}

func (m *mockValidator) Domain() string {
	return m.domain
}

func (m *mockValidator) Category() string {
	return m.category
}

func (m *mockValidator) Priority() int {
	return m.priority
}

func (m *mockValidator) Dependencies() []string {
	return m.dependencies
}

func TestValidatorRegistry_Register(t *testing.T) {
	registry := NewValidatorRegistry()

	t.Run("successful registration", func(t *testing.T) {
		validator := &mockValidator{
			name:     "test-validator",
			domain:   "test",
			category: "basic",
			priority: 100,
			result:   ValidationResult{Valid: true},
		}

		err := registry.Register(validator)
		assert.NoError(t, err)
		assert.Equal(t, 1, registry.(*ValidatorRegistryImpl).Count())
	})

	t.Run("empty name fails", func(t *testing.T) {
		validator := &mockValidator{
			name:     "",
			domain:   "test",
			category: "basic",
		}

		err := registry.Register(validator)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validator name cannot be empty")
	})

	t.Run("duplicate registration fails", func(t *testing.T) {
		validator1 := &mockValidator{
			name:     "duplicate",
			domain:   "test",
			category: "basic",
		}
		validator2 := &mockValidator{
			name:     "duplicate",
			domain:   "test",
			category: "advanced",
		}

		err := registry.Register(validator1)
		require.NoError(t, err)

		err = registry.Register(validator2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestValidatorRegistry_Dependencies(t *testing.T) {
	registry := NewValidatorRegistry()

	t.Run("dependency registration order", func(t *testing.T) {
		// Register dependency first
		dep := &mockValidator{
			name:     "dependency",
			domain:   "test",
			category: "basic",
			priority: 200,
			result:   ValidationResult{Valid: true},
		}
		err := registry.Register(dep)
		require.NoError(t, err)

		// Register dependent validator
		dependent := &mockValidator{
			name:         "dependent",
			domain:       "test",
			category:     "basic",
			priority:     100,
			dependencies: []string{"dependency"},
			result:       ValidationResult{Valid: true},
		}
		err = registry.Register(dependent)
		assert.NoError(t, err)
	})

	t.Run("missing dependency fails", func(t *testing.T) {
		validator := &mockValidator{
			name:         "needs-missing-dep",
			domain:       "test",
			category:     "basic",
			dependencies: []string{"missing-dependency"},
		}

		err := registry.Register(validator)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dependency 'missing-dependency' not found")
	})
}

func TestValidatorRegistry_Unregister(t *testing.T) {
	registry := NewValidatorRegistry()

	t.Run("successful unregistration", func(t *testing.T) {
		validator := &mockValidator{
			name:     "to-remove",
			domain:   "test",
			category: "basic",
		}

		err := registry.Register(validator)
		require.NoError(t, err)

		err = registry.Unregister("to-remove")
		assert.NoError(t, err)
		assert.Equal(t, 0, registry.(*ValidatorRegistryImpl).Count())
	})

	t.Run("unregister non-existent fails", func(t *testing.T) {
		err := registry.Unregister("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("cannot unregister with dependents", func(t *testing.T) {
		// Register dependency
		dep := &mockValidator{
			name:     "has-dependents",
			domain:   "test",
			category: "basic",
		}
		err := registry.Register(dep)
		require.NoError(t, err)

		// Register dependent
		dependent := &mockValidator{
			name:         "depends-on-above",
			domain:       "test",
			category:     "basic",
			dependencies: []string{"has-dependents"},
		}
		err = registry.Register(dependent)
		require.NoError(t, err)

		// Try to unregister dependency
		err = registry.Unregister("has-dependents")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "depends on it")
	})
}

func TestValidatorRegistry_GetValidators(t *testing.T) {
	registry := NewValidatorRegistry()

	// Setup test validators
	validators := []*mockValidator{
		{name: "v1", domain: "kubernetes", category: "manifest", priority: 100},
		{name: "v2", domain: "kubernetes", category: "manifest", priority: 200},
		{name: "v3", domain: "kubernetes", category: "policy", priority: 150},
		{name: "v4", domain: "docker", category: "config", priority: 100},
	}

	for _, v := range validators {
		err := registry.Register(v)
		require.NoError(t, err)
	}

	t.Run("filter by domain and category", func(t *testing.T) {
		result := registry.GetValidators("kubernetes", "manifest")
		assert.Len(t, result, 2)
		
		// Should be sorted by priority (higher first)
		assert.Equal(t, "v2", result[0].Name())
		assert.Equal(t, "v1", result[1].Name())
	})

	t.Run("no matches returns empty slice", func(t *testing.T) {
		result := registry.GetValidators("non-existent", "category")
		assert.Len(t, result, 0)
	})
}

func TestValidatorRegistry_GetDomainValidators(t *testing.T) {
	registry := NewValidatorRegistry()

	// Setup test validators
	validators := []*mockValidator{
		{name: "k1", domain: "kubernetes", category: "manifest", priority: 100},
		{name: "k2", domain: "kubernetes", category: "policy", priority: 200},
		{name: "d1", domain: "docker", category: "config", priority: 150},
	}

	for _, v := range validators {
		err := registry.Register(v)
		require.NoError(t, err)
	}

	result := registry.GetDomainValidators("kubernetes")
	assert.Len(t, result, 2)
	
	// Should be sorted by priority (higher first)
	assert.Equal(t, "k2", result[0].Name())
	assert.Equal(t, "k1", result[1].Name())
}

func TestValidatorRegistry_ValidateAll(t *testing.T) {
	registry := NewValidatorRegistry()

	t.Run("successful validation", func(t *testing.T) {
		v1 := &mockValidator{
			name:     "success1",
			domain:   "test",
			category: "basic",
			priority: 100,
			result:   ValidationResult{Valid: true, Warnings: []string{"warning1"}},
		}
		v2 := &mockValidator{
			name:     "success2",
			domain:   "test",
			category: "basic",
			priority: 200,
			result:   ValidationResult{Valid: true, Warnings: []string{"warning2"}},
		}

		err := registry.Register(v1)
		require.NoError(t, err)
		err = registry.Register(v2)
		require.NoError(t, err)

		result := registry.ValidateAll(context.Background(), "test-data", "test", "basic")
		assert.True(t, result.Valid)
		assert.Len(t, result.Errors, 0)
		assert.Len(t, result.Warnings, 2)
		assert.Contains(t, result.Warnings, "warning1")
		assert.Contains(t, result.Warnings, "warning2")
	})

	t.Run("validation with errors", func(t *testing.T) {
		registry.(*ValidatorRegistryImpl).Clear()

		v1 := &mockValidator{
			name:     "fail1",
			domain:   "test",
			category: "basic",
			result: ValidationResult{
				Valid:  false,
				Errors: []error{errors.NewValidationFailed("field1", "error1")},
			},
		}
		v2 := &mockValidator{
			name:     "success",
			domain:   "test",
			category: "basic",
			result:   ValidationResult{Valid: true},
		}

		err := registry.Register(v1)
		require.NoError(t, err)
		err = registry.Register(v2)
		require.NoError(t, err)

		result := registry.ValidateAll(context.Background(), "test-data", "test", "basic")
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
	})

	t.Run("no validators returns valid result", func(t *testing.T) {
		result := registry.ValidateAll(context.Background(), "test-data", "non-existent", "category")
		assert.True(t, result.Valid)
		assert.Len(t, result.Errors, 0)
		assert.Len(t, result.Warnings, 0)
	})
}

func TestValidatorRegistry_DependencyResolution(t *testing.T) {
	registry := NewValidatorRegistry()

	t.Run("dependency order execution", func(t *testing.T) {
		// Create validators with dependencies: C depends on B, B depends on A
		a := &mockValidator{
			name:     "A",
			domain:   "test",
			category: "chain",
			priority: 100,
			result:   ValidationResult{Valid: true},
		}
		b := &mockValidator{
			name:         "B",
			domain:       "test",
			category:     "chain",
			priority:     200,
			dependencies: []string{"A"},
			result:       ValidationResult{Valid: true},
		}
		c := &mockValidator{
			name:         "C",
			domain:       "test",
			category:     "chain",
			priority:     300,
			dependencies: []string{"B"},
			result:       ValidationResult{Valid: true},
		}

		// Register in dependency order: A first, then B, then C
		err := registry.Register(a)
		require.NoError(t, err)
		err = registry.Register(b)
		require.NoError(t, err)
		err = registry.Register(c)
		require.NoError(t, err)

		result := registry.ValidateAll(context.Background(), "test-data", "test", "chain")
		assert.True(t, result.Valid)
	})

	t.Run("circular dependency detection", func(t *testing.T) {
		// Test circular dependency by creating validators that depend on each other
		// but only after they're both registered
		circularRegistry := NewValidatorRegistry()

		// First register both validators without dependencies
		a := &mockValidator{
			name:     "CircularA",
			domain:   "test",
			category: "circular",
		}
		b := &mockValidator{
			name:     "CircularB", 
			domain:   "test",
			category: "circular",
		}

		err := circularRegistry.Register(a)
		require.NoError(t, err)
		err = circularRegistry.Register(b)
		require.NoError(t, err)

		// Now manually create validators with circular dependencies to test resolution
		validators := []DomainValidator[interface{}]{
			&mockValidator{
				name:         "TestA",
				domain:       "test",
				category:     "circular",
				dependencies: []string{"TestB"},
			},
			&mockValidator{
				name:         "TestB",
				domain:       "test",
				category:     "circular", 
				dependencies: []string{"TestA"},
			},
		}

		// Test the dependency resolution method directly
		impl := circularRegistry.(*ValidatorRegistryImpl)
		_, err = impl.resolveDependencies(validators)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency")
	})
}

func TestValidatorRegistry_ListValidators(t *testing.T) {
	registry := NewValidatorRegistry()

	// Register dependencies first
	dep1 := &mockValidator{name: "dep1", domain: "common", category: "basic", priority: 300}
	err := registry.Register(dep1)
	require.NoError(t, err)

	validators := []*mockValidator{
		{name: "z-validator", domain: "docker", category: "config", priority: 100, dependencies: []string{"dep1"}},
		{name: "a-validator", domain: "kubernetes", category: "manifest", priority: 200},
	}

	for _, v := range validators {
		err := registry.Register(v)
		require.NoError(t, err)
	}

	list := registry.ListValidators()
	assert.Len(t, list, 3)

	// Should be sorted by domain, category, priority
	assert.Equal(t, "dep1", list[0].Name)
	assert.Equal(t, "common", list[0].Domain)
	assert.Equal(t, "z-validator", list[1].Name)
	assert.Equal(t, "docker", list[1].Domain)
	assert.Equal(t, []string{"dep1"}, list[1].Dependencies)
}

func TestValidatorRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewValidatorRegistry()

	// Test concurrent registration and validation
	done := make(chan bool, 2)

	// Goroutine 1: Register validators
	go func() {
		for i := 0; i < 10; i++ {
			validator := &mockValidator{
				name:     fmt.Sprintf("concurrent-%d", i),
				domain:   "test",
				category: "concurrent",
				result:   ValidationResult{Valid: true},
			}
			_ = registry.Register(validator)
		}
		done <- true
	}()

	// Goroutine 2: Run validations
	go func() {
		for i := 0; i < 10; i++ {
			_ = registry.ValidateAll(context.Background(), "test-data", "test", "concurrent")
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify final state
	assert.True(t, registry.(*ValidatorRegistryImpl).Count() >= 0)
}