import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { fixDockerfileSchema, type FixDockerfileParams } from './schema';
import type { AIResponse } from '../ai-response-types';
import { DockerfileParser } from 'dockerfile-ast';
import validateDockerfileLib from 'validate-dockerfile';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';

export async function fixDockerfile(
  params: FixDockerfileParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = fixDockerfileSchema.parse(params);
  const { targetEnvironment: environment = 'production', path } = validatedParams;

  // Get Dockerfile content from either path or direct content
  let content = validatedParams.dockerfile || '';
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
    const trimmed = line.trim();
    if (trimmed === 'COPY' || trimmed === 'RUN' || trimmed === 'ADD') {
      parseIssues.push(`Line ${idx + 1}: Empty ${trimmed} instruction without arguments`);
    }
  });

  // Parse with dockerfile-ast for additional checks
  try {
    const dockerfile = DockerfileParser.parse(content);
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

    // Check for :latest tags
    instructions.forEach((inst) => {
      if (inst.getInstruction() === 'FROM') {
        const args = inst.getArguments();
        if (args.some((arg) => arg.getValue().includes(':latest'))) {
          parseIssues.push('Using :latest tag - pin to specific version for reproducibility');
        }
      }
    });
  } catch (parseError) {
    parseIssues.push(
      `Dockerfile syntax error: ${parseError instanceof Error ? parseError.message : 'Unknown error'}`,
    );
  }

  // Generate prompt from template - instruct AI to analyze actual issues
  const analysisPrompt = `First, analyze this Dockerfile to identify specific issues.

Issues already detected by parser:
${parseIssues.map((issue, i) => `${i + 1}. ${issue}`).join('\n')}

Now analyze for additional issues:
1. Run a security scan to find vulnerabilities (check for running as root, exposed secrets, insecure base images)
2. Check for size optimization opportunities (unnecessary packages, poor layer caching, large base images)
3. Identify layer ordering problems (frequently changing layers before stable ones)
4. Find best practice violations (missing HEALTHCHECK, no USER directive, hardcoded values)
5. Check for missing security hardening (no COPY --chown, world-writable files, etc.)

Then fix the identified issues.`;

  const basePrompt = promptTemplates.fix('dockerfile', content, [analysisPrompt]);

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

  // Return result with workflow hints
  try {
    const responseText = response.content[0]?.text || '';

    // Extract actual Dockerfile content if wrapped in JSON
    let fixedDockerfile = responseText;
    try {
      const parsed = JSON.parse(responseText);
      if (parsed.dockerfile_v1 || parsed.dockerfile) {
        fixedDockerfile = parsed.dockerfile_v1 || parsed.dockerfile;
      }
    } catch {
      // Not JSON, use as-is
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
}

export const metadata = {
  name: 'fix-dockerfile',
  description: 'Fix and optimize existing Dockerfiles',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
