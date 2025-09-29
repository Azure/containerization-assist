/**
 * Prompt Engine - Structured prompt building with role-based message composition.
 *
 * This module provides a unified interface for building AI prompts with:
 * - Role separation (system, developer, user)
 * - Selective knowledge injection with budgeting
 * - Policy constraint integration
 * - Output contract specification
 */

import {
  type AIMessage,
  type AIMessages,
  type BuildPromptParams,
  type OutputContract,
  type PromptEnvelope,
  type Result,
  type Topic,
  Success,
  Failure,
} from '@/types/index';
import type { KnowledgeSnippet } from '@/knowledge/schemas';
import { getKnowledgeSnippets } from '@/knowledge/matcher';

/**
 * Options for knowledge selection.
 */
interface KnowledgeSelectionOptions {
  environment: string;
  tool: string;
  language?: string;
  framework?: string;
  maxChars?: number;
  maxSnippets?: number;
}

/**
 * Options for message building.
 */
interface MessageBuildOptions {
  includeMetadata?: boolean;
  forceSystemRole?: boolean;
  forceDeveloperRole?: boolean;
}

/**
 * Builds developer message with output contract.
 *
 * @param contract - Optional output contract specification
 * @returns Developer message string or undefined
 */
function buildDeveloperMessage(contract?: OutputContract): string | undefined {
  if (!contract) {
    return undefined;
  }

  const parts: string[] = [];

  parts.push(`Output strictly as JSON matching schema "${contract.name}".`);

  if (contract.description) {
    parts.push(contract.description);
  }

  parts.push('No commentary outside JSON structure.');

  return parts.join(' ');
}

/**
 * Selects and weights knowledge snippets for inclusion.
 *
 * @param topic - Topic to search for
 * @param options - Selection options
 * @returns Promise resolving to selected snippets
 */
async function selectKnowledgeSnippets(
  topic: Topic,
  options: KnowledgeSelectionOptions,
): Promise<KnowledgeSnippet[]> {
  try {
    const snippets = await getKnowledgeSnippets(topic, {
      environment: options.environment,
      tool: options.tool,
      maxChars: options.maxChars || 3000,
      ...(options.maxSnippets !== undefined && { maxSnippets: options.maxSnippets }),
    });

    return snippets;
  } catch (error) {
    console.warn('Failed to retrieve knowledge snippets:', error);
    return [];
  }
}

/**
 * Formats knowledge snippets for inclusion in prompt.
 *
 * @param snippets - Knowledge snippets to format
 * @returns Formatted knowledge text
 */
function formatKnowledgeText(snippets: KnowledgeSnippet[]): string {
  if (!snippets || snippets.length === 0) {
    return '';
  }

  const header = '\n\nRelevant knowledge:';
  const items = snippets.map((s) => {
    // Include source/category if available for context
    const prefix = s.source ? `[${s.source}] ` : '';
    return `- ${prefix}${s.text}`;
  });

  return `${header}\n${items.join('\n')}`;
}

/**
 * Builds user message with base prompt and knowledge.
 *
 * @param basePrompt - Base prompt text
 * @param knowledgeText - Formatted knowledge text
 * @returns Combined user message
 */
function buildUserMessage(basePrompt: string, knowledgeText: string): string {
  return basePrompt + knowledgeText;
}

/**
 * Creates an AI message object.
 *
 * @param role - Message role
 * @param text - Message text
 * @returns AIMessage object
 */
function createMessage(
  role: 'system' | 'developer' | 'user' | 'assistant',
  text: string,
): AIMessage {
  return {
    role,
    content: [{ type: 'text', text }],
  };
}

/**
 * Truncates text if it exceeds the maximum length.
 *
 * @param text - Text to potentially truncate
 * @param maxLength - Maximum allowed length
 * @returns Truncated text with indicator if truncated
 */
function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, maxLength)}\n[truncated]`;
}

/**
 * Main entry point for building structured AI messages.
 *
 * @param params - Parameters for building the prompt
 * @param options - Optional message building options
 * @returns Promise resolving to AIMessages structure
 */
export async function buildMessages(
  params: BuildPromptParams,
  options?: MessageBuildOptions & { maxLength?: number },
): Promise<AIMessages> {
  const messages: AIMessage[] = [];
  const maxLength = options?.maxLength || 10000;

  if (options?.forceSystemRole) {
    messages.push(createMessage('system', ''));
  }

  // Build developer message with output contract
  const developerText = buildDeveloperMessage(params.contract);
  if (developerText || options?.forceDeveloperRole) {
    const truncatedDeveloperText = truncateText(developerText || '', maxLength);
    messages.push(createMessage('developer', truncatedDeveloperText));
  }

  // Select and format knowledge
  const knowledgeSnippets = await selectKnowledgeSnippets(params.topic, {
    environment: params.environment,
    tool: params.tool,
    ...(params.language && { language: params.language }),
    ...(params.framework && { framework: params.framework }),
    ...(params.knowledgeBudget !== undefined && { maxChars: params.knowledgeBudget }),
  });
  const knowledgeText = formatKnowledgeText(knowledgeSnippets);

  // Build user message
  const userText = buildUserMessage(params.basePrompt, knowledgeText);
  const truncatedUserText = truncateText(userText, maxLength);
  messages.push(createMessage('user', truncatedUserText));

  return { messages };
}

/**
 * Builds a prompt envelope with structured roles and metadata.
 *
 * @param params - Parameters for building the prompt
 * @returns Promise resolving to PromptEnvelope
 */
export async function buildPromptEnvelope(
  params: BuildPromptParams,
): Promise<Result<PromptEnvelope>> {
  try {
    // Build developer message
    const developerText = buildDeveloperMessage(params.contract);

    // Select knowledge
    const knowledgeSnippets = await selectKnowledgeSnippets(params.topic, {
      environment: params.environment,
      tool: params.tool,
      ...(params.language && { language: params.language }),
      ...(params.framework && { framework: params.framework }),
      ...(params.knowledgeBudget !== undefined && { maxChars: params.knowledgeBudget }),
    });
    const knowledgeText = formatKnowledgeText(knowledgeSnippets);

    // Build user message
    const userText = buildUserMessage(params.basePrompt, knowledgeText);

    const envelope: PromptEnvelope = {
      ...(developerText && { developer: developerText }),
      user: userText,
      metadata: {
        tool: params.tool,
        environment: params.environment,
        topic: params.topic,
        knowledgeCount: knowledgeSnippets.length,
      },
    };

    return Success(envelope);
  } catch (error) {
    return Failure(`Failed to build prompt envelope: ${error}`);
  }
}

/**
 * Estimates the character count of messages.
 *
 * @param messages - AI messages to estimate
 * @returns Total character count
 */
export function estimateMessageSize(messages: AIMessages): number {
  let total = 0;

  for (const message of messages.messages) {
    if (typeof message.content === 'string') {
      total += message.content.length;
    } else if (Array.isArray(message.content)) {
      for (const block of message.content) {
        if (block.text) {
          total += block.text.length;
        }
      }
    }
  }

  return total;
}

/**
 * Validates that messages meet basic requirements.
 *
 * @param messages - Messages to validate
 * @returns Result indicating success or validation errors
 */
export function validateMessages(messages: AIMessages): Result<void> {
  if (!messages.messages || messages.messages.length === 0) {
    return Failure('No messages provided');
  }

  // Must have at least one user message
  const hasUserMessage = messages.messages.some((m: AIMessage) => m.role === 'user');
  if (!hasUserMessage) {
    return Failure('Messages must include at least one user message');
  }

  // Check for empty content
  for (const message of messages.messages) {
    if (
      !message.content ||
      (typeof message.content === 'string' && message.content.trim() === '') ||
      (Array.isArray(message.content) && message.content.length === 0)
    ) {
      return Failure(`Empty content in ${message.role} message`);
    }
  }

  return Success(undefined);
}
