package validation

import (
	"context"
	"fmt"
	"testing"
)

// BenchmarkValidatorRegistry_ValidateAll benchmarks the unified validation system
func BenchmarkValidatorRegistry_ValidateAll(b *testing.B) {
	registry := NewValidatorRegistry()

	// Register validators
	kubernetesValidator := NewKubernetesManifestValidator()
	dockerValidator := NewDockerConfigValidator()
	securityValidator := NewSecurityPolicyValidator()

	_ = registry.Register(kubernetesValidator)
	_ = registry.Register(dockerValidator)
	_ = registry.Register(securityValidator)

	// Test data
	kubernetesManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test-container",
					"image": "nginx:latest",
				},
			},
		},
	}

	dockerConfig := map[string]interface{}{
		"image": "nginx:latest",
		"ports": []interface{}{"80:8080", "443:8443"},
		"environment": map[string]interface{}{
			"ENV_VAR": "value",
			"DEBUG":   "false",
		},
	}

	securityPolicy := map[string]interface{}{
		"securityContext": map[string]interface{}{
			"runAsNonRoot":           true,
			"readOnlyRootFilesystem": true,
		},
		"privileged":  false,
		"hostNetwork": false,
	}

	ctx := context.Background()

	b.Run("Kubernetes Manifest Validation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ValidateAll(ctx, kubernetesManifest, "kubernetes", "manifest")
		}
	})

	b.Run("Docker Config Validation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ValidateAll(ctx, dockerConfig, "docker", "config")
		}
	})

	b.Run("Security Policy Validation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ValidateAll(ctx, securityPolicy, "security", "policy")
		}
	})

	b.Run("Multiple Domain Validation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ValidateAll(ctx, kubernetesManifest, "kubernetes", "manifest")
			_ = registry.ValidateAll(ctx, securityPolicy, "security", "policy")
		}
	})
}

// BenchmarkValidatorChain benchmarks validator chain execution
func BenchmarkValidatorChain(b *testing.B) {
	// Create mock validators with different performance characteristics
	fastValidator := &mockValidator{
		name:   "fast",
		result: ValidationResult{Valid: true},
	}

	mediumValidator := &mockValidator{
		name:   "medium",
		result: ValidationResult{Valid: true, Warnings: []string{"warning"}},
	}

	slowValidator := &mockValidator{
		name:   "slow",
		result: ValidationResult{Valid: true, Warnings: []string{"warning1", "warning2"}},
	}

	testData := "benchmark-data"
	ctx := context.Background()

	b.Run("Single Validator", func(b *testing.B) {
		chain := NewValidatorChain[interface{}](ContinueOnError)
		chain.Add(fastValidator)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = chain.Validate(ctx, testData)
		}
	})

	b.Run("Three Validators Continue On Error", func(b *testing.B) {
		chain := NewValidatorChain[interface{}](ContinueOnError)
		chain.Add(fastValidator).Add(mediumValidator).Add(slowValidator)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = chain.Validate(ctx, testData)
		}
	})

	b.Run("Three Validators Stop On Error", func(b *testing.B) {
		chain := NewValidatorChain[interface{}](StopOnFirstError)
		chain.Add(fastValidator).Add(mediumValidator).Add(slowValidator)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = chain.Validate(ctx, testData)
		}
	})
}

// BenchmarkValidatorRegistry_Operations benchmarks registry operations
func BenchmarkValidatorRegistry_Operations(b *testing.B) {
	registry := NewValidatorRegistry()

	// Pre-populate with validators
	for i := 0; i < 100; i++ {
		validator := &mockValidator{
			name:     fmt.Sprintf("validator-%d", i),
			domain:   fmt.Sprintf("domain-%d", i%10),
			category: fmt.Sprintf("category-%d", i%5),
			priority: i,
			result:   ValidationResult{Valid: true},
		}
		_ = registry.Register(validator)
	}

	b.Run("Register", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			testValidator := &mockValidator{
				name:     fmt.Sprintf("bench-validator-%d", i),
				domain:   "bench",
				category: "test",
				result:   ValidationResult{Valid: true},
			}
			_ = registry.Register(testValidator)
			_ = registry.Unregister(testValidator.Name())
		}
	})

	b.Run("GetValidators", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.GetValidators("domain-1", "category-1")
		}
	})

	b.Run("GetDomainValidators", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.GetDomainValidators("domain-1")
		}
	})

	b.Run("ListValidators", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ListValidators()
		}
	})
}

// BenchmarkDependencyResolution benchmarks the dependency resolution algorithm
func BenchmarkDependencyResolution(b *testing.B) {
	impl := &ValidatorRegistryImpl{
		validators: make(map[string]DomainValidator[interface{}]),
	}

	// Create a complex dependency graph
	// A -> B -> C
	//   -> D -> E
	//      F -> G
	validators := []DomainValidator[interface{}]{
		&mockValidator{name: "A", dependencies: []string{"B", "D"}},
		&mockValidator{name: "B", dependencies: []string{"C"}},
		&mockValidator{name: "C", dependencies: []string{}},
		&mockValidator{name: "D", dependencies: []string{"E", "F"}},
		&mockValidator{name: "E", dependencies: []string{}},
		&mockValidator{name: "F", dependencies: []string{"G"}},
		&mockValidator{name: "G", dependencies: []string{}},
	}

	b.Run("Complex Dependency Resolution", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = impl.resolveDependencies(validators)
		}
	})

	// Linear dependency chain
	linearValidators := make([]DomainValidator[interface{}], 20)
	for i := 0; i < 20; i++ {
		var deps []string
		if i > 0 {
			deps = []string{fmt.Sprintf("linear-%d", i-1)}
		}
		linearValidators[i] = &mockValidator{
			name:         fmt.Sprintf("linear-%d", i),
			dependencies: deps,
		}
	}

	b.Run("Linear Dependency Chain", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = impl.resolveDependencies(linearValidators)
		}
	})
}

// BenchmarkValidatorComparison compares different validation approaches
func BenchmarkValidatorComparison(b *testing.B) {
	// Test data
	kubernetesManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
	}

	ctx := context.Background()

	// Direct validator call (baseline)
	kubernetesValidator := NewKubernetesManifestValidator()

	b.Run("Direct Validator Call", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = kubernetesValidator.Validate(ctx, kubernetesManifest)
		}
	})

	// Registry with single validator
	registry := NewValidatorRegistry()
	_ = registry.Register(kubernetesValidator)

	b.Run("Registry Single Validator", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ValidateAll(ctx, kubernetesManifest, "kubernetes", "manifest")
		}
	})

	// Validator chain
	chain := NewValidatorChain[interface{}](ContinueOnError)
	chain.Add(kubernetesValidator)

	b.Run("Validator Chain Single", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = chain.Validate(ctx, kubernetesManifest)
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	registry := NewValidatorRegistry()
	validator := NewKubernetesManifestValidator()
	_ = registry.Register(validator)

	kubernetesManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
	}

	ctx := context.Background()

	b.Run("Memory Allocation", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := registry.ValidateAll(ctx, kubernetesManifest, "kubernetes", "manifest")
			// Access result to prevent optimization
			_ = result.Valid
		}
	})
}
