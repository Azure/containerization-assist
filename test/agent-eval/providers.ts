import { createOpenAI } from '@ai-sdk/openai';
import type { LanguageModel } from 'ai';

export interface ResolvedModel {
  model: LanguageModel;
  providerOptions?: Record<string, Record<string, unknown>>;
}

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

  const model = createOpenAI({ apiKey, baseURL }).chat(deployment);
  // Partner models (Llama et al.) only support one tool call per turn.
  if (provider === 'foundry') {
    return { model, providerOptions: { openai: { parallelToolCalls: false } } };
  }
  return { model };
}
