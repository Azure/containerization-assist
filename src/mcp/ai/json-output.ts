import { z } from 'zod';
import type { Logger } from 'pino';
import { extractErrorMessage } from '@lib/error-utils';
import { Success, Failure, type Result } from '@types';
import { getPromptMetadata } from '@prompts/prompt-registry';

interface AITemplate {
  template: string;
  [key: string]: unknown;
}

async function loadAITemplate(promptId: string): Promise<Result<AITemplate>> {
  try {
    const promptEntry = await getPromptMetadata(promptId);

    if (!promptEntry) {
      return Failure(`Template ${promptId} not found in prompt registry`);
    }

    if (!promptEntry.template) {
      return Failure(`Template ${promptId} missing required 'template' field`);
    }

    // Convert PromptEntry to AITemplate format
    const template: AITemplate = promptEntry as AITemplate;

    return Success(template);
  } catch (error) {
    return Failure(`Failed to load template ${promptId}: ${extractErrorMessage(error)}`);
  }
}

async function runHostAssist(
  prompt: string,
  options: { maxTokens: number; temperature?: number },
  context?: {
    sampling?: {
      createMessage: (req: unknown) => Promise<{ content: Array<{ text: string }> }>;
    };
  },
): Promise<string> {
  // If no context or sampling available, can't repair
  if (!context?.sampling) {
    throw new Error('AI sampling context not available for JSON repair');
  }

  try {
    const response = await context.sampling.createMessage({
      messages: [
        {
          role: 'user',
          content: [
            {
              type: 'text',
              text: prompt,
            },
          ],
        },
      ],
      maxTokens: options.maxTokens,
      temperature: options.temperature,
      includeContext: 'none',
    });

    // Extract text from response - content is an array of content items
    if (response.content && Array.isArray(response.content)) {
      const text = response.content
        .filter(
          (c: unknown) => c !== null && typeof c === 'object' && 'type' in c && c.type === 'text',
        )
        .map((c: unknown) =>
          c !== null && typeof c === 'object' && 'text' in c && typeof c.text === 'string'
            ? c.text
            : '',
        )
        .join('');

      if (text) {
        return text;
      }
    }

    throw new Error('AI response missing text content');
  } catch (error) {
    throw new Error(`AI repair failed: ${error instanceof Error ? error.message : String(error)}`);
  }
}

function stripCodeFence(s: string): string {
  const BOM = /^\uFEFF/;
  const trimmed = s.replace(BOM, '').trim();

  // Match various code fence patterns
  const patterns = [/^```(?:json)?\s*([\s\S]*?)```$/i, /^```\s*([\s\S]*?)```$/, /^`([\s\S]*?)`$/];

  for (const pattern of patterns) {
    const match = trimmed.match(pattern);
    if (match?.[1]) {
      return match[1].trim();
    }
  }

  return trimmed;
}

function rejectNovelTopLevelFields<T>(
  parsed: unknown,
  schema: z.ZodSchema<T>,
  logger?: Logger,
): void {
  // Infer allowed top-level keys from the schema if it's an object schema
  const schemaType = (schema as z.ZodTypeAny)._def?.typeName;

  // Only check for object schemas
  if (schemaType !== z.ZodFirstPartyTypeKind.ZodObject) {
    return; // non-object, skip
  }

  const shape = (schema as unknown as z.ZodObject<z.ZodRawShape>)._def?.shape();
  if (!shape) return;

  const allowed = new Set(Object.keys(shape));
  const novelFields: string[] = [];

  for (const k of Object.keys(parsed as object)) {
    if (!allowed.has(k)) {
      novelFields.push(k);
    }
  }

  if (novelFields.length > 0) {
    const error = `Unexpected top-level field(s) in model output: ${novelFields.map((f) => `"${f}"`).join(', ')}`;
    logger?.warn({ novelFields }, error);
    throw new Error(error);
  }
}

/**
 * Transform common AI response format issues
 */
function transformAIResponse(obj: unknown, _schema: z.ZodSchema<unknown>, logger: Logger): unknown {
  // Special case for Dockerfile generation - AI sometimes returns wrong format
  if (typeof obj === 'object' && obj !== null) {
    // Check if this looks like a Dockerfile response with wrong field names
    if ('Dockerfile' in obj || 'dockerfile' in obj) {
      logger.info('Transforming Dockerfile response from incorrect format');

      const dockerfileContent = (obj as any).Dockerfile || (obj as any).dockerfile;
      const notes = (obj as any).notes;

      // Transform array of lines to single string if needed
      const content = Array.isArray(dockerfileContent)
        ? dockerfileContent.join('\n')
        : dockerfileContent || '';

      // Build the correct structure
      const transformed = {
        content: content || '',
        metadata: {
          baseImage: extractBaseImage(content),
          hasHealthCheck: content?.includes('HEALTHCHECK') || false,
          isMultiStage: content?.includes('AS builder') || content?.includes('AS build') || false,
          exposedPorts: extractExposedPorts(content),
        },
        recommendations: normalizeRecommendations(notes),
      };

      logger.info(
        { transformed: JSON.stringify(transformed).slice(0, 200) },
        'Transformed response',
      );
      return transformed;
    }
  }

  return obj;
}

function extractBaseImage(dockerfile: string | undefined): string {
  if (!dockerfile) return 'unknown';
  const match = dockerfile.match(/^FROM\s+(\S+)/m);
  return match?.[1] || 'unknown';
}

function extractExposedPorts(dockerfile: string | undefined): number[] {
  if (!dockerfile) return [];
  const matches = dockerfile.match(/^EXPOSE\s+(\d+)/gm);
  if (!matches) return [];
  return matches
    .map((m) => {
      const portMatch = m.match(/\d+/);
      return portMatch?.[0] ? parseInt(portMatch[0], 10) : 0;
    })
    .filter((p) => p > 0);
}

function normalizeRecommendations(notes: unknown): string[] {
  if (!notes) return [];

  // If it's already an array, flatten and stringify each item
  if (Array.isArray(notes)) {
    return notes.map((note) => {
      if (typeof note === 'string') return note;
      if (Array.isArray(note)) return note.join(' ');
      return String(note);
    });
  }

  // If it's a string, return as single-item array
  if (typeof notes === 'string') {
    return [notes];
  }

  // For any other type, stringify it
  return [String(notes)];
}

/**
 * Parse a model string, attempt a single repair via json-repair template on failure,
 * validate with Zod, and reject novel top-level fields to prevent drift.
 */
export async function parseAndValidateJson<T>(
  raw: string,
  schema: z.ZodSchema<T>,
  logger: Logger,
  context?: {
    sampling?: {
      createMessage: (req: unknown) => Promise<{ content: Array<{ text: string }> }>;
    };
  },
): Promise<T> {
  const tryParse = (text: string, attemptNumber: number = 1): T => {
    const stripped = stripCodeFence(text);

    logger.debug({ attempt: attemptNumber, length: stripped.length }, 'Attempting JSON parse');

    let obj: unknown;
    try {
      obj = JSON.parse(stripped);
    } catch (e) {
      logger.debug({ error: extractErrorMessage(e), attempt: attemptNumber }, 'JSON parse failed');
      throw e;
    }

    // Transform common AI response issues before validation
    obj = transformAIResponse(obj, schema, logger);

    // Check for novel fields before Zod validation
    try {
      rejectNovelTopLevelFields(obj, schema, logger);
    } catch (e) {
      logger.warn(
        { error: extractErrorMessage(e), attempt: attemptNumber },
        'Novel fields detected',
      );
      throw e;
    }

    // Validate with Zod
    try {
      return schema.parse(obj);
    } catch (e) {
      logger.debug(
        { error: extractErrorMessage(e), attempt: attemptNumber },
        'Zod validation failed',
      );
      throw e;
    }
  };

  // First attempt: try to parse directly
  try {
    return tryParse(raw, 1);
  } catch (firstError) {
    logger.warn(
      { error: extractErrorMessage(firstError) },
      'Invalid JSON; attempting single repair',
    );

    // Second attempt: try to repair once
    try {
      // Load the JSON repair template
      const repairResult = await loadAITemplate('json-repair');
      if (!repairResult.ok) {
        throw new Error(`Failed to load repair template: ${repairResult.error}`);
      }

      const repairInstruction =
        'Fix syntax only. Do NOT add or rename top-level fields. Return ONLY valid JSON.';

      const prompt = repairResult.value.template
        .replace('{{malformed_json}}', stripCodeFence(raw))
        .replace('{{error_message}}', extractErrorMessage(firstError))
        .replace('{{repair_instruction}}', repairInstruction);

      // Call model to repair
      const fixed = await runHostAssist(
        prompt,
        {
          maxTokens: 2048,
          temperature: 0.1, // Low temperature for deterministic repairs
        },
        context,
      );

      logger.debug('JSON repair attempted');

      // Try to parse the repaired JSON
      return tryParse(fixed, 2);
    } catch (secondError) {
      // Log both errors for debugging
      logger.error(
        {
          originalError: extractErrorMessage(firstError),
          repairError: extractErrorMessage(secondError),
          rawLength: raw.length,
        },
        'JSON parsing failed after repair attempt',
      );

      // Re-throw the repair error
      throw new Error(`Failed to parse JSON after repair: ${extractErrorMessage(secondError)}`);
    }
  }
}
