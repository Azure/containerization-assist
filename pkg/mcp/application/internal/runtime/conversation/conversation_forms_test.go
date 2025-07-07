package conversation

import (
	"testing"
)

func TestStructuredFormCreation(t *testing.T) {

	dockerForm := NewDockerfileConfigForm()
	if dockerForm.ID != "dockerfile_config" {
		t.Errorf("Expected dockerfile_config, got %s", dockerForm.ID)
	}
	if len(dockerForm.Fields) == 0 {
		t.Error("Expected dockerfile form to have fields")
	}
	repoForm := NewRepositoryAnalysisForm()
	if repoForm.ID != "repository_analysis" {
		t.Errorf("Expected repository_analysis, got %s", repoForm.ID)
	}
	if !repoForm.CanSkip {
		t.Error("Expected repository analysis form to be skippable")
	}
	k8sForm := NewKubernetesDeploymentForm()
	if k8sForm.ID != "kubernetes_deployment" {
		t.Errorf("Expected kubernetes_deployment, got %s", k8sForm.ID)
	}
	if k8sForm.CanSkip {
		t.Error("Expected kubernetes deployment form to be required")
	}
	registryForm := NewRegistryConfigForm()
	if registryForm.ID != "registry_config" {
		t.Errorf("Expected registry_config, got %s", registryForm.ID)
	}
	if !registryForm.CanSkip {
		t.Error("Expected registry config form to be skippable")
	}
}

func TestFormResponseParsing(t *testing.T) {

	jsonInput := `{
		"form_id": "dockerfile_config",
		"values": {
			"optimization": "size",
			"include_health_check": true
		},
		"skipped": false
	}`

	response, err := ParseFormResponse(jsonInput, "dockerfile_config")
	if err != nil {
		t.Fatalf("Failed to parse JSON form response: %v", err)
	}

	if response.FormID != "dockerfile_config" {
		t.Errorf("Expected dockerfile_config, got %s", response.FormID)
	}

	if response.Skipped {
		t.Error("Expected response not to be skipped")
	}

	optimization, ok := response.Values["optimization"].(string)
	if !ok || optimization != "size" {
		t.Errorf("Expected optimization to be 'size', got %v", response.Values["optimization"])
	}
	skipResponse, err := ParseFormResponse("skip", "dockerfile_config")
	if err != nil {
		t.Fatalf("Failed to parse skip response: %v", err)
	}

	if !skipResponse.Skipped {
		t.Error("Expected skip response to be marked as skipped")
	}
}

func TestFormApplication(t *testing.T) {

	state := NewConversationState("test-session", "/tmp/workspace")
	form := NewDockerfileConfigForm()
	response := &FormResponse{
		FormID: "dockerfile_config",
		Values: map[string]interface{}{
			"optimization":         "security",
			"base_image":           "alpine:latest",
			"include_health_check": false,
			"platform":             "linux/arm64",
		},
		Skipped: false,
	}
	err := form.ApplyFormResponse(response, state)
	if err != nil {
		t.Fatalf("Failed to apply form response: %v", err)
	}
	if value := GetFormValue(state, "dockerfile_config", "optimization", ""); value != "security" {
		t.Errorf("Expected optimization to be 'security', got %v", value)
	}

	if value := GetFormValue(state, "dockerfile_config", "base_image", ""); value != "alpine:latest" {
		t.Errorf("Expected base_image to be 'alpine:latest', got %v", value)
	}

	if value := GetFormValue(state, "dockerfile_config", "include_health_check", true); value != false {
		t.Errorf("Expected include_health_check to be false, got %v", value)
	}
	if completed, ok := state.Context["dockerfile_config_completed"].(bool); !ok || !completed {
		t.Error("Expected form to be marked as completed")
	}
}

func TestFormSkipping(t *testing.T) {

	state := NewConversationState("test-session", "/tmp/workspace")
	form := NewRepositoryAnalysisForm()
	response := &FormResponse{
		FormID:  "repository_analysis",
		Values:  map[string]interface{}{},
		Skipped: true,
	}
	err := form.ApplyFormResponse(response, state)
	if err != nil {
		t.Fatalf("Failed to apply skip response: %v", err)
	}
	if skipped, ok := state.Context["repository_analysis_skipped"].(bool); !ok || !skipped {
		t.Error("Expected form to be marked as skipped")
	}
	defaultValue := GetFormValue(state, "repository_analysis", "branch", "main")
	if defaultValue != "main" {
		t.Errorf("Expected default value 'main', got %v", defaultValue)
	}
}
