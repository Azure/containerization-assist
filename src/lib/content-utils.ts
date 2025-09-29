/**
 * Content Processing Utilities
 *
 * Centralized utilities for processing AI responses, text content, and token estimation.
 * Consolidates string operations from across the codebase into a single module.
 */

import { Success, Failure, type Result } from '@/types';
import { extractJsonFromText } from '@/mcp/ai/response-parser';

/**
 * Extract JSON content from various text formats (re-exported from response-parser)
 */
export { extractJsonFromText } from '@/mcp/ai/response-parser';

/**
 * Sanitize response text by removing common AI response artifacts
 *
 * Removes markdown formatting, extra whitespace, and common response prefixes
 * while preserving the core content structure.
 *
 * @param text - The text to sanitize
 * @returns Sanitized text content
 */
export function sanitizeResponseText(text: string): string {
  if (!text || typeof text !== 'string') {
    return '';
  }

  let sanitized = text;

  // Remove common AI response prefixes
  const prefixPatterns = [
    /^(?:Here'?s?|Here is|This is)\s+(?:the|a|an)?\s*/i,
    /^(?:I'll|I will)\s+.*?:\s*/i,
    /^(?:Let me|Allow me to)\s+.*?:\s*/i,
    /^(?:Based on|According to)\s+.*?[,:]\s*/i,
  ];

  for (const pattern of prefixPatterns) {
    sanitized = sanitized.replace(pattern, '');
  }

  // Remove markdown formatting while preserving structure
  sanitized = sanitized
    // Remove bold/italic markers but keep content
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .replace(/__([^_]+)__/g, '$1')
    .replace(/_([^_]+)_/g, '$1')
    // Normalize line breaks
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    // Remove excessive whitespace but preserve paragraph breaks
    .replace(/[ \t]+/g, ' ')
    .replace(/\n[ \t]+/g, '\n')
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    // Trim start and end
    .trim();

  return sanitized;
}

/**
 * Estimate token count for text content
 *
 * Uses a simple heuristic: ~4 characters per token for English text,
 * with adjustments for common patterns.
 *
 * @param text - The text to analyze
 * @returns Estimated token count
 */
export function estimateTokenCount(text: string): number {
  if (!text || typeof text !== 'string') {
    return 0;
  }

  // Base calculation: 4 characters per token (rough average for English)
  let tokenCount = Math.ceil(text.length / 4);

  // Adjust for whitespace (counts less)
  const whitespaceCount = (text.match(/\s/g) || []).length;
  tokenCount -= Math.floor(whitespaceCount * 0.25);

  // Adjust for common tokens
  const commonPatterns = [
    // Common words count as single tokens
    /\b(?:the|and|or|but|in|on|at|to|for|of|with|by)\b/gi,
    // Punctuation typically doesn't add much
    /[.,;:!?]/g,
    // JSON structure tokens
    /[{}[\]]/g,
  ];

  let adjustments = 0;
  for (const pattern of commonPatterns) {
    const matches = text.match(pattern);
    if (matches) {
      adjustments += matches.length * 0.1; // Small positive adjustment
    }
  }

  return Math.max(1, Math.floor(tokenCount + adjustments));
}

/**
 * Truncate text while preserving structure
 *
 * Intelligently truncates text at sentence or paragraph boundaries
 * when possible, preserving JSON structure for structured content.
 *
 * @param text - The text to truncate
 * @param maxLength - Maximum character length
 * @returns Truncated text with preserved structure
 */
export function truncatePreservingStructure(text: string, maxLength: number): string {
  if (!text || typeof text !== 'string' || text.length <= maxLength) {
    return text;
  }

  // If it looks like JSON, try to preserve structure
  if (text.trim().startsWith('{') || text.trim().startsWith('[')) {
    const jsonResult = extractJsonFromText(text);
    if (jsonResult.ok) {
      try {
        const stringified = JSON.stringify(jsonResult.value, null, 2);
        if (stringified.length <= maxLength) {
          return stringified;
        }
        // If JSON is too long, fall back to compact form
        const compact = JSON.stringify(jsonResult.value);
        if (compact.length <= maxLength) {
          return compact;
        }
        // If still too long, truncate at maxLength - 3 and add "..."
        return `${compact.substring(0, maxLength - 3)}...`;
      } catch {
        // Fall through to regular truncation
      }
    }
  }

  // For regular text, find good breakpoints
  const truncated = text.substring(0, maxLength);

  // Try to break at sentence boundaries
  const sentenceBreak = truncated.match(/^(.*[.!?])\s/);
  if (sentenceBreak?.[1] && sentenceBreak[1].length > maxLength * 0.7) {
    return sentenceBreak[1];
  }

  // Try to break at paragraph boundaries
  const paragraphBreak = truncated.lastIndexOf('\n\n');
  if (paragraphBreak > maxLength * 0.5) {
    return text.substring(0, paragraphBreak);
  }

  // Try to break at word boundaries
  const wordBreak = truncated.lastIndexOf(' ');
  if (wordBreak > maxLength * 0.8) {
    return `${text.substring(0, wordBreak)}...`;
  }

  // Last resort: hard truncation with ellipsis
  return `${text.substring(0, maxLength - 3)}...`;
}

/**
 * Extract structured content from mixed text
 *
 * Attempts to extract and parse structured content (JSON, YAML, etc.)
 * from mixed text that may contain explanations and formatting.
 *
 * @param text - The text containing structured content
 * @param type - Expected content type ('json', 'yaml', 'auto')
 * @returns Result containing extracted structured content
 */
export function extractStructuredContent(
  text: string,
  type: 'json' | 'yaml' | 'auto' = 'auto',
): Result<unknown> {
  if (!text || typeof text !== 'string') {
    return Failure('Empty or invalid text provided');
  }

  // For JSON content
  if (type === 'json' || type === 'auto') {
    const jsonResult = extractJsonFromText(text);
    if (jsonResult.ok) {
      return jsonResult;
    }
  }

  // For YAML content (basic implementation)
  if (type === 'yaml' || type === 'auto') {
    // Look for YAML-like content (lines with key: value pairs)
    const yamlPattern = /```(?:yaml|yml)?\s*([\s\S]*?)\s*```/;
    const yamlMatch = text.match(yamlPattern);

    if (yamlMatch?.[1]) {
      try {
        // Basic YAML-like parsing (very simple, mainly for detection)
        const yamlText = yamlMatch[1].trim();
        if (yamlText.includes(':') && !yamlText.startsWith('{')) {
          return Success(yamlText); // Return as string for now
        }
      } catch {
        // Continue to other parsing attempts
      }
    }
  }

  return Failure(`No structured content found for type: ${type}`);
}

/**
 * Normalize whitespace in text content
 *
 * Standardizes whitespace while preserving intentional formatting
 * like code blocks and structured content.
 *
 * @param text - The text to normalize
 * @returns Text with normalized whitespace
 */
export function normalizeWhitespace(text: string): string {
  if (!text || typeof text !== 'string') {
    return '';
  }

  // Check if this looks like structured content that should preserve formatting
  const isStructured =
    text.trim().startsWith('{') ||
    text.trim().startsWith('[') ||
    text.includes('```') ||
    text.includes('    '); // Indented code-like content

  if (isStructured) {
    // For structured content, only normalize excessive blank lines
    return text
      .replace(/\r\n/g, '\n') // Normalize line endings
      .replace(/\r/g, '\n')
      .replace(/\n{4,}/g, '\n\n\n'); // Max 3 consecutive newlines
  }

  // For regular text, normalize more aggressively
  return text
    .replace(/\r\n/g, '\n') // Normalize line endings
    .replace(/\r/g, '\n')
    .replace(/[ \t]+/g, ' ') // Multiple spaces/tabs to single space
    .replace(/\n[ \t]+/g, '\n') // Remove leading whitespace on lines
    .replace(/[ \t]+\n/g, '\n') // Remove trailing whitespace on lines
    .replace(/\n{3,}/g, '\n\n') // Max 2 consecutive newlines
    .trim();
}

/**
 * Check if text appears to contain code
 *
 * Uses heuristics to determine if text content appears to be code
 * based on common code patterns and syntax.
 *
 * @param text - The text to analyze
 * @returns True if text appears to contain code
 */
export function appearsToBeCode(text: string): boolean {
  if (!text || typeof text !== 'string') {
    return false;
  }

  const codeIndicators = [
    // Common code syntax
    /\{[\s\S]*\}/, // Curly braces
    /\[[\s\S]*\]/, // Square brackets with content
    /^\s*(?:FROM|RUN|COPY|ADD|WORKDIR|EXPOSE|ENV|CMD|ENTRYPOINT)\b/im, // Dockerfile instructions
    /^\s*(?:apiVersion|kind|metadata):/im, // Kubernetes YAML
    /[a-zA-Z_]\w*\s*[=:]\s*[^=]/, // Assignment patterns
    /(?:function|class|interface|const|let|var)\s+\w+/i, // Programming keywords
    /#!\//, // Shebang
    /\/\*[\s\S]*?\*\//, // Multi-line comments
    /\/\/.*$/m, // Single-line comments
  ];

  let indicatorCount = 0;
  for (const pattern of codeIndicators) {
    if (pattern.test(text)) {
      indicatorCount++;
    }
  }

  // If we found multiple indicators, it's probably code
  return indicatorCount >= 2;
}
