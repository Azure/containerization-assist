/**
 * Multi-step Dockerfile generator to avoid timeout issues
 * Breaks down the generation into smaller, focused API calls
 */

import type { ToolContext } from '@/mcp/context';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { Success, Failure, type Result } from '@/types';

interface StepResult {
  content: string;
  success: boolean;
  error?: string;
}

/**
 * Extracts clean Dockerfile instructions from AI response
 */
function extractInstructions(text: string): string {
  if (!text) {
    console.error('[extractInstructions] Received empty text');
    return '';
  }

  console.log('[extractInstructions] Input text (first 200 chars):', text.substring(0, 200));

  // First, try to parse as JSON
  try {
    const parsed = JSON.parse(text);
    console.log(
      '[extractInstructions] Parsed as JSON:',
      typeof parsed,
      Array.isArray(parsed) ? 'array' : 'not array',
    );

    // Handle the expected contract format
    if (parsed.dockerfile_partial || parsed.dockerfile_v1) {
      const content = parsed.dockerfile_partial || parsed.dockerfile_v1;
      console.log(
        '[extractInstructions] Found dockerfile_partial/v1, content type:',
        typeof content,
      );

      if (Array.isArray(content)) {
        console.log('[extractInstructions] Content is array with', content.length, 'items');
        // Filter out any [object Object] entries and properly stringify
        return content
          .filter((item) => item !== null && item !== undefined)
          .map((item) => {
            if (typeof item === 'string') return item;
            if (typeof item === 'object') {
              // Try to extract meaningful content from object
              if ('instruction' in item) return String(item.instruction);
              if ('line' in item) return String(item.line);
              if ('content' in item) return String(item.content);
              if ('value' in item) return String(item.value);
              // Don't return [object Object]
              console.warn('Skipping unrecognized object in Dockerfile content:', item);
              return '';
            }
            return String(item);
          })
          .filter((line) => line && line !== '[object Object]')
          .join('\n');
      }

      return typeof content === 'string' ? content : String(content);
    }

    // Handle plain string JSON
    if (typeof parsed === 'string') {
      return parsed;
    }

    // Handle array at root
    if (Array.isArray(parsed)) {
      return parsed
        .filter((item) => item !== null && item !== undefined)
        .map((item) => {
          if (typeof item === 'string') return item;
          if (typeof item === 'object') {
            if ('instruction' in item) return String(item.instruction);
            if ('line' in item) return String(item.line);
            if ('content' in item) return String(item.content);
            console.warn('Skipping unrecognized object:', item);
            return '';
          }
          return String(item);
        })
        .filter((line) => line && line !== '[object Object]')
        .join('\n');
    }

    // If JSON but not in expected format, might be the instructions directly
    if (typeof parsed === 'object' && parsed.content) {
      return extractInstructions(String(parsed.content));
    }
  } catch {
    // Not JSON, treat as plain text
  }

  // Remove markdown code blocks and clean up
  let cleaned = text;

  // Remove markdown code fences
  cleaned = cleaned.replace(/```dockerfile?\s*/gi, '');
  cleaned = cleaned.replace(/```\s*/g, '');

  // Remove any [object Object] lines
  cleaned = cleaned
    .split('\n')
    .filter((line) => !line.includes('[object Object]'))
    .join('\n');

  return cleaned.trim();
}

/**
 * Generate base image and initial setup
 */
export async function generateBaseImage(
  language: string,
  framework: string | null,
  context: ToolContext,
  environment: string,
): Promise<Result<StepResult>> {
  try {
    const prompt = `Generate ONLY the FROM instruction and initial setup for a ${language} ${
      framework ? `(${framework})` : ''
    } application.
Return only the first 3-5 lines of a Dockerfile including:
1. FROM instruction with appropriate base image
2. WORKDIR setup
3. Any initial ENV variables or ARG declarations
Keep it minimal and focused.`;

    const messages = await buildMessages({
      basePrompt: prompt,
      topic: 'dockerfile_base',
      tool: 'generate-dockerfile',
      environment,
      contract: {
        name: 'dockerfile_partial',
        description: 'Base Dockerfile instructions only',
      },
      knowledgeBudget: 50, // Minimal knowledge for speed
    });

    const response = await context.sampling.createMessage({
      ...toMCPMessages(messages),
      maxTokens: 500, // Small response expected
      modelPreferences: {
        hints: [{ name: 'dockerfile-base' }],
      },
    });

    const rawText = response.content[0]?.text || '';
    context.logger.debug({ rawText: rawText.substring(0, 200) }, 'Raw AI response for base image');

    const content = extractInstructions(rawText);

    if (!content?.includes('FROM')) {
      context.logger.error(
        { content, rawText: rawText.substring(0, 200) },
        'Base image missing FROM instruction',
      );
      return Failure('Failed to generate base image instructions');
    }

    return Success({ content, success: true });
  } catch (error) {
    return Failure(
      `Base image generation failed: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Generate dependency installation steps
 */
export async function generateDependencies(
  language: string,
  framework: string | null,
  baseInstructions: string,
  dependencies: string[],
  context: ToolContext,
  environment: string,
): Promise<Result<StepResult>> {
  try {
    const depList = dependencies.length > 0 ? dependencies.slice(0, 5).join(', ') : '';
    const prompt = `Continue this Dockerfile for a ${language} ${
      framework ? `(${framework})` : ''
    } application by adding dependency installation.

Current Dockerfile start:
${baseInstructions}

${depList ? `Key dependencies to install: ${depList}` : ''}

Add:
1. COPY instructions for dependency files (package.json, pom.xml, requirements.txt, etc.)
2. RUN commands to install dependencies
3. Any caching optimizations

Return ONLY the new instructions to add (not the existing ones).`;

    const messages = await buildMessages({
      basePrompt: prompt,
      topic: 'dockerfile_dependencies',
      tool: 'generate-dockerfile',
      environment,
      contract: {
        name: 'dockerfile_partial',
        description: 'Dependency installation instructions',
      },
      knowledgeBudget: 100, // Some knowledge needed for dependencies
    });

    const response = await context.sampling.createMessage({
      ...toMCPMessages(messages),
      maxTokens: 1000,
      modelPreferences: {
        hints: [{ name: 'dockerfile-deps' }],
      },
    });

    const content = extractInstructions(response.content[0]?.text || '');

    if (!content) {
      return Failure('Failed to generate dependency instructions');
    }

    return Success({ content, success: true });
  } catch (error) {
    return Failure(
      `Dependency generation failed: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Generate application code and build steps
 */
export async function generateBuildSteps(
  language: string,
  framework: string | null,
  context: ToolContext,
  environment: string,
): Promise<Result<StepResult>> {
  try {
    const prompt = `Generate the application build steps for a ${language} ${
      framework ? `(${framework})` : ''
    } application.

Add:
1. COPY instructions for application source code
2. RUN commands for building/compiling if needed
3. Any build-time optimizations

Return ONLY the build-related instructions.`;

    const messages = await buildMessages({
      basePrompt: prompt,
      topic: 'dockerfile_build',
      tool: 'generate-dockerfile',
      environment,
      contract: {
        name: 'dockerfile_partial',
        description: 'Build step instructions',
      },
      knowledgeBudget: 50,
    });

    const response = await context.sampling.createMessage({
      ...toMCPMessages(messages),
      maxTokens: 800,
      modelPreferences: {
        hints: [{ name: 'dockerfile-build' }],
      },
    });

    const content = extractInstructions(response.content[0]?.text || '');

    if (!content) {
      return Failure('Failed to generate build instructions');
    }

    return Success({ content, success: true });
  } catch (error) {
    return Failure(
      `Build steps generation failed: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Generate runtime configuration
 */
export async function generateRuntime(
  language: string,
  framework: string | null,
  ports: number[],
  securityHardening: boolean,
  optimization: string | undefined,
  context: ToolContext,
  environment: string,
): Promise<Result<StepResult>> {
  try {
    const port = ports[0] || 8080;
    const prompt = `Complete the Dockerfile with runtime configuration for a ${language} ${
      framework ? `(${framework})` : ''
    } application.

Requirements:
- Application runs on port ${port}
${securityHardening ? '- Include security hardening with non-root user' : ''}
${optimization ? `- Optimize for ${optimization}` : ''}

Add:
1. EXPOSE ${port}
${securityHardening ? '2. USER directive for non-root user' : ''}
3. HEALTHCHECK if appropriate
4. CMD or ENTRYPOINT to start the application

Return ONLY the runtime configuration instructions.`;

    const messages = await buildMessages({
      basePrompt: prompt,
      topic: 'dockerfile_runtime',
      tool: 'generate-dockerfile',
      environment,
      contract: {
        name: 'dockerfile_partial',
        description: 'Runtime configuration instructions',
      },
      knowledgeBudget: 50,
    });

    const response = await context.sampling.createMessage({
      ...toMCPMessages(messages),
      maxTokens: 600,
      modelPreferences: {
        hints: [{ name: 'dockerfile-runtime' }],
      },
    });

    const content = extractInstructions(response.content[0]?.text || '');

    if (!content || (!content.includes('CMD') && !content.includes('ENTRYPOINT'))) {
      return Failure('Failed to generate runtime instructions');
    }

    return Success({ content, success: true });
  } catch (error) {
    return Failure(
      `Runtime generation failed: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Combine all steps into a complete Dockerfile
 */
export function combineDockerfileSteps(
  baseImage: string,
  dependencies: string,
  buildSteps: string,
  runtime: string,
): string {
  const sections = [baseImage];

  // Add dependencies if not duplicate
  if (dependencies && !dependencies.startsWith('FROM')) {
    sections.push(dependencies);
  }

  // Add build steps if not duplicate
  if (buildSteps && !buildSteps.startsWith('FROM')) {
    sections.push(buildSteps);
  }

  // Add runtime if not duplicate
  if (runtime && !runtime.startsWith('FROM')) {
    sections.push(runtime);
  }

  return sections.join('\n\n');
}
