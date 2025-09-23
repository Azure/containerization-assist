import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type HelmChartPromptParams } from '@/prompts/templates';
import { buildPolicyConstraints } from '@/config/policy-prompt';
import { enhancePrompt } from '../knowledge-helper';
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

  // Enhance with knowledge base
  const enhancedPrompt = await enhancePrompt(basePrompt, 'generate_helm_charts', {
    environment: 'production',
  });

  // Apply policy constraints
  const constraints = buildPolicyConstraints({
    tool: 'generate-helm-charts',
    environment: 'production',
  });
  const constrained =
    constraints.length > 0
      ? `${enhancedPrompt}\n\nPolicy Constraints:\n${constraints.join('\n')}`
      : enhancedPrompt;

  // Execute via AI
  const response = await context.sampling.createMessage({
    messages: [
      {
        role: 'user',
        content: [{ type: 'text', text: constrained }],
      },
    ],
    maxTokens: 8192,
    modelPreferences: {
      hints: [{ name: 'helm-charts' }],
    },
  });

  // Return result
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
