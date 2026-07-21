import { createOpenAI } from '@ai-sdk/openai';
import type { LanguageModel } from 'ai';

export interface ResolvedModel {
  model: LanguageModel;
  providerOptions?: Record<string, Record<string, unknown>>;
}

/**
 * Per-request wall-clock ceiling for a single model HTTP call. The AI SDK's
 * OpenAI provider has no default request timeout, so a stalled connection to
 * the Foundry deployment hangs silently until the OS socket timeout (observed
 * as 20+ minute dead-air gaps in CI). Bounding each request lets a stalled call
 * abort and be retried by the driver's backoff instead of blocking a whole
 * eval cell. Override with `AGENT_EVAL_REQUEST_TIMEOUT_MS`.
 */
const REQUEST_TIMEOUT_MS =
  Number.parseInt(process.env.AGENT_EVAL_REQUEST_TIMEOUT_MS ?? '', 10) || 180_000;

/**
 * `fetch` wrapper that aborts any request exceeding {@link REQUEST_TIMEOUT_MS}.
 * A caller-supplied `signal` (e.g. the SDK's own abort) is chained so either
 * source can cancel the request.
 */
const timeoutFetch: typeof fetch = (input, init) => {
  const controller = new AbortController();
  const timer = setTimeout(
    () => controller.abort(new Error(`model request exceeded ${REQUEST_TIMEOUT_MS}ms timeout`)),
    REQUEST_TIMEOUT_MS,
  );
  const callerSignal = init?.signal;
  if (callerSignal) {
    if (callerSignal.aborted) controller.abort(callerSignal.reason);
    else callerSignal.addEventListener('abort', () => controller.abort(callerSignal.reason), { once: true });
  }
  return fetch(input, { ...init, signal: controller.signal }).finally(() => clearTimeout(timer));
};

export function validateProviderEnv(): void {
  if (!process.env.AZURE_FOUNDRY_API_KEY) {
    throw new Error(
      'AZURE_FOUNDRY_API_KEY not set. Create a .env file at the repo root with AZURE_FOUNDRY_API_KEY and AZURE_FOUNDRY_ENDPOINT.',
    );
  }
  if (!process.env.AZURE_FOUNDRY_ENDPOINT) {
    throw new Error(
      "AZURE_FOUNDRY_ENDPOINT not set (e.g. 'https://<resource>.services.ai.azure.com/').",
    );
  }
}

export function getModel(spec: string): ResolvedModel {
  const [provider, ...rest] = spec.split(':');
  const deployment = rest.join(':');
  if (!provider || !deployment) {
    throw new Error(`Invalid model spec '${spec}'. Expected '<provider>:<deployment-name>'.`);
  }
  if (provider !== 'azure' && provider !== 'foundry') {
    throw new Error(
      `Unknown provider '${provider}'. Supported: azure, foundry.`,
    );
  }

  const apiKey = process.env.AZURE_FOUNDRY_API_KEY;
  if (!apiKey) {
    throw new Error(
      'AZURE_FOUNDRY_API_KEY not set. Create a .env file at the repo root with AZURE_FOUNDRY_API_KEY and AZURE_FOUNDRY_ENDPOINT.',
    );
  }
  const rawEndpoint = process.env.AZURE_FOUNDRY_ENDPOINT;
  if (!rawEndpoint) {
    throw new Error(
      "AZURE_FOUNDRY_ENDPOINT not set (e.g. 'https://<resource>.services.ai.azure.com/').",
    );
  }
  // Normalize: accept the bare resource URL from the Azure portal and append
  // `/openai/v1` if missing. Trailing slash is tolerated either way.
  const trimmed = rawEndpoint.replace(/\/+$/, '');
  const baseURL = /\/openai\/v\d+$/.test(trimmed) ? trimmed : `${trimmed}/openai/v1`;

  const model = createOpenAI({ apiKey, baseURL, fetch: timeoutFetch }).chat(deployment);
  // Partner models (Llama et al.) only support one tool call per turn.
  if (provider === 'foundry') {
    return { model, providerOptions: { openai: { parallelToolCalls: false } } };
  }
  return { model };
}
