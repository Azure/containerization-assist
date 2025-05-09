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
// / it will drive init→generate→iterate→finalize in exactly this sequence.
func NewRunner(pipelineMap map[string]PipelineStage, order []string, out io.Writer) *Runner {
	if len(order) == 0 {
		panic("pipeline order must be non-empty")
	}
	return &Runner{
		stages: pipelineMap,
		order:  order,
		out:    out,
	}
}

// Run drives the full pipeline workflow: init → generate → iterate → finalize.
func (r *Runner) Run(
	ctx context.Context,
	state *PipelineState,
	pathMap map[string]string,
	opts RunnerOptions,
	clients interface{},
) error {
	if err := r.initialize(ctx, state, pathMap); err != nil {
		return err
	}
	if err := r.generate(ctx, state, opts.TargetDirectory); err != nil {
		return err
	}
	errs := r.iterate(ctx, state, opts.CompleteLoopMaxIterations, clients, opts)
	if err := r.updateFiles(state); err != nil {
		fmt.Fprintf(r.out, "⚠️ Warning: %v\n", err)
	}
	if len(errs) > 0 {
		return errors.New("pipeline errors:\n" + strings.Join(errs, "\n"))
	}
	return nil
}

func (r *Runner) initialize(ctx context.Context, state *PipelineState, pathMap map[string]string) error {
	for _, key := range r.order {
		p, exists := r.stages[key]
		if !exists {
			continue
		}
		path, ok := pathMap[key]
		if !ok {
			return fmt.Errorf("missing path for pipeline %q", key)
		}
		if err := p.Initialize(ctx, state, path); err != nil {
			return fmt.Errorf("initialize %s: %w", key, err)
		}
	}
	return nil
}

func (r *Runner) generate(ctx context.Context, state *PipelineState, targetDir string) error {
	for _, key := range r.order {
		logger.Infof("🔧 Generating artifacts for %s...", key)
		// ensure the pipeline exists
		p, exists := r.stages[key]
		if !exists {
			return fmt.Errorf("missing pipeline %q", key)
		}
		if err := p.Generate(ctx, state, targetDir); err != nil {
			return fmt.Errorf("generate %s: %w", key, err)
		}
	}
	return nil
}

func (r *Runner) iterate(
	ctx context.Context,
	state *PipelineState,
	completeLoopIterations int,
	clients interface{},
	opts RunnerOptions,
) []string {
	var allErrs []string
	success := make(map[string]bool)

	for i := 1; i <= completeLoopIterations; i++ {
		fmt.Fprintf(r.out, "\n=== Iteration %d/%d ===\n", i, completeLoopIterations)

		// Iterate through each pipeline for maxIterations
		iterErrs := r.runIteration(ctx, state, clients, success, opts)
		allErrs = append(allErrs, iterErrs...)
		if len(iterErrs) != 0 {
			// Return early on docker pipeline error
			if _, hasDocker := r.stages[dockerPipeline]; hasDocker && !success[dockerPipeline] {
				logger.Warnf("Docker pipeline failed; stopping iteration")
				break
			}
		}

		if len(iterErrs) == 0 {
			fmt.Fprintln(r.out, "🎉 All pipelines completed successfully!")
			state.Success = true
			break
		}
		fmt.Fprintln(r.out, "❌ Iteration completed with errors; retrying...")
	}

	return allErrs
}

func (r *Runner) runIteration(
	ctx context.Context,
	state *PipelineState,
	clients interface{},
	success map[string]bool,
	opts RunnerOptions,
) []string {
	var errs []string

	for _, key := range r.order {
		if success[key] {
			fmt.Fprintf(r.out, "⏭ Skipping %s (already succeeded)\n", key)
			continue
		}

		p := r.stages[key]
		if err := p.Run(ctx, state, clients, opts); err != nil {
			msg := fmt.Sprintf("%s run error: %v", key, err)
			fmt.Fprintf(r.out, "❌ %s failed: %v\n", key, err)
			// Fail fast on docker pipeline error
			if key == dockerPipeline {
				return []string{msg}
			}
			errs = append(errs, msg)
			continue
		}

		if report := p.GetErrors(state); report != "" {
			msg := fmt.Sprintf("%s reported errors:\n%s", key, report)
			errs = append(errs, msg)
			fmt.Fprintf(r.out, "❌ %s errors:\n%s\n", key, report)
			continue
		}

		success[key] = true
		fmt.Fprintf(r.out, "✅ %s succeeded\n", key)

		fmt.Fprintf(r.out, "🚀 Deploying %s...\n", key)
		if err := p.Deploy(ctx, state, clients); err != nil {
			msg := fmt.Sprintf("%s deploy error: %v", key, err)
			errs = append(errs, msg)
			fmt.Fprintf(r.out, "❌ %s deployment failed: %v\n", key, err)
		} else {
			fmt.Fprintf(r.out, "✅ %s deployed\n", key)
		}
	}

	return errs
}

func (r *Runner) updateFiles(state *PipelineState) error {
	var errs []string
	for _, p := range r.stages {
		if err := p.WriteSuccessfulFiles(state); err != nil {
			errs = append(errs, fmt.Sprintf("%T: %v", p, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("write successful files errors: %s", strings.Join(errs, "; "))
	}
	fmt.Fprintln(r.out, "\n🎉 Updated files for successful pipelines!")
	return nil
}
