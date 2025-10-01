import { promises as fs } from 'node:fs';
import path from 'node:path';
import { Success, Failure, type Result, type ToolContext, TOPICS } from '@/types';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { scoreRepositoryAnalysis } from '@/lib/scoring';
import { analyzeRepoSchema, type RepositoryAnalysis } from './schema';
import { extractJsonContent } from '@/lib/content-extraction';
import { storeToolResults } from '@/lib/tool-helpers';
import type { AIResponse } from '../ai-response-types';
import type { Tool } from '@/types/tool';
import type { z } from 'zod';

/**
 * Analyze repository structure and detect technologies
 */
async function run(
  input: z.infer<typeof analyzeRepoSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  let { path: repoPath } = input;
  const { sessionId } = input;

  // Convert to absolute path if relative
  if (!path.isAbsolute(repoPath)) {
    repoPath = path.resolve(process.cwd(), repoPath);
  }

  // SessionId is required - it should be provided by the orchestrator
  if (!sessionId) {
    return Failure(
      'sessionId is required for analyze-repo. The orchestrator should provide a sessionId.',
    );
  }

  // Read the actual repository files
  let fileList = '';
  let configContent = '';
  let directoryStructure = '';

  try {
    // Get file list
    const files = await fs.readdir(repoPath);
    fileList = files.join('\n');

    // Read common config files
    const configFiles = [
      'package.json',
      'pom.xml',
      'build.gradle',
      'requirements.txt',
      'go.mod',
      'Cargo.toml',
    ];
    for (const configFile of configFiles) {
      try {
        const content = await fs.readFile(path.join(repoPath, configFile), 'utf-8');
        configContent += `\n=== ${configFile} ===\n${content}\n`;
      } catch {
        // File doesn't exist
      }
    }

    // Get directory structure (simplified)
    const getDirStructure = async (dir: string, prefix = '', depth = 0): Promise<string> => {
      if (depth > 2) return '';
      let structure = '';
      const items = await fs.readdir(dir, { withFileTypes: true });
      for (const item of items) {
        if (item.name.startsWith('.') || item.name === 'node_modules') continue;
        structure += `${prefix}${item.name}${item.isDirectory() ? '/' : ''}\n`;
        if (item.isDirectory() && depth < 2) {
          structure += await getDirStructure(path.join(dir, item.name), `${prefix}  `, depth + 1);
        }
      }
      return structure;
    };
    directoryStructure = await getDirStructure(repoPath);
  } catch (error) {
    ctx.logger.error({ error, repoPath }, 'Failed to read repository files');
    fileList = 'Could not read directory';
    configContent = 'No config files found';
    directoryStructure = 'Could not read structure';
  }

  // Generate prompt with actual file contents
  const basePrompt = promptTemplates.repositoryAnalysis({
    fileList,
    configFiles: configContent,
    directoryTree: directoryStructure,
    sessionId,
  });

  // Log the generated prompt
  ctx.logger.info(
    {
      promptLength: basePrompt.length,
      promptPreview: basePrompt.substring(0, 500),
      fullPrompt: basePrompt,
    },
    'Generated analysis prompt',
  );

  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.ANALYZE_REPOSITORY,
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

  // Log the messages being sent to AI
  ctx.logger.info(
    {
      messageCount: mcpMessages.messages.length,
      messages: mcpMessages.messages.map((m) => ({
        role: m.role,
        contentLength: JSON.stringify(m.content).length,
        contentPreview: JSON.stringify(m.content).substring(0, 200),
      })),
    },
    'Sending messages to AI',
  );

  let response;
  try {
    response = await sampleWithRerank(
      ctx,
      async (attempt) => ({
        ...mcpMessages,
        maxTokens: 4096,
        modelPreferences: {
          hints: [{ name: 'repo-analysis' }],
          intelligencePriority: 0.9,
          speedPriority: attempt > 0 ? 0.7 : 0.4,
        },
      }),
      scoreRepositoryAnalysis,
      {},
    );
  } catch (error) {
    ctx.logger.error(
      { error: error instanceof Error ? error.message : String(error) },
      'AI sampling failed',
    );
    return Failure(`AI sampling failed: ${error instanceof Error ? error.message : String(error)}`);
  }

  if (!response.ok) {
    return Failure(`AI sampling failed: ${response.error}`);
  }

  // Return parsed result
  const responseText = response.value.text;
  ctx.logger.info(
    {
      responseLength: responseText.length,
      responsePreview: responseText.substring(0, 200),
      hasContent: responseText.length,
    },
    'Received AI response',
  );

  const jsonExtraction = extractJsonContent(responseText);
  if (!jsonExtraction) {
    ctx.logger.error(
      { responseText: responseText.substring(0, 500) },
      'AI response did not contain valid JSON',
    );
    return Failure('AI response did not contain valid JSON');
  }

  try {
    const analysisResult = jsonExtraction as RepositoryAnalysis;

    // Store the analysis result in session for other tools to use
    const result: RepositoryAnalysis & { sessionId: string } = {
      ...analysisResult,
      sessionId,
    };

    // Store repository path and key metadata in session for downstream tools
    if (ctx.session) {
      ctx.session.set('analyzedPath', repoPath);
      ctx.session.set('appName', result.name || path.basename(repoPath));
      if (result.ports && result.ports.length > 0) {
        ctx.session.set('appPorts', result.ports);
      }
      // Store the full analysis result for downstream tools
      ctx.session.storeResult('analyze-repo', result);

      // Store monorepo/multi-module information if detected
      if (result.isMonorepo === true && result.modules && result.modules.length > 0) {
        ctx.session.set('isMonorepo', true);
        ctx.session.set('modules', result.modules);
        ctx.logger.info(
          {
            sessionId,
            repoPath,
            appName: result.name,
            moduleCount: result.modules.length,
          },
          'Stored monorepo context with multiple modules in session',
        );
      } else {
        ctx.logger.info(
          { sessionId, repoPath, appName: result.name },
          'Stored repository context in session for downstream tools',
        );
      }
    }

    // Store in sessionManager for cross-tool persistence using helper
    await storeToolResults(
      ctx,
      sessionId,
      'analyze-repo',
      result as unknown as Record<string, unknown>,
      {
        analyzedPath: repoPath,
        appName: result.name || path.basename(repoPath),
        ...(result.isMonorepo && { isMonorepo: true }),
        ...(result.modules && { modules: result.modules }),
      },
    );

    // Add sessionId and workflowHints to the result
    const moduleHint =
      result.isMonorepo && result.modules && result.modules.length > 0
        ? ` Detected ${result.modules.length} modules that can be containerized separately.`
        : '';

    return Success({
      ...result,
      sessionId,
      analyzedPath: repoPath,
      workflowHints: {
        nextStep: 'generate-dockerfile',
        message: `Repository analysis complete.${moduleHint} Use "generate-dockerfile" with sessionId ${sessionId} to create an optimized Dockerfile for your ${result.language || 'application'}.`,
      },
    });
  } catch (e) {
    const error = e as Error;
    const invalidJson = (() => {
      try {
        return responseText.substring(0, 200);
      } catch {
        return '[unavailable]';
      }
    })();
    return Failure(
      `AI response parsing failed: ${error.message}\nInvalid JSON snippet: ${invalidJson}${
        responseText.length > 200 ? '...' : ''
      }`,
    );
  }
}

const tool: Tool<typeof analyzeRepoSchema, AIResponse> = {
  name: 'analyze-repo',
  description: 'Analyze repository structure and detect technologies',
  version: '3.0.0',
  schema: analyzeRepoSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'analysis', 'technology-detection'],
  },
  run,
};

export default tool;
