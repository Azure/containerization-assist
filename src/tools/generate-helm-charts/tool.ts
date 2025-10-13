/**
 * Generate Helm Charts tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import { promptTemplates, type HelmChartPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { scoreHelmChart } from '@/lib/scoring';
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
  const { chartVersion = '0.1.0', description } = input;

  const appName = input.appName;
  const imageId = input.imageId;

  // Use chartName from input or default to appName
  const chartName = input.chartName || appName;

  if (!chartName) {
    return Failure(
      'Chart name or application name is required. Provide chartName or appName parameter.',
    );
  }
  if (!imageId) {
    return Failure('Container image is required. Provide imageId parameter.');
  }

  try {
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

    // Execute via AI with deterministic sampling
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
      {},
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
        score: samplingResult.value.score,
        scoreBreakdown: samplingResult.value.scoreBreakdown,
      },
      'Helm chart generated with sampling',
    );

    return Success({ charts: responseText });
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(`Helm chart generation failed: ${errorMessage}`);
  }
}

const tool: MCPTool<typeof generateHelmChartsSchema, AIResponse> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateHelmChartsSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'helm-templating', 'chart-optimization'],
  },
  run,
};

export default tool;
