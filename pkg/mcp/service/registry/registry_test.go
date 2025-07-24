package registry

import (
	"sync"
	"testing"
)

func TestRegistry_BasicOperations(t *testing.T) {
	registry := New[string]()

	// Test Add and Get
	registry.Add("key1", "value1")
	value, exists := registry.Get("key1")
	if !exists || value != "value1" {
		t.Errorf("Expected value1, got %s, exists: %v", value, exists)
	}

	// Test non-existent key
	_, exists = registry.Get("nonexistent")
	if exists {
		t.Error("Expected key to not exist")
	}

	// Test Exists
	if !registry.Exists("key1") {
		t.Error("Expected key1 to exist")
	}
	if registry.Exists("nonexistent") {
		t.Error("Expected nonexistent key to not exist")
	}
}

func TestRegistry_All(t *testing.T) {
	registry := New[int]()

	// Add some values
	registry.Add("one", 1)
	registry.Add("two", 2)
	registry.Add("three", 3)

	all := registry.All()
	if len(all) != 3 {
		t.Errorf("Expected 3 items, got %d", len(all))
	}

	// Test copy-on-iterate safety
	all["four"] = 4
	if registry.Exists("four") {
		t.Error("Modifying returned map should not affect registry")
	}
}

func TestRegistry_Keys(t *testing.T) {
	registry := New[string]()

	registry.Add("a", "value_a")
	registry.Add("b", "value_b")

	keys := registry.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	// Verify keys are present
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}

	if !keyMap["a"] || !keyMap["b"] {
		t.Error("Missing expected keys")
	}
}

func TestRegistry_Remove(t *testing.T) {
	registry := New[string]()

	registry.Add("key1", "value1")
	registry.Add("key2", "value2")

	// Remove existing key
	if !registry.Remove("key1") {
		t.Error("Expected removal to succeed")
	}

	// Verify removal
	if registry.Exists("key1") {
		t.Error("Key should have been removed")
	}

	// Remove non-existent key
	if registry.Remove("nonexistent") {
		t.Error("Expected removal to fail")
	}
}

func TestRegistry_SizeAndClear(t *testing.T) {
	registry := New[string]()

	if registry.Size() != 0 {
		t.Error("Expected empty registry")
	}

	registry.Add("key1", "value1")
	registry.Add("key2", "value2")

	if registry.Size() != 2 {
		t.Errorf("Expected size 2, got %d", registry.Size())
	}

	registry.Clear()

	if registry.Size() != 0 {
		t.Error("Expected empty registry after clear")
	}
}

func TestRegistry_GetOrError(t *testing.T) {
	registry := New[string]()

	// Test with non-existent key
	_, err := registry.GetOrError("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent key")
	}

	// Test with existing key
	registry.Add("key1", "value1")
	value, err := registry.GetOrError("key1")
	if err != nil || value != "value1" {
		t.Errorf("Expected value1, got %s, error: %v", value, err)
	}
}

func TestRegistry_ThreadSafety(t *testing.T) {
	registry := New[int]()

	// Run concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + id%26))
			registry.Add(key, id)
			registry.Get(key)
			registry.Exists(key)
			registry.All()
			registry.Keys()
		}(i)
	}

	wg.Wait()

	// Verify some operations worked
	if registry.Size() == 0 {
		t.Error("Expected some items to be added")
	}
}

func TestRegistry_TypeSafety(t *testing.T) {
	// Test with different types
	stringRegistry := New[string]()
	intRegistry := New[int]()

	stringRegistry.Add("key", "string_value")
	intRegistry.Add("key", 42)

	strVal, _ := stringRegistry.Get("key")
	intVal, _ := intRegistry.Get("key")

	if strVal != "string_value" {
		t.Errorf("Expected string_value, got %s", strVal)
	}

	if intVal != 42 {
		t.Errorf("Expected 42, got %d", intVal)
	}
}
