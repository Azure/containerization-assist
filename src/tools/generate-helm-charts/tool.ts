import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type HelmChartPromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { generateHelmChartsSchema, type GenerateHelmChartsParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function generateHelmCharts(
  params: GenerateHelmChartsParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = generateHelmChartsSchema.parse(params);
  const { chartName, appName, chartVersion = '0.1.0', description } = validatedParams;

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
    topic: 'generate_helm_charts',
    tool: 'generate-helm-charts',
    environment: validatedParams.environment || 'production',
    contract: {
      name: 'helm_chart_v1',
      description: 'Generate Helm charts for Kubernetes deployments',
    },
    knowledgeBudget: 4000, // Character budget for knowledge snippets
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
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

export const metadata = {
  name: 'generate-helm-charts',
  description: 'Generate Helm charts for Kubernetes deployments',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
