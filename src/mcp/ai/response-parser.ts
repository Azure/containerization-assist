/**
 * AI Response Parser
 *
 * Provides JSON-first parsing for AI responses with automatic repair capabilities
 */

import { z } from 'zod';
import type { ToolContext } from '@/mcp/context';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { Success, Failure, type Result } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';

export interface ParseOptions {
  /** Number of repair attempts if JSON parsing fails */
  repairAttempts?: number;
  /** Maximum token limit for repair responses */
  maxTokens?: number;
  /** Whether to log parsing attempts */
  debug?: boolean;
}

/**
 * Parse AI response using structured JSON schema with automatic repair
 */
export async function parseAIResponse<T>(
  text: string,
  schema: z.ZodSchema<T>,
  ctx: ToolContext,
  options: ParseOptions = {},
): Promise<Result<T>> {
  const { repairAttempts = 1, maxTokens = 256, debug = false } = options;

  // First try: direct JSON extraction and parsing
  const directResult = await tryDirectParse(text, schema, debug);
  if (directResult.ok) {
    return directResult;
  }

  // Second try: extract JSON from markdown or other formats
  const extractedResult = await tryExtractedParse(text, schema, debug);
  if (extractedResult.ok) {
    return extractedResult;
  }

  // Final attempts: AI-powered JSON repair
  if (repairAttempts > 0) {
    if (debug) {
      console.info('[Parser] Attempting AI repair for malformed JSON');
    }

    for (let attempt = 1; attempt <= repairAttempts; attempt++) {
      const repairResult = await repairJsonResponse(text, schema, ctx, { maxTokens, debug });
      if (repairResult.ok) {
        return repairResult;
      }

      if (debug) {
        console.info(`[Parser] Repair attempt ${attempt}/${repairAttempts} failed`);
      }
    }
  }

  return Failure(`Failed to parse AI response: ${directResult.error}`);
}

/**
 * Repair malformed JSON response using AI
 */
export async function repairJsonResponse<T>(
  brokenJson: string,
  schema: z.ZodSchema<T>,
  ctx: ToolContext,
  options: { maxTokens?: number; debug?: boolean } = {},
): Promise<Result<T>> {
  const { maxTokens = 256, debug = false } = options;

  try {
    // Build repair prompt
    const repairPrompt = `Fix this malformed JSON to match the required schema. Return only valid JSON, no explanation.

Original response:
${brokenJson}

Required schema: ${getSchemaDescription(schema)}

Fixed JSON:`;

    const repairMessages = await buildMessages({
      basePrompt: repairPrompt,
      topic: 'knowledge_enhancement', // Using knowledge enhancement topic for repair
      tool: 'response-parser',
      environment: 'development',
    });

    const mcpMessages = toMCPMessages(repairMessages);

    if (debug) {
      console.info('[Parser] Sending repair request to AI');
    }

    const response = await ctx.sampling.createMessage({
      messages: mcpMessages.messages,
      maxTokens,
      modelPreferences: {
        hints: [{ name: 'json-repair' }],
        intelligencePriority: 0.8,
        costPriority: 0.8, // Higher cost priority for simple repair
      },
    });

    const repairedText = response.content?.[0]?.text || '';
    if (!repairedText.trim()) {
      return Failure('AI repair returned empty response');
    }

    return await tryDirectParse(repairedText, schema, debug);
  } catch (error) {
    return Failure(`JSON repair error: ${extractErrorMessage(error)}`);
  }
}

/**
 * Try parsing text directly as JSON
 */
async function tryDirectParse<T>(
  text: string,
  schema: z.ZodSchema<T>,
  debug: boolean,
): Promise<Result<T>> {
  try {
    const trimmed = text.trim();
    if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) {
      return Failure('Text does not appear to be JSON');
    }

    const parsed = JSON.parse(trimmed);
    const validated = schema.parse(parsed);

    if (debug) {
      console.info('[Parser] Direct JSON parse successful');
    }

    return Success(validated);
  } catch (error) {
    if (error instanceof z.ZodError) {
      return Failure(`Schema validation failed: ${error.issues.map((e) => e.message).join(', ')}`);
    }
    return Failure(`JSON parse failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Try extracting JSON from markdown code blocks or other formats
 */
async function tryExtractedParse<T>(
  text: string,
  schema: z.ZodSchema<T>,
  debug: boolean,
): Promise<Result<T>> {
  // Try various extraction patterns
  const patterns = [
    // Markdown code blocks
    /```(?:json)?\s*([^`]+)\s*```/,
    // JSON wrapped in other text (limit depth to prevent catastrophic backtracking)
    /\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}/,
    // Array wrapped in other text (limit depth to prevent catastrophic backtracking)
    /\[[^\]]*(?:\[[^\]]*\][^\]]*)*\]/,
  ];

  for (const pattern of patterns) {
    const match = text.match(pattern);
    if (match?.[1] || match?.[0]) {
      const extracted = match[1] || match[0];
      const result = await tryDirectParse(extracted, schema, debug);
      if (result.ok) {
        if (debug) {
          console.info('[Parser] JSON extraction successful');
        }
        return result;
      }
    }
  }

  return Failure('No valid JSON found in text');
}

/**
 * Get human-readable schema description for repair prompts
 */
function getSchemaDescription(schema: z.ZodSchema<unknown>): string {
  try {
    // Try to get schema shape if available
    if ('shape' in schema && typeof schema.shape === 'object') {
      const shape = schema.shape as Record<string, unknown>;
      const fields = Object.keys(shape).map((key) => {
        const field = shape[key];
        let type = 'unknown';

        if (field && typeof field === 'object' && '_def' in field) {
          const def = (field as { _def: { typeName: string } })._def;
          switch (def.typeName) {
            case 'ZodString':
              type = 'string';
              break;
            case 'ZodNumber':
              type = 'number';
              break;
            case 'ZodBoolean':
              type = 'boolean';
              break;
            case 'ZodArray':
              type = 'array';
              break;
            case 'ZodObject':
              type = 'object';
              break;
          }
        }

        return `${key}: ${type}`;
      });

      return `{ ${fields.join(', ')} }`;
    }

    return 'JSON object matching expected schema';
  } catch {
    return 'Valid JSON object';
  }
}

/**
 * Utility to extract JSON content from various text formats
 */
export function extractJsonFromText(text: string): Result<unknown> {
  const patterns = [
    // Direct JSON
    /^\s*(\{[^}]*\})\s*$/,
    /^\s*(\[[^\]]*\])\s*$/,
    // Markdown code blocks
    /```(?:json)?\s*([^`]+)\s*```/,
    // JSON in mixed content (limited nesting to prevent catastrophic backtracking)
    /(\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\})/,
    /(\[[^\]]*(?:\[[^\]]*\][^\]]*)*\])/,
  ];

  for (const pattern of patterns) {
    const match = text.match(pattern);
    if (match?.[1]) {
      try {
        const parsed = JSON.parse(match[1]);
        return Success(parsed);
      } catch {
        continue;
      }
    }
  }

  return Failure('No valid JSON found in text');
}
