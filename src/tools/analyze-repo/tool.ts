import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/prompts/templates';
import { applyPolicyConstraints } from '@/config/policy-prompt';
import { analyzeRepoSchema, type AnalyzeRepoParams } from './schema';
import type { AIResponse } from '../ai-response-types';

export async function analyzeRepo(
  params: AnalyzeRepoParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = analyzeRepoSchema.parse(params);
  const { path: repoPath } = validatedParams;

  // Generate prompt from template
  const prompt = promptTemplates.repositoryAnalysis({
    fileList: `ls -la ${repoPath}`,
    configFiles: 'package.json, pom.xml, go.mod, requirements.txt',
    directoryTree: `tree ${repoPath}`,
  });

  // Apply policy constraints
  const constrained = applyPolicyConstraints(prompt, {
    tool: 'analyze-repo',
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
    maxTokens: 4096,
    modelPreferences: {
      hints: [{ name: 'code-analysis' }, { name: 'json-output' }],
    },
  });

  // Return parsed result
  try {
    const responseText = response.content[0]?.text || '';
    const jsonMatch = responseText.match(/\{[\s\S]*\}/);
    if (!jsonMatch) {
      return Failure('AI response did not contain valid JSON');
    }
    return Success(JSON.parse(jsonMatch[0]));
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'analyze-repo',
  description: 'Analyze repository structure and detect technologies',
  version: '2.0.0',
  aiDriven: true,
};
