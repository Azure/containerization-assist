/**
 * Text Processing Utilities
 *
 * Utility functions for processing AI responses and text content,
 * particularly for cleaning up code generation responses.
 */

/**
 * Strips code fences and noise from AI-generated content
 *
 * This function removes common formatting artifacts from AI responses:
 * - Code fence markers (```language and ```)
 * - Leading/trailing whitespace
 * - Language specifiers in fence markers
 *
 * @param text - The text content to clean
 * @returns Cleaned text with fences and noise removed
 *
 * @example
 * ```typescript
 * const response = "```dockerfile\nFROM node:18\nWORKDIR /app\n```";
 * const cleaned = stripFencesAndNoise(response);
 * // Result: "FROM node:18\nWORKDIR /app"
 * ```
 */
export const stripFencesAndNoise = (text: string): string => {
  return text
    .replace(/^```[a-z]*\n?/i, '')
    .replace(/```$/, '')
    .trim();
};

/**
 * Validates that text content looks like a Dockerfile
 *
 * Performs basic validation to ensure content appears to be valid Dockerfile content:
 * - Must contain a FROM instruction
 * - Should not be empty after cleaning
 * - Should contain typical Dockerfile instructions
 *
 * @param content - The content to validate
 * @returns True if content appears to be a valid Dockerfile
 *
 * @example
 * ```typescript
 * const dockerfile = "FROM node:18\nWORKDIR /app\nCOPY . .";
 * const isValid = isValidDockerfileContent(dockerfile);
 * // Result: true
 * ```
 */
export const isValidDockerfileContent = (content: string): boolean => {
  const cleaned = content.trim();

  if (!cleaned) {
    return false;
  }

  // Must have a FROM instruction (case insensitive)
  const hasFrom = /^\s*FROM\s+\S+/im.test(cleaned) || /\nFROM\s+\S+/im.test(cleaned);

  return hasFrom;
};

/**
 * Extracts the base image from Dockerfile content
 *
 * Finds and extracts the base image specification from FROM instructions.
 * Handles multi-stage builds by returning the first FROM instruction.
 *
 * @param dockerfileContent - The Dockerfile content to analyze
 * @returns The base image string, or null if no FROM found
 *
 * @example
 * ```typescript
 * const dockerfile = "FROM node:18-alpine\nWORKDIR /app";
 * const baseImage = extractBaseImage(dockerfile);
 * // Result: "node:18-alpine"
 * ```
 */
export const extractBaseImage = (dockerfileContent: string): string | null => {
  const fromMatch = dockerfileContent.match(/^\s*FROM\s+(\S+)/im);
  return fromMatch?.[1] ?? null;
};

/**
 * Validates that text content looks like valid Kubernetes manifest(s)
 *
 * Performs basic validation for YAML Kubernetes manifests:
 * - Must contain apiVersion and kind fields
 * - Should be valid YAML structure
 * - Should contain typical Kubernetes resource fields
 *
 * @param content - The content to validate
 * @returns True if content appears to be valid Kubernetes YAML
 *
 * @example
 * ```typescript
 * const manifest = `
 * apiVersion: apps/v1
 * kind: Deployment
 * metadata:
 *   name: my-app
 * `;
 * const isValid = isValidKubernetesContent(manifest);
 * // Result: true
 * ```
 */
export const isValidKubernetesContent = (content: string): boolean => {
  const cleaned = content.trim();

  if (!cleaned) {
    return false;
  }

  // Must have apiVersion and kind (basic Kubernetes resource requirements)
  const hasApiVersion =
    /^\s*apiVersion:\s*\S+/im.test(cleaned) || /\napiVersion:\s*\S+/im.test(cleaned);
  const hasKind = /^\s*kind:\s*\S+/im.test(cleaned) || /\nkind:\s*\S+/im.test(cleaned);

  return hasApiVersion && hasKind;
};
