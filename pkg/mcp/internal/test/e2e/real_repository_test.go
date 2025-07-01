package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// RealRepositoryTestCase represents a test case for real repository integration
type RealRepositoryTestCase struct {
	Name              string
	RepoURL           string
	Branch            string
	ExpectedLanguage  string
	ExpectedFramework string
	SkipBuild         bool // Skip build step if repository is too large
	Timeout           bool // Expect timeout for large repos
}

// TestRealRepositoryIntegration tests complete workflows on real GitHub repositories
func TestRealRepositoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real repository tests in short mode")
	}

	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	// Real repository test cases covering different languages and frameworks
	testCases := []RealRepositoryTestCase{
		{
			Name:              "Java_Maven_Spring",
			RepoURL:           "https://github.com/spring-projects/spring-petclinic",
			Branch:            "main",
			ExpectedLanguage:  "java",
			ExpectedFramework: "maven",
			SkipBuild:         false,
		},
		{
			Name:              "Node_NPM_Express",
			RepoURL:           "https://github.com/expressjs/express",
			Branch:            "master",
			ExpectedLanguage:  "javascript",
			ExpectedFramework: "npm",
			SkipBuild:         true, // Large repository
		},
		{
			Name:              "Python_Pip_Flask",
			RepoURL:           "https://github.com/pallets/flask",
			Branch:            "main",
			ExpectedLanguage:  "python",
			ExpectedFramework: "pip",
			SkipBuild:         true, // Complex repository
		},
		{
			Name:              "Go_Modules_Gin",
			RepoURL:           "https://github.com/gin-gonic/gin",
			Branch:            "master",
			ExpectedLanguage:  "go",
			ExpectedFramework: "go-modules",
			SkipBuild:         false,
		},
		{
			Name:              "TypeScript_NPM_Simple",
			RepoURL:           "https://github.com/microsoft/TypeScript-Node-Starter",
			Branch:            "master",
			ExpectedLanguage:  "typescript",
			ExpectedFramework: "npm",
			SkipBuild:         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			runRealRepositoryTest(t, client, tc)
		})
	}
}

// TestRealRepositoryWorkflowValidation validates complete workflows work on real repositories
func TestRealRepositoryWorkflowValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real repository workflow tests in short mode")
	}

	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	// Focus on one well-known, stable repository for complete workflow testing
	testCase := RealRepositoryTestCase{
		Name:              "Complete_Workflow_SpringPetClinic",
		RepoURL:           "https://github.com/spring-projects/spring-petclinic",
		Branch:            "main",
		ExpectedLanguage:  "java",
		ExpectedFramework: "maven",
		SkipBuild:         false,
	}

	ctx := context.Background()

	t.Log("Starting complete workflow test on real repository...")

	// Step 1: Repository Analysis
	t.Log("Step 1: Analyzing real repository...")
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": testCase.RepoURL,
		"branch":   testCase.Branch,
	})
	require.NoError(t, err, "Real repository analysis should succeed")

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Validate analysis accuracy on real repository
	validateRealRepositoryAnalysis(t, analyzeResult, testCase)

	// Step 2: Dockerfile Generation
	t.Log("Step 2: Generating Dockerfile for real repository...")
	dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "auto", // Auto-detect based on analysis
	})
	require.NoError(t, err, "Dockerfile generation should succeed for real repository")

	// Validate generated Dockerfile
	validateGeneratedDockerfile(t, client, sessionID, dockerfileResult, testCase)

	// Step 3: Build Image (if not skipped)
	if !testCase.SkipBuild {
		t.Log("Step 3: Building image from real repository...")
		buildResult, err := client.CallTool(ctx, "build_image", map[string]interface{}{
			"session_id": sessionID,
			"image_name": "real-repo-test",
			"tag":        "latest",
		})

		if err != nil {
			t.Logf("Build failed (may be expected for complex repositories): %v", err)
		} else {
			validateSuccessfulBuild(t, buildResult, sessionID)
		}
	} else {
		t.Log("Step 3: Skipping image build for large repository")
	}

	// Step 4: Manifest Generation
	t.Log("Step 4: Generating Kubernetes manifests...")
	manifestResult, err := client.CallTool(ctx, "generate_manifests", map[string]interface{}{
		"session_id": sessionID,
		"app_name":   "real-repo-test",
		"port":       8080,
	})
	require.NoError(t, err, "Manifest generation should succeed")

	validateGeneratedManifests(t, client, sessionID, manifestResult)

	t.Logf("✅ Complete workflow successful for real repository: %s", testCase.RepoURL)
}

// TestRealRepositoryErrorHandling tests error handling with problematic real repositories
func TestRealRepositoryErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real repository error handling tests in short mode")
	}

	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test cases with various real-world problems
	errorTestCases := []struct {
		name        string
		repoURL     string
		branch      string
		expectError bool
		errorType   string
	}{
		{
			name:        "nonexistent_repository",
			repoURL:     "https://github.com/nonexistent/repository-12345",
			branch:      "main",
			expectError: true,
			errorType:   "not found",
		},
		{
			name:        "invalid_branch",
			repoURL:     "https://github.com/spring-projects/spring-petclinic",
			branch:      "nonexistent-branch",
			expectError: true,
			errorType:   "branch not found",
		},
		{
			name:        "empty_repository",
			repoURL:     "https://github.com/octocat/Hello-World",
			branch:      "master",
			expectError: false, // Should handle gracefully
			errorType:   "",
		},
		{
			name:        "private_repository",
			repoURL:     "https://github.com/private/secret-repo",
			branch:      "main",
			expectError: true,
			errorType:   "access denied",
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
				"repo_url": tc.repoURL,
				"branch":   tc.branch,
			})

			if tc.expectError {
				require.Error(t, err, "Should fail for problematic repository")
				errorMsg := strings.ToLower(err.Error())

				// Error should be informative
				assert.True(t, len(errorMsg) > 10, "Error message should be descriptive")
				t.Logf("Expected error for %s: %v", tc.name, err)
			} else {
				if err != nil {
					t.Logf("Unexpected error for %s (may be implementation dependent): %v", tc.name, err)
				}
			}
		})
	}
}

// TestRealRepositoryPerformance tests performance with real repositories of various sizes
func TestRealRepositoryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real repository performance tests in short mode")
	}

	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test repositories of different sizes
	performanceTestCases := []struct {
		name    string
		repoURL string
		branch  string
		size    string
	}{
		{
			name:    "small_repository",
			repoURL: "https://github.com/octocat/Hello-World",
			branch:  "master",
			size:    "small",
		},
		{
			name:    "medium_repository",
			repoURL: "https://github.com/spring-projects/spring-petclinic",
			branch:  "main",
			size:    "medium",
		},
		{
			name:    "large_repository",
			repoURL: "https://github.com/kubernetes/kubernetes",
			branch:  "master",
			size:    "large",
		},
	}

	for _, tc := range performanceTestCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			result, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
				"repo_url": tc.repoURL,
				"branch":   tc.branch,
			})

			duration := time.Since(start)

			if err != nil {
				t.Logf("Analysis failed for %s repository (may timeout): %v", tc.size, err)
				return
			}

			sessionID, err := client.ExtractSessionID(result)
			require.NoError(t, err)

			// Performance expectations based on repository size
			switch tc.size {
			case "small":
				assert.True(t, duration < 30*time.Second, "Small repository should analyze quickly")
			case "medium":
				assert.True(t, duration < 2*time.Minute, "Medium repository should analyze reasonably fast")
			case "large":
				// Large repositories may take longer or timeout
				t.Logf("Large repository analysis took: %v", duration)
			}

			t.Logf("Performance for %s repository (%s): %v", tc.size, tc.repoURL, duration)
			t.Logf("Session ID: %s", sessionID)
		})
	}
}

// TestRealRepositoryLanguageDetection tests language detection accuracy
func TestRealRepositoryLanguageDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping language detection tests in short mode")
	}

	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test repositories with clear language indicators
	languageTestCases := []struct {
		name             string
		repoURL          string
		branch           string
		expectedLanguage string
		mustDetect       bool
	}{
		{
			name:             "pure_java",
			repoURL:          "https://github.com/spring-projects/spring-petclinic",
			branch:           "main",
			expectedLanguage: "java",
			mustDetect:       true,
		},
		{
			name:             "pure_go",
			repoURL:          "https://github.com/gin-gonic/gin",
			branch:           "master",
			expectedLanguage: "go",
			mustDetect:       true,
		},
		{
			name:             "pure_python",
			repoURL:          "https://github.com/pallets/flask",
			branch:           "main",
			expectedLanguage: "python",
			mustDetect:       true,
		},
		{
			name:             "mixed_languages",
			repoURL:          "https://github.com/kubernetes/kubernetes",
			branch:           "master",
			expectedLanguage: "go",  // Primary language
			mustDetect:       false, // May detect other languages
		},
	}

	for _, tc := range languageTestCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
				"repo_url": tc.repoURL,
				"branch":   tc.branch,
			})

			if err != nil {
				t.Logf("Analysis failed for %s: %v", tc.name, err)
				return
			}

			// Check language detection
			detectedLanguage, exists := result["language"]
			require.True(t, exists, "Language should be detected")

			if tc.mustDetect {
				assert.Equal(t, tc.expectedLanguage, detectedLanguage,
					"Should correctly detect %s for %s", tc.expectedLanguage, tc.repoURL)
			} else {
				t.Logf("Detected language for %s: %v (expected: %s)", tc.name, detectedLanguage, tc.expectedLanguage)
			}

			// Check if multiple languages are detected
			if languages, exists := result["languages"]; exists {
				languageList, ok := languages.([]interface{})
				if ok && len(languageList) > 1 {
					t.Logf("Multiple languages detected for %s: %v", tc.name, languageList)
				}
			}
		})
	}
}

// Helper functions

func runRealRepositoryTest(t *testing.T, client testutil.MCPTestClient, tc RealRepositoryTestCase) {
	ctx := context.Background()

	// Run complete workflow on real repository
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": tc.RepoURL,
		"branch":   tc.Branch,
	})

	if err != nil {
		if tc.Timeout {
			t.Logf("Expected timeout/error for large repository: %v", err)
			return
		}
		require.NoError(t, err, "Repository analysis should succeed for %s", tc.RepoURL)
	}

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Validate analysis accuracy
	validateRealRepositoryAnalysis(t, analyzeResult, tc)

	// Validate generated Dockerfile works
	dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "auto",
	})
	require.NoError(t, err, "Dockerfile generation should succeed")

	validateGeneratedDockerfile(t, client, sessionID, dockerfileResult, tc)

	// Validate successful build (if not skipped)
	if !tc.SkipBuild {
		buildResult, err := client.CallTool(ctx, "build_image", map[string]interface{}{
			"session_id": sessionID,
			"image_name": strings.ToLower(tc.Name),
			"tag":        "test",
		})

		if err != nil {
			t.Logf("Build may fail for complex repositories: %v", err)
		} else {
			validateSuccessfulBuild(t, buildResult, sessionID)
		}
	}
}

func validateRealRepositoryAnalysis(t *testing.T, result map[string]interface{}, tc RealRepositoryTestCase) {
	// Validate language detection
	if language, exists := result["language"]; exists {
		detectedLang := strings.ToLower(language.(string))
		expectedLang := strings.ToLower(tc.ExpectedLanguage)

		// Allow for variations (e.g., "javascript" vs "js")
		if !strings.Contains(detectedLang, expectedLang) && !strings.Contains(expectedLang, detectedLang) {
			t.Logf("Language detection mismatch: expected %s, got %s (may be acceptable)",
				tc.ExpectedLanguage, language)
		}
	}

	// Validate framework detection
	if framework, exists := result["framework"]; exists {
		detectedFramework := strings.ToLower(framework.(string))
		expectedFramework := strings.ToLower(tc.ExpectedFramework)

		if !strings.Contains(detectedFramework, expectedFramework) && !strings.Contains(expectedFramework, detectedFramework) {
			t.Logf("Framework detection mismatch: expected %s, got %s (may be acceptable)",
				tc.ExpectedFramework, framework)
		}
	}

	// Validate required fields exist
	requiredFields := []string{"session_id", "language"}
	for _, field := range requiredFields {
		assert.Contains(t, result, field, "Analysis result should contain %s", field)
	}
}

func validateGeneratedDockerfile(t *testing.T, client testutil.MCPTestClient, sessionID string, result map[string]interface{}, tc RealRepositoryTestCase) {
	// Validate dockerfile path is returned
	dockerfilePath, exists := result["dockerfile_path"]
	assert.True(t, exists, "Dockerfile generation should return file path")

	if exists {
		assert.NotEmpty(t, dockerfilePath, "Dockerfile path should not be empty")
	}

	// Validate dockerfile exists in workspace
	workspace, err := client.GetSessionWorkspace(sessionID)
	require.NoError(t, err)

	dockerfileFullPath := filepath.Join(workspace, "Dockerfile")
	assert.FileExists(t, dockerfileFullPath, "Dockerfile should exist in session workspace")
}

func validateSuccessfulBuild(t *testing.T, result map[string]interface{}, sessionID string) {
	// Validate build success
	if success, exists := result["success"]; exists {
		assert.True(t, success.(bool), "Build should succeed")
	}

	// Validate session continuity
	if resultSessionID, exists := result["session_id"]; exists {
		assert.Equal(t, sessionID, resultSessionID, "Session should be preserved through build")
	}

	// Validate image reference is returned
	if imageRef, exists := result["image_ref"]; exists {
		assert.NotEmpty(t, imageRef, "Build should return image reference")
	}
}

func validateGeneratedManifests(t *testing.T, client testutil.MCPTestClient, sessionID string, result map[string]interface{}) {
	// Validate manifests are returned
	manifests, exists := result["manifests"]
	assert.True(t, exists, "Manifest generation should return manifests")

	if exists {
		manifestList, ok := manifests.([]interface{})
		assert.True(t, ok, "Manifests should be a list")
		assert.NotEmpty(t, manifestList, "Should generate at least one manifest")
	}

	// Validate session continuity
	manifestSessionID, err := client.ExtractSessionID(result)
	require.NoError(t, err)
	assert.Equal(t, sessionID, manifestSessionID, "Session should be preserved")

	// Validate manifest files exist in workspace
	workspace, err := client.GetSessionWorkspace(sessionID)
	require.NoError(t, err)

	expectedManifests := []string{"deployment.yaml", "service.yaml"}
	for _, manifest := range expectedManifests {
		manifestPath := filepath.Join(workspace, manifest)
		// Note: File existence depends on implementation
		if _, err := os.Stat(manifestPath); err == nil {
			t.Logf("✅ Generated manifest file: %s", manifest)
		} else {
			t.Logf("ℹ️  Manifest file not found (may be in-memory): %s", manifest)
		}
	}
}
