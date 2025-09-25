import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { analyzeRepoSchema, type AnalyzeRepoParams } from './schema';
import type { AIResponse } from '../ai-response-types';
import { randomUUID } from 'node:crypto';
import { updateSession, ensureSession } from '@/mcp/tool-session-helpers';
import { promises as fs } from 'node:fs';
import path from 'node:path';

const CONFIG_FILES = [
  'package.json',
  'pom.xml',
  'build.gradle',
  'requirements.txt',
  'go.mod',
  'Cargo.toml',
];

export async function analyzeRepo(
  params: AnalyzeRepoParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = analyzeRepoSchema.parse(params);
  let { path: repoPath } = validatedParams;
  const { sessionId } = validatedParams;

  // Convert to absolute path if relative
  if (!path.isAbsolute(repoPath)) {
    repoPath = path.resolve(process.cwd(), repoPath);
  }

  // Log the path being analyzed
  context.logger.info(
    {
      requestedPath: repoPath,
      params: validatedParams,
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

    for (const configFile of CONFIG_FILES) {
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
    context.logger.error({ error, repoPath }, 'Failed to read repository files');
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
  context.logger.info(
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
  context.logger.info(
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
    response = await context.sampling.createMessage({
      ...mcpMessages,
      maxTokens: 4096,
      modelPreferences: {
        hints: [{ name: 'code-analysis' }, { name: 'json-output' }],
      },
    });
  } catch (error) {
    context.logger.error(
      { error: error instanceof Error ? error.message : String(error) },
      'AI sampling failed',
    );
    return Failure(`AI sampling failed: ${error instanceof Error ? error.message : String(error)}`);
  }

  // Return parsed result
  const responseText = response.content[0]?.text || '';
  context.logger.info(
    {
      responseLength: responseText.length,
      responsePreview: responseText.substring(0, 200),
      hasContent: response.content.length,
    },
    'Received AI response',
  );

  const jsonMatch = responseText.match(/\{[\s\S]*\}/);
  if (!jsonMatch) {
    context.logger.error(
      { responseText: responseText.substring(0, 500) },
      'AI response did not contain valid JSON',
    );
    return Failure('AI response did not contain valid JSON');
  }

  try {
    const analysisResult = JSON.parse(jsonMatch[0]);

    // Store the analysis result in session for other tools to use
    const sessionResult = await ensureSession(context, workflowSessionId);
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
        context,
      );
      context.logger.info(
        { sessionId: workflowSessionId },
        'Stored repository analysis in session',
      );
    } else {
      context.logger.warn(
        'Could not store analysis in session - session manager may not be available',
      );
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

export const metadata = {
  name: 'analyze-repo',
  description: 'Analyze repository structure and detect technologies',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
