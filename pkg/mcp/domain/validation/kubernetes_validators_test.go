package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubernetesManifestValidator_BasicValidation(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("valid basic pod manifest", func(t *testing.T) {
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

		result := validator.Validate(context.Background(), manifest)
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("missing required fields", func(t *testing.T) {
		manifest := map[string]interface{}{
			"kind": "Pod",
			// Missing apiVersion and metadata
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.NotEmpty(t, result.Errors)
	})

	t.Run("invalid input type", func(t *testing.T) {
		result := validator.Validate(context.Background(), "not a manifest")
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "expected map[string]interface{}")
	})
}

func TestKubernetesManifestValidator_ResourceNames(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	testCases := []struct {
		name     string
		manifest map[string]interface{}
		valid    bool
	}{
		{
			name: "valid resource name",
			manifest: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "valid-name-123",
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test",
							"image": "test:latest",
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid resource name - uppercase",
			manifest: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "Invalid-Name",
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test",
							"image": "test:latest",
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid resource name - starts with number",
			manifest: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "123-invalid",
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test",
							"image": "test:latest",
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid resource name - too long",
			manifest: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "this-is-a-very-long-name-that-exceeds-the-maximum-allowed-length-for-kubernetes-resource-names-which-is-253-characters-and-this-string-is-definitely-longer-than-that-limit-so-it-should-fail-validation-when-we-test-it-here-in-this-unit-test-case-and-i-need-to-add-more-characters",
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test",
							"image": "test:latest",
						},
					},
				},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.Validate(context.Background(), tc.manifest)
			assert.Equal(t, tc.valid, result.Valid, "Test case: %s", tc.name)
			if !tc.valid {
				assert.NotEmpty(t, result.Errors)
			}
		})
	}
}

func TestKubernetesManifestValidator_Namespaces(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("forbidden namespace", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "kube-system",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test:latest",
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.NotEmpty(t, result.Errors)
		assert.Contains(t, result.Errors[0].Error(), "reserved")
	})

	t.Run("custom allowed namespaces", func(t *testing.T) {
		validatorWithAllowed := NewKubernetesManifestValidatorWithOptions(KubernetesValidatorOptions{
			AllowedNamespaces: []string{"prod", "staging"},
		})

		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "dev", // Not in allowed list
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test:latest",
					},
				},
			},
		}

		result := validatorWithAllowed.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "not in the allowed list")
	})
}

func TestKubernetesManifestValidator_Labels(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("valid labels", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
				"labels": map[string]interface{}{
					"app":                    "myapp",
					"version":                "v1.2.3",
					"component":              "frontend",
					"app.kubernetes.io/name": "myapp",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test:latest",
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.True(t, result.Valid)
	})

	t.Run("invalid label key - too long", func(t *testing.T) {
		longKey := "this-is-a-very-long-label-key-that-exceeds-sixty-three-characters"
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
				"labels": map[string]interface{}{
					longKey: "value",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test:latest",
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "too long")
	})

	t.Run("invalid label value - too long", func(t *testing.T) {
		longValue := "this-is-a-very-long-label-value-that-exceeds-sixty-three-characters-limit"
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
				"labels": map[string]interface{}{
					"app": longValue,
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test:latest",
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "too long")
	})
}

func TestKubernetesManifestValidator_APIVersionKind(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	testCases := []struct {
		name       string
		apiVersion string
		kind       string
		valid      bool
	}{
		{"valid Pod", "v1", "Pod", true},
		{"valid Deployment", "apps/v1", "Deployment", true},
		{"valid Service", "v1", "Service", true},
		{"invalid combination", "v1", "Deployment", false},
		{"unknown apiVersion", "unknown/v1", "Pod", true}, // Valid in non-strict mode
		{"empty apiVersion", "", "Pod", false},
		{"empty kind", "v1", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := map[string]interface{}{
				"apiVersion": tc.apiVersion,
				"kind":       tc.kind,
				"metadata": map[string]interface{}{
					"name": "test-resource",
				},
			}

			// Add spec for resources that need it
			if tc.apiVersion != "" && tc.kind != "" {
				switch tc.kind {
				case "Pod":
					manifest["spec"] = map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test",
								"image": "test:latest",
							},
						},
					}
				case "Deployment":
					manifest["spec"] = map[string]interface{}{
						"replicas": 1,
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
										"name":  "test",
										"image": "test:latest",
									},
								},
							},
						},
					}
				case "Service":
					manifest["spec"] = map[string]interface{}{
						"type": "ClusterIP",
						"ports": []interface{}{
							map[string]interface{}{
								"port":     80,
								"protocol": "TCP",
							},
						},
						"selector": map[string]interface{}{
							"app": "test",
						},
					}
				}
			}

			result := validator.Validate(context.Background(), manifest)
			assert.Equal(t, tc.valid, result.Valid, "Test case: %s", tc.name)
		})
	}
}

func TestKubernetesManifestValidator_PodSpec(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("valid pod spec", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test-container",
						"image": "nginx:1.20",
					},
				},
				"restartPolicy": "Always",
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.True(t, result.Valid)
	})

	t.Run("missing containers", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				// Missing containers
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "containers")
	})

	t.Run("empty containers array", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "at least one container")
	})

	t.Run("invalid restart policy", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test-container",
						"image": "nginx:1.20",
					},
				},
				"restartPolicy": "InvalidPolicy",
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "invalid restart policy")
	})
}

func TestKubernetesManifestValidator_ContainerValidation(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("missing container name", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"image": "nginx:1.20",
						// Missing name
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "container name is required")
	})

	t.Run("missing container image", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name": "test-container",
						// Missing image
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "container image is required")
	})

	t.Run("invalid container port", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test-container",
						"image": "nginx:1.20",
						"ports": []interface{}{
							map[string]interface{}{
								"containerPort": 99999, // Invalid port number
							},
						},
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "containerPort must be between")
	})
}

func TestKubernetesManifestValidator_DeploymentValidation(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("valid deployment", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
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

		result := validator.Validate(context.Background(), manifest)
		assert.True(t, result.Valid)
	})

	t.Run("negative replicas", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": -1,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "test",
					},
				},
				"template": map[string]interface{}{
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

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "replicas cannot be negative")
	})
}

func TestKubernetesManifestValidator_ServiceValidation(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("valid service", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "test-service",
			},
			"spec": map[string]interface{}{
				"type": "ClusterIP",
				"ports": []interface{}{
					map[string]interface{}{
						"port":       80,
						"targetPort": 8080,
						"protocol":   "TCP",
					},
				},
				"selector": map[string]interface{}{
					"app": "test",
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.True(t, result.Valid)
	})

	t.Run("invalid service type", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "test-service",
			},
			"spec": map[string]interface{}{
				"type": "InvalidType",
				"ports": []interface{}{
					map[string]interface{}{
						"port": 80,
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "invalid service type")
	})
}

func TestKubernetesManifestValidator_ValidatorMetadata(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	assert.Equal(t, "KubernetesManifestValidator", validator.Name())
	assert.Equal(t, "kubernetes", validator.Domain())
	assert.Equal(t, "manifest", validator.Category())
	assert.Equal(t, 100, validator.Priority())
	assert.Empty(t, validator.Dependencies())
}

func TestKubernetesManifestValidator_Options(t *testing.T) {
	t.Run("security validation disabled", func(t *testing.T) {
		validator := NewKubernetesManifestValidatorWithOptions(KubernetesValidatorOptions{
			ValidateSecurity: false,
		})

		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test-container",
						"image": "nginx:1.20",
						"securityContext": map[string]interface{}{
							"privileged": true, // This would normally fail security validation
						},
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		// Should pass because security validation is disabled
		assert.True(t, result.Valid)
	})

	t.Run("strict mode enabled", func(t *testing.T) {
		validator := NewKubernetesManifestValidatorWithOptions(KubernetesValidatorOptions{
			StrictMode: true,
		})

		manifest := map[string]interface{}{
			"apiVersion": "custom/v1", // Unknown API version
			"kind":       "CustomResource",
			"metadata": map[string]interface{}{
				"name": "test-resource",
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid) // Should fail in strict mode
	})
}
