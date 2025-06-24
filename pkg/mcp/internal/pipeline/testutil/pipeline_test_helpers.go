package testutil

import (
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/pipeline"
)

// MockMetadataManager provides a controllable mock for metadata operations
type MockMetadataManager struct {
	mu       sync.RWMutex
	metadata map[pipeline.MetadataKey]interface{}

	// Configuration
	GetFunc    func(key pipeline.MetadataKey) (interface{}, bool)
	SetFunc    func(key pipeline.MetadataKey, value interface{})
	ShouldFail bool
}

// NewMockMetadataManager creates a new mock metadata manager
func NewMockMetadataManager() *MockMetadataManager {
	return &MockMetadataManager{
		metadata: make(map[pipeline.MetadataKey]interface{}),
	}
}

// Get implements metadata retrieval
func (m *MockMetadataManager) Get(key pipeline.MetadataKey) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetFunc != nil {
		return m.GetFunc(key)
	}

	value, exists := m.metadata[key]
	return value, exists
}

// Set implements metadata storage
func (m *MockMetadataManager) Set(key pipeline.MetadataKey, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SetFunc != nil {
		m.SetFunc(key, value)
		return
	}

	m.metadata[key] = value
}

// GetString retrieves a string value from metadata
func (m *MockMetadataManager) GetString(key pipeline.MetadataKey) (string, bool) {
	value, exists := m.Get(key)
	if !exists {
		return "", false
	}

	if str, ok := value.(string); ok {
		return str, true
	}
	return "", false
}

// GetInt retrieves an integer value from metadata
func (m *MockMetadataManager) GetInt(key pipeline.MetadataKey) (int, bool) {
	value, exists := m.Get(key)
	if !exists {
		return 0, false
	}

	if i, ok := value.(int); ok {
		return i, true
	}
	return 0, false
}

// GetBool retrieves a boolean value from metadata
func (m *MockMetadataManager) GetBool(key pipeline.MetadataKey) (bool, bool) {
	value, exists := m.Get(key)
	if !exists {
		return false, false
	}

	if b, ok := value.(bool); ok {
		return b, true
	}
	return false, false
}

// Clear resets the metadata manager
func (m *MockMetadataManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metadata = make(map[pipeline.MetadataKey]interface{})
}

// GetAllMetadata returns a copy of all metadata
func (m *MockMetadataManager) GetAllMetadata() map[pipeline.MetadataKey]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[pipeline.MetadataKey]interface{})
	for k, v := range m.metadata {
		result[k] = v
	}
	return result
}

// TestAnalysisConverter provides test data builders for analysis conversion
type TestAnalysisConverter struct {
	predefinedAnalysis map[string]interface{}
	conversionResults  map[string]map[string]interface{}
}

// NewTestAnalysisConverter creates a new test analysis converter
func NewTestAnalysisConverter() *TestAnalysisConverter {
	return &TestAnalysisConverter{
		predefinedAnalysis: make(map[string]interface{}),
		conversionResults:  make(map[string]map[string]interface{}),
	}
}

// WithPredefinedAnalysis adds predefined analysis data for testing
func (c *TestAnalysisConverter) WithPredefinedAnalysis(key string, analysis interface{}) *TestAnalysisConverter {
	c.predefinedAnalysis[key] = analysis
	return c
}

// ToMap converts analysis to map format (mock implementation)
func (c *TestAnalysisConverter) ToMap(analysis interface{}) (map[string]interface{}, error) {
	// Try to find predefined result
	for key, predefined := range c.predefinedAnalysis {
		if predefined == analysis {
			if result, exists := c.conversionResults[key]; exists {
				return result, nil
			}
		}
	}

	// Default conversion for test
	return map[string]interface{}{
		"language":     "go",
		"framework":    "standard",
		"port":         8080,
		"dependencies": []string{"github.com/rs/zerolog"},
		"test_mode":    true,
	}, nil
}

// GetLanguage extracts language from analysis map
func (c *TestAnalysisConverter) GetLanguage(analysis map[string]interface{}) string {
	if lang, exists := analysis["language"]; exists {
		if langStr, ok := lang.(string); ok {
			return langStr
		}
	}
	return "unknown"
}

// GetFramework extracts framework from analysis map
func (c *TestAnalysisConverter) GetFramework(analysis map[string]interface{}) string {
	if framework, exists := analysis["framework"]; exists {
		if frameworkStr, ok := framework.(string); ok {
			return frameworkStr
		}
	}
	return "unknown"
}

// SetConversionResult sets a specific conversion result for testing
func (c *TestAnalysisConverter) SetConversionResult(key string, result map[string]interface{}) {
	c.conversionResults[key] = result
}

// MockInsightGenerator provides controllable insight generation for testing
type MockInsightGenerator struct {
	mu                     sync.RWMutex
	repositoryInsights     []string
	dockerInsights         []string
	manifestInsights       []string
	commonInsights         []string
	customInsightGenerator func(stage string, metadata *MockMetadataManager) []string
}

// NewMockInsightGenerator creates a new mock insight generator
func NewMockInsightGenerator() *MockInsightGenerator {
	return &MockInsightGenerator{
		repositoryInsights: []string{"Repository analysis completed successfully"},
		dockerInsights:     []string{"Docker build completed successfully"},
		manifestInsights:   []string{"Manifest generation completed successfully"},
		commonInsights:     []string{"Operation completed within expected time"},
	}
}

// WithRepositoryInsights sets custom repository insights
func (g *MockInsightGenerator) WithRepositoryInsights(insights []string) *MockInsightGenerator {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.repositoryInsights = insights
	return g
}

// WithDockerInsights sets custom Docker insights
func (g *MockInsightGenerator) WithDockerInsights(insights []string) *MockInsightGenerator {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.dockerInsights = insights
	return g
}

// WithManifestInsights sets custom manifest insights
func (g *MockInsightGenerator) WithManifestInsights(insights []string) *MockInsightGenerator {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.manifestInsights = insights
	return g
}

// WithCommonInsights sets custom common insights
func (g *MockInsightGenerator) WithCommonInsights(insights []string) *MockInsightGenerator {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.commonInsights = insights
	return g
}

// WithCustomGenerator sets a custom insight generator function
func (g *MockInsightGenerator) WithCustomGenerator(generator func(stage string, metadata *MockMetadataManager) []string) *MockInsightGenerator {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.customInsightGenerator = generator
	return g
}

// GenerateRepositoryInsights generates insights for repository analysis
func (g *MockInsightGenerator) GenerateRepositoryInsights(metadata *MockMetadataManager) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.customInsightGenerator != nil {
		return g.customInsightGenerator("repository", metadata)
	}

	return copyStringSlice(g.repositoryInsights)
}

// GenerateDockerInsights generates insights for Docker operations
func (g *MockInsightGenerator) GenerateDockerInsights(metadata *MockMetadataManager) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.customInsightGenerator != nil {
		return g.customInsightGenerator("docker", metadata)
	}

	return copyStringSlice(g.dockerInsights)
}

// GenerateManifestInsights generates insights for manifest operations
func (g *MockInsightGenerator) GenerateManifestInsights(metadata *MockMetadataManager) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.customInsightGenerator != nil {
		return g.customInsightGenerator("manifest", metadata)
	}

	return copyStringSlice(g.manifestInsights)
}

// GenerateCommonInsights generates common insights
func (g *MockInsightGenerator) GenerateCommonInsights(metadata *MockMetadataManager) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.customInsightGenerator != nil {
		return g.customInsightGenerator("common", metadata)
	}

	return copyStringSlice(g.commonInsights)
}

// PipelineStateBuilder provides a builder pattern for creating complex pipeline states
type PipelineStateBuilder struct {
	state *pipeline.PipelineState
}

// NewPipelineStateBuilder creates a new pipeline state builder
func NewPipelineStateBuilder() *PipelineStateBuilder {
	return &PipelineStateBuilder{
		state: &pipeline.PipelineState{
			Metadata: make(map[pipeline.MetadataKey]interface{}),
		},
	}
}

// WithImageName sets the image name
func (b *PipelineStateBuilder) WithImageName(imageName string) *PipelineStateBuilder {
	b.state.ImageName = imageName
	return b
}

// WithRegistryURL sets the registry URL
func (b *PipelineStateBuilder) WithRegistryURL(registryURL string) *PipelineStateBuilder {
	b.state.RegistryURL = registryURL
	return b
}

// WithRepoFileTree sets the repository file tree
func (b *PipelineStateBuilder) WithRepoFileTree(fileTree string) *PipelineStateBuilder {
	b.state.RepoFileTree = fileTree
	return b
}

// WithExtraContext sets the extra context
func (b *PipelineStateBuilder) WithExtraContext(context string) *PipelineStateBuilder {
	b.state.ExtraContext = context
	return b
}

// WithMetadata adds metadata entries
func (b *PipelineStateBuilder) WithMetadata(key pipeline.MetadataKey, value interface{}) *PipelineStateBuilder {
	b.state.Metadata[key] = value
	return b
}

// WithAnalysisResult adds repository analysis result metadata
func (b *PipelineStateBuilder) WithAnalysisResult(analysis map[string]interface{}) *PipelineStateBuilder {
	b.state.Metadata[pipeline.RepoAnalysisResultKey] = analysis
	return b
}

// WithSessionMetadata adds session-related metadata
func (b *PipelineStateBuilder) WithSessionMetadata(sessionID string, createdAt, updatedAt time.Time) *PipelineStateBuilder {
	b.state.Metadata[pipeline.MetadataKey("mcp_session_id")] = sessionID
	b.state.Metadata[pipeline.MetadataKey("session_created_at")] = createdAt
	b.state.Metadata[pipeline.MetadataKey("session_updated_at")] = updatedAt
	return b
}

// WithDockerfile adds Dockerfile information
func (b *PipelineStateBuilder) WithDockerfile(content, path string) *PipelineStateBuilder {
	b.state.Dockerfile.Content = content
	b.state.Dockerfile.Path = path
	return b
}

// Build creates the pipeline state
func (b *PipelineStateBuilder) Build() *pipeline.PipelineState {
	// Return a copy to avoid mutation issues
	result := &pipeline.PipelineState{
		ImageName:    b.state.ImageName,
		RegistryURL:  b.state.RegistryURL,
		RepoFileTree: b.state.RepoFileTree,
		ExtraContext: b.state.ExtraContext,
		Dockerfile:   b.state.Dockerfile,
		Metadata:     make(map[pipeline.MetadataKey]interface{}),
	}

	// Copy metadata
	for k, v := range b.state.Metadata {
		result.Metadata[k] = v
	}

	return result
}

// SessionTestHelpers provides utilities for session state testing
type SessionTestHelpers struct {
	sessionStates map[string]interface{}
}

// NewSessionTestHelpers creates new session test helpers
func NewSessionTestHelpers() *SessionTestHelpers {
	return &SessionTestHelpers{
		sessionStates: make(map[string]interface{}),
	}
}

// CreateMockSessionState creates a mock session state for testing
func (h *SessionTestHelpers) CreateMockSessionState(sessionID string) interface{} {
	mockState := map[string]interface{}{
		"session_id":    sessionID,
		"created_at":    time.Now(),
		"last_accessed": time.Now(),
		"workspace_dir": "/tmp/test-workspace/" + sessionID,
		"repo_analysis": map[string]interface{}{
			"language":  "go",
			"framework": "standard",
			"port":      8080,
		},
		"dockerfile": map[string]interface{}{
			"content": "FROM golang:1.21\nWORKDIR /app\nCOPY . .\nRUN go build -o app\nEXPOSE 8080\nCMD [\"./app\"]",
			"path":    "/tmp/test-workspace/" + sessionID + "/Dockerfile",
			"built":   true,
		},
		"image_ref": map[string]interface{}{
			"registry":   "localhost:5000",
			"repository": "test/app",
			"tag":        "latest",
		},
		"build_logs": []string{
			"Build started",
			"Dependencies downloaded",
			"Build completed successfully",
		},
		"k8s_manifests": map[string]interface{}{
			"deployment": map[string]interface{}{
				"name":    "deployment",
				"kind":    "Deployment",
				"content": "",
				"applied": false,
				"status":  "generated",
			},
		},
		"labels": []string{"test", "mock"},
	}

	h.sessionStates[sessionID] = mockState
	return mockState
}

// GetMockSessionState retrieves a mock session state
func (h *SessionTestHelpers) GetMockSessionState(sessionID string) (interface{}, bool) {
	state, exists := h.sessionStates[sessionID]
	return state, exists
}

// UpdateMockSessionState updates a mock session state
func (h *SessionTestHelpers) UpdateMockSessionState(sessionID string, updateFunc func(state map[string]interface{})) {
	if state, exists := h.sessionStates[sessionID]; exists {
		if stateMap, ok := state.(map[string]interface{}); ok {
			updateFunc(stateMap)
		}
	}
}

// CreateRepositoryAnalysisState creates a pipeline state for repository analysis testing
func (h *SessionTestHelpers) CreateRepositoryAnalysisState(sessionID, targetRepo, extraContext string) *pipeline.PipelineState {
	builder := NewPipelineStateBuilder()
	return builder.
		WithRepoFileTree("mock file tree for "+targetRepo).
		WithExtraContext(extraContext).
		WithSessionMetadata(sessionID, time.Now().Add(-1*time.Hour), time.Now()).
		WithAnalysisResult(map[string]interface{}{
			"language":     "go",
			"framework":    "standard",
			"port":         8080,
			"dependencies": []string{"github.com/rs/zerolog"},
		}).
		Build()
}

// CreateDockerState creates a pipeline state for Docker operations testing
func (h *SessionTestHelpers) CreateDockerState(sessionID, imageName, registryURL string) *pipeline.PipelineState {
	builder := NewPipelineStateBuilder()
	return builder.
		WithImageName(imageName).
		WithRegistryURL(registryURL).
		WithRepoFileTree("mock file tree").
		WithSessionMetadata(sessionID, time.Now().Add(-1*time.Hour), time.Now()).
		WithAnalysisResult(map[string]interface{}{
			"language":  "go",
			"framework": "standard",
			"port":      8080,
		}).
		WithDockerfile("FROM golang:1.21\nWORKDIR /app\nCOPY . .\nEXPOSE 8080", "/tmp/Dockerfile").
		Build()
}

// CreateManifestState creates a pipeline state for manifest operations testing
func (h *SessionTestHelpers) CreateManifestState(sessionID, namespace string) *pipeline.PipelineState {
	builder := NewPipelineStateBuilder()
	return builder.
		WithImageName("localhost:5000/test/app:latest").
		WithRepoFileTree("mock file tree").
		WithSessionMetadata(sessionID, time.Now().Add(-1*time.Hour), time.Now()).
		WithAnalysisResult(map[string]interface{}{
			"language":  "go",
			"framework": "standard",
			"port":      8080,
		}).
		WithDockerfile("FROM golang:1.21\nWORKDIR /app", "/tmp/Dockerfile").
		WithMetadata("namespace", namespace).
		Build()
}

// Utility functions

func copyStringSlice(src []string) []string {
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
