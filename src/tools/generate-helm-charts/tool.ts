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
import { scoreHelmChart } from '@/lib/scoring';
import { generateHelmChartsSchema } from './schema';
import { getWorkflowSession } from '@/lib/tool-helpers';
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

  // Get session once for all session operations
  const sessionResult = input.sessionId ? getWorkflowSession(ctx, name) : null;
  const session = sessionResult?.ok ? sessionResult.value : null;

  // Retrieve appName from session if not provided
  let appName = input.appName;
  if (!appName && session) {
    appName = session.get<string>('appName');
    if (appName) {
      ctx.logger.info({ appName }, 'Using app name from session (analyze-repo)');
    }
  }

  // Retrieve imageId from session if not provided
  let imageId = input.imageId;
  if (!imageId && session) {
    const buildResult = session.getResult<{ tags?: string[] }>('build-image');
    if (buildResult?.tags && buildResult.tags.length > 0) {
      imageId = buildResult.tags[0];
      ctx.logger.info({ imageId }, 'Using image from session (build-image)');
    }
  }

  // Retrieve port from session if not explicitly provided in input
  let servicePort: number | undefined = input.port;
  if (!servicePort || servicePort === 8080) {
    // If port is default or not provided, try to get from session
    if (session) {
      const appPorts = session.get<number[]>('appPorts');
      if (appPorts && appPorts.length > 0) {
        servicePort = appPorts[0];
        ctx.logger.info({ port: servicePort }, 'Using port from session (analyze-repo)');
      }
    }
  }

  // Use chartName from input or default to appName
  const chartName = input.chartName || appName;

  // Validate required parameters
  if (!chartName) {
    return Failure(
      'Chart name or application name is required. Either provide chartName/appName parameter or run analyze-repo first with a sessionId.',
    );
  }
  if (!imageId) {
    return Failure(
      'Container image is required. Either provide imageId parameter or run build-image first with a sessionId.',
    );
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

const tool: Tool<typeof generateHelmChartsSchema, AIResponse> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateHelmChartsSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'helm-templating', 'chart-optimization'],
  },
  run,
};

export default tool;
