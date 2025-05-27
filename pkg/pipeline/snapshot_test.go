package pipeline

import (
	"bytes"
	"context"
	"testing"
)

func TestSnapshotHistory(t *testing.T) {
	w := bytes.NewBuffer(nil)
	state := &PipelineState{}
	r := NewRunner([]*StageConfig{
		{
			Id: "fake-1", Stage: &FakeStage{},
		},
		{
			Id: "fake-2", Stage: &FakeStage{},
		},
	}, w)
	err := r.Run(context.Background(), state, RunnerOptions{
		GenerateSnapshot: true,
		TargetDirectory:  t.TempDir(),
	}, nil)
	if err != nil {
		t.Errorf("failed to run pipeline: %v", err)
	}

	hist := state.StageHistory
	if len(hist) != 2 {
		t.Errorf("expected 2 stages in history, got %d", len(hist))
	}
	if hist[0].StageID != "fake-1" {
		t.Errorf("expected stage ID 'fake-1', got %s", hist[0].StageID)
	}
	if hist[1].StageID != "fake-2" {
		t.Errorf("expected stage ID 'fake-2', got %s", hist[1].StageID)
	}
}

var _ PipelineStage = &FakeStage{}

type FakeStage struct{}

func (s *FakeStage) Initialize(ctx context.Context, state *PipelineState, path string) error {
	return nil
}
func (s *FakeStage) Generate(ctx context.Context, state *PipelineState, targetDir string) error {
	return nil
}
func (s *FakeStage) GetErrors(state *PipelineState) string {
	return ""
}
func (s *FakeStage) WriteSuccessfulFiles(state *PipelineState) error {
	return nil
}
func (s *FakeStage) Run(ctx context.Context, state *PipelineState, clients interface{}, options RunnerOptions) error {
	return nil
}
func (s *FakeStage) Deploy(ctx context.Context, state *PipelineState, clients interface{}) error {
	return nil
}
