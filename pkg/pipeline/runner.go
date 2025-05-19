package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/container-copilot/pkg/logger"
)

const (
	dockerPipeline string = "docker"
)

// NewRunner constructs a Runner. You must pass a non-empty order;
// All stages' Init are called in order first
// then all stages' Generate are called in order, and finally
// stages' Run methods are executed in order, respecting the flow
// as defined in the StageConfigs (MaxRetries and OnFailGoto).
func NewRunner(stageConfigs []*StageConfig, out io.Writer) *Runner {
	if len(stageConfigs) == 0 {
		panic("pipeline order must be non-empty")
	}
	// Ensure no nil stages
	// Populate the id2Stage map
	// Fill in missing OnSuccessGotos
	id2Stage := make(map[string]*StageConfig)
	var prevStageConfig *StageConfig
	for i, sc := range stageConfigs {
		if sc == nil {
			panic(fmt.Sprintf("pipeline StageConfig %d must not be nil", i))
		}
		if sc.Stage == nil {
			panic(fmt.Sprintf("pipeline StageConfig.Stage %d must not be nil", i))
		}
		if _, exists := id2Stage[string(sc.Id)]; exists {
			panic(fmt.Sprintf("duplicate stage ID %s", sc.Id))
		}
		id2Stage[sc.Id] = sc
		// Backfill OnSuccessGoto if not set, with default to next stage
		if prevStageConfig != nil && prevStageConfig.OnSuccessGoto == "" {
			prevStageConfig.OnSuccessGoto = sc.Id //
		}
		// Backfill OnFailGoto if not set, with default to first stage
		if sc.OnFailGoto == "" {
			sc.OnFailGoto = stageConfigs[0].Id // Default to first stage
		}
		prevStageConfig = sc
	}
	// Second pass to ensure failure stages are valid now that id2Stage is populated
	for _, stage := range stageConfigs {
		if stage.OnFailGoto != "" {
			if _, exists := id2Stage[stage.OnFailGoto]; !exists {
				panic(fmt.Sprintf("invalid OnFailGoto id %s for stage %s", stage.OnFailGoto, stage.Id))
			}
		}
	}
	return &Runner{
		stageConfigs: stageConfigs,
		out:          out,
		id2Stage:     id2Stage,
	}
}

// Run drives the full pipeline workflow: init → generate → iterate → finalize.
func (r *Runner) Run(
	ctx context.Context,
	state *PipelineState,
	opts RunnerOptions,
	clients interface{},
) error {
	// Initialize the pipeline stages in order
	for _, sc := range r.stageConfigs {
		if err := sc.Stage.Initialize(ctx, state, sc.Path); err != nil {
			return fmt.Errorf("initializing stage %s: %w", sc.Id, err)
		}
	}
	// Generate artifacts for each stage in order
	for _, sc := range r.stageConfigs {
		logger.Infof("🔧 Generating artifacts for %s...", sc.Id)
		// ensure the pipeline exists
		if err := sc.Stage.Generate(ctx, state, opts.TargetDirectory); err != nil {
			return fmt.Errorf("generate %s: %w", sc.Id, err)
		}
	}

	// Advance Stages until all are successful or max iterations reached
	if r.stageConfigs == nil {
		return errors.New("no stages to run")
	}
	if r.stageConfigs[0].Stage == nil {
		return errors.New("first stage is nil")
	}
	if r.stageConfigs[0].Id == "" {
		return errors.New("first stage ID is empty")
	}
	currentStageConfig := r.stageConfigs[0]
	for {
		state.IterationCount++
		if ctx.Err() != nil {
			return fmt.Errorf("abort iterating with context err: %w", ctx.Err())
		}
		stage := currentStageConfig.Stage
		if state.RetryCount == 0 {
			fmt.Fprintf(r.out, "=== Running stage %s (iteration %d) ===", currentStageConfig.Id, state.IterationCount)
		} else {
			fmt.Fprintf(r.out, "  === Retrying stage %s %d/%d  (iteration %d) ===", currentStageConfig.Id, state.RetryCount, currentStageConfig.MaxRetries, state.IterationCount)
		}

		err := stage.Run(ctx, state, clients, opts)
		if err != nil {
			state.RetryCount++
			if state.RetryCount > currentStageConfig.MaxRetries {
				// If max retries reached, move to failed stage
				currentStageConfig = r.id2Stage[currentStageConfig.OnFailGoto]
				fmt.Fprintf(r.out, "❌ Stage %s failed max times %d: %v\n", currentStageConfig.Id, state.RetryCount, err)
				continue
			}
			fmt.Fprintf(r.out, "  ❌ Stage %s failed: %v\n", currentStageConfig.Id, err)
			continue
		}
		fmt.Fprintf(r.out, "  ✅ Stage %s succeeded, deploying...\n", currentStageConfig.Id)
		err = stage.Deploy(ctx, state, clients)
		if err != nil {
			fmt.Fprintf(r.out, "⚠️ Deploy failed for stage %s: %v\n", currentStageConfig.Id, err)
		}
		// If the stage succeeded, reset retry count
		state.RetryCount = 0
		nextStageId := currentStageConfig.OnSuccessGoto
		if nextStageId == "" {
			// If no next stage, we are done
			fmt.Fprintln(r.out, "🎉 All stages completed successfully!")
			state.Success = true
			break
		}
		// Move to next stage
		currentStageConfig = r.id2Stage[nextStageId]
	}

	if err := r.updateFiles(state); err != nil {
		fmt.Fprintf(r.out, "⚠️ Warning: %v\n", err)
	}
	return nil
}

func (r *Runner) updateFiles(state *PipelineState) error {
	var errs []string
	for _, p := range r.stageConfigs {
		if err := p.Stage.WriteSuccessfulFiles(state); err != nil {
			errs = append(errs, fmt.Sprintf("%T: %v", p, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("write successful files errors: %s", strings.Join(errs, "; "))
	}
	fmt.Fprintln(r.out, "\n🎉 Updated files for successful pipelines!")
	return nil
}
