/**
 * Generate Helm Charts tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates, type HelmChartPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { scoreHelmChart } from '@/lib/sampling';
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

  // Execute via AI with sampling and reranking
  const samplingResult = await sampleWithRerank(
    ctx,
    async (attemptIndex) => ({
      ...toMCPMessages(messages),
      maxTokens: 8192,
      modelPreferences: {
        hints: [{ name: 'helm-charts' }],
        intelligencePriority: 0.85,
        speedPriority: attemptIndex > 0 ? 0.8 : 0.5,
        costPriority: 0.3,
      },
    }),
    scoreHelmChart,
    { count: 3, stopAt: 85 },
  );

  if (!samplingResult.ok) {
    return Failure(`Helm chart generation failed: ${samplingResult.error}`);
  }

  const responseText = samplingResult.value.text;
  if (!responseText) {
    return Failure('Empty response from AI');
  }

  ctx.logger.info(
    {
      score: samplingResult.value.winner.score,
      scoreBreakdown: samplingResult.value.winner.scoreBreakdown,
    },
    'Helm chart generated with sampling',
  );

  return Success({ charts: responseText });
}

const tool: Tool<typeof generateHelmChartsSchema, AIResponse> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateHelmChartsSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'rerank',
    enhancementCapabilities: ['content-generation', 'helm-templating', 'chart-optimization'],
  },
  run,
};

export default tool;
