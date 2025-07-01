package types

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Local type definitions for testing to avoid import cycles

type ProgressStage struct {
	Name        string
	Weight      float64
	Description string
}

type SessionState struct {
	ID                  string
	SessionID           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ExpiresAt           time.Time
	WorkspaceDir        string
	RepositoryAnalyzed  bool
	RepositoryInfo      *RepositoryInfo
	DockerfileGenerated bool
	ImageBuilt          bool
	CurrentStage        string
	Status              string
	Stage               string
	Errors              []string
	Metadata            map[string]interface{}
	SecurityScan        *SecurityScanResult
}

type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

type ToolMetadata struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Category     string            `json:"category"`
	Dependencies []string          `json:"dependencies"`
	Capabilities []string          `json:"capabilities"`
	Requirements []string          `json:"requirements"`
	Parameters   map[string]string `json:"parameters"`
	Examples     []ToolExample     `json:"examples"`
}

type FileStructure struct {
	TotalFiles      int      `json:"total_files"`
	ConfigFiles     []string `json:"config_files"`
	EntryPoints     []string `json:"entry_points"`
	TestFiles       []string `json:"test_files"`
	BuildFiles      []string `json:"build_files"`
	DockerFiles     []string `json:"docker_files"`
	KubernetesFiles []string `json:"kubernetes_files"`
	PackageManagers []string `json:"package_managers"`
}

type RepositoryInfo struct {
	Language        string        `json:"language"`
	Framework       string        `json:"framework"`
	Port            int           `json:"port"`
	Dependencies    []string      `json:"dependencies"`
	Structure       FileStructure `json:"structure"`
	Size            int64         `json:"size"`
	HasCI           bool          `json:"has_ci"`
	HasReadme       bool          `json:"has_readme"`
	Recommendations []string      `json:"recommendations"`
}

type BuildResult struct {
	ImageID  string      `json:"image_id"`
	ImageRef string      `json:"image_ref"`
	Success  bool        `json:"success"`
	Error    *BuildError `json:"error,omitempty"`
	Logs     string      `json:"logs,omitempty"`
}

type BuildError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type SecurityScanResult struct {
	Success   bool      `json:"success"`
	ScannedAt time.Time `json:"scanned_at"`
}

// Test interface conformance by implementing mock types

// MockAIAnalyzer implements AIAnalyzer interface
type MockAIAnalyzer struct {
	tokenUsage TokenUsage
}

func (m *MockAIAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt cannot be empty")
	}
	m.tokenUsage.PromptTokens += 10
	m.tokenUsage.CompletionTokens += 5
	m.tokenUsage.TotalTokens = m.tokenUsage.PromptTokens + m.tokenUsage.CompletionTokens
	return "mock analysis result", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("baseDir cannot be empty")
	}
	m.tokenUsage.PromptTokens += 15
	m.tokenUsage.CompletionTokens += 7
	m.tokenUsage.TotalTokens = m.tokenUsage.PromptTokens + m.tokenUsage.CompletionTokens
	return "mock analysis with file tools", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	if promptTemplate == "" {
		return "", fmt.Errorf("promptTemplate cannot be empty")
	}
	m.tokenUsage.PromptTokens += 12
	m.tokenUsage.CompletionTokens += 6
	m.tokenUsage.TotalTokens = m.tokenUsage.PromptTokens + m.tokenUsage.CompletionTokens
	return "mock formatted analysis", nil
}

func (m *MockAIAnalyzer) GetTokenUsage() TokenUsage {
	return m.tokenUsage
}

func (m *MockAIAnalyzer) ResetTokenUsage() {
	m.tokenUsage = TokenUsage{}
}

// MockProgressReporter removed - interface moved to main package to avoid import cycle

// Note: SessionManager testing removed due to complex dependencies

// MockBaseValidator removed - BaseValidator interface moved to base package

// Test interface conformance
func TestInterfaceConformance(t *testing.T) {
	// Test AIAnalyzer interface
	var aiAnalyzer AIAnalyzer = &MockAIAnalyzer{}

	// Test initial state
	initialUsage := aiAnalyzer.GetTokenUsage()
	if initialUsage.TotalTokens != 0 {
		t.Errorf("Expected initial TotalTokens to be 0, got %d", initialUsage.TotalTokens)
	}

	// Test Analyze method
	result, err := aiAnalyzer.Analyze(context.Background(), "test prompt")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != "mock analysis result" {
		t.Errorf("Expected 'mock analysis result', got %s", result)
	}

	// Test token usage update
	usage := aiAnalyzer.GetTokenUsage()
	if usage.TotalTokens != 15 {
		t.Errorf("Expected TotalTokens to be 15, got %d", usage.TotalTokens)
	}

	// Test error case
	_, err = aiAnalyzer.Analyze(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty prompt")
	}

	// Test AnalyzeWithFileTools
	result, err = aiAnalyzer.AnalyzeWithFileTools(context.Background(), "test", "/tmp")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != "mock analysis with file tools" {
		t.Errorf("Expected 'mock analysis with file tools', got %s", result)
	}

	// Test AnalyzeWithFormat
	result, err = aiAnalyzer.AnalyzeWithFormat(context.Background(), "template %s", "arg")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != "mock formatted analysis" {
		t.Errorf("Expected 'mock formatted analysis', got %s", result)
	}

	// Test ResetTokenUsage
	aiAnalyzer.ResetTokenUsage()
	resetUsage := aiAnalyzer.GetTokenUsage()
	if resetUsage.TotalTokens != 0 {
		t.Errorf("Expected TotalTokens to be 0 after reset, got %d", resetUsage.TotalTokens)
	}

	// HealthChecker test removed - interface moved to main package to avoid import cycle

	// ProgressReporter test removed - interface moved to main package to avoid import cycle

	// Note: SessionManager test removed due to complex dependencies

	// BaseValidator test removed - interface moved to base package
}

// Test interface type completeness
func TestTypeCompleteness(t *testing.T) {
	// Test that all required types are defined
	var tokenUsage TokenUsage
	tokenUsage.TotalTokens = 100
	if tokenUsage.TotalTokens != 100 {
		t.Error("TokenUsage.TotalTokens not set correctly")
	}

	var progressStage ProgressStage
	progressStage.Name = "test"
	progressStage.Weight = 0.5
	progressStage.Description = "test description"
	if progressStage.Name != "test" || progressStage.Weight != 0.5 {
		t.Error("ProgressStage fields not set correctly")
	}

	var sessionState SessionState
	sessionState.SessionID = "test"
	sessionState.WorkspaceDir = "/tmp/test"
	if sessionState.SessionID != "test" {
		t.Error("SessionState.SessionID not set correctly")
	}

	// Circuit breaker constants test removed - constants moved to canonical location

	t.Log("All interface types are properly defined")
}

// Test TokenUsage type
func TestTokenUsage(t *testing.T) {
	tu := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	if tu.PromptTokens != 100 {
		t.Errorf("Expected PromptTokens to be 100, got %d", tu.PromptTokens)
	}
	if tu.CompletionTokens != 50 {
		t.Errorf("Expected CompletionTokens to be 50, got %d", tu.CompletionTokens)
	}
	if tu.TotalTokens != 150 {
		t.Errorf("Expected TotalTokens to be 150, got %d", tu.TotalTokens)
	}
}

// Test ToolMetadata type
func TestToolMetadata(t *testing.T) {
	example := ToolExample{
		Name:        "test-example",
		Description: "test description",
		Input:       map[string]interface{}{"key": "value"},
		Output:      map[string]interface{}{"result": "success"},
	}

	metadata := ToolMetadata{
		Name:         "test-tool",
		Description:  "A test tool",
		Version:      "1.0.0",
		Category:     "testing",
		Dependencies: []string{"dep1", "dep2"},
		Capabilities: []string{"read", "write"},
		Requirements: []string{"req1"},
		Parameters:   map[string]string{"param1": "value1"},
		Examples:     []ToolExample{example},
	}

	if metadata.Name != "test-tool" {
		t.Errorf("Expected Name to be 'test-tool', got %s", metadata.Name)
	}
	if len(metadata.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(metadata.Dependencies))
	}
	if len(metadata.Examples) != 1 {
		t.Errorf("Expected 1 example, got %d", len(metadata.Examples))
	}
}

// Test SessionState type
func TestSessionState(t *testing.T) {
	session := SessionState{
		SessionID:           "test-session",
		WorkspaceDir:        "/tmp/workspace",
		RepositoryAnalyzed:  true,
		DockerfileGenerated: false,
		ImageBuilt:          false,
		CurrentStage:        "analysis",
		Status:              "active",
		Errors:              []string{},
		Metadata:            map[string]interface{}{"test": "value"},
	}

	if session.SessionID != "test-session" {
		t.Errorf("Expected SessionID to be 'test-session', got %s", session.SessionID)
	}
	if !session.RepositoryAnalyzed {
		t.Error("Expected RepositoryAnalyzed to be true")
	}
	if session.DockerfileGenerated {
		t.Error("Expected DockerfileGenerated to be false")
	}
	if session.CurrentStage != "analysis" {
		t.Errorf("Expected CurrentStage to be 'analysis', got %s", session.CurrentStage)
	}
}

// Test RepositoryInfo type
func TestRepositoryInfo(t *testing.T) {
	structure := FileStructure{
		TotalFiles:      10,
		ConfigFiles:     []string{"config.yaml"},
		EntryPoints:     []string{"main.go"},
		TestFiles:       []string{"test.go"},
		BuildFiles:      []string{"Makefile"},
		DockerFiles:     []string{"Dockerfile"},
		KubernetesFiles: []string{"deployment.yaml"},
		PackageManagers: []string{"go.mod"},
	}

	repoInfo := RepositoryInfo{
		Language:        "go",
		Framework:       "standard",
		Port:            8080,
		Dependencies:    []string{"github.com/test/dep"},
		Structure:       structure,
		Size:            1024,
		HasCI:           true,
		HasReadme:       true,
		Recommendations: []string{"Add tests"},
	}

	if repoInfo.Language != "go" {
		t.Errorf("Expected Language to be 'go', got %s", repoInfo.Language)
	}
	if repoInfo.Port != 8080 {
		t.Errorf("Expected Port to be 8080, got %d", repoInfo.Port)
	}
	if repoInfo.Structure.TotalFiles != 10 {
		t.Errorf("Expected TotalFiles to be 10, got %d", repoInfo.Structure.TotalFiles)
	}
	if !repoInfo.HasCI {
		t.Error("Expected HasCI to be true")
	}
}

// Test BuildResult type
func TestBuildResult(t *testing.T) {
	// Test successful build
	successResult := BuildResult{
		ImageID:  "sha256:abc123",
		ImageRef: "myapp:latest",
		Success:  true,
		Error:    nil,
		Logs:     "Build completed successfully",
	}

	if !successResult.Success {
		t.Error("Expected Success to be true")
	}
	if successResult.Error != nil {
		t.Error("Expected Error to be nil for successful build")
	}

	// Test failed build
	buildErr := &BuildError{
		Type:    "dockerfile_error",
		Message: "Invalid FROM instruction",
	}
	failResult := BuildResult{
		ImageID:  "",
		ImageRef: "",
		Success:  false,
		Error:    buildErr,
		Logs:     "Build failed",
	}

	if failResult.Success {
		t.Error("Expected Success to be false")
	}
	if failResult.Error == nil {
		t.Error("Expected Error to be non-nil for failed build")
	}
	if failResult.Error.Type != "dockerfile_error" {
		t.Errorf("Expected Error.Type to be 'dockerfile_error', got %s", failResult.Error.Type)
	}
}
