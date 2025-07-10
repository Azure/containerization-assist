package di

import (
	"context"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/core/security"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// sessionStoreStub is a stub implementation
type sessionStoreStub struct{}

func (s *sessionStoreStub) Create(ctx context.Context, session *api.Session) error {
	return nil
}

func (s *sessionStoreStub) Get(ctx context.Context, sessionID string) (*api.Session, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (s *sessionStoreStub) Update(ctx context.Context, session *api.Session) error {
	return nil
}

func (s *sessionStoreStub) Delete(ctx context.Context, sessionID string) error {
	return nil
}

func (s *sessionStoreStub) List(ctx context.Context) ([]*api.Session, error) {
	return nil, nil
}

// sessionStateStub is a stub implementation
type sessionStateStub struct{}

func (s *sessionStateStub) SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error {
	return nil
}

func (s *sessionStateStub) GetState(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return nil, nil
}

func (s *sessionStateStub) CreateCheckpoint(ctx context.Context, sessionID string, name string) error {
	return nil
}

func (s *sessionStateStub) RestoreCheckpoint(ctx context.Context, sessionID string, name string) error {
	return nil
}

func (s *sessionStateStub) ListCheckpoints(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

func (s *sessionStateStub) GetWorkspaceDir(ctx context.Context, sessionID string) (string, error) {
	return "", nil
}

func (s *sessionStateStub) SetWorkspaceDir(ctx context.Context, sessionID string, dir string) error {
	return nil
}

func (s *sessionStateStub) GetSessionMetadata(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return nil, nil
}

func (s *sessionStateStub) UpdateSessionData(ctx context.Context, sessionID string, data map[string]interface{}) error {
	return nil
}

// buildExecutorStub is a stub implementation
type buildExecutorStub struct{}

// Remove old methods that are not part of the interface

func (b *buildExecutorStub) QuickBuild(ctx context.Context, dockerfileContent string, targetDir string, options docker.BuildOptions) (*docker.BuildResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (b *buildExecutorStub) QuickPush(ctx context.Context, imageRef string, options docker.PushOptions) (*docker.RegistryPushResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (b *buildExecutorStub) QuickPull(ctx context.Context, imageRef string) (*docker.PullResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

// toolRegistryServiceAdapter adapts api.ToolRegistry to services.ToolRegistry
type toolRegistryServiceAdapter struct {
	registry api.ToolRegistry
}

func (t *toolRegistryServiceAdapter) RegisterTool(_ context.Context, name string, tool api.Tool) error {
	// Wrap the tool as a factory function
	factory := func() api.Tool { return tool }
	return t.registry.Register(name, factory)
}

func (t *toolRegistryServiceAdapter) GetTool(_ context.Context, name string) (api.Tool, error) {
	result, err := t.registry.Discover(name)
	if err != nil {
		return nil, err
	}

	tool, ok := result.(api.Tool)
	if !ok {
		return nil, errors.NewError().
			Code(errors.CodeTypeMismatch).
			Message("registered item is not a tool").
			Context("name", name).
			Build()
	}

	return tool, nil
}

func (t *toolRegistryServiceAdapter) ListTools(_ context.Context) []string {
	return t.registry.List()
}

func (t *toolRegistryServiceAdapter) GetMetadata(name string) (*api.ToolMetadata, error) {
	metadata, err := t.registry.Metadata(name)
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (t *toolRegistryServiceAdapter) ExecuteGeneric(ctx context.Context, name string, input interface{}) (interface{}, error) {
	// Convert input to ToolInput
	toolInput := api.ToolInput{
		Data: map[string]interface{}{"input": input},
	}

	output, err := t.registry.Execute(ctx, name, toolInput)
	if err != nil {
		return nil, err
	}

	return output.Data, nil
}

func (t *toolRegistryServiceAdapter) GetMetrics(ctx context.Context) api.RegistryMetrics {
	// TODO: Implement metrics collection
	return api.RegistryMetrics{
		TotalTools:           len(t.registry.List()),
		ActiveTools:          len(t.registry.List()),
		TotalExecutions:      0,
		FailedExecutions:     0,
		AverageExecutionTime: 0,
		UpTime:               0,
	}
}

func (t *toolRegistryServiceAdapter) Close() error {
	return t.registry.Close()
}

func (t *toolRegistryServiceAdapter) Register(name string, factory interface{}) error {
	return t.registry.Register(name, factory)
}

func (t *toolRegistryServiceAdapter) Discover(name string) (interface{}, error) {
	return t.registry.Discover(name)
}

func (t *toolRegistryServiceAdapter) List() []string {
	return t.registry.List()
}

func (t *toolRegistryServiceAdapter) Metadata(name string) (api.ToolMetadata, error) {
	return t.registry.Metadata(name)
}

func (t *toolRegistryServiceAdapter) SetMetadata(name string, metadata api.ToolMetadata) error {
	return t.registry.SetMetadata(name, metadata)
}

func (t *toolRegistryServiceAdapter) Unregister(name string) error {
	return t.registry.Unregister(name)
}

func (t *toolRegistryServiceAdapter) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	return t.registry.Execute(ctx, name, input)
}

// workflowExecutorStub is a stub implementation
type workflowExecutorStub struct{}

// Removed duplicate methods

func (w *workflowExecutorStub) ExecuteWorkflow(ctx context.Context, workflow *api.Workflow) (*api.WorkflowResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (w *workflowExecutorStub) ExecuteStep(ctx context.Context, step *api.WorkflowStep) (*api.StepResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (w *workflowExecutorStub) ValidateWorkflow(ctx context.Context, workflow *api.Workflow) error {
	return nil
}

// Removed methods not in interface

// scannerStub is a stub implementation
type scannerStub struct{}

func (s *scannerStub) ScanImage(ctx context.Context, image string, options security.ScanOptionsService) (*security.ScanResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (s *scannerStub) ScanDirectory(ctx context.Context, path string, options security.ScanOptionsService) (*security.ScanResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (s *scannerStub) GetAvailableScanners(ctx context.Context) []string {
	return []string{"trivy", "grype"}
}

// configValidatorStub is a stub implementation
type configValidatorStub struct{}

func (c *configValidatorStub) ValidateDockerfile(ctx context.Context, content string) (*services.ValidationResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (c *configValidatorStub) ValidateManifest(ctx context.Context, content string) (*services.ValidationResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

func (c *configValidatorStub) ValidateConfig(ctx context.Context, config map[string]interface{}) (*services.ValidationResult, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("not implemented").Build()
}

// errorReporterStub is a stub implementation
type errorReporterStub struct{}

func (e *errorReporterStub) ReportError(ctx context.Context, err error, context map[string]interface{}) {
	// No-op
}

func (e *errorReporterStub) GetErrorStats(ctx context.Context) services.ErrorStats {
	return services.ErrorStats{}
}

func (e *errorReporterStub) SuggestFix(ctx context.Context, err error) []string {
	return nil
}
