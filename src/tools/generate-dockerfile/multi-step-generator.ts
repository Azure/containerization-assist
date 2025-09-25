/**
 * Multi-step Dockerfile generator to avoid timeout issues
 * Breaks down the generation into smaller, focused API calls
 */

import type { ToolContext } from '@/mcp/context';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { Success, Failure, type Result, TOPICS, type Topic } from '@/types';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';

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
  topic: Topic;
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
 * Validates and fixes SHA256 digests in Dockerfile content
 */
function fixInvalidDigests(content: string): string {
  // Pattern to match invalid SHA256 digests (repeated characters or placeholder patterns)
  const invalidDigestPattern = /@sha256:([0-9a-f]{1,64})\1{2,}|@sha256:[67e]+$/gi;

  // Remove invalid SHA256 digests, keeping just the tag
  return content.replace(invalidDigestPattern, '');
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
          return fixInvalidDigests(processInstructionArray(content));
        }

        const stringContent = typeof content === 'string' ? content : String(content);
        return fixInvalidDigests(stringContent);
      }

      // Check for nested content property
      if (response.content) {
        return extractInstructions(String(response.content));
      }
    }

    // Handle plain string JSON
    if (typeof parsed === 'string') {
      return fixInvalidDigests(parsed);
    }

    // Handle array at root
    if (Array.isArray(parsed)) {
      return fixInvalidDigests(processInstructionArray(parsed));
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

  // Fix any invalid SHA256 digests before returning
  return fixInvalidDigests(cleaned.trim());
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
  baseImagePreference?: string,
): Promise<Result<StepResult>> {
  const preferenceHint = baseImagePreference
    ? `\nBase image preference: ${baseImagePreference}\n`
    : '';

  const config: GeneratorConfig = {
    prompt: `Generate ONLY the FROM instruction and initial setup for a ${language} ${
      framework ? `(${framework})` : ''
    } application.${preferenceHint}
Return only the first 3-5 lines of a Dockerfile including:
1. FROM instruction with appropriate base image
2. WORKDIR setup
3. Any initial ENV variables or ARG declarations
Keep it minimal and focused.`,
    topic: TOPICS.DOCKERFILE_BASE,
    contractDescription: 'Base Dockerfile instructions only',
    knowledgeBudget: 20,
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
  projectPath?: string,
): Promise<Result<StepResult>> {
  const depList = dependencies.length > 0 ? dependencies.slice(0, 5).join(', ') : '';

  // Check which dependency files actually exist
  const existingFiles: string[] = [];
  if (projectPath) {
    const commonDepFiles = [
      'package.json',
      'package-lock.json',
      'yarn.lock',
      'pnpm-lock.yaml',
      'pom.xml',
      'mvnw',
      '.mvn',
      'requirements.txt',
      'Pipfile',
      'Pipfile.lock',
      'poetry.lock',
      'pyproject.toml',
      'go.mod',
      'go.sum',
      'Gemfile',
      'Gemfile.lock',
      'build.gradle',
      'gradlew',
      'gradle',
      'Cargo.toml',
      'Cargo.lock',
      'composer.json',
      'composer.lock',
    ];

    for (const file of commonDepFiles) {
      try {
        const filePath = nodePath.join(projectPath, file);
        const stats = await fs.stat(filePath);
        if (stats.isFile() || stats.isDirectory()) {
          existingFiles.push(file);
        }
      } catch {
        // File doesn't exist, skip it
      }
    }

    context.logger.info({ projectPath, existingFiles }, 'Detected existing dependency files');
  }

  const existingFilesHint =
    existingFiles.length > 0
      ? `\nIMPORTANT: Only use COPY instructions for these files that actually exist in the project: ${existingFiles.join(', ')}\nDo NOT use COPY for files that are not in this list.`
      : '';

  const config: GeneratorConfig = {
    prompt: `Continue this Dockerfile for a ${language} ${
      framework ? `(${framework})` : ''
    } application by adding dependency installation.

Current Dockerfile start:
${baseInstructions}

${depList ? `Key dependencies to install: ${depList}` : ''}
${existingFilesHint}

Add:
1. COPY instructions ONLY for dependency files that exist
2. RUN commands to install dependencies
3. Any caching optimizations

Return ONLY the new instructions to add (not the existing ones).`,
    topic: TOPICS.DOCKERFILE_DEPENDENCIES,
    contractDescription: 'Dependency installation instructions',
    knowledgeBudget: 30,
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
  projectPath?: string,
): Promise<Result<StepResult>> {
  // Check for common source directories and files
  const existingSourcePaths: string[] = [];
  if (projectPath) {
    const commonSourcePaths = [
      'src',
      'app',
      'lib',
      'bin',
      'cmd',
      'pkg',
      'public',
      'static',
      'dist',
      'build',
      'target',
      '.', // Current directory for simple projects
    ];

    for (const path of commonSourcePaths) {
      try {
        const fullPath = nodePath.join(projectPath, path);
        const stats = await fs.stat(fullPath);
        if (stats.isDirectory()) {
          existingSourcePaths.push(path);
        }
      } catch {
        // Path doesn't exist
      }
    }

    context.logger.info({ projectPath, existingSourcePaths }, 'Detected existing source paths');
  }

  const sourcePathHint =
    existingSourcePaths.length > 0
      ? `\nIMPORTANT: The project has these directories: ${existingSourcePaths.join(', ')}. Use COPY . . or COPY src src/ based on what actually exists.`
      : '\nIMPORTANT: Use COPY . . to copy all project files since specific source directories were not detected.';

  const config: GeneratorConfig = {
    prompt: `Generate the application build steps for a ${language} ${
      framework ? `(${framework})` : ''
    } application.
${sourcePathHint}

Add:
1. COPY instructions for application source code (use paths that exist)
2. RUN commands for building/compiling if needed
3. Any build-time optimizations

Return ONLY the build-related instructions.`,
    topic: TOPICS.DOCKERFILE_BUILD,
    contractDescription: 'Build step instructions',
    knowledgeBudget: 20,
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
    topic: TOPICS.DOCKERFILE_RUNTIME,
    contractDescription: 'Runtime configuration instructions',
    knowledgeBudget: 20,
    maxTokens: 600,
    hint: 'dockerfile-runtime',
  };

  const result = await generateStep(config, context, environment);

  // Additional validation for runtime
  if (result.ok) {
    const content = result.value.content;
    if (!content.includes('CMD') && !content.includes('ENTRYPOINT')) {
      return Failure('Failed to generate runtime instructions: missing CMD or ENTRYPOINT');
    }
    // Validation passed, return the successful result
    return result;
  }

  // If generateStep failed, return its error
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

  return sections.join('\n\n');
}
