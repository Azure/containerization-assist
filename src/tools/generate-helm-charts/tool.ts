/**
 * Generate Helm Charts tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates, type HelmChartPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { generateHelmChartsSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import type { z } from 'zod';

const name = 'generate-helm-charts';
const description = 'Generate Helm charts for Kubernetes deployments';
const version = '2.1.0';

async function run(
  input: z.infer<typeof generateHelmChartsSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const { chartName, appName, chartVersion = '0.1.0', description } = input;

  // Generate prompt from template
  const promptParams = {
    appName: chartName || appName,
    version: chartVersion,
    dependencies: [],
    values: {},
    description,
  } as HelmChartPromptParams;
  const basePrompt = promptTemplates.helmChart(promptParams);

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.GENERATE_HELM_CHARTS,
    tool: name,
    environment: input.environment || 'production',
    contract: {
      name: 'helm_chart_v1',
      description: 'Generate Helm charts for Kubernetes deployments',
    },
    knowledgeBudget: 4000, // Character budget for knowledge snippets
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await ctx.sampling.createMessage({
    ...mcpMessages, // Spreads the messages array
    maxTokens: 8192,
    modelPreferences: {
      hints: [{ name: 'helm-charts' }],
    },
  });

  try {
    const responseText = response.content[0]?.text || '';
    return Success({ charts: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

const tool: Tool<typeof generateHelmChartsSchema, AIResponse> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateHelmChartsSchema,
  run,
};

export default tool;

export const metadata = {
  name,
  description,
  version,
  aiDriven: true,
  knowledgeEnhanced: true,
};
