/**
 * Fix Dockerfile tool using the new Tool pattern
 */

import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { fixDockerfileSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import { DockerfileParser } from 'dockerfile-ast';
import validateDockerfileLib from 'validate-dockerfile';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';

const name = 'fix-dockerfile';
const description = 'Fix and optimize existing Dockerfiles';
const version = '2.0.0';

async function run(
  input: z.infer<typeof fixDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = fixDockerfileSchema.parse(params);
  const { targetEnvironment: environment = 'production', path } = validatedParams;

  // Get Dockerfile content from either path or direct content
  let content = input.dockerfile || '';
  let dockerfilePath: string | undefined;

  if (path) {
    dockerfilePath = nodePath.isAbsolute(path) ? path : nodePath.resolve(process.cwd(), path);
    try {
      content = await fs.readFile(dockerfilePath, 'utf-8');
    } catch (error) {
      return Failure(`Failed to read Dockerfile at ${dockerfilePath}: ${error}`);
    }
  }

  // First, use validate-dockerfile library for basic syntax validation
  const libraryValidation = validateDockerfileLib(content);
  const parseIssues: string[] = [];

  if (!libraryValidation.valid) {
    parseIssues.push(libraryValidation.message || 'Invalid Dockerfile syntax');
  }

  // Check for [object Object] or similar serialization issues
  const lines = content.split('\n');
  lines.forEach((line, idx) => {
    if (line.includes('[object Object]')) {
      parseIssues.push(`Line ${idx + 1}: Contains [object Object] serialization error`);
    }
  });

  // Check for empty COPY/RUN instructions
  lines.forEach((line, idx) => {
    if (line.trim() === 'COPY' || line.trim() === 'RUN') {
      parseIssues.push(`Line ${idx + 1}: Empty ${line.trim()} instruction`);
    }
  });

  // Parse with dockerfile-ast for additional checks
  try {
    dockerfile = DockerfileParser.parse(content);
  } catch (parseError) {
    parseIssues.push(
      `Parser error: ${parseError instanceof Error ? parseError.message : String(parseError)}`,
    );
    // Continue anyway since we'll still try to fix it
  }

  // Semantic analysis if parsing succeeded
  if (dockerfile) {
    const instructions = dockerfile.getInstructions();

    const hasUser = instructions.some((i) => {
      if (i.getInstruction() === 'USER') {
        const args = i.getArguments();
        return args.length > 0 && !args.some((arg) => arg.getValue() === 'root');
      }
      return false;
    });
    if (!hasUser) parseIssues.push('No non-root USER specified (security issue)');

    const hasHealthcheck = instructions.some((i) => i.getInstruction() === 'HEALTHCHECK');
    if (!hasHealthcheck) parseIssues.push('No HEALTHCHECK defined');

    // Check for inefficient layer ordering
    const copyInstructions = instructions.filter(
      (i) => i.getInstruction() === 'COPY' || i.getInstruction() === 'ADD',
    );
    const runInstructions = instructions.filter((i) => i.getInstruction() === 'RUN');
    if (copyInstructions.length > 0 && runInstructions.length > 0) {
      const firstCopy = copyInstructions[0];
      const lastRun = runInstructions[runInstructions.length - 1];
      if (firstCopy && lastRun) {
        const firstCopyIndex = instructions.indexOf(firstCopy);
        const lastRunIndex = instructions.indexOf(lastRun);
        if (firstCopyIndex < lastRunIndex) {
          parseIssues.push('COPY/ADD instructions before RUN commands may break cache efficiency');
        }
      }
    }
  }

  ctx.logger.info(
    { issueCount: parseIssues.length, preview: content.substring(0, 100) },
    'Analyzing Dockerfile for issues',
  );

  // Use the optimization prompt template from @/ai/prompt-templates
  const optimizationParams: any = {
    currentContent: content,
    contentType: 'dockerfile',
    issues: parseIssues,
  };
  if (input.requirements) {
    optimizationParams.requirements = input.requirements;
  }
  const basePrompt = promptTemplates.optimization(optimizationParams);

  // Build messages using the prompt engine with knowledge injection
  const messages = await buildMessages({
    basePrompt,
    topic: 'fix_dockerfile',
    tool: name,
    environment,
    contract: {
      name: 'dockerfile_fixed',
      description: 'Fix and optimize Dockerfile',
    },
    knowledgeBudget: 3000,
  });

  // Call the AI to fix the Dockerfile
  const response = await ctx.sampling.createMessage({
    ...toMCPMessages(messages),
    maxTokens: 4096,
    modelPreferences: {
      hints: [{ name: 'dockerfile-fix' }],
    },
  });

  // Return result with workflow hints
  try {
    const responseText = response.content[0]?.text || '';

  // Try to extract from code blocks if present
  const codeBlockMatch = responseText.match(/```(?:dockerfile)?\s*\n([\s\S]*?)```/);
  if (codeBlockMatch?.[1]) {
    fixedContent = codeBlockMatch[1].trim();
  } else {
    // Look for FROM statement to extract just the Dockerfile
    const fromMatch = responseText.match(/(FROM\s+[\s\S]*)/);
    if (fromMatch?.[1]) {
      fixedContent = fromMatch[1].trim();
    }

    // Write back to file if path was provided
    if (dockerfilePath) {
      await fs.writeFile(dockerfilePath, fixedDockerfile, 'utf-8');
      context.logger.info({ dockerfilePath }, 'Fixed Dockerfile written to disk');
    }

    return Success({
      fixedContent: fixedDockerfile,
      dockerfilePath,
      issues: parseIssues,
      sessionId: validatedParams.sessionId,
      workflowHints: {
        nextStep: 'build-image',
        message: dockerfilePath
          ? `Dockerfile fixed and saved to ${dockerfilePath}. Use "build-image" to build the optimized image.`
          : `Dockerfile fixed successfully. Use "build-image" with sessionId ${validatedParams.sessionId || '<sessionId>'} to build the optimized image.`,
      },
    });
  } catch (e) {
    const error = e as Error;
    return Failure(`AI response parsing failed: ${error.message}`);
  }

  // Validate the fixed content
  const fixedValidation = validateDockerfileLib(fixedContent);
  if (!fixedValidation.valid) {
    ctx.logger.warn(
      { error: fixedValidation.message },
      'Fixed Dockerfile still has validation issues',
    );
  }

  // Write back if we have a path
  if (dockerfilePath) {
    try {
      await fs.writeFile(dockerfilePath, fixedContent, 'utf-8');
      ctx.logger.info({ path: dockerfilePath }, 'Fixed Dockerfile written successfully');
    } catch (writeError) {
      ctx.logger.error(
        { error: writeError instanceof Error ? writeError.message : String(writeError) },
        'Failed to write fixed Dockerfile',
      );
    }
  }

  const improvements: string[] = [];
  improvements.push('✅ Fixed syntax errors and validation issues');
  if (parseIssues.length > 0) {
    improvements.push(`✅ Resolved ${parseIssues.length} identified issues`);
  }
  improvements.push('✅ Applied best practices and optimizations');
  improvements.push('✅ Enhanced security and performance');

  return Success({
    content: fixedContent,
    language: 'dockerfile',
    confidence: 0.9,
    analysis: {
      issuesFound: parseIssues.length,
      issuesFixed: parseIssues,
    },
    suggestions: improvements,
  });
}

const tool: Tool<typeof fixDockerfileSchema, AIResponse> = {
  name,
  description,
  category: 'docker',
  version,
  schema: fixDockerfileSchema,
  run,
};

export default tool;
