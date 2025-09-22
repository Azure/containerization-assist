import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type HelmChartPromptParams } from '@/prompts/templates';
import { applyPolicyConstraints } from '@/config/policy-prompt';
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
  const prompt = promptTemplates.helmChart(promptParams);

  // Apply policy constraints
  const constrained = applyPolicyConstraints(prompt, {
    tool: 'generate-helm-charts',
    environment: 'production',
  });

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
  version: '2.0.0',
  aiDriven: true,
};
