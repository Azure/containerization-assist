/**
 * AI-powered Dockerfile fixing tool
 */

import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result } from '@types';
import { extractErrorMessage } from '@lib/error-utils';
import {
  DockerfileFixOutputSchema,
  fixDockerfileSchema,
  type FixDockerfileParams,
  type DockerfileFixOutput,
} from './schema';

/**
 * Fix and optimize Dockerfile using AI analysis
 */
async function fixDockerfileHandler(
  params: FixDockerfileParams,
  context: ToolContext,
): Promise<Result<DockerfileFixOutput>> {
  const { logger } = context;

  try {
    // Create prompt for AI analysis
    const prompt = `Fix and optimize the following Dockerfile.

Original Dockerfile:
${params.dockerfile}

${params.error ? `Error to fix: ${params.error}\n` : ''}
${params.issues?.length ? `Issues to address:\n- ${params.issues.join('\n- ')}\n` : ''}

Target environment: ${params.targetEnvironment || 'production'}

Analyze the Dockerfile and provide:
1. Fixed Dockerfile content
2. List of changes made with explanations
3. Security improvements applied
4. Performance optimizations applied
5. Size reduction techniques used
6. Validation status

Return the result in JSON format:
{
  "fixedDockerfile": "string - the complete fixed Dockerfile content",
  "changes": [{
    "type": "security" | "performance" | "best-practice" | "syntax" | "deprecation",
    "description": "string",
    "before": "string",
    "after": "string",
    "impact": "high" | "medium" | "low"
  }],
  "securityImprovements": ["string"],
  "performanceOptimizations": ["string"],
  "sizeReduction": "string or null",
  "warnings": ["string"] or null,
  "isValid": boolean,
  "estimatedBuildTime": "string or null",
  "estimatedImageSize": "string or null"
}`;

    // Call AI through context
    const response = await context.sampling.createMessage({
      messages: [
        {
          role: 'user',
          content: [{ type: 'text', text: prompt }],
        },
      ],
      maxTokens: 4096,
    });

    // Extract text from response
    const text = response.content
      .filter((c) => c.type === 'text')
      .map((c) => c.text)
      .join('');

    // Parse JSON response
    const result = JSON.parse(text);

    // Validate with schema
    const validated = DockerfileFixOutputSchema.parse(result);

    logger.info(
      {
        changeCount: validated.changes?.length || 0,
        sessionId: params.sessionId,
      },
      'Dockerfile fixed successfully',
    );

    return Success(validated);
  } catch (error) {
    logger.error({ error: extractErrorMessage(error) }, 'Failed to fix Dockerfile');
    return Failure(`Failed to fix Dockerfile: ${extractErrorMessage(error)}`);
  }
}

/**
 * Standard tool export for MCP server integration
 */
export const tool = {
  type: 'standard' as const,
  name: 'fix-dockerfile',
  description: 'Fix and optimize Dockerfiles using AI analysis and security best practices',
  inputSchema: fixDockerfileSchema,
  execute: fixDockerfileHandler,
};

// Export for backward compatibility
export const fixDockerfile = fixDockerfileHandler;
