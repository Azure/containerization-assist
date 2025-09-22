import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/prompts/templates';
import { applyPolicyConstraints } from '@/config/policy-prompt';
import { fixDockerfileSchema, type FixDockerfileParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function fixDockerfile(
  params: FixDockerfileParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = fixDockerfileSchema.parse(params);
  const { dockerfile: content = '', targetEnvironment: environment = 'production' } =
    validatedParams;

  // Generate prompt from template
  const issues = [
    'Security vulnerabilities',
    'Size optimizations needed',
    'Layer ordering issues',
    'Best practices violations',
  ];
  const prompt = promptTemplates.fix('dockerfile', content, issues);

  // Apply policy constraints
  const constrained = applyPolicyConstraints(prompt, {
    tool: 'fix-dockerfile',
    environment,
  });

  // Execute via AI
  const response = await context.sampling.createMessage({
    messages: [
      {
        role: 'user',
        content: [{ type: 'text', text: constrained }],
      },
    ],
    maxTokens: 4096,
    modelPreferences: {
      hints: [{ name: 'dockerfile-optimization' }],
    },
  });

  // Return result
  try {
    const responseText = response.content[0]?.text || '';
    return Success({ fixedContent: responseText });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'fix-dockerfile',
  description: 'Fix and optimize existing Dockerfiles',
  version: '2.0.0',
  aiDriven: true,
};
