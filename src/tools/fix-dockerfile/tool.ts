import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/prompts/templates';
import { buildMessages, toMCPMessages } from '@/ai/prompt-engine';
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

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: 'fix_dockerfile',
    tool: 'fix-dockerfile',
    environment,
    contract: {
      name: 'dockerfile_fix_v1',
      description: 'Fix and optimize the Dockerfile',
    },
    knowledgeBudget: 2500,
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
    ...mcpMessages,
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
