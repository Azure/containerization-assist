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

// Type-safe instruction item types
type InstructionItem =
  | string
  | { instruction: string }
  | { line: string }
  | { content: string }
  | { value: string }
  | { text: string }
  | { [command: string]: string }; // For {COPY: "source dest"} format

interface DockerfileResponse {
  dockerfile_partial?: string | InstructionItem[];
  dockerfile_v1?: string | InstructionItem[];
  content?: string;
}

// Common generator configuration
interface GeneratorConfig {
  prompt: string;
  topic: string;
  contractDescription: string;
  knowledgeBudget: number;
  maxTokens: number;
  hint: string;
}

/**
 * Extracts meaningful content from various instruction item formats
 */
function extractFromItem(item: unknown): string {
  if (item === null || item === undefined) return '';

  if (typeof item === 'string') return item;

  if (typeof item !== 'object') return String(item);

  const obj = item as Record<string, unknown>;

  // Check for known property names
  const knownProps = ['instruction', 'line', 'content', 'value', 'text'] as const;
  for (const prop of knownProps) {
    if (prop in obj && typeof obj[prop] === 'string') {
      return obj[prop];
    }
  }

  // Check for single-key command objects like {COPY: "source dest"}
  const keys = Object.keys(obj);
  if (keys.length === 1) {
    const [command] = keys;
    const value = command ? obj[command] : undefined;
    if (typeof value === 'string' && value.trim()) {
      return `${command} ${value}`;
    }
  }

  // Try toString if it's not the default
  const str = String(item);
  if (str && str !== '[object Object]') {
    return str;
  }

  console.warn('Skipping unrecognized object in Dockerfile content:', item);
  return '';
}

/**
 * Processes an array of instruction items into a string
 */
function processInstructionArray(items: unknown[]): string {
  return items
    .map(extractFromItem)
    .filter((line) => line && line !== '[object Object]')
    .join('\n');
}

/**
 * Extracts clean Dockerfile instructions from AI response
 */
function extractInstructions(text: string): string {
  if (!text) {
    console.error('[extractInstructions] Received empty text');
    return '';
  }

  // Log input for debugging
  // console.log('[extractInstructions] Input text (first 200 chars):', text.substring(0, 200));

  // First, try to parse as JSON
  try {
    const parsed = JSON.parse(text) as unknown;
    // Log parsed type for debugging
    // console.log(
    //   '[extractInstructions] Parsed as JSON:',
    //   typeof parsed,
    //   Array.isArray(parsed) ? 'array' : 'not array',
    // );

    // Handle the expected contract format
    if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
      const response = parsed as DockerfileResponse;

      const content = response.dockerfile_partial || response.dockerfile_v1;
      if (content !== undefined) {
        // Log content type for debugging
        // console.log(
        //   '[extractInstructions] Found dockerfile_partial/v1, content type:',
        //   typeof content,
        // );

        if (Array.isArray(content)) {
          // Log array length for debugging
          // console.log('[extractInstructions] Content is array with', content.length, 'items');
          return processInstructionArray(content);
        }

        return typeof content === 'string' ? content : String(content);
      }

      // Check for nested content property
      if (response.content) {
        return extractInstructions(String(response.content));
      }
    }

    // Handle plain string JSON
    if (typeof parsed === 'string') {
      return parsed;
    }

    // Handle array at root
    if (Array.isArray(parsed)) {
      return processInstructionArray(parsed);
    }
  } catch {
    // Not JSON, treat as plain text
  }

  // Remove markdown code blocks and clean up
  const cleaned = text
    .replace(/```dockerfile?\s*/gi, '')
    .replace(/```\s*/g, '')
    .split('\n')
    .filter((line) => !line.includes('[object Object]'))
    .join('\n');

  return cleaned.trim();
}

/**
 * Common generator function to reduce duplication
 */
async function generateStep(
  config: GeneratorConfig,
  context: ToolContext,
  environment: string,
): Promise<Result<StepResult>> {
  try {
    const messages = await buildMessages({
      basePrompt: config.prompt,
      topic: config.topic,
      tool: 'generate-dockerfile',
      environment,
      contract: {
        name: 'dockerfile_partial',
        description: config.contractDescription,
      },
      knowledgeBudget: config.knowledgeBudget,
    });

    const response = await context.sampling.createMessage({
      ...toMCPMessages(messages),
      maxTokens: config.maxTokens,
      modelPreferences: {
        hints: [{ name: config.hint }],
      },
    });

    const rawText = response.content[0]?.text || '';
    context.logger.debug(
      { rawText: rawText.substring(0, 200) },
      `Raw AI response for ${config.topic}`,
    );

    const content = extractInstructions(rawText);

    if (!content) {
      return Failure(`Failed to generate ${config.topic} instructions`);
    }

    return Success({ content, success: true });
  } catch (error) {
    return Failure(
      `${config.topic} generation failed: ${
        error instanceof Error ? error.message : String(error)
      }`,
    );
  }
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
  const config: GeneratorConfig = {
    prompt: `Generate ONLY the FROM instruction and initial setup for a ${language} ${
      framework ? `(${framework})` : ''
    } application.
Return only the first 3-5 lines of a Dockerfile including:
1. FROM instruction with appropriate base image
2. WORKDIR setup
3. Any initial ENV variables or ARG declarations
Keep it minimal and focused.`,
    topic: 'dockerfile_base',
    contractDescription: 'Base Dockerfile instructions only',
    knowledgeBudget: 50,
    maxTokens: 500,
    hint: 'dockerfile-base',
  };

  const result = await generateStep(config, context, environment);

  // Additional validation for base image
  if (result.ok && !result.value.content.includes('FROM')) {
    context.logger.error({ content: result.value.content }, 'Base image missing FROM instruction');
    return Failure('Failed to generate base image instructions');
  }

  return result;
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
  const depList = dependencies.length > 0 ? dependencies.slice(0, 5).join(', ') : '';

  const config: GeneratorConfig = {
    prompt: `Continue this Dockerfile for a ${language} ${
      framework ? `(${framework})` : ''
    } application by adding dependency installation.

Current Dockerfile start:
${baseInstructions}

${depList ? `Key dependencies to install: ${depList}` : ''}

Add:
1. COPY instructions for dependency files (package.json, pom.xml, requirements.txt, etc.)
2. RUN commands to install dependencies
3. Any caching optimizations

Return ONLY the new instructions to add (not the existing ones).`,
    topic: 'dockerfile_dependencies',
    contractDescription: 'Dependency installation instructions',
    knowledgeBudget: 100,
    maxTokens: 1000,
    hint: 'dockerfile-deps',
  };

  return generateStep(config, context, environment);
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
  const config: GeneratorConfig = {
    prompt: `Generate the application build steps for a ${language} ${
      framework ? `(${framework})` : ''
    } application.

Add:
1. COPY instructions for application source code
2. RUN commands for building/compiling if needed
3. Any build-time optimizations

Return ONLY the build-related instructions.`,
    topic: 'dockerfile_build',
    contractDescription: 'Build step instructions',
    knowledgeBudget: 50,
    maxTokens: 800,
    hint: 'dockerfile-build',
  };

  return generateStep(config, context, environment);
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
  const port = ports[0] || 8080;

  const config: GeneratorConfig = {
    prompt: `Complete the Dockerfile with runtime configuration for a ${language} ${
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

Return ONLY the runtime configuration instructions.`,
    topic: 'dockerfile_runtime',
    contractDescription: 'Runtime configuration instructions',
    knowledgeBudget: 50,
    maxTokens: 600,
    hint: 'dockerfile-runtime',
  };

  const result = await generateStep(config, context, environment);

  // Additional validation for runtime
  if (result.ok) {
    const content = result.value.content;
    if (!content.includes('CMD') && !content.includes('ENTRYPOINT')) {
      return Failure('Failed to generate runtime instructions');
    }
  }

  return result;
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
  // Build sections array, keeping base image always first
  const sections: string[] = [];

  if (baseImage) {
    sections.push(baseImage);
  }

  // Add other sections only if they don't duplicate FROM
  if (dependencies && !dependencies.startsWith('FROM')) {
    sections.push(dependencies);
  }

  if (buildSteps && !buildSteps.startsWith('FROM')) {
    sections.push(buildSteps);
  }

  if (runtime && !runtime.startsWith('FROM')) {
    sections.push(runtime);
  }

  // Ensure we have at least the base image
  if (sections.length === 0) {
    return baseImage || '';
  }

  return sections.join('\n\n');
}
