package pipeline

import (
	"context"

	"github.com/Azure/container-copilot/pkg/ai"
)

type trackedAIClient struct {
	base    ai.LLMClient
	state   *PipelineState
	stageID string
	opts    RunnerOptions
}

func WrapForTracking(base ai.LLMClient, state *PipelineState, stageID string, opts RunnerOptions) ai.LLMClient {
	return &trackedAIClient{
		base:    base,
		state:   state,
		stageID: stageID,
		opts:    opts,
	}
}

func (t *trackedAIClient) GetChatCompletion(ctx context.Context, prompt string) (string, ai.TokenUsage, error) {
	resp, usage, err := t.base.GetChatCompletion(ctx, prompt)

	if err == nil {
		// Always accumulate global token usage
		t.state.TokenUsage.PromptTokens += usage.PromptTokens
		t.state.TokenUsage.CompletionTokens += usage.CompletionTokens
		t.state.TokenUsage.TotalTokens += usage.TotalTokens
	}

	// Only store detailed snapshot if enabled and successful
	if err == nil && t.opts.GenerateSnapshot {
		t.state.LLMCompletions = append(t.state.LLMCompletions, ai.LLMCompletion{
			StageID:    t.stageID,
			Iteration:  t.state.IterationCount,
			Response:   resp,
			TokenUsage: usage,
		})
	}

	return resp, usage, err
}

func (t *trackedAIClient) GetChatCompletionWithFileTools(ctx context.Context, prompt, baseDir string) (string, ai.TokenUsage, error) {
	resp, usage, err := t.base.GetChatCompletionWithFileTools(ctx, prompt, baseDir)

	if err == nil {
		t.state.TokenUsage.PromptTokens += usage.PromptTokens
		t.state.TokenUsage.CompletionTokens += usage.CompletionTokens
		t.state.TokenUsage.TotalTokens += usage.TotalTokens
	}

	if err == nil && t.opts.GenerateSnapshot {
		t.state.LLMCompletions = append(t.state.LLMCompletions, ai.LLMCompletion{
			StageID:    t.stageID,
			Iteration:  t.state.IterationCount,
			Response:   resp,
			TokenUsage: usage,
		})
	}

	return resp, usage, err
}
func (t *trackedAIClient) GetChatCompletionWithFormat(ctx context.Context, prompt string, args ...interface{}) (string, ai.TokenUsage, error) {
	resp, usage, err := t.base.GetChatCompletionWithFormat(ctx, prompt, args...)

	if err == nil {
		t.state.TokenUsage.PromptTokens += usage.PromptTokens
		t.state.TokenUsage.CompletionTokens += usage.CompletionTokens
		t.state.TokenUsage.TotalTokens += usage.TotalTokens
	}

	if err == nil && t.opts.GenerateSnapshot {
		t.state.LLMCompletions = append(t.state.LLMCompletions, ai.LLMCompletion{
			StageID:    t.stageID,
			Iteration:  t.state.IterationCount,
			Response:   resp,
			TokenUsage: usage,
		})
	}

	return resp, usage, err
}

func (t *trackedAIClient) GetTokenUsage() ai.TokenUsage {
	return t.base.GetTokenUsage()
}

// AIClientInjectable can be implemented by stages that accept an LLMClient.
type AIClientInjectable interface {
	SetAIClient(client ai.LLMClient)
}
