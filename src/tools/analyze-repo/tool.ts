import { promises as fs } from 'node:fs';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { Success, Failure, type Result, type ToolContext } from '@/types';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { updateSession, ensureSession } from '@/mcp/tool-session-helpers';
import { analyzeRepoSchema } from './schema';
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

  // Log the path being analyzed
  ctx.logger.info(
    {
      requestedPath: repoPath,
      params: input,
    },
    'Starting repository analysis',
  );

  // Use provided sessionId or generate a new one for the workflow
  const workflowSessionId = sessionId || randomUUID();

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
    sessionId: workflowSessionId,
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
    response = await ctx.sampling.createMessage({
      ...mcpMessages,
      maxTokens: 4096,
      modelPreferences: {
        hints: [{ name: 'code-analysis' }, { name: 'json-output' }],
      },
    });
  } catch (error) {
    ctx.logger.error(
      { error: error instanceof Error ? error.message : String(error) },
      'AI sampling failed',
    );
    return Failure(`AI sampling failed: ${error instanceof Error ? error.message : String(error)}`);
  }

  // Return parsed result
  const responseText = response.content[0]?.text || '';
  ctx.logger.info(
    {
      responseLength: responseText.length,
      responsePreview: responseText.substring(0, 200),
      hasContent: response.content.length,
    },
    'Received AI response',
  );

  const jsonMatch = responseText.match(/\{[\s\S]*\}/);
  if (!jsonMatch) {
    ctx.logger.error(
      { responseText: responseText.substring(0, 500) },
      'AI response did not contain valid JSON',
    );
    return Failure('AI response did not contain valid JSON');
  }

  try {
    const analysisResult = JSON.parse(jsonMatch[0]);

    // Store the analysis result in session for other tools to use
    const sessionResult = await ensureSession(ctx, workflowSessionId);
    if (sessionResult.ok) {
      await updateSession(
        workflowSessionId,
        {
          metadata: {
            ...sessionResult.value.state.metadata,
            repositoryAnalysis: analysisResult,
            analyzedPath: repoPath,
          },
          current_step: 'analyze-repo',
          completed_steps: [...(sessionResult.value.state.completed_steps || []), 'analyze-repo'],
        },
        ctx,
      );
      ctx.logger.info({ sessionId: workflowSessionId }, 'Stored repository analysis in session');
    } else {
      ctx.logger.warn('Could not store analysis in session - session manager may not be available');
    }

    // Add sessionId to the result
    return Success({ ...analysisResult, sessionId: workflowSessionId });
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

const tool: Tool<typeof analyzeRepoSchema, AIResponse> = {
  name: 'analyze-repo',
  description: 'Analyze repository structure and detect technologies',
  version: '3.0.0',
  schema: analyzeRepoSchema,
  run,
};

export default tool;
