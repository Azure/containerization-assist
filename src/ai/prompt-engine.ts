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
  Success,
  Failure,
} from '@/types/index';
import type { KnowledgeSnippet } from '@/knowledge/schemas';
import { getKnowledgeSnippets } from '@/knowledge/matcher';
import { buildPolicyConstraints } from '@/config/policy-prompt';

/**
 * Options for knowledge selection.
 */
interface KnowledgeSelectionOptions {
  environment: string;
  tool: string;
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
 * Builds policy constraints for the system role.
 *
 * @param tool - Tool name for context
 * @param environment - Environment (e.g., 'production', 'development')
 * @returns Array of policy constraint strings
 */
function buildSystemMessage(tool: string, environment: string): string | undefined {
  try {
    const constraints = buildPolicyConstraints({ tool, environment });

    if (!constraints || constraints.length === 0) {
      return undefined;
    }

    return [
      'You must follow these organizational policies:',
      ...constraints.map((c: string) => `- ${c}`),
    ].join('\n');
  } catch (error) {
    // If policy loading fails, return undefined to omit system message
    console.warn('Failed to load policy constraints:', error);
    return undefined;
  }
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
  topic: string,
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
 * Main entry point for building structured AI messages.
 *
 * @param params - Parameters for building the prompt
 * @param options - Optional message building options
 * @returns Promise resolving to AIMessages structure
 */
export async function buildMessages(
  params: BuildPromptParams,
  options?: MessageBuildOptions,
): Promise<AIMessages> {
  const messages: AIMessage[] = [];

  // Build system message with policies
  const systemText = buildSystemMessage(params.tool, params.environment);
  if (systemText || options?.forceSystemRole) {
    messages.push(createMessage('system', systemText || ''));
  }

  // Build developer message with output contract
  const developerText = buildDeveloperMessage(params.contract);
  if (developerText || options?.forceDeveloperRole) {
    messages.push(createMessage('developer', developerText || ''));
  }

  // Select and format knowledge
  const knowledgeSnippets = await selectKnowledgeSnippets(params.topic, {
    environment: params.environment,
    tool: params.tool,
    ...(params.knowledgeBudget !== undefined && { maxChars: params.knowledgeBudget }),
  });
  const knowledgeText = formatKnowledgeText(knowledgeSnippets);

  // Build user message
  const userText = buildUserMessage(params.basePrompt, knowledgeText);
  messages.push(createMessage('user', userText));

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
    // Build system message
    const systemText = buildSystemMessage(params.tool, params.environment);

    // Build developer message
    const developerText = buildDeveloperMessage(params.contract);

    // Select knowledge
    const knowledgeSnippets = await selectKnowledgeSnippets(params.topic, {
      environment: params.environment,
      tool: params.tool,
      ...(params.knowledgeBudget !== undefined && { maxChars: params.knowledgeBudget }),
    });
    const knowledgeText = formatKnowledgeText(knowledgeSnippets);

    // Build user message
    const userText = buildUserMessage(params.basePrompt, knowledgeText);

    // Count constraints (approximate by counting lines)
    const policyCount = systemText ? systemText.split('\n').length - 1 : 0;

    const envelope: PromptEnvelope = {
      ...(systemText && { system: systemText }),
      ...(developerText && { developer: developerText }),
      user: userText,
      metadata: {
        tool: params.tool,
        environment: params.environment,
        topic: params.topic,
        knowledgeCount: knowledgeSnippets.length,
        policyCount,
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

/**
 * Converts AIMessages to MCP-compatible TextMessage format.
 *
 * MCP protocol only supports 'user' and 'assistant' roles, so we need to:
 * - Combine system and developer messages into the first user message
 * - Ensure all messages have the proper content array format
 *
 * @param aiMessages - Messages from the prompt engine
 * @returns MCP-compatible TextMessage array
 */
export function toMCPMessages(aiMessages: AIMessages): {
  messages: Array<{ role: 'user' | 'assistant'; content: Array<{ type: 'text'; text: string }> }>;
} {
  const messages = aiMessages.messages;
  const mcpMessages: Array<{
    role: 'user' | 'assistant';
    content: Array<{ type: 'text'; text: string }>;
  }> = [];

  // Collect system and developer messages
  const systemMessages: string[] = [];

  for (const message of messages) {
    if (message.role === 'system' || message.role === 'developer') {
      const text =
        typeof message.content === 'string' ? message.content : message.content[0]?.text || '';
      if (text) {
        systemMessages.push(text);
      }
    } else if (message.role === 'user') {
      // Combine system messages with the first user message
      let userText =
        typeof message.content === 'string' ? message.content : message.content[0]?.text || '';

      if (systemMessages.length > 0) {
        userText = `${systemMessages.join('\n\n')}\n\n${userText}`;
        systemMessages.length = 0; // Clear after using
      }

      mcpMessages.push({
        role: 'user',
        content: [{ type: 'text', text: userText }],
      });
    } else if (message.role === 'assistant') {
      const text =
        typeof message.content === 'string' ? message.content : message.content[0]?.text || '';

      mcpMessages.push({
        role: 'assistant',
        content: [{ type: 'text', text }],
      });
    }
  }

  return { messages: mcpMessages };
}
