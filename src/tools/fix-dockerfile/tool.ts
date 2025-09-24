import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates } from '@/prompts/templates';
import { buildMessages, toMCPMessages } from '@/ai/prompt-engine';
import { fixDockerfileSchema, type FixDockerfileParams } from './schema';
import type { AIResponse } from '../ai-response-types';
import { DockerfileParser } from 'dockerfile-ast';

export async function fixDockerfile(
  params: FixDockerfileParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = fixDockerfileSchema.parse(params);
  const { dockerfile: content = '', targetEnvironment: environment = 'production' } =
    validatedParams;

  // First, parse the Dockerfile to identify real issues
  const parseIssues: string[] = [];
  try {
    const dockerfile = DockerfileParser.parse(content);
    const instructions = dockerfile.getInstructions();

    // Check for common issues
    const hasFrom = instructions.some((i) => i.getInstruction() === 'FROM');
    if (!hasFrom) parseIssues.push('Missing FROM instruction');

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
    return Success({
      fixedContent: responseText,
      sessionId: validatedParams.sessionId,
      workflowHints: {
        nextStep: 'build-image',
        message: `Dockerfile fixed successfully. Use "build-image" with sessionId ${validatedParams.sessionId || '<sessionId>'} to build the optimized image.`,
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
