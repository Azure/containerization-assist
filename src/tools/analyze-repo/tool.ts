import path from 'node:path';
import { Success, Failure, type Result, type ToolContext } from '@/types';
import { analyzeRepoSchema, type RepositoryAnalysis } from './schema';
import type { MCPTool } from '@/types/tool';
import type { z } from 'zod';

/**
 * Analyze repository structure and detect technologies
 */
async function run(
  input: z.infer<typeof analyzeRepoSchema>,
  _ctx: ToolContext,
): Promise<Result<RepositoryAnalysis>> {
  let { repositoryPathAbsoluteUnix: repoPath } = input;

  // Convert to absolute path if relative
  if (!path.isAbsolute(repoPath)) {
    repoPath = path.resolve(process.cwd(), repoPath);
  }

  try {
    const result: RepositoryAnalysis = input;
    const numberOfModules = result.modules ? result.modules.length : 0;
    if (numberOfModules === 0) {
      throw new Error(
        'No modules found in the repository. Please supply at least one module for structured analysis.',
      );
    }
    result.isMonorepo = numberOfModules > 0;

    // Add workflowHints to the result
    const moduleHint = result.isMonorepo
      ? ` Detected ${numberOfModules} modules that can be containerized separately.`
      : '';

    return Success({
      ...result,
      analyzedPath: repoPath,
      workflowHints: {
        message: `Repository analysis complete. ${moduleHint}`,
      },
    });
  } catch (e) {
    const error = e as Error;
    return Failure(error.message);
  }
}

const tool: MCPTool<typeof analyzeRepoSchema, RepositoryAnalysis> = {
  name: 'analyze-repo',
  description: 'Analyze repository structure and detect technologies',
  version: '3.0.0',
  schema: analyzeRepoSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'analysis', 'technology-detection'],
  },
  run,
};

export default tool;
