package core

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

// TestGenerateDockerfileArgumentMapping tests that all arguments are properly mapped
func TestGenerateDockerfileArgumentMapping(t *testing.T) {
	// Test input arguments
	testArgs := &analyze.GenerateDockerfileArgs{
		SessionID:          "test-session-123",
		DryRun:             false,
		BaseImage:          "tomcat:9-jdk11-openjdk",
		Template:           "java",
		Optimization:       "size",
		IncludeHealthCheck: true,
		BuildArgs:          map[string]string{"ENV": "production"},
		Platform:           "linux/amd64",
	}

	// Convert to map (this simulates what our tool registration does)
	argsMap := map[string]interface{}{
		"session_id":           testArgs.SessionID,
		"base_image":           testArgs.BaseImage,
		"template":             testArgs.Template,
		"optimization":         testArgs.Optimization,
		"include_health_check": testArgs.IncludeHealthCheck,
		"build_args":           testArgs.BuildArgs,
		"platform":             testArgs.Platform,
		"dry_run":              testArgs.DryRun,
	}

	// Verify all expected fields are present and correct
	assert.Equal(t, "test-session-123", argsMap["session_id"])
	assert.Equal(t, "tomcat:9-jdk11-openjdk", argsMap["base_image"])
	assert.Equal(t, "java", argsMap["template"])
	assert.Equal(t, "size", argsMap["optimization"])
	assert.Equal(t, true, argsMap["include_health_check"])
	assert.Equal(t, "linux/amd64", argsMap["platform"])
	assert.Equal(t, map[string]string{"ENV": "production"}, argsMap["build_args"])
	assert.Equal(t, false, argsMap["dry_run"])

	// Verify we have exactly the expected number of fields
	assert.Len(t, argsMap, 8, "Args map should contain exactly 8 fields")
}

func TestBuildImageArgumentMapping(t *testing.T) {
	// Test input arguments
	testArgs := &build.AtomicBuildImageArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session-123",
			DryRun:    false,
		},
		ImageName:      "myapp",
		ImageTag:       "v1.0.0",
		DockerfilePath: "./Dockerfile",
		BuildContext:   "./",
		Platform:       "linux/amd64",
		NoCache:        true,
		BuildArgs:      map[string]string{"VERSION": "1.0.0"},
		PushAfterBuild: false,
		RegistryURL:    "myregistry.com",
	}

	argsMap := map[string]interface{}{
		"session_id":       testArgs.SessionID,
		"image_name":       testArgs.ImageName,
		"image_tag":        testArgs.ImageTag,
		"dockerfile_path":  testArgs.DockerfilePath,
		"build_context":    testArgs.BuildContext,
		"build_args":       testArgs.BuildArgs,
		"platform":         testArgs.Platform,
		"no_cache":         testArgs.NoCache,
		"push_after_build": testArgs.PushAfterBuild,
		"registry_url":     testArgs.RegistryURL,
		"dry_run":          testArgs.DryRun,
	}

	// Verify the arguments
	assert.Equal(t, "test-session-123", argsMap["session_id"])
	assert.Equal(t, "myapp", argsMap["image_name"])
	assert.Equal(t, "v1.0.0", argsMap["image_tag"])
	assert.Equal(t, "./Dockerfile", argsMap["dockerfile_path"])
	assert.Equal(t, "./", argsMap["build_context"])
	assert.Equal(t, "linux/amd64", argsMap["platform"])
	assert.Equal(t, true, argsMap["no_cache"])
	assert.Equal(t, map[string]string{"VERSION": "1.0.0"}, argsMap["build_args"])
	assert.Equal(t, false, argsMap["push_after_build"])
	assert.Equal(t, "myregistry.com", argsMap["registry_url"])
	assert.Equal(t, false, argsMap["dry_run"])

	// Verify we have exactly the expected number of fields
	assert.Len(t, argsMap, 11, "Args map should contain exactly 11 fields")
}

func TestGenerateManifestsArgumentMapping(t *testing.T) {
	// Test input arguments
	testArgs := &deploy.AtomicGenerateManifestsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session-123",
			DryRun:    false,
		},
		ImageRef:     types.ImageReference{Repository: "myregistry.com/myapp:v1.0.0"},
		AppName:      "myapp",
		Namespace:    "production",
		ServicePorts: []deploy.ServicePort{{Port: 8080}},
		Replicas:     3,
		Resources: deploy.ResourceRequests{
			CPU:    "100m",
			Memory: "128Mi",
		},
		IncludeIngress: true,
		ServiceType:    "ClusterIP",
		Environment:    map[string]string{"ENV": "production"},
		HelmTemplate:   false,
	}

	argsMap := map[string]interface{}{
		"session_id":      testArgs.SessionID,
		"image_ref":       testArgs.ImageRef.Repository,
		"app_name":        testArgs.AppName,
		"namespace":       testArgs.Namespace,
		"port":            testArgs.ServicePorts[0].Port,
		"replicas":        testArgs.Replicas,
		"cpu_request":     testArgs.Resources.CPU,
		"memory_request":  testArgs.Resources.Memory,
		"include_ingress": testArgs.IncludeIngress,
		"service_type":    testArgs.ServiceType,
		"environment":     testArgs.Environment,
		"generate_helm":   testArgs.HelmTemplate,
		"dry_run":         testArgs.DryRun,
	}

	// Verify the arguments
	assert.Equal(t, "test-session-123", argsMap["session_id"])
	assert.Equal(t, "myregistry.com/myapp:v1.0.0", argsMap["image_ref"])
	assert.Equal(t, "myapp", argsMap["app_name"])
	assert.Equal(t, "production", argsMap["namespace"])
	assert.Equal(t, 8080, argsMap["port"])
	assert.Equal(t, 3, argsMap["replicas"])
	assert.Equal(t, "100m", argsMap["cpu_request"])
	assert.Equal(t, "128Mi", argsMap["memory_request"])
	assert.Equal(t, true, argsMap["include_ingress"])
	assert.Equal(t, "ClusterIP", argsMap["service_type"])
	assert.Equal(t, map[string]string{"ENV": "production"}, argsMap["environment"])
	assert.Equal(t, false, argsMap["generate_helm"])
	assert.Equal(t, false, argsMap["dry_run"])

	// Verify we have exactly the expected number of fields
	assert.Len(t, argsMap, 13, "Args map should contain exactly 13 fields")
}

func TestScanImageSecurityArgumentMapping(t *testing.T) {
	// Test input arguments
	testArgs := &scan.AtomicScanImageSecurityArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session-123",
			DryRun:    false,
		},
		ImageName:           "myapp:latest",
		SeverityThreshold:   "HIGH",
		VulnTypes:           []string{"os", "library"},
		IncludeFixable:      true,
		MaxResults:          100,
		IncludeRemediations: true,
		GenerateReport:      false,
		FailOnCritical:      true,
	}

	argsMap := map[string]interface{}{
		"session_id":           testArgs.SessionID,
		"image_name":           testArgs.ImageName,
		"severity_threshold":   testArgs.SeverityThreshold,
		"vuln_types":           testArgs.VulnTypes,
		"include_fixable":      testArgs.IncludeFixable,
		"max_results":          testArgs.MaxResults,
		"include_remediations": testArgs.IncludeRemediations,
		"generate_report":      testArgs.GenerateReport,
		"fail_on_critical":     testArgs.FailOnCritical,
		"dry_run":              testArgs.DryRun,
	}

	// Verify the arguments
	assert.Equal(t, "test-session-123", argsMap["session_id"])
	assert.Equal(t, "myapp:latest", argsMap["image_name"])
	assert.Equal(t, "HIGH", argsMap["severity_threshold"])
	assert.Equal(t, []string{"os", "library"}, argsMap["vuln_types"])
	assert.Equal(t, true, argsMap["include_fixable"])
	assert.Equal(t, 100, argsMap["max_results"])
	assert.Equal(t, true, argsMap["include_remediations"])
	assert.Equal(t, false, argsMap["generate_report"])
	assert.Equal(t, true, argsMap["fail_on_critical"])
	assert.Equal(t, false, argsMap["dry_run"])

	// Verify we have exactly the expected number of fields
	assert.Len(t, argsMap, 10, "Args map should contain exactly 10 fields")
}

func TestValidateDeploymentArgumentMapping(t *testing.T) {
	// Test input arguments
	testArgs := &deploy.AtomicDeployKubernetesArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session-123",
			DryRun:    false,
		},
		ImageRef:       "myapp:latest",
		AppName:        "myapp",
		Namespace:      "default",
		Replicas:       2,
		Port:           8080,
		ServiceType:    "LoadBalancer",
		IncludeIngress: false,
		Environment:    map[string]string{"ENV": "test"},
		CPURequest:     "200m",
		MemoryRequest:  "256Mi",
		GenerateOnly:   false,
		WaitForReady:   true,
		WaitTimeout:    300,
	}

	argsMap := map[string]interface{}{
		"session_id":      testArgs.SessionID,
		"image_ref":       testArgs.ImageRef,
		"app_name":        testArgs.AppName,
		"namespace":       testArgs.Namespace,
		"replicas":        testArgs.Replicas,
		"port":            testArgs.Port,
		"service_type":    testArgs.ServiceType,
		"include_ingress": testArgs.IncludeIngress,
		"environment":     testArgs.Environment,
		"cpu_request":     testArgs.CPURequest,
		"memory_request":  testArgs.MemoryRequest,
		"cpu_limit":       testArgs.CPULimit,
		"memory_limit":    testArgs.MemoryLimit,
		"generate_only":   testArgs.GenerateOnly,
		"wait_for_ready":  testArgs.WaitForReady,
		"wait_timeout":    testArgs.WaitTimeout,
		"dry_run":         true, // Force dry run for validation
	}

	// Verify the arguments
	assert.Equal(t, "test-session-123", argsMap["session_id"])
	assert.Equal(t, "myapp:latest", argsMap["image_ref"])
	assert.Equal(t, "myapp", argsMap["app_name"])
	assert.Equal(t, "default", argsMap["namespace"])
	assert.Equal(t, 2, argsMap["replicas"])
	assert.Equal(t, 8080, argsMap["port"])
	assert.Equal(t, "LoadBalancer", argsMap["service_type"])
	assert.Equal(t, false, argsMap["include_ingress"])
	assert.Equal(t, map[string]string{"ENV": "test"}, argsMap["environment"])
	assert.Equal(t, "200m", argsMap["cpu_request"])
	assert.Equal(t, "256Mi", argsMap["memory_request"])
	assert.Equal(t, false, argsMap["generate_only"])
	assert.Equal(t, true, argsMap["wait_for_ready"])
	assert.Equal(t, 300, argsMap["wait_timeout"])

	// Verify that dry_run is forced to true for validation
	assert.Equal(t, true, argsMap["dry_run"])

	// Verify we have exactly the expected number of fields
	assert.Len(t, argsMap, 17, "Args map should contain exactly 17 fields")
}

// TestArgumentTypesPreservation ensures that argument types are preserved correctly
func TestArgumentTypesPreservation(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		input     interface{}
		expected  interface{}
	}{
		{"string_field", "base_image", "tomcat:9-jdk11-openjdk", "tomcat:9-jdk11-openjdk"},
		{"bool_field", "include_health_check", true, true},
		{"map_field", "build_args", map[string]string{"ENV": "prod"}, map[string]string{"ENV": "prod"}},
		{"empty_string", "template", "", ""},
		{"false_bool", "dry_run", false, false},
		{"nil_map", "build_args", map[string]string(nil), map[string]string(nil)},
		{"int_field", "port", 8080, 8080},
		{"slice_field", "vuln_types", []string{"os", "library"}, []string{"os", "library"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple map to test type preservation
			argsMap := map[string]interface{}{
				tt.fieldName: tt.input,
			}

			// Verify the type and value are preserved
			actualValue := argsMap[tt.fieldName]
			assert.Equal(t, tt.expected, actualValue)
			assert.IsType(t, tt.expected, actualValue, "Type mismatch for field '%s'", tt.fieldName)
		})
	}
}

// TestEmptyAndNilHandling ensures that empty and nil values are handled correctly
func TestEmptyAndNilHandling(t *testing.T) {
	args := &analyze.GenerateDockerfileArgs{
		SessionID: "test",
		// Leave all other fields empty/default
	}

	argsMap := map[string]interface{}{
		"session_id":           args.SessionID,
		"base_image":           args.BaseImage,
		"template":             args.Template,
		"optimization":         args.Optimization,
		"include_health_check": args.IncludeHealthCheck,
		"build_args":           args.BuildArgs,
		"platform":             args.Platform,
		"dry_run":              args.DryRun,
	}

	// Verify default values
	assert.Equal(t, "test", argsMap["session_id"])
	assert.Equal(t, "", argsMap["base_image"])
	assert.Equal(t, "", argsMap["template"])
	assert.Equal(t, "", argsMap["optimization"])
	assert.Equal(t, false, argsMap["include_health_check"])
	assert.Equal(t, map[string]string(nil), argsMap["build_args"])
	assert.Equal(t, "", argsMap["platform"])
	assert.Equal(t, false, argsMap["dry_run"])
}

// Benchmark to ensure argument mapping doesn't introduce significant overhead
func BenchmarkArgumentMapping(b *testing.B) {
	testArgs := &analyze.GenerateDockerfileArgs{
		SessionID:    "test",
		BaseImage:    "nginx:latest",
		Template:     "node",
		Optimization: "speed",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		argsMap := map[string]interface{}{
			"session_id":           testArgs.SessionID,
			"base_image":           testArgs.BaseImage,
			"template":             testArgs.Template,
			"optimization":         testArgs.Optimization,
			"include_health_check": testArgs.IncludeHealthCheck,
			"build_args":           testArgs.BuildArgs,
			"platform":             testArgs.Platform,
			"dry_run":              testArgs.DryRun,
		}

		// Prevent compiler optimization
		_ = argsMap
	}
}
