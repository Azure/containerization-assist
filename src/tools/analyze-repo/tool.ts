import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/prompts/templates';
import { buildMessages, toMCPMessages } from '@/ai/prompt-engine';
import { analyzeRepoSchema, type AnalyzeRepoParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function analyzeRepo(
  params: AnalyzeRepoParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = analyzeRepoSchema.parse(params);
  const { path: repoPath } = validatedParams;

  // Generate prompt from template
  const basePrompt = promptTemplates.repositoryAnalysis({
    fileList: `ls -la ${repoPath}`,
    configFiles: 'package.json, pom.xml, go.mod, requirements.txt',
    directoryTree: `tree ${repoPath}`,
  });

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: 'analyze_repository',
    tool: 'analyze-repo',
    environment: 'production',
    contract: {
      name: 'repository_analysis_v1',
      description: 'Analyze repository structure and return JSON',
    },
    knowledgeBudget: 2500,
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);
  const response = await context.sampling.createMessage({
    ...mcpMessages,
    maxTokens: 4096,
    modelPreferences: {
      hints: [{ name: 'code-analysis' }, { name: 'json-output' }],
    },
  });

  // Return parsed result
  const responseText = response.content[0]?.text || '';
  const jsonMatch = responseText.match(/\{[\s\S]*\}/);
  if (!jsonMatch) {
    return Failure('AI response did not contain valid JSON');
  }

  try {
    return Success(JSON.parse(jsonMatch[0]));
  } catch (e) {
    const error = e as Error;
    const invalidJson = (() => {
      try {
        return jsonMatch ? jsonMatch[0].substring(0, 200) : responseText.substring(0, 200);
      } catch {
        return '[unavailable]';
      }
    })();
    return Failure(
      `AI response parsing failed: ${error.message}\nInvalid JSON snippet: ${invalidJson}${
        jsonMatch[0].length > 200 ? '...' : ''
      }`,
    );
  }
}

export const metadata = {
  name: 'analyze-repo',
  description: 'Analyze repository structure and detect technologies',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
