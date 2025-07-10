package validation

import (
	"context"
	"testing"
)

func BenchmarkKubernetesManifestValidator(b *testing.B) {
	validator := NewKubernetesManifestValidator()

	manifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "test",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test-container",
					"image": "nginx:1.20",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 80,
							"protocol":      "TCP",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, manifest)
		if !result.Valid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}
}

func BenchmarkKubernetesManifestValidatorWithOptions(b *testing.B) {
	validator := NewKubernetesManifestValidatorWithOptions(KubernetesValidatorOptions{
		ValidateSecurity:  true,
		ValidateResources: true,
		StrictMode:        true,
		AllowedNamespaces: []string{"default", "test"},
	})

	manifest := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "test-deployment",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"replicas": 3,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "test",
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "test",
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test-container",
							"image": "nginx:1.20",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, manifest)
		if !result.Valid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}
}

func BenchmarkKubernetesSecurityValidator(b *testing.B) {
	validator := NewKubernetesSecurityValidator()

	manifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test-container",
					"image": "nginx:1.20",
					"securityContext": map[string]interface{}{
						"runAsNonRoot":             true,
						"readOnlyRootFilesystem":   true,
						"allowPrivilegeEscalation": false,
					},
				},
			},
			"securityContext": map[string]interface{}{
				"runAsUser":  1000,
				"runAsGroup": 3000,
				"fsGroup":    2000,
			},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, manifest)
		if !result.Valid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}
}
