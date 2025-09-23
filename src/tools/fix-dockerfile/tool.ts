import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/prompts/templates';
import { buildPolicyConstraints } from '@/config/policy-prompt';
import { enhancePrompt } from '../knowledge-helper';
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
  const basePrompt = promptTemplates.fix('dockerfile', content, issues);

  // Enhance with knowledge base
  const enhancedPrompt = await enhancePrompt(basePrompt, 'fix_dockerfile', {
    environment,
  });

  // Apply policy constraints
  const constraints = buildPolicyConstraints({
    tool: 'fix-dockerfile',
    environment,
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
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
